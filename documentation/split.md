To split your Go program into multiple files for better organization, we can create separate files for different components such as:

1. **Main application logic (`main.go`)**
2. **JWT and public key management (`jwt.go`)**
3. **Utility functions (`utils.go`)**

Here’s how you can structure the code:

### 1. **`main.go`** (Application entry point)

```go
package main

import (
	"log"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
	"time"
)

func main() {
	// Replace with the correct JWKS URL from Okta or Keycloak
	jwksURL := "http://localhost:8080/realms/baeldung-keycloak/protocol/openid-connect/certs" // Keycloak JWKS URL

	// Fetch the public keys once when the server starts
	if err := fetchPublicKeys(jwksURL); err != nil {
		log.Fatalf("Error fetching public keys: %v", err)
	}

	// Start a goroutine to periodically refresh the public keys (optional)
	// This can be used to refresh keys if they rotate over time.
	go func() {
		for {
			// Refresh the keys every hour (you can adjust the interval)
			err := fetchPublicKeys(jwksURL)
			if err != nil {
				log.Printf("Error refreshing public keys: %v", err)
			}
			// Sleep for 24 hour before refreshing again
			time.Sleep(24 * time.Hour)
		}
	}()

	app := fiber.New()

	// Reverse proxy handler
	app.All("/*", proxyHandler)

	log.Fatal(app.Listen(":3001"))
}
```

### 2. **`jwt.go`** (Handles JWT parsing, public key management)

```go
package main

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/golang-jwt/jwt/v4"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"
	"sync"
)

// Principal structure (you can modify this based on your JWT content)
type Principal struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// Map to store the public keys by kid (Key ID)
var publicKeysCache = make(map[string]*rsa.PublicKey)

// Mutex to ensure thread-safe access to the cache
var cacheMutex sync.RWMutex

// Function to fetch the JWKS from a given URL and cache the public keys
func fetchPublicKeys(jwksURL string) error {
	// Fetch the JWKS from the Okta/Keycloak URL
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

// Function to extract claims as string from JWT token
func getClaimAsString(claims jwt.MapClaims, key string) string {
	if value, ok := claims[key].(string); ok {
		return value
	}
	return "" // Return an empty string if the claim is not found or not a string
}
```

### 3. **`proxy.go`** (Handles proxying requests and validating JWT tokens)

```go
package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
	"github.com/golang-jwt/jwt/v4"
	"strings"
)

// Proxy handler function to validate JWT token and proxy the request
func proxyHandler(c *fiber.Ctx) error {
	// Extract the JWT token from the Authorization header
	tokenString := c.Get("Authorization")
	if tokenString == "" || !strings.HasPrefix(tokenString, "Bearer ") {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing or malformed token")
	}

	tokenString = tokenString[len("Bearer "):] // Remove "Bearer " prefix

	// Parse the JWT token to access the header and extract the 'kid'
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "Error parsing token header")
	}

	// Extract 'kid' from the JWT header
	kid, ok := token.Header["kid"].(string)
	if !ok || kid == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing key ID (kid) in JWT header")
	}

	// Fetch the public key from the cache
	cacheMutex.RLock()
	publicKey, exists := publicKeysCache[kid]
	cacheMutex.RUnlock()

	if !exists {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid key ID (kid) or public key not found in cache")
	}

	// Parse and validate the JWT token using the cached public key
	token, err = jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Ensure token signing method matches
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid signing method")
		}

		// Return the public key used for verifying the JWT
		return publicKey, nil
	})

	if err != nil || !token.Valid {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid token")
	}

	// Extract claims (user info, etc.) from the token
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid claims")
	}

	// Map to your Principal struct
	principal := Principal{
		UserID:   getClaimAsString(claims, "user_id"),
		Username: getClaimAsString(claims, "username"),
		Email:    getClaimAsString(claims, "email"),
	}

	// Set principal as a custom header
	c.Locals("Principal", principal)

	// Proxy the request to the real backend
	target := "https://httpbin.org" + c.OriginalURL() // replace with your actual service

	return proxy.Do(c, target)
}
```

