# shutdown — Shutdown gracioso ordenado, dirigido por sinal

Um `ShutdownManager` pequeno que coleta hooks de cleanup nomeados com prioridades, espera `SIGTERM`/`SIGINT`, e então roda os hooks em **ordem ascendente de prioridade** com timeout configurável. Hooks na mesma prioridade rodam **concorrentemente**.

> Nome do pacote: `shutdown` (diretório: `shutdown/`).

## Superfície

### Construtor

```go
func New() *ShutdownManager
```

### Tipos

```go
type Shutdowner interface {
    Shutdown(ctx context.Context) error
}

type ShutdownManager struct { /* unexported */ }
```

### Registro

```go
func (m *ShutdownManager) Register(name string, priority int, s Shutdowner)
func (m *ShutdownManager) RegisterFunc(name string, priority int, fn func(ctx context.Context) error)
```

`Register` é açúcar para `RegisterFunc(name, priority, s.Shutdown)` — use o que for mais natural para o caller.

Internamente, cada hook é armazenado como `{Name, Priority, Fn}` e o slice é protegido por mutex.

### Ciclo de vida

```go
func (m *ShutdownManager) WaitForSignal(timeout time.Duration)
func (m *ShutdownManager) IsShuttingDown() bool
func (m *ShutdownManager) SetShuttingDown(v bool) // teste
```

`WaitForSignal` bloqueia a goroutine até `SIGTERM` ou `SIGINT` chegar. É o entry point que:

1. Seta a flag `terminating` (`IsShuttingDown() == true`).
2. Loga `[SHUTDOWN] Received <sinal>, starting graceful shutdown (timeout: <d>)…`.
3. Tira snapshot dos hooks registrados, ordena por prioridade ascendente, agrupa hooks consecutivos com a mesma prioridade.
4. Para cada grupo: dispara uma goroutine por hook compartilhando o `context.WithTimeout`, espera o grupo terminar, e avança para a próxima prioridade.
5. Se o contexto expirar no meio do shutdown, loga `[SHUTDOWN] Timeout reached, aborting remaining hooks` e quebra.
6. Em conclusão de cada hook, loga sucesso (`[SHUTDOWN] <nome> done (<duração>)`) ou falha (`[SHUTDOWN] <nome> failed (<duração>): <err>` em Warn).
7. Ao final, loga `[SHUTDOWN] Graceful shutdown complete (<total>)`.

A função retorna quando o loop termina. **Não chama `os.Exit` por si só** — o caller decide o que fazer depois.

## Bandas de prioridade recomendadas

Os doc-comments sugerem essas bandas. São guidelines, não constantes:

| Prioridade | Preocupação | Por que essa ordem |
|---|---|---|
| **P0** | Servidor HTTP | Para de aceitar novos requests; drena os em vôo. |
| **P1** | Consumers de mensagens | Para de fazer fetch; termina o batch atual. |
| **P2** | Goroutines de fundo | Tickers, sweep loops. |
| **P3** | Publishers / flush | Garante que mensagens pendentes saiam antes de fechar conexões. |
| **P4** | Caches | TieredCache, caches em memória. |
| **P5** | Conexões | MongoDB, Redis, NATS, ClickHouse. |

Hooks de mesma prioridade rodam em paralelo, então coloque a "camada lógica" mais lenta na própria prioridade e deixe paralelizar dentro dela.

## Uso

```go
sm := shutdown.New()

// HTTP primeiro
sm.RegisterFunc("http", 0, func(ctx context.Context) error {
    return server.Shutdown(ctx)
})

// Depois consumers
sm.Register("nats-events", 1, eventConsumer) // implementa Shutdowner

// Depois trabalho de fundo
sm.RegisterFunc("sweep-loop", 2, sweeper.Stop)

// Por último: conexões (rodam em paralelo porque todas são P5)
sm.Register("mongo",     5, mongoMgr)
sm.Register("redis-app", 5, redisApp)
sm.Register("nats-bus",  5, natsClient)

sm.WaitForSignal(20 * time.Second)
// continua main(), ou os.Exit(0)
```

## Comportamentos testados

- `RegisterFunc` e `Register` aceitam hooks; `IsShuttingDown` reflete o estado setado explicitamente via `SetShuttingDown(true)` (usado em testes).
- `groupByPriority` (`internals.go`) agrupa hooks consecutivos com prioridade igual. Depende do input já estar ordenado ascendentemente.
- Execução concorrente dentro de um grupo de prioridade é garantida por um `sync.WaitGroup`; o próximo grupo só inicia após o anterior terminar.
- Um hook que retorna erro é logado mas **não** aborta o restante do grupo — o manager é best-effort, não transacional.

## Notas

- `WaitForSignal` lê exatamente **um** sinal e retorna; não fica em loop. Re-armar exige nova chamada de `WaitForSignal` (raro na prática).
- O conjunto de sinais é `SIGTERM`, `SIGINT`. Outros sinais não são tratados.
- `SetShuttingDown(true)` é exposto apenas para testes — não há equivalente automático para testes unitários que queiram dirigir o ciclo de vida sem enviar sinais reais.
