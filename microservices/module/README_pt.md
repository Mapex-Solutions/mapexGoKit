# module — `ModuleConfig` legada (3 fases)

Um único tipo — `ModuleConfig` — que descreve o formato original de bootstrap de módulos em 3 fases usado nos serviços Mapex. O sucessor de 4 fases (que adiciona `InitListeners`) vive em `microservices/common`; ambos ainda existem no código. Novos módulos devem preferir a variante de `common`.

> Nome do pacote: `module` (diretório: `module/`).

## O que tem aqui

```go
type ModuleConfig struct {
    Name string

    // Reservado para lazy-loading futuro; não implementado.
    Lazy bool

    InitRepositories func() // fase 1 — registra repositórios no DIG
    InitServices     func() // fase 2 — registra serviços no DIG
    InitInterfaces   func() // fase 3 — registra rotas HTTP e consumers
}
```

Todos os callbacks `Init*` são opcionais — deixar qualquer um `nil` pula aquela fase.

## Como módulos usam

Cada módulo define uma função que retorna `ModuleConfig`; o bootstrap da aplicação percorre os módulos chamando `InitRepositories`, depois `InitServices`, depois `InitInterfaces`. Isso mantém a ordem de registro no DI determinística entre serviços heterogêneos.

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

## Relação com `common.ModuleConfig`

`microservices/common.ModuleConfig` tem o mesmo formato **mais** uma quarta fase:

```go
InitListeners func() // fase 4 — inicia listeners NATS APÓS todos os módulos estarem prontos
```

Use a variante de `common` em qualquer módulo que assine eventos NATS; caso contrário a ordem entre "serviço pronto no DIG" e "listener iniciado" não é garantida.
