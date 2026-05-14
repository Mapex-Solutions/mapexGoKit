package random

import (
	"regexp"
	"testing"
	"time"
)

func TestNewRunID_Format(t *testing.T) {
	id := NewRunID()

	pattern := regexp.MustCompile(`^\d{14}-[0-9a-f]{8}$`)
	if !pattern.MatchString(id) {
		t.Fatalf("NewRunID = %q does not match YYYYMMDDhhmmss-xxxxxxxx", id)
	}
}

func TestNewRunID_TimestampIsRecent(t *testing.T) {
	before := time.Now().UTC().Add(-time.Second)
	id := NewRunID()
	after := time.Now().UTC().Add(time.Second)

	ts, err := time.Parse("20060102150405", id[:14])
	if err != nil {
		t.Fatalf("timestamp prefix not parseable: %v", err)
	}
	if ts.Before(before) || ts.After(after) {
		t.Fatalf("timestamp %v outside [%v, %v]", ts, before, after)
	}
}

func TestNewRunID_Uniqueness(t *testing.T) {
	const n = 1000
	seen := make(map[string]struct{}, n)
	for i := range n {
		id := NewRunID()
		if _, dup := seen[id]; dup {
			t.Fatalf("collision after %d ids: %s", i, id)
		}
		seen[id] = struct{}{}
	}
}
