package mongoModel

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

//
// INSTANCE
//

// DIRECT returns the raw *mongo.Collection bound to the model.
// Useful for low-level operations when more flexibility is required.
func (m *Model[T]) DIRECT() *mongo.Collection {
	return m.col
}

//
// CREATE
//

// CreateOne inserts a single document into the collection.
// It automatically sets the _id and created fields if they are zero.
//
// Example:
//
//	user := User{Name: "John"}
//	inserted, err := model.CreateOne(ctx, user)
func (m *Model[T]) CreateOne(ctx context.Context, item *T, opts ...*CommonOpts) (*T, error) {
	opt := firstOpt(opts...)
	ctx, cancel := normCtx(ctx, m.cfg, opt)
	defer cancel()

	setIDAndCreated(item)

	if _, err := m.col.InsertOne(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

// CreateMany inserts multiple documents at once.
// It automatically sets the _id and created fields on each item.
//
// Returns all created documents.
// Returns ErrEmptyItems if the input slice is empty.
func (m *Model[T]) CreateMany(ctx context.Context, items []T, opts ...*CommonOpts) ([]T, error) {
	opt := firstOpt(opts...)
	ctx, cancel := normCtx(ctx, m.cfg, opt)
	defer cancel()

	if len(items) == 0 {
		return nil, ErrEmptyItems
	}

	// mutate in place to set ids/created
	docs := make([]interface{}, len(items))
	for i := range items {
		setIDAndCreated(&items[i])
		docs[i] = items[i]
	}

	if _, err := m.col.InsertMany(ctx, docs); err != nil {
		return nil, err
	}

	return items, nil
}

//
// FIND
//

// FindByOffset performs offset-based pagination using page/perPage strategy.
//
// It returns a PaginatedResult including items and metadata.
// Empty filters are allowed (e.g., ROOT users with unrestricted access).
//
// Example:
//
//	result, err := model.FindByOffset(ctx, Map{"type": "admin"}, &PaginationOpts{Page: 1, PerPage: 10})
func (m *Model[T]) FindByOffset(
	ctx context.Context,
	filter Map,
	pagination *PaginationOpts,
	opts ...*CommonOpts,
) (*PaginatedResult[T], error) {

	// Note: Empty filters are allowed for ROOT users with unrestricted access
	// No validation check here - empty filter means "no filtering"

	fmt.Printf("FindByOffset called with filter: %+v\n", filter)

	// Defaults e limites
	page := DefaultPage
	perPage := DefaultPerPage

	if pagination != nil {
		if pagination.Page > 0 {
			page = pagination.Page
		}
		if pagination.PerPage > 0 && pagination.PerPage <= MaxOffsetPerPage {
			perPage = pagination.PerPage
		}
	}

	skip := (page - 1) * perPage
	if skip > MaxOffsetSkip {
		skip = 0
	}

	// Normaliza contexto e opções
	opt := firstOpt(opts...)
	ctx, cancel := normCtx(ctx, m.cfg, opt)
	defer cancel()

	findOptions := applyCommonOptions(options.Find(), opt)
	findOptions.SetSkip(skip)
	findOptions.SetLimit(perPage)

	// Executa a query
	cursor, err := m.col.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// Initialize as empty slice to ensure JSON marshals to [] instead of null when empty
	items := make([]T, 0)
	for cursor.Next(ctx) {
		var doc T
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		items = append(items, doc)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	// Conta total de documentos
	totalItems, err := m.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Cálculo de total de páginas
	totalPages := totalItems / perPage
	if totalItems%perPage != 0 {
		totalPages++
	}

	// Retorno completo
	return &PaginatedResult[T]{
		Items: items,
		Pagination: Pagination{
			Page:       page,
			PerPage:    perPage,
			TotalItems: totalItems,
			TotalPages: totalPages,
		},
	}, nil
}

// FindByCursor performs cursor-based pagination on the collection.
// It is useful for deep pagination using a MongoDB ObjectID (_id) as the cursor.
//
// PaginationOpts expected values:
//   - CursorID: ID to start from (as string or ObjectID)
//   - Limit: max number of documents to retrieve (defaults applied)
//   - SortDirection: 1 for next page (forward), -1 for previous page (backward)
//   - UseCursor: must be true to enable this method
//
// Example:
//
//	result, err := model.FindByCursor(ctx, Map{}, &PaginationOpts{
//	  CursorID:      "66b0112345aa0fe76c98dcba",
//	  PerPage:       10,
//	  SortDirection: 1,
//	  UseCursor:     true,
//	})
func (m *Model[T]) FindByCursor(
	ctx context.Context,
	filter Map,
	pagination *PaginationOpts,
	opts ...*CommonOpts,
) (*PaginatedResult[T], error) {
	if pagination == nil || !pagination.UseCursor {
		return nil, ErrCursorPaginationRequired
	}

	// Validate sort direction
	direction := pagination.SortDirection
	if direction != 1 && direction != -1 {
		return nil, ErrInvalidCursorDirection
	}

	// Apply limit with fallback
	limit := pagination.PerPage
	if limit <= 0 || limit > MaxOffsetPerPage {
		limit = DefaultPerPage
	}

	if filter == nil {
		filter = Map{}
	}

	// Apply cursor filter
	if pagination.CursorID != nil {
		cursorID, err := ToObjectID(pagination.CursorID)
		if err != nil {
			return nil, ErrInvalidID
		}

		operator := "$gt"
		if direction == -1 {
			operator = "$lt"
		}
		filter["_id"] = Map{operator: cursorID}
	}

	// Normalize context and options
	opt := firstOpt(opts...)
	ctx, cancel := normCtx(ctx, m.cfg, opt)
	defer cancel()

	findOpts := applyCommonOptions(options.Find(), opt)
	findOpts.SetSort(Map{"_id": direction})
	findOpts.SetLimit(limit + 1) // fetch one extra to check for hasNext/hasPrev

	cursor, err := m.col.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// Initialize as empty slice to ensure JSON marshals to [] instead of null when empty
	docs := make([]T, 0)
	for cursor.Next(ctx) {
		var doc T
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}

	hasMore := int64(len(docs)) > limit
	if hasMore {
		docs = docs[:limit]
	}

	// If going backward, reverse the order to maintain consistency
	if direction == -1 {
		for i, j := 0, len(docs)-1; i < j; i, j = i+1, j-1 {
			docs[i], docs[j] = docs[j], docs[i]
		}
	}

	hasNext := direction == 1 && hasMore
	hasPrev := direction == -1 && hasMore

	if direction == 1 && pagination.CursorID != nil {
		hasPrev = true
	}

	if direction == -1 && pagination.CursorID != nil {
		hasNext = true
	}

	return &PaginatedResult[T]{
		Items: docs,
		Pagination: Pagination{
			PerPage: limit,
			HasNext: &hasNext,
			HasPrev: &hasPrev,
		},
	}, nil
}

// FindByID retrieves a document by its _id (string or ObjectID).
//
// It applies any provided projection or sorting via CommonOpts.
// Returns ErrInvalidID if the id is invalid, ErrNotFound if not found.
func (m *Model[T]) FindByID(ctx context.Context, id any, opts ...*CommonOpts) (*T, error) {

	opt := firstOpt(opts...)
	ctx, cancel := normCtx(ctx, m.cfg, opt)
	defer cancel()

	// Apply common find options
	findOptions := applyCommonOptions(options.FindOne(), opt)

	// Convert id to ObjectID
	oid, errToConvert := ToObjectID(id)
	if errToConvert != nil {
		return nil, ErrInvalidID
	}

	// Map to hold the result
	var out T

	// Search by ObjectID
	err := m.col.FindOne(ctx, Map{"_id": oid}, findOptions).Decode(&out)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &out, nil
}

// FindOne finds a single document matching the provided filters.
//
// The result is returned as a Map. You can apply projection/sorting via CommonOpts.
// Returns ErrNotFound if no match is found.
func (m *Model[T]) FindOne(ctx context.Context, filters *Map, opts ...*CommonOpts) (*T, error) {

	opt := firstOpt(opts...)

	ctx, cancel := normCtx(ctx, m.cfg, opt)
	defer cancel()

	findOptions := applyCommonOptions(options.FindOne(), opt)

	var out T
	err := m.col.FindOne(ctx, filters, findOptions).Decode(&out)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &out, nil
}

//
// UPDATE
//

// FindAndUpdateMany updates multiple documents matching the given filter.
//
// The payload is the update document (e.g. $set). Supports CommonOpts like upsert.
// Returns the UpdateResult, or ErrNotFound if no match.
func (m *Model[T]) FindAndUpdateMany(ctx context.Context, filters Map, payload Map, opts ...*CommonOpts) (*mongo.UpdateResult, error) {

	// Set query options
	opt := firstOpt(opts...)

	// Set context and cancel function
	ctx, cancel := normCtx(ctx, m.cfg, opt)
	defer cancel()

	// Apply common find options
	findOptions := applyCommonOptions(options.UpdateMany(), opt)

	// Search by generic filters
	results, err := m.col.UpdateMany(ctx, filters, payload, findOptions)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return results, nil
}

// FindByIDAndUpdate updates a document by its _id and returns the updated document.
//
// You can control projection, return behavior (before/after), upsert, etc. via CommonOpts.
// Returns ErrInvalidID if the id is not valid.
func (m *Model[T]) FindByIDAndUpdate(ctx context.Context, id any, payload Map, opts ...*CommonOpts) (T, error) {

	// Map to hold the result
	var out T

	opt := firstOpt(opts...)
	ctx, cancel := normCtx(ctx, m.cfg, opt)
	defer cancel()

	// Apply common find options
	findOptions := applyCommonOptions(options.FindOneAndUpdate(), opt)

	// Convert id to ObjectID
	oid, errToConvert := ToObjectID(id)
	if errToConvert != nil {
		return out, ErrInvalidID
	}

	// Search by ObjectID
	err := m.col.FindOneAndUpdate(ctx, Map{"_id": oid}, payload, findOptions).Decode(&out)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return out, ErrNotFound
		}
		return out, err
	}
	return out, nil
}

// FindOneAndUpdate updates the first document matching the filter and returns it.
//
// The payload is the update document (e.g. $set). Supports CommonOpts like upsert, projection, etc.
// Returns ErrNotFound if no document matches.
func (m *Model[T]) FindOneAndUpdate(ctx context.Context, filters *Map, payload *Map, opts ...*CommonOpts) (*T, error) {

	// Map to hold the result
	var out T

	opt := firstOpt(opts...)
	ctx, cancel := normCtx(ctx, m.cfg, opt)
	defer cancel()

	// Apply common find options
	findOptions := applyCommonOptions(options.FindOneAndUpdate(), opt)

	// Search by ObjectID
	err := m.col.FindOneAndUpdate(ctx, filters, payload, findOptions).Decode(&out)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return &out, ErrNotFound
		}
		return &out, err
	}
	return &out, nil
}

