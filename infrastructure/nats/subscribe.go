package natsModel

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// SubscribeWithOptions subscribes to a NATS JetStream subject using the provided options.
//
// It supports both **pull-based** and **push-based** subscriptions. You must provide
// a valid message handler in `opts.Handler`, and configure the subscription mode via `opts.Pull`.
//
// For **pull-based** subscriptions, it uses `PullSubscribe` and ignores the handler.
// For **push-based** subscriptions, it uses `Subscribe` or `QueueSubscribe` depending on
// whether `opts.DeliverGroup` is set.
//
// Required fields:
//   - opts.Subject: the subject to subscribe to.
//   - opts.Stream: the name of the stream (used with `BindStream`).
//   - opts.Durable: durable consumer name.
//   - opts.Handler: function to handle incoming messages (ignored for pull).
//
// Optional fields:
//   - opts.Pull: whether to use pull-based subscription.
//   - opts.DeliverGroup: the queue group name (for shared consumers in push mode).
//
// Returns a `*nats.Subscription` on success or an error if any required field is missing
// or the subscription setup fails.
//
// Example:
//
//	sub, err := client.SubscribeWithOptions(natsModel.SubscribeOptions{
//	  Subject:      "devices.sensor1.data",
//	  Stream:       "SENSORS",
//	  Durable:      "sensor-processor",
//	  DeliverGroup: "workers",
//	  Handler: func(msg *nats.Msg) {
//	    log.Printf("Received message: %s", string(msg.Data))
//	    _ = msg.Ack()
//	  },
//	})
//	if err != nil {
//	  log.Fatal("Subscribe failed:", err)
//	}
func (c *Client) SubscribeWithOptions(opts SubscribeOptions) (*nats.Subscription, error) {

	if opts.Handler == nil {
		return nil, ErrMissingHandler
	}

	// Automatically ensure stream and consumer exist
	if err := c.createOrGetConsumer(opts); err != nil {
		return nil, fmt.Errorf("failed to ensure stream/consumer: %w", err)
	}

	if opts.Pull {
		return c.js.PullSubscribe(
			opts.Subject,
			opts.Durable,
			nats.BindStream(opts.Stream),
		)
	}

	// Default options for push subscriber
	subOpts := []nats.SubOpt{
		nats.Durable(opts.Durable),
		nats.ManualAck(),
		nats.BindStream(opts.Stream),
	}

	if opts.DeliverGroup != "" {
		return c.js.QueueSubscribe(opts.Subject, opts.DeliverGroup, opts.Handler, subOpts...)
	}

	return c.js.Subscribe(opts.Subject, opts.Handler, subOpts...)
}

