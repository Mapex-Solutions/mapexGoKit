package timeUtil

import (
	"encoding/json"
	"testing"
	"time"
)

func TestToRFC3339Milli(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected *string
	}{
		{
			name:     "zero time returns nil",
			input:    time.Time{},
			expected: nil,
		},
		{
			name:  "valid time returns formatted string",
			input: time.Date(2023, 12, 25, 15, 30, 45, 123456789, time.UTC),
			expected: func() *string {
				s := "2023-12-25T15:30:45.123Z"
				return &s
			}(),
		},
		{
			name:  "non-UTC time is converted to UTC",
			input: time.Date(2023, 6, 15, 10, 0, 0, 0, time.FixedZone("EST", -5*3600)),
			expected: func() *string {
				s := "2023-06-15T15:00:00.000Z"
				return &s
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToRFC3339Milli(tt.input)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", *result)
				}
			} else {
				if result == nil {
					t.Errorf("expected %v, got nil", *tt.expected)
				} else if *result != *tt.expected {
					t.Errorf("expected %v, got %v", *tt.expected, *result)
				}
			}
		})
	}
}

func TestNullTime_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    NullTime
		expected string
	}{
		{
			name:     "zero time marshals to null",
			input:    NullTime{time.Time{}},
			expected: "null",
		},
		{
			name:     "valid time marshals to RFC3339 with milliseconds",
			input:    NullTime{time.Date(2023, 12, 25, 15, 30, 45, 123456789, time.UTC)},
			expected: `"2023-12-25T15:30:45.123Z"`,
		},
		{
			name:     "non-UTC time is converted to UTC in JSON",
			input:    NullTime{time.Date(2023, 6, 15, 10, 0, 0, 0, time.FixedZone("EST", -5*3600))},
			expected: `"2023-06-15T15:00:00.000Z"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.input.MarshalJSON()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(result) != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, string(result))
			}
		})
	}
}

func TestNullTime_Integration(t *testing.T) {
	type TestStruct struct {
		CreatedAt NullTime `json:"created_at"`
		UpdatedAt NullTime `json:"updated_at"`
	}

	testTime := time.Date(2023, 12, 25, 15, 30, 45, 123456789, time.UTC)

	tests := []struct {
		name     string
		input    TestStruct
		expected string
	}{
		{
			name: "both times are zero",
			input: TestStruct{
				CreatedAt: NullTime{time.Time{}},
				UpdatedAt: NullTime{time.Time{}},
			},
			expected: `{"created_at":null,"updated_at":null}`,
		},
		{
			name: "one time is zero, one is valid",
			input: TestStruct{
				CreatedAt: NullTime{testTime},
				UpdatedAt: NullTime{time.Time{}},
			},
			expected: `{"created_at":"2023-12-25T15:30:45.123Z","updated_at":null}`,
		},
		{
			name: "both times are valid",
			input: TestStruct{
				CreatedAt: NullTime{testTime},
				UpdatedAt: NullTime{testTime.Add(time.Hour)},
			},
			expected: `{"created_at":"2023-12-25T15:30:45.123Z","updated_at":"2023-12-25T16:30:45.123Z"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(result) != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, string(result))
			}
		})
	}
}