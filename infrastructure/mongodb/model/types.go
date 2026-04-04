package mongoModel

import (
	"reflect"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	typeObjectID = reflect.TypeOf(bson.ObjectID{})
	typeTime     = reflect.TypeOf(time.Time{})
)

// ── BSON Aliases ──

// Map is an alias for bson.M — used for filters, projections, updates.
type Map = bson.M

// ObjectId is an alias for bson.ObjectID.
type ObjectId = bson.ObjectID

// ── Collection Aliases ──

// Collection is an alias for mongo.Collection.
type Collection = mongo.Collection

// ── Options Aliases ──

// ReturnDoc is an alias for options.ReturnDocument used in FindAndUpdate operations.
type ReturnDoc = options.ReturnDocument

// BulkWriteOptions is an alias for options.BulkWriteOptionsBuilder.
type BulkWriteOptions = options.BulkWriteOptionsBuilder

// ── Write Model Aliases ──

// WriteModel is an alias for mongo.WriteModel — used in BulkWrite operations.
type WriteModel = mongo.WriteModel

// ── Result Aliases ──

// UpdateResult is an alias for mongo.UpdateResult.
type UpdateResult = mongo.UpdateResult

// DeleteResult is an alias for mongo.DeleteResult.
type DeleteResult = mongo.DeleteResult

// BulkWriteResult is an alias for mongo.BulkWriteResult.
type BulkWriteResult = mongo.BulkWriteResult

// IndexDefinition defines a MongoDB index to be created on collection initialization.
type IndexDefinition struct {
	// Name is the unique identifier for this index (required).
	// Example: "idx_email_unique", "idx_assignee_org"
	Name string

	// Keys defines the fields and their sort order.
	// Use 1 for ascending, -1 for descending.
	// Example: map[string]int{"email": 1} or map[string]int{"orgId": 1, "assigneeId": 1}
	Keys map[string]int

	// Unique ensures all values in the indexed field are unique.
	// Default: false
	Unique bool

	// Sparse only includes documents that have the indexed field.
	// Useful for optional fields with unique constraint.
	// Default: false
	Sparse bool

	// PartialFilterExpression limits the index to documents matching this filter.
	// Uses bson.M format. Only documents matching the filter are included in the index.
	// Example: bson.M{"timerExpiresAt": bson.M{"$type": "date"}}
	// Default: nil (no filter — full index)
	PartialFilterExpression bson.M

	// ExpireAfterSeconds enables MongoDB TTL (Time-To-Live) for automatic document deletion.
	// The indexed field must contain a date value. MongoDB deletes documents when the date
	// plus this duration has passed. Use 0 when the field itself contains the exact expiry time.
	// Default: nil (no TTL — documents are not auto-deleted)
	ExpireAfterSeconds *int32
}

// Config holds default behaviors for the model layer.
type Config struct {
	// DefaultTimeout defines a default timeout applied when the incoming context has no deadline.
	// A value of 0 means no default timeout will be applied.
	DefaultTimeout time.Duration

	// Indexes defines the indexes to be created on collection initialization.
	// Indexes are created only if they don't already exist (idempotent).
	// Example:
	//   Indexes: []IndexDefinition{
	//       {Name: "idx_email_unique", Keys: map[string]int{"email": 1}, Unique: true},
	//       {Name: "idx_org_path", Keys: map[string]int{"orgId": 1, "pathKey": 1}},
	//   }
	Indexes []IndexDefinition
}

// CommonOpts represents optional parameters that can be passed to various MongoDB operations.
// These options mirror the MongoDB driver's options for fine-grained control over queries and updates.
type CommonOpts struct {
	// Shared options
	Session    *mongo.Session     // Optional session to use during the operation
	Projection interface{}        // Fields to include or exclude
	Sort       interface{}        // Sort order
	Hint       interface{}        // Index hint
	Collation  *options.Collation // Collation rules for string comparison

	// Update-specific options
	Upsert                   *bool                   // Create a new document if no match is found
	ReturnDocument           *options.ReturnDocument // Whether to return the document before or after the update
	BypassDocumentValidation *bool                   // Allows the write to opt-out of document validation
	Comment                  interface{}             // Optional comment to include in the query
	MaxTime                  *time.Duration          // Maximum time to allow the query to run
	Let                      interface{}             // Parameters for the update expression
	ArrayFilters             []interface{}           // Filters to apply to array fields during update
}

