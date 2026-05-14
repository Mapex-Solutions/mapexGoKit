package natsModel

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Alias for nats.Msg — core NATS message type used in publish operations
type Msg = nats.Msg

// Alias for nats.Option — connection option type
type Option = nats.Option

// Alias for nats.Options — connection options struct
type Options = nats.Options

// Alias for jetstream.StorageType — storage backend type (FileStorage, MemoryStorage)
type StorageType = jetstream.StorageType

type Client struct {
	nc *nats.Conn
	js jetstream.JetStream
}

type Config struct {
	Options Options

	// OnConnect is an optional callback invoked after the initial connection
	// and after every reconnect. Use it to publish bootstrap messages
	// (e.g., Reconciler sweep trigger) without accessing the raw nats.Conn.
	OnConnect func(c *Client)
}

type SubscribeOptions struct {
	Stream          string
	Subject         string
	Durable         string
	DeliverGroup    string          // Optional: Queue group (round-robin)
	AckWait         time.Duration   // Optional
	MaxAckPending   int             // Optional
	Pull            bool            // true = PullConsumer
	Handler         nats.MsgHandler // Required if Push
	RetryPolicy     *RetryPolicy    // Optional: Retry configuration
	DuplicateWindow time.Duration   // Optional: JetStream dedup window (default: 15min)
}

type PushOptions struct {
	Ctx             context.Context   // Optional: context for async flow
	Subject         string            // Required
	Data            []byte            // Required
	Headers         map[string]string // Optional: custom headers
	ExpectStream    string            // Optional: Nats-Expected-Stream
	MsgId           string            // Optional: Nats-Msg-Id
	ExpectLastMsgId string            // Optional: Nats-Expected-Last-Msg-Id
	Async           bool              // Async publish
	Timeout         time.Duration     // Used only if no Ctx is given and Async is true
}

// PublishCoreOptions configures a core NATS publish (fire-and-forget).
// The message is enqueued in the TCP send buffer — no server ACK is awaited.
// Use FlushConnection() after a batch of PublishCore() calls to guarantee
// all messages reached the NATS server in a single TCP roundtrip.
type PublishCoreOptions struct {
	Subject string            // Required: NATS subject
	Data    []byte            // Required: message payload (pre-marshaled)
	MsgId   string            // Optional: Nats-Msg-Id for JetStream dedup
	Headers map[string]string // Optional: custom headers
}

// PublishCoreConfig configures a core NATS publish via Bus adapter.
// Similar to PublishConfig but uses core NATS (no JetStream ACK).
type PublishCoreConfig struct {
	Subject string            // Required: NATS subject
	Data    any               // Required: message payload (will be JSON marshaled)
	MsgId   string            // Optional: Nats-Msg-Id for JetStream dedup
	Headers map[string]string // Optional: custom headers
}

// FetchOptions configures pull consumer fetch operations
type FetchOptions struct {
	Stream    string        // Required: Stream name
	Subject   string        // Required: Subject pattern
	Durable   string        // Required: Consumer name
	BatchSize int           // Optional: Number of messages to fetch (default: 1 for Fetch, 10 for FetchBatch)
	Timeout   time.Duration // Optional: Fetch timeout (default: 5s)
}

// MessageHandler defines the function signature for processing individual messages.
// Each message is processed independently with its own ACK/NAK.
//
// Parameters:
//   - data: The message payload
//   - index: Position in batch (0 for single messages)
//   - headers: Message headers (if any)
//
// Returns:
//   - error: nil for success (ACK), error for failure (NAK)
type MessageHandler func(data []byte, index int, headers map[string][]string) error

// BatchMessage represents a single message within a batch for BatchHandler.
// Provides access to message data and manual ACK/NAK control.
type BatchMessage struct {
	Data    []byte              // The message payload
	Headers map[string][]string // Message headers
	msg     jetstream.Msg       // Internal reference for ACK/NAK
}

// Ack acknowledges the message (removes from queue)
func (m *BatchMessage) Ack() error {
	if m.msg != nil {
		return m.msg.Ack()
	}
	return nil
}

