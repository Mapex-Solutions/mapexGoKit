package natsModel

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

// =============================================================================
// Test Configuration
// =============================================================================

func getTestConfig() Config {
	url := os.Getenv("NATS_TEST_URL")
	if url == "" {
		url = "nats://localhost:4222"
	}

	username := os.Getenv("NATS_TEST_USERNAME")
	if username == "" {
		username = "service"
	}

	password := os.Getenv("NATS_TEST_PASSWORD")
	if password == "" {
		password = "service_secret"
	}

	opts := nats.GetDefaultOptions()
	opts.Url = url
	opts.User = username
	opts.Password = password

	return Config{Options: opts}
}

// skipIfNoNATS skips the test if NATS is not available.
func skipIfNoNATS(t *testing.T) *Bus {
	t.Helper()

	config := getTestConfig()
	client, err := New(config)
	if err != nil {
		t.Skipf("NATS not available: %v", err)
		return nil
	}

	return NewBus(client)
}

// =============================================================================
// Unit Tests - Types
// =============================================================================

func TestPublishConfig(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		config := PublishConfig{
			Subject: "test.subject",
			Data:    map[string]string{"key": "value"},
		}

		if config.Subject != "test.subject" {
			t.Errorf("Subject = %s, want test.subject", config.Subject)
		}

		if config.Ctx != nil {
			t.Error("Ctx should be nil by default")
		}
	})

	t.Run("with context", func(t *testing.T) {
		ctx := context.Background()
		config := PublishConfig{
			Ctx:     ctx,
			Subject: "test.subject",
			Data:    "test",
		}

		if config.Ctx == nil {
			t.Error("Ctx should not be nil")
		}
	})
}

func TestSubscribeConfig(t *testing.T) {
	t.Run("required fields", func(t *testing.T) {
		config := SubscribeConfig{
			Stream:  "TEST-STREAM",
			Subject: "test.subject",
			Durable: "test-consumer",
			Handler: func(data []byte) error { return nil },
		}

		if config.Stream != "TEST-STREAM" {
			t.Errorf("Stream = %s, want TEST-STREAM", config.Stream)
		}
		if config.Subject != "test.subject" {
			t.Errorf("Subject = %s, want test.subject", config.Subject)
		}
		if config.Durable != "test-consumer" {
			t.Errorf("Durable = %s, want test-consumer", config.Durable)
		}
		if config.Handler == nil {
			t.Error("Handler should not be nil")
		}
	})
}

func TestFetchConfig(t *testing.T) {
	t.Run("with batch handler", func(t *testing.T) {
		config := FetchConfig{
			Stream:    "TEST-STREAM",
			Subject:   "test.subject",
			Durable:   "test-consumer",
			BatchMode: true,
			BatchSize: 50,
			Timeout:   10,
			BatchHandler: func(messages []BatchMessage) error {
				return nil
			},
		}

		if config.BatchSize != 50 {
			t.Errorf("BatchSize = %d, want 50", config.BatchSize)
		}
		if config.Timeout != 10 {
			t.Errorf("Timeout = %d, want 10", config.Timeout)
		}
		if !config.BatchMode {
			t.Error("BatchMode should be true")
		}
	})
}

func TestConsumerOptions(t *testing.T) {
	t.Run("with retry policy", func(t *testing.T) {
		opts := ConsumerOptions{
			Stream:       "TEST-STREAM",
			Subject:      "test.subject",
			Durable:      "test-consumer",
			BatchSize:    100,
			FetchTimeout: 5 * time.Second,
			RetryPolicy: &RetryPolicy{
				MaxRetries: 3,
				Backoff:    []time.Duration{1 * time.Second, 5 * time.Second},
			},
		}

		if opts.RetryPolicy == nil {
			t.Error("RetryPolicy should not be nil")
		}
		if opts.RetryPolicy.MaxRetries != 3 {
			t.Errorf("MaxRetries = %d, want 3", opts.RetryPolicy.MaxRetries)
		}
	})

	t.Run("with DLQ policy", func(t *testing.T) {
		opts := ConsumerOptions{
			Stream:  "TEST-STREAM",
			Subject: "test.subject",
			Durable: "test-consumer",
			DLQPolicy: &DLQPolicy{
				Stream:      "TEST-DLQ",
				Subject:     "dlq.test",
				ServiceName: "test-service",
				ServiceType: "processor",
				EventType:   "test",
			},
		}

		if opts.DLQPolicy == nil {
			t.Error("DLQPolicy should not be nil")
		}
		if opts.DLQPolicy.ServiceName != "test-service" {
			t.Errorf("ServiceName = %s, want test-service", opts.DLQPolicy.ServiceName)
		}
	})
}

