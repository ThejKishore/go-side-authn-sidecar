You're absolutely right! The functions `parseRSAPublicKey` and `fetchPublicKeys` are directly related to how we handle and parse the JWT public keys and should indeed be part of the `jwt_parser.go` file rather than in `main.go`. This will make the separation of concerns clearer and encapsulate all JWT-related logic within the `jwt_parser.go` file.

Let's correct that and move the `parseRSAPublicKey` and `fetchPublicKeys` functions to `jwt_parser.go`. We'll also keep the logic in `main.go` to initialize the public keys and use the `jwtparser` package for parsing the tokens.

### Updated `jwt_parser.go`

```go
package jwtparser

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"sync"

	"github.com/golang-jwt/jwt/v4"
)

// Map to store the public keys by kid (Key ID)
var publicKeysCache = make(map[string]*rsa.PublicKey)

// Mutex to ensure thread-safe access to the cache
var cacheMutex sync.RWMutex

// Principal structure (you can modify this based on your JWT content)
type Principal struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// Function to fetch the JWKS from a given URL and cache the public keys
func FetchPublicKeys(jwksURL string) error {
	// Fetch the JWKS from the URL
	resp, err := http.Get(jwksURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Parse the JWKS response
	var jwks map[string][]map[string]interface{}
	if err := json.Unmarshal(body, &jwks); err != nil {
		return err
	}

	// Lock the cache for writing
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Loop through the keys and cache the public keys by kid
	for _, key := range jwks["keys"] {
		kidFromKey, ok := key["kid"].(string)
		if !ok {
			continue
		}

		// Check if we have an RSA key
		if key["kty"] == "RSA" {
			n := key["n"].(string)
			e := key["e"].(string)

			// Parse the modulus 'n' and exponent 'e' to get the public key
			pubKey, err := parseRSAPublicKey(n, e)
			if err != nil {
				return err
			}

			// Cache the public key by kid
			publicKeysCache[kidFromKey] = pubKey
		}
	}

	return nil
}

// Function to parse the modulus and exponent to RSA public key
func parseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	// Decode the base64-encoded modulus (n) and exponent (e)
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, errors.New("failed to decode modulus")
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, errors.New("failed to decode exponent")
	}

	// Convert the decoded modulus (n) to a big.Int
	n := new(big.Int)
	n.SetBytes(nBytes)

	// Convert the exponent (e) to an integer (usually a small value like 65537)
	e := new(big.Int)
	e.SetBytes(eBytes)
	exponent := int(e.Int64())

	// Create the RSA public key
	pubKey := &rsa.PublicKey{
		N: n,
		E: exponent,
	}

	return pubKey, nil
}

// Function to set the public keys in the cache manually
func SetPublicKeysCache(keys map[string]*rsa.PublicKey) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	for kid, key := range keys {
		publicKeysCache[kid] = key
	}
}

// Function to parse and validate the JWT token using the public key
func ParseToken(tokenString string) (*jwt.Token, error) {
	// Parse the JWT token to access the header and extract the 'kid'
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}

	// Extract 'kid' from the JWT header
	kid, ok := token.Header["kid"].(string)
	if !ok || kid == "" {
		return nil, errors.New("missing key ID (kid) in JWT header")
	}

	// Fetch the public key from the cache
	cacheMutex.RLock()
	publicKey, exists := publicKeysCache[kid]
	cacheMutex.RUnlock()

	if !exists {
		return nil, errors.New("invalid key ID (kid) or public key not found in cache")
	}

	// Parse and validate the JWT token using the cached public key
	token, err = jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Ensure token signing method matches
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("invalid signing method")
		}

		// Return the public key used for verifying the JWT
		return publicKey, nil
	})

	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return token, nil
}
```

### Updated `main.go`

Now, in `main.go`, you can use the functions from `jwt_parser.go` without needing the `parseRSAPublicKey` or `fetchPublicKeys` functions directly inside the `main.go`.

Here’s the updated `main.go`:

