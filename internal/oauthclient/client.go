package oauthclient

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"reverseProxy/internal/egressconfig"
	"reverseProxy/internal/tokenstorage"
)

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// OAuthClient handles OAuth token fetching
type OAuthClient struct {
	idpType string
	config  egressconfig.OAuthClientConfig
	client  *http.Client
}

// NewOAuthClient creates a new OAuth client for the given IDP type
func NewOAuthClient(idpType string) (*OAuthClient, error) {
	config, err := egressconfig.GetOAuthConfig(idpType)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Configure TLS if certificate is provided
	if config.ClientCertificate != "" {
		tlsConfig, err := loadClientCertificate(config.ClientCertificate)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		httpClient.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	}

	return &OAuthClient{
		idpType: idpType,
		config:  config,
		client:  httpClient,
	}, nil
}

// FetchToken fetches a new token from the OAuth provider
func (oc *OAuthClient) FetchToken() (string, time.Duration, error) {
	// Prepare the token request
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", oc.config.ClientID)
	data.Set("client_secret", oc.config.ClientSecret)
	if len(oc.config.Scope) > 0 {
		data.Set("scope", strings.Join(oc.config.Scope, " "))
	}

	req, err := http.NewRequest("POST", oc.config.TokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := oc.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to fetch token: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("failed to fetch token: status %d, response: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", 0, fmt.Errorf("failed to decode token response: %w", err)
	}

	expiresIn := time.Duration(tokenResp.ExpiresIn) * time.Second
	return tokenResp.AccessToken, expiresIn, nil
}

// RefreshToken fetches and stores a new token
func (oc *OAuthClient) RefreshToken() error {
	token, expiresIn, err := oc.FetchToken()
	if err != nil {
		return err
	}

	storage := tokenstorage.GetInstance()
	return storage.SaveToken(oc.idpType, token, expiresIn)
}

// loadClientCertificate loads a client certificate from a file (PEM or PKCS12)
func loadClientCertificate(certPath string) (*tls.Config, error) {
	if strings.HasSuffix(strings.ToLower(certPath), ".pfx") || strings.HasSuffix(strings.ToLower(certPath), ".p12") {
		return loadPKCS12Certificate(certPath)
	}
	// Assume PEM format
	return loadPEMCertificate(certPath)
}

// loadPEMCertificate loads a PEM certificate
func loadPEMCertificate(certPath string) (*tls.Config, error) {
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate: %w", err)
	}

	cert, err := tls.X509KeyPair(certData, certData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
	}, nil
}

// loadPKCS12Certificate loads a PKCS12 certificate
// Note: Go's standard library doesn't directly support PKCS12, so this is a placeholder
// In production, you would need to use a third-party library or convert to PEM first
func loadPKCS12Certificate(_ string) (*tls.Config, error) {
	// For now, return an error prompting the user to convert to PEM
	return nil, fmt.Errorf("PKCS12 certificates not directly supported; please convert to PEM format")
}
