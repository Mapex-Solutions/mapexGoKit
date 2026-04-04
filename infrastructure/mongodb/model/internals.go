package mongoModel

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// applyCommonOptions applies CommonOpts to various MongoDB driver option types.
// It supports options such as FindOneOptions, FindOptions, UpdateOptions, etc.
func applyCommonOptions[T any](dst T, opt *CommonOpts) T {
	if opt == nil {
		return dst
	}

	switch v := any(dst).(type) {
	case *options.FindOneOptionsBuilder:
		if opt.Projection != nil {
			v.SetProjection(opt.Projection)
		}
		if opt.Sort != nil {
			v.SetSort(opt.Sort)
		}
		if opt.Hint != nil {
			v.SetHint(opt.Hint)
		}
		if opt.Collation != nil {
			v.SetCollation(opt.Collation)
		}
		return any(v).(T)

	case *options.FindOptionsBuilder:
		if opt.Projection != nil {
			v.SetProjection(opt.Projection)
		}
		if opt.Sort != nil {
			v.SetSort(opt.Sort)
		}
		if opt.Hint != nil {
			v.SetHint(opt.Hint)
		}
		if opt.Collation != nil {
			v.SetCollation(opt.Collation)
		}
		return any(v).(T)

	case *options.FindOneAndUpdateOptionsBuilder:
		if opt.Projection != nil {
			v.SetProjection(opt.Projection)
		}
		if opt.Sort != nil {
			v.SetSort(opt.Sort)
		}
		if opt.Hint != nil {
			v.SetHint(opt.Hint)
		}
		if opt.Collation != nil {
			v.SetCollation(opt.Collation)
		}
		if opt.Upsert != nil {
			v.SetUpsert(*opt.Upsert)
		}
		if opt.ReturnDocument != nil {
			v.SetReturnDocument(*opt.ReturnDocument)
		}
		if opt.BypassDocumentValidation != nil {
			v.SetBypassDocumentValidation(*opt.BypassDocumentValidation)
		}
		if opt.Comment != nil {
			v.SetComment(opt.Comment)
		}
		if opt.Let != nil {
			v.SetLet(opt.Let)
		}
		if opt.ArrayFilters != nil {
			v.SetArrayFilters(opt.ArrayFilters)
		}
		return any(v).(T)

	case *options.UpdateManyOptionsBuilder:
		if opt.Hint != nil {
			v.SetHint(opt.Hint)
		}
		if opt.Collation != nil {
			v.SetCollation(opt.Collation)
		}
		if opt.Upsert != nil {
			v.SetUpsert(*opt.Upsert)
		}
		if opt.BypassDocumentValidation != nil {
			v.SetBypassDocumentValidation(*opt.BypassDocumentValidation)
		}
		if opt.Comment != nil {
			v.SetComment(opt.Comment)
		}
		if opt.Let != nil {
			v.SetLet(opt.Let)
		}
		return any(v).(T)
	}

	return dst
}

// firstOpt returns the first *CommonOpts from the variadic parameter list.
//
// In this library, only a single CommonOpts is considered during execution to keep
// behavior deterministic and simple. Even though the function accepts variadic
// parameters for flexibility (e.g., for optional usage), only the first element
// is actually used. This avoids ambiguity or unintended overrides from multiple
// options.
//
// If no options are provided, it returns nil, allowing the calling code to fall
// back to default values.
func firstOpt(opts ...*CommonOpts) *CommonOpts {
	if len(opts) == 0 {
		return nil
	}
	return opts[0]
}

// normCtx normalizes the context:
// - if ctx is nil, it creates one
// - if there is no deadline and a default timeout exists, it applies it
// - if a session is provided, it attaches it to the context
func normCtx(ctx context.Context, cfg Config, opts *CommonOpts) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}

	withSession := func(c context.Context) context.Context {
		if opts != nil && opts.Session != nil {
			return mongo.NewSessionContext(c, opts.Session)
		}
		return c
	}

	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return withSession(ctx), func() {}
	}

	if cfg.DefaultTimeout > 0 {
		ctx2, cancel := context.WithTimeout(ctx, cfg.DefaultTimeout)
		return withSession(ctx2), cancel
	}

	return withSession(ctx), func() {}
}

