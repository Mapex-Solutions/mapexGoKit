package common

import (
	"context"
	"time"
)

/**
* CACHE TYPE
 */

// CacheMetrics provides optional observability data from cache operations.
// Pass a non-nil pointer in GetOrSetParams.Metrics to receive hit/miss info.
// When nil, the operation behaves identically to before (zero overhead).
type CacheMetrics struct {
	Hit bool // true if value was served from cache, false if callback was invoked
}

type GetOrSetParams struct {
	Ctx      context.Context
	CacheKey string
	CacheTTL int // seconds (para Ex)
	Callback func() (interface{}, error)
	Dest     interface{}   // Pointer to the destination type for unmarshaling (optional, only for GetOrSetEx)
	Metrics  *CacheMetrics // Optional: populated with hit/miss info when non-nil
}

type SetOptions struct {
	TTL         time.Duration
	NX, XX      bool
	KeepTTL     bool
	Tags        []string
	Compression bool
}

/**
 * INTERFACES INPLEMENTED BY THE CACHE MANAGER / OTHERS
 */
type Cache interface {
	Set(ctx context.Context, key string, value interface{}) error
	SetEx(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Get(ctx context.Context, key string, dest interface{}) error
	Del(ctx context.Context, key string) error
}

type CacheWithTTL interface {
	SetEx(ctx context.Context, key string, value any, ttl time.Duration) error
}

type CacheWithOptions interface {
	SetWithOptions(ctx context.Context, key string, value any, opts *SetOptions) error
}

type CacheGetOrSet interface {
	// GetOrSet fetches a cached value or computes it using Callback and stores it without TTL.
	GetOrSet(params GetOrSetParams) (any, error)
}

type CacheGetOrSetEx interface {
	// GetOrSetEx fetches a cached value or computes it using Callback and stores it with TTL (seconds).
	GetOrSetEx(params GetOrSetParams) (any, error)
}

// Specialized cache types for dependency injection.
// These are DISTINCT interfaces (not aliases) so DIG can differentiate them.
// AppCache is used for service-private Redis data (each service has its own DB 0).
// SharedCache is used for cross-service shared data (e.g., authorization cache DB 5).
type AppCache interface {
	Cache
	CacheGetOrSetEx
}

type SharedCache interface {
	Cache
}

/**
 * LOCAL CACHE INTERFACES (In-memory L0 + Disk L1)
 *
 * LocalCache provides tiered caching with L0 (RAM) and L1 (NVMe/Disk).
 * Used for high-performance local caching with optional L2 (MinIO) fallback.
 *
 * Architecture:
 *   L0 (RAM)  → ristretto/LRU cache (microseconds latency)
 *   L1 (Disk) → NVMe/SSD storage (sub-millisecond latency)
 *   L2 (S3)   → MinIO object storage (network latency, source of truth)
 *
 * Cache invalidation is handled via NATS fanout messages.
 */

// LocalCacheLoader is a function that loads data from L2 (source of truth).
// Used when data is not found in L0 or L1.
type LocalCacheLoader func(ctx context.Context, key string) ([]byte, error)

// LocalCacheInvalidator is a function called when cache is invalidated.
// Used to propagate invalidation to other tiers.
type LocalCacheInvalidator func(ctx context.Context, key string) error

// LocalCache provides tiered local caching (L0 RAM + L1 Disk).
// Implements Cache interface for compatibility with existing code.
type LocalCache interface {
	// Get retrieves a value from cache (L0 → L1 → L2).
	// Returns data and cache tier hit (0=L0, 1=L1, 2=L2, -1=miss).
	Get(ctx context.Context, key string) ([]byte, int, error)

	// Set stores a value in cache (L0 and L1).
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes a value from all cache tiers.
	Delete(ctx context.Context, key string) error

	// Invalidate removes a value from local cache only (L0 + L1).
	// Does not affect L2 (source of truth).
	Invalidate(ctx context.Context, key string) error

	// Stats returns cache statistics.
	Stats() LocalCacheStats
}

// LocalCacheStats contains statistics about cache usage.
type LocalCacheStats struct {
	// L0 (RAM) statistics
	L0Hits   uint64
	L0Misses uint64
	L0Size   int64 // bytes used

	// L1 (Disk) statistics
	L1Hits   uint64
	L1Misses uint64
	L1Size   int64 // bytes used

	// L2 (Remote) statistics
	L2Hits   uint64
	L2Misses uint64

	// Fallback (HTTP) statistics
	FallbackHits   uint64 // HTTP fallback hits when L2 misses
	FallbackMisses uint64 // HTTP fallback misses

	// Lazy cleanup statistics (on-read expiration)
	L1LazyExpired uint64 // Files removed during read (TTL expired)
}

// TieredCache extends LocalCache with explicit tier control.
type TieredCache interface {
	LocalCache

	// GetFromL0 retrieves directly from RAM cache.
	GetFromL0(key string) ([]byte, bool)

	// GetFromL1 retrieves directly from disk cache.
	GetFromL1(ctx context.Context, key string) ([]byte, error)

	// Warmup preloads keys into L0/L1 from L2.
	Warmup(ctx context.Context, keys []string) error
}
