package utils

import (
	"fmt"
	"log"
	"strings"

	"github.com/golang-jwt/jwt"
)

const unknown = "unknown"

func parseJwt(tokenString string) (map[string]interface{}, error) {
	// Remove Bearer prefix if present
	if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
		tokenString = tokenString[7:]
	}

	// Parse without verification since we only need to extract info
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		err := fmt.Errorf("[parseJwt] Invalid JWT claims structure")
		return nil, err
	}

	return claims, nil
}

// ExtractUserNameFromToken parses JWT token to extract name and email claims.
// If parsing fails, returns "unknown". Missing fields are replaced with "unknown".
func ExtractUserNameFromToken(tokenString string) string {
	var name, contactDetals string

	claims, err := parseJwt(tokenString)
	if err != nil {
		log.Printf("[ExtractUserNameFromToken] Error parsing JWT: %v", err)
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

// ExtractLoginFromToken parses JWT token to extract login type.
func ExtractLoginFromToken(tokenString string) string {
	claims, err := parseJwt(tokenString)
	if err != nil {
		log.Printf("[ExtractUserNameFromToken] Error parsing JWT: %v", err)
		return unknown
	}

	if n, ok := claims["github"].(string); ok && n != "" {
		return "github"
	}

	if n, ok := claims["email"].(string); ok && n != "" {
		if strings.Contains(n, "@sangfor.com") {
			return "sangfor"
		}
	}

	if n, ok := claims["phone"].(string); ok && n != "" {
		return "phone"
	}
	if n, ok := claims["phone_number"].(string); ok && n != "" {
		return "phone"
	}

	return unknown
}