// =============================================================================
// Unit Tests - RetryPolicy
// =============================================================================

func TestRetryPolicy_GetMaxDeliver(t *testing.T) {
	tests := []struct {
		name       string
		policy     *RetryPolicy
		wantDeliver int
	}{
		{
			name:        "nil policy",
			policy:      nil,
			wantDeliver: 6, // default 5 retries + 1
		},
		{
			name:        "zero retries",
			policy:      &RetryPolicy{MaxRetries: 0},
			wantDeliver: 6, // default
		},
		{
			name:        "custom retries",
			policy:      &RetryPolicy{MaxRetries: 3},
			wantDeliver: 4, // 3 + 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.GetMaxDeliver()
			if got != tt.wantDeliver {
				t.Errorf("GetMaxDeliver() = %d, want %d", got, tt.wantDeliver)
			}
		})
	}
}

func TestRetryPolicy_GetBackoffDurations(t *testing.T) {
	t.Run("nil policy returns defaults", func(t *testing.T) {
		var policy *RetryPolicy
		backoff := policy.GetBackoffDurations()

		if len(backoff) == 0 {
			t.Error("Should return default backoff durations")
		}
	})

	t.Run("custom backoff", func(t *testing.T) {
		policy := &RetryPolicy{
			Backoff: []time.Duration{1 * time.Second, 2 * time.Second},
		}
		backoff := policy.GetBackoffDurations()

		if len(backoff) != 2 {
			t.Errorf("len(backoff) = %d, want 2", len(backoff))
		}
		if backoff[0] != 1*time.Second {
			t.Errorf("backoff[0] = %v, want 1s", backoff[0])
		}
	})
}

func TestRetryPolicy_GetAckWait(t *testing.T) {
	t.Run("nil policy returns default", func(t *testing.T) {
		var policy *RetryPolicy
		ackWait := policy.GetAckWait()

		if ackWait != 30*time.Second {
			t.Errorf("GetAckWait() = %v, want 30s", ackWait)
		}
	})

	t.Run("custom ack wait", func(t *testing.T) {
		policy := &RetryPolicy{
			AckWait: 60 * time.Second,
		}
		ackWait := policy.GetAckWait()

		if ackWait != 60*time.Second {
			t.Errorf("GetAckWait() = %v, want 60s", ackWait)
		}
	})
}

// =============================================================================
// Unit Tests - FANOUT Types
// =============================================================================

func TestFanoutStreamConfig(t *testing.T) {
	t.Run("with all fields", func(t *testing.T) {
		config := FanoutStreamConfig{
			Name:        "TEST-FANOUT",
			Subjects:    []string{"test.fanout.>"},
			MaxAge:      10 * time.Minute,
			MaxMsgs:     5000,
			MaxBytes:    5 * 1024 * 1024,
			Description: "Test fanout stream",
		}

		if config.Name != "TEST-FANOUT" {
			t.Errorf("Name = %s, want TEST-FANOUT", config.Name)
		}
		if len(config.Subjects) != 1 {
			t.Errorf("len(Subjects) = %d, want 1", len(config.Subjects))
		}
		if config.MaxAge != 10*time.Minute {
			t.Errorf("MaxAge = %v, want 10m", config.MaxAge)
		}
	})

	t.Run("defaults should be applied by EnsureFanoutStream", func(t *testing.T) {
		config := FanoutStreamConfig{
			Name:     "TEST-FANOUT",
			Subjects: []string{"test.>"},
		}

		// These should be zero - defaults are applied in EnsureFanoutStream
		if config.MaxAge != 0 {
			t.Error("MaxAge should be zero before processing")
		}
		if config.MaxMsgs != 0 {
			t.Error("MaxMsgs should be zero before processing")
		}
	})
}

func TestFanoutHandler(t *testing.T) {
	t.Run("handler signature", func(t *testing.T) {
		called := false
		var handler FanoutHandler = func(data []byte) error {
			called = true
			return nil
		}

		err := handler([]byte("test"))
		if err != nil {
			t.Errorf("handler returned error: %v", err)
		}
		if !called {
			t.Error("handler was not called")
		}
	})
}

// =============================================================================
// Unit Tests - Errors
// =============================================================================

func TestErrors(t *testing.T) {
	errors := []error{
		ErrMissingHandler,
		ErrMissingSubject,
		ErrMaxRetriesExceeded,
		ErrDLQPublishFailed,
		ErrMessageMetadataFailed,
		ErrDLQNotConfigured,
	}

	for _, err := range errors {
		if err == nil {
			t.Error("Error should not be nil")
		}
		if err.Error() == "" {
			t.Error("Error message should not be empty")
		}
	}
}

