// Package context defines request-scoped auth/coverage state used across
// Mapex microservices. These types represent WHO is making a request and
// which organizations they have access to — a concept shared by every Mapex
// project that consumes mapexGoKit.
package context

// RequestContext contains all context data injected by middleware for request processing.
// This is populated by the coverage middleware and passed to service layer.
//
// ALL microservices have access to the shared coverage cache, including:
//   - MapexOS
//   - Assets
//   - Router
//
// Usage in middleware:
//
//	reqContext := &RequestContext{
//	    ScopedOrgIds:   coverage.AccessibleOrgIds,
//	    CoverageOrgs:   coverageOrgs,
//	    OrgContext:     orgContextPtr,
//	    OrgContextData: orgContextData,
//	    UserId:         userId,
//	}
//	c.Locals("requestContext", reqContext)
//
// Usage in handler:
//
//	reqContext := c.Locals("requestContext").(*context.RequestContext)
type RequestContext struct {
	// ScopedOrgIds contains all organization IDs the user has access to.
	// This is extracted from the coverage cache.
	// Used for $in queries when no specific org context is provided.
	ScopedOrgIds []string `json:"scopedOrgIds"`

	// CoverageOrgs contains detailed organization data with pathKey.
	// This is extracted from the coverage cache and includes the hierarchical pathKey
	// for each accessible organization.
	CoverageOrgs []CoverageOrg `json:"coverageOrgs"`

	// OrgContext is the organization ID from the X-Org-Context header.
	// When provided, filters results to this specific organization (and optionally its children).
	// nil means no specific context (show all accessible orgs).
	OrgContext *string `json:"orgContext,omitempty"`

	// OrgContextData contains the detailed data for the organization specified in OrgContext.
	// Includes the pathKey which is essential for hierarchical queries.
	// nil when OrgContext is not provided.
	OrgContextData *CoverageOrg `json:"orgContextData,omitempty"`

	// UserId is the authenticated user's ID (from JWT token).
	UserId string `json:"userId"`
}

// CoverageOrg represents an organization with its hierarchical information.
// The PathKey field is critical for hierarchical queries and is already available
// in the coverage cache (no additional queries needed).
type CoverageOrg struct {
	// ID is the organization's unique identifier (MongoDB ObjectId as hex string)
	ID string `json:"id"`

	// Name is the organization's display name
	Name string `json:"name"`

	// Type is the organization type (customer, vendor, site, building, zone, etc.)
	Type string `json:"type"`

	// PathKey is the hierarchical path key for range queries.
	// Format: "000001/0001/0001/001"
	// This is already available in the coverage cache - no extra queries needed!
	PathKey string `json:"pathKey"`
}
