package natsModel

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// StartConsumer creates and starts a new managed consumer with automatic goroutine handling
// This method encapsulates all the goroutine management and ensures proper lifecycle control
//
// Parameters:
//   - opts: Consumer configuration including handler function
//
// Returns:
//   - *Consumer: Consumer instance for lifecycle management
//   - error: Any initialization error
//
// Example:
//
//	consumer, err := client.StartConsumer(natsModel.ConsumerOptions{
//	    Stream:       "ORDERS",
//	    Subject:      "order.*.created",
//	    Durable:      "order-processor",
//	    QueueGroup:   "order-workers",
//	    BatchSize:    20,
//	    FetchTimeout: 5 * time.Second,
//	    Handler: func(data []byte, index int, headers map[string][]string) error {
//	        // Your business logic here
//	        return processOrder(data)
//	    },
//	})
func (c *Client) StartConsumer(opts ConsumerOptions) (*Consumer, error) {
	if err := c.validateConsumerOptions(opts); err != nil {
		return nil, fmt.Errorf("invalid consumer options: %w", err)
	}

	// Set defaults
	opts = c.setConsumerDefaults(opts)

	// Create consumer instance
	consumer := &Consumer{
		client:   c,
		options:  opts,
		stopChan: make(chan struct{}),
		stopped:  false,
	}

	// Ensure stream and consumer exist
	// MaxAckPending = BatchSize × 2 to allow double-buffer (1 batch processing + 1 prefetch in flight)
	maxAckPending := opts.BatchSize * 2
	if maxAckPending < 128 {
		maxAckPending = 128 // Minimum default
	}

	if err := c.createOrGetConsumer(SubscribeOptions{
		Stream:          opts.Stream,
		Subject:         opts.Subject,
		Durable:         opts.Durable,
		MaxAckPending:   maxAckPending,
		RetryPolicy:     opts.RetryPolicy,
		DuplicateWindow: opts.DuplicateWindow,
		Pull:            true, // StartConsumer uses PullSubscribe
	}); err != nil {
		return nil, fmt.Errorf("failed to ensure stream/consumer: %w", err)
	}

	// Start the consumer goroutine (encapsulated within the package)
	go consumer.start()

	logger.Info(fmt.Sprintf("[INFRA:NATS] Consumer Started: stream=%s, durable=%s, batch_size=%d",
		opts.Stream, opts.Durable, opts.BatchSize))

	return consumer, nil
}

// fetchResult holds the result of an async fetch operation used by the double-buffer pattern.
type fetchResult struct {
	msgs []*Msg
	err  error
}

// asyncFetch starts a fetch in a background goroutine and returns a channel with the result.
// This enables the double-buffer pattern: prefetch batch N+1 while processing batch N.
func (c *Consumer) asyncFetch(sub *nats.Subscription) <-chan fetchResult {
	ch := make(chan fetchResult, 1)
	go func() {
		msgs, err := sub.Fetch(c.options.BatchSize, nats.MaxWait(c.options.FetchTimeout))
		ch <- fetchResult{msgs: msgs, err: err}
	}()
	return ch
}

// start is the internal goroutine function that handles message consumption.
// It uses a permanent double-buffer pattern:
//   - Always has an async prefetch in flight (zero idle time between batches)
//   - When stream is empty, sub.Fetch(MaxWait) blocks server-side → natural rate-limiting
//   - When stream has data, next batch is already fetched → 0ms idle
func (c *Consumer) start() {
	retryCount := 0

	// Create pull subscription once
	sub, err := c.client.js.PullSubscribe(
		c.options.Subject, // Use the consumer's filter subject
		c.options.Durable,
		nats.BindStream(c.options.Stream),
	)
	if err != nil {
		logger.Error(err, fmt.Sprintf("[INFRA:NATS] Consumer Failed to create pull subscription: %s", c.options.Durable))
		return
	}
	defer sub.Unsubscribe()

	// Always-on double-buffer: first fetch kicks off the pipeline
	nextFetch := c.asyncFetch(sub)

	for {
		// Wait for prefetch result or stop signal
		select {
		case <-c.stopChan:
			logger.Info(fmt.Sprintf("[INFRA:NATS] Consumer Stopping: %s", c.options.Durable))
			return
		case result := <-nextFetch:
			msgs, fetchErr := result.msgs, result.err

			// Immediately start next prefetch (always in flight)
			nextFetch = c.asyncFetch(sub)

			// Handle fetch errors
			if fetchErr != nil {
				if errors.Is(fetchErr, nats.ErrTimeout) {
					continue
				}

				retryCount++
				logger.Error(fetchErr, fmt.Sprintf("[INFRA:NATS] Consumer Fetch error (attempt %d/%d)",
					retryCount, c.options.MaxRetries))

				if retryCount >= c.options.MaxRetries {
					if c.options.StopOnError {
						logger.Error(fmt.Errorf("max retries reached"), fmt.Sprintf("[INFRA:NATS] Consumer Stopping consumer: %s", c.options.Durable))
						c.Stop()
						return
					}
					retryCount = 0
				}

				time.Sleep(c.options.RetryDelay)
				continue
			}

			retryCount = 0

			if len(msgs) == 0 {
				continue
			}

			c.processBatch(msgs)
		}
	}
}

