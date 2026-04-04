package chModel

import "errors"

var (
	// ErrEmptyItems is returned when attempting to insert an empty slice.
	ErrEmptyItems = errors.New("items slice cannot be empty")

	// ErrInvalidType is returned when the type parameter is not a struct.
	ErrInvalidType = errors.New("type parameter must be a struct")

	// ErrNoFields is returned when no ch-tagged fields are found in the struct.
	ErrNoFields = errors.New("no ch-tagged fields found in struct")

	// ErrNotFound is returned when a query returns no results.
	ErrNotFound = errors.New("record not found")

	// ErrInvalidFilter is returned when a filter is malformed.
	ErrInvalidFilter = errors.New("invalid filter")

	// ErrQueryFailed is returned when a query execution fails.
	ErrQueryFailed = errors.New("query execution failed")

	// ErrScanFailed is returned when scanning results fails.
	ErrScanFailed = errors.New("failed to scan results")

	// ErrBatchFailed is returned when batch insert fails.
	ErrBatchFailed = errors.New("batch insert failed")

	// ErrMarshalFailed is returned when JSON marshaling fails.
	ErrMarshalFailed = errors.New("failed to marshal JSON field")
)
