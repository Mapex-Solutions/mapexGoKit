# http — Framework HTTP para serviços Mapex (baseado em Fiber)

Tudo o que transforma um `*fiber.App` em um serviço Mapex: formato padrão de resposta, auth JWT/API key, escopo multi-tenant via coverage, gating por permissão, validação de hierarquia organizacional, validação de request, timeout de request, extração de refresh token, endpoint `/health` com adapters de infra e tratamento compartilhado de erros.

## Árvore

```
http/
├── auth/                    Primitivas de auth de baixo nível (usadas pelos middlewares)
├── customErrors/            Tipos de erro + handler global do Fiber
├── health/                  Endpoint /health + adapters por infra
├── middlewares/
│   ├── apiKey/              Auth por X-API-Key (fail-secure quando key vazia)
│   ├── auth/                Auth JWT (HS*) e OAuth2 (RS*/JWKS) + GetUserIdFromToken
│   ├── contextInjector/     context.WithTimeout por request em c.UserContext
│   ├── coverage/            InjectRequestContext (escopo multi-tenant via coverage cache)
│   ├── orghierarchy/        Regras de hierarquia em criação de organização
│   ├── permission/          RequirePermission(s) — auth-cache versionado + lazy build
│   ├── refreshTokenExtractor/ Põe X-Refresh-Token em Locals
│   └── resquestTimeout/     ⚠ diretório com typo "resquest" — TimeoutMiddlewareFactory (504)
├── requestValidation/       Pipeline DTO de Body/Query/Params + GetDTO[T]
├── response/                Resposta JSON padrão + helpers (Success/BadRequest/…)
└── status/                  Constantes de HTTP status
```

> Mismatches entre nome de pacote e diretório que aparecerão nos imports:
> - `middlewares/auth` → `package middlewaresAuth`
> - `middlewares/apiKey` → `package middlewaresApiKeyAuth` (o doc comment de `apiKey.go` diz `middlewaresAuth` por engano)
> - `middlewares/contextInjector` → `package httpContextInjectorMiddleware`
> - `middlewares/coverage` → `package middlewaresCoverage`
> - `middlewares/orghierarchy` → `package middlewaresOrgHierarchy`
> - `middlewares/permission` → `package middlewaresPermission`
> - `middlewares/refreshTokenExtractor` → `package refreshTokenExtractor`
> - `middlewares/resquestTimeout` → `package httpRequestTimeoutMiddleware`

## `response` — Formato JSON padrão + helpers

```go
type Response struct {
    Status int         `json:"status"`
    Errors []string    `json:"errors"`
    Data   interface{} `json:"data"`
}
```

Todo handler que retorna body deve usar `response.*` para que todos os serviços emitam o mesmo JSON.

| Helper | Status | Notas |
|---|---|---|
| `Custom(c, code, errors)` | `code` | Livre. |
| `Success(c, data)` | `200` | Errors=nil. |
| `Created(c, data)` | `201` | Errors=nil. |
| `BadRequest(c, errors)` | `400` | Loga em error com URI+Method. |
| `Conflict(c, errors)` | `409` | Loga em error com URI+Method. |
| `NotFound(c, err)` | `404` | Errors incluem `"Resource not found"` e a mensagem do erro. |
| `InternalServerError(c, message, err)` | `500` | Loga em error; se `message==""` usa `"Internal server error occurred"`. |
| `TimeoutServerError(c, message, err)` | `504` | Mesmo formato de log de InternalServerError. |

## `status` — Constantes de status HTTP

Lista plana de constantes inteiras (`OK=200`, `CREATED=201`, …, `BAD_REQUEST=400`, `UNAUTHORIZED=401`, `FORBIDDEN=403`, `NOT_FOUND=404`, `INTERNAL_SERVER_ERROR=500`, `GATEWAY_TIMEOUT=504`, etc.). Use ao invés de números mágicos quando chamar `response.Custom`.

## `customErrors` — Erros tipados + handler global do Fiber

