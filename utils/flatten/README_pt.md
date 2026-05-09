# flatten — Tradução map aninhado ↔ chaves planas + lookup por path

Duas preocupações cooperantes:

1. **Resolução de path** — `GetValueByPath(data, "sensors.temp")` percorre um `map[string]any` aninhado, fazendo fan-out por intermediários do tipo array.
2. **Achatamento** — `Flatten(...)` / `FlattenString(...)` colapsam uma estrutura arbitrariamente aninhada em um map de um nível chaveado por paths pontilhados (ou outro estilo).

Ambos trabalham em valores no formato JSON: `map[string]any`, `[]any`, escalares. Não há caminho via struct/reflection.

> Nome do pacote: `flatten` (diretório: `flatten/`).

## Estilos de separador

```go
type SeparatorStyle struct { Before, Middle, After string }

var (
    DotStyle        = SeparatorStyle{Middle: "."}        // a.b.c
    PathStyle       = SeparatorStyle{Middle: "/"}        // a/b/c
    RailsStyle      = SeparatorStyle{Before: "[", After: "]"} // a[b][c]
    UnderscoreStyle = SeparatorStyle{Middle: "_"}        // a_b_c
)
```

Estilos custom são só três strings — qualquer coisa expressável como `prefix Before Middle key After` serve.

## Erros

```go
var ErrNotValidInput     = errors.New("flatten: input must be map[string]any or []any")
var ErrNotValidJSONInput = errors.New("flatten: JSON input must start with an object")
var ErrNilData           = errors.New("flatten: nil data")
var ErrEmptyPath         = errors.New("flatten: empty path")
var ErrPathNotFound      = errors.New("flatten: path not found")
```

## `GetValueByPath` — Lookup por path com fan-out em arrays

```go
func GetValueByPath(data map[string]any, path string) (value any, multi bool, err error)
```

Percorre `data` segmento por segmento (split por `.`). Quando qualquer intermediário é `[]any`, o walker **faz fan-out** — continua percorrendo com os segmentos restantes dentro de cada elemento e agrega os resultados. O tipo do resultado é reportado via `multi`:

| Cenário | Retorna |
|---|---|
| Escalar simples | `(value, false, nil)` |
| Path termina em `[]any` | `([]any, true, nil)` |
| Path passa por array de objetos | `([]any{e1.path, e2.path, …}, true, nil)` |
| Path ausente | `(nil, false, ErrPathNotFound)` |
| `data` nil | `(nil, false, ErrNilData)` |
| `path` vazio | `(nil, false, ErrEmptyPath)` |

Exemplos:

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

Em fan-out, elementos onde o sub-path não existe são pulados silenciosamente. Se **todos** falharem, a chamada retorna `ErrPathNotFound` ao invés de slice vazio.

## `Flatten` — colapsa um map aninhado

```go
func Flatten(nested map[string]any, prefix string, style SeparatorStyle) (map[string]any, error)
```

Percorre a estrutura em depth-first. Chaves de map viram componentes de path; índices de slice viram tokens numéricos (`strconv.Itoa(i)`):

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

O parâmetro `prefix` é adicionado **uma vez** (sem separador entre o prefix e a primeira chave). Passe `""` para começar do zero.

`Flatten` só aceita `map[string]any` e `[]any` para valores aninhados — qualquer outro retorna `ErrNotValidInput`.

### `FlattenString` — Conveniência para JSON em string

```go
func FlattenString(nestedJSON, prefix string, style SeparatorStyle) (string, error)
```

Pré-filtro barato de `{` via `looksLikeJSONObject`, depois `json.Unmarshal` → `Flatten` → `json.Marshal`. Inputs que não começam com `{` (depois de trim de whitespace) retornam `ErrNotValidJSONInput`.

## Notas

- O check inicial de `{` é um pré-filtro rápido — a validade JSON real é checada por `json.Unmarshal` em seguida.
- Elementos de slice que são escalares são achatados por índice. Um slice de objetos gera `key.0.field`, `key.1.field`, etc.
- `Flatten` não preserva ordem além da iteração de map em Go; o output é um `map`, não uma lista ordenada.
