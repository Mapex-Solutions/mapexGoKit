package shutdown

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// --- groupByPriority ---

func TestGroupByPriority_Empty(t *testing.T) {
	groups := groupByPriority(nil)
	if groups != nil {
		t.Fatalf("expected nil, got %v", groups)
	}
}

func TestGroupByPriority_SingleHook(t *testing.T) {
	hooks := []shutdownHook{{Name: "a", Priority: 0}}
	groups := groupByPriority(hooks)
	if len(groups) != 1 || len(groups[0]) != 1 {
		t.Fatalf("expected 1 group with 1 hook, got %d groups", len(groups))
	}
	if groups[0][0].Name != "a" {
		t.Fatalf("expected hook name 'a', got %q", groups[0][0].Name)
	}
}

func TestGroupByPriority_MultiplePriorities(t *testing.T) {
	hooks := []shutdownHook{
		{Name: "a", Priority: 0},
		{Name: "b", Priority: 0},
		{Name: "c", Priority: 3},
		{Name: "d", Priority: 5},
		{Name: "e", Priority: 5},
	}
	groups := groupByPriority(hooks)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	if len(groups[0]) != 2 {
		t.Fatalf("expected group[0] with 2 hooks (P0), got %d", len(groups[0]))
	}
	if len(groups[1]) != 1 {
		t.Fatalf("expected group[1] with 1 hook (P3), got %d", len(groups[1]))
	}
	if len(groups[2]) != 2 {
		t.Fatalf("expected group[2] with 2 hooks (P5), got %d", len(groups[2]))
	}
}

// --- Register ---

func TestRegisterFunc_AddsHook(t *testing.T) {
	sm := New()
	sm.RegisterFunc("test", 3, func(_ context.Context) error { return nil })

	sm.mu.Lock()
	defer sm.mu.Unlock()
	if len(sm.hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(sm.hooks))
	}
	if sm.hooks[0].Name != "test" || sm.hooks[0].Priority != 3 {
		t.Fatalf("unexpected hook: name=%q priority=%d", sm.hooks[0].Name, sm.hooks[0].Priority)
	}
}

type mockShutdowner struct {
	called atomic.Bool
}

func (m *mockShutdowner) Shutdown(_ context.Context) error {
	m.called.Store(true)
	return nil
}

func TestRegister_ShutdownerInterface(t *testing.T) {
	sm := New()
	s := &mockShutdowner{}
	sm.Register("mock", 5, s)

	sm.mu.Lock()
	if len(sm.hooks) != 1 {
		sm.mu.Unlock()
		t.Fatalf("expected 1 hook, got %d", len(sm.hooks))
	}
	fn := sm.hooks[0].Fn
	sm.mu.Unlock()

	// Verify the registered function delegates to Shutdowner.Shutdown
	_ = fn(context.Background())
	if !s.called.Load() {
		t.Fatal("expected Shutdowner.Shutdown to be called")
	}
}

// --- WaitForSignal (signal-based integration tests) ---

func sendSignalAfter(d time.Duration) {
	go func() {
		time.Sleep(d)
		p, _ := os.FindProcess(os.Getpid())
		p.Signal(syscall.SIGINT)
	}()
}

func TestWaitForSignal_PriorityOrder(t *testing.T) {
	sm := New()

	var order []string
	var mu sync.Mutex

	record := func(name string) func(context.Context) error {
		return func(_ context.Context) error {
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
			return nil
		}
	}

	// Register out of order to verify sorting
	sm.RegisterFunc("p5-conn", 5, record("p5"))
	sm.RegisterFunc("p0-http", 0, record("p0"))
	sm.RegisterFunc("p3-flush", 3, record("p3"))

	sendSignalAfter(50 * time.Millisecond)
	sm.WaitForSignal(5 * time.Second)

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 {
		t.Fatalf("expected 3 hooks executed, got %d: %v", len(order), order)
	}
	if order[0] != "p0" || order[1] != "p3" || order[2] != "p5" {
		t.Fatalf("expected execution order [p0, p3, p5], got %v", order)
	}
}

