package graceful_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/hzhyvinskyi/distrocommerce/pkg/graceful"
)

// triggerShutdown returns a context that is already cancelled,
// causing Run to proceed immediately without waiting for an OS signal.
func triggerShutdown() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func nopLogger() *zap.Logger {
	return zap.NewNop()
}

// Add / NotifyOnShutdown guards

func TestAdd_ReturnsErrAlreadyStarted(t *testing.T) {
	r := graceful.New(nopLogger(), time.Second)

	// Run in background, wait until it picks up the cancelled ctx
	done := make(chan error, 1)
	go func() { done <- r.Run(triggerShutdown()) }()
	<-done // wait for Run to finish

	err := r.Add(1, func(_ context.Context) error { return nil })
	if !errors.Is(err, graceful.ErrAlreadyStarted) {
		t.Fatalf("expected ErrAlreadyStarted, got %v", err)
	}
}

func TestNotifyOnShutdown_ReturnsErrAlreadyStarted(t *testing.T) {
	r := graceful.New(nopLogger(), time.Second)

	done := make(chan error, 1)
	go func() { done <- r.Run(triggerShutdown()) }()
	<-done

	notifier := &mockNotifier{}
	err := r.NotifyOnShutdown(notifier)
	if !errors.Is(err, graceful.ErrAlreadyStarted) {
		t.Fatalf("expected ErrAlreadyStarted, got %v", err)
	}
}

// Notifier

func TestNotifyOnShutdown_CalledBeforePhases(t *testing.T) {
	r := graceful.New(nopLogger(), time.Second)

	var order []string
	notifier := &mockNotifier{fn: func() { order = append(order, "notifier") }}
	if err := r.NotifyOnShutdown(notifier); err != nil {
		t.Fatal(err)
	}
	if err := r.Add(1, func(_ context.Context) error {
		order = append(order, "hook")
		return nil
	}, "hook"); err != nil {
		t.Fatal(err)
	}

	if err := r.Run(triggerShutdown()); err != nil {
		t.Fatal(err)
	}

	if len(order) != 2 || order[0] != "notifier" || order[1] != "hook" {
		t.Fatalf("expected [notifier hook], got %v", order)
	}
}

func TestNotifyOnShutdown_MultipleNotifiers(t *testing.T) {
	r := graceful.New(nopLogger(), time.Second)

	var count atomic.Int32
	for i := 0; i < 3; i++ {
		n := &mockNotifier{fn: func() { count.Add(1) }}
		if err := r.NotifyOnShutdown(n); err != nil {
			t.Fatal(err)
		}
	}

	if err := r.Run(triggerShutdown()); err != nil {
		t.Fatal(err)
	}

	if count.Load() != 3 {
		t.Fatalf("expected 3 notifiers called, got %d", count.Load())
	}
}

// Phase ordering