// setIDAndCreated sets _id and created/createdAt fields using reflection
// if they exist and are currently zero-valued.
func setIDAndCreated(ptr interface{}) {
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Pointer {
		return
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	now := time.Now().UTC()

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		fv := v.Field(i)

		if !fv.CanSet() {
			continue
		}

		if isIDField(sf) && fv.Type() == typeObjectID && fv.IsZero() {
			fv.Set(reflect.ValueOf(bson.NewObjectID()))
			continue
		}

		if isCreatedField(sf) && fv.Type() == typeTime && fv.IsZero() {
			fv.Set(reflect.ValueOf(now))
			continue
		}
	}
}

// isIDField checks if a struct field is an _id field based on name or BSON tag.
func isIDField(f reflect.StructField) bool {
	bsonTag := f.Tag.Get("bson")
	name := strings.ToLower(f.Name)
	return strings.HasPrefix(bsonTag, "_id") || name == "id"
}

// isCreatedField checks if a struct field represents a creation timestamp.
func isCreatedField(f reflect.StructField) bool {
	bsonTag := f.Tag.Get("bson")
	name := strings.ToLower(f.Name)
	return strings.HasPrefix(bsonTag, "created") || name == "created" || name == "createdat"
}

// ensureIndexes creates indexes that don't already exist on the collection.
// This operation is idempotent - existing indexes are skipped.
func ensureIndexes(ctx context.Context, col *mongo.Collection, indexes []IndexDefinition) error {
	if len(indexes) == 0 {
		return nil
	}

	collName := col.Name()

	// Get existing indexes
	existingIndexes, err := getExistingIndexNames(ctx, col)
	if err != nil {
		return err
	}

	// Filter indexes that need to be created
	var indexesToCreate []mongo.IndexModel
	var indexNames []string

	for _, idx := range indexes {
		// Skip if index already exists
		if _, exists := existingIndexes[idx.Name]; exists {
			continue
		}

		// Build keys document (ordered)
		keys := bson.D{}
		for field, order := range idx.Keys {
			keys = append(keys, bson.E{Key: field, Value: order})
		}

		// Build index options
		indexOpts := options.Index().SetName(idx.Name)
		if idx.Unique {
			indexOpts.SetUnique(true)
		}
		if idx.Sparse {
			indexOpts.SetSparse(true)
		}
		if idx.PartialFilterExpression != nil {
			indexOpts.SetPartialFilterExpression(idx.PartialFilterExpression)
		}
		if idx.ExpireAfterSeconds != nil {
			indexOpts.SetExpireAfterSeconds(*idx.ExpireAfterSeconds)
		}

		indexModel := mongo.IndexModel{
			Keys:    keys,
			Options: indexOpts,
		}

		indexesToCreate = append(indexesToCreate, indexModel)
		indexNames = append(indexNames, idx.Name)
	}

	// Create indexes if any
	if len(indexesToCreate) > 0 {
		_, err := col.Indexes().CreateMany(ctx, indexesToCreate)
		if err != nil {
			return err
		}

		logger.Info(fmt.Sprintf("[INFRA:MONGODB] Created indexes on %s: %v", collName, indexNames))
	}

	return nil
}

// getExistingIndexNames returns a map of existing index names for quick lookup.
func getExistingIndexNames(ctx context.Context, col *mongo.Collection) (map[string]bool, error) {
	cursor, err := col.Indexes().List(ctx)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	existingIndexes := make(map[string]bool)
	for cursor.Next(ctx) {
		var index bson.M
		if err := cursor.Decode(&index); err != nil {
			continue
		}
		if name, ok := index["name"].(string); ok {
			existingIndexes[name] = true
		}
	}

	return existingIndexes, nil
}
