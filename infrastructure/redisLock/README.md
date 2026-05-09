# redisLock — Distributed locks on Redis (redsync wrapper)

Thin wrapper around [`go-redsync/redsync/v4`](https://github.com/go-redsync/redsync) providing context-aware distributed locking. Implements the [Redlock algorithm](https://redis.io/docs/latest/develop/use/patterns/distributed-locks/) over a single Redis client.

> Package name: `redisLockModel` (directory: `redisLock/`).

## Surface

### Constructor

```go
func New(client *goredislib.Client) *LockManager
```

Builds a `LockManager` backed by the supplied `go-redis/v9` client. The redsync pool is created internally.

### Methods on `*LockManager`

| Method | Purpose |
|---|---|
| `SetLock(ctx, key, ttl) (*redsync.Mutex, error)` | Acquires the lock or returns an error. Caller owns the returned mutex and is responsible for unlocking. |
| `SetUnlock(ctx, mutex) error` | Releases a previously acquired mutex. |
| `SetWithLock(ctx, key, ttl, fn) error` | Acquires → runs `fn` → unlocks (deferred). The unlock error is ignored; only `fn`'s error / acquire error propagates. |

### Constants (`constants.go`)

| Constant | Value |
|---|---|
| `DefaultTries` | `3` |
| `DefaultRetryDelay` | `200 * time.Millisecond` |
| `MinTTL` | `100 * time.Millisecond` |

`SetLock` always passes these to redsync via `WithExpiry(ttl)`, `WithTries(DefaultTries)`, `WithRetryDelay(DefaultRetryDelay)`. They are not currently configurable per call — change in `methods.go` if you need different values.

### Errors (`errors.go`)

| Sentinel | Message |
|---|---|
| `ErrLockAcquire` | `redis: failed to acquire lock` |
| `ErrLockRelease` | `redis: failed to release lock` |
| `ErrTTLTooShort` | `redis: TTL must be at least 100ms` |

`SetLock` wraps redsync errors with `fmt.Errorf("%w: %v", ErrLockAcquire, err)` — use `errors.Is(err, ErrLockAcquire)` to detect acquisition failures.

## Validation

`SetLock` rejects `ttl < MinTTL` (100 ms) before contacting Redis. Tested values: `0`, negative durations, `50ms`, `99ms` all return `ErrTTLTooShort` (see `redislock_test.go`).

## Usage

### Acquire / release manually

```go
lm := redisLockModel.New(redisClient)

mutex, err := lm.SetLock(ctx, "asset:123:edit", 5*time.Second)
if err != nil {
    return err
}
defer lm.SetUnlock(ctx, mutex)

// critical section
```

### Run with auto-unlock

```go
err := lm.SetWithLock(ctx, "asset:123:edit", 5*time.Second, func() error {
    // critical section
    return doWork()
})
```

## Notes

- The unlock inside `SetWithLock` is deferred and its error is silently dropped. If you need to surface unlock failures, use `SetLock` + `SetUnlock` explicitly.
- Lock context is propagated to `mutex.LockContext(ctx)` — cancelling the context aborts the acquisition retry loop.
- Single-instance Redlock: the wrapper uses one `*goredislib.Client`. For true Redlock across N independent Redis nodes you would need to construct redsync with multiple pools directly.
