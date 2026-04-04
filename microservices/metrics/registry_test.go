package metrics

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
)

/** Helpers */

// newTestRegistry creates a fresh Registry with the given namespace for test isolation.
func newTestRegistry(namespace string) *Registry {
	return NewRegistry(namespace)
}

// collectMetricsOutput gathers the Prometheus text exposition from a registry via Fiber test request.
func collectMetricsOutput(t *testing.T, reg *Registry) string {
	t.Helper()

	app := fiber.New()
	reg.RegisterEndpoint(app)

	req := httptest.NewRequest("GET", "/metrics", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("failed to execute test request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	return string(body)
}

/** NewRegistry */

func TestNewRegistry(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
	}{
		{
			name:      "creates registry with httpgw namespace",
			namespace: "httpgw",
		},
		{
			name:      "creates registry with triggers namespace",
			namespace: "triggers",
		},
		{
			name:      "creates registry with empty namespace",
			namespace: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := NewRegistry(tt.namespace)

			if reg == nil {
				t.Fatal("expected non-nil registry")
			}
			if reg.namespace != tt.namespace {
				t.Errorf("expected namespace %q, got %q", tt.namespace, reg.namespace)
			}
			if reg.registry == nil {
				t.Fatal("expected non-nil prometheus.Registry")
			}
		})
	}
}

func TestNewRegistry_IsolatedRegistries(t *testing.T) {
	reg1 := NewRegistry("svc1")
	reg2 := NewRegistry("svc2")

	// Register a counter in reg1 only
	reg1.NewCounter(CounterOpts{Name: "requests_total", Help: "Total requests"})

	// reg2 should not contain svc1 metrics
	output := collectMetricsOutput(t, reg2)
	if strings.Contains(output, "svc1_requests_total") {
		t.Error("reg2 should not contain metrics from reg1")
	}
}

/** NewCounter */

func TestNewCounter(t *testing.T) {
	reg := newTestRegistry("test")

	counter := reg.NewCounter(CounterOpts{
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total HTTP requests",
	})

	if counter == nil {
		t.Fatal("expected non-nil counter")
	}

	counter.Inc()
	counter.Inc()
	counter.Add(3)

	output := collectMetricsOutput(t, reg)

	if !strings.Contains(output, "test_http_requests_total") {
		t.Error("expected metric name with namespace and subsystem in output")
	}
	if !strings.Contains(output, "5") {
		t.Error("expected counter value of 5 in output")
	}
}

func TestNewCounter_WithoutSubsystem(t *testing.T) {
	reg := newTestRegistry("myapp")

	counter := reg.NewCounter(CounterOpts{
		Name: "events_total",
		Help: "Total events",
	})

	counter.Inc()

	output := collectMetricsOutput(t, reg)

	if !strings.Contains(output, "myapp_events_total") {
		t.Error("expected metric name with namespace (no subsystem) in output")
	}
}

/** NewCounterVec */

func TestNewCounterVec(t *testing.T) {
	reg := newTestRegistry("httpgw")

	cv := reg.NewCounterVec(CounterOpts{
		Subsystem: "event",
		Name:      "auth_total",
		Help:      "Webhook auth attempts",
	}, []string{"auth_type", "result"})

	if cv == nil {
		t.Fatal("expected non-nil CounterVec")
	}

	cv.WithLabelValues("jwt", "success").Inc()
	cv.WithLabelValues("jwt", "failure").Inc()
	cv.WithLabelValues("jwt", "failure").Inc()
	cv.WithLabelValues("apiKey", "success").Add(5)

	output := collectMetricsOutput(t, reg)

	if !strings.Contains(output, `httpgw_event_auth_total{auth_type="jwt",result="success"} 1`) {
		t.Error("expected jwt/success counter with value 1")
	}
	if !strings.Contains(output, `httpgw_event_auth_total{auth_type="jwt",result="failure"} 2`) {
		t.Error("expected jwt/failure counter with value 2")
	}
	if !strings.Contains(output, `httpgw_event_auth_total{auth_type="apiKey",result="success"} 5`) {
		t.Error("expected apiKey/success counter with value 5")
	}
}

/** NewHistogram */

func TestNewHistogram(t *testing.T) {
	reg := newTestRegistry("test")

	h := reg.NewHistogram(HistogramOpts{
		Subsystem: "event",
		Name:      "duration_seconds",
		Help:      "Processing duration",
		Buckets:   []float64{0.01, 0.05, 0.1, 0.5, 1.0},
	})

	if h == nil {
		t.Fatal("expected non-nil histogram")
	}

	h.Observe(0.03)
	h.Observe(0.07)
	h.Observe(0.5)

	output := collectMetricsOutput(t, reg)

	if !strings.Contains(output, "test_event_duration_seconds_count 3") {
		t.Error("expected histogram count of 3")
	}
	if !strings.Contains(output, "test_event_duration_seconds_bucket") {
		t.Error("expected histogram bucket lines in output")
	}
}