// Nak negatively acknowledges the message (will be redelivered)
func (m *BatchMessage) Nak() error {
	if m.msg != nil {
		return m.msg.Nak()
	}
	return nil
}

// BatchHandler defines the function signature for processing a complete batch of messages.
// All messages are passed at once, allowing bulk operations (e.g., batch database inserts).
// The handler is responsible for calling Ack()/Nak() on each message, OR returning
// an error to NAK all messages, OR returning nil to ACK all messages.
//
// Parameters:
//   - messages: Slice of BatchMessage containing all messages in the batch
//
// Returns:
//   - error: nil to ACK all messages, error to NAK all messages
//
// Example usage:
//
//	BatchHandler: func(messages []BatchMessage) error {
//	    // Process all messages
//	    entities := make([]*Entity, len(messages))
//	    for i, msg := range messages {
//	        entity, err := parseMessage(msg.Data)
//	        if err != nil {
//	            msg.Nak() // NAK individual bad message
//	            continue
//	        }
//	        entities[i] = entity
//	    }
//	    // Bulk insert
//	    if err := repo.SaveBatch(entities); err != nil {
//	        return err // NAK all remaining
//	    }
//	    return nil // ACK all
//	}
type BatchHandler func(messages []BatchMessage) error

// MessageHandlerV2 is the new handler signature with full control over message lifecycle.
// The handler receives a Message wrapper with retry-aware methods.
// The handler is responsible for calling Ack(), Nack(err), Reject(reason), or Term().
//
// Example:
//
//	func(msg *Message) {
//	    if err := process(msg.Data); err != nil {
//	        if isFatal(err) {
//	            msg.Reject("invalid format")
//	        } else {
//	            msg.Nack(err) // Will retry with backoff
//	        }
//	        return
//	    }
//	    msg.Ack()
//	}
type MessageHandlerV2 func(msg *Message)

// BatchMessageHandlerV2 is the new batch handler with full control over message lifecycle.
// Each message can be handled individually with Ack/Nack/Reject/Term.
//
// Example:
//
//	func(messages []*Message) {
//	    for _, msg := range messages {
//	        if err := process(msg.Data); err != nil {
//	            msg.Nack(err)
//	        } else {
//	            msg.Ack()
//	        }
//	    }
//	}
type BatchMessageHandlerV2 func(messages []*Message)

