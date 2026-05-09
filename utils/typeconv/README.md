# typeconv — Permissive `interface{}` → typed conversions

Run-time helpers for shapes you receive as `interface{}` (parsed JSON, MongoDB documents, query parameters, dynamic schemas, etc.). Every conversion comes in two forms — `Toxxx(value) (xxx, error)` and `Tryxxx(value) (xxx, bool)` — to fit either error-aware or boolean-style call sites.

> Package name: `typeconv` (directory: `typeconv/`).

## Errors

```go
var ErrNilValue       = fmt.Errorf("nil value")
var ErrTypeConversion = fmt.Errorf("type conversion error")
```

`Toxxx` wraps the underlying failure with `%w: …`, so `errors.Is(err, ErrTypeConversion)` works.

## Numeric conversions

```go
func ToFloat64(v interface{}) (float64, error)
func TryFloat64(v interface{}) (float64, bool)
func ToInt64(v interface{})   (int64, error)
func TryInt64(v interface{})  (int64, bool)
```

Accepted inputs:

| Source | Behaviour |
|---|---|
| `float64`/`float32` | Direct |
| `int`/`int8…64` / `uint`/`uint8…64` | Cast |
| `bool` | `true→1`, `false→0` |
| `string` | `strconv.ParseFloat` (numeric strings only) |
| anything else | `ErrTypeConversion: unsupported type %T for number conversion` |
| `nil` | `ErrNilValue` |

`ToInt64` truncates the result of `ToFloat64`.

## String conversion

```go
func ToString(v interface{}) (string, error)
func TryString(v interface{}) (string, bool)
```

| Source | Behaviour |
|---|---|
| `string` | Direct |
| `[]byte` | `string(v)` |
| `fmt.Stringer` | `v.String()` |
| `bool` | `strconv.FormatBool` |
| integers / floats | `fmt.Sprintf("%d"/"%v", ...)` |
| anything else | `fmt.Sprintf("%v", v)` (never errors) |
| `nil` | `ErrNilValue` |

## Boolean conversion

```go
func ToBool(v interface{}) (bool, error)
func TryBool(v interface{}) (bool, bool)
```

Accepted strings (case-insensitive, trimmed): `"true"`, `"1"`, `"yes"`, `"on"` → `true`; `"false"`, `"0"`, `"no"`, `"off"`, `""` → `false`. Any other string yields `ErrTypeConversion`. Numeric inputs follow the rule "non-zero is true".

## Time conversion

```go
func ToTime(v interface{}, timezone string) (time.Time, error)
func TryTime(v interface{}, timezone string) (time.Time, bool)
```

`timezone` is the name passed to `time.LoadLocation`. Empty or invalid → falls back to UTC (`GetLocation`).

| Source | Treated as |
|---|---|
| `time.Time` | Re-located via `t.In(loc)` |
| `string` | Tried against `DateFormats` then `TimeFormats` (see below); first match wins |
| `int64` / `int` | Unix seconds (`time.Unix(v, 0)`) |
| `float64` | Unix seconds + sub-second fraction (`(v - sec) * 1e9` ns) |
| anything else | `ErrTypeConversion` |
| `nil` | `ErrNilValue` |

### `DateFormats`

```go
[]string{
    time.RFC3339,
    time.RFC3339Nano,
    "2006-01-02T15:04:05",
    "2006-01-02 15:04:05",
    "2006-01-02",
    "02/01/2006",
    "01/02/2006",
    "2006/01/02",
}
```

### `TimeFormats`

```go
[]string{"15:04:05", "15:04", "3:04 PM", "3:04:05 PM"}
```

Both lists are package-level `var`s — assign before parsing if you need to add formats globally.

## Timezone helpers

```go
func GetLocation(timezone string) *time.Location  // empty/invalid → UTC
func GetCurrentTime(timezone string) time.Time    // = time.Now().In(GetLocation(tz))
```

## Type checks

```go
func IsNumeric(v interface{}) bool
func IsString(v interface{}) bool
func IsBool(v interface{}) bool
```

`IsNumeric` and `IsBool` use the matching `Tryxxx` (so a numeric string returns `true` for `IsNumeric`). `IsString` is a strict assertion — only `string` returns `true`.

## Pointer factories

```go
func PtrString(s string)    *string
func PtrBool(b bool)        *bool
func PtrInt(i int)          *int
func PtrInt64(i int64)      *int64
func PtrFloat64(f float64)  *float64
func PtrTime(t time.Time)   *time.Time
```

Inline pointer creators for optional DTO/query fields. Mirror `zerovalue.Ptr[T]` for the common scalar types.

## Dynamic conversion

```go
func ConvertByKind(value interface{}, kind string, timezone string) (interface{}, error)
```

Routes by lowercased `kind`:

| `kind` | Maps to |
|---|---|
| `"string"` | `ToString` |
| `"number"` / `"float"` / `"integer"` / `"int"` | `ToFloat64` |
| `"boolean"` / `"bool"` | `ToBool` |
| `"date"` / `"datetime"` / `"time"` | `ToTime(value, timezone)` |
| anything else | Returns the value as-is, no error |
| `nil` value | `ErrNilValue` |

Useful for dynamic schemas where the field type is described as a string.

## Notes

- `ToFloat64` / `ToInt64` accept booleans (intentional). If you need strict numeric input, type-assert first.
- `ConvertByKind`'s `"int"`/`"integer"` path returns a `float64`, not an `int64` — keep this in mind when the downstream expects an integer (call `ToInt64` instead).
- `ToString` never returns `ErrTypeConversion` — every non-nil value falls through to `%v`. The error path only triggers on `nil`.
