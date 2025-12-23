package authorization

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reverseProxy/internal/ingress/jwtauth"
	"strings"
	"time"
)

// RequestInfo captures minimal request context sent to validation services
type RequestInfo struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	FullURL string            `json:"full_url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// GetHeader retrieves a header value (case-insensitive)
func (r RequestInfo) GetHeader(key string) string {
	// Try exact match first
	if val, ok := r.Headers[key]; ok {
		return val
	}
	// Try case-insensitive match
	for k, v := range r.Headers {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
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

var httpClient = &http.Client{
	Timeout: 5 * time.Second,
}

// CheckCoarseAccess performs coarse authorization using config.coarse-check from authorization.yaml.
// Returns (allow, reason, error). If section disabled or URL is not set, it returns allow=true.
func CheckCoarseAccess(req RequestInfo, p jwtauth.Principal) (bool, string, error) {
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
	return postCoarseCheck(c.Coarse, payload)
}

func postCoarseCheck(conf CoarseConfig, payload coarsePayload) (bool, string, error) {
	contentByteArray, marshalErr := json.Marshal(payload)

	if marshalErr != nil {
		return false, "", marshalErr
	}

	newHttpReq, netWorkErr := http.NewRequest(http.MethodPost, conf.ValidationURL, bytes.NewReader(contentByteArray))

	if netWorkErr != nil {
		return false, "", marshalErr
	}

	newHttpReq.Header.Set("Content-Type", "application/json")
	// client_secret_basic support

	if conf.ClientAuthMethod == "client_secret_basic" && conf.ClientID != "" {
		newHttpReq.SetBasicAuth(conf.ClientID, conf.ClientSecret)
	} else if conf.ClientAuthMethod != "" && conf.ClientAuthMethod != "client_secret_basic" {
		// unsupported method configured
		return false, "", fmt.Errorf("unsupported client auth method: %s", conf.ClientAuthMethod)
	}
	resp, netWorkErr := httpClient.Do(newHttpReq)

	if netWorkErr != nil {
		return false, "", netWorkErr
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
