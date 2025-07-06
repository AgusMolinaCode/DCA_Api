package middleware

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateResetToken generates a JWT token for password reset functionality
// This is kept for potential future use with password reset features
func GenerateResetToken(email string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email": email,
		"exp":   time.Now().Add(time.Hour * 24).Unix(),
	})

	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}