//
// DELETE
//

// DeleteOne deletes a single document matching the filter.
//
// Returns ErrNotFound if no match is found.
func (m *Model[T]) DeleteByID(ctx context.Context, id any, opts ...*CommonOpts) error {
	opt := firstOpt(opts...)
	ctx, cancel := normCtx(ctx, m.cfg, opt)
	defer cancel()

	// Convert id to ObjectID
	oid, errToConvert := ToObjectID(id)
	if errToConvert != nil {
		return ErrInvalidID
	}

	res, err := m.col.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return err
	}

	if res.DeletedCount == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteOne deletes a single document matching the filter.
//
// Returns ErrNotFound if no match is found.
func (m *Model[T]) DeleteOne(ctx context.Context, filters *Map, opts ...*CommonOpts) error {
	opt := firstOpt(opts...)
	ctx, cancel := normCtx(ctx, m.cfg, opt)
	defer cancel()

	res, err := m.col.DeleteOne(ctx, filters)
	if err != nil {
		return err
	}

	if res.DeletedCount == 0 {
		return ErrNotFound
	}

	return nil
}

// DeleteMany deletes all documents that match the given filter.
//
// Returns the number of documents deleted.
// Returns ErrEmptyFilters if filter is nil or empty.
func (m *Model[T]) DeleteMany(ctx context.Context, filter Map, opts ...*CommonOpts) (int64, error) {

	if len(filter) == 0 {
		return 0, ErrEmptyFilters
	}

	opt := firstOpt(opts...)
	ctx, cancel := normCtx(ctx, m.cfg, opt)
	defer cancel()

	if filter == nil {
		filter = Map{}
	}

	res, err := m.col.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}

	if res.DeletedCount == 0 {
		return 0, ErrNotFound
	}

	return res.DeletedCount, nil
}

