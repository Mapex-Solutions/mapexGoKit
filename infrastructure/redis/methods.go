package redisModel

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	common "github.com/Mapex-Solutions/mapexGoKit/infrastructure/common/ports"
	serialize "github.com/Mapex-Solutions/mapexGoKit/utils/serialize"
)

var (
	_ common.Cache            = (*RedisClient)(nil) // base
	_ common.CacheWithTTL     = (*RedisClient)(nil) // TTL
	_ common.CacheWithOptions = (*RedisClient)(nil) // opções
	_ common.CacheGetOrSet    = (*RedisClient)(nil)
	_ common.CacheGetOrSetEx  = (*RedisClient)(nil)
)

// Set stores a value in Redis without expiration.
//
// It accepts any value type. Strings and []byte are stored as-is,
// and other types are serialized using the internal serialization strategy.
//
// The key is automatically prefixed using the RedisClient's keyPrefix.
// The value will be stored permanently (no TTL).
//
// Critical behavior:
//   - Uses prepareValue() to convert the input into a storable string.
//   - Uses NoExpiration (0) to disable key expiration.
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}) error {
	prefixed := r.keyPrefix + ":" + key

	data, err := r.prepareValue(value)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, prefixed, data, NoExpiration).Err()
}

// SetEx stores a value in Redis with a defined TTL (time-to-live).
//
// It accepts any value type. Strings and []byte are stored as-is,
// and other types are serialized using prepareValue().
//
// The key will automatically expire after the given ttl.
// If ttl is 0 or negative, Redis will return an error.
//
// Use this when you want the data to expire after a certain time.
//
// Critical behavior:
//   - TTL must be greater than 0 to avoid Redis error.
//   - Serialization is handled internally.
func (r *RedisClient) SetEx(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	prefixed := r.keyPrefix + ":" + key

	data, err := r.prepareValue(value)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, prefixed, data, ttl).Err()
}

// SetWithOptions stores a value in Redis using advanced SetOptions.
//
// You can control TTL, conditional behavior (NX or XX), TTL retention (KeepTTL),
// and optionally extend the options for tracing, logging, or compression.
//
// TTL behavior:
//   - If SetOptions.TTL == 0, the value will not expire.
//   - If TTL < 0, it is treated as no expiration (handled internally as 0).
//
// Mode behavior:
//   - NX: Set only if the key does not exist.
//   - XX: Set only if the key already exists.
//   - NX and XX are mutually exclusive (validated internally).
//
// Critical behavior:
//   - Calls r.getSetMode() to resolve NX/XX rules.
//   - Uses SetArgs, which is Redis-native and allows fine-grained control.
func (r *RedisClient) SetWithOptions(ctx context.Context, key string, value interface{}, opts *common.SetOptions) error {
	prefixed := r.keyPrefix + ":" + key

	data, err := r.prepareValue(value)
	if err != nil {
		return err
	}

	cmd := r.client.SetArgs(ctx, prefixed, data, redis.SetArgs{
		TTL:     opts.TTL,
		Mode:    r.getSetMode(opts), // NX, XX, or ""
		KeepTTL: opts.KeepTTL,
	})

	return cmd.Err()
}

// Get retrieves a value from Redis using the provided key and populates the dest.
//
// If dest is a *string or *[]byte, the value is returned directly without unmarshaling.
// For all other types, the value is unmarshaled using the serialize package.
//
// Critical points:
//   - dest must be a non-nil pointer to the target type.
//   - If dest is not *string or *[]byte, the value must be in a format compatible with Unmarshal.
func (r *RedisClient) Get(ctx context.Context, key string, dest interface{}) error {
	if dest == nil {
		return ErrNilDestination
	}

	prefixed := r.keyPrefix + ":" + key

	val, err := r.client.Get(ctx, prefixed).Result()
	if err != nil {
		return err
	}

	switch d := dest.(type) {
	case *string:
		*d = val
		return nil
	case *[]byte:
		*d = []byte(val)
		return nil
	default:
		return serialize.Unmarshal(val, dest)
	}
}