// processBatch handles the processing of a batch of messages.
// Prints blank lines before each batch for visual separation in terminal logs.
// Priority: BatchMessageHandlerV2 > MessageHandlerV2 > BatchHandler > Handler
func (c *Consumer) processBatch(msgs []*Msg) {
	fmt.Println()
	fmt.Println()
	// NEW: BatchMessageHandlerV2 (recommended for batch with retry control)
	if c.options.BatchMessageHandlerV2 != nil {
		c.processBatchV2(msgs)
		return
	}

	// NEW: MessageHandlerV2 (recommended for individual with retry control)
	if c.options.MessageHandlerV2 != nil {
		c.processParallelV2(msgs)
		return
	}

	// Legacy: BatchHandler (bulk processing mode)
	if c.options.BatchHandler != nil {
		c.processBatchBulk(msgs)
		return
	}

	// Legacy: Handler (parallel processing)
	c.processBatchParallel(msgs)
}

// processBatchBulk processes all messages at once using BatchHandler.
// Used for bulk operations like batch database inserts.
func (c *Consumer) processBatchBulk(msgs []*Msg) {
	// Build BatchMessage slice
	batchMessages := make([]BatchMessage, len(msgs))
	for i, msg := range msgs {
		headers := make(map[string][]string)
		if msg.Header != nil {
			for key, values := range msg.Header {
				headers[key] = values
			}
		}
		batchMessages[i] = BatchMessage{
			Data:    msg.Data,
			Headers: headers,
			msg:     msg,
		}
	}

	// Call BatchHandler with all messages
	err := c.options.BatchHandler(batchMessages)

	if err != nil {
		// NAK all messages on error
		logger.Error(err, fmt.Sprintf("[INFRA:NATS] Consumer Batch processing failed, NAKing %d messages", len(msgs)))
		for _, msg := range msgs {
			msg.Nak()
		}
	} else {
		// ACK all messages on success
		logger.Debug(fmt.Sprintf("[INFRA:NATS] Consumer Batch processed successfully, ACKing %d messages", len(msgs)))
		for _, msg := range msgs {
			msg.Ack()
		}
	}
}

// processBatchParallel processes messages in parallel using goroutines.
// Each message is handled independently with its own ACK/NAK.
func (c *Consumer) processBatchParallel(msgs []*Msg) {
	var wg sync.WaitGroup

	// Process messages in parallel using goroutines
	for i, msg := range msgs {
		wg.Add(1)

		// Launch goroutine for each message
		go func(index int, message *Msg) {
			defer wg.Done()

			// Extract headers
			headers := make(map[string][]string)
			if message.Header != nil {
				for key, values := range message.Header {
					headers[key] = values
				}
			}

			// Call user's handler function
			err := c.options.Handler(message.Data, index, headers)

			if err != nil {
				logger.Error(err, fmt.Sprintf("[INFRA:NATS] Consumer Message %d processing failed", index))
				message.Nak() // Reject message - will be redelivered
			} else {
				logger.Debug(fmt.Sprintf("[INFRA:NATS] Consumer Message %d processed successfully", index))
				message.Ack() // Acknowledge message - remove from queue
			}
		}(i, msg)
	}

	// Wait for all messages to be processed
	wg.Wait()
	logger.Debug(fmt.Sprintf("[INFRA:NATS] Consumer Completed parallel processing of %d messages", len(msgs)))
}

