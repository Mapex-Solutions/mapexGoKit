package flatten

import (
	"reflect"
	"testing"
)

func TestFlatten(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		prefix   string
		style    SeparatorStyle
		expected map[string]any
	}{
		{
			name: "simple nested map with dot style",
			input: map[string]any{
				"name": "John",
				"age":  30,
				"address": map[string]any{
					"street": "123 Main St",
					"city":   "New York",
				},
			},
			prefix: "",
			style:  DotStyle,
			expected: map[string]any{
				"name":           "John",
				"age":            30,
				"address.street": "123 Main St",
				"address.city":   "New York",
			},
		},
		{
			name: "nested map with prefix and dot style",
			input: map[string]any{
				"user": map[string]any{
					"profile": map[string]any{
						"name": "Alice",
						"age":  25,
					},
				},
			},
			prefix: "data.",
			style:  DotStyle,
			expected: map[string]any{
				"data.user.profile.name": "Alice",
				"data.user.profile.age":  25,
			},
		},
		{
			name: "array with dot style",
			input: map[string]any{
				"items": []any{"apple", "banana", "cherry"},
			},
			prefix: "",
			style:  DotStyle,
			expected: map[string]any{
				"items.0": "apple",
				"items.1": "banana",
				"items.2": "cherry",
			},
		},
		{
			name: "nested array with objects",
			input: map[string]any{
				"users": []any{
					map[string]any{"name": "John", "age": 30},
					map[string]any{"name": "Jane", "age": 25},
				},
			},
			prefix: "",
			style:  DotStyle,
			expected: map[string]any{
				"users.0.name": "John",
				"users.0.age":  30,
				"users.1.name": "Jane",
				"users.1.age":  25,
			},
		},
		{
			name: "rails style",
			input: map[string]any{
				"user": map[string]any{
					"profile": map[string]any{
						"name": "Bob",
					},
				},
			},
			prefix: "",
			style:  RailsStyle,
			expected: map[string]any{
				"user[profile][name]": "Bob",
			},
		},
		{
			name: "underscore style",
			input: map[string]any{
				"user": map[string]any{
					"first_name": "Charlie",
					"last_name":  "Brown",
				},
			},
			prefix: "",
			style:  UnderscoreStyle,
			expected: map[string]any{
				"user_first_name": "Charlie",
				"user_last_name":  "Brown",
			},
		},
		{
			name: "path style",
			input: map[string]any{
				"config": map[string]any{
					"database": map[string]any{
						"host": "localhost",
						"port": 5432,
					},
				},
			},
			prefix: "",
			style:  PathStyle,
			expected: map[string]any{
				"config/database/host": "localhost",
				"config/database/port": 5432,
			},
		},
		{
			name: "empty nested map",
			input: map[string]any{
				"empty": map[string]any{},
				"value": "test",
			},
			prefix: "",
			style:  DotStyle,
			expected: map[string]any{
				"value": "test",
			},
		},
		{
			name: "null and zero values",
			input: map[string]any{
				"null":   nil,
				"zero":   0,
				"empty":  "",
				"false":  false,
				"nested": map[string]any{"null": nil},
			},
			prefix: "",
			style:  DotStyle,
			expected: map[string]any{
				"null":        nil,
				"zero":        0,
				"empty":       "",
				"false":       false,
				"nested.null": nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Flatten(tt.input, tt.prefix, tt.style)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFlattenString(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		prefix      string
		style       SeparatorStyle
		expected    string
		expectError bool
	}{
		{
			name:     "simple JSON object",
			input:    `{"name":"John","age":30,"address":{"street":"123 Main St","city":"New York"}}`,
			prefix:   "",
			style:    DotStyle,
			expected: `{"address.city":"New York","address.street":"123 Main St","age":30,"name":"John"}`,
		},
		{
			name:     "JSON with array",
			input:    `{"items":["apple","banana","cherry"]}`,
			prefix:   "",
			style:    DotStyle,
			expected: `{"items.0":"apple","items.1":"banana","items.2":"cherry"}`,
		},
		{
			name:     "JSON with prefix",
			input:    `{"user":{"name":"Alice"}}`,
			prefix:   "data.",
			style:    DotStyle,
			expected: `{"data.user.name":"Alice"}`,
		},
		{
			name:        "invalid JSON",
			input:       `{"name":"John","age":}`,
			prefix:      "",
			style:       DotStyle,
			expected:    "",
			expectError: true,
		},
		{
			name:        "JSON array (not object)",
			input:       `["item1","item2"]`,
			prefix:      "",
			style:       DotStyle,
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty string",
			input:       "",
			prefix:      "",
			style:       DotStyle,
			expected:    "",
			expectError: true,
		},
		{
			name:     "JSON with whitespace",
			input:    `  {"name":"John"}  `,
			prefix:   "",
			style:    DotStyle,
			expected: `{"name":"John"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FlattenString(tt.input, tt.prefix, tt.style)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestLooksLikeJSONObject(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid JSON object",
			input:    `{"key":"value"}`,
			expected: true,
		},
		{
			name:     "JSON object with leading whitespace",
			input:    `  {"key":"value"}`,
			expected: true,
		},
		{
			name:     "JSON object with various whitespace",
			input:    " \n\r\t{\"key\":\"value\"}",
			expected: true,
		},
		{
			name:     "JSON array",
			input:    `["item1","item2"]`,
			expected: false,
		},
		{
			name:     "JSON string",
			input:    `"string value"`,
			expected: false,
		},
		{
			name:     "JSON number",
			input:    `123`,
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "only whitespace",
			input:    "   \n\r\t  ",
			expected: false,
		},
		{
			name:     "invalid JSON starting with {",
			input:    `{invalid`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := looksLikeJSONObject(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSeparatorStyles(t *testing.T) {
	input := map[string]any{
		"level1": map[string]any{
			"level2": "value",
		},
	}

	tests := []struct {
		name     string
		style    SeparatorStyle
		expected string
	}{
		{
			name:     "DotStyle",
			style:    DotStyle,
			expected: "level1.level2",
		},
		{
			name:     "PathStyle",
			style:    PathStyle,
			expected: "level1/level2",
		},
		{
			name:     "RailsStyle",
			style:    RailsStyle,
			expected: "level1[level2]",
		},
		{
			name:     "UnderscoreStyle",
			style:    UnderscoreStyle,
			expected: "level1_level2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Flatten(input, "", tt.style)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result[tt.expected] != "value" {
				t.Errorf("expected key %s with value 'value', got %v", tt.expected, result)
			}
		})
	}
}

func TestFlattenErrors(t *testing.T) {
	t.Run("ErrNotValidInput defined", func(t *testing.T) {
		if ErrNotValidInput == nil {
			t.Error("ErrNotValidInput should be defined")
		}
	})

	t.Run("ErrNotValidJSONInput defined", func(t *testing.T) {
		if ErrNotValidJSONInput == nil {
			t.Error("ErrNotValidJSONInput should be defined")
		}
	})
}

func TestGetValueByPath(t *testing.T) {
	tests := []struct {
		name          string
		data          map[string]any
		path          string
		expectedValue any
		expectedMulti bool
		expectedErr   error
	}{
		// Single value cases
		{
			name:          "simple top-level key",
			data:          map[string]any{"name": "John"},
			path:          "name",
			expectedValue: "John",
			expectedMulti: false,
		},
		{
			name: "nested path",
			data: map[string]any{
				"address": map[string]any{
					"city": "New York",
				},
			},
			path:          "address.city",
			expectedValue: "New York",
			expectedMulti: false,
		},
		{
			name: "deep nested path",
			data: map[string]any{
				"data": map[string]any{
					"result": map[string]any{
						"values": map[string]any{
							"temp": 25.5,
						},
					},
				},
			},
			path:          "data.result.values.temp",
			expectedValue: 25.5,
			expectedMulti: false,
		},
		{
			name: "numeric value",
			data: map[string]any{
				"sensor": map[string]any{"value": float64(42)},
			},
			path:          "sensor.value",
			expectedValue: float64(42),
			expectedMulti: false,
		},
		{
			name: "boolean value",
			data: map[string]any{
				"config": map[string]any{"enabled": true},
			},
			path:          "config.enabled",
			expectedValue: true,
			expectedMulti: false,
		},
		{
			name: "nil value at path",
			data: map[string]any{
				"data": map[string]any{"value": nil},
			},
			path:          "data.value",
			expectedValue: nil,
			expectedMulti: false,
		},

		// Multi value cases (arrays)
		{
			name: "path ending on array of scalars",
			data: map[string]any{
				"values": []any{1, 2, 3},
			},
			path:          "values",
			expectedValue: []any{1, 2, 3},
			expectedMulti: true,
		},
		{
			name: "path through array of objects",
			data: map[string]any{
				"sensors": []any{
					map[string]any{"temp": 25.5},
					map[string]any{"temp": 30.2},
				},
			},
			path:          "sensors.temp",
			expectedValue: []any{25.5, 30.2},
			expectedMulti: true,
		},
		{
			name: "deep path through array",
			data: map[string]any{
				"data": map[string]any{
					"sensors": []any{
						map[string]any{"reading": map[string]any{"temp": 25.5}},
						map[string]any{"reading": map[string]any{"temp": 30.2}},
					},
				},
			},
			path:          "data.sensors.reading.temp",
			expectedValue: []any{25.5, 30.2},
			expectedMulti: true,
		},
		{
			name: "nested arrays (fan out across two levels)",
			data: map[string]any{
				"groups": []any{
					map[string]any{
						"sensors": []any{
							map[string]any{"temp": 25.5},
							map[string]any{"temp": 30.2},
						},
					},
					map[string]any{
						"sensors": []any{
							map[string]any{"temp": 18.0},
						},
					},
				},
			},
			path:          "groups.sensors.temp",
			expectedValue: []any{25.5, 30.2, 18.0},
			expectedMulti: true,
		},
		{
			name: "array where some elements lack the key",
			data: map[string]any{
				"items": []any{
					map[string]any{"temp": 10.0},
					map[string]any{"humidity": 60.0},
					map[string]any{"temp": 20.0},
				},
			},
			path:          "items.temp",
			expectedValue: []any{10.0, 20.0},
			expectedMulti: true,
		},
		{
			name: "nested path ending on array",
			data: map[string]any{
				"data": map[string]any{
					"tags": []any{"a", "b", "c"},
				},
			},
			path:          "data.tags",
			expectedValue: []any{"a", "b", "c"},
			expectedMulti: true,
		},

		// Error cases
		{
			name:        "nil data",
			data:        nil,
			path:        "name",
			expectedErr: ErrNilData,
		},
		{
			name:        "empty path",
			data:        map[string]any{"name": "John"},
			path:        "",
			expectedErr: ErrEmptyPath,
		},
		{
			name:        "path not found",
			data:        map[string]any{"name": "John"},
			path:        "age",
			expectedErr: ErrPathNotFound,
		},
		{
			name: "partial path not found",
			data: map[string]any{
				"data": map[string]any{"name": "John"},
			},
			path:        "data.address.city",
			expectedErr: ErrPathNotFound,
		},
		{
			name: "path through scalar (cannot traverse)",
			data: map[string]any{
				"name": "John",
			},
			path:        "name.first",
			expectedErr: ErrPathNotFound,
		},
		{
			name: "array where no elements have the key",
			data: map[string]any{
				"items": []any{
					map[string]any{"a": 1},
					map[string]any{"b": 2},
				},
			},
			path:        "items.c",
			expectedErr: ErrPathNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, multi, err := GetValueByPath(tt.data, tt.path)

			if tt.expectedErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.expectedErr)
				}
				if err != tt.expectedErr {
					t.Errorf("expected error %v, got %v", tt.expectedErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if multi != tt.expectedMulti {
				t.Errorf("expected multi=%v, got multi=%v", tt.expectedMulti, multi)
			}

			if !reflect.DeepEqual(value, tt.expectedValue) {
				t.Errorf("expected value %v (%T), got %v (%T)", tt.expectedValue, tt.expectedValue, value, value)
			}
		})
	}
}

func BenchmarkGetValueByPath(b *testing.B) {
	data := map[string]any{
		"data": map[string]any{
			"sensors": []any{
				map[string]any{"temp": 25.5, "humidity": 60.0},
				map[string]any{"temp": 30.2, "humidity": 45.0},
				map[string]any{"temp": 18.0, "humidity": 80.0},
			},
		},
	}

	b.Run("simple path", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			GetValueByPath(data, "data.sensors")
		}
	})

	b.Run("path through array", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			GetValueByPath(data, "data.sensors.temp")
		}
	})
}

func BenchmarkFlatten(b *testing.B) {
	input := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"level3": map[string]any{
					"value": "test",
					"array": []any{1, 2, 3, 4, 5},
				},
			},
		},
		"simple": "value",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Flatten(input, "", DotStyle)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFlattenString(b *testing.B) {
	input := `{"level1":{"level2":{"level3":{"value":"test","array":[1,2,3,4,5]}}},"simple":"value"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := FlattenString(input, "", DotStyle)
		if err != nil {
			b.Fatal(err)
		}
	}
}