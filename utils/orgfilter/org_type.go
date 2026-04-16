package orgfilter

import (
	"errors"
	"strings"
)

// OrgType represents the hierarchical type of an organization based on pathKey depth.
type OrgType string

const (
	OrgTypeVendor   OrgType = "vendor"   // Depth 1: "000001/"
	OrgTypeCustomer OrgType = "customer" // Depth 2: "000001/0001/"
	OrgTypeSite     OrgType = "site"     // Depth 3: "000001/0001/0003/"
	OrgTypeOther    OrgType = "other"    // Depth 4+: building, zone, etc.
	OrgTypeUnknown  OrgType = "unknown"  // Invalid or empty pathKey
)

// GetOrgTypeFromPathKey determines the organization type based on pathKey depth.
//
// PathKey format examples:
//   - "000001/" → vendor (depth 1)
//   - "000001/0001/" → customer (depth 2)
//   - "000001/0001/0003/" → site (depth 3)
//   - "000001/0001/0003/0004/" → other (depth 4+, building/zone/etc)
//
// Returns:
//   - OrgTypeVendor: First level organization
//   - OrgTypeCustomer: Second level organization
//   - OrgTypeSite: Third level organization
//   - OrgTypeOther: Fourth+ level organization (building, zone, etc.)
//   - OrgTypeUnknown: Invalid or empty pathKey
//
// Example usage:
//
//	orgType := orgfilter.GetOrgTypeFromPathKey("000001/0001/")
//	// Returns: OrgTypeCustomer
func GetOrgTypeFromPathKey(pathKey string) OrgType {
	trimmed := strings.TrimSuffix(pathKey, "/")
	if trimmed == "" {
		return OrgTypeUnknown
	}

	parts := strings.Split(trimmed, "/")
	depth := len(parts)

	switch depth {
	case 1:
		return OrgTypeVendor
	case 2:
		return OrgTypeCustomer
	case 3:
		return OrgTypeSite
	default:
		if depth > 3 {
			return OrgTypeOther
		}
		return OrgTypeUnknown
	}
}

// CanCreateTemplate checks if an organization type is allowed to create templates.
// Only vendors and customers can create templates.
//
// Business rule: Sites, buildings, zones, and other lower-level organizations
// cannot create templates. They can only create local resources.
//
// Returns:
//   - true: If orgType is vendor or customer (allowed to create templates)
//   - false: If orgType is site, other, or unknown (not allowed)
//
// Example usage:
//
//	orgType := orgfilter.GetOrgTypeFromPathKey(pathKey)
//	if !orgfilter.CanCreateTemplate(orgType) {
//	    return errors.New("only vendors and customers can create templates")
//	}
func CanCreateTemplate(orgType OrgType) bool {
	return orgType == OrgTypeVendor || orgType == OrgTypeCustomer
}

// ValidateTemplateCreation validates if the current organization can create a template.
// This is a convenience function that combines GetOrgTypeFromPathKey and CanCreateTemplate.
//
// Returns error if organization type cannot create templates.
//
// Example usage:
//
//	if err := orgfilter.ValidateTemplateCreation(requestContext.OrgContextData.PathKey); err != nil {
//	    return nil, err
//	}
func ValidateTemplateCreation(pathKey string) error {
	orgType := GetOrgTypeFromPathKey(pathKey)

	if !CanCreateTemplate(orgType) {
		return errors.New("only vendors and customers can create templates")
	}

	return nil
}
