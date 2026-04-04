package mongoManager

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Config struct {
	URI             string
	Database        string
	EnableMonitor   bool
	MonitorInterval time.Duration

	// UseBsonD controls how the driver unmarshals nested documents into interface{} fields.
	// Default (false): uses map[string]interface{} (bson.M) — recommended for most services.
	// When true: uses bson.D (ordered document) — only for services that need field ordering.
	UseBsonD bool

	// Backpressure (opt-in, default: false = backward compatible).
	// When enabled, write latencies are tracked in a circular buffer and
	// P99 is computed every 5 seconds to determine the current mode.
	EnableBackpressure   bool
	BackpressureWindow   int   // circular buffer capacity (default: 1000)
	ThrottledThresholdMs int64 // P99 above this → Throttled (default: 150)
	BackoffThresholdMs   int64 // P99 above this → Backoff   (default: 500)
}

type MongoManager struct {
	client     *mongo.Client
	dbInstance *mongo.Database
	dbName     string

	isConnected atomic.Bool
	lastLatency atomic.Int64
	mu          sync.RWMutex
	cfg         Config

	bp       *backpressureTracker // nil when backpressure disabled
	bpCancel context.CancelFunc  // stops the background goroutine
}
