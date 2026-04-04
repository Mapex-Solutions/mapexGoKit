package middlewaresApiKeyAuth

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

func TestMain(m *testing.M) {
	logger.InitLogger(logger.LoggerOptions{
		ServiceName: "test",
		Environment: "test",
		Level:       logger.ErrorLevel,
	})
	os.Exit(m.Run())
}

func TestApiKeyAuthMiddleware_ValidKey(t *testing.T) {
	app := fiber.New()
	app.Use(ApiKeyAuthMiddleware("valid-key"))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "valid-key")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestApiKeyAuthMiddleware_InvalidKey(t *testing.T) {
	app := fiber.New()
	app.Use(ApiKeyAuthMiddleware("valid-key"))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestApiKeyAuthMiddleware_MissingKey(t *testing.T) {
	app := fiber.New()
	app.Use(ApiKeyAuthMiddleware("valid-key"))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestApiKeyAuthMiddleware_EmptyKey(t *testing.T) {
	app := fiber.New()
	app.Use(ApiKeyAuthMiddleware("valid-key"))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestApiKeyAuthMiddleware_NextHandlerCalled(t *testing.T) {
	app := fiber.New()
	app.Use(ApiKeyAuthMiddleware("valid-key"))

	called := false
	app.Get("/test", func(c *fiber.Ctx) error {
		called = true
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "valid-key")
	_, _ = app.Test(req, -1)

	if !called {
		t.Error("expected next handler to be called on valid API key")
	}
}

/** SECURITY: Server key not configured */

func TestApiKeyAuthMiddleware_ServerKeyNotConfigured_RejectsAll(t *testing.T) {
	app := fiber.New()
	app.Use(ApiKeyAuthMiddleware("")) // simulates MY_API_KEY not set
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "any-key")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 401 {
		t.Errorf("SECURITY: when server key is empty, ALL requests must be rejected. Got %d", resp.StatusCode)
	}
}

func TestApiKeyAuthMiddleware_ServerKeyNotConfigured_EmptyClientKey(t *testing.T) {
	app := fiber.New()
	app.Use(ApiKeyAuthMiddleware("")) // simulates MY_API_KEY not set
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// No X-API-Key header at all — both server and client are empty
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 401 {
		t.Errorf("SECURITY: empty server key + empty client key must be 401, got %d", resp.StatusCode)
	}
}

func TestApiKeyAuthMiddleware_ServerKeyNotConfigured_NextNotCalled(t *testing.T) {
	app := fiber.New()
	app.Use(ApiKeyAuthMiddleware(""))

	called := false
	app.Get("/test", func(c *fiber.Ctx) error {
		called = true
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req, -1)

	if called {
		t.Error("SECURITY: next handler must NOT be called when server key is not configured")
	}
}
