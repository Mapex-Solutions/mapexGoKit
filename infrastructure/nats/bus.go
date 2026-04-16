// Package natsadapter maps the ports to your concrete natsModel client.
// It stays thin: just translate calls to Push/StartConsumer.
package natsModel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	natsgo "github.com/nats-io/nats.go"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// Bus adapts natsModel.Client to the shared ports.
type Bus struct{ c *Client }

// NewBus wires an existing natsModel.Client.
func NewBus(c *Client) *Bus {
	logger.Info("[INFRA:NATS] Creating NATS Bus adapter")
	return &Bus{c: c}
}

// GetConn returns the underlying NATS connection for direct operations.
// Use this for request-reply patterns like Auth Callout.
func (b *Bus) GetConn() *natsgo.Conn {
	if b.c == nil {
		return nil
	}
	return b.c.nc
}

// Compile-time guarantees.
var _ Publisher = (*Bus)(nil)
var _ Subscriber = (*Bus)(nil)
var _ Fetcher = (*Bus)(nil)
var _ Fanout = (*Bus)(nil)
var _ CorePublisher = (*Bus)(nil)
var _ ScheduleManager = (*Bus)(nil)

// Publish sends a message using struct configuration (new recommended method)
func (b *Bus) Publish(config PublishConfig) error {
	// Set default context if not provided
	if config.Ctx == nil {
		config.Ctx = context.Background()
	}

	// Marshal the payload to bytes
	data, err := json.Marshal(config.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	return b.c.Push(PushOptions{
		Ctx:     config.Ctx,
		Subject: config.Subject,
		Data:    data,
		Headers: config.Headers,
		Async:   true, // set false if you prefer sync by default
	})
}

// Subscribe starts a subscription using struct configuration
func (b *Bus) Subscribe(config SubscribeConfig) (func() error, error) {
	// Set default context if not provided
	if config.Ctx == nil {
		config.Ctx = context.Background()
	}

	logger.Info("[INFRA:NATS] Creating subscription to subject: " + config.Subject)

	ctx := context.Background()

	// Ensure stream and consumer exist
	if err := b.c.createOrGetConsumer(SubscribeOptions{
		Stream:  config.Stream,
		Subject: config.Subject,
		Durable: config.Durable,
		Pull:    config.Pull,
	}); err != nil {
		return nil, fmt.Errorf("failed to ensure stream/consumer: %w", err)
	}

	// Get consumer handle
	cons, err := b.c.js.Consumer(ctx, config.Stream, config.Durable)
	if err != nil {
		return nil, fmt.Errorf("failed to get consumer: %w", err)
	}

	// Use Consume for callback-based message handling
	cc, err := cons.Consume(func(msg jetstream.Msg) {
		if err := config.Handler(msg.Data()); err == nil {
			_ = msg.Ack() // ack only on success
		} else {
			_ = msg.Nak() // nak on error for redelivery
		}
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start consume: %w", err)
	}

	return func() error { cc.Drain(); return nil }, nil
}

// Fetch starts a fetch-based consumer using struct configuration
func (b *Bus) Fetch(config FetchConfig) (func() error, error) {
	// Set default context if not provided
	if config.Ctx == nil {
		config.Ctx = context.Background()
	}

	// Set default values
	if config.BatchSize <= 0 {
		config.BatchSize = 10
	}
	if config.Timeout <= 0 {
		config.Timeout = 5
	}

	// Validate that either Handler or BatchHandler is set
	if config.Handler == nil && config.BatchHandler == nil {
		return nil, fmt.Errorf("either Handler or BatchHandler is required")
	}
	if config.Handler != nil && config.BatchHandler != nil {
		return nil, fmt.Errorf("only one of Handler or BatchHandler can be set, not both")
	}

	logger.Info("[INFRA:NATS] Creating fetch consumer for subject: " + config.Subject)

	if config.BatchMode {
		// Build consumer options
		opts := ConsumerOptions{
			Stream:       config.Stream,
			Subject:      config.Subject,
			Durable:      config.Durable,
			QueueGroup:   config.Group,
			BatchSize:    config.BatchSize,
			FetchTimeout: time.Duration(config.Timeout) * time.Second,
			RetryDelay:   2 * time.Second,
			MaxRetries:   3,
			StopOnError:  false,
		}

		// Set the appropriate handler
		if config.BatchHandler != nil {
			// Use BatchHandler for bulk processing
			opts.BatchHandler = config.BatchHandler
			logger.Info("[INFRA:NATS] Using BatchHandler for bulk processing")
		} else {
			// Use Handler for individual message processing
			opts.Handler = func(data []byte, index int, headers map[string][]string) error {
				return config.Handler(data)
			}
		}

		consumer, err := b.c.StartConsumer(opts)
		if err != nil {
			return nil, err
		}
		return func() error { consumer.Stop(); return nil }, nil
	}

	// Single fetch mode - fall back to regular subscribe (only supports Handler)
	if config.BatchHandler != nil {
		return nil, fmt.Errorf("BatchHandler requires BatchMode to be true")
	}

	return b.Subscribe(SubscribeConfig{
		Ctx:     config.Ctx,
		Stream:  config.Stream,
		Subject: config.Subject,
		Durable: config.Durable,
		Group:   config.Group,
		Handler: config.Handler,
		Pull:    true,
	})
}

// PublishFanout publishes a message to a FANOUT subject.
// All ephemeral subscribers will receive this message.
func (b *Bus) PublishFanout(ctx context.Context, subject string, data []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if subject == "" {
		return fmt.Errorf("subject is required")
	}

	err := b.c.Push(PushOptions{
		Ctx:     ctx,
		Subject: subject,
		Data:    data,
		Async:   true,
	})

	if err != nil {
		logger.Warn(fmt.Sprintf("[INFRA:NATS] FANOUT failed to publish to %s: %v", subject, err))
		return err
	}

	logger.Debug(fmt.Sprintf("[INFRA:NATS] FANOUT published to %s (%d bytes)", subject, len(data)))
	return nil
}

// PublishCore publishes using core NATS (fire-and-forget, no JetStream ACK).
// Messages are enqueued in the TCP buffer. Call FlushConnection() after
// a batch to guarantee delivery in a single TCP roundtrip.
func (b *Bus) PublishCore(config PublishCoreConfig) error {
	data, err := json.Marshal(config.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	return b.c.PublishCore(PublishCoreOptions{
		Subject: config.Subject,
		Data:    data,
		MsgId:   config.MsgId,
		Headers: config.Headers,
	})
}

// FlushConnection flushes the connection buffer, ensuring all PublishCore()
// messages reach the NATS server in a single TCP roundtrip.
func (b *Bus) FlushConnection() error {
	return b.c.FlushConnection()
}

// PublishScheduled publishes a message with Nats-Schedule headers for delayed delivery.
// The message is stored in the schedule stream and delivered to TargetSubject at ScheduleAt time.
// Requires the target stream to have AllowMsgSchedules: true.
func (b *Bus) PublishScheduled(config ScheduledPublishConfig) error {
	if config.Subject == "" {
		return fmt.Errorf("subject is required")
	}
	if config.TargetSubject == "" {
		return fmt.Errorf("target subject is required")
	}

	data, err := json.Marshal(config.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal scheduled payload: %w", err)
	}

	msg := &natsgo.Msg{
		Subject: config.Subject,
		Data:    data,
		Header:  natsgo.Header{},
	}

	msg.Header.Set("Nats-Schedule", fmt.Sprintf("@at %s", config.ScheduleAt.UTC().Format(time.RFC3339)))
	msg.Header.Set("Nats-Schedule-Target", config.TargetSubject)

	for k, v := range config.Headers {
		msg.Header.Set(k, v)
	}

	_, err = b.c.js.PublishMsg(context.Background(), msg)
	if err != nil {
		return fmt.Errorf("failed to publish scheduled message: %w", err)
	}

	logger.Debug(fmt.Sprintf("[INFRA:NATS] Scheduled message published: subject=%s target=%s at=%s",
		config.Subject, config.TargetSubject, config.ScheduleAt.UTC().Format(time.RFC3339)))

	return nil
}

// PurgeStreamSubject purges all messages matching a subject pattern from a stream.
// Idempotent: returns nil if no messages match or the stream doesn't exist.
func (b *Bus) PurgeStreamSubject(streamName, subject string) error {
	ctx := context.Background()

	stream, err := b.c.js.Stream(ctx, streamName)
	if err != nil {
		if errors.Is(err, jetstream.ErrStreamNotFound) {
			return nil
		}
		return fmt.Errorf("failed to get stream %s: %w", streamName, err)
	}

	if err := stream.Purge(ctx, jetstream.WithPurgeSubject(subject)); err != nil {
		return fmt.Errorf("failed to purge subject %s from stream %s: %w", subject, streamName, err)
	}

	return nil
}

// EnsureStream creates or updates a JetStream stream. Idempotent.
func (b *Bus) EnsureStream(config jetstream.StreamConfig) error {
	_, err := b.c.js.CreateOrUpdateStream(context.Background(), config)
	return err
}

// StartConsumer creates and starts a new managed consumer with automatic goroutine handling.
// This is the recommended way to create consumers with retry/DLQ support.
func (b *Bus) StartConsumer(opts ConsumerOptions) (*Consumer, error) {
	return b.c.StartConsumer(opts)
}
