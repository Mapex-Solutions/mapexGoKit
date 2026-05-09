# mongodb — Manager de conexão e camada genérica de model

Dois subpacotes cooperantes construídos sobre [`go.mongodb.org/mongo-driver/v2`](https://github.com/mongodb/mongo-go-driver):

| Caminho | Pacote | Papel |
|---|---|---|
| `mongodb/manager/` | `mongoManager` | Ciclo de vida da conexão, health monitoring, transações, backpressure de escrita |
| `mongodb/model/` | `mongoModel` | `Model[T]` genérico sobre uma única coleção — CRUD, paginação, delegação de transação, helpers de BSON |

O pacote `model` depende de `manager` para transações (`mongoModel.Model.RunTransaction` delega para `manager.RunTransactionWithClient`).

> Documentação detalhada de cada subpacote vive em `manager/docs/README.md` e `model/docs/README.md`.

## Manager: `mongoManager`

### Config

```go
type Config struct {
    URI             string        // obrigatório
    Database        string        // obrigatório
    EnableMonitor   bool
    MonitorInterval time.Duration // veja warning abaixo
    UseBsonD        bool

    // Backpressure (opt-in)
    EnableBackpressure   bool
    BackpressureWindow   int   // padrão: 1000
    ThrottledThresholdMs int64 // padrão: 150
    BackoffThresholdMs   int64 // padrão: 500
}
```

> **Quirk de `MonitorInterval`.** A goroutine startMonitor usa `time.NewTicker(m.cfg.MonitorInterval * time.Second)`. O field é tipado como `time.Duration` mas o código trata o valor como **número de segundos** (a constante `DefaultMonitorInterval = 10` também é um untyped int). Passar `10 * time.Second` resulta em ticker de 10²⁰ ns — efetivamente nunca dispara. Passe int-as-Duration (ex: `MonitorInterval: 10`) ou corrija a multiplicação.

### Construção

```go
func New(cfg Config) (*MongoManager, error)
```

1. Retorna `ErrMissingURIOrDatabase` se `URI` ou `Database` está vazio.
2. Chama `connect()`: aplica `BSONOptions{ DefaultDocumentMap: true }` a menos que `UseBsonD` esteja ativo, abre o cliente, faz ping com timeout de **3 s**. Erro subjacente é retornado em falha.
3. Inicia `startMonitor()` se `EnableMonitor`.
4. Inicia o tracker de backpressure se `EnableBackpressure`.
5. Loga `[INFRA:MONGODB] Initialized`.

`Close(ctx)` cancela o tracker (se houver) e desconecta o cliente.

### Toggle `UseBsonD`

Quando `false` (padrão), o driver decodifica documentos aninhados como `map[string]any` via `BSONOptions.DefaultDocumentMap=true`. Asserções padrão como `val.(map[string]interface{})` funcionam.

Quando `true`, o driver retorna `bson.D` ordenado para os mesmos caminhos. Use apenas se precisar de ordem de fields.

### Métodos

| Método | Notas |
|---|---|
| `IsConnected() bool` | atômico, lock-free |
| `GetClient() *mongo.Client` / `GetDatabase() *mongo.Database` / `GetDatabaseName() string` | |
| `LastLatency() int64` | última latência de ping em ms (atualizada pelo monitor) |
| `Close(ctx) error` | para o tracker + Disconnect |
| `RunTransaction(ctx, txnFunc) (any, error)` | ciclo de vida completo de transação com retry |
| `NewSession(ctx) (*mongo.Session, error)` | caller possui o `EndSession` |
| `RecordWriteLatency(d time.Duration)` | grava amostra para backpressure (no-op se desabilitado) |
| `GetBackpressureMode() BackpressureMode` | `Normal` se desabilitado |
| `WriteP99() int64` | último P99 calculado em ms (`0` se desabilitado / sem amostras) |

Helper standalone:

```go
func RunTransactionWithClient(ctx, *mongo.Client, TransactionFunc) (any, error)
```

Usado por `mongoModel.Model.RunTransaction` para manter a lógica de transação centralizada mesmo quando os callers só têm um `*mongo.Client`.

### Transações

Ambas entradas compartilham `runTransactionWithRetryInternal`:

1. `session.StartTransaction()`
2. Executa `txnFunc(sessCtx)` onde `sessCtx = mongo.NewSessionContext(ctx, session)`
3. Aborta em erro. Se o erro tem o label `TransientTransactionError` → retenta a transação inteira.
4. Commit. Se commit falhar com `UnknownTransactionCommitResult` → retenta o commit. Se falhar com `TransientTransactionError` → retenta a transação inteira.

`hasErrorLabel` checa tanto `mongo.CommandError` quanto `mongo.WriteException` via `errors.As`.

### Backpressure

Três modos (`BackpressureMode int32`):

| Modo | Significado | Sugestão de comportamento |
|---|---|---|
| `Normal` (0) | P99 abaixo de `ThrottledThresholdMs` | Comportamento padrão |
| `Throttled` (1) | P99 ≥ `ThrottledThresholdMs` por 3 windows | Reduzir batch size |
| `Backoff` (2) | P99 ≥ `BackoffThresholdMs` por 3 windows | Reduzir mais + adicionar pausa |

Internals (`backpressure.go`):

- Buffer circular lock-free (`samples []int64`), índice de escrita atômico.
- P99 é recalculado a cada **5 s** (`computeInterval`) pela goroutine de fundo.
- Regras de transição: 3 windows consecutivas acima do threshold para subir (`windowsToTransition = 3`); P99 abaixo de `ThrottledThresholdMs` reseta imediatamente para `Normal`.
- Cada tick não-Normal loga `[INFRA:MONGODB] Backpressure mode=… P99=…ms` em Warn.

`BackpressureMode.String()` retorna `Normal`/`Throttled`/`Backoff`/`Unknown`.

### Constantes e erros

```go
const DefaultMonitorInterval = 10 // (ver quirk acima)
const defaultBackpressureWindow   = 1000
const defaultThrottledThresholdMs = 150
const defaultBackoffThresholdMs   = 500
const computeInterval             = 5 * time.Second
const windowsToTransition         = 3
```

```go
var ErrMissingURIOrDatabase = errors.New("URI and Database are required")
var ErrNotConnected         = errors.New("MongoDB client is not connected")
```

## Model: `mongoModel` — Wrapper genérico de coleção

`Model[T any]` provê CRUD tipado e paginação sobre uma única coleção. O field `T` da struct deve seguir tags `bson` padrão.

### Construção

```go
func New[T any](db *mongo.Database, collection string, cfg Config) *Model[T]
```

Comportamento:

- Lista coleções; se `collection` está ausente, chama `CreateCollection`. Erros de listagem/criação são **logados, não retornados**.
- Chama `ensureIndexes(...)` — idempotente (pula nomes de índice existentes). Erros de criação são **logados em Warn, não retornados**.
- Retorna o `*Model[T]` mesmo em falha parcial.

`New` **não** retorna erro — diagnostique pelos logs.

### `Config`

```go
type Config struct {
    DefaultTimeout time.Duration       // aplicado quando ctx não tem deadline
    Indexes        []IndexDefinition
}

type IndexDefinition struct {
    Name                    string             // obrigatório
    Keys                    map[string]int     // 1 = ASC, -1 = DESC
    Unique                  bool
    Sparse                  bool
    PartialFilterExpression bson.M
    ExpireAfterSeconds      *int32             // TTL — use *int32(0) quando o field já guarda o expiry absoluto
}
```

> Nota: `Keys` é um `map[string]int`. A ordem dos fields em índice composto **não é determinística** sob iteração de map em Go. Para multi-key onde ordem importa, mantenha `Name` estável e aceite que `Keys` seguem ordem de iteração; se depende de ordem, construa o índice manualmente via `col.Indexes()`.

### Fields auto-populados (reflection)

`CreateOne` / `CreateMany` percorrem fields da struct e, se zero-valued, setam:

| Field detectado por | Atribuído |
|---|---|
| Tag BSON com prefixo `_id` ou nome de field `id` (lowercase), do tipo `bson.ObjectID` | `bson.NewObjectID()` |
| Tag BSON com prefixo `created` ou nome de field `created`/`createdat`, do tipo `time.Time` | `time.Now().UTC()` |

### CRUD

Todos os métodos aceitam variadic `*CommonOpts` — apenas o **primeiro** é honrado. Normalização de contexto: se `ctx` não tem deadline e `Config.DefaultTimeout > 0`, um `WithTimeout` é aplicado; se `CommonOpts.Session` está setado, a sessão é vinculada via `mongo.NewSessionContext`.

| Método | Comportamento |
|---|---|
| `DIRECT() *mongo.Collection` | Escape hatch para a coleção crua. |
| `CreateOne(ctx, *T, opts...) (*T, error)` | InsertOne após auto-popular `_id`/`created`. |
| `CreateMany(ctx, []T, opts...) ([]T, error)` | InsertMany; slice vazio → `ErrEmptyItems`. |
| `FindByID(ctx, id, opts...) (*T, error)` | `id` pode ser string ou `bson.ObjectID`. Input ruim → `ErrInvalidID`. Miss → `ErrNotFound`. |
| `FindOne(ctx, *Map, opts...) (*T, error)` | Miss → `ErrNotFound`. |
| `FindByOffset(ctx, Map, *PaginationOpts, opts...) (*PaginatedResult[T], error)` | Paginação skip/limit. **Sempre imprime `FindByOffset called with filter: …` em stdout** — diagnostique / remova se incomodar. Page > `MaxOffsetSkip/PerPage` é silenciosamente coercido para `skip = 0` (página 1). Filter vazio é permitido. |
| `FindByCursor(ctx, Map, *PaginationOpts, opts...) (*PaginatedResult[T], error)` | Paginação por cursor `_id` legada. Exige `pagination.UseCursor=true` e `SortDirection ∈ {1, -1}`. |
| `FindWithCursor(ctx, Map, *CursorOpts, projection) (*CursorResult[T], error)` | Nova paginação por cursor `_id` bidirecional com `Direction = CursorNext/CursorPrevious`, `SortAsc`, `Limit` padrão `300`. |
| `FindAndUpdateMany(ctx, Map, Map, opts...) (*UpdateResult, error)` | UpdateMany. Honra upsert/comment/let/etc. via opts. |
| `FindByIDAndUpdate(ctx, id, Map, opts...) (T, error)` | FindOneAndUpdate por ObjectID. |
| `FindOneAndUpdate(ctx, *Map, *Map, opts...) (*T, error)` | FindOneAndUpdate por filter. |
| `DeleteByID(ctx, id, opts...) error` | Resultado vazio → `ErrNotFound`. |
| `DeleteOne(ctx, *Map, opts...) error` | Resultado vazio → `ErrNotFound`. |
| `DeleteMany(ctx, Map, opts...) (int64, error)` | Filter vazio → `ErrEmptyFilters`. Resultado vazio → `ErrNotFound`. |

### `CommonOpts`

```go
type CommonOpts struct {
    // Compartilhados
    Session    *mongo.Session
    Projection interface{}
    Sort       interface{}
    Hint       interface{}
    Collation  *options.Collation

    // Específicos de update
    Upsert                   *bool
    ReturnDocument           *options.ReturnDocument
    BypassDocumentValidation *bool
    Comment                  interface{}
    MaxTime                  *time.Duration
    Let                      interface{}
    ArrayFilters             []interface{}
}
```

`applyCommonOptions` aplica o subset correspondente a quatro tipos suportados de builder: `FindOneOptionsBuilder`, `FindOptionsBuilder`, `FindOneAndUpdateOptionsBuilder`, `UpdateManyOptionsBuilder`. Builders não suportados passam intactos.

### Tipos de paginação

```go
type PaginationOpts struct {
    // Offset
    Page    int64
    PerPage int64

    // Cursor (legacy)
    CursorID      any  // ObjectID ou string
    SortDirection int  // 1 = forward, -1 = backward
    UseCursor     bool
}

type Pagination struct {
    Page, PerPage, TotalItems, TotalPages int64
    HasNext, HasPrev *bool
}

type PaginatedResult[T any] struct { Items []T; Pagination Pagination }

// FindWithCursor / CursorOpts / CursorResult — superfície nova
type CursorDirection string
const ( CursorNext CursorDirection = "next"; CursorPrevious = "previous" )

type CursorOpts struct { Cursor string; Direction CursorDirection; Limit int64; SortAsc bool }
type CursorResult[T any] struct { Items []T; NextCursor, PrevCursor string; HasNext, HasPrevious bool }
```

### Padrões de paginação

```go
const DefaultPage      int64 = 1
const DefaultPerPage   int64 = 25
const MaxOffsetPerPage int64 = 300
const MaxOffsetSkip    int64 = 500   // skip além disso vira 0 silenciosamente
```

### Aliases (re-exports)

```go
type Map               = bson.M
type ObjectId          = bson.ObjectID
type Collection        = mongo.Collection
type ReturnDoc         = options.ReturnDocument
type BulkWriteOptions  = options.BulkWriteOptionsBuilder
type WriteModel        = mongo.WriteModel
type UpdateResult      = mongo.UpdateResult
type DeleteResult      = mongo.DeleteResult
type BulkWriteResult   = mongo.BulkWriteResult

const ReturnDocOld = options.Before
const ReturnDocNew = options.After
```

### Utilitários

| Função | Efeito |
|---|---|
| `NewObjectID() bson.ObjectID` | Novo ObjectID único. |
| `StringToProjection("a, b, c") Map` | `{a:1, b:1, c:1}` (pula partes vazias). |
| `ToObjectID(any) (bson.ObjectID, error)` | Aceita `bson.ObjectID` ou string hex. Input ruim → `ErrInvalidID`. |
| `NewInsertOneModel`/`NewUpdateOneModel`/`NewReplaceOneModel` | Factories de modelo bulk-write (re-exports). |
| `BulkWrite()`/`FindOptions()` | Factories de options builder. |
| `IsDuplicateKeyError(err) bool` | Re-export de `mongo.IsDuplicateKeyError`. |

### Acessores de Map (`mapget.go`)

Acessores tipo-seguros para `map[string]interface{}` / `bson.M` (e transparentemente `bson.D`):

| Função | Retorna | Zero em miss/tipo errado |
|---|---|---|
| `MapGetString(m, key) string` | string | `""` |
| `MapGetInt(m, key) int` | int (lida com int/int32/int64/float64) | `0` |
| `MapGetBool(m, key) bool` | bool | `false` |
| `MapGetMap(m, key) map[string]any` | map aninhado (converte `bson.M` e `bson.D`) | `nil` |
| `MapGetSlice(m, key) []any` | slice (converte `bson.A`) | `nil` |
| `MapGetStringSlice(m, key) []string` | slice de strings filtrado | `nil` |
| `ToMap(val any) map[string]any` | converte `bson.M`/`bson.D` | `nil` |

### Transações (lado model)

```go
func (m *Model[T]) NewSession(ctx) (*mongo.Session, error)
func (m *Model[T]) RunTransaction(ctx, TransactionFunc) (any, error)
func (m *Model[T]) RunTransactionWithRetry(ctx, *mongo.Session, TransactionFunc) (any, error)
func (m *Model[T]) CommitWithRetry(ctx, *mongo.Session) error  // deprecated
```

`TransactionFunc` é alias do pacote manager. `RunTransaction` extrai o `*mongo.Client` via `m.col.Database().Client()` e chama `manager.RunTransactionWithClient` para que a lógica de retry fique em um único lugar. `CommitWithRetry` é um pass-through fino mantido para retrocompatibilidade.

### Erros

| Sentinel | Disparado |
|---|---|
| `ErrNotFound` | Nenhum documento bateu (Find / FindByID / Update / Delete) |
| `ErrInvalidID` | `ToObjectID` não conseguiu fazer parse |
| `ErrEmptyItems` | `CreateMany` com slice vazio |
| `ErrEmptyFilters` | `DeleteMany` com map vazio/nil |
| `ErrCursorPaginationRequired` | `FindByCursor` sem `UseCursor=true` |
| `ErrInvalidCursorDirection` | `FindByCursor` com `SortDirection ∉ {1, -1}` |
| `ErrNotConnected` | `Model.NewSession` quando o client é nil |

> O arquivo de erros se chama `erros.go` (typo no source — mesmo pacote, sem impacto para o caller).

## Exemplo end-to-end

```go
mgr, err := mongoManager.New(mongoManager.Config{
    URI:                "mongodb://localhost:27017",
    Database:           "mapex",
    EnableMonitor:      true,
    MonitorInterval:    10,                   // ← segundos, não time.Second (ver quirk)
    EnableBackpressure: true,
})
if err != nil { return err }
defer mgr.Close(ctx)

type User struct {
    ID      bson.ObjectID `bson:"_id,omitempty"`
    Email   string        `bson:"email"`
    Created time.Time     `bson:"created,omitempty"`
}

users := mongoModel.New[User](mgr.GetDatabase(), "users", mongoModel.Config{
    DefaultTimeout: 5 * time.Second,
    Indexes: []mongoModel.IndexDefinition{
        {Name: "idx_email_unique", Keys: map[string]int{"email": 1}, Unique: true},
    },
})

// CreateOne preenche _id e Created automaticamente.
u, err := users.CreateOne(ctx, &User{Email: "alice@example.com"})

// Paginação por cursor
res, err := users.FindWithCursor(ctx,
    mongoModel.Map{},
    &mongoModel.CursorOpts{Direction: mongoModel.CursorNext, Limit: 50, SortAsc: true},
    nil,
)

// Transação
result, err := users.RunTransaction(ctx, func(sessCtx context.Context) (any, error) {
    if _, err := users.CreateOne(sessCtx, &User{Email: "bob@example.com"}); err != nil {
        return nil, err // aborta
    }
    return "ok", nil
})

// Loop em batch ciente de backpressure
start := time.Now()
_, err = users.DIRECT().BulkWrite(ctx, models)
mgr.RecordWriteLatency(time.Since(start))
switch mgr.GetBackpressureMode() {
case mongoManager.Throttled: // halve next batch
case mongoManager.Backoff:   // halve + sleep
}
```
