# ClickHouse Infrastructure Package

This package provides an abstraction layer for interacting with ClickHouse, similar to the `mongoModel` package for MongoDB.

## Structure

```
clickhouse/
├── manager/     # Connection management with health monitoring
│   ├── manager.go
│   └── types.go
├── model/       # Generic ORM with reflection and query builder
│   ├── model.go
│   ├── methods.go
│   ├── query_builder.go
│   ├── reflection.go
│   ├── types.go
│   └── utils.go
└── README.md
```

## Manager

The `manager` is responsible for managing the ClickHouse connection, including:
- Connection with automatic retry
- Background health monitoring
- Connection status via atomic bool

### Usage

```go
import chManager "github.com/Mapex-Solutions/MapexOS/infrastructure/clickhouse/manager"

// Create instance with configuration
ch, err := chManager.New(chManager.Config{
    Host:            "localhost",
    Port:            9000,
    Database:        "mapexos",
    Username:        "default",
    Password:        "password",
    EnableMonitor:   true,
    MonitorInterval: 30 * time.Second,
})
if err != nil {
    log.Fatal(err)
}

// Check connection status
if ch.IsConnected() {
    fmt.Println("Connected to ClickHouse")
}

// Get raw connection for direct use
conn := ch.GetConn()

// Get health status (for /health endpoints)
status := ch.GetHealthStatus()
```

## Model (chModel)

The `model` provides a generic reflection-based ORM for CRUD operations with ClickHouse.

### Defining Entities

Use the `ch:` tag to map struct fields to ClickHouse columns:

```go
type RawEvent struct {
    // Timestamp should be the first field (used for partitioning)
    Timestamp     time.Time              `ch:"timestamp" json:"timestamp"`
    ThreadId      string                 `ch:"thread_id" json:"threadId"`
    OrgId         string                 `ch:"org_id" json:"orgId"`
    PathKey       string                 `ch:"path_key" json:"pathKey"`
    Source        string                 `ch:"source" json:"source"`

    // Map/slice fields are automatically serialized as JSON
    Payload       map[string]interface{} `ch:"payload" json:"payload"`
    Metadata      map[string]interface{} `ch:"metadata" json:"metadata"`

    RetentionDays uint16                 `ch:"retention_days" json:"retentionDays"`
}
```

### Creating a Table

```go
import chModel "github.com/Mapex-Solutions/MapexOS/infrastructure/clickhouse/model"

// Create table instance
table, err := chModel.NewTable[RawEvent](conn, "events_raw", chModel.TableConfig{
    TimestampField: "timestamp",
    DefaultOrder:   "DESC",
})
if err != nil {
    log.Fatal(err)
}
```

### Insert Operations

```go
// Single insert
event := &RawEvent{
    Timestamp: time.Now(),
    ThreadId:  "ds-123",
    OrgId:     "org-456",
    PathKey:   "org-456/site-1/device-1",
    Source:    "http_gateway",
    Payload:   map[string]interface{}{"temperature": 25.5},
    Metadata:  map[string]interface{}{"ip": "192.168.1.1"},
}
err := table.Insert(ctx, event)

// Batch insert (optimized for large volumes)
events := []*RawEvent{event1, event2, event3}
err := table.InsertBatch(ctx, events)
```

### Query Operations

#### FindByOffset (Simple Pagination)

```go
// Simple filters (equality)
filters := chModel.Map{
    "org_id": "org-456",
    "source": "http_gateway",
}

pagination := &chModel.PaginationOpts{
    Page:    1,
    PerPage: 50,
}

result, err := table.FindByOffset(ctx, filters, pagination, "timestamp:desc")
// result.Items      → []RawEvent
// result.Pagination → {Page, PerPage, TotalItems, TotalPages}
```

#### FindWithFilters (Advanced Filters)

```go
filters := []chModel.Filter{
    // Equality
    {Field: "org_id", Operator: chModel.OpEqual, Value: "org-456"},

    // LIKE (for pathKey with wildcard)
    {Field: "path_key", Operator: chModel.OpLike, Value: "org-456/site-1%"},

    // Date range with BETWEEN
    {
        Field:    "timestamp",
        Operator: chModel.OpBetween,
        Value:    startTime,    // time.Time
        EndValue: endTime,      // time.Time
    },

    // Greater than or equal
    {Field: "retention_days", Operator: chModel.OpGreaterEqual, Value: 7},

    // IN (list of values)
    {Field: "source", Operator: chModel.OpIn, Value: []string{"http_gateway", "mqtt_gateway"}},
}

result, err := table.FindWithFilters(ctx, filters, pagination, "timestamp:desc")
```

