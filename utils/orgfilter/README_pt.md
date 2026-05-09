# orgfilter — Filtros multi-tenant de org e escopo por pathKey

Constrói o filtro MongoDB / ClickHouse que faz o escopo de uma query para as orgs que um usuário pode ver, baseado no `RequestContext` produzido pelo middleware de coverage. Também classifica orgs por profundidade hierárquica e contém a regra de criação de template ("apenas vendors e customers podem criar templates").

> Nome do pacote: `orgfilter` (diretório: `orgfilter/`).

## Classificação de tipo de org (`org_type.go`)

```go
type OrgType string
const (
    OrgTypeVendor   = "vendor"   // profundidade 1: "000001/"
    OrgTypeCustomer = "customer" // profundidade 2: "000001/0001/"
    OrgTypeSite     = "site"     // profundidade 3: "000001/0001/0003/"
    OrgTypeOther    = "other"    // profundidade 4+ (building/floor/zone)
    OrgTypeUnknown  = "unknown"  // vazio / inválido
)

func GetOrgTypeFromPathKey(pathKey string) OrgType
func CanCreateTemplate(orgType OrgType) bool
func ValidateTemplateCreation(pathKey string) error
```

`GetOrgTypeFromPathKey` remove a `/` final, faz split em `/` e usa a contagem de segmentos.

`CanCreateTemplate` retorna `true` apenas para `vendor` e `customer` — sites/buildings/floors/zones só podem criar recursos locais. `ValidateTemplateCreation` é o wrapper de conveniência que retorna a string de erro `"only vendors and customers can create templates"` quando violado.

## Builders de filtro

O par `BuildOrgFilter` (MongoDB) e `BuildOrgFilterClickHouse` (chaves string) compartilham o mesmo algoritmo mas produzem formatos de valor diferentes.

### `BuildFilterParams`

```go
type BuildFilterParams struct {
    ReqContext *ctx.RequestContext
    Query      interface{ GetIncludeChildren() bool } // qualquer DTO que exponha a flag
}
```

`Query` é tipicamente um DTO de listagem que embeda um tipo base com `GetIncludeChildren()`. Passe `nil` se seu endpoint nunca inclui filhos.

### Três modos

| # | Gatilho | Filtro resultante |
|---:|---|---|
| 1 | `OrgContext != nil` **e** `IncludeChildren == true` | `pathKey: { $gte: <pk>, $lt: <nextSibling(pk)> }` — a org **e** todos os descendentes. Exige `OrgContextData.PathKey` (caso contrário erro `"org context data or pathKey missing"`). |
| 2 | `OrgContext != nil` (children off) | `orgId: <ObjectID>` (Mongo) ou `orgId: <string>` (ClickHouse) — apenas aquela org. |
| 3 | Sem `OrgContext` e `len(ScopedOrgIds) > 0` | `orgId: { $in: [...] }` (Mongo: `[]ObjectId`; ClickHouse: `[]string`). |
| 4 | Sem `OrgContext` e `ScopedOrgIds` vazio | Filtro vazio `{}` — escopo de super-admin. |

No modo Mongo, todo valor `orgId` é construído via `model.ToObjectID(...)`; IDs inválidos em `ScopedOrgIds` são silenciosamente pulados, mas se **todos** forem inválidos a chamada retorna `"no valid organization IDs in scope"`.

### `CalculateNextSiblingPathKey`

Re-exportado aqui para callers que já importam `orgfilter`. Delega para `utils/pathkey.CalculateNextSiblingPathKey`.

## Helpers de validação

```go
func ValidateOrgContext(orgContext string, scopedOrgIds []string) bool
func FindOrgInCoverage(orgId string, coverageOrgs []ctx.CoverageOrg) *ctx.CoverageOrg
func ValidateOrgContextForNonSystem(reqContext *ctx.RequestContext) error
```

| Helper | Propósito |
|---|---|
| `ValidateOrgContext` | Retorna `true` quando `orgContext` está vazio (sem intenção de escopo) ou bate com um dos `scopedOrgIds`. Usado pelo middleware de coverage antes de injetar contexto. |
| `FindOrgInCoverage` | Busca uma org por ID na lista de coverage do usuário. Retorna `nil` se não encontrada. |
| `ValidateOrgContextForNonSystem` | Para create/update de templates não-system e recursos locais: exige `OrgContext != nil` E `OrgContextData.PathKey` setado, caso contrário retorna uma das duas strings de erro (`"org context required for non-system resources"` / `"org pathKey required for non-system resources"`). |

## Helper de projection

```go
func BuildProjection(projectionStr *string) map[string]interface{}
```

Divide uma string CSV em projection Mongo. `nil` ou vazio → `nil`. Whitespace é trimado; entradas vazias são puladas:

```
"name, type, status" → { "name": 1, "type": 1, "status": 1 }
```

## Filtro de ancestrais para templates

```go
func GetAncestorPathKeysIncludingSelf(pathKey string) []string
func BuildTemplateAncestorFilter(reqContext *ctx.RequestContext) (map[string]interface{}, error)
```

`GetAncestorPathKeysIncludingSelf` é parecido com `pathkey.GetAncestorPaths` mas **inclui** o pathKey atual:

```
"000001/0001/0003" → ["000001", "000001/0001", "000001/0001/0003"]
```

`BuildTemplateAncestorFilter` produz o filtro Mongo para "templates que o usuário atual pode ver":

| Tipo da org atual | Formato do filtro |
|---|---|
| `vendor` | `{ isTemplate: true, pathKey: "<próprio pathKey>" }` — apenas próprios templates. |
| `customer` | `{ isTemplate: true, pathKey: { $in: [vendor, customer] } }` — templates do vendor e próprios. |
| `site` / `other` (building/floor/zone) | `{ isTemplate: true, pathKey: { $in: [vendor, customer] } }` — site não pode criar templates, então só vê os herdados de vendor e customer. |

Erros: `"org context required to filter templates"`, `"no vendor or customer ancestors found"`, `"invalid organization type"`.

## Notas

- `BuildOrgFilter` (variante Mongo) retorna `map[string]interface{}` pronto para mesclar no map `filters` existente. Atenção: para o **caso 1** a chave é `pathKey`, não `orgId` — sobrescrevendo `filters["pathKey"]` se você setar em outro lugar.
- A variante ClickHouse pula totalmente a conversão para ObjectID; org IDs ficam como string no CH.
- Super-admin (caso 4) não produz filtros de tipo algum — combine com os filtros não-org que seu endpoint exigir.
