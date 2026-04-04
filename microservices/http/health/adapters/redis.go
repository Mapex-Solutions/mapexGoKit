package adapters

import (
	"context"
	"time"

	common "github.com/Mapex-Solutions/mapexGoKit/infrastructure/common/ports"
	redisModel "github.com/Mapex-Solutions/mapexGoKit/infrastructure/redis"
)

// RedisAdapter checks Redis health by issuing a PING command.
type RedisAdapter struct {
	client *redisModel.RedisClient
	name   string
}

// NewRedisAdapter creates a new Redis health adapter with the given instance name.
func NewRedisAdapter(client *redisModel.RedisClient, name string) *RedisAdapter {
	return &RedisAdapter{client: client, name: name}
}

// Name returns the adapter identifier for this Redis instance.
func (a *RedisAdapter) Name() string { return "redis:" + a.name }

// Check pings Redis and returns the health status with measured latency.
func (a *RedisAdapter) Check(ctx context.Context) common.HealthStatus {
	start := time.Now()
	err := a.client.Ping(ctx)
	latency := time.Since(start).Milliseconds()

	status := common.HealthStatus{
		Service:     "redis:" + a.name,
		Connected:   err == nil,
		LatencyMs:   latency,
		LastCheckAt: time.Now(),
	}
	if err != nil {
		status.ErrorMessage = err.Error()
	}

	return status
}
