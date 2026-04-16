package shutdown

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// IsShuttingDown returns true if the process received a termination signal.
func (m *ShutdownManager) IsShuttingDown() bool {
	return m.terminating.Load()
}

// SetShuttingDown sets the termination flag programmatically. Used for testing.
func (m *ShutdownManager) SetShuttingDown(v bool) {
	m.terminating.Store(v)
}

// Register adds a Shutdowner component to the shutdown sequence.
func (m *ShutdownManager) Register(name string, priority int, s Shutdowner) {
	m.RegisterFunc(name, priority, s.Shutdown)
}

// RegisterFunc adds a cleanup function to the shutdown sequence.
func (m *ShutdownManager) RegisterFunc(name string, priority int, fn func(ctx context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = append(m.hooks, shutdownHook{Name: name, Priority: priority, Fn: fn})
}

// WaitForSignal blocks until SIGTERM or SIGINT is received, then executes
// all registered hooks in priority order with the given timeout.
// Hooks with the same priority run concurrently.
// After all hooks complete (or timeout), the process exits.
func (m *ShutdownManager) WaitForSignal(timeout time.Duration) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	received := <-sig
	m.terminating.Store(true)

	logger.Info(fmt.Sprintf("[SHUTDOWN] Received %s, starting graceful shutdown (timeout: %s)...", received, timeout))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	m.mu.Lock()
	hooks := make([]shutdownHook, len(m.hooks))
	copy(hooks, m.hooks)
	m.mu.Unlock()

	// Sort by priority ascending
	sort.Slice(hooks, func(i, j int) bool {
		return hooks[i].Priority < hooks[j].Priority
	})

	// Group by priority and execute
	groups := groupByPriority(hooks)

	start := time.Now()
	for _, group := range groups {
		if ctx.Err() != nil {
			logger.Warn("[SHUTDOWN] Timeout reached, aborting remaining hooks")
			break
		}

		var wg sync.WaitGroup
		for _, hook := range group {
			wg.Add(1)
			go func(h shutdownHook) {
				defer wg.Done()
				hookStart := time.Now()
				if err := h.Fn(ctx); err != nil {
					logger.Warn(fmt.Sprintf("[SHUTDOWN] %s failed (%s): %v", h.Name, time.Since(hookStart), err))
				} else {
					logger.Info(fmt.Sprintf("[SHUTDOWN] %s done (%s)", h.Name, time.Since(hookStart)))
				}
			}(hook)
		}
		wg.Wait()
	}

	logger.Info(fmt.Sprintf("[SHUTDOWN] Graceful shutdown complete (%s)", time.Since(start)))
}
