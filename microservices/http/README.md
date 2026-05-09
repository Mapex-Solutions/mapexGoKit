# http — HTTP framework for Mapex services (Fiber-based)

Everything that turns a `*fiber.App` into a Mapex service: standard response shape, JWT/API-key auth, coverage-aware multi-tenant scoping, permission gating, organization-hierarchy validation, request validation, request timeout, refresh-token extraction, a `/health` endpoint with infra adapters, and shared error handling.

## Tree

```
http/
├── auth/                    Low-level auth primitives (used by middlewares)
├── customErrors/            Typed error wrappers + global Fiber error handler
├── health/                  /health endpoint + adapters per infra
├── middlewares/
│   ├── apiKey/              X-API-Key auth (fail-secure on empty key)
│   ├── auth/                JWT (HS*) and OAuth2 (RS*/JWKS) auth + GetUserIdFromToken
│   ├── contextInjector/     Per-request context.WithTimeout into c.UserContext
│   ├── coverage/            InjectRequestContext (multi-tenant scope from coverage cache)
│   ├── orghierarchy/        Hierarchy rule enforcement on org-create requests
│   ├── permission/          RequirePermission(s) — versioned auth-cache + lazy build
│   ├── refreshTokenExtractor/ Pulls X-Refresh-Token into Locals
│   └── resquestTimeout/     ⚠ directory has typo "resquest" — TimeoutMiddlewareFactory (504)
├── requestValidation/       Body/Query/Params DTO pipeline + GetDTO[T]
├── response/                Standard JSON response + helpers (Success/BadRequest/…)
└── status/                  HTTP status code constants
```

