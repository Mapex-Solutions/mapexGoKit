package mapper

import (
	"reflect"
	"time"

	"github.com/jinzhu/copier"
	model "github.com/Mapex-Solutions/mapexGoKit/infrastructure/mongodb/model"
	serialize "github.com/Mapex-Solutions/mapexGoKit/utils/serialize"
	timeUtil "github.com/Mapex-Solutions/mapexGoKit/utils/time"
)

// MapperOptions configures the mapping behavior
type MapperOptions struct {
	// ObjectIdToString converts model.ObjectId fields to string in the destination
	// Use this when mapping Entity → Response DTO
	ObjectIdToString bool

	// StringToObjectId converts string fields to model.ObjectId in the destination
	// Use this when mapping DTO → Entity
	StringToObjectId bool
}

// DtoToEntity converts a DTO to an Entity using deep copy.
//
// Parameters:
//
//	dto - A pointer to the DTO to be converted.
//
// Returns:
//
//	A pointer to the newly created Entity of type E, or an error if the conversion fails.
func DtoToEntity[D any, E any](dto *D) (*E, error) {
	var entity E
	if err := copier.CopyWithOption(&entity, dto, copier.Option{DeepCopy: true}); err != nil {
		return nil, err
	}
	return &entity, nil
}

// DtoToEntityWithOptions converts a DTO to an Entity using deep copy with custom options.
//
// Parameters:
//
//	dto - A pointer to the DTO to be converted.
//	opts - MapperOptions to configure conversion behavior.
//
// Returns:
//
//	A pointer to the newly created Entity of type E, or an error if the conversion fails.
func DtoToEntityWithOptions[D any, E any](dto *D, opts MapperOptions) (*E, error) {
	var entity E

	// Build copier options with converters based on flags
	copierOpts := copier.Option{DeepCopy: true}

	if opts.StringToObjectId {
		copierOpts.Converters = append(copierOpts.Converters, getStringToObjectIdConverters()...)
	}

	if err := copier.CopyWithOption(&entity, dto, copierOpts); err != nil {
		return nil, err
	}

	// Post-process for fields that copier couldn't convert (slices, pointers)
	if opts.StringToObjectId {
		postProcessStringToObjectId(&entity, dto)
	}

	return &entity, nil
}

// EntityToDto converts an Entity into a DTO using deep copy.
// It also normalizes Created and Updated fields if they exist.
func EntityToDto[E any, D any](entity *E) (*D, error) {
	var dto D
	if err := copier.CopyWithOption(&dto, entity, copier.Option{DeepCopy: true}); err != nil {
		return nil, err
	}

	// Post-processing for common system fields: Created and Updated
	// Check each interface independently (not using switch to allow both to be set)
	if setter, ok := any(&dto).(interface{ SetCreated(*timeUtil.NullTime) }); ok {
		if c, ok := any(entity).(interface{ GetCreated() (t time.Time) }); ok {
			if !c.GetCreated().IsZero() {
				setter.SetCreated(&timeUtil.NullTime{Time: c.GetCreated()})
			}
		}
	}

	if setter, ok := any(&dto).(interface{ SetUpdated(*timeUtil.NullTime) }); ok {
		if u, ok := any(entity).(interface{ GetUpdated() (t time.Time) }); ok {
			if !u.GetUpdated().IsZero() {
				setter.SetUpdated(&timeUtil.NullTime{Time: u.GetUpdated()})
			}
		}
	}

	return &dto, nil
}

