package jwtauth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/big"
	"net/http"
	"sync"
)

// Principal represents the authenticated user extracted from JWT claims
type Principal struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// publicKeysCache stores the public keys by kid (Key ID)
var publicKeysCache = make(map[string]*rsa.PublicKey)

// cacheMutex ensures thread-safe access to the cache
var cacheMutex sync.RWMutex

// FetchPublicKeys fetches the JWKS from a given URL and caches the public keys
func FetchPublicKeys(jwksURL string) error {
	resp, err := http.Get(jwksURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var jwks map[string][]map[string]interface{}
	if err := json.Unmarshal(body, &jwks); err != nil {
		return err
	}

	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	for _, key := range jwks["keys"] {
		kidFromKey, ok := key["kid"].(string)
		if !ok {
			continue
		}
		if key["kty"] == "RSA" {
			nVal, nOK := key["n"].(string)
			eVal, eOK := key["e"].(string)
			if !nOK || !eOK {
				continue
			}
			pubKey, err := parseRSAPublicKey(nVal, eVal)
			if err != nil {
				return err
			}
			publicKeysCache[kidFromKey] = pubKey
		}
	}
	return nil
}

// parseRSAPublicKey converts modulus and exponent to RSA public key
func parseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, errors.New("failed to decode modulus")
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, errors.New("failed to decode exponent")
	}
	n := new(big.Int)
	n.SetBytes(nBytes)
	e := new(big.Int)
	e.SetBytes(eBytes)
	exponent := int(e.Int64())
	return &rsa.PublicKey{N: n, E: exponent}, nil
}

// GetPublicKey returns a cached public key for a given kid and a boolean indicating existence
func GetPublicKey(kid string) (*rsa.PublicKey, bool) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	pk, ok := publicKeysCache[kid]
	return pk, ok
}

// SetPublicKeyForTest allows tests to seed the cache. Do not use in production code paths.
func SetPublicKeyForTest(kid string, pk *rsa.PublicKey) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	publicKeysCache[kid] = pk
}