```go
package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
	"log"
	"strings"
	"time"
	"yourapp/jwtparser" // import the jwtparser package
)

// Function to load configuration from config.yml
func loadConfig(configPath string) (*Config, error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func main() {
	// Load configuration from config.yml
	config, err := loadConfig("config.yml") // Make sure config.yml is in the same directory
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// Replace with the correct JWKS URL from Okta or Keycloak
	jwksURL := "http://localhost:8080/realms/baeldung-keycloak/protocol/openid-connect/certs" // Keycloak JWKS URL

	// Fetch the public keys once when the server starts
	if err := jwtparser.FetchPublicKeys(jwksURL); err != nil {
		log.Fatalf("Error fetching public keys: %v", err)
	}

	// Start a goroutine to periodically refresh the public keys (optional)
	// This can be used to refresh keys if they rotate over time.
	go func() {
		for {
			// Refresh the keys every hour (you can adjust the interval)
			err := jwtparser.FetchPublicKeys(jwksURL)
			if err != nil {
				log.Printf("Error refreshing public keys: %v", err)
			}
			// Sleep for 24 hours before refreshing again
			time.Sleep(24 * time.Hour)
		}
	}()

	app := fiber.New()

	// Reverse proxy handler
	app.All("/*", func(c *fiber.Ctx) error {
		// Skip authentication for Actuator endpoints as defined in config.yml
		if strings.HasPrefix(c.Path(), config.Actuator.PathPrefix) {
			// Directly proxy the request without authentication
			target := config.Actuator.TargetURL + c.OriginalURL() // Use target URL from config
			return proxy.Do(c, target)
		}

		// Extract the JWT token from the Authorization header
		tokenString := c.Get("Authorization")
		if tokenString == "" || !strings.HasPrefix(tokenString, "Bearer ") {
			return fiber.NewError(fiber.StatusUnauthorized, "Missing or malformed token")
		}

		tokenString = tokenString[len("Bearer "):] // Remove "Bearer " prefix

		// Parse and validate the JWT token using the jwtparser package
		token, err := jwtparser.ParseToken(tokenString)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, err.Error())
		}

		// Extract claims (user info, etc.) from the token
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return fiber.NewError(fiber.StatusUnauthorized, "Invalid claims")
		}

		// Map to your Principal struct
		principal := jwtparser.Principal{
			UserID:   getClaimAsString(claims, "user_id"),
			Username: getClaimAsString(claims, "username"),
			Email:    getClaimAsString(claims, "email"),
		}

		// Set principal as a custom header
		c.Locals("Principal", principal)

		// Proxy the request to the real backend
		target := "https://httpbin.org" + c.OriginalURL() // replace with your actual service
		return proxy.Do(c, target)
	})

	log.Fatal(app.Listen(":3001"))
}

func getClaimAsString(claims jwt.MapClaims, key string) string {
	if value, ok := claims[key].(string); ok {
		return value
	}
	return "" // Return an empty string if the claim is not found or not a string
}
```

### Summary of Changes:
1. **Encapsulation**:
    - All JWT-related logic (key fetching, key parsing, and token validation) is now handled within the `jwtparser` package.
    - `main.go` only handles the core application logic, including proxying requests and interacting with the `jwtparser` package.

2. **Refactoring**:
    - Moved `parseRSAPublicKey` and `fetchPublicKeys` to `jwtparser.go` to make the JWT handling module self-contained.

By doing this, your code is more modular, maintainable, and easy to test. The `main.go` is now focused solely on the application logic while the JWT processing is abstracted into a dedicated file.

Writing unit tests for your Go project involves testing the core functionality, especially the JWT parsing and public key fetching logic. Here, I'll show you how to write unit tests for the `jwtparser` package and the HTTP handler in `main.go` using the `testing` package, along with `github.com/stretchr/testify` for assertions and mock dependencies.

### Step 1: Install Testing Dependencies

First, install the necessary testing dependencies if you haven’t already:

```bash
go get github.com/stretchr/testify
```

### Step 2: Write Unit Tests for `jwtparser` Package

We'll begin by writing tests for the core JWT parsing and key fetching logic. Create a new test file in the `jwtparser` directory, for example, `jwtparser_test.go`.

#### `jwtparser/jwtparser_test.go`

