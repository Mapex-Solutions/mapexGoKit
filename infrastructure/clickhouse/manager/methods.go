package chManager

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// IsConnected returns the current connection status.
// This is thread-safe and can be called from any goroutine.
func (m *ClickHouseManager) IsConnected() bool {
	return m.isConnected.Load()
}

// GetConn returns the underlying ClickHouse driver connection.
// This allows direct access for low-level operations when needed.
//
// Note: The connection may be nil if not yet established.
// Use IsConnected() to verify the connection status first.
func (m *ClickHouseManager) GetConn() driver.Conn {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.conn
}

// GetDatabase returns the configured database name.
func (m *ClickHouseManager) GetDatabase() string {
	return m.database
}

// GetConfig returns a copy of the current configuration.
// Sensitive fields (password) are masked.
func (m *ClickHouseManager) GetConfig() Config {
	cfg := m.cfg
	cfg.Password = "***" // Mask password
	return cfg
}

// Health returns the current health status of the ClickHouse connection.
// This can be used for health check endpoints and monitoring.
func (m *ClickHouseManager) Health(ctx context.Context) HealthStatus {
	status := HealthStatus{
		Connected:   m.isConnected.Load(),
		Database:    m.database,
		Host:        m.cfg.Host,
		Port:        m.cfg.Port,
		LastCheckAt: time.Now(),
	}

	// Perform a live ping if context allows
	if ctx != nil {
		if err := m.ping(ctx); err != nil {
			status.Connected = false
			status.ErrorMessage = err.Error()
		}
	}

	return status
}

// LastLatency returns the last measured ping latency in milliseconds.
func (m *ClickHouseManager) LastLatency() int64 {
	return m.lastLatency.Load()
}

// Close gracefully closes the ClickHouse connection.
// This should be called when the application is shutting down.
func (m *ClickHouseManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn != nil {
		m.isConnected.Store(false)
		return m.conn.Close()
	}
	return nil
}
