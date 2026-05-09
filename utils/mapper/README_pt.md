# mapper — Cópia genérica DTO ↔ Entity com conversões de ObjectId

Helpers genéricos de deep-copy para a fronteira DTO ↔ Entity. Construído sobre [`jinzhu/copier`](https://github.com/jinzhu/copier) com conversores opcionais específicos do Mapex que transformam `bson.ObjectID` em `string` (e vice-versa) — a conversão que sempre quebra o `copier` porque cruza tipos nomeados não relacionados.

> Nome do pacote: `mapper` (diretório: `mapper/`).

## Superfície

```go
type MapperOptions struct {
    ObjectIdToString bool // direção Entity → DTO
    StringToObjectId bool // direção DTO → Entity
}

// Cópia simples (sem conversão de ID, sem tratamento de Created/Updated além da direção DTO)
func DtoToEntity[D any, E any](dto *D)       (*E, error)
func EntityToDto[E any, D any](entity *E)    (*D, error)

// Com opções (conversão de ObjectId + tratamento de Created/Updated em Entity→DTO)
func DtoToEntityWithOptions[D any, E any](dto *D, opts MapperOptions)       (*E, error)
func EntityToDtoWithOptions[E any, D any](entity *E, opts MapperOptions)    (*D, error)

// Round-trips via Map
func DtoToMap[D any](dto *D)                              (map[string]interface{}, error)
func MapToStruct[T any](data map[string]interface{})      (*T, error)
func MapToStructFromAny[T any](data interface{})          (*T, error)
```

`DtoToEntity` / `EntityToDto` são wrappers finos sobre `copier.CopyWithOption(..., DeepCopy: true)`. As variantes `WithOptions` adicionam duas etapas de pós-processamento que o `copier` não consegue fazer sozinho.

## Conversões de ObjectId (o valor central)

`copier` não consegue traduzir automaticamente entre dois tipos nomeados não relacionados como `bson.ObjectID` e `string`. Os helpers `WithOptions` registram `copier.TypeConverter`s para os casos de field **direto** e usam um pós-passo via reflection para os casos de **container**.

### Conversões diretas (tratadas por `copier.TypeConverter`)

| Direção | Origem → Destino |
|---|---|
| Entity → DTO (`ObjectIdToString`) | `model.ObjectId` → `string` (ObjectID zero → `""`) |
| Entity → DTO (`ObjectIdToString`) | `model.ObjectId` → `*string` (zero → `nil`) |
| DTO → Entity (`StringToObjectId`) | `string` → `model.ObjectId` (vazio/inválido → ObjectID zero) |
| DTO → Entity (`StringToObjectId`) | `*string` → `model.ObjectId` (nil/vazio/inválido → ObjectID zero) |

### Conversões de container (pós-processadas via reflection)

| Direção | Origem → Destino |
|---|---|
| Entity → DTO | `[]model.ObjectId` → `[]string` |
| Entity → DTO | `[]model.ObjectId` → `*[]string` |
| Entity → DTO | `*model.ObjectId` → `*string` |
| Entity → DTO | `[]string` → `*[]string` (útil para listas anuláveis) |
| DTO → Entity | `[]string` → `[]model.ObjectId` |
| DTO → Entity | `*[]string` → `[]model.ObjectId` |

Slices vazios/nil na origem e ObjectIDs zero deixam o destino intacto. Strings ObjectId inválidas são **silenciosamente puladas** nas conversões de slice e viram ObjectIDs zero nas conversões escalares — não há caminho de erro para "passei lixo".

Os checks `isObjectIdSlice`/`isObjectIdPtr` aceitam tanto `primitive.ObjectID` (mongo-driver v1) quanto `bson.ObjectID` (v2) inspecionando a string do nome do tipo.

## Tap de interface `Created` / `Updated` (apenas Entity → DTO)

Quando a Entity expõe qualquer um:

```go
GetCreated() time.Time
GetUpdated() time.Time
```

…e o DTO implementa qualquer um:

```go
SetCreated(*timeUtil.NullTime)
SetUpdated(*timeUtil.NullTime)
```

…os helpers embrulham os valores não-zero em `*timeUtil.NullTime` e atribuem. As duas interfaces são checadas **independentemente** (um DTO pode implementar só uma, ou as duas). Valores `time.Time` zero são pulados — seu DTO vê `nil`, que `timeUtil.NullTime` serializa como JSON `null`.

As variantes `WithOptions` rodam essa mesma lógica; o `EntityToDto` simples também faz (independente de opções).

## Round-trips por Map

```go
func DtoToMap[D any](dto *D)                          (map[string]interface{}, error)
func MapToStruct[T any](data map[string]interface{})  (*T, error)
func MapToStructFromAny[T any](data interface{})      (*T, error)
```

Os três fazem round-trip via `serialize.Marshal` / `serialize.Unmarshal` — ou seja, JSON. Útil quando seu handler recebe um `interface{}` opaco (ex: resultado genérico de API) e precisa de struct tipada, ou vice-versa. **Fidelidade de time, marshalers customizados e valores JSON-irrepresentáveis perdem informação** aqui, como em qualquer round-trip JSON.

## Uso

### DTO ← Entity (fluxo de Read típico)

```go
dto, err := mapper.EntityToDtoWithOptions[UserEntity, UserDTO](&entity, mapper.MapperOptions{
    ObjectIdToString: true,
})
// dto.ID, dto.Created, dto.Updated populados; ObjectIDs já viraram strings.
```

### DTO → Entity (fluxo de Create/Update típico)

```go
entity, err := mapper.DtoToEntityWithOptions[CreateUserDTO, UserEntity](dto, mapper.MapperOptions{
    StringToObjectId: true,
})
// entity.OrgId, entity.AssigneeIds, etc. parseados a partir de fields string.
```

### Cópia simples (sem mágica de ID/timestamp)

```go
dst, err := mapper.DtoToEntity[FilterDTO, FilterEntity](&filterDTO)
```

## Notas

- Todos os helpers usam `copier.Option{DeepCopy: true}` para que ponteiros e slices aninhados fiquem independentes da origem.
- `EntityToDto` (sem opções) **aplica** o tap de Created/Updated. Se quer uma cópia pura sem pós-processamento, construa com `copier.CopyWithOption` diretamente.
- O pós-passo via reflection percorre fields da struct por nome — fields renomeados entre Entity e DTO não são pontes. Mantenha nomes em sync, ou implemente mapper manual para fields divergentes.
- ObjectIds inválidos são silenciosamente descartados nas conversões de slice. Se precisa de validação estrita, valide antes de chamar o mapper.
