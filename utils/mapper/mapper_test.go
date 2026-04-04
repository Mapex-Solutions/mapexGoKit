package mapper

import (
	"reflect"
	"testing"
	"time"

	model "github.com/Mapex-Solutions/mapexGoKit/infrastructure/mongodb/model"
	timeUtil "github.com/Mapex-Solutions/mapexGoKit/utils/time"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Test structs to simulate DTOs and Entities
type UserEntity struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Age       int       `json:"age"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (u *UserEntity) GetCreated() time.Time {
	return u.CreatedAt
}

func (u *UserEntity) GetUpdated() time.Time {
	return u.UpdatedAt
}

type UserDTO struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	Email     string             `json:"email"`
	Age       int                `json:"age"`
	CreatedAt *timeUtil.NullTime `json:"created_at"`
	UpdatedAt *timeUtil.NullTime `json:"updated_at"`
}

func (u *UserDTO) SetCreated(t *timeUtil.NullTime) {
	u.CreatedAt = t
}

func (u *UserDTO) SetUpdated(t *timeUtil.NullTime) {
	u.UpdatedAt = t
}

type SimpleEntity struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

type SimpleDTO struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// ============================================================================
//                    OBJECTID CONVERSION TEST STRUCTS
// ============================================================================

// Entity with ObjectId fields (simulates MongoDB entity)
type BusinessRuleEntity struct {
	ID         model.ObjectId   `bson:"_id"`
	Name       string           `bson:"name"`
	RuleId     model.ObjectId   `bson:"ruleId"`
	AssetIds   []model.ObjectId `bson:"assetIds"`
	ThreadIds  []string         `bson:"threadIds"`
	OrgID      *model.ObjectId  `bson:"orgId"`
	CustomerId *model.ObjectId  `bson:"customerId"`
}

// Response DTO with string fields (simulates API response)
type BusinessRuleResponse struct {
	ID         *string   `json:"id"`
	Name       *string   `json:"name"`
	RuleId     *string   `json:"ruleId"`
	AssetIds   *[]string `json:"assetIds"`
	ThreadIds  *[]string `json:"threadIds"`
	OrgID      *string   `json:"orgId"`
	CustomerId *string   `json:"customerId"`
}

// Create DTO with string fields (simulates API input)
type BusinessRuleCreate struct {
	Name       string   `json:"name"`
	RuleId     string   `json:"ruleId"`
	AssetIds   []string `json:"assetIds"`
	ThreadIds  []string `json:"threadIds"`
	OrgID      *string  `json:"orgId"`
	CustomerId *string  `json:"customerId"`
}

func TestDtoToEntity(t *testing.T) {
	tests := []struct {
		name        string
		dto         interface{}
		expected    interface{}
		expectError bool
	}{
		{
			name: "simple DTO to Entity conversion",
			dto: &SimpleDTO{
				Name:  "Test",
				Value: 42,
			},
			expected: &SimpleEntity{
				Name:  "Test",
				Value: 42,
			},
			expectError: false,
		},
		{
			name: "user DTO to Entity conversion",
			dto: &UserDTO{
				ID:    "123",
				Name:  "John Doe",
				Email: "john@example.com",
				Age:   30,
			},
			expected: &UserEntity{
				ID:    "123",
				Name:  "John Doe",
				Email: "john@example.com",
				Age:   30,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result interface{}
			var err error

			switch dto := tt.dto.(type) {
			case *SimpleDTO:
				result, err = DtoToEntity[SimpleDTO, SimpleEntity](dto)
			case *UserDTO:
				result, err = DtoToEntity[UserDTO, UserEntity](dto)
			default:
				t.Fatalf("unsupported DTO type: %T", dto)
			}

			if tt.expectError {
				if err == nil {
					t.Error("expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if !reflect.DeepEqual(result, tt.expected) {
					t.Errorf("expected %+v, got %+v", tt.expected, result)
				}
			}
		})
	}
}

func TestEntityToDto(t *testing.T) {
	now := time.Now()
	userEntity := &UserEntity{
		ID:        "123",
		Name:      "Jane Doe",
		Email:     "jane@example.com",
		Age:       25,
		CreatedAt: now,
		UpdatedAt: now.Add(time.Hour),
	}

	tests := []struct {
		name        string
		entity      interface{}
		expectError bool
		checkFields func(interface{}) error
	}{
		{
			name: "simple Entity to DTO conversion",
			entity: &SimpleEntity{
				Name:  "Test Entity",
				Value: 100,
			},
			expectError: false,
			checkFields: func(result interface{}) error {
				dto, ok := result.(*SimpleDTO)
				if !ok {
					t.Errorf("expected *SimpleDTO, got %T", result)
					return nil
				}
				if dto.Name != "Test Entity" {
					t.Errorf("expected Name 'Test Entity', got %s", dto.Name)
				}
				if dto.Value != 100 {
					t.Errorf("expected Value 100, got %d", dto.Value)
				}
				return nil
			},
		},
		{
			name:        "user Entity to DTO conversion with time normalization",
			entity:      userEntity,
			expectError: false,
			checkFields: func(result interface{}) error {
				dto, ok := result.(*UserDTO)
				if !ok {
					t.Errorf("expected *UserDTO, got %T", result)
					return nil
				}
				if dto.ID != "123" {
					t.Errorf("expected ID '123', got %s", dto.ID)
				}
				if dto.Name != "Jane Doe" {
					t.Errorf("expected Name 'Jane Doe', got %s", dto.Name)
				}
				if dto.Email != "jane@example.com" {
					t.Errorf("expected Email 'jane@example.com', got %s", dto.Email)
				}
				if dto.Age != 25 {
					t.Errorf("expected Age 25, got %d", dto.Age)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result interface{}
			var err error

			switch entity := tt.entity.(type) {
			case *SimpleEntity:
				result, err = EntityToDto[SimpleEntity, SimpleDTO](entity)
			case *UserEntity:
				result, err = EntityToDto[UserEntity, UserDTO](entity)
			default:
				t.Fatalf("unsupported Entity type: %T", entity)
			}

			if tt.expectError {
				if err == nil {
					t.Error("expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if tt.checkFields != nil {
					tt.checkFields(result)
				}
			}
		})
	}
}

func TestDtoToMap(t *testing.T) {
	tests := []struct {
		name        string
		dto         interface{}
		expected    map[string]interface{}
		expectError bool
	}{
		{
			name: "simple DTO to map",
			dto: &SimpleDTO{
				Name:  "Test Map",
				Value: 42,
			},
			expected: map[string]interface{}{
				"name":  "Test Map",
				"value": float64(42), // JSON unmarshaling converts numbers to float64
			},
			expectError: false,
		},
		{
			name: "user DTO to map",
			dto: &UserDTO{
				ID:    "456",
				Name:  "Alice",
				Email: "alice@example.com",
				Age:   28,
			},
			expected: map[string]interface{}{
				"id":         "456",
				"name":       "Alice",
				"email":      "alice@example.com",
				"age":        float64(28),
				"created_at": nil,
				"updated_at": nil,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]interface{}
			var err error

			switch dto := tt.dto.(type) {
			case *SimpleDTO:
				result, err = DtoToMap[SimpleDTO](dto)
			case *UserDTO:
				result, err = DtoToMap[UserDTO](dto)
			default:
				t.Fatalf("unsupported DTO type: %T", dto)
			}

			if tt.expectError {
				if err == nil {
					t.Error("expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if !reflect.DeepEqual(result, tt.expected) {
					t.Errorf("expected %+v, got %+v", tt.expected, result)
				}
			}
		})
	}
}

func TestRoundTripConversion(t *testing.T) {
	originalEntity := &SimpleEntity{
		Name:  "RoundTrip Test",
		Value: 999,
	}

	// Entity -> DTO -> Entity
	dto, err := EntityToDto[SimpleEntity, SimpleDTO](originalEntity)
	if err != nil {
		t.Fatalf("failed to convert entity to DTO: %v", err)
	}

	finalEntity, err := DtoToEntity[SimpleDTO, SimpleEntity](dto)
	if err != nil {
		t.Fatalf("failed to convert DTO back to entity: %v", err)
	}

	if !reflect.DeepEqual(originalEntity, finalEntity) {
		t.Errorf("roundtrip conversion failed: expected %+v, got %+v", originalEntity, finalEntity)
	}
}

// ============================================================================
//                    OBJECTID CONVERSION TESTS
// ============================================================================

func TestEntityToDtoWithOptions_ObjectIdToString(t *testing.T) {
	// Create test ObjectIds
	id := bson.NewObjectID()
	ruleId := bson.NewObjectID()
	assetId1 := bson.NewObjectID()
	assetId2 := bson.NewObjectID()
	orgId := bson.NewObjectID()
	customerId := bson.NewObjectID()

	entity := &BusinessRuleEntity{
		ID:         id,
		Name:       "Test Rule",
		RuleId:     ruleId,
		AssetIds:   []model.ObjectId{assetId1, assetId2},
		ThreadIds:  []string{"thread1", "thread2"},
		OrgID:      &orgId,
		CustomerId: &customerId,
	}

	t.Run("ObjectIdToString=true converts all ObjectId fields to strings", func(t *testing.T) {
		result, err := EntityToDtoWithOptions[BusinessRuleEntity, BusinessRuleResponse](entity, MapperOptions{
			ObjectIdToString: true,
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check RuleId conversion (ObjectId → *string)
		if result.RuleId == nil {
			t.Error("RuleId should not be nil")
		} else if *result.RuleId != ruleId.Hex() {
			t.Errorf("RuleId: expected %s, got %s", ruleId.Hex(), *result.RuleId)
		}

		// Check AssetIds conversion ([]ObjectId → *[]string)
		if result.AssetIds == nil {
			t.Error("AssetIds should not be nil")
		} else {
			assetIds := *result.AssetIds
			if len(assetIds) != 2 {
				t.Errorf("AssetIds: expected 2 items, got %d", len(assetIds))
			} else {
				if assetIds[0] != assetId1.Hex() {
					t.Errorf("AssetIds[0]: expected %s, got %s", assetId1.Hex(), assetIds[0])
				}
				if assetIds[1] != assetId2.Hex() {
					t.Errorf("AssetIds[1]: expected %s, got %s", assetId2.Hex(), assetIds[1])
				}
			}
		}

		// Check ThreadIds (already []string, should be preserved)
		if result.ThreadIds == nil {
			t.Error("ThreadIds should not be nil")
		} else {
			threadIds := *result.ThreadIds
			if len(threadIds) != 2 {
				t.Errorf("ThreadIds: expected 2 items, got %d", len(threadIds))
			}
		}

		// Check OrgID conversion (*ObjectId → *string)
		if result.OrgID == nil {
			t.Error("OrgID should not be nil")
		} else if *result.OrgID != orgId.Hex() {
			t.Errorf("OrgID: expected %s, got %s", orgId.Hex(), *result.OrgID)
		}

		// Check CustomerId conversion (*ObjectId → *string)
		if result.CustomerId == nil {
			t.Error("CustomerId should not be nil")
		} else if *result.CustomerId != customerId.Hex() {
			t.Errorf("CustomerId: expected %s, got %s", customerId.Hex(), *result.CustomerId)
		}
	})

	t.Run("ObjectIdToString=false does NOT convert ObjectId fields", func(t *testing.T) {
		result, err := EntityToDtoWithOptions[BusinessRuleEntity, BusinessRuleResponse](entity, MapperOptions{
			ObjectIdToString: false,
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Without ObjectIdToString, the RuleId should be nil or empty (copier can't convert types)
		if result.RuleId != nil && *result.RuleId != "" && *result.RuleId == ruleId.Hex() {
			t.Errorf("RuleId should NOT be converted when ObjectIdToString=false, but got %v", *result.RuleId)
		}
	})
}

func TestEntityToDtoWithOptions_EmptyObjectIds(t *testing.T) {
	// Entity with zero/empty ObjectIds
	entity := &BusinessRuleEntity{
		ID:         bson.ObjectID{},
		Name:       "Empty Test",
		RuleId:     bson.ObjectID{},
		AssetIds:   []model.ObjectId{},
		ThreadIds:  []string{},
		OrgID:      nil,
		CustomerId: nil,
	}

	result, err := EntityToDtoWithOptions[BusinessRuleEntity, BusinessRuleResponse](entity, MapperOptions{
		ObjectIdToString: true,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Zero ObjectId should result in nil or empty string
	if result.RuleId != nil && *result.RuleId != "" {
		t.Errorf("RuleId should be nil or empty for zero ObjectId, got %v", *result.RuleId)
	}

	// Empty slice should result in nil
	if result.AssetIds != nil && len(*result.AssetIds) > 0 {
		t.Errorf("AssetIds should be nil or empty, got %v", *result.AssetIds)
	}

	// Nil pointer should remain nil
	if result.OrgID != nil {
		t.Errorf("OrgID should be nil, got %v", *result.OrgID)
	}
}

func TestDtoToEntityWithOptions_StringToObjectId(t *testing.T) {
	// Create test ObjectIds for comparison
	ruleId := bson.NewObjectID()
	assetId1 := bson.NewObjectID()
	assetId2 := bson.NewObjectID()

	dto := &BusinessRuleCreate{
		Name:      "Test Rule",
		RuleId:    ruleId.Hex(),
		AssetIds:  []string{assetId1.Hex(), assetId2.Hex()},
		ThreadIds: []string{"thread1", "thread2"},
	}

	t.Run("StringToObjectId=true converts string fields to ObjectId", func(t *testing.T) {
		result, err := DtoToEntityWithOptions[BusinessRuleCreate, BusinessRuleEntity](dto, MapperOptions{
			StringToObjectId: true,
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check RuleId conversion (string → ObjectId)
		if result.RuleId.Hex() != ruleId.Hex() {
			t.Errorf("RuleId: expected %s, got %s", ruleId.Hex(), result.RuleId.Hex())
		}

		// Check AssetIds conversion ([]string → []ObjectId)
		if len(result.AssetIds) != 2 {
			t.Errorf("AssetIds: expected 2 items, got %d", len(result.AssetIds))
		} else {
			if result.AssetIds[0].Hex() != assetId1.Hex() {
				t.Errorf("AssetIds[0]: expected %s, got %s", assetId1.Hex(), result.AssetIds[0].Hex())
			}
			if result.AssetIds[1].Hex() != assetId2.Hex() {
				t.Errorf("AssetIds[1]: expected %s, got %s", assetId2.Hex(), result.AssetIds[1].Hex())
			}
		}

		// Check ThreadIds (string slice should be preserved)
		if len(result.ThreadIds) != 2 {
			t.Errorf("ThreadIds: expected 2 items, got %d", len(result.ThreadIds))
		}
	})
}

func TestEntityToDtoWithOptions_StringSliceToStringSlicePtr(t *testing.T) {
	entity := &BusinessRuleEntity{
		ID:        bson.NewObjectID(),
		Name:      "Test Rule",
		RuleId:    bson.NewObjectID(),
		ThreadIds: []string{"thread-1", "thread-2", "thread-3"},
	}

	result, err := EntityToDtoWithOptions[BusinessRuleEntity, BusinessRuleResponse](entity, MapperOptions{
		ObjectIdToString: true,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check ThreadIds conversion ([]string → *[]string)
	if result.ThreadIds == nil {
		t.Fatal("ThreadIds should not be nil")
	}

	threadIds := *result.ThreadIds
	if len(threadIds) != 3 {
		t.Errorf("ThreadIds: expected 3 items, got %d", len(threadIds))
	}

	expectedIds := []string{"thread-1", "thread-2", "thread-3"}
	for i, expected := range expectedIds {
		if threadIds[i] != expected {
			t.Errorf("ThreadIds[%d]: expected %s, got %s", i, expected, threadIds[i])
		}
	}
}

func TestDtoToEntityWithOptions_InvalidObjectId(t *testing.T) {
	dto := &BusinessRuleCreate{
		Name:     "Test Rule",
		RuleId:   "invalid-object-id",
		AssetIds: []string{"invalid1", "invalid2"},
	}

	result, err := DtoToEntityWithOptions[BusinessRuleCreate, BusinessRuleEntity](dto, MapperOptions{
		StringToObjectId: true,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalid ObjectId string should result in zero ObjectId
	if !result.RuleId.IsZero() {
		t.Errorf("RuleId should be zero for invalid string, got %s", result.RuleId.Hex())
	}

	// Invalid strings should be skipped
	if len(result.AssetIds) != 0 {
		t.Errorf("AssetIds should be empty for invalid strings, got %d items", len(result.AssetIds))
	}
}

// ============================================================================
//                              BENCHMARKS
// ============================================================================

func BenchmarkDtoToEntity(b *testing.B) {
	dto := &SimpleDTO{
		Name:  "Benchmark Test",
		Value: 12345,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := DtoToEntity[SimpleDTO, SimpleEntity](dto)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEntityToDto(b *testing.B) {
	entity := &SimpleEntity{
		Name:  "Benchmark Test",
		Value: 12345,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := EntityToDto[SimpleEntity, SimpleDTO](entity)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEntityToDtoWithOptions_ObjectIdToString(b *testing.B) {
	id := bson.NewObjectID()
	ruleId := bson.NewObjectID()
	assetId1 := bson.NewObjectID()
	assetId2 := bson.NewObjectID()
	orgId := bson.NewObjectID()

	entity := &BusinessRuleEntity{
		ID:        id,
		Name:      "Benchmark Test",
		RuleId:    ruleId,
		AssetIds:  []model.ObjectId{assetId1, assetId2},
		ThreadIds: []string{"thread1", "thread2"},
		OrgID:     &orgId,
	}

	opts := MapperOptions{ObjectIdToString: true}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := EntityToDtoWithOptions[BusinessRuleEntity, BusinessRuleResponse](entity, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDtoToMap(b *testing.B) {
	dto := &SimpleDTO{
		Name:  "Benchmark Test",
		Value: 12345,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := DtoToMap[SimpleDTO](dto)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ============================================================================
//                    MAP TO STRUCT TESTS
// ============================================================================

func TestMapToStruct(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		expected    interface{}
		expectError bool
	}{
		{
			name: "simple map to struct",
			input: map[string]interface{}{
				"name":  "Test Struct",
				"value": float64(42), // JSON numbers are float64
			},
			expected: &SimpleDTO{
				Name:  "Test Struct",
				Value: 42,
			},
			expectError: false,
		},
		{
			name: "user map to struct",
			input: map[string]interface{}{
				"id":    "123",
				"name":  "John Doe",
				"email": "john@example.com",
				"age":   float64(30),
			},
			expected: &UserDTO{
				ID:    "123",
				Name:  "John Doe",
				Email: "john@example.com",
				Age:   30,
			},
			expectError: false,
		},
		{
			name:        "empty map to struct",
			input:       map[string]interface{}{},
			expected:    &SimpleDTO{},
			expectError: false,
		},
		{
			name:        "nil map to struct",
			input:       nil,
			expected:    &SimpleDTO{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch expected := tt.expected.(type) {
			case *SimpleDTO:
				result, err := MapToStruct[SimpleDTO](tt.input)
				if tt.expectError {
					if err == nil {
						t.Error("expected error, but got none")
					}
				} else {
					if err != nil {
						t.Errorf("unexpected error: %v", err)
					}
					if !reflect.DeepEqual(result, expected) {
						t.Errorf("expected %+v, got %+v", expected, result)
					}
				}
			case *UserDTO:
				result, err := MapToStruct[UserDTO](tt.input)
				if tt.expectError {
					if err == nil {
						t.Error("expected error, but got none")
					}
				} else {
					if err != nil {
						t.Errorf("unexpected error: %v", err)
					}
					// Compare only non-pointer fields for UserDTO
					if result.ID != expected.ID || result.Name != expected.Name ||
						result.Email != expected.Email || result.Age != expected.Age {
						t.Errorf("expected %+v, got %+v", expected, result)
					}
				}
			}
		})
	}
}

func TestMapToStructFromAny(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expected    *SimpleDTO
		expectError bool
	}{
		{
			name: "map[string]interface{} to struct",
			input: map[string]interface{}{
				"name":  "From Any",
				"value": float64(100),
			},
			expected: &SimpleDTO{
				Name:  "From Any",
				Value: 100,
			},
			expectError: false,
		},
		{
			name: "struct to struct (via JSON)",
			input: SimpleDTO{
				Name:  "Source Struct",
				Value: 200,
			},
			expected: &SimpleDTO{
				Name:  "Source Struct",
				Value: 200,
			},
			expectError: false,
		},
		{
			name:        "nil input",
			input:       nil,
			expected:    &SimpleDTO{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MapToStructFromAny[SimpleDTO](tt.input)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if !reflect.DeepEqual(result, tt.expected) {
					t.Errorf("expected %+v, got %+v", tt.expected, result)
				}
			}
		})
	}
}

func TestMapToStruct_NestedStruct(t *testing.T) {
	type Address struct {
		Street string `json:"street"`
		City   string `json:"city"`
	}
	type Person struct {
		Name    string  `json:"name"`
		Address Address `json:"address"`
	}

	input := map[string]interface{}{
		"name": "John",
		"address": map[string]interface{}{
			"street": "123 Main St",
			"city":   "New York",
		},
	}

	expected := &Person{
		Name: "John",
		Address: Address{
			Street: "123 Main St",
			City:   "New York",
		},
	}

	result, err := MapToStruct[Person](input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %+v, got %+v", expected, result)
	}
}

func TestMapToStruct_WithSlice(t *testing.T) {
	type Item struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	type Container struct {
		Items []Item `json:"items"`
	}

	input := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"id": float64(1), "name": "Item 1"},
			map[string]interface{}{"id": float64(2), "name": "Item 2"},
		},
	}

	expected := &Container{
		Items: []Item{
			{ID: 1, Name: "Item 1"},
			{ID: 2, Name: "Item 2"},
		},
	}

	result, err := MapToStruct[Container](input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %+v, got %+v", expected, result)
	}
}

func TestMapToStruct_RoundTrip(t *testing.T) {
	original := &SimpleDTO{
		Name:  "RoundTrip Test",
		Value: 999,
	}

	// Struct → Map → Struct
	mapResult, err := DtoToMap(original)
	if err != nil {
		t.Fatalf("failed to convert struct to map: %v", err)
	}

	finalResult, err := MapToStruct[SimpleDTO](mapResult)
	if err != nil {
		t.Fatalf("failed to convert map back to struct: %v", err)
	}

	if !reflect.DeepEqual(original, finalResult) {
		t.Errorf("roundtrip conversion failed: expected %+v, got %+v", original, finalResult)
	}
}

func BenchmarkMapToStruct(b *testing.B) {
	input := map[string]interface{}{
		"name":  "Benchmark Test",
		"value": float64(12345),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := MapToStruct[SimpleDTO](input)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMapToStructFromAny(b *testing.B) {
	input := map[string]interface{}{
		"name":  "Benchmark Test",
		"value": float64(12345),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := MapToStructFromAny[SimpleDTO](input)
		if err != nil {
			b.Fatal(err)
		}
	}
}
