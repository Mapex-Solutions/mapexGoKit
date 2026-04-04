package chManager

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Config holds the ClickHouse connection configuration.
//
// Fields:
//   - Host: ClickHouse server hostname (e.g., "localhost")
//   - Port: ClickHouse native protocol port (default: 9000)
//   - Database: Database name to connect to
//   - Username: Authentication username
//   - Password: Authentication password
//   - MaxOpenConns: Max open connections in pool (0 = default 10)
//   - MaxIdleConns: Max idle connections retained (0 = default 5)
//   - EnableMonitor: Enable background health monitoring
//   - MonitorInterval: Interval in seconds between health checks
type Config struct {
	Host            string
	Port            int
	Database        string
	Username        string
	Password        string
	MaxOpenConns    int
	MaxIdleConns    int
	EnableMonitor   bool
	MonitorInterval time.Duration
}

// ClickHouseManager manages the ClickHouse connection lifecycle.
//
// It provides:
//   - Connection management with automatic reconnection
//   - Health monitoring via background goroutine
//   - Thread-safe access to the connection
//   - Status reporting for health endpoints
type ClickHouseManager struct {
	conn     driver.Conn
	database string

	isConnected atomic.Bool
	lastLatency atomic.Int64
	mu          sync.RWMutex
	cfg         Config
}

// HealthStatus represents the current health state of the ClickHouse connection.
// This can be used for health check endpoints and Prometheus metrics.
type HealthStatus struct {
	Connected    bool      `json:"connected"`
	Database     string    `json:"database"`
	Host         string    `json:"host"`
	Port         int       `json:"port"`
	LastCheckAt  time.Time `json:"lastCheckAt"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
}
