package tokenstorage

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TokenStorage manages token storage and retrieval
type TokenStorage struct {
	tokenDir string
	mu       sync.RWMutex
	tokens   map[string]tokenEntry
}

type tokenEntry struct {
	token     string
	expiresAt time.Time
}

var instance *TokenStorage
var once sync.Once

// GetInstance returns the singleton TokenStorage instance
func GetInstance() *TokenStorage {
	once.Do(func() {
		instance = &TokenStorage{
			tokenDir: "/tmp/egress-tokens",
			tokens:   make(map[string]tokenEntry),
		}
		// Create token directory if it doesn't exist
		_ = os.MkdirAll(instance.tokenDir, 0o700)
	})
	return instance
}

// SaveToken saves a token for a given IDP type
func (ts *TokenStorage) SaveToken(idpType, token string, expiresIn time.Duration) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	expiresAt := time.Now().Add(expiresIn)
	ts.tokens[idpType] = tokenEntry{
		token:     token,
		expiresAt: expiresAt,
	}

	// Also persist to file
	filePath := filepath.Join(ts.tokenDir, fmt.Sprintf("%s-token.txt", idpType))
	return os.WriteFile(filePath, []byte(token), 0o600)
}

// GetToken retrieves a token for a given IDP type
func (ts *TokenStorage) GetToken(idpType string) (string, error) {
	ts.mu.RLock()
	entry, exists := ts.tokens[idpType]
	ts.mu.RUnlock()

	if exists && entry.expiresAt.After(time.Now()) {
		return entry.token, nil
	}

	// Try to load from file if not in memory or expired
	filePath := filepath.Join(ts.tokenDir, fmt.Sprintf("%s-token.txt", idpType))
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("token not found for IDP type '%s': %w", idpType, err)
	}

	return string(data), nil
}

// TokenExists checks if a token exists and is not expired
func (ts *TokenStorage) TokenExists(idpType string) bool {
	ts.mu.RLock()
	entry, exists := ts.tokens[idpType]
	ts.mu.RUnlock()

	if exists && entry.expiresAt.After(time.Now()) {
		return true
	}

	filePath := filepath.Join(ts.tokenDir, fmt.Sprintf("%s-token.txt", idpType))
	_, err := os.Stat(filePath)
	return err == nil
}

// ClearToken removes a token for a given IDP type
func (ts *TokenStorage) ClearToken(idpType string) error {
	ts.mu.Lock()
	delete(ts.tokens, idpType)
	ts.mu.Unlock()

	filePath := filepath.Join(ts.tokenDir, fmt.Sprintf("%s-token.txt", idpType))
	return os.Remove(filePath)
}
