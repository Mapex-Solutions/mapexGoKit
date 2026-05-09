# mongodb — Connection manager and generic model layer

Two cooperating subpackages built on top of [`go.mongodb.org/mongo-driver/v2`](https://github.com/mongodb/mongo-go-driver):

| Path | Package | Role |
|---|---|---|
| `mongodb/manager/` | `mongoManager` | Connection lifecycle, health monitoring, transactions, write-pressure backpressure |
| `mongodb/model/` | `mongoModel` | Generic `Model[T]` over a single collection — CRUD, pagination, transaction delegation, BSON helpers |

The `model` package depends on `manager` for transactions (`mongoModel.Model.RunTransaction` delegates to `manager.RunTransactionWithClient`).

> Long-form, more detailed docs for each subpackage live in `manager/docs/README.md` and `model/docs/README.md`.

## Manager: `mongoManager`

### Config

```go
type Config struct {
    URI             string        // required
    Database        string        // required
    EnableMonitor   bool
    MonitorInterval time.Duration // see warning below
    UseBsonD        bool

    // Backpressure (opt-in)
    EnableBackpressure   bool
    BackpressureWindow   int   // default: 1000
    ThrottledThresholdMs int64 // default: 150
    BackoffThresholdMs   int64 // default: 500
}
```

> **`MonitorInterval` quirk.** The startMonitor goroutine uses `time.NewTicker(m.cfg.MonitorInterval * time.Second)`. The field is typed as `time.Duration` but the code treats the value as a **count of seconds** (the constant `DefaultMonitorInterval = 10` is also an untyped int). Passing `10 * time.Second` therefore yields a ticker interval of 10²⁰ ns — effectively never-fires. Pass an int-as-Duration (e.g. `MonitorInterval: 10`) or fix the multiplication site.

### Construction

```go
func New(cfg Config) (*MongoManager, error)
```

1. Returns `ErrMissingURIOrDatabase` if `URI` or `Database` is empty.
2. Calls `connect()`: applies `BSONOptions{ DefaultDocumentMap: true }` unless `UseBsonD` is set, opens the client, pings with a **3 s** timeout. Returns the underlying error on failure.
3. Spawns `startMonitor()` if `EnableMonitor`.
4. Spawns the backpressure tracker if `EnableBackpressure`.
5. Logs `[INFRA:MONGODB] Initialized`.

`Close(ctx)` cancels the backpressure tracker (if any) and disconnects the client.

### `UseBsonD` toggle

When `false` (default), the driver decodes nested documents into `map[string]any` via `BSONOptions.DefaultDocumentMap=true`. Standard Go assertions like `val.(map[string]interface{})` work.

When `true`, the driver returns ordered `bson.D` for the same paths. Only opt-in if you need field ordering.

### Methods

| Method | Notes |
|---|---|
| `IsConnected() bool` | atomic, lock-free |
| `GetClient() *mongo.Client` / `GetDatabase() *mongo.Database` / `GetDatabaseName() string` | |
| `LastLatency() int64` | last ping latency in ms (updated by monitor) |
| `Close(ctx) error` | stop tracker + Disconnect |
| `RunTransaction(ctx, txnFunc) (any, error)` | full transaction lifecycle with retry |
| `NewSession(ctx) (*mongo.Session, error)` | caller owns `EndSession` |
| `RecordWriteLatency(d time.Duration)` | record sample for backpressure (no-op if disabled) |
| `GetBackpressureMode() BackpressureMode` | `Normal` if disabled |
| `WriteP99() int64` | last computed P99 in ms (`0` if disabled / no samples) |

Standalone helper:

```go
func RunTransactionWithClient(ctx, *mongo.Client, TransactionFunc) (any, error)
```

Used by `mongoModel.Model.RunTransaction` to keep transaction logic centralised even when callers only have a `*mongo.Client`.

### Transactions

Both entry points share `runTransactionWithRetryInternal`:

1. `session.StartTransaction()`
2. Run `txnFunc(sessCtx)` where `sessCtx = mongo.NewSessionContext(ctx, session)`
3. Abort on error. If the error has the `TransientTransactionError` label → retry the whole transaction.
4. Commit. If commit fails with `UnknownTransactionCommitResult` → retry the commit. If it fails with `TransientTransactionError` → retry the whole transaction.

`hasErrorLabel` checks both `mongo.CommandError` and `mongo.WriteException` via `errors.As`.

### Backpressure

Three modes (`BackpressureMode int32`):

| Mode | Meaning | Caller behaviour suggestion |
|---|---|---|
| `Normal` (0) | P99 below `ThrottledThresholdMs` | Default behaviour |
| `Throttled` (1) | P99 ≥ `ThrottledThresholdMs` for 3 windows | Reduce batch size |
| `Backoff` (2) | P99 ≥ `BackoffThresholdMs` for 3 windows | Reduce further + add a pause |

Internals (`backpressure.go`):

- Lock-free circular buffer (`samples []int64`), atomic write index.
- P99 is recomputed every **5 s** (`computeInterval`) by the background goroutine.
- Transition rules: 3 consecutive windows above threshold to step UP (`windowsToTransition = 3`); P99 below `ThrottledThresholdMs` resets immediately to `Normal`.
- Each non-Normal tick logs `[INFRA:MONGODB] Backpressure mode=… P99=…ms` at Warn.

`BackpressureMode.String()` returns `Normal`/`Throttled`/`Backoff`/`Unknown`.

### Constants and errors

```go
const DefaultMonitorInterval = 10 // (see quirk above)
const defaultBackpressureWindow   = 1000
const defaultThrottledThresholdMs = 150
const defaultBackoffThresholdMs   = 500
const computeInterval             = 5 * time.Second
const windowsToTransition         = 3
```

```go
var ErrMissingURIOrDatabase = errors.New("URI and Database are required")
var ErrNotConnected         = errors.New("MongoDB client is not connected")
```

## Model: `mongoModel` — Generic collection wrapper

A `Model[T any]` provides typed CRUD and pagination over a single collection. The struct field `T` is assumed to follow standard `bson` tags.

### Construction

```go
func New[T any](db *mongo.Database, collection string, cfg Config) *Model[T]
```

Behaviour:

- Lists collections; if `collection` is missing, calls `CreateCollection`. Listing/creation errors are **logged, not returned**.
- Calls `ensureIndexes(...)` — idempotent (skips existing index names). Index creation errors are **logged at Warn, not returned**.
- Returns the `*Model[T]` even on partial failure.

`New` does **not** return an error — diagnose via logs.

### `Config`

```go
type Config struct {
    DefaultTimeout time.Duration       // applied when ctx has no deadline
    Indexes        []IndexDefinition
}

type IndexDefinition struct {
    Name                    string             // required
    Keys                    map[string]int     // 1 = ASC, -1 = DESC
    Unique                  bool
    Sparse                  bool
    PartialFilterExpression bson.M
    ExpireAfterSeconds      *int32             // TTL — set to *int32(0) when the field already holds the absolute expiry
}
```

> Note: `Keys` is a `map[string]int`. Compound-index field order is **not deterministic** under Go map iteration. For multi-key indexes where order matters, prefer to keep `Name` stable and accept that index `Keys` are emitted in iteration order; if you depend on key order, build the index manually via `col.Indexes()`.

### Auto-populated fields (reflection)

`CreateOne` / `CreateMany` walk struct fields and, if zero-valued, set:

| Field detected by | Assigned |
|---|---|
| BSON tag prefix `_id` or field name `id` (lowercased), of type `bson.ObjectID` | `bson.NewObjectID()` |
| BSON tag prefix `created` or field name `created`/`createdat`, of type `time.Time` | `time.Now().UTC()` |

### CRUD

All methods accept variadic `*CommonOpts` — only the **first** is honoured. Context normalisation: if `ctx` has no deadline and `Config.DefaultTimeout > 0`, a `WithTimeout` is applied; if `CommonOpts.Session` is set, the session is bound via `mongo.NewSessionContext`.

| Method | Behaviour |
|---|---|
| `DIRECT() *mongo.Collection` | Escape hatch to the raw collection. |
| `CreateOne(ctx, *T, opts...) (*T, error)` | InsertOne after auto-populating `_id`/`created`. |
| `CreateMany(ctx, []T, opts...) ([]T, error)` | InsertMany; empty slice → `ErrEmptyItems`. |
| `FindByID(ctx, id, opts...) (*T, error)` | `id` may be string or `bson.ObjectID`. Bad input → `ErrInvalidID`. Miss → `ErrNotFound`. |
| `FindOne(ctx, *Map, opts...) (*T, error)` | Miss → `ErrNotFound`. |
| `FindByOffset(ctx, Map, *PaginationOpts, opts...) (*PaginatedResult[T], error)` | Skip/limit pagination. **Always prints `FindByOffset called with filter: …` to stdout** — diagnose / strip if noisy. Page > `MaxOffsetSkip/PerPage` is silently coerced to `skip = 0` (page 1). Empty filter is allowed. |
| `FindByCursor(ctx, Map, *PaginationOpts, opts...) (*PaginatedResult[T], error)` | Legacy `_id`-cursor pagination. Requires `pagination.UseCursor=true` and `SortDirection ∈ {1, -1}`. |
| `FindWithCursor(ctx, Map, *CursorOpts, projection) (*CursorResult[T], error)` | New `_id`-cursor pagination with bi-directional `Direction = CursorNext/CursorPrevious`, `SortAsc`, default `Limit = 300`. |
| `FindAndUpdateMany(ctx, Map, Map, opts...) (*UpdateResult, error)` | UpdateMany. Honours upsert/comment/let/etc. via opts. |
| `FindByIDAndUpdate(ctx, id, Map, opts...) (T, error)` | FindOneAndUpdate by ObjectID. |
| `FindOneAndUpdate(ctx, *Map, *Map, opts...) (*T, error)` | FindOneAndUpdate by filter. |
| `DeleteByID(ctx, id, opts...) error` | Empty result → `ErrNotFound`. |
| `DeleteOne(ctx, *Map, opts...) error` | Empty result → `ErrNotFound`. |
| `DeleteMany(ctx, Map, opts...) (int64, error)` | Empty filter → `ErrEmptyFilters`. Empty result → `ErrNotFound`. |

### `CommonOpts`

```go
type CommonOpts struct {
    // Shared
    Session    *mongo.Session
    Projection interface{}
    Sort       interface{}
    Hint       interface{}
    Collation  *options.Collation

    // Update-specific
    Upsert                   *bool
    ReturnDocument           *options.ReturnDocument
    BypassDocumentValidation *bool
    Comment                  interface{}
    MaxTime                  *time.Duration
    Let                      interface{}
    ArrayFilters             []interface{}
}
```

`applyCommonOptions` applies the matching subset to the four supported builder types: `FindOneOptionsBuilder`, `FindOptionsBuilder`, `FindOneAndUpdateOptionsBuilder`, `UpdateManyOptionsBuilder`. Unsupported builders pass through untouched.

### Pagination types

```go
type PaginationOpts struct {
    // Offset-based
    Page    int64
    PerPage int64

    // Cursor-based (legacy)
    CursorID      any  // ObjectID or string
    SortDirection int  // 1 = forward, -1 = backward
    UseCursor     bool
}

type Pagination struct {
    Page, PerPage, TotalItems, TotalPages int64
    HasNext, HasPrev *bool
}

type PaginatedResult[T any] struct { Items []T; Pagination Pagination }

// FindWithCursor / CursorOpts / CursorResult — newer surface
type CursorDirection string
const ( CursorNext CursorDirection = "next"; CursorPrevious = "previous" )

type CursorOpts struct { Cursor string; Direction CursorDirection; Limit int64; SortAsc bool }
type CursorResult[T any] struct { Items []T; NextCursor, PrevCursor string; HasNext, HasPrevious bool }
```

### Pagination defaults

```go
const DefaultPage      int64 = 1
const DefaultPerPage   int64 = 25
const MaxOffsetPerPage int64 = 300
const MaxOffsetSkip    int64 = 500   // skip beyond this silently becomes 0
```

### Aliases (re-exports)

```go
type Map               = bson.M
type ObjectId          = bson.ObjectID
type Collection        = mongo.Collection
type ReturnDoc         = options.ReturnDocument
type BulkWriteOptions  = options.BulkWriteOptionsBuilder
type WriteModel        = mongo.WriteModel
type UpdateResult      = mongo.UpdateResult
type DeleteResult      = mongo.DeleteResult
type BulkWriteResult   = mongo.BulkWriteResult

const ReturnDocOld = options.Before
const ReturnDocNew = options.After
```

### Utilities

| Function | Effect |
|---|---|
| `NewObjectID() bson.ObjectID` | New unique ObjectID. |
| `StringToProjection("a, b, c") Map` | `{a:1, b:1, c:1}` (skips empty parts). |
| `ToObjectID(any) (bson.ObjectID, error)` | Accepts `bson.ObjectID` or hex string. Bad input → `ErrInvalidID`. |
| `NewInsertOneModel`/`NewUpdateOneModel`/`NewReplaceOneModel` | Bulk-write model factories (re-exports). |
| `BulkWrite()`/`FindOptions()` | Options builder factories. |
| `IsDuplicateKeyError(err) bool` | Re-export of `mongo.IsDuplicateKeyError`. |

### Map accessors (`mapget.go`)

Type-safe accessors for `map[string]interface{}` / `bson.M` (and transparently `bson.D`):

| Function | Returns | Zero on miss/wrong type |
|---|---|---|
| `MapGetString(m, key) string` | string | `""` |
| `MapGetInt(m, key) int` | int (handles int/int32/int64/float64) | `0` |
| `MapGetBool(m, key) bool` | bool | `false` |
| `MapGetMap(m, key) map[string]any` | nested map (converts `bson.M` and `bson.D`) | `nil` |
| `MapGetSlice(m, key) []any` | slice (converts `bson.A`) | `nil` |
| `MapGetStringSlice(m, key) []string` | filtered string slice | `nil` |
| `ToMap(val any) map[string]any` | converts `bson.M`/`bson.D` | `nil` |

### Transactions (model side)

```go
func (m *Model[T]) NewSession(ctx) (*mongo.Session, error)
func (m *Model[T]) RunTransaction(ctx, TransactionFunc) (any, error)
func (m *Model[T]) RunTransactionWithRetry(ctx, *mongo.Session, TransactionFunc) (any, error)
func (m *Model[T]) CommitWithRetry(ctx, *mongo.Session) error  // deprecated
```

`TransactionFunc` is aliased from the manager package. `RunTransaction` extracts `*mongo.Client` via `m.col.Database().Client()` and calls `manager.RunTransactionWithClient` so that retry behaviour stays in one place. `CommitWithRetry` is a thin pass-through retained for backwards compatibility.

### Errors

| Sentinel | Triggered |
|---|---|
| `ErrNotFound` | No documents matched (Find / FindByID / Update / Delete) |
| `ErrInvalidID` | `ToObjectID` could not parse |
| `ErrEmptyItems` | `CreateMany` with empty slice |
| `ErrEmptyFilters` | `DeleteMany` with empty/nil map |
| `ErrCursorPaginationRequired` | `FindByCursor` without `UseCursor=true` |
| `ErrInvalidCursorDirection` | `FindByCursor` with `SortDirection ∉ {1, -1}` |
| `ErrNotConnected` | `Model.NewSession` when client is nil |

> The errors file is named `erros.go` (typo in the source — same package, no caller impact).

## End-to-end example

```go
mgr, err := mongoManager.New(mongoManager.Config{
    URI:                "mongodb://localhost:27017",
    Database:           "mapex",
    EnableMonitor:      true,
    MonitorInterval:    10,                   // ← seconds, not time.Second (see quirk)
    EnableBackpressure: true,
})
if err != nil { return err }
defer mgr.Close(ctx)

type User struct {
    ID      bson.ObjectID `bson:"_id,omitempty"`
    Email   string        `bson:"email"`
    Created time.Time     `bson:"created,omitempty"`
}

users := mongoModel.New[User](mgr.GetDatabase(), "users", mongoModel.Config{
    DefaultTimeout: 5 * time.Second,
    Indexes: []mongoModel.IndexDefinition{
        {Name: "idx_email_unique", Keys: map[string]int{"email": 1}, Unique: true},
    },
})

// CreateOne auto-fills _id and Created.
u, err := users.CreateOne(ctx, &User{Email: "alice@example.com"})

// Cursor pagination
res, err := users.FindWithCursor(ctx,
    mongoModel.Map{},
    &mongoModel.CursorOpts{Direction: mongoModel.CursorNext, Limit: 50, SortAsc: true},
    nil,
)

// Transaction
result, err := users.RunTransaction(ctx, func(sessCtx context.Context) (any, error) {
    if _, err := users.CreateOne(sessCtx, &User{Email: "bob@example.com"}); err != nil {
        return nil, err // aborts
    }
    return "ok", nil
})

// Backpressure-aware batch loop
start := time.Now()
_, err = users.DIRECT().BulkWrite(ctx, models)
mgr.RecordWriteLatency(time.Since(start))
switch mgr.GetBackpressureMode() {
case mongoManager.Throttled: // halve next batch
case mongoManager.Backoff:   // halve + sleep
}
```
