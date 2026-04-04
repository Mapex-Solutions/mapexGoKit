package chManager

import (
	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// New creates and returns a new ClickHouseManager based on the given configuration.
//
// It validates the required configuration fields, establishes the initial connection,
// and optionally starts a monitoring goroutine if EnableMonitor is true.
//
// Parameters:
//
//	cfg – configuration struct containing:
//	  • Host           – ClickHouse server hostname (required)
//	  • Port           – ClickHouse native protocol port (default: 9000)
//	  • Database       – name of the database to use (required)
//	  • Username       – authentication username (required)
//	  • Password       – authentication password (required)
//	  • EnableMonitor  – flag to spawn a background monitor for health checks
//	  • MonitorInterval – interval between health checks (default: 10s)
//
// Returns:
//   - *ClickHouseManager – pointer to an initialized ClickHouseManager instance
//   - error              – nil on success, or an error if validation or connection fails
//
// Example:
//
//	cfg := chManager.Config{
//	  Host:           "localhost",
//	  Port:           9000,
//	  Database:       "mapexos",
//	  Username:       "default",
//	  Password:       "secret",
//	  EnableMonitor:  true,
//	  MonitorInterval: 10 * time.Second,
//	}
//	mgr, err := chManager.New(cfg)
//	if err != nil {
//	  log.Fatal("failed to create ClickHouseManager:", err)
//	}
//	defer mgr.Close()
func New(cfg Config) (*ClickHouseManager, error) {
	// Validate required fields
	if cfg.Host == "" || cfg.Database == "" || cfg.Username == "" {
		return nil, ErrMissingConfig
	}

	// Apply defaults
	if cfg.Port == 0 {
		cfg.Port = DefaultPort
	}
	if cfg.MonitorInterval == 0 {
		cfg.MonitorInterval = DefaultMonitorInterval
	}

	m := &ClickHouseManager{
		cfg:      cfg,
		database: cfg.Database,
	}

	// Establish initial connection
	if err := m.connect(); err != nil {
		return nil, err
	}

	// Start background health monitor if enabled
	if cfg.EnableMonitor {
		go m.startMonitor()
	}

	logger.Info("[INFRA:CLICKHOUSE] Manager initialized")
	return m, nil
}
