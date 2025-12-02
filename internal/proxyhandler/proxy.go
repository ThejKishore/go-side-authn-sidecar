package proxyhandler

import (
	"reverseProxy/internal/authorization"
	"reverseProxy/internal/jwtauth"
	"reverseProxy/internal/util"
	"strings"

	"github.com/gofiber/fiber/v3"
	fiberproxy "github.com/gofiber/fiber/v3/middleware/proxy"
	"github.com/golang-jwt/jwt/v4"
)

// doProxy is an indirection over proxy.Do to allow stubbing in tests
var doProxy = func(c fiber.Ctx, url string) error { return fiberproxy.Do(c, url) }

// Handler validates JWT, sets principal, and proxies the request
func Handler(c fiber.Ctx) error {
	// Extract the JWT token from the Authorization header
	err2, done := jwtAuthenticate(c)
	if done {
		return err2
	}

	// Run coarse and fine-grain authorization if configured
	principal, _ := c.Locals("Principal").(jwtauth.Principal)
	reqInfo := authorization.RequestInfo{Method: c.Method(), Path: c.OriginalURL()}

	if allow, reason, err := authorization.CheckCoarse(reqInfo, principal); err != nil {
		return fiber.NewError(fiber.StatusForbidden, "coarse authorization error: "+err.Error())
	} else if !allow {
		if reason == "" {
			reason = "coarse authorization denied"
		}
		return fiber.NewError(fiber.StatusForbidden, reason)
	}

	if allow, reason, err := authorization.CheckFineGrain(reqInfo, principal); err != nil {
		return fiber.NewError(fiber.StatusForbidden, "fine-grain authorization error: "+err.Error())
	} else if !allow {
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

	// Parse the JWT token to access the header and extract the 'kid'
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "Error parsing token header"), true
	}
	kid, ok := token.Header["kid"].(string)
	if !ok || kid == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "Missing key ID (kid) in JWT header"), true
	}

	// Fetch the public key from the cache
	publicKey, exists := jwtauth.GetPublicKey(kid)
	if !exists {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid key ID (kid) or public key not found in cache"), true
	}

	// Parse and validate the JWT token using the cached public key
	token, err = jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Ensure token signing method matches
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid signing method")
		}
		return publicKey, nil
	})
	if err != nil || !token.Valid {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid token"), true
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid claims"), true
	}
	principal := jwtauth.Principal{
		UserID:   util.GetClaimAsString(claims, "user_id"),
		Username: util.GetClaimAsString(claims, "username"),
		Email:    util.GetClaimAsString(claims, "email"),
	}
	c.Locals("Principal", principal)
	return nil, false
}
