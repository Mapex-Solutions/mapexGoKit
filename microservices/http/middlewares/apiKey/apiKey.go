// Package middlewaresAuth provides Fiber middleware for authenticating requests
// using either a symmetric JWT strategy or an OAuth2/JWKS strategy.
package middlewaresApiKeyAuth

import (
	"github.com/gofiber/fiber/v2"

	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/auth"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/response"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/status"
)

// AuthMiddleware is a Fiber middleware function that authenticates requests using an API key.
// The middleware validates the provided API key against a given key and returns a 401 Unauthorized
// status if the key is invalid. If the key is valid, the middleware allows the request to proceed
// to the next middleware/handler.
//
// The middleware uses the auth.ValidateAPIKey function to perform the API key validation.
//
// Parameters:
// - key: A string representing the API key to be validated.
//
// Returns:
// - A fiber.Handler function that can be used as a middleware in a Fiber application.
func ApiKeyAuthMiddleware(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// If no API key is configured, reject ALL requests immediately.
		// This prevents accidental access when the key is not set.
		if key == "" {
			return response.Custom(c, status.UNAUTHORIZED, []string{"Unauthorized: API Key not configured on server"})
		}

		allowed := auth.ValidateAPIKey(c, key, "header", "X-API-Key")

		if !allowed {
			return response.Custom(c, status.UNAUTHORIZED, []string{"Unauthorized: Invalid API Key"})
		}

		// Continue to next middleware/handler
		return c.Next()
	}
}
