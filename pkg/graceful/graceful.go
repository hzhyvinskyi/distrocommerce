// Package graceful provides phase-based concurrent shutdown with OS signal handling.
//
// Phases execute sequentially in ascending order. Within each phase all hooks
// run concurrently under a single shared deadline. The deadline is created once
// at shutdown start and shared across all phases — matching the k8s
// terminationGracePeriodSeconds budget.
//
// Typical layout for a gRPC service:
//
//	Phase 1 — stop accepting traffic : HTTP server, gRPC server  (concurrent)
//	Phase 2 — stop background workers: Kafka consumers, relay    (concurrent)
//	Phase 3 — flush / close          : Kafka producer, DB, tracer (concurrent)
package graceful

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.uber.org/zap"
)

var ErrAlreadyStarted = errors.New("shutdown: cannot add hook after runner has started")

// Hook is a shutdown function for a single component.
type Hook func(ctx context.Context) error

// Notifier is implemented by any component that needs to know shutdown has started.
// Called synchronously before any phase runs — before HTTP server closes,
// before any hook executes. Use this to signal readiness probes to return 503
// so k8s stops routing new requests before the server socket closes.
//
// MarkNotReady MUST be non-blocking — it must not acquire locks,
// make network calls, or perform any I/O. A blocking implementation
// will stall the entire shutdown before any phase runs.
// Typical implementation: atomic.Bool.Store(false).
type Notifier interface {
	MarkNotReady()
}

// Runner manages phase-based graceful shutdown.
type Runner struct {
	logger    *zap.Logger
	timeout   time.Duration
	notifiers []Notifier
	mu        sync.Mutex
	phases    map[int][]namedHook
	started   atomic.Bool
}

type namedHook struct {
	name string
	fn   Hook
}

// New creates a new Runner.
// timeout is the total budget for the entire shutdown sequence — set this to
// terminationGracePeriodSeconds minus a small buffer (e.g. 2s).
func New(logger *zap.Logger, timeout time.Duration) *Runner {
	return &Runner{
		logger:  logger,
		timeout: timeout,
		phases:  make(map[int][]namedHook),
	}
}

// NotifyOnShutdown registers a Notifier whose MarkNotReady is called
// synchronously as the very first step of shutdown — before any phase runs.
// This ensures readiness probes return 503 before the server socket closes,
// giving k8s time to drain in-flight requests.
// Returns ErrAlreadyStarted if called after Run has started.
func (r *Runner) NotifyOnShutdown(n Notifier) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started.Load() {
		return ErrAlreadyStarted
	}

	r.notifiers = append(r.notifiers, n)

	return nil
}

// Add registers a hook in the given phase.
// Lower order = earlier phase. Hooks within the same phase run concurrently.
// Returns ErrAlreadyStarted if called after Run has started.
// name is optional and used only for logging.
func (r *Runner) Add(order int, h Hook, name ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started.Load() {
		return ErrAlreadyStarted
	}

	n := ""
	if len(name) > 0 {
		n = name[0]
	}

	r.phases[order] = append(r.phases[order], namedHook{name: n, fn: h})

	return nil
}

