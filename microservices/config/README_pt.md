# config — Singleton tipado de configuração via env + nomenclatura canônica

Singleton de escopo de processo que lê valores tipados de variáveis de ambiente (com defaults) e expõe via getters tipados. Por cima oferece builders de alto nível que retornam configs prontas dos adapters de infraestrutura (Mongo, Redis, NATS, ClickHouse, Auth) e três funções puras para nomes canônicos de JetStream stream/subject/durable.

> Nome do pacote: `configuration` (diretório: `config/`).

## Superfície

### Bootstrap

```go
type ConfigDefinition struct {
    Key     string      // chave lógica usada pelos getters (ex: "redis_host")
    Env     string      // variável de ambiente (ex: "REDIS_HOST")
    Type    string      // "string" | "int" | "bool" | "array" | "json"
    Default interface{} // default tipado — deve casar com Type, ou InitConfig loga e pula esta entrada
}

func InitConfig(definitions []ConfigDefinition) *ConfigModule
```

`InitConfig` é protegido por `sync.Once`. A primeira chamada avalia toda definição e armazena no singleton; chamadas subsequentes retornam a instância existente sem reler o ambiente. Valores `Type` não suportados são logados via `log.Printf("Type not supported: …")` e pulados.

### Contrato por `Type`

| `Type` | Leitor | Tipo do Default esperado | Notas |
|---|---|---|---|
| `"string"` | `getEnvString` | `string` | Var vazia cai no default. |
| `"int"` | `getEnvInt` | `int` | Falha de parse loga `Error converting … to int` e usa default. |
| `"bool"` | `getEnvBool` | `bool` | Igual a int mas com `strconv.ParseBool`. |
| `"array"` | `getEnvArray` | `[]string` | Split por `,`. Sem trim. |
| `"json"` | `getEnvJSON` | `map[string]interface{}` | Falha de parse loga `Error parsing …`. |

> O `Default` é cast incondicional dentro de `InitConfig`. Um `Default` com tipo errado faz panic no startup — defina types e defaults consistentes.

### Getters genéricos (panic se config não foi inicializado)

```go
func GetConfigValue(key string) interface{}     // valor cru, ou nil se ausente
func GetStringValue(key string) (string, error) // erro em ausência ou tipo errado
func GetIntValue(key string)    (int,    error)
func GetBoolValue(key string)   (bool,   error)
```

**Não** existem `GetArrayValue` / `GetJSONValue` — valores array/JSON são lidos via `GetConfigValue(key).([]string)` / `.(map[string]interface{})` no call site.

## Builders para configs de infra

Todo builder usa as chaves compartilhadas e monta a struct correta do pacote `infrastructure/*` correspondente. Nenhum valida mais do que o getter subjacente — campos em zero-value se chaves estão ausentes.

| Builder | Retorna | Lê chaves |
|---|---|---|
| `GetMongoConfig()` | `mongoManager.Config` | `mongo_uri`, `mongo_database`, `go_env` — Database é `"<go_env>-<mongo_database>"`. `EnableMonitor: true`, `MonitorInterval: 10`. |
| `GetAuthConfig()` | `middlewaresAuth.AuthConfig` | `auth_strategy`, `auth_secret`, `auth_jwks_url`, `auth_algorithm`, `auth_roles_source`, `auth_roles_path`, `auth_roles_api_url`. Loga erros de validação via `logger.Error(nil, …)` em combinações inconsistentes mas **não retorna erro**. O caller deve garantir as chaves obrigatórias. |
| `GetRedisConfig()` | `redisModel.Config` (privado do serviço) | `service_name`, `go_env`, `redis_host`, `redis_port`, `redis_username`, `redis_password`, `redis_db`. `KeyPrefix = "<go_env>:<service_name>"`. |
| `GetSharedRedisConfig()` | `redisModel.Config` (cross-service) | `go_env`, `redis_host`, `redis_port`, `redis_username`, `redis_password`, `redis_shared_db`. `KeyPrefix = "<go_env>:shared"`. |
| `GetNatsConfig()` | `natsModel.Config` | `nats_url`, `nats_username`, `nats_password`, `nats_client_name`. `MaxReconnect: -1` (ilimitado), `Timeout: 5s`. |
| `GetNatsCoreConfig()` | `natsModel.Config` | Mesmo formato com chaves `nats_core_*` — usado para JetStream + eventos de domínio. |
| `GetNatsLeafConfig()` | `natsModel.Config` | Mesmo formato com chaves `nats_leaf_*` — usado pelo leaf node que atende clientes MQTT (usuário `auth_service` precisa de permissão para subscrever em `$SYS.REQ.USER.AUTH` para Auth Callout). |
| `GetClickHouseConfig()` | `clickhouseModel.Config` | `clickhouse_host`, `clickhouse_port`, `clickhouse_database`, `clickhouse_username`, `clickhouse_password`. |
| `GetMyApiKey()` | `string` | `my_api_key`. Loga Warn (`MY_API_KEY is not configured. All API key auth requests will be rejected.`) quando vazio. |

