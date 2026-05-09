# deepcopy — JSON-roundtrip deep copy for `map[string]interface{}`

Two helpers that deep-copy `map[string]any` (and the map-of-maps variant) by serialising to JSON and back. Loses anything JSON cannot represent (channels, functions, NaN/Inf, time fidelity beyond the JSON layout, etc.) — use only when the values are JSON-friendly.

> Package name: `deepcopy` (directory: `deepcopy/`).

## Surface

```go
func Map(src map[string]interface{}) map[string]interface{}
func MapOfMaps(src map[string]map[string]interface{}) map[string]map[string]interface{}
```

| Function | Behaviour |
|---|---|
| `Map(src)` | `nil` source → empty map. Marshal → Unmarshal. If JSON fails, returns an empty map (errors are swallowed). |
| `MapOfMaps(src)` | `nil` source → `nil`. Otherwise allocates a new top-level map and deep-copies each inner map via `Map`. |

## Usage

```go
src := map[string]interface{}{
    "user": map[string]interface{}{"name": "Alice"},
    "tags": []string{"a", "b"},
}
cp := deepcopy.Map(src)
cp["user"].(map[string]interface{})["name"] = "Bob"
// src["user"]["name"] still "Alice" — deep-copied
```

## Notes

- Both functions ignore JSON marshal/unmarshal errors. If you need to detect failure, marshal yourself.
- Numbers are decoded as `float64` (the standard `encoding/json` behaviour).
