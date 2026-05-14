package orgfilter

import (
	"errors"
	"strings"

	ctx "github.com/Mapex-Solutions/mapexGoKit/microservices/common/context"
	model "github.com/Mapex-Solutions/mapexGoKit/infrastructure/mongodb/model"
	"github.com/Mapex-Solutions/mapexGoKit/utils/pathkey"
)

// BuildFilterParams contains the parameters needed to build an organization filter.
type BuildFilterParams struct {
	// ReqContext contains the request context with coverage data from middleware
	ReqContext *ctx.RequestContext

	// Query is any DTO that implements GetIncludeChildren() bool
	// Typically this is a query DTO that embeds BaseQueryDTO
	Query interface{ GetIncludeChildren() bool }
}

// BuildOrgFilter builds a MongoDB filter for organization-based queries.
// It supports three modes based on the request context and query parameters:
//
//  1. Org Context + Include Children = pathKey range query
//     Query: { "pathKey": { "$gte": "000001/0001", "$lt": "000001/0002" } }
//     Use case: User viewing customer and all child sites/buildings/zones
//
//  2. Org Context alone = specific orgId query
//     Query: { "orgId": ObjectId("...") }
//     Use case: User viewing only a specific organization
//
//  3. No context = all accessible orgs ($in query)
//     Query: { "orgId": { "$in": [ObjectId("..."), ...] } }
//     Use case: User viewing all organizations they have access to
//
// Returns an empty filter for super admin (no scoping needed).
//
// Example usage:
//
//	orgFilter, err := orgfilter.BuildOrgFilter(orgfilter.BuildFilterParams{
//	    ReqContext: reqContext,
//	    Query:      query,
//	})
//	if err != nil {
//	    return nil, err
//	}
//
//	// Merge into main filters
//	for k, v := range orgFilter {
//	    filters[k] = v
//	}
func BuildOrgFilter(params BuildFilterParams) (map[string]interface{}, error) {
	reqCtx := params.ReqContext
	includeChildren := false
	if params.Query != nil {
		includeChildren = params.Query.GetIncludeChildren()
	}

	// Case 1: Org Context + Include Children = pathKey range
	if reqCtx.OrgContext != nil && includeChildren {
		if reqCtx.OrgContextData == nil || reqCtx.OrgContextData.PathKey == "" {
			return nil, errors.New("org context data or pathKey missing")
		}

		pathKeyValue := reqCtx.OrgContextData.PathKey
		nextSibling := CalculateNextSiblingPathKey(pathKeyValue)

		return map[string]interface{}{
			"pathKey": map[string]interface{}{
				"$gte": pathKeyValue,
				"$lt":  nextSibling,
			},
		}, nil
	}

	// Case 2: Org Context alone = specific orgId
	if reqCtx.OrgContext != nil {
		orgObjectID, err := model.ToObjectID(*reqCtx.OrgContext)
		if err != nil {
			return nil, errors.New("invalid org context ID: " + err.Error())
		}

		return map[string]interface{}{
			"orgId": orgObjectID,
		}, nil
	}

	// Case 3: No context = all accessible orgs ($in)
	if len(reqCtx.ScopedOrgIds) > 0 {
		orgObjectIDs := make([]model.ObjectId, 0, len(reqCtx.ScopedOrgIds))
		for _, orgId := range reqCtx.ScopedOrgIds {
			objID, err := model.ToObjectID(orgId)
			if err != nil {
				// Skip invalid IDs but continue processing
				continue
			}
			orgObjectIDs = append(orgObjectIDs, objID)
		}

		if len(orgObjectIDs) == 0 {
			return nil, errors.New("no valid organization IDs in scope")
		}

		return map[string]interface{}{
			"orgId": map[string]interface{}{"$in": orgObjectIDs},
		}, nil
	}

	// No filtering (super admin - has access to everything)
	return map[string]interface{}{}, nil
}

// CalculateNextSiblingPathKey calculates the next sibling pathKey for range queries.
// This is used to create the upper bound ($lt) for pathKey range queries.
//
// PathKey format: "000001/0001/0002/003"
// Segments are zero-padded numbers separated by "/"
//
// Algorithm:
//  1. Split pathKey by "/"
//  2. Increment the last segment by 1
//  3. Zero-pad to maintain the same width
//  4. Join back with "/"
//
// Examples:
//   - "000001" -> "000002"
//   - "000001/0001" -> "000001/0002"
//   - "000001/0001/0002" -> "000001/0001/0003"
//   - "000001/0009" -> "000001/0010"
//   - "000999" -> "001000"
//
// Note: This function delegates to the existing pathkey util for consistency.
func CalculateNextSiblingPathKey(pathKeyValue string) string {
	if pathKeyValue == "" {
		return ""
	}

	// Delegate to existing pathkey utility
	return pathkey.CalculateNextSiblingPathKey(pathKeyValue)
}

