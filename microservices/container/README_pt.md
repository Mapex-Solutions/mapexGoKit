# container — Container DI singleton (wrapper de uber/dig)

Wrapper enxuto sobre [`go.uber.org/dig`](https://github.com/uber-go/dig). Expõe um `*dig.Container` singleton no escopo do processo e re-exporta as primitivas mais usadas do dig para que o código de serviço não precise importar dig diretamente.

> Nome do pacote: `container` (diretório: `container/`).

## Superfície

### Ciclo de vida do singleton (`container.go`, `methods.go`)

```go
func InitContainer() *dig.Container  // cria o singleton na primeira chamada (sync.Once); retorna a mesma instância depois
func GetContainer() *dig.Container   // retorna o singleton — nil antes de InitContainer ser chamado
```

`InitContainer` é seguro para chamadas concorrentes — o `sync.Once` interno garante uma única alocação.

### Re-exports (`types.go`)

| Re-export | Subjacente | Propósito |
|---|---|---|
| `type In = dig.In`   | marcador de struct embed | Parâmetros nomeados/agrupados em providers |
| `type Out = dig.Out` | marcador de struct embed | Múltiplos valores em um único construtor |
| `var Name = dig.Name`| opção `dig.Name(string)` | Fornece dependência nomeada |
| `var Group = dig.Group`| opção `dig.Group(string)` | Fornece dependência agrupada |

## Uso

### Bootstrap

```go
c := container.InitContainer()
c.Provide(redisModel.New)
c.Provide(natsModel.New)
// ... em InitServices, InitInterfaces, etc.
```

### Injeção nomeada (`In`)

```go
c.Provide(func(params struct {
    container.In
    RC *redisModel.RedisClient `name:"app"`
}) common.AppCache {
    return params.RC
})
```

### Provider multi-valor (`Out`)

```go
c.Provide(func() (struct {
    container.Out
    AppCache    common.Cache `name:"app"`
    SharedCache common.Cache `name:"shared"`
}) {
    // constrói + retorna ambos
})
```

### Provider nomeado

```go
c.Provide(func() *redisModel.RedisClient { return appRedis }, container.Name("app"))
c.Provide(func() *redisModel.RedisClient { return sharedRedis }, container.Name("shared"))
```

### Provider agrupado

```go
c.Provide(func() http.Handler { return assetsHandler }, container.Group("handlers"))
c.Provide(func() http.Handler { return tagsHandler },   container.Group("handlers"))
```

## Notas

- O singleton **não é resetado** entre chamadas — testes que precisam de container limpo devem construir o próprio `dig.New()` ao invés de chamar `InitContainer`.
- Este pacote não expõe `Invoke` / `Provide` diretamente; chame esses métodos no `*dig.Container` retornado (interaja com a API do dig normalmente).
