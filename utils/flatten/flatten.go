package flatten

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// SeparatorStyle defines how to join nested keys.
type SeparatorStyle struct {
	Before string // prepend to key (each join)
	Middle string // separator between prefix and next token
	After  string // append to key (each join)
}

// Default styles
var (
	DotStyle        = SeparatorStyle{Middle: "."}
	PathStyle       = SeparatorStyle{Middle: "/"}
	RailsStyle      = SeparatorStyle{Before: "[", After: "]"}
	UnderscoreStyle = SeparatorStyle{Middle: "_"}
)

// Errors
var (
	ErrNotValidInput     = errors.New("flatten: input must be map[string]any or []any")
	ErrNotValidJSONInput = errors.New("flatten: JSON input must start with an object")
	ErrNilData           = errors.New("flatten: nil data")
	ErrEmptyPath         = errors.New("flatten: empty path")
	ErrPathNotFound      = errors.New("flatten: path not found")
)

// GetValueByPath extracts a value from a nested map using dot-notation path.
// Handles arrays transparently: when any intermediate value is []any,
// it fans out and collects results from each element.
//
// Returns:
//   - value: the resolved value (single scalar or []any when multi)
//   - multi: true if the path crossed an array, meaning value is []any
//   - err: ErrNilData, ErrEmptyPath, or ErrPathNotFound
//
// Examples:
//
//	// Simple path
//	GetValueByPath(data, "name")         → ("John", false, nil)
//
//	// Nested path
//	GetValueByPath(data, "address.city") → ("NYC", false, nil)
//
//	// Path through array of objects
//	// data = {"sensors": [{"temp": 25.5}, {"temp": 30.2}]}
//	GetValueByPath(data, "sensors.temp") → ([]any{25.5, 30.2}, true, nil)
//
//	// Path ending on array
//	// data = {"values": [1, 2, 3]}
//	GetValueByPath(data, "values")       → ([]any{1, 2, 3}, true, nil)
func GetValueByPath(data map[string]any, path string) (value any, multi bool, err error) {
	if data == nil {
		return nil, false, ErrNilData
	}
	if path == "" {
		return nil, false, ErrEmptyPath
	}

	parts := strings.Split(path, ".")
	return resolvePath(data, parts)
}

// resolvePath recursively traverses a nested structure following path parts.
// When it encounters a []any, it fans out and collects results from each element.
// Recursion depth is bounded by len(parts) (typically 3-5 for IoT paths).
func resolvePath(current any, parts []string) (any, bool, error) {
	// Base case: all parts consumed, return current value
	if len(parts) == 0 {
		if arr, ok := current.([]any); ok {
			return arr, true, nil
		}
		return current, false, nil
	}

	switch v := current.(type) {
	case map[string]any:
		val, exists := v[parts[0]]
		if !exists {
			return nil, false, ErrPathNotFound
		}
		return resolvePath(val, parts[1:])

	case []any:
		// Fan out: traverse each array element with remaining parts (including current part)
		var results []any
		for _, elem := range v {
			val, isMulti, err := resolvePath(elem, parts)
			if err != nil {
				continue // skip elements where path doesn't exist
			}
			if isMulti {
				if arr, ok := val.([]any); ok {
					results = append(results, arr...)
				}
			} else {
				results = append(results, val)
			}
		}
		if len(results) == 0 {
			return nil, false, ErrPathNotFound
		}
		return results, true, nil

	default:
		return nil, false, ErrPathNotFound
	}
}

// Flatten flattens a nested map into dot-notation (or other style).
// Input may contain maps, slices, and scalars (no structs).
// Keys are joined with the provided style; an optional prefix is prepended once.
func Flatten(nested map[string]any, prefix string, style SeparatorStyle) (map[string]any, error) {
	// Small capacity hint
	out := make(map[string]any, len(nested)*2)
	if err := flatten(true, out, nested, prefix, style); err != nil {
		return nil, err
	}
	return out, nil
}

// FlattenString takes a JSON string representing a nested structure and flattens it
// into a single-level JSON string using the specified separator style.
//
// Parameters:
//   - nestedJSON: A string containing the JSON representation of the nested structure.
//   - prefix: A string to prepend to each key in the flattened structure.
//   - style: A SeparatorStyle that defines how keys are joined in the flattened structure.
//
// Returns:
//   - A string containing the JSON representation of the flattened structure.
//   - An error if the input is not valid JSON or if the flattening process fails.
func FlattenString(nestedJSON, prefix string, style SeparatorStyle) (string, error) {
    // Cheap check for leading '{'
    if !looksLikeJSONObject(nestedJSON) {
        return "", ErrNotValidJSONInput
    }

    var nested map[string]any
    if err := json.Unmarshal([]byte(nestedJSON), &nested); err != nil {
        return "", err
    }

    flat, err := Flatten(nested, prefix, style)
    if err != nil {
        return "", err
    }

    b, err := json.Marshal(&flat)
    if err != nil {
        return "", err
    }
    return string(b), nil
}

func looksLikeJSONObject(s string) bool {
	// Trim leading ASCII spaces only (fast path)
	i := 0
	for i < len(s) {
		switch s[i] {
		case ' ', '\n', '\r', '\t':
			i++
			continue
		}
		return s[i] == '{'
	}
	return false
}

func flatten(top bool, out map[string]any, nested any, prefix string, style SeparatorStyle) error {
	switch v := nested.(type) {
	case map[string]any:
		for k, val := range v {
			newKey := joinKey(top, prefix, k, style)
			switch vv := val.(type) {
			case map[string]any, []any:
				if err := flatten(false, out, vv, newKey, style); err != nil {
					return err
				}
			default:
				out[newKey] = vv
			}
		}
	case []any:
		for i, val := range v {
			// Convert index cheaply
			idx := strconv.Itoa(i)
			newKey := joinKey(top, prefix, idx, style)
			switch vv := val.(type) {
			case map[string]any, []any:
				if err := flatten(false, out, vv, newKey, style); err != nil {
					return err
				}
			default:
				out[newKey] = vv
			}
		}
	default:
		return ErrNotValidInput
	}
	return nil
}

func joinKey(top bool, prefix, sub string, style SeparatorStyle) string {
	// Fast paths to avoid allocations when possible
	if top {
		if prefix == "" {
			return sub
		}
		// top + prefix means caller handed us an initial prefix; we append sub without style
		var b strings.Builder
		b.Grow(len(prefix) + len(sub))
		b.WriteString(prefix)
		b.WriteString(sub)
		return b.String()
	}

	// Non-top join: prefix + Before + Middle + sub + After
	// Handle prefix == "" gracefully as well.
	var b strings.Builder
	// rough capacity guess
	b.Grow(len(prefix) + len(style.Before) + len(style.Middle) + len(sub) + len(style.After))
	b.WriteString(prefix)
	b.WriteString(style.Before)
	b.WriteString(style.Middle)
	b.WriteString(sub)
	b.WriteString(style.After)
	return b.String()
}