```go
package jwtparser

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/golang-jwt/jwt/v4"
)

// Test the FetchPublicKeys function
func TestFetchPublicKeys(t *testing.T) {
	// Mock a JWKS response
	mockJWKS := `{
		"keys": [
			{
				"kty": "RSA",
				"kid": "test-key-id",
				"n": "some-modulus-base64",
				"e": "AQAB"
			}
		]
	}`

	// Mock the http.Get function (you can use the httptest package for real HTTP mocking)
	oldGet := httpGet
	defer func() { httpGet = oldGet }() // Restore original function after the test
	httpGet = func(url string) (*http.Response, error) {
		// Return a mocked response
		return &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(strings.NewReader(mockJWKS)),
		}, nil
	}

	err := FetchPublicKeys("http://example.com/.well-known/jwks.json")
	assert.Nil(t, err, "Should not return an error")

	// Ensure the public key is cached correctly
	cacheMutex.RLock()
	publicKey, exists := publicKeysCache["test-key-id"]
	cacheMutex.RUnlock()
	assert.True(t, exists, "Public key should be cached")
	assert.NotNil(t, publicKey, "Public key should not be nil")
}

// Test the parseRSAPublicKey function with mock data
func TestParseRSAPublicKey(t *testing.T) {
	// Mock base64-encoded modulus and exponent
	nStr := "some-modulus-base64"
	eStr := "AQAB"

	// Call the parse function
	pubKey, err := parseRSAPublicKey(nStr, eStr)

	// Validate the result
	assert.Nil(t, err, "Should not return an error")
	assert.NotNil(t, pubKey, "Public key should not be nil")
}

// Test ParseToken function for valid token parsing
func TestParseTokenValid(t *testing.T) {
	// Create a dummy token with a mocked "kid" header
	claims := jwt.MapClaims{
		"user_id": "123",
		"username": "testuser",
		"email": "test@example.com",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test-key-id"

	// Mock the public key cache
	mockPublicKey := &rsa.PublicKey{
		N: big.NewInt(123),
		E: 65537,
	}
	SetPublicKeysCache(map[string]*rsa.PublicKey{
		"test-key-id": mockPublicKey,
	})

	// Mock the signing process
	tokenString, err := token.SignedString(mockPublicKey)
	assert.Nil(t, err, "Should sign the token without errors")

	// Now call the ParseToken function
	parsedToken, err := ParseToken(tokenString)
	assert.Nil(t, err, "Should not return an error")
	assert.True(t, parsedToken.Valid, "Token should be valid")
}

// Test ParseToken function for invalid token
func TestParseTokenInvalid(t *testing.T) {
	// Call ParseToken with an invalid token string
	invalidTokenString := "invalid-token-string"
	_, err := ParseToken(invalidTokenString)
	assert.NotNil(t, err, "Should return an error for an invalid token")
}

// Test for when no public key is found in cache
func TestParseTokenKeyNotFound(t *testing.T) {
	// Create a dummy token with a mocked "kid" header
	claims := jwt.MapClaims{
		"user_id": "123",
		"username": "testuser",
		"email": "test@example.com",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "nonexistent-key-id"

	// Mock the signing process
	tokenString, err := token.SignedString(nil)
	assert.Nil(t, err, "Should sign the token without errors")

	// Now call the ParseToken function
	_, err = ParseToken(tokenString)
	assert.NotNil(t, err, "Should return an error when public key is not found in cache")
}

```

### Step 3: Mock HTTP Requests for Fetching Keys

In the test above, we mock the HTTP request used in `FetchPublicKeys` to simulate receiving a JWKS (JSON Web Key Set) response. We use a simple string response (`mockJWKS`) for this purpose.

Additionally, the `ParseToken` function expects to find a public key in the cache by `kid`. In this case, we directly manipulate the cache with the `SetPublicKeysCache` function to insert a mock key.

### Step 4: Unit Tests for `main.go`

Unit tests for `main.go` are generally more complex due to the dependencies on HTTP request handling and the external services. You can mock external requests, such as HTTP proxies and JWT validation. Here's an example of a basic test setup for your `main.go` handler.

#### `main_test.go`

```go
package main

import (
	"testing"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock HTTP request for Actuator endpoints
func TestActuatorPathSkipping(t *testing.T) {
	app := fiber.New()

	// Setup mock for proxying requests (mocking the proxy.Do method)
	mockProxy := new(MockProxy)
	mockProxy.On("Do", mock.Anything, mock.Anything).Return(nil)

	// Define a simple route
	app.All("/actuator/*", func(c *fiber.Ctx) error {
		// Simulate skipping authentication for actuator paths
		mockProxy.Do(c, "http://localhost:8080"+c.OriginalURL())
		return nil
	})

	// Send a request to the /actuator/health endpoint
	req := httptest.NewRequest("GET", "/actuator/health", nil)
	resp, _ := app.Test(req)

	// Check the response
	assert.Equal(t, 200, resp.StatusCode, "Should return 200 for actuator health endpoint")
}

// Mock for proxy.Do method
type MockProxy struct {
	mock.Mock
}

func (m *MockProxy) Do(c *fiber.Ctx, target string) error {
	args := m.Called(c, target)
	return args.Error(0)
}
```

In the above test, we mock the `proxy.Do` method using `testify/mock`. This allows us to test the handler logic for the `/actuator` path without actually proxying requests.

### Step 5: Running Tests

Once you've written the tests, you can run them using the `go test` command:

```bash
go test -v
```

This will execute all the tests in your project, including those in the `jwtparser_test.go` and `main_test.go` files.

---

### Summary

- **Unit Testing JWT Parsing**: We've written unit tests for the functions responsible for fetching public keys, parsing RSA keys, and validating JWT tokens.
- **Mocking Dependencies**: We've used the `testify` package to mock HTTP requests and proxy functionality for isolated testing.
- **Handler Tests**: We've written basic tests for the `main.go` handler logic to verify that certain paths (e.g., `/actuator`) bypass authentication.

This should give you a good starting point for writing unit tests for the entire application. You can expand the tests further by mocking more complex dependencies and validating more edge cases.
