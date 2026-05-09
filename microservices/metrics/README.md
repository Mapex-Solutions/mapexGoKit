# metrics — Per-service Prometheus registry + Fiber endpoint

Thin wrapper over [`prometheus/client_golang`](https://github.com/prometheus/client_golang). Each service builds **one** isolated `Registry` with a namespace prefix, registers its counters/histograms/gauges through it, and serves `/metrics` via a Fiber-compatible handler.

> Package name: `metrics` (directory: `metrics/`).

## Surface

### Constructor

```go
func NewRegistry(namespace string) *Registry
```

`namespace` is automatically prepended to every metric name registered through this `*Registry`. Two registries with different namespaces are fully isolated — they share no collectors and serialise independently. Per service, build exactly one and put it in DI.

### Option structs (`types.go`)

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
    Buckets   []float64 // nil → prometheus default buckets
}

type GaugeOpts struct {
    Subsystem string
    Name      string
    Help      string
}
```

The final exposed metric name follows Prometheus conventions: `<namespace>_<subsystem>_<name>` (subsystem can be empty).

### Metric factories on `*Registry`

| Method | Returns | Notes |
|---|---|---|
| `NewCounter(opts)` | `prometheus.Counter` | Single counter, no labels. |
| `NewCounterVec(opts, labels)` | `*prometheus.CounterVec` | Labelled counter. |
| `NewHistogram(opts)` | `prometheus.Histogram` | Single histogram. `Buckets == nil` → prometheus default buckets. |
| `NewHistogramVec(opts, labels)` | `*prometheus.HistogramVec` | Labelled histogram. |
| `NewGauge(opts)` | `prometheus.Gauge` | Single gauge. |
| `NewGaugeVec(opts, labels)` | `*prometheus.GaugeVec` | Labelled gauge. |

Every factory uses `MustRegister` internally → **registering the same metric twice panics** (the test suite covers this for `Counter`, `Histogram`, `Gauge` and the system collectors).

### System collectors

| Method | Effect |
|---|---|
| `EnableGoCollector()` | Adds `collectors.NewGoCollector()` — goroutines, GC, memory. |
| `EnableProcessCollector()` | Adds `collectors.NewProcessCollector` — CPU, FDs, RSS. |

Both panic on duplicate registration — call exactly once per registry.

### HTTP exposure

| Method | Effect |
|---|---|
| `Handler() fiber.Handler` | Returns a Fiber handler that serialises this registry only (Prometheus text format). |
| `RegisterEndpoint(app *fiber.App)` | Convenience: mounts `GET /metrics` on the supplied Fiber app using `Handler()`. |

The handler routes through `promhttp.HandlerFor(reg, ...)` → `adaptor.HTTPHandler` so it stays compatible with Fiber middleware chains.

## Bootstrap pattern

```go
reg := metrics.NewRegistry("httpgw") // service-wide namespace
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

In your DIG container:

```go
container.Provide(func() *metrics.Registry { return reg })
```

Modules pull `*metrics.Registry` via DI and call `reg.NewCounter(...)` etc. There is no global state — keep one registry per service for clean isolation.

## Tested behaviours (`registry_test.go`)

- `NewRegistry` builds an isolated `*prometheus.Registry`; two registries with different namespaces never see each other's metrics.
- All six factories register the metric and prepend the namespace correctly. `Subsystem == ""` is permitted.
- `Histogram` with `Buckets == nil` uses prometheus's defaults; the buckets you pass are honoured otherwise.
- Re-registering the same metric panics (`MustRegister`).
- `EnableGoCollector` / `EnableProcessCollector` panic on duplicate calls.
- `Handler()` and `RegisterEndpoint` produce valid Prometheus text output (`# HELP`, `# TYPE` lines, `text/plain; version=0.0.4`) including bodies for histograms (`_bucket`, `_sum`, `_count`) and gauges (`# TYPE … gauge`).
- `RegisterEndpoint` on an empty registry returns `200 OK` with an empty body (no metrics yet).
- The full bootstrap pattern (`TestFullBootstrapPattern`) is exercised end-to-end through Fiber's test rig.

Benchmarks cover counter creation/increment, labelled-counter access, histogram observation, and the `/metrics` handler invocation.
