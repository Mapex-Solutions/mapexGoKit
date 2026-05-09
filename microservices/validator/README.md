# validator — Generic DTO unmarshal + defaults + validate + transform pipeline

Reusable validation utility for any DTO, regardless of source (NATS messages, files, raw bytes). Mirrors the pattern in `http/requestValidation` but is decoupled from HTTP — you bring the bytes, you get a populated and validated struct.

> Package name: `validator` (directory: `validator/`).

## Pipeline

```
JSON bytes ──▶ Unmarshal ──▶ Apply defaults ──▶ Validate ──▶ Transform (deep) ──▶ ✅
                  │              │                │              │
                  │              │                │              └─ DTOTransformer
                  │              │                └─ utils/validations.ValidateStruct
                  │              └─ creasty/defaults — `default:"…"` struct tags
                  └─ encoding/json
```

Every step that fails wraps its error with `failed to unmarshal JSON`, `failed to apply defaults`, `validation failed`, or `transform failed` (always via `fmt.Errorf("%w: …")` shape) so callers can pinpoint the failing stage from the error string.

## Surface

```go
type DTOTransformer interface { Transform() error }

func UnmarshalAndValidate(data []byte, dto interface{}) error
func Validate(dto interface{}) error
```

| Function | When to use |
|---|---|
| `UnmarshalAndValidate(data, dto)` | You have raw JSON bytes — runs the full 4-step pipeline. |
| `Validate(dto)` | You already populated the DTO yourself — runs steps 2–4 (defaults → validate → transform). |

Both reject `dto == nil` or non-pointer-to-struct values up front (`dto must be a pointer to a struct, got <kind>`).

## DTO contract

### Field tags consumed

| Tag | Source | Effect |
|---|---|---|
| `json:"…"` | encoding/json | Field name during unmarshal |
| `default:"…"` | [`creasty/defaults`](https://github.com/creasty/defaults) | Value applied when field is zero-valued after unmarshal |
| `validate:"…"` | `utils/validations.ValidateStruct` (go-playground/validator wrapped) | Validation rule(s) — `required`, `email`, `min`, etc. |

### Optional `Transform()` hook

A DTO can implement `DTOTransformer` to mutate itself after validation:

```go
type EmailDTO struct {
    Value string `json:"value" validate:"required"`
}

func (d *EmailDTO) Transform() error {
    d.Value = strings.ToLower(strings.TrimSpace(d.Value))
    return nil
}
```

Transform runs **after** validation succeeded — use it for normalisation, not validation.

### Deep transform traversal (`runTransformsDeep`)

The walker is recursive and visits **every** struct/slice/array/map element in post-order. If any nested value implements `DTOTransformer`, its `Transform()` is invoked. The traversal:

- Unwraps pointers and interfaces (skips on `nil`).
- Recurses into struct fields, slice/array elements, and map values (map keys are not visited).
- Skips unexported fields (cannot be reached via reflection).
- Calls `Transform()` on the addressable form first, falling back to the value form when the value cannot be addressed.

Implication: a parent's `Transform()` runs **after** its children's. Build invariants on that order.

## Usage

### From raw bytes (NATS, file, etc.)

```go
type CreateAssetDTO struct {
    Name  string `json:"name"  validate:"required"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age"   default:"18"`
}

dto := &CreateAssetDTO{}
if err := validator.UnmarshalAndValidate(payload, dto); err != nil {
    return err
}
// dto is populated, defaulted, validated, and (if applicable) transformed.
```

### When you already have the struct

```go
dto := &CreateAssetDTO{Name: req.Name, Email: req.Email}
if err := validator.Validate(dto); err != nil {
    return err
}
// defaults applied, validation passed, transforms ran.
```

## Tested behaviours (`validator_test.go`)

- `validateDTOType` rejects: `nil`, non-pointer values, pointer-to-non-struct.
- `UnmarshalAndValidate`:
  - happy path populates all fields,
  - invalid JSON returns an unmarshal error,
  - missing `required` field returns a validation error,
  - `default:"18"` is applied when the JSON omits the field,
  - `Transform()` runs and mutates the DTO after validation.