// =============================================================================
// Unit Tests - BatchMessage
// =============================================================================

func TestBatchMessage(t *testing.T) {
	t.Run("struct fields", func(t *testing.T) {
		headers := map[string][]string{
			"X-Custom": {"value1", "value2"},
		}
		msg := BatchMessage{
			Data:    []byte("test data"),
			Headers: headers,
		}

		if string(msg.Data) != "test data" {
			t.Errorf("Data = %s, want 'test data'", string(msg.Data))
		}
		if len(msg.Headers["X-Custom"]) != 2 {
			t.Error("Headers should have 2 values for X-Custom")
		}
	})
}

// =============================================================================
// Unit Tests - DLQMessage
// =============================================================================

func TestDLQMessage(t *testing.T) {
	t.Run("all fields", func(t *testing.T) {
		now := time.Now()
		msg := DLQMessage{
			ID:              "msg-123",
			OrgId:           "org-1",
			PathKey:         "path-1",
			ServiceName:     "test-service",
			ServiceType:     "processor",
			EventType:       "test.event",
			OriginalSubject: "test.subject",
			OriginalStream:  "TEST-STREAM",
			LastError:       "connection timeout",
			ErrorCount:      3,
			FirstDelivery:   now.Add(-1 * time.Hour),
			LastDelivery:    now,
			TotalDeliveries: 5,
			ConsumerName:    "test-consumer",
			SentToDLQAt:     now,
		}

		if msg.ID != "msg-123" {
			t.Errorf("ID = %s, want msg-123", msg.ID)
		}
		if msg.ErrorCount != 3 {
			t.Errorf("ErrorCount = %d, want 3", msg.ErrorCount)
		}
		if msg.TotalDeliveries != 5 {
			t.Errorf("TotalDeliveries = %d, want 5", msg.TotalDeliveries)
		}
	})
}

// =============================================================================
// Integration Tests (require NATS server)
// =============================================================================

func TestNewClient_Integration(t *testing.T) {
	bus := skipIfNoNATS(t)
	if bus == nil {
		return
	}

	conn := bus.GetConn()
	if conn == nil {
		t.Error("GetConn() should not be nil")
	}

	if !conn.IsConnected() {
		t.Error("Connection should be connected")
	}
}

func TestPublish_Integration(t *testing.T) {
	bus := skipIfNoNATS(t)
	if bus == nil {
		return
	}

	t.Run("publish with struct config", func(t *testing.T) {
		err := bus.Publish(PublishConfig{
			Subject: "test.publish.integration",
			Data:    map[string]string{"test": "data"},
		})

		if err != nil {
			t.Errorf("Publish() error = %v", err)
		}
	})
}

func TestPublishFanout_Integration(t *testing.T) {
	bus := skipIfNoNATS(t)
	if bus == nil {
		return
	}

	ctx := context.Background()

	t.Run("publish fanout", func(t *testing.T) {
		err := bus.PublishFanout(ctx, "test.fanout.integration", []byte(`{"test":"data"}`))
		if err != nil {
			t.Errorf("PublishFanout() error = %v", err)
		}
	})

	t.Run("publish fanout empty subject", func(t *testing.T) {
		err := bus.PublishFanout(ctx, "", []byte(`{"test":"data"}`))
		if err == nil {
			t.Error("PublishFanout() should return error for empty subject")
		}
	})

	t.Run("publish fanout nil context", func(t *testing.T) {
		err := bus.PublishFanout(nil, "test.fanout.integration", []byte(`{"test":"data"}`))
		if err != nil {
			t.Errorf("PublishFanout() with nil context should not error: %v", err)
		}
	})
}

func TestEnsureFanoutStream_Integration(t *testing.T) {
	bus := skipIfNoNATS(t)
	if bus == nil {
		return
	}

	t.Run("create fanout stream", func(t *testing.T) {
		config := FanoutStreamConfig{
			Name:        "TEST-FANOUT-INTEGRATION",
			Subjects:    []string{"test.fanout.integration.>"},
			Description: "Integration test stream",
			MaxAge:      1 * time.Minute,
			MaxMsgs:     100,
			MaxBytes:    1024 * 1024,
		}

		err := bus.EnsureFanoutStream(config)
		if err != nil {
			t.Errorf("EnsureFanoutStream() error = %v", err)
		}

		// Call again - should not error (idempotent)
		err = bus.EnsureFanoutStream(config)
		if err != nil {
			t.Errorf("EnsureFanoutStream() second call error = %v", err)
		}
	})

	t.Run("missing stream name", func(t *testing.T) {
		config := FanoutStreamConfig{
			Subjects: []string{"test.>"},
		}

		err := bus.EnsureFanoutStream(config)
		if err == nil {
			t.Error("EnsureFanoutStream() should error with missing name")
		}
	})

	t.Run("missing subjects", func(t *testing.T) {
		config := FanoutStreamConfig{
			Name: "TEST-FANOUT",
		}

		err := bus.EnsureFanoutStream(config)
		if err == nil {
			t.Error("EnsureFanoutStream() should error with missing subjects")
		}
	})
}

