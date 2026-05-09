# redis — Mapex Redis client (`redisModel`)

Wrapper around [`redis/go-redis/v9`](https://github.com/redis/go-redis) implementing the cache contracts defined in `infrastructure/common/ports`. Adds:

- Key prefixing on every command (`<keyPrefix>:<key>`)
- Marshal / unmarshal via `utils/serialize` for non-string types
- `GetOrSet` / `GetOrSetEx` cache-aside helpers (with optional `Dest` typed result and `Metrics` hit indicator)
- Collection helpers: sorted sets, hashes, sets, atomic pipelines
- Time-score conversion utilities for sorted-set timestamps

> Package name: `redisModel` (directory: `redis/`).

## Surface

### Configuration

```go
type Config struct {
    Host      string
    Port      int
    Username  string
    Password  string
    DB        int
    KeyPrefix string
}
```

### Constructors

| Function | Behaviour |
|---|---|
| `New(cfg) (*RedisClient, error)` | Builds the wrapper, pings with a **2 s** timeout, returns error on failure. Logs `[INFRA:REDIS] Initialized`. |
| `NewGoRedisClient(cfg) *redis.Client` | Returns the raw `*redis.Client` (no wrapper). Pings with 2 s timeout and **calls `logger.Panic` on failure** — used by components that need direct access (e.g. `redisLock`). |

### Constants (`constants.go`)

| Name | Value |
|---|---|
| `DefaultTTLSeconds` | `300` |
| `NoExpiration` | `0` |

### Errors (`errors.go`)

| Sentinel | Message |
|---|---|
| `ErrKeyNotFound` | `key not found in cache` |
| `ErrNilDestination` | `redis: destination is nil` |

`Get` raises `ErrNilDestination` when called with `dest == nil`. Misses surface as the underlying `redis.Nil` error — check with `errors.Is(err, redis.Nil)`.

### Implements

```go
var (
    _ common.Cache            = (*RedisClient)(nil)
    _ common.CacheWithTTL     = (*RedisClient)(nil)
    _ common.CacheWithOptions = (*RedisClient)(nil)
    _ common.CacheGetOrSet    = (*RedisClient)(nil)
    _ common.CacheGetOrSetEx  = (*RedisClient)(nil)
)
```

These compile-time assertions live at the top of `methods.go`.

## Key/value operations (`methods.go`)

Every method prefixes `key` with `cfg.KeyPrefix + ":"`.

| Method | Notes |
|---|---|
| `Set(ctx, key, value)` | No TTL (`NoExpiration`). |
| `SetEx(ctx, key, value, ttl)` | Redis errors when `ttl <= 0`. |
| `SetWithOptions(ctx, key, value, *common.SetOptions)` | Uses `redis.SetArgs{TTL, Mode, KeepTTL}`. Mode = `NX` / `XX` / empty (resolved by `getSetMode`). |
| `Get(ctx, key, dest)` | `*string` and `*[]byte` short-circuit (no unmarshal); other types via `serialize.Unmarshal`. Returns `ErrNilDestination` if `dest == nil`. |
| `GetOrSet(GetOrSetParams) (any, error)` | Cache-aside without TTL. On miss runs `Callback`, stores via `Set`. Cache-store error is **swallowed** and returns the fresh value. |
| `GetOrSetEx(GetOrSetParams) (any, error)` | Cache-aside with `SetEx(ttl = CacheTTL seconds)`. Honors `params.Dest` (typed destination) and `params.Metrics.Hit`. |
| `Ping(ctx) error` | PING. |
| `Del(ctx, key) error` | Same prefix convention as `Set`. |

### `prepareValue` (internal)

`string` → as-is, `[]byte` → string-cast, anything else → `serialize.Marshal`.

### `GetOrSet` vs `GetOrSetEx`

| | `GetOrSet` | `GetOrSetEx` |
|---|---|---|
| TTL on cache miss | none (`Set`) | `time.Duration(CacheTTL) * time.Second` (`SetEx`) |
| `Dest` populated | no | yes — re-marshals fresh value and unmarshals into `Dest` |
| `Metrics.Hit` populated | no | yes (true on hit, false on miss) |

## Collection operations (`methods_collections.go`)

Every method prefixes the key with `<keyPrefix>:` and returns the underlying `go-redis` error verbatim.

### Sorted sets

| Method | Purpose |
|---|---|
| `ZAdd(ctx, key, score, member)` | Add/update a member. |
| `ZScore(ctx, key, member) (float64, error)` | `redis.Nil` if missing. |
| `ZMScore(ctx, key, members...) ([]float64, error)` | Single round-trip; missing members return `NaN`. |
| `ZRangeByScore(ctx, key, min, max, offset, count) ([]string, error)` | Paginated range query. |
| `ZRem(ctx, key, member)` | Remove a member. |

### Hashes

| Method | Purpose |
|---|---|
| `HIncrBy(ctx, key, field, incr) (int64, error)` | Atomic increment, returns new value. |
| `HDel(ctx, key, field)` | Remove a field. |
| `HSet(ctx, key, field, value)` | String value. |
| `HSetInt64(ctx, key, field, value)` | Convenience for unix timestamps etc. |
| `HGet(ctx, key, field) (string, error)` | `redis.Nil` if missing. |
| `HGetInt64(ctx, key, field) (int64, error)` | `redis.Nil` if missing; parses via go-redis `Int64()`. |

### Sets

| Method | Purpose |
|---|---|
| `SAdd(ctx, key, member)` / `SRem(ctx, key, member)` | Add / remove. |
| `SRemN(ctx, key, member) (int64, error)` | Race-free transitions: only the caller that gets `n=1` actually removed. |
| `SIsMember(ctx, key, member) (bool, error)` | Single-membership check. |
| `SMIsMember(ctx, key, members...) ([]bool, error)` | Multi-membership in one round-trip. |
| `SMembers(ctx, key) ([]string, error)` | All members. |

### Pipelines

| Method | Purpose |
|---|---|
| `PipelineRemoveFromCollections(ctx, zsetKey, hashKey, setKey, member)` | Atomic `ZRem` + `HDel` + `SRem` for the same `member`. |

### Time/score helpers (package-level functions)

| Function | Purpose |
|---|---|
| `ScoreToTime(score) *time.Time` | Returns `nil` when score is `0` or `NaN`. |
| `TimeToScore(t) float64` | `float64(t.Unix())`. |
| `ScoresToTimeMap(members, scores) map[string]*time.Time` | Skips members with score 0/NaN. |
| `BoolSliceToMap(members, flags) map[string]bool` | Pairs SMIsMember output with member names. |
| `FormatCutoff(t) string` | `fmt.Sprintf("%d", t.Unix())` — for `ZRangeByScore` boundaries. |

## Usage

### Cache-aside with typed destination

```go
client, err := redisModel.New(redisModel.Config{
    Host: "localhost", Port: 6379, KeyPrefix: "mapex",
})
if err != nil { return err }
defer client.Close()

var asset Asset
metrics := common.CacheMetrics{}
res, err := client.GetOrSetEx(common.GetOrSetParams{
    Ctx:      ctx,
    CacheKey: "asset:" + uuid,
    CacheTTL: 300,
    Dest:     &asset,
    Metrics:  &metrics,
    Callback: func() (any, error) { return loadFromDB(uuid) },
})
if err != nil { return err }
if metrics.Hit {
    // served from Redis
}
_ = res // res points to &asset when Dest is set
```

### Sorted-set time index

```go
err := client.ZAdd(ctx, "device:lastseen", redisModel.TimeToScore(now), assetUUID)
ts, err := client.ZScore(ctx, "device:lastseen", assetUUID)
recent := redisModel.ScoreToTime(ts) // nil if not present
```

### Race-free state transition

```go
n, err := client.SRemN(ctx, "alerted", assetUUID)
if err != nil { return err }
if n == 1 {
    // exactly this caller cleared the alert
}
```
