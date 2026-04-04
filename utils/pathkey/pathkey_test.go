package pathkey

import (
	"testing"
)

func TestCalculateNextSiblingPathKey(t *testing.T) {
	tests := []struct {
		name     string
		pathKey  string
		expected string
	}{
		{
			name:     "Increment customer level (6 chars)",
			pathKey:  "000001",
			expected: "000002",
		},
		{
			name:     "Increment site level (4 chars)",
			pathKey:  "000001/0001",
			expected: "000001/0002",
		},
		{
			name:     "Increment floor level (3 chars)",
			pathKey:  "000001/000001/001",
			expected: "000001/000001/002",
		},
		{
			name:     "Increment with Base36 rollover (9 → A)",
			pathKey:  "000001/000009",
			expected: "000001/00000A",
		},
		{
			name:     "Increment Z to 10",
			pathKey:  "000001/00000Z",
			expected: "000001/000010",
		},
		{
			name:     "Complex path",
			pathKey:  "000001/000001/0001/0001",
			expected: "000001/000001/0001/0002",
		},
		{
			name:     "Empty string",
			pathKey:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateNextSiblingPathKey(tt.pathKey)
			if result != tt.expected {
				t.Errorf("CalculateNextSiblingPathKey(%q) = %q, want %q", tt.pathKey, result, tt.expected)
			}
		})
	}
}

func TestIsDescendant(t *testing.T) {
	tests := []struct {
		name          string
		childPathKey  string
		parentPathKey string
		expected      bool
	}{
		{
			name:          "Direct child",
			childPathKey:  "000001/000001",
			parentPathKey: "000001",
			expected:      true,
		},
		{
			name:          "Grandchild",
			childPathKey:  "000001/000001/0001",
			parentPathKey: "000001",
			expected:      true,
		},
		{
			name:          "Not descendant - different branch",
			childPathKey:  "000001/000002/0001",
			parentPathKey: "000001/000001",
			expected:      false,
		},
		{
			name:          "Same pathKey",
			childPathKey:  "000001/000001",
			parentPathKey: "000001/000001",
			expected:      false,
		},
		{
			name:          "Empty parent (root)",
			childPathKey:  "000001/000001/0001",
			parentPathKey: "",
			expected:      true,
		},
		{
			name:          "Parent longer than child",
			childPathKey:  "000001",
			parentPathKey: "000001/000001",
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDescendant(tt.childPathKey, tt.parentPathKey)
			if result != tt.expected {
				t.Errorf("IsDescendant(%q, %q) = %v, want %v", tt.childPathKey, tt.parentPathKey, result, tt.expected)
			}
		})
	}
}

func TestIsDescendantOrSelf(t *testing.T) {
	tests := []struct {
		name          string
		childPathKey  string
		parentPathKey string
		expected      bool
	}{
		{
			name:          "Same pathKey",
			childPathKey:  "000001/000001",
			parentPathKey: "000001/000001",
			expected:      true,
		},
		{
			name:          "Direct child",
			childPathKey:  "000001/000001",
			parentPathKey: "000001",
			expected:      true,
		},
		{
			name:          "Not descendant",
			childPathKey:  "000001/000002",
			parentPathKey: "000001/000001",
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDescendantOrSelf(tt.childPathKey, tt.parentPathKey)
			if result != tt.expected {
				t.Errorf("IsDescendantOrSelf(%q, %q) = %v, want %v", tt.childPathKey, tt.parentPathKey, result, tt.expected)
			}
		})
	}
}

func TestGetAncestorPaths(t *testing.T) {
	tests := []struct {
		name     string
		pathKey  string
		expected []string
	}{
		{
			name:     "Root level - no ancestors",
			pathKey:  "000001",
			expected: []string{},
		},
		{
			name:     "Second level - one ancestor",
			pathKey:  "000001/000002",
			expected: []string{"000001"},
		},
		{
			name:     "Third level - two ancestors",
			pathKey:  "000001/000002/0003",
			expected: []string{"000001", "000001/000002"},
		},
		{
			name:     "Fourth level - three ancestors",
			pathKey:  "000001/000002/0003/0004",
			expected: []string{"000001", "000001/000002", "000001/000002/0003"},
		},
		{
			name:     "Empty pathKey",
			pathKey:  "",
			expected: []string{},
		},
		{
			name:     "Real example - Building",
			pathKey:  "000002/000003/0001/0001",
			expected: []string{"000002", "000002/000003", "000002/000003/0001"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetAncestorPaths(tt.pathKey)

			// Check length
			if len(result) != len(tt.expected) {
				t.Errorf("GetAncestorPaths(%q) returned %d ancestors, want %d", tt.pathKey, len(result), len(tt.expected))
				return
			}

			// Check each ancestor
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("GetAncestorPaths(%q)[%d] = %q, want %q", tt.pathKey, i, result[i], expected)
				}
			}
		})
	}
}
