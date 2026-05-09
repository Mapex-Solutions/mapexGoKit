# clickhouse — ClickHouse client, manager, and generic table layer

Three cooperating subpackages built on top of [`ClickHouse/clickhouse-go/v2`](https://github.com/ClickHouse/clickhouse-go):

| Path | Package | Role |
|---|---|---|
| `clickhouse/` (root) | `clickhouseModel` | Minimal `Client` wrapper — open + ping + raw `driver.Conn` access |
| `clickhouse/manager/` | `chManager` | Connection-lifecycle manager with health monitoring and reconnect |
| `clickhouse/model/` | `chModel` | Generic `Table[T]` + `QueryBuilder` + reflection-based column mapping |

Common across all three: native protocol port `9000`, LZ4 compression, `[INFRA:CLICKHOUSE]` log prefix.

## Root: `clickhouseModel`

Minimum viable wrapper. Use this when you need a `driver.Conn` and don't want the manager's lifecycle features.

### Config

```go
type Config struct {
    Host, Database, Username, Password string
    Port                                int
}
```

### Surface

```go
func New(cfg Config) (*Client, error)
func (c *Client) GetConn() driver.Conn
```

`New` opens the connection (LZ4 compression on) and pings with a **2 s** timeout. Errors are wrapped: `clickhouse connection failed: %w` or `clickhouse ping failed: %w`. Logs `[INFRA:CLICKHOUSE] Initialized successfully`.

## Manager: `chManager`

Long-lived manager that owns a connection, tracks health, and reconnects in the background.

### Config

```go
type Config struct {
    Host            string        // required
    Port            int           // default 9000
    Database        string        // required
    Username        string        // required
    Password        string        // (not validated by New, despite ErrMissingConfig text)
    MaxOpenConns    int           // default 10
    MaxIdleConns    int           // default 5
    EnableMonitor   bool
    MonitorInterval time.Duration // default 10s
}
```

### Validation

`New` returns `ErrMissingConfig` when `Host`, `Database` or `Username` is empty. **The message reads "host, database, username, and password are required"** but the code does not actually check `Password` — it is accepted empty.

### Connection (`internals.go`)

| Setting | Value |
|---|---|
| Compression | `LZ4` |
| `Settings: max_execution_time` | `60` |
| `DialTimeout` | `DefaultConnectTimeout` (5 s) |
| `MaxOpenConns` / `MaxIdleConns` | from cfg, defaults `10` / `5` |
| `ConnMaxLifetime` | `1 hour` |
| Ping timeout | `DefaultPingTimeout` (3 s) |

### Background monitor

`startMonitor()` ticks at `MonitorInterval`. On every tick it pings; on failure it logs and calls `connect()` to reconnect. On success it stores the latency in `m.lastLatency`. The monitor is **only started if `Config.EnableMonitor == true`**.

### Methods

| Method | Notes |
|---|---|
| `IsConnected() bool` | atomic, lock-free |
| `GetConn() driver.Conn` | nil until first successful connect — gate with `IsConnected()` |
| `GetDatabase() string` | the configured DB |
| `GetConfig() Config` | password is masked as `"***"` |
| `Health(ctx) HealthStatus` | live ping when `ctx != nil`; updates `Connected` + `ErrorMessage` |
| `LastLatency() int64` | last ping latency in ms |
| `Close() error` | sets `isConnected=false` and closes the connection |

### `HealthStatus`

```go
type HealthStatus struct {
    Connected    bool      `json:"connected"`
    Database     string    `json:"database"`
    Host         string    `json:"host"`
    Port         int       `json:"port"`
    LastCheckAt  time.Time `json:"lastCheckAt"`
    ErrorMessage string    `json:"errorMessage,omitempty"`
}
```

### Constants

| Constant | Value |
|---|---|
| `DefaultMonitorInterval` | `10 * time.Second` |
| `DefaultPort` | `9000` |
| `DefaultConnectTimeout` | `5 * time.Second` |
| `DefaultPingTimeout` | `3 * time.Second` |

### Errors

| Sentinel | Message |
|---|---|
| `ErrMissingConfig` | `host, database, username, and password are required` |
| `ErrNotConnected` | `clickhouse is not connected` |
| `ErrConnectionFailed` | `failed to connect to clickhouse` |
| `ErrPingFailed` | `clickhouse ping failed` |

## Model: `chModel` — Generic table layer

A `Table[T any]` provides typed Insert/Query/Pagination on a single ClickHouse table. Field metadata is derived once via reflection and cached.

### Construction

```go
type Event struct {
    Timestamp time.Time              `ch:"timestamp"`
    OrgId     string                 `ch:"org_id"`
    Payload   map[string]interface{} `ch:"payload"`   // marshaled to JSON
}

table, err := chModel.NewTable[Event](conn, "events", chModel.TableConfig{
    TimestampField: "timestamp", // default "timestamp"
    DefaultOrder:   "DESC",      // default "DESC"
    DefaultTimeout: 30*time.Second, // default 30s
})
```

`NewTable` returns `ErrInvalidType` when `T` is not a struct, `ErrNoFields` when no exported fields carry a `ch` (or fallback `json`) tag.

### Field metadata (`reflection.go`)

For each exported struct field, the column name comes from the `ch` tag, falling back to `json` (first segment before `,`). Tag value `"-"` or empty skips the field.

`fieldInfo` flags computed once per struct:

| Flag | Meaning |
|---|---|
| `IsJSON` | `true` for `map` or `slice` types — JSON-marshaled on insert, JSON-unmarshaled on scan. **Exceptions:** `[]byte` (passes raw) and `map[<numericKey>]V` (native ClickHouse `Map(K, V)`, not JSON). |
| `IsPointer` | `true` when field is a pointer; nil pointers map to SQL `nil` on insert. |
| `IsTime` | `true` for `time.Time` or `*time.Time` — used by `FindByCursor`. |

### Operations on `*Table[T]`

| Method | Behaviour |
|---|---|
| `Insert(ctx, *T) error` | `Exec` `INSERT INTO ... VALUES (?, ?, ...)`. Errors wrap `ErrQueryFailed`. |
| `InsertBatch(ctx, []*T) error` | `PrepareBatch` + `batch.Append` per item + `batch.Send`. Empty slice → `ErrEmptyItems`. Errors wrap `ErrBatchFailed`. Logs `[INFRA:CLICKHOUSE] Batch inserted: %d records into %s`. |
| `Count(ctx, Map) (uint64, error)` | `SELECT COUNT(*) FROM ... WHERE field = ?`. |
| `FindByOffset(ctx, Map, *PaginationOpts, sort) (*PaginatedResult[T], error)` | Page/PerPage pagination with `Count` round-trip. |
| `FindWithFilters(ctx, []Filter, *PaginationOpts, sort) (*PaginatedResult[T], error)` | Like `FindByOffset` but accepts richer `Filter` operators. |
| `FindByCursor(ctx, []Filter, *TimeCursorOpts) (*TimeCursorResult[T], error)` | Time-based cursor pagination — no `COUNT(*)`. **Logs the generated SQL via `logger.Info` on every call.** Returns `HasNext`/`HasPrevious`, `NextCursor`/`PrevCursor`. |
| `TableName() string`, `Config() TableConfig`, `Columns() []string`, `Conn() driver.Conn` | Introspection / escape hatches. |
| `Query() *QueryBuilder` | Manual query construction. |

### Pagination defaults

| Constant | Value |
|---|---|
| `DefaultPage` | `1` |
| `DefaultPerPage` | `25` |
| `MaxPerPage` | `300` (silently clamps `pagination.PerPage > 300` to default) |
| `MaxOffset` | `10000` (silently clamps offsets greater than this) |

### Sort string parsing

`ParseSort("timestamp:desc", "timestamp", "DESC")` → `"timestamp DESC"`. Direction is uppercased; unrecognised direction falls back to `defaultOrder`. Empty `sort` falls back to `<defaultField> <defaultOrder>`.

### `QueryBuilder` (`query_builder.go`)

Fluent SQL builder. `BuildSelect` and `BuildCount` return `(query, args)` ready for `conn.Query`.

| Method | SQL |
|---|---|
| `Select(cols...)` | `SELECT col1, col2, ...` (default `SELECT *`) |
| `Where(field, op, value)` | `field <op> ?` (or `field IN (?)` / `field BETWEEN ? AND ?`) |
| `WhereFilter(Filter)` / `WhereFilters([]Filter)` | Same with `Filter` struct (handles `OpBetween` two-value) |
| `WhereMap(Map)` | Equality-only convenience — every entry becomes `field = ?` |
| `WhereLike(field, pattern)` | `field LIKE ?` |
| `WhereRaw(clause, args...)` | Untemplated escape hatch |
| `OrderBy(...)`, `Limit(n)`, `Offset(n)` | Final clauses |
| `BuildSelect()` / `BuildCount()` | Render |

Package-level builders: `BuildInsert(table, cols)`, `BuildInsertBatch(table, cols)` (no `VALUES`, for `PrepareBatch`).

### Filter operators

```go
OpEqual        FilterOperator = "="
OpNotEqual     FilterOperator = "!="
OpGreater      FilterOperator = ">"
OpGreaterEqual FilterOperator = ">="
OpLess         FilterOperator = "<"
OpLessEqual    FilterOperator = "<="
OpLike         FilterOperator = "LIKE"
OpIn           FilterOperator = "IN"
OpNotIn        FilterOperator = "NOT IN"
OpBetween      FilterOperator = "BETWEEN"
```

### `Map` shortcut operators (in `buildWhereFromMap`)

A helper exists that recognises MongoDB-style operator maps inside `Map` filters. It is defined in `methods.go` but is **not currently invoked** by the public `Find*` methods — they use `WhereMap` (equality-only) for `Map` filters and require `[]Filter` for the other operators.

```go
// Recognised tokens (when this helper is wired)
$regex  → field LIKE ?     // strips leading "^", appends "%"
$gt     → field > ?
$gte    → field >= ?
$lt     → field < ?
$lte    → field <= ?
$ne     → field != ?
$in     → field IN (?)
```

### `TimeCursorOpts` / `TimeCursorResult`

```go
type TimeCursorOpts struct {
    Cursor    interface{} // RFC3339 string or time.Time; nil = first page
    Direction string      // "next" (default) or "prev"
    Limit     int64       // capped by MaxPerPage; default 20
    SortAsc   bool        // false = DESC (default)
}

type TimeCursorResult[T any] struct {
    Items       []T       `json:"items"`
    NextCursor  time.Time `json:"nextCursor,omitempty"`
    PrevCursor  time.Time `json:"prevCursor,omitempty"`
    HasNext     bool      `json:"hasNext"`
    HasPrevious bool      `json:"hasPrevious"`
}
```

The cursor field is taken from `TableConfig.TimestampField` (default `"timestamp"`). The implementation fetches `limit + 1` rows to detect the "more" boundary.

### Errors

| Sentinel | Triggered when |
|---|---|
| `ErrEmptyItems` | `InsertBatch` called with empty slice |
| `ErrInvalidType` | `T` is not a struct in `NewTable[T]` |
| `ErrNoFields` | No `ch`/`json`-tagged exported fields |
| `ErrNotFound` | Defined; not raised by current code |
| `ErrInvalidFilter` | Defined; not raised by current code |
| `ErrQueryFailed` | Wraps any `Query`/`QueryRow`/`Exec` failure |
| `ErrScanFailed` | Wraps `rows.Scan` failure |
| `ErrBatchFailed` | Wraps `PrepareBatch` / `Append` / `Send` failure |
| `ErrMarshalFailed` | Wraps `json.Marshal` failure during insert |

JSON unmarshal errors during `scanRows` are **logged as `Warn` and the field is left zero-valued** — they do not abort the query.

## End-to-end example

```go
// Manager (long-lived, with monitor)
mgr, err := chManager.New(chManager.Config{
    Host: "ch.local", Database: "mapex", Username: "default", Password: secret,
    EnableMonitor: true, MonitorInterval: 10*time.Second,
})
if err != nil { return err }
defer mgr.Close()

// Generic table on top of the manager's connection
type Event struct {
    Timestamp time.Time `ch:"timestamp"`
    OrgId     string    `ch:"org_id"`
    Payload   chModel.Map `ch:"payload"`  // JSON-marshaled
}
events, _ := chModel.NewTable[Event](mgr.GetConn(), "events", chModel.TableConfig{})

// Batch insert
_ = events.InsertBatch(ctx, batch)

// Cursor pagination, last 50 events of the org
res, err := events.FindByCursor(ctx,
    []chModel.Filter{{Field: "org_id", Operator: chModel.OpEqual, Value: "org-123"}},
    &chModel.TimeCursorOpts{Limit: 50, Direction: "next"})
if err != nil { return err }
for _, ev := range res.Items { _ = ev }
if res.HasNext {
    // pass res.NextCursor as TimeCursorOpts.Cursor on the next call
}
```
