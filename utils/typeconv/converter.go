package typeconv

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ============================================================
// Common Date/Time Formats
// ============================================================

// DateFormats contains common date formats for parsing
var DateFormats = []string{
	time.RFC3339,
	time.RFC3339Nano,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"02/01/2006",
	"01/02/2006",
	"2006/01/02",
}

// TimeFormats contains common time-only formats for parsing
var TimeFormats = []string{
	"15:04:05",
	"15:04",
	"3:04 PM",
	"3:04:05 PM",
}

// ============================================================
// Error Definitions
// ============================================================

var (
	// ErrNilValue indicates a nil value was provided
	ErrNilValue = fmt.Errorf("nil value")
	// ErrTypeConversion indicates a type conversion failure
	ErrTypeConversion = fmt.Errorf("type conversion error")
)

// ============================================================
// ToFloat64 - Convert to float64
// ============================================================

// ToFloat64 converts a value to float64.
// Supports: int, float, string (numeric), bool (1/0).
//
// Returns (float64, nil) on success or (0, error) on failure.
func ToFloat64(value interface{}) (float64, error) {
	if value == nil {
		return 0, ErrNilValue
	}

	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: cannot convert '%s' to number", ErrTypeConversion, v)
		}
		return f, nil
	default:
		return 0, fmt.Errorf("%w: unsupported type %T for number conversion", ErrTypeConversion, value)
	}
}

// TryFloat64 converts a value to float64.
// Returns (float64, true) on success or (0, false) on failure.
// This is the "ok" style variant for use in comparisons.
func TryFloat64(value interface{}) (float64, bool) {
	f, err := ToFloat64(value)
	return f, err == nil
}

// ============================================================
// ToInt64 - Convert to int64
// ============================================================

// ToInt64 converts a value to int64.
// Supports: int, float (truncated), string (numeric), bool (1/0).
func ToInt64(value interface{}) (int64, error) {
	f, err := ToFloat64(value)
	if err != nil {
		return 0, err
	}
	return int64(f), nil
}

// TryInt64 converts a value to int64.
// Returns (int64, true) on success or (0, false) on failure.
func TryInt64(value interface{}) (int64, bool) {
	i, err := ToInt64(value)
	return i, err == nil
}

// ============================================================
// ToString - Convert to string
// ============================================================

// ToString converts a value to string.
// Handles all types by converting to their string representation.
func ToString(value interface{}) (string, error) {
	if value == nil {
		return "", ErrNilValue
	}

	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	case fmt.Stringer:
		return v.String(), nil
	case bool:
		return strconv.FormatBool(v), nil
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v), nil
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v), nil
	case float32, float64:
		return fmt.Sprintf("%v", v), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

// TryString converts a value to string.
// Returns (string, true) on success or ("", false) on failure.
func TryString(value interface{}) (string, bool) {
	s, err := ToString(value)
	return s, err == nil
}

// ============================================================
// ToBool - Convert to bool
// ============================================================

// ToBool converts a value to bool.
// Supports: bool, string ("true"/"false"/"1"/"0"/"yes"/"no"), numbers (0=false, else=true).
func ToBool(value interface{}) (bool, error) {
	if value == nil {
		return false, ErrNilValue
	}

	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		lower := strings.ToLower(strings.TrimSpace(v))
		switch lower {
		case "true", "1", "yes", "on":
			return true, nil
		case "false", "0", "no", "off", "":
			return false, nil
		default:
			return false, fmt.Errorf("%w: cannot convert '%s' to boolean", ErrTypeConversion, v)
		}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		f, _ := ToFloat64(v)
		return f != 0, nil
	default:
		return false, fmt.Errorf("%w: unsupported type %T for boolean conversion", ErrTypeConversion, value)
	}
}

// TryBool converts a value to bool.
// Returns (bool, true) on success or (false, false) on failure.
func TryBool(value interface{}) (bool, bool) {
	b, err := ToBool(value)
	return b, err == nil
}

// ============================================================
// ToTime - Convert to time.Time
// ============================================================

