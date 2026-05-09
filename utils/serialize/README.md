# serialize — Thin JSON marshal/unmarshal helpers

Two-line wrapper around `encoding/json`. The wrapper exists so other packages (e.g. `redis`, `mapper`) can depend on `serialize.{Marshal,Unmarshal}` and have a single place to swap the implementation later (e.g. for `bytedance/sonic` or `easyjson`) without touching call sites.

> Package name: `serialize` (directory: `serialize/`).

## Surface

```go
func Marshal(v interface{}) (string, error)
func Unmarshal(data string, v interface{}) error

var ErrMarshal   = errors.New("failed to marshal data")
var ErrUnmarshal = errors.New("failed to unmarshal data")
```

| Function | Behaviour |
|---|---|
| `Marshal(v)` | `json.Marshal(v)` then `string(...)`. On error returns `"", ErrMarshal` — the underlying error is **not** wrapped. |
| `Unmarshal(data, v)` | `json.Unmarshal([]byte(data), v)` — returns the original `encoding/json` error. `ErrUnmarshal` is defined but **not currently raised**. |

## Usage

```go
str, err := serialize.Marshal(map[string]int{"a": 1, "b": 2})
// str = `{"a":1,"b":2}`

var out map[string]int
if err := serialize.Unmarshal(str, &out); err != nil { return err }
```

## Notes

- `Marshal` discards the original `encoding/json` error and returns the sentinel — use this only when the caller does not need to discriminate between `json.UnsupportedTypeError`, `json.UnsupportedValueError`, etc.
- `ErrUnmarshal` is exported but unused by the package. Defined for symmetry; safe to ignore.
