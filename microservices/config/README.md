# config — Typed env-driven configuration singleton + canonical naming

Process-wide singleton that reads typed values from environment variables (with defaults) and exposes them via typed getters. On top of that, it offers high-level builders that return ready-to-use configs for the infrastructure adapters (Mongo, Redis, NATS, ClickHouse, Auth) and three pure helpers for canonical JetStream stream/subject/durable names.

> Package name: `configuration` (directory: `config/`).

## Surface

### Bootstrap

```go
type ConfigDefinition struct {
    Key     string      // logical key used by getters (e.g. "redis_host")
    Env     string      // environment variable name (e.g. "REDIS_HOST")
    Type    string      // "string" | "int" | "bool" | "array" | "json"
    Default interface{} // typed default — must match Type or InitConfig logs and skips this entry
}

func InitConfig(definitions []ConfigDefinition) *ConfigModule
```

`InitConfig` is `sync.Once`-guarded. The first call evaluates every definition and stores it in the singleton; subsequent calls return the existing instance without re-reading the environment. Unsupported `Type` values are logged via `log.Printf("Type not supported: …")` and skipped.

### Type contract per `Type`

| `Type` | Reader | Default kind expected | Notes |
|---|---|---|---|
| `"string"` | `getEnvString` | `string` | Empty env var falls back to default. |
| `"int"` | `getEnvInt` | `int` | Parse failure logs `Error converting … to int` and uses default. |
| `"bool"` | `getEnvBool` | `bool` | Same as int but with `strconv.ParseBool`. |
| `"array"` | `getEnvArray` | `[]string` | CSV split on `,`. No trimming. |
| `"json"` | `getEnvJSON` | `map[string]interface{}` | Parse failure logs `Error parsing …`. |

> The `Default` is type-asserted unconditionally inside `InitConfig`. A mismatched `Default` will panic at startup — define types and defaults consistently.

### Generic getters (panic if config is not initialised)

```go
func GetConfigValue(key string) interface{}     // raw value or nil if missing
func GetStringValue(key string) (string, error) // error on missing or wrong type
func GetIntValue(key string)    (int,    error)
func GetBoolValue(key string)   (bool,   error)
```

There is **no** `GetArrayValue` / `GetJSONValue` — array/JSON values are read via `GetConfigValue(key).([]string)` / `.(map[string]interface{})` at the call site.

## Builders for infrastructure configs

Every builder uses the shared keys above and assembles the right struct from the corresponding `infrastructure/*` package. None of them validate harder than the underlying getters — they will return zero-valued fields when keys are missing.

| Builder | Returns | Reads keys |
|---|---|---|
| `GetMongoConfig()` | `mongoManager.Config` | `mongo_uri`, `mongo_database`, `go_env` — Database is set to `"<go_env>-<mongo_database>"`. `EnableMonitor: true`, `MonitorInterval: 10`. |
| `GetAuthConfig()` | `middlewaresAuth.AuthConfig` | `auth_strategy`, `auth_secret`, `auth_jwks_url`, `auth_algorithm`, `auth_roles_source`, `auth_roles_path`, `auth_roles_api_url`. Logs validation errors via `logger.Error(nil, …)` for inconsistent combinations but **does not return an error**. Caller must ensure required keys exist. |
| `GetRedisConfig()` | `redisModel.Config` (service-private) | `service_name`, `go_env`, `redis_host`, `redis_port`, `redis_username`, `redis_password`, `redis_db`. `KeyPrefix = "<go_env>:<service_name>"`. |
| `GetSharedRedisConfig()` | `redisModel.Config` (cross-service) | `go_env`, `redis_host`, `redis_port`, `redis_username`, `redis_password`, `redis_shared_db`. `KeyPrefix = "<go_env>:shared"`. |
| `GetNatsConfig()` | `natsModel.Config` | `nats_url`, `nats_username`, `nats_password`, `nats_client_name`. `MaxReconnect: -1` (unlimited), `Timeout: 5s`. |
| `GetNatsCoreConfig()` | `natsModel.Config` | Same shape with `nats_core_*` keys — used for JetStream + domain events. |
| `GetNatsLeafConfig()` | `natsModel.Config` | Same shape with `nats_leaf_*` keys — used for the leaf node that handles MQTT clients (`auth_service` user must be allowed to subscribe to `$SYS.REQ.USER.AUTH` for Auth Callout). |
| `GetClickHouseConfig()` | `clickhouseModel.Config` | `clickhouse_host`, `clickhouse_port`, `clickhouse_database`, `clickhouse_username`, `clickhouse_password`. |
| `GetMyApiKey()` | `string` | `my_api_key`. Logs a Warn (`MY_API_KEY is not configured. All API key auth requests will be rejected.`) when empty. |