func TestSubscribeFanout_Integration(t *testing.T) {
	bus := skipIfNoNATS(t)
	if bus == nil {
		return
	}

	// First ensure stream exists
	err := bus.EnsureFanoutStream(FanoutStreamConfig{
		Name:     "TEST-FANOUT-SUB",
		Subjects: []string{"test.fanout.sub.>"},
	})
	if err != nil {
		t.Fatalf("EnsureFanoutStream() error = %v", err)
	}

	t.Run("subscribe to fanout", func(t *testing.T) {
		received := make(chan []byte, 1)

		sub, err := bus.SubscribeFanout(
			"TEST-FANOUT-SUB",
			"test-service",
			"test.fanout.sub.>",
			func(data []byte) error {
				received <- data
				return nil
			},
		)
		if err != nil {
			t.Fatalf("SubscribeFanout() error = %v", err)
		}
		defer sub.Stop()

		// Publish a message
		ctx := context.Background()
		testData := []byte(`{"message":"hello"}`)
		err = bus.PublishFanout(ctx, "test.fanout.sub.test", testData)
		if err != nil {
			t.Fatalf("PublishFanout() error = %v", err)
		}

		// Wait for message
		select {
		case data := <-received:
			if string(data) != string(testData) {
				t.Errorf("Received data = %s, want %s", string(data), string(testData))
			}
		case <-time.After(5 * time.Second):
			t.Error("Timeout waiting for fanout message")
		}
	})

	t.Run("missing handler", func(t *testing.T) {
		_, err := bus.SubscribeFanout("TEST-STREAM", "test-service", "test.>", nil)
		if err == nil {
			t.Error("SubscribeFanout() should error with nil handler")
		}
	})

	t.Run("missing service name", func(t *testing.T) {
		_, err := bus.SubscribeFanout("TEST-STREAM", "", "test.>", func(data []byte) error { return nil })
		if err == nil {
			t.Error("SubscribeFanout() should error with empty service name")
		}
	})

	t.Run("missing subject", func(t *testing.T) {
		_, err := bus.SubscribeFanout("TEST-STREAM", "test-service", "", func(data []byte) error { return nil })
		if err == nil {
			t.Error("SubscribeFanout() should error with empty subject")
		}
	})

	t.Run("missing stream", func(t *testing.T) {
		_, err := bus.SubscribeFanout("", "test-service", "test.>", func(data []byte) error { return nil })
		if err == nil {
			t.Error("SubscribeFanout() should error with empty stream")
		}
	})
}

// =============================================================================
// Unit Tests - PublishCore Types
// =============================================================================

func TestPublishCoreOptions(t *testing.T) {
	t.Run("all fields", func(t *testing.T) {
		opts := PublishCoreOptions{
			Subject: "test.core.subject",
			Data:    []byte(`{"key":"value"}`),
			MsgId:   "msg-abc-123",
			Headers: map[string]string{"X-Custom": "value"},
		}

		if opts.Subject != "test.core.subject" {
			t.Errorf("Subject = %s, want test.core.subject", opts.Subject)
		}
		if string(opts.Data) != `{"key":"value"}` {
			t.Errorf("Data = %s, want {\"key\":\"value\"}", string(opts.Data))
		}
		if opts.MsgId != "msg-abc-123" {
			t.Errorf("MsgId = %s, want msg-abc-123", opts.MsgId)
		}
		if opts.Headers["X-Custom"] != "value" {
			t.Errorf("Headers[X-Custom] = %s, want value", opts.Headers["X-Custom"])
		}
	})

	t.Run("minimal fields", func(t *testing.T) {
		opts := PublishCoreOptions{
			Subject: "test.subject",
			Data:    []byte("data"),
		}

		if opts.MsgId != "" {
			t.Error("MsgId should be empty by default")
		}
		if opts.Headers != nil {
			t.Error("Headers should be nil by default")
		}
	})
}

