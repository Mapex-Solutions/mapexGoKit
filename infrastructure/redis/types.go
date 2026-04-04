package redisModel

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	client    *redis.Client
	keyPrefix string
}

type Config struct {
	Host      string
	Port      int
	Username  string
	Password  string
	DB        int
	KeyPrefix string
}

type GetOrSetParams struct {
	Ctx      context.Context
	CacheKey string
	CacheTTL int // in seconds
	Callback func() (interface{}, error)
}

type SetOptions struct {
	TTL         time.Duration // Time-to-live (0 = no expiration)
	NX          bool          // Only set if key does not exist (SET NX)
	XX          bool          // Only set if key already exists (SET XX)
	KeepTTL     bool          // Retain existing TTL (SET KEEPTTL)
	Tags        []string      // Optional metadata (e.g. for observability or grouping)
	Compression bool          // If true, compress data before setting
}
