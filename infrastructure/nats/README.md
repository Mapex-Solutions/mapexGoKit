# nats — JetStream client, consumers, KV, FANOUT, DLQ

Wrapper around [`nats.go`](https://github.com/nats-io/nats.go) and [`nats.go/jetstream`](https://github.com/nats-io/nats.go/tree/main/jetstream). Covers the full set of patterns Mapex uses on a single bus:

- JetStream **publish** (sync/async, dedup, optimistic concurrency)
- Core NATS **fire-and-forget** publish + batch flush
- **Pull-based managed consumer** with double-buffer prefetch, retries, DLQ, V1 + V2 handler generations
- **FANOUT** (ephemeral broadcast) over JetStream
- **KV bucket** with CAS (Compare-And-Swap)
- **Scheduled publish** via `Nats-Schedule` header
- Stream housekeeping: `EnsureStream`, `PurgeStreamSubject`, `HasPendingMessages`

> Package name: `natsModel` (directory: `nats/`).

## Architecture in one diagram

```
                        ┌────────────┐  ports.go
                        │  Publisher │
                        │ Subscriber │
                        │   Fetcher  │
   service code ───────▶│   Fanout   │◀── *Bus  ──┐
                        │ CorePub.   │             │
                        │ ScheduleM. │             │
                        └────────────┘             │
                                                   ▼
                                       ┌────────────────────┐
                                       │   *Client          │  nats.go +
                                       │  nc, js (JetStream)│  jetstream
                                       └────────┬───────────┘
                                                │
                          Push / PublishCore / StartConsumer / KV / FANOUT
```

`Bus` is a thin adapter that implements every port and forwards to `Client`. Service code depends on the *interfaces* in `ports.go`, not on `Client` directly.

## Construction

### `Config`

```go
type Config struct {
    Options   nats.Options                    // built via nats.GetDefaultOptions().*, nats.Servers(...), etc.
    OnConnect func(c *Client)                 // called once after initial connect, then again after every reconnect
}
```

### `New(cfg) (*Client, error)`

1. `cfg.Options.Connect()` opens the underlying `*nats.Conn`.
2. `jetstream.New(nc)` builds the JetStream context.
3. If `OnConnect` is set, invokes it once and registers a reconnect handler that re-invokes it after every successful reconnect (logged as `[INFRA:NATS] Reconnected — calling OnConnect callback`).
4. Logs `[INFRA:NATS] Connected to server`.

Methods on `*Client`:

| Method | Purpose |
|---|---|
| `Ping() (time.Duration, error)` | Native PING/PONG round-trip via `nc.RTT()`. Errors when `nc.IsClosed()`. |
| `IsConnected() bool` | `nc != nil && !nc.IsClosed()`. |
| `Close()` | Idempotent — safe on already-closed client. |

`Bus` wraps a `Client`:

```go
bus := natsModel.NewBus(client)
_ = (*natsModel.Bus)(nil) // implements: Publisher, Subscriber, Fetcher, Fanout, CorePublisher, ScheduleManager
bus.GetConn() // raw *nats.Conn for request-reply patterns (Auth Callout)
```

## Ports (interfaces in `ports.go`)

```go
type Publisher       interface { Publish(PublishConfig) error }
type Subscriber      interface { Subscribe(SubscribeConfig) (stop func() error, err error) }
type Fetcher         interface { Fetch(FetchConfig) (stop func() error, err error) }
type Fanout          interface { PublishFanout(ctx, subject, data); SubscribeFanout(stream, serviceName, subject, FanoutHandler); EnsureFanoutStream(FanoutStreamConfig) }
type CorePublisher   interface { PublishCore(PublishCoreConfig); FlushConnection() }
type ScheduleManager interface { PublishScheduled(ScheduledPublishConfig); PurgeStreamSubject(stream, subject); HasPendingMessages(stream, subject) (bool, error) }
type ConnectionProvider interface { GetConn() *nats.Conn }
type KeyValueStore   interface { Get; Put; Create; Update; Delete; Purge; Keys; Bucket }
```

## Publishing

### JetStream `Push` (sync or async)

```go
err := bus.Publish(natsModel.PublishConfig{
    Ctx:     ctx,                          // optional
    Subject: "events.raw",                 // required
    Data:    payload,                      // any — JSON-marshaled by Bus
    Headers: map[string]string{"x-trace": traceID},
})
```

Underneath, `Client.Push(PushOptions)` adds optional headers:

| Field | Header set |
|---|---|
| `MsgId` | `Nats-Msg-Id` (deduplication within stream `Duplicates` window) |
| `ExpectStream` | `Nats-Expected-Stream` |
| `ExpectLastMsgId` | `Nats-Expected-Last-Msg-Id` (optimistic concurrency) |

When `Async=true`, `Push` waits on **the per-message future** (`future.Ok()` / `future.Err()`) — not `PublishAsyncComplete()` — to avoid the "convoy effect" where every goroutine blocks on every pending message. Honors `Ctx` first, then `Timeout`, then `context.Background()`.

### Core NATS fire-and-forget

For high-throughput batches:

```go
for _, e := range events {
    _ = bus.PublishCore(natsModel.PublishCoreConfig{
        Subject: "events.raw",
        Data:    e,
        MsgId:   e.ID, // dedup if stream has Duplicates window
    })
}
_ = bus.FlushConnection() // single TCP roundtrip for the whole batch
```

`PublishCore` enqueues into the TCP buffer without awaiting an ACK. `FlushConnection()` is what guarantees the bytes left the box.

### Scheduled publish (`Nats-Schedule` header)

```go
err := bus.PublishScheduled(natsModel.ScheduledPublishConfig{
    Subject:       "schedules.events",
    TargetSubject: "events.delayed",
    ScheduleAt:    time.Now().Add(10*time.Minute),
    Data:          payload,
    MsgId:         "evt-123", // dedup within the schedule stream
})
```

The wrapper sets `Nats-Schedule: @at <RFC3339>` and `Nats-Schedule-Target: <subject>`. **Requires the target stream to have `AllowMsgSchedules: true`.**

## Subscribing

There are three subscribe paths with different ergonomics. Pick exactly one per consumer.

### `Bus.Subscribe` — callback-based, simple

```go
stop, err := bus.Subscribe(natsModel.SubscribeConfig{
    Stream:  "EVENTS",
    Subject: "events.>",
    Durable: "events-printer",
    Handler: func(data []byte) error { fmt.Println(string(data)); return nil },
    // Pull: true → uses pull consumer
})
defer stop()
```

Auto-creates the stream and consumer. Returns `nil` on the handler → `Ack`. Returns error → `Nak` (immediate redelivery, no retry/DLQ).

### `Bus.Fetch` — `BatchMode` opt-in to bulk processing

```go
stop, err := bus.Fetch(natsModel.FetchConfig{
    Stream:    "EVENTS",
    Subject:   "events.>",
    Durable:   "events-bulk",
    BatchMode: true,
    BatchSize: 100,
    BatchHandler: func(msgs []natsModel.BatchMessage) error {
        // bulk DB insert. nil → ACK all; error → NAK all.
        return nil
    },
})
```

Default `BatchSize=10`, `Timeout=5s`. Setting both `Handler` and `BatchHandler` is rejected. `BatchHandler` requires `BatchMode=true`. Without `BatchMode`, the call falls through to `Subscribe(Pull=true)`.

### `Bus.StartConsumer` — managed, full lifecycle, retry + DLQ

This is the production path. `ConsumerOptions` has **two handler generations** — set exactly one:

| Generation | Handlers | Control model |
|---|---|---|
| Legacy V1 | `Handler` (per-message goroutines) / `BatchHandler` (bulk) | Return `nil` → ACK all; error → NAK all. **No retry/DLQ.** |
| New V2 (recommended) | `MessageHandlerV2(*Message)` / `BatchMessageHandlerV2([]*Message)` | Caller calls `msg.Ack()` / `msg.Nack(err)` / `msg.Reject(reason)` / `msg.Term()`. **Retry + DLQ supported.** |

The V2 `*Message` wrapper carries:

```go
Data    []byte
Headers map[string][]string
Subject string
DeliveryCount int
Timestamp     time.Time

// set by caller before Reject/Nack so the DLQ payload includes them
OrgId, PathKey, EventTrackerId string
```

#### V2 lifecycle methods

| Method | Behaviour |
|---|---|
| `Ack()` | Removes message from stream. |
| `Nack(err)` | If no `RetryPolicy`: immediate redelivery. With `RetryPolicy`: `NakWithDelay(GetDelayForAttempt(DeliveryCount))`; once `DeliveryCount > MaxRetries`, sends to DLQ and ACKs. |
| `Reject(reason)` | Sends straight to DLQ and ACKs (no retry — fatal/invalid data). |
| `Term()` | Discards silently — no retry, no DLQ. |

If `Nack`/`Reject` fire without a `DLQPolicy`, the message is `Term()`'d (logged at Warn).

### `ConsumerOptions` defaults (`setConsumerDefaults`)

| Field | Default |
|---|---|
| `BatchSize` | `50` |
| `FetchTimeout` | `1 * time.Second` |
| `RetryDelay` | `2 * time.Second` |
| `MaxRetries` | `5` (consumer-loop retries on **fetch** failure) |
| `StopOnError` | `false` |
| `DuplicateWindow` (in `createOrGetConsumer`) | `15 * time.Minute` |

`MaxAckPending` for the JetStream consumer is `BatchSize × 2` with a floor of `128` (double-buffer headroom: one batch processing + one batch prefetched).

### Stream / consumer auto-provisioning (`createOrGetConsumer`)

Idempotent:

- If the stream is missing → creates it: `WorkQueuePolicy`, `FileStorage`, `Duplicates = DuplicateWindow`.
- If the stream exists but does not capture the subject (exact match or `*.>` wildcard prefix) → updates the stream to add the subject.
- If the stream's `Duplicates` window differs from the desired one → updates it.
- If the consumer is missing → creates it: `AckExplicitPolicy`, `DeliverAllPolicy`, `FilterSubject`, `MaxAckPending=128` (or override). When `RetryPolicy` is provided, `MaxDeliver = MaxRetries + 1` and optionally `AckWait = RetryPolicy.AckWait`. `AckWait` on the server is a safety net (high value); per-attempt backoff is enforced **client-side** via `NakWithDelay`.
- If the consumer exists with a different `MaxAckPending` than requested → deletes and recreates.

### Double-buffer prefetch (`Consumer.start`)

The consumer keeps **one prefetch always in flight**: while the worker processes batch N, batch N+1 is being fetched. When the stream is empty, `cons.Fetch(MaxWait)` blocks server-side and acts as natural rate-limiting; when there is data, the next batch arrives with 0 ms idle.

Fetch loop notes worth knowing:

- `processBatch` prints **two blank lines** to stdout before each batch (visual separator). Strip if noisy.
- On `jetstream.ErrNoMessages` it loops without counting against `MaxRetries`.
- On other fetch errors it sleeps `RetryDelay`. After `MaxRetries` consecutive failures, behavior depends on `StopOnError`: stop or reset the retry counter.

### V1 vs V2 routing (`processBatch`)

Priority order (only one fires per batch):

1. `BatchMessageHandlerV2`
2. `MessageHandlerV2` (per-message goroutines)
3. `BatchHandler` (bulk, return-based)
4. `Handler` (per-message goroutines, return-based)

`validateConsumerOptions` rejects calls that set zero or more than one handler.

### `Consumer` lifecycle

| Method | Notes |
|---|---|
| `Stop()` | Idempotent via `stopped` flag (not atomic — concurrent calls share the same intent; first close wins on `stopChan`). |
| `IsRunning() bool` | `!stopped`. |
| `GetOptions() ConsumerOptions` | Snapshot. |

`ConsumerManager` is a small in-memory map keyed by name: `Add`, `Stop`, `StopAll`, `Get`.

## Retry policy (`retry.go`)

```go
func DefaultRetryPolicy() *RetryPolicy {
    return &RetryPolicy{
        MaxRetries: 5,
        Backoff:    []time.Duration{1*time.Second, 5*time.Second, 30*time.Second, 2*time.Minute, 10*time.Minute},
        AckWait:    5 * time.Minute,
    }
}
```

| Helper | Effect |
|---|---|
| `GetBackoffDurations()` | Falls back to defaults on nil/empty. |
| `GetDelayForAttempt(deliveryCount)` | `index = deliveryCount - 1`, clamped to last entry. |
| `GetMaxDeliver()` | `MaxRetries + 1` (NATS counts the original delivery). Default `6`. |
| `GetAckWait()` | Falls back to `30 * time.Second` if not set. |
| `ShouldRetry(deliveryCount)` | `deliveryCount <= MaxRetries`. Treats `nil` policy as `MaxRetries=5`. |

## DLQ (`dlq.go`)

```go
func DefaultDLQPolicy(serviceName string) *DLQPolicy {
    return &DLQPolicy{
        Stream:      "MAPEXOS-DLQ",
        Subject:     "dlq.mapexos",
        ServiceName: serviceName,
        ServiceType: "unknown",
        EventType:   "unknown",
    }
}
```

The DLQ payload is a `DLQMessage` JSON with:

- Identity: `ID` (UUID v4), `EventTrackerId`
- Tenant: `OrgId`, `PathKey` (mandatory for multi-tenant filtering)
- Service context: `ServiceName`, `ServiceType`, `EventType`
- Original: `OriginalSubject`, `OriginalStream`, `OriginalData` (raw JSON), `OriginalHeaders`
- Error: `LastError`, `ErrorCount`
- Delivery: `FirstDelivery`, `LastDelivery`, `TotalDeliveries`
- Consumer: `ConsumerName`
- Timestamps: `SentToDLQAt`

`Reject()` and `Nack()` (after max retries) publish this payload to `DLQPolicy.GetSubject()` and `Ack` the original message. If the DLQ publish itself fails, the original is `Nak()`ed (fallback).

## FANOUT (broadcast)

```go
err := bus.EnsureFanoutStream(natsModel.FanoutStreamConfig{
    Name:     "MAPEXOS-FANOUT",
    Subjects: []string{"mapexos.cache.>"},
    // Defaults: MaxAge=5m, MaxMsgs=10000, MaxBytes=10MB
})

sub, err := bus.SubscribeFanout("MAPEXOS-FANOUT", "assets-svc", "mapexos.cache.invalidate.assets",
    func(data []byte) error { handleInvalidate(data); return nil })
defer sub.Stop()

err = bus.PublishFanout(ctx, "mapexos.cache.invalidate.assets", payload)
```

Implementation notes:

- The fanout stream uses `MemoryStorage`, `LimitsPolicy`, `Replicas: 1`, `Discard: DiscardOld`.
- The ephemeral consumer name is `<serviceName>-fanout-<YYYYMMDD-HHMMSS>-<8 hex chars>`. The random suffix (via `utils/random.GenerateSessionID(4)`) prevents collisions when a service registers multiple fanout subscribers within the same second — without it, the second `CreateOrUpdateConsumer` overwrites the first and breaks its `FilterSubject`.
- `DeliverNewPolicy`, `AckWait=30s`, `InactiveThreshold=5min`.
- Handler errors are logged at Warn but the message is **always ACKed**.

## KV (CAS-aware bucket)

```go
store, err := client.CreateKeyValue(natsModel.KVConfig{
    Bucket:   "WORKFLOW-INSTANCES",
    Replicas: 3,
    History:  1,
    Storage:  jetstream.FileStorage, // default
})

rev, _ := store.Put("inst:123", payload)

entry, _ := store.Get("inst:123")
newRev, err := store.Update("inst:123", newPayload, entry.Revision)
if errors.Is(err, natsModel.ErrKVCASConflict) {
    // re-read and retry
}
```

| Method | Errors translated |
|---|---|
| `Get(key)` | `jetstream.ErrKeyNotFound` → `ErrKVKeyNotFound` |
| `Put(key, value)` | none specific (always create-or-overwrite) |
| `Create(key, value)` | `jetstream.ErrKeyExists` → `ErrKVKeyExists` |
| `Update(key, value, expectedRevision)` | `jetstream.ErrKeyExists` (used as CAS marker) → `ErrKVCASConflict` |
| `Delete(key)` | `jetstream.ErrKeyNotFound` → `ErrKVKeyNotFound` |
| `Purge(key)` | passes through |
| `Keys()` | `jetstream.ErrNoKeysFound` → `[]string{}` |
| `Bucket() string` | the underlying bucket name |

Defaults: `Replicas=1`, `History=1`, `Storage=FileStorage`.

## Stream housekeeping

| Method | Behaviour |
|---|---|
| `Bus.EnsureStream(jetstream.StreamConfig)` | `CreateOrUpdateStream` — idempotent. |
| `Bus.PurgeStreamSubject(streamName, subject)` | Returns `nil` if the stream does not exist. |
| `Bus.HasPendingMessages(streamName, subject)` | `false` if the stream does not exist. Inspects `info.State.Subjects` via subject-filter. |

## Test helpers

`message.go` exposes a unit-test entry point that does not need a real NATS connection:

```go
msg := natsModel.NewTestMessage([]byte("payload"), 0, &natsModel.TestMessageCallbacks{
    OnAck:    func() error { acked = true; return nil },
    OnNack:   func(err error) error { nacked = err; return nil },
    OnReject: func(reason string) error { rejected = reason; return nil },
})
yourHandler(msg)
```

Mocks for the public Bus surface live under `nats/mocks/`.

## Errors

| Sentinel | Source |
|---|---|
| `ErrMissingHandler` | (defined; not raised by current code) |
| `ErrMissingSubject` | `Push` / `PublishCore` with empty subject |
| `ErrMaxRetriesExceeded`, `ErrDLQPublishFailed`, `ErrMessageMetadataFailed`, `ErrDLQNotConfigured` | (defined; not raised by current code — raw errors are wrapped via `fmt.Errorf` instead) |
| `ErrKVKeyNotFound`, `ErrKVKeyExists`, `ErrKVCASConflict` | KV operations |

> The errors file is named `erros.go` (typo in the source — same package, no caller impact).

## Aliases (re-exports)

```go
type Msg         = nats.Msg
type Option      = nats.Option
type Options     = nats.Options
type StorageType = jetstream.StorageType
```

## End-to-end example (V2 + retry + DLQ)

```go
client, err := natsModel.New(natsModel.Config{Options: nats.GetDefaultOptions()})
if err != nil { return err }
defer client.Close()
bus := natsModel.NewBus(client)

// Idempotent stream provisioning
_ = bus.EnsureStream(jetstream.StreamConfig{
    Name:       "EVENTS-RAW",
    Subjects:   []string{"events.raw"},
    Retention:  jetstream.WorkQueuePolicy,
    Storage:    jetstream.FileStorage,
    Duplicates: 15 * time.Minute,
})

consumer, err := bus.StartConsumer(natsModel.ConsumerOptions{
    Stream:  "EVENTS-RAW",
    Subject: "events.raw",
    Durable: "events-processor",
    BatchSize: 100,
    RetryPolicy: natsModel.DefaultRetryPolicy(),
    DLQPolicy: &natsModel.DLQPolicy{
        ServiceName: "events-service",
        ServiceType: "processor",
        EventType:   "raw",
    },
    BatchMessageHandlerV2: func(messages []*natsModel.Message) {
        for _, msg := range messages {
            evt, err := parse(msg.Data)
            if err != nil {
                msg.Reject("invalid payload") // → DLQ now
                continue
            }
            msg.OrgId = evt.OrgId       // populate before Nack/Reject so DLQ has tenant context
            msg.PathKey = evt.PathKey
            msg.EventTrackerId = evt.TrackerID

            if err := process(evt); err != nil {
                msg.Nack(err) // backoff retry; DLQ after MaxRetries
                continue
            }
            msg.Ack()
        }
    },
})
if err != nil { return err }
defer consumer.Stop()
```