func TestNewHistogram_DefaultBuckets(t *testing.T) {
	reg := newTestRegistry("test")

	h := reg.NewHistogram(HistogramOpts{
		Name:    "latency_seconds",
		Help:    "Latency",
		Buckets: prometheus.DefBuckets,
	})

	h.Observe(0.1)

	output := collectMetricsOutput(t, reg)

	// DefBuckets includes le="0.1"
	if !strings.Contains(output, `le="0.1"`) {
		t.Error("expected default bucket le=0.1 in output")
	}
}

func TestNewHistogram_NilBuckets(t *testing.T) {
	reg := newTestRegistry("test")

	// nil Buckets should fallback to Prometheus default buckets
	h := reg.NewHistogram(HistogramOpts{
		Name: "test_metric",
		Help: "Test",
	})

	h.Observe(1.0)

	output := collectMetricsOutput(t, reg)

	if !strings.Contains(output, "test_test_metric_count 1") {
		t.Error("expected histogram with nil buckets to use defaults")
	}
}

/** NewHistogramVec */

func TestNewHistogramVec(t *testing.T) {
	reg := newTestRegistry("httpgw")

	hv := reg.NewHistogramVec(HistogramOpts{
		Subsystem: "ds",
		Name:      "operation_duration_seconds",
		Help:      "CRUD operation latency",
		Buckets:   []float64{0.01, 0.05, 0.1, 0.5, 1.0},
	}, []string{"operation"})

	if hv == nil {
		t.Fatal("expected non-nil HistogramVec")
	}

	hv.WithLabelValues("create").Observe(0.02)
	hv.WithLabelValues("create").Observe(0.03)
	hv.WithLabelValues("delete").Observe(0.5)

	output := collectMetricsOutput(t, reg)

	if !strings.Contains(output, `httpgw_ds_operation_duration_seconds_count{operation="create"} 2`) {
		t.Error("expected create histogram count of 2")
	}
	if !strings.Contains(output, `httpgw_ds_operation_duration_seconds_count{operation="delete"} 1`) {
		t.Error("expected delete histogram count of 1")
	}
}

/** NewGauge */

func TestNewGauge(t *testing.T) {
	reg := newTestRegistry("test")

	g := reg.NewGauge(GaugeOpts{
		Subsystem: "vm",
		Name:      "contexts_active",
		Help:      "Active VM contexts",
	})

	if g == nil {
		t.Fatal("expected non-nil gauge")
	}

	g.Set(10)
	g.Inc()
	g.Dec()
	g.Add(5)
	g.Sub(2)

	output := collectMetricsOutput(t, reg)

	if !strings.Contains(output, "test_vm_contexts_active 13") {
		t.Error("expected gauge value of 13 (10+1-1+5-2)")
	}
}

/** NewGaugeVec */

func TestNewGaugeVec(t *testing.T) {
	reg := newTestRegistry("test")

	gv := reg.NewGaugeVec(GaugeOpts{
		Name: "connections",
		Help: "Active connections",
	}, []string{"protocol"})

	if gv == nil {
		t.Fatal("expected non-nil GaugeVec")
	}

	gv.WithLabelValues("http").Set(100)
	gv.WithLabelValues("grpc").Set(50)

	output := collectMetricsOutput(t, reg)

	if !strings.Contains(output, `test_connections{protocol="http"} 100`) {
		t.Error("expected http gauge value of 100")
	}
	if !strings.Contains(output, `test_connections{protocol="grpc"} 50`) {
		t.Error("expected grpc gauge value of 50")
	}
}

/** Collectors */

func TestEnableGoCollector(t *testing.T) {
	reg := newTestRegistry("test")
	reg.EnableGoCollector()

	output := collectMetricsOutput(t, reg)

	if !strings.Contains(output, "go_goroutines") {
		t.Error("expected go_goroutines metric from GoCollector")
	}
	if !strings.Contains(output, "go_gc") {
		t.Error("expected go_gc metrics from GoCollector")
	}
}

func TestEnableProcessCollector(t *testing.T) {
	reg := newTestRegistry("test")
	reg.EnableProcessCollector()

	output := collectMetricsOutput(t, reg)

	if !strings.Contains(output, "process_") {
		t.Error("expected process_* metrics from ProcessCollector")
	}
}

func TestEnableGoCollector_DuplicatePanics(t *testing.T) {
	reg := newTestRegistry("test")
	reg.EnableGoCollector()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate GoCollector registration")
		}
	}()

	reg.EnableGoCollector()
}

