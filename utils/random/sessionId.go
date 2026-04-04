package random

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateSessionID generates a cryptographically secure random session ID.
//
// The `length` parameter specifies the number of random bytes to generate.
// Since each byte is represented by two hexadecimal characters, the resulting
// string will be twice as long as `length`.
//
// For example:
//   - length = 4  → 8-character hex string (e.g., "9a7b3c4d")
//   - length = 16 → 32-character hex string (e.g., "4f2a9b3e8d7c6b1a...")
//
// The function uses crypto/rand to ensure the output is unpredictable and
// suitable for use as a secure identifier, such as a session ID in a token.
//
// Example:
//
//	sid, err := GenerateSessionID(4)
//	if err != nil {
//	    log.Fatalf("failed to generate session ID: %v", err)
//	}
//	fmt.Println(sid)
//
// Returns:
//   - The generated session ID as a lowercase hexadecimal string.
//   - An error if the random number generator fails.
func GenerateSessionID(length int) (string, error) {
	bytes := make([]byte, length) // length in bytes (2 hex chars per byte)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
