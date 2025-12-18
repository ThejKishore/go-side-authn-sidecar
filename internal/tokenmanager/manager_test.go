package tokenmanager

import (
	"sync"
	"testing"
	"time"
)

func TestTokenManagerSingleton(t *testing.T) {
	// Reset singleton for testing
	instance = nil
	once = sync.Once{}

	mgr1 := GetInstance()
	mgr2 := GetInstance()

	if mgr1 != mgr2 {
		t.Error("TokenManager should be a singleton")
	}
}

func TestStartTokenRefreshWithEmptyConfig(t *testing.T) {
	// Reset singleton for testing
	instance = nil
	once = sync.Once{}

	mgr := GetInstance()

	// Should not error even with empty config
	err := mgr.StartTokenRefresh(1 * time.Minute)
	if err != nil {
		t.Errorf("Expected no error with empty config, got: %v", err)
	}

	// Stop the refresh
	mgr.StopTokenRefresh()
}
