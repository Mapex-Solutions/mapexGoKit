package random

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// NewRunID returns an identifier safe to embed into payloads whose entries
// will later be cleaned up by prefix scan. The format is:
//
//	YYYYMMDDhhmmss-XXXXXXXX
//
// where the first segment is the UTC timestamp at the moment NewRunID was
// called, and the second segment is 4 random bytes encoded as 8 hex chars.
// The timestamp guarantees lexicographic order across runs; the random
// suffix guarantees uniqueness when several runs start in the same second.
func NewRunID() string {
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	return time.Now().UTC().Format("20060102150405") + "-" + hex.EncodeToString(buf)
}
