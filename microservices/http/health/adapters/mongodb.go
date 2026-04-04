package adapters

import (
	"context"
	"time"

	common "github.com/Mapex-Solutions/mapexGoKit/infrastructure/common/ports"
	mongoManager "github.com/Mapex-Solutions/mapexGoKit/infrastructure/mongodb/manager"
)

// MongoAdapter checks MongoDB health using the existing monitor (zero-cost).
type MongoAdapter struct {
	manager *mongoManager.MongoManager
}

// NewMongoAdapter creates a new MongoDB health adapter.
func NewMongoAdapter(manager *mongoManager.MongoManager) *MongoAdapter {
	return &MongoAdapter{manager: manager}
}

// Name returns the adapter identifier for MongoDB.
func (a *MongoAdapter) Name() string { return "mongodb" }

// Check returns the current health status using the background monitor (zero-cost).
func (a *MongoAdapter) Check(_ context.Context) common.HealthStatus {
	return common.HealthStatus{
		Service:     "mongodb",
		Connected:   a.manager.IsConnected(),
		LatencyMs:   a.manager.LastLatency(),
		LastCheckAt: time.Now(),
	}
}
