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

## Performance: NVMe vs Redis (Why this matters at scale)

At 200M+ assets, the question "why not just use Redis everywhere?" comes up often. The short answer: **at large scale, the network hop to a remote Redis cluster is what dominates latency** — not the cache lookup itself. A local NVMe tier operates in the same latency band as networked Redis, while a hot RAM tier (L0) covers the concurrent-key scenario where Redis would otherwise pull ahead.

### Latency budget (2026)

| Layer | Typical latency | Notes |
|---|---|---|
| DRAM (local) | ~80 ns | Baseline — what L0 (ristretto) hits internally |
| CXL Type-3 memory pool | ~200–500 ns | ~2× DDR5 local; in production on Azure since Nov/2025 |
| **L0 — ristretto (RAM)** | **~50 µs** (TieredCache) | Includes admission policy, sharding, TTL check |
| NVMe Gen5 enterprise (e.g. Micron 7500 MAX) | ~70 µs typical, p99 ~80 µs | 4 KB random read, device-level |
| NVMe-oF (network-attached NVMe) | ~20–30 µs | Close to local NVMe |
| **L1 — local NVMe (TieredCache)** | **~500 µs** | Includes hash-shard path + open + read + decode |
| Redis over Unix socket (same host) | ~30 µs | Only when colocated |
| Redis over 1 GbE | ~200 µs | Network dominates |
| NVMe Gen5 p99 under load | ~0.75 ms | vs 7.5 ms on SATA — "10× lower tail latency" |
| L2 — MinIO/S3 | ~5–50 ms | Cold tier |
| SATA SSD | ~100–200 µs | Reference |
| Spinning disk | ~5–10 ms | Reference |

### Where Redis still wins (and how TieredCache addresses it)

| Redis advantage | TieredCache mitigation |
|---|---|
| ~33% higher concurrent read throughput than NVMe (single-key hotspots) | **L0 (ristretto)** handles hot keys in DRAM at ~50 µs — Redis's concurrent read advantage disappears |
| ~41% higher write throughput (no fsync/flush) | Cache is **read-mostly**; writes go to L2 (MinIO) as source of truth, propagated via NATS FANOUT invalidation |
| Cluster horizontal scaling | TieredCache scales **linearly with service instances** (no shared cache to size) |
| Job queues / pub-sub primitives | Out of scope — TieredCache is a cache, not a broker. NATS handles broker concerns |

### What changed in 2026 (and what didn't)

- **Device-level NVMe latency has not improved meaningfully** since 2019 — random 4 KB reads remain ~20–70 µs. Gen5/Gen6 gains are in **sequential throughput** and **IOPS**, not in random-read latency.
- **Tail latency improved** — modern enterprise NVMe (Micron 7500 MAX, ScaleFlux CSD5310) delivers p99 ~80 µs and "sub-1 ms 6×9" consistency, addressing the historical "NVMe is fast on average but spiky under load" concern.
- **CXL went mainstream** — Azure shipped the first CXL-equipped cloud instances in November 2025. CXL Type-3 memory at ~200–500 ns load-to-use latency creates a new tier between DRAM and NVMe that may eventually slot above L1.
- **Network was already the bottleneck for Redis** — Redis's own docs state: *"Redis throughput is limited by the network well before being limited by the CPU."* That has not changed.

### Honest caveat: what to monitor

NVMe wins on **average**, but **p99/p99.9 under mixed read+write+fsync load** is where it can degrade if the controller, filesystem, or flush behavior aren't dimensioned correctly. Track:

- L1 read p99 latency (alert if > 1 ms sustained)
- Filesystem fsync cost during heavy writes
- Controller thermal throttling on sustained workloads

### References (2025–2026)

- [Hacker News — "Isn't Redis just a lot less relevant these days since enterprise NVMe storage is…" (2026 discussion)](https://news.ycombinator.com/item?id=46616513)
- [OneUptime — How to Estimate Redis Hardware Requirements (Mar/2026)](https://oneuptime.com/blog/post/2026-03-31-redis-estimate-hardware-requirements/view)
- [ServerMall — PCIe Gen4/Gen5 in Servers: Bandwidth, Limits, Bottlenecks in 2026](https://servermall.com/blog/pcie-gen4-gen5-bandwidth-and-bottlenecks/)
- [StorageNewsletter — Validation of PCIe Gen5 NVMe Storage Expansion Adapters (Feb/2026)](https://www.storagenewsletter.com/2026/02/06/highpoint-technologies-validation-of-pcie-gen5-nvme-storage-expansion-adapters-with-scaleflux-csd5310-series-enterprise-ssds/)
- [Newegg Insider — PCIe 5.0 SSDs in 2026: Speed, Performance & Best Buys](https://www.newegg.com/insider/breaking-the-speed-barrier-a-complete-guide-to-pcie-5-0-ssds-in-2026/)
- [Tom's Hardware — Best SSDs 2026](https://www.tomshardware.com/reviews/best-ssds,3891.html)
- [Tech-Insider — SSD vs HDD 2026: 14,500 MB/s vs 285 MB/s (tested)](https://tech-insider.org/ssd-vs-hdd-2026/)
- [ServerMall — CXL in 2026: Server Memory Expansion & Pooling](https://servermall.com/blog/cxl-in-2026-memory-expansion-and-pooling/)
- [KAD — CXL Goes Mainstream: The Memory Fabric Era in 2026](https://www.kad8.com/hardware/cxl-opens-a-new-era-of-memory-expansion/)
- [Colobird — CXL 3.0 Memory Pooling on Dedicated Servers: 2026 Gains](https://www.colobird.com/blogs/cxl-3-memory-pooling-dedicated-servers/)
- [CXL Consortium — Q3 2025 Webinar: How CXL Transforms Server Memory Infrastructure (PDF)](https://computeexpresslink.org/wp-content/uploads/2025/10/CXL_Q3-2025-Webinar_FINAL.pdf)
- [Introl — CXL 4.0 Infrastructure Planning Guide (2025)](https://introl.com/blog/cxl-4-0-infrastructure-planning-guide-memory-pooling-2025)
- [Corewave Labs — Persistent Memory vs RAM (2025): CXL & Post-Optane Guide](https://corewavelabs.com/persistent-memory-vs-ram-cxl/)
- [USENIX OSDI '24 — Managing Memory Tiers with CXL in Virtualized Environments (PDF)](https://www.usenix.org/system/files/osdi24-zhong-yuhong.pdf)
- [simplyblock — What Is NVMe Latency? Performance Benchmarks Explained](https://www.simplyblock.io/glossary/nvme-latency/)
- [Micron 7500 MAX — Enterprise NVMe latency profile (p99 ~80 µs)](https://openmetal.io/resources/blog/micron-max-7500-nvme-enterprise-storage-details-and-performance/)
- [Redis docs — Benchmarks (network as primary bottleneck)](https://redis.io/docs/latest/operate/oss_and_stack/management/optimization/benchmarks/)
- [Redis docs — Diagnosing latency issues](https://redis.io/docs/latest/operate/oss_and_stack/management/optimization/latency/)
- [maxcluster — Application cache benchmark: NVMe SSD vs Redis vs Memcached](https://maxcluster.de/en/blog/2019/09/redis-part-2-application-cache-benchmark)

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
