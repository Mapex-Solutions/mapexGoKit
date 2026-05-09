# redis — Cliente Redis Mapex (`redisModel`)

Wrapper sobre [`redis/go-redis/v9`](https://github.com/redis/go-redis) que implementa os contratos de cache definidos em `infrastructure/common/ports`. Adiciona:

- Prefixo de chave em todo comando (`<keyPrefix>:<key>`)
- Marshal / unmarshal via `utils/serialize` para tipos não-string
- Helpers cache-aside `GetOrSet` / `GetOrSetEx` (com `Dest` tipado opcional e indicador `Metrics`)
- Helpers de coleção: sorted sets, hashes, sets, pipelines atômicos
- Utilitários de conversão tempo↔score para timestamps em sorted-sets

> Nome do pacote: `redisModel` (diretório: `redis/`).

## Superfície

### Configuração

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

### Construtores

| Função | Comportamento |
|---|---|
| `New(cfg) (*RedisClient, error)` | Constrói o wrapper, faz ping com timeout de **2 s**, retorna erro em falha. Loga `[INFRA:REDIS] Initialized`. |
| `NewGoRedisClient(cfg) *redis.Client` | Retorna o `*redis.Client` cru (sem wrapper). Faz ping com timeout de 2 s e **chama `logger.Panic` em falha** — usado por componentes que precisam acesso direto (ex: `redisLock`). |

### Constantes (`constants.go`)

| Nome | Valor |
|---|---|
| `DefaultTTLSeconds` | `300` |
| `NoExpiration` | `0` |

### Erros (`errors.go`)

| Sentinel | Mensagem |
|---|---|
| `ErrKeyNotFound` | `key not found in cache` |
| `ErrNilDestination` | `redis: destination is nil` |

`Get` levanta `ErrNilDestination` quando chamado com `dest == nil`. Misses são representados pelo erro `redis.Nil` subjacente — verifique com `errors.Is(err, redis.Nil)`.

### Implementa

```go
var (
    _ common.Cache            = (*RedisClient)(nil)
    _ common.CacheWithTTL     = (*RedisClient)(nil)
    _ common.CacheWithOptions = (*RedisClient)(nil)
    _ common.CacheGetOrSet    = (*RedisClient)(nil)
    _ common.CacheGetOrSetEx  = (*RedisClient)(nil)
)
```

Essas asserções de tempo de compilação ficam no topo de `methods.go`.

## Operações chave/valor (`methods.go`)

Toda método prefixa `key` com `cfg.KeyPrefix + ":"`.

| Método | Notas |
|---|---|
| `Set(ctx, key, value)` | Sem TTL (`NoExpiration`). |
| `SetEx(ctx, key, value, ttl)` | Redis erra quando `ttl <= 0`. |
| `SetWithOptions(ctx, key, value, *common.SetOptions)` | Usa `redis.SetArgs{TTL, Mode, KeepTTL}`. Mode = `NX` / `XX` / vazio (resolvido por `getSetMode`). |
| `Get(ctx, key, dest)` | `*string` e `*[]byte` saem por short-circuit (sem unmarshal); outros tipos via `serialize.Unmarshal`. Retorna `ErrNilDestination` se `dest == nil`. |
| `GetOrSet(GetOrSetParams) (any, error)` | Cache-aside sem TTL. Em miss executa `Callback` e armazena via `Set`. Erro de armazenamento é **silenciosamente descartado** e o valor fresco é retornado. |
| `GetOrSetEx(GetOrSetParams) (any, error)` | Cache-aside com `SetEx(ttl = CacheTTL segundos)`. Respeita `params.Dest` (destino tipado) e `params.Metrics.Hit`. |
| `Ping(ctx) error` | PING. |
| `Del(ctx, key) error` | Mesma convenção de prefixo de `Set`. |

### `prepareValue` (interno)

`string` → como está, `[]byte` → cast para string, qualquer outra coisa → `serialize.Marshal`.

### `GetOrSet` vs `GetOrSetEx`

| | `GetOrSet` | `GetOrSetEx` |
|---|---|---|
| TTL no miss | nenhum (`Set`) | `time.Duration(CacheTTL) * time.Second` (`SetEx`) |
| `Dest` populado | não | sim — refaz marshal do valor fresco e unmarshal em `Dest` |
| `Metrics.Hit` populado | não | sim (true em hit, false em miss) |

## Operações em coleções (`methods_collections.go`)

Todo método prefixa a chave com `<keyPrefix>:` e retorna o erro do `go-redis` verbatim.

### Sorted sets

| Método | Propósito |
|---|---|
| `ZAdd(ctx, key, score, member)` | Adiciona/atualiza um member. |
| `ZScore(ctx, key, member) (float64, error)` | `redis.Nil` se inexistente. |
| `ZMScore(ctx, key, members...) ([]float64, error)` | Round-trip único; members ausentes retornam `NaN`. |
| `ZRangeByScore(ctx, key, min, max, offset, count) ([]string, error)` | Range com paginação. |
| `ZRem(ctx, key, member)` | Remove um member. |

### Hashes

| Método | Propósito |
|---|---|
| `HIncrBy(ctx, key, field, incr) (int64, error)` | Incremento atômico, retorna novo valor. |
| `HDel(ctx, key, field)` | Remove um field. |
| `HSet(ctx, key, field, value)` | Valor string. |
| `HSetInt64(ctx, key, field, value)` | Conveniência para timestamps unix etc. |
| `HGet(ctx, key, field) (string, error)` | `redis.Nil` se inexistente. |
| `HGetInt64(ctx, key, field) (int64, error)` | `redis.Nil` se inexistente; parse via `Int64()` do go-redis. |

### Sets

| Método | Propósito |
|---|---|
| `SAdd(ctx, key, member)` / `SRem(ctx, key, member)` | Adiciona / remove. |
| `SRemN(ctx, key, member) (int64, error)` | Transições race-free: apenas o caller que recebe `n=1` realmente removeu. |
| `SIsMember(ctx, key, member) (bool, error)` | Verificação de membership única. |
| `SMIsMember(ctx, key, members...) ([]bool, error)` | Multi-membership em um round-trip. |
| `SMembers(ctx, key) ([]string, error)` | Todos os members. |

### Pipelines

| Método | Propósito |
|---|---|
| `PipelineRemoveFromCollections(ctx, zsetKey, hashKey, setKey, member)` | `ZRem` + `HDel` + `SRem` atômicos para o mesmo `member`. |

### Helpers de tempo/score (funções de pacote)

| Função | Propósito |
|---|---|
| `ScoreToTime(score) *time.Time` | Retorna `nil` quando o score é `0` ou `NaN`. |
| `TimeToScore(t) float64` | `float64(t.Unix())`. |
| `ScoresToTimeMap(members, scores) map[string]*time.Time` | Pula members com score 0/NaN. |
| `BoolSliceToMap(members, flags) map[string]bool` | Pareia output de SMIsMember com nomes dos members. |
| `FormatCutoff(t) string` | `fmt.Sprintf("%d", t.Unix())` — para limites de `ZRangeByScore`. |

## Uso

### Cache-aside com destino tipado

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
    // veio do Redis
}
_ = res // res aponta para &asset quando Dest é informado
```

### Índice temporal por sorted-set

```go
err := client.ZAdd(ctx, "device:lastseen", redisModel.TimeToScore(now), assetUUID)
ts, err := client.ZScore(ctx, "device:lastseen", assetUUID)
recent := redisModel.ScoreToTime(ts) // nil se ausente
```

### Transição de estado race-free

```go
n, err := client.SRemN(ctx, "alerted", assetUUID)
if err != nil { return err }
if n == 1 {
    // este exato caller limpou o alerta
}
```
