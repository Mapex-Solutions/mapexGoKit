# module — Legacy `ModuleConfig` (3-phase)

A single type — `ModuleConfig` — describing the original 3-phase module bootstrap shape used across Mapex services. The 4-phase successor (which adds `InitListeners`) lives in `microservices/common`; both still exist in the codebase. New modules should prefer the `common` variant.

> Package name: `module` (directory: `module/`).

## What is here

```go
type ModuleConfig struct {
    Name string

    // Reserved for future lazy-loading; not implemented yet.
    Lazy bool

    InitRepositories func() // phase 1 — register repositories in DIG
    InitServices     func() // phase 2 — register services in DIG
    InitInterfaces   func() // phase 3 — register HTTP routes and consumers
}
```

All `Init*` callbacks are optional — leaving any of them `nil` skips that phase.

## How modules use it

Each module defines a function that returns a `ModuleConfig`; the application's bootstrap walks all modules calling `InitRepositories` first, then `InitServices`, then `InitInterfaces`. This keeps DI registration order deterministic across heterogeneous services.

```go
package mymodule

func Module() module.ModuleConfig {
    return module.ModuleConfig{
        Name: "mymodule",
        InitRepositories: func() { /* container.Provide(NewRepository) */ },
        InitServices:     func() { /* container.Provide(NewService) */ },
        InitInterfaces:   func() { /* router.Register(NewHandler) */ },
    }
}
```

## Relationship with `common.ModuleConfig`

`microservices/common.ModuleConfig` is the same shape **plus** a fourth phase:

```go
InitListeners func() // phase 4 — start NATS listeners AFTER all modules are ready
```

Use the `common` variant for any module that subscribes to NATS events; otherwise the order between "service ready in DIG" and "listener started" is not guaranteed.
