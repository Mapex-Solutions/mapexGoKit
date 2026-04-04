package jwt

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestSignJWT(t *testing.T) {
	secret := "test-secret-key"
	userID := "user123"
	sessionID := "session456"
	email := "test@example.com"
	ttl := time.Hour

	tests := []struct {
		name        string
		secret      string
		userID      string
		sessionID   string
		email       string
		ttl         time.Duration
		expectError bool
	}{
		{
			name:        "valid token creation",
			secret:      secret,
			userID:      userID,
			sessionID:   sessionID,
			email:       email,
			ttl:         ttl,
			expectError: false,
		},
		{
			name:        "empty secret",
			secret:      "",
			userID:      userID,
			sessionID:   sessionID,
			email:       email,
			ttl:         ttl,
			expectError: false, // JWT library allows empty secrets
		},
		{
			name:        "empty userID",
			secret:      secret,
			userID:      "",
			sessionID:   sessionID,
			email:       email,
			ttl:         ttl,
			expectError: false,
		},
		{
			name:        "zero TTL",
			secret:      secret,
			userID:      userID,
			sessionID:   sessionID,
			email:       email,
			ttl:         0,
			expectError: false,
		},
		{
			name:        "negative TTL",
			secret:      secret,
			userID:      userID,
			sessionID:   sessionID,
			email:       email,
			ttl:         -time.Hour,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := SignJWT(tt.secret, tt.userID, tt.sessionID, tt.email, tt.ttl)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				// Verify token format
				if token == "" {
					t.Error("expected non-empty token")
				}

				// JWT tokens should have 3 parts separated by dots
				parts := strings.Split(token, ".")
				if len(parts) != 3 {
					t.Errorf("expected 3 parts in JWT, got %d", len(parts))
				}
			}
		})
	}
}

func TestParseJWT(t *testing.T) {
	secret := "test-secret-key"
	userID := "user123"
	sessionID := "session456"
	email := "test@example.com"
	ttl := time.Hour

	// Create a valid token
	validToken, err := SignJWT(secret, userID, sessionID, email, ttl)
	if err != nil {
		t.Fatalf("failed to create test token: %v", err)
	}

	// Create an expired token
	expiredToken, err := SignJWT(secret, userID, sessionID, email, -time.Hour)
	if err != nil {
		t.Fatalf("failed to create expired test token: %v", err)
	}

	// Create token with different secret
	differentSecretToken, err := SignJWT("different-secret", userID, sessionID, email, ttl)
	if err != nil {
		t.Fatalf("failed to create token with different secret: %v", err)
	}

	tests := []struct {
		name        string
		secret      string
		token       string
		expectError bool
		expectedID  string
		expectedUID string
		expectedEmail string
	}{
		{
			name:          "valid token",
			secret:        secret,
			token:         validToken,
			expectError:   false,
			expectedID:    sessionID,
			expectedUID:   userID,
			expectedEmail: email,
		},
		{
			name:        "expired token",
			secret:      secret,
			token:       expiredToken,
			expectError: true,
		},
		{
			name:        "wrong secret",
			secret:      secret,
			token:       differentSecretToken,
			expectError: true,
		},
		{
			name:        "malformed token",
			secret:      secret,
			token:       "invalid.token.format",
			expectError: true,
		},
		{
			name:        "empty token",
			secret:      secret,
			token:       "",
			expectError: true,
		},
		{
			name:        "missing parts",
			secret:      secret,
			token:       "header.payload",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := ParseJWT(tt.secret, tt.token)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, but got none")
				}
				if claims != nil {
					t.Error("expected nil claims on error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if claims == nil {
					t.Error("expected claims, got nil")
				} else {
					if claims.ID != tt.expectedID {
						t.Errorf("expected ID %s, got %s", tt.expectedID, claims.ID)
					}
					if claims.UserID != tt.expectedUID {
						t.Errorf("expected UserID %s, got %s", tt.expectedUID, claims.UserID)
					}
					if claims.Email != tt.expectedEmail {
						t.Errorf("expected Email %s, got %s", tt.expectedEmail, claims.Email)
					}
				}
			}
		})
	}
}

func TestSignRefreshToken(t *testing.T) {
	secret := "test-secret-key"
	userID := "user123"
	sessionID := "session456"
	ttl := time.Hour * 24 * 7 // 7 days

	tests := []struct {
		name        string
		secret      string
		userID      string
		sessionID   string
		ttl         time.Duration
		expectError bool
	}{
		{
			name:        "valid refresh token creation",
			secret:      secret,
			userID:      userID,
			sessionID:   sessionID,
			ttl:         ttl,
			expectError: false,
		},
		{
			name:        "empty secret",
			secret:      "",
			userID:      userID,
			sessionID:   sessionID,
			ttl:         ttl,
			expectError: false,
		},
		{
			name:        "zero TTL",
			secret:      secret,
			userID:      userID,
			sessionID:   sessionID,
			ttl:         0,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := SignRefreshToken(tt.secret, tt.userID, tt.sessionID, tt.ttl)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if token == "" {
					t.Error("expected non-empty refresh token")
				}

				parts := strings.Split(token, ".")
				if len(parts) != 3 {
					t.Errorf("expected 3 parts in JWT, got %d", len(parts))
				}
			}
		})
	}
}

