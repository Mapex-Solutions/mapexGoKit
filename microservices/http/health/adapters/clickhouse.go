package adapters

import (
	"context"
	"time"

	common "github.com/Mapex-Solutions/mapexGoKit/infrastructure/common/ports"
	chManager "github.com/Mapex-Solutions/mapexGoKit/infrastructure/clickhouse/manager"
)

// ClickHouseAdapter checks ClickHouse health using the existing monitor (zero-cost).
type ClickHouseAdapter struct {
	manager *chManager.ClickHouseManager
}

// NewClickHouseAdapter creates a new ClickHouse health adapter.
func NewClickHouseAdapter(manager *chManager.ClickHouseManager) *ClickHouseAdapter {
	return &ClickHouseAdapter{manager: manager}
}

// Name returns the adapter identifier for ClickHouse.
func (a *ClickHouseAdapter) Name() string { return "clickhouse" }

// Check returns the current health status using the background monitor (zero-cost).
func (a *ClickHouseAdapter) Check(_ context.Context) common.HealthStatus {
	return common.HealthStatus{
		Service:     "clickhouse",
		Connected:   a.manager.IsConnected(),
		LatencyMs:   a.manager.LastLatency(),
		LastCheckAt: time.Now(),
	}
}
