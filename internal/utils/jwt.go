package utils

import (
	"fmt"
	"log"

	"github.com/golang-jwt/jwt"
)

// ExtractUserNameFromToken parses JWT token to extract name and email claims.
// If parsing fails, returns "unknown". Missing fields are replaced with "unknown".
func ExtractUserNameFromToken(tokenString string) string {
	var name, contactDetals string
	unknown := "unknown"

	// Remove Bearer prefix if present
	if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
		tokenString = tokenString[7:]
	}

	// Parse without verification since we only need to extract info
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		log.Printf("[ExtractUserNameFromToken] Failed to parse JWT token: %v", err)
		return unknown
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		log.Println("[ExtractUserNameFromToken] Invalid JWT claims structure")
		return unknown
	}

	if n, ok := claims["name"].(string); ok && n != "" {
		name = n
	}

	if e, ok := claims["email"].(string); ok && e != "" {
		contactDetals = e
	} else if p, ok := claims["phone_number"].(string); ok && p != "" {
		contactDetals = p
	}

	switch {
	case name != "" && contactDetals != "":
		return fmt.Sprintf("%s<%s>", name, contactDetals)
	case name != "":
		return name
	case contactDetals != "":
		return contactDetals
	default:
		return unknown
	}
}