// EntityToDtoWithOptions converts an Entity into a DTO using deep copy with custom options.
// It also normalizes Created and Updated fields if they exist.
//
// Parameters:
//
//	entity - A pointer to the Entity to be converted.
//	opts - MapperOptions to configure conversion behavior.
//
// Returns:
//
//	A pointer to the newly created DTO of type D, or an error if the conversion fails.
func EntityToDtoWithOptions[E any, D any](entity *E, opts MapperOptions) (*D, error) {
	var dto D

	// Build copier options with converters based on flags
	copierOpts := copier.Option{DeepCopy: true}

	if opts.ObjectIdToString {
		copierOpts.Converters = append(copierOpts.Converters, getObjectIdToStringConverters()...)
	}

	if err := copier.CopyWithOption(&dto, entity, copierOpts); err != nil {
		return nil, err
	}

	// Post-process for fields that copier couldn't convert (slices, pointers)
	if opts.ObjectIdToString {
		postProcessObjectIdToString(&dto, entity)
	}

	// Post-processing for common system fields: Created and Updated
	// Check each interface independently (not using switch to allow both to be set)
	if setter, ok := any(&dto).(interface{ SetCreated(*timeUtil.NullTime) }); ok {
		if c, ok := any(entity).(interface{ GetCreated() (t time.Time) }); ok {
			if !c.GetCreated().IsZero() {
				setter.SetCreated(&timeUtil.NullTime{Time: c.GetCreated()})
			}
		}
	}

	if setter, ok := any(&dto).(interface{ SetUpdated(*timeUtil.NullTime) }); ok {
		if u, ok := any(entity).(interface{ GetUpdated() (t time.Time) }); ok {
			if !u.GetUpdated().IsZero() {
				setter.SetUpdated(&timeUtil.NullTime{Time: u.GetUpdated()})
			}
		}
	}

	return &dto, nil
}

// DtoToMap converts a DTO into a map representation.
//
// Parameters:
//
//	dto - A pointer to the DTO to be converted.
//
// Returns:
//
//	A map[string]interface{} representing the DTO, or an error if the conversion fails.
func DtoToMap[D any](dto *D) (map[string]interface{}, error) {
	raw, err := serialize.Marshal(dto)
	if err != nil {
		return nil, err
	}

	var nested map[string]any
	if err := serialize.Unmarshal(raw, &nested); err != nil {
		return nil, err
	}

	return nested, nil
}

