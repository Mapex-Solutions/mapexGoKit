package orgfilter

import (
	"testing"

	ctx "github.com/Mapex-Solutions/mapexGoKit/microservices/common/context"
	model "github.com/Mapex-Solutions/mapexGoKit/infrastructure/mongodb/model"
)

// Mock query type that implements GetIncludeChildren
type mockQuery struct {
	includeChildren bool
}

func (m *mockQuery) GetIncludeChildren() bool {
	return m.includeChildren
}

// TestBuildOrgFilter_Mode1_OrgContextWithChildren tests pathKey range query mode
func TestBuildOrgFilter_Mode1_OrgContextWithChildren(t *testing.T) {
	orgContext := "507f1f77bcf86cd799439011"
	reqContext := &ctx.RequestContext{
		OrgContext: &orgContext,
		OrgContextData: &ctx.CoverageOrg{
			ID:      orgContext,
			PathKey: "000001/0001",
		},
	}
	query := &mockQuery{includeChildren: true}

	result, err := BuildOrgFilter(BuildFilterParams{
		ReqContext: reqContext,
		Query:      query,
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should have pathKey range
	pathKeyFilter, ok := result["pathKey"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected pathKey filter to be map[string]interface{}, got %T", result["pathKey"])
	}

	if pathKeyFilter["$gte"] != "000001/0001" {
		t.Errorf("Expected $gte to be '000001/0001', got %v", pathKeyFilter["$gte"])
	}

	if pathKeyFilter["$lt"] != "000001/0002" {
		t.Errorf("Expected $lt to be '000001/0002', got %v", pathKeyFilter["$lt"])
	}
}

// TestBuildOrgFilter_Mode1_MissingPathKey tests error when pathKey is missing
func TestBuildOrgFilter_Mode1_MissingPathKey(t *testing.T) {
	orgContext := "507f1f77bcf86cd799439011"
	reqContext := &ctx.RequestContext{
		OrgContext:     &orgContext,
		OrgContextData: nil, // Missing context data
	}
	query := &mockQuery{includeChildren: true}

	_, err := BuildOrgFilter(BuildFilterParams{
		ReqContext: reqContext,
		Query:      query,
	})

	if err == nil {
		t.Fatal("Expected error when OrgContextData is missing, got nil")
	}

	expectedError := "org context data or pathKey missing"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

// TestBuildOrgFilter_Mode2_OrgContextWithoutChildren tests direct orgId query
func TestBuildOrgFilter_Mode2_OrgContextWithoutChildren(t *testing.T) {
	orgContext := "507f1f77bcf86cd799439011"
	reqContext := &ctx.RequestContext{
		OrgContext: &orgContext,
		OrgContextData: &ctx.CoverageOrg{
			ID:      orgContext,
			PathKey: "000001/0001",
		},
	}
	query := &mockQuery{includeChildren: false}

	result, err := BuildOrgFilter(BuildFilterParams{
		ReqContext: reqContext,
		Query:      query,
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should have direct orgId
	orgId, ok := result["orgId"].(model.ObjectId)
	if !ok {
		t.Fatalf("Expected orgId to be model.ObjectId, got %T", result["orgId"])
	}

	expectedOrgId, _ := model.ToObjectID(orgContext)
	if orgId != expectedOrgId {
		t.Errorf("Expected orgId to be %v, got %v", expectedOrgId, orgId)
	}
}

// TestBuildOrgFilter_Mode2_InvalidOrgId tests error for invalid org context ID
func TestBuildOrgFilter_Mode2_InvalidOrgId(t *testing.T) {
	invalidOrgContext := "invalid-id"
	reqContext := &ctx.RequestContext{
		OrgContext: &invalidOrgContext,
	}
	query := &mockQuery{includeChildren: false}

	_, err := BuildOrgFilter(BuildFilterParams{
		ReqContext: reqContext,
		Query:      query,
	})

	if err == nil {
		t.Fatal("Expected error for invalid org context ID, got nil")
	}

	if err.Error()[:24] != "invalid org context ID: " {
		t.Errorf("Expected error to start with 'invalid org context ID: ', got '%s'", err.Error())
	}
}

// TestBuildOrgFilter_Mode3_NoContextWithScopedOrgs tests $in query
func TestBuildOrgFilter_Mode3_NoContextWithScopedOrgs(t *testing.T) {
	reqContext := &ctx.RequestContext{
		OrgContext: nil,
		ScopedOrgIds: []string{
			"507f1f77bcf86cd799439011",
			"507f1f77bcf86cd799439012",
			"507f1f77bcf86cd799439013",
		},
	}
	query := &mockQuery{includeChildren: false}

	result, err := BuildOrgFilter(BuildFilterParams{
		ReqContext: reqContext,
		Query:      query,
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should have $in query
	orgIdFilter, ok := result["orgId"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected orgId filter to be map[string]interface{}, got %T", result["orgId"])
	}

	orgIds, ok := orgIdFilter["$in"].([]model.ObjectId)
	if !ok {
		t.Fatalf("Expected $in to be []model.ObjectId, got %T", orgIdFilter["$in"])
	}

	if len(orgIds) != 3 {
		t.Errorf("Expected 3 org IDs, got %d", len(orgIds))
	}
}

// TestBuildOrgFilter_Mode3_NoValidOrgIds tests error when all IDs are invalid
func TestBuildOrgFilter_Mode3_NoValidOrgIds(t *testing.T) {
	reqContext := &ctx.RequestContext{
		OrgContext: nil,
		ScopedOrgIds: []string{
			"invalid-id-1",
			"invalid-id-2",
		},
	}
	query := &mockQuery{includeChildren: false}

	_, err := BuildOrgFilter(BuildFilterParams{
		ReqContext: reqContext,
		Query:      query,
	})

	if err == nil {
		t.Fatal("Expected error when no valid org IDs, got nil")
	}

	expectedError := "no valid organization IDs in scope"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

// TestBuildOrgFilter_SuperAdmin tests empty filter for super admin
func TestBuildOrgFilter_SuperAdmin(t *testing.T) {
	reqContext := &ctx.RequestContext{
		OrgContext:   nil,
		ScopedOrgIds: []string{}, // Empty = super admin
	}
	query := &mockQuery{includeChildren: false}

	result, err := BuildOrgFilter(BuildFilterParams{
		ReqContext: reqContext,
		Query:      query,
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty filter for super admin, got %v", result)
	}
}

// TestCalculateNextSiblingPathKey tests pathKey sibling calculation
func TestCalculateNextSiblingPathKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Single segment", "000001", "000002"},
		{"Two segments", "000001/0001", "000001/0002"},
		{"Three segments", "000001/0001/0002", "000001/0001/0003"},
		{"Base36 - 9 to A", "000001/0009", "000001/000A"},
		{"Base36 - Z to 10", "00000Z", "000010"},
		{"Complex path", "000001/0002/0003/0004", "000001/0002/0003/0005"},
		{"Empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateNextSiblingPathKey(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestValidateOrgContext tests org context validation
func TestValidateOrgContext(t *testing.T) {
	scopedOrgIds := []string{
		"507f1f77bcf86cd799439011",
		"507f1f77bcf86cd799439012",
		"507f1f77bcf86cd799439013",
	}

	tests := []struct {
		name       string
		orgContext string
		expected   bool
	}{
		{"Valid org context", "507f1f77bcf86cd799439011", true},
		{"Another valid org", "507f1f77bcf86cd799439012", true},
		{"Invalid org context", "507f1f77bcf86cd799439999", false},
		{"Empty context (valid)", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateOrgContext(tt.orgContext, scopedOrgIds)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestFindOrgInCoverage tests finding org in coverage data
func TestFindOrgInCoverage(t *testing.T) {
	coverageOrgs := []ctx.CoverageOrg{
		{ID: "507f1f77bcf86cd799439011", Name: "Customer A", Type: "customer", PathKey: "000001"},
		{ID: "507f1f77bcf86cd799439012", Name: "Site B", Type: "site", PathKey: "000001/0001"},
		{ID: "507f1f77bcf86cd799439013", Name: "Building C", Type: "building", PathKey: "000001/0001/0001"},
	}

	t.Run("Find existing org", func(t *testing.T) {
		result := FindOrgInCoverage("507f1f77bcf86cd799439012", coverageOrgs)
		if result == nil {
			t.Fatal("Expected to find org, got nil")
		}
		if result.Name != "Site B" {
			t.Errorf("Expected name 'Site B', got '%s'", result.Name)
		}
		if result.PathKey != "000001/0001" {
			t.Errorf("Expected pathKey '000001/0001', got '%s'", result.PathKey)
		}
	})

	t.Run("Find non-existing org", func(t *testing.T) {
		result := FindOrgInCoverage("507f1f77bcf86cd799439999", coverageOrgs)
		if result != nil {
			t.Errorf("Expected nil for non-existing org, got %v", result)
		}
	})

	t.Run("Find first org", func(t *testing.T) {
		result := FindOrgInCoverage("507f1f77bcf86cd799439011", coverageOrgs)
		if result == nil {
			t.Fatal("Expected to find org, got nil")
		}
		if result.Type != "customer" {
			t.Errorf("Expected type 'customer', got '%s'", result.Type)
		}
	})
}

// TestBuildProjection tests projection string parsing
func TestBuildProjection(t *testing.T) {
	t.Run("Valid projection string", func(t *testing.T) {
		proj := "name,type,status"
		result := BuildProjection(&proj)

		if result == nil {
			t.Fatal("Expected projection map, got nil")
		}

		if len(result) != 3 {
			t.Errorf("Expected 3 fields, got %d", len(result))
		}

		if result["name"] != 1 {
			t.Error("Expected 'name' field to be 1")
		}
		if result["type"] != 1 {
			t.Error("Expected 'type' field to be 1")
		}
		if result["status"] != 1 {
			t.Error("Expected 'status' field to be 1")
		}
	})

	t.Run("Projection with spaces", func(t *testing.T) {
		proj := "name, type , status  "
		result := BuildProjection(&proj)

		if len(result) != 3 {
			t.Errorf("Expected 3 fields after trimming, got %d", len(result))
		}
	})

	t.Run("Empty projection string", func(t *testing.T) {
		proj := ""
		result := BuildProjection(&proj)

		if result != nil {
			t.Errorf("Expected nil for empty string, got %v", result)
		}
	})

	t.Run("Nil projection pointer", func(t *testing.T) {
		result := BuildProjection(nil)

		if result != nil {
			t.Errorf("Expected nil for nil pointer, got %v", result)
		}
	})

	t.Run("Projection with empty fields", func(t *testing.T) {
		proj := "name,,type,,,status"
		result := BuildProjection(&proj)

		if len(result) != 3 {
			t.Errorf("Expected 3 valid fields, got %d", len(result))
		}
	})

	t.Run("Single field projection", func(t *testing.T) {
		proj := "name"
		result := BuildProjection(&proj)

		if len(result) != 1 {
			t.Errorf("Expected 1 field, got %d", len(result))
		}

		if result["name"] != 1 {
			t.Error("Expected 'name' field to be 1")
		}
	})

	t.Run("Projection with only commas and spaces", func(t *testing.T) {
		proj := " , , , "
		result := BuildProjection(&proj)

		if result != nil {
			t.Errorf("Expected nil for projection with only separators, got %v", result)
		}
	})
}
