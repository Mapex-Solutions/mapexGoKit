package validations

import "github.com/go-playground/validator/v10"

// Lightweight re-exports so consumers don't need to import validator directly.
type (
	Validate      = validator.Validate
	FieldLevel    = validator.FieldLevel
	CustomFunc    = validator.Func
	CustomFuncCtx = validator.FuncCtx
)

// ValidationError represents a single validation failure.
type ValidationError struct {
	Field string `json:"field"`
	Tag   string `json:"tag"`
	Param string `json:"param,omitempty"`
}

// ValidationErrors is a slice of ValidationError.
// It implements the error interface so it can be returned directly.
type ValidationErrors []string

func (ves ValidationErrors) Error() string {
	return "validation failed"
}
