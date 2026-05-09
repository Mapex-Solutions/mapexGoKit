# common — Cross-cutting types for module bootstrap and request scope

Shared types used across the microservices framework: a 4-phase `ModuleConfig`, a generic identifier carrier, optional service lifecycle hooks, and (in `common/context/`) the multi-tenant request-scope shape that auth/coverage middleware produces.

> Package name: `common` (root) and `context` (in `common/context/`).

## What is here

```
common/
├── moduleConfig.type.go   → ModuleConfig (4 phases, supersedes microservices/module)
├── lifecycle.go           → Mountable interface + RunLifecycleHooks helper
├── context.type.go        → ContextIDs (UserID/TenantID pointer pair)
└── context/
    └── request_context.go → RequestContext + CoverageOrg (auth + multi-tenant scope)
```

## `ModuleConfig` — 4-phase module bootstrap

```go
type ModuleConfig struct {
    Name             string
    Lazy             bool   // reserved — lazy-loading not implemented

    InitRepositories func() // 1 — register repositories in DIG
    InitServices     func() // 2 — register services in DIG
    InitInterfaces   func() // 3 — register HTTP routes and consumers
    InitListeners    func() // 4 — start NATS listeners AFTER all modules initialised
}
```

All `Init*` are optional. The 4-phase shape supersedes `microservices/module.ModuleConfig`; use the 3-phase variant only if your module truly has no NATS listeners and you don't need to depend on bootstrap order.

The bootstrap should iterate **all modules per phase** before advancing — phase 4 (`InitListeners`) must happen only after every module's repositories, services and interfaces are wired, otherwise listeners may receive events before their handlers are ready.

## `Mountable` — optional service lifecycle hook

```go
type Mountable interface { OnMount() }

func RunLifecycleHooks(service interface{}, moduleName string)
```

A service that implements `OnMount()` will receive the call after construction, when its dependencies are wired. Use it for bootstrap schedules, seed data, or initial publishes.

```go
c.Invoke(func(svc ports.MyServicePort) {
    common.RunLifecycleHooks(svc, "MyModule")
})
```

`RunLifecycleHooks` is safe on any value — services that do not implement `Mountable` are skipped silently. When the hook fires it logs `[MODULE:<moduleName>] Running OnMount lifecycle hook` at info level.

## `ContextIDs` — generic identifier pair

```go
type ContextIDs struct {
    UserID   *string
    TenantID *string
}
```

A small carrier used in places where a typed `RequestContext` is overkill (background jobs, internal helpers). `nil` pointers mean "not provided".

## `context.RequestContext` — auth + coverage request scope

The `common/context` subpackage carries the per-request authentication and multi-tenant scope that downstream handlers need. It is populated by middleware (typically the coverage middleware) and stored under `c.Locals("requestContext")`.

```go
type RequestContext struct {
    ScopedOrgIds   []string       // every org the user can access (from coverage cache)
    CoverageOrgs   []CoverageOrg  // same orgs but with hierarchical pathKey
    OrgContext     *string        // X-Org-Context header — nil = no specific org
    OrgContextData *CoverageOrg   // detailed data for OrgContext (incl. pathKey)
    UserId         string         // authenticated user (from JWT)
}

type CoverageOrg struct {
    ID      string // MongoDB ObjectId hex
    Name    string
    Type    string // customer, vendor, site, building, zone, …
    PathKey string // hierarchical path — e.g. "000001/0001/0001/001"
}
```

### Producer (middleware)

```go
reqContext := &context.RequestContext{
    ScopedOrgIds:   coverage.AccessibleOrgIds,
    CoverageOrgs:   coverageOrgs,
    OrgContext:     orgContextPtr,
    OrgContextData: orgContextData,
    UserId:         userId,
}
c.Locals("requestContext", reqContext)
```

### Consumer (handler)

```go
reqContext := c.Locals("requestContext").(*context.RequestContext)

// Use ScopedOrgIds for $in queries when no specific OrgContext is provided.
// Use OrgContextData.PathKey for hierarchical range queries.
```

Why `PathKey` lives here: it is already in the coverage cache, so handlers can do hierarchical filtering without a second query.

## Layout note

This package mixes a couple of concerns (module bootstrap + lifecycle + request context) intentionally — they are all "shared scaffolding that does not belong to any single subsystem". If a file grows large enough to be its own concept, expect it to graduate to a dedicated subpackage (as `context/` already has).
