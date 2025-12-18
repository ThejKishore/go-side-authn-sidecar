package egressproxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestHandlerMissingBackendURL(t *testing.T) {
	app := fiber.New()
	app.All("/*", Handler)

	req := httptest.NewRequest("GET", "http://localhost:3002/test", nil)
	resp, err := app.Test(req)

	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestHandlerWithBackendURL(t *testing.T) {
	// Create a mock backend server
	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"status":"ok"}`)
	}))
	defer mockBackend.Close()

	app := fiber.New()
	app.All("/*", Handler)

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(body) != `{"status":"ok"}` {
		t.Errorf("Expected response body '{{\"status\":\"ok\"}}', got '%s'", string(body))
	}
}

func TestHandlerForwardsHeaders(t *testing.T) {
	// Create a mock backend server that checks headers
	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		customHeader := r.Header.Get("X-Custom-Header")
		if customHeader != "custom-value" {
			t.Errorf("Expected custom header value, got '%s'", customHeader)
		}

		// X-Backend-Url and X-Idp-Type should not be forwarded
		if r.Header.Get("X-Backend-Url") != "" || r.Header.Get("X-Idp-Type") != "" {
			t.Error("X-Backend-Url and X-Idp-Type should not be forwarded")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer mockBackend.Close()

	app := fiber.New()
	app.All("/*", Handler)

	req := httptest.NewRequest("GET", "http://localhost:3002/test", nil)
	req.Header.Set("X-Backend-Url", mockBackend.URL)
	req.Header.Set("X-Idp-Type", "noIdp")
	req.Header.Set("X-Custom-Header", "custom-value")

	resp, err := app.Test(req)

	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestHandlerBackendError(t *testing.T) {
	// Create a mock backend server that returns an error
	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "Internal Server Error")
	}))
	defer mockBackend.Close()

	app := fiber.New()
	app.All("/*", Handler)

	req := httptest.NewRequest("GET", "http://localhost:3002/test", nil)
	req.Header.Set("X-Backend-Url", mockBackend.URL)
	req.Header.Set("X-Idp-Type", "noIdp")

	resp, err := app.Test(req)

	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	// Should forward the error status from backend
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", resp.StatusCode)
	}
}
