# mapper — Generic DTO ↔ Entity copy with ObjectId conversions

Generic deep-copy helpers for the DTO ↔ Entity boundary. Built on [`jinzhu/copier`](https://github.com/jinzhu/copier) with optional Mapex-specific converters that turn `bson.ObjectID` into `string` (and back) — the conversion that always trips `copier` because it crosses unrelated named types.

> Package name: `mapper` (directory: `mapper/`).

## Surface

```go
type MapperOptions struct {
    ObjectIdToString bool // Entity → DTO direction
    StringToObjectId bool // DTO → Entity direction
}

// Plain copy (no ID conversion, no Created/Updated handling beyond DTO direction)
func DtoToEntity[D any, E any](dto *D)       (*E, error)
func EntityToDto[E any, D any](entity *E)    (*D, error)

// With options (ObjectId conversion + Created/Updated handling on Entity→DTO)
func DtoToEntityWithOptions[D any, E any](dto *D, opts MapperOptions)       (*E, error)
func EntityToDtoWithOptions[E any, D any](entity *E, opts MapperOptions)    (*D, error)

// Map round-trips
func DtoToMap[D any](dto *D)                              (map[string]interface{}, error)
func MapToStruct[T any](data map[string]interface{})      (*T, error)
func MapToStructFromAny[T any](data interface{})          (*T, error)
```

`DtoToEntity` / `EntityToDto` are thin wrappers around `copier.CopyWithOption(..., DeepCopy: true)`. The `WithOptions` variants add two pieces of post-processing that `copier` cannot do alone.

## ObjectId conversions (the core value)

`copier` cannot translate between two unrelated named types like `bson.ObjectID` and `string` automatically. The `WithOptions` helpers register `copier.TypeConverter`s for the **direct** field cases and use a reflective post-pass for the **container** cases.

### Direct conversions (handled by `copier.TypeConverter`)

| Direction | Source → Destination |
|---|---|
| Entity → DTO (`ObjectIdToString`) | `model.ObjectId` → `string` (zero ObjectID → `""`) |
| Entity → DTO (`ObjectIdToString`) | `model.ObjectId` → `*string` (zero → `nil`) |
| DTO → Entity (`StringToObjectId`) | `string` → `model.ObjectId` (empty/invalid → zero ObjectID) |
| DTO → Entity (`StringToObjectId`) | `*string` → `model.ObjectId` (nil/empty/invalid → zero ObjectID) |

### Container conversions (post-processed via reflection)

| Direction | Source → Destination |
|---|---|
| Entity → DTO | `[]model.ObjectId` → `[]string` |
| Entity → DTO | `[]model.ObjectId` → `*[]string` |
| Entity → DTO | `*model.ObjectId` → `*string` |
| Entity → DTO | `[]string` → `*[]string` (handy for nullable lists) |
| DTO → Entity | `[]string` → `[]model.ObjectId` |
| DTO → Entity | `*[]string` → `[]model.ObjectId` |

Empty/nil source slices and zero ObjectIDs leave the destination untouched. Invalid ObjectId strings are **silently skipped** in slice conversions and become zero ObjectIDs in scalar conversions — there is no error path for "I gave you garbage".

The `isObjectIdSlice`/`isObjectIdPtr` checks accept both `primitive.ObjectID` (mongo-driver v1) and `bson.ObjectID` (v2) by inspecting the type name string.

## `Created` / `Updated` interface taps (Entity → DTO only)

When the Entity exposes either of:

```go
GetCreated() time.Time
GetUpdated() time.Time
```

…and the DTO implements either:

```go
SetCreated(*timeUtil.NullTime)
SetUpdated(*timeUtil.NullTime)
```

…the helpers wrap the non-zero values into `*timeUtil.NullTime` and assign them. The two interfaces are checked **independently** (a DTO can implement just one, or both). Zero `time.Time` values are skipped — your DTO will see `nil`, which `timeUtil.NullTime` serialises as JSON `null`.

The `WithOptions` variants run this same logic; the plain `EntityToDto` also does it (independent of any options).

## Map round-trips

```go
func DtoToMap[D any](dto *D)                          (map[string]interface{}, error)
func MapToStruct[T any](data map[string]interface{})  (*T, error)
func MapToStructFromAny[T any](data interface{})      (*T, error)
```

All three round-trip through `serialize.Marshal` / `serialize.Unmarshal` — that is, JSON. Useful when your handler receives an opaque `interface{}` (e.g. a generic API result) and needs a typed struct, or vice versa. **Time fidelity, custom marshalers, and unrepresentable JSON values lose information** here, just like any JSON round-trip.

## Usage

### DTO ← Entity (typical Read flow)

```go
dto, err := mapper.EntityToDtoWithOptions[UserEntity, UserDTO](&entity, mapper.MapperOptions{
    ObjectIdToString: true,
})
// dto.ID, dto.Created, dto.Updated populated; ObjectIDs already turned into strings.
```

### DTO → Entity (typical Create/Update flow)

```go
entity, err := mapper.DtoToEntityWithOptions[CreateUserDTO, UserEntity](dto, mapper.MapperOptions{
    StringToObjectId: true,
})
// entity.OrgId, entity.AssigneeIds, etc. parsed from string fields.
```

### Plain copy (no ID/timestamp magic)

```go
dst, err := mapper.DtoToEntity[FilterDTO, FilterEntity](&filterDTO)
```

## Notes

- All helpers use `copier.Option{DeepCopy: true}` so nested pointers and slices are independent of the source.
- `EntityToDto` (no options) **does** apply the Created/Updated tap. If you want a pure copy with zero post-processing, build with `copier.CopyWithOption` directly.
- The reflective post-pass walks struct fields by name — fields renamed between Entity and DTO are not bridged. Keep names in sync, or implement a manual mapper for the diverging fields.
- Invalid ObjectIds are silently dropped in slice conversions. If you need strict validation, validate before calling the mapper.
