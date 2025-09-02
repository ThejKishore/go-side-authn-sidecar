package jwtauth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
)

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func TestParseRSAPublicKey_Valid(t *testing.T) {
	// generate a key and reconstruct via parseRSAPublicKey inputs
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil { t.Fatal(err) }
	n := priv.PublicKey.N.Bytes()
	e := big.NewInt(int64(priv.PublicKey.E)).Bytes()
	pk, err := parseRSAPublicKey(b64url(n), b64url(e))
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if pk.N.Cmp(priv.PublicKey.N) != 0 || pk.E != priv.PublicKey.E {
		t.Fatalf("parsed key does not match original")
	}
}

func TestParseRSAPublicKey_InvalidBase64(t *testing.T) {
	if _, err := parseRSAPublicKey("***", "AQAB"); err == nil {
		t.Fatalf("expected error for invalid modulus base64")
	}
	// valid n, invalid e
	priv, _ := rsa.GenerateKey(rand.Reader, 512)
	if _, err := parseRSAPublicKey(b64url(priv.PublicKey.N.Bytes()), "***"); err == nil {
		t.Fatalf("expected error for invalid exponent base64")
	}
}

func TestFetchPublicKeysAndGet(t *testing.T) {
	// create an RSA public key and expose as JWKS via httptest server
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil { t.Fatal(err) }
	// build JWKS
	jwks := map[string][]map[string]interface{}{
		"keys": {
			{
				"kty": "RSA",
				"kid": "test-kid",
				"n":   b64url(priv.PublicKey.N.Bytes()),
				"e":   b64url(big.NewInt(int64(priv.PublicKey.E)).Bytes()),
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	if err := FetchPublicKeys(srv.URL); err != nil {
		t.Fatalf("FetchPublicKeys error: %v", err)
	}
	pk, ok := GetPublicKey("test-kid")
	if !ok || pk == nil {
		t.Fatalf("expected key in cache")
	}
}

// ensure package exported types compile in tests (avoid unused imports)
func TestPrincipalType(t *testing.T) {
	_ = Principal{UserID: "u", Username: "n"}
}
