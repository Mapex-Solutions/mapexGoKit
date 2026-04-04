package validations

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	custom "github.com/Mapex-Solutions/mapexGoKit/utils/validations/customvalidation"
)

// registerCustoms registers custom validation rules for the validator.Validate instance.
//
// This function is responsible for registering custom validation rules for the provided
// validator.Validate instance. The custom validation rules are implemented in the customvalidation
// package.
//
// Parameters:
//   - v: A pointer to a validator.Validate instance. This instance will have the custom validation
//     rules registered after this function is called.
//
// Return:
// This function does not return any value. However, it modifies the provided validator.Validate
// instance by registering custom validation rules.
func registerCustoms(v *validator.Validate) {
	// Custom validation rules
	custom.RegisterMongoID(v)
	custom.RegisterUUID(v)
}

// formatValidationError formats a validation error into a human-readable error message.
//
// Parameters:
//   - err: The error to be formatted, expected to be of type validator.ValidationErrors.
//
// Returns:
//   - An error with a formatted message if the input error is of type validator.ValidationErrors.
//     Otherwise, returns the original error.
//
// formatValidationError converts validator.ValidationErrors into []string with
// friendly, tag-specific messages. It NEVER returns a single string.
func formatValidationError(err error) []string {
	if ve, ok := err.(validator.ValidationErrors); ok {
		msgs := make([]string, 0, len(ve))
		for _, fe := range ve {
			msgs = append(msgs, humanizeFieldError(fe))
		}
		return msgs
	}
	// Fallback: wrap the original error in a single-element slice.
	return []string{err.Error()}
}

// humanizeFieldError maps common tags to clear messages.
// Unknown tags fall back to a generic "invalid (tag[=param])" message.
func humanizeFieldError(fe validator.FieldError) string {
	field := fe.Field()
	tag := fe.ActualTag()
	param := fe.Param()

	switch tag {

	// Presence
	case "required":
		return fmt.Sprintf("The field '%s' is required.", field)

	// Equality / inequality against other fields
	case "eqfield":
		return fmt.Sprintf("The field '%s' must be equal to the field '%s'.", field, param)
	case "nefield":
		return fmt.Sprintf("The field '%s' must be different from the field '%s'.", field, param)

	// Length / range (string, slice, map -> characters/items; numeric -> value)
	case "len":
		return fmt.Sprintf("The field '%s' must be exactly %s characters long.", field, param)
	case "min":
		if isStringy(fe) {
			return fmt.Sprintf("The field '%s' must have at least %s characters.", field, param)
		}
		return fmt.Sprintf("The field '%s' must be at least %s.", field, param)
	case "max":
		if isStringy(fe) {
			return fmt.Sprintf("The field '%s' must have at most %s characters.", field, param)
		}
		return fmt.Sprintf("The field '%s' must be at most %s.", field, param)
	case "gt":
		return fmt.Sprintf("The field '%s' must be greater than %s.", field, param)
	case "gte":
		return fmt.Sprintf("The field '%s' must be greater than or equal to %s.", field, param)
	case "lt":
		return fmt.Sprintf("The field '%s' must be less than %s.", field, param)
	case "lte":
		return fmt.Sprintf("The field '%s' must be less than or equal to %s.", field, param)

	// Format / type validators
	case "email":
		return fmt.Sprintf("The field '%s' must be a valid email.", field)
	case "alphanum":
		return fmt.Sprintf("The field '%s' must contain only alphanumeric characters.", field)
	case "alpha":
		return fmt.Sprintf("The field '%s' must contain only letters.", field)
	case "numeric":
		return fmt.Sprintf("The field '%s' must contain only numbers.", field)
	case "boolean":
		return fmt.Sprintf("The field '%s' must be a boolean.", field)
	case "url":
		return fmt.Sprintf("The field '%s' must be a valid URL.", field)
	case "uuid":
		return fmt.Sprintf("The field '%s' must be a valid UUID.", field)
	case "ipv4":
		return fmt.Sprintf("The field '%s' must be a valid IPv4 address.", field)
	case "ipv6":
		return fmt.Sprintf("The field '%s' must be a valid IPv6 address.", field)

	// Time / date
	case "datetime": // uses param as layout, e.g. "2006-01-02"
		if param != "" {
			return fmt.Sprintf("The field '%s' must match the date/time format '%s'.", field, param)
		}
		return fmt.Sprintf("The field '%s' must be a valid date/time.", field)

	// Membership / enums
	case "oneof": // param is like: "A B C"
		options := strings.ReplaceAll(param, " ", ", ")
		return fmt.Sprintf("The field '%s' must be one of: %s.", field, options)

	// Custom examples you mentioned/are using
	case "mongoid":
		return fmt.Sprintf("The field '%s' must be a valid MongoDB ObjectID (24 hex chars).", field)

	// Add any other custom tags here...

	default:
		// Sensible default with tag (+ param if present)
		if param != "" {
			return fmt.Sprintf("The field '%s' is invalid (%s=%s).", field, tag, param)
		}
		return fmt.Sprintf("The field '%s' is invalid (%s).", field, tag)
	}
}

// isStringy helps craft messages for min/max/len depending on the field kind.
func isStringy(fe validator.FieldError) bool {
	switch fe.Kind() {
	case reflect.String, reflect.Slice, reflect.Map, reflect.Array:
		return true
	default:
		return false
	}
}
