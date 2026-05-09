# shutdown — Ordered, signal-driven graceful shutdown

A small `ShutdownManager` that collects named cleanup hooks with priorities, waits for `SIGTERM`/`SIGINT`, then runs the hooks in **ascending priority order** with a configurable timeout. Hooks at the same priority run **concurrently**.

> Package name: `shutdown` (directory: `shutdown/`).

## Surface

### Constructor

```go
func New() *ShutdownManager
```

### Types

```go
type Shutdowner interface {
    Shutdown(ctx context.Context) error
}

type ShutdownManager struct { /* unexported */ }
```

### Registration

```go
func (m *ShutdownManager) Register(name string, priority int, s Shutdowner)
func (m *ShutdownManager) RegisterFunc(name string, priority int, fn func(ctx context.Context) error)
```

`Register` is sugar for `RegisterFunc(name, priority, s.Shutdown)` — use whichever is more natural for the caller.

Internally, every hook is stored as `{Name, Priority, Fn}` and the slice is guarded by a mutex.

### Lifecycle

```go
func (m *ShutdownManager) WaitForSignal(timeout time.Duration)
func (m *ShutdownManager) IsShuttingDown() bool
func (m *ShutdownManager) SetShuttingDown(v bool) // testing
```

`WaitForSignal` blocks the goroutine until `SIGTERM` or `SIGINT` arrives. It is the entry point that:

1. Sets the `terminating` flag (`IsShuttingDown() == true`).
2. Logs `[SHUTDOWN] Received <signal>, starting graceful shutdown (timeout: <d>)…`.
3. Snapshots the registered hooks, sorts by priority ascending, groups consecutive hooks with the same priority.
4. For each group: launches one goroutine per hook with a shared `context.WithTimeout`, waits for the group to finish, then advances to the next priority.
5. If the context expires mid-shutdown, logs `[SHUTDOWN] Timeout reached, aborting remaining hooks` and breaks.
6. On per-hook completion, logs success (`[SHUTDOWN] <name> done (<duration>)`) or failure (`[SHUTDOWN] <name> failed (<duration>): <err>` at Warn).
7. On overall completion, logs `[SHUTDOWN] Graceful shutdown complete (<total>)`.

The function returns when the loop ends. **It does not call `os.Exit` itself** — the caller decides what to do after.

## Recommended priority bands

The doc-comments suggest these bands. They are guidelines, not constants:

| Priority | Concern | Why this order |
|---|---|---|
| **P0** | HTTP server | Stop accepting new requests; drain in-flight ones. |
| **P1** | Message consumers | Stop fetching new messages; finish the current batch. |
| **P2** | Background goroutines | Tickers, sweep loops. |
| **P3** | Publishers / flush | Make sure pending messages are sent before connections close. |
| **P4** | Caches | TieredCache, in-memory caches. |
| **P5** | Connections | MongoDB, Redis, NATS, ClickHouse. |

Same-priority hooks run concurrently, so put the slowest "logical layer" at its own priority and let everything inside it parallelise.

## Usage

```go
sm := shutdown.New()

// HTTP first
sm.RegisterFunc("http", 0, func(ctx context.Context) error {
    return server.Shutdown(ctx)
})

// Then consumers
sm.Register("nats-events", 1, eventConsumer) // implements Shutdowner

// Then background work
sm.RegisterFunc("sweep-loop", 2, sweeper.Stop)

// Last: connections (run in parallel because they all live at P5)
sm.Register("mongo",     5, mongoMgr)
sm.Register("redis-app", 5, redisApp)
sm.Register("nats-bus",  5, natsClient)

sm.WaitForSignal(20 * time.Second)
// process the rest of main(), or os.Exit(0)
```

## Tested behaviours

- `RegisterFunc` and `Register` accept hooks; `IsShuttingDown` reflects the state set explicitly via `SetShuttingDown(true)` (used in tests).
- `groupByPriority` (`internals.go`) groups consecutive hooks by equal priority. Behaviour relies on the input being already sorted ascending.
- Concurrent execution within a priority group is enforced via a `sync.WaitGroup`; the next group only starts after the previous one finishes.
- A hook that returns an error is logged but does **not** abort the rest of the group — the manager is best-effort, not transactional.

## Notes

- `WaitForSignal` reads exactly **one** signal then returns; it does not loop. Re-arming requires a fresh `WaitForSignal` call (rare in practice).
- The signal set is `SIGTERM`, `SIGINT`. Other signals are not handled.
- `SetShuttingDown(true)` is exposed for tests only — there is no automatic equivalent for unit tests that want to drive the lifecycle without sending real signals.