### Directory Structure:
```
/your-project-directory
  |- main.go
  |- jwt.go
  |- proxy.go
  |- utils.go
```

### Explanation:

- **`main.go`**: Starts the application, manages the periodic refresh of public keys, and sets up the Fiber server.
- **`jwt.go`**: Manages JWT token parsing, public key fetching, and caching.
- **`proxy.go`**: Handles reverse proxying of requests and validates JWT tokens.

This structure will keep the code modular and easy to maintain, making it easier to work with JWT tokens and reverse proxy requests efficiently.

To bypass the authentication check for certain endpoints (such as `/healthz` or other Spring Boot actuator endpoints), you can modify the request handling logic in the `proxyHandler` function.

You can add a simple check at the start of the handler to skip the authentication process for those specific paths.

Here’s how you can modify the `proxyHandler` in `proxy.go` to implement this logic:

### Updated `proxy.go` with bypass logic:

```go
package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
	"strings"
)

// Proxy handler function to validate JWT token and proxy the request
func proxyHandler(c *fiber.Ctx) error {
	// Bypass authentication for health check or actuator endpoints
	if shouldBypassAuth(c.OriginalURL()) {
		// Simply proxy the request without any authentication check
		target := "https://httpbin.org" + c.OriginalURL() // Replace with your actual service
		return proxy.Do(c, target)
	}

	// Extract the JWT token from the Authorization header
	tokenString := c.Get("Authorization")
	if tokenString == "" || !strings.HasPrefix(tokenString, "Bearer ") {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing or malformed token")
	}

	tokenString = tokenString[len("Bearer "):] // Remove "Bearer " prefix

	// Parse the JWT token to access the header and extract the 'kid'
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "Error parsing token header")
	}

	// Extract 'kid' from the JWT header
	kid, ok := token.Header["kid"].(string)
	if !ok || kid == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing key ID (kid) in JWT header")
	}

	// Fetch the public key from the cache
	cacheMutex.RLock()
	publicKey, exists := publicKeysCache[kid]
	cacheMutex.RUnlock()

	if !exists {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid key ID (kid) or public key not found in cache")
	}

	// Parse and validate the JWT token using the cached public key
	token, err = jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Ensure token signing method matches
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid signing method")
		}

		// Return the public key used for verifying the JWT
		return publicKey, nil
	})

	if err != nil || !token.Valid {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid token")
	}

	// Extract claims (user info, etc.) from the token
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid claims")
	}

	// Map to your Principal struct
	principal := Principal{
		UserID:   getClaimAsString(claims, "user_id"),
		Username: getClaimAsString(claims, "username"),
		Email:    getClaimAsString(claims, "email"),
	}

	// Set principal as a custom header
	c.Locals("Principal", principal)

	// Proxy the request to the real backend
	target := "https://httpbin.org" + c.OriginalURL() // replace with your actual service

	return proxy.Do(c, target)
}

// Helper function to check if the endpoint should bypass authentication
func shouldBypassAuth(url string) bool {
	// Add more endpoints as needed
	skipPaths := []string{"/healthz", "/actuator", "/actuator/health", "/actuator/info"}
	for _, path := range skipPaths {
		if strings.HasPrefix(url, path) {
			return true
		}
	}
	return false
}
```

### Explanation of Changes:

1. **`shouldBypassAuth` function**:
    - This function takes the requested URL and checks if it matches any of the paths that should bypass authentication (e.g., `/healthz`, `/actuator`, or any other health check endpoints).
    - If the URL matches any of these paths, the function returns `true` to indicate that authentication should be bypassed.

2. **Bypass Authentication Logic**:
    - In the `proxyHandler`, the `shouldBypassAuth` function is called at the beginning of the request handler.
    - If the request path is a health check or actuator endpoint, the handler immediately proxies the request without performing any authentication checks.
    - If the path is not one of the exempt paths, the JWT authentication process proceeds as usual.

