package auth

import (
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// ParseJWTTokenWithSecret parses and validates a JWT token string using a given secret and expected signing algorithm.
//
// Parameters:
//   - tokenString: the JWT token string to be parsed and validated.
//   - secret: the shared secret used to verify the token's signature.
//   - algorithm: the expected signing algorithm (e.g., "HS256").
//
// Returns:
//   - *jwt.Token: the parsed token object if valid.
//   - jwt.MapClaims: the token's claims as a map if successfully parsed.
//   - error: an error if the token is invalid, the algorithm does not match, or the claims cannot be parsed.
//
// The function checks whether the token uses the expected signing algorithm and whether the token is valid.
// It also attempts to cast the token's claims to `jwt.MapClaims` and returns an error if the cast fails.
func ParseJWTTokenWithSecret(tokenString, secret, algorithm string) (*jwt.Token, jwt.MapClaims, error) {

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != algorithm {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil || !token.Valid {
		return nil, nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, nil, fmt.Errorf("invalid claims format")
	}

	return token, claims, nil
}
