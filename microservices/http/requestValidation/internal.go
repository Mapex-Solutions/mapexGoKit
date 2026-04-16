package requestValidation

import (
	"fmt"
	"reflect"

	"github.com/creasty/defaults"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
	"github.com/Mapex-Solutions/mapexGoKit/utils/validations"
)

// validateInstance is a global instance of the validations engine,
// reused across all requests to improve performance and avoid reinitialization.
var validateInstance = validations.New()

// runTransformsDeep walks an arbitrarily nested DTO (structs, slices/arrays, maps)
// and executes Transform() on every node that implements DTOTransformer.
//
// Traversal order is post-order (children first, then the current node), so parent
// transforms can rely on already-normalized children.
func runTransformsDeep(v any) error {
	// Nothing to do for nil roots.
	if v == nil {
		return nil
	}

	// Start traversal with a reflective view of the input.
	return walk(reflect.ValueOf(v))
}

// walk is the internal recursive function that preserves addressability by
// passing reflect.Value down the traversal instead of converting to Interface()
// too early (which would copy values and lose addressability).
func walk(rv reflect.Value) error {
	// 1) Unwrap pointers and interfaces to reach the concrete value.
	//    If we encounter a nil pointer/interface, there's nothing to transform.
	for rv.IsValid() && (rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface) {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	// If after unwrapping the value is invalid, abort.
	if !rv.IsValid() {
		return nil
	}

	// 2) Recurse into children first (post-order traversal).
	switch rv.Kind() {
	case reflect.Struct:
		// Skip well-known library types that have unexported fields
		// and don't need transformation (e.g., time.Time).
		typeName := rv.Type().String()
		if typeName == "time.Time" {
			return nil
		}

		// Iterate over struct fields.
		rt := rv.Type()
		for i := 0; i < rv.NumField(); i++ {
			// Skip unexported fields (they can't be accessed via Interface())
			if !rt.Field(i).IsExported() {
				continue
			}
			f := rv.Field(i)
			// Important: pass the reflect.Value directly to preserve addressability.
			if err := walk(f); err != nil {
				return err
			}
		}

	case reflect.Slice, reflect.Array:
		// Visit each element.
		for i := 0; i < rv.Len(); i++ {
			if err := walk(rv.Index(i)); err != nil {
				return err
			}
		}

	case reflect.Map:
		// Iterate over map values (keys usually do not need Transform()).
		// NOTE: Map element values are NOT addressable if the map stores values (not pointers).
		//       If you need pointer-receiver transforms for elements, use map[K]*T.
		it := rv.MapRange()
		for it.Next() {
			if err := walk(it.Value()); err != nil {
				return err
			}
		}
	}

	// 3) Transform the current node itself.
	// Prefer pointer assertion first because most transforms mutate the receiver.
	// CanAddr() ensures Addr() won't panic (map element copies are not addressable).
	if rv.CanAddr() {
		if tr, ok := rv.Addr().Interface().(DTOTransformer); ok {
			return tr.Transform()
		}
	}

	// Fallback: support value-receiver Transform() (non-addressable or immutable cases).
	if tr, ok := rv.Interface().(DTOTransformer); ok {
		return tr.Transform()
	}

	return nil
}

// getType verifies that the given DTO is a pointer to a struct,
// and returns the underlying struct type.
//
// It panics if the input is not a pointer to a struct.
func getType(dto interface{}) reflect.Type {
	if dto == nil {
		return nil
	}

	t := reflect.TypeOf(dto)

	if t.Kind() != reflect.Ptr || t.Elem().Kind() != reflect.Struct {
		panic(fmt.Sprintf("DTO must be a pointer to a struct, got %v", t.Kind()))
	}

	return t.Elem()
}

// validateAndBind handles the parsing, default value population,
// and validation of a given DTO using the provided parser function.
// After successful validation, it runs deep transforms on the DTO.
//
// Returns a slice of validation or parsing error messages.
func validateAndBind(parser func(interface{}) error, dto interface{}) []string {
	// Step 1: parse incoming request (body/query/params) into the DTO instance.
	if err := parser(dto); err != nil {
		return validations.ParseTypeError(err)
	}

	// Step 2: apply defaults from `default:"..."` struct tags (creasty/defaults).
	if err := defaults.Set(dto); err != nil {
		logger.Error(err, "Failed to apply default values")
		return []string{"Internal error while setting default values"}
	}

	// Step 3: validate struct using Mapex-Solutions/MapexOS/validations package.
	if err := validations.ValidateStruct(dto); err != nil {
		return err
	}

	// Step 4: run post-validation, deep transforms (children first, then parent).
	if err := runTransformsDeep(dto); err != nil {
		logger.Error(err, "[VALIDATION] Transform step phase failed")
		return []string{err.Error()}
	}

	// All good.
	return nil
}
