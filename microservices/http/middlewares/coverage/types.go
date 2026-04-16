package middlewaresCoverage

import (
	"time"
)

// UserAccess represents the complete access information for a user.
// This is a copy of the domain model to avoid circular dependencies.
type UserAccess struct {
	UserID           string                   `json:"userId"`
	AccessibleOrgIds []string                 `json:"accessibleOrgIds"` // Flat list for quick queries
	Organizations    []OrganizationCoverage   `json:"organizations"`    // Detailed info
	LastUpdated      time.Time                `json:"lastUpdated"`
	Version          int                      `json:"version"`
}

// OrganizationCoverage represents an organization in the coverage cache
type OrganizationCoverage struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	PathKey      string   `json:"pathKey,omitempty"`
	Scope        string   `json:"scope"`        // "local", "recursive", or "inherited"
	MembershipID string   `json:"membershipId"` // ID of the membership that granted access
	RoleIDs      []string `json:"roleIds"`      // Role IDs for this membership
}