// ToTime converts a value to time.Time.
// Supports: time.Time, string (RFC3339, common formats), int64/float64 (unix timestamp).
//
// The timezone parameter is used for parsing strings without timezone info.
func ToTime(value interface{}, timezone string) (time.Time, error) {
	if value == nil {
		return time.Time{}, ErrNilValue
	}

	loc := GetLocation(timezone)

	switch v := value.(type) {
	case time.Time:
		return v.In(loc), nil
	case string:
		return parseTimeString(v, loc)
	case int64:
		// Unix timestamp (seconds)
		return time.Unix(v, 0).In(loc), nil
	case float64:
		// Unix timestamp with milliseconds
		sec := int64(v)
		nsec := int64((v - float64(sec)) * 1e9)
		return time.Unix(sec, nsec).In(loc), nil
	case int:
		return time.Unix(int64(v), 0).In(loc), nil
	default:
		return time.Time{}, fmt.Errorf("%w: unsupported type %T for time conversion", ErrTypeConversion, value)
	}
}

// TryTime converts a value to time.Time.
// Returns (time.Time, true) on success or (zero, false) on failure.
func TryTime(value interface{}, timezone string) (time.Time, bool) {
	t, err := ToTime(value, timezone)
	return t, err == nil
}

// parseTimeString attempts to parse a string as time using common formats.
func parseTimeString(s string, loc *time.Location) (time.Time, error) {
	// Try date formats first
	for _, format := range DateFormats {
		if t, err := time.ParseInLocation(format, s, loc); err == nil {
			return t, nil
		}
	}

	// Try time-only formats
	for _, format := range TimeFormats {
		if t, err := time.ParseInLocation(format, s, loc); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("%w: cannot parse '%s' as time", ErrTypeConversion, s)
}

// ============================================================
// Timezone Helpers
// ============================================================

// GetLocation returns a time.Location for the given timezone.
// Falls back to UTC if timezone is empty or invalid.
func GetLocation(timezone string) *time.Location {
	if timezone == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// GetCurrentTime returns the current time in the specified timezone.
func GetCurrentTime(timezone string) time.Time {
	return time.Now().In(GetLocation(timezone))
}

// ============================================================
// Type Checking Helpers
// ============================================================

// IsNumeric checks if a value can be converted to a number.
func IsNumeric(value interface{}) bool {
	_, ok := TryFloat64(value)
	return ok
}

// IsString checks if a value is a string.
func IsString(value interface{}) bool {
	_, ok := value.(string)
	return ok
}

// IsBool checks if a value can be converted to a boolean.
func IsBool(value interface{}) bool {
	_, ok := TryBool(value)
	return ok
}

// ============================================================
// Pointer Helpers - Create pointers from values
// ============================================================

// PtrString returns a pointer to the given string value.
// Useful for optional fields in DTOs and query parameters.
func PtrString(s string) *string {
	return &s
}

// PtrBool returns a pointer to the given bool value.
func PtrBool(b bool) *bool {
	return &b
}

// PtrInt returns a pointer to the given int value.
func PtrInt(i int) *int {
	return &i
}

// PtrInt64 returns a pointer to the given int64 value.
func PtrInt64(i int64) *int64 {
	return &i
}

// PtrFloat64 returns a pointer to the given float64 value.
func PtrFloat64(f float64) *float64 {
	return &f
}

// PtrTime returns a pointer to the given time.Time value.
func PtrTime(t time.Time) *time.Time {
	return &t
}

// ============================================================
// ConvertByKind - Dynamic Type Conversion
// ============================================================

// ConvertByKind converts a value to the specified kind (type hint).
//
// Supported kinds: "string", "number", "float", "integer", "int", "boolean", "bool", "date", "datetime", "time"
func ConvertByKind(value interface{}, kind string, timezone string) (interface{}, error) {
	if value == nil {
		return nil, ErrNilValue
	}

	switch strings.ToLower(kind) {
	case "string":
		return ToString(value)
	case "number", "float", "integer", "int":
		return ToFloat64(value)
	case "boolean", "bool":
		return ToBool(value)
	case "date", "datetime", "time":
		return ToTime(value, timezone)
	default:
		// Unknown kind, return as-is
		return value, nil
	}
}