Notas a destacar:

- A validação de `AuthConfig` é **best-effort por log**. Chaves ausentes para a estratégia escolhida não param o bootstrap — o middleware de auth falha por request depois.
- `MongoConfig.Database` é **prefixado por `go_env`**, então o mesmo cluster Mongo separa dev/staging/prod por database naturalmente.
- Redis privado vs compartilhado se diferenciam apenas por **número de DB e KeyPrefix**; todo o resto (host/porta/credenciais) é o mesmo.

## Helpers de nomenclatura canônica

Esses três helpers compõem nomes JetStream/NATS a partir de `go_env`, do serviço e de uma action/context. São funções puras (sem estado externo) — `GetEnv()` é o único pedaço env-aware.

```go
func GetEnv() string                       // "go_env" ou "dev" (também "dev" se config não inicializado)
func StreamName(service, context string) string  // "${ENV}-MAPEXOS-{SERVICE}[-{CONTEXT}]"
func Subject(service, action string) string     // "${env}.mapexos.{service}.{action}"
func Durable(service, context string) string    // "${env}-{service}-{context}-consumer"
```

Regras de capitalização:

| Helper | Env case | Service/context case |
|---|---|---|
| `StreamName` | UPPER | UPPER |
| `Subject` | lower | lower |
| `Durable` | lower | lower |

`GetEnv()` é intencionalmente tolerante a singleton não inicializado (retorna `"dev"`) para que constantes de naming possam ser computadas em init de pacote em testes que não chamam `InitConfig`. É o único getter com essa propriedade.

### Exemplos

```go
configuration.StreamName("ASSETS", "HEARTBEAT")  // "DEV-MAPEXOS-ASSETS-HEARTBEAT" (ou PROD-… em prod)
configuration.StreamName("ASSETS", "")           // "DEV-MAPEXOS-ASSETS"
configuration.Subject("events", "save")           // "dev.mapexos.events.save"
configuration.Durable("events", "save")           // "dev-events-save-consumer"
```

## Padrão de bootstrap

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

## Comportamentos testados (`config_test.go`, `naming_test.go`)

- `getEnvString` / `getEnvBool` / `getEnvInt` / `getEnvArray` / `getEnvJSON` caem no default em valor unset/vazio/inválido; bool/int/JSON inválidos logam mensagem mas não fazem panic.
- `getEnvArray` faz split em `,`; valor único retorna slice de um elemento.
- `InitConfig` popula o singleton; `GetStringValue` / `GetIntValue` / `GetBoolValue` round-trip o valor.
- Chave ausente retorna erro `key %s not found`; tipo errado retorna `error converting %s to <type>`.
- `GetConfigValue` faz panic com `Config not initialized. Call InitConfig first.` quando chamado antes de `InitConfig`.
- `GetMyApiKey` retorna a chave configurada quando setada, retorna `""` (e loga Warn) quando ausente — nunca retorna fallback hardcoded.
- `GetEnv` retorna `"dev"` quando unset / quando o singleton é nil; retorna valor explícito quando setado.
- `StreamName`/`Subject`/`Durable` respeitam regras de capitalização e fallback de context vazio em `StreamName`.

## Notas

- O singleton **não é resetado** entre chamadas. Testes usam o helper unexported `resetConfigSingleton()` para resetar estado entre casos.
- Builders importam vários pacotes `infrastructure/*` e `microservices/http/middlewares/auth`. Isso torna `config` um ponto de integração folha — importa muito, mas outros pacotes devem depender **de `config`**, não o contrário.
