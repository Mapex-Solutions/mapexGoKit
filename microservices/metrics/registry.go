package metrics

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry wraps a custom prometheus.Registry.
// Each service creates ONE registry in its bootstrap.
// The registry is provided to the DIG container so any module can use it.
type Registry struct {
	namespace string
	registry  *prometheus.Registry
}

// NewRegistry creates an isolated Prometheus registry for a service.
//
// Parameters:
//   - namespace: Service prefix for all metrics (e.g., "httpgw", "triggers")
//
// Returns:
//   - *Registry: The registry instance to create metrics and expose the endpoint
func NewRegistry(namespace string) *Registry {
	return &Registry{
		namespace: namespace,
		registry:  prometheus.NewRegistry(),
	}
}

// NewCounterVec creates a labeled counter and registers it in this registry.
// The namespace is automatically prepended.
//
// Parameters:
//   - opts: Counter configuration (subsystem, name, help)
//   - labels: Label names for this counter
//
// Returns:
//   - *prometheus.CounterVec: The registered counter vector
func (r *Registry) NewCounterVec(opts CounterOpts, labels []string) *prometheus.CounterVec {
	cv := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: r.namespace,
		Subsystem: opts.Subsystem,
		Name:      opts.Name,
		Help:      opts.Help,
	}, labels)
	r.registry.MustRegister(cv)
	return cv
}

// NewCounter creates a single counter (no labels) and registers it in this registry.
// The namespace is automatically prepended.
//
// Parameters:
//   - opts: Counter configuration (subsystem, name, help)
//
// Returns:
//   - prometheus.Counter: The registered counter
func (r *Registry) NewCounter(opts CounterOpts) prometheus.Counter {
	c := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: r.namespace,
		Subsystem: opts.Subsystem,
		Name:      opts.Name,
		Help:      opts.Help,
	})
	r.registry.MustRegister(c)
	return c
}

// NewHistogramVec creates a labeled histogram and registers it in this registry.
// The namespace is automatically prepended.
//
// Parameters:
//   - opts: Histogram configuration (subsystem, name, help, buckets)
//   - labels: Label names for this histogram
//
// Returns:
//   - *prometheus.HistogramVec: The registered histogram vector
func (r *Registry) NewHistogramVec(opts HistogramOpts, labels []string) *prometheus.HistogramVec {
	hv := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: r.namespace,
		Subsystem: opts.Subsystem,
		Name:      opts.Name,
		Help:      opts.Help,
		Buckets:   opts.Buckets,
	}, labels)
	r.registry.MustRegister(hv)
	return hv
}

// NewHistogram creates a single histogram (no labels) and registers it in this registry.
// The namespace is automatically prepended.
//
// Parameters:
//   - opts: Histogram configuration (subsystem, name, help, buckets)
//
// Returns:
//   - prometheus.Histogram: The registered histogram
func (r *Registry) NewHistogram(opts HistogramOpts) prometheus.Histogram {
	h := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: r.namespace,
		Subsystem: opts.Subsystem,
		Name:      opts.Name,
		Help:      opts.Help,
		Buckets:   opts.Buckets,
	})
	r.registry.MustRegister(h)
	return h
}

// NewGauge creates a single gauge (no labels) and registers it in this registry.
// The namespace is automatically prepended.
//
// Parameters:
//   - opts: Gauge configuration (subsystem, name, help)
//
// Returns:
//   - prometheus.Gauge: The registered gauge
func (r *Registry) NewGauge(opts GaugeOpts) prometheus.Gauge {
	g := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: r.namespace,
		Subsystem: opts.Subsystem,
		Name:      opts.Name,
		Help:      opts.Help,
	})
	r.registry.MustRegister(g)
	return g
}

// NewGaugeVec creates a labeled gauge and registers it in this registry.
// The namespace is automatically prepended.
//
// Parameters:
//   - opts: Gauge configuration (subsystem, name, help)
//   - labels: Label names for this gauge
//
// Returns:
//   - *prometheus.GaugeVec: The registered gauge vector
func (r *Registry) NewGaugeVec(opts GaugeOpts, labels []string) *prometheus.GaugeVec {
	gv := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: r.namespace,
		Subsystem: opts.Subsystem,
		Name:      opts.Name,
		Help:      opts.Help,
	}, labels)
	r.registry.MustRegister(gv)
	return gv
}

// EnableGoCollector registers Go runtime metrics (goroutines, GC, memory).
func (r *Registry) EnableGoCollector() {
	r.registry.MustRegister(collectors.NewGoCollector())
}

// EnableProcessCollector registers process metrics (CPU, FDs, memory RSS).
func (r *Registry) EnableProcessCollector() {
	r.registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
}

// Handler returns a Fiber handler that serves the /metrics endpoint.
// It serializes ONLY the metrics registered in THIS registry.
//
// Returns:
//   - fiber.Handler: A handler that writes Prometheus text format
func (r *Registry) Handler() fiber.Handler {
	h := promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{})
	return adaptor.HTTPHandler(http.HandlerFunc(h.ServeHTTP))
}

// RegisterEndpoint registers GET /metrics on the Fiber app.
// Convenience method that calls Handler() internally.
//
// Parameters:
//   - app: The Fiber application instance
func (r *Registry) RegisterEndpoint(app *fiber.App) {
	app.Get("/metrics", r.Handler())
}