```go
type ValidationError struct { Errors []string }                // joined com "; "
type ServerCustomError struct { Code int; Errors []string }    // formato "code=N: msg1; msg2"

func FiberErrorHandler(ctx *fiber.Ctx, err error) error
```

`FiberErrorHandler` é a função única que você liga em `fiber.New(fiber.Config{ ErrorHandler: customErrors.FiberErrorHandler })`. Roteamento:

| Detectado | Resposta |
|---|---|
| `*fiber.Error` com `StatusGatewayTimeout` / `StatusRequestTimeout` | `response.TimeoutServerError` |
| `errors.Is(err, context.DeadlineExceeded \|\| context.Canceled)` | `response.TimeoutServerError` (`"Request timed out"`) |
| `errors.As(err, **ValidationError)` | `response.BadRequest` com os erros de validação |
| `errors.As(err, **ServerCustomError)` | `response.Custom(code, errors)` |
| `*fiber.Error` com `StatusNotFound` | `response.NotFound` |
| qualquer outro | `response.InternalServerError("Internal error", err)` |

O handler loga toda entrada em `Error` antes do roteamento.

## `auth` — Primitivas de baixo nível (sem middleware Fiber aqui)

```go
func ValidateAPIKey(c *fiber.Ctx, expectedKey, fieldType, fieldName string) bool
func ValidateIPWhitelist(c *fiber.Ctx, allowedIPs []string) bool
func ParseJWTTokenWithSecret(tokenString, secret, algorithm string) (*jwt.Token, jwt.MapClaims, error)
func ParseJWTTokenWithJWKS(tokenString, jwksURL string)            (*jwtv4.Token, jwtv4.MapClaims, error)
```

### `ValidateAPIKey`

Compara `expectedKey` contra uma única fonte — `fieldType` é **ou** `"header"` ou `"query"`, e `fieldName` é o nome do header/parâmetro. Match em body **não está implementado** apesar de comentários antigos sugerirem.

### `ValidateIPWhitelist`

Resolve `c.IP()` e compara com a allowlist. Cada entrada pode ser:

- IPv4/IPv6 exato (mapeados como `::ffff:127.0.0.1` são normalizados para v4 antes da comparação).
- CIDR (`192.168.0.0/24`, `::1/128`, etc.).
- Literal `"localhost"` ou `"loopback"` — match qualquer endereço de loopback.
- Hostname (qualquer string com letras) — DNS-resolvido e comparado (executa lookup DNS real por request).

### Helpers JWT

`ParseJWTTokenWithSecret` usa `golang-jwt/jwt/v5` (família HS); `ParseJWTTokenWithJWKS` usa `golang-jwt/jwt/v4` + `MicahParks/keyfunc` com **refresh interval de 1 hora** no JWKS. Os dois retornam tipos `*jwt.Token` diferentes — o middleware de auth faz uma cópia rasa de claims v4 para v5 para o resto do código ficar uniforme.

## `middlewares/auth` — Middleware JWT / OAuth2

```go
type AuthConfig struct {
    Strategy    string // "jwt" ou "oauth2"
    Secret      string // jwt
    JWKSURL     string // oauth2
    Algorithm   string // ex: "HS256" ou "RS256"
    RolesPath   string // opcional, usado downstream
    RolesSource string // "token" | "db" | "api"
    RolesAPIURL string // quando RolesSource == "api"
}

func AuthMiddleware(cfg AuthConfig) fiber.Handler
func GetUserIdFromToken(c *fiber.Ctx) (string, bool)
```

### `AuthMiddleware`

1. Lê o header `Authorization`. Ausente → `401 missing Authorization header`.
2. Remove o prefixo `Bearer `.
3. Faz parse de acordo com `Strategy`:
   - `"jwt"` → `auth.ParseJWTTokenWithSecret(token, Secret, Algorithm)`.
   - `"oauth2"` → `auth.ParseJWTTokenWithJWKS(token, JWKSURL)`, claims convertidas v4→v5.
   - qualquer outro → `401 unsupported auth strategy` (note: 401, não 500).
4. Em qualquer erro de parse → `401 invalid token` mais a mensagem do erro subjacente.
5. Em sucesso: `c.Locals("user", claims)` (claims são `jwtv5.MapClaims`) e `c.Locals("token", tokenString)`.