func TestRun_PhasesExecuteInOrder(t *testing.T) {
	r := graceful.New(nopLogger(), time.Second)

	// Phases are sequential — no mutex needed, append is never concurrent.
	var order []int

	for _, phase := range []int{3, 1, 2} {
		phase := phase
		if err := r.Add(phase, func(_ context.Context) error {
			order = append(order, phase)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}

	if err := r.Run(triggerShutdown()); err != nil {
		t.Fatal(err)
	}

	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Fatalf("expected [1 2 3], got %v", order)
	}
}

func TestRun_HooksInSamePhaseRunConcurrently(t *testing.T) {
	r := graceful.New(nopLogger(), time.Second)

	// Both hooks block on a gate. If they ran sequentially, deadlock would occur.
	gate := make(chan struct{})
	var reached atomic.Int32

	for i := 0; i < 2; i++ {
		if err := r.Add(1, func(_ context.Context) error {
			reached.Add(1)
			<-gate
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}

	go func() {
		// Close gate once both hooks have reached the barrier
		for reached.Load() < 2 {
			time.Sleep(time.Millisecond)
		}
		close(gate)
	}()

	if err := r.Run(triggerShutdown()); err != nil {
		t.Fatal(err)
	}
}

func TestRun_Phase2StartsAfterPhase1Completes(t *testing.T) {
	r := graceful.New(nopLogger(), time.Second)

	var phase1Done atomic.Bool

	if err := r.Add(1, func(_ context.Context) error {
		time.Sleep(20 * time.Millisecond)
		phase1Done.Store(true)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.Add(2, func(_ context.Context) error {
		if !phase1Done.Load() {
			return errors.New("phase 2 started before phase 1 finished")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := r.Run(triggerShutdown()); err != nil {
		t.Fatal(err)
	}
}

// Error handling

func TestRun_CollectsAllErrors(t *testing.T) {
	r := graceful.New(nopLogger(), time.Second)

	err1 := errors.New("error-1")
	err2 := errors.New("error-2")

	if err := r.Add(1, func(_ context.Context) error { return err1 }); err != nil {
		t.Fatal(err)
	}
	if err := r.Add(1, func(_ context.Context) error { return err2 }); err != nil {
		t.Fatal(err)
	}

	got := r.Run(triggerShutdown())

	if !errors.Is(got, err1) {
		t.Errorf("expected err1 in result, got %v", got)
	}
	if !errors.Is(got, err2) {
		t.Errorf("expected err2 in result, got %v", got)
	}
}

func TestRun_FailedPhase1_Phase2StillRuns(t *testing.T) {
	r := graceful.New(nopLogger(), time.Second)

	var phase2ran atomic.Bool

	if err := r.Add(1, func(_ context.Context) error {
		return errors.New("phase1 error")
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.Add(2, func(_ context.Context) error {
		phase2ran.Store(true)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	_ = r.Run(triggerShutdown())

	if !phase2ran.Load() {
		t.Fatal("phase 2 must run even if phase 1 has errors")
	}
}

// Deadline

func TestRun_HookReceivesDeadlineContext(t *testing.T) {
	r := graceful.New(nopLogger(), 50*time.Millisecond)

	var ctxErr error
	if err := r.Add(1, func(ctx context.Context) error {
		// Block until context deadline
		<-ctx.Done()
		ctxErr = ctx.Err()
		return ctxErr
	}); err != nil {
		t.Fatal(err)
	}

	_ = r.Run(triggerShutdown())

	if !errors.Is(ctxErr, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", ctxErr)
	}
}

func TestRun_HookPanic_RecoveredAsError(t *testing.T) {
	r := graceful.New(nopLogger(), time.Second)

	var phase2ran atomic.Bool

	if err := r.Add(1, func(_ context.Context) error {
		panic("something went wrong")
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.Add(2, func(_ context.Context) error {
		phase2ran.Store(true)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	err := r.Run(triggerShutdown())

	if err == nil {
		t.Fatal("expected error from panic, got nil")
	}
	if !phase2ran.Load() {
		t.Fatal("phase 2 must run even if phase 1 hook panicked")
	}
}

func TestGRPCShutdownHook_GracefulStop(t *testing.T) {
	srv := &mockGRPCServer{}
	hook := graceful.GRPCShutdownHook(srv)

	ctx := context.Background()
	if err := hook(ctx); err != nil {
		t.Fatal(err)
	}
	if !srv.gracefulStopped {
		t.Fatal("GracefulStop was not called")
	}
	if srv.hardStopped {
		t.Fatal("Stop should not be called on clean shutdown")
	}
}

func TestGRPCShutdownHook_FallsBackToStop(t *testing.T) {
	srv := &mockGRPCServer{block: true}
	hook := graceful.GRPCShutdownHook(srv)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := hook(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
	if !srv.hardStopped {
		t.Fatal("Stop must be called when deadline exceeded")
	}
}

func TestHTTPShutdownHook_Clean(t *testing.T) {
	srv := &http.Server{Addr: "127.0.0.1:0"}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln) //nolint:errcheck

	hook := graceful.HTTPShutdownHook(srv)
	ctx := context.Background()
	if err := hook(ctx); err != nil {
		t.Fatalf("clean shutdown failed: %v", err)
	}
}

func TestHTTPShutdownHook_ForceClosesOnDeadline(t *testing.T) {
	// Handler that never returns — simulates slow/hung client.
	// handlerReached is closed once the request enters the handler.
	handlerReached := make(chan struct{})
	srv := &http.Server{
		Addr: "127.0.0.1:0",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(handlerReached)
			select {} // block forever
		}),
	}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln) //nolint:errcheck

	// Send a request to trigger the hung handler.
	go func() {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			return
		}
		conn.Write([]byte("GET / HTTP/1.1\r\nHost: test\r\n\r\n")) //nolint:errcheck
		defer conn.Close()                                         //nolint:errcheck
	}()

	// Wait until the handler is actually running before starting shutdown.
	select {
	case <-handlerReached:
	case <-time.After(2 * time.Second):
		t.Fatal("handler never reached")
	}

	hook := graceful.HTTPShutdownHook(srv)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	err = hook(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}

	// Verify server is closed — new connection must fail.
	_, dialErr := net.DialTimeout("tcp", ln.Addr().String(), 50*time.Millisecond)
	if dialErr == nil {
		t.Fatal("server should be closed after force-close")
	}
}

type mockNotifier struct {
	fn func()
}

func (m *mockNotifier) MarkNotReady() {
	if m.fn != nil {
		m.fn()
	}
}

type mockGRPCServer struct {
	block           bool
	gracefulStopped bool
	hardStopped     bool
}

func (m *mockGRPCServer) GracefulStop() {
	if m.block {
		// Block forever — simulates hung in-flight requests
		select {}
	}
	m.gracefulStopped = true
}

func (m *mockGRPCServer) Stop() {
	m.hardStopped = true
}