### How It Works:

- When a request is received, the handler first checks if the URL matches one of the exempt paths (`/healthz`, `/actuator`, etc.).
- If it does, the proxy request is forwarded without verifying the JWT token.
- If it doesn't match, the standard JWT validation and authentication logic are applied.

### Example of Supported Bypassed URLs:

- `/healthz` (commonly used for health checks in Kubernetes, etc.)
- `/actuator` (common Spring Boot actuator endpoints)
- `/actuator/health` (specific Spring Boot health endpoint)
- `/actuator/info` (Spring Boot info endpoint)

You can easily extend the `skipPaths` slice in the `shouldBypassAuth` function to add more endpoints that should bypass the authentication check.

To integrate external authorization using a Policy Decision Point (PDP) like **PlainID** in the `proxy.go`, you will need to send the JWT principal along with the request payload to the PDP for evaluation.

Here's a breakdown of the steps we will take to accomplish this:

1. **Send the JWT principal and payload** to the PlainID PDP (Policy Decision Point).
2. **Perform the authorization check**: The PDP will evaluate whether the request is allowed or denied based on the configured policies.
3. **Forward the request** to the backend if authorization is granted.
4. **Deny the request** if the authorization fails, and return a 403 Forbidden status.

### Key Assumptions:
- You will be sending an HTTP request to the PlainID PDP for authorization.
- The PlainID PDP expects a request with the JWT claims (the principal) and the request payload (e.g., HTTP method, URL, etc.).
- You will need to adjust the URL and headers to fit your PlainID configuration.

### Updated `proxy.go` with PlainID External Authorization

```go
package main

import (
	"bytes"
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
	"github.com/golang-jwt/jwt/v4"
	"net/http"
	"strings"
	"log"
)

// Proxy handler function to validate JWT token, apply external authorization, and proxy the request
func proxyHandler(c *fiber.Ctx) error {
	// Bypass authentication for health check or actuator endpoints
	if shouldBypassAuth(c.OriginalURL()) {
		// Simply proxy the request without any authentication check
		target := "https://httpbin.org" + c.OriginalURL() // Replace with your actual service
		return proxy.Do(c, target)
	}

	// Extract the JWT token from the Authorization header
	tokenString := c.Get("Authorization")
	if tokenString == "" || !strings.HasPrefix(tokenString, "Bearer ") {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing or malformed token")
	}

	tokenString = tokenString[len("Bearer "):] // Remove "Bearer " prefix

	// Parse the JWT token to access the header and extract the 'kid'
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "Error parsing token header")
	}

	// Extract 'kid' from the JWT header
	kid, ok := token.Header["kid"].(string)
	if !ok || kid == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing key ID (kid) in JWT header")
	}

	// Fetch the public key from the cache
	cacheMutex.RLock()
	publicKey, exists := publicKeysCache[kid]
	cacheMutex.RUnlock()

	if !exists {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid key ID (kid) or public key not found in cache")
	}

	// Parse and validate the JWT token using the cached public key
	token, err = jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Ensure token signing method matches
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid signing method")
		}

		// Return the public key used for verifying the JWT
		return publicKey, nil
	})

	if err != nil || !token.Valid {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid token")
	}

	// Extract claims (user info, etc.) from the token
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid claims")
	}

	// Map to your Principal struct
	principal := Principal{
		UserID:   getClaimAsString(claims, "user_id"),
		Username: getClaimAsString(claims, "username"),
		Email:    getClaimAsString(claims, "email"),
	}

	// Perform external authorization via PlainID PDP
	authorized, err := externalAuthorization(c, principal, claims)
	if err != nil || !authorized {
		return fiber.NewError(fiber.StatusForbidden, "Forbidden: External authorization failed")
	}

	// Set principal as a custom header
	c.Locals("Principal", principal)

	// Proxy the request to the real backend
	target := "https://httpbin.org" + c.OriginalURL() // replace with your actual service

	return proxy.Do(c, target)
}

// Function to perform external authorization via PlainID PDP
func externalAuthorization(c *fiber.Ctx, principal Principal, claims jwt.MapClaims) (bool, error) {
	// Prepare the request payload for the PDP
	authorizationRequest := map[string]interface{}{
		"user": principal, // Pass the JWT principal (claims) to the PDP
		"method": c.Method(), // HTTP method (e.g., GET, POST)
		"url": c.OriginalURL(), // Requested URL
		"headers": c.GetReqHeaders(), // Headers from the request
		"body": c.Body(), // Request body (if applicable)
	}

	// Convert the authorization request to JSON
	authorizationRequestJSON, err := json.Marshal(authorizationRequest)
	if err != nil {
		return false, err
	}

	// Make a request to PlainID PDP for evaluation (replace URL with your PDP endpoint)
	plainIDURL := "https://your-plainid-pdp-url" // Replace with your actual PDP URL
	req, err := http.NewRequest("POST", plainIDURL, bytes.NewBuffer(authorizationRequestJSON))
	if err != nil {
		return false, err
	}

	// Set headers if necessary (for example, API key for PlainID)
	req.Header.Set("Content-Type", "application/json")

	// Send the request to PlainID PDP
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// Check the response from PlainID
	if resp.StatusCode != http.StatusOK {
		log.Printf("PlainID PDP authorization failed: %s", resp.Status)
		return false, nil // Deny the request if the PDP response is not OK
	}

	// If the PDP response is successful, assume the request is authorized
	// Optionally, you could parse the response and check additional attributes.
	return true, nil
}

// Helper function to check if the endpoint should bypass authentication
func shouldBypassAuth(url string) bool {
	// Add more endpoints as needed
	skipPaths := []string{"/healthz", "/actuator", "/actuator/health", "/actuator/info"}
	for _, path := range skipPaths {
		if strings.HasPrefix(url, path) {
			return true
		}
	}
	return false
}
```

