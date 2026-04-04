package password

import (
	"strings"
	"testing"
	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "valid password",
			password: "mypassword123",
			wantErr:  false,
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  false,
		},
		{
			name:     "long password",
			password: strings.Repeat("a", 72),
			wantErr:  false,
		},
		{
			name:     "password with special characters",
			password: "p@ssw0rd!#$%^&*()",
			wantErr:  false,
		},
		{
			name:     "unicode password",
			password: "пароль123",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			
			if (err != nil) != tt.wantErr {
				t.Errorf("HashPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the hash is not empty
				if hash == "" {
					t.Error("HashPassword() returned empty hash")
				}

				// Verify the hash starts with bcrypt identifier
				if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") && !strings.HasPrefix(hash, "$2y$") {
					t.Errorf("HashPassword() hash doesn't have valid bcrypt format: %s", hash)
				}

				// Verify we can verify the password with the hash
				if !CheckPassword(hash, tt.password) {
					t.Error("HashPassword() generated hash that doesn't verify against original password")
				}
			}
		})
	}
}

func TestCheckPassword(t *testing.T) {
	validPassword := "testpassword123"
	validHash, err := bcrypt.GenerateFromPassword([]byte(validPassword), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to generate test hash: %v", err)
	}

	tests := []struct {
		name           string
		hashedPassword string
		plainPassword  string
		expected       bool
	}{
		{
			name:           "correct password",
			hashedPassword: string(validHash),
			plainPassword:  validPassword,
			expected:       true,
		},
		{
			name:           "incorrect password",
			hashedPassword: string(validHash),
			plainPassword:  "wrongpassword",
			expected:       false,
		},
		{
			name:           "empty plain password",
			hashedPassword: string(validHash),
			plainPassword:  "",
			expected:       false,
		},
		{
			name:           "empty hash",
			hashedPassword: "",
			plainPassword:  validPassword,
			expected:       false,
		},
		{
			name:           "invalid hash format",
			hashedPassword: "invalid_hash",
			plainPassword:  validPassword,
			expected:       false,
		},
		{
			name:           "case sensitive password",
			hashedPassword: string(validHash),
			plainPassword:  "TestPassword123",
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckPassword(tt.hashedPassword, tt.plainPassword)
			if result != tt.expected {
				t.Errorf("CheckPassword() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestHashPassword_Uniqueness(t *testing.T) {
	password := "testpassword"
	hash1, err1 := HashPassword(password)
	hash2, err2 := HashPassword(password)

	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}

	// Bcrypt should generate different hashes for the same password due to salt
	if hash1 == hash2 {
		t.Error("HashPassword() generated identical hashes, should be different due to salt")
	}

	// But both should validate against the original password
	if !CheckPassword(hash1, password) {
		t.Error("first hash doesn't validate")
	}
	if !CheckPassword(hash2, password) {
		t.Error("second hash doesn't validate")
	}
}

func TestHashPasswordAndCheck_Integration(t *testing.T) {
	tests := []string{
		"simplepass",
		"complex!@#$%^&*()_+",
		"",
		"verylongpasswordthatexceedstypicallengthbutshouldsillwork",
		"123456789",
		"пароль",
		"🔐🗝️🔑",
	}

	for _, password := range tests {
		t.Run("password_"+password, func(t *testing.T) {
			// Hash the password
			hash, err := HashPassword(password)
			if err != nil {
				t.Fatalf("failed to hash password: %v", err)
			}

			// Verify correct password
			if !CheckPassword(hash, password) {
				t.Error("correct password failed verification")
			}

			// Verify incorrect password
			if CheckPassword(hash, password+"wrong") {
				t.Error("incorrect password passed verification")
			}
		})
	}
}

func BenchmarkHashPassword(b *testing.B) {
	password := "benchmarkpassword123"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := HashPassword(password)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCheckPassword(b *testing.B) {
	password := "benchmarkpassword123"
	hash, err := HashPassword(password)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CheckPassword(hash, password)
	}
}