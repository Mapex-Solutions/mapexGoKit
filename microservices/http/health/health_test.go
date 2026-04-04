package health

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	common "github.com/Mapex-Solutions/mapexGoKit/infrastructure/common/ports"
)

/** Mock Checker */

type mockChecker struct {
	name      string
	connected bool
	latencyMs int64
	errMsg    string
}

func (m *mockChecker) Name() string {
	return m.name
}

func (m *mockChecker) Check(ctx context.Context) common.HealthStatus {
	return common.HealthStatus{
		Connected:    m.connected,
		Service:      m.name,
		LatencyMs:    m.latencyMs,
		LastCheckAt:  time.Now(),
		ErrorMessage: m.errMsg,
	}
}

/** NewService */

func TestNewService_DefaultCacheTTL(t *testing.T) {
	svc := NewService(Config{ServiceName: "test", Version: "1.0"})
	if svc.cfg.CacheTTL != 10*time.Second {
		t.Errorf("expected default CacheTTL 10s, got %v", svc.cfg.CacheTTL)
	}
}

func TestNewService_DefaultTimeout(t *testing.T) {
	svc := NewService(Config{ServiceName: "test", Version: "1.0"})
	if svc.cfg.Timeout != 5*time.Second {
		t.Errorf("expected default Timeout 5s, got %v", svc.cfg.Timeout)
	}
}

func TestNewService_CustomCacheTTL(t *testing.T) {
	svc := NewService(Config{ServiceName: "test", Version: "1.0", CacheTTL: 30 * time.Second})
	if svc.cfg.CacheTTL != 30*time.Second {
		t.Errorf("expected CacheTTL 30s, got %v", svc.cfg.CacheTTL)
	}
}

func TestNewService_CustomTimeout(t *testing.T) {
	svc := NewService(Config{ServiceName: "test", Version: "1.0", Timeout: 15 * time.Second})
	if svc.cfg.Timeout != 15*time.Second {
		t.Errorf("expected Timeout 15s, got %v", svc.cfg.Timeout)
	}
}

/** Check */

func TestCheck_AllHealthy(t *testing.T) {
	svc := NewService(
		Config{ServiceName: "test", Version: "1.0"},
		CheckerConfig{Checker: &mockChecker{name: "mongo", connected: true, latencyMs: 5}, Critical: true},
		CheckerConfig{Checker: &mockChecker{name: "redis", connected: true, latencyMs: 2}, Critical: false},
	)

	resp := svc.Check(context.Background())

	if resp.Status != "healthy" {
		t.Errorf("expected 'healthy', got %q", resp.Status)
	}
	if resp.Service != "test" {
		t.Errorf("expected service 'test', got %q", resp.Service)
	}
	if len(resp.Checks) != 2 {
		t.Errorf("expected 2 checks, got %d", len(resp.Checks))
	}
}

func TestCheck_NonCriticalDown_Degraded(t *testing.T) {
	svc := NewService(
		Config{ServiceName: "test", Version: "1.0"},
		CheckerConfig{Checker: &mockChecker{name: "mongo", connected: true}, Critical: true},
		CheckerConfig{Checker: &mockChecker{name: "redis", connected: false, errMsg: "connection refused"}, Critical: false},
	)

	resp := svc.Check(context.Background())

	if resp.Status != "degraded" {
		t.Errorf("expected 'degraded', got %q", resp.Status)
	}
}

func TestCheck_CriticalDown_Unhealthy(t *testing.T) {
	svc := NewService(
		Config{ServiceName: "test", Version: "1.0"},
		CheckerConfig{Checker: &mockChecker{name: "mongo", connected: false, errMsg: "timeout"}, Critical: true},
		CheckerConfig{Checker: &mockChecker{name: "redis", connected: true}, Critical: false},
	)

	resp := svc.Check(context.Background())

	if resp.Status != "unhealthy" {
		t.Errorf("expected 'unhealthy', got %q", resp.Status)
	}
}

