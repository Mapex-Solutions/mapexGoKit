package chManager

import "errors"

var (
	// ErrMissingConfig is returned when required configuration fields are missing.
	ErrMissingConfig = errors.New("host, database, username, and password are required")

	// ErrNotConnected is returned when attempting operations without an active connection.
	ErrNotConnected = errors.New("clickhouse is not connected")

	// ErrConnectionFailed is returned when the connection attempt fails.
	ErrConnectionFailed = errors.New("failed to connect to clickhouse")

	// ErrPingFailed is returned when the ping/health check fails.
	ErrPingFailed = errors.New("clickhouse ping failed")
)