// Run blocks until SIGINT/SIGTERM (or ctx cancellation), then executes all
// phases in ascending order. Returns joined errors from all hooks.
//
// Run MUST be called exactly once. Multiple concurrent calls are not supported.
//
// A second SIGINT/SIGTERM during shutdown cancels the shutdown context
// immediately — all running hooks receive context.Canceled and exit.
func (r *Runner) Run(ctx context.Context) error {
	sigCtx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	r.logger.Info("shutdown runner started")
	<-sigCtx.Done()
	r.logger.Warn("shutdown signal received")

	// Snapshot phases and mark started — no more Add/NotifyOnShutdown calls allowed.
	r.mu.Lock()
	r.started.Store(true)
	snapshot := r.snapshotPhases()
	notifiers := make([]Notifier, len(r.notifiers))
	copy(notifiers, r.notifiers)
	r.mu.Unlock()

	// Notify all readiness components immediately — before any phase runs.
	// This makes /readyz return 503 so k8s stops routing new requests
	// before the server socket closes.
	if len(notifiers) > 0 {
		for _, n := range notifiers {
			n.MarkNotReady()
		}
		r.logger.Info("readiness marked not ready")
	}

	// Re-arm signal handling — a second SIGINT/SIGTERM cancels the shutdown
	// context immediately, interrupting all running hooks.
	// In k8s this never fires (only one SIGTERM is sent, then SIGKILL).
	// In local dev: Ctrl+C twice force-exits instead of hanging.
	forceCtx, forceStop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer forceStop()

	// One deadline for the entire shutdown — all phases share this budget.
	// Parented on forceCtx so a second signal cancels it immediately.
	shutdownCtx, cancel := context.WithTimeout(forceCtx, r.timeout)
	defer cancel()

	var allErrs []error
	for _, p := range snapshot {
		r.logger.Info("shutdown phase started",
			zap.Int("phase", p.order),
			zap.Int("hooks", len(p.hooks)),
		)

		if errs := r.runPhase(shutdownCtx, p.order, p.hooks); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}

		r.logger.Info("shutdown phase completed", zap.Int("phase", p.order))
	}

	r.logger.Info("shutdown completed")

	return errors.Join(allErrs...)
}

// phaseSnapshot is a sorted, immutable copy of one phase taken under lock.
type phaseSnapshot struct {
	order int
	hooks []namedHook
}

// snapshotPhases returns a sorted copy of all phases.
func (r *Runner) snapshotPhases() []phaseSnapshot {
	orders := make([]int, 0, len(r.phases))
	for k := range r.phases {
		orders = append(orders, k)
	}
	sort.Ints(orders)

	result := make([]phaseSnapshot, 0, len(orders))
	for _, order := range orders {
		hooks := make([]namedHook, len(r.phases[order]))
		copy(hooks, r.phases[order])
		result = append(result, phaseSnapshot{order: order, hooks: hooks})
	}

	return result
}

// runPhase executes all hooks in a phase concurrently under the shared ctx.
func (r *Runner) runPhase(ctx context.Context, order int, hooks []namedHook) []error {
	var (
		wg    sync.WaitGroup
		errCh = make(chan error, len(hooks))
	)

	for i, h := range hooks {
		wg.Add(1)
		go func(i int, h namedHook) {
			lbl := label(h.name, i)

			defer wg.Done()
			defer func() {
				if rec := recover(); rec != nil {
					err := fmt.Errorf("hook panic: %v", rec)
					r.logger.Error("hook panicked",
						zap.Int("phase", order),
						zap.String("hook", lbl),
						zap.Any("panic", rec),
					)
					errCh <- err
				}
			}()

			start := time.Now()
			r.logger.Info("hook shutting down",
				zap.Int("phase", order),
				zap.String("hook", lbl),
			)

			if err := h.fn(ctx); err != nil {
				r.logger.Error("hook failed",
					zap.Int("phase", order),
					zap.String("hook", lbl),
					zap.Error(err),
				)
				errCh <- err

				return
			}

			r.logger.Info("hook completed",
				zap.Int("phase", order),
				zap.String("hook", lbl),
				zap.Duration("duration", time.Since(start)),
			)
		}(i, h)
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	return errs
}

func label(name string, i int) string {
	if name != "" {
		return name
	}

	return fmt.Sprintf("hook-%d", i)
}

func HTTPShutdownHook(srv *http.Server) Hook {
	return func(ctx context.Context) error {
		err := srv.Shutdown(ctx)
		if errors.Is(err, context.DeadlineExceeded) {
			_ = srv.Close()
		}

		return err
	}
}

func GRPCShutdownHook(srv interface {
	GracefulStop()
	Stop()
}) Hook {
	return func(ctx context.Context) error {
		done := make(chan struct{})
		go func() {
			srv.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
			return nil
		case <-ctx.Done():
			srv.Stop()
			return ctx.Err()
		}
	}
}