// ValidateOrgContext validates that the org context is within the user's scoped org IDs.
// This should be called by middleware before injecting the request context.
//
// Returns true if valid, false otherwise.
func ValidateOrgContext(orgContext string, scopedOrgIds []string) bool {
	if orgContext == "" {
		return true // No context is always valid
	}

	for _, orgId := range scopedOrgIds {
		if orgId == orgContext {
			return true
		}
	}

	return false
}

// FindOrgInCoverage finds an organization by ID in the coverage data.
// Returns nil if not found.
//
// This is useful for extracting the org context data from coverage
// when processing the X-Org-Context header in middleware.
func FindOrgInCoverage(orgId string, coverageOrgs []ctx.CoverageOrg) *ctx.CoverageOrg {
	for i := range coverageOrgs {
		if coverageOrgs[i].ID == orgId {
			return &coverageOrgs[i]
		}
	}
	return nil
}

// BuildOrgFilterClickHouse builds a ClickHouse filter for organization-based queries.
// Unlike BuildOrgFilter (for MongoDB), this returns string IDs instead of ObjectIDs.
//
// It supports three modes based on the request context and query parameters:
//
//  1. Org Context + Include Children = pathKey range query
//     Filter: { "pathKey": { "$gte": "000001/0001", "$lt": "000001/0002" } }
//
//  2. Org Context alone = specific orgId query (STRING, not ObjectID)
//     Filter: { "orgId": "68f5bbce1aef22967c3ebb30" }
//
//  3. No context = all accessible orgs ($in query with STRING IDs)
//     Filter: { "orgId": { "$in": ["68f5bbce...", "68f5bbcf..."] } }
//
// Returns an empty filter for super admin (no scoping needed).
//
// Example usage:
//
//	orgFilter, err := orgfilter.BuildOrgFilterClickHouse(orgfilter.BuildFilterParams{
//	    ReqContext: reqContext,
//	    Query:      query,
//	})
func BuildOrgFilterClickHouse(params BuildFilterParams) (map[string]interface{}, error) {
	reqCtx := params.ReqContext
	includeChildren := false
	if params.Query != nil {
		includeChildren = params.Query.GetIncludeChildren()
	}

	// Case 1: Org Context + Include Children = pathKey range
	if reqCtx.OrgContext != nil && includeChildren {
		if reqCtx.OrgContextData == nil || reqCtx.OrgContextData.PathKey == "" {
			return nil, errors.New("org context data or pathKey missing")
		}

		pathKeyValue := reqCtx.OrgContextData.PathKey
		nextSibling := CalculateNextSiblingPathKey(pathKeyValue)

		return map[string]interface{}{
			"pathKey": map[string]interface{}{
				"$gte": pathKeyValue,
				"$lt":  nextSibling,
			},
		}, nil
	}

	// Case 2: Org Context alone = specific orgId (as STRING for ClickHouse)
	if reqCtx.OrgContext != nil {
		return map[string]interface{}{
			"orgId": *reqCtx.OrgContext, // String ID, not ObjectID
		}, nil
	}

	// Case 3: No context = all accessible orgs ($in with STRING IDs)
	if len(reqCtx.ScopedOrgIds) > 0 {
		// ScopedOrgIds is already []string, no conversion needed
		return map[string]interface{}{
			"orgId": map[string]interface{}{"$in": reqCtx.ScopedOrgIds},
		}, nil
	}

	// No filtering (super admin - has access to everything)
	return map[string]interface{}{}, nil
}

// BuildProjection builds a MongoDB projection map from a projection string.
// Projection string format: "field1,field2,field3"
//
// Returns a map where each field is set to 1 (include).
// Returns nil if projection string is empty.
//
// Example:
//
//	projection := BuildProjection("name,type,status")
//	// Returns: map[string]interface{}{"name": 1, "type": 1, "status": 1}
func BuildProjection(projectionStr *string) map[string]interface{} {
	if projectionStr == nil || *projectionStr == "" {
		return nil
	}

	fields := strings.Split(*projectionStr, ",")
	projection := make(map[string]interface{}, len(fields))

	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			projection[field] = 1
		}
	}

	if len(projection) == 0 {
		return nil
	}

	return projection
}

// ValidateOrgContextForNonSystem validates that org context and pathKey exist for non-system resources.
// This should be called when creating/updating template or local resources.
//
// Returns error if validation fails.
//
// Example usage:
//
//	if err := orgfilter.ValidateOrgContextForNonSystem(requestContext); err != nil {
//	    return nil, err
//	}
func ValidateOrgContextForNonSystem(reqContext *ctx.RequestContext) error {
	if reqContext.OrgContext == nil || *reqContext.OrgContext == "" {
		return errors.New("org context required for non-system resources")
	}

	if reqContext.OrgContextData == nil || reqContext.OrgContextData.PathKey == "" {
		return errors.New("org pathKey required for non-system resources")
	}

	return nil
}


