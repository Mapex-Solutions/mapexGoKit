package customvalidation

import (
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// RegisterUUID registers the "uuid" custom validation tag.
func RegisterUUID(v *validator.Validate) {
	_ = v.RegisterValidation("uuid", func(fl validator.FieldLevel) bool {
		_, err := uuid.Parse(fl.Field().String())
		return err == nil
	})
}
