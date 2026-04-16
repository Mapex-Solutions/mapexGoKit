package shutdown

import (
	"context"
	"sync"
	"sync/atomic"
)

// Shutdowner is implemented by any component that needs graceful cleanup.
// Infrastructure clients (MongoDB, Redis, NATS, etc.) can implement this
// to participate in the ordered shutdown sequence.
type Shutdowner interface {
	Shutdown(ctx context.Context) error
}

// shutdownHook holds a named cleanup function with an execution priority.
type shutdownHook struct {
	Name     string
	Priority int
	Fn       func(ctx context.Context) error
}

// ShutdownManager manages the ordered graceful shutdown sequence.
// Hooks are executed by priority (ascending): P0 first, P5 last.
// Hooks with the same priority run concurrently.
//
// Recommended priorities:
//
//	P0 — HTTP Server (stop accepting, drain in-flight requests)
//	P1 — Message Consumers (stop fetching, finish current batch)
//	P2 — Background goroutines (tickers, sweep loops)
//	P3 — Publishers/flush (ensure pending messages are sent)
//	P4 — Caches (TieredCache, in-memory caches)
//	P5 — Connections (MongoDB, Redis, NATS, ClickHouse)
type ShutdownManager struct {
	hooks       []shutdownHook
	mu          sync.Mutex
	terminating atomic.Bool
}
