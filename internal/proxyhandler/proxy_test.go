package proxyhandler

import (
    "crypto/rand"
    "crypto/rsa"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/gofiber/fiber/v3"
    "github.com/golang-jwt/jwt/v5"

    "reverseProxy/internal/jwtauth"
)

func makeRSAToken(t *testing.T, kid string, priv *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	if claims == nil {
		claims = jwt.MapClaims{}
	}
	claims["exp"] = time.Now().Add(1 * time.Hour).Unix()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	s, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("sign error: %v", err)
	}
	return s
}

func TestHandler_SuccessAndPrincipal(t *testing.T) {
	app := fiber.New()
	// stub proxy to avoid network
	called := false
	doProxy = func(c fiber.Ctx, url string) error { called = true; return nil }

	// prepare key and cache
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	kid := "kid1"
	jwtauth.SetPublicKeyForTest(kid, &priv.PublicKey)

	// create a request with valid token and custom claims
	token := makeRSAToken(t, kid, priv, jwt.MapClaims{
		"user_id":  "u1",
		"username": "alice",
		"email":    "a@example.com",
	})

	app.All("/*", Handler)

	req := httptest.NewRequest("GET", "/anything", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: -1})
	if err != nil {
		t.Fatalf("app.Test error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if !called {
		t.Fatalf("expected proxy to be called")
	}
}

func TestHandler_MissingAuthHeader(t *testing.T) {
	app := fiber.New()
	app.All("/*", Handler)
	req := httptest.NewRequest("GET", "/x", nil)
	resp, _ := app.Test(req, fiber.TestConfig{Timeout: -1})
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestHandler_InvalidSigningMethod(t *testing.T) {
	app := fiber.New()
	doProxy = func(c fiber.Ctx, url string) error { return nil }
	// seed cache with any key under kid
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	kid := "kid2"
	jwtauth.SetPublicKeyForTest(kid, &rsa.PublicKey{N: priv.N, E: priv.E})

	// Create HS256 token but kid present
	claims := jwt.MapClaims{"user_id": "u2"}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tok.Header["kid"] = kid
	s, _ := tok.SignedString([]byte("secret"))

	app.All("/*", Handler)
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+s)
	resp, _ := app.Test(req, fiber.TestConfig{Timeout: -1})
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid signing method, got %d", resp.StatusCode)
	}
}
