package mongoModel

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// New creates and returns a new generic Model[T] instance,
// which acts as a wrapper around a MongoDB collection.
//
// If the collection doesn't exist, it will be created.
// If indexes are defined in Config, they will be created (idempotent).
//
// Parameters:
//   - db: the *mongo.Database instance.
//   - collection: the name of the MongoDB collection.
//   - cfg: configuration for timeouts, indexes, and default behavior.
//
// Example:
//
//	model := mongoModel.New[User](db, "users", mongoModel.Config{
//	    Indexes: []mongoModel.IndexDefinition{
//	        {Name: "idx_email_unique", Keys: map[string]int{"email": 1}, Unique: true},
//	    },
//	})
func New[T any](db *mongo.Database, collection string, cfg Config) *Model[T] {
	ctx := context.Background()

	// Check if collection exists, create if not
	collections, err := db.ListCollectionNames(ctx, Map{"name": collection})
	if err != nil {
		logger.Error(err, "[INFRA:MONGODB] Failed to list collections")
	} else if len(collections) == 0 {
		err := db.CreateCollection(ctx, collection)
		if err != nil {
			logger.Error(err, "[INFRA:MONGODB] Failed to create collection")
		} else {
			logger.Info("[INFRA:MONGODB] Created collection " + collection)
		}
	}

	// Get collection reference
	col := db.Collection(collection)

	// Ensure indexes (idempotent - skips existing)
	if len(cfg.Indexes) > 0 {
		if err := ensureIndexes(ctx, col, cfg.Indexes); err != nil {
			// Log error but don't fail - indexes can be created later
			logger.Warn(fmt.Sprintf("[INFRA:MONGODB] Index creation failed for %s: %v", collection, err))
		}
	}

	return &Model[T]{col: col, cfg: cfg}
}
