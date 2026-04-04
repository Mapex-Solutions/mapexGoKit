package chModel

import (
	"strings"
	"testing"
)

/** NewQueryBuilder */

func TestNewQueryBuilder(t *testing.T) {
	qb := NewQueryBuilder("events")
	query, args := qb.BuildSelect()

	if query != "SELECT * FROM events" {
		t.Errorf("expected 'SELECT * FROM events', got %q", query)
	}
	if len(args) != 0 {
		t.Errorf("expected 0 args, got %d", len(args))
	}
}

/** Select */

func TestSelect_SingleColumn(t *testing.T) {
	query, _ := NewQueryBuilder("events").Select("timestamp").BuildSelect()
	if query != "SELECT timestamp FROM events" {
		t.Errorf("expected 'SELECT timestamp FROM events', got %q", query)
	}
}

func TestSelect_MultipleColumns(t *testing.T) {
	query, _ := NewQueryBuilder("events").Select("timestamp", "org_id", "payload").BuildSelect()
	if query != "SELECT timestamp, org_id, payload FROM events" {
		t.Errorf("expected 'SELECT timestamp, org_id, payload FROM events', got %q", query)
	}
}

func TestSelect_EmptyColumns_DefaultsStar(t *testing.T) {
	query, _ := NewQueryBuilder("events").Select().BuildSelect()
	if query != "SELECT * FROM events" {
		t.Errorf("expected 'SELECT * FROM events', got %q", query)
	}
}

/** Where */

func TestWhere_Equal(t *testing.T) {
	query, args := NewQueryBuilder("events").Where("org_id", OpEqual, "org-1").BuildSelect()
	if !strings.Contains(query, "WHERE org_id = ?") {
		t.Errorf("expected WHERE clause, got %q", query)
	}
	if len(args) != 1 || args[0] != "org-1" {
		t.Errorf("expected args ['org-1'], got %v", args)
	}
}

func TestWhere_NotEqual(t *testing.T) {
	query, _ := NewQueryBuilder("events").Where("status", OpNotEqual, "deleted").BuildSelect()
	if !strings.Contains(query, "WHERE status != ?") {
		t.Errorf("expected != operator, got %q", query)
	}
}

func TestWhere_GreaterThan(t *testing.T) {
	query, _ := NewQueryBuilder("events").Where("count", OpGreater, 10).BuildSelect()
	if !strings.Contains(query, "WHERE count > ?") {
		t.Errorf("expected > operator, got %q", query)
	}
}

func TestWhere_LessThanOrEqual(t *testing.T) {
	query, _ := NewQueryBuilder("events").Where("count", OpLessEqual, 100).BuildSelect()
	if !strings.Contains(query, "WHERE count <= ?") {
		t.Errorf("expected <= operator, got %q", query)
	}
}

func TestWhere_In(t *testing.T) {
	query, _ := NewQueryBuilder("events").Where("status", OpIn, []string{"active", "pending"}).BuildSelect()
	if !strings.Contains(query, "WHERE status IN (?)") {
		t.Errorf("expected IN clause, got %q", query)
	}
}

func TestWhere_NotIn(t *testing.T) {
	query, _ := NewQueryBuilder("events").Where("status", OpNotIn, []string{"deleted"}).BuildSelect()
	if !strings.Contains(query, "WHERE status NOT IN (?)") {
		t.Errorf("expected NOT IN clause, got %q", query)
	}
}

func TestWhere_Between(t *testing.T) {
	query, _ := NewQueryBuilder("events").Where("timestamp", OpBetween, "2024-01-01").BuildSelect()
	if !strings.Contains(query, "WHERE timestamp BETWEEN ? AND ?") {
		t.Errorf("expected BETWEEN clause, got %q", query)
	}
}

