package chModel

import (
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Map is a shorthand alias for map[string]interface{}, commonly used for filters.
type Map = map[string]interface{}

// Table represents a ClickHouse table wrapper for a specific type T.
//
// This struct provides high-level methods such as Insert, InsertBatch, FindByOffset,
// and Count using generics to ensure type safety.
type Table[T any] struct {
	conn      driver.Conn
	tableName string
	cfg       TableConfig

	// Cached field metadata extracted via reflection
	fields []fieldInfo
}

// TableConfig holds configuration for table operations.
type TableConfig struct {
	// TimestampField is the field used for default ordering (e.g., "timestamp")
	TimestampField string

	// DefaultOrder is the default sort order ("ASC" or "DESC")
	DefaultOrder string

	// DefaultTimeout defines a default timeout applied when the context has no deadline.
	DefaultTimeout time.Duration
}

// fieldInfo holds metadata about a struct field for ClickHouse operations.
type fieldInfo struct {
	Name       string // Go struct field name
	Column     string // ClickHouse column name (from ch tag)
	Index      int    // Field index in struct
	IsJSON     bool   // Whether to marshal as JSON (for map/slice types)
	IsPointer  bool   // Whether the field is a pointer type
	IsTime     bool   // Whether the field is time.Time
}

// PaginationOpts holds pagination parameters for queries.
type PaginationOpts struct {
	Page    int64 `json:"page"`    // Page number (starting from 1)
	PerPage int64 `json:"perPage"` // Items per page
}

// Pagination provides metadata about the paginated result.
type Pagination struct {
	Page       int64 `json:"page"`
	PerPage    int64 `json:"perPage"`
	TotalItems int64 `json:"totalItems"`
	TotalPages int64 `json:"totalPages"`
}

// PaginatedResult represents the output of a paginated query.
type PaginatedResult[T any] struct {
	Items      []T        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

// QueryOpts holds optional parameters for query operations.
type QueryOpts struct {
	// Select specifies which columns to retrieve (nil = all columns)
	Select []string

	// OrderBy specifies the sort order (e.g., "timestamp DESC")
	OrderBy string

	// Limit limits the number of results
	Limit int64

	// Offset skips the first N results
	Offset int64
}

// FilterOperator defines comparison operators for filters.
type FilterOperator string

// Filter represents a single filter condition.
type Filter struct {
	Field    string
	Operator FilterOperator
	Value    interface{}
	// EndValue is used for BETWEEN operator
	EndValue interface{}
}

// TimeCursorOpts holds options for time-based cursor pagination.
// This is optimized for ClickHouse's timestamp-ordered data.
type TimeCursorOpts struct {
	// Cursor is the timestamp to start from (as RFC3339 string or time.Time)
	Cursor interface{}

	// Direction: "next" for forward, "prev" for backward
	Direction string

	// Limit is the maximum number of items to return
	Limit int64

	// SortAsc controls the sort direction (true = ASC, false = DESC)
	SortAsc bool
}

// TimeCursorResult contains the result of a time-based cursor pagination query.
type TimeCursorResult[T any] struct {
	Items       []T       `json:"items"`
	NextCursor  time.Time `json:"nextCursor,omitempty"`
	PrevCursor  time.Time `json:"prevCursor,omitempty"`
	HasNext     bool      `json:"hasNext"`
	HasPrevious bool      `json:"hasPrevious"`
}
