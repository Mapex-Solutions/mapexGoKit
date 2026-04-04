package natsModel

import (
	"time"
)

/**
RetryPolicy Methods
*/

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries: 5,
		Backoff: []time.Duration{
			1 * time.Second,
			5 * time.Second,
			30 * time.Second,
			2 * time.Minute,
			10 * time.Minute,
		},
		AckWait: 30 * time.Second,
	}
}

// GetBackoffDurations returns the backoff sequence for NATS consumer config
func (p *RetryPolicy) GetBackoffDurations() []time.Duration {
	if p == nil || len(p.Backoff) == 0 {
		return DefaultRetryPolicy().Backoff
	}
	return p.Backoff
}

// GetDelayForAttempt returns the delay for a specific delivery attempt.
// deliveryCount starts at 1 for first delivery.
func (p *RetryPolicy) GetDelayForAttempt(deliveryCount int) time.Duration {
	// GetBackoffDurations handles nil receiver
	backoff := p.GetBackoffDurations()

	// First delivery (deliveryCount=1) means no retry yet
	// Second delivery (deliveryCount=2) means first retry, use backoff[0]
	index := deliveryCount - 1
	if index < 0 {
		index = 0
	}
	if index >= len(backoff) {
		index = len(backoff) - 1
	}

	return backoff[index]
}

// GetMaxDeliver returns MaxRetries + 1 (for NATS MaxDeliver config)
// MaxDeliver includes the first delivery attempt
func (p *RetryPolicy) GetMaxDeliver() int {
	if p == nil || p.MaxRetries <= 0 {
		return 6 // Default: 5 retries + 1 initial = 6 max deliveries
	}
	return p.MaxRetries + 1
}

// GetAckWait returns the AckWait duration or default
func (p *RetryPolicy) GetAckWait() time.Duration {
	if p == nil || p.AckWait <= 0 {
		return 30 * time.Second
	}
	return p.AckWait
}

// ShouldRetry checks if the message should be retried based on delivery count
func (p *RetryPolicy) ShouldRetry(deliveryCount int) bool {
	if p == nil {
		// Use default MaxRetries (5)
		return deliveryCount <= 5
	}
	return deliveryCount <= p.MaxRetries
}
