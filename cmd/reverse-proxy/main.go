package main

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v3"

	"reverseProxy/internal/authorization"
	"reverseProxy/internal/egressconfig"
	"reverseProxy/internal/egressproxy"
	"reverseProxy/internal/jwtauth"
	"reverseProxy/internal/proxyhandler"
	"reverseProxy/internal/tokenmanager"
)

func main() {
	// Replace with the correct JWKS URL from Okta or Keycloak
	jwksURL := "http://localhost:8080/realms/baeldung-keycloak/protocol/openid-connect/certs" // Keycloak JWKS URL

	// Fetch the public keys once when the server starts
	if err := jwtauth.FetchPublicKeys(jwksURL); err != nil {
		log.Fatalf("Error fetching public keys: %v", err)
	}

	// Load authorization rules from YAML (authorization.yaml at project root by default)
	if err := authorization.Load("authorization.yaml"); err != nil {
		// Not fatal: allow running without external authorization during local dev
		log.Printf("authorization config not loaded: %v (authorization checks may be skipped)", err)
	}

	// Start a goroutine to periodically refresh the public keys (optional)
	// This can be used to refresh keys if they rotate over time.
	go func() {
		for {
			// Refresh the keys every hour (you can adjust the interval)
			err := jwtauth.FetchPublicKeys(jwksURL)
			if err != nil {
				log.Printf("Error refreshing public keys: %v", err)
			}
			// Sleep for 24 hour before refreshing again
			time.Sleep(24 * time.Hour)
		}
	}()

	go egressProxy()

	app := fiber.New()

	// Reverse proxy handler
	app.All("/*", proxyhandler.Handler)

	log.Fatal(app.Listen(":3001"))
}

func egressProxy() {
	// Load egress configuration from YAML (egress-config.yaml at project root by default)
	if err := egressconfig.Load("egress-config.yaml"); err != nil {
		log.Printf("egress config not loaded: %v (egress proxy will operate in noIdp mode only)", err)
	}

	// Start token refresh manager (10-minute interval)
	tokenMgr := tokenmanager.GetInstance()
	if err := tokenMgr.StartTokenRefresh(10 * time.Minute); err != nil {
		log.Printf("Failed to start token refresh manager: %v", err)
	}

	app := fiber.New()

	// Egress proxy handler
	app.All("/*", egressproxy.Handler)

	log.Fatal(app.Listen(":3002"))
}