func TestCheck_CacheHit(t *testing.T) {
	callCount := 0
	checker := &mockChecker{name: "mongo", connected: true}

	svc := NewService(
		Config{ServiceName: "test", Version: "1.0", CacheTTL: 5 * time.Second},
		CheckerConfig{Checker: checker, Critical: true},
	)

	// Wrap original checker to count calls
	originalChecker := checker
	countingChecker := &countingMockChecker{mock: originalChecker, count: &callCount}
	svc.checkers[0].Checker = countingChecker

	ctx := context.Background()
	resp1 := svc.Check(ctx)
	resp2 := svc.Check(ctx) // Should be cached

	if resp1.Status != resp2.Status {
		t.Errorf("expected same status from cache, got %q and %q", resp1.Status, resp2.Status)
	}

	if callCount != 1 {
		t.Errorf("expected checker to be called once (cached), got %d", callCount)
	}
}

// countingMockChecker wraps a mockChecker and counts calls.
type countingMockChecker struct {
	mock  *mockChecker
	count *int
}

func (c *countingMockChecker) Name() string {
	return c.mock.Name()
}

func (c *countingMockChecker) Check(ctx context.Context) common.HealthStatus {
	*c.count++
	return c.mock.Check(ctx)
}

func TestCheck_CacheExpired(t *testing.T) {
	callCount := 0
	checker := &mockChecker{name: "mongo", connected: true}
	countingChecker := &countingMockChecker{mock: checker, count: &callCount}

	svc := NewService(
		Config{ServiceName: "test", Version: "1.0", CacheTTL: 1 * time.Millisecond},
		CheckerConfig{Checker: countingChecker, Critical: true},
	)

	ctx := context.Background()
	svc.Check(ctx)
	time.Sleep(5 * time.Millisecond) // Wait for cache to expire
	svc.Check(ctx)

	if callCount != 2 {
		t.Errorf("expected checker to be called twice (cache expired), got %d", callCount)
	}
}

func TestCheck_NoCheckers_Healthy(t *testing.T) {
	svc := NewService(Config{ServiceName: "test", Version: "1.0"})
	resp := svc.Check(context.Background())

	if resp.Status != "healthy" {
		t.Errorf("expected 'healthy' with no checkers, got %q", resp.Status)
	}
}

func TestCheck_CheckDetail_Fields(t *testing.T) {
	svc := NewService(
		Config{ServiceName: "test", Version: "1.0"},
		CheckerConfig{Checker: &mockChecker{name: "redis", connected: false, latencyMs: 10, errMsg: "timeout"}, Critical: true},
	)

	resp := svc.Check(context.Background())
	detail, ok := resp.Checks["redis"]
	if !ok {
		t.Fatal("expected 'redis' check in response")
	}
	if detail.Connected {
		t.Error("expected Connected=false")
	}
	if !detail.Critical {
		t.Error("expected Critical=true")
	}
	if detail.LatencyMs != 10 {
		t.Errorf("expected LatencyMs=10, got %d", detail.LatencyMs)
	}
	if detail.ErrorMessage != "timeout" {
		t.Errorf("expected ErrorMessage 'timeout', got %q", detail.ErrorMessage)
	}
}

/** Handler */

func TestHandler_Healthy(t *testing.T) {
	svc := NewService(
		Config{ServiceName: "test", Version: "1.0"},
		CheckerConfig{Checker: &mockChecker{name: "mongo", connected: true}, Critical: true},
	)

	app := fiber.New()
	app.Get("/health", Handler(svc))

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	var r struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	json.Unmarshal(body, &r)

	if r.Data.Status != "healthy" {
		t.Errorf("expected 'healthy' in response, got %q", r.Data.Status)
	}
}

func TestHandler_Unhealthy(t *testing.T) {
	svc := NewService(
		Config{ServiceName: "test", Version: "1.0"},
		CheckerConfig{Checker: &mockChecker{name: "mongo", connected: false}, Critical: true},
	)

	app := fiber.New()
	app.Get("/health", Handler(svc))

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 503 {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
}

func TestHandler_Degraded(t *testing.T) {
	svc := NewService(
		Config{ServiceName: "test", Version: "1.0"},
		CheckerConfig{Checker: &mockChecker{name: "mongo", connected: true}, Critical: true},
		CheckerConfig{Checker: &mockChecker{name: "redis", connected: false}, Critical: false},
	)

	app := fiber.New()
	app.Get("/health", Handler(svc))

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// degraded returns 200 (not 503)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 for degraded, got %d", resp.StatusCode)
	}
}
