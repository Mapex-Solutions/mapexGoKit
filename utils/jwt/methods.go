// Package jwt provides helper functions for signing and parsing
// JSON Web Tokens (JWTs) for both access and refresh tokens.
//
// It wraps the github.com/golang-jwt/jwt/v5 package with convenience
// methods to create signed tokens using HMAC SHA-256 and extract claims.
package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// SignJWT creates and signs a JWT access token using the provided secret.
//
// The token includes:
//   - A session ID (`ID` claim)
//   - User ID (`UserID` in CustomClaims)
//   - Email (`Email` in CustomClaims)
//   - Standard registered claims (ExpiresAt, IssuedAt)
//
// Parameters:
//   - secret: HMAC secret key used for signing (HS256)
//   - userID: unique identifier of the authenticated user
//   - sessionId: unique session identifier to track the session
//   - email: user's email address
//   - ttl: token time-to-live
//
// Returns:
//   - signed JWT string
//   - error if signing fails
func SignJWT(secret string, userID, sessionId, email string, ttl time.Duration) (string, error) {
	claims := CustomClaims{
		ID:     sessionId,
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseJWT validates an access token and extracts its CustomClaims.
//
// Parameters:
//   - secret: HMAC secret key used for signing
//   - tokenStr: raw JWT string
//
// Returns:
//   - pointer to CustomClaims if valid
//   - error if token is invalid, expired, or claims type mismatches
func ParseJWT(secret string, tokenStr string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid or expired token")
	}
	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		return nil, errors.New("invalid claims type")
	}
	return claims, nil
}

// SignRefreshToken creates and signs a minimal refresh token using the provided secret.
//
// The token includes only standard registered claims:
//   - Subject (userID)
//   - ID (sessionId)
//   - ExpiresAt
//   - IssuedAt
//
// Parameters:
//   - secret: HMAC secret key used for signing (HS256)
//   - userID: unique identifier of the authenticated user
//   - sessionId: unique session identifier
//   - ttl: refresh token time-to-live
//
// Returns:
//   - signed refresh token string
//   - error if signing fails
func SignRefreshToken(secret string, userID string, sessionId string, ttl time.Duration) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		ID:        sessionId,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseRefreshToken validates a refresh token and extracts its RegisteredClaims.
//
// Parameters:
//   - secret: HMAC secret key used for signing
//   - tokenStr: raw JWT string
//
// Returns:
//   - pointer to jwt.RegisteredClaims if valid
//   - error if token is invalid, expired, or claims type mismatches
func ParseRefreshToken(secret string, tokenStr string) (*jwt.RegisteredClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid or expired refresh token")
	}
	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return nil, errors.New("invalid claims type")
	}
	return claims, nil
}
