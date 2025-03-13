1. **Install necessary packages**:
   You need the `fiber` package, the JWT validation package, and possibly a proxy package. You can install them using:

   ```bash
   go get github.com/gofiber/fiber/v2
   go get github.com/gofiber/jwt/v2
   go get github.com/gofiber/fiber/v2/middleware/proxy
   ```


To improve the performance of your application by avoiding repeated HTTP requests to fetch the public key from the JWKS endpoint, you can fetch all the public keys once and store them in a map where the `kid` (Key ID) is the key. This way, you can use the cached public key without making an HTTP request each time a JWT is validated.

Here’s how you can optimize the code:

### Key Improvements:
1. **Fetching and Caching Public Keys**: We will fetch the JWKS once when the server starts and store the public keys in a map, indexed by their `kid`. This reduces the need to hit the JWKS URL multiple times.
2. **Reusing Public Keys**: On each request, the JWT will be parsed, and the `kid` will be used to retrieve the public key from the map (instead of calling the JWKS URL each time).

### Updated Code:

```go
package main

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
	"github.com/golang-jwt/jwt/v4"
	"io/ioutil"
	"log"
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
			// Sleep for 1 hour before refreshing again
			time.Sleep(1 * time.Hour)
		}
	}()

	app := fiber.New()

	// Reverse proxy handler
	app.All("/*", func(c *fiber.Ctx) error {

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

### Key Improvements:

1. **Public Key Caching**:
   - All public keys fetched from the JWKS URL are stored in a `map[string]*rsa.PublicKey` where the key is the `kid` (Key ID).
   - The cache is updated once when the server starts and can be periodically refreshed if needed.

2. **Thread-Safety**:
   - Access to the `publicKeysCache` map is synchronized using `sync.RWMutex` to allow concurrent reads and writes safely.

3. **Cache Refreshing (Optional)**:
   - A background goroutine refreshes the public keys periodically (e.g., every hour), ensuring the cache stays up to date if the keys rotate.

4. **Improved Token Validation**:
   - The `kid` from the JWT header is used to retrieve the corresponding public key from the cache. This avoids hitting the JWKS URL on each request, improving performance.

### Additional Considerations:
- **Cache Expiration**: Depending on your system requirements, you may want to implement cache expiration or validation mechanisms to ensure that keys are fresh.
- **Concurrency**: The use of `sync.RWMutex` ensures that multiple requests can read from the cache concurrently without causing race conditions.