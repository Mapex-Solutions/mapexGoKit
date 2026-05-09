# orgfilter — Multi-tenant org filters and pathKey-based scoping

Builds the MongoDB / ClickHouse filter that scopes a query to the orgs a user can see, based on the `RequestContext` produced by the coverage middleware. Also classifies orgs by hierarchy depth and contains the template-creation rule that "only vendors and customers can create templates".

> Package name: `orgfilter` (directory: `orgfilter/`).

## Org type classification (`org_type.go`)

```go
type OrgType string
const (
    OrgTypeVendor   = "vendor"   // depth 1: "000001/"
    OrgTypeCustomer = "customer" // depth 2: "000001/0001/"
    OrgTypeSite     = "site"     // depth 3: "000001/0001/0003/"
    OrgTypeOther    = "other"    // depth 4+ (building/floor/zone)
    OrgTypeUnknown  = "unknown"  // empty / invalid
)

func GetOrgTypeFromPathKey(pathKey string) OrgType
func CanCreateTemplate(orgType OrgType) bool
func ValidateTemplateCreation(pathKey string) error
```

`GetOrgTypeFromPathKey` strips a trailing `/`, splits on `/`, and uses the segment count.

`CanCreateTemplate` returns `true` for `vendor` and `customer` only — sites/buildings/floors/zones can only create local resources. `ValidateTemplateCreation` is the convenience wrapper that returns the error string `"only vendors and customers can create templates"` when violated.

## Filter builders

The pair `BuildOrgFilter` (MongoDB) and `BuildOrgFilterClickHouse` (string-keyed) share the same algorithm but produce different value shapes.

### `BuildFilterParams`

```go
type BuildFilterParams struct {
    ReqContext *ctx.RequestContext
    Query      interface{ GetIncludeChildren() bool } // any DTO that exposes the flag
}
```

`Query` is typically a list-DTO that embeds a base type with `GetIncludeChildren()`. Pass `nil` if your endpoint never includes children.

### Three modes

| # | Trigger | Resulting filter |
|---:|---|---|
| 1 | `OrgContext != nil` **and** `IncludeChildren == true` | `pathKey: { $gte: <pk>, $lt: <nextSibling(pk)> }` — the org **and** all descendants. Requires `OrgContextData.PathKey` (else error `"org context data or pathKey missing"`). |
| 2 | `OrgContext != nil` (children off) | `orgId: <ObjectID>` (Mongo) or `orgId: <string>` (ClickHouse) — exactly that org. |
| 3 | No `OrgContext` and `len(ScopedOrgIds) > 0` | `orgId: { $in: [...] }` (Mongo: `[]ObjectId`; ClickHouse: `[]string`). |
| 4 | No `OrgContext` and `ScopedOrgIds` empty | Empty filter `{}` — super-admin scope. |

In Mongo mode, every Mongo `orgId` value is built via `model.ToObjectID(...)`; invalid IDs in `ScopedOrgIds` are silently skipped, but if **all** are invalid the call returns `"no valid organization IDs in scope"`.

### `CalculateNextSiblingPathKey`

Re-exported here for callers that already import `orgfilter`. Delegates to `utils/pathkey.CalculateNextSiblingPathKey`.

## Validation helpers

```go
func ValidateOrgContext(orgContext string, scopedOrgIds []string) bool
func FindOrgInCoverage(orgId string, coverageOrgs []ctx.CoverageOrg) *ctx.CoverageOrg
func ValidateOrgContextForNonSystem(reqContext *ctx.RequestContext) error
```

| Helper | Purpose |
|---|---|
| `ValidateOrgContext` | Returns `true` when `orgContext` is empty (no scope intent) or matches one of `scopedOrgIds`. Used by the coverage middleware before injecting context. |
| `FindOrgInCoverage` | Looks up an org by ID inside the user's coverage list. Returns `nil` when not found. |
| `ValidateOrgContextForNonSystem` | For create/update of non-system templates and local resources: requires `OrgContext != nil` AND `OrgContextData.PathKey` set, otherwise returns one of the two error strings (`"org context required for non-system resources"` / `"org pathKey required for non-system resources"`). |

## Projection helper

```go
func BuildProjection(projectionStr *string) map[string]interface{}
```

Splits a CSV string into a Mongo projection. `nil` or empty → `nil`. Whitespace trimmed; empty entries skipped:

```
"name, type, status" → { "name": 1, "type": 1, "status": 1 }
```

## Template ancestor filter

```go
func GetAncestorPathKeysIncludingSelf(pathKey string) []string
func BuildTemplateAncestorFilter(reqContext *ctx.RequestContext) (map[string]interface{}, error)
```

`GetAncestorPathKeysIncludingSelf` is similar to `pathkey.GetAncestorPaths` but **includes** the current pathKey:

```
"000001/0001/0003" → ["000001", "000001/0001", "000001/0001/0003"]
```

`BuildTemplateAncestorFilter` produces the Mongo filter for "templates the current user is allowed to see":

| Current org type | Filter shape |
|---|---|
| `vendor` | `{ isTemplate: true, pathKey: "<own pathKey>" }` — only its own templates. |
| `customer` | `{ isTemplate: true, pathKey: { $in: [vendor, customer] } }` — vendor and own templates. |
| `site` / `other` (building/floor/zone) | `{ isTemplate: true, pathKey: { $in: [vendor, customer] } }` — site cannot create templates, so it sees only the inherited ones from vendor and customer. |

Errors: `"org context required to filter templates"`, `"no vendor or customer ancestors found"`, `"invalid organization type"`.

## Notes

- `BuildOrgFilter` (Mongo variant) returns `map[string]interface{}` ready to merge into the existing `filters` map. Be aware that for **case 1** the key is `pathKey`, not `orgId` — overwriting `filters["pathKey"]` if you also set it elsewhere.
- The ClickHouse variant skips ObjectID conversion entirely; org IDs are stored as strings in CH.
- Super-admin (case 4) does not produce filters of any kind — combine with whatever non-org filters your endpoint requires.