// processBatchV2 processes all messages using BatchMessageHandlerV2.
// Each message is wrapped with retry-aware Ack/Nack/Reject methods.
func (c *Consumer) processBatchV2(msgs []*Msg) {
	// Convert to Message wrappers
	messages := make([]*Message, 0, len(msgs))
	for i, msg := range msgs {
		wrapped, err := newMessage(msg, c, i)
		if err != nil {
			logger.Error(err, fmt.Sprintf("[INFRA:NATS] Consumer Failed to wrap message %d, NAKing", i))
			msg.Nak()
			continue
		}
		messages = append(messages, wrapped)
	}

	if len(messages) == 0 {
		return
	}

	// Call user's batch handler - user controls Ack/Nack/Reject for each message
	c.options.BatchMessageHandlerV2(messages)

	logger.Debug(fmt.Sprintf("[INFRA:NATS] Consumer Completed batch V2 processing of %d messages", len(messages)))
}

// processParallelV2 processes messages in parallel using MessageHandlerV2.
// Each message is wrapped with retry-aware Ack/Nack/Reject methods.
func (c *Consumer) processParallelV2(msgs []*Msg) {
	var wg sync.WaitGroup

	for i, msg := range msgs {
		wg.Add(1)

		go func(index int, natsMsg *Msg) {
			defer wg.Done()

			// Wrap the message
			wrapped, err := newMessage(natsMsg, c, index)
			if err != nil {
				logger.Error(err, fmt.Sprintf("[INFRA:NATS] Consumer Failed to wrap message %d, NAKing", index))
				natsMsg.Nak()
				return
			}

			// Call user's handler - user controls Ack/Nack/Reject
			c.options.MessageHandlerV2(wrapped)
		}(i, msg)
	}

	wg.Wait()
	logger.Debug(fmt.Sprintf("[INFRA:NATS] Consumer Completed parallel V2 processing of %d messages", len(msgs)))
}

// Stop gracefully stops the consumer
func (c *Consumer) Stop() {
	if c.stopped {
		return
	}

	c.stopped = true
	close(c.stopChan)
	logger.Info(fmt.Sprintf("[INFRA:NATS] Consumer Stopped: %s", c.options.Durable))
}

// IsRunning returns true if the consumer is currently running
func (c *Consumer) IsRunning() bool {
	return !c.stopped
}

// GetOptions returns a copy of the consumer options
func (c *Consumer) GetOptions() ConsumerOptions {
	return c.options
}

// validateConsumerOptions validates the required fields
func (c *Client) validateConsumerOptions(opts ConsumerOptions) error {
	if opts.Stream == "" {
		return fmt.Errorf("stream name is required")
	}
	if opts.Subject == "" {
		return fmt.Errorf("subject is required")
	}
	if opts.Durable == "" {
		return fmt.Errorf("durable name is required")
	}

	// Count how many handlers are set
	handlerCount := 0
	if opts.Handler != nil {
		handlerCount++
	}
	if opts.BatchHandler != nil {
		handlerCount++
	}
	if opts.MessageHandlerV2 != nil {
		handlerCount++
	}
	if opts.BatchMessageHandlerV2 != nil {
		handlerCount++
	}

	if handlerCount == 0 {
		return fmt.Errorf("one of Handler, BatchHandler, MessageHandlerV2, or BatchMessageHandlerV2 is required")
	}
	if handlerCount > 1 {
		return fmt.Errorf("only one handler type can be set")
	}

	return nil
}

// setConsumerDefaults sets default values for optional fields
func (c *Client) setConsumerDefaults(opts ConsumerOptions) ConsumerOptions {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 50
	}
	if opts.FetchTimeout == 0 {
		opts.FetchTimeout = 5 * time.Second
	}
	if opts.RetryDelay == 0 {
		opts.RetryDelay = 2 * time.Second
	}
	if opts.MaxRetries <= 0 {
		opts.MaxRetries = 5
	}
	return opts
}

// ConsumerManager helps manage multiple consumers
type ConsumerManager struct {
	consumers map[string]*Consumer
	mu        sync.RWMutex
}

// NewConsumerManager creates a new consumer manager
func NewConsumerManager() *ConsumerManager {
	return &ConsumerManager{
		consumers: make(map[string]*Consumer),
	}
}

