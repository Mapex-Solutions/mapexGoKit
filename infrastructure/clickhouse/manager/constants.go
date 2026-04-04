package chManager

import "time"

const (
	// DefaultMonitorInterval is the default interval between health checks (in seconds).
	DefaultMonitorInterval = 10 * time.Second

	// DefaultPort is the default ClickHouse native protocol port.
	DefaultPort = 9000

	// DefaultConnectTimeout is the timeout for initial connection.
	DefaultConnectTimeout = 5 * time.Second

	// DefaultPingTimeout is the timeout for ping/health check operations.
	DefaultPingTimeout = 3 * time.Second
)
