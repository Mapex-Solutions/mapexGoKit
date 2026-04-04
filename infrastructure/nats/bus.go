// Package natsadapter maps the ports to your concrete natsModel client.
// It stays thin: just translate calls to Push/SubscribeWithOptions.
package natsModel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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

	sub, err := b.c.SubscribeWithOptions(SubscribeOptions{
		Stream:       config.Stream,
		Subject:      config.Subject,
		Durable:      config.Durable,
		DeliverGroup: config.Group,
		Pull:         config.Pull,
		Handler: func(m *natsgo.Msg) {
			if err := config.Handler(m.Data); err == nil {
				_ = m.Ack() // ack only on success
			}
		},
	})
	if err != nil {
		return nil, err
	}
	return sub.Drain, nil
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

// StartConsumer creates and starts a new managed consumer with automatic goroutine handling.
// This is the recommended way to create consumers with retry/DLQ support.
//
// Parameters:
//   - opts: Consumer configuration including handler function, retry policy, and DLQ policy
//
// Returns:
//   - *Consumer: Consumer instance for lifecycle management
//   - error: Any initialization error
//
// Example:
//
//	consumer, err := bus.StartConsumer(natsModel.ConsumerOptions{
//	    Stream:       "EVENTS-RAW",
//	    Subject:      "events.raw",
//	    Durable:      "events-processor",
//	    BatchSize:    50,
//	    FetchTimeout: 5 * time.Second,
//	    RetryPolicy: &natsModel.RetryPolicy{
//	        MaxRetries: 5,
//	        Backoff:    []time.Duration{1*time.Second, 5*time.Second, 15*time.Second},
//	        AckWait:    30 * time.Second,
//	    },
//	    DLQPolicy: &natsModel.DLQPolicy{
//	        Stream:      "MAPEXOS-DLQ",
//	        Subject:     "dlq.events.raw",
//	        ServiceName: "events-service",
//	        ServiceType: "events",
//	        EventType:   "raw",
//	    },
//	    BatchMessageHandlerV2: func(messages []*natsModel.Message) {
//	        for _, msg := range messages {
//	            if err := process(msg.Data); err != nil {
//	                msg.Nack(err)
//	            } else {
//	                msg.Ack()
//	            }
//	        }
//	    },
//	})
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

func (b *Bus) StartConsumer(opts ConsumerOptions) (*Consumer, error) {
	return b.c.StartConsumer(opts)
}
