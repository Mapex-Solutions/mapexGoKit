# common — Tipos transversais para bootstrap de módulo e escopo de request

Tipos compartilhados pelo framework de microsserviços: um `ModuleConfig` de 4 fases, um carrier genérico de identificadores, hooks opcionais de ciclo de vida de serviço, e (em `common/context/`) o formato de escopo multi-tenant de request que o middleware de auth/coverage produz.

> Nome dos pacotes: `common` (raiz) e `context` (em `common/context/`).

## O que tem aqui

```
common/
├── moduleConfig.type.go   → ModuleConfig (4 fases, sucessor de microservices/module)
├── lifecycle.go           → interface Mountable + helper RunLifecycleHooks
├── context.type.go        → ContextIDs (par de ponteiros UserID/TenantID)
└── context/
    └── request_context.go → RequestContext + CoverageOrg (escopo de auth + multi-tenant)
```

## `ModuleConfig` — bootstrap de módulo em 4 fases

```go
type ModuleConfig struct {
    Name             string
    Lazy             bool   // reservado — lazy-loading não implementado

    InitRepositories func() // 1 — registra repositórios no DIG
    InitServices     func() // 2 — registra serviços no DIG
    InitInterfaces   func() // 3 — registra rotas HTTP e consumers
    InitListeners    func() // 4 — inicia listeners NATS APÓS todos os módulos inicializados
}
```

Todos os `Init*` são opcionais. O formato de 4 fases sucede `microservices/module.ModuleConfig`; use a variante de 3 fases apenas se o módulo realmente não tiver listeners NATS e a ordem de bootstrap for irrelevante.

O bootstrap deve iterar **todos os módulos por fase** antes de avançar — a fase 4 (`InitListeners`) só deve acontecer depois que todos os repositórios, serviços e interfaces de todos os módulos estiverem ligados; caso contrário os listeners podem receber eventos antes dos handlers estarem prontos.

## `Mountable` — hook opcional de ciclo de vida do serviço

```go
type Mountable interface { OnMount() }

func RunLifecycleHooks(service interface{}, moduleName string)
```

Um serviço que implementa `OnMount()` recebe a chamada após a construção, quando suas dependências estão conectadas. Use para bootstrap de schedules, seed de dados, ou publishes iniciais.

```go
c.Invoke(func(svc ports.MyServicePort) {
    common.RunLifecycleHooks(svc, "MyModule")
})
```

`RunLifecycleHooks` é seguro com qualquer valor — serviços que não implementam `Mountable` são pulados silenciosamente. Quando o hook dispara, loga `[MODULE:<moduleName>] Running OnMount lifecycle hook` em info.

## `ContextIDs` — par genérico de identificadores

```go
type ContextIDs struct {
    UserID   *string
    TenantID *string
}
```

Um carrier pequeno para lugares onde um `RequestContext` tipado é exagero (jobs em background, helpers internos). Ponteiros `nil` significam "não informado".

## `context.RequestContext` — escopo de request (auth + coverage)

O subpacote `common/context` carrega o escopo de autenticação e multi-tenant por request que os handlers precisam downstream. É populado por middleware (tipicamente o de coverage) e armazenado em `c.Locals("requestContext")`.

```go
type RequestContext struct {
    ScopedOrgIds   []string       // toda org que o usuário pode acessar (do cache de coverage)
    CoverageOrgs   []CoverageOrg  // as mesmas orgs com pathKey hierárquico
    OrgContext     *string        // header X-Org-Context — nil = sem org específica
    OrgContextData *CoverageOrg   // dados detalhados de OrgContext (incl. pathKey)
    UserId         string         // usuário autenticado (do JWT)
}

type CoverageOrg struct {
    ID      string // hex do ObjectId MongoDB
    Name    string
    Type    string // customer, vendor, site, building, zone, …
    PathKey string // path hierárquico — ex: "000001/0001/0001/001"
}
```

### Produtor (middleware)

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

### Consumidor (handler)

```go
reqContext := c.Locals("requestContext").(*context.RequestContext)

// Use ScopedOrgIds para queries $in quando OrgContext específico não foi informado.
// Use OrgContextData.PathKey para queries hierárquicas (range).
```

Por que `PathKey` vive aqui: ele já está no cache de coverage, então handlers podem fazer filtragem hierárquica sem uma segunda query.

## Nota de layout

Este pacote mistura algumas preocupações (bootstrap de módulo + lifecycle + contexto de request) intencionalmente — todas são "andaime compartilhado que não pertence a um subsistema único". Se um arquivo crescer ao ponto de virar um conceito próprio, espere que ele seja promovido a um subpacote dedicado (como `context/` já é).
