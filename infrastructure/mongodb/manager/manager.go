package mongoManager

import (
	"context"

	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// New creates and returns a MongoManager based on the given configuration.
// It validates that both the URI and Database fields are provided,
// establishes the initial connection, and optionally starts a monitoring
// goroutine if EnableMonitor is true.
//
// Parameters:
//
//	cfg – configuration struct containing:
//	  • URI           – MongoDB connection string (must be non-empty)
//	  • Database      – name of the database to use (must be non-empty)
//	  • EnableMonitor – flag to spawn a background monitor for metrics/logs
//
// Returns:
//   - *MongoManager – pointer to an initialized MongoManager instance
//   - error         – nil on success, or an error if validation, connection,
//     or monitoring setup fails (e.g., ErrMissingURIOrDatabase)
//
// Example:
//
//	cfg := Config{
//	  URI:           "mongodb://localhost:27017",
//	  Database:      "mydb",
//	  EnableMonitor: true,
//	}
//	mgr, err := New(cfg)
//	if err != nil {
//	  log.Fatal().Err(err).Msg("failed to create MongoManager")
//	}
func New(cfg Config) (*MongoManager, error) {

	if cfg.URI == "" || cfg.Database == "" {
		return nil, ErrMissingURIOrDatabase
	}

	m := &MongoManager{cfg: cfg, dbName: cfg.Database}

	if err := m.connect(); err != nil {
		return nil, err
	}

	if cfg.EnableMonitor {
		go m.startMonitor()
	}

	// Initialize backpressure tracker if enabled (opt-in)
	if cfg.EnableBackpressure {
		window := cfg.BackpressureWindow
		if window <= 0 {
			window = defaultBackpressureWindow
		}
		throttledMs := cfg.ThrottledThresholdMs
		if throttledMs <= 0 {
			throttledMs = defaultThrottledThresholdMs
		}
		backoffMs := cfg.BackoffThresholdMs
		if backoffMs <= 0 {
			backoffMs = defaultBackoffThresholdMs
		}

		m.bp = newTracker(window, throttledMs, backoffMs)
		ctx, cancel := context.WithCancel(context.Background())
		m.bpCancel = cancel
		go m.bp.start(ctx)
	}

	logger.Info("[INFRA:MONGODB] Initialized")
	return m, nil
}

// Close gracefully disconnects the MongoDB client and releases resources.
// It stops the backpressure tracker (if running) before disconnecting.
func (m *MongoManager) Close(ctx context.Context) error {
	// Stop backpressure tracker first
	if m.bpCancel != nil {
		m.bpCancel()
	}

	if m.client != nil {
		logger.Info("[INFRA:MONGODB] Disconnecting...")
		return m.client.Disconnect(ctx)
	}
	return nil
}
