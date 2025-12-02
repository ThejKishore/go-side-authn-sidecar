package authorization

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"reverseProxy/internal/jwtauth"
)

// RequestInfo captures minimal request context sent to validation services
type RequestInfo struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
}

// coarsePayload is sent to the coarse validation-url
type coarsePayload struct {
	Principal       jwtauth.Principal `json:"principal"`
	Request         RequestInfo       `json:"request"`
	Resource        string            `json:"resource"`
	AnonymousAccess bool              `json:"anonymous_access"`
}

type validationResponse struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason,omitempty"`
}

var httpClient = &http.Client{Timeout: 5 * time.Second}

// CheckCoarse performs coarse authorization using config.coarse-check from authorization.yaml.
// Returns (allow, reason, error). If section disabled or URL is not set, it returns allow=true.
func CheckCoarse(req RequestInfo, p jwtauth.Principal) (bool, string, error) {
	c := ConfigOrNil()
	if c == nil || !c.Coarse.Enabled || c.Coarse.ValidationURL == "" {
		return true, "coarse check skipped (no config)", nil
	}
	resource, ok := c.Coarse.MatchResource(req.Path)
	if !ok {
		if c.Coarse.AnonymousAccess {
			return true, "coarse check allowed (no matching resource; anonymous-access=true)", nil
		}
		return false, "coarse check denied (no matching resource)", nil
	}
	payload := coarsePayload{
		Principal:       p,
		Request:         req,
		Resource:        resource,
		AnonymousAccess: c.Coarse.AnonymousAccess,
	}
	return postValidateCoarse(c.Coarse, payload)
}

func postValidateCoarse(conf CoarseConfig, payload coarsePayload) (bool, string, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return false, "", err
	}
	req, err := http.NewRequest(http.MethodPost, conf.ValidationURL, bytes.NewReader(b))
	if err != nil {
		return false, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	// client_secret_basic support
	if conf.ClientAuthMethod == "client_secret_basic" && conf.ClientID != "" {
		req.SetBasicAuth(conf.ClientID, conf.ClientSecret)
	} else if conf.ClientAuthMethod != "" && conf.ClientAuthMethod != "client_secret_basic" {
		// unsupported method configured
		return false, "", fmt.Errorf("unsupported client auth method: %s", conf.ClientAuthMethod)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, "non-2xx from validation service", errors.New(resp.Status)
	}
	var vr validationResponse
	if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil {
		return false, "", err
	}
	return vr.Allow, vr.Reason, nil
}
