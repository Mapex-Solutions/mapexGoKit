package requestValidation

import (
	"fmt"
	"reflect"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/response"
)

// prefixErrors adds a context prefix to each error message for easier debugging.
func prefixErrors(context string, errs []string) []string {
	result := make([]string, len(errs))
	for i, err := range errs {
		result[i] = fmt.Sprintf("[%s] %s", context, err)
	}
	return result
}

// validateInstance already exists in your file:
// var validateInstance = validator.New()

// Struct validates a struct value using the shared global validator instance.
//
// It returns an error that may be of type validator.ValidationErrors,
// which is a slice of FieldError describing each failed rule.
//
// Concurrency: safe for concurrent calls after all registrations
// (custom tags, struct-level validators, etc.) are finished.
//
// Example:
//
//	if err := requestValidation.Struct(&dto); err != nil {
//	    if verrs, ok := err.(validator.ValidationErrors); ok {
//	        for _, fe := range verrs {
//	            // fe.Field(), fe.Tag(), fe.Param(), fe.ActualTag(), fe.Value(), ...
//	        }
//	    }
//	    return err
//	}
func Struct(v any) error {
	return validateInstance.Struct(v)
}

// Var validates a single value against a validation tag expression (e.g. "required,email").
//
// This is useful for ad-hoc checks outside of struct validation, or when you
// want to validate derived values.
//
// Example:
//
//	if err := requestValidation.Var(email, "required,email"); err != nil {
//	    // handle invalid email
//	}
func Var(v any, tag string) error {
	return validateInstance.Var(v, tag)
}

// RegisterValidation registers a custom field-level validation function under
// the provided tag name (e.g. "cidr", "kebab", "ulid").
//
// Call this during application startup (e.g. in init()) before the validator
// is used concurrently. Registering the same tag twice will panic.
//
// Example:
//
//	func cidr(fl validator.FieldLevel) bool {
//	    s, _ := fl.Field().Interface().(string)
//	    return isValidCIDR(s)
//	}
//
//	func init() {
//	    _ = requestValidation.RegisterValidation("cidr", cidr)
//	}
//
//	type Net struct {
//	    Block string `validate:"required,cidr"`
//	}
func RegisterValidation(tag string, fn validator.Func) error {
	return validateInstance.RegisterValidation(tag, fn)
}

// RegisterStructValidation registers a struct-level validation function for the
// given struct type. Use this for cross-field rules that cannot be expressed
// with simple field tags (e.g., "exactly one of X, Y, Z must be set").
//
// Call this during application startup (e.g. in init()) before the validator
// is used concurrently.
//
// Example:
//
//	type Creds struct {
//	    User string `validate:"omitempty"`
//	    Pass string `validate:"omitempty,min=8"`
//	    Token string `validate:"omitempty"`
//	}
//
//	func credsConsistency(sl validator.StructLevel) {
//	    c := sl.Current().Interface().(Creds)
//	    if (c.User == "" || c.Pass == "") && c.Token == "" {
//	        sl.ReportError(c.User, "User", "User", "userOrToken", "")
//	    }
//	}
//
//	func init() {
//	    requestValidation.RegisterStructValidation(credsConsistency, Creds{})
//	}
func RegisterStructValidation(fn validator.StructLevelFunc, t any) {
	validateInstance.RegisterStructValidation(fn, t)
}

// Engine exposes the underlying *validator.Validate instance for advanced
// configuration (e.g., tag name func, translations, custom type funcs).
//
// Prefer using the thin wrappers (Struct, Var, RegisterValidation, etc.)
// for common operations. Only reach for Engine() when you need features
// not covered by the helpers.
//
// Example:
//
//	v := requestValidation.Engine()
//	v.SetTagName("validate") // already the default; example only
func Engine() *validator.Validate {
	return validateInstance
}

// NewValidation creates a new Validation instance,
// storing the type information for each DTO (body, query, and params).
//
// All arguments must be pointers to structs, or nil if not needed.
//
// Example:
//
//	requestValidation.NewValidation(&LoginDTO{}, nil, &ParamsDTO{})
func NewValidation(bodyDTO, queryDTO, paramsDTO interface{}) Validation {
	return Validation{
		bodyType:   getType(bodyDTO),
		queryType:  getType(queryDTO),
		paramsType: getType(paramsDTO),
	}
}

// ValidationMiddleware returns a Fiber middleware that automatically:
//   - Parses the request body, query, and params into the DTOs.
//   - Applies default values using struct tags (e.g., `default:"value"`).
//   - Validates the DTO using go-playground/validator.
//
// If any error occurs, it returns a custom validation error.
// Make sure to initialize your error handling (e.g., customError.NewValidationError).
func ValidationMiddleware(v Validation) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if v.bodyType != nil {
			bodyDTO := reflect.New(v.bodyType).Interface()
			if errs := validateAndBind(c.BodyParser, bodyDTO); len(errs) > 0 {
				return response.BadRequest(c, prefixErrors("BODY_VALIDATION", errs))
			}
			c.Locals("bodyDTO", bodyDTO)
		}

		if v.queryType != nil {
			queryDTO := reflect.New(v.queryType).Interface()
			if errs := validateAndBind(c.QueryParser, queryDTO); len(errs) > 0 {
				return response.BadRequest(c, prefixErrors("QUERY_VALIDATION", errs))
			}
			c.Locals("queryDTO", queryDTO)
		}

		if v.paramsType != nil {
			paramsDTO := reflect.New(v.paramsType).Interface()
			if errs := validateAndBind(c.ParamsParser, paramsDTO); len(errs) > 0 {
				return response.BadRequest(c, prefixErrors("PARAMS_VALIDATION", errs))
			}
			c.Locals("paramsDTO", paramsDTO)
		}

		return c.Next()
	}
}

// GetDTO retrieves a strongly typed DTO from the Fiber context
// using the provided key. It performs a type assertion and returns
// an error if the value is missing or has an unexpected type.
//
// Example usage:
//
//	dto, err := requestValidation.GetDTO[*LoginDTO](c, "bodyDTO")
func GetDTO[T any](c *fiber.Ctx, key string) (T, error) {
	dto, ok := c.Locals(key).(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("H130: invalid or missing %s", key)
	}
	return dto, nil
}
