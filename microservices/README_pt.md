# microservices — Framework de serviços Mapex

Andaime compartilhado em cima do qual todo serviço Go do Mapex é construído: configuração tipada via env, logging estruturado, container DI, bootstrap de lifecycle/módulos, shutdown gracioso, métricas Prometheus, validação genérica de DTOs e uma camada HTTP completa baseada em Fiber (auth, permissões, escopo por coverage, validação de hierarquia, validação de request, endpoint de health, respostas padronizadas).

## Subpacotes

| Caminho | O que oferece | Quando precisa |
|---|---|---|
| [`common`](./common) | `ModuleConfig` de 4 fases, hook `Mountable`, `ContextIDs`, escopo de request `RequestContext` + `CoverageOrg` (em `common/context`) | Todo serviço — são os tipos transversais de que dependem o bootstrap de módulos e os contratos de middleware. |
| [`config`](./config) | Singleton tipado de config via env + builders para `mongoManager`, `redisModel`, `natsModel` (core/leaf), `clickhouseModel`, `middlewaresAuth`. Helpers puros de naming (`StreamName`, `Subject`, `Durable`). | Bootstrap. Chame `configuration.InitConfig(...)` antes de tudo. |
| [`container`](./container) | Wrapper singleton sobre `go.uber.org/dig` com re-exports `In`/`Out`/`Name`/`Group` | Onde quer que se conectem dependências. |
| [`http`](./http) | Framework Fiber: `auth/`, `customErrors/`, `health/` (com adapters de infra), 8 middlewares, `requestValidation/`, `response/`, `status/` | Qualquer serviço que exponha HTTP. |
| [`logger`](./logger) | Logger estruturado singleton sobre `rs/zerolog` com helpers de pacote | Todo serviço. Chame `logger.InitLogger(...)` cedo. |
| [`metrics`](./metrics) | Registry Prometheus por serviço + endpoint Fiber `/metrics` | Serviços que expõem métricas Prometheus. |
| [`module`](./module) | `ModuleConfig` legada de 3 fases. Substituída por `common.ModuleConfig` para qualquer módulo com listeners NATS. | Módulos existentes usando o formato antigo. Código novo deve preferir `common`. |
| [`shutdown`](./shutdown) | Manager de shutdown gracioso dirigido por sinal, com bandas de prioridade e execução concorrente dentro da banda | Bootstrap de qualquer processo de longa vida. |
| [`validator`](./validator) | Pipeline DTO genérico: unmarshal + defaults + validação + deep transform (sem HTTP) | Onde quer que se manipulem DTOs fora de HTTP (NATS, arquivos, etc.). |

## Ordem típica de bootstrap

```go
// 1. Config
configuration.InitConfig(definitions)

// 2. Logger
logger.InitLogger(logger.LoggerOptions{
    ServiceName:    serviceName,
    ServiceVersion: version,
    Environment:    configuration.GetEnv(),
    Level:          logger.InfoLevel,
})

// 3. Container DI
c := container.InitContainer()

// 4. Clientes de infra (Mongo / Redis / NATS / ClickHouse / MinIO)
//    Construa via builders configuration.Get*Config → registre em c.

// 5. Metrics + health
metricsReg := metrics.NewRegistry(serviceName)
metricsReg.EnableGoCollector()
metricsReg.EnableProcessCollector()

app := fiber.New(fiber.Config{ ErrorHandler: customErrors.FiberErrorHandler })
metricsReg.RegisterEndpoint(app)
health.RegisterRoutes(app, healthCfg, healthCheckers...)

// 6. Singletons dos middlewares HTTP
middlewaresCoverage.InitCoverageMiddleware(sharedCache)
middlewaresCoverage.InitCacheBuildClient(internalAPIBase, internalAPIKey)
middlewaresPermission.InitPermissionMiddleware(sharedCache, internalAPIBase, internalAPIKey)

// 7. Módulos (4 fases)
for _, m := range modules {
    if m.InitRepositories != nil { m.InitRepositories() }
}
for _, m := range modules {
    if m.InitServices != nil { m.InitServices() }
}
for _, m := range modules {
    if m.InitInterfaces != nil { m.InitInterfaces() }
}
for _, m := range modules {
    if m.InitListeners != nil { m.InitListeners() }
}

// 8. Shutdown manager
sm := shutdown.New()
sm.RegisterFunc("http",       0, func(ctx context.Context) error { return app.ShutdownWithContext(ctx) })
sm.Register   ("nats",        1, natsClient)
sm.Register   ("mongo",       5, mongoMgr)
sm.Register   ("redis-app",   5, redisApp)

go app.Listen(":" + httpPort)
sm.WaitForSignal(20 * time.Second)
```

## Convenções transversais

- **Prefixo de log.** Todo adapter de infra/middleware loga com prefixo entre colchetes (`[INFRA:MONGODB]`, `[INFRA:NATS]`, `[INFRA:CLICKHOUSE]`, `[INFRA:MINIO]`, `[INFRA:REDIS]`, `[MIDDLEWARE:Coverage]`, `[MIDDLEWARE:Permission]`, `[MIDDLEWARE:OrgHierarchy]`, `[SHUTDOWN]`, …). Use grep nesses prefixos para seguir um request.
- **Resposta padrão.** Todos os handlers HTTP devem retornar via `microservices/http/response`, que produz `{ status, errors, data }`. O customErrors handler emite o mesmo formato em caso de erro.
- **Escopo multi-tenant.** `coverage.InjectRequestContext` é a única fonte da verdade para "quem pode ver o quê". Handlers leem `c.Locals("requestContext")` e passam para `utils/orgfilter.BuildOrgFilter`.
- **Checagem de permissão.** `permission.RequirePermission(s)` faz o gating. ROOT (`mapex.*`) ignora a obrigatoriedade de org-context; `admin_vendor.*` / `admin_customer.*` fazem short-circuit por permissão.
- **Bootstrap de módulo.** Use `common.ModuleConfig` (4 fases) para módulos novos. A `module.ModuleConfig` legada de 3 fases é mantida por compatibilidade mas nunca use em código que assina NATS.

## Naming por ambiente (recap)

`configuration.GetEnv()` é a fonte única para nomes derivados de ambiente. Os três helpers puros abaixo permitem aos serviços nomear artefatos JetStream canônicamente sem espalhar `GO_ENV` pelo código:

```go
configuration.StreamName("ASSETS", "HEARTBEAT") // "DEV-MAPEXOS-ASSETS-HEARTBEAT" (ou PROD-…)
configuration.Subject("events", "save")          // "dev.mapexos.events.save"
configuration.Durable("events", "save")          // "dev-events-save-consumer"
```

Para detalhes de cada subpacote — tipos, defaults, edge cases, comportamentos testados — veja o README em cada diretório.
