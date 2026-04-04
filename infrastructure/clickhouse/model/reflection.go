package chModel

import (
	"reflect"
	"strings"
	"time"
)

var (
	typeTime = reflect.TypeOf(time.Time{})
)

// extractFields extracts field metadata from a struct type using reflection.
// It looks for the "ch" tag to determine the ClickHouse column name.
// Falls back to "json" tag if "ch" is not present.
//
// Example struct:
//
//	type Event struct {
//	    Timestamp time.Time              `ch:"timestamp"`
//	    OrgId     string                 `ch:"org_id"`
//	    Payload   map[string]interface{} `ch:"payload"`  // Will be marshaled as JSON
//	}
func extractFields[T any]() ([]fieldInfo, error) {
	var zero T
	t := reflect.TypeOf(zero)

	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil, ErrInvalidType
	}

	fields := make([]fieldInfo, 0, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)

		// Skip unexported fields
		if !sf.IsExported() {
			continue
		}

		// Get column name from ch tag, fallback to json tag
		column := sf.Tag.Get(TagName)
		if column == "" {
			column = sf.Tag.Get(JSONTagName)
			if column != "" {
				// Extract first part before comma (e.g., "field,omitempty" -> "field")
				if idx := strings.Index(column, ","); idx != -1 {
					column = column[:idx]
				}
			}
		}

		// Skip fields without tags
		if column == "" || column == "-" {
			continue
		}

		// Determine if field needs JSON marshaling
		isJSON := false
		fieldType := sf.Type
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() == reflect.Map || fieldType.Kind() == reflect.Slice {
			// Exception: []byte is not JSON
			if fieldType.Kind() == reflect.Slice && fieldType.Elem().Kind() == reflect.Uint8 {
				isJSON = false
			} else if fieldType.Kind() == reflect.Map && isNumericKey(fieldType.Key()) {
				// Exception: Map with numeric keys (e.g., map[uint16]float64) is native ClickHouse Map type
				isJSON = false
			} else {
				isJSON = true
			}
		}

		fields = append(fields, fieldInfo{
			Name:      sf.Name,
			Column:    column,
			Index:     i,
			IsJSON:    isJSON,
			IsPointer: sf.Type.Kind() == reflect.Ptr,
			IsTime:    sf.Type == typeTime || (sf.Type.Kind() == reflect.Ptr && sf.Type.Elem() == typeTime),
		})
	}

	if len(fields) == 0 {
		return nil, ErrNoFields
	}

	return fields, nil
}

// isNumericKey returns true if the reflect.Type is a numeric kind (uint8, uint16, uint32, uint64, int, etc.).
// Used to detect ClickHouse native Map types like Map(UInt16, Float64).
func isNumericKey(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Int, reflect.Uint:
		return true
	}
	return false
}

// getFieldValue extracts a field value from a struct by index.
func getFieldValue(v reflect.Value, index int) reflect.Value {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return v.Field(index)
}

// setFieldValue sets a field value in a struct by index.
func setFieldValue(v reflect.Value, index int, value interface{}) {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	field := v.Field(index)
	if field.CanSet() {
		field.Set(reflect.ValueOf(value))
	}
}

// getColumnNames returns the ClickHouse column names from field metadata.
func getColumnNames(fields []fieldInfo) []string {
	columns := make([]string, len(fields))
	for i, f := range fields {
		columns[i] = f.Column
	}
	return columns
}

// buildInsertColumns builds the column list for INSERT statement.
func buildInsertColumns(fields []fieldInfo) string {
	columns := getColumnNames(fields)
	return strings.Join(columns, ", ")
}

// buildPlaceholders builds the placeholder list for INSERT statement.
func buildPlaceholders(count int) string {
	placeholders := make([]string, count)
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return strings.Join(placeholders, ", ")
}
