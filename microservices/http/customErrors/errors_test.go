package customErrors

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/response"
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

/** ValidationError.Error() */

func TestValidationError_Single(t *testing.T) {
	err := &ValidationError{Errors: []string{"field is required"}}
	if err.Error() != "field is required" {
		t.Errorf("expected 'field is required', got %q", err.Error())
	}
}

func TestValidationError_Multiple(t *testing.T) {
	err := &ValidationError{Errors: []string{"field A required", "field B invalid"}}
	expected := "field A required; field B invalid"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestValidationError_Nil(t *testing.T) {
	var err *ValidationError
	if err.Error() != "<nil>" {
		t.Errorf("expected '<nil>', got %q", err.Error())
	}
}

func TestValidationError_EmptySlice(t *testing.T) {
	err := &ValidationError{Errors: []string{}}
	if err.Error() != "" {
		t.Errorf("expected empty string, got %q", err.Error())
	}
}

/** ServerCustomError.Error() */

func TestServerCustomError_Format(t *testing.T) {
	err := &ServerCustomError{Code: 422, Errors: []string{"field X is required", "field Y must be an email"}}
	expected := "code=422: field X is required; field Y must be an email"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestServerCustomError_Nil(t *testing.T) {
	var err *ServerCustomError
	if err.Error() != "<nil>" {
		t.Errorf("expected '<nil>', got %q", err.Error())
	}
}

func TestServerCustomError_SingleError(t *testing.T) {
	err := &ServerCustomError{Code: 409, Errors: []string{"conflict"}}
	expected := "code=409: conflict"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

/** FiberErrorHandler */

// parseRespBody is a helper for reading Fiber test responses.
func parseRespBody(t *testing.T, body io.ReadCloser) response.Response {
	t.Helper()
	defer body.Close()
	data, _ := io.ReadAll(body)
	var r response.Response
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	return r
}

func TestFiberErrorHandler_GatewayTimeout(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: FiberErrorHandler})
	app.Get("/test", func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusGatewayTimeout, "gateway timeout")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != 504 {
		t.Errorf("expected 504, got %d", resp.StatusCode)
	}
}

func TestFiberErrorHandler_RequestTimeout(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: FiberErrorHandler})
	app.Get("/test", func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusRequestTimeout, "request timeout")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, _ := app.Test(req, -1)

	r := parseRespBody(t, resp.Body)
	if r.Status != 504 {
		t.Errorf("expected 504 in body for 408 error, got %d", r.Status)
	}
}

func TestFiberErrorHandler_DeadlineExceeded(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: FiberErrorHandler})
	app.Get("/test", func(c *fiber.Ctx) error {
		return context.DeadlineExceeded
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, _ := app.Test(req, -1)

	r := parseRespBody(t, resp.Body)
	if r.Status != 504 {
		t.Errorf("expected 504 for DeadlineExceeded, got %d", r.Status)
	}
}

func TestFiberErrorHandler_Canceled(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: FiberErrorHandler})
	app.Get("/test", func(c *fiber.Ctx) error {
		return context.Canceled
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, _ := app.Test(req, -1)

	r := parseRespBody(t, resp.Body)
	if r.Status != 504 {
		t.Errorf("expected 504 for Canceled, got %d", r.Status)
	}
}

func TestFiberErrorHandler_ValidationError(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: FiberErrorHandler})
	app.Get("/test", func(c *fiber.Ctx) error {
		return &ValidationError{Errors: []string{"field required"}}
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}

	r := parseRespBody(t, resp.Body)
	if r.Status != 400 {
		t.Errorf("expected body status 400, got %d", r.Status)
	}
	found := false
	for _, e := range r.Errors {
		if strings.Contains(e, "field required") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'field required' in errors, got %v", r.Errors)
	}
}

func TestFiberErrorHandler_ServerCustomError(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: FiberErrorHandler})
	app.Get("/test", func(c *fiber.Ctx) error {
		return &ServerCustomError{Code: 422, Errors: []string{"invalid input"}}
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, _ := app.Test(req, -1)

	r := parseRespBody(t, resp.Body)
	if r.Status != 422 {
		t.Errorf("expected body status 422, got %d", r.Status)
	}
}

func TestFiberErrorHandler_NotFound(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: FiberErrorHandler})
	app.Get("/test", func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusNotFound, "not found")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != 404 {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestFiberErrorHandler_GenericError(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: FiberErrorHandler})
	app.Get("/test", func(c *fiber.Ctx) error {
		return errors.New("something went wrong")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != 500 {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

func TestFiberErrorHandler_WrappedDeadlineExceeded(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: FiberErrorHandler})
	app.Get("/test", func(c *fiber.Ctx) error {
		return errors.Join(errors.New("operation failed"), context.DeadlineExceeded)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, _ := app.Test(req, -1)

	r := parseRespBody(t, resp.Body)
	if r.Status != 504 {
		t.Errorf("expected 504 for wrapped DeadlineExceeded, got %d", r.Status)
	}
}
