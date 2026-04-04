# TieredCache - Multi-Tier Caching for MapexOS

## Overview

TieredCache provides a high-performance, multi-tier caching solution for MapexOS services. It implements a four-tier architecture designed for scenarios with 200M+ assets.

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                         TieredCache Architecture                              │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                               │
│   L0 (RAM)        L1 (NVMe)         L2 (MinIO)         Fallback (HTTP)       │
│   ┌─────────┐     ┌─────────┐       ┌─────────┐        ┌─────────┐           │
│   │ristretto│ ──▶ │  Disk   │  ──▶  │   S3    │  ──▶   │  API    │           │
│   │  256MB  │     │   1GB   │       │ (Truth) │        │(MongoDB)│           │
│   └─────────┘     └─────────┘       └─────────┘        └─────────┘           │
│      ~50µs          ~500µs           ~5-50ms            ~10-100ms            │
│                                                                               │
│   Hot data        Warm data         Cold data          Source Recovery       │
│   (frequently     (recent           (source of         (repopulates L2       │
│    accessed)       access)           truth)             on miss)             │
│                                                                               │
└──────────────────────────────────────────────────────────────────────────────┘
```

## Features

### L0 (RAM Cache)
- Ultra-fast in-memory cache using [ristretto](https://github.com/dgraph-io/ristretto)
- LRU eviction with frequency-based admission
- TTL support with automatic expiration
- Thread-safe concurrent access
- Latency: **~50µs**

### L1 (Disk Cache)
- Local NVMe/SSD storage
- Hash-based sharding for filesystem scalability (65,536 directories)
- Metadata with expiration tracking
- Lazy cleanup of expired entries on read
- Latency: **~500µs**

### L2 (Remote Storage)
- MinIO/S3 object storage
- Configurable loader function
- Source of truth for all data
- Automatic promotion to L0/L1 on access
- Latency: **~5-50ms**

### Fallback (HTTP API)
- Called when L2 cache misses
- Fetches from source service (MongoDB)
- Automatically repopulates L2 cache
- Latency: **~10-100ms**

## Installation

The package is part of MapexOS infrastructure:

```go
import "github.com/Mapex-Solutions/MapexOS/infrastructure/tieredcache"
```

## Usage

### Basic Usage

```go
// Create cache with L0 + L1 + L2
cache, err := tieredcache.New(tieredcache.Config{
    EnableL0:     true,
    L0MaxSize:    256 * 1024 * 1024, // 256MB RAM
    L0DefaultTTL: 5 * time.Minute,

    EnableL1:     true,
    L1Dir:        "/var/cache/mapexos/assets",
    L1MaxSize:    1 * 1024 * 1024 * 1024, // 1GB
    L1DefaultTTL: 1 * time.Hour,

    // L2 loader (MinIO)
    EnableL2: true,
    L2Loader: func(ctx context.Context, key string) ([]byte, error) {
        result, err := minioClient.Get(ctx, key)
        if err != nil {
            return nil, err
        }
        return result.Data, nil
    },
})
if err != nil {
    log.Fatal(err)
}
defer cache.Close()

// Get data (automatically checks L0 → L1 → L2 → Fallback)
data, tier, err := cache.Get(ctx, "68f5bbce1aef22967c3ebb30/asset-uuid-123")
if err != nil {
    // Handle cache miss
}
fmt.Printf("Data from tier: %s\n", tieredcache.CacheTier(tier).String())
```

### With HTTP Fallback

```go
cache, err := tieredcache.New(tieredcache.Config{
    EnableL0:     true,
    L0MaxSize:    256 * 1024 * 1024,

    EnableL1:     true,
    L1Dir:        "/var/cache/mapexos/assets",

    // Fallback configuration - calls source service when L2 misses
    FallbackBaseURL:  "http://assets-service:5001",
    FallbackAPIKey:   "internal-api-key",
    FallbackEndpoint: "/internal/assets",
    FallbackTimeout:  5 * time.Second,
})
```

When L2 misses, the cache automatically calls:
```
GET {FallbackBaseURL}{FallbackEndpoint}/{resourceId}
```

**Note:** The cache key format is `{orgId}/{resourceId}`. The fallback extracts only the `resourceId` for the API call.

## Key Format and Tenant Isolation

TieredCache uses a key format that supports multi-tenant isolation:

```
{orgId}/{resourceId}
```

### Examples

| Resource | Key Format | Example |
|----------|------------|---------|
| Asset | `{asset.orgId}/{assetUUID}` | `68f5bbce.../aav9bpg0v0qg00boitc0` |
| Template | `{templateOrgId}/{templateId}` | `mapexos_public/691bb4071e717d77a2430b46` |
| Bytecode | `{templateOrgId}/{templateId}/{scriptType}` | `mapexos_public/691bb.../VALIDATION` |

### System Templates (Shared Cache)

For system templates (`IsSystem=true`), the `templateOrgId` is `mapexos_public`. This allows **all organizations** to share the same cache entry:

```
Organization A uses template X → cache key: mapexos_public/templateX/VALIDATION
Organization B uses template X → cache key: mapexos_public/templateX/VALIDATION  ← SAME!
```

**Result:** 1 million devices using the same system template = **1 cache entry** (not 1 million).

## L1 Disk Structure

L1 uses hash-based sharding to handle 200M+ assets without filesystem issues:

```go
// Key: "mapexos_public/691bb.../VALIDATION"
// SHA256 hash: "a1b2c3d4e5f67890..."
// File path: /cache/a1/b2/a1b2c3d4e5f67890.cache
```

This creates 65,536 directories (256 × 256), keeping ~3K files per directory.

**Important:** The full key is hashed, so same key = same hash = same file. No duplication.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `EnableL0` | `bool` | `false` | Enable L0 RAM cache |
| `L0MaxSize` | `int64` | 256MB | Maximum RAM cache size |
| `L0MaxItems` | `int64` | 100,000 | Maximum items in RAM |
| `L0DefaultTTL` | `Duration` | 5min | Default TTL for L0 entries |
| `EnableL1` | `bool` | `false` | Enable L1 disk cache |
| `L1Dir` | `string` | `/tmp/mapexos-cache` | L1 cache directory |
| `L1MaxSize` | `int64` | 1GB | Maximum disk cache size |
| `L1DefaultTTL` | `Duration` | 1hour | Default TTL for L1 entries |
| `KeyPrefix` | `string` | `""` | Prefix for all cache keys |
| `EnableMetrics` | `bool` | `false` | Enable detailed metrics |
| `EnableL2` | `bool` | `false` | Enable L2 remote loader (MinIO/S3) |
| `L2Loader` | `LocalCacheLoader` | `nil` | Loader function for L2 (required if EnableL2) |
| `FallbackBaseURL` | `string` | `""` | Base URL for fallback API |
| `FallbackAPIKey` | `string` | `""` | API Key for fallback authentication |
| `FallbackEndpoint` | `string` | `""` | Endpoint path for fallback |
| `FallbackTimeout` | `Duration` | 5s | Timeout for fallback requests |

## Cache Invalidation

Cache invalidation is handled via NATS FANOUT messages. When any instance invalidates a key, all instances receive the message:

```go
// Invalidation message format
type InvalidateMessage struct {
    OrgId      string `json:"orgId"`
    ResourceId string `json:"resourceId"`
}

