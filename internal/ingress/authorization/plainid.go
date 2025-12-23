package authorization

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reverseProxy/internal/ingress/jwtauth"
	"strings"
)

// PlainIdRequest represents the request structure for plainId API
type PlainIdRequest struct {
	Method  string                 `json:"method"`
	Headers map[string]string      `json:"headers"`
	URI     PlainIdURI             `json:"uri"`
	Body    map[string]interface{} `json:"body"`
	Meta    PlainIdMeta            `json:"meta"`
}

// PlainIdURI represents the URI component in plainId request
type PlainIdURI struct {
	Schema    string                 `json:"schema"`
	Authority map[string]string      `json:"authority"`
	Path      []string               `json:"path"`
	Query     map[string]interface{} `json:"query"`
}

// PlainIdMeta represents the metadata component in plainId request
type PlainIdMeta struct {
	RuntimeFineTune RuntimeFineTune `json:"runtimeFineTune"`
}

// RuntimeFineTune represents runtime fine tuning options
type RuntimeFineTune struct {
	CombinedMultiValue bool `json:"combinedMultiValue"`
}

// PlainIdResponse represents the response from plainId API
type PlainIdResponse struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason,omitempty"`
	Permit string `json:"permit,omitempty"`
	Deny   string `json:"deny,omitempty"`
}

// CheckPlainIdAccess performs plainId fine-grained authorization.
// Returns (allow, reason, error). If section disabled or URL is not set, it returns allow=true.
func CheckPlainIdAccess(req RequestInfo, p jwtauth.Principal, bodyData map[string]interface{}) (bool, string, error) {
	c := ConfigOrNil()
	if c == nil || !c.FineGrain.Enabled || c.FineGrain.ValidationURL == "" {
		return true, "plainId check skipped (no config)", nil
	}

	rule, ok := c.FineGrain.MatchRule(req.Method, req.Path)
	if !ok {
		// By default, if no fine-grain rule matches, allow and proceed
		return true, "plainId check skipped (no matching rule)", nil
	}

	plainIdReq, err := buildPlainIdRequest(req, p, rule, bodyData)
	if err != nil {
		return false, "", fmt.Errorf("failed to build plainId request: %w", err)
	}

	return postPlainIdCheck(c.FineGrain, plainIdReq)
}

// buildPlainIdRequest constructs a PlainIdRequest from the incoming request and configuration
func buildPlainIdRequest(req RequestInfo, p jwtauth.Principal, rule FineRule, bodyData map[string]interface{}) (PlainIdRequest, error) {
	// Parse the full URL to extract URI components
	parsedURL, err := url.Parse(req.FullURL)
	if err != nil {
		return PlainIdRequest{}, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Extract authority (host/domain parameters)
	authority := make(map[string]string)
	if parsedURL.Host != "" {
		parts := strings.Split(parsedURL.Host, ":")
		if len(parts) > 0 {
			authority["host"] = parts[0]
		}
		if len(parts) > 1 {
			authority["port"] = parts[1]
		}
	}

	// Build path array
	pathSegments := []string{req.Path}
	pathParts := strings.Split(strings.TrimPrefix(req.Path, "/"), "/")
	pathSegments = append(pathSegments, pathParts...)

	// Extract query parameters
	queryParams := make(map[string]interface{})
	for key, values := range parsedURL.Query() {
		if len(values) == 1 {
			queryParams[key] = values[0]
		} else {
			queryParams[key] = values
		}
	}

	// Build request body by extracting values using JSON paths
	requestBody, err := extractBodyFromRule(bodyData, rule)
	if err != nil {
		return PlainIdRequest{}, fmt.Errorf("failed to extract body fields: %w", err)
	}

	// Build headers - include Authorization and X-Request-Id
	headers := make(map[string]string)
	headers["x-request-id"] = req.GetHeader("X-Request-Id")
	if authHeader, ok := req.Headers["Authorization"]; ok {
		headers["Authorization"] = authHeader
	}

	// Determine schema
	schema := "http"
	if parsedURL.Scheme != "" {
		schema = parsedURL.Scheme
	}

	plainIdReq := PlainIdRequest{
		Method:  req.Method,
		Headers: headers,
		URI: PlainIdURI{
			Schema:    schema,
			Authority: authority,
			Path:      pathSegments,
			Query:     queryParams,
		},
		Body: requestBody,
		Meta: PlainIdMeta{
			RuntimeFineTune: RuntimeFineTune{
				CombinedMultiValue: false,
			},
		},
	}

	return plainIdReq, nil
}

// extractBodyFromRule extracts values from the request body using JSON paths defined in the rule
func extractBodyFromRule(bodyData map[string]interface{}, rule FineRule) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for fieldName, jsonPath := range rule.Body {
		value, err := extractValueFromPath(bodyData, jsonPath)
		if err != nil {
			// If a required field cannot be extracted, return error
			return nil, fmt.Errorf("failed to extract field %q from path %q: %w", fieldName, jsonPath, err)
		}
		result[fieldName] = value
	}

	return result, nil
}

