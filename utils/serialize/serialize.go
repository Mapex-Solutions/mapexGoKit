// Package serialize provides simple helpers for JSON serialization
// and deserialization of Go values.
package serialize

import (
	"encoding/json"
)

// Marshal serializes a Go value into its JSON string representation.
//
// Parameters:
//   - v: the value to serialize. It can be any Go type supported by json.Marshal.
//
// Returns:
//   - JSON string representation of v
//   - ErrMarshal if encoding fails
func Marshal(v interface{}) (string, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return "", ErrMarshal
	}
	return string(bytes), nil
}

// Unmarshal parses a JSON string and stores the result in the value
// pointed to by v.
//
// Parameters:
//   - data: JSON string to decode
//   - v: pointer to the destination value (must be addressable)
//
// Returns:
//   - error if decoding fails or the data is not valid JSON
func Unmarshal(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}
