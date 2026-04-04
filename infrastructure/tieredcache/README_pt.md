# TieredCache - Cache Multi-Camadas para MapexOS

## Visão Geral

TieredCache oferece uma solução de cache multi-camadas de alta performance para os serviços do MapexOS. Implementa uma arquitetura de quatro camadas projetada para cenários com 200M+ assets.

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                         Arquitetura TieredCache                               │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                               │
│   L0 (RAM)        L1 (NVMe)         L2 (MinIO)         Fallback (HTTP)       │
│   ┌─────────┐     ┌─────────┐       ┌─────────┐        ┌─────────┐           │
│   │ristretto│ ──▶ │  Disco  │  ──▶  │   S3    │  ──▶   │   API   │           │
│   │  256MB  │     │   1GB   │       │(Verdade)│        │(MongoDB)│           │
│   └─────────┘     └─────────┘       └─────────┘        └─────────┘           │
│      ~50µs          ~500µs           ~5-50ms            ~10-100ms            │
│                                                                               │
│   Dados quentes   Dados mornos      Dados frios        Recuperação           │
│   (acesso         (acesso           (fonte da          (repopula L2          │
│    frequente)      recente)          verdade)           em caso de miss)     │
│                                                                               │
└──────────────────────────────────────────────────────────────────────────────┘
```

## Funcionalidades

### L0 (Cache em RAM)
- Cache em memória ultra-rápido usando [ristretto](https://github.com/dgraph-io/ristretto)
- Evicção LRU com admissão baseada em frequência
- Suporte a TTL com expiração automática
- Acesso concorrente thread-safe
- Latência: **~50µs**

### L1 (Cache em Disco)
- Armazenamento local NVMe/SSD
- Sharding baseado em hash para escalabilidade do filesystem (65.536 diretórios)
- Metadados com rastreamento de expiração
- Limpeza lazy de entradas expiradas na leitura
- Latência: **~500µs**

### L2 (Armazenamento Remoto)
- Armazenamento de objetos MinIO/S3
- Função loader configurável
- Fonte da verdade para todos os dados
- Promoção automática para L0/L1 no acesso
- Latência: **~5-50ms**

### Fallback (API HTTP)
- Chamado quando há miss no L2
- Busca do serviço fonte (MongoDB)
- Repopula automaticamente o cache L2
- Latência: **~10-100ms**

## Instalação

O pacote faz parte da infraestrutura MapexOS:

```go
import "github.com/Mapex-Solutions/MapexOS/infrastructure/tieredcache"
```

## Uso

### Uso Básico

```go
// Criar cache com L0 + L1 + L2
cache, err := tieredcache.New(tieredcache.Config{
    EnableL0:     true,
    L0MaxSize:    256 * 1024 * 1024, // 256MB RAM
    L0DefaultTTL: 5 * time.Minute,

    EnableL1:     true,
    L1Dir:        "/var/cache/mapexos/assets",
    L1MaxSize:    1 * 1024 * 1024 * 1024, // 1GB
    L1DefaultTTL: 1 * time.Hour,

    // Configurar loader L2 (MinIO)
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

// Buscar dados (verifica automaticamente L0 → L1 → L2 → Fallback)
data, tier, err := cache.Get(ctx, "68f5bbce1aef22967c3ebb30/asset-uuid-123")
if err != nil {
    // Tratar cache miss
}
fmt.Printf("Dados da camada: %s\n", tieredcache.CacheTier(tier).String())
```

### Com Fallback HTTP

```go
cache, err := tieredcache.New(tieredcache.Config{
    EnableL0:     true,
    L0MaxSize:    256 * 1024 * 1024,

    EnableL1:     true,
    L1Dir:        "/var/cache/mapexos/assets",

    // Configuração do fallback - chama serviço fonte quando L2 falha
    FallbackBaseURL:  "http://assets-service:5001",
    FallbackAPIKey:   "internal-api-key",
    FallbackEndpoint: "/internal/assets",
    FallbackTimeout:  5 * time.Second,
})
```

Quando há miss no L2, o cache automaticamente chama:
```
GET {FallbackBaseURL}{FallbackEndpoint}/{resourceId}
```

**Nota:** O formato da chave do cache é `{orgId}/{resourceId}`. O fallback extrai apenas o `resourceId` para a chamada da API.

## Formato de Chave e Isolamento de Tenant

TieredCache usa um formato de chave que suporta isolamento multi-tenant:

```
{orgId}/{resourceId}
```

### Exemplos

| Recurso | Formato da Chave | Exemplo |
|---------|------------------|---------|
| Asset | `{asset.orgId}/{assetUUID}` | `68f5bbce.../aav9bpg0v0qg00boitc0` |
| Template | `{templateOrgId}/{templateId}` | `mapexos_public/691bb4071e717d77a2430b46` |
| Bytecode | `{templateOrgId}/{templateId}/{scriptType}` | `mapexos_public/691bb.../VALIDATION` |

### Templates de Sistema (Cache Compartilhado)

Para templates de sistema (`IsSystem=true`), o `templateOrgId` é `mapexos_public`. Isso permite que **todas as organizações** compartilhem a mesma entrada de cache:

```
Organização A usa template X → chave do cache: mapexos_public/templateX/VALIDATION
Organização B usa template X → chave do cache: mapexos_public/templateX/VALIDATION  ← MESMO!
```

**Resultado:** 1 milhão de dispositivos usando o mesmo template de sistema = **1 entrada de cache** (não 1 milhão).

## Estrutura de Disco L1

L1 usa sharding baseado em hash para lidar com 200M+ assets sem problemas de filesystem:

```go
// Chave: "mapexos_public/691bb.../VALIDATION"
// Hash SHA256: "a1b2c3d4e5f67890..."
// Caminho do arquivo: /cache/a1/b2/a1b2c3d4e5f67890.cache
```

Isso cria 65.536 diretórios (256 × 256), mantendo ~3K arquivos por diretório.

**Importante:** A chave completa é hasheada, então mesma chave = mesmo hash = mesmo arquivo. Sem duplicação.

## Configuração

| Campo | Tipo | Padrão | Descrição |
|-------|------|--------|-----------|
| `EnableL0` | `bool` | `false` | Habilitar cache L0 em RAM |
| `L0MaxSize` | `int64` | 256MB | Tamanho máximo do cache RAM |
| `L0MaxItems` | `int64` | 100.000 | Máximo de itens em RAM |
| `L0DefaultTTL` | `Duration` | 5min | TTL padrão para entradas L0 |
| `EnableL1` | `bool` | `false` | Habilitar cache L1 em disco |
| `L1Dir` | `string` | `/tmp/mapexos-cache` | Diretório do cache L1 |
| `L1MaxSize` | `int64` | 1GB | Tamanho máximo do cache em disco |
| `L1DefaultTTL` | `Duration` | 1hora | TTL padrão para entradas L1 |
| `KeyPrefix` | `string` | `""` | Prefixo para todas as chaves do cache |
| `EnableMetrics` | `bool` | `false` | Habilitar métricas detalhadas |
| `EnableL2` | `bool` | `false` | Habilitar loader remoto L2 (MinIO/S3) |
| `L2Loader` | `LocalCacheLoader` | `nil` | Função loader para L2 (obrigatório se EnableL2) |
| `FallbackBaseURL` | `string` | `""` | URL base para API de fallback |
| `FallbackAPIKey` | `string` | `""` | API Key para autenticação do fallback |
| `FallbackEndpoint` | `string` | `""` | Caminho do endpoint para fallback |
| `FallbackTimeout` | `Duration` | 5s | Timeout para requisições de fallback |

## Invalidação de Cache

A invalidação de cache é feita via mensagens NATS FANOUT. Quando qualquer instância invalida uma chave, todas as instâncias recebem a mensagem:

```go
// Formato da mensagem de invalidação
type InvalidateMessage struct {
    OrgId      string `json:"orgId"`
    ResourceId string `json:"resourceId"`
}

// Publicador (serviço dono)
natsBus.Publish("mapexos.cache.invalidate.assets", InvalidateMessage{
    OrgId:      "68f5bbce1aef22967c3ebb30",
    ResourceId: "aav9bpg0v0qg00boitc0",
})

// Assinante (todas as instâncias - consumer FANOUT)
natsBus.SubscribeFanout("mapexos.cache.invalidate.assets", func(msg InvalidateMessage) {
    cacheKey := fmt.Sprintf("%s/%s", msg.OrgId, msg.ResourceId)
    cache.Invalidate(cacheKey)
})
```

## Estatísticas

```go
stats := cache.Stats()
fmt.Printf("L0 Hits: %d, Misses: %d\n", stats.L0Hits, stats.L0Misses)
fmt.Printf("L1 Hits: %d, Misses: %d\n", stats.L1Hits, stats.L1Misses)
fmt.Printf("L2 Hits: %d, Misses: %d\n", stats.L2Hits, stats.L2Misses)
fmt.Printf("Fallback Hits: %d, Misses: %d\n", stats.FallbackHits, stats.FallbackMisses)
fmt.Printf("L1 Lazy Expired: %d\n", stats.L1LazyExpired)
```

## Fluxo das Camadas de Cache

```
Get(key) chamado
    │
    ▼
┌─────────┐  hit
│   L0    │ ────────▶ retorna dados (tier: L0-RAM)
│  (RAM)  │
└────┬────┘
     │ miss
     ▼
┌─────────┐  hit
│   L1    │ ────────▶ promove para L0, retorna dados (tier: L1-Disk)
│ (Disco) │
└────┬────┘
     │ miss
     ▼
┌─────────┐  hit
│   L2    │ ────────▶ promove para L0/L1, retorna dados (tier: L2-Remote)
│ (MinIO) │
└────┬────┘
     │ miss
     ▼
┌──────────┐  hit
│ Fallback │ ────────▶ L2 repopulado pelo serviço fonte,
│  (HTTP)  │           promove para L0/L1, retorna dados (tier: Fallback-HTTP)
└────┬─────┘
     │ miss
     ▼
  retorna ErrCacheMiss
```

## Boas Práticas

1. **Dimensione L0 adequadamente**: Dados quentes devem caber na RAM. Monitore a taxa de hit do L0.
2. **Use NVMe para L1**: Armazenamento SSD proporciona latência sub-milissegundo.
3. **Defina TTLs apropriados**: Balance atualização vs performance.
4. **Configure Fallback**: Garante recuperação de dados quando L2 falha.
5. **Use formato de chave consistente**: `{orgId}/{resourceId}` para isolamento de tenant.
6. **Compartilhe recursos de sistema**: Use orgId `mapexos_public` para templates compartilhados.
7. **Monitore estatísticas**: Acompanhe taxas de hit para ajustar configuração.

## Registros de Decisão de Arquitetura

### Por que não Redis SharedCache?

Para 200M+ assets, Redis SharedCache tem limitações:
- Latência de rede para cada acesso
- Custo de memória para todas as instâncias
- Ponto único de falha

TieredCache com L0/L1 local oferece:
- Acesso L0 sub-microssegundo
- Sem salto de rede para dados em cache
- Escala linear com instâncias

### Por que ristretto?

- Política de admissão previne poluição do cache
- Melhor taxa de hit que LRU simples
- Thread-safe com contenção mínima
- Suporte a TTL integrado

### Por que sharding baseado em hash no L1?

- Performance do filesystem degrada com 100K+ arquivos por diretório
- Sharding por hash cria 65.536 diretórios
- Distribuição uniforme independente dos padrões de chave
- Mesma chave sempre mapeia para o mesmo arquivo (sem duplicação)

### Por que Fallback HTTP?

- L2 (MinIO) pode ter dados obsoletos ou estar temporariamente indisponível
- Fallback garante recuperação de dados da fonte (MongoDB)
- Serviço fonte repopula L2 automaticamente
- Arquitetura de cache auto-recuperável
