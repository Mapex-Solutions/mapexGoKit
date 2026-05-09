# time — RFC3339-with-milliseconds helpers (`timeUtil`)

Tiny helpers around `time.Time` for the MongoDB-style ISO date layout (`RFC3339` + `.000` milliseconds). Useful for JSON DTOs that must serialise zero values as `null` and non-zero values in millisecond precision.

> Package name: `timeUtil` (directory: `time/`).

## Surface

```go
const RFC3339Milli = "2006-01-02T15:04:05.000Z07:00"

func ToRFC3339Milli(t time.Time) *string

type NullTime struct { time.Time }
func (NullTime) MarshalJSON() ([]byte, error)
```

### `ToRFC3339Milli`

Returns a `*string`:

- `nil` when `t.IsZero()`.
- Otherwise the UTC instant formatted with `RFC3339Milli`.

Useful for optional fields in DTOs where omitting/`null` is more correct than `"0001-01-01T..."`.

### `NullTime`

A `time.Time` wrapper with a `MarshalJSON` that emits:

- `null` when the inner time is zero.
- A double-quoted RFC3339-millisecond UTC string otherwise.

Embed it in DTOs:

```go
type AssetDTO struct {
    Created *timeUtil.NullTime `json:"created"`
    Updated *timeUtil.NullTime `json:"updated"`
}
```

`NullTime` only customises serialisation. **It does not implement `UnmarshalJSON`**, so JSON `null` will not survive a round-trip — the field will remain at its zero value, which is what you usually want anyway for these conventional fields.
