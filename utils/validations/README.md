# validations — Singleton `go-playground/validator` with Mapex extras

Wraps [`go-playground/validator/v10`](https://github.com/go-playground/validator) into a process-wide singleton with Mapex's custom rules pre-registered (`mongoid`, `uuid`) and a humanised error formatter that returns `[]string` instead of opaque tag-tuples.

> Package name: `validations` (directory: `validations/`); custom rules under `validations/customvalidation/`.

## Surface

```go
// Re-exports
type Validate      = validator.Validate
type FieldLevel    = validator.FieldLevel
type CustomFunc    = validator.Func
type CustomFuncCtx = validator.FuncCtx

// Errors
type ValidationError struct {
    Field string `json:"field"`
    Tag   string `json:"tag"`
    Param string `json:"param,omitempty"`
}
type ValidationErrors []string  // implements error; .Error() returns "validation failed"

// Singleton
func New() *validator.Validate                              // sync.Once-guarded; registers custom rules
func ValidateStruct(s any) []string
func ValidateStructCtx(ctx context.Context, s any) []string
func ParseTypeError(err error) []string
```

### Singleton

`New()` returns the same `*validator.Validate` on every call. The first call registers two custom tags via `customvalidation.RegisterMongoID` and `customvalidation.RegisterUUID`. Anything you need beyond that must be registered on the returned instance before validation runs concurrently.

### `ValidateStruct` / `ValidateStructCtx`

- Returns `nil` on success.
- Returns `[]string` with **one humanised message per failed field** when validation fails — see the message catalogue below. Errors that are not `validator.ValidationErrors` are wrapped in a single-element slice with `err.Error()`.

`ValidateStructCtx` is the same with a context (cancellation/timeout flows through to struct-level validators).

### `ParseTypeError`

Friendlier message for `*json.UnmarshalTypeError`. Useful when a JSON body has a wrong type for a field:

```go
errs := validations.ParseTypeError(err)
// e.g. ["field changePasswordNextLogin expected bool but came string"]
```

Falls back to `["erro de parsing: <err>"]` when `err` is not an `UnmarshalTypeError`.

## Message catalogue (`humanizeFieldError`)

Recognised tags and their generated messages:

| Tag | Message template (`{field}` / `{param}`) |
|---|---|
| `required` | `The field '{f}' is required.` |
| `eqfield` / `nefield` | `The field '{f}' must be equal to / different from the field '{p}'.` |
| `len` | `The field '{f}' must be exactly {p} characters long.` |
| `min` / `max` (string/slice/map/array) | `The field '{f}' must have at least/at most {p} characters.` |
| `min` / `max` (numeric) | `The field '{f}' must be at least/at most {p}.` |
| `gt` / `gte` / `lt` / `lte` | `The field '{f}' must be greater/less than (or equal to) {p}.` |
| `email` / `alphanum` / `alpha` / `numeric` / `boolean` / `url` / `uuid` / `ipv4` / `ipv6` | `The field '{f}' must be a valid …` |
| `datetime` | With param: `… must match the date/time format '{p}'.` Without: `… must be a valid date/time.` |
| `oneof` | `The field '{f}' must be one of: {p with spaces→commas}.` |
| `mongoid` | `The field '{f}' must be a valid MongoDB ObjectID (24 hex chars).` |
| anything else | `The field '{f}' is invalid ({tag}[={param}]).` |

The min/max branch picks the "characters" wording when the field's reflect kind is `String`, `Slice`, `Map`, or `Array`; otherwise it uses the numeric wording.

## Custom validators (`customvalidation/`)

```go
func RegisterMongoID(v *validator.Validate)  // registers tag "mongoid"
func RegisterUUID(v *validator.Validate)     // registers tag "uuid"
```

| Tag | Rule |
|---|---|
| `mongoid` | Matches `^[a-fA-F0-9]{24}$`. |
| `uuid` | Parsed via `google/uuid.Parse`. |

Both register-functions ignore the `RegisterValidation` error — the assumption is that you call them once via `New()`.

## Usage

```go
type CreateUserDTO struct {
    Email    string `json:"email"     validate:"required,email"`
    Password string `json:"password"  validate:"required,min=8"`
    OrgID    string `json:"orgId"     validate:"required,mongoid"`
    Role     string `json:"role"      validate:"oneof=admin user viewer"`
}

if errs := validations.ValidateStruct(dto); errs != nil {
    return c.Status(400).JSON(map[string]any{"errors": errs})
}
```

## Notes

- `ValidationError` (the struct) is **defined** in `types.go` but is not currently produced by any function in this package. The public output of `ValidateStruct(Ctx)` is `[]string`. Treat the struct as forward-compat for callers that want a richer shape.
- `ValidationErrors` (the slice) implements `error` so callers can return it as an error value, but `.Error()` always returns the literal `"validation failed"` — log or surface the slice contents separately.
