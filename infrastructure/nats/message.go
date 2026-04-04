package natsModel

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// TestMessageCallbacks allows unit tests to intercept Ack/Nack/Reject calls
// without requiring a real NATS connection. Only used via NewTestMessage.
type TestMessageCallbacks struct {
	OnAck    func() error
	OnNack   func(error) error
	OnReject func(string) error
}

// Message wraps nats.Msg with retry-aware methods.
// Users interact with this wrapper instead of the raw NATS message.
type Message struct {
	// Public fields (user can read)
	Data    []byte
	Headers map[string][]string
	Subject string

	// Tenant context (set by service after parsing, used for DLQ)
	OrgId          string
	PathKey        string
	EventTrackerId string

	// Metadata (read-only)
	DeliveryCount int
	Timestamp     time.Time

	// Internal (not exported)
	natsMsg       *nats.Msg
	consumer      *Consumer
	meta          *nats.MsgMetadata
	index         int // Position in batch
	testCallbacks *TestMessageCallbacks
}

// NewTestMessage creates a Message for unit testing.
// Test callbacks intercept Ack/Nack/Reject without requiring a real NATS connection.
// If callbacks is nil, the methods are no-ops that return nil.
func NewTestMessage(data []byte, index int, callbacks *TestMessageCallbacks) *Message {
	return &Message{
		Data:          data,
		index:         index,
		testCallbacks: callbacks,
	}
}

// newMessage creates a new Message wrapper from a NATS message
func newMessage(natsMsg *nats.Msg, consumer *Consumer, index int) (*Message, error) {
	meta, err := natsMsg.Metadata()
	if err != nil {
		return nil, fmt.Errorf("failed to get message metadata: %w", err)
	}

	// Extract headers
	headers := make(map[string][]string)
	if natsMsg.Header != nil {
		for key, values := range natsMsg.Header {
			headers[key] = values
		}
	}

	return &Message{
		Data:          natsMsg.Data,
		Headers:       headers,
		Subject:       natsMsg.Subject,
		DeliveryCount: int(meta.NumDelivered),
		Timestamp:     meta.Timestamp,
		natsMsg:       natsMsg,
		consumer:      consumer,
		meta:          meta,
		index:         index,
	}, nil
}

// Ack acknowledges successful processing.
// The message is removed from the stream.
func (m *Message) Ack() error {
	if m.testCallbacks != nil {
		if m.testCallbacks.OnAck != nil {
			return m.testCallbacks.OnAck()
		}
		return nil
	}
	if err := m.natsMsg.Ack(); err != nil {
		logger.Error(err, fmt.Sprintf("[INFRA:NATS] Failed to ACK message %d", m.index))
		return err
	}
	logger.Debug(fmt.Sprintf("[INFRA:NATS] Message %d ACKed successfully", m.index))
	return nil
}

// Nack signals processing failure.
// If RetryPolicy is configured:
//   - If max retries not reached: message is redelivered with backoff delay
//   - If max retries reached: message is sent to DLQ and ACKed
//
// If no RetryPolicy: message is immediately redelivered (standard NATS behavior)
func (m *Message) Nack(err error) error {
	if m.testCallbacks != nil {
		if m.testCallbacks.OnNack != nil {
			return m.testCallbacks.OnNack(err)
		}
		return nil
	}

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	// No retry policy - just NAK (immediate redelivery)
	if m.consumer.options.RetryPolicy == nil {
		logger.Warn(fmt.Sprintf("[INFRA:NATS] Message %d NAKed (no retry policy): %s", m.index, errMsg))
		return m.natsMsg.Nak()
	}

	policy := m.consumer.options.RetryPolicy

	// Check if max retries exceeded - send to DLQ
	if !policy.ShouldRetry(m.DeliveryCount) {
		logger.Warn(fmt.Sprintf("[INFRA:NATS] Message %d max retries (%d) exceeded, sending to DLQ: %s",
			m.index, policy.MaxRetries, errMsg))
		return m.sendToDLQAndAck(errMsg)
	}

	// Calculate delay and NAK with backoff
	delay := policy.GetDelayForAttempt(m.DeliveryCount)

	logger.Warn(fmt.Sprintf("[INFRA:NATS] Message %d attempt %d/%d failed, retry in %s: %s",
		m.index, m.DeliveryCount, policy.MaxRetries, delay, errMsg))

	return m.natsMsg.NakWithDelay(delay)
}

