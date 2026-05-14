// Package natsports declares tiny, tech-agnostic interfaces that your
// services/consumers depend on. Keep them minimal and reusable.
package natsModel

import (
	"context"

	natsgo "github.com/nats-io/nats.go"
)

// Publisher is the minimal contract for sending messages.
type Publisher interface {
	Publish(config PublishConfig) error
}

// Subscriber is the minimal contract for receiving messages.
type Subscriber interface {
	Subscribe(config SubscribeConfig) (stop func() error, err error)
}

// Fetcher is the contract for fetch-based message consumption.
type Fetcher interface {
	Fetch(config FetchConfig) (stop func() error, err error)
}

// Fanout is the contract for FANOUT broadcast messaging.
// All subscribers receive all messages - no queue groups, no durable consumers.
type Fanout interface {
	// PublishFanout broadcasts a message to all subscribers of a subject
	PublishFanout(ctx context.Context, subject string, data []byte) error

	// SubscribeFanout creates an ephemeral subscription (broadcast pattern)
	SubscribeFanout(stream, serviceName, subject string, handler FanoutHandler) (*FanoutSubscription, error)

	// EnsureFanoutStream creates a FANOUT stream if it doesn't exist
	EnsureFanoutStream(config FanoutStreamConfig) error
}

// CorePublisher is the contract for fire-and-forget publishing with batch flush.
// Use this for high-throughput consumers that publish multiple messages per event.
// Pattern: PublishCore() × N → FlushConnection() → single TCP roundtrip.
type CorePublisher interface {
	PublishCore(config PublishCoreConfig) error
	FlushConnection() error
}

// ScheduleManager is the contract for NATS JetStream message scheduling operations.
// Supports scheduled publish (@at), subject-based stream purge, and pending message checks.
type ScheduleManager interface {
	// PublishScheduled publishes a message with Nats-Schedule headers for delayed delivery.
	// If MsgId is set, NATS rejects duplicates within the stream's Duplicates window.
	PublishScheduled(config ScheduledPublishConfig) error

	// PurgeStreamSubject purges all messages matching a subject pattern from a stream.
	// Idempotent: returns nil if no messages match (already fired or never published).
	PurgeStreamSubject(stream, subject string) error

	// HasPendingMessages checks if a stream has any messages matching the given subject.
	// Returns true if at least one message exists, false otherwise.
	// Idempotent: returns false if the stream doesn't exist.
	HasPendingMessages(stream, subject string) (bool, error)
}

// ConnectionProvider provides access to the underlying NATS connection.
// Used for special cases like Auth Callout that require request-reply pattern.
type ConnectionProvider interface {
	GetConn() *natsgo.Conn
}

/**
Combined Interfaces
*/

// FanoutPublisher combines Fanout publishing with connection access.
// Use this for services that need to publish FANOUT messages.
type FanoutPublisher interface {
	Fanout
}

// MessageSubscriber combines Subscribe with connection access.
// Use this for consumers that need standard JetStream subscriptions.
type MessageSubscriber interface {
	Subscriber
}

// AuthCalloutSubscriber provides connection access for Auth Callout pattern.
// Auth Callout requires direct NATS connection for request-reply.
type AuthCalloutSubscriber interface {
	ConnectionProvider
}

// KeyValueStore is the contract for NATS KV operations with CAS (Compare-And-Swap) support.
// Used for hot state persistence where atomicity and low-latency are critical.
//
// CAS workflow:
//
//	entry, err := kv.Get("inst:123")       // → value + revision
//	// ... modify value ...
//	_, err = kv.Update("inst:123", newVal, entry.Revision)  // → fails if revision changed
type KeyValueStore interface {
	// Get retrieves a value by key. Returns ErrKVKeyNotFound if the key doesn't exist.
	Get(key string) (*KVEntry, error)

	// Put stores a value, creating or overwriting the key. Returns the new revision.
	Put(key string, value []byte) (uint64, error)

	// Create stores a value ONLY IF the key doesn't exist.
	// Returns ErrKVKeyExists if the key already exists.
	Create(key string, value []byte) (uint64, error)

	// Update stores a value ONLY IF the current revision matches expectedRevision (CAS).
	// Returns ErrKVCASConflict if the revision doesn't match.
	Update(key string, value []byte, expectedRevision uint64) (uint64, error)

	// Delete soft-deletes a key (marks as deleted, preservable in history).
	// Returns ErrKVKeyNotFound if the key doesn't exist.
	Delete(key string) error

	// Purge removes a key and ALL its historical revisions.
	Purge(key string) error

	// Keys returns all keys in the bucket. Use sparingly — scans entire bucket.
	Keys() ([]string, error)

	// Bucket returns the name of the underlying KV bucket.
	Bucket() string
}
