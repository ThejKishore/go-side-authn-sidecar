package authorization

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reverseProxy/internal/ingress/jwtauth"
	"testing"
)

// MockPlainIdServer creates a mock plainId service for testing
type MockPlainIdServer struct {
	Server   *httptest.Server
	Handler  http.HandlerFunc
	LastReq  *PlainIdRequest
	Requests []*PlainIdRequest
}

// NewMockPlainIdServer creates a new mock plainId server that accepts requests and returns a default allow response
func NewMockPlainIdServer() *MockPlainIdServer {
	m := &MockPlainIdServer{
		Requests: make([]*PlainIdRequest, 0),
	}

	m.Handler = func(w http.ResponseWriter, r *http.Request) {
		var req PlainIdRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		m.LastReq = &req
		m.Requests = append(m.Requests, &req)

		// Default response is allow
		resp := PlainIdResponse{Allow: true, Reason: "mock allow"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}

	m.Server = httptest.NewServer(m.Handler)
	return m
}

// Close shuts down the mock server
func (m *MockPlainIdServer) Close() {
	m.Server.Close()
}

// URL returns the mock server URL
func (m *MockPlainIdServer) URL() string {
	return m.Server.URL
}

// SetHandler allows changing the response behavior
func (m *MockPlainIdServer) SetHandler(handler http.HandlerFunc) {
	m.Handler = handler
	m.Server.Config.Handler = handler
}

// SetDenyResponse configures the server to return a deny response
func (m *MockPlainIdServer) SetDenyResponse(reason string) {
	m.SetHandler(func(w http.ResponseWriter, r *http.Request) {
		var req PlainIdRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		m.LastReq = &req
		m.Requests = append(m.Requests, &req)

		resp := PlainIdResponse{Allow: false, Reason: reason}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// SetPermitResponse configures the server to return a permit response
func (m *MockPlainIdServer) SetPermitResponse(permit string) {
	m.SetHandler(func(w http.ResponseWriter, r *http.Request) {
		var req PlainIdRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		m.LastReq = &req
		m.Requests = append(m.Requests, &req)

		resp := PlainIdResponse{Permit: permit}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// SetErrorResponse configures the server to return an error
func (m *MockPlainIdServer) SetErrorResponse(statusCode int, message string) {
	m.SetHandler(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, message, statusCode)
	})
}

// TestHelper provides utility methods for testing plainId authorization
type TestHelper struct {
	t      *testing.T
	mock   *MockPlainIdServer
	config *Config
}

// NewTestHelper creates a new test helper with a mock plainId server
func NewTestHelper(t *testing.T) *TestHelper {
	mock := NewMockPlainIdServer()
	return &TestHelper{
		t:    t,
		mock: mock,
	}
}

// Close cleans up the test helper resources
func (h *TestHelper) Close() {
	h.mock.Close()
}

// SetupConfig configures the authorization system for testing
func (h *TestHelper) SetupConfig(resourceMap map[string]FineRule) {
	h.config = &Config{
		FineGrain: FineGrainConfig{
			Enabled:       true,
			ValidationURL: h.mock.URL(),
			ResourceMap:   resourceMap,
		},
	}

	// Save old config
	oldCfg := cfg
	cfg = h.config

	// Register cleanup
	h.t.Cleanup(func() { cfg = oldCfg })
}

// CheckAccess performs a plainId authorization check for testing
func (h *TestHelper) CheckAccess(method, path, fullURL string, headers map[string]string, bodyData map[string]interface{}) (bool, string, error) {
	req := RequestInfo{
		Method:  method,
		Path:    path,
		FullURL: fullURL,
		Headers: headers,
	}

	principal := jwtauth.Principal{
		UserID:   "test-user",
		Username: "testuser",
		Email:    "test@example.com",
	}

	return CheckPlainIdAccess(req, principal, bodyData)
}

// GetLastRequest returns the last request received by the mock server
func (h *TestHelper) GetLastRequest() *PlainIdRequest {
	return h.mock.LastReq
}

// GetAllRequests returns all requests received by the mock server
func (h *TestHelper) GetAllRequests() []*PlainIdRequest {
	return h.mock.Requests
}

// SetDenyResponse configures the mock to return a deny response
func (h *TestHelper) SetDenyResponse(reason string) {
	h.mock.SetDenyResponse(reason)
}

// SetPermitResponse configures the mock to return a permit response
func (h *TestHelper) SetPermitResponse(permit string) {
	h.mock.SetPermitResponse(permit)
}

// SetErrorResponse configures the mock to return an error
func (h *TestHelper) SetErrorResponse(statusCode int, message string) {
	h.mock.SetErrorResponse(statusCode, message)
}

// AssertHeaderPresent verifies a header is present in the plainId request
func (h *TestHelper) AssertHeaderPresent(headerName string) {
	if h.mock.LastReq == nil {
		h.t.Fatal("no request was made to plainId")
	}
	if _, ok := h.mock.LastReq.Headers[headerName]; !ok {
		h.t.Errorf("expected header %q to be present", headerName)
	}
}

// AssertBodyField verifies a body field was extracted correctly
func (h *TestHelper) AssertBodyField(fieldName string, expectedValue interface{}) {
	if h.mock.LastReq == nil {
		h.t.Fatal("no request was made to plainId")
	}
	actual, ok := h.mock.LastReq.Body[fieldName]
	if !ok {
		h.t.Errorf("expected field %q to be present in body", fieldName)
		return
	}
	if actual != expectedValue {
		h.t.Errorf("field %q: expected %v, got %v", fieldName, expectedValue, actual)
	}
}

// AssertPathSegment verifies a path segment was extracted correctly
func (h *TestHelper) AssertPathSegment(index int, expectedSegment string) {
	if h.mock.LastReq == nil {
		h.t.Fatal("no request was made to plainId")
	}
	if index >= len(h.mock.LastReq.URI.Path) {
		h.t.Errorf("path only has %d segments, requested index %d", len(h.mock.LastReq.URI.Path), index)
		return
	}
	if h.mock.LastReq.URI.Path[index] != expectedSegment {
		h.t.Errorf("path segment %d: expected %q, got %q", index, expectedSegment, h.mock.LastReq.URI.Path[index])
	}
}

// AssertQueryParam verifies a query parameter was extracted
func (h *TestHelper) AssertQueryParam(paramName string, expectedValue interface{}) {
	if h.mock.LastReq == nil {
		h.t.Fatal("no request was made to plainId")
	}
	actual, ok := h.mock.LastReq.URI.Query[paramName]
	if !ok {
		h.t.Errorf("expected query param %q to be present", paramName)
		return
	}
	if actual != expectedValue {
		h.t.Errorf("query param %q: expected %v, got %v", paramName, expectedValue, actual)
	}
}

// AssertURISchema verifies the URI schema
func (h *TestHelper) AssertURISchema(expectedSchema string) {
	if h.mock.LastReq == nil {
		h.t.Fatal("no request was made to plainId")
	}
	if h.mock.LastReq.URI.Schema != expectedSchema {
		h.t.Errorf("expected schema %q, got %q", expectedSchema, h.mock.LastReq.URI.Schema)
	}
}

// AssertURIHost verifies the URI host in authority
func (h *TestHelper) AssertURIHost(expectedHost string) {
	if h.mock.LastReq == nil {
		h.t.Fatal("no request was made to plainId")
	}
	actual, ok := h.mock.LastReq.URI.Authority["host"]
	if !ok {
		h.t.Error("expected host to be present in authority")
		return
	}
	if actual != expectedHost {
		h.t.Errorf("expected host %q, got %q", expectedHost, actual)
	}
}

// AssertRequestCount verifies the number of requests made to plainId
func (h *TestHelper) AssertRequestCount(expected int) {
	if len(h.mock.Requests) != expected {
		h.t.Errorf("expected %d requests, got %d", expected, len(h.mock.Requests))
	}
}
