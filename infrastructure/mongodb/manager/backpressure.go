package mongoManager

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync/atomic"
	"time"

	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// BackpressureMode represents the current MongoDB write pressure level.
// Callers should query this mode and adapt their behavior accordingly.
type BackpressureMode int32

const (
	// Normal indicates MongoDB is responding within acceptable latency.
	Normal BackpressureMode = 0

	// Throttled indicates elevated P99 write latency.
	// Callers should reduce batch sizes to ease pressure.
	Throttled BackpressureMode = 1

	// Backoff indicates critically high P99 write latency.
	// Callers should reduce batch sizes further and add a pause before writing.
	Backoff BackpressureMode = 2
)

const (
	// defaultBackpressureWindow is the default circular buffer capacity.
	defaultBackpressureWindow = 1000

	// defaultThrottledThresholdMs is the default P99 threshold for Throttled mode.
	defaultThrottledThresholdMs int64 = 150

	// defaultBackoffThresholdMs is the default P99 threshold for Backoff mode.
	defaultBackoffThresholdMs int64 = 500

	// computeInterval is how often the background goroutine recalculates P99.
	computeInterval = 5 * time.Second

	// windowsToTransition is the number of consecutive windows above a threshold
	// required before transitioning to a higher mode.
	windowsToTransition = 3
)

// backpressureTracker monitors MongoDB write latencies using a lock-free
// circular buffer and periodically computes P99 to determine the current mode.
type backpressureTracker struct {
	samples    []int64      // circular buffer of write latencies (ms)
	size       int          // buffer capacity
	pos        atomic.Int64 // next write position (wraps around)
	p99        atomic.Int64 // last computed P99 in ms
	mode       atomic.Int32 // current BackpressureMode
	thresholds [2]int64     // [throttledMs, backoffMs]

	// aboveCount tracks consecutive windows above each threshold.
	// Index 0 = throttled, index 1 = backoff.
	aboveCount [2]int
}

// newTracker creates a backpressure tracker with the given buffer size and thresholds.
func newTracker(size int, throttledMs, backoffMs int64) *backpressureTracker {
	return &backpressureTracker{
		samples:    make([]int64, size),
		size:       size,
		thresholds: [2]int64{throttledMs, backoffMs},
	}
}

// record stores a write latency sample in the circular buffer.
// This is lock-free (~50ns overhead) and safe for concurrent callers.
func (t *backpressureTracker) record(d time.Duration) {
	ms := d.Milliseconds()
	idx := t.pos.Add(1) - 1
	t.samples[idx%int64(t.size)] = ms
}

// computeP99 copies the current buffer, sorts it, and extracts the P99 value.
// It then evaluates thresholds and transitions the mode accordingly.
//
// Transition rules:
//   - 3 consecutive windows above a threshold → transition UP (Normal→Throttled or Throttled→Backoff)
//   - P99 drops below throttled threshold → immediate reset to Normal
func (t *backpressureTracker) computeP99() {
	// Copy non-zero samples from the buffer
	buf := make([]int64, 0, t.size)
	for i := 0; i < t.size; i++ {
		v := atomic.LoadInt64(&t.samples[i])
		if v > 0 {
			buf = append(buf, v)
		}
	}

	if len(buf) == 0 {
		t.p99.Store(0)
		t.mode.Store(int32(Normal))
		t.aboveCount = [2]int{0, 0}
		return
	}

	sort.Slice(buf, func(i, j int) bool { return buf[i] < buf[j] })

	idx := int(math.Ceil(float64(len(buf))*0.99)) - 1
	if idx < 0 {
		idx = 0
	}
	p99 := buf[idx]
	t.p99.Store(p99)

	// Evaluate thresholds
	if p99 >= t.thresholds[1] {
		// Above backoff threshold
		t.aboveCount[1]++
		t.aboveCount[0]++ // also above throttled
		if t.aboveCount[1] >= windowsToTransition {
			t.mode.Store(int32(Backoff))
			return
		}
	} else if p99 >= t.thresholds[0] {
		// Above throttled but below backoff
		t.aboveCount[0]++
		t.aboveCount[1] = 0 // reset backoff counter
		if t.aboveCount[0] >= windowsToTransition {
			t.mode.Store(int32(Throttled))
			return
		}
	} else {
		// Below all thresholds → immediate recovery
		t.aboveCount = [2]int{0, 0}
		t.mode.Store(int32(Normal))
		return
	}

	// Not enough consecutive windows yet — keep current mode
}

// start runs the P99 computation loop. It blocks until ctx is cancelled.
func (t *backpressureTracker) start(ctx context.Context) {
	ticker := time.NewTicker(computeInterval)
	defer ticker.Stop()

	logger.Info("[INFRA:MONGODB] Backpressure tracker started")

	for {
		select {
		case <-ctx.Done():
			logger.Info("[INFRA:MONGODB] Backpressure tracker stopped")
			return
		case <-ticker.C:
			t.computeP99()

			mode := BackpressureMode(t.mode.Load())
			if mode != Normal {
				logger.Warn(fmt.Sprintf("[INFRA:MONGODB] Backpressure mode=%s P99=%dms", mode.String(), t.p99.Load()))
			}
		}
	}
}

// String returns a human-readable representation of the backpressure mode.
func (m BackpressureMode) String() string {
	switch m {
	case Normal:
		return "Normal"
	case Throttled:
		return "Throttled"
	case Backoff:
		return "Backoff"
	default:
		return "Unknown"
	}
}
