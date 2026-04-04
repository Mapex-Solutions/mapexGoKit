package mongoManager

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

func (m *MongoManager) IsConnected() bool {
	return m.isConnected.Load()
}

func (m *MongoManager) GetClient() *mongo.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.client
}

func (m *MongoManager) GetDatabase() *mongo.Database {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.client.Database(m.dbName)
}

func (m *MongoManager) LastLatency() int64 {
	return m.lastLatency.Load()
}

func (m *MongoManager) GetDatabaseName() string {
	return m.dbName
}

// GetBackpressureMode returns the current write pressure level.
// Returns Normal if backpressure tracking is disabled.
func (m *MongoManager) GetBackpressureMode() BackpressureMode {
	if m.bp == nil {
		return Normal
	}
	return BackpressureMode(m.bp.mode.Load())
}

// WriteP99 returns the last computed P99 write latency in milliseconds.
// Returns 0 if backpressure tracking is disabled or no samples recorded.
func (m *MongoManager) WriteP99() int64 {
	if m.bp == nil {
		return 0
	}
	return m.bp.p99.Load()
}

// RecordWriteLatency records a write operation latency sample for backpressure tracking.
// This is a no-op if backpressure tracking is disabled.
// Callers should measure elapsed time around BulkWrite/Insert operations and report it here.
func (m *MongoManager) RecordWriteLatency(d time.Duration) {
	if m.bp == nil {
		return
	}
	m.bp.record(d)
}
