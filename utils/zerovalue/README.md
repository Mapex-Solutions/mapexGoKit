# zerovalue — Zero-value generators and a generic `Ptr` helper

Two distinct concerns under one roof — both answer the same question: "give me the canonical value for a field whose type the caller cannot inline."

> Package name: `zerovalue` (directory: `zerovalue/`).

## Surface

```go
type FieldType string
const (
    TypeNumber  FieldType = "number"
    TypeString  FieldType = "string"
    TypeBoolean FieldType = "boolean"
    TypeArray   FieldType = "array"
    TypeObject  FieldType = "object"
)

func GetZeroValue(fieldType string)          interface{}
func GetZeroValueTyped(fieldType FieldType)   interface{}
func IsValidType(fieldType string)            bool

func Ptr[T any](v T) *T
```

### `GetZeroValue` / `GetZeroValueTyped` — String-keyed zero values

For dynamic schemas (think `Rule.LocalState` or form-field defaults) where the type is described as a string:

| Input | Output |
|---|---|
| `"number"` | `0` (`int`) |
| `"string"` | `""` |
| `"boolean"` | `false` |
| `"array"` | `[]interface{}{}` |
| `"object"` | `map[string]interface{}{}` |
| anything else | `nil` |

`GetZeroValueTyped` is the same function with the typed wrapper for callers that already hold a `FieldType`.

### `IsValidType`

Returns `true` for any of the five known constants, `false` for everything else. Use it before persisting a user-supplied type string.

### `Ptr[T any](v T) *T`

Inline pointer creator. Lets callers build payloads with optional pointer fields without introducing a throwaway variable at every call site:

```go
dto := AssetCreate{
    ThresholdMinutes: zerovalue.Ptr(10),
    Enabled:          zerovalue.Ptr(true),
    Tags:             zerovalue.Ptr([]string{"a", "b"}),
}
```

The helper is in this package because both `Ptr(v)` and `GetZeroValue(...)` answer the same family of question — "give me the canonical value for a field whose type the caller cannot inline."

## Notes

- `GetZeroValue("number")` returns an `int` (`0`), **not a `float64`**. If your dynamic schema layer uses `float64` everywhere (e.g. JSON-decoded data), wrap the result accordingly.
- The "array" zero is `[]interface{}{}`, not `nil` — persistence layers that distinguish "absent" from "empty" should read this as "empty".
