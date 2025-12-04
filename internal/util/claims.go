package util

import "github.com/golang-jwt/jwt/v5"

// GetClaimAsString safely extracts a string claim from jwt.MapClaims
func GetClaimAsString(claims jwt.MapClaims, key string) string {
	if value, ok := claims[key].(string); ok {
		return value
	}
	return ""
}
