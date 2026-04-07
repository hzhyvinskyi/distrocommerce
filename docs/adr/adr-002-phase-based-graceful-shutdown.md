# ADR-002 — Phase-Based Graceful Shutdown


**Status:** Accepted

**Context:** Services have multiple components that must shut down in dependency order — HTTP/gRPC servers must stop accepting traffic before workers, workers before producers, producers before connections.

**Decision:** `pkg/graceful` phase-based runner:
- Phases execute sequentially in ascending order
- Hooks within a phase run concurrently (WaitGroup)
- Single global deadline shared across all phases — matches `terminationGracePeriodSeconds`
- `NotifyOnShutdown(Notifier)` — calls `MarkNotReady()` before any phase runs, making `/readyz` return 503 so k8s stops routing before server socket closes
- `GRPCShutdownHook` — `GracefulStop()` with fallback to `Stop()` on deadline
- `HTTPShutdownHook` — `Shutdown()` with `Close()` on `DeadlineExceeded` to force-close sockets

```
Signal → MarkNotReady() → Phase 1: HTTP+gRPC → Phase 2: workers → Phase 3: producers+tracer
```

**Critical design:** `context.WithTimeout(context.Background(), timeout)` — not parented on signal context which is already cancelled. All hooks receive a fresh deadline context.

**Consequences:** k8s-safe shutdown sequence. No hook can starve later phases. Panic in any hook is recovered and converted to error — remaining phases still execute.
