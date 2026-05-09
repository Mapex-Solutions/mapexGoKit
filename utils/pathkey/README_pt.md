# pathkey — Utilitários de pathKey hierárquico

Helpers para trabalhar com as strings `pathKey` hierárquicas que o Mapex armazena em todo documento de organização. PathKeys são como a plataforma codifica a árvore de orgs (vendor → customer → site → building → floor → zone) em uma string ordenável e amigável a queries de range.

> Nome do pacote: `pathkey` (diretório: `pathkey/`).

## Formato do pathKey

```
000001/000002/0003
```

- Cada segmento é **Base36** (dígitos + letras maiúsculas), com zero-padding em largura fixa.
- Segmentos separados por `/`.
- A largura varia por tipo de org — vendors/customers usam 6, sites/buildings 4, floors/zones 3 (o pacote em si não força larguras; preserva a largura que o segmento já tem).

## Superfície

```go
func CalculateNextSiblingPathKey(pathKey string) string
func IsDescendant(child, parent string) bool
func IsDescendantOrSelf(child, parent string) bool
func GetAncestorPaths(pathKey string) []string
```

### `CalculateNextSiblingPathKey`

Retorna o pathKey do próximo irmão de `pathKey` incrementando o último segmento em 1 em Base36, preservando a largura via zero-padding. Útil para o limite superior de uma query de range que seleciona "esta org e todos os descendentes":

```
"000001/000001/0001" → "000001/000001/0002"
"000001/00000Z"      → "000001/000010"
"000001/000001/000Z" → "000001/000001/0010"
"" (vazio)           → ""
```

Se o último segmento não puder ser parseado como Base36, o `pathKey` original é retornado.

#### Padrão de range-query (MongoDB)

```go
filters["pathKey"] = bson.M{
    "$gte": org.PathKey,
    "$lt":  pathkey.CalculateNextSiblingPathKey(org.PathKey),
}
```

Seleciona a org **e** todos seus descendentes sem usar regex.

### `IsDescendant` / `IsDescendantOrSelf`

| Função | Verdadeiro quando |
|---|---|
| `IsDescendant(child, parent)` | `child` começa estritamente com `parent` e é **mais longo**. `parent` vazio retorna `true` para qualquer child não-vazio. |
| `IsDescendantOrSelf(child, parent)` | O mesmo, mais pathKeys iguais retornam `true`. |

São checks de prefixo puros — não validam larguras de segmento. Se você mistura larguras entre tipos de org, garanta que os inputs seguem a mesma convenção.

### `GetAncestorPaths`

Retorna todos pathKeys ancestrais de raiz até o pai imediato (**não** inclui o input).

```
""                             → []
"000001"                       → []           // raiz não tem ancestrais
"000001/000002"                → ["000001"]
"000001/000002/0003"           → ["000001", "000001/000002"]
"000001/000002/0003/0004"      → ["000001", "000001/000002", "000001/000002/0003"]
```

#### Padrão: recursos herdáveis

```go
ancestors := pathkey.GetAncestorPaths(currentOrg.PathKey)
db.roles.find({
    $or: []bson.M{
        {"isSystem": true},
        {"pathKey": currentOrg.PathKey},                            // local
        {"pathKey": bson.M{"$in": ancestors}, "scope": "global"},   // herdado
    },
})
```

## Notas

- `pathkey` não valida largura Base36 nem regras de tipo de org — preserva o que você passa. Para inferência de tipo de org, use `utils/orgfilter.GetOrgTypeFromPathKey`.
- O incremento Base36 é **case-insensitive na entrada** (`ParseInt` aceita as duas grafias) mas sempre emite **maiúsculas** no segmento incrementado. Impacto prático: se seus pathKeys armazenados usam minúsculas, espere resultados mistos depois de `CalculateNextSiblingPathKey` — mantenha o resto do código em maiúsculas para evitar surpresas.
