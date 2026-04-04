package customErrors

import (
	"fmt"
	"strings"
)

// ValidationError represents a validation error with multiple messages.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return strings.Join(e.Errors, "; ")
}

// ServerCustomError represents a custom server error with a specific HTTP code.
type ServerCustomError struct {
	Code   int
	Errors []string
}

func (e *ServerCustomError) Error() string {
	if e == nil {
		return "<nil>"
	}
	// Ex.: "code=422: field X is required; field Y must be an email"
	return fmt.Sprintf("code=%d: %s", e.Code, strings.Join(e.Errors, "; "))
}
