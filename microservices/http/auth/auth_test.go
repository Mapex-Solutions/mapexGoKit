package auth

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

/** ValidateAPIKey */

func TestValidateAPIKey_HeaderMatch(t *testing.T) {
	app := fiber.New()
	var result bool

	app.Get("/test", func(c *fiber.Ctx) error {
		result = ValidateAPIKey(c, "my-secret-key", "header", "X-API-Key")
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "my-secret-key")
	_, _ = app.Test(req, -1)

	if !result {
		t.Error("expected ValidateAPIKey to return true for matching header")
	}
}

func TestValidateAPIKey_HeaderMismatch(t *testing.T) {
	app := fiber.New()
	var result bool

	app.Get("/test", func(c *fiber.Ctx) error {
		result = ValidateAPIKey(c, "my-secret-key", "header", "X-API-Key")
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	_, _ = app.Test(req, -1)

	if result {
		t.Error("expected ValidateAPIKey to return false for mismatched header")
	}
}

func TestValidateAPIKey_HeaderMissing(t *testing.T) {
	app := fiber.New()
	var result bool

	app.Get("/test", func(c *fiber.Ctx) error {
		result = ValidateAPIKey(c, "my-secret-key", "header", "X-API-Key")
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req, -1)

	if result {
		t.Error("expected ValidateAPIKey to return false when header is absent")
	}
}

func TestValidateAPIKey_QueryMatch(t *testing.T) {
	app := fiber.New()
	var result bool

	app.Get("/test", func(c *fiber.Ctx) error {
		result = ValidateAPIKey(c, "my-secret-key", "query", "apiKey")
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test?apiKey=my-secret-key", nil)
	_, _ = app.Test(req, -1)

	if !result {
		t.Error("expected ValidateAPIKey to return true for matching query param")
	}
}

func TestValidateAPIKey_QueryMismatch(t *testing.T) {
	app := fiber.New()
	var result bool

	app.Get("/test", func(c *fiber.Ctx) error {
		result = ValidateAPIKey(c, "my-secret-key", "query", "apiKey")
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test?apiKey=wrong-key", nil)
	_, _ = app.Test(req, -1)

	if result {
		t.Error("expected ValidateAPIKey to return false for mismatched query param")
	}
}

func TestValidateAPIKey_InvalidFieldType(t *testing.T) {
	app := fiber.New()
	var result bool

	app.Get("/test", func(c *fiber.Ctx) error {
		result = ValidateAPIKey(c, "my-secret-key", "body", "apiKey")
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req, -1)

	if result {
		t.Error("expected ValidateAPIKey to return false for unsupported fieldType")
	}
}

/** ParseJWTTokenWithSecret */

func TestParseJWTTokenWithSecret_ValidHS256(t *testing.T) {
	secret := "test-secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": "user-abc",
		"exp":    time.Now().Add(time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	parsedToken, claims, err := ParseJWTTokenWithSecret(tokenString, secret, "HS256")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if parsedToken == nil {
		t.Fatal("expected non-nil token")
	}
	if claims["userId"] != "user-abc" {
		t.Errorf("expected userId 'user-abc', got %v", claims["userId"])
	}
}

func TestParseJWTTokenWithSecret_WrongSecret(t *testing.T) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": "user-abc",
		"exp":    time.Now().Add(time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte("correct-secret"))

	_, _, err := ParseJWTTokenWithSecret(tokenString, "wrong-secret", "HS256")
	if err == nil {
		t.Error("expected error for wrong secret")
	}
}

func TestParseJWTTokenWithSecret_WrongAlgorithm(t *testing.T) {
	secret := "test-secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": "user-abc",
		"exp":    time.Now().Add(time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte(secret))

	_, _, err := ParseJWTTokenWithSecret(tokenString, secret, "RS256")
	if err == nil {
		t.Error("expected error for algorithm mismatch")
	}
}

func TestParseJWTTokenWithSecret_Malformed(t *testing.T) {
	_, _, err := ParseJWTTokenWithSecret("not.a.valid.token", "secret", "HS256")
	if err == nil {
		t.Error("expected error for malformed token")
	}
}

func TestParseJWTTokenWithSecret_Expired(t *testing.T) {
	secret := "test-secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": "user-abc",
		"exp":    time.Now().Add(-time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte(secret))

	_, _, err := ParseJWTTokenWithSecret(tokenString, secret, "HS256")
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestParseJWTTokenWithSecret_ClaimsPreserved(t *testing.T) {
	secret := "test-secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": "user-123",
		"role":   "admin",
		"orgId":  "org-456",
		"exp":    time.Now().Add(time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte(secret))

	_, claims, err := ParseJWTTokenWithSecret(tokenString, secret, "HS256")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims["userId"] != "user-123" {
		t.Errorf("expected userId 'user-123', got %v", claims["userId"])
	}
	if claims["role"] != "admin" {
		t.Errorf("expected role 'admin', got %v", claims["role"])
	}
	if claims["orgId"] != "org-456" {
		t.Errorf("expected orgId 'org-456', got %v", claims["orgId"])
	}
}

/** ValidateIPWhitelist */

func TestValidateIPWhitelist_ExactMatch(t *testing.T) {
	app := fiber.New()
	var result bool

	app.Get("/test", func(c *fiber.Ctx) error {
		result = ValidateIPWhitelist(c, []string{"0.0.0.0"})
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req, -1)

	// fiber.Test uses 0.0.0.0 as client IP by default
	if !result {
		t.Error("expected ValidateIPWhitelist to return true for exact IP match")
	}
}

func TestValidateIPWhitelist_IPNotInList(t *testing.T) {
	app := fiber.New()
	var result bool

	app.Get("/test", func(c *fiber.Ctx) error {
		result = ValidateIPWhitelist(c, []string{"192.168.1.100"})
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req, -1)

	if result {
		t.Error("expected ValidateIPWhitelist to return false when IP is not in list")
	}
}

func TestValidateIPWhitelist_CIDRMatch(t *testing.T) {
	app := fiber.New()
	var result bool

	app.Get("/test", func(c *fiber.Ctx) error {
		result = ValidateIPWhitelist(c, []string{"0.0.0.0/0"})
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req, -1)

	if !result {
		t.Error("expected ValidateIPWhitelist to return true for CIDR range match")
	}
}

func TestValidateIPWhitelist_EmptyList(t *testing.T) {
	app := fiber.New()
	var result bool

	app.Get("/test", func(c *fiber.Ctx) error {
		result = ValidateIPWhitelist(c, []string{})
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req, -1)

	if result {
		t.Error("expected ValidateIPWhitelist to return false for empty list")
	}
}

func TestValidateIPWhitelist_WhitespaceEntry(t *testing.T) {
	app := fiber.New()
	var result bool

	app.Get("/test", func(c *fiber.Ctx) error {
		result = ValidateIPWhitelist(c, []string{"  0.0.0.0  "})
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req, -1)

	if !result {
		t.Error("expected ValidateIPWhitelist to trim whitespace and match")
	}
}

/** hasLetters */

func TestHasLetters_WithLetters(t *testing.T) {
	if !hasLetters("hello") {
		t.Error("expected true for string with letters")
	}
}

func TestHasLetters_OnlyNumbers(t *testing.T) {
	if hasLetters("12345") {
		t.Error("expected false for string with only numbers")
	}
}

func TestHasLetters_Empty(t *testing.T) {
	if hasLetters("") {
		t.Error("expected false for empty string")
	}
}

func TestHasLetters_MixedWithDots(t *testing.T) {
	if !hasLetters("192.168.1.abc") {
		t.Error("expected true for string containing letters")
	}
}

func TestHasLetters_OnlyDotsAndNumbers(t *testing.T) {
	if hasLetters("192.168.1.1") {
		t.Error("expected false for IP-like string without letters")
	}
}

func TestHasLetters_UpperCase(t *testing.T) {
	if !hasLetters("ABC") {
		t.Error("expected true for uppercase letters")
	}
}
