package mongoManager

import (
	"testing"
	"time"
)

func TestRecord_CircularBuffer(t *testing.T) {
	tracker := newTracker(4, 150, 500)

	// Fill buffer beyond capacity to test wrap-around
	tracker.record(10 * time.Millisecond)
	tracker.record(20 * time.Millisecond)
	tracker.record(30 * time.Millisecond)
	tracker.record(40 * time.Millisecond)
	tracker.record(50 * time.Millisecond) // wraps to slot 0

	// Slot 0 should now be 50 (overwritten from 10)
	if tracker.samples[0] != 50 {
		t.Errorf("expected slot 0 = 50, got %d", tracker.samples[0])
	}
	// Slot 1 should still be 20
	if tracker.samples[1] != 20 {
		t.Errorf("expected slot 1 = 20, got %d", tracker.samples[1])
	}
}

func TestComputeP99_Normal(t *testing.T) {
	tracker := newTracker(100, 150, 500)

	// Fill with latencies well below threshold
	for i := 0; i < 100; i++ {
		tracker.record(time.Duration(10+i) * time.Millisecond) // 10-109ms
	}

	// Run 3 windows to allow any transition
	for i := 0; i < 3; i++ {
		tracker.computeP99()
	}

	mode := BackpressureMode(tracker.mode.Load())
	if mode != Normal {
		t.Errorf("expected Normal, got %s", mode.String())
	}

	p99 := tracker.p99.Load()
	if p99 <= 0 {
		t.Errorf("expected positive P99, got %d", p99)
	}
}

func TestComputeP99_Throttled(t *testing.T) {
	tracker := newTracker(100, 150, 500)

	// Fill buffer: all samples at 200ms → P99 = 200ms (above 150ms throttled, below 500ms backoff)
	for i := 0; i < 100; i++ {
		tracker.record(200 * time.Millisecond)
	}

	// Need 3 consecutive windows above threshold to transition
	for i := 0; i < 3; i++ {
		tracker.computeP99()
	}

	mode := BackpressureMode(tracker.mode.Load())
	if mode != Throttled {
		t.Errorf("expected Throttled, got %s (P99=%dms)", mode.String(), tracker.p99.Load())
	}
}

func TestComputeP99_Backoff(t *testing.T) {
	tracker := newTracker(100, 150, 500)

	// Fill buffer with latencies above backoff threshold
	for i := 0; i < 100; i++ {
		tracker.record(600 * time.Millisecond)
	}

	// 3 consecutive windows
	for i := 0; i < 3; i++ {
		tracker.computeP99()
	}

	mode := BackpressureMode(tracker.mode.Load())
	if mode != Backoff {
		t.Errorf("expected Backoff, got %s", mode.String())
	}
}

func TestComputeP99_Recovery(t *testing.T) {
	tracker := newTracker(100, 150, 500)

	// First, drive to Throttled state
	for i := 0; i < 100; i++ {
		tracker.record(200 * time.Millisecond)
	}
	for i := 0; i < 3; i++ {
		tracker.computeP99()
	}

	if BackpressureMode(tracker.mode.Load()) != Throttled {
		t.Fatal("expected Throttled before recovery test")
	}

	// Now fill buffer with low-latency samples → immediate recovery
	for i := 0; i < 100; i++ {
		tracker.record(10 * time.Millisecond)
	}
	tracker.computeP99()

	mode := BackpressureMode(tracker.mode.Load())
	if mode != Normal {
		t.Errorf("expected Normal after recovery, got %s", mode.String())
	}
}

func TestComputeP99_EmptyBuffer(t *testing.T) {
	tracker := newTracker(100, 150, 500)

	tracker.computeP99()

	if tracker.p99.Load() != 0 {
		t.Errorf("expected P99=0 for empty buffer, got %d", tracker.p99.Load())
	}
	if BackpressureMode(tracker.mode.Load()) != Normal {
		t.Error("expected Normal for empty buffer")
	}
}

func TestDisabled_ReturnsNormal(t *testing.T) {
	m := &MongoManager{} // bp == nil

	if m.GetBackpressureMode() != Normal {
		t.Error("expected Normal when backpressure disabled")
	}

	if m.WriteP99() != 0 {
		t.Error("expected WriteP99=0 when backpressure disabled")
	}
}

func TestRecordWriteLatency_NoOp(t *testing.T) {
	m := &MongoManager{} // bp == nil

	// Should not panic
	m.RecordWriteLatency(100 * time.Millisecond)
}

func TestBackpressureMode_String(t *testing.T) {
	tests := []struct {
		mode BackpressureMode
		want string
	}{
		{Normal, "Normal"},
		{Throttled, "Throttled"},
		{Backoff, "Backoff"},
		{BackpressureMode(99), "Unknown"},
	}

	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("BackpressureMode(%d).String() = %s, want %s", tt.mode, got, tt.want)
		}
	}
}
