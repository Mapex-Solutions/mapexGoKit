package chModel

import "time"

//
// PAGINATION DEFAULTS
//

const (
	// DefaultPage is the default page number for offset-based pagination.
	DefaultPage int64 = 1

	// DefaultPerPage is the default number of items per page.
	DefaultPerPage int64 = 25

	// MaxPerPage is the maximum items per page allowed.
	MaxPerPage int64 = 300

	// MaxOffset is the maximum offset allowed for pagination.
	MaxOffset int64 = 10000
)

//
// QUERY DEFAULTS
//

const (
	// DefaultTimeout is the default query timeout.
	DefaultTimeout = 30 * time.Second

	// DefaultTimestampField is the default field used for time-based ordering.
	DefaultTimestampField = "timestamp"

	// DefaultOrder is the default sort order.
	DefaultOrder = "DESC"
)

//
// STRUCT TAGS
//

const (
	// TagName is the struct tag used for ClickHouse column mapping.
	TagName = "ch"

	// JSONTagName is the struct tag used for JSON column mapping fallback.
	JSONTagName = "json"
)

//
// FILTER OPERATORS
//

const (
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
)