// AddConsumer adds a consumer to the manager
func (cm *ConsumerManager) AddConsumer(name string, consumer *Consumer) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.consumers[name] = consumer
}

// StopConsumer stops a specific consumer
func (cm *ConsumerManager) StopConsumer(name string) {
	cm.mu.RLock()
	consumer, exists := cm.consumers[name]
	cm.mu.RUnlock()

	if exists {
		consumer.Stop()
	}
}

// StopAll stops all consumers
func (cm *ConsumerManager) StopAll() {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, consumer := range cm.consumers {
		consumer.Stop()
	}
}

// GetConsumer returns a consumer by name
func (cm *ConsumerManager) GetConsumer(name string) (*Consumer, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	consumer, exists := cm.consumers[name]
	return consumer, exists
}

// =============================================================================
// FANOUT - Ephemeral Broadcast Consumers
// =============================================================================

// Stop gracefully stops the FANOUT subscription.
func (fs *FanoutSubscription) Stop() error {
	var err error
	fs.StopOnce.Do(func() {
		if fs.Sub != nil {
			err = fs.Sub.Drain()
		}
	})
	return err
}

// SubscribeFanout creates an ephemeral subscription to a FANOUT subject.
// All instances receive all messages (broadcast pattern).
func (b *Bus) SubscribeFanout(stream, serviceName, subject string, handler FanoutHandler) (*FanoutSubscription, error) {
	if handler == nil {
		return nil, fmt.Errorf("handler is required")
	}
	if serviceName == "" {
		return nil, fmt.Errorf("service name is required")
	}
	if subject == "" {
		return nil, fmt.Errorf("subject is required")
	}
	if stream == "" {
		return nil, fmt.Errorf("stream is required")
	}

	timestamp := time.Now().Format("20060102-150405")
	consumerName := fmt.Sprintf("%s-fanout-%s", serviceName, timestamp)

	logger.Info(fmt.Sprintf("[INFRA:NATS] FANOUT Creating ephemeral subscription: %s -> %s (stream: %s)", consumerName, subject, stream))

	sub, err := b.c.js.Subscribe(
		subject,
		func(msg *nats.Msg) {
			if err := handler(msg.Data); err != nil {
				logger.Warn(fmt.Sprintf("[INFRA:NATS] FANOUT Handler error on %s: %v", subject, err))
			}
			_ = msg.Ack()
		},
		nats.BindStream(stream),
		nats.DeliverNew(),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.InactiveThreshold(5*time.Minute),
		nats.ConsumerName(consumerName),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create fanout subscription: %w", err)
	}

	logger.Info(fmt.Sprintf("[INFRA:NATS] FANOUT Subscription active: %s", consumerName))
	return &FanoutSubscription{Sub: sub}, nil
}

// EnsureFanoutStream ensures a FANOUT stream exists with appropriate settings.
func (b *Bus) EnsureFanoutStream(config FanoutStreamConfig) error {
	if config.Name == "" {
		return fmt.Errorf("stream name is required")
	}
	if len(config.Subjects) == 0 {
		return fmt.Errorf("at least one subject is required")
	}

	jsm := b.c.js

	_, err := jsm.StreamInfo(config.Name)
	if err == nil {
		return nil
	}

	maxAge := config.MaxAge
	if maxAge == 0 {
		maxAge = 5 * time.Minute
	}
	maxMsgs := config.MaxMsgs
	if maxMsgs == 0 {
		maxMsgs = 10000
	}
	maxBytes := config.MaxBytes
	if maxBytes == 0 {
		maxBytes = 10 * 1024 * 1024
	}

	streamConfig := &nats.StreamConfig{
		Name:        config.Name,
		Description: config.Description,
		Subjects:    config.Subjects,
		Retention:   nats.LimitsPolicy,
		MaxAge:      maxAge,
		MaxMsgs:     maxMsgs,
		MaxBytes:    maxBytes,
		Storage:     nats.MemoryStorage,
		Replicas:    1,
		Discard:     nats.DiscardOld,
	}

	_, err = jsm.AddStream(streamConfig)
	if err != nil {
		return fmt.Errorf("failed to create stream %s: %w", config.Name, err)
	}

	logger.Info(fmt.Sprintf("[INFRA:NATS] FANOUT Created stream: %s", config.Name))
	return nil
}
