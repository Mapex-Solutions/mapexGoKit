# templatereplace — `{{path.to.value}}` interpolation in JSON-like trees

Recursively replaces `{{path.to.value}}` placeholders inside any value (string, `map[string]any`, `[]any`) using a flat `contexts map[string]any` whose values may themselves be nested maps. Pure function, zero dependencies beyond stdlib.

> Package name: `templatereplace` (directory: `templatereplace/`).

## Surface

```go
func Resolve(value interface{}, contexts map[string]interface{}) interface{}
func ResolveString(s string, contexts map[string]interface{}) string
```

| Function | Behaviour |
|---|---|
| `Resolve(value, contexts)` | Walks `string` / `map[string]interface{}` / `[]interface{}` recursively. Other types are returned as-is (no copy). New maps/slices are allocated; the input is not mutated. |
| `ResolveString(s, contexts)` | Replaces all `{{…}}` occurrences in a single string. Short-circuits when the string does not contain `"{{"`. |

## Resolution algorithm

1. Match every `{{...}}` group via the package-level regex.
2. Strip the braces → the inner is treated as a dotted path (e.g. `config.chatId`).
3. Walk `contexts` segment by segment: each segment must resolve to either a value (terminal) or a `map[string]interface{}` (intermediate).
4. If the path resolves, replace the match with `fmt.Sprintf("%v", resolved)`.
5. If any segment is missing or the intermediate is not a map, **leave the original `{{...}}` text untouched** — useful for templates that intentionally span multiple resolution passes (e.g. a placeholder like `{{before.token}}` that will be filled later).

## Usage

```go
contexts := map[string]interface{}{
    "config":   map[string]interface{}{"chatId": "abc-123"},
    "manifest": map[string]interface{}{"defaults": map[string]interface{}{"baseUrl": "https://api"}},
}

// Single string
templatereplace.ResolveString("{{config.chatId}}", contexts) // "abc-123"

// Nested structure
payload := map[string]interface{}{
    "url":  "{{manifest.defaults.baseUrl}}/v1",
    "tags": []interface{}{"chat:{{config.chatId}}", "{{unresolved.path}}"},
}
out := templatereplace.Resolve(payload, contexts)
// out["url"]  == "https://api/v1"
// out["tags"] == ["chat:abc-123", "{{unresolved.path}}"]   // 2nd left intact
```

## Notes

- The placeholder regex is `\{\{([^}]+)\}\}` — it does **not** allow `}` inside the path. Useful in practice; do not pass paths containing `}`.
- Resolution always converts the value via `fmt.Sprintf("%v", v)`. Values of complex types (maps, slices) become Go's default formatted representation — usually not what you want; keep terminal placeholders pointing at scalars.