### `GetUserIdFromToken`

Lê `c.Locals("user")`, faz type-assert para `jwtv5.MapClaims`, retorna `(claims["userId"].(string), true)` ou `("", false)` em qualquer mismatch. Usado pelos middlewares de coverage e permission.

## `middlewares/apiKey` — API key por header único

```go
func ApiKeyAuthMiddleware(key string) fiber.Handler
```

- Se `key == ""`, **toda** request é rejeitada com `401 Unauthorized: API Key not configured on server` (fail-secure — intencionalmente sem default).
- Caso contrário a request precisa enviar `X-API-Key: <key>`.

## `middlewares/contextInjector` — Timeout context por request

```go
func ContextInjector(seconds int) fiber.Handler
```

Deriva `context.WithTimeout(c.UserContext(), seconds)` e armazena via `c.SetUserContext(ctx)` para que camadas downstream (DB, NATS, …) herdem o deadline. Retorna o erro do próximo handler verbatim — **não** converte timeout em 504; combine com `resquestTimeout` se também precisar de uma resposta 504 hard.

## `middlewares/resquestTimeout` — Wrapper hard de 504

```go
func TimeoutMiddlewareFactory(seconds int) fiber.Handler
```

Envolve a cadeia com `fiber/middleware/timeout.NewWithContext` e responde `504 Gateway Timeout` automaticamente quando o deadline expira. Sem goroutine manual (sem data races). Use quando precisar de garantia hard de que o request não passa de `N` segundos.

> O nome do diretório tem typo (`resquestTimeout` → deveria ser `requestTimeout`). Pacote é `httpRequestTimeoutMiddleware`. O arquivo se chama `requestTImeout.go`.

## `middlewares/refreshTokenExtractor` — Pega `X-Refresh-Token`

```go
const RefreshTokenLocalKey   = "refreshToken"
const RefreshTokenHeaderName = "X-Refresh-Token"

func RefreshTokenExtractor() fiber.Handler
```

Lê `X-Refresh-Token` (cru ou `Bearer <token>`) e armazena em `c.Locals("refreshToken")`. **Não valida.** Aplique apenas em rotas que precisam (ex: `/auth/refresh`).

## `middlewares/coverage` — Escopo multi-tenant via coverage cache

Esse middleware é a fundação do padrão de list-endpoint padronizado. Produz `*context.RequestContext` (definido em `microservices/common/context`) e armazena em `c.Locals("requestContext")`.

### Bootstrap (chamar uma vez no startup)

```go
func InitCoverageMiddleware(sharedCache common.SharedCache)
func InitCacheBuildClient(baseURL, apiKey string)
```

- `InitCoverageMiddleware` injeta o cache Redis compartilhado usado para ler dados de coverage.
- `InitCacheBuildClient` informa a API HTTP interna usada para **construir** coverage em cache miss (timeout: **30 s**, maior que o do permission cache porque expansão hierárquica é mais pesada).

### Middleware

```go
func InjectRequestContext() fiber.Handler
```

Fluxo:

1. Pega `userId` do JWT (`auth.GetUserIdFromToken`). Ausente → `401`.
2. Tenta `coverage:user:{userId}` no cache compartilhado. Em `redis.Nil`, chama a API interna (`POST /internal/auth/build-coverage`) com `X-API-Key`, espera `data.organizations`, falha com 500 se algo der errado.
3. Lê `X-Org-Context`. **Obrigatório para todo usuário não-ROOT** — apenas usuários com permissão `mapex.*` podem consultar sem ele (caso contrário retorna `403 X-Org-Context header required.`). Detecção de ROOT é via `hasRootPermissions`, que:
   - Lê `auth:org:global:user:{userId}:ver` no cache compartilhado.
   - Se presente, lê `auth:org:global:user:{userId}:v{N}` com **10× retry** (`200 ms` cada, total 2 s) para cobrir uma janela de race conhecida onde o version pointer é atualizado antes do dado versionado chegar.
   - Considera o usuário ROOT se qualquer permissão retornada for `mapex.*`.
