# microservices — Mapex service framework

Shared scaffolding every Mapex Go service builds on top of: typed env-driven configuration, structured logging, DI container, lifecycle/module bootstrap, graceful shutdown, Prometheus metrics, generic DTO validation, and a full Fiber-based HTTP layer (auth, permissions, coverage scoping, hierarchy validation, request validation, health endpoint, standardised responses).

## Subpackages

| Path | What it gives you | When you need it |
|---|---|---|
| [`common`](./common) | 4-phase `ModuleConfig`, `Mountable` lifecycle hook, `ContextIDs`, request-scope `RequestContext` + `CoverageOrg` (in `common/context`) | Every service — these are the cross-cutting types module bootstrap and middleware contracts depend on. |
| [`config`](./config) | Env-driven typed config singleton + builders for `mongoManager`, `redisModel`, `natsModel` (core/leaf), `clickhouseModel`, `middlewaresAuth`. Pure naming helpers (`StreamName`, `Subject`, `Durable`). | Bootstrap. Call `configuration.InitConfig(...)` before anything else. |
| [`container`](./container) | Singleton wrapper around `go.uber.org/dig` plus `In`/`Out`/`Name`/`Group` re-exports | Wherever you wire dependencies. |
| [`http`](./http) | Fiber framework: `auth/`, `customErrors/`, `health/` (with infra adapters), 8 middlewares, `requestValidation/`, `response/`, `status/` | Any service exposing HTTP. |
| [`logger`](./logger) | Singleton structured logger over `rs/zerolog` with package-level helpers | Every service. Call `logger.InitLogger(...)` early. |
| [`metrics`](./metrics) | Per-service Prometheus registry + Fiber `/metrics` endpoint | Services that expose Prometheus metrics. |
| [`module`](./module) | Legacy 3-phase `ModuleConfig`. Superseded by `common.ModuleConfig` for anything with NATS listeners. | Existing modules using the older shape. New code should prefer `common`. |
| [`shutdown`](./shutdown) | Signal-driven graceful shutdown manager with priority bands and concurrent execution per band | Bootstrap of any long-running process. |
| [`validator`](./validator) | Generic DTO unmarshal + defaults + validate + deep transform pipeline (HTTP-agnostic) | Anywhere outside HTTP that handles DTOs (NATS, files, etc.). |

## Typical bootstrap order

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

// 3. DI container
c := container.InitContainer()

// 4. Infra clients (Mongo / Redis / NATS / ClickHouse / MinIO)
//    Construct via configuration.Get*Config builders → register in c.

// 5. Metrics + health
metricsReg := metrics.NewRegistry(serviceName)
metricsReg.EnableGoCollector()
metricsReg.EnableProcessCollector()

app := fiber.New(fiber.Config{ ErrorHandler: customErrors.FiberErrorHandler })
metricsReg.RegisterEndpoint(app)
health.RegisterRoutes(app, healthCfg, healthCheckers...)

// 6. HTTP middleware singletons
middlewaresCoverage.InitCoverageMiddleware(sharedCache)
middlewaresCoverage.InitCacheBuildClient(internalAPIBase, internalAPIKey)
middlewaresPermission.InitPermissionMiddleware(sharedCache, internalAPIBase, internalAPIKey)

// 7. Modules (4-phase)
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

## Cross-cutting conventions

- **Logging prefix.** Every infra/middleware adapter logs with a bracketed prefix (`[INFRA:MONGODB]`, `[INFRA:NATS]`, `[INFRA:CLICKHOUSE]`, `[INFRA:MINIO]`, `[INFRA:REDIS]`, `[MIDDLEWARE:Coverage]`, `[MIDDLEWARE:Permission]`, `[MIDDLEWARE:OrgHierarchy]`, `[SHUTDOWN]`, …). Grep for these to follow a request.
- **Standard response.** All HTTP handlers should return through `microservices/http/response`, which yields `{ status, errors, data }`. The customErrors handler emits the same shape on errors.
- **Multi-tenant scope.** `coverage.InjectRequestContext` is the single source of truth for "who can see what". Handlers read `c.Locals("requestContext")` and pass it to `utils/orgfilter.BuildOrgFilter`.
- **Permission checks.** `permission.RequirePermission(s)` enforces gating. ROOT (`mapex.*`) bypasses org-context requirement; `admin_vendor.*` / `admin_customer.*` short-circuit per-permission matches.
- **Module bootstrap.** Use `common.ModuleConfig` (4 phases) for new modules. The legacy 3-phase `module.ModuleConfig` is kept for compatibility but never use it for code that subscribes to NATS.

## Environment naming (recap)

`configuration.GetEnv()` is the single source for environment-derived names. The three pure helpers below let services name JetStream artefacts canonically without leaking `GO_ENV` into code:

```go
configuration.StreamName("ASSETS", "HEARTBEAT") // "DEV-MAPEXOS-ASSETS-HEARTBEAT" (or PROD-…)
configuration.Subject("events", "save")          // "dev.mapexos.events.save"
configuration.Durable("events", "save")          // "dev-events-save-consumer"
```

For details on every subpackage — types, defaults, edge cases, tested behaviours — see the README in each directory.
