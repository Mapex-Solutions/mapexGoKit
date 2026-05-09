# serialize — Helpers finos de marshal/unmarshal JSON

Wrapper de duas linhas sobre `encoding/json`. O wrapper existe para que outros pacotes (ex: `redis`, `mapper`) dependam de `serialize.{Marshal,Unmarshal}` e tenham um único lugar para trocar a implementação depois (ex: `bytedance/sonic` ou `easyjson`) sem alterar call sites.

> Nome do pacote: `serialize` (diretório: `serialize/`).

## Superfície

```go
func Marshal(v interface{}) (string, error)
func Unmarshal(data string, v interface{}) error

var ErrMarshal   = errors.New("failed to marshal data")
var ErrUnmarshal = errors.New("failed to unmarshal data")
```

| Função | Comportamento |
|---|---|
| `Marshal(v)` | `json.Marshal(v)` e depois `string(...)`. Em erro retorna `"", ErrMarshal` — o erro subjacente **não** é embrulhado. |
| `Unmarshal(data, v)` | `json.Unmarshal([]byte(data), v)` — retorna o erro original do `encoding/json`. `ErrUnmarshal` está definido mas **não é levantado hoje**. |

## Uso

```go
str, err := serialize.Marshal(map[string]int{"a": 1, "b": 2})
// str = `{"a":1,"b":2}`

var out map[string]int
if err := serialize.Unmarshal(str, &out); err != nil { return err }
```

## Notas

- `Marshal` descarta o erro original do `encoding/json` e retorna o sentinel — use apenas quando o caller não precisa diferenciar `json.UnsupportedTypeError`, `json.UnsupportedValueError`, etc.
- `ErrUnmarshal` é exportado mas não é usado pelo pacote. Definido por simetria; pode ignorar.