> Package-name vs directory mismatches you will see in imports:
> - `middlewares/auth` → `package middlewaresAuth`
> - `middlewares/apiKey` → `package middlewaresApiKeyAuth` (the `apiKey.go` file's doc comment incorrectly says `middlewaresAuth`)
> - `middlewares/contextInjector` → `package httpContextInjectorMiddleware`
> - `middlewares/coverage` → `package middlewaresCoverage`
> - `middlewares/orghierarchy` → `package middlewaresOrgHierarchy`
> - `middlewares/permission` → `package middlewaresPermission`
> - `middlewares/refreshTokenExtractor` → `package refreshTokenExtractor`
> - `middlewares/resquestTimeout` → `package httpRequestTimeoutMiddleware`

## `response` — Standard JSON shape + helpers

```go
type Response struct {
    Status int         `json:"status"`
    Errors []string    `json:"errors"`
    Data   interface{} `json:"data"`
}
```

Every handler that returns a body should use `response.*` so all services emit the same JSON.

| Helper | Status | Notes |
|---|---|---|
| `Custom(c, code, errors)` | `code` | Free-form. |
| `Success(c, data)` | `200` | Errors=nil. |
| `Created(c, data)` | `201` | Errors=nil. |
| `BadRequest(c, errors)` | `400` | Logs at error with URI+Method context. |
| `Conflict(c, errors)` | `409` | Logs at error with URI+Method. |
| `NotFound(c, err)` | `404` | Errors include `"Resource not found"` and the underlying error message. |
| `InternalServerError(c, message, err)` | `500` | Logs at error; if `message==""` uses `"Internal server error occurred"`. |
| `TimeoutServerError(c, message, err)` | `504` | Same logging shape as InternalServerError. |

## `status` — HTTP status code constants

A flat list of integer constants (`OK=200`, `CREATED=201`, …, `BAD_REQUEST=400`, `UNAUTHORIZED=401`, `FORBIDDEN=403`, `NOT_FOUND=404`, `INTERNAL_SERVER_ERROR=500`, `GATEWAY_TIMEOUT=504`, etc.). Use these instead of magic numbers when calling `response.Custom`.

## `customErrors` — Typed errors + global Fiber error handler

```go
type ValidationError struct { Errors []string }                // joined with "; "
type ServerCustomError struct { Code int; Errors []string }    // formatted "code=N: msg1; msg2"

func FiberErrorHandler(ctx *fiber.Ctx, err error) error
```

`FiberErrorHandler` is the single function you wire into `fiber.New(fiber.Config{ ErrorHandler: customErrors.FiberErrorHandler })`. Routing:

| Detected | Reply |
|---|---|
| `*fiber.Error` with `StatusGatewayTimeout` / `StatusRequestTimeout` | `response.TimeoutServerError` |
| `errors.Is(err, context.DeadlineExceeded \|\| context.Canceled)` | `response.TimeoutServerError` (`"Request timed out"`) |
| `errors.As(err, **ValidationError)` | `response.BadRequest` with the validation errors |
| `errors.As(err, **ServerCustomError)` | `response.Custom(code, errors)` |
| `*fiber.Error` with `StatusNotFound` | `response.NotFound` |
| anything else | `response.InternalServerError("Internal error", err)` |

The handler logs every incoming error at `Error` level before routing.

## `auth` — Low-level primitives (no Fiber middleware here)

```go
func ValidateAPIKey(c *fiber.Ctx, expectedKey, fieldType, fieldName string) bool
func ValidateIPWhitelist(c *fiber.Ctx, allowedIPs []string) bool
func ParseJWTTokenWithSecret(tokenString, secret, algorithm string) (*jwt.Token, jwt.MapClaims, error)
func ParseJWTTokenWithJWKS(tokenString, jwksURL string)            (*jwtv4.Token, jwtv4.MapClaims, error)
```

### `ValidateAPIKey`

Compares `expectedKey` against a single source — `fieldType` is **either** `"header"` or `"query"`, and `fieldName` is the header/parameter name. Body matching is **not implemented** despite older comments suggesting it.

### `ValidateIPWhitelist`

Resolves `c.IP()` and matches it against the allowlist. Each entry can be:

- An exact IPv4/IPv6 (mapped IPv6 like `::ffff:127.0.0.1` is normalised to v4 for comparison).
- A CIDR range (`192.168.0.0/24`, `::1/128`, etc.).
- The literal `"localhost"` or `"loopback"` — matches any loopback address.
- A hostname (anything containing letters) — DNS-resolved, then compared (note: this performs a real DNS lookup per request).

### JWT helpers

`ParseJWTTokenWithSecret` uses `golang-jwt/jwt/v5` (HS-family); `ParseJWTTokenWithJWKS` uses `golang-jwt/jwt/v4` + `MicahParks/keyfunc` with a **1-hour JWKS refresh interval**. The two return different `*jwt.Token` types — the auth middleware shallow-copies v4 claims into a v5 map so the rest of the code is uniform.

## `middlewares/auth` — JWT / OAuth2 middleware

```go
type AuthConfig struct {
    Strategy    string // "jwt" or "oauth2"
    Secret      string // jwt
    JWKSURL     string // oauth2
    Algorithm   string // e.g. "HS256" or "RS256"
    RolesPath   string // optional, used downstream
    RolesSource string // "token" | "db" | "api"
    RolesAPIURL string // when RolesSource == "api"
}

func AuthMiddleware(cfg AuthConfig) fiber.Handler
func GetUserIdFromToken(c *fiber.Ctx) (string, bool)
```

### `AuthMiddleware`

1. Reads the `Authorization` header. Missing → `401 missing Authorization header`.
2. Strips `Bearer ` prefix.
3. Parses according to `Strategy`:
   - `"jwt"` → `auth.ParseJWTTokenWithSecret(token, Secret, Algorithm)`.
   - `"oauth2"` → `auth.ParseJWTTokenWithJWKS(token, JWKSURL)`, claims are converted v4→v5.
   - anything else → `401 unsupported auth strategy` (note: returned as 401, not 500).
4. On any parse error → `401 invalid token` plus the underlying error message.
5. On success: `c.Locals("user", claims)` (claims are `jwtv5.MapClaims`) and `c.Locals("token", tokenString)`.

### `GetUserIdFromToken`

Reads `c.Locals("user")`, type-asserts to `jwtv5.MapClaims`, returns `(claims["userId"].(string), true)` or `("", false)` on any mismatch. Used by the coverage and permission middlewares.

## `middlewares/apiKey` — Single-header API key

```go
func ApiKeyAuthMiddleware(key string) fiber.Handler
```

- If `key == ""`, **every** request is rejected with `401 Unauthorized: API Key not configured on server` (fail-secure — there is intentionally no default key).
- Otherwise the request must send `X-API-Key: <key>`.

## `middlewares/contextInjector` — Per-request timeout context

```go
func ContextInjector(seconds int) fiber.Handler
```

Derives `context.WithTimeout(c.UserContext(), seconds)` and stores it back via `c.SetUserContext(ctx)` so downstream layers (DB calls, NATS publishes, …) inherit the deadline. Returns the next handler's error verbatim — it does **not** convert timeout into a 504; pair it with `resquestTimeout` if you also need a hard 504 reply.

## `middlewares/resquestTimeout` — Hard 504 wrapper

```go
func TimeoutMiddlewareFactory(seconds int) fiber.Handler
```

Wraps the chain with `fiber/middleware/timeout.NewWithContext` and replies with `504 Gateway Timeout` automatically when the deadline expires. Avoids manual goroutines (no data races). Use this when you need a hard guarantee that the request will not exceed `N` seconds.

> Directory name has a typo (`resquestTimeout` → should be `requestTimeout`). Package name is `httpRequestTimeoutMiddleware`. The file name is also `requestTImeout.go`.

## `middlewares/refreshTokenExtractor` — Pull `X-Refresh-Token`

```go
const RefreshTokenLocalKey   = "refreshToken"
const RefreshTokenHeaderName = "X-Refresh-Token"

func RefreshTokenExtractor() fiber.Handler
```

Reads `X-Refresh-Token` (raw or `Bearer <token>`) and stores it under `c.Locals("refreshToken")`. **Does not validate.** Apply only on routes that need it (e.g. `/auth/refresh`).

## `middlewares/coverage` — Multi-tenant scope from coverage cache

This middleware is the foundation for the standardised list-endpoint pattern. It produces `*context.RequestContext` (defined in `microservices/common/context`) and stores it under `c.Locals("requestContext")`.

### Bootstrap (call once at startup)

```go
func InitCoverageMiddleware(sharedCache common.SharedCache)
func InitCacheBuildClient(baseURL, apiKey string)
```

- `InitCoverageMiddleware` injects the shared Redis cache used to read coverage data.
- `InitCacheBuildClient` supplies the internal HTTP API used to **build** coverage on cache miss (timeout: **30 s**, longer than the permission cache because hierarchy expansion is heavier).

### Middleware

```go
func InjectRequestContext() fiber.Handler
```

Flow:

1. Pull `userId` from JWT (via `auth.GetUserIdFromToken`). Missing → `401`.
2. Try `coverage:user:{userId}` from the shared cache. On `redis.Nil`, call the internal API (`POST /internal/auth/build-coverage`) with `X-API-Key`, expect `data.organizations`, fail with 500 if anything goes wrong.
3. Read `X-Org-Context`. **Required for every non-ROOT user** — only users with the `mapex.*` permission may query without it (returns `403 X-Org-Context header required.` otherwise). Detection of ROOT happens via `hasRootPermissions`, which:
   - Reads `auth:org:global:user:{userId}:ver` from the shared cache.
   - If present, reads `auth:org:global:user:{userId}:v{N}` with **10× retry** (`200 ms` apart, total 2 s) to cover a known race window where the version pointer is updated before the versioned data lands.
   - Considers the user ROOT if any returned permission equals `mapex.*`.
4. If `X-Org-Context` is set, validate it against `userAccess.AccessibleOrgIds` via `orgfilter.ValidateOrgContext`. Not in scope → `403`. Then look up the detailed `CoverageOrg` via `orgfilter.FindOrgInCoverage` (must be present — defensive 403 if missing).
5. ROOT users with no org context get `ScopedOrgIds = []` (empty array meaning "no filter"); everyone else gets `userAccess.AccessibleOrgIds`.
6. Build and inject:

```go
&context.RequestContext{
    ScopedOrgIds:   scopedOrgIds,
    CoverageOrgs:   coverageOrgs,
    OrgContext:     orgContext,
    OrgContextData: orgContextData,
    UserId:         userId,
}
```

### Constants

```go
var RootPermissions = []string{"mapex.*"} // ONLY this is treated as ROOT
```

## `middlewares/permission` — Versioned permission cache + lazy build

### Bootstrap

```go
func InitPermissionMiddleware(sharedCache common.SharedCache, baseURL, apiKey string)
// internally: globalSharedCache = sharedCache; InitCacheBuildClient(baseURL, apiKey)
```

The internal HTTP client uses a **10 s** timeout (vs 30 s for coverage).

### Middleware

```go
func RequirePermission(permission string) fiber.Handler
func RequirePermissions(requiredPermissions ...string) fiber.Handler
```

Flow per request:

1. `userId` from JWT (`401` if missing).
2. Read `X-Org-Context`. Missing → fall back to a **global permissions check**:
   - Fetch the user's permissions in the global scope (`orgId=""` → cache key normalises to `global`).
   - Allow only if the user holds `mapex.*` (`RootPermission`), `admin_vendor.*`, or `admin_customer.*`.
   - Otherwise → `403 Organization context required (X-Org-Context header missing)`.
3. Otherwise read the org-scoped permissions and allow if `hasAnyPermission(userPerms, requiredPermissions)` — see precedence rules below.

### Permission match precedence (`matchesPermission`)

In order:

1. `mapex.*` (ROOT) → match anything.
2. `admin_vendor.*` → match anything (vendor scope).
3. `admin_customer.*` → match anything (customer scope).
4. `admin.*` → match anything.
5. Exact match.
6. Wildcard `<resource>.*` matches `<resource>.<anything>`.

### Cache layout

```
auth:org:{orgId}:user:{userId}:ver       → integer (1..N)
auth:org:{orgId}:user:{userId}:v{N}      → []string permissions (TTL 30 days)
```

`orgId == ""` is normalised to `global`.

### Lazy build + retry semantics

`getUserPermissionsWithVersioning`:

1. Read `:ver`. `redis.Nil` → call `POST /internal/auth/build-authorization` and return the freshly built permissions.
2. Parse the integer version. Read `:v{N}` with **10× retry × 200 ms** (max 2 s) to handle the race window where the version pointer is updated before the data is written.
3. If the versioned key is still missing after retries, call the build API once more to repair.
4. Build failures are logged but **fail-open as empty permissions** (so a bad cache build does not freeze the service — middleware will then reply `403 insufficient permissions`).

### Constants

```go
const RootPermission           = "mapex.*"
const AdminVendorPermission    = "admin_vendor.*"
const AdminCustomerPermission  = "admin_customer.*"
const AdminPermission          = "admin.*"
const CacheKeyFormat           = "auth:org:%s:user:%s"
```

## `middlewares/orghierarchy` — Enforce vendor → customer → site → building → floor → zone

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

The middleware is DTO-agnostic — consumers provide an `extractor` that pulls their concrete create-DTO out of `c.Locals("bodyDTO")` and adapts it to `OrganizationCreateContract`.

### Order in the chain

Must run **after** `requestValidation.ValidationMiddleware` (so the body DTO is in `Locals`) and **after** `coverage.InjectRequestContext` (so `requestContext` is available).

### Validations

- `GetParentOrgID() == nil` → only `vendor` is allowed (root organization).
- Else, `parentOrgID` must be in the user's `CoverageOrgs` (else `403`).
- If parent type is `zone` → `400` (zone is the leaf).
- Otherwise the requested type must equal `ValidChildTypes[parentType]` (else `400`).
- Unknown parent type → `400`.

## `requestValidation` — Body/Query/Params DTO pipeline

A Fiber-flavoured equivalent of the standalone `microservices/validator` package: parse + apply defaults + validate + deep-transform, all per-request.

### Bootstrap of the validator instance

A package-level singleton (`validateInstance = validations.New()`) is shared by every helper. Register custom validations / struct-level validations / change configuration **before** the validator runs concurrently (typically `init()` or bootstrap).

```go
func RegisterValidation(tag string, fn validator.Func) error
func RegisterStructValidation(fn validator.StructLevelFunc, t any)
func Engine() *validator.Validate         // raw access for advanced cases
func Struct(v any) error                  // validates a struct
func Var(v any, tag string) error         // validates a single value
```

### Middleware factory

```go
type Validation struct{ /* unexported reflect.Type triple */ }

func NewValidation(bodyDTO, queryDTO, paramsDTO interface{}) Validation
func ValidationMiddleware(v Validation) fiber.Handler
```

Each non-nil DTO is allocated via reflection (`reflect.New(...).Interface()`), parsed via `c.BodyParser` / `c.QueryParser` / `c.ParamsParser`, defaulted (`creasty/defaults`), validated (`utils/validations.ValidateStruct`), and finally walked for deep transforms (post-order, `time.Time` skipped). Failures reply with `400` and prefixed errors:

```
[BODY_VALIDATION]   …
[QUERY_VALIDATION]  …
[PARAMS_VALIDATION] …
```

DTOs are stored under `c.Locals("bodyDTO")`, `"queryDTO"`, `"paramsDTO"`.

### Retrieval helper

```go
func GetDTO[T any](c *fiber.Ctx, key string) (T, error)
```

Reads `c.Locals(key)` and asserts to `T`. Missing/wrong type → `H130: invalid or missing <key>`.

### `DTOTransformer`

```go
type DTOTransformer interface { Transform() error }
```

Same contract as in `microservices/validator`, but **a separate copy** in this package (do not type-assert across the two). The walker skips unexported fields and `time.Time`, and prefers the addressable `*T` form before falling back to value-form `T`.

## `health` — `/health` endpoint with infra adapters

### Surface

```go
type Config struct {
    ServiceName string
    Version     string
    CacheTTL    time.Duration   // default: 10s
    Timeout     time.Duration   // default:  5s
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

### Behaviour

- `Service.Check` runs every checker **in parallel** with a per-call `context.WithTimeout(cfg.Timeout)`, then caches the response for `cfg.CacheTTL`.
- Status aggregation: `unhealthy` if any *critical* check is down; `degraded` if any non-critical check is down; `healthy` otherwise.
- HTTP: `Handler` returns `200` for healthy/degraded, `503 Service Unavailable` for unhealthy. The body is always wrapped in `response.Response.Data`.
- `RegisterRoutes` mounts `GET /health` on the supplied app **before** any global middleware, so it requires no auth.

### Adapters (`health/adapters/*.go`)

| File | `Name()` | Implementation |
|---|---|---|
| `clickhouse.go` | `clickhouse` | Reads `manager.IsConnected()` + `LastLatency()` (zero-cost — relies on the chManager monitor goroutine). |
| `mongodb.go` | `mongodb` | Same pattern, reads the mongoManager monitor (zero-cost). |
| `nats.go` | `nats:<name>` | Calls `client.Ping()` (native PING/PONG). |
| `redis.go` | `redis:<name>` | Calls `client.Ping(ctx)` and measures wall-clock latency. |
| `minio.go` | `minio:<name>` | Calls `client.Ping(ctx)` (which `ListBuckets`) and measures latency. |

Pattern when wiring a service:

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

## End-to-end pattern

```go
app := fiber.New(fiber.Config{ ErrorHandler: customErrors.FiberErrorHandler })

// /health (no auth)
health.RegisterRoutes(app, healthCfg, healthCheckers...)

// Global middlewares
app.Use(httpRequestTimeoutMiddleware.TimeoutMiddlewareFactory(30))

// Bootstrap middleware singletons
middlewaresCoverage.InitCoverageMiddleware(sharedCache)
middlewaresCoverage.InitCacheBuildClient(internalAPIBase, internalAPIKey)
middlewaresPermission.InitPermissionMiddleware(sharedCache, internalAPIBase, internalAPIKey)

// Group with auth + permission + coverage + validation
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
