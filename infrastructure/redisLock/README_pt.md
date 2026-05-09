# redisLock — Locks distribuídos no Redis (wrapper do redsync)

Wrapper enxuto sobre [`go-redsync/redsync/v4`](https://github.com/go-redsync/redsync) provendo locking distribuído com suporte a `context`. Implementa o [algoritmo Redlock](https://redis.io/docs/latest/develop/use/patterns/distributed-locks/) sobre um único cliente Redis.

> Nome do pacote: `redisLockModel` (diretório: `redisLock/`).

## Superfície

### Construtor

```go
func New(client *goredislib.Client) *LockManager
```

Constrói um `LockManager` apoiado pelo cliente `go-redis/v9` informado. O pool do redsync é criado internamente.

### Métodos em `*LockManager`

| Método | Propósito |
|---|---|
| `SetLock(ctx, key, ttl) (*redsync.Mutex, error)` | Adquire o lock ou retorna erro. O caller possui o mutex retornado e é responsável por liberá-lo. |
| `SetUnlock(ctx, mutex) error` | Libera um mutex previamente adquirido. |
| `SetWithLock(ctx, key, ttl, fn) error` | Adquire → executa `fn` → libera (via defer). O erro de unlock é ignorado; apenas o erro de `fn` ou de aquisição é propagado. |

### Constantes (`constants.go`)

| Constante | Valor |
|---|---|
| `DefaultTries` | `3` |
| `DefaultRetryDelay` | `200 * time.Millisecond` |
| `MinTTL` | `100 * time.Millisecond` |

`SetLock` sempre repassa essas constantes ao redsync via `WithExpiry(ttl)`, `WithTries(DefaultTries)`, `WithRetryDelay(DefaultRetryDelay)`. Atualmente não são configuráveis por chamada — altere `methods.go` se precisar de valores diferentes.

### Erros (`errors.go`)

| Sentinel | Mensagem |
|---|---|
| `ErrLockAcquire` | `redis: failed to acquire lock` |
| `ErrLockRelease` | `redis: failed to release lock` |
| `ErrTTLTooShort` | `redis: TTL must be at least 100ms` |

`SetLock` envolve erros do redsync com `fmt.Errorf("%w: %v", ErrLockAcquire, err)` — use `errors.Is(err, ErrLockAcquire)` para detectar falhas de aquisição.

## Validação

`SetLock` rejeita `ttl < MinTTL` (100 ms) antes de contatar o Redis. Valores testados: `0`, durações negativas, `50ms`, `99ms` — todos retornam `ErrTTLTooShort` (ver `redislock_test.go`).

## Uso

### Adquirir / liberar manualmente

```go
lm := redisLockModel.New(redisClient)

mutex, err := lm.SetLock(ctx, "asset:123:edit", 5*time.Second)
if err != nil {
    return err
}
defer lm.SetUnlock(ctx, mutex)

// seção crítica
```

### Executar com auto-unlock

```go
err := lm.SetWithLock(ctx, "asset:123:edit", 5*time.Second, func() error {
    // seção crítica
    return doWork()
})
```

## Notas

- O unlock dentro de `SetWithLock` é via defer e seu erro é silenciosamente descartado. Se precisar tratar falhas de unlock, use `SetLock` + `SetUnlock` explicitamente.
- O contexto do lock é propagado para `mutex.LockContext(ctx)` — cancelar o contexto aborta o loop de retentativa de aquisição.
- Redlock de instância única: o wrapper usa um único `*goredislib.Client`. Para Redlock real entre N nós Redis independentes seria necessário construir redsync com múltiplos pools diretamente.
