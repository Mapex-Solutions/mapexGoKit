# metrics — Registry Prometheus por serviço + endpoint Fiber

Wrapper enxuto sobre [`prometheus/client_golang`](https://github.com/prometheus/client_golang). Cada serviço cria **um** `Registry` isolado com prefixo de namespace, registra seus counters/histograms/gauges através dele, e serve `/metrics` via um handler compatível com Fiber.

> Nome do pacote: `metrics` (diretório: `metrics/`).

## Superfície

### Construtor

```go
func NewRegistry(namespace string) *Registry
```

`namespace` é automaticamente prefixado em toda métrica registrada por este `*Registry`. Dois registries com namespaces diferentes ficam totalmente isolados — não compartilham collectors e serializam de forma independente. Por serviço, construa exatamente um e coloque no DI.

### Structs de opção (`types.go`)

```go
type CounterOpts struct {
    Subsystem string
    Name      string
    Help      string
}

type HistogramOpts struct {
    Subsystem string
    Name      string
    Help      string
    Buckets   []float64 // nil → buckets padrão do prometheus
}

type GaugeOpts struct {
    Subsystem string
    Name      string
    Help      string
}
```

O nome final exposto segue convenções Prometheus: `<namespace>_<subsystem>_<name>` (subsystem pode ser vazio).

### Factories de métrica em `*Registry`

| Método | Retorna | Notas |
|---|---|---|
| `NewCounter(opts)` | `prometheus.Counter` | Counter único, sem labels. |
| `NewCounterVec(opts, labels)` | `*prometheus.CounterVec` | Counter labelado. |
| `NewHistogram(opts)` | `prometheus.Histogram` | Histograma único. `Buckets == nil` → buckets padrão. |
| `NewHistogramVec(opts, labels)` | `*prometheus.HistogramVec` | Histograma labelado. |
| `NewGauge(opts)` | `prometheus.Gauge` | Gauge único. |
| `NewGaugeVec(opts, labels)` | `*prometheus.GaugeVec` | Gauge labelado. |

Toda factory usa `MustRegister` internamente → **registrar a mesma métrica duas vezes faz panic** (o test suite cobre isso para `Counter`, `Histogram`, `Gauge` e os collectors do sistema).

### Collectors do sistema

| Método | Efeito |
|---|---|
| `EnableGoCollector()` | Adiciona `collectors.NewGoCollector()` — goroutines, GC, memória. |
| `EnableProcessCollector()` | Adiciona `collectors.NewProcessCollector` — CPU, FDs, RSS. |

Ambos fazem panic em registro duplicado — chame exatamente uma vez por registry.

### Exposição HTTP

| Método | Efeito |
|---|---|
| `Handler() fiber.Handler` | Retorna handler Fiber que serializa apenas este registry (formato texto Prometheus). |
| `RegisterEndpoint(app *fiber.App)` | Conveniência: monta `GET /metrics` no app Fiber usando `Handler()`. |

O handler passa por `promhttp.HandlerFor(reg, ...)` → `adaptor.HTTPHandler` para se manter compatível com middlewares Fiber.

## Padrão de bootstrap

```go
reg := metrics.NewRegistry("httpgw") // namespace do serviço
reg.EnableGoCollector()
reg.EnableProcessCollector()

reqDuration := reg.NewHistogramVec(metrics.HistogramOpts{
    Subsystem: "http",
    Name:      "request_duration_seconds",
    Help:      "Request duration in seconds",
    Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
}, []string{"method", "route", "status"})

dbCalls := reg.NewCounterVec(metrics.CounterOpts{
    Subsystem: "mongo",
    Name:      "calls_total",
    Help:      "MongoDB calls grouped by collection and outcome",
}, []string{"collection", "outcome"})

reg.RegisterEndpoint(app) // GET /metrics
```

No container DIG:

```go
container.Provide(func() *metrics.Registry { return reg })
```

Módulos pegam `*metrics.Registry` via DI e chamam `reg.NewCounter(...)` etc. Sem estado global — mantenha um registry por serviço para isolamento limpo.

## Comportamentos testados (`registry_test.go`)

- `NewRegistry` constrói um `*prometheus.Registry` isolado; dois registries com namespaces diferentes nunca veem as métricas um do outro.
- As 6 factories registram a métrica e prefixam o namespace corretamente. `Subsystem == ""` é permitido.
- `Histogram` com `Buckets == nil` usa os defaults do prometheus; os buckets passados são honrados em outros casos.
- Re-registrar a mesma métrica faz panic (`MustRegister`).
- `EnableGoCollector` / `EnableProcessCollector` fazem panic em chamadas duplicadas.
- `Handler()` e `RegisterEndpoint` produzem output texto Prometheus válido (linhas `# HELP`, `# TYPE`, `text/plain; version=0.0.4`) incluindo corpos para histogramas (`_bucket`, `_sum`, `_count`) e gauges (`# TYPE … gauge`).
- `RegisterEndpoint` em registry vazio retorna `200 OK` com body vazio (sem métricas ainda).
- O padrão completo de bootstrap (`TestFullBootstrapPattern`) é exercitado end-to-end pelo test rig do Fiber.

Benchmarks cobrem criação/incremento de counter, acesso a counter labelado, observação em histograma e a chamada do handler `/metrics`.
