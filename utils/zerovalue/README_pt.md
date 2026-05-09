# zerovalue — Geradores de valor zero e um helper `Ptr` genérico

Duas preocupações distintas no mesmo pacote — ambas respondem à mesma pergunta: "me dê o valor canônico para um campo cujo tipo o caller não consegue inlinear."

> Nome do pacote: `zerovalue` (diretório: `zerovalue/`).

## Superfície

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

### `GetZeroValue` / `GetZeroValueTyped` — Valores zero por chave string

Para schemas dinâmicos (pense em `Rule.LocalState` ou defaults de campo de formulário) onde o tipo é descrito como string:

| Input | Output |
|---|---|
| `"number"` | `0` (`int`) |
| `"string"` | `""` |
| `"boolean"` | `false` |
| `"array"` | `[]interface{}{}` |
| `"object"` | `map[string]interface{}{}` |
| qualquer outro | `nil` |

`GetZeroValueTyped` é a mesma função com o wrapper tipado para callers que já têm um `FieldType`.

### `IsValidType`

Retorna `true` para qualquer um dos 5 constantes conhecidos, `false` para tudo mais. Use antes de persistir uma string de tipo informada pelo usuário.

### `Ptr[T any](v T) *T`

Criador de ponteiro inline. Permite construir payloads com fields ponteiro opcionais sem variável descartável em cada call site:

```go
dto := AssetCreate{
    ThresholdMinutes: zerovalue.Ptr(10),
    Enabled:          zerovalue.Ptr(true),
    Tags:             zerovalue.Ptr([]string{"a", "b"}),
}
```

O helper vive aqui porque tanto `Ptr(v)` quanto `GetZeroValue(...)` respondem à mesma família de pergunta — "me dê o valor canônico para um campo cujo tipo o caller não consegue inlinear."

## Notas

- `GetZeroValue("number")` retorna um `int` (`0`), **não um `float64`**. Se sua camada de schema dinâmico usa `float64` em todo lugar (ex: dados decodificados de JSON), encapsule o resultado.
- O zero de "array" é `[]interface{}{}`, não `nil` — camadas de persistência que distinguem "ausente" de "vazio" devem interpretar isto como "vazio".
