# clickhouse — Cliente ClickHouse, manager e camada genérica de tabela

Três subpacotes cooperantes construídos sobre [`ClickHouse/clickhouse-go/v2`](https://github.com/ClickHouse/clickhouse-go):

| Caminho | Pacote | Papel |
|---|---|---|
| `clickhouse/` (raiz) | `clickhouseModel` | Wrapper `Client` mínimo — open + ping + acesso bruto a `driver.Conn` |
| `clickhouse/manager/` | `chManager` | Manager de ciclo de vida da conexão com health monitoring e reconnect |
| `clickhouse/model/` | `chModel` | `Table[T]` genérica + `QueryBuilder` + mapeamento de colunas via reflection |

Em comum nos três: porta nativa `9000`, compressão LZ4, prefixo de log `[INFRA:CLICKHOUSE]`.

## Raiz: `clickhouseModel`

Wrapper minimamente viável. Use quando precisar de um `driver.Conn` e não quiser as features de ciclo de vida do manager.

### Config

```go
type Config struct {
    Host, Database, Username, Password string
    Port                                int
}
```

### Superfície

```go
func New(cfg Config) (*Client, error)
func (c *Client) GetConn() driver.Conn
```

`New` abre a conexão (compressão LZ4 ligada) e faz ping com timeout de **2 s**. Erros são embrulhados: `clickhouse connection failed: %w` ou `clickhouse ping failed: %w`. Loga `[INFRA:CLICKHOUSE] Initialized successfully`.

## Manager: `chManager`

Manager de longa vida que mantém uma conexão, monitora saúde e reconecta em background.

### Config

```go
type Config struct {
    Host            string        // obrigatório
    Port            int           // padrão 9000
    Database        string        // obrigatório
    Username        string        // obrigatório
    Password        string        // (não validado por New, apesar do texto de ErrMissingConfig)
    MaxOpenConns    int           // padrão 10
    MaxIdleConns    int           // padrão 5
    EnableMonitor   bool
    MonitorInterval time.Duration // padrão 10s
}
```

### Validação

`New` retorna `ErrMissingConfig` quando `Host`, `Database` ou `Username` está vazio. **A mensagem diz "host, database, username, and password are required"** mas o código não verifica `Password` — pode estar vazio.

### Conexão (`internals.go`)

| Setting | Valor |
|---|---|
| Compressão | `LZ4` |
| `Settings: max_execution_time` | `60` |
| `DialTimeout` | `DefaultConnectTimeout` (5 s) |
| `MaxOpenConns` / `MaxIdleConns` | da cfg, padrões `10` / `5` |
| `ConnMaxLifetime` | `1 hora` |
| Timeout do ping | `DefaultPingTimeout` (3 s) |

### Monitor de fundo

`startMonitor()` faz tick a cada `MonitorInterval`. A cada tick faz ping; em falha loga e chama `connect()` para reconectar. Em sucesso armazena a latência em `m.lastLatency`. **O monitor só inicia se `Config.EnableMonitor == true`**.

### Métodos

| Método | Notas |
|---|---|
| `IsConnected() bool` | atômico, lock-free |
| `GetConn() driver.Conn` | nil até o primeiro connect bem-sucedido — proteja com `IsConnected()` |
| `GetDatabase() string` | DB configurado |
| `GetConfig() Config` | password é mascarado como `"***"` |
| `Health(ctx) HealthStatus` | ping ao vivo quando `ctx != nil`; atualiza `Connected` + `ErrorMessage` |
| `LastLatency() int64` | última latência de ping em ms |
| `Close() error` | seta `isConnected=false` e fecha a conexão |

### `HealthStatus`

```go
type HealthStatus struct {
    Connected    bool      `json:"connected"`
    Database     string    `json:"database"`
    Host         string    `json:"host"`
    Port         int       `json:"port"`
    LastCheckAt  time.Time `json:"lastCheckAt"`
    ErrorMessage string    `json:"errorMessage,omitempty"`
}
```

### Constantes

| Constante | Valor |
|---|---|
| `DefaultMonitorInterval` | `10 * time.Second` |
| `DefaultPort` | `9000` |
| `DefaultConnectTimeout` | `5 * time.Second` |
| `DefaultPingTimeout` | `3 * time.Second` |

### Erros

| Sentinel | Mensagem |
|---|---|
| `ErrMissingConfig` | `host, database, username, and password are required` |
| `ErrNotConnected` | `clickhouse is not connected` |
| `ErrConnectionFailed` | `failed to connect to clickhouse` |
| `ErrPingFailed` | `clickhouse ping failed` |

## Model: `chModel` — Camada genérica de tabela

`Table[T any]` provê Insert/Query/Pagination tipados sobre uma única tabela ClickHouse. Metadados de campo são derivados uma vez via reflection e cacheados.

### Construção

```go
type Event struct {
    Timestamp time.Time              `ch:"timestamp"`
    OrgId     string                 `ch:"org_id"`
    Payload   map[string]interface{} `ch:"payload"`   // marshalado como JSON
}

table, err := chModel.NewTable[Event](conn, "events", chModel.TableConfig{
    TimestampField: "timestamp", // padrão "timestamp"
    DefaultOrder:   "DESC",      // padrão "DESC"
    DefaultTimeout: 30*time.Second, // padrão 30s
})
```

`NewTable` retorna `ErrInvalidType` quando `T` não é struct, `ErrNoFields` quando nenhum field exportado tem tag `ch` (ou fallback `json`).

### Metadados de campo (`reflection.go`)

Para cada field exportado, o nome da coluna vem da tag `ch`, com fallback para `json` (primeiro segmento antes de `,`). Tag `"-"` ou vazia pula o field.

Flags do `fieldInfo` calculadas uma vez por struct:

| Flag | Significado |
|---|---|
| `IsJSON` | `true` para tipos `map` ou `slice` — marshal JSON em insert, unmarshal JSON em scan. **Exceções:** `[]byte` (cru) e `map[<chaveNumérica>]V` (tipo `Map(K, V)` nativo do ClickHouse, não JSON). |
| `IsPointer` | `true` quando field é ponteiro; ponteiros nil mapeiam para SQL `nil` em insert. |
| `IsTime` | `true` para `time.Time` ou `*time.Time` — usado por `FindByCursor`. |

### Operações em `*Table[T]`

| Método | Comportamento |
|---|---|
| `Insert(ctx, *T) error` | `Exec` `INSERT INTO ... VALUES (?, ?, ...)`. Erros embrulham `ErrQueryFailed`. |
| `InsertBatch(ctx, []*T) error` | `PrepareBatch` + `batch.Append` por item + `batch.Send`. Slice vazio → `ErrEmptyItems`. Erros embrulham `ErrBatchFailed`. Loga `[INFRA:CLICKHOUSE] Batch inserted: %d records into %s`. |
| `Count(ctx, Map) (uint64, error)` | `SELECT COUNT(*) FROM ... WHERE field = ?`. |
| `FindByOffset(ctx, Map, *PaginationOpts, sort) (*PaginatedResult[T], error)` | Paginação por Page/PerPage com round-trip de `Count`. |
| `FindWithFilters(ctx, []Filter, *PaginationOpts, sort) (*PaginatedResult[T], error)` | Igual a `FindByOffset` mas aceita operadores de `Filter` mais ricos. |
| `FindByCursor(ctx, []Filter, *TimeCursorOpts) (*TimeCursorResult[T], error)` | Paginação por cursor temporal — sem `COUNT(*)`. **Loga o SQL gerado via `logger.Info` em toda chamada.** Retorna `HasNext`/`HasPrevious`, `NextCursor`/`PrevCursor`. |
| `TableName() string`, `Config() TableConfig`, `Columns() []string`, `Conn() driver.Conn` | Introspecção / escape hatches. |
| `Query() *QueryBuilder` | Construção manual de query. |

### Padrões de paginação

| Constante | Valor |
|---|---|
| `DefaultPage` | `1` |
| `DefaultPerPage` | `25` |
| `MaxPerPage` | `300` (silenciosamente coage `pagination.PerPage > 300` para o default) |
| `MaxOffset` | `10000` (silenciosamente coage offsets maiores) |

### Parsing da string de sort

`ParseSort("timestamp:desc", "timestamp", "DESC")` → `"timestamp DESC"`. Direção é uppercased; direção desconhecida cai no `defaultOrder`. `sort` vazio cai em `<defaultField> <defaultOrder>`.

### `QueryBuilder` (`query_builder.go`)

Construtor SQL fluente. `BuildSelect` e `BuildCount` retornam `(query, args)` prontos para `conn.Query`.

| Método | SQL |
|---|---|
| `Select(cols...)` | `SELECT col1, col2, ...` (padrão `SELECT *`) |
| `Where(field, op, value)` | `field <op> ?` (ou `field IN (?)` / `field BETWEEN ? AND ?`) |
| `WhereFilter(Filter)` / `WhereFilters([]Filter)` | O mesmo via `Filter` (trata os dois valores de `OpBetween`) |
| `WhereMap(Map)` | Conveniência só para igualdade — toda entrada vira `field = ?` |
| `WhereLike(field, pattern)` | `field LIKE ?` |
| `WhereRaw(clause, args...)` | Escape hatch sem template |
| `OrderBy(...)`, `Limit(n)`, `Offset(n)` | Cláusulas finais |
| `BuildSelect()` / `BuildCount()` | Renderiza |

Builders no nível de pacote: `BuildInsert(table, cols)`, `BuildInsertBatch(table, cols)` (sem `VALUES`, para `PrepareBatch`).

### Operadores de filtro

```go
OpEqual        FilterOperator = "="
OpNotEqual     FilterOperator = "!="
OpGreater      FilterOperator = ">"
OpGreaterEqual FilterOperator = ">="
OpLess         FilterOperator = "<"
OpLessEqual    FilterOperator = "<="
OpLike         FilterOperator = "LIKE"
OpIn           FilterOperator = "IN"
OpNotIn        FilterOperator = "NOT IN"
OpBetween      FilterOperator = "BETWEEN"
```

### Operadores shortcut em `Map` (em `buildWhereFromMap`)

Existe um helper que reconhece maps de operador estilo MongoDB dentro de filtros `Map`. Está definido em `methods.go` mas **não é chamado pelos métodos `Find*` públicos hoje** — eles usam `WhereMap` (somente igualdade) para filtros `Map` e exigem `[]Filter` para os outros operadores.

```go
// Tokens reconhecidos (quando este helper estiver conectado)
$regex  → field LIKE ?     // remove "^" inicial, adiciona "%" no fim
$gt     → field > ?
$gte    → field >= ?
$lt     → field < ?
$lte    → field <= ?
$ne     → field != ?
$in     → field IN (?)
```

### `TimeCursorOpts` / `TimeCursorResult`

```go
type TimeCursorOpts struct {
    Cursor    interface{} // string RFC3339 ou time.Time; nil = primeira página
    Direction string      // "next" (padrão) ou "prev"
    Limit     int64       // limitado por MaxPerPage; padrão 20
    SortAsc   bool        // false = DESC (padrão)
}

type TimeCursorResult[T any] struct {
    Items       []T       `json:"items"`
    NextCursor  time.Time `json:"nextCursor,omitempty"`
    PrevCursor  time.Time `json:"prevCursor,omitempty"`
    HasNext     bool      `json:"hasNext"`
    HasPrevious bool      `json:"hasPrevious"`
}
```

O field do cursor vem de `TableConfig.TimestampField` (padrão `"timestamp"`). A implementação busca `limit + 1` linhas para detectar a fronteira "tem mais".

### Erros

| Sentinel | Disparado quando |
|---|---|
| `ErrEmptyItems` | `InsertBatch` chamado com slice vazio |
| `ErrInvalidType` | `T` não é struct em `NewTable[T]` |
| `ErrNoFields` | Nenhum field exportado com tag `ch`/`json` |
| `ErrNotFound` | Definido; não levantado pelo código atual |
| `ErrInvalidFilter` | Definido; não levantado pelo código atual |
| `ErrQueryFailed` | Embrulha qualquer falha de `Query`/`QueryRow`/`Exec` |
| `ErrScanFailed` | Embrulha falha de `rows.Scan` |
| `ErrBatchFailed` | Embrulha falha de `PrepareBatch` / `Append` / `Send` |
| `ErrMarshalFailed` | Embrulha falha de `json.Marshal` em insert |

Erros de unmarshal JSON durante `scanRows` são **logados como `Warn` e o field fica com valor zero** — não abortam a query.

## Exemplo end-to-end

```go
// Manager (longa vida, com monitor)
mgr, err := chManager.New(chManager.Config{
    Host: "ch.local", Database: "mapex", Username: "default", Password: secret,
    EnableMonitor: true, MonitorInterval: 10*time.Second,
})
if err != nil { return err }
defer mgr.Close()

// Tabela genérica sobre a conexão do manager
type Event struct {
    Timestamp time.Time   `ch:"timestamp"`
    OrgId     string      `ch:"org_id"`
    Payload   chModel.Map `ch:"payload"`  // JSON-marshalado
}
events, _ := chModel.NewTable[Event](mgr.GetConn(), "events", chModel.TableConfig{})

// Insert em batch
_ = events.InsertBatch(ctx, batch)

// Paginação por cursor, últimos 50 eventos da org
res, err := events.FindByCursor(ctx,
    []chModel.Filter{{Field: "org_id", Operator: chModel.OpEqual, Value: "org-123"}},
    &chModel.TimeCursorOpts{Limit: 50, Direction: "next"})
if err != nil { return err }
for _, ev := range res.Items { _ = ev }
if res.HasNext {
    // passe res.NextCursor como TimeCursorOpts.Cursor na próxima chamada
}
```
