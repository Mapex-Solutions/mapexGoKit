# ClickHouse Infrastructure Package

Este pacote fornece uma camada de abstração para interagir com ClickHouse, similar ao `mongoModel` do MongoDB.

## Estrutura

```
clickhouse/
├── manager/     # Gerenciamento de conexão com health monitoring
│   ├── manager.go
│   └── types.go
├── model/       # ORM genérico com reflection e query builder
│   ├── model.go
│   ├── methods.go
│   ├── query_builder.go
│   ├── reflection.go
│   ├── types.go
│   └── utils.go
└── README.md
```

## Manager

O `manager` é responsável por gerenciar a conexão com ClickHouse, incluindo:
- Conexão com retry automático
- Health monitoring em background
- Status de conexão via atomic bool

### Uso

```go
import chManager "github.com/Mapex-Solutions/MapexOS/infrastructure/clickhouse/manager"

// Criar instância com configuração
ch, err := chManager.New(chManager.Config{
    Host:            "localhost",
    Port:            9000,
    Database:        "mapexos",
    Username:        "default",
    Password:        "password",
    EnableMonitor:   true,
    MonitorInterval: 30 * time.Second,
})
if err != nil {
    log.Fatal(err)
}

// Verificar status de conexão
if ch.IsConnected() {
    fmt.Println("Connected to ClickHouse")
}

// Obter conexão raw para uso direto
conn := ch.GetConn()

// Obter status de saúde (para endpoints /health)
status := ch.GetHealthStatus()
```

## Model (chModel)

O `model` fornece um ORM genérico baseado em reflection para operações CRUD com ClickHouse.

### Definindo Entidades

Use a tag `ch:` para mapear campos da struct para colunas do ClickHouse:

```go
type RawEvent struct {
    // Timestamp deve ser o primeiro campo (usado para particionamento)
    Timestamp     time.Time              `ch:"timestamp" json:"timestamp"`
    ThreadId      string                 `ch:"thread_id" json:"threadId"`
    OrgId         string                 `ch:"org_id" json:"orgId"`
    PathKey       string                 `ch:"path_key" json:"pathKey"`
    Source        string                 `ch:"source" json:"source"`

    // Campos map/slice são automaticamente serializados como JSON
    Payload       map[string]interface{} `ch:"payload" json:"payload"`
    Metadata      map[string]interface{} `ch:"metadata" json:"metadata"`

    RetentionDays uint16                 `ch:"retention_days" json:"retentionDays"`
}
```

### Criando uma Table

```go
import chModel "github.com/Mapex-Solutions/MapexOS/infrastructure/clickhouse/model"

// Criar instância da tabela
table, err := chModel.NewTable[RawEvent](conn, "events_raw", chModel.TableConfig{
    TimestampField: "timestamp",
    DefaultOrder:   "DESC",
})
if err != nil {
    log.Fatal(err)
}
```

### Insert Operations

```go
// Insert único
event := &RawEvent{
    Timestamp: time.Now(),
    ThreadId:  "ds-123",
    OrgId:     "org-456",
    PathKey:   "org-456/site-1/device-1",
    Source:    "http_gateway",
    Payload:   map[string]interface{}{"temperature": 25.5},
    Metadata:  map[string]interface{}{"ip": "192.168.1.1"},
}
err := table.Insert(ctx, event)

// Insert em batch (otimizado para grandes volumes)
events := []*RawEvent{event1, event2, event3}
err := table.InsertBatch(ctx, events)
```

### Query Operations

#### FindByOffset (Paginação simples)

```go
// Filtros simples (igualdade)
filters := chModel.Map{
    "org_id": "org-456",
    "source": "http_gateway",
}

pagination := &chModel.PaginationOpts{
    Page:    1,
    PerPage: 50,
}

result, err := table.FindByOffset(ctx, filters, pagination, "timestamp:desc")
// result.Items      → []RawEvent
// result.Pagination → {Page, PerPage, TotalItems, TotalPages}
```

#### FindWithFilters (Filtros avançados)

```go
filters := []chModel.Filter{
    // Igualdade
    {Field: "org_id", Operator: chModel.OpEqual, Value: "org-456"},

    // LIKE (para pathKey com wildcard)
    {Field: "path_key", Operator: chModel.OpLike, Value: "org-456/site-1%"},

    // Intervalo de datas com BETWEEN
    {
        Field:    "timestamp",
        Operator: chModel.OpBetween,
        Value:    startTime,    // time.Time
        EndValue: endTime,      // time.Time
    },

    // Maior ou igual
    {Field: "retention_days", Operator: chModel.OpGreaterEqual, Value: 7},

    // IN (lista de valores)
    {Field: "source", Operator: chModel.OpIn, Value: []string{"http_gateway", "mqtt_gateway"}},
}

result, err := table.FindWithFilters(ctx, filters, pagination, "timestamp:desc")
```

### Operadores Disponíveis

