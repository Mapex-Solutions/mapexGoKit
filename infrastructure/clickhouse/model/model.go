package chModel

import (
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// NewTable creates and returns a new generic Table[T] instance,
// which acts as a wrapper around a ClickHouse table.
//
// It uses reflection to extract field metadata from the struct type T,
// mapping struct fields to ClickHouse columns via the "ch" tag.
//
// Parameters:
//   - conn: the ClickHouse driver connection
//   - tableName: the name of the ClickHouse table
//   - cfg: optional configuration for timeouts and default behavior
//
// Example:
//
//	type Event struct {
//	    Timestamp time.Time `ch:"timestamp"`
//	    OrgId     string    `ch:"org_id"`
//	    Payload   string    `ch:"payload"`
//	}
//
//	table := chModel.NewTable[Event](conn, "events", chModel.TableConfig{
//	    TimestampField: "timestamp",
//	    DefaultOrder:   "DESC",
//	})
//
//	// Insert single record
//	err := table.Insert(ctx, &event)
//
//	// Insert batch
//	err := table.InsertBatch(ctx, events)
//
//	// Query with pagination
//	result, err := table.FindByOffset(ctx, filters, pagination, "timestamp:desc")
func NewTable[T any](conn driver.Conn, tableName string, cfg TableConfig) (*Table[T], error) {
	// Extract field metadata from type T
	fields, err := extractFields[T]()
	if err != nil {
		return nil, err
	}

	// Apply defaults
	if cfg.TimestampField == "" {
		cfg.TimestampField = DefaultTimestampField
	}
	if cfg.DefaultOrder == "" {
		cfg.DefaultOrder = DefaultOrder
	}
	if cfg.DefaultTimeout == 0 {
		cfg.DefaultTimeout = DefaultTimeout
	}

	logger.Info("[INFRA:CLICKHOUSE] Table initialized: " + tableName)

	return &Table[T]{
		conn:      conn,
		tableName: tableName,
		cfg:       cfg,
		fields:    fields,
	}, nil
}

// TableName returns the name of the underlying table.
func (t *Table[T]) TableName() string {
	return t.tableName
}

// Config returns the table configuration.
func (t *Table[T]) Config() TableConfig {
	return t.cfg
}

// Columns returns the list of ClickHouse column names.
func (t *Table[T]) Columns() []string {
	return getColumnNames(t.fields)
}

// Conn returns the underlying ClickHouse connection for direct access.
// Use this for complex queries that are not covered by the model methods.
func (t *Table[T]) Conn() driver.Conn {
	return t.conn
}

// Query returns a new QueryBuilder for this table.
// This allows building complex queries with filters, ordering, and pagination.
//
// Example:
//
//	query, args := table.Query().
//	    Select("timestamp", "org_id", "payload").
//	    Where("org_id", chModel.OpEqual, "org-123").
//	    Where("timestamp", chModel.OpGreaterEqual, startTime).
//	    OrderBy("timestamp DESC").
//	    Limit(100).
//	    BuildSelect()
func (t *Table[T]) Query() *QueryBuilder {
	return NewQueryBuilder(t.tableName)
}