// Option is a functional option type, currently unused but reserved for future use.
type Option func(*CommonOpts)

// Model represents a MongoDB collection wrapper for a specific type T.
//
// This struct provides high-level methods such as Create, Find, Update, and Delete
// using generics to ensure type safety.
type Model[T any] struct {
	col *mongo.Collection
	cfg Config
}

// PaginationOpts holds the input parameters used to control pagination behavior.
// It supports both offset-based and future cursor-based strategies.
type PaginationOpts struct {
	// Offset-based
	Page    int64 `json:"page"`    // Page number (starting from 1)
	PerPage int64 `json:"perPage"` // Items per page

	// Cursor-based (planned for future use)
	CursorID      any  `json:"cursorId"`      // The ID to start from (ObjectID or string)
	SortDirection int  `json:"sortDirection"` // 1 for ascending, -1 for descending
	UseCursor     bool `json:"useCursor"`     // Whether to use cursor-based pagination
}

// PaginatedResult represents the output of a paginated query,
// including the items retrieved and metadata about pagination.
// PaginatedResult is a generic type that represents paginated query results.
// The Items field contains the actual documents of type T.
type PaginatedResult[T any] struct {
	Items      []T        `json:"items"`      // Documents returned (typed)
	Pagination Pagination `json:"pagination"` // Metadata describing pagination state
}

// Pagination provides metadata about the current paginated result,
// such as the current page, total pages, and total items.
// Fields HasNext and HasPrev are optional and may be used only in cursor-based queries.
type Pagination struct {
	Page       int64 `json:"page,omitempty"`       // Current page
	PerPage    int64 `json:"perPage,omitempty"`    // Number of items per page
	TotalItems int64 `json:"totalItems,omitempty"` // Total number of items matching the query
	TotalPages int64 `json:"totalPages,omitempty"` // Total pages available
	HasNext    *bool `json:"hasNext,omitempty"`    // Optional: true if there is a next page
	HasPrev    *bool `json:"hasPrev,omitempty"`    // Optional: true if there is a previous page
}

// CursorDirection defines the direction for cursor-based pagination.
type CursorDirection string

const (
	// CursorNext indicates forward pagination (next page).
	CursorNext CursorDirection = "next"

	// CursorPrevious indicates backward pagination (previous page).
	CursorPrevious CursorDirection = "previous"
)

// CursorOpts contains options for cursor-based pagination.
// This pagination strategy is optimal for large datasets and infinite scroll scenarios
// as it provides consistent performance regardless of page depth.
type CursorOpts struct {
	// Cursor is the ID (_id field) to start from, represented as a hex string.
	// Empty string means start from the beginning.
	Cursor string

	// Direction specifies whether to paginate forward (next) or backward (previous).
	Direction CursorDirection

	// Limit is the maximum number of items to return.
	// If <= 0, defaults to 300.
	Limit int64

	// SortAsc controls the sort direction.
	// true = ascending (_id: 1), false = descending (_id: -1).
	// Default is true.
	SortAsc bool
}

// CursorResult contains the result of a cursor-based pagination query.
// It includes the retrieved items and cursor information for navigating pages.
type CursorResult[T any] struct {
	// Items contains the documents returned by the query.
	Items []T

	// NextCursor is the cursor value to fetch the next page.
	// Empty string if there are no more pages forward.
	NextCursor string

	// PrevCursor is the cursor value to fetch the previous page.
	// Empty string if there are no pages backward.
	PrevCursor string

	// HasNext indicates whether there are more pages forward.
	HasNext bool

	// HasPrevious indicates whether there are pages backward.
	HasPrevious bool
}
