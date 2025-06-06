package utils

import (
	"testing"

	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"
)

func TestExtractUserInfoFromToken(t *testing.T) {
	// Test case: valid token with name and email
	t.Run("name and email exists", func(t *testing.T) {
		claims := jwt.MapClaims{
			"name":  "John Doe",
			"email": "john.doe@example.com",
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte("test-secret"))

		result := ExtractUserNameFromToken(tokenString)
		assert.Equal(t, "John Doe<john.doe@example.com>", result)
	})

	// Test case: email replaced by phone_number
	t.Run("email replaced by phone_number", func(t *testing.T) {
		claims := jwt.MapClaims{
			"name":         "John Doe",
			"phone_number": "13800138000",
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte("test-secret"))

		assert.Equal(t, "John Doe<13800138000>", ExtractUserNameFromToken(tokenString))
	})

	// Test case: only name exists
	t.Run("only name exists", func(t *testing.T) {
		claims := jwt.MapClaims{"name": "John Doe"}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte("test-secret"))

		assert.Equal(t, "John Doe", ExtractUserNameFromToken(tokenString))
	})

	// Test case: only phone_number exists
	t.Run("only phone_number exists", func(t *testing.T) {
		claims := jwt.MapClaims{"phone_number": "13800138000"}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte("test-secret"))

		assert.Equal(t, "13800138000", ExtractUserNameFromToken(tokenString))
	})

	// Test case: no valid claims
	t.Run("no valid claims", func(t *testing.T) {
		claims := jwt.MapClaims{}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte("test-secret"))

		assert.Equal(t, "unknown", ExtractUserNameFromToken(tokenString))
	})

	// Test case: invalid token
	t.Run("invalid token", func(t *testing.T) {
		assert.Equal(t, "unknown", ExtractUserNameFromToken("invalid.token"))
	})
}
