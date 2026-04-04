package mongoModel

import "go.mongodb.org/mongo-driver/v2/bson"

/*
 * SAFE MAP ACCESSORS
 * Type-safe accessors for map[string]interface{} and bson.M values.
 * Returns zero value when key is missing or type assertion fails.
 * Designed for parsing untyped maps from MongoDB documents.
 *
 * Handles both map[string]interface{} (from JSON/cache) and bson.M (from MongoDB driver).
 * Nested documents (bson.D) are automatically converted to map[string]interface{}.
 */

// MapGetString returns the string value for the key, or "" if missing or wrong type.
func MapGetString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// MapGetInt returns the int value for the key, handling int32/int64/float64 conversions.
// Returns 0 if missing or incompatible type.
func MapGetInt(m map[string]interface{}, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// MapGetBool returns the bool value for the key, or false if missing or wrong type.
func MapGetBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// MapGetMap returns the nested map for the key, or nil if missing or wrong type.
// Handles map[string]interface{} (JSON/cache), bson.M (MongoDB driver), and bson.D (ordered document).
func MapGetMap(m map[string]interface{}, key string) map[string]interface{} {
	switch v := m[key].(type) {
	case map[string]interface{}:
		return v
	case bson.M:
		return map[string]interface{}(v)
	case bson.D:
		return bsonDToMap(v)
	default:
		return nil
	}
}

// MapGetSlice returns the []interface{} for the key, or nil if missing or wrong type.
// Handles both []interface{} (JSON/cache) and bson.A (MongoDB driver).
func MapGetSlice(m map[string]interface{}, key string) []interface{} {
	switch v := m[key].(type) {
	case []interface{}:
		return v
	case bson.A:
		return []interface{}(v)
	default:
		return nil
	}
}

// MapGetStringSlice returns []string for the key, filtering out non-string elements.
// Returns nil if key is missing or not a slice.
func MapGetStringSlice(m map[string]interface{}, key string) []string {
	raw := MapGetSlice(m, key)
	if raw == nil {
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// ToMap converts an interface{} to map[string]interface{}.
// Handles map[string]interface{}, bson.M, and bson.D transparently.
// Returns nil if the value cannot be converted to a map.
func ToMap(val interface{}) map[string]interface{} {
	switch v := val.(type) {
	case map[string]interface{}:
		return v
	case bson.M:
		return map[string]interface{}(v)
	case bson.D:
		return bsonDToMap(v)
	default:
		return nil
	}
}

// bsonDToMap converts bson.D (ordered document) to map[string]interface{}.
func bsonDToMap(d bson.D) map[string]interface{} {
	result := make(map[string]interface{}, len(d))
	for _, elem := range d {
		result[elem.Key] = elem.Value
	}
	return result
}
