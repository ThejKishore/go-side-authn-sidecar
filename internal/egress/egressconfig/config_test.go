package egressconfig

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	configContent := `multi-oauth-client-config:
  "ping":
    tokenUrl: https://ping.example.com/token
    clientId: ping-client
    clientSecret: ping-secret
    scope:
      - openid
  "okta":
    tokenUrl: https://okta.example.com/token
    clientId: okta-client
    clientSecret: okta-secret
    scope:
      - openid
`

	tmpFile, err := os.CreateTemp("", "egress-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Reset global config for testing
	globalConfig = EgressConfig{}

	// Load the config
	if err := Load(tmpFile.Name()); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify the config was loaded
	if len(globalConfig.MultiOAuthClientConfig) != 2 {
		t.Errorf("Expected 2 IDP configs, got %d", len(globalConfig.MultiOAuthClientConfig))
	}

	// Test GetOAuthConfig
	pingConfig, err := GetOAuthConfig("ping")
	if err != nil {
		t.Errorf("Failed to get ping config: %v", err)
	}
	if pingConfig.ClientID != "ping-client" {
		t.Errorf("Expected client ID 'ping-client', got '%s'", pingConfig.ClientID)
	}

	// Test GetAllIDPTypes
	idpTypes := GetAllIDPTypes()
	if len(idpTypes) != 2 {
		t.Errorf("Expected 2 IDP types, got %d", len(idpTypes))
	}
}

func TestGetOAuthConfigNotFound(t *testing.T) {
	// Reset global config
	globalConfig = EgressConfig{
		MultiOAuthClientConfig: make(map[string]OAuthClientConfig),
	}

	_, err := GetOAuthConfig("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent IDP type")
	}
}
