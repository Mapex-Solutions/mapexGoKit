package natsModel

import (
	"context"

	"github.com/nats-io/nats.go"
)

// Push publishes a message to a NATS JetStream subject using the configured options.
//
// It supports both synchronous and asynchronous publishing. If `Async` is true,
// the message will be sent using `PublishMsgAsync` and the method will wait until
// the publish is completed or the context expires. If `Async` is false, the message
// is published synchronously using `PublishMsg`.
//
// Required fields:
//   - opts.Subject: the subject to publish the message to.
//
// Optional fields in PushOptions:
//   - opts.Data: message payload (required if you want to send content).
//   - opts.Headers: custom headers to include in the message.
//   - opts.ExpectStream: sets the `Nats-Expected-Stream` header.
//   - opts.MsgId: sets the `Nats-Msg-Id` header to support de-duplication.
//   - opts.ExpectLastMsgId: sets the `Nats-Expected-Last-Msg-Id` header for optimistic concurrency.
//   - opts.Async: whether to publish asynchronously.
//   - opts.Ctx: optional context to control the timeout for asynchronous confirmation.
//   - opts.Timeout: duration to wait for async publish completion if no context is provided.
//
// Returns an error if:
//   - The subject is missing.
//   - There is an error during publish.
//   - Async confirmation times out or is cancelled by context.
//
// Example:
//
//	err := client.Push(natsModel.PushOptions{
//	  Subject: "devices.sensor1.data",
//	  Data:    []byte("temperature=22.5"),
//	  MsgId:   "msg-1234",
//	  Async:   true,
//	  Timeout: 2 * time.Second,
//	})
//	if err != nil {
//	  log.Println("Push failed:", err)
//	}
func (c *Client) Push(opts PushOptions) error {
	if opts.Subject == "" {
		return ErrMissingSubject
	}

	msg := &nats.Msg{
		Subject: opts.Subject,
		Data:    opts.Data,
		Header:  nats.Header{},
	}

	for k, v := range opts.Headers {
		msg.Header.Add(k, v)
	}

	if opts.ExpectStream != "" {
		msg.Header.Set("Nats-Expected-Stream", opts.ExpectStream)
	}
	if opts.MsgId != "" {
		msg.Header.Set("Nats-Msg-Id", opts.MsgId)
	}
	if opts.ExpectLastMsgId != "" {
		msg.Header.Set("Nats-Expected-Last-Msg-Id", opts.ExpectLastMsgId)
	}

	if opts.Async {
		// PublishMsgAsync returns a PubAckFuture for THIS specific message.
		// We wait on future.Ok() instead of PublishAsyncComplete() to avoid
		// the convoy effect where all goroutines block waiting for ALL pending
		// messages. Each goroutine now only waits for its own ACK.
		future, err := c.js.PublishMsgAsync(msg)
		if err != nil {
			return err
		}

		// Use provided context or create a default one
		ctx := opts.Ctx
		if ctx == nil {
			if opts.Timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(context.Background(), opts.Timeout)
				defer cancel()
			} else {
				ctx = context.Background()
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-future.Ok():
			return nil
		case err := <-future.Err():
			return err
		}
	}

	_, err := c.js.PublishMsg(msg)
	return err
}

// PublishCore publishes a message using core NATS (fire-and-forget).
// The message is enqueued in the TCP send buffer — no server ACK is awaited.
// Use FlushConnection() after a batch of PublishCore() calls to guarantee
// all messages reached the NATS server in a single TCP roundtrip.
//
// Supports Nats-Msg-Id header for JetStream deduplication when the target
// stream has duplicate_window configured.
func (c *Client) PublishCore(opts PublishCoreOptions) error {
	if opts.Subject == "" {
		return ErrMissingSubject
	}

	msg := &nats.Msg{
		Subject: opts.Subject,
		Data:    opts.Data,
		Header:  nats.Header{},
	}

	for k, v := range opts.Headers {
		msg.Header.Add(k, v)
	}

	if opts.MsgId != "" {
		msg.Header.Set("Nats-Msg-Id", opts.MsgId)
	}

	return c.nc.PublishMsg(msg)
}

// FlushConnection flushes the NATS connection buffer.
// Call this after a batch of PublishCore() calls to ensure all enqueued
// messages are sent to the NATS server in a single TCP roundtrip.
func (c *Client) FlushConnection() error {
	return c.nc.Flush()
}
