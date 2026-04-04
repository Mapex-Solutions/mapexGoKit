package requestValidation

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"reflect"
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

/** Test DTOs */

type testBodyDTO struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" default:"18"`
}

type testQueryDTO struct {
	Page  int `query:"page" default:"1"`
	Limit int `query:"limit" default:"10"`
}

type testParamsDTO struct {
	ID string `params:"id" validate:"required"`
}

type testTransformDTO struct {
	Name string `json:"name" validate:"required"`
}

func (d *testTransformDTO) Transform() error {
	d.Name = "transformed:" + d.Name
	return nil
}

/** getType */

func TestGetType_Nil(t *testing.T) {
	result := getType(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
}

func TestGetType_PtrToStruct(t *testing.T) {
	result := getType(&testBodyDTO{})
	if result == nil {
		t.Fatal("expected non-nil type")
	}
	if result.Kind() != reflect.Struct {
		t.Errorf("expected struct kind, got %v", result.Kind())
	}
}

func TestGetType_NonPtr_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for non-pointer input")
		}
	}()
	getType(testBodyDTO{})
}

func TestGetType_PtrToNonStruct_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for pointer to non-struct")
		}
	}()
	s := "hello"
	getType(&s)
}

/** GetDTO */

func TestGetDTO_Valid(t *testing.T) {
	app := fiber.New()

	var result *testBodyDTO
	var getErr error

	app.Get("/test", func(c *fiber.Ctx) error {
		dto := &testBodyDTO{Name: "test", Email: "test@test.com"}
		c.Locals("bodyDTO", dto)
		result, getErr = GetDTO[*testBodyDTO](c, "bodyDTO")
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req, -1)

	if getErr != nil {
		t.Fatalf("unexpected error: %v", getErr)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Name != "test" {
		t.Errorf("expected 'test', got %q", result.Name)
	}
}

func TestGetDTO_WrongType(t *testing.T) {
	app := fiber.New()

	var getErr error

	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("bodyDTO", "not a dto")
		_, getErr = GetDTO[*testBodyDTO](c, "bodyDTO")
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req, -1)

	if getErr == nil {
		t.Error("expected error for wrong type")
	}
}

func TestGetDTO_Missing(t *testing.T) {
	app := fiber.New()

	var getErr error

	app.Get("/test", func(c *fiber.Ctx) error {
		_, getErr = GetDTO[*testBodyDTO](c, "bodyDTO")
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	_, _ = app.Test(req, -1)

	if getErr == nil {
		t.Error("expected error when key is missing")
	}
}

/** ValidationMiddleware */

func parseTestResponse(t *testing.T, body io.ReadCloser) response.Response {
	t.Helper()
	defer body.Close()
	data, _ := io.ReadAll(body)
	var r response.Response
	json.Unmarshal(data, &r)
	return r
}

func TestValidationMiddleware_ValidBody(t *testing.T) {
	v := NewValidation(&testBodyDTO{}, nil, nil)

	var storedDTO *testBodyDTO

	app := fiber.New()
	app.Post("/test", ValidationMiddleware(v), func(c *fiber.Ctx) error {
		dto, err := GetDTO[*testBodyDTO](c, "bodyDTO")
		if err != nil {
			return c.SendStatus(500)
		}
		storedDTO = dto
		return c.SendStatus(200)
	})

	body := `{"name":"John","email":"john@example.com"}`
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if storedDTO == nil {
		t.Fatal("expected storedDTO to be set")
	}
	if storedDTO.Name != "John" {
		t.Errorf("expected 'John', got %q", storedDTO.Name)
	}
	if storedDTO.Age != 18 {
		t.Errorf("expected default age 18, got %d", storedDTO.Age)
	}
}

func TestValidationMiddleware_InvalidBody(t *testing.T) {
	v := NewValidation(&testBodyDTO{}, nil, nil)

	app := fiber.New()
	app.Post("/test", ValidationMiddleware(v), func(c *fiber.Ctx) error {
		return c.SendStatus(200)
	})

	// Missing required fields
	body := `{"age": 25}`
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 400 {
		t.Errorf("expected 400 for invalid body, got %d", resp.StatusCode)
	}
}

func TestValidationMiddleware_QueryParsed(t *testing.T) {
	v := NewValidation(nil, &testQueryDTO{}, nil)

	var storedDTO *testQueryDTO

	app := fiber.New()
	app.Get("/test", ValidationMiddleware(v), func(c *fiber.Ctx) error {
		dto, err := GetDTO[*testQueryDTO](c, "queryDTO")
		if err != nil {
			return c.SendStatus(500)
		}
		storedDTO = dto
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test?page=5&limit=20", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if storedDTO == nil {
		t.Fatal("expected queryDTO to be set")
	}
	if storedDTO.Page != 5 {
		t.Errorf("expected page 5, got %d", storedDTO.Page)
	}
	if storedDTO.Limit != 20 {
		t.Errorf("expected limit 20, got %d", storedDTO.Limit)
	}
}

func TestValidationMiddleware_DefaultsApplied(t *testing.T) {
	v := NewValidation(nil, &testQueryDTO{}, nil)

	var storedDTO *testQueryDTO

	app := fiber.New()
	app.Get("/test", ValidationMiddleware(v), func(c *fiber.Ctx) error {
		dto, _ := GetDTO[*testQueryDTO](c, "queryDTO")
		storedDTO = dto
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if storedDTO == nil {
		t.Fatal("expected queryDTO to be set")
	}
	if storedDTO.Page != 1 {
		t.Errorf("expected default page 1, got %d", storedDTO.Page)
	}
	if storedDTO.Limit != 10 {
		t.Errorf("expected default limit 10, got %d", storedDTO.Limit)
	}
}

func TestValidationMiddleware_TransformCalled(t *testing.T) {
	v := NewValidation(&testTransformDTO{}, nil, nil)

	var storedDTO *testTransformDTO

	app := fiber.New()
	app.Post("/test", ValidationMiddleware(v), func(c *fiber.Ctx) error {
		dto, _ := GetDTO[*testTransformDTO](c, "bodyDTO")
		storedDTO = dto
		return c.SendStatus(200)
	})

	body := `{"name":"hello"}`
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if storedDTO == nil {
		t.Fatal("expected bodyDTO to be set")
	}
	if storedDTO.Name != "transformed:hello" {
		t.Errorf("expected 'transformed:hello', got %q", storedDTO.Name)
	}
}

/** runTransformsDeep */

func TestRunTransformsDeep_Nil(t *testing.T) {
	err := runTransformsDeep(nil)
	if err != nil {
		t.Errorf("expected nil error for nil input, got %v", err)
	}
}

func TestRunTransformsDeep_WithTransformer(t *testing.T) {
	dto := &testTransformDTO{Name: "test"}
	err := runTransformsDeep(dto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "transformed:test" {
		t.Errorf("expected 'transformed:test', got %q", dto.Name)
	}
}

func TestRunTransformsDeep_WithoutTransformer(t *testing.T) {
	dto := &testBodyDTO{Name: "test"}
	err := runTransformsDeep(dto)
	if err != nil {
		t.Errorf("expected nil error for struct without Transform, got %v", err)
	}
}

/** NewValidation */

func TestNewValidation_AllNil(t *testing.T) {
	v := NewValidation(nil, nil, nil)
	if v.bodyType != nil || v.queryType != nil || v.paramsType != nil {
		t.Error("expected all types to be nil")
	}
}

func TestNewValidation_WithBody(t *testing.T) {
	v := NewValidation(&testBodyDTO{}, nil, nil)
	if v.bodyType == nil {
		t.Error("expected bodyType to be non-nil")
	}
	if v.queryType != nil {
		t.Error("expected queryType to be nil")
	}
}

func TestNewValidation_WithAll(t *testing.T) {
	v := NewValidation(&testBodyDTO{}, &testQueryDTO{}, &testParamsDTO{})
	if v.bodyType == nil || v.queryType == nil || v.paramsType == nil {
		t.Error("expected all types to be non-nil")
	}
}
