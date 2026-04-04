package customvalidation

import (
	"regexp"

	"github.com/go-playground/validator/v10"
)

// Precompiled regex for MongoDB ObjectIDs (24 hex chars).
var mongoIDRegex = regexp.MustCompile(`^[a-fA-F0-9]{24}$`)

// RegisterMongoID registers the "mongoid" custom validation tag.
func RegisterMongoID(v *validator.Validate) {
	_ = v.RegisterValidation("mongoid", func(fl validator.FieldLevel) bool {
		val := fl.Field().String()
		return mongoIDRegex.MatchString(val)
	})
}
