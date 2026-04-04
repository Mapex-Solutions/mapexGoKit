package health

import (
	"context"
	"time"

	common "github.com/Mapex-Solutions/mapexGoKit/infrastructure/common/ports"
)

// Config holds the configuration for the health check service.
type Config struct {
	ServiceName string
	Version     string
	CacheTTL    time.Duration
	Timeout     time.Duration
}

// CheckerConfig pairs a health checker with its criticality level.
type CheckerConfig struct {
	Checker  Checker
	Critical bool
}

// Checker is the interface that infrastructure adapters must implement.
type Checker interface {
	Name() string
	Check(ctx context.Context) common.HealthStatus
}

// Response is the JSON payload returned by the /health endpoint.
type Response struct {
	Status      string                `json:"status"`
	Service     string                `json:"service"`
	Version     string                `json:"version"`
	Uptime      string                `json:"uptime"`
	Timestamp   time.Time             `json:"timestamp"`
	LastCheckAt time.Time             `json:"lastCheckAt"`
	Checks      map[string]CheckDetail `json:"checks"`
}

// CheckDetail is the status of an individual dependency in the health response.
type CheckDetail struct {
	Connected    bool   `json:"connected"`
	Critical     bool   `json:"critical"`
	LatencyMs    int64  `json:"latencyMs,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}
