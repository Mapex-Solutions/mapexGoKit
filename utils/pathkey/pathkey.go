// Package pathkey provides utilities for working with hierarchical pathKeys
// used in the multi-tenant organization structure.
//
// PathKey format: "000001/000002/0003"
// - Each segment is Base36 encoded
// - Segments are separated by "/"
// - Width varies by org type (vendor/customer: 6, site/building: 4, floor/zone: 3)
package pathkey

import (
	"strconv"
	"strings"
)

// CalculateNextSiblingPathKey calculates the next sibling pathKey for range queries.
// This is used to create the upper bound ($lt) in MongoDB range queries.
//
// The function increments the last segment of the pathKey by 1 in Base36.
//
// Examples:
//   - "000001/000001/0001" → "000001/000001/0002"
//   - "000001/00000Z" → "000001/000010"
//   - "000001/000001/000Z" → "000001/000001/0010"
//
// Usage in MongoDB query:
//
//	filters["pathKey"] = map[string]any{
//	    "$gte": org.PathKey,                            // "000001/000001/0001"
//	    "$lt":  CalculateNextSiblingPathKey(org.PathKey), // "000001/000001/0002"
//	}
//
// This query efficiently selects the org and ALL its descendants without using regex.
//
// Parameters:
//   - pathKey: The pathKey to increment (e.g., "000001/000001/0001")
//
// Returns:
//   - string: The next sibling pathKey (e.g., "000001/000001/0002")
func CalculateNextSiblingPathKey(pathKey string) string {
	if pathKey == "" {
		return ""
	}

	// Split pathKey into segments
	segments := strings.Split(pathKey, "/")
	if len(segments) == 0 {
		return pathKey
	}

	// Get last segment
	lastSegment := segments[len(segments)-1]
	segmentWidth := len(lastSegment)

	// Convert from Base36 to int
	num, err := strconv.ParseInt(lastSegment, 36, 64)
	if err != nil {
		// If conversion fails, return original pathKey
		return pathKey
	}

	// Increment
	num++

	// Convert back to Base36 with same width (zero-padded)
	nextSegment := strings.ToUpper(strconv.FormatInt(num, 36))

	// Pad with zeros to maintain width
	if len(nextSegment) < segmentWidth {
		nextSegment = strings.Repeat("0", segmentWidth-len(nextSegment)) + nextSegment
	}

	// Replace last segment
	segments[len(segments)-1] = nextSegment

	// Join back
	return strings.Join(segments, "/")
}

// IsDescendant checks if childPathKey is a descendant of parentPathKey.
//
// Examples:
//   - IsDescendant("000001/000001/0001", "000001") → true
//   - IsDescendant("000001/000001/0001", "000001/000001") → true
//   - IsDescendant("000001/000001/0001", "000001/000002") → false
//
// Parameters:
//   - childPathKey: The child pathKey to check
//   - parentPathKey: The parent pathKey
//
// Returns:
//   - bool: true if child is descendant of parent
func IsDescendant(childPathKey, parentPathKey string) bool {
	if parentPathKey == "" {
		return true // Empty parent means root - everything is descendant
	}

	// Child must start with parent pathKey
	if !strings.HasPrefix(childPathKey, parentPathKey) {
		return false
	}

	// Child must be longer than parent (not equal)
	return len(childPathKey) > len(parentPathKey)
}

// IsDescendantOrSelf checks if childPathKey is a descendant of parentPathKey OR equal to it.
//
// Examples:
//   - IsDescendantOrSelf("000001/000001/0001", "000001") → true
//   - IsDescendantOrSelf("000001/000001", "000001/000001") → true (same)
//   - IsDescendantOrSelf("000001/000001", "000001/000002") → false
//
// Parameters:
//   - childPathKey: The child pathKey to check
//   - parentPathKey: The parent pathKey
//
// Returns:
//   - bool: true if child is descendant of parent or equals parent
func IsDescendantOrSelf(childPathKey, parentPathKey string) bool {
	if childPathKey == parentPathKey {
		return true
	}
	return IsDescendant(childPathKey, parentPathKey)
}

// GetAncestorPaths returns all ancestor pathKeys for a given pathKey.
// This is used for hierarchical queries to find resources that should be inherited.
//
// Examples:
//   - GetAncestorPaths("000001") → []
//   - GetAncestorPaths("000001/000002") → ["000001"]
//   - GetAncestorPaths("000001/000002/0003") → ["000001", "000001/000002"]
//   - GetAncestorPaths("000001/000002/0003/0004") → ["000001", "000001/000002", "000001/000002/0003"]
//
// Usage in MongoDB query to find resources with scope: "global":
//
//	ancestors := pathkey.GetAncestorPaths(currentOrg.PathKey)
//	db.roles.find({
//	    $or: [
//	        { isSystem: true },
//	        { pathKey: currentPathKey },                    // Local resources
//	        { pathKey: { $in: ancestors }, scope: "global" } // Inherited resources
//	    ]
//	})
//
// Parameters:
//   - pathKey: The pathKey to get ancestors for (e.g., "000001/000002/0003")
//
// Returns:
//   - []string: Slice of ancestor pathKeys in order from root to parent
func GetAncestorPaths(pathKey string) []string {
	if pathKey == "" {
		return []string{}
	}

	segments := strings.Split(pathKey, "/")
	if len(segments) <= 1 {
		// Root level has no ancestors
		return []string{}
	}

	ancestors := make([]string, 0, len(segments)-1)

	// Build ancestors from root to parent
	for i := 1; i < len(segments); i++ {
		ancestorPath := strings.Join(segments[:i], "/")
		ancestors = append(ancestors, ancestorPath)
	}

	return ancestors
}