// GetOrSet attempts to retrieve a value from Redis using the specified parameters.
// If the value is not found or deserialization fails, it calls the provided callback,
// stores the result in Redis, and returns it.
//
// Params:
//   - params.CacheKey: the key to retrieve from Redis
//   - params.CacheTTL: TTL in seconds for caching the fresh value
//   - params.Callback: a fallback function that returns a fresh value on cache miss
//
// Critical points:
//   - Always stores the result of Callback in Redis on cache miss
//   - Returns error from Callback or serialization if any step fails
func (r *RedisClient) GetOrSet(params common.GetOrSetParams) (interface{}, error) {
	var out interface{}

	err := r.Get(params.Ctx, params.CacheKey, &out)
	if err == nil {
		return out, nil
	}

	// Fallback to fresh data on cache miss
	fresh, err := params.Callback()
	if err != nil {
		return out, err
	}

	_ = r.Set(params.Ctx, params.CacheKey, fresh)
	return fresh, nil
}

// GetOrSetEx attempts to retrieve a value from Redis using the specified parameters.
// If the value is not found or deserialization fails, it calls the provided callback,
// stores the result in Redis, and returns it.
//
// Params:
//   - params.CacheKey: the key to retrieve from Redis
//   - params.CacheTTL: TTL in seconds for caching the fresh value
//   - params.Callback: a fallback function that returns a fresh value on cache miss
//   - params.Dest: optional pointer to destination type. When provided, always populated with result
//
// Critical points:
//   - If Dest is provided, it's always populated (cache hit or miss), eliminating type assertion
//   - Cache hit: deserializes directly into Dest
//   - Cache miss: executes callback, stores in cache, and copies to Dest if provided
func (r *RedisClient) GetOrSetEx(params common.GetOrSetParams) (interface{}, error) {
	// Try to get from cache using Dest if provided
	if params.Dest != nil {
		err := r.Get(params.Ctx, params.CacheKey, params.Dest)
		if err == nil {
			if params.Metrics != nil {
				params.Metrics.Hit = true
			}
			return params.Dest, nil
		}
	} else {
		// Fallback to generic interface{} if no Dest provided
		var out interface{}
		err := r.Get(params.Ctx, params.CacheKey, &out)
		if err == nil {
			if params.Metrics != nil {
				params.Metrics.Hit = true
			}
			return out, nil
		}
	}

	// Cache miss
	if params.Metrics != nil {
		params.Metrics.Hit = false
	}

	// Execute callback
	fresh, err := params.Callback()
	if err != nil {
		return nil, err
	}

	// Store in cache
	_ = r.SetEx(params.Ctx, params.CacheKey, fresh, time.Duration(params.CacheTTL)*time.Second)

	// If Dest provided, populate it with fresh data
	if params.Dest != nil && fresh != nil {

		// Serialize and deserialize to populate Dest with the correct type
		data, err := r.prepareValue(fresh)
		if err != nil {
			return fresh, nil // Return fresh even if we can't populate Dest
		}
		_ = serialize.Unmarshal(data, params.Dest)
		return params.Dest, nil
	}

	return fresh, nil
}

// Ping checks the Redis connection health by issuing a PING command.
func (r *RedisClient) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Del removes a key from Redis.
//
// The key is automatically prefixed using the RedisClient's keyPrefix.
// If the key does not exist, Redis returns 0 and no error.
//
// Critical behavior:
//   - Uses the same prefixing convention as Set/SetEx/SetWithOptions (keyPrefix + ":" + key).
//   - Returns any error received from the Redis client.
func (r *RedisClient) Del(ctx context.Context, key string) error {
	prefixed := r.keyPrefix + ":" + key
	return r.client.Del(ctx, prefixed).Err()
}