func TestPublishCoreConfig(t *testing.T) {
	t.Run("all fields", func(t *testing.T) {
		config := PublishCoreConfig{
			Subject: "test.core.subject",
			Data:    map[string]string{"key": "value"},
			MsgId:   "msg-abc-123",
			Headers: map[string]string{"X-Custom": "value"},
		}

		if config.Subject != "test.core.subject" {
			t.Errorf("Subject = %s, want test.core.subject", config.Subject)
		}
		if config.MsgId != "msg-abc-123" {
			t.Errorf("MsgId = %s, want msg-abc-123", config.MsgId)
		}
		if config.Headers["X-Custom"] != "value" {
			t.Errorf("Headers[X-Custom] = %s, want value", config.Headers["X-Custom"])
		}
	})

	t.Run("minimal fields", func(t *testing.T) {
		config := PublishCoreConfig{
			Subject: "test.subject",
			Data:    "test",
		}

		if config.MsgId != "" {
			t.Error("MsgId should be empty by default")
		}
		if config.Headers != nil {
			t.Error("Headers should be nil by default")
		}
	})
}

// =============================================================================
// Integration Tests - PublishCore (require NATS server)
// =============================================================================

func TestPublishCore_Integration(t *testing.T) {
	bus := skipIfNoNATS(t)
	if bus == nil {
		return
	}

	t.Run("publish core and flush", func(t *testing.T) {
		err := bus.PublishCore(PublishCoreConfig{
			Subject: "test.core.publish.integration",
			Data:    map[string]string{"test": "core-data"},
		})
		if err != nil {
			t.Errorf("PublishCore() error = %v", err)
		}

		err = bus.FlushConnection()
		if err != nil {
			t.Errorf("FlushConnection() error = %v", err)
		}
	})
}

func TestPublishCore_WithMsgId_Integration(t *testing.T) {
	bus := skipIfNoNATS(t)
	if bus == nil {
		return
	}

	t.Run("publish core with MsgId header", func(t *testing.T) {
		err := bus.PublishCore(PublishCoreConfig{
			Subject: "test.core.publish.dedup",
			Data:    map[string]string{"test": "dedup-data"},
			MsgId:   "dedup-msg-001",
		})
		if err != nil {
			t.Errorf("PublishCore() with MsgId error = %v", err)
		}

		err = bus.FlushConnection()
		if err != nil {
			t.Errorf("FlushConnection() error = %v", err)
		}
	})
}

func TestPublishCore_MissingSubject_Integration(t *testing.T) {
	bus := skipIfNoNATS(t)
	if bus == nil {
		return
	}

	t.Run("missing subject returns error", func(t *testing.T) {
		err := bus.PublishCore(PublishCoreConfig{
			Subject: "",
			Data:    "test",
		})
		if err == nil {
			t.Error("PublishCore() should return error for empty subject")
		}
	})
}

func TestFlushConnection_Integration(t *testing.T) {
	bus := skipIfNoNATS(t)
	if bus == nil {
		return
	}

	t.Run("flush without prior publish", func(t *testing.T) {
		err := bus.FlushConnection()
		if err != nil {
			t.Errorf("FlushConnection() error = %v", err)
		}
	})
}

func TestPublishCoreBatch_Integration(t *testing.T) {
	bus := skipIfNoNATS(t)
	if bus == nil {
		return
	}

	t.Run("batch publish N messages then flush", func(t *testing.T) {
		batchSize := 10

		for i := 0; i < batchSize; i++ {
			err := bus.PublishCore(PublishCoreConfig{
				Subject: "test.core.batch.integration",
				Data:    map[string]int{"index": i},
			})
			if err != nil {
				t.Fatalf("PublishCore() batch item %d error = %v", i, err)
			}
		}

		err := bus.FlushConnection()
		if err != nil {
			t.Errorf("FlushConnection() after batch error = %v", err)
		}
	})
}

func TestFanoutSubscription_Stop(t *testing.T) {
	bus := skipIfNoNATS(t)
	if bus == nil {
		return
	}

	// Ensure stream exists
	err := bus.EnsureFanoutStream(FanoutStreamConfig{
		Name:     "TEST-FANOUT-STOP",
		Subjects: []string{"test.fanout.stop.>"},
	})
	if err != nil {
		t.Fatalf("EnsureFanoutStream() error = %v", err)
	}

	sub, err := bus.SubscribeFanout(
		"TEST-FANOUT-STOP",
		"test-service",
		"test.fanout.stop.>",
		func(data []byte) error { return nil },
	)
	if err != nil {
		t.Fatalf("SubscribeFanout() error = %v", err)
	}

	// Stop should not error
	err = sub.Stop()
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	// Stop again should be idempotent (no error)
	err = sub.Stop()
	if err != nil {
		t.Errorf("Stop() second call error = %v", err)
	}
}
