package serialize

import (
	"errors"
	"reflect"
	"testing"
)

type TestStruct struct {
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email"`
}

type InvalidStruct struct {
	Ch chan int // channels cannot be marshaled to JSON
}

func TestMarshal(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expected    string
		expectError bool
	}{
		{
			name:        "marshal simple struct",
			input:       TestStruct{Name: "John", Age: 30, Email: "john@example.com"},
			expected:    `{"name":"John","age":30,"email":"john@example.com"}`,
			expectError: false,
		},
		{
			name:        "marshal empty struct",
			input:       TestStruct{},
			expected:    `{"name":"","age":0,"email":""}`,
			expectError: false,
		},
		{
			name:        "marshal string",
			input:       "hello world",
			expected:    `"hello world"`,
			expectError: false,
		},
		{
			name:        "marshal number",
			input:       42,
			expected:    "42",
			expectError: false,
		},
		{
			name:        "marshal boolean",
			input:       true,
			expected:    "true",
			expectError: false,
		},
		{
			name:        "marshal slice",
			input:       []int{1, 2, 3},
			expected:    "[1,2,3]",
			expectError: false,
		},
		{
			name:        "marshal map",
			input:       map[string]int{"a": 1, "b": 2},
			expected:    `{"a":1,"b":2}`,
			expectError: false,
		},
		{
			name:        "marshal nil",
			input:       nil,
			expected:    "null",
			expectError: false,
		},
		{
			name:        "marshal invalid type (channel)",
			input:       InvalidStruct{Ch: make(chan int)},
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Marshal(tt.input)
			
			if tt.expectError {
				if err == nil {
					t.Error("expected error, but got none")
				}
				if !errors.Is(err, ErrMarshal) {
					t.Errorf("expected ErrMarshal, got %v", err)
				}
				if result != "" {
					t.Errorf("expected empty result on error, got %s", result)
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

func TestUnmarshal(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		target      interface{}
		expected    interface{}
		expectError bool
	}{
		{
			name:   "unmarshal to struct",
			input:  `{"name":"John","age":30,"email":"john@example.com"}`,
			target: &TestStruct{},
			expected: &TestStruct{
				Name:  "John",
				Age:   30,
				Email: "john@example.com",
			},
			expectError: false,
		},
		{
			name:        "unmarshal to string",
			input:       `"hello world"`,
			target:      new(string),
			expected:    func() *string { s := "hello world"; return &s }(),
			expectError: false,
		},
		{
			name:        "unmarshal to int",
			input:       "42",
			target:      new(int),
			expected:    func() *int { i := 42; return &i }(),
			expectError: false,
		},
		{
			name:        "unmarshal to boolean",
			input:       "true",
			target:      new(bool),
			expected:    func() *bool { b := true; return &b }(),
			expectError: false,
		},
		{
			name:        "unmarshal to slice",
			input:       "[1,2,3]",
			target:      &[]int{},
			expected:    &[]int{1, 2, 3},
			expectError: false,
		},
		{
			name:        "unmarshal to map",
			input:       `{"a":1,"b":2}`,
			target:      &map[string]int{},
			expected:    &map[string]int{"a": 1, "b": 2},
			expectError: false,
		},
		{
			name:        "unmarshal invalid json",
			input:       `{"name":"John","age":}`,
			target:      &TestStruct{},
			expected:    &TestStruct{},
			expectError: true,
		},
		{
			name:        "unmarshal empty string",
			input:       "",
			target:      &TestStruct{},
			expected:    &TestStruct{},
			expectError: true,
		},
		{
			name:        "unmarshal null",
			input:       "null",
			target:      &TestStruct{},
			expected:    &TestStruct{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Unmarshal(tt.input, tt.target)
			
			if tt.expectError {
				if err == nil {
					t.Error("expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if !reflect.DeepEqual(tt.target, tt.expected) {
					t.Errorf("expected %+v, got %+v", tt.expected, tt.target)
				}
			}
		})
	}
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		original interface{}
	}{
		{
			name:     "struct roundtrip",
			original: TestStruct{Name: "Alice", Age: 25, Email: "alice@example.com"},
		},
		{
			name:     "string roundtrip",
			original: "test string",
		},
		{
			name:     "int roundtrip",
			original: 123,
		},
		{
			name:     "slice roundtrip",
			original: []string{"a", "b", "c"},
		},
		{
			name:     "map roundtrip",
			original: map[string]interface{}{"key": "value", "number": 42.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal
			jsonStr, err := Marshal(tt.original)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}

			// Create a new instance of the same type
			resultPtr := reflect.New(reflect.TypeOf(tt.original))
			result := resultPtr.Interface()

			// Unmarshal
			err = Unmarshal(jsonStr, result)
			if err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}

			// Compare
			resultValue := resultPtr.Elem().Interface()
			if !reflect.DeepEqual(tt.original, resultValue) {
				t.Errorf("roundtrip failed: expected %+v, got %+v", tt.original, resultValue)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	t.Run("ErrMarshal exists", func(t *testing.T) {
		if ErrMarshal == nil {
			t.Error("ErrMarshal should be defined")
		}
		if ErrMarshal.Error() != "failed to marshal data" {
			t.Errorf("unexpected error message: %s", ErrMarshal.Error())
		}
	})

	t.Run("ErrUnmarshal exists", func(t *testing.T) {
		if ErrUnmarshal == nil {
			t.Error("ErrUnmarshal should be defined")
		}
		if ErrUnmarshal.Error() != "failed to unmarshal data" {
			t.Errorf("unexpected error message: %s", ErrUnmarshal.Error())
		}
	})
}

func BenchmarkMarshal(b *testing.B) {
	data := TestStruct{Name: "John", Age: 30, Email: "john@example.com"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Marshal(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	jsonStr := `{"name":"John","age":30,"email":"john@example.com"}`
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result TestStruct
		err := Unmarshal(jsonStr, &result)
		if err != nil {
			b.Fatal(err)
		}
	}
}