package natsModel

import (
	"testing"
	"time"
)

/** DefaultRetryPolicy */

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()

	if policy == nil {
		t.Fatal("DefaultRetryPolicy() returned nil")
	}
	if policy.MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %d", policy.MaxRetries)
	}
	if len(policy.Backoff) != 5 {
		t.Fatalf("expected 5 backoff durations, got %d", len(policy.Backoff))
	}
	if policy.Backoff[0] != 1*time.Second {
		t.Errorf("expected backoff[0] = 1s, got %v", policy.Backoff[0])
	}
	if policy.Backoff[4] != 10*time.Minute {
		t.Errorf("expected backoff[4] = 10m, got %v", policy.Backoff[4])
	}
	if policy.AckWait != 30*time.Second {
		t.Errorf("expected AckWait 30s, got %v", policy.AckWait)
	}
}

/** GetDelayForAttempt */

func TestGetDelayForAttempt_FirstDelivery(t *testing.T) {
	policy := &RetryPolicy{
		Backoff: []time.Duration{1 * time.Second, 5 * time.Second, 30 * time.Second},
	}
	delay := policy.GetDelayForAttempt(1)
	if delay != 1*time.Second {
		t.Errorf("expected 1s for deliveryCount=1, got %v", delay)
	}
}

func TestGetDelayForAttempt_SecondDelivery(t *testing.T) {
	policy := &RetryPolicy{
		Backoff: []time.Duration{1 * time.Second, 5 * time.Second, 30 * time.Second},
	}
	delay := policy.GetDelayForAttempt(2)
	if delay != 5*time.Second {
		t.Errorf("expected 5s for deliveryCount=2, got %v", delay)
	}
}

func TestGetDelayForAttempt_BeyondBackoffLength(t *testing.T) {
	policy := &RetryPolicy{
		Backoff: []time.Duration{1 * time.Second, 5 * time.Second},
	}
	// deliveryCount=10 is well beyond backoff length, should clamp to last
	delay := policy.GetDelayForAttempt(10)
	if delay != 5*time.Second {
		t.Errorf("expected 5s (clamped to last), got %v", delay)
	}
}

func TestGetDelayForAttempt_ZeroDeliveryCount(t *testing.T) {
	policy := &RetryPolicy{
		Backoff: []time.Duration{1 * time.Second, 5 * time.Second},
	}
	delay := policy.GetDelayForAttempt(0)
	if delay != 1*time.Second {
		t.Errorf("expected 1s for deliveryCount=0 (clamped to 0), got %v", delay)
	}
}

func TestGetDelayForAttempt_NegativeDeliveryCount(t *testing.T) {
	policy := &RetryPolicy{
		Backoff: []time.Duration{1 * time.Second},
	}
	delay := policy.GetDelayForAttempt(-1)
	if delay != 1*time.Second {
		t.Errorf("expected 1s for negative deliveryCount, got %v", delay)
	}
}

func TestGetDelayForAttempt_NilPolicy(t *testing.T) {
	var policy *RetryPolicy
	// Should use default backoff
	delay := policy.GetDelayForAttempt(1)
	if delay != 1*time.Second {
		t.Errorf("expected 1s from default backoff, got %v", delay)
	}
}

/** ShouldRetry */

func TestShouldRetry_WithinLimit(t *testing.T) {
	policy := &RetryPolicy{MaxRetries: 5}
	if !policy.ShouldRetry(3) {
		t.Error("expected ShouldRetry=true for deliveryCount=3 with max=5")
	}
}

func TestShouldRetry_AtLimit(t *testing.T) {
	policy := &RetryPolicy{MaxRetries: 5}
	if !policy.ShouldRetry(5) {
		t.Error("expected ShouldRetry=true for deliveryCount=5 with max=5")
	}
}

func TestShouldRetry_BeyondLimit(t *testing.T) {
	policy := &RetryPolicy{MaxRetries: 5}
	if policy.ShouldRetry(6) {
		t.Error("expected ShouldRetry=false for deliveryCount=6 with max=5")
	}
}

func TestShouldRetry_NilPolicy(t *testing.T) {
	var policy *RetryPolicy
	// Default max retries is 5
	if !policy.ShouldRetry(5) {
		t.Error("expected ShouldRetry=true for deliveryCount=5 with nil policy (default=5)")
	}
	if policy.ShouldRetry(6) {
		t.Error("expected ShouldRetry=false for deliveryCount=6 with nil policy (default=5)")
	}
}

func TestShouldRetry_ZeroMaxRetries(t *testing.T) {
	policy := &RetryPolicy{MaxRetries: 0}
	if policy.ShouldRetry(1) {
		t.Error("expected ShouldRetry=false for deliveryCount=1 with max=0")
	}
}

/** GetBackoffDurations (additional tests) */

func TestGetBackoffDurations_EmptyBackoff(t *testing.T) {
	policy := &RetryPolicy{Backoff: []time.Duration{}}
	backoff := policy.GetBackoffDurations()
	if len(backoff) == 0 {
		t.Error("expected default backoff for empty Backoff slice")
	}
}

/** GetMaxDeliver (additional tests) */

func TestGetMaxDeliver_NegativeRetries(t *testing.T) {
	policy := &RetryPolicy{MaxRetries: -1}
	result := policy.GetMaxDeliver()
	if result != 6 {
		t.Errorf("expected default 6 for negative MaxRetries, got %d", result)
	}
}

/** GetAckWait (additional tests) */

func TestGetAckWait_NegativeValue(t *testing.T) {
	policy := &RetryPolicy{AckWait: -1 * time.Second}
	result := policy.GetAckWait()
	if result != 30*time.Second {
		t.Errorf("expected default 30s for negative AckWait, got %v", result)
	}
}

func TestGetAckWait_ZeroValue(t *testing.T) {
	policy := &RetryPolicy{AckWait: 0}
	result := policy.GetAckWait()
	if result != 30*time.Second {
		t.Errorf("expected default 30s for zero AckWait, got %v", result)
	}
}
