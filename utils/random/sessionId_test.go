package random

import (
	"fmt"
	"regexp"
	"testing"
)

func TestGenerateSessionID(t *testing.T) {
	tests := []struct {
		name           string
		length         int
		expectedLength int
		shouldError    bool
	}{
		{
			name:           "length 4 generates 8-character hex string",
			length:         4,
			expectedLength: 8,
			shouldError:    false,
		},
		{
			name:           "length 16 generates 32-character hex string",
			length:         16,
			expectedLength: 32,
			shouldError:    false,
		},
		{
			name:           "length 1 generates 2-character hex string",
			length:         1,
			expectedLength: 2,
			shouldError:    false,
		},
		{
			name:           "length 32 generates 64-character hex string",
			length:         32,
			expectedLength: 64,
			shouldError:    false,
		},
		{
			name:           "zero length generates empty string",
			length:         0,
			expectedLength: 0,
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GenerateSessionID(tt.length)
			
			if tt.shouldError {
				if err == nil {
					t.Error("expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result) != tt.expectedLength {
				t.Errorf("expected length %d, got %d", tt.expectedLength, len(result))
			}

			// Verify the result contains only valid hexadecimal characters
			if tt.expectedLength > 0 {
				hexPattern := regexp.MustCompile(`^[a-f0-9]+$`)
				if !hexPattern.MatchString(result) {
					t.Errorf("result contains non-hexadecimal characters: %s", result)
				}
			}
		})
	}
}

func TestGenerateSessionID_Uniqueness(t *testing.T) {
	const iterations = 1000
	const length = 16
	
	generated := make(map[string]bool)
	
	for i := 0; i < iterations; i++ {
		sessionID, err := GenerateSessionID(length)
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		
		if generated[sessionID] {
			t.Errorf("duplicate session ID generated: %s", sessionID)
		}
		
		generated[sessionID] = true
	}
	
	if len(generated) != iterations {
		t.Errorf("expected %d unique session IDs, got %d", iterations, len(generated))
	}
}

func TestGenerateSessionID_Properties(t *testing.T) {
	sessionID, err := GenerateSessionID(16)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	// Should be lowercase
	for _, char := range sessionID {
		if char >= 'A' && char <= 'F' {
			t.Errorf("session ID contains uppercase characters: %s", sessionID)
		}
	}
	
	// Should be exactly 32 characters for length 16
	if len(sessionID) != 32 {
		t.Errorf("expected 32 characters, got %d", len(sessionID))
	}
	
	// Should contain only valid hex characters
	hexPattern := regexp.MustCompile(`^[a-f0-9]+$`)
	if !hexPattern.MatchString(sessionID) {
		t.Errorf("session ID contains invalid characters: %s", sessionID)
	}
}

func BenchmarkGenerateSessionID(b *testing.B) {
	lengths := []int{4, 8, 16, 32}
	
	for _, length := range lengths {
		b.Run(fmt.Sprintf("length_%d", length), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := GenerateSessionID(length)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}