// ConsumerOptions configures the consumer behavior.
//
// # Handler Types
//
// There are two generations of handlers: Legacy (V1) and New (V2).
// Only ONE handler type should be set per consumer.
//
// ## Legacy Handlers (V1) - Return-based control
//
// These handlers return an error to signal success/failure.
// The framework automatically ACKs (nil) or NAKs (error) all messages.
//
//   - Handler: Process messages individually (parallel goroutines)
//   - BatchHandler: Process all messages at once (bulk operations)
//
// Limitations:
//   - All-or-nothing: error NAKs ALL messages, nil ACKs ALL
//   - No retry/DLQ support: immediate redelivery on NAK
//   - No per-message control
//
// ## New Handlers (V2) - Direct control (RECOMMENDED)
//
// These handlers give you full control over each message's lifecycle.
// You call msg.Ack(), msg.Nack(err), msg.Reject(reason), or msg.Term() directly.
//
//   - MessageHandlerV2: Process messages individually with full control
//   - BatchMessageHandlerV2: Process batches with per-message control
//
// Benefits:
//   - Per-message decisions: ACK some, Reject others, Nack rest
//   - Retry with backoff: msg.Nack(err) retries with exponential backoff
//   - DLQ support: msg.Reject(reason) sends directly to DLQ
//   - Silent discard: msg.Term() drops message without retry/DLQ
//
// ## Message Methods (V2 only)
//
//   - msg.Ack(): Success - remove from queue
//   - msg.Nack(err): Retry with backoff, then DLQ after max retries
//   - msg.Reject(reason): Skip retry, send directly to DLQ (invalid data)
//   - msg.Term(): Discard silently (no retry, no DLQ)
//
// ## When to use each method
//
//   - Ack(): Message processed successfully
//   - Nack(err): Transient error (DB timeout, network issue) - worth retrying
//   - Reject(reason): Permanent error (invalid JSON, schema mismatch) - no point retrying
//   - Term(): Intentionally ignore (duplicate, outdated, test data)
//
// # Example (V2 Recommended)
//
//	consumer, err := bus.StartConsumer(natsModel.ConsumerOptions{
//	    Stream:  "EVENTS-RAW",
//	    Subject: "events.raw",
//	    Durable: "events-processor",
//	    RetryPolicy: &natsModel.RetryPolicy{
//	        MaxRetries: 5,
//	        Backoff:    []time.Duration{1*time.Second, 5*time.Second, 30*time.Second},
//	    },
//	    DLQPolicy: &natsModel.DLQPolicy{
//	        ServiceName: "events-service",
//	        ServiceType: "processor",
//	        EventType:   "raw",
//	    },
//	    BatchMessageHandlerV2: func(messages []*natsModel.Message) {
//	        for _, msg := range messages {
//	            if err := validate(msg.Data); err != nil {
//	                msg.Reject("invalid format")  // → DLQ immediately
//	                continue
//	            }
//	            if err := process(msg.Data); err != nil {
//	                msg.Nack(err)  // → Retry with backoff
//	                continue
//	            }
//	            msg.Ack()  // → Success
//	        }
//	    },
//	})
type ConsumerOptions struct {
	Stream       string        // Required: Stream name
	Subject      string        // Required: Subject pattern to subscribe to
	Durable      string        // Required: Consumer name
	QueueGroup   string        // Optional: Queue group for load balancing
	BatchSize    int           // Optional: Messages per batch (default: 50)
	FetchTimeout time.Duration // Optional: Fetch timeout (default: 5s)
	RetryDelay   time.Duration // Optional: Delay before retry on error (default: 2s)
	MaxRetries   int           // Optional: Max retries before giving up (default: 5)
	StopOnError  bool          // Optional: Stop consumer on persistent errors (default: false)

	// DuplicateWindow configures the JetStream stream's Duplicates window.
	// When set (> 0), the stream will reject messages with the same Nats-Msg-Id
	// within this time window. Requires publishers to set MsgId on PublishCore calls.
	// Default: 15 minutes (covers worst-case retry backoff of ~12.6min + margin).
	DuplicateWindow time.Duration

	// RetryPolicy configures automatic retry with exponential backoff.
	// Used by V2 handlers for msg.Nack(err) behavior.
	// If nil, Nack causes immediate redelivery (no backoff).
	RetryPolicy *RetryPolicy

	// DLQPolicy configures Dead Letter Queue behavior.
	// Used by V2 handlers for msg.Nack(err) after max retries and msg.Reject(reason).
	// If nil, messages are terminated (discarded) after max retries.
	// Default stream: MAPEXOS-DLQ, default subject: dlq.mapexos
	DLQPolicy *DLQPolicy

	// Handler processes messages individually in parallel goroutines.
	// LEGACY (V1): Returns error to NAK, nil to ACK. No retry/DLQ support.
	Handler MessageHandler

	// BatchHandler processes all messages in a batch at once.
	// LEGACY (V1): Returns error to NAK all, nil to ACK all. No retry/DLQ support.
	BatchHandler BatchHandler

	// MessageHandlerV2 processes messages individually with full lifecycle control.
	// RECOMMENDED: Use msg.Ack(), msg.Nack(err), msg.Reject(reason), or msg.Term().
	MessageHandlerV2 MessageHandlerV2

	// BatchMessageHandlerV2 processes batches with per-message lifecycle control.
	// RECOMMENDED: Use msg.Ack(), msg.Nack(err), msg.Reject(reason), or msg.Term().
	BatchMessageHandlerV2 BatchMessageHandlerV2
}

// Consumer represents a running consumer instance
type Consumer struct {
	client   *Client
	options  ConsumerOptions
	stopChan chan struct{}
	stopped  bool
}

