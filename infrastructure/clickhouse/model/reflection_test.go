package chModel

import (
	"reflect"
	"testing"
	"time"
)

/** Test Structs */

type testEvent struct {
	Timestamp time.Time              `ch:"timestamp"`
	OrgId     string                 `ch:"org_id"`
	Payload   map[string]interface{} `ch:"payload"`
	Value     float64                `ch:"value"`
}

type testWithJSON struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type testMixed struct {
	ChField   string `ch:"ch_field"`
	JSONField string `json:"json_field"`
	NoTag     string
}

type testPointer struct {
	Value *string    `ch:"value"`
	Time  *time.Time `ch:"time"`
}

type testSlice struct {
	Tags   []string `ch:"tags"`
	Scores []int    `ch:"scores"`
	Bytes  []byte   `ch:"bytes"`
}

type testNumericMap struct {
	Buckets map[uint16]float64 `ch:"buckets"`
}

type testEmpty struct {
	unexported string
}

type testSkip struct {
	Active string `ch:"-"`
	Name   string `ch:"name"`
}

/** extractFields */

func TestExtractFields_ChTags(t *testing.T) {
	fields, err := extractFields[testEvent]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 4 {
		t.Fatalf("expected 4 fields, got %d", len(fields))
	}

	if fields[0].Column != "timestamp" {
		t.Errorf("expected column 'timestamp', got %q", fields[0].Column)
	}
	if fields[0].IsTime != true {
		t.Error("expected Timestamp field IsTime=true")
	}
	if fields[2].Column != "payload" {
		t.Errorf("expected column 'payload', got %q", fields[2].Column)
	}
	if !fields[2].IsJSON {
		t.Error("expected Payload field IsJSON=true")
	}
}

func TestExtractFields_JSONFallback(t *testing.T) {
	fields, err := extractFields[testWithJSON]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fields))
	}
	if fields[0].Column != "id" {
		t.Errorf("expected column 'id', got %q", fields[0].Column)
	}
	// "name,omitempty" should extract "name"
	if fields[1].Column != "name" {
		t.Errorf("expected column 'name' (stripped omitempty), got %q", fields[1].Column)
	}
}

func TestExtractFields_MixedTags(t *testing.T) {
	fields, err := extractFields[testMixed]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only ch_field and json_field should be extracted (NoTag has no tags)
	if len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fields))
	}
}

func TestExtractFields_PointerFields(t *testing.T) {
	fields, err := extractFields[testPointer]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fields))
	}
	if !fields[0].IsPointer {
		t.Error("expected *string to be IsPointer=true")
	}
	if !fields[1].IsTime {
		t.Error("expected *time.Time to be IsTime=true")
	}
	if !fields[1].IsPointer {
		t.Error("expected *time.Time to be IsPointer=true")
	}
}

func TestExtractFields_SliceFields(t *testing.T) {
	fields, err := extractFields[testSlice]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}
	// []string should be JSON
	if !fields[0].IsJSON {
		t.Error("expected []string to be IsJSON=true")
	}
	// []int should be JSON
	if !fields[1].IsJSON {
		t.Error("expected []int to be IsJSON=true")
	}
	// []byte should NOT be JSON (exception)
	if fields[2].IsJSON {
		t.Error("expected []byte to be IsJSON=false")
	}
}

func TestExtractFields_NumericMapKey(t *testing.T) {
	fields, err := extractFields[testNumericMap]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	// map[uint16]float64 should NOT be JSON (native ClickHouse Map type)
	if fields[0].IsJSON {
		t.Error("expected map[uint16]float64 to be IsJSON=false (native Map)")
	}
}

func TestExtractFields_NoExportedFields_Error(t *testing.T) {
	_, err := extractFields[testEmpty]()
	if err != ErrNoFields {
		t.Errorf("expected ErrNoFields, got %v", err)
	}
}

