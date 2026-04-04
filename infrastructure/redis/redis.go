package redisModel

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

func New(config Config) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.Host + ":" + strconv.Itoa(config.Port),
		Username: config.Username,
		Password: config.Password,
		DB:       config.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	logger.Info("[INFRA:REDIS] Initialized")

	return &RedisClient{
		client:    rdb,
		keyPrefix: config.KeyPrefix,
	}, nil
}

// Close gracefully closes the Redis client connection.
func (r *RedisClient) Close() error {
	if r.client != nil {
		logger.Info("[INFRA:REDIS] Closing connection...")
		return r.client.Close()
	}
	return nil
}

// NewGoRedisClient creates a raw go-redis client (without RedisClient wrapper).
// Used for components that need direct access to *redis.Client (e.g., RedisLock).
func NewGoRedisClient(config Config) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.Host + ":" + strconv.Itoa(config.Port),
		Username: config.Username,
		Password: config.Password,
		DB:       config.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Panic(fmt.Sprintf("redis ping failed: %v", err))
	}

	return rdb
}
