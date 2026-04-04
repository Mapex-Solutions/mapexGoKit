# MongoDB Model Package

`packages/infrastructure/mongodb/model`

Generic data access layer for MongoDB. Provides type-safe CRUD, dual pagination (offset + cursor), idempotent index creation, transaction support, and optional query parameters.

---

## Quick Start

```go
import (
    mongoModel "github.com/Mapex-Solutions/MapexOS/infrastructure/mongodb/model"
    mongoManager "github.com/Mapex-Solutions/MapexOS/infrastructure/mongodb/manager"
)

// Define your entity
type User struct {
    ID        bson.ObjectID `bson:"_id"`
    Name      string        `bson:"name"`
    Email     string        `bson:"email"`
    OrgID     bson.ObjectID `bson:"orgId"`
    Created   time.Time     `bson:"created"`
}

// Create model instance
db := mgr.GetDatabase()
userModel := mongoModel.New[User](db, "users", mongoModel.Config{
    DefaultTimeout: 10 * time.Second,
    Indexes: []mongoModel.IndexDefinition{
        {Name: "idx_email_unique", Keys: map[string]int{"email": 1}, Unique: true},
        {Name: "idx_org", Keys: map[string]int{"orgId": 1}},
    },
})
```

---

## Entity Requirements

Your entity struct must use `bson` tags. Two fields are auto-populated on create:

| Field | Type | Auto-set When |
|-------|------|---------------|
| `_id` (or field named `ID`) | `bson.ObjectID` | Zero value on `CreateOne`/`CreateMany` |
| `created` (or `createdAt`) | `time.Time` | Zero value on `CreateOne`/`CreateMany` |

Detection is by BSON tag or field name (case-insensitive). You can set them manually to override.

```go
type Asset struct {
    ID        bson.ObjectID `bson:"_id"`       // auto-set
    Name      string        `bson:"name"`
    Created   time.Time     `bson:"created"`   // auto-set
    Updated   time.Time     `bson:"updated"`   // NOT auto-set (manual)
}
```

---

## CRUD Operations

### Create

```go
// Single document
user := &User{Name: "Alice", Email: "alice@example.com"}
created, err := userModel.CreateOne(ctx, user)
// created.ID is now set

// Multiple documents
users := []User{
    {Name: "Bob", Email: "bob@example.com"},
    {Name: "Carol", Email: "carol@example.com"},
}
inserted, err := userModel.CreateMany(ctx, users)
```

### Find

```go
// By ID (string or ObjectID)
user, err := userModel.FindByID(ctx, "66b0112345aa0fe76c98dcba")

// By filter (single document)
user, err := userModel.FindOne(ctx, &mongoModel.Map{"email": "alice@example.com"})

// With projection
user, err := userModel.FindByID(ctx, id, &mongoModel.CommonOpts{
    Projection: mongoModel.Map{"name": 1, "email": 1},
})
```

### Update

```go
// By ID — returns updated document
updated, err := userModel.FindByIDAndUpdate(ctx, id, mongoModel.Map{
    "$set": mongoModel.Map{"name": "Alice Updated"},
})

// By filter — returns updated document
updated, err := userModel.FindOneAndUpdate(ctx,
    &mongoModel.Map{"email": "alice@example.com"},
    &mongoModel.Map{"$set": mongoModel.Map{"name": "Alice New"}},
)

// Multiple documents
result, err := userModel.FindAndUpdateMany(ctx,
    mongoModel.Map{"orgId": orgID},
    mongoModel.Map{"$set": mongoModel.Map{"active": false}},
)
// result.ModifiedCount
```

### Delete

```go
// By ID
err := userModel.DeleteByID(ctx, id)

// By filter (single)
err := userModel.DeleteOne(ctx, &mongoModel.Map{"email": "alice@example.com"})

// By filter (multiple) — returns count
count, err := userModel.DeleteMany(ctx, mongoModel.Map{"orgId": orgID})
```

---

## Pagination

### Offset-Based

Best for small-to-medium datasets with page numbers in the UI.

```go
result, err := userModel.FindByOffset(ctx,
    mongoModel.Map{"orgId": orgID},
    &mongoModel.PaginationOpts{Page: 2, PerPage: 25},
)

// result.Items          []User
// result.Pagination.Page          2
// result.Pagination.PerPage       25
// result.Pagination.TotalItems    150
// result.Pagination.TotalPages    6
```

**Limits:** `MaxOffsetPerPage = 300`, `MaxOffsetSkip = 500`. Values exceeding these are clamped.

### Cursor-Based (Recommended for Large Datasets)

Consistent O(1) performance regardless of page depth.

```go
// First page
result, err := userModel.FindWithCursor(ctx,
    mongoModel.Map{"orgId": orgID},
    &mongoModel.CursorOpts{
        Direction: mongoModel.CursorNext,
        Limit:     50,
        SortAsc:   false, // newest first
    },
    nil, // projection (optional)
)

// result.Items        []User
// result.NextCursor   "66b0112345aa0fe76c98dcba"
// result.HasNext      true
// result.HasPrevious  false

// Next page
result2, err := userModel.FindWithCursor(ctx,
    mongoModel.Map{"orgId": orgID},
    &mongoModel.CursorOpts{
        Cursor:    result.NextCursor,
        Direction: mongoModel.CursorNext,
        Limit:     50,
        SortAsc:   false,
    },
    nil,
)
```