func TestExtractFields_NonStruct_Error(t *testing.T) {
	_, err := extractFields[string]()
	if err != ErrInvalidType {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}
}

func TestExtractFields_SkipDashTag(t *testing.T) {
	fields, err := extractFields[testSkip]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("expected 1 field (skipping ch:\"-\"), got %d", len(fields))
	}
	if fields[0].Column != "name" {
		t.Errorf("expected column 'name', got %q", fields[0].Column)
	}
}

/** isNumericKey */

func TestIsNumericKey_UintTypes(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeOf(uint8(0)),
		reflect.TypeOf(uint16(0)),
		reflect.TypeOf(uint32(0)),
		reflect.TypeOf(uint64(0)),
		reflect.TypeOf(uint(0)),
	}
	for _, typ := range types {
		if !isNumericKey(typ) {
			t.Errorf("expected %v to be numeric key", typ)
		}
	}
}

func TestIsNumericKey_IntTypes(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeOf(int8(0)),
		reflect.TypeOf(int16(0)),
		reflect.TypeOf(int32(0)),
		reflect.TypeOf(int64(0)),
		reflect.TypeOf(int(0)),
	}
	for _, typ := range types {
		if !isNumericKey(typ) {
			t.Errorf("expected %v to be numeric key", typ)
		}
	}
}

func TestIsNumericKey_NonNumeric(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeOf(""),
		reflect.TypeOf(false),
		reflect.TypeOf(float32(0)),
		reflect.TypeOf(float64(0)),
	}
	for _, typ := range types {
		if isNumericKey(typ) {
			t.Errorf("expected %v NOT to be numeric key", typ)
		}
	}
}

/** getFieldValue */

func TestGetFieldValue_Struct(t *testing.T) {
	event := testEvent{OrgId: "org-123"}
	v := reflect.ValueOf(event)
	result := getFieldValue(v, 1) // OrgId is index 1
	if result.String() != "org-123" {
		t.Errorf("expected 'org-123', got %q", result.String())
	}
}

func TestGetFieldValue_Pointer(t *testing.T) {
	event := &testEvent{OrgId: "org-ptr"}
	v := reflect.ValueOf(event)
	result := getFieldValue(v, 1)
	if result.String() != "org-ptr" {
		t.Errorf("expected 'org-ptr', got %q", result.String())
	}
}

/** setFieldValue */

func TestSetFieldValue_Struct(t *testing.T) {
	event := testEvent{}
	v := reflect.ValueOf(&event)
	setFieldValue(v, 1, "new-org")
	if event.OrgId != "new-org" {
		t.Errorf("expected 'new-org', got %q", event.OrgId)
	}
}

/** getColumnNames */

func TestGetColumnNames(t *testing.T) {
	fields := []fieldInfo{
		{Column: "timestamp"},
		{Column: "org_id"},
		{Column: "payload"},
	}
	cols := getColumnNames(fields)
	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(cols))
	}
	if cols[0] != "timestamp" || cols[1] != "org_id" || cols[2] != "payload" {
		t.Errorf("unexpected columns: %v", cols)
	}
}

/** buildInsertColumns */

func TestBuildInsertColumns(t *testing.T) {
	fields := []fieldInfo{
		{Column: "timestamp"},
		{Column: "org_id"},
	}
	result := buildInsertColumns(fields)
	if result != "timestamp, org_id" {
		t.Errorf("expected 'timestamp, org_id', got %q", result)
	}
}

/** buildPlaceholders */

func TestBuildPlaceholders(t *testing.T) {
	result := buildPlaceholders(3)
	if result != "?, ?, ?" {
		t.Errorf("expected '?, ?, ?', got %q", result)
	}
}

func TestBuildPlaceholders_Single(t *testing.T) {
	result := buildPlaceholders(1)
	if result != "?" {
		t.Errorf("expected '?', got %q", result)
	}
}

func TestBuildPlaceholders_Zero(t *testing.T) {
	result := buildPlaceholders(0)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}