// MapToStruct converts a map[string]interface{} into a struct.
// This is the inverse of DtoToMap - useful for converting API responses
// or dynamic data into typed structs.
//
// Example:
//
//	data := map[string]interface{}{"name": "Test", "value": 42}
//	result, err := mapper.MapToStruct[MyStruct](data)
//
// Parameters:
//
//	data - The map to be converted.
//
// Returns:
//
//	A pointer to the newly created struct of type T, or an error if the conversion fails.
func MapToStruct[T any](data map[string]interface{}) (*T, error) {
	raw, err := serialize.Marshal(data)
	if err != nil {
		return nil, err
	}

	var result T
	if err := serialize.Unmarshal(raw, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// MapToStructFromAny converts an interface{} (typically a map or struct) into a struct.
// Useful when the source type is unknown at compile time (e.g., interface{} from API).
//
// Example:
//
//	var response interface{} = apiCall()
//	result, err := mapper.MapToStructFromAny[MyStruct](response)
//
// Parameters:
//
//	data - The interface{} to be converted (should be a map or compatible type).
//
// Returns:
//
//	A pointer to the newly created struct of type T, or an error if the conversion fails.
func MapToStructFromAny[T any](data interface{}) (*T, error) {
	raw, err := serialize.Marshal(data)
	if err != nil {
		return nil, err
	}

	var result T
	if err := serialize.Unmarshal(raw, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ============================================================================
//                              TYPE CONVERTERS
// ============================================================================

// getObjectIdToStringConverters returns copier TypeConverters for ObjectId → string conversions.
// Handles direct field conversions:
//   - model.ObjectId → string
//   - model.ObjectId → *string
//
// Returns:
//
//	[]copier.TypeConverter - Slice of converters to be used with copier.CopyWithOption
func getObjectIdToStringConverters() []copier.TypeConverter {
	return []copier.TypeConverter{
		// ObjectId → string
		{
			SrcType: model.ObjectId{},
			DstType: "",
			Fn: func(src interface{}) (interface{}, error) {
				if oid, ok := src.(model.ObjectId); ok {
					if oid.IsZero() {
						return "", nil
					}
					return oid.Hex(), nil
				}
				return "", nil
			},
		},
		// ObjectId → *string
		{
			SrcType: model.ObjectId{},
			DstType: (*string)(nil),
			Fn: func(src interface{}) (interface{}, error) {
				if oid, ok := src.(model.ObjectId); ok {
					if oid.IsZero() {
						return (*string)(nil), nil
					}
					hex := oid.Hex()
					return &hex, nil
				}
				return (*string)(nil), nil
			},
		},
	}
}

// getStringToObjectIdConverters returns copier TypeConverters for string → ObjectId conversions.
// Handles direct field conversions:
//   - string → model.ObjectId
//   - *string → model.ObjectId
//
// Invalid ObjectId strings are silently converted to zero ObjectId.
//
// Returns:
//
//	[]copier.TypeConverter - Slice of converters to be used with copier.CopyWithOption
func getStringToObjectIdConverters() []copier.TypeConverter {
	return []copier.TypeConverter{
		// string → ObjectId
		{
			SrcType: "",
			DstType: model.ObjectId{},
			Fn: func(src interface{}) (interface{}, error) {
				if str, ok := src.(string); ok {
					if str == "" {
						return model.ObjectId{}, nil
					}
					oid, err := model.ToObjectID(str)
					if err != nil {
						return model.ObjectId{}, nil
					}
					return oid, nil
				}
				return model.ObjectId{}, nil
			},
		},
		// *string → ObjectId
		{
			SrcType: (*string)(nil),
			DstType: model.ObjectId{},
			Fn: func(src interface{}) (interface{}, error) {
				if strPtr, ok := src.(*string); ok && strPtr != nil {
					if *strPtr == "" {
						return model.ObjectId{}, nil
					}
					oid, err := model.ToObjectID(*strPtr)
					if err != nil {
						return model.ObjectId{}, nil
					}
					return oid, nil
				}
				return model.ObjectId{}, nil
			},
		},
	}
}

// ============================================================================
//                              POST-PROCESSORS
// ============================================================================

// postProcessObjectIdToString handles complex type conversions that copier TypeConverters
// cannot process automatically. Uses reflection to iterate struct fields and convert:
//   - []model.ObjectId → *[]string
//   - []model.ObjectId → []string
//   - *model.ObjectId → *string
//   - []string → *[]string
//
// Parameters:
//
//	dto - Pointer to the destination DTO struct
//	entity - Pointer to the source Entity struct
func postProcessObjectIdToString(dto interface{}, entity interface{}) {
	dtoVal := reflect.ValueOf(dto)
	entityVal := reflect.ValueOf(entity)

	if dtoVal.Kind() == reflect.Ptr {
		dtoVal = dtoVal.Elem()
	}
	if entityVal.Kind() == reflect.Ptr {
		entityVal = entityVal.Elem()
	}

	if dtoVal.Kind() != reflect.Struct || entityVal.Kind() != reflect.Struct {
		return
	}

	dtoType := dtoVal.Type()
	entityType := entityVal.Type()

	for i := 0; i < dtoVal.NumField(); i++ {
		dtoField := dtoVal.Field(i)
		dtoFieldType := dtoType.Field(i)

		if !dtoField.CanSet() {
			continue
		}

		// Find matching field in entity by name
		entityField := entityVal.FieldByName(dtoFieldType.Name)
		if !entityField.IsValid() {
			continue
		}

		entityFieldType, found := entityType.FieldByName(dtoFieldType.Name)
		if !found {
			continue
		}

		// Handle []ObjectId → *[]string
		if isObjectIdSlice(entityFieldType.Type) && isStringSlicePtr(dtoFieldType.Type) {
			convertObjectIdSliceToStringSlicePtr(entityField, dtoField)
			continue
		}

		// Handle []ObjectId → []string
		if isObjectIdSlice(entityFieldType.Type) && isStringSlice(dtoFieldType.Type) {
			convertObjectIdSliceToStringSlice(entityField, dtoField)
			continue
		}

		// Handle *ObjectId → *string
		if isObjectIdPtr(entityFieldType.Type) && isStringPtr(dtoFieldType.Type) {
			convertObjectIdPtrToStringPtr(entityField, dtoField)
			continue
		}

		// Handle []string → *[]string (copier doesn't convert slice to slice pointer)
		if isStringSlice(entityFieldType.Type) && isStringSlicePtr(dtoFieldType.Type) {
			convertStringSliceToStringSlicePtr(entityField, dtoField)
			continue
		}
	}
}

// postProcessStringToObjectId handles complex type conversions that copier TypeConverters
// cannot process automatically. Uses reflection to iterate struct fields and convert:
//   - []string → []model.ObjectId
//   - *[]string → []model.ObjectId
//
// Invalid ObjectId strings are silently skipped.
//
// Parameters:
//
//	entity - Pointer to the destination Entity struct
//	dto - Pointer to the source DTO struct
func postProcessStringToObjectId(entity interface{}, dto interface{}) {
	entityVal := reflect.ValueOf(entity)
	dtoVal := reflect.ValueOf(dto)

	if entityVal.Kind() == reflect.Ptr {
		entityVal = entityVal.Elem()
	}
	if dtoVal.Kind() == reflect.Ptr {
		dtoVal = dtoVal.Elem()
	}

	if entityVal.Kind() != reflect.Struct || dtoVal.Kind() != reflect.Struct {
		return
	}

	entityType := entityVal.Type()
	dtoType := dtoVal.Type()

	for i := 0; i < entityVal.NumField(); i++ {
		entityField := entityVal.Field(i)
		entityFieldType := entityType.Field(i)

		if !entityField.CanSet() {
			continue
		}

		// Find matching field in DTO by name
		dtoField := dtoVal.FieldByName(entityFieldType.Name)
		if !dtoField.IsValid() {
			continue
		}

		dtoFieldType, found := dtoType.FieldByName(entityFieldType.Name)
		if !found {
			continue
		}

		// Handle []string → []ObjectId
		if isStringSlice(dtoFieldType.Type) && isObjectIdSlice(entityFieldType.Type) {
			convertStringSliceToObjectIdSlice(dtoField, entityField)
			continue
		}

		// Handle *[]string → []ObjectId
		if isStringSlicePtr(dtoFieldType.Type) && isObjectIdSlice(entityFieldType.Type) {
			convertStringSlicePtrToObjectIdSlice(dtoField, entityField)
			continue
		}
	}
}

// ============================================================================
//                              TYPE CHECKERS
// ============================================================================

// isObjectIdSlice checks if the given type is a slice of ObjectId.
// Supports both mongo-driver v1 (primitive.ObjectID) and v2 (bson.ObjectID).
func isObjectIdSlice(t reflect.Type) bool {
	if t.Kind() != reflect.Slice {
		return false
	}
	elemStr := t.Elem().String()
	// Support both v1 (primitive.ObjectID) and v2 (bson.ObjectID)
	return elemStr == "primitive.ObjectID" || elemStr == "bson.ObjectID"
}

// isStringSlice checks if the given type is a slice of strings ([]string).
func isStringSlice(t reflect.Type) bool {
	return t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.String
}

// isStringSlicePtr checks if the given type is a pointer to a slice of strings (*[]string).
func isStringSlicePtr(t reflect.Type) bool {
	return t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Slice && t.Elem().Elem().Kind() == reflect.String
}

// isObjectIdPtr checks if the given type is a pointer to ObjectId (*ObjectId).
// Supports both mongo-driver v1 (primitive.ObjectID) and v2 (bson.ObjectID).
func isObjectIdPtr(t reflect.Type) bool {
	if t.Kind() != reflect.Ptr {
		return false
	}
	elemStr := t.Elem().String()
	return elemStr == "primitive.ObjectID" || elemStr == "bson.ObjectID"
}

// isStringPtr checks if the given type is a pointer to string (*string).
func isStringPtr(t reflect.Type) bool {
	return t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.String
}

// ============================================================================
//                              CONVERTERS
// ============================================================================

// convertObjectIdSliceToStringSlicePtr converts []ObjectId → *[]string.
// Each ObjectId is converted to its hex string representation.
// Empty or nil source slices result in no change to destination.
func convertObjectIdSliceToStringSlicePtr(src, dst reflect.Value) {
	if src.IsNil() || src.Len() == 0 {
		return
	}

	result := make([]string, src.Len())
	for i := 0; i < src.Len(); i++ {
		oid := src.Index(i).Interface().(model.ObjectId)
		result[i] = oid.Hex()
	}
	dst.Set(reflect.ValueOf(&result))
}

// convertObjectIdSliceToStringSlice converts []ObjectId → []string.
// Each ObjectId is converted to its hex string representation.
// Empty or nil source slices result in no change to destination.
func convertObjectIdSliceToStringSlice(src, dst reflect.Value) {
	if src.IsNil() || src.Len() == 0 {
		return
	}

	result := make([]string, src.Len())
	for i := 0; i < src.Len(); i++ {
		oid := src.Index(i).Interface().(model.ObjectId)
		result[i] = oid.Hex()
	}
	dst.Set(reflect.ValueOf(result))
}

// convertObjectIdPtrToStringPtr converts *ObjectId → *string.
// Zero ObjectIds result in no change to destination.
func convertObjectIdPtrToStringPtr(src, dst reflect.Value) {
	if src.IsNil() {
		return
	}

	oid := src.Elem().Interface().(model.ObjectId)
	if oid.IsZero() {
		return
	}

	hex := oid.Hex()
	dst.Set(reflect.ValueOf(&hex))
}

// convertStringSliceToStringSlicePtr converts []string → *[]string.
// Creates a copy of the source slice and assigns its pointer to destination.
// Empty or nil source slices result in no change to destination.
func convertStringSliceToStringSlicePtr(src, dst reflect.Value) {
	if src.IsNil() || src.Len() == 0 {
		return
	}

	result := make([]string, src.Len())
	for i := 0; i < src.Len(); i++ {
		result[i] = src.Index(i).String()
	}
	dst.Set(reflect.ValueOf(&result))
}

// convertStringSliceToObjectIdSlice converts []string → []ObjectId.
// Invalid ObjectId strings are silently skipped (not added to result).
// Empty or nil source slices result in no change to destination.
func convertStringSliceToObjectIdSlice(src, dst reflect.Value) {
	if src.IsNil() || src.Len() == 0 {
		return
	}

	result := make([]model.ObjectId, 0, src.Len())
	for i := 0; i < src.Len(); i++ {
		str := src.Index(i).String()
		if str != "" {
			oid, err := model.ToObjectID(str)
			if err == nil {
				result = append(result, oid)
			}
		}
	}
	dst.Set(reflect.ValueOf(result))
}

// convertStringSlicePtrToObjectIdSlice converts *[]string → []ObjectId.
// Invalid ObjectId strings are silently skipped (not added to result).
// Nil source pointer or empty slice results in no change to destination.
func convertStringSlicePtrToObjectIdSlice(src, dst reflect.Value) {
	if src.IsNil() {
		return
	}

	slice := src.Elem()
	if slice.IsNil() || slice.Len() == 0 {
		return
	}

	result := make([]model.ObjectId, 0, slice.Len())
	for i := 0; i < slice.Len(); i++ {
		str := slice.Index(i).String()
		if str != "" {
			oid, err := model.ToObjectID(str)
			if err == nil {
				result = append(result, oid)
			}
		}
	}
	dst.Set(reflect.ValueOf(result))
}
