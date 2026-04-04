package chModel

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

//
// INSERT OPERATIONS
//

// Insert inserts a single record into the table.
//
// It extracts field values from the struct using reflection and the "ch" tag mapping.
// Fields tagged with maps or slices are automatically marshaled to JSON.
//
// Example:
//
//	event := Event{
//	    Timestamp: time.Now(),
//	    OrgId:     "org-123",
//	    Payload:   `{"temperature": 25.5}`,
//	}
//	err := table.Insert(ctx, &event)
func (t *Table[T]) Insert(ctx context.Context, item *T) error {
	values, err := t.extractValues(item)
	if err != nil {
		return err
	}

	columns := getColumnNames(t.fields)
	query := BuildInsert(t.tableName, columns)

	if err := t.conn.Exec(ctx, query, values...); err != nil {
		logger.Error(err, fmt.Sprintf("[INFRA:CLICKHOUSE] Insert failed: %s", t.tableName))
		return fmt.Errorf("%w: %v", ErrQueryFailed, err)
	}

	return nil
}

// InsertBatch inserts multiple records efficiently using ClickHouse batch API.
//
// This method uses PrepareBatch for optimal performance when inserting
// large numbers of records. ClickHouse is optimized for batch operations.
//
// Example:
//
//	events := []*Event{event1, event2, event3}
//	err := table.InsertBatch(ctx, events)
func (t *Table[T]) InsertBatch(ctx context.Context, items []*T) error {
	if len(items) == 0 {
		return ErrEmptyItems
	}

	columns := getColumnNames(t.fields)
	query := BuildInsertBatch(t.tableName, columns)

	batch, err := t.conn.PrepareBatch(ctx, query)
	if err != nil {
		logger.Error(err, fmt.Sprintf("[INFRA:CLICKHOUSE] PrepareBatch failed: %s", t.tableName))
		return fmt.Errorf("%w: %v", ErrBatchFailed, err)
	}

	for _, item := range items {
		values, err := t.extractValues(item)
		if err != nil {
			return err
		}

		if err := batch.Append(values...); err != nil {
			logger.Error(err, fmt.Sprintf("[INFRA:CLICKHOUSE] Batch append failed: %s", t.tableName))
			return fmt.Errorf("%w: %v", ErrBatchFailed, err)
		}
	}

	if err := batch.Send(); err != nil {
		logger.Error(err, fmt.Sprintf("[INFRA:CLICKHOUSE] Batch send failed: %s", t.tableName))
		return fmt.Errorf("%w: %v", ErrBatchFailed, err)
	}

	logger.Info(fmt.Sprintf("[INFRA:CLICKHOUSE] Batch inserted: %d records into %s", len(items), t.tableName))
	return nil
}

//
// QUERY OPERATIONS
//

// Count returns the total number of records matching the filters.
//
// Example:
//
//	count, err := table.Count(ctx, chModel.Map{"org_id": "org-123"})
func (t *Table[T]) Count(ctx context.Context, filters Map) (uint64, error) {
	qb := t.Query().WhereMap(filters)
	query, args := qb.BuildCount()

	var count uint64
	if err := t.conn.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		logger.Error(err, fmt.Sprintf("[INFRA:CLICKHOUSE] Count failed: %s", t.tableName))
		return 0, fmt.Errorf("%w: %v", ErrQueryFailed, err)
	}

	return count, nil
}

