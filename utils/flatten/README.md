# flatten — Nested map ↔ flat-key translation + dotted-path lookup

Two cooperating concerns:

1. **Path resolution** — `GetValueByPath(data, "sensors.temp")` walks a nested `map[string]any`, fanning out across array intermediaries.
2. **Flattening** — `Flatten(...)` / `FlattenString(...)` collapse an arbitrarily nested structure into a single-level map keyed by dotted (or other) paths.

Both work on JSON-shaped values: `map[string]any`, `[]any`, scalars. There are no struct/reflection paths.

> Package name: `flatten` (directory: `flatten/`).

## Separator styles

```go
type SeparatorStyle struct { Before, Middle, After string }

var (
    DotStyle        = SeparatorStyle{Middle: "."}        // a.b.c
    PathStyle       = SeparatorStyle{Middle: "/"}        // a/b/c
    RailsStyle      = SeparatorStyle{Before: "[", After: "]"} // a[b][c]
    UnderscoreStyle = SeparatorStyle{Middle: "_"}        // a_b_c
)
```

Custom styles are just three strings — anything you can express as `prefix Before Middle key After` is fine.

## Errors

```go
var ErrNotValidInput     = errors.New("flatten: input must be map[string]any or []any")
var ErrNotValidJSONInput = errors.New("flatten: JSON input must start with an object")
var ErrNilData           = errors.New("flatten: nil data")
var ErrEmptyPath         = errors.New("flatten: empty path")
var ErrPathNotFound      = errors.New("flatten: path not found")
```

## `GetValueByPath` — dotted-path lookup with array fan-out

```go
func GetValueByPath(data map[string]any, path string) (value any, multi bool, err error)
```

Walks `data` segment by segment (split by `.`). When any intermediate value is `[]any`, the walker **fans out** — it keeps walking with the remaining segments inside each element and aggregates the results. The result type is reported via `multi`:

| Scenario | Returns |
|---|---|
| Simple scalar | `(value, false, nil)` |
| Path ends on `[]any` | `([]any, true, nil)` |
| Path crosses an array of objects | `([]any{e1.path, e2.path, …}, true, nil)` |
| Path missing | `(nil, false, ErrPathNotFound)` |
| `nil` data | `(nil, false, ErrNilData)` |
| empty `path` | `(nil, false, ErrEmptyPath)` |

Examples:

```go
data := map[string]any{
    "name": "John",
    "address": map[string]any{"city": "NYC"},
    "sensors": []any{
        map[string]any{"temp": 25.5},
        map[string]any{"temp": 30.2},
    },
    "values": []any{1, 2, 3},
}

GetValueByPath(data, "name")          // ("John",            false, nil)
GetValueByPath(data, "address.city")  // ("NYC",             false, nil)
GetValueByPath(data, "sensors.temp")  // ([]any{25.5, 30.2}, true,  nil)
GetValueByPath(data, "values")        // ([]any{1, 2, 3},    true,  nil)
GetValueByPath(data, "missing")       // (nil,               false, ErrPathNotFound)
```

When fanning out, elements where the sub-path doesn't exist are silently skipped. If **every** element fails, the call returns `ErrPathNotFound` rather than an empty slice.

## `Flatten` — collapse a nested map

```go
func Flatten(nested map[string]any, prefix string, style SeparatorStyle) (map[string]any, error)
```

Walks the structure depth-first. Map keys become path components; slice indexes become numeric tokens (`strconv.Itoa(i)`):

```go
in := map[string]any{
    "user": map[string]any{
        "name": "Alice",
        "tags": []any{"a", "b"},
    },
}
out, _ := flatten.Flatten(in, "", flatten.DotStyle)
// out == {
//   "user.name":   "Alice",
//   "user.tags.0": "a",
//   "user.tags.1": "b",
// }
```

The `prefix` parameter is prepended **once** (no separator between prefix and the first key). Pass `""` to start fresh.

`Flatten` only accepts `map[string]any` and `[]any` for nested values — anything else is returned via `ErrNotValidInput`.

### `FlattenString` — JSON-string convenience

```go
func FlattenString(nestedJSON, prefix string, style SeparatorStyle) (string, error)
```

Cheap leading-`{` check via `looksLikeJSONObject`, then `json.Unmarshal` → `Flatten` → `json.Marshal`. Inputs that don't start with `{` (after trimming whitespace) return `ErrNotValidJSONInput`.

## Notes

- The leading-`{` check is a fast pre-filter — actual JSON validity is enforced by `json.Unmarshal` afterwards.
- Slice elements that are scalars are flattened by index. A slice of objects yields `key.0.field`, `key.1.field`, etc.
- `Flatten` does not preserve order beyond Go's map iteration order; the output is a `map`, not an ordered list.