| Operador | Descrição | Exemplo |
|----------|-----------|---------|
| `OpEqual` | Igualdade | `field = value` |
| `OpNotEqual` | Diferente | `field != value` |
| `OpGreater` | Maior que | `field > value` |
| `OpGreaterEqual` | Maior ou igual | `field >= value` |
| `OpLess` | Menor que | `field < value` |
| `OpLessEqual` | Menor ou igual | `field <= value` |
| `OpLike` | Pattern matching | `field LIKE 'value%'` |
| `OpIn` | Lista de valores | `field IN (v1, v2)` |
| `OpBetween` | Intervalo | `field BETWEEN v1 AND v2` |

### Query Builder (Uso Avançado)

Para queries mais complexas, use o QueryBuilder diretamente:

```go
qb := table.Query().
    Select("timestamp", "org_id", "payload").
    Where("org_id", chModel.OpEqual, "org-456").
    Where("timestamp", chModel.OpGreaterEqual, startTime).
    OrderBy("timestamp DESC").
    Limit(100).
    Offset(0)

query, args := qb.BuildSelect()
// query: "SELECT timestamp, org_id, payload FROM events_raw WHERE org_id = ? AND timestamp >= ? ORDER BY timestamp DESC LIMIT 100 OFFSET 0"
// args: ["org-456", startTime]

// Execute manualmente
rows, err := conn.Query(ctx, query, args...)
```

### Count

```go
count, err := table.Count(ctx, chModel.Map{"org_id": "org-456"})
```

## Exemplo Completo: Repository

```go
package clickhouseRepo

import (
    "context"
    "time"

    "myservice/domain/entities"
    "myservice/domain/repositories"

    "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
    chModel "github.com/Mapex-Solutions/MapexOS/infrastructure/clickhouse/model"
)

type EventRepositoryClickHouse struct {
    conn          driver.Conn
    rawEventTable *chModel.Table[entities.RawEvent]
}

func NewEventRepository(conn driver.Conn) repositories.EventRepository {
    rawTable, err := chModel.NewTable[entities.RawEvent](conn, "events_raw", chModel.TableConfig{
        TimestampField: "timestamp",
        DefaultOrder:   "DESC",
    })
    if err != nil {
        // Handle error
    }

    return &EventRepositoryClickHouse{
        conn:          conn,
        rawEventTable: rawTable,
    }
}

func (r *EventRepositoryClickHouse) SaveRawEventBatch(ctx context.Context, events []*entities.RawEvent) error {
    return r.rawEventTable.InsertBatch(ctx, events)
}

func (r *EventRepositoryClickHouse) QueryEventsRaw(
    ctx context.Context,
    orgFilter chModel.Map,
    startTime *time.Time,
    endTime *time.Time,
    pagination *chModel.PaginationOpts,
    sort string,
) (*chModel.PaginatedResult[entities.RawEvent], error) {

    filters := []chModel.Filter{}

    // Org filter
    if orgId, ok := orgFilter["orgId"]; ok {
        filters = append(filters, chModel.Filter{
            Field:    "org_id",
            Operator: chModel.OpEqual,
            Value:    orgId,
        })
    }

    // Date range with BETWEEN
    if startTime != nil && endTime != nil {
        filters = append(filters, chModel.Filter{
            Field:    "timestamp",
            Operator: chModel.OpBetween,
            Value:    *startTime,
            EndValue: *endTime,
        })
    }

    return r.rawEventTable.FindWithFilters(ctx, filters, pagination, sort)
}
```

## Configuração no DI Container (dig)

```go
import (
    "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
    chManager "github.com/Mapex-Solutions/MapexOS/infrastructure/clickhouse/manager"
)

// Provide ClickHouseManager
c.Provide(func() *chManager.ClickHouseManager {
    ch, err := chManager.New(chManager.Config{
        Host:            cfg.Host,
        Port:            cfg.Port,
        Database:        cfg.Database,
        Username:        cfg.Username,
        Password:        cfg.Password,
        EnableMonitor:   true,
        MonitorInterval: 30 * time.Second,
    })
    if err != nil {
        logger.Panic(err.Error())
    }
    return ch
})

// Provide raw connection for repositories
c.Provide(func(ch *chManager.ClickHouseManager) driver.Conn {
    return ch.GetConn()
})
```

## Notas Importantes

1. **Ordem dos campos**: A ordem dos campos na struct deve corresponder à ordem das colunas na tabela ClickHouse para melhor performance.

2. **JSON automático**: Campos do tipo `map[string]interface{}` ou `[]interface{}` são automaticamente serializados/deserializados como JSON.

3. **Timestamps**: Configure `TimestampField` para habilitar ordenação padrão por tempo.

4. **Batch inserts**: Use `InsertBatch` para grandes volumes de dados - ClickHouse é otimizado para bulk operations.

5. **BETWEEN vs range**: Para filtros de data, prefira `OpBetween` em vez de dois filtros separados (`>=` e `<=`) para queries mais eficientes.
