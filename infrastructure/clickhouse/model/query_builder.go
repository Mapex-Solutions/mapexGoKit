package chModel

import (
	"fmt"
	"strings"
)

// QueryBuilder helps construct ClickHouse SQL queries dynamically.
type QueryBuilder struct {
	table      string
	selectCols []string
	whereParts []string
	whereArgs  []interface{}
	orderBy    string
	limit      int64
	offset     int64
}

// NewQueryBuilder creates a new QueryBuilder for the specified table.
func NewQueryBuilder(table string) *QueryBuilder {
	return &QueryBuilder{
		table:      table,
		selectCols: []string{},
		whereParts: []string{},
		whereArgs:  []interface{}{},
	}
}

// Select specifies which columns to retrieve.
func (q *QueryBuilder) Select(cols ...string) *QueryBuilder {
	q.selectCols = cols
	return q
}

// Where adds a filter condition.
func (q *QueryBuilder) Where(field string, op FilterOperator, value interface{}) *QueryBuilder {
	switch op {
	case OpIn, OpNotIn:
		q.whereParts = append(q.whereParts, fmt.Sprintf("%s %s (?)", field, op))
	case OpBetween:
		q.whereParts = append(q.whereParts, fmt.Sprintf("%s BETWEEN ? AND ?", field))
	default:
		q.whereParts = append(q.whereParts, fmt.Sprintf("%s %s ?", field, op))
	}
	q.whereArgs = append(q.whereArgs, value)
	return q
}

// WhereFilter adds a Filter struct as a condition.
func (q *QueryBuilder) WhereFilter(f Filter) *QueryBuilder {
	if f.Operator == OpBetween {
		q.whereParts = append(q.whereParts, fmt.Sprintf("%s BETWEEN ? AND ?", f.Field))
		q.whereArgs = append(q.whereArgs, f.Value, f.EndValue)
	} else {
		return q.Where(f.Field, f.Operator, f.Value)
	}
	return q
}

// WhereFilters adds multiple Filter conditions.
func (q *QueryBuilder) WhereFilters(filters []Filter) *QueryBuilder {
	for _, f := range filters {
		q.WhereFilter(f)
	}
	return q
}

// WhereMap adds conditions from a map (equality only).
func (q *QueryBuilder) WhereMap(m Map) *QueryBuilder {
	for field, value := range m {
		q.Where(field, OpEqual, value)
	}
	return q
}

// WhereLike adds a LIKE condition with pattern support.
func (q *QueryBuilder) WhereLike(field string, pattern string) *QueryBuilder {
	q.whereParts = append(q.whereParts, fmt.Sprintf("%s LIKE ?", field))
	q.whereArgs = append(q.whereArgs, pattern)
	return q
}

// WhereRaw adds a raw WHERE clause (use with caution).
func (q *QueryBuilder) WhereRaw(clause string, args ...interface{}) *QueryBuilder {
	q.whereParts = append(q.whereParts, clause)
	q.whereArgs = append(q.whereArgs, args...)
	return q
}

// OrderBy sets the ORDER BY clause.
func (q *QueryBuilder) OrderBy(order string) *QueryBuilder {
	q.orderBy = order
	return q
}

// Limit sets the LIMIT clause.
func (q *QueryBuilder) Limit(limit int64) *QueryBuilder {
	q.limit = limit
	return q
}

// Offset sets the OFFSET clause.
func (q *QueryBuilder) Offset(offset int64) *QueryBuilder {
	q.offset = offset
	return q
}

// BuildSelect builds a SELECT query string.
func (q *QueryBuilder) BuildSelect() (string, []interface{}) {
	var sb strings.Builder

	// SELECT
	sb.WriteString("SELECT ")
	if len(q.selectCols) > 0 {
		sb.WriteString(strings.Join(q.selectCols, ", "))
	} else {
		sb.WriteString("*")
	}

	// FROM
	sb.WriteString(" FROM ")
	sb.WriteString(q.table)

	// WHERE
	if len(q.whereParts) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(q.whereParts, " AND "))
	}

	// ORDER BY
	if q.orderBy != "" {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(q.orderBy)
	}

	// LIMIT
	if q.limit > 0 {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", q.limit))
	}

	// OFFSET
	if q.offset > 0 {
		sb.WriteString(fmt.Sprintf(" OFFSET %d", q.offset))
	}

	return sb.String(), q.whereArgs
}

// BuildCount builds a COUNT(*) query string.
func (q *QueryBuilder) BuildCount() (string, []interface{}) {
	var sb strings.Builder

	sb.WriteString("SELECT COUNT(*) FROM ")
	sb.WriteString(q.table)

	if len(q.whereParts) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(q.whereParts, " AND "))
	}

	return sb.String(), q.whereArgs
}

// BuildInsert builds an INSERT query string.
func BuildInsert(table string, columns []string) string {
	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)
}

// BuildInsertBatch builds an INSERT query for batch operations (no VALUES, for PrepareBatch).
func BuildInsertBatch(table string, columns []string) string {
	return fmt.Sprintf(
		"INSERT INTO %s (%s)",
		table,
		strings.Join(columns, ", "),
	)
}

// ParseSort parses a sort string (e.g., "timestamp:desc") into ORDER BY format.
func ParseSort(sort string, defaultField string, defaultOrder string) string {
	if sort == "" {
		if defaultField == "" {
			return ""
		}
		return fmt.Sprintf("%s %s", defaultField, defaultOrder)
	}

	parts := strings.Split(sort, ":")
	if len(parts) != 2 {
		return fmt.Sprintf("%s %s", defaultField, defaultOrder)
	}

	field := parts[0]
	direction := strings.ToUpper(parts[1])

	if direction != "ASC" && direction != "DESC" {
		direction = defaultOrder
	}

	return fmt.Sprintf("%s %s", field, direction)
}
