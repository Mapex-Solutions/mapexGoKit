package typeconv

import (
	"testing"
	"time"
)

// ============================================================
// ToFloat64 Tests
// ============================================================

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected float64
		wantErr  bool
	}{
		{"float64", 3.14, 3.14, false},
		{"float32", float32(3.0), 3.0, false}, // Use whole number to avoid precision issues
		{"int", 42, 42.0, false},
		{"int64", int64(100), 100.0, false},
		{"uint", uint(50), 50.0, false},
		{"string number", "42", 42.0, false},
		{"string float", "3.14", 3.14, false},
		{"bool true", true, 1.0, false},
		{"bool false", false, 0.0, false},
		{"nil", nil, 0, true},
		{"invalid string", "hello", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ToFloat64(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToFloat64(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("ToFloat64(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTryFloat64(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected float64
		ok       bool
	}{
		{"valid int", 42, 42.0, true},
		{"valid string", "3.14", 3.14, true},
		{"invalid", "hello", 0, false},
		{"nil", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := TryFloat64(tt.input)
			if ok != tt.ok {
				t.Errorf("TryFloat64(%v) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if ok && result != tt.expected {
				t.Errorf("TryFloat64(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================
// ToInt64 Tests
// ============================================================

func TestToInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int64
		wantErr  bool
	}{
		{"int", 42, 42, false},
		{"float64", 3.9, 3, false}, // truncated
		{"string", "100", 100, false},
		{"nil", nil, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ToInt64(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToInt64(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("ToInt64(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================
// ToString Tests
// ============================================================

func TestToString(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
		wantErr  bool
	}{
		{"string", "hello", "hello", false},
		{"int", 42, "42", false},
		{"float64", 3.14, "3.14", false},
		{"bool true", true, "true", false},
		{"bool false", false, "false", false},
		{"bytes", []byte("test"), "test", false},
		{"nil", nil, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ToString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToString(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("ToString(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================
// ToBool Tests
// ============================================================

func TestToBool(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
		wantErr  bool
	}{
		{"bool true", true, true, false},
		{"bool false", false, false, false},
		{"string true", "true", true, false},
		{"string false", "false", false, false},
		{"string TRUE", "TRUE", true, false},
		{"string FALSE", "FALSE", false, false},
		{"string 1", "1", true, false},
		{"string 0", "0", false, false},
		{"string yes", "yes", true, false},
		{"string no", "no", false, false},
		{"string on", "on", true, false},
		{"string off", "off", false, false},
		{"int 1", 1, true, false},
		{"int 0", 0, false, false},
		{"int non-zero", 42, true, false},
		{"float non-zero", 3.14, true, false},
		{"float zero", 0.0, false, false},
		{"nil", nil, false, true},
		{"invalid string", "maybe", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ToBool(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToBool(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("ToBool(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================
// ToTime Tests
// ============================================================

func TestToTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		input   interface{}
		tz      string
		wantErr bool
	}{
		{"time.Time", now, "UTC", false},
		{"RFC3339 string", "2024-01-15T10:30:00Z", "UTC", false},
		{"date string", "2024-01-15", "UTC", false},
		{"datetime string", "2024-01-15 10:30:00", "UTC", false},
		{"unix timestamp int64", int64(1705315800), "UTC", false},
		{"unix timestamp float64", float64(1705315800.5), "UTC", false},
		{"unix timestamp int", 1705315800, "UTC", false},
		{"nil", nil, "UTC", true},
		{"invalid string", "not a date", "UTC", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ToTime(tt.input, tt.tz)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToTime(%v, %s) error = %v, wantErr %v", tt.input, tt.tz, err, tt.wantErr)
			}
		})
	}
}

func TestToTime_Timezone(t *testing.T) {
	// Test that timezone is respected
	input := "2024-01-15 10:30:00"

	utcTime, _ := ToTime(input, "UTC")
	spTime, _ := ToTime(input, "America/Sao_Paulo")

	if utcTime.Location().String() != "UTC" {
		t.Errorf("Expected UTC location, got %s", utcTime.Location())
	}

	if spTime.Location().String() != "America/Sao_Paulo" {
		t.Errorf("Expected America/Sao_Paulo location, got %s", spTime.Location())
	}
}

// ============================================================
// GetLocation Tests
// ============================================================

func TestGetLocation(t *testing.T) {
	tests := []struct {
		name     string
		timezone string
		expected string
	}{
		{"UTC", "UTC", "UTC"},
		{"empty", "", "UTC"},
		{"Sao Paulo", "America/Sao_Paulo", "America/Sao_Paulo"},
		{"invalid", "Invalid/Timezone", "UTC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc := GetLocation(tt.timezone)
			if loc.String() != tt.expected {
				t.Errorf("GetLocation(%s) = %s, want %s", tt.timezone, loc.String(), tt.expected)
			}
		})
	}
}

// ============================================================
// Type Checking Tests
// ============================================================

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{"int", 42, true},
		{"float", 3.14, true},
		{"string number", "42", true},
		{"string non-number", "hello", false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNumeric(tt.input)
			if result != tt.expected {
				t.Errorf("IsNumeric(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsString(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{"string", "hello", true},
		{"int", 42, false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsString(tt.input)
			if result != tt.expected {
				t.Errorf("IsString(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsBool(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{"bool", true, true},
		{"string true", "true", true},
		{"int", 1, true},
		{"invalid string", "maybe", false},
		{"nil", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBool(tt.input)
			if result != tt.expected {
				t.Errorf("IsBool(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================
// ConvertByKind Tests
// ============================================================

func TestConvertByKind(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		kind     string
		tz       string
		expected interface{}
		wantErr  bool
	}{
		{"to string", 42, "string", "", "42", false},
		{"to number", "42", "number", "", 42.0, false},
		{"to float", "3.14", "float", "", 3.14, false},
		{"to boolean", "true", "boolean", "", true, false},
		{"to bool", 1, "bool", "", true, false},
		{"unknown kind", "test", "unknown", "", "test", false},
		{"nil value", nil, "string", "", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertByKind(tt.value, tt.kind, tt.tz)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertByKind(%v, %s) error = %v, wantErr %v", tt.value, tt.kind, err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("ConvertByKind(%v, %s) = %v, want %v", tt.value, tt.kind, result, tt.expected)
			}
		})
	}
}

// ============================================================
// Pointer Helpers Tests
// ============================================================

func TestPtrString(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"simple string", "hello"},
		{"string with spaces", "hello world"},
		{"unicode string", "olá mundo 🌍"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PtrString(tt.input)
			if result == nil {
				t.Error("PtrString() returned nil")
				return
			}
			if *result != tt.input {
				t.Errorf("PtrString(%q) = %q, want %q", tt.input, *result, tt.input)
			}
		})
	}
}

func TestPtrBool(t *testing.T) {
	tests := []struct {
		name  string
		input bool
	}{
		{"true", true},
		{"false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PtrBool(tt.input)
			if result == nil {
				t.Error("PtrBool() returned nil")
				return
			}
			if *result != tt.input {
				t.Errorf("PtrBool(%v) = %v, want %v", tt.input, *result, tt.input)
			}
		})
	}
}

func TestPtrInt(t *testing.T) {
	tests := []struct {
		name  string
		input int
	}{
		{"zero", 0},
		{"positive", 42},
		{"negative", -42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PtrInt(tt.input)
			if result == nil {
				t.Error("PtrInt() returned nil")
				return
			}
			if *result != tt.input {
				t.Errorf("PtrInt(%d) = %d, want %d", tt.input, *result, tt.input)
			}
		})
	}
}

func TestPtrInt64(t *testing.T) {
	tests := []struct {
		name  string
		input int64
	}{
		{"zero", 0},
		{"positive", int64(9223372036854775807)}, // max int64
		{"negative", int64(-9223372036854775808)}, // min int64
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PtrInt64(tt.input)
			if result == nil {
				t.Error("PtrInt64() returned nil")
				return
			}
			if *result != tt.input {
				t.Errorf("PtrInt64(%d) = %d, want %d", tt.input, *result, tt.input)
			}
		})
	}
}

func TestPtrFloat64(t *testing.T) {
	tests := []struct {
		name  string
		input float64
	}{
		{"zero", 0.0},
		{"positive", 3.14159},
		{"negative", -3.14159},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PtrFloat64(tt.input)
			if result == nil {
				t.Error("PtrFloat64() returned nil")
				return
			}
			if *result != tt.input {
				t.Errorf("PtrFloat64(%f) = %f, want %f", tt.input, *result, tt.input)
			}
		})
	}
}

func TestPtrTime(t *testing.T) {
	now := time.Now()
	zero := time.Time{}

	tests := []struct {
		name  string
		input time.Time
	}{
		{"now", now},
		{"zero", zero},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PtrTime(tt.input)
			if result == nil {
				t.Error("PtrTime() returned nil")
				return
			}
			if !result.Equal(tt.input) {
				t.Errorf("PtrTime(%v) = %v, want %v", tt.input, *result, tt.input)
			}
		})
	}
}

// ============================================================
// Benchmarks
// ============================================================

func BenchmarkPtrString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = PtrString("test")
	}
}

func BenchmarkToFloat64_Int(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = ToFloat64(42)
	}
}

func BenchmarkToFloat64_String(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = ToFloat64("42")
	}
}

func BenchmarkToBool_String(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = ToBool("true")
	}
}

func BenchmarkToTime_String(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = ToTime("2024-01-15T10:30:00Z", "UTC")
	}
}
