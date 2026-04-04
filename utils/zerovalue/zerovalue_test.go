package zerovalue

import (
	"reflect"
	"testing"
)

func TestGetZeroValue(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
		expected  interface{}
	}{
		{
			name:      "number returns 0",
			fieldType: "number",
			expected:  0,
		},
		{
			name:      "string returns empty string",
			fieldType: "string",
			expected:  "",
		},
		{
			name:      "boolean returns false",
			fieldType: "boolean",
			expected:  false,
		},
		{
			name:      "array returns empty slice",
			fieldType: "array",
			expected:  []interface{}{},
		},
		{
			name:      "object returns empty map",
			fieldType: "object",
			expected:  map[string]interface{}{},
		},
		{
			name:      "unknown type returns nil",
			fieldType: "unknown",
			expected:  nil,
		},
		{
			name:      "empty string returns nil",
			fieldType: "",
			expected:  nil,
		},
		{
			name:      "invalid type returns nil",
			fieldType: "invalid_type",
			expected:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetZeroValue(tt.fieldType)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("GetZeroValue(%q) = %v (%T), want %v (%T)",
					tt.fieldType, result, result, tt.expected, tt.expected)
			}
		})
	}
}

func TestGetZeroValueTyped(t *testing.T) {
	tests := []struct {
		name      string
		fieldType FieldType
		expected  interface{}
	}{
		{
			name:      "TypeNumber returns 0",
			fieldType: TypeNumber,
			expected:  0,
		},
		{
			name:      "TypeString returns empty string",
			fieldType: TypeString,
			expected:  "",
		},
		{
			name:      "TypeBoolean returns false",
			fieldType: TypeBoolean,
			expected:  false,
		},
		{
			name:      "TypeArray returns empty slice",
			fieldType: TypeArray,
			expected:  []interface{}{},
		},
		{
			name:      "TypeObject returns empty map",
			fieldType: TypeObject,
			expected:  map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetZeroValueTyped(tt.fieldType)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("GetZeroValueTyped(%q) = %v (%T), want %v (%T)",
					tt.fieldType, result, result, tt.expected, tt.expected)
			}
		})
	}
}

func TestIsValidType(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
		expected  bool
	}{
		{
			name:      "number is valid",
			fieldType: "number",
			expected:  true,
		},
		{
			name:      "string is valid",
			fieldType: "string",
			expected:  true,
		},
		{
			name:      "boolean is valid",
			fieldType: "boolean",
			expected:  true,
		},
		{
			name:      "array is valid",
			fieldType: "array",
			expected:  true,
		},
		{
			name:      "object is valid",
			fieldType: "object",
			expected:  true,
		},
		{
			name:      "unknown is invalid",
			fieldType: "unknown",
			expected:  false,
		},
		{
			name:      "empty string is invalid",
			fieldType: "",
			expected:  false,
		},
		{
			name:      "int is invalid (use number)",
			fieldType: "int",
			expected:  false,
		},
		{
			name:      "float is invalid (use number)",
			fieldType: "float",
			expected:  false,
		},
		{
			name:      "bool is invalid (use boolean)",
			fieldType: "bool",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidType(tt.fieldType)
			if result != tt.expected {
				t.Errorf("IsValidType(%q) = %v, want %v",
					tt.fieldType, result, tt.expected)
			}
		})
	}
}

func TestGetZeroValue_ArrayIsModifiable(t *testing.T) {
	// Ensure the returned array is a new instance each time
	arr1 := GetZeroValue("array").([]interface{})
	arr2 := GetZeroValue("array").([]interface{})

	arr1 = append(arr1, "test")

	if len(arr2) != 0 {
		t.Error("GetZeroValue(\"array\") should return a new slice each time")
	}
}

func TestGetZeroValue_ObjectIsModifiable(t *testing.T) {
	// Ensure the returned map is a new instance each time
	obj1 := GetZeroValue("object").(map[string]interface{})
	obj2 := GetZeroValue("object").(map[string]interface{})

	obj1["key"] = "value"

	if len(obj2) != 0 {
		t.Error("GetZeroValue(\"object\") should return a new map each time")
	}
}

func TestFieldTypeConstants(t *testing.T) {
	// Verify constants have expected string values
	tests := []struct {
		constant FieldType
		expected string
	}{
		{TypeNumber, "number"},
		{TypeString, "string"},
		{TypeBoolean, "boolean"},
		{TypeArray, "array"},
		{TypeObject, "object"},
	}

	for _, tt := range tests {
		if string(tt.constant) != tt.expected {
			t.Errorf("FieldType constant %v = %q, want %q",
				tt.constant, string(tt.constant), tt.expected)
		}
	}
}

// Benchmark tests
func BenchmarkGetZeroValue(b *testing.B) {
	types := []string{"number", "string", "boolean", "array", "object", "unknown"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, t := range types {
			GetZeroValue(t)
		}
	}
}

func BenchmarkIsValidType(b *testing.B) {
	types := []string{"number", "string", "boolean", "array", "object", "unknown"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, t := range types {
			IsValidType(t)
		}
	}
}
