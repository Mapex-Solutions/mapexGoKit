package clickhouseModel

import (
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Client wraps a ClickHouse driver connection
type Client struct {
	conn driver.Conn
}

// Config holds the ClickHouse connection configuration
type Config struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
}
