# MongoDB Manager Package

`packages/infrastructure/mongodb/manager`

Singleton MongoDB connection manager with health monitoring, transaction support, and write-latency backpressure tracking.

---

## Quick Start

```go
import mongoManager "github.com/Mapex-Solutions/MapexOS/infrastructure/mongodb/manager"

mgr, err := mongoManager.New(mongoManager.Config{
    URI:           "mongodb://localhost:27017",
    Database:      "mydb",
    EnableMonitor: true,
})
if err != nil {
    log.Fatal(err)
}
defer mgr.Close(context.Background())
```

Register in DIG container (standard pattern):

```go
func InitMongo(c *dig.Container) {
    c.Provide(func() *mongoManager.MongoManager {
        mgr, err := mongoManager.New(mongoManager.Config{
            URI:           os.Getenv("MONGO_URI"),
            Database:      "production-mydb",
            EnableMonitor: true,
        })
        if err != nil {
            logger.Panic(err.Error())
        }
        return mgr
    })
}
```

---

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `URI` | `string` | **required** | MongoDB connection string |
| `Database` | `string` | **required** | Database name |
| `EnableMonitor` | `bool` | `false` | Start background goroutine that pings MongoDB every N seconds |
| `MonitorInterval` | `time.Duration` | `10` | Ping interval in seconds (only if `EnableMonitor=true`) |
| `EnableBackpressure` | `bool` | `false` | Enable write-latency tracking (opt-in) |
| `BackpressureWindow` | `int` | `1000` | Circular buffer capacity for latency samples |
| `ThrottledThresholdMs` | `int64` | `150` | P99 above this (ms) triggers Throttled mode |
| `BackoffThresholdMs` | `int64` | `500` | P99 above this (ms) triggers Backoff mode |

If `URI` or `Database` is empty, `New()` returns `ErrMissingURIOrDatabase`.

---

## Connection Monitoring

When `EnableMonitor: true`, a background goroutine pings MongoDB at the configured interval and updates:

- `IsConnected()` — `true` if last ping succeeded
- `LastLatency()` — round-trip latency of last ping in milliseconds

```go
if mgr.IsConnected() {
    fmt.Printf("MongoDB latency: %dms\n", mgr.LastLatency())
}
```

---

## Transactions

### Simple Transaction

`RunTransaction()` handles session lifecycle, retry on `TransientTransactionError`, and commit retry on `UnknownTransactionCommitResult`.

```go
result, err := mgr.RunTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
    // All operations using sessCtx participate in the transaction
    user, err := userRepo.CreateOne(sessCtx, userData)
    if err != nil {
        return nil, err // Aborts transaction
    }
    membership, err := membershipRepo.CreateOne(sessCtx, membershipData)
    if err != nil {
        return nil, err // Aborts, user creation rolled back
    }
    return user, nil // Commits transaction
})
```

### Manual Session

For advanced use cases:

```go
session, err := mgr.NewSession(ctx)
if err != nil {
    return err
}
defer session.EndSession(ctx)
// Use session manually...
```

### Without MongoManager

If you only have a `*mongo.Client` (e.g., from Model[T]):

```go
result, err := mongoManager.RunTransactionWithClient(ctx, client, func(sessCtx context.Context) (interface{}, error) {
    // transaction logic
    return nil, nil
})
```

---

## Backpressure

Backpressure tracking monitors MongoDB write latency and exposes a mode that callers can use to adapt their behavior. **The library never auto-throttles** — it only provides information.

### Enable

```go
mgr, _ := mongoManager.New(mongoManager.Config{
    URI:                "mongodb://localhost:27017",
    Database:           "mydb",
    EnableBackpressure: true, // Opt-in
    // Optional overrides:
    // BackpressureWindow:   1000,
    // ThrottledThresholdMs: 150,
    // BackoffThresholdMs:   500,
})
```

### Modes

| Mode | Trigger | Recommendation |
|------|---------|----------------|
| `Normal` | P99 < 150ms | Full batch, no changes |
| `Throttled` | P99 > 150ms for 3 consecutive windows (15s) | Reduce batch size |
| `Backoff` | P99 > 500ms for 3 consecutive windows (15s) | Reduce further + pause before write |

Recovery is **immediate**: as soon as P99 drops below the throttled threshold, mode resets to Normal.

### Usage in a Consumer

```go
func (s *MyService) ProcessBatch(messages []*nats.Message) {
    mode := s.mongoManager.GetBackpressureMode()

    if mode == mongoManager.Backoff {
        logger.Warn("MongoDB backoff, pausing 2s")
        time.Sleep(2 * time.Second)
    }

    start := time.Now()
    s.repo.BulkWrite(ctx, data)
    s.mongoManager.RecordWriteLatency(time.Since(start))
}
```

### Disable (Default)

When `EnableBackpressure` is `false` (the default):

- `GetBackpressureMode()` always returns `Normal`
- `WriteP99()` always returns `0`
- `RecordWriteLatency()` is a no-op

**Zero overhead. Zero code changes required in existing services.**

### Performance

| Operation | Cost |
|-----------|------|
| `RecordWriteLatency()` | ~50ns (atomic store) |
| `GetBackpressureMode()` | ~1ns (atomic load) |
| Background P99 computation | ~50us every 5s (off hot path) |

---

## Graceful Shutdown

```go
import "github.com/Mapex-Solutions/MapexOS/microservices/shutdown"

sm.RegisterFunc("mongodb", 5, func(ctx context.Context) error {
    return mgr.Close(ctx)
})
```

`Close()` stops the backpressure tracker (if running) and disconnects the client.

---

## API Reference

### Constructor

| Function | Description |
|----------|-------------|
| `New(cfg Config) (*MongoManager, error)` | Create manager, connect, start optional monitor + backpressure |

### Connection

| Method | Returns | Description |
|--------|---------|-------------|
| `GetClient()` | `*mongo.Client` | Raw MongoDB client (thread-safe) |
| `GetDatabase()` | `*mongo.Database` | Database instance |
| `GetDatabaseName()` | `string` | Database name |
| `IsConnected()` | `bool` | `true` if last health check passed |
| `LastLatency()` | `int64` | Last ping latency in ms |

### Transactions

| Method | Description |
|--------|-------------|
| `RunTransaction(ctx, txnFunc)` | Full transaction with session management + retry |
| `NewSession(ctx)` | Create raw session (caller manages lifecycle) |
| `RunTransactionWithClient(ctx, client, txnFunc)` | Standalone — no MongoManager needed |

### Backpressure

| Method | Description |
|--------|-------------|
| `GetBackpressureMode()` | Current mode: `Normal`, `Throttled`, or `Backoff` |
| `WriteP99()` | Last computed P99 write latency (ms) |
| `RecordWriteLatency(d)` | Record a write latency sample |

### Lifecycle

| Method | Description |
|--------|-------------|
| `Close(ctx)` | Stop backpressure tracker + disconnect client |

### Errors

| Error | When |
|-------|------|
| `ErrMissingURIOrDatabase` | `URI` or `Database` is empty in Config |
| `ErrNotConnected` | Operation attempted without active connection |