func TestWaitForSignal_SamePriorityConcurrent(t *testing.T) {
	sm := New()

	var running atomic.Int32
	var maxConcurrent atomic.Int32

	makeHook := func() func(context.Context) error {
		return func(_ context.Context) error {
			cur := running.Add(1)
			// Update max concurrent using CAS
			for {
				old := maxConcurrent.Load()
				if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			running.Add(-1)
			return nil
		}
	}

	sm.RegisterFunc("a", 5, makeHook())
	sm.RegisterFunc("b", 5, makeHook())
	sm.RegisterFunc("c", 5, makeHook())

	sendSignalAfter(50 * time.Millisecond)
	sm.WaitForSignal(5 * time.Second)

	if maxConcurrent.Load() < 2 {
		t.Fatalf("expected concurrent execution (max >= 2), got %d", maxConcurrent.Load())
	}
}

func TestWaitForSignal_HookErrorDoesNotStopOthers(t *testing.T) {
	sm := New()

	var p5Called atomic.Bool

	sm.RegisterFunc("fail-p0", 0, func(_ context.Context) error {
		return errors.New("boom")
	})
	sm.RegisterFunc("conn-p5", 5, func(_ context.Context) error {
		p5Called.Store(true)
		return nil
	})

	sendSignalAfter(50 * time.Millisecond)
	sm.WaitForSignal(5 * time.Second)

	if !p5Called.Load() {
		t.Fatal("expected P5 hook to run despite P0 hook error")
	}
}

func TestWaitForSignal_TimeoutAbortsRemainingGroups(t *testing.T) {
	sm := New()

	var p5Called atomic.Bool

	sm.RegisterFunc("slow-p0", 0, func(_ context.Context) error {
		// Sleep longer than the total shutdown timeout
		time.Sleep(200 * time.Millisecond)
		return nil
	})
	sm.RegisterFunc("conn-p5", 5, func(_ context.Context) error {
		p5Called.Store(true)
		return nil
	})

	sendSignalAfter(50 * time.Millisecond)
	sm.WaitForSignal(100 * time.Millisecond) // Short timeout — P0 exceeds it

	if p5Called.Load() {
		t.Fatal("expected P5 hook to be skipped due to timeout")
	}
}

func TestWaitForSignal_NoHooks(t *testing.T) {
	sm := New()

	sendSignalAfter(50 * time.Millisecond)
	start := time.Now()
	sm.WaitForSignal(5 * time.Second)

	if time.Since(start) > 2*time.Second {
		t.Fatal("WaitForSignal with no hooks should complete quickly after signal")
	}
}

// --- IsShuttingDown ---

func TestIsShuttingDown_FalseByDefault(t *testing.T) {
	sm := New()
	if sm.IsShuttingDown() {
		t.Fatal("expected IsShuttingDown() to return false on a new ShutdownManager")
	}
}

func TestIsShuttingDown_TrueAfterTerminating(t *testing.T) {
	sm := New()
	sm.terminating.Store(true)
	if !sm.IsShuttingDown() {
		t.Fatal("expected IsShuttingDown() to return true after terminating flag set")
	}
}

func TestFlagSetBeforeHooksExecute(t *testing.T) {
	sm := New()

	var flagDuringHook atomic.Bool

	sm.RegisterFunc("check-flag", 0, func(_ context.Context) error {
		flagDuringHook.Store(sm.IsShuttingDown())
		return nil
	})

	sendSignalAfter(50 * time.Millisecond)
	sm.WaitForSignal(5 * time.Second)

	if !flagDuringHook.Load() {
		t.Fatal("expected IsShuttingDown() to be true inside hook — flag must be set BEFORE hooks execute")
	}
}
