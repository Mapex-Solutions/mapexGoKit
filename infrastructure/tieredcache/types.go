package tieredcache

import (
	"sync/atomic"
	"time"

	"github.com/dgraph-io/ristretto"
	common "github.com/Mapex-Solutions/mapexGoKit/infrastructure/common/ports"
	"github.com/Mapex-Solutions/mapexGoKit/infrastructure/httpclient"
)

// TieredCacheClient implements a multi-tier cache system.
//
// Architecture:
//   - L0 (RAM): ristretto in-memory cache (~50µs latency)
//   - L1 (Disk): Local NVMe/SSD storage (~500µs latency)
//   - L2 (Remote): MinIO/S3 source of truth (~5-50ms latency)
//   - Fallback (HTTP): API call to source service when L2 misses (~10-100ms latency)
//
// Cache invalidation is handled via NATS fanout - when any instance
// invalidates a key, all instances receive the invalidation message.
type TieredCacheClient struct {
	// L0 - In-memory cache (ristretto)
	l0Cache *ristretto.Cache

	// L1 - Disk cache directory
	l1Dir string

	// L2 - Remote loader function (MinIO)
	l2Loader common.LocalCacheLoader

	// Fallback - HTTP client for L2 miss recovery
	fallbackClient   *httpclient.HTTPClient
	fallbackEndpoint string

	// Configuration
	config Config

	// Statistics (atomic for thread safety)
	stats CacheStats

	// Key prefix for all cache operations
	keyPrefix string

	// Note: Background cleanup removed - use external script (cache-cleanup.sh)
	// Cleanup is now:
	//   1. Lazy: removes expired files on read
	//   2. External: cron script removes idle files based on atime
}

// Config holds the TieredCache configuration.
type Config struct {
	// L0 (RAM) Configuration
	EnableL0     bool          // Enable L0 RAM cache (default: true)
	L0MaxSize    int64         // Maximum size in bytes for L0 cache
	L0MaxItems   int64         // Maximum number of items in L0
	L0DefaultTTL time.Duration // Default TTL for L0 entries

	// L1 (Disk) Configuration
	EnableL1     bool          // Enable L1 disk cache (default: true)
	L1Dir        string        // Directory for L1 disk cache
	L1MaxSize    int64         // Maximum size in bytes for L1 cache
	L1DefaultTTL time.Duration // Default TTL for L1 entries (0 = no TTL)

	// L2 (Remote) Configuration
	EnableL2 bool                    // Enable L2 remote loader (MinIO/S3)
	L2Loader common.LocalCacheLoader // Loader function for L2 (required if EnableL2 is true)

	// Fallback Configuration (optional)
	// Used when L2 (MinIO) misses - calls HTTP API to fetch from source
	FallbackBaseURL  string        // Base URL of source service (e.g., "http://assets-service:5001")
	FallbackAPIKey   string        // API Key for authentication
	FallbackEndpoint string        // Endpoint path (e.g., "/internal/assets")
	FallbackTimeout  time.Duration // Request timeout (default: 5 seconds)

	// General Configuration
	KeyPrefix     string // Prefix for all cache keys
	EnableMetrics bool   // Enable detailed metrics collection
}

// CacheStats holds atomic cache statistics.
type CacheStats struct {
	L0Hits   atomic.Uint64
	L0Misses atomic.Uint64
	L0Size   atomic.Int64

	L1Hits   atomic.Uint64
	L1Misses atomic.Uint64
	L1Size   atomic.Int64

	L2Hits   atomic.Uint64
	L2Misses atomic.Uint64

	FallbackHits   atomic.Uint64 // HTTP fallback hits
	FallbackMisses atomic.Uint64 // HTTP fallback misses

	// Lazy cleanup statistics (on-read expiration removal)
	L1LazyExpired atomic.Uint64 // Files removed during read (TTL expired)
}

// CacheEntry represents a cached item with metadata.
type CacheEntry struct {
	Data      []byte    `json:"data"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	Size      int64     `json:"size"`
}

// CacheTier represents which cache tier served the data.
type CacheTier int

const (
	// TierMiss indicates no cache hit
	TierMiss CacheTier = -1
	// TierL0 indicates hit from RAM cache
	TierL0 CacheTier = 0
	// TierL1 indicates hit from disk cache
	TierL1 CacheTier = 1
	// TierL2 indicates hit from remote storage (MinIO)
	TierL2 CacheTier = 2
	// TierFallback indicates hit from HTTP fallback API
	TierFallback CacheTier = 3
)

// String returns a human-readable tier name.
func (t CacheTier) String() string {
	switch t {
	case TierL0:
		return "L0-RAM"
	case TierL1:
		return "L1-Disk"
	case TierL2:
		return "L2-Remote"
	case TierFallback:
		return "Fallback-HTTP"
	default:
		return "MISS"
	}
}