There is also `FindByCursor()` which uses the `PaginationOpts` struct for backward compatibility.

---

## Indexes

Indexes are created **idempotently** on `New()` — existing indexes are skipped.

```go
mongoModel.New[User](db, "users", mongoModel.Config{
    Indexes: []mongoModel.IndexDefinition{
        // Simple index
        {Name: "idx_org", Keys: map[string]int{"orgId": 1}},

        // Unique index
        {Name: "idx_email_unique", Keys: map[string]int{"email": 1}, Unique: true},

        // Compound index
        {Name: "idx_org_email", Keys: map[string]int{"orgId": 1, "email": 1}},

        // Sparse index (only docs with the field)
        {Name: "idx_phone_sparse", Keys: map[string]int{"phone": 1}, Sparse: true},

        // Partial index (only docs matching the filter)
        {
            Name: "idx_timer_partial",
            Keys: map[string]int{"timerExpiresAt": 1, "status": 1},
            PartialFilterExpression: bson.M{"timerExpiresAt": bson.M{"$type": "date"}},
        },
    },
})
```

---

## Transactions

Model[T] delegates to the manager package for centralized retry logic.

```go
result, err := userModel.RunTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
    user, err := userModel.CreateOne(sessCtx, &User{Name: "Alice"})
    if err != nil {
        return nil, err // abort
    }
    _, err = memberModel.CreateOne(sessCtx, &Member{UserID: user.ID})
    if err != nil {
        return nil, err // abort, user creation rolled back
    }
    return user, nil // commit
})
```

For manual session management:

```go
session, err := userModel.NewSession(ctx)
if err != nil {
    return err
}
defer session.EndSession(ctx)
```

---

## CommonOpts

Fine-grained control over individual operations. Pass as the last argument to any CRUD method.

```go
opts := &mongoModel.CommonOpts{
    Projection:     mongoModel.Map{"name": 1, "email": 1},
    Sort:           mongoModel.Map{"created": -1},
    Hint:           "idx_org_email",
    Session:        session,               // join a transaction
    Upsert:         boolPtr(true),         // create if not found
    ReturnDocument: &mongoModel.ReturnDocNew, // return doc after update
}

user, err := userModel.FindByIDAndUpdate(ctx, id, payload, opts)
```

| Field | Type | Used By |
|-------|------|---------|
| `Projection` | `interface{}` | Find, FindOne, FindByID, FindByIDAndUpdate |
| `Sort` | `interface{}` | Find, FindOne, FindByIDAndUpdate |
| `Hint` | `interface{}` | All operations |
| `Collation` | `*options.Collation` | All operations |
| `Session` | `*mongo.Session` | All operations (joins transaction) |
| `Upsert` | `*bool` | Update operations |
| `ReturnDocument` | `*options.ReturnDocument` | FindByIDAndUpdate, FindOneAndUpdate |
| `BypassDocumentValidation` | `*bool` | Update operations |
| `Comment` | `interface{}` | Update operations |
| `MaxTime` | `*time.Duration` | All operations |
| `ArrayFilters` | `[]interface{}` | Update operations |

---

## Utilities

```go
// Type aliases
type Map = bson.M              // shorthand for filters, projections, updates
type ObjectId = bson.ObjectID
type Collection = mongo.Collection

// Generate new ObjectID
id := mongoModel.NewObjectID()

// Convert string/ObjectID to ObjectID
oid, err := mongoModel.ToObjectID("66b0112345aa0fe76c98dcba")
oid2, err := mongoModel.ToObjectID(existingOID) // passthrough

// Convert comma-separated fields to projection
proj := mongoModel.StringToProjection("name, email, orgId")
// => Map{"name": 1, "email": 1, "orgId": 1}

// Return document constants
mongoModel.ReturnDocOld // options.Before
mongoModel.ReturnDocNew // options.After
```

---

## Direct Access

For operations not covered by Model[T] (e.g., `BulkWrite`, aggregation pipelines):

```go
col := userModel.DIRECT() // returns *mongo.Collection

// BulkWrite example
models := []mongo.WriteModel{
    mongo.NewInsertOneModel().SetDocument(doc1),
    mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update),
}
result, err := col.BulkWrite(ctx, models)

// Aggregation example
cursor, err := col.Aggregate(ctx, pipeline)
```

---

## Error Handling

All errors are `var` declarations — use `errors.Is()` for comparison.

| Error | When |
|-------|------|
| `ErrNotFound` | Document not found (FindByID, FindOne, DeleteByID, etc.) |
| `ErrInvalidID` | ID is not a valid ObjectID string |
| `ErrEmptyItems` | Empty slice passed to `CreateMany` |
| `ErrEmptyFilters` | Empty/nil filter passed to `DeleteMany` |
| `ErrCursorPaginationRequired` | `FindByCursor` called without `UseCursor: true` |
| `ErrInvalidCursorDirection` | `SortDirection` is not 1 or -1 |
| `ErrNotConnected` | Client is nil (transaction methods) |

```go
user, err := userModel.FindByID(ctx, id)
if errors.Is(err, mongoModel.ErrNotFound) {
    // handle 404
}
```

---

## Pagination Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `DefaultPage` | `1` | Default page number |
| `DefaultPerPage` | `25` | Default items per page |
| `MaxOffsetPerPage` | `300` | Max items per page (offset mode) |
| `MaxOffsetSkip` | `500` | Max documents to skip |
