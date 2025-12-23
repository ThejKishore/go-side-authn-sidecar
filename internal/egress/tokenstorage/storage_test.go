package tokenstorage

import (
	"os"
	"testing"
	"time"
)

func TestSaveAndGetToken(t *testing.T) {
	// Create a fresh instance for testing
	testStorage := &TokenStorage{
		tokenDir: "/tmp/test-egress-tokens",
		tokens:   make(map[string]tokenEntry),
	}
	os.MkdirAll(testStorage.tokenDir, 0o700)
	defer os.RemoveAll(testStorage.tokenDir)

	token := "test-token-123"
	expiresIn := 1 * time.Hour

	// Save token
	if err := testStorage.SaveToken("test-idp", token, expiresIn); err != nil {
		t.Fatalf("Failed to save token: %v", err)
	}

	// Retrieve token
	retrievedToken, err := testStorage.GetToken("test-idp")
	if err != nil {
		t.Fatalf("Failed to get token: %v", err)
	}

	if retrievedToken != token {
		t.Errorf("Expected token '%s', got '%s'", token, retrievedToken)
	}
}

func TestTokenExpiration(t *testing.T) {
	testStorage := &TokenStorage{
		tokenDir: "/tmp/test-egress-tokens",
		tokens:   make(map[string]tokenEntry),
	}

	token := "expired-token"
	expiresIn := -1 * time.Hour // Already expired

	testStorage.SaveToken("test-idp", token, expiresIn)

	// Token should not be found in memory since it's expired
	testStorage.mu.RLock()
	entry, exists := testStorage.tokens["test-idp"]
	testStorage.mu.RUnlock()

	if !exists {
		t.Error("Token entry should exist in memory")
	}

	if entry.expiresAt.After(time.Now()) {
		t.Error("Token should be expired")
	}
}

func TestClearToken(t *testing.T) {
	testStorage := &TokenStorage{
		tokenDir: "/tmp/test-egress-tokens",
		tokens:   make(map[string]tokenEntry),
	}
	os.MkdirAll(testStorage.tokenDir, 0o700)
	defer os.RemoveAll(testStorage.tokenDir)

	// Save a token
	testStorage.SaveToken("test-idp", "test-token", 1*time.Hour)

	// Clear the token
	if err := testStorage.ClearToken("test-idp"); err != nil {
		t.Fatalf("Failed to clear token: %v", err)
	}

	// Token should be deleted
	testStorage.mu.RLock()
	_, exists := testStorage.tokens["test-idp"]
	testStorage.mu.RUnlock()

	if exists {
		t.Error("Token should be deleted from memory")
	}
}
