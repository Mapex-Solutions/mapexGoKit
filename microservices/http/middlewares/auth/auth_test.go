package middlewaresAuth

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	jwtv4 "github.com/golang-jwt/jwt/v4"
	jwtv5 "github.com/golang-jwt/jwt/v5"

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

// signTestToken creates a valid HS256 JWT for testing.
func signTestToken(claims jwtv5.MapClaims, secret string) string {
	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	s, _ := token.SignedString([]byte(secret))
	return s
}

// parseResponseBody parses the JSON response body into response.Response.
func parseResponseBody(t *testing.T, resp *io.ReadCloser) response.Response {
	t.Helper()
	body, _ := io.ReadAll(*resp)
	var r response.Response
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	return r
}

/** AuthMiddleware */

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	app := fiber.New()
	app.Use(AuthMiddleware(AuthConfig{Strategy: "jwt", Secret: "secret", Algorithm: "HS256"}))
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_JWT_ValidToken(t *testing.T) {
	secret := "test-secret"
	tokenStr := signTestToken(jwtv5.MapClaims{
		"userId": "user-abc",
		"exp":    time.Now().Add(time.Hour).Unix(),
	}, secret)

	var userClaims jwtv5.MapClaims
	var tokenLocal string

	app := fiber.New()
	app.Use(AuthMiddleware(AuthConfig{Strategy: "jwt", Secret: secret, Algorithm: "HS256"}))
	app.Get("/test", func(c *fiber.Ctx) error {
		userClaims, _ = c.Locals("user").(jwtv5.MapClaims)
		tokenLocal, _ = c.Locals("token").(string)
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if userClaims["userId"] != "user-abc" {
		t.Errorf("expected userId 'user-abc', got %v", userClaims["userId"])
	}
	if tokenLocal != tokenStr {
		t.Error("expected token to be stored in Locals")
	}
}

func TestAuthMiddleware_JWT_InvalidToken(t *testing.T) {
	app := fiber.New()
	app.Use(AuthMiddleware(AuthConfig{Strategy: "jwt", Secret: "secret", Algorithm: "HS256"}))
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 401 {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_UnknownStrategy(t *testing.T) {
	app := fiber.New()
	app.Use(AuthMiddleware(AuthConfig{Strategy: "unknown"}))
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 401 {
		t.Errorf("expected 401 for unknown strategy, got %d", resp.StatusCode)
	}
}

func TestAuthMiddleware_JWT_ExpiredToken(t *testing.T) {
	secret := "test-secret"
	tokenStr := signTestToken(jwtv5.MapClaims{
		"userId": "user-abc",
		"exp":    time.Now().Add(-time.Hour).Unix(),
	}, secret)

	app := fiber.New()
	app.Use(AuthMiddleware(AuthConfig{Strategy: "jwt", Secret: secret, Algorithm: "HS256"}))
	app.Get("/test", func(c *fiber.Ctx) error { return c.SendStatus(200) })

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 401 {
		t.Errorf("expected 401 for expired token, got %d", resp.StatusCode)
	}
}

/** GetUserIdFromToken */

func TestGetUserIdFromToken_ValidClaims(t *testing.T) {
	app := fiber.New()
	var userID string
	var ok bool

	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("user", jwtv5.MapClaims{"userId": "user-123"})
		userID, ok = GetUserIdFromToken(c)
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req, -1)

	if !ok {
		t.Error("expected ok to be true")
	}
	if userID != "user-123" {
		t.Errorf("expected 'user-123', got %q", userID)
	}
}

func TestGetUserIdFromToken_NilLocals(t *testing.T) {
	app := fiber.New()
	var userID string
	var ok bool

	app.Get("/test", func(c *fiber.Ctx) error {
		userID, ok = GetUserIdFromToken(c)
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req, -1)

	if ok {
		t.Error("expected ok to be false when Locals is nil")
	}
	if userID != "" {
		t.Errorf("expected empty string, got %q", userID)
	}
}

func TestGetUserIdFromToken_MissingUserId(t *testing.T) {
	app := fiber.New()
	var ok bool

	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("user", jwtv5.MapClaims{"role": "admin"})
		_, ok = GetUserIdFromToken(c)
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req, -1)

	if ok {
		t.Error("expected ok to be false when userId is missing")
	}
}

func TestGetUserIdFromToken_UserIdNotString(t *testing.T) {
	app := fiber.New()
	var ok bool

	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("user", jwtv5.MapClaims{"userId": 12345})
		_, ok = GetUserIdFromToken(c)
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req, -1)

	if ok {
		t.Error("expected ok to be false when userId is not a string")
	}
}

func TestGetUserIdFromToken_EmptyUserId(t *testing.T) {
	app := fiber.New()
	var ok bool

	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("user", jwtv5.MapClaims{"userId": ""})
		_, ok = GetUserIdFromToken(c)
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req, -1)

	if ok {
		t.Error("expected ok to be false when userId is empty string")
	}
}

/** convertMapClaimsV4ToV5 */

func TestConvertMapClaimsV4ToV5_CopiesAll(t *testing.T) {
	v4Claims := jwtv4.MapClaims{
		"userId": "user-abc",
		"role":   "admin",
		"exp":    float64(9999999999),
	}

	v5Claims := convertMapClaimsV4ToV5(v4Claims)

	if len(v5Claims) != len(v4Claims) {
		t.Errorf("expected %d claims, got %d", len(v4Claims), len(v5Claims))
	}
	if v5Claims["userId"] != "user-abc" {
		t.Errorf("expected userId 'user-abc', got %v", v5Claims["userId"])
	}
	if v5Claims["role"] != "admin" {
		t.Errorf("expected role 'admin', got %v", v5Claims["role"])
	}
}

func TestConvertMapClaimsV4ToV5_EmptyMap(t *testing.T) {
	v4Claims := jwtv4.MapClaims{}
	v5Claims := convertMapClaimsV4ToV5(v4Claims)

	if len(v5Claims) != 0 {
		t.Errorf("expected 0 claims, got %d", len(v5Claims))
	}
}
