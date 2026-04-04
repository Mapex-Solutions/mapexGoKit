package slice

import (
	"reflect"
	"testing"
)

func TestReverse(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{
			name:     "Odd number of elements",
			input:    []int{1, 2, 3, 4, 5},
			expected: []int{5, 4, 3, 2, 1},
		},
		{
			name:     "Even number of elements",
			input:    []int{1, 2, 3, 4},
			expected: []int{4, 3, 2, 1},
		},
		{
			name:     "Single element",
			input:    []int{1},
			expected: []int{1},
		},
		{
			name:     "Empty slice",
			input:    []int{},
			expected: []int{},
		},
		{
			name:     "Two elements",
			input:    []int{1, 2},
			expected: []int{2, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Reverse(tt.input)
			if !reflect.DeepEqual(tt.input, tt.expected) {
				t.Errorf("Reverse() = %v, want %v", tt.input, tt.expected)
			}
		})
	}
}

func TestReverseStrings(t *testing.T) {
	input := []string{"a", "b", "c", "d"}
	expected := []string{"d", "c", "b", "a"}

	Reverse(input)

	if !reflect.DeepEqual(input, expected) {
		t.Errorf("Reverse() with strings = %v, want %v", input, expected)
	}
}