### Explanation of Changes:

1. **`externalAuthorization` function**:
    - This function sends a `POST` request to the PlainID PDP to check whether the current user (JWT principal) is authorized to access the requested resource.
    - The request payload includes the JWT claims (principal), HTTP method, URL, headers, and body of the incoming request.
    - The response from PlainID is expected to be a status indicating whether the request is authorized. If it is, the request proceeds; if not, a `403 Forbidden` status is returned.

2. **Authorization Request Payload**:
    - The payload includes the JWT claims (mapped to the `Principal` struct), HTTP method, URL, request headers, and body.
    - This gives the PDP all the necessary context to evaluate the authorization decision.

3. **PDP Request to PlainID**:
    - A `POST` request is sent to the PlainID PDP with the authorization request as JSON.
    - Ensure that you replace the `plainIDURL` with your actual PDP URL and adjust the headers or authentication (e.g., API key) as needed for PlainID.

4. **Authorization Decision**:
    - If the PlainID PDP responds with a `200 OK` status, the request is considered authorized and the proxy process continues.
    - If the PDP returns any other status (e.g., `403 Forbidden`), the request is blocked, and the handler returns an error.

5. **`shouldBypassAuth`**:
    - This function remains the same, allowing certain endpoints (like health checks) to bypass authentication.

### Important Considerations:

- **PlainID PDP URL**: Ensure that the `plainIDURL` variable points to your actual PlainID PDP endpoint.
- **PDP Response Handling**: In this implementation, if the response status is `200 OK`, it’s considered authorized. You may want to parse the response body further if PlainID provides more detailed information (e.g., reasons for denial).
- **Security**: Ensure you are using secure communication (HTTPS) when sending data to the PlainID PDP, especially when transmitting sensitive information like JWT claims.

This setup should now integrate external authorization into your proxy handler, leveraging PlainID as a PDP for fine-grained access control.