// Reject immediately sends the message to DLQ without retrying.
// Use this for fatal/unrecoverable errors (e.g., invalid JSON, schema mismatch).
func (m *Message) Reject(reason string) error {
	if m.testCallbacks != nil {
		if m.testCallbacks.OnReject != nil {
			return m.testCallbacks.OnReject(reason)
		}
		return nil
	}
	logger.Warn(fmt.Sprintf("[INFRA:NATS] Message %d rejected, sending to DLQ: %s", m.index, reason))
	return m.sendToDLQAndAck(reason)
}

// Term terminates the message processing.
// The message is discarded without retry and without DLQ.
// Use this when you want to silently drop the message.
func (m *Message) Term() error {
	if err := m.natsMsg.Term(); err != nil {
		logger.Error(err, fmt.Sprintf("[INFRA:NATS] Failed to TERM message %d", m.index))
		return err
	}
	logger.Debug(fmt.Sprintf("[INFRA:NATS] Message %d terminated (discarded)", m.index))
	return nil
}

// sendToDLQAndAck sends the message to DLQ and ACKs the original
func (m *Message) sendToDLQAndAck(errorMsg string) error {
	// Check if DLQ policy is configured
	if m.consumer.options.DLQPolicy == nil {
		logger.Warn(fmt.Sprintf("[INFRA:NATS] No DLQ policy configured, terminating message %d", m.index))
		return m.natsMsg.Term()
	}

	dlqPolicy := m.consumer.options.DLQPolicy

	// Extract headers as map[string]string
	headers := make(map[string]string)
	for k, v := range m.Headers {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	// Create DLQ message with tenant context and event tracking
	dlqMsg := NewDLQMessage(
		dlqPolicy,
		m.consumer.options.Durable,
		m.EventTrackerId,
		m.OrgId,
		m.PathKey,
		m.Subject,
		m.consumer.options.Stream,
		m.Data,
		headers,
		errorMsg,
		m.DeliveryCount,
		m.Timestamp,
	)

	// Marshal to JSON
	data, err := dlqMsg.ToJSON()
	if err != nil {
		logger.Error(err, fmt.Sprintf("[INFRA:NATS] DLQ failed to marshal message %d", m.index))
		return m.natsMsg.Nak() // Fallback to NAK
	}

	// Publish to DLQ
	dlqSubject := dlqPolicy.GetSubject()
	if err := m.consumer.client.Push(PushOptions{
		Subject: dlqSubject,
		Data:    data,
	}); err != nil {
		logger.Error(err, fmt.Sprintf("[INFRA:NATS] DLQ failed to publish to %s", dlqSubject))
		return m.natsMsg.Nak() // Fallback to NAK
	}

	logger.Warn(fmt.Sprintf("[INFRA:NATS] DLQ message %d sent to: %s (ID: %s)",
		m.index, dlqSubject, dlqMsg.ID))

	// ACK original message to remove from source stream
	return m.natsMsg.Ack()
}

// GetRetryInfo returns information about retries for logging/debugging
func (m *Message) GetRetryInfo() (currentAttempt int, maxRetries int, isLastAttempt bool) {
	if m.consumer == nil || m.consumer.options.RetryPolicy == nil {
		return m.DeliveryCount, 0, false
	}
	policy := m.consumer.options.RetryPolicy
	return m.DeliveryCount, policy.MaxRetries, m.DeliveryCount >= policy.MaxRetries
}
