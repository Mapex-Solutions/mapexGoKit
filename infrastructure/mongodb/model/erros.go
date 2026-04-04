package mongoModel

import (
	"errors"
)

// Common error definitions used across MongoDB model operations.
var (
	// ErrNotFound is returned when a document is not found in the collection.
	ErrNotFound = errors.New("document not found")

	// ErrInvalidID is returned when the provided ID is not a valid ObjectID.
	ErrInvalidID = errors.New("invalid objectID format")

	// ErrEmptyItems is returned when attempting to insert an empty slice of documents.
	ErrEmptyItems = errors.New("no items to insert")

	// ErrEmptyFilters is returned when a query or delete operation is attempted without any filters.
	ErrEmptyFilters = errors.New("filters must be provided")

	// ErrCursorPaginationRequired is returned when cursor pagination is requested but not properly configured.
	ErrCursorPaginationRequired = errors.New("pagination must be enabled with UseCursor = true")

	// ErrInvalidCursorDirection is returned when the cursor pagination direction is not 1 (forward) or -1 (backward).
	ErrInvalidCursorDirection = errors.New("invalid SortDirection, must be 1 (forward) or -1 (backward)")

	// ErrNotConnected is returned when attempting to use the model without an active connection.
	ErrNotConnected = errors.New("MongoDB client is not connected")
)