// PublishConfig configures message publishing
type PublishConfig struct {
	Ctx     context.Context   `json:"-"`              // Optional: context for cancellation
	Subject string            `json:"subject"`        // Required: NATS subject
	Data    any               `json:"data"`           // Required: message payload (will be JSON marshaled)
	Headers map[string]string `json:"headers"`        // Optional: message headers
}

// SubscribeConfig configures message subscription
type SubscribeConfig struct {
	Ctx     context.Context     `json:"-"`         // Optional: context for cancellation
	Stream  string              `json:"stream"`    // Required: JetStream stream name
	Subject string              `json:"subject"`   // Required: NATS subject pattern
	Durable string              `json:"durable"`   // Required: durable consumer name
	Group   string              `json:"group"`     // Optional: queue group for load balancing
	Handler func([]byte) error  `json:"-"`         // Required: message handler function
	Pull    bool                `json:"pull"`      // Optional: use pull-based subscription (default: false)
}

// FetchConfig configures fetch-based message consumption with batch support
type FetchConfig struct {
	Ctx       context.Context    `json:"-"`          // Optional: context for cancellation
	Stream    string             `json:"stream"`     // Required: JetStream stream name
	Subject   string             `json:"subject"`    // Required: NATS subject pattern
	Durable   string             `json:"durable"`    // Required: durable consumer name
	Group     string             `json:"group"`      // Optional: queue group for load balancing
	BatchMode bool               `json:"batch_mode"` // Optional: enable batch processing (default: false)
	BatchSize int                `json:"batch_size"` // Optional: messages per batch (default: 10)
	Timeout   int                `json:"timeout"`    // Optional: fetch timeout in seconds (default: 5)

	// Handler processes messages individually (one goroutine per message).
	// Use for independent message processing.
	// Either Handler OR BatchHandler must be set, not both.
	Handler func([]byte) error `json:"-"`

	// BatchHandler processes all messages in a batch at once.
	// Use for bulk operations like batch database inserts.
	// Either Handler OR BatchHandler must be set, not both.
	BatchHandler func(messages []BatchMessage) error `json:"-"`
}

/**
Retry Policy Types
*/

// RetryPolicy configures retry behavior for failed messages.
// When a message processing fails, the library will automatically retry
// with exponential backoff until MaxRetries is reached.
type RetryPolicy struct {
	// MaxRetries is the maximum number of retry attempts before sending to DLQ.
	// Default: 5
	MaxRetries int

	// Backoff defines the delay sequence for retries.
	// E.g., [1s, 5s, 30s, 2m, 10m]
	// If fewer entries than MaxRetries, last value repeats for remaining retries.
	// Default: [1s, 5s, 30s, 2m, 10m]
	Backoff []time.Duration

	// AckWait is the time the server waits for an ACK before considering
	// the message unacknowledged and redelivering it.
	// Default: 30s
	AckWait time.Duration
}

/**
DLQ (Dead Letter Queue) Types
*/

// DLQPolicy configures Dead Letter Queue behavior and context.
// Provides metadata for filtering and querying failed messages.
type DLQPolicy struct {
	// Stream is the DLQ stream name.
	// Default: "MAPEXOS-DLQ"
	Stream string

	// Subject is the subject pattern for DLQ messages.
	// Default: "dlq.{ServiceName}"
	Subject string

	// === Context for filtering in DB ===

	// ServiceName identifies the service (e.g., "events-service", "router-service")
	ServiceName string

	// ServiceType categorizes the service (e.g., "processor", "gateway", "worker")
	ServiceType string

	// EventType describes what kind of events this consumer handles
	// (e.g., "sensor.data", "user.created", "order.payment")
	EventType string
}

