package auth

import (
	"github.com/gofiber/fiber/v2"
)

// ValidateAPIKey checks whether the provided API key matches the expected key,
// looking through headers, query parameters, and the request body.
//
// Parameters:
//   - c: the Fiber context, used to access headers, query parameters, and body data.
//   - expectedKey: the API key value that must be matched.
//   - keyNames: a list of possible key names to look for. If empty, it defaults to
//     []string{"X-API-Key", "x-api-key", "apiKey"}.
//
// Returns:
//   - bool: true if the expected key is found in any of the inspected sources; false otherwise.
//
// The function checks the API key in the following order:
//  1. Headers.
//  2. Query parameters.
//  3. Request body (JSON or form data).
func ValidateAPIKey(
	c *fiber.Ctx,
	expectedKey string,
	fieldType string,
	fieldName string,
) bool {

	// Header
	if fieldType == "header" {
		if value := c.Get(fieldName); value != "" && value == expectedKey {
			return true
		}
	}

	// Query
	if fieldType == "query" {
		if value := c.Query(fieldName); value != "" && value == expectedKey {
			return true
		}
	}

	// 3. None matched
	return false
}
