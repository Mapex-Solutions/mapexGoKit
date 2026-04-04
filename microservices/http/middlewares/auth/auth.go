// Package middlewaresAuth provides Fiber middleware for authenticating requests
// using either a symmetric JWT strategy or an OAuth2/JWKS strategy.
package middlewaresAuth

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	jwtv4 "github.com/golang-jwt/jwt/v4"
	jwtv5 "github.com/golang-jwt/jwt/v5"

	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/auth"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/response"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/status"
)

// convertMapClaimsV4ToV5 creates a shallow copy of jwtv4.MapClaims into
// jwtv5.MapClaims. It performs no type coercion or validation—this is
// strictly a map-to-map transfer to help interop when parsing with v4
// but handling claims as v5 in the rest of the code.
//
// Returns a new jwtv5.MapClaims with the same key/value pairs.
func convertMapClaimsV4ToV5(claims jwtv4.MapClaims) jwtv5.MapClaims {
	v5Claims := jwtv5.MapClaims{}
	for k, v := range claims {
		v5Claims[k] = v
	}
	return v5Claims
}

// AuthConfig controls how AuthMiddleware validates incoming requests.
// Expected fields (define this struct in your codebase):
//
//   type AuthConfig struct {
//       Strategy  string // "jwt" or "oauth2"
//       // For Strategy == "jwt":
//       Secret    string // HMAC/secret key
//       Algorithm string // e.g. "HS256"
//       // For Strategy == "oauth2":
//       JWKSURL   string // JWKS endpoint for key discovery
//   }
//
// Only the fields relevant to the selected Strategy are required.

// AuthMiddleware returns a Fiber middleware that validates the Authorization
// header as a Bearer token and, when valid, stores the token claims under
// c.Locals("user") as jwtv5.MapClaims.
//
// Behavior:
//   - If the Authorization header is missing: responds 401 with a JSON error.
//   - If Strategy == "jwt": verifies the token using a shared secret/algorithm.
//   - If Strategy == "oauth2": verifies the token using keys from the JWKS URL.
//   - On invalid tokens: responds 401 with a JSON error (includes details).
//   - On unknown strategy: responds 500 with a JSON error.
//   - On success: calls next handler with claims available via c.Locals("user").
//
// Example:
//
//	app := fiber.New()
//	app.Use(middlewaresAuth.AuthMiddleware(middlewaresAuth.AuthConfig{
//	    Strategy:  "jwt",
//	    Secret:    os.Getenv("JWT_SECRET"),
//	    Algorithm: "HS256",
//	}))
//
//	// or OAuth2/JWKS:
//	app.Use(middlewaresAuth.AuthMiddleware(middlewaresAuth.AuthConfig{
//	    Strategy: "oauth2",
//	    JWKSURL:  "https://issuer.example.com/.well-known/jwks.json",
//	}))
func AuthMiddleware(cfg AuthConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Custom(c, status.UNAUTHORIZED, []string{"missing Authorization header"})
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		var claims jwtv5.MapClaims
		var err error

		switch cfg.Strategy {
		case "jwt":
			_, claims, err = auth.ParseJWTTokenWithSecret(tokenString, cfg.Secret, cfg.Algorithm)

		case "oauth2":
			var v4Claims jwtv4.MapClaims
			_, v4Claims, err = auth.ParseJWTTokenWithJWKS(tokenString, cfg.JWKSURL)
			claims = convertMapClaimsV4ToV5(v4Claims)

		default:
			return response.Custom(c, status.UNAUTHORIZED, []string{"unsupported auth strategy"})
		}

		if err != nil {
			return response.Custom(
				c,
				status.UNAUTHORIZED,
				[]string{
					"invalid token",
					err.Error(),
				},
			)
		}

		// Make claims available to downstream handlers.
		c.Locals("user", claims)
		c.Locals("token", tokenString)

		return c.Next()
	}
}

// GetUserIdFromToken extracts the "userId" claim from the JWT stored
// in the Fiber context.
//
// This function expects that:
//   - The request has already passed through an authentication middleware
//     that parses the JWT and stores its claims in c.Locals("user").
//   - The stored claims are of type jwtv5.MapClaims.
//   - The "userId" claim exists and is a string.
//
// Parameters:
//   - c: *fiber.Ctx from which to retrieve the claims.
//
// Returns:
//   - the "userId" claim as a string and true if found, or empty string and false if not.
func GetUserIdFromToken(c *fiber.Ctx) (string, bool) {
	// Get the user claims from JWT
	claimsRaw := c.Locals("user")
	claims, ok := claimsRaw.(jwtv5.MapClaims)
	if !ok {
		return "", false
	}

	// Extract "userId" safely
	userIDVal, exists := claims["userId"]
	if !exists {
		return "", false
	}

	userID, ok := userIDVal.(string)
	if !ok || userID == "" {
		return "", false
	}

	return userID, true
}