func TestWhere_MultipleConditions(t *testing.T) {
	query, args := NewQueryBuilder("events").
		Where("org_id", OpEqual, "org-1").
		Where("status", OpEqual, "active").
		BuildSelect()

	if !strings.Contains(query, "WHERE org_id = ? AND status = ?") {
		t.Errorf("expected AND-joined conditions, got %q", query)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

/** WhereFilter */

func TestWhereFilter_Basic(t *testing.T) {
	filter := Filter{Field: "org_id", Operator: OpEqual, Value: "org-1"}
	query, args := NewQueryBuilder("events").WhereFilter(filter).BuildSelect()

	if !strings.Contains(query, "WHERE org_id = ?") {
		t.Errorf("expected WHERE clause, got %q", query)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
}

func TestWhereFilter_Between(t *testing.T) {
	filter := Filter{
		Field:    "timestamp",
		Operator: OpBetween,
		Value:    "2024-01-01",
		EndValue: "2024-12-31",
	}
	query, args := NewQueryBuilder("events").WhereFilter(filter).BuildSelect()

	if !strings.Contains(query, "WHERE timestamp BETWEEN ? AND ?") {
		t.Errorf("expected BETWEEN clause, got %q", query)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args for BETWEEN, got %d", len(args))
	}
}

/** WhereFilters */

func TestWhereFilters_Multiple(t *testing.T) {
	filters := []Filter{
		{Field: "org_id", Operator: OpEqual, Value: "org-1"},
		{Field: "status", Operator: OpNotEqual, Value: "deleted"},
	}
	query, args := NewQueryBuilder("events").WhereFilters(filters).BuildSelect()

	if !strings.Contains(query, "AND") {
		t.Errorf("expected AND in query, got %q", query)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestWhereFilters_Empty(t *testing.T) {
	query, args := NewQueryBuilder("events").WhereFilters(nil).BuildSelect()
	if strings.Contains(query, "WHERE") {
		t.Errorf("expected no WHERE for empty filters, got %q", query)
	}
	if len(args) != 0 {
		t.Errorf("expected 0 args, got %d", len(args))
	}
}

/** WhereMap */

func TestWhereMap(t *testing.T) {
	m := Map{"org_id": "org-1"}
	query, args := NewQueryBuilder("events").WhereMap(m).BuildSelect()

	if !strings.Contains(query, "WHERE org_id = ?") {
		t.Errorf("expected WHERE clause, got %q", query)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
}

/** WhereLike */

func TestWhereLike(t *testing.T) {
	query, args := NewQueryBuilder("events").WhereLike("name", "%test%").BuildSelect()
	if !strings.Contains(query, "WHERE name LIKE ?") {
		t.Errorf("expected LIKE clause, got %q", query)
	}
	if len(args) != 1 || args[0] != "%test%" {
		t.Errorf("expected args ['%%test%%'], got %v", args)
	}
}

/** WhereRaw */

func TestWhereRaw(t *testing.T) {
	query, args := NewQueryBuilder("events").
		WhereRaw("org_id = ? AND timestamp > ?", "org-1", "2024-01-01").
		BuildSelect()

	if !strings.Contains(query, "WHERE org_id = ? AND timestamp > ?") {
		t.Errorf("expected raw WHERE, got %q", query)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

/** OrderBy */

func TestOrderBy(t *testing.T) {
	query, _ := NewQueryBuilder("events").OrderBy("timestamp DESC").BuildSelect()
	if !strings.Contains(query, "ORDER BY timestamp DESC") {
		t.Errorf("expected ORDER BY, got %q", query)
	}
}

func TestOrderBy_Empty(t *testing.T) {
	query, _ := NewQueryBuilder("events").BuildSelect()
	if strings.Contains(query, "ORDER BY") {
		t.Errorf("expected no ORDER BY, got %q", query)
	}
}

/** Limit */

func TestLimit(t *testing.T) {
	query, _ := NewQueryBuilder("events").Limit(50).BuildSelect()
	if !strings.Contains(query, "LIMIT 50") {
		t.Errorf("expected LIMIT 50, got %q", query)
	}
}

func TestLimit_Zero_NoClause(t *testing.T) {
	query, _ := NewQueryBuilder("events").Limit(0).BuildSelect()
	if strings.Contains(query, "LIMIT") {
		t.Errorf("expected no LIMIT for 0, got %q", query)
	}
}

/** Offset */

func TestOffset(t *testing.T) {
	query, _ := NewQueryBuilder("events").Offset(100).BuildSelect()
	if !strings.Contains(query, "OFFSET 100") {
		t.Errorf("expected OFFSET 100, got %q", query)
	}
}

func TestOffset_Zero_NoClause(t *testing.T) {
	query, _ := NewQueryBuilder("events").Offset(0).BuildSelect()
	if strings.Contains(query, "OFFSET") {
		t.Errorf("expected no OFFSET for 0, got %q", query)
	}
}

/** BuildSelect Full Query */

func TestBuildSelect_FullQuery(t *testing.T) {
	query, args := NewQueryBuilder("events").
		Select("timestamp", "org_id", "payload").
		Where("org_id", OpEqual, "org-1").
		Where("status", OpEqual, "active").
		OrderBy("timestamp DESC").
		Limit(25).
		Offset(50).
		BuildSelect()

	expected := "SELECT timestamp, org_id, payload FROM events WHERE org_id = ? AND status = ? ORDER BY timestamp DESC LIMIT 25 OFFSET 50"
	if query != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, query)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

/** BuildCount */

func TestBuildCount_NoWhere(t *testing.T) {
	query, args := NewQueryBuilder("events").BuildCount()
	if query != "SELECT COUNT(*) FROM events" {
		t.Errorf("expected 'SELECT COUNT(*) FROM events', got %q", query)
	}
	if len(args) != 0 {
		t.Errorf("expected 0 args, got %d", len(args))
	}
}

func TestBuildCount_WithWhere(t *testing.T) {
	query, args := NewQueryBuilder("events").
		Where("org_id", OpEqual, "org-1").
		BuildCount()

	if query != "SELECT COUNT(*) FROM events WHERE org_id = ?" {
		t.Errorf("expected COUNT with WHERE, got %q", query)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
}

/** BuildInsert */

func TestBuildInsert(t *testing.T) {
	result := BuildInsert("events", []string{"timestamp", "org_id", "payload"})
	expected := "INSERT INTO events (timestamp, org_id, payload) VALUES (?, ?, ?)"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

func TestBuildInsert_SingleColumn(t *testing.T) {
	result := BuildInsert("events", []string{"id"})
	expected := "INSERT INTO events (id) VALUES (?)"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

/** BuildInsertBatch */

func TestBuildInsertBatch(t *testing.T) {
	result := BuildInsertBatch("events", []string{"timestamp", "org_id", "payload"})
	expected := "INSERT INTO events (timestamp, org_id, payload)"
	if result != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, result)
	}
}

/** ParseSort */

func TestParseSort_ValidInput(t *testing.T) {
	result := ParseSort("timestamp:desc", "id", "ASC")
	if result != "timestamp DESC" {
		t.Errorf("expected 'timestamp DESC', got %q", result)
	}
}

func TestParseSort_AscInput(t *testing.T) {
	result := ParseSort("name:asc", "id", "DESC")
	if result != "name ASC" {
		t.Errorf("expected 'name ASC', got %q", result)
	}
}

func TestParseSort_EmptyUsesDefault(t *testing.T) {
	result := ParseSort("", "timestamp", "DESC")
	if result != "timestamp DESC" {
		t.Errorf("expected 'timestamp DESC', got %q", result)
	}
}

func TestParseSort_EmptyNoDefault(t *testing.T) {
	result := ParseSort("", "", "DESC")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestParseSort_InvalidFormat(t *testing.T) {
	result := ParseSort("justfield", "timestamp", "DESC")
	if result != "timestamp DESC" {
		t.Errorf("expected default 'timestamp DESC', got %q", result)
	}
}

func TestParseSort_InvalidDirection(t *testing.T) {
	result := ParseSort("field:invalid", "timestamp", "DESC")
	if result != "field DESC" {
		t.Errorf("expected 'field DESC' (fallback direction), got %q", result)
	}
}

func TestParseSort_TooManyParts(t *testing.T) {
	result := ParseSort("field:desc:extra", "timestamp", "ASC")
	if result != "timestamp ASC" {
		t.Errorf("expected default 'timestamp ASC', got %q", result)
	}
}

/** Chaining */

func TestChaining_Fluent(t *testing.T) {
	qb := NewQueryBuilder("events")
	result := qb.
		Select("id", "name").
		Where("active", OpEqual, true).
		OrderBy("name ASC").
		Limit(10).
		Offset(0)

	// Verify chaining returns same pointer
	if result != qb {
		t.Error("chained methods should return the same QueryBuilder instance")
	}
}