4. Se `X-Org-Context` foi setado, valida contra `userAccess.AccessibleOrgIds` via `orgfilter.ValidateOrgContext`. Fora de escopo → `403`. Depois busca o `CoverageOrg` detalhado via `orgfilter.FindOrgInCoverage` (defensivo: 403 se ausente).
5. Usuários ROOT sem org context recebem `ScopedOrgIds = []` (array vazio = "sem filtro"); todos os outros recebem `userAccess.AccessibleOrgIds`.
6. Constrói e injeta:

```go
&context.RequestContext{
    ScopedOrgIds:   scopedOrgIds,
    CoverageOrgs:   coverageOrgs,
    OrgContext:     orgContext,
    OrgContextData: orgContextData,
    UserId:         userId,
}
```

### Constantes

```go
var RootPermissions = []string{"mapex.*"} // SOMENTE este é tratado como ROOT
```

## `middlewares/permission` — Cache versionado de permissões + lazy build

### Bootstrap

```go
func InitPermissionMiddleware(sharedCache common.SharedCache, baseURL, apiKey string)
// internamente: globalSharedCache = sharedCache; InitCacheBuildClient(baseURL, apiKey)
```

O HTTP client interno usa timeout de **10 s** (vs 30 s do coverage).

### Middleware

```go
func RequirePermission(permission string) fiber.Handler
func RequirePermissions(requiredPermissions ...string) fiber.Handler
```

Fluxo por request:

1. `userId` do JWT (`401` se ausente).
2. Lê `X-Org-Context`. Ausente → fallback para **checagem global de permissões**:
   - Busca permissões do usuário no escopo global (`orgId=""` → cache key normalizado para `global`).
   - Permite apenas se o usuário tem `mapex.*` (`RootPermission`), `admin_vendor.*` ou `admin_customer.*`.
   - Caso contrário → `403 Organization context required (X-Org-Context header missing)`.
3. Caso contrário lê permissões org-scoped e permite se `hasAnyPermission(userPerms, requiredPermissions)` — ver regras de precedência abaixo.

### Precedência de match (`matchesPermission`)

Em ordem:

1. `mapex.*` (ROOT) → match qualquer coisa.
2. `admin_vendor.*` → match qualquer coisa (escopo vendor).
3. `admin_customer.*` → match qualquer coisa (escopo customer).
4. `admin.*` → match qualquer coisa.
5. Match exato.
6. Wildcard `<resource>.*` casa com `<resource>.<qualquer>`.

### Layout do cache

```
auth:org:{orgId}:user:{userId}:ver       → integer (1..N)
auth:org:{orgId}:user:{userId}:v{N}      → []string permissões (TTL 30 dias)
```

`orgId == ""` é normalizado para `global`.

### Lazy build + retry

`getUserPermissionsWithVersioning`:

1. Lê `:ver`. `redis.Nil` → chama `POST /internal/auth/build-authorization` e retorna as permissões recém-construídas.
2. Faz parse do version inteiro. Lê `:v{N}` com **10× retry × 200 ms** (máximo 2 s) para tratar a janela de race onde o version pointer é atualizado antes do dado ser escrito.
3. Se a chave versionada continua ausente após retries, chama a API de build novamente para reparar.
4. Falhas no build são logadas mas **fail-open com permissões vazias** (para que cache build ruim não congele o serviço — o middleware responde `403 insufficient permissions` em seguida).

### Constantes

```go
const RootPermission           = "mapex.*"
const AdminVendorPermission    = "admin_vendor.*"
const AdminCustomerPermission  = "admin_customer.*"
const AdminPermission          = "admin.*"
const CacheKeyFormat           = "auth:org:%s:user:%s"
```

## `middlewares/orghierarchy` — Hierarquia vendor → customer → site → building → floor → zone

```go
type OrganizationCreateContract interface {
    GetType() string
    GetParentOrgID() *string
}

type DTOExtractor func(c *fiber.Ctx) (OrganizationCreateContract, error)

func ValidateOrgHierarchy(extractor DTOExtractor) fiber.Handler

var ValidChildTypes = map[string]string{
    "vendor":   "customer",
    "customer": "site",
    "site":     "building",
    "building": "floor",
    "floor":    "zone",
}
```

