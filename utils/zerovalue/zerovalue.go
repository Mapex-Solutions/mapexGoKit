// Package zerovalue provides utility functions for getting zero/default values
// based on type names used in dynamic schemas (e.g., LocalState, form fields).
package zerovalue

// FieldType represents the supported field types for zero value generation.
// These types are commonly used in dynamic schemas like Rule.LocalState.
type FieldType string

const (
	// TypeNumber represents a numeric field (returns 0)
	TypeNumber FieldType = "number"

	// TypeString represents a text field (returns "")
	TypeString FieldType = "string"

	// TypeBoolean represents a boolean field (returns false)
	TypeBoolean FieldType = "boolean"

	// TypeArray represents an array/list field (returns empty slice)
	TypeArray FieldType = "array"

	// TypeObject represents an object/map field (returns empty map)
	TypeObject FieldType = "object"
)

// GetZeroValue returns the zero/default value for a given field type.
// This is useful for initializing dynamic state fields with appropriate defaults
// when no explicit default value is provided.
//
// Supported types:
//   - "number"  → 0 (int)
//   - "string"  → "" (empty string)
//   - "boolean" → false
//   - "array"   → []interface{}{} (empty slice)
//   - "object"  → map[string]interface{}{} (empty map)
//   - unknown   → nil
//
// Example:
//
//	val := zerovalue.GetZeroValue("number")   // returns 0
//	val := zerovalue.GetZeroValue("string")  // returns ""
//	val := zerovalue.GetZeroValue("boolean") // returns false
//	val := zerovalue.GetZeroValue("array")   // returns []interface{}{}
//	val := zerovalue.GetZeroValue("object")  // returns map[string]interface{}{}
func GetZeroValue(fieldType string) interface{} {
	switch FieldType(fieldType) {
	case TypeNumber:
		return 0
	case TypeString:
		return ""
	case TypeBoolean:
		return false
	case TypeArray:
		return []interface{}{}
	case TypeObject:
		return map[string]interface{}{}
	default:
		return nil
	}
}

// GetZeroValueTyped is a type-safe version that accepts FieldType directly.
//
// Example:
//
//	val := zerovalue.GetZeroValueTyped(zerovalue.TypeNumber) // returns 0
func GetZeroValueTyped(fieldType FieldType) interface{} {
	return GetZeroValue(string(fieldType))
}

// IsValidType checks if the given type string is a valid FieldType.
//
// Example:
//
//	zerovalue.IsValidType("number")  // returns true
//	zerovalue.IsValidType("invalid") // returns false
func IsValidType(fieldType string) bool {
	switch FieldType(fieldType) {
	case TypeNumber, TypeString, TypeBoolean, TypeArray, TypeObject:
		return true
	default:
		return false
	}
}