### Available Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `OpEqual` | Equality | `field = value` |
| `OpNotEqual` | Not equal | `field != value` |
| `OpGreater` | Greater than | `field > value` |
| `OpGreaterEqual` | Greater than or equal | `field >= value` |
| `OpLess` | Less than | `field < value` |
| `OpLessEqual` | Less than or equal | `field <= value` |
| `OpLike` | Pattern matching | `field LIKE 'value%'` |
| `OpIn` | List of values | `field IN (v1, v2)` |
| `OpBetween` | Range | `field BETWEEN v1 AND v2` |

### Query Builder (Advanced Usage)

For more complex queries, use the QueryBuilder directly:

```go
qb := table.Query().
    Select("timestamp", "org_id", "payload").
    Where("org_id", chModel.OpEqual, "org-456").
    Where("timestamp", chModel.OpGreaterEqual, startTime).
    OrderBy("timestamp DESC").
    Limit(100).
    Offset(0)

query, args := qb.BuildSelect()
// query: "SELECT timestamp, org_id, payload FROM events_raw WHERE org_id = ? AND timestamp >= ? ORDER BY timestamp DESC LIMIT 100 OFFSET 0"
// args: ["org-456", startTime]

// Execute manually
rows, err := conn.Query(ctx, query, args...)
```

### Count

```go
count, err := table.Count(ctx, chModel.Map{"org_id": "org-456"})
```

## Complete Example: Repository

```go
package clickhouseRepo

import (
    "context"
    "time"

    "myservice/domain/entities"
    "myservice/domain/repositories"

    "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
    chModel "github.com/Mapex-Solutions/MapexOS/infrastructure/clickhouse/model"
)

type EventRepositoryClickHouse struct {
    conn          driver.Conn
    rawEventTable *chModel.Table[entities.RawEvent]
}

func NewEventRepository(conn driver.Conn) repositories.EventRepository {
    rawTable, err := chModel.NewTable[entities.RawEvent](conn, "events_raw", chModel.TableConfig{
        TimestampField: "timestamp",
        DefaultOrder:   "DESC",
    })
    if err != nil {
        // Handle error
    }

    return &EventRepositoryClickHouse{
        conn:          conn,
        rawEventTable: rawTable,
    }
}

func (r *EventRepositoryClickHouse) SaveRawEventBatch(ctx context.Context, events []*entities.RawEvent) error {
    return r.rawEventTable.InsertBatch(ctx, events)
}

func (r *EventRepositoryClickHouse) QueryEventsRaw(
    ctx context.Context,
    orgFilter chModel.Map,
    startTime *time.Time,
    endTime *time.Time,
    pagination *chModel.PaginationOpts,
    sort string,
) (*chModel.PaginatedResult[entities.RawEvent], error) {

    filters := []chModel.Filter{}

    // Org filter
    if orgId, ok := orgFilter["orgId"]; ok {
        filters = append(filters, chModel.Filter{
            Field:    "org_id",
            Operator: chModel.OpEqual,
            Value:    orgId,
        })
    }

    // Date range with BETWEEN
    if startTime != nil && endTime != nil {
        filters = append(filters, chModel.Filter{
            Field:    "timestamp",
            Operator: chModel.OpBetween,
            Value:    *startTime,
            EndValue: *endTime,
        })
    }

    return r.rawEventTable.FindWithFilters(ctx, filters, pagination, sort)
}
```

## DI Container Configuration (dig)

```go
import (
    "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
    chManager "github.com/Mapex-Solutions/MapexOS/infrastructure/clickhouse/manager"
)

// Provide ClickHouseManager
c.Provide(func() *chManager.ClickHouseManager {
    ch, err := chManager.New(chManager.Config{
        Host:            cfg.Host,
        Port:            cfg.Port,
        Database:        cfg.Database,
        Username:        cfg.Username,
        Password:        cfg.Password,
        EnableMonitor:   true,
        MonitorInterval: 30 * time.Second,
    })
    if err != nil {
        logger.Panic(err.Error())
    }
    return ch
})

// Provide raw connection for repositories
c.Provide(func(ch *chManager.ClickHouseManager) driver.Conn {
    return ch.GetConn()
})
```

## Important Notes

1. **Field order**: The order of fields in the struct should match the order of columns in the ClickHouse table for better performance.

2. **Automatic JSON**: Fields of type `map[string]interface{}` or `[]interface{}` are automatically serialized/deserialized as JSON.

3. **Timestamps**: Configure `TimestampField` to enable default time-based ordering.

4. **Batch inserts**: Use `InsertBatch` for large data volumes - ClickHouse is optimized for bulk operations.

5. **BETWEEN vs range**: For date filters, prefer `OpBetween` instead of two separate filters (`>=` and `<=`) for more efficient queries.
