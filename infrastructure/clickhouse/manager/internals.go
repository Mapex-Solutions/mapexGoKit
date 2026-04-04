package chManager

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// connect establishes a connection to ClickHouse using the configured options.
// It performs a ping to verify the connection is healthy.
func (m *ClickHouseManager) connect() error {
	maxOpenConns := m.cfg.MaxOpenConns
	if maxOpenConns == 0 {
		maxOpenConns = 10
	}
	maxIdleConns := m.cfg.MaxIdleConns
	if maxIdleConns == 0 {
		maxIdleConns = 5
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)},
		Auth: clickhouse.Auth{
			Database: m.cfg.Database,
			Username: m.cfg.Username,
			Password: m.cfg.Password,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout:     DefaultConnectTimeout,
		MaxOpenConns:    maxOpenConns,
		MaxIdleConns:    maxIdleConns,
		ConnMaxLifetime: time.Hour,
	})

	if err != nil {
		logger.Error(err, "[INFRA:CLICKHOUSE] Connection error")
		m.isConnected.Store(false)
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	// Verify connection with ping
	ctx, cancel := context.WithTimeout(context.Background(), DefaultPingTimeout)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		logger.Error(err, "[INFRA:CLICKHOUSE] Ping failed")
		m.isConnected.Store(false)
		return fmt.Errorf("%w: %v", ErrPingFailed, err)
	}

	m.mu.Lock()
	m.conn = conn
	m.mu.Unlock()

	m.isConnected.Store(true)
	logger.Info("[INFRA:CLICKHOUSE] Connected to ClickHouse.")
	return nil
}

// startMonitor runs a background goroutine that periodically checks the connection health.
// If the connection is lost, it logs an error and updates the isConnected status.
func (m *ClickHouseManager) startMonitor() {
	interval := m.cfg.MonitorInterval
	if interval == 0 {
		interval = DefaultMonitorInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.RLock()
		conn := m.conn
		m.mu.RUnlock()

		if conn == nil {
			m.isConnected.Store(false)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), DefaultPingTimeout)
		start := time.Now()
		err := conn.Ping(ctx)
		latency := time.Since(start)
		cancel()

		if err != nil {
			logger.Error(err, "[INFRA:CLICKHOUSE] Connection lost, attempting reconnect...")
			m.isConnected.Store(false)

			// Attempt to reconnect
			if reconnErr := m.connect(); reconnErr != nil {
				logger.Error(reconnErr, "[INFRA:CLICKHOUSE] Reconnection failed")
			}
		} else {
			m.lastLatency.Store(latency.Milliseconds())
			if !m.isConnected.Load() {
				logger.Info("[INFRA:CLICKHOUSE] Connection restored")
			}
			m.isConnected.Store(true)
		}
	}
}

// ping performs a health check on the connection.
func (m *ClickHouseManager) ping(ctx context.Context) error {
	m.mu.RLock()
	conn := m.conn
	m.mu.RUnlock()

	if conn == nil {
		return ErrNotConnected
	}

	return conn.Ping(ctx)
}