// DLQMessage represents a message sent to the Dead Letter Queue.
// Contains all context needed for debugging and reprocessing.
type DLQMessage struct {
	// === Unique identifier ===
	ID string `json:"id"`

	// === Event tracking (for correlation across pipeline) ===
	EventTrackerId string `json:"eventTrackerId"`

	// === Tenant context (MANDATORY for multi-tenant filtering) ===
	OrgId   string `json:"orgId"`
	PathKey string `json:"pathKey"`

	// === Service context (for filtering) ===
	ServiceName string `json:"serviceName"`
	ServiceType string `json:"serviceType"`
	EventType   string `json:"eventType"`

	// === Original message ===
	OriginalSubject string            `json:"originalSubject"`
	OriginalStream  string            `json:"originalStream"`
	OriginalData    json.RawMessage   `json:"originalData"`
	OriginalHeaders map[string]string `json:"originalHeaders,omitempty"`

	// === Error information ===
	LastError  string `json:"lastError"`
	ErrorCount int    `json:"errorCount"`

	// === Delivery tracking ===
	FirstDelivery   time.Time `json:"firstDelivery"`
	LastDelivery    time.Time `json:"lastDelivery"`
	TotalDeliveries int       `json:"totalDeliveries"`

	// === Consumer context ===
	ConsumerName string `json:"consumerName"`

	// === Timestamps ===
	SentToDLQAt time.Time `json:"sentToDLQAt"`
}

/**
FANOUT Types - Ephemeral Broadcast Messaging
*/

// FanoutHandler processes a FANOUT message.
// Returns error only for logging purposes (message is always acked).
type FanoutHandler func(data []byte) error

// FanoutStreamConfig configures a FANOUT stream.
type FanoutStreamConfig struct {
	// Name is the stream name (required)
	Name string

	// Subjects is the list of subject patterns (required)
	// Example: []string{"mapexos.cache.>"}
	Subjects []string

	// MaxAge is how long messages are kept (default: 5 minutes)
	MaxAge time.Duration

	// MaxMsgs is max number of messages (default: 10000)
	MaxMsgs int64

	// MaxBytes is max total size (default: 10MB)
	MaxBytes int64

	// Description is optional stream description
	Description string
}

// FanoutSubscription represents an active ephemeral FANOUT subscription.
type FanoutSubscription struct {
	Sub      *nats.Subscription      // Legacy: kept for core NATS subscriptions
	cc       jetstream.ConsumeContext // New: used by jetstream consumers
	StopOnce sync.Once
}

/**
Schedule Types
*/

// ScheduledPublishConfig configures a NATS JetStream scheduled message publish.
// The message is stored in the stream and delivered to TargetSubject at ScheduleAt time.
// Uses Nats-Schedule: @at {RFC3339} header (ADR-51, requires AllowMsgSchedules on stream).
type ScheduledPublishConfig struct {
	Subject       string            // Required: subject in the schedule stream
	TargetSubject string            // Required: where NATS delivers the message at ScheduleAt time
	ScheduleAt    time.Time         // Required: when to deliver (UTC, RFC3339)
	Data          any               // Required: message payload (will be JSON marshaled)
	MsgId         string            // Optional: deduplication ID (rejected if duplicate within stream's Duplicates window)
	Headers       map[string]string // Optional: additional headers
}

/**
KeyValue (KV) Types
*/

// KVConfig configures a NATS KeyValue bucket.
// Used by Client.CreateKeyValue to create or bind to an existing bucket.
type KVConfig struct {
	// Bucket is the name of the KV bucket (required).
	// Must be a valid NATS subject token (alphanumeric, dash, underscore).
	Bucket string

	// Description is an optional human-readable description.
	Description string

	// TTL is the time-to-live for keys. 0 means no expiry.
	TTL time.Duration

	// MaxBytes is the maximum total size of the bucket. 0 means unlimited.
	MaxBytes int64

	// MaxValueSize is the maximum size of a single value. 0 means default (server limit).
	MaxValueSize int32

	// History is the number of historical values to keep per key. Default: 1 (no history).
	History uint8

	// Replicas is the number of stream replicas for the KV bucket.
	// Default: 1 (single node). Production: 3 (cluster HA).
	Replicas int

	// Storage selects the storage backend. Default: jetstream.FileStorage (disk-backed).
	Storage StorageType
}

// KVEntry represents a single entry retrieved from a NATS KV bucket.
type KVEntry struct {
	// Key is the entry key.
	Key string

	// Value is the entry payload.
	Value []byte

	// Revision is the sequence number of this entry. Used for CAS (Compare-And-Swap).
	Revision uint64

	// Created is the timestamp when this revision was stored.
	Created time.Time
}