Notes worth flagging:

- `AuthConfig` validation is **best-effort logging**. Missing keys for a chosen strategy will not stop bootstrap — the auth middleware fails per-request later.
- `MongoConfig.Database` is **prefixed with `go_env`**, so the same Mongo cluster naturally splits dev/staging/prod databases.
- Redis private vs shared are differentiated by **DB number and KeyPrefix only**; everything else (host/port/credentials) is shared.

## Canonical naming helpers

These three helpers compose JetStream/NATS names from `go_env`, the service, and an action/context. They are pure functions (no external state) — `GetEnv()` is the only environment-aware piece.

```go
func GetEnv() string                       // "go_env" or "dev" (also "dev" if config not initialised)
func StreamName(service, context string) string  // "${ENV}-MAPEXOS-{SERVICE}[-{CONTEXT}]"
func Subject(service, action string) string     // "${env}.mapexos.{service}.{action}"
func Durable(service, context string) string    // "${env}-{service}-{context}-consumer"
```

Casing rules:

| Helper | Env case | Service/context case |
|---|---|---|
| `StreamName` | UPPER | UPPER |
| `Subject` | lower | lower |
| `Durable` | lower | lower |

`GetEnv()` is intentionally tolerant of an uninitialised singleton (returns `"dev"`) so naming constants can be computed at package init in tests that do not call `InitConfig`. This is the only getter with that property.

### Examples

```go
configuration.StreamName("ASSETS", "HEARTBEAT")  // "DEV-MAPEXOS-ASSETS-HEARTBEAT" (or PROD-… in prod)
configuration.StreamName("ASSETS", "")           // "DEV-MAPEXOS-ASSETS"
configuration.Subject("events", "save")           // "dev.mapexos.events.save"
configuration.Durable("events", "save")           // "dev-events-save-consumer"
```

## Bootstrap pattern

```go
configuration.InitConfig([]configuration.ConfigDefinition{
    {Key: "go_env",        Env: "GO_ENV",        Type: "string", Default: "dev"},
    {Key: "service_name",  Env: "SERVICE_NAME",  Type: "string", Default: "my-service"},

    {Key: "mongo_uri",     Env: "MONGO_URI",     Type: "string", Default: "mongodb://localhost:27017"},
    {Key: "mongo_database",Env: "MONGO_DB",      Type: "string", Default: "mapex"},

    {Key: "redis_host",    Env: "REDIS_HOST",    Type: "string", Default: "localhost"},
    {Key: "redis_port",    Env: "REDIS_PORT",    Type: "int",    Default: 6379},
    {Key: "redis_db",      Env: "REDIS_DB",      Type: "int",    Default: 0},
    {Key: "redis_shared_db", Env: "REDIS_SHARED_DB", Type: "int", Default: 5},
    // ... auth_*, nats_*, clickhouse_*, my_api_key
})

mongoCfg := configuration.GetMongoConfig()
redisCfg := configuration.GetRedisConfig()
streamName := configuration.StreamName("ASSETS", "HEARTBEAT") // "DEV-MAPEXOS-ASSETS-HEARTBEAT"
```

## Tested behaviours (`config_test.go`, `naming_test.go`)

- `getEnvString` / `getEnvBool` / `getEnvInt` / `getEnvArray` / `getEnvJSON` all fall back to the default on unset, empty, or invalid values; invalid bool/int/JSON logs a message but does not panic.
- `getEnvArray` splits on `,`; a single value returns a one-element slice.
- `InitConfig` populates the singleton; `GetStringValue` / `GetIntValue` / `GetBoolValue` round-trip the value.
- Missing key returns an error of the form `key %s not found`; wrong type returns `error converting %s to <type>`.
- `GetConfigValue` panics with `Config not initialized. Call InitConfig first.` when called before `InitConfig`.
- `GetMyApiKey` returns the configured key when set, returns `""` (and logs a Warn) when missing — never returns a hard-coded fallback.
- `GetEnv` returns `"dev"` when unset / when config singleton is nil; returns the explicit value when set.
- `StreamName`/`Subject`/`Durable` honour casing rules and the empty-context fallback for `StreamName`.

## Notes

- The singleton is **not reset** between calls. Tests use the unexported `resetConfigSingleton()` helper to reset state across test cases.
- Builders import several `infrastructure/*` packages and `microservices/http/middlewares/auth`. This makes `config` a leaf integration point — it imports a lot, but other packages should depend **on `config`**, not the other way around.