// FindByOffset performs offset-based pagination using page/perPage strategy.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - filters: Map of field=value filters (equality only)
//   - pagination: Pagination options (page, perPage)
//   - sort: Sort order string (e.g., "timestamp:desc")
//
// Returns:
//   - PaginatedResult containing items and pagination metadata
//
// Example:
//
//	result, err := table.FindByOffset(ctx,
//	    chModel.Map{"org_id": "org-123"},
//	    &chModel.PaginationOpts{Page: 1, PerPage: 20},
//	    "timestamp:desc",
//	)
func (t *Table[T]) FindByOffset(
	ctx context.Context,
	filters Map,
	pagination *PaginationOpts,
	sort string,
) (*PaginatedResult[T], error) {
	// Apply pagination defaults
	page := DefaultPage
	perPage := DefaultPerPage

	if pagination != nil {
		if pagination.Page > 0 {
			page = pagination.Page
		}
		if pagination.PerPage > 0 && pagination.PerPage <= MaxPerPage {
			perPage = pagination.PerPage
		}
	}

	offset := (page - 1) * perPage
	if offset > MaxOffset {
		offset = MaxOffset
	}

	// Parse sort order
	orderBy := ParseSort(sort, t.cfg.TimestampField, t.cfg.DefaultOrder)

	// Count total items
	totalItemsU64, err := t.Count(ctx, filters)
	if err != nil {
		return nil, err
	}
	totalItems := int64(totalItemsU64)

	// Build and execute query
	columns := getColumnNames(t.fields)
	qb := t.Query().
		Select(columns...).
		WhereMap(filters).
		OrderBy(orderBy).
		Limit(perPage).
		Offset(offset)

	query, args := qb.BuildSelect()

	rows, err := t.conn.Query(ctx, query, args...)
	if err != nil {
		logger.Error(err, fmt.Sprintf("[INFRA:CLICKHOUSE] Query failed: %s", t.tableName))
		return nil, fmt.Errorf("%w: %v", ErrQueryFailed, err)
	}
	defer rows.Close()

	// Scan results
	items, err := t.scanRows(rows)
	if err != nil {
		return nil, err
	}

	// Calculate total pages
	totalPages := (totalItems + perPage - 1) / perPage

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

// FindWithFilters performs a query with advanced filter conditions.
//
// This method supports complex filters beyond simple equality.
//
// Example:
//
//	filters := []chModel.Filter{
//	    {Field: "org_id", Operator: chModel.OpEqual, Value: "org-123"},
//	    {Field: "timestamp", Operator: chModel.OpGreaterEqual, Value: startTime},
//	    {Field: "timestamp", Operator: chModel.OpLessEqual, Value: endTime},
//	}
//	result, err := table.FindWithFilters(ctx, filters, pagination, "timestamp:desc")
func (t *Table[T]) FindWithFilters(
	ctx context.Context,
	filters []Filter,
	pagination *PaginationOpts,
	sort string,
) (*PaginatedResult[T], error) {
	// Apply pagination defaults
	page := DefaultPage
	perPage := DefaultPerPage

	if pagination != nil {
		if pagination.Page > 0 {
			page = pagination.Page
		}
		if pagination.PerPage > 0 && pagination.PerPage <= MaxPerPage {
			perPage = pagination.PerPage
		}
	}

	offset := (page - 1) * perPage
	if offset > MaxOffset {
		offset = MaxOffset
	}

	// Parse sort order
	orderBy := ParseSort(sort, t.cfg.TimestampField, t.cfg.DefaultOrder)

	// Count total items
	countQB := t.Query().WhereFilters(filters)
	countQuery, countArgs := countQB.BuildCount()

	var totalItemsU64 uint64
	if err := t.conn.QueryRow(ctx, countQuery, countArgs...).Scan(&totalItemsU64); err != nil {
		logger.Error(err, fmt.Sprintf("[INFRA:CLICKHOUSE] Count failed: %s", t.tableName))
		return nil, fmt.Errorf("%w: %v", ErrQueryFailed, err)
	}
	totalItems := int64(totalItemsU64)

	// Build and execute data query
	columns := getColumnNames(t.fields)
	qb := t.Query().
		Select(columns...).
		WhereFilters(filters).
		OrderBy(orderBy).
		Limit(perPage).
		Offset(offset)

	query, args := qb.BuildSelect()

	rows, err := t.conn.Query(ctx, query, args...)
	if err != nil {
		logger.Error(err, fmt.Sprintf("[INFRA:CLICKHOUSE] Query failed: %s", t.tableName))
		return nil, fmt.Errorf("%w: %v", ErrQueryFailed, err)
	}
	defer rows.Close()

	// Scan results
	items, err := t.scanRows(rows)
	if err != nil {
		return nil, err
	}

	// Calculate total pages
	totalPages := (totalItems + perPage - 1) / perPage

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

//
// INTERNAL HELPERS
//

// extractValues extracts field values from a struct for INSERT operations.
// JSON-tagged fields (maps, slices) are marshaled to JSON strings.
func (t *Table[T]) extractValues(item *T) ([]interface{}, error) {
	v := reflect.ValueOf(item)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	values := make([]interface{}, len(t.fields))

	for i, field := range t.fields {
		fv := v.Field(field.Index)

		// Handle pointer types
		if field.IsPointer && fv.IsNil() {
			values[i] = nil
			continue
		}
		if field.IsPointer {
			fv = fv.Elem()
		}

		// Handle JSON marshaling for maps/slices
		if field.IsJSON {
			jsonBytes, err := json.Marshal(fv.Interface())
			if err != nil {
				logger.Error(err, fmt.Sprintf("[INFRA:CLICKHOUSE] JSON marshal failed for field: %s", field.Name))
				return nil, fmt.Errorf("%w: field %s: %v", ErrMarshalFailed, field.Name, err)
			}
			values[i] = string(jsonBytes)
		} else {
			values[i] = fv.Interface()
		}
	}

	return values, nil
}

// scanRows scans query results into a slice of T.
func (t *Table[T]) scanRows(rows interface{ Next() bool; Scan(dest ...interface{}) error; Err() error }) ([]T, error) {
	items := make([]T, 0)

	for rows.Next() {
		var item T
		v := reflect.ValueOf(&item).Elem()

		// Create scan destinations
		scanDest := make([]interface{}, len(t.fields))
		jsonFields := make(map[int]*string) // Track JSON fields that need unmarshaling

		for i, field := range t.fields {
			fv := v.Field(field.Index)

			if field.IsJSON {
				// For JSON fields, scan into a string first
				var jsonStr string
				scanDest[i] = &jsonStr
				jsonFields[i] = &jsonStr
			} else {
				scanDest[i] = fv.Addr().Interface()
			}
		}

		if err := rows.Scan(scanDest...); err != nil {
			logger.Error(err, fmt.Sprintf("[INFRA:CLICKHOUSE] Scan failed: %s", t.tableName))
			return nil, fmt.Errorf("%w: %v", ErrScanFailed, err)
		}

		// Unmarshal JSON fields
		for i, jsonStr := range jsonFields {
			if jsonStr != nil && *jsonStr != "" {
				fv := v.Field(t.fields[i].Index)
				if err := json.Unmarshal([]byte(*jsonStr), fv.Addr().Interface()); err != nil {
					logger.Warn(fmt.Sprintf("[INFRA:CLICKHOUSE] JSON unmarshal failed for field %s: %v", t.fields[i].Name, err))
					// Don't fail, just leave the field with zero value
				}
			}
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		logger.Error(err, fmt.Sprintf("[INFRA:CLICKHOUSE] Rows iteration error: %s", t.tableName))
		return nil, fmt.Errorf("%w: %v", ErrQueryFailed, err)
	}

	return items, nil
}

// buildWhereFromMap builds WHERE clause parts from a Map filter.
func buildWhereFromMap(filters Map) ([]string, []interface{}) {
	whereParts := make([]string, 0, len(filters))
	args := make([]interface{}, 0, len(filters))

	for field, value := range filters {
		// Handle special operators in value
		if valueMap, ok := value.(map[string]interface{}); ok {
			for op, v := range valueMap {
				switch op {
				case "$regex":
					// Convert MongoDB-style regex to LIKE
					pattern := fmt.Sprintf("%v", v)
					pattern = strings.TrimPrefix(pattern, "^")
					whereParts = append(whereParts, fmt.Sprintf("%s LIKE ?", field))
					args = append(args, pattern+"%")
				case "$gt":
					whereParts = append(whereParts, fmt.Sprintf("%s > ?", field))
					args = append(args, v)
				case "$gte":
					whereParts = append(whereParts, fmt.Sprintf("%s >= ?", field))
					args = append(args, v)
				case "$lt":
					whereParts = append(whereParts, fmt.Sprintf("%s < ?", field))
					args = append(args, v)
				case "$lte":
					whereParts = append(whereParts, fmt.Sprintf("%s <= ?", field))
					args = append(args, v)
				case "$ne":
					whereParts = append(whereParts, fmt.Sprintf("%s != ?", field))
					args = append(args, v)
				case "$in":
					whereParts = append(whereParts, fmt.Sprintf("%s IN (?)", field))
					args = append(args, v)
				}
			}
		} else {
			// Simple equality
			whereParts = append(whereParts, fmt.Sprintf("%s = ?", field))
			args = append(args, value)
		}
	}

	return whereParts, args
}

//
// CURSOR PAGINATION OPERATIONS
//

// FindByCursor performs time-based cursor pagination optimized for ClickHouse.
//
// This method is efficient for large datasets because it doesn't require counting
// total records. Instead, it uses the timestamp field as a cursor for pagination.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - filters: Slice of Filter conditions
//   - opts: Cursor pagination options (cursor, direction, limit)
//
// Returns:
//   - TimeCursorResult containing items and cursor metadata
//
// Example:
//
//	result, err := table.FindByCursor(ctx,
//	    []chModel.Filter{
//	        {Field: "org_id", Operator: chModel.OpEqual, Value: "org-123"},
//	    },
//	    &chModel.TimeCursorOpts{
//	        Cursor:    "2024-01-01T00:00:00Z", // or time.Time
//	        Direction: "next",
//	        Limit:     20,
//	        SortAsc:   false, // DESC order (newest first)
//	    },
//	)
func (t *Table[T]) FindByCursor(
	ctx context.Context,
	filters []Filter,
	opts *TimeCursorOpts,
) (*TimeCursorResult[T], error) {
	// Default limit
	limit := int64(20)
	if opts != nil && opts.Limit > 0 && opts.Limit <= MaxPerPage {
		limit = opts.Limit
	}

	// Determine sort direction
	sortDir := "DESC"
	if opts != nil && opts.SortAsc {
		sortDir = "ASC"
	}

	// Get timestamp field from config
	timestampField := t.cfg.TimestampField
	if timestampField == "" {
		timestampField = "timestamp"
	}

	// Build cursor filter if provided
	cursorFilters := make([]Filter, len(filters))
	copy(cursorFilters, filters)

	if opts != nil && opts.Cursor != nil {
		var cursorOp FilterOperator
		if opts.Direction == "prev" {
			// Going backward: get items AFTER cursor (in DESC order, this means newer)
			if sortDir == "DESC" {
				cursorOp = OpGreater
			} else {
				cursorOp = OpLess
			}
		} else {
			// Going forward (next): get items BEFORE cursor (in DESC order, this means older)
			if sortDir == "DESC" {
				cursorOp = OpLess
			} else {
				cursorOp = OpGreater
			}
		}

		cursorFilters = append(cursorFilters, Filter{
			Field:    timestampField,
			Operator: cursorOp,
			Value:    opts.Cursor,
		})
	}

	// Fetch limit + 1 to check if there are more items
	fetchLimit := limit + 1

	// Build query
	columns := getColumnNames(t.fields)
	orderBy := fmt.Sprintf("%s %s", timestampField, sortDir)

	qb := t.Query().
		Select(columns...).
		WhereFilters(cursorFilters).
		OrderBy(orderBy).
		Limit(fetchLimit)

	query, args := qb.BuildSelect()

	// Debug log: show generated SQL and arguments
	logger.Info(fmt.Sprintf("[INFRA:CLICKHOUSE] FindByCursor SQL: %s | Args: %+v", query, args))

	rows, err := t.conn.Query(ctx, query, args...)
	if err != nil {
		logger.Error(err, fmt.Sprintf("[INFRA:CLICKHOUSE] Cursor query failed: %s", t.tableName))
		return nil, fmt.Errorf("%w: %v", ErrQueryFailed, err)
	}
	defer rows.Close()

	items, err := t.scanRows(rows)
	if err != nil {
		return nil, err
	}

	// Determine if there are more items
	hasMore := int64(len(items)) > limit
	if hasMore {
		items = items[:limit] // Remove the extra item
	}

	// Build result
	result := &TimeCursorResult[T]{
		Items: items,
	}

	// Set cursors based on direction
	if len(items) > 0 {
		// Get timestamp from first and last items using reflection
		firstTimestamp := t.getTimestampFromItem(&items[0], timestampField)
		lastTimestamp := t.getTimestampFromItem(&items[len(items)-1], timestampField)

		if opts != nil && opts.Direction == "prev" {
			// When going backward, we have previous items
			result.HasPrevious = hasMore
			result.HasNext = opts.Cursor != nil // If we had a cursor, there are items after
			result.PrevCursor = firstTimestamp
			result.NextCursor = lastTimestamp
		} else {
			// When going forward (default)
			result.HasNext = hasMore
			result.HasPrevious = opts != nil && opts.Cursor != nil
			result.NextCursor = lastTimestamp
			result.PrevCursor = firstTimestamp
		}
	}

	return result, nil
}

// getTimestampFromItem extracts the timestamp field value from an item using reflection.
func (t *Table[T]) getTimestampFromItem(item *T, fieldName string) time.Time {
	v := reflect.ValueOf(item)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Try to find field by ch tag or name
	for _, field := range t.fields {
		if strings.EqualFold(field.Column, fieldName) || strings.EqualFold(field.Name, fieldName) {
			fv := v.Field(field.Index)
			if field.IsTime {
				return fv.Interface().(time.Time)
			}
		}
	}

	return time.Time{}
}
