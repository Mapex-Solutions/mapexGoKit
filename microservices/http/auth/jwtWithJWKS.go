package auth

import (
	"fmt"
	"time"

	"github.com/MicahParks/keyfunc"
	"github.com/golang-jwt/jwt/v4"
)

// ParseJWTTokenWithJWKS parses and validates a JWT token using a JWKS (JSON Web Key Set) URL.
//
// Parameters:
//   - tokenString: the JWT token string to be parsed and validated.
//   - jwksURL: the URL of the JWKS endpoint used to fetch public keys for token verification.
//
// Returns:
//   - *jwt.Token: the parsed JWT token object if it is valid.
//   - jwt.MapClaims: the token's claims as a map if parsing succeeds.
//   - error: an error if the JWKS cannot be loaded, the token is invalid, or the claims format is incorrect.
//
// The JWKS is fetched and cached with a refresh interval of one hour. If the token's signature cannot be
// verified using the retrieved keys, or if the claims cannot be cast to `jwt.MapClaims`, an error is returned.
func ParseJWTTokenWithJWKS(tokenString, jwksURL string) (*jwt.Token, jwt.MapClaims, error) {
	keyFunc, err := keyfunc.Get(jwksURL, keyfunc.Options{
		RefreshInterval: time.Hour,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load JWKS: %w", err)
	}

	token, err := jwt.Parse(tokenString, keyFunc.Keyfunc)
	if err != nil || !token.Valid {
		return nil, nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, nil, fmt.Errorf("invalid claims format")
	}

	return token, claims, nil
}
