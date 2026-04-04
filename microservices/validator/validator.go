/**
 * Package validator provides generic validation utilities for DTOs.
 * It can be used with any data source (NATS messages, files, etc.),
 * not just HTTP requests.
 *
 * This package mirrors the HTTP requestValidation pattern but is
 * decoupled from HTTP-specific dependencies.
 */
package validator

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/creasty/defaults"
	"github.com/Mapex-Solutions/mapexGoKit/utils/validations"
)

/**
 * DTOTransformer is an interface that DTOs can implement to perform
 * custom transformations after validation.
 */
type DTOTransformer interface {
	Transform() error
}

/**
 * UnmarshalAndValidate unmarshals JSON bytes into a DTO, applies defaults,
 * and validates the struct.
 *
 * Flow:
 *   1. Unmarshal JSON bytes into DTO
 *   2. Apply defaults from `default:"..."` struct tags
 *   3. Validate struct using validation tags
 *   4. Run Transform() if DTO implements DTOTransformer
 *
 * Parameters:
 *   - data: JSON bytes to unmarshal
 *   - dto: Pointer to the DTO struct to populate
 *
 * Returns:
 *   - nil: Validation successful
 *   - error: Parsing or validation failed
 */
func UnmarshalAndValidate(data []byte, dto interface{}) error {
	if err := validateDTOType(dto); err != nil {
		return err
	}

	// Step 1: Unmarshal JSON into DTO
	if err := json.Unmarshal(data, dto); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Step 2: Apply defaults
	if err := defaults.Set(dto); err != nil {
		return fmt.Errorf("failed to apply defaults: %w", err)
	}

	// Step 3: Validate struct
	if errs := validations.ValidateStruct(dto); errs != nil {
		return fmt.Errorf("validation failed: %v", errs)
	}

	// Step 4: Run transforms if DTO implements DTOTransformer
	if err := runTransformsDeep(dto); err != nil {
		return fmt.Errorf("transform failed: %w", err)
	}

	return nil
}

/**
 * Validate validates an already populated DTO, applying defaults and
 * running validations. Use this when you've already unmarshaled the data.
 *
 * Parameters:
 *   - dto: Pointer to the DTO struct to validate
 *
 * Returns:
 *   - nil: Validation successful
 *   - error: Validation failed
 */
func Validate(dto interface{}) error {
	if err := validateDTOType(dto); err != nil {
		return err
	}

	// Step 1: Apply defaults
	if err := defaults.Set(dto); err != nil {
		return fmt.Errorf("failed to apply defaults: %w", err)
	}

	// Step 2: Validate struct
	if errs := validations.ValidateStruct(dto); errs != nil {
		return fmt.Errorf("validation failed: %v", errs)
	}

	// Step 3: Run transforms if DTO implements DTOTransformer
	if err := runTransformsDeep(dto); err != nil {
		return fmt.Errorf("transform failed: %w", err)
	}

	return nil
}

// validateDTOType ensures the DTO is a pointer to a struct.
func validateDTOType(dto interface{}) error {
	if dto == nil {
		return fmt.Errorf("dto cannot be nil")
	}

	t := reflect.TypeOf(dto)
	if t.Kind() != reflect.Ptr || t.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("dto must be a pointer to a struct, got %v", t.Kind())
	}

	return nil
}

// runTransformsDeep walks an arbitrarily nested DTO and executes Transform()
// on every node that implements DTOTransformer.
func runTransformsDeep(v interface{}) error {
	if v == nil {
		return nil
	}
	return walk(reflect.ValueOf(v))
}

// walk is the internal recursive function for transform traversal.
func walk(rv reflect.Value) error {
	// Unwrap pointers and interfaces
	for rv.IsValid() && (rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface) {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}

	if !rv.IsValid() {
		return nil
	}

	// Skip unexported fields - they cannot be accessed via reflection
	if !rv.CanInterface() {
		return nil
	}

	// Recurse into children first (post-order traversal)
	switch rv.Kind() {
	case reflect.Struct:
		rt := rv.Type()
		for i := 0; i < rv.NumField(); i++ {
			// Skip unexported fields (fields starting with lowercase)
			if !rt.Field(i).IsExported() {
				continue
			}
			if err := walk(rv.Field(i)); err != nil {
				return err
			}
		}

	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			if err := walk(rv.Index(i)); err != nil {
				return err
			}
		}

	case reflect.Map:
		it := rv.MapRange()
		for it.Next() {
			if err := walk(it.Value()); err != nil {
				return err
			}
		}
	}

	// Transform the current node (only if we can get the interface)
	if rv.CanAddr() && rv.Addr().CanInterface() {
		if tr, ok := rv.Addr().Interface().(DTOTransformer); ok {
			return tr.Transform()
		}
	}

	if rv.CanInterface() {
		if tr, ok := rv.Interface().(DTOTransformer); ok {
			return tr.Transform()
		}
	}

	return nil
}
