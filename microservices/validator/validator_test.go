package validator

import (
	"testing"
)

/** Test DTOs */

type validDTO struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" default:"18"`
}

type optionalDTO struct {
	Name string `json:"name"`
	Tag  string `json:"tag" default:"default-tag"`
}

type transformDTO struct {
	Value string `json:"value" validate:"required"`
}

func (d *transformDTO) Transform() error {
	d.Value = "transformed:" + d.Value
	return nil
}

/** validateDTOType */

func TestValidateDTOType_Nil(t *testing.T) {
	err := validateDTOType(nil)
	if err == nil {
		t.Error("expected error for nil input")
	}
}

func TestValidateDTOType_NonPtr(t *testing.T) {
	err := validateDTOType(validDTO{})
	if err == nil {
		t.Error("expected error for non-pointer input")
	}
}

func TestValidateDTOType_PtrToNonStruct(t *testing.T) {
	s := "hello"
	err := validateDTOType(&s)
	if err == nil {
		t.Error("expected error for pointer to non-struct")
	}
}

func TestValidateDTOType_Valid(t *testing.T) {
	err := validateDTOType(&validDTO{})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

/** UnmarshalAndValidate */

func TestUnmarshalAndValidate_Valid(t *testing.T) {
	data := []byte(`{"name":"John","email":"john@example.com","age":30}`)
	dto := &validDTO{}
	err := UnmarshalAndValidate(data, dto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "John" {
		t.Errorf("expected 'John', got %q", dto.Name)
	}
	if dto.Email != "john@example.com" {
		t.Errorf("expected 'john@example.com', got %q", dto.Email)
	}
	if dto.Age != 30 {
		t.Errorf("expected 30, got %d", dto.Age)
	}
}

func TestUnmarshalAndValidate_InvalidJSON(t *testing.T) {
	data := []byte(`{invalid json}`)
	dto := &validDTO{}
	err := UnmarshalAndValidate(data, dto)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestUnmarshalAndValidate_MissingRequired(t *testing.T) {
	data := []byte(`{"age": 25}`)
	dto := &validDTO{}
	err := UnmarshalAndValidate(data, dto)
	if err == nil {
		t.Error("expected error for missing required fields")
	}
}

func TestUnmarshalAndValidate_DefaultApplied(t *testing.T) {
	data := []byte(`{"name":"John","email":"john@example.com"}`)
	dto := &validDTO{}
	err := UnmarshalAndValidate(data, dto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Age != 18 {
		t.Errorf("expected default age 18, got %d", dto.Age)
	}
}

func TestUnmarshalAndValidate_WithTransform(t *testing.T) {
	data := []byte(`{"value":"hello"}`)
	dto := &transformDTO{}
	err := UnmarshalAndValidate(data, dto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Value != "transformed:hello" {
		t.Errorf("expected 'transformed:hello', got %q", dto.Value)
	}
}

func TestUnmarshalAndValidate_NilDTO(t *testing.T) {
	data := []byte(`{"name":"John"}`)
	err := UnmarshalAndValidate(data, nil)
	if err == nil {
		t.Error("expected error for nil DTO")
	}
}

func TestUnmarshalAndValidate_InvalidEmail(t *testing.T) {
	data := []byte(`{"name":"John","email":"not-an-email"}`)
	dto := &validDTO{}
	err := UnmarshalAndValidate(data, dto)
	if err == nil {
		t.Error("expected error for invalid email")
	}
}

/** Validate */

func TestValidate_Valid(t *testing.T) {
	dto := &validDTO{Name: "John", Email: "john@example.com", Age: 25}
	err := Validate(dto)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_Invalid(t *testing.T) {
	dto := &validDTO{Name: "", Email: ""}
	err := Validate(dto)
	if err == nil {
		t.Error("expected error for invalid DTO")
	}
}

func TestValidate_DefaultsApplied(t *testing.T) {
	dto := &optionalDTO{Name: "Test"}
	err := Validate(dto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Tag != "default-tag" {
		t.Errorf("expected 'default-tag', got %q", dto.Tag)
	}
}

func TestValidate_WithTransform(t *testing.T) {
	dto := &transformDTO{Value: "hello"}
	err := Validate(dto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Value != "transformed:hello" {
		t.Errorf("expected 'transformed:hello', got %q", dto.Value)
	}
}

func TestValidate_Nil(t *testing.T) {
	err := Validate(nil)
	if err == nil {
		t.Error("expected error for nil DTO")
	}
}

func TestValidate_NonPtr(t *testing.T) {
	err := Validate(validDTO{Name: "John", Email: "john@example.com"})
	if err == nil {
		t.Error("expected error for non-pointer DTO")
	}
}

/** runTransformsDeep */

func TestRunTransformsDeep_Nil(t *testing.T) {
	err := runTransformsDeep(nil)
	if err != nil {
		t.Errorf("expected nil for nil input, got %v", err)
	}
}

func TestRunTransformsDeep_WithTransformer(t *testing.T) {
	dto := &transformDTO{Value: "test"}
	err := runTransformsDeep(dto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Value != "transformed:test" {
		t.Errorf("expected 'transformed:test', got %q", dto.Value)
	}
}

func TestRunTransformsDeep_WithoutTransformer(t *testing.T) {
	dto := &validDTO{Name: "test"}
	err := runTransformsDeep(dto)
	if err != nil {
		t.Errorf("expected nil for non-transformer, got %v", err)
	}
}
