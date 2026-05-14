package middlewaresOrgHierarchy

// OrganizationCreateContract is the minimal interface that an organization-
// creation DTO must implement for the ValidateOrgHierarchy middleware to
// enforce hierarchy rules. Consumer projects implement this interface on
// their own concrete DTO (which may have additional project-specific fields
// like AuthConfig, AccessPolicy, validation tags, etc.).
//
// Usage in consumer project:
//
//	// On the consumer's own DTO type:
//	func (d *OrganizationCreate) GetType() string         { return d.Type }
//	func (d *OrganizationCreate) GetParentOrgID() *string { return d.ParentOrgID }
//
//	// When registering routes:
//	orgHierarchyMw.ValidateOrgHierarchy(func(c *fiber.Ctx) (orgHierarchyMw.OrganizationCreateContract, error) {
//	    return requestValidation.GetDTO[*orgDtos.OrganizationCreate](c, "bodyDTO")
//	}),
type OrganizationCreateContract interface {
	// GetType returns the organization type (e.g. "vendor", "customer", "site", "building", "floor", "zone").
	GetType() string

	// GetParentOrgID returns the parent organization's ID. nil indicates a
	// root-level organization (only valid for "vendor" type).
	GetParentOrgID() *string
}