// Publisher (owner service)
natsBus.Publish("mapexos.cache.invalidate.assets", InvalidateMessage{
    OrgId:      "68f5bbce1aef22967c3ebb30",
    ResourceId: "aav9bpg0v0qg00boitc0",
})

// Subscriber (all instances - FANOUT consumer)
natsBus.SubscribeFanout("mapexos.cache.invalidate.assets", func(msg InvalidateMessage) {
    cacheKey := fmt.Sprintf("%s/%s", msg.OrgId, msg.ResourceId)
    cache.Invalidate(cacheKey)
})
```

## Statistics

```go
stats := cache.Stats()
fmt.Printf("L0 Hits: %d, Misses: %d\n", stats.L0Hits, stats.L0Misses)
fmt.Printf("L1 Hits: %d, Misses: %d\n", stats.L1Hits, stats.L1Misses)
fmt.Printf("L2 Hits: %d, Misses: %d\n", stats.L2Hits, stats.L2Misses)
fmt.Printf("Fallback Hits: %d, Misses: %d\n", stats.FallbackHits, stats.FallbackMisses)
fmt.Printf("L1 Lazy Expired: %d\n", stats.L1LazyExpired)
```

## Cache Tier Flow

```
Get(key) called
    │
    ▼
┌─────────┐  hit
│   L0    │ ────────▶ return data (tier: L0-RAM)
│  (RAM)  │
└────┬────┘
     │ miss
     ▼
┌─────────┐  hit
│   L1    │ ────────▶ promote to L0, return data (tier: L1-Disk)
│ (Disk)  │
└────┬────┘
     │ miss
     ▼
┌─────────┐  hit
│   L2    │ ────────▶ promote to L0/L1, return data (tier: L2-Remote)
│ (MinIO) │
└────┬────┘
     │ miss
     ▼
┌──────────┐  hit
│ Fallback │ ────────▶ L2 repopulated by source service,
│  (HTTP)  │           promote to L0/L1, return data (tier: Fallback-HTTP)
└────┬─────┘
     │ miss
     ▼
  return ErrCacheMiss
```

## Best Practices

1. **Size L0 appropriately**: Hot data should fit in RAM. Monitor L0 hit rate.
2. **Use NVMe for L1**: SSD storage provides sub-millisecond latency.
3. **Set appropriate TTLs**: Balance freshness vs performance.
4. **Configure Fallback**: Ensures data recovery when L2 misses.
5. **Use consistent key format**: `{orgId}/{resourceId}` for tenant isolation.
6. **Share system resources**: Use `mapexos_public` orgId for shared templates.
7. **Monitor stats**: Track hit rates to tune configuration.

## Architecture Decision Records

### Why not Redis SharedCache?

For 200M+ assets, Redis SharedCache has limitations:
- Network latency for every access
- Memory cost for all instances
- Single point of failure

TieredCache with local L0/L1 provides:
- Sub-microsecond L0 access
- No network hop for cached data
- Linear scaling with instances

### Why ristretto?

- Admission policy prevents cache pollution
- Better hit rate than simple LRU
- Thread-safe with minimal contention
- Built-in TTL support

### Why hash-based L1 sharding?

- Filesystem performance degrades with 100K+ files per directory
- Hash sharding creates 65,536 directories
- Even distribution regardless of key patterns
- Same key always maps to same file (no duplication)

### Why HTTP Fallback?

- L2 (MinIO) may have stale data or be temporarily unavailable
- Fallback ensures data recovery from source (MongoDB)
- Source service repopulates L2 automatically
- Self-healing cache architecture
