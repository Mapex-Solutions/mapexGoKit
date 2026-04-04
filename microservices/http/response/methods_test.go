package response

import (
	"encoding/json"
	"errors"
	"io"
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

// parseBody reads and unmarshals the response body into a Response struct.
func parseBody(t *testing.T, resp *io.ReadCloser) Response {
	t.Helper()
	body, _ := io.ReadAll(*resp)
	var r Response
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("failed to parse body: %v", err)
	}
	return r
}

/** Success */

func TestSuccess(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		return Success(c, map[string]string{"key": "value"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	r := parseBody(t, &resp.Body)
	defer resp.Body.Close()

	if r.Status != 200 {
		t.Errorf("expected body status 200, got %d", r.Status)
	}
	if r.Errors != nil {
		t.Errorf("expected no errors, got %v", r.Errors)
	}
	if r.Data == nil {
		t.Error("expected data to be non-nil")
	}
}

/** Created */

func TestCreated(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		return Created(c, map[string]string{"id": "123"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 201 {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	r := parseBody(t, &resp.Body)
	defer resp.Body.Close()

	if r.Status != 201 {
		t.Errorf("expected body status 201, got %d", r.Status)
	}
}

/** BadRequest */

func TestBadRequest(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		return BadRequest(c, []string{"field is required", "invalid email"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}

	r := parseBody(t, &resp.Body)
	defer resp.Body.Close()

	if r.Status != 400 {
		t.Errorf("expected body status 400, got %d", r.Status)
	}
	if len(r.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(r.Errors))
	}
}

/** Conflict */

func TestConflict(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		return Conflict(c, []string{"resource already exists"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 409 {
		t.Errorf("expected 409, got %d", resp.StatusCode)
	}

	r := parseBody(t, &resp.Body)
	defer resp.Body.Close()

	if r.Status != 409 {
		t.Errorf("expected body status 409, got %d", r.Status)
	}
}

/** InternalServerError */

func TestInternalServerError_DefaultMessage(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		return InternalServerError(c, "", nil)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 500 {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}

	r := parseBody(t, &resp.Body)
	defer resp.Body.Close()

	if r.Status != 500 {
		t.Errorf("expected body status 500, got %d", r.Status)
	}
	if len(r.Errors) == 0 || r.Errors[0] != "Internal server error occurred" {
		t.Errorf("expected default message, got %v", r.Errors)
	}
}

func TestInternalServerError_CustomMessage(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		return InternalServerError(c, "database connection failed", errors.New("conn err"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 500 {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}

	r := parseBody(t, &resp.Body)
	defer resp.Body.Close()

	if len(r.Errors) == 0 || r.Errors[0] != "database connection failed" {
		t.Errorf("expected custom message, got %v", r.Errors)
	}
}

/** TimeoutServerError */

func TestTimeoutServerError(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		return TimeoutServerError(c, "request timed out", nil)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 504 {
		t.Errorf("expected 504, got %d", resp.StatusCode)
	}

	r := parseBody(t, &resp.Body)
	defer resp.Body.Close()

	if r.Status != 504 {
		t.Errorf("expected body status 504, got %d", r.Status)
	}
}

/** NotFound */

func TestNotFound(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		return NotFound(c, errors.New("user not found"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 404 {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}

	r := parseBody(t, &resp.Body)
	defer resp.Body.Close()

	if r.Status != 404 {
		t.Errorf("expected body status 404, got %d", r.Status)
	}
	if len(r.Errors) < 1 || r.Errors[0] != "Resource not found" {
		t.Errorf("expected 'Resource not found', got %v", r.Errors)
	}
}

/** Custom */

func TestCustom(t *testing.T) {
	tests := []struct {
		name       string
		code       int
		errors     []string
		expectCode int
	}{
		{
			name:       "unauthorized",
			code:       401,
			errors:     []string{"missing token"},
			expectCode: 401,
		},
		{
			name:       "forbidden",
			code:       403,
			errors:     []string{"access denied"},
			expectCode: 403,
		},
		{
			name:       "unprocessable entity",
			code:       422,
			errors:     []string{"invalid field", "missing param"},
			expectCode: 422,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/test", func(c *fiber.Ctx) error {
				return Custom(c, tt.code, tt.errors)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.StatusCode != tt.expectCode {
				t.Errorf("expected %d, got %d", tt.expectCode, resp.StatusCode)
			}

			r := parseBody(t, &resp.Body)
			defer resp.Body.Close()

			if r.Status != tt.expectCode {
				t.Errorf("expected body status %d, got %d", tt.expectCode, r.Status)
			}
			if len(r.Errors) != len(tt.errors) {
				t.Errorf("expected %d errors, got %d", len(tt.errors), len(r.Errors))
			}
		})
	}
}