// createOrGetConsumer ensures that the stream and consumer exist, creating them if necessary.
// This is equivalent to the JavaScript function createOrGetConsumer.
//
// Parameters:
//   - opts: SubscribeOptions containing stream, subject, durable, and other configuration
//
// Returns:
//   - An error if the stream/consumer creation fails, otherwise nil
//
// This function will:
// 1. Check if the stream exists, create it if it doesn't
// 2. Check if the consumer exists, create it if it doesn't
func (c *Client) createOrGetConsumer(opts SubscribeOptions) error {
	if opts.Stream == "" || opts.Durable == "" || opts.Subject == "" {
		return fmt.Errorf("stream, subject and durable name are required for subscriptions")
	}

	// Use JetStream context as manager (it implements JetStreamManager)
	jsm := c.js

	// Resolve duplicate window: default 15min covers worst-case retry backoff (~12.6min) + margin
	dupWindow := 15 * time.Minute
	if opts.DuplicateWindow > 0 {
		dupWindow = opts.DuplicateWindow
	}

	// First, ensure the stream exists and captures our subject
	streamInfo, err := jsm.StreamInfo(opts.Stream)
	if err != nil {
		// Stream doesn't exist, create it
		streamConfig := &nats.StreamConfig{
			Name:       opts.Stream,
			Subjects:   []string{opts.Subject},
			Retention:  nats.WorkQueuePolicy,
			Storage:    nats.FileStorage,
			Duplicates: dupWindow,
		}

		_, err = jsm.AddStream(streamConfig)
		if err != nil {
			return fmt.Errorf("failed to create stream %s: %w", opts.Stream, err)
		}
	} else {
		// Stream exists — check if subject or Duplicates window needs updating
		needsUpdate := false
		updatedConfig := streamInfo.Config

		// Ensure our subject is captured
		found := false
		for _, s := range streamInfo.Config.Subjects {
			if s == opts.Subject {
				found = true
				break
			}
		}
		if !found {
			updatedConfig.Subjects = append(updatedConfig.Subjects, opts.Subject)
			needsUpdate = true
		}

		// Ensure Duplicates window is set (upgrade existing streams)
		if streamInfo.Config.Duplicates != dupWindow {
			logger.Info(fmt.Sprintf("[INFRA:NATS] Stream %s Duplicates window mismatch: existing=%v, desired=%v — updating",
				opts.Stream, streamInfo.Config.Duplicates, dupWindow))
			updatedConfig.Duplicates = dupWindow
			needsUpdate = true
		}

		if needsUpdate {
			if _, err := jsm.UpdateStream(&updatedConfig); err != nil {
				logger.Warn("[INFRA:NATS] Failed to update stream " + opts.Stream + ": " + err.Error())
			} else {
				logger.Info("[INFRA:NATS] Updated stream " + opts.Stream + " (subjects/duplicates)")
			}
		}
	}

	// Now try to get existing consumer
	consumerInfo, err := jsm.ConsumerInfo(opts.Stream, opts.Durable)
	if err != nil {
		// Consumer doesn't exist, create it
		consumerConfig := &nats.ConsumerConfig{
			Durable:       opts.Durable,
			AckPolicy:     nats.AckExplicitPolicy,
			DeliverPolicy: nats.DeliverAllPolicy, // WorkQueue streams require DeliverAll
			FilterSubject: opts.Subject,
			MaxAckPending: 128,
		}

		// For push-based consumers (not pull), we need a DeliverSubject
		// Pull consumers don't need DeliverSubject
		if !opts.Pull {
			// Use inbox-style delivery for push consumers
			consumerConfig.DeliverSubject = nats.NewInbox()
			// Set DeliverGroup if queue group is specified
			if opts.DeliverGroup != "" {
				consumerConfig.DeliverGroup = opts.DeliverGroup
			}
		}

		// Set optional fields if provided
		if opts.AckWait > 0 {
			consumerConfig.AckWait = opts.AckWait
		}
		if opts.MaxAckPending > 0 {
			consumerConfig.MaxAckPending = opts.MaxAckPending
		}

		// Set retry policy configuration (MaxDeliver and Backoff)
		if opts.RetryPolicy != nil {
			consumerConfig.MaxDeliver = opts.RetryPolicy.GetMaxDeliver()
			consumerConfig.BackOff = opts.RetryPolicy.GetBackoffDurations()
			if opts.RetryPolicy.AckWait > 0 {
				consumerConfig.AckWait = opts.RetryPolicy.GetAckWait()
			}
		}

		_, err = jsm.AddConsumer(opts.Stream, consumerConfig)
		if err != nil {
			return fmt.Errorf("failed to create consumer %s for stream %s: %w", opts.Durable, opts.Stream, err)
		}
	} else {
		// Consumer exists - check if it matches what we need
		needsRecreate := false

		// Check pull vs push mode mismatch
		isPullConsumer := consumerInfo.Config.DeliverSubject == ""
		if (isPullConsumer && !opts.Pull) || (!isPullConsumer && opts.Pull) {
			needsRecreate = true
		}

		// Check MaxAckPending mismatch (critical for throughput)
		if opts.MaxAckPending > 0 && consumerInfo.Config.MaxAckPending != opts.MaxAckPending {
			logger.Info(fmt.Sprintf("[INFRA:NATS] Consumer %s MaxAckPending mismatch: existing=%d, desired=%d — recreating",
				opts.Durable, consumerInfo.Config.MaxAckPending, opts.MaxAckPending))
			needsRecreate = true
		}

		if needsRecreate {
			if err := jsm.DeleteConsumer(opts.Stream, opts.Durable); err != nil {
				return fmt.Errorf("failed to delete mismatched consumer %s: %w", opts.Durable, err)
			}
			return c.createOrGetConsumer(opts)
		}
	}

	return nil
}

// Fetch pulls a single message from a pull consumer
// Uses the provided FetchOptions to configure the fetch operation
func (c *Client) Fetch(opts FetchOptions) (*nats.Msg, error) {
	if opts.Stream == "" || opts.Durable == "" {
		return nil, fmt.Errorf("stream and durable name are required for fetch")
	}

	// Set default timeout if not provided
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	// Ensure stream and consumer exist
	if err := c.createOrGetConsumer(SubscribeOptions{
		Stream:  opts.Stream,
		Subject: opts.Subject,
		Durable: opts.Durable,
	}); err != nil {
		return nil, fmt.Errorf("failed to ensure consumer exists: %w", err)
	}

	// Create pull subscription
	sub, err := c.js.PullSubscribe(
		"", // Empty subject - uses consumer's filter subject
		opts.Durable,
		nats.BindStream(opts.Stream),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull subscription: %w", err)
	}
	defer sub.Unsubscribe()

	// Fetch single message
	msgs, err := sub.Fetch(1, nats.MaxWait(timeout))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch message: %w", err)
	}

	if len(msgs) == 0 {
		return nil, fmt.Errorf("no messages available")
	}

	return msgs[0], nil
}

// FetchBatch pulls multiple messages from a pull consumer
// Uses the provided FetchOptions to configure the batch fetch operation
func (c *Client) FetchBatch(opts FetchOptions) ([]*nats.Msg, error) {
	if opts.Stream == "" || opts.Durable == "" {
		return nil, fmt.Errorf("stream and durable name are required for fetch batch")
	}

	// Set default values
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = 10 // Default batch size
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	// Ensure stream and consumer exist
	if err := c.createOrGetConsumer(SubscribeOptions{
		Stream:  opts.Stream,
		Subject: opts.Subject,
		Durable: opts.Durable,
	}); err != nil {
		return nil, fmt.Errorf("failed to ensure consumer exists: %w", err)
	}

	// Create pull subscription
	sub, err := c.js.PullSubscribe(
		"", // Empty subject - uses consumer's filter subject
		opts.Durable,
		nats.BindStream(opts.Stream),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull subscription: %w", err)
	}
	defer sub.Unsubscribe()

	// Fetch batch of messages
	msgs, err := sub.Fetch(batchSize, nats.MaxWait(timeout))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch batch: %w", err)
	}

	return msgs, nil
}