### Wiring

O middleware é DTO-agnóstico — consumers fornecem um `extractor` que pega o create-DTO concreto de `c.Locals("bodyDTO")` e adapta para `OrganizationCreateContract`.

### Ordem na cadeia

Deve rodar **depois** de `requestValidation.ValidationMiddleware` (para o body DTO estar em `Locals`) e **depois** de `coverage.InjectRequestContext` (para `requestContext` estar disponível).

### Validações

- `GetParentOrgID() == nil` → apenas `vendor` é permitido (organização raiz).
- Caso contrário, `parentOrgID` deve estar em `CoverageOrgs` do usuário (caso contrário `403`).
- Se o tipo do parent é `zone` → `400` (zone é folha).
- Caso contrário o tipo solicitado deve ser igual a `ValidChildTypes[parentType]` (caso contrário `400`).
- Tipo de parent desconhecido → `400`.

## `requestValidation` — Pipeline DTO de Body/Query/Params

Versão Fiber-flavored do pacote standalone `microservices/validator`: parse + aplica defaults + valida + deep-transform, por request.

### Bootstrap da instância de validador

Um singleton de pacote (`validateInstance = validations.New()`) é compartilhado por todo helper. Registre custom validations / struct-level validations / mude config **antes** do validador rodar concorrentemente (tipicamente `init()` ou bootstrap).

```go
func RegisterValidation(tag string, fn validator.Func) error
func RegisterStructValidation(fn validator.StructLevelFunc, t any)
func Engine() *validator.Validate         // acesso cru para casos avançados
func Struct(v any) error                  // valida uma struct
func Var(v any, tag string) error         // valida um valor único
```

### Factory de middleware

```go
type Validation struct{ /* triple unexported de reflect.Type */ }

func NewValidation(bodyDTO, queryDTO, paramsDTO interface{}) Validation
func ValidationMiddleware(v Validation) fiber.Handler
```

Cada DTO não-nil é alocado via reflection (`reflect.New(...).Interface()`), parseado por `c.BodyParser` / `c.QueryParser` / `c.ParamsParser`, com defaults (`creasty/defaults`), validado (`utils/validations.ValidateStruct`), e por fim percorrido para deep transforms (pós-ordem, `time.Time` pulado). Falhas respondem com `400` e erros prefixados:

```
[BODY_VALIDATION]   …
[QUERY_VALIDATION]  …
[PARAMS_VALIDATION] …
```

DTOs ficam em `c.Locals("bodyDTO")`, `"queryDTO"`, `"paramsDTO"`.

### Helper de retrieval

```go
func GetDTO[T any](c *fiber.Ctx, key string) (T, error)
```

Lê `c.Locals(key)` e faz assert para `T`. Ausente/tipo errado → `H130: invalid or missing <key>`.

### `DTOTransformer`

```go
type DTOTransformer interface { Transform() error }
```

Mesmo contrato de `microservices/validator`, mas **uma cópia separada** neste pacote (não faça type-assert entre os dois). O walker pula fields unexported e `time.Time`, e prefere a forma endereçável `*T` antes do fallback para a forma valor `T`.

## `health` — Endpoint `/health` com adapters de infra

### Superfície

```go
type Config struct {
    ServiceName string
    Version     string
    CacheTTL    time.Duration   // padrão: 10s
    Timeout     time.Duration   // padrão:  5s
}
type CheckerConfig struct { Checker Checker; Critical bool }
type Checker interface { Name() string; Check(ctx) common.HealthStatus }

type Response struct {
    Status      string                  // "healthy" | "degraded" | "unhealthy"
    Service     string
    Version     string
    Uptime      string
    Timestamp   time.Time
    LastCheckAt time.Time
    Checks      map[string]CheckDetail
}
type CheckDetail struct { Connected bool; Critical bool; LatencyMs int64; ErrorMessage string }

func NewService(cfg Config, checkers ...CheckerConfig) *Service
func RegisterRoutes(app *fiber.App, cfg Config, checkers ...CheckerConfig) *Service
func Handler(service *Service) fiber.Handler
```

