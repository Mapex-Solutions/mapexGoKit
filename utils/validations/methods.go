package validations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ValidateStruct validates a struct using the singleton validator.
//
// Parameters:
//   - s: The struct to be validated.
//
// Returns:
//   - An error if validation fails, formatted as a human-readable message.
//     Returns nil if validation is successful.
func ValidateStruct(s any) []string {
	if err := New().Struct(s); err != nil {
		return formatValidationError(err)
	}
	return nil
}

// ValidateStructCtx validates a struct using the singleton validator with context support.
//
// Parameters:
//   - ctx: The context to control cancellation and deadlines.
//   - s: The struct to be validated.
//
// Returns:
//   - An error if validation fails, formatted as a human-readable message.
//     Returns nil if validation is successful.
func ValidateStructCtx(ctx context.Context, s any) []string {
	if err := New().StructCtx(ctx, s); err != nil {
		return formatValidationError(err)
	}
	return nil
}

// ParseTypeError recebe um erro de parsing JSON e, se for do tipo
// *json.UnmarshalTypeError, retorna uma string explicando o campo,
// o tipo esperado e o tipo recebido.
//
// Exemplo de saída: "campo changePasswordNextLogin esperado bool veio string"
func ParseTypeError(err error) []string {
	var ute *json.UnmarshalTypeError
	if errors.As(err, &ute) {
		field := ute.Field
		expected := ute.Type.String() // tipo Go esperado
		got := ute.Value              // valor JSON recebido (string, number, bool...)
		return []string{
			fmt.Sprintf("field %s expected %s but came %s", field, expected, got),
		}
	}
	// Se não for erro de tipo, devolve genérico no mesmo formato
	return []string{fmt.Sprintf("erro de parsing: %v", err)}
}
