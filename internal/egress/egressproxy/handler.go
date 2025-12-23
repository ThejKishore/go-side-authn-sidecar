package egressproxy

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"reverseProxy/internal/egress/tokenstorage"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// Handler handles egress proxy requests
func Handler(c fiber.Ctx) error {
	// Get the backend URL from the X-Backend-Url header
	backendURL := c.Get("X-Backend-Url")
	if backendURL == "" {
		return fiber.NewError(fiber.StatusBadRequest, "X-Backend-Url header is required")
	}

	// Get the IDP type from the X-Idp-Type header
	idpType := c.Get("X-Idp-Type")
	if idpType == "" {
		idpType = "noIdp" // Default to no IDP if not specified
	}

	// Normalize IDP type to lowercase for consistent lookup
	idpType = strings.ToLower(idpType)

	// Build the target URL - use Path and Query
	path := c.Path()
	query := c.Request().URI().QueryString()
	if len(query) > 0 {
		path = path + "?" + string(query)
	}

	// Ensure backend URL ends properly and path starts with /
	if !strings.HasSuffix(backendURL, "/") {
		backendURL = backendURL + "/"
	}
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}

	targetURL := backendURL + path

	// Create a new HTTP request
	req, err := createHTTPRequest(c, targetURL, idpType)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, fmt.Sprintf("failed to create request: %v", err))
	}

	// Execute the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		// Forward backend errors as-is
		log.Printf("Backend request failed: %v", err)
		return fiber.NewError(fiber.StatusBadGateway, fmt.Sprintf("backend request failed: %v", err))
	}
	defer resp.Body.Close()

	// Copy response headers to the Fiber context
	for key, values := range resp.Header {
		for _, value := range values {
			c.Append(key, value)
		}
	}

	// Read and send the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response body: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "failed to read response body")
	}

	return c.Status(resp.StatusCode).Send(body)
}

// createHTTPRequest creates an HTTP request with proper headers and authentication
func createHTTPRequest(c fiber.Ctx, targetURL, idpType string) (*http.Request, error) {
	// Create request
	req, err := http.NewRequest(c.Method(), targetURL, nil)
	if err != nil {
		return nil, err
	}

	// Forward request body if present
	if c.Method() != "GET" && c.Method() != "HEAD" {
		body := c.Body()
		if len(body) > 0 {
			req.Body = io.NopCloser(strings.NewReader(string(body)))
		}
	}

	// Copy headers from the incoming request, excluding headers we handle specially
	excludeHeaders := map[string]bool{
		"Host":           true, // Will be set by http.Request
		"Content-Length": true, // Will be set by http.Request
		"X-Backend-Url":  true,
		"X-Idp-Type":     true,
	}

	c.Request().Header.VisitAll(func(key, value []byte) {
		headerName := string(key)
		if !excludeHeaders[headerName] {
			req.Header.Set(headerName, string(value))
		}
	})

	// Add authorization header if IDP type is not "noIdp"
	// Skip Authorization header for noIdp mode (case-insensitive)
	if idpType != "noidp" {
		token, err := getToken(idpType)
		if err != nil {
			log.Printf("Failed to get token for IDP type '%s': %v", idpType, err)
			// Continue without token - let the backend handle it
		} else if token != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		}
	}
	// For noIdp mode, no Authorization header is added

	return req, nil
}

// getToken retrieves a token for the given IDP type
func getToken(idpType string) (string, error) {
	storage := tokenstorage.GetInstance()
	token, err := storage.GetToken(idpType)
	if err != nil {
		return "", err
	}
	return token, nil
}