func TestEnableProcessCollector_DuplicatePanics(t *testing.T) {
	reg := newTestRegistry("test")
	reg.EnableProcessCollector()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate ProcessCollector registration")
		}
	}()

	reg.EnableProcessCollector()
}

/** Duplicate metric registration */

func TestDuplicateMetricRegistration_Panics(t *testing.T) {
	reg := newTestRegistry("test")

	reg.NewCounter(CounterOpts{Name: "dup_counter", Help: "first"})

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate metric registration")
		}
	}()

	reg.NewCounter(CounterOpts{Name: "dup_counter", Help: "second"})
}

/** Handler & RegisterEndpoint */

func TestHandler_ReturnsValidFiberHandler(t *testing.T) {
	reg := newTestRegistry("test")
	reg.NewCounter(CounterOpts{Name: "ping_total", Help: "Pings"})

	handler := reg.Handler()
	if handler == nil {
		t.Fatal("expected non-nil fiber.Handler")
	}

	app := fiber.New()
	app.Get("/custom-metrics", handler)

	req := httptest.NewRequest("GET", "/custom-metrics", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("failed to execute test request: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if !strings.Contains(string(body), "test_ping_total") {
		t.Error("expected metric in custom endpoint response")
	}
}

func TestRegisterEndpoint(t *testing.T) {
	reg := newTestRegistry("test")
	counter := reg.NewCounter(CounterOpts{Name: "hits_total", Help: "Hits"})
	counter.Add(42)

	app := fiber.New()
	reg.RegisterEndpoint(app)

	req := httptest.NewRequest("GET", "/metrics", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("failed to execute test request: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	output := string(body)

	if !strings.Contains(output, "test_hits_total 42") {
		t.Error("expected counter value of 42 at /metrics endpoint")
	}
}

func TestRegisterEndpoint_ContentType(t *testing.T) {
	reg := newTestRegistry("test")

	app := fiber.New()
	reg.RegisterEndpoint(app)

	req := httptest.NewRequest("GET", "/metrics", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("failed to execute test request: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/plain") && !strings.Contains(ct, "text/openmetrics") {
		t.Errorf("expected Prometheus text content type, got %q", ct)
	}
}

func TestRegisterEndpoint_EmptyRegistry(t *testing.T) {
	reg := newTestRegistry("test")

	app := fiber.New()
	reg.RegisterEndpoint(app)

	req := httptest.NewRequest("GET", "/metrics", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("failed to execute test request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200 for empty registry, got %d", resp.StatusCode)
	}
}

/** Namespace prepend */

func TestNamespacePrepend(t *testing.T) {
	tests := []struct {
		name           string
		namespace      string
		subsystem      string
		metricName     string
		expectedPrefix string
	}{
		{
			name:           "namespace + subsystem + name",
			namespace:      "httpgw",
			subsystem:      "event",
			metricName:     "auth_total",
			expectedPrefix: "httpgw_event_auth_total",
		},
		{
			name:           "namespace + name (no subsystem)",
			namespace:      "httpgw",
			subsystem:      "",
			metricName:     "uptime_seconds",
			expectedPrefix: "httpgw_uptime_seconds",
		},
		{
			name:           "empty namespace + subsystem + name",
			namespace:      "",
			subsystem:      "event",
			metricName:     "count",
			expectedPrefix: "event_count",
		},
		{
			name:           "empty namespace + no subsystem",
			namespace:      "",
			subsystem:      "",
			metricName:     "total",
			expectedPrefix: "total",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := newTestRegistry(tt.namespace)

			reg.NewCounter(CounterOpts{
				Subsystem: tt.subsystem,
				Name:      tt.metricName,
				Help:      "Test metric",
			})

			output := collectMetricsOutput(t, reg)

			if !strings.Contains(output, tt.expectedPrefix) {
				t.Errorf("expected %q in output, got:\n%s", tt.expectedPrefix, output)
			}
		})
	}
}

/** Full integration: simulates HttpGatewayMetrics bootstrap pattern */

func TestFullBootstrapPattern(t *testing.T) {
	reg := NewRegistry("httpgw")
	reg.EnableGoCollector()
	reg.EnableProcessCollector()

	// Declare metrics following the same pattern as bootstrap/metrics.go
	eventAuthTotal := reg.NewCounterVec(CounterOpts{
		Subsystem: "event", Name: "auth_total", Help: "Webhook auth attempts",
	}, []string{"auth_type", "result"})

	eventsProcessed := reg.NewCounterVec(CounterOpts{
		Subsystem: "event", Name: "processed_total", Help: "Events processed",
	}, []string{"status"})

	eventPayloadSize := reg.NewHistogram(HistogramOpts{
		Subsystem: "event", Name: "payload_size_bytes", Help: "Payload size",
		Buckets: []float64{100, 500, 1000, 5000, 10000, 50000, 100000},
	})

	dsOperations := reg.NewCounterVec(CounterOpts{
		Subsystem: "ds", Name: "operations_total", Help: "CRUD operations",
	}, []string{"operation", "status"})

	// Simulate service usage
	eventAuthTotal.WithLabelValues("jwt", "success").Inc()
	eventAuthTotal.WithLabelValues("apiKey", "failure").Inc()
	eventsProcessed.WithLabelValues("success").Add(100)
	eventsProcessed.WithLabelValues("error").Add(2)
	eventPayloadSize.Observe(1500)
	eventPayloadSize.Observe(3000)
	dsOperations.WithLabelValues("create", "success").Inc()
	dsOperations.WithLabelValues("list", "success").Add(10)

	output := collectMetricsOutput(t, reg)

	// Verify service-specific metrics
	expectedMetrics := []string{
		`httpgw_event_auth_total{auth_type="jwt",result="success"} 1`,
		`httpgw_event_auth_total{auth_type="apiKey",result="failure"} 1`,
		`httpgw_event_processed_total{status="success"} 100`,
		`httpgw_event_processed_total{status="error"} 2`,
		"httpgw_event_payload_size_bytes_count 2",
		`httpgw_ds_operations_total{operation="create",status="success"} 1`,
		`httpgw_ds_operations_total{operation="list",status="success"} 10`,
	}

	for _, expected := range expectedMetrics {
		if !strings.Contains(output, expected) {
			t.Errorf("expected %q in output", expected)
		}
	}

	// Verify Go runtime and process collectors
	if !strings.Contains(output, "go_goroutines") {
		t.Error("expected go_goroutines from GoCollector")
	}
	if !strings.Contains(output, "process_") {
		t.Error("expected process_* from ProcessCollector")
	}
}

/** Help text */

func TestHelpTextInOutput(t *testing.T) {
	reg := newTestRegistry("test")

	reg.NewCounter(CounterOpts{
		Name: "requests_total",
		Help: "Total number of requests",
	})

	output := collectMetricsOutput(t, reg)

	if !strings.Contains(output, "# HELP test_requests_total Total number of requests") {
		t.Error("expected HELP line with correct help text")
	}
	if !strings.Contains(output, "# TYPE test_requests_total counter") {
		t.Error("expected TYPE line indicating counter type")
	}
}

func TestHistogramTypeInOutput(t *testing.T) {
	reg := newTestRegistry("test")

	reg.NewHistogram(HistogramOpts{
		Name:    "duration_seconds",
		Help:    "Duration in seconds",
		Buckets: []float64{0.1, 0.5, 1.0},
	})

	output := collectMetricsOutput(t, reg)

	if !strings.Contains(output, "# TYPE test_duration_seconds histogram") {
		t.Error("expected TYPE line indicating histogram type")
	}
}

func TestGaugeTypeInOutput(t *testing.T) {
	reg := newTestRegistry("test")

	g := reg.NewGauge(GaugeOpts{
		Name: "temperature",
		Help: "Current temperature",
	})
	g.Set(36.6)

	output := collectMetricsOutput(t, reg)

	if !strings.Contains(output, "# TYPE test_temperature gauge") {
		t.Error("expected TYPE line indicating gauge type")
	}
	if !strings.Contains(output, "test_temperature 36.6") {
		t.Error("expected gauge value 36.6")
	}
}

/** Benchmarks */

func BenchmarkNewCounter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		reg := newTestRegistry("bench")
		reg.NewCounter(CounterOpts{Name: "total", Help: "bench"})
	}
}

func BenchmarkCounterInc(b *testing.B) {
	reg := newTestRegistry("bench")
	c := reg.NewCounter(CounterOpts{Name: "total", Help: "bench"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Inc()
	}
}

func BenchmarkCounterVecWithLabelValues(b *testing.B) {
	reg := newTestRegistry("bench")
	cv := reg.NewCounterVec(CounterOpts{Name: "total", Help: "bench"}, []string{"status"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cv.WithLabelValues("success").Inc()
	}
}

func BenchmarkHistogramObserve(b *testing.B) {
	reg := newTestRegistry("bench")
	h := reg.NewHistogram(HistogramOpts{Name: "duration", Help: "bench", Buckets: prometheus.DefBuckets})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Observe(0.05)
	}
}

func BenchmarkHandler(b *testing.B) {
	reg := newTestRegistry("bench")
	reg.NewCounter(CounterOpts{Name: "total", Help: "bench"})

	app := fiber.New()
	reg.RegisterEndpoint(app)

	req := httptest.NewRequest("GET", "/metrics", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := app.Test(req, -1)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}
