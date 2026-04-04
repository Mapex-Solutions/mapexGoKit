package validations

import (
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	once     sync.Once
	instance *validator.Validate
)

// New returns the singleton *validator.Validate* initialized with
// package defaults and custom rules (see internals.go).
func New() *validator.Validate {
	once.Do(func() {
		v := validator.New()

		// registers all built-in custom tags here
		registerCustoms(v)
		instance = v
	})

	return instance
}
