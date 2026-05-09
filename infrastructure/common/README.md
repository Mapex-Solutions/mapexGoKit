# common — Shared infrastructure contracts

Port (interface) definitions consumed across `infrastructure/*` packages. **No implementations live here** — only types, struct shapes, and interface contracts.

> Directory layout note: files live under `infrastructure/common/ports/` but declare `package common`.

## What this package defines

### Health (`ports/health.ports.go`)

```go
type HealthStatus struct {
    Connected    bool      `json:"connected"`
    Service      string    `json:"service"`
    LatencyMs    int64     `json:"latencyMs,omitempty"`
    LastCheckAt  time.Time `json:"lastCheckAt"`
    ErrorMessage string    `json:"errorMessage,omitempty"`
}
```

Used by infra adapters to expose a uniform health-probe shape.

### Cache contracts (`ports/cache.ports.go`)

#### Types

| Type | Purpose |
|---|---|
| `CacheMetrics{ Hit bool }` | Optional observability; pass `*CacheMetrics` in `GetOrSetParams.Metrics` to receive hit/miss info. `nil` = zero overhead. |
| `GetOrSetParams` | Parameter object for `GetOrSet`/`GetOrSetEx`. Fields: `Ctx`, `CacheKey`, `CacheTTL` (seconds), `Callback func() (interface{}, error)`, `Dest interface{}`, `Metrics *CacheMetrics`. |
| `SetOptions` | Options for `SetWithOptions`: `TTL`, `NX`, `XX`, `KeepTTL`, `Tags []string`, `Compression`. |

#### Interfaces

| Interface | Methods |
|---|---|
| `Cache` | `Set`, `SetEx`, `Get`, `Del` |
| `CacheWithTTL` | `SetEx(ctx, key, value, ttl)` |
| `CacheWithOptions` | `SetWithOptions(ctx, key, value, *SetOptions)` |
| `CacheGetOrSet` | `GetOrSet(GetOrSetParams) (any, error)` — no TTL |
| `CacheGetOrSetEx` | `GetOrSetEx(GetOrSetParams) (any, error)` — with TTL |
| `AppCache` | `Cache` + `CacheGetOrSetEx` — service-private (e.g. Redis DB 0) |
| `SharedCache` | `Cache` — cross-service shared (e.g. Redis DB 5) |

`AppCache` and `SharedCache` are **distinct interfaces**, not aliases — DIG (the DI container) needs them separable to inject different cache instances.

### Local / tiered cache contracts

```go
type LocalCacheLoader      func(ctx context.Context, key string) ([]byte, error)
type LocalCacheInvalidator func(ctx context.Context, key string) error
```

| Interface | Methods |
|---|---|
| `LocalCache` | `Get(ctx, key) ([]byte, int, error)` (tier: 0=L0, 1=L1, 2=L2, -1=miss); `Set`; `Delete` (all tiers); `Invalidate` (L0+L1 only); `Stats() LocalCacheStats` |
| `TieredCache` | `LocalCache` + `GetFromL0`, `GetFromL1`, `Warmup(ctx, keys)` |

### `LocalCacheStats`

Counters used by tiered cache implementations:

- L0 (RAM): `L0Hits`, `L0Misses`, `L0Size`
- L1 (Disk): `L1Hits`, `L1Misses`, `L1Size`
- L2 (Remote): `L2Hits`, `L2Misses`
- Fallback (HTTP): `FallbackHits`, `FallbackMisses`
- `L1LazyExpired` — files removed during reads when TTL expired

## Implemented by

- `infrastructure/redis` — implements `Cache`, `AppCache`, `SharedCache`
- `infrastructure/tieredcache` — implements `LocalCache` / `TieredCache`

## Why ports live here

A single source of truth for cache and health contracts so that consumers depend on `common` rather than the concrete adapter — this is what lets DI swap Redis ↔ in-memory ↔ tiered cache without changing call sites.
