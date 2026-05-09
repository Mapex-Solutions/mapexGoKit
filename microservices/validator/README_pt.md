# validator — Pipeline genérica de DTO: unmarshal + defaults + validação + transform

Utilitário reutilizável de validação para qualquer DTO, independente da origem (mensagens NATS, arquivos, bytes crus). Espelha o padrão de `http/requestValidation` mas é desacoplado de HTTP — você traz os bytes, recebe uma struct populada e validada.

> Nome do pacote: `validator` (diretório: `validator/`).

## Pipeline

```
JSON bytes ──▶ Unmarshal ──▶ Aplica defaults ──▶ Valida ──▶ Transform (deep) ──▶ ✅
                  │              │                │            │
                  │              │                │            └─ DTOTransformer
                  │              │                └─ utils/validations.ValidateStruct
                  │              └─ creasty/defaults — tags `default:"…"`
                  └─ encoding/json
```

Cada passo que falha embrulha o erro com `failed to unmarshal JSON`, `failed to apply defaults`, `validation failed` ou `transform failed` (sempre no formato `fmt.Errorf("%w: …")`) para que callers identifiquem a etapa que falhou pela string.

## Superfície

```go
type DTOTransformer interface { Transform() error }

func UnmarshalAndValidate(data []byte, dto interface{}) error
func Validate(dto interface{}) error
```

| Função | Quando usar |
|---|---|
| `UnmarshalAndValidate(data, dto)` | Você tem JSON cru — roda os 4 passos completos. |
| `Validate(dto)` | Você já populou o DTO — roda passos 2–4 (defaults → validate → transform). |

Ambas rejeitam `dto == nil` ou valores que não são ponteiro-para-struct logo no início (`dto must be a pointer to a struct, got <kind>`).

## Contrato do DTO

### Tags de field consumidas

| Tag | Fonte | Efeito |
|---|---|---|
| `json:"…"` | encoding/json | Nome do field durante unmarshal |
| `default:"…"` | [`creasty/defaults`](https://github.com/creasty/defaults) | Valor aplicado quando o field está zero-valued após unmarshal |
| `validate:"…"` | `utils/validations.ValidateStruct` (wrapper de go-playground/validator) | Regra(s) de validação — `required`, `email`, `min`, etc. |

### Hook opcional `Transform()`

Um DTO pode implementar `DTOTransformer` para mutar a si próprio após a validação:

```go
type EmailDTO struct {
    Value string `json:"value" validate:"required"`
}

func (d *EmailDTO) Transform() error {
    d.Value = strings.ToLower(strings.TrimSpace(d.Value))
    return nil
}
```

Transform roda **depois** que a validação passou — use para normalização, não para validação.

### Travessia deep do transform (`runTransformsDeep`)

O walker é recursivo e visita **todo** elemento de struct/slice/array/map em pós-ordem. Se qualquer valor aninhado implementar `DTOTransformer`, seu `Transform()` é invocado. A travessia:

- Desempacota ponteiros e interfaces (pula em `nil`).
- Recursa em fields de struct, elementos de slice/array e valores de map (chaves de map não são visitadas).
- Pula fields unexported (inacessíveis via reflection).
- Chama `Transform()` na forma addressable primeiro, com fallback para a forma valor quando não é endereçável.

Implicação: o `Transform()` do pai roda **depois** dos filhos. Construa invariantes com base nessa ordem.

## Uso

### A partir de bytes (NATS, arquivo, etc.)

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
// dto populado, com defaults, validado e (se aplicável) transformado.
```

### Quando você já tem a struct

```go
dto := &CreateAssetDTO{Name: req.Name, Email: req.Email}
if err := validator.Validate(dto); err != nil {
    return err
}
// defaults aplicados, validação ok, transforms rodaram.
```

## Comportamentos testados (`validator_test.go`)

- `validateDTOType` rejeita: `nil`, valores que não são ponteiro, ponteiro-para-não-struct.
- `UnmarshalAndValidate`:
  - caminho feliz popula todos os fields,
  - JSON inválido retorna erro de unmarshal,
  - field `required` ausente retorna erro de validação,
  - `default:"18"` é aplicado quando o JSON omite o field,
  - `Transform()` roda e muta o DTO após a validação.