// extractValueFromPath extracts a value from a JSON object using a JSON path (e.g., $.fieldName or $.array[*].field)
// Supports:
// - Simple paths: $.fieldName
// - Nested paths: $.parent.child
// - Array wildcards: $.array[*].field
// - Direct existence check: if field exists, use true/false value
func extractValueFromPath(data map[string]interface{}, jsonPath string) (interface{}, error) {
	// Remove leading $. if present
	path := strings.TrimPrefix(jsonPath, "$.")
	if path == jsonPath && strings.HasPrefix(jsonPath, "$") {
		path = strings.TrimPrefix(jsonPath, "$")
	}

	// Handle array wildcard patterns like array[*].field
	if strings.Contains(path, "[*]") {
		return extractArrayWildcard(data, path)
	}

	// Handle simple path traversal
	parts := strings.Split(path, ".")
	current := interface{}(data)

	for _, part := range parts {
		if part == "" {
			continue
		}

		switch v := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = v[part]
			if !ok {
				// Field doesn't exist - check if this is a boolean existence check
				// If the rule expects a field like "tranTemplateUsed", return false
				if strings.Contains(part, "Used") || strings.Contains(part, "Exists") {
					return false, nil
				}
				return nil, fmt.Errorf("path %q: field %q not found in object", jsonPath, part)
			}
		default:
			return nil, fmt.Errorf("path %q: cannot traverse into non-object at step %q", jsonPath, part)
		}
	}

	return current, nil
}

// extractArrayWildcard extracts values from array elements using wildcard pattern
// e.g., fromAccount[*].accountId extracts all accountId values from fromAccount array
func extractArrayWildcard(data map[string]interface{}, path string) (interface{}, error) {
	// Split path at [*] to get array name and field
	parts := strings.Split(path, "[*]")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid wildcard pattern: %q", path)
	}

	arrayPath := parts[0]
	fieldPath := strings.TrimPrefix(parts[1], ".")

	// Navigate to the array
	arrayValue, err := extractValueFromPath(data, arrayPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract array at path %q: %w", arrayPath, err)
	}

	arr, ok := arrayValue.([]interface{})
	if !ok {
		return nil, fmt.Errorf("path %q does not point to an array", arrayPath)
	}

	// Extract the field from each array element
	var results []interface{}
	for i, item := range arr {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("array element at index %d is not an object", i)
		}

		// Handle nested field extraction
		if fieldPath == "" {
			results = append(results, item)
		} else if strings.Contains(fieldPath, ".") {
			// Nested field extraction
			subParts := strings.Split(fieldPath, ".")
			current := interface{}(itemMap)
			for _, part := range subParts {
				if subMap, ok := current.(map[string]interface{}); ok {
					current = subMap[part]
				}
			}
			if current != nil {
				results = append(results, current)
			}
		} else {
			// Simple field extraction
			if value, ok := itemMap[fieldPath]; ok {
				results = append(results, value)
			}
		}
	}

	return results, nil
}

// postPlainIdCheck sends the plainId request to the validation URL and handles the response
func postPlainIdCheck(conf FineGrainConfig, plainIdReq PlainIdRequest) (bool, string, error) {
	contentByteArray, err := json.Marshal(plainIdReq)
	if err != nil {
		return false, "", fmt.Errorf("failed to marshal plainId request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, conf.ValidationURL, bytes.NewReader(contentByteArray))
	if err != nil {
		return false, "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add authentication if configured
	if conf.ClientAuthMethod == "client_secret_basic" && conf.ClientID != "" {
		req.SetBasicAuth(conf.ClientID, conf.ClientSecret)
	} else if conf.ClientAuthMethod != "" && conf.ClientAuthMethod != "client_secret_basic" {
		return false, "", fmt.Errorf("unsupported client auth method: %s", conf.ClientAuthMethod)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("failed to send plainId request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, "non-2xx from plainId service", errors.New(resp.Status)
	}

	var vr PlainIdResponse
	if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil {
		return false, "", fmt.Errorf("failed to decode plainId response: %w", err)
	}

	// Determine if access is allowed based on response
	// If Permit is set, it means explicitly permitted
	if vr.Permit != "" {
		return true, vr.Permit, nil
	}
	// If Deny is set, it means explicitly denied
	if vr.Deny != "" {
		return false, vr.Deny, nil
	}
	// Fall back to Allow field
	return vr.Allow, vr.Reason, nil
}
