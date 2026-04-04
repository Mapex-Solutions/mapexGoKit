package adapters

import (
	"context"
	"time"

	common "github.com/Mapex-Solutions/mapexGoKit/infrastructure/common/ports"
	minioModel "github.com/Mapex-Solutions/mapexGoKit/infrastructure/minio"
)

// MinIOAdapter checks MinIO health by listing buckets.
type MinIOAdapter struct {
	client *minioModel.MinIOClient
	name   string
}

// NewMinIOAdapter creates a new MinIO health adapter with the given instance name.
func NewMinIOAdapter(client *minioModel.MinIOClient, name string) *MinIOAdapter {
	return &MinIOAdapter{client: client, name: name}
}

// Name returns the adapter identifier for this MinIO instance.
func (a *MinIOAdapter) Name() string { return "minio:" + a.name }

// Check pings MinIO via ListBuckets and returns the health status with measured latency.
func (a *MinIOAdapter) Check(ctx context.Context) common.HealthStatus {
	start := time.Now()
	err := a.client.Ping(ctx)
	latency := time.Since(start).Milliseconds()

	status := common.HealthStatus{
		Service:     "minio:" + a.name,
		Connected:   err == nil,
		LatencyMs:   latency,
		LastCheckAt: time.Now(),
	}
	if err != nil {
		status.ErrorMessage = err.Error()
	}

	return status
}
