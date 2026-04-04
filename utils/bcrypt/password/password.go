// Package password provides helpers for hashing and verifying passwords
// using the bcrypt algorithm.
package password

import (
	"golang.org/x/crypto/bcrypt"
)

// HashPassword generates a bcrypt hash from the given plain-text password.
//
// Parameters:
//   - password: plain-text password to hash.
//
// Returns:
//   - hashed password as a string
//   - error if hashing fails
//
// The hashing cost is set to bcrypt.DefaultCost, which is a good balance
// between security and performance for most applications.
func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// CheckPassword compares a bcrypt hashed password with a plain-text password.
//
// Parameters:
//   - hashedPassword: bcrypt-hashed password string
//   - plainPassword: plain-text password to verify
//
// Returns:
//   - true if the passwords match
//   - false if they do not match or if the hash is invalid
//
// Example:
//
//	ok := CheckPassword(hashed, "mypassword!")
//	if ok {
//	    fmt.Println("Password is correct")
//	} else {
//	    fmt.Println("Password is incorrect")
//	}
func CheckPassword(hashedPassword string, plainPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
	return err == nil
}
