package proxyhandler

import (
    "encoding/base64"
    "encoding/json"
    "log"
    "reverseProxy/internal/authorization"
    "reverseProxy/internal/jwtauth"
    "reverseProxy/internal/util"
    "strings"

    "github.com/gofiber/fiber/v3"
    fiberproxy "github.com/gofiber/fiber/v3/middleware/proxy"
    "github.com/golang-jwt/jwt/v5"
)

// doProxy is an indirection over proxy.Do to allow stubbing in tests
var doProxy = func(c fiber.Ctx, url string) error { return fiberproxy.Do(c, url) }

// Handler validates JWT, sets principal, and proxies the request
func Handler(c fiber.Ctx) error {
	// Extract the JWT token from the Authorization header
	jwtError, isJwtError := jwtAuthenticate(c)
	if isJwtError {
		return jwtError
	}

	// Run coarse and fine-grain authorization if configured
	principal, _ := c.Locals("Principal").(jwtauth.Principal)

	log.Printf("Authorization: %s", principal)

	reqInfo := authorization.RequestInfo{
		Method: c.Method(),
		Path:   c.OriginalURL(),
	}

 // Run coarse and fine-grain authorization concurrently and wait for both
 type authResult struct {
     allow  bool
     reason string
     err    error
 }

 coarseCh := make(chan authResult, 1)
 fineCh := make(chan authResult, 1)

 go func() {
     allow, reason, err := authorization.CheckCoarseAccess(reqInfo, principal)
     coarseCh <- authResult{allow: allow, reason: reason, err: err}
 }()

 go func() {
     allow, reason, err := authorization.CheckFineGrainAccess(reqInfo, principal)
     fineCh <- authResult{allow: allow, reason: reason, err: err}
 }()

 coarseRes := <-coarseCh
 fineRes := <-fineCh

 // Validate both results before proxying
 if coarseRes.err != nil {
     return fiber.NewError(fiber.StatusForbidden, "coarse authorization error: "+coarseRes.err.Error())
 }
 if !coarseRes.allow {
     reason := coarseRes.reason
     if reason == "" {
         reason = "coarse authorization denied"
     }
     return fiber.NewError(fiber.StatusForbidden, reason)
 }

 if fineRes.err != nil {
     return fiber.NewError(fiber.StatusForbidden, "fine-grain authorization error: "+fineRes.err.Error())
 }
 if !fineRes.allow {
     reason := fineRes.reason
     if reason == "" {
         reason = "fine-grain authorization denied"
     }
     return fiber.NewError(fiber.StatusForbidden, reason)
 }

	// Proxy the request to the real backend
	target := "https://httpbin.org" + c.OriginalURL() // replace with your actual service
	return doProxy(c, target)
}

func jwtAuthenticate(c fiber.Ctx) (error, bool) {
	tokenString := c.Get("Authorization")
	if tokenString == "" || !strings.HasPrefix(tokenString, "Bearer ") {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing or malformed token"), true
	}
	// Remove "Bearer " prefix
	tokenString = tokenString[len("Bearer "):]

 // Parse the JWT header manually to extract the 'kid'
 parts := strings.Split(tokenString, ".")
 if len(parts) < 2 {
     return fiber.NewError(fiber.StatusUnauthorized, "Malformed token"), true
 }
 headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
 if err != nil {
     return fiber.NewError(fiber.StatusUnauthorized, "Error decoding token header"), true
 }
 var header map[string]interface{}
 if err := json.Unmarshal(headerBytes, &header); err != nil {
     return fiber.NewError(fiber.StatusUnauthorized, "Error parsing token header"), true
 }
 kid, ok := header["kid"].(string)
 if !ok || kid == "" {
     return fiber.NewError(fiber.StatusUnauthorized, "Missing key ID (kid) in JWT header"), true
 }

	// Fetch the public key from the cache
	publicKey, exists := jwtauth.GetPublicKey(kid)
	if !exists {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid key ID (kid) or public key not found in cache"), true
	}

 // Parse and validate the JWT token using the cached public key
 claims := jwt.MapClaims{}
 _, err = jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
     // Ensure token signing method matches
     if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
         return nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid signing method")
     }
     return publicKey, nil
 })
 if err != nil {
     return fiber.NewError(fiber.StatusUnauthorized, "Invalid token"), true
 }
	principal := jwtauth.Principal{
		UserID:   util.GetClaimAsString(claims, "user_id"),
		Username: util.GetClaimAsString(claims, "username"),
		Email:    util.GetClaimAsString(claims, "email"),
	}
	c.Locals("Principal", principal)
	return nil, false
}
