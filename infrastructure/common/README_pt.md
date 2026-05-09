# common — Contratos compartilhados de infraestrutura

Definições de portas (interfaces) consumidas pelos pacotes `infrastructure/*`. **Nenhuma implementação vive aqui** — apenas tipos, formato de structs e contratos de interface.

> Nota de layout: arquivos ficam em `infrastructure/common/ports/` mas declaram `package common`.

## O que este pacote define

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

Usado pelos adapters de infra para expor um formato uniforme de health-probe.

### Contratos de cache (`ports/cache.ports.go`)

#### Tipos

| Tipo | Propósito |
|---|---|
| `CacheMetrics{ Hit bool }` | Observabilidade opcional; passe `*CacheMetrics` em `GetOrSetParams.Metrics` para receber info de hit/miss. `nil` = zero overhead. |
| `GetOrSetParams` | Parameter object para `GetOrSet`/`GetOrSetEx`. Campos: `Ctx`, `CacheKey`, `CacheTTL` (segundos), `Callback func() (interface{}, error)`, `Dest interface{}`, `Metrics *CacheMetrics`. |
| `SetOptions` | Opções para `SetWithOptions`: `TTL`, `NX`, `XX`, `KeepTTL`, `Tags []string`, `Compression`. |

#### Interfaces

| Interface | Métodos |
|---|---|
| `Cache` | `Set`, `SetEx`, `Get`, `Del` |
| `CacheWithTTL` | `SetEx(ctx, key, value, ttl)` |
| `CacheWithOptions` | `SetWithOptions(ctx, key, value, *SetOptions)` |
| `CacheGetOrSet` | `GetOrSet(GetOrSetParams) (any, error)` — sem TTL |
| `CacheGetOrSetEx` | `GetOrSetEx(GetOrSetParams) (any, error)` — com TTL |
| `AppCache` | `Cache` + `CacheGetOrSetEx` — privado por serviço (ex: Redis DB 0) |
| `SharedCache` | `Cache` — compartilhado entre serviços (ex: Redis DB 5) |

`AppCache` e `SharedCache` são **interfaces distintas**, não aliases — o DIG (container de DI) precisa separá-las para injetar instâncias diferentes de cache.

### Contratos de cache local / tiered

```go
type LocalCacheLoader      func(ctx context.Context, key string) ([]byte, error)
type LocalCacheInvalidator func(ctx context.Context, key string) error
```

| Interface | Métodos |
|---|---|
| `LocalCache` | `Get(ctx, key) ([]byte, int, error)` (tier: 0=L0, 1=L1, 2=L2, -1=miss); `Set`; `Delete` (todas as camadas); `Invalidate` (apenas L0+L1); `Stats() LocalCacheStats` |
| `TieredCache` | `LocalCache` + `GetFromL0`, `GetFromL1`, `Warmup(ctx, keys)` |

### `LocalCacheStats`

Contadores usados pelas implementações de tiered cache:

- L0 (RAM): `L0Hits`, `L0Misses`, `L0Size`
- L1 (Disco): `L1Hits`, `L1Misses`, `L1Size`
- L2 (Remoto): `L2Hits`, `L2Misses`
- Fallback (HTTP): `FallbackHits`, `FallbackMisses`
- `L1LazyExpired` — arquivos removidos durante leituras quando o TTL expirou

## Implementado por

- `infrastructure/redis` — implementa `Cache`, `AppCache`, `SharedCache`
- `infrastructure/tieredcache` — implementa `LocalCache` / `TieredCache`

## Por que as portas vivem aqui

Uma única fonte da verdade para contratos de cache e health, fazendo com que os consumidores dependam de `common` em vez do adapter concreto — é isso que permite ao DI trocar Redis ↔ in-memory ↔ tiered cache sem alterar os call sites.
