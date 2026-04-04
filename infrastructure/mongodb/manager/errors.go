package mongoManager

import "errors"

// ErrMissingURIOrDatabase is returned when URI or Database configuration is missing.
var ErrMissingURIOrDatabase = errors.New("URI and Database are required")

// ErrNotConnected is returned when attempting to use the manager without an active connection.
var ErrNotConnected = errors.New("MongoDB client is not connected")