func TestParseRefreshToken(t *testing.T) {
	secret := "test-secret-key"
	userID := "user123"
	sessionID := "session456"
	ttl := time.Hour * 24

	validRefreshToken, err := SignRefreshToken(secret, userID, sessionID, ttl)
	if err != nil {
		t.Fatalf("failed to create test refresh token: %v", err)
	}

	expiredRefreshToken, err := SignRefreshToken(secret, userID, sessionID, -time.Hour)
	if err != nil {
		t.Fatalf("failed to create expired refresh token: %v", err)
	}

	differentSecretRefreshToken, err := SignRefreshToken("different-secret", userID, sessionID, ttl)
	if err != nil {
		t.Fatalf("failed to create refresh token with different secret: %v", err)
	}

	tests := []struct {
		name              string
		secret            string
		token             string
		expectError       bool
		expectedSubject   string
		expectedID        string
	}{
		{
			name:            "valid refresh token",
			secret:          secret,
			token:           validRefreshToken,
			expectError:     false,
			expectedSubject: userID,
			expectedID:      sessionID,
		},
		{
			name:        "expired refresh token",
			secret:      secret,
			token:       expiredRefreshToken,
			expectError: true,
		},
		{
			name:        "wrong secret",
			secret:      secret,
			token:       differentSecretRefreshToken,
			expectError: true,
		},
		{
			name:        "malformed refresh token",
			secret:      secret,
			token:       "invalid.token.format",
			expectError: true,
		},
		{
			name:        "empty refresh token",
			secret:      secret,
			token:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := ParseRefreshToken(tt.secret, tt.token)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, but got none")
				}
				if claims != nil {
					t.Error("expected nil claims on error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if claims == nil {
					t.Error("expected claims, got nil")
				} else {
					if claims.Subject != tt.expectedSubject {
						t.Errorf("expected Subject %s, got %s", tt.expectedSubject, claims.Subject)
					}
					if claims.ID != tt.expectedID {
						t.Errorf("expected ID %s, got %s", tt.expectedID, claims.ID)
					}
				}
			}
		})
	}
}

func TestJWTRoundTrip(t *testing.T) {
	secret := "test-secret-key"
	userID := "user123"
	sessionID := "session456"
	email := "test@example.com"
	ttl := time.Hour

	// Test access token roundtrip
	t.Run("access token roundtrip", func(t *testing.T) {
		token, err := SignJWT(secret, userID, sessionID, email, ttl)
		if err != nil {
			t.Fatalf("failed to sign JWT: %v", err)
		}

		claims, err := ParseJWT(secret, token)
		if err != nil {
			t.Fatalf("failed to parse JWT: %v", err)
		}

		if claims.UserID != userID {
			t.Errorf("expected UserID %s, got %s", userID, claims.UserID)
		}
		if claims.ID != sessionID {
			t.Errorf("expected ID %s, got %s", sessionID, claims.ID)
		}
		if claims.Email != email {
			t.Errorf("expected Email %s, got %s", email, claims.Email)
		}
	})

	// Test refresh token roundtrip
	t.Run("refresh token roundtrip", func(t *testing.T) {
		refreshToken, err := SignRefreshToken(secret, userID, sessionID, ttl)
		if err != nil {
			t.Fatalf("failed to sign refresh token: %v", err)
		}

		claims, err := ParseRefreshToken(secret, refreshToken)
		if err != nil {
			t.Fatalf("failed to parse refresh token: %v", err)
		}

		if claims.Subject != userID {
			t.Errorf("expected Subject %s, got %s", userID, claims.Subject)
		}
		if claims.ID != sessionID {
			t.Errorf("expected ID %s, got %s", sessionID, claims.ID)
		}
	})
}

func TestCustomClaims(t *testing.T) {
	claims := CustomClaims{
		ID:     "test-session",
		UserID: "test-user",
		Email:  "test@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	if claims.ID != "test-session" {
		t.Errorf("expected ID 'test-session', got %s", claims.ID)
	}
	if claims.UserID != "test-user" {
		t.Errorf("expected UserID 'test-user', got %s", claims.UserID)
	}
	if claims.Email != "test@example.com" {
		t.Errorf("expected Email 'test@example.com', got %s", claims.Email)
	}
}

func BenchmarkSignJWT(b *testing.B) {
	secret := "test-secret-key"
	userID := "user123"
	sessionID := "session456"
	email := "test@example.com"
	ttl := time.Hour

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := SignJWT(secret, userID, sessionID, email, ttl)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseJWT(b *testing.B) {
	secret := "test-secret-key"
	userID := "user123"
	sessionID := "session456"
	email := "test@example.com"
	ttl := time.Hour

	token, err := SignJWT(secret, userID, sessionID, email, ttl)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseJWT(secret, token)
		if err != nil {
			b.Fatal(err)
		}
	}
}