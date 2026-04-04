package timeUtil

import (
	"fmt"
	"time"
)

// RFC3339Milli defines the layout used by MongoDB (ISODate with milliseconds).
const RFC3339Milli = "2006-01-02T15:04:05.000Z07:00"

// ToRFC3339Milli converts a time.Time into a *string
// formatted as RFC3339 with milliseconds (MongoDB style).
// Returns nil if the input time is zero.
func ToRFC3339Milli(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	str := t.UTC().Format(RFC3339Milli)
	return &str
}

// NullTime is a wrapper around time.Time for JSON serialization.
// It serializes into RFC3339 with milliseconds or null if the time is zero.
type NullTime struct {
	time.Time
}

func (nt NullTime) MarshalJSON() ([]byte, error) {
	if nt.Time.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf(`"%s"`, nt.UTC().Format(RFC3339Milli))), nil
}
