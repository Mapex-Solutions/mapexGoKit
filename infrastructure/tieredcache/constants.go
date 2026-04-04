package tieredcache

import "time"

// Default L0 (RAM) configuration.
const (
	// DefaultL0MaxSize is the default maximum RAM cache size (256 MB).
	DefaultL0MaxSize = 256 * 1024 * 1024

	// DefaultL0MaxItems is the default maximum number of items in RAM cache.
	DefaultL0MaxItems = 100_000

	// DefaultL0TTL is the default TTL for RAM cache entries (5 minutes).
	DefaultL0TTL = 5 * time.Minute

	// L0BufferItems is the number of items to buffer before eviction.
	L0BufferItems = 64
)

// Default L1 (Disk) configuration.
const (
	// DefaultL1MaxSize is the default maximum disk cache size (10 GB).
	// For 200M+ assets scenario, 10GB allows caching ~4M assets of 2.5KB each.
	DefaultL1MaxSize = 10 * 1024 * 1024 * 1024

	// DefaultL1TTL is the default TTL for disk cache entries (1 hour).
	DefaultL1TTL = 1 * time.Hour

	// DefaultL1Dir is the default directory for disk cache.
	DefaultL1Dir = "/tmp/mapexos-cache"

	// L1FileExtension is the file extension for cached items.
	L1FileExtension = ".cache"

	// L1MetaExtension is the file extension for cache metadata.
	L1MetaExtension = ".meta"

	// L1SubdirDepth is the number of subdirectory levels for L1 cache.
	// Depth 2 = 65,536 directories (256 × 256), ~3K files per dir for 200M assets.
	L1SubdirDepth = 2

	// Note: Background cleanup constants removed.
	// Cleanup is now handled by external script (cache-cleanup.sh) using Linux atime.
)

// Ristretto tuning parameters.
const (
	// RistrettoNumCounters is the number of counters for admission policy.
	// Recommended: 10x the number of max items.
	RistrettoNumCounters = 1_000_000
)
