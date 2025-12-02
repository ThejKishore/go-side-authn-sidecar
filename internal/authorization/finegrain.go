package authorization

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"reverseProxy/internal/jwtauth"
)

// finePayload is sent to the fine-grain validation-url
type finePayload struct {
	Principal jwtauth.Principal `json:"principal"`
	Request   RequestInfo       `json:"request"`
	Rule      FineRule          `json:"rule"`
}

// CheckFineGrain performs fine-grained authorization using config.finegrain-check.
// Returns (allow, reason, error). If section disabled or URL is not set, it returns allow=true.
func CheckFineGrain(req RequestInfo, p jwtauth.Principal) (bool, string, error) {
	c := ConfigOrNil()
	if c == nil || !c.FineGrain.Enabled || c.FineGrain.ValidationURL == "" {
		return true, "fine-grain check skipped (no config)", nil
	}
	rule, ok := c.FineGrain.MatchRule(req.Method, req.Path)
	if !ok {
		// By default, if no fine-grain rule matches, allow and proceed
		return true, "fine-grain check skipped (no matching rule)", nil
	}
	payload := finePayload{
		Principal: p,
		Request:   req,
		Rule:      rule,
	}
	return postValidateFine(c.FineGrain, payload)
}

func postValidateFine(conf FineGrainConfig, payload finePayload) (bool, string, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return false, "", err
	}
	req, err := http.NewRequest(http.MethodPost, conf.ValidationURL, bytes.NewReader(b))
	if err != nil {
		return false, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if conf.ClientAuthMethod == "client_secret_basic" && conf.ClientID != "" {
		req.SetBasicAuth(conf.ClientID, conf.ClientSecret)
	} else if conf.ClientAuthMethod != "" && conf.ClientAuthMethod != "client_secret_basic" {
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
