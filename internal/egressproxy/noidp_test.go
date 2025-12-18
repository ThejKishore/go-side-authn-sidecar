package egressproxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestHandlerNoIdpSkipsAuthorizationHeader(t *testing.T) {
	// Create a mock backend server that checks for Authorization header
	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			t.Errorf("Expected no Authorization header in noIdp mode, got '%s'", authHeader)
		}

		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"status":"ok"}`)
	}))
	defer mockBackend.Close()

	app := fiber.New()
	app.All("/*", Handler)

	// Test with noIdp (lowercase)
	req := httptest.NewRequest("GET", "http://localhost:3002/test", nil)
	req.Header.Set("X-Backend-Url", mockBackend.URL)
	req.Header.Set("X-Idp-Type", "noIdp")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestHandlerNoIdpVariations(t *testing.T) {
	testCases := []string{
		"noIdp",
		"noidp",
		"NOIDP",
		"NoIdp",
	}

	for _, testIdpType := range testCases {
		t.Run("noIdp_"+testIdpType, func(t *testing.T) {
			mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authHeader := r.Header.Get("Authorization")
				if authHeader != "" {
					t.Errorf("For %s: Expected no Authorization header, got '%s'", testIdpType, authHeader)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer mockBackend.Close()

			app := fiber.New()
			app.All("/*", Handler)

			req := httptest.NewRequest("GET", "http://localhost:3002/test", nil)
			req.Header.Set("X-Backend-Url", mockBackend.URL)
			req.Header.Set("X-Idp-Type", testIdpType)

			resp, _ := app.Test(req)
			if resp.StatusCode != http.StatusOK {
				t.Errorf("For %s: Expected status 200, got %d", testIdpType, resp.StatusCode)
			}
		})
	}
}
