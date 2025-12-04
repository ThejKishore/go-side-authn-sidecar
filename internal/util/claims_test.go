package util

import (
    "testing"

    "github.com/golang-jwt/jwt/v5"
)

func TestGetClaimAsString(t *testing.T) {
	claims := jwt.MapClaims{
		"user_id":  "123",
		"username": "alice",
		"number":   42,
	}
	if got := GetClaimAsString(claims, "user_id"); got != "123" {
		t.Fatalf("expected 123, got %q", got)
	}
	if got := GetClaimAsString(claims, "missing"); got != "" {
		t.Fatalf("expected empty for missing, got %q", got)
	}
	if got := GetClaimAsString(claims, "number"); got != "" {
		t.Fatalf("expected empty for non-string, got %q", got)
	}
}
