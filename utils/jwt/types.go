package jwt

import (
	"github.com/golang-jwt/jwt/v5"
)

// CustomClaims for access tokens
type CustomClaims struct {
	ID        string `json:"sessionId"`
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	FisrtName string `json:"firtName"`
	LastName  string `json:"lastName"`
	jwt.RegisteredClaims
}
