package mongoModel

import (
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// NewObjectID generates a new unique MongoDB ObjectID.
func NewObjectID() bson.ObjectID {
	return bson.NewObjectID()
}

// StringToProjection converts a comma-separated list of fields
// into a projection map: field -> 1.
//
// Examples:
//
//	"name"                  => {"name": 1}
//	"name, lastname, email" => {"name": 1, "lastname": 1, "email": 1}
func StringToProjection(value string) Map {
	proj := Map{}
	for _, part := range strings.Split(value, ",") {
		field := strings.TrimSpace(part)
		if field == "" {
			continue
		}
		proj[field] = 1
	}
	return proj
}

// ToObjectID converts a string or bson.ObjectID to a valid ObjectID.
//
// Returns ErrInvalidID if conversion fails.
func ToObjectID(id any) (bson.ObjectID, error) {
	switch v := id.(type) {
	case bson.ObjectID:
		return v, nil
	case string:
		return bson.ObjectIDFromHex(v)
	default:
		return bson.NilObjectID, ErrInvalidID
	}
}

// ── Write Model Factories ──

// NewInsertOneModel creates a new InsertOneModel for BulkWrite operations.
func NewInsertOneModel() *mongo.InsertOneModel {
	return mongo.NewInsertOneModel()
}

// NewUpdateOneModel creates a new UpdateOneModel for BulkWrite operations.
func NewUpdateOneModel() *mongo.UpdateOneModel {
	return mongo.NewUpdateOneModel()
}

// NewReplaceOneModel creates a new ReplaceOneModel for BulkWrite operations.
func NewReplaceOneModel() *mongo.ReplaceOneModel {
	return mongo.NewReplaceOneModel()
}

// ── Options Factories ──

// BulkWrite creates BulkWrite options builder.
func BulkWrite() *options.BulkWriteOptionsBuilder {
	return options.BulkWrite()
}

// FindOptions creates Find options builder.
func FindOptions() *options.FindOptionsBuilder {
	return options.Find()
}

// ── Error Helpers ──

// IsDuplicateKeyError checks if the error is a MongoDB duplicate key error.
func IsDuplicateKeyError(err error) bool {
	return mongo.IsDuplicateKeyError(err)
}

// buildCursorFilter creates a MongoDB filter for cursor-based pagination.
// It combines the base filter with cursor conditions based on direction and sort order.
//
// The function adds _id comparison operators ($gt or $lt) to enable efficient
// cursor pagination without using skip/offset.
//
// Logic:
//   - Forward (next) + ASC: _id > cursor
//   - Forward (next) + DESC: _id < cursor
//   - Backward (previous) + ASC: _id < cursor
//   - Backward (previous) + DESC: _id > cursor
//
// If cursor is empty or invalid, returns the base filter unchanged.
func buildCursorFilter(filter Map, opts *CursorOpts) Map {
	cursorFilter := make(Map)

	// Copy existing filters
	for k, v := range filter {
		cursorFilter[k] = v
	}

	// Add cursor condition if provided
	if opts.Cursor != "" {
		cursorObjId, err := bson.ObjectIDFromHex(opts.Cursor)
		if err != nil {
			// Invalid cursor, return base filter unchanged
			return cursorFilter
		}

		// Determine comparison operator based on direction and sort
		if opts.Direction == CursorNext {
			// Forward pagination
			if opts.SortAsc {
				cursorFilter["_id"] = bson.M{"$gt": cursorObjId}
			} else {
				cursorFilter["_id"] = bson.M{"$lt": cursorObjId}
			}
		} else {
			// Backward pagination
			if opts.SortAsc {
				cursorFilter["_id"] = bson.M{"$lt": cursorObjId}
			} else {
				cursorFilter["_id"] = bson.M{"$gt": cursorObjId}
			}
		}
	}

	return cursorFilter
}