// GetAncestorPathKeysIncludingSelf returns all ancestor pathKeys INCLUDING the current pathKey.
// This is used for template visibility queries.
//
// Examples:
//   - "000001" → ["000001"]
//   - "000001/0001" → ["000001", "000001/0001"]
//   - "000001/0001/0003" → ["000001", "000001/0001", "000001/0001/0003"]
//
// Parameters:
//   - pathKey: The pathKey to get ancestors for
//
// Returns:
//   - []string: Slice of ancestor pathKeys INCLUDING self (WITHOUT trailing slashes)
func GetAncestorPathKeysIncludingSelf(pathKey string) []string {
	if pathKey == "" {
		return []string{}
	}

	// Remove trailing slash for consistency (pathKeys in DB don't have trailing slashes)
	trimmed := strings.TrimSuffix(pathKey, "/")
	if trimmed == "" {
		return []string{}
	}

	parts := strings.Split(trimmed, "/")
	ancestors := make([]string, 0, len(parts))

	current := ""
	for i, part := range parts {
		if i > 0 {
			current += "/"
		}
		current += part
		ancestors = append(ancestors, current)
	}

	return ancestors
}

// BuildTemplateAncestorFilter builds a filter for templates created by ancestor organizations.
// This is used when user wants to see shared templates (isTemplate=true).
//
// Logic:
//   - Vendor logged in (depth=1, pathKey="000001"): Returns {"isTemplate": true, "pathKey": "000001"}
//   - Customer logged in (depth=2, pathKey="000001/0001"): Returns {"isTemplate": true, "pathKey": {"$in": ["000001", "000001/0001"]}}
//   - Site logged in (depth=3, pathKey="000001/0001/0003"): Returns {"isTemplate": true, "pathKey": {"$in": ["000001", "000001/0001"]}}
//
// Important: Sites and lower levels can ONLY see vendor and customer templates (NOT site templates).
// This is because only vendors and customers can CREATE templates (enforced by ValidateTemplateCreation).
//
// Parameters:
//   - reqContext: Request context with org access data
//
// Returns:
//   - map[string]interface{}: MongoDB filter for ancestor templates
//   - error: If org context is missing
//
// Example usage in service:
//
//	if query.IsTemplate != nil && *query.IsTemplate {
//	    templateFilter, err := orgfilter.BuildTemplateAncestorFilter(requestContext)
//	    if err != nil {
//	        return nil, err
//	    }
//	    filters = templateFilter
//	}
func BuildTemplateAncestorFilter(reqContext *ctx.RequestContext) (map[string]interface{}, error) {
	// Validate org context exists
	if reqContext.OrgContextData == nil || reqContext.OrgContextData.PathKey == "" {
		return nil, errors.New("org context required to filter templates")
	}

	pathKey := reqContext.OrgContextData.PathKey

	// Get organization type to determine ancestor scope
	orgType := GetOrgTypeFromPathKey(pathKey)

	switch orgType {
	case OrgTypeVendor:
		// Vendor: Can only see their own templates
		return map[string]interface{}{
			"isTemplate": true,
			"pathKey":    pathKey,
		}, nil

	case OrgTypeCustomer:
		// Customer: Can see vendor + customer templates
		ancestorPathKeys := GetAncestorPathKeysIncludingSelf(pathKey)
		return map[string]interface{}{
			"isTemplate": true,
			"pathKey":    map[string]interface{}{"$in": ancestorPathKeys},
		}, nil

	case OrgTypeSite, OrgTypeOther:
		// Site and lower: Can see vendor + customer templates ONLY (not site templates)
		// Because sites cannot create templates
		ancestorPathKeys := GetAncestorPathKeysIncludingSelf(pathKey)

		// Filter to keep only vendor and customer pathKeys (depth <= 2)
		vendorCustomerPathKeys := []string{}
		for _, pk := range ancestorPathKeys {
			pkType := GetOrgTypeFromPathKey(pk)
			if pkType == OrgTypeVendor || pkType == OrgTypeCustomer {
				vendorCustomerPathKeys = append(vendorCustomerPathKeys, pk)
			}
		}

		if len(vendorCustomerPathKeys) == 0 {
			return nil, errors.New("no vendor or customer ancestors found")
		}

		return map[string]interface{}{
			"isTemplate": true,
			"pathKey":    map[string]interface{}{"$in": vendorCustomerPathKeys},
		}, nil

	default:
		return nil, errors.New("invalid organization type")
	}
}
