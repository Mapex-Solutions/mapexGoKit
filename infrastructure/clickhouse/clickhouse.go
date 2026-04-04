package clickhouseModel

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// New creates a new ClickHouse client with the provided configuration.
// It establishes a connection to the ClickHouse server, tests it with a ping,
// and returns a Client instance wrapping the connection.
func New(config Config) (*Client, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", config.Host, config.Port)},
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.Username,
			Password: config.Password,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("clickhouse connection failed: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("clickhouse ping failed: %w", err)
	}

	logger.Info("[INFRA:CLICKHOUSE] Initialized successfully")

	return &Client{
		conn: conn,
	}, nil
}

// GetConn returns the underlying driver.Conn for direct access.
// This allows services to use the raw ClickHouse connection when needed.
func (c *Client) GetConn() driver.Conn {
	return c.conn
}