// FindWithCursor performs cursor-based pagination using _id field.
// This method provides bi-directional navigation (forward/backward) with consistent
// performance regardless of page depth.
//
// The method extracts _id from BSON documents before decoding to type T,
// eliminating the need for reflection or interface constraints.
//
// Features:
//   - Forward and backward pagination
//   - Ascending and descending sort
//   - Automatic cursor calculation (next/prev)
//   - Detects hasNext and hasPrevious
//
// Usage:
//
//	opts := &CursorOpts{
//	    Cursor:    "",              // Empty for first page
//	    Direction: CursorNext,      // Forward pagination
//	    Limit:     50,              // Items per page
//	    SortAsc:   true,            // Ascending sort
//	}
//	result, err := model.FindWithCursor(ctx, filters, opts, projection)
func (m *Model[T]) FindWithCursor(
	ctx context.Context,
	filter Map,
	cursorOpts *CursorOpts,
	projection Map,
) (*CursorResult[T], error) {

	if cursorOpts == nil {
		return nil, errors.New("cursorOpts cannot be nil")
	}

	// Apply default limit if not provided or invalid
	if cursorOpts.Limit <= 0 {
		cursorOpts.Limit = 300
	}

	// Determine sort direction
	sortDirection := 1
	if !cursorOpts.SortAsc {
		sortDirection = -1
	}

	// Build cursor filter combining base filters with cursor conditions
	cursorFilter := buildCursorFilter(filter, cursorOpts)

	// Build find options with limit + 1 to detect hasMore
	findOpts := options.Find().
		SetLimit(cursorOpts.Limit + 1).
		SetSort(bson.D{{Key: "_id", Value: sortDirection}})

	if projection != nil {
		findOpts.SetProjection(projection)
	}

	// Execute query
	cursor, err := m.col.Find(ctx, cursorFilter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// Extract IDs from BSON before decoding to T
	// Initialize as empty slice to ensure JSON marshals to [] instead of null when empty
	results := make([]T, 0)
	var firstID, lastID, prevLastID string
	count := 0

	for cursor.Next(ctx) {
		// Extract _id from BSON document
		raw := cursor.Current
		idValue := raw.Lookup("_id")
		objID, _ := idValue.ObjectIDOK()

		// Track first, last, and previous last IDs
		if count == 0 {
			firstID = objID.Hex()
		}
		prevLastID = lastID
		lastID = objID.Hex()

		// Decode document to type T
		var item T
		if err := cursor.Decode(&item); err != nil {
			return nil, err
		}

		results = append(results, item)
		count++
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	// Detect if there are more items
	hasMore := len(results) > int(cursorOpts.Limit)

	if hasMore {
		// Remove the extra item
		results = results[:cursorOpts.Limit]
		// Use previous last ID (which is now the actual last after removal)
		lastID = prevLastID
	}

	// If backward pagination, reverse results and IDs
	if cursorOpts.Direction == CursorPrevious {
		// Reverse slice (imported from utils/slice package)
		for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
			results[i], results[j] = results[j], results[i]
		}
		// Swap first and last IDs
		firstID, lastID = lastID, firstID
	}

	// Build cursor result
	cursorResult := &CursorResult[T]{
		Items: results,
	}

	if len(results) > 0 {
		if cursorOpts.Direction == CursorNext {
			// Forward pagination
			cursorResult.HasNext = hasMore
			cursorResult.HasPrevious = cursorOpts.Cursor != ""

			if cursorResult.HasNext {
				cursorResult.NextCursor = lastID
			}
			if cursorResult.HasPrevious {
				cursorResult.PrevCursor = firstID
			}
		} else {
			// Backward pagination
			cursorResult.HasNext = cursorOpts.Cursor != ""
			cursorResult.HasPrevious = hasMore

			if cursorResult.HasNext {
				cursorResult.NextCursor = lastID
			}
			if cursorResult.HasPrevious {
				cursorResult.PrevCursor = firstID
			}
		}
	}

	return cursorResult, nil
}
