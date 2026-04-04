package refreshTokenExtractor

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

const (
	RefreshTokenLocalKey   = "refreshToken"
	RefreshTokenHeaderName = "X-Refresh-Token"
)

// RefreshTokenExtractor extracts the refresh token from the request header
// and stores it in Fiber's Locals for later retrieval in the handler.
//
// Expected header format:
//
//	X-Refresh-Token: <token>
//	or
//	X-Refresh-Token: Bearer <token>
//
// Notes:
//   - This middleware does NOT validate the token; it only extracts and stores it.
//   - Apply only to routes that require a refresh token (e.g., /auth/refresh).
func RefreshTokenExtractor() fiber.Handler {
	return func(c *fiber.Ctx) error {
		h := c.Get(RefreshTokenHeaderName)

		if h != "" {
			// support raw token or "Bearer <token>"
			if strings.HasPrefix(strings.ToLower(h), "bearer ") && len(h) > 7 {
				c.Locals(RefreshTokenLocalKey, h[7:])
			} else {
				c.Locals(RefreshTokenLocalKey, h)
			}
		}
		return c.Next()
	}
}
