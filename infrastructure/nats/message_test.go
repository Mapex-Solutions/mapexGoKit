package natsModel

import (
	"errors"
	"os"
	"testing"

	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

func TestMain(m *testing.M) {
	logger.InitLogger(logger.LoggerOptions{
		ServiceName: "test",
		Environment: "test",
		Level:       logger.ErrorLevel,
	})
	os.Exit(m.Run())
}

/** NewTestMessage */

func TestNewTestMessage_BasicFields(t *testing.T) {
	data := []byte(`{"event":"test"}`)
	msg := NewTestMessage(data, 0, nil)

	if msg == nil {
		t.Fatal("NewTestMessage returned nil")
	}
	if string(msg.Data) != `{"event":"test"}` {
		t.Errorf("expected Data preserved, got %q", string(msg.Data))
	}
}

func TestNewTestMessage_WithIndex(t *testing.T) {
	msg := NewTestMessage([]byte("data"), 5, nil)
	if msg.index != 5 {
		t.Errorf("expected index 5, got %d", msg.index)
	}
}

func TestNewTestMessage_NilCallbacks(t *testing.T) {
	msg := NewTestMessage([]byte("data"), 0, nil)
	if msg.testCallbacks != nil {
		t.Error("expected nil testCallbacks")
	}
}

func TestNewTestMessage_WithCallbacks(t *testing.T) {
	cb := &TestMessageCallbacks{}
	msg := NewTestMessage([]byte("data"), 0, cb)
	if msg.testCallbacks != cb {
		t.Error("expected testCallbacks to be set")
	}
}

/** Ack with TestMessageCallbacks */

func TestAck_WithCallback(t *testing.T) {
	acked := false
	msg := NewTestMessage([]byte("data"), 0, &TestMessageCallbacks{
		OnAck: func() error {
			acked = true
			return nil
		},
	})

	err := msg.Ack()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !acked {
		t.Error("expected OnAck to be called")
	}
}

func TestAck_WithCallbackError(t *testing.T) {
	msg := NewTestMessage([]byte("data"), 0, &TestMessageCallbacks{
		OnAck: func() error {
			return errors.New("ack failed")
		},
	})

	err := msg.Ack()
	if err == nil {
		t.Error("expected error from OnAck")
	}
}

func TestAck_NilOnAck_NoOp(t *testing.T) {
	msg := NewTestMessage([]byte("data"), 0, &TestMessageCallbacks{})

	err := msg.Ack()
	if err != nil {
		t.Fatalf("expected nil error for nil OnAck, got %v", err)
	}
}

/** Nack with TestMessageCallbacks */

func TestNack_WithCallback(t *testing.T) {
	var receivedErr error
	msg := NewTestMessage([]byte("data"), 0, &TestMessageCallbacks{
		OnNack: func(err error) error {
			receivedErr = err
			return nil
		},
	})

	testErr := errors.New("processing failed")
	err := msg.Nack(testErr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedErr != testErr {
		t.Errorf("expected received error to match, got %v", receivedErr)
	}
}

func TestNack_WithCallbackError(t *testing.T) {
	msg := NewTestMessage([]byte("data"), 0, &TestMessageCallbacks{
		OnNack: func(err error) error {
			return errors.New("nack callback failed")
		},
	})

	err := msg.Nack(errors.New("original error"))
	if err == nil {
		t.Error("expected error from OnNack")
	}
}

func TestNack_NilOnNack_NoOp(t *testing.T) {
	msg := NewTestMessage([]byte("data"), 0, &TestMessageCallbacks{})

	err := msg.Nack(errors.New("test"))
	if err != nil {
		t.Fatalf("expected nil error for nil OnNack, got %v", err)
	}
}

func TestNack_NilError(t *testing.T) {
	var receivedErr error
	msg := NewTestMessage([]byte("data"), 0, &TestMessageCallbacks{
		OnNack: func(err error) error {
			receivedErr = err
			return nil
		},
	})

	err := msg.Nack(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedErr != nil {
		t.Error("expected nil error passed to OnNack")
	}
}

/** Reject with TestMessageCallbacks */

func TestReject_WithCallback(t *testing.T) {
	var receivedReason string
	msg := NewTestMessage([]byte("data"), 0, &TestMessageCallbacks{
		OnReject: func(reason string) error {
			receivedReason = reason
			return nil
		},
	})

	err := msg.Reject("invalid JSON")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedReason != "invalid JSON" {
		t.Errorf("expected reason 'invalid JSON', got %q", receivedReason)
	}
}

func TestReject_WithCallbackError(t *testing.T) {
	msg := NewTestMessage([]byte("data"), 0, &TestMessageCallbacks{
		OnReject: func(reason string) error {
			return errors.New("reject callback failed")
		},
	})

	err := msg.Reject("bad data")
	if err == nil {
		t.Error("expected error from OnReject")
	}
}

func TestReject_NilOnReject_NoOp(t *testing.T) {
	msg := NewTestMessage([]byte("data"), 0, &TestMessageCallbacks{})

	err := msg.Reject("test reason")
	if err != nil {
		t.Fatalf("expected nil error for nil OnReject, got %v", err)
	}
}

/** GetRetryInfo */

func TestGetRetryInfo_NilConsumer(t *testing.T) {
	msg := NewTestMessage([]byte("data"), 0, nil)
	msg.DeliveryCount = 3

	attempt, maxRetries, isLast := msg.GetRetryInfo()
	if attempt != 3 {
		t.Errorf("expected attempt 3, got %d", attempt)
	}
	if maxRetries != 0 {
		t.Errorf("expected maxRetries 0 (nil consumer), got %d", maxRetries)
	}
	if isLast {
		t.Error("expected isLast=false for nil consumer")
	}
}

func TestGetRetryInfo_WithPolicy(t *testing.T) {
	msg := NewTestMessage([]byte("data"), 0, nil)
	msg.DeliveryCount = 3
	msg.consumer = &Consumer{
		options: ConsumerOptions{
			RetryPolicy: &RetryPolicy{MaxRetries: 5},
		},
	}

	attempt, maxRetries, isLast := msg.GetRetryInfo()
	if attempt != 3 {
		t.Errorf("expected attempt 3, got %d", attempt)
	}
	if maxRetries != 5 {
		t.Errorf("expected maxRetries 5, got %d", maxRetries)
	}
	if isLast {
		t.Error("expected isLast=false for attempt 3/5")
	}
}

func TestGetRetryInfo_LastAttempt(t *testing.T) {
	msg := NewTestMessage([]byte("data"), 0, nil)
	msg.DeliveryCount = 5
	msg.consumer = &Consumer{
		options: ConsumerOptions{
			RetryPolicy: &RetryPolicy{MaxRetries: 5},
		},
	}

	_, _, isLast := msg.GetRetryInfo()
	if !isLast {
		t.Error("expected isLast=true for attempt 5/5")
	}
}

func TestGetRetryInfo_BeyondLastAttempt(t *testing.T) {
	msg := NewTestMessage([]byte("data"), 0, nil)
	msg.DeliveryCount = 10
	msg.consumer = &Consumer{
		options: ConsumerOptions{
			RetryPolicy: &RetryPolicy{MaxRetries: 5},
		},
	}

	_, _, isLast := msg.GetRetryInfo()
	if !isLast {
		t.Error("expected isLast=true for attempt 10/5")
	}
}

func TestGetRetryInfo_NilRetryPolicy(t *testing.T) {
	msg := NewTestMessage([]byte("data"), 0, nil)
	msg.DeliveryCount = 2
	msg.consumer = &Consumer{
		options: ConsumerOptions{},
	}

	attempt, maxRetries, isLast := msg.GetRetryInfo()
	if attempt != 2 {
		t.Errorf("expected attempt 2, got %d", attempt)
	}
	if maxRetries != 0 {
		t.Errorf("expected maxRetries 0 (nil policy), got %d", maxRetries)
	}
	if isLast {
		t.Error("expected isLast=false for nil policy")
	}
}

/** Callback Tracking Pattern */

func TestCallbackTracking_AllOperations(t *testing.T) {
	var operations []string

	callbacks := &TestMessageCallbacks{
		OnAck: func() error {
			operations = append(operations, "ack")
			return nil
		},
		OnNack: func(err error) error {
			operations = append(operations, "nack")
			return nil
		},
		OnReject: func(reason string) error {
			operations = append(operations, "reject")
			return nil
		},
	}

	msg1 := NewTestMessage([]byte("data1"), 0, callbacks)
	msg2 := NewTestMessage([]byte("data2"), 1, callbacks)
	msg3 := NewTestMessage([]byte("data3"), 2, callbacks)

	msg1.Ack()
	msg2.Nack(errors.New("fail"))
	msg3.Reject("invalid")

	if len(operations) != 3 {
		t.Fatalf("expected 3 operations, got %d", len(operations))
	}
	if operations[0] != "ack" || operations[1] != "nack" || operations[2] != "reject" {
		t.Errorf("unexpected operations order: %v", operations)
	}
}
