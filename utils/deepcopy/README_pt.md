# deepcopy — Deep copy via JSON-roundtrip para `map[string]interface{}`

Dois helpers que fazem deep-copy de `map[string]any` (e a variante map-de-maps) serializando para JSON e voltando. Perde qualquer coisa que JSON não representa (channels, funções, NaN/Inf, precisão de time além do layout JSON, etc.) — use apenas quando os valores forem JSON-friendly.

> Nome do pacote: `deepcopy` (diretório: `deepcopy/`).

## Superfície

```go
func Map(src map[string]interface{}) map[string]interface{}
func MapOfMaps(src map[string]map[string]interface{}) map[string]map[string]interface{}
```

| Função | Comportamento |
|---|---|
| `Map(src)` | `nil` na origem → map vazio. Marshal → Unmarshal. Se o JSON falhar, retorna map vazio (erros silenciados). |
| `MapOfMaps(src)` | `nil` na origem → `nil`. Caso contrário aloca um novo map externo e faz deep-copy de cada map interno via `Map`. |

## Uso

```go
src := map[string]interface{}{
    "user": map[string]interface{}{"name": "Alice"},
    "tags": []string{"a", "b"},
}
cp := deepcopy.Map(src)
cp["user"].(map[string]interface{})["name"] = "Bob"
// src["user"]["name"] continua "Alice" — deep-copy real
```

## Notas

- Ambas funções ignoram erros de marshal/unmarshal. Se precisa detectar falha, faça o marshal por conta própria.
- Números são decodificados como `float64` (comportamento padrão do `encoding/json`).
