package utils

import (
	"fmt"
	"log"

	"github.com/golang-jwt/jwt"
)

// ExtractUserNameFromToken parses JWT token to extract name and email claims.
// If parsing fails, returns "unknown". Missing fields are replaced with "unknown".
func ExtractUserNameFromToken(tokenString string) string {
	log.Printf("[ExtractUserNameFromToken] Extracting user info from token: %s", tokenString)
	var name, email string

	// Remove Bearer prefix if present
	if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
		tokenString = tokenString[7:]
	}

	// Parse without verification since we only need to extract info
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		log.Printf("[ExtractUserNameFromToken] Failed to parse JWT token: %v", err)
		return "unknown"
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		log.Println("[ExtractUserNameFromToken] Invalid JWT claims structure")
		return "unknown"
	}

	if n, ok := claims["name"].(string); ok && n != "" {
		name = n
	}

	if e, ok := claims["email"].(string); ok && e != "" {
		email = e
	} else if p, ok := claims["phone_number"].(string); ok && p != "" {
		email = p
	}

	log.Printf("[ExtractUserNameFromToken] Extracted - name: %v, email: %v", name, email)

	switch {
	case name != "" && email != "":
		return fmt.Sprintf("%s<%s>", name, email)
	case name != "":
		return name
	case email != "":
		return email
	default:
		return "unknown"
	}
}
