package tokenmanager

import (
	"log"
	"reverseProxy/internal/egress/egressconfig"
	"reverseProxy/internal/egress/oauthclient"
	"sync"
	"time"
)

// TokenManager manages token fetching and refreshing for all IDP types
type TokenManager struct {
	mu      sync.Mutex
	stopCh  map[string]chan struct{}
	running bool
}

var instance *TokenManager
var once sync.Once

// GetInstance returns the singleton TokenManager instance
func GetInstance() *TokenManager {
	once.Do(func() {
		instance = &TokenManager{
			stopCh: make(map[string]chan struct{}),
		}
	})
	return instance
}

// StartTokenRefresh starts the token refresh routine for all configured IDP types
func (tm *TokenManager) StartTokenRefresh(refreshInterval time.Duration) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.running {
		return nil // Already running
	}

	tm.running = true

	// Get all configured IDP types
	idpTypes := egressconfig.GetAllIDPTypes()

	for _, idpType := range idpTypes {
		tm.startRefreshForIDP(idpType, refreshInterval)
	}

	// Also handle "noIdp" case - no token fetching needed
	log.Println("Token refresh started for all configured IDP types")
	return nil
}

// startRefreshForIDP starts the token refresh routine for a specific IDP type
func (tm *TokenManager) startRefreshForIDP(idpType string, interval time.Duration) {
	stopCh := make(chan struct{})
	tm.stopCh[idpType] = stopCh

	go func() {
		// Fetch token immediately on startup
		err := tm.refreshTokenForIDP(idpType)
		if err != nil {
			log.Printf("Failed to fetch initial token for IDP type '%s': %v", idpType, err)
		}

		// Then refresh periodically
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				err := tm.refreshTokenForIDP(idpType)
				if err != nil {
					log.Printf("Failed to refresh token for IDP type '%s': %v", idpType, err)
				}
			case <-stopCh:
				log.Printf("Stopped token refresh for IDP type '%s'", idpType)
				return
			}
		}
	}()
}

// refreshTokenForIDP refreshes the token for a specific IDP type
func (tm *TokenManager) refreshTokenForIDP(idpType string) error {
	client, err := oauthclient.NewOAuthClient(idpType)
	if err != nil {
		return err
	}

	if err := client.RefreshToken(); err != nil {
		return err
	}

	log.Printf("Successfully refreshed token for IDP type '%s'", idpType)
	return nil
}

// StopTokenRefresh stops all token refresh routines
func (tm *TokenManager) StopTokenRefresh() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for idpType, stopCh := range tm.stopCh {
		close(stopCh)
		log.Printf("Stopping token refresh for IDP type '%s'", idpType)
	}

	tm.stopCh = make(map[string]chan struct{})
	tm.running = false
}
