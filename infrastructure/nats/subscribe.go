package natsModel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// createOrGetConsumer ensures that the stream and consumer exist, creating them if necessary.
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

	ctx := context.Background()

	// Resolve duplicate window: default 15min covers worst-case retry backoff (~12.6min) + margin
	dupWindow := 15 * time.Minute
	if opts.DuplicateWindow > 0 {
		dupWindow = opts.DuplicateWindow
	}

	// First, ensure the stream exists and captures our subject
	stream, err := c.js.Stream(ctx, opts.Stream)
	if err != nil {
		if !errors.Is(err, jetstream.ErrStreamNotFound) {
			return fmt.Errorf("failed to get stream %s: %w", opts.Stream, err)
		}

		// Stream doesn't exist, create it
		streamConfig := jetstream.StreamConfig{
			Name:       opts.Stream,
			Subjects:   []string{opts.Subject},
			Retention:  jetstream.WorkQueuePolicy,
			Storage:    jetstream.FileStorage,
			Duplicates: dupWindow,
		}

		_, err = c.js.CreateStream(ctx, streamConfig)
		if err != nil {
			return fmt.Errorf("failed to create stream %s: %w", opts.Stream, err)
		}
	} else {
		// Stream exists — check if subject or Duplicates window needs updating
		needsUpdate := false
		info := stream.CachedInfo()
		updatedConfig := info.Config

		// Ensure our subject is captured (exact match or wildcard coverage)
		found := false
		for _, s := range info.Config.Subjects {
			if s == opts.Subject {
				found = true
				break
			}
			if strings.HasSuffix(s, ".>") {
				prefix := strings.TrimSuffix(s, ">")
				if strings.HasPrefix(opts.Subject, prefix) {
					found = true
					break
				}
			}
		}
		if !found {
			updatedConfig.Subjects = append(updatedConfig.Subjects, opts.Subject)
			needsUpdate = true
		}

		// Ensure Duplicates window is set (upgrade existing streams)
		if info.Config.Duplicates != dupWindow {
			logger.Info(fmt.Sprintf("[INFRA:NATS] Stream %s Duplicates window mismatch: existing=%v, desired=%v — updating",
				opts.Stream, info.Config.Duplicates, dupWindow))
			updatedConfig.Duplicates = dupWindow
			needsUpdate = true
		}

		if needsUpdate {
			if _, err := c.js.UpdateStream(ctx, updatedConfig); err != nil {
				logger.Warn("[INFRA:NATS] Failed to update stream " + opts.Stream + ": " + err.Error())
			} else {
				logger.Info("[INFRA:NATS] Updated stream " + opts.Stream + " (subjects/duplicates)")
			}
		}
	}

	// Now try to get existing consumer
	cons, err := c.js.Consumer(ctx, opts.Stream, opts.Durable)
	if err != nil {
		if !errors.Is(err, jetstream.ErrConsumerNotFound) {
			return fmt.Errorf("failed to get consumer %s: %w", opts.Durable, err)
		}

		// Consumer doesn't exist, create it
		consumerConfig := jetstream.ConsumerConfig{
			Durable:       opts.Durable,
			AckPolicy:     jetstream.AckExplicitPolicy,
			DeliverPolicy: jetstream.DeliverAllPolicy,
			FilterSubject: opts.Subject,
			MaxAckPending: 128,
		}

		// Set optional fields if provided
		if opts.AckWait > 0 {
			consumerConfig.AckWait = opts.AckWait
		}
		if opts.MaxAckPending > 0 {
			consumerConfig.MaxAckPending = opts.MaxAckPending
		}

		// Set retry policy configuration (MaxDeliver + AckWait only)
		// Backoff is NOT set on the consumer — retry timing is controlled client-side
		// via msg.NakWithDelay() in the MessageHandlerV2/BatchMessageHandlerV2 wrapper.
		// AckWait is a safety net for pod crash only (high value = 5 min).
		if opts.RetryPolicy != nil {
			consumerConfig.MaxDeliver = opts.RetryPolicy.GetMaxDeliver()
			if opts.RetryPolicy.AckWait > 0 {
				consumerConfig.AckWait = opts.RetryPolicy.GetAckWait()
			}
		}

		_, err = c.js.CreateOrUpdateConsumer(ctx, opts.Stream, consumerConfig)
		if err != nil {
			return fmt.Errorf("failed to create consumer %s for stream %s: %w", opts.Durable, opts.Stream, err)
		}
	} else {
		// Consumer exists - check if it matches what we need
		needsRecreate := false
		consInfo := cons.CachedInfo()

		// Check MaxAckPending mismatch (critical for throughput)
		if opts.MaxAckPending > 0 && consInfo.Config.MaxAckPending != opts.MaxAckPending {
			logger.Info(fmt.Sprintf("[INFRA:NATS] Consumer %s MaxAckPending mismatch: existing=%d, desired=%d — recreating",
				opts.Durable, consInfo.Config.MaxAckPending, opts.MaxAckPending))
			needsRecreate = true
		}

		if needsRecreate {
			if err := c.js.DeleteConsumer(ctx, opts.Stream, opts.Durable); err != nil {
				return fmt.Errorf("failed to delete mismatched consumer %s: %w", opts.Durable, err)
			}
			return c.createOrGetConsumer(opts)
		}
	}

	return nil
}
