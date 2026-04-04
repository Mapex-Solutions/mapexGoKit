// Package deepcopy provides deep copy utilities for generic Go data structures.
package deepcopy

import "encoding/json"

// Map creates a deep copy of a map[string]interface{} via JSON round-trip.
// Returns an empty map if src is nil or serialization fails.
func Map(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return make(map[string]interface{})
	}
	data, _ := json.Marshal(src)
	var dst map[string]interface{}
	json.Unmarshal(data, &dst)
	if dst == nil {
		return make(map[string]interface{})
	}
	return dst
}

// MapOfMaps creates a deep copy of a map[string]map[string]interface{}.
// Each inner map is deep-copied via JSON round-trip.
// Returns nil if src is nil.
func MapOfMaps(src map[string]map[string]interface{}) map[string]map[string]interface{} {
	if src == nil {
		return nil
	}
	dst := make(map[string]map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = Map(v)
	}
	return dst
}
