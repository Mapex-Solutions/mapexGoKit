# container — Singleton DI container (uber/dig wrapper)

Thin wrapper around [`go.uber.org/dig`](https://github.com/uber-go/dig). Exposes a process-wide singleton `*dig.Container` and re-exports the most commonly used dig primitives so service code does not need to import dig directly.

> Package name: `container` (directory: `container/`).

## Surface

### Singleton lifecycle (`container.go`, `methods.go`)

```go
func InitContainer() *dig.Container  // creates the singleton on first call (sync.Once); returns the same instance afterwards
func GetContainer() *dig.Container   // returns the singleton — nil before InitContainer is called
```

`InitContainer` is safe to call concurrently — internal `sync.Once` guarantees a single allocation.

### Re-exports (`types.go`)

| Re-export | Underlying | Purpose |
|---|---|---|
| `type In = dig.In`   | struct embed marker | Named/grouped parameters in providers |
| `type Out = dig.Out` | struct embed marker | Multiple values from a single constructor |
| `var Name = dig.Name`| `dig.Name(string)` option | Provide a named dependency |
| `var Group = dig.Group`| `dig.Group(string)` option | Provide a grouped dependency |

## Usage

### Bootstrap

```go
c := container.InitContainer()
c.Provide(redisModel.New)
c.Provide(natsModel.New)
// ... in InitServices, InitInterfaces, etc.
```

### Named injection (`In`)

```go
c.Provide(func(params struct {
    container.In
    RC *redisModel.RedisClient `name:"app"`
}) common.AppCache {
    return params.RC
})
```

### Multi-value provider (`Out`)

```go
c.Provide(func() (struct {
    container.Out
    AppCache    common.Cache `name:"app"`
    SharedCache common.Cache `name:"shared"`
}) {
    // construct + return both
})
```

### Named provider

```go
c.Provide(func() *redisModel.RedisClient { return appRedis }, container.Name("app"))
c.Provide(func() *redisModel.RedisClient { return sharedRedis }, container.Name("shared"))
```

### Grouped provider

```go
c.Provide(func() http.Handler { return assetsHandler }, container.Group("handlers"))
c.Provide(func() http.Handler { return tagsHandler },   container.Group("handlers"))
```

## Notes

- The singleton is **not reset** between calls — tests that need a fresh container must build their own `dig.New()` rather than calling `InitContainer`.
- This package does not expose `Invoke` / `Provide` directly; call them on the returned `*dig.Container` (i.e. interact with the dig API as you would normally).