### Comportamento

- `Service.Check` roda todo checker **em paralelo** com um `context.WithTimeout(cfg.Timeout)` por chamada, depois cacheia a resposta por `cfg.CacheTTL`.
- Agregação de status: `unhealthy` se qualquer check *crítico* está down; `degraded` se qualquer check não-crítico está down; `healthy` caso contrário.
- HTTP: `Handler` retorna `200` para healthy/degraded, `503 Service Unavailable` para unhealthy. O body é sempre embrulhado em `response.Response.Data`.
- `RegisterRoutes` monta `GET /health` no app **antes** de qualquer middleware global, então não exige auth.

### Adapters (`health/adapters/*.go`)

| Arquivo | `Name()` | Implementação |
|---|---|---|
| `clickhouse.go` | `clickhouse` | Lê `manager.IsConnected()` + `LastLatency()` (zero-cost — depende da goroutine de monitor do chManager). |
| `mongodb.go` | `mongodb` | Mesmo padrão, lê o monitor do mongoManager (zero-cost). |
| `nats.go` | `nats:<name>` | Chama `client.Ping()` (PING/PONG nativo). |
| `redis.go` | `redis:<name>` | Chama `client.Ping(ctx)` e mede latência wall-clock. |
| `minio.go` | `minio:<name>` | Chama `client.Ping(ctx)` (que faz `ListBuckets`) e mede latência. |

Padrão de wiring de um serviço:

```go
health.RegisterRoutes(app, health.Config{
    ServiceName: "auth-service",
    Version:     buildInfo.Version,
}, []health.CheckerConfig{
    { Checker: adapters.NewMongoAdapter(mongoMgr),                    Critical: true  },
    { Checker: adapters.NewRedisAdapter(redisApp, "app"),             Critical: true  },
    { Checker: adapters.NewRedisAdapter(redisShared, "shared"),       Critical: false },
    { Checker: adapters.NewNATSAdapter(natsClient, "core"),           Critical: true  },
    { Checker: adapters.NewClickHouseAdapter(clickhouseMgr),          Critical: false },
    { Checker: adapters.NewMinIOAdapter(minioClient, "templates"),    Critical: false },
}...)
```

## Padrão end-to-end

```go
app := fiber.New(fiber.Config{ ErrorHandler: customErrors.FiberErrorHandler })

// /health (sem auth)
health.RegisterRoutes(app, healthCfg, healthCheckers...)

// Middlewares globais
app.Use(httpRequestTimeoutMiddleware.TimeoutMiddlewareFactory(30))

// Bootstrap dos singletons de middleware
middlewaresCoverage.InitCoverageMiddleware(sharedCache)
middlewaresCoverage.InitCacheBuildClient(internalAPIBase, internalAPIKey)
middlewaresPermission.InitPermissionMiddleware(sharedCache, internalAPIBase, internalAPIKey)

// Group com auth + permission + coverage + validation
api := app.Group("/api/v1",
    middlewaresAuth.AuthMiddleware(authCfg),
)

api.Get("/orgs",
    middlewaresPermission.RequirePermission("organization.list"),
    middlewaresCoverage.InjectRequestContext(),
    requestValidation.ValidationMiddleware(requestValidation.NewValidation(nil, &orgListQuery{}, nil)),
    handlers.ListOrgs,
)

api.Post("/orgs",
    middlewaresPermission.RequirePermission("organization.create"),
    requestValidation.ValidationMiddleware(requestValidation.NewValidation(&orgCreateBody{}, nil, nil)),
    middlewaresCoverage.InjectRequestContext(),
    middlewaresOrgHierarchy.ValidateOrgHierarchy(func(c *fiber.Ctx) (middlewaresOrgHierarchy.OrganizationCreateContract, error) {
        return requestValidation.GetDTO[*orgCreateBody](c, "bodyDTO")
    }),
    handlers.CreateOrg,
)
```
