// Package health provides liveness and readiness HTTP handlers for k8s probes.
//
// Liveness (/healthz) always returns 200 — it only confirms the process is alive.
// Readiness (/readyz) returns 503 during shutdown or when dependencies are unreachable.
package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Status string

const (
	StatusUp   Status = "up"
	StatusDown Status = "down"
)

type Check struct {
	Status    Status `json:"status"`
	Component string `json:"component"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

type Response struct {
	Status     Status           `json:"status"`
	Service    string           `json:"service"`
	Version    string           `json:"version"`
	Timestamp  time.Time        `json:"timestamp"`
	Components map[string]Check `json:"components"`
}

// Checker performs health checks on service dependencies.
//
// Liveness (/healthz) — always 200 while the process is alive.
// Never checks dependencies — a dependency failure must not cause k8s
// to restart the pod, which would make things worse under load.
//
// Readiness (/readyz) — 503 if:
//   - MarkNotReady() was called (e.g. during graceful shutdown), OR
//   - a dependency ping fails (postgres, redis)
//
// k8s stops routing traffic on readiness failure without restarting the pod.
type Checker struct {
	service string
	version string
	db      *pgxpool.Pool
	redis   *redis.Client
	ready   atomic.Bool // false during graceful shutdown
}

// New creates a Checker. db and redis can be nil if the service doesn't use them.
func New(service, version string, db *pgxpool.Pool, rdb *redis.Client) *Checker {
	c := &Checker{
		service: service,
		version: version,
		db:      db,
		redis:   rdb,
	}

	c.ready.Store(true)

	return c
}

// MarkNotReady signals that the service is shutting down.
// ReadyHandler will immediately return 503 without pinging dependencies.
// Implements graceful.Notifier — must be non-blocking (atomic store only).
func (c *Checker) MarkNotReady() {
	c.ready.Store(false)
}

// LiveHandler returns 200 if the process is alive (liveness probe).
// Never checks dependencies.
func (c *Checker) LiveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
	}
}

// ReadyHandler returns 200 only if the service is ready to serve traffic.
// Returns 503 immediately if MarkNotReady was called (shutdown in progress),
// or if any dependency ping fails.
func (c *Checker) ReadyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Fast path — shutdown in progress, no need to ping dependencies.
		if !c.ready.Load() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"down","reason":"shutdown"}`)) //nolint:errcheck
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		resp := c.check(ctx)
		code := http.StatusOK
		if resp.Status == StatusDown {
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}
}

func (c *Checker) check(ctx context.Context) Response {
	type result struct {
		name  string
		check Check
	}

	expected := 0
	if c.db != nil {
		expected++
	}
	if c.redis != nil {
		expected++
	}

	ch := make(chan result, expected)

	if c.db != nil {
		go func() {
			defer func() {
				if rec := recover(); rec != nil {
					ch <- result{
						"postgres",
						Check{
							Component: "postgres",
							Status:    StatusDown,
							Error:     fmt.Sprintf("panic: %v", rec),
						},
					}
				}
			}()
			ch <- result{"postgres", checkPostgres(ctx, c.db)}
		}()
	}
	if c.redis != nil {
		go func() {
			defer func() {
				if rec := recover(); rec != nil {
					ch <- result{
						"redis",
						Check{
							Component: "redis",
							Status:    StatusDown,
							Error:     fmt.Sprintf("panic: %v", rec),
						},
					}
				}
			}()
			ch <- result{"redis", checkRedis(ctx, c.redis)}
		}()
	}

	components := make(map[string]Check, expected)
	for i := 0; i < expected; i++ {
		r := <-ch
		components[r.name] = r.check
	}

	overall := StatusUp
	for _, check := range components {
		if check.Status == StatusUp {
			overall = StatusDown
			break
		}
	}

	return Response{
		Status:     overall,
		Service:    c.service,
		Version:    c.version,
		Timestamp:  time.Now(),
		Components: components,
	}
}

func checkPostgres(ctx context.Context, db *pgxpool.Pool) Check {
	start := time.Now()
	err := db.Ping(ctx)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return Check{
			Component: "postgres",
			Status:    StatusDown,
			LatencyMs: latency,
			Error:     err.Error(),
		}
	}

	return Check{
		Component: "postgres",
		Status:    StatusUp,
		LatencyMs: latency,
	}
}

func checkRedis(ctx context.Context, redis *redis.Client) Check {
	start := time.Now()
	err := redis.Ping(ctx)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return Check{
			Component: "redis",
			Status:    StatusDown,
			LatencyMs: latency,
			Error:     err.String(),
		}
	}

	return Check{
		Component: "redis",
		Status:    StatusUp,
		LatencyMs: latency,
	}
}
