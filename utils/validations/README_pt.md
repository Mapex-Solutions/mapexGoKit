# validations — Singleton de `go-playground/validator` com extras Mapex

Empacota o [`go-playground/validator/v10`](https://github.com/go-playground/validator) em um singleton de escopo de processo com as regras customizadas Mapex pré-registradas (`mongoid`, `uuid`) e um formatador humanizado de erro que retorna `[]string` ao invés de tag-tuples opacos.

> Nome do pacote: `validations` (diretório: `validations/`); regras customizadas em `validations/customvalidation/`.

## Superfície

```go
// Re-exports
type Validate      = validator.Validate
type FieldLevel    = validator.FieldLevel
type CustomFunc    = validator.Func
type CustomFuncCtx = validator.FuncCtx

// Erros
type ValidationError struct {
    Field string `json:"field"`
    Tag   string `json:"tag"`
    Param string `json:"param,omitempty"`
}
type ValidationErrors []string  // implementa error; .Error() retorna "validation failed"

// Singleton
func New() *validator.Validate                              // protegido por sync.Once; registra regras custom
func ValidateStruct(s any) []string
func ValidateStructCtx(ctx context.Context, s any) []string
func ParseTypeError(err error) []string
```

### Singleton

`New()` retorna o mesmo `*validator.Validate` em toda chamada. A primeira chamada registra dois tags customizados via `customvalidation.RegisterMongoID` e `customvalidation.RegisterUUID`. Qualquer coisa adicional deve ser registrada na instância retornada antes da validação rodar concorrentemente.

### `ValidateStruct` / `ValidateStructCtx`

- Retorna `nil` em sucesso.
- Retorna `[]string` com **uma mensagem humanizada por campo falho** quando a validação falha — ver catálogo abaixo. Erros que não são `validator.ValidationErrors` são embrulhados em slice de um elemento com `err.Error()`.

`ValidateStructCtx` é o mesmo com contexto (cancelamento/timeout flui para validators struct-level).

### `ParseTypeError`

Mensagem mais amigável para `*json.UnmarshalTypeError`. Útil quando um JSON tem o tipo errado num campo:

```go
errs := validations.ParseTypeError(err)
// ex: ["field changePasswordNextLogin expected bool but came string"]
```

Fallback para `["erro de parsing: <err>"]` quando `err` não é `UnmarshalTypeError`.

## Catálogo de mensagens (`humanizeFieldError`)

Tags reconhecidos e mensagens geradas:

| Tag | Template (`{field}` / `{param}`) |
|---|---|
| `required` | `The field '{f}' is required.` |
| `eqfield` / `nefield` | `The field '{f}' must be equal to / different from the field '{p}'.` |
| `len` | `The field '{f}' must be exactly {p} characters long.` |
| `min` / `max` (string/slice/map/array) | `The field '{f}' must have at least/at most {p} characters.` |
| `min` / `max` (numérico) | `The field '{f}' must be at least/at most {p}.` |
| `gt` / `gte` / `lt` / `lte` | `The field '{f}' must be greater/less than (or equal to) {p}.` |
| `email` / `alphanum` / `alpha` / `numeric` / `boolean` / `url` / `uuid` / `ipv4` / `ipv6` | `The field '{f}' must be a valid …` |
| `datetime` | Com param: `… must match the date/time format '{p}'.` Sem: `… must be a valid date/time.` |
| `oneof` | `The field '{f}' must be one of: {p com espaços→vírgulas}.` |
| `mongoid` | `The field '{f}' must be a valid MongoDB ObjectID (24 hex chars).` |
| qualquer outro | `The field '{f}' is invalid ({tag}[={param}]).` |

A lógica de min/max escolhe a frase com "characters" quando o reflect kind do campo é `String`, `Slice`, `Map` ou `Array`; caso contrário usa frase numérica.

## Validators customizados (`customvalidation/`)

```go
func RegisterMongoID(v *validator.Validate)  // registra tag "mongoid"
func RegisterUUID(v *validator.Validate)     // registra tag "uuid"
```

| Tag | Regra |
|---|---|
| `mongoid` | Casa `^[a-fA-F0-9]{24}$`. |
| `uuid` | Parse via `google/uuid.Parse`. |

Ambas as funções de registro ignoram o erro de `RegisterValidation` — a premissa é chamá-las uma vez via `New()`.

## Uso

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

## Notas

- `ValidationError` (a struct) está **definida** em `types.go` mas não é produzida por nenhuma função do pacote hoje. O output público de `ValidateStruct(Ctx)` é `[]string`. Trate a struct como forward-compat para callers que queiram um formato mais rico.
- `ValidationErrors` (o slice) implementa `error` para callers retornarem como valor de erro, mas `.Error()` sempre retorna o literal `"validation failed"` — logue ou exponha os conteúdos do slice separadamente.
