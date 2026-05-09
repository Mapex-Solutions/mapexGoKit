# pathkey — Hierarchical pathKey utilities

Helpers for working with the hierarchical `pathKey` strings Mapex stores on every organization document. PathKeys are how the platform encodes the org tree (vendor → customer → site → building → floor → zone) into a sortable, range-friendly string.

> Package name: `pathkey` (directory: `pathkey/`).

## PathKey shape

```
000001/000002/0003
```

- Each segment is **Base36** (digits + uppercase letters), zero-padded to a fixed width.
- Segments are joined by `/`.
- Width varies by org type — vendors/customers use 6, sites/buildings 4, floors/zones 3 (the package itself does not enforce widths; it preserves whatever width the segment already has).

## Surface

```go
func CalculateNextSiblingPathKey(pathKey string) string
func IsDescendant(child, parent string) bool
func IsDescendantOrSelf(child, parent string) bool
func GetAncestorPaths(pathKey string) []string
```

### `CalculateNextSiblingPathKey`

Returns the next sibling pathKey of `pathKey` by incrementing the last segment by 1 in Base36, preserving the segment width with zero padding. Useful for the upper bound of a range query that selects "this org and all its descendants":

```
"000001/000001/0001" → "000001/000001/0002"
"000001/00000Z"      → "000001/000010"
"000001/000001/000Z" → "000001/000001/0010"
"" (empty)           → ""
```

If the last segment is not parseable as Base36 the original `pathKey` is returned unchanged.

#### Range-query pattern (MongoDB)

```go
filters["pathKey"] = bson.M{
    "$gte": org.PathKey,
    "$lt":  pathkey.CalculateNextSiblingPathKey(org.PathKey),
}
```

This selects the org **and** all its descendants without using regex.

### `IsDescendant` / `IsDescendantOrSelf`

| Function | True when |
|---|---|
| `IsDescendant(child, parent)` | `child` strictly starts with `parent` and is **longer**. Empty parent returns `true` for any non-empty child. |
| `IsDescendantOrSelf(child, parent)` | Same, plus equal pathKeys return `true`. |

These are pure prefix checks — they don't validate segment widths. If you mix widths across org types, ensure the inputs follow the same convention.

### `GetAncestorPaths`

Returns every ancestor pathKey from root to the immediate parent (does **not** include the input itself).

```
""                             → []
"000001"                       → []           // root has no ancestors
"000001/000002"                → ["000001"]
"000001/000002/0003"           → ["000001", "000001/000002"]
"000001/000002/0003/0004"      → ["000001", "000001/000002", "000001/000002/0003"]
```

#### Pattern: inheritable resources

```go
ancestors := pathkey.GetAncestorPaths(currentOrg.PathKey)
db.roles.find({
    $or: []bson.M{
        {"isSystem": true},
        {"pathKey": currentOrg.PathKey},                            // local
        {"pathKey": bson.M{"$in": ancestors}, "scope": "global"},   // inherited
    },
})
```

## Notes

- `pathkey` does not validate Base36 width or org-type rules — it preserves what you give it. For org-type inference, use `utils/orgfilter.GetOrgTypeFromPathKey`.
- The Base36 increment is **case-insensitive on input** (`ParseInt` accepts both cases) but always emits **uppercase** for the incremented segment. Practical impact: if your stored pathKeys use lowercase letters, expect mixed-case results after `CalculateNextSiblingPathKey` — keep the rest of your codebase using uppercase to avoid surprises.
