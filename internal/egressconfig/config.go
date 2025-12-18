package egressconfig

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// OAuthClientConfig represents the configuration for a single OAuth provider
type OAuthClientConfig struct {
	TokenURL          string   `yaml:"tokenUrl"`
	ClientID          string   `yaml:"clientId"`
	ClientSecret      string   `yaml:"clientSecret"`
	ClientCertificate string   `yaml:"clientCertificate"`
	Scope             []string `yaml:"scope"`
}

// EgressConfig represents the entire egress proxy configuration
type EgressConfig struct {
	MultiOAuthClientConfig map[string]OAuthClientConfig `yaml:"multi-oauth-client-config"`
}

var globalConfig EgressConfig

// Load loads the egress configuration from a YAML file
func Load(configPath string) error {
	if configPath == "" {
		configPath = "egress-config.yaml"
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &globalConfig); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if globalConfig.MultiOAuthClientConfig == nil {
		globalConfig.MultiOAuthClientConfig = make(map[string]OAuthClientConfig)
	}

	return nil
}

// GetOAuthConfig returns the OAuth configuration for a given IDP type
func GetOAuthConfig(idpType string) (OAuthClientConfig, error) {
	config, exists := globalConfig.MultiOAuthClientConfig[idpType]
	if !exists {
		return OAuthClientConfig{}, fmt.Errorf("IDP type '%s' not found in configuration", idpType)
	}
	return config, nil
}

// GetAllIDPTypes returns all configured IDP types
func GetAllIDPTypes() []string {
	idpTypes := make([]string, 0, len(globalConfig.MultiOAuthClientConfig))
	for idpType := range globalConfig.MultiOAuthClientConfig {
		idpTypes = append(idpTypes, idpType)
	}
	return idpTypes
}
