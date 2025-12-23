package authorization

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reverseProxy/internal/ingress/jwtauth"
	"testing"
)

func TestExtractValueFromPath_SimpleField(t *testing.T) {
	data := map[string]interface{}{
		"username": "alice",
		"email":    "alice@example.com",
	}

	tests := []struct {
		name     string
		path     string
		expected interface{}
		wantErr  bool
	}{
		{
			name:     "simple field with $.",
			path:     "$.username",
			expected: "alice",
			wantErr:  false,
		},
		{
			name:     "simple field with $",
			path:     "$username",
			expected: "alice",
			wantErr:  false,
		},
		{
			name:     "field without $ prefix",
			path:     "username",
			expected: "alice",
			wantErr:  false,
		},
		{
			name:    "missing field",
			path:    "$.notfound",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractValueFromPath(data, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractValueFromPath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("extractValueFromPath() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExtractValueFromPath_NestedField(t *testing.T) {
	data := map[string]interface{}{
		"user": map[string]interface{}{
			"profile": map[string]interface{}{
				"name": "Bob",
			},
		},
	}

	value, err := extractValueFromPath(data, "$.user.profile.name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "Bob" {
		t.Errorf("expected 'Bob', got %v", value)
	}
}

func TestExtractValueFromPath_ArrayWildcard(t *testing.T) {
	data := map[string]interface{}{
		"fromAccount": []interface{}{
			map[string]interface{}{
				"accountId":    "1234567890",
				"accountValue": float64(10),
			},
			map[string]interface{}{
				"accountId":    "1234567891",
				"accountValue": float64(80),
			},
			map[string]interface{}{
				"accountId":    "1234567892",
				"accountValue": float64(10),
			},
		},
	}

	tests := []struct {
		name     string
		path     string
		expected []interface{}
		wantErr  bool
	}{
		{
			name: "array wildcard for accountIds",
			path: "$.fromAccount[*].accountId",
			expected: []interface{}{
				"1234567890",
				"1234567891",
				"1234567892",
			},
			wantErr: false,
		},
		{
			name: "array wildcard for accountValues",
			path: "$.fromAccount[*].accountValue",
			expected: []interface{}{
				float64(10),
				float64(80),
				float64(10),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractValueFromPath(data, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractValueFromPath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				gotArr, ok := got.([]interface{})
				if !ok {
					t.Errorf("expected []interface{}, got %T", got)
				}
				if len(gotArr) != len(tt.expected) {
					t.Errorf("expected length %d, got %d", len(tt.expected), len(gotArr))
				}
				for i, v := range gotArr {
					if v != tt.expected[i] {
						t.Errorf("array element %d: expected %v, got %v", i, tt.expected[i], v)
					}
				}
			}
		})
	}
}

func TestBuildPlainIdRequest(t *testing.T) {
	bodyData := map[string]interface{}{
		"transactionName":   "Test",
		"transactionAmount": float64(100),
		"tranTemplateID":    "TestTemplate",
		"fromAccount": []interface{}{
			map[string]interface{}{
				"accountId":    "1234567890",
				"accountValue": float64(10),
			},
			map[string]interface{}{
				"accountId":    "1234567891",
				"accountValue": float64(80),
			},
		},
		"toAccount": []interface{}{
			map[string]interface{}{
				"accountId":    "1234567893",
				"accountValue": float64(10),
			},
		},
	}

	req := RequestInfo{
		Method:  "POST",
		Path:    "/mm/web/v1/transaction",
		FullURL: "https://localhost:8080/mm/web/v1/transaction?details=true",
		Headers: map[string]string{
			"X-Request-Id":    "8CDAC3e6r4D252ABE60EFD7A31AFEEBA",
			"Authorization":   "Bearer eyJhbG...lXvZQ",
			"X-Custom-Header": "custom-value",
		},
	}

	rule := FineRule{
		Roles:       []string{"ROLE_USER"},
		RulesetName: "mm-transaction",
		RulesetID:   "10201",
		Body: map[string]string{
			"transactionName":   "$.transactionName",
			"transactionAmount": "$.transactionAmount",
			"fromAccountIds":    "$.fromAccount[*].accountId",
			"toAccountIds":      "$.toAccount[*].accountId",
		},
	}

	principal := jwtauth.Principal{
		UserID:   "user123",
		Username: "alice",
		Email:    "alice@example.com",
	}

	plainIdReq, err := buildPlainIdRequest(req, principal, rule, bodyData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify basic fields
	if plainIdReq.Method != "POST" {
		t.Errorf("expected method POST, got %s", plainIdReq.Method)
	}

	if len(plainIdReq.URI.Path) < 2 {
		t.Errorf("expected at least 2 path segments, got %d", len(plainIdReq.URI.Path))
	}

	// Verify headers
	if plainIdReq.Headers["x-request-id"] != "8CDAC3e6r4D252ABE60EFD7A31AFEEBA" {
		t.Errorf("expected x-request-id to match, got %s", plainIdReq.Headers["x-request-id"])
	}

	if plainIdReq.Headers["Authorization"] != "Bearer eyJhbG...lXvZQ" {
		t.Errorf("expected Authorization header to be set")
	}

	// Verify body fields were extracted correctly
	if plainIdReq.Body["transactionName"] != "Test" {
		t.Errorf("expected transactionName 'Test', got %v", plainIdReq.Body["transactionName"])
	}

	if plainIdReq.Body["transactionAmount"] != float64(100) {
		t.Errorf("expected transactionAmount 100, got %v", plainIdReq.Body["transactionAmount"])
	}

	fromIds, ok := plainIdReq.Body["fromAccountIds"].([]interface{})
	if !ok {
		t.Errorf("expected fromAccountIds to be array, got %T", plainIdReq.Body["fromAccountIds"])
	} else if len(fromIds) != 2 {
		t.Errorf("expected 2 fromAccountIds, got %d", len(fromIds))
	}
}

func TestCheckPlainIdAccess_Allow(t *testing.T) {
	var seenReq PlainIdRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&seenReq); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		resp := PlainIdResponse{Allow: true, Reason: "permitted"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	old := cfg
	cfg = &Config{
		FineGrain: FineGrainConfig{
			Enabled:       true,
			ValidationURL: srv.URL,
			ResourceMap: map[string]FineRule{
				"[/api/test:POST]": {
					Roles:       []string{"ROLE_USER"},
					RulesetName: "test-rule",
					RulesetID:   "123",
					Body: map[string]string{
						"name": "$.name",
					},
				},
			},
		},
	}
	t.Cleanup(func() { cfg = old })

	req := RequestInfo{
		Method:  "POST",
		Path:    "/api/test",
		FullURL: "https://localhost:8080/api/test",
		Headers: map[string]string{
			"X-Request-Id":  "test-request-id",
			"Authorization": "Bearer test-token",
		},
	}

	bodyData := map[string]interface{}{
		"name": "test-user",
	}

	p := jwtauth.Principal{UserID: "u1", Username: "alice"}
	allow, reason, err := CheckPlainIdAccess(req, p, bodyData)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allow {
		t.Errorf("expected allow=true, got false")
	}
	if reason != "permitted" {
		t.Errorf("expected reason 'permitted', got %q", reason)
	}

	// Verify the request sent to plainId service
	if seenReq.Method != "POST" {
		t.Errorf("expected method POST in plainId request, got %s", seenReq.Method)
	}
	if seenReq.Body["name"] != "test-user" {
		t.Errorf("expected name in body, got %v", seenReq.Body)
	}
}

func TestCheckPlainIdAccess_Deny(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := PlainIdResponse{Allow: false, Reason: "access denied"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	old := cfg
	cfg = &Config{
		FineGrain: FineGrainConfig{
			Enabled:       true,
			ValidationURL: srv.URL,
			ResourceMap: map[string]FineRule{
				"[/api/test:POST]": {
					Body: map[string]string{
						"name": "$.name",
					},
				},
			},
		},
	}
	t.Cleanup(func() { cfg = old })

	req := RequestInfo{
		Method:  "POST",
		Path:    "/api/test",
		FullURL: "https://localhost:8080/api/test",
		Headers: map[string]string{},
	}

	p := jwtauth.Principal{UserID: "u1"}
	allow, reason, err := CheckPlainIdAccess(req, p, map[string]interface{}{"name": "test"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allow {
		t.Errorf("expected allow=false, got true")
	}
	if reason != "access denied" {
		t.Errorf("expected reason 'access denied', got %q", reason)
	}
}

func TestCheckPlainIdAccess_SkipWhenDisabled(t *testing.T) {
	old := cfg
	cfg = &Config{
		FineGrain: FineGrainConfig{
			Enabled:       false,
			ValidationURL: "http://localhost:8080",
		},
	}
	t.Cleanup(func() { cfg = old })

	req := RequestInfo{Method: "GET", Path: "/x", FullURL: "http://localhost/x"}
	allow, reason, err := CheckPlainIdAccess(req, jwtauth.Principal{}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allow {
		t.Errorf("expected allow=true when disabled, got false")
	}
	if reason == "" {
		t.Errorf("expected non-empty reason")
	}
}

func TestCheckPlainIdAccess_SkipWhenNoMatchingRule(t *testing.T) {
	old := cfg
	cfg = &Config{
		FineGrain: FineGrainConfig{
			Enabled:       true,
			ValidationURL: "http://localhost:8080",
			ResourceMap: map[string]FineRule{
				"[/api/other:POST]": {Body: map[string]string{"id": "$.id"}},
			},
		},
	}
	t.Cleanup(func() { cfg = old })

	req := RequestInfo{
		Method:  "GET",
		Path:    "/api/test",
		FullURL: "http://localhost/api/test",
		Headers: map[string]string{},
	}

	allow, reason, err := CheckPlainIdAccess(req, jwtauth.Principal{}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allow {
		t.Errorf("expected allow=true when no rule matches, got false")
	}
	if reason == "" {
		t.Errorf("expected non-empty reason")
	}
}

func TestCheckPlainIdAccess_PlainIdPermit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := PlainIdResponse{Permit: "PERMIT_EXPLICIT"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	old := cfg
	cfg = &Config{
		FineGrain: FineGrainConfig{
			Enabled:       true,
			ValidationURL: srv.URL,
			ResourceMap: map[string]FineRule{
				"[/api/test:POST]": {
					Body: map[string]string{"id": "$.id"},
				},
			},
		},
	}
	t.Cleanup(func() { cfg = old })

	req := RequestInfo{
		Method:  "POST",
		Path:    "/api/test",
		FullURL: "https://localhost/api/test",
		Headers: map[string]string{},
	}

	allow, reason, err := CheckPlainIdAccess(req, jwtauth.Principal{}, map[string]interface{}{"id": "123"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allow {
		t.Errorf("expected allow=true for PERMIT, got false")
	}
	if reason != "PERMIT_EXPLICIT" {
		t.Errorf("expected reason 'PERMIT_EXPLICIT', got %q", reason)
	}
}

func TestCheckPlainIdAccess_PlainIdDeny(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := PlainIdResponse{Deny: "DENY_POLICY_VIOLATION"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	old := cfg
	cfg = &Config{
		FineGrain: FineGrainConfig{
			Enabled:       true,
			ValidationURL: srv.URL,
			ResourceMap: map[string]FineRule{
				"[/api/test:POST]": {
					Body: map[string]string{"id": "$.id"},
				},
			},
		},
	}
	t.Cleanup(func() { cfg = old })

	req := RequestInfo{
		Method:  "POST",
		Path:    "/api/test",
		FullURL: "https://localhost/api/test",
		Headers: map[string]string{},
	}

	allow, reason, err := CheckPlainIdAccess(req, jwtauth.Principal{}, map[string]interface{}{"id": "123"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allow {
		t.Errorf("expected allow=false for DENY, got true")
	}
	if reason != "DENY_POLICY_VIOLATION" {
		t.Errorf("expected reason 'DENY_POLICY_VIOLATION', got %q", reason)
	}
}

func TestExtractValueFromPath_ExistenceCheck_FieldPresent(t *testing.T) {
	// When a field with "Used" or "Exists" suffix is present, it should return the actual value
	data := map[string]interface{}{
		"tranTemplateID": "TestTemplate",
	}

	value, err := extractValueFromPath(data, "$.tranTemplateID")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "TestTemplate" {
		t.Errorf("expected 'TestTemplate', got %v", value)
	}
}

func TestExtractValueFromPath_ExistenceCheck_FieldAbsent(t *testing.T) {
	// When a field with "Used" suffix is absent, it should return false
	data := map[string]interface{}{
		"transactionAmount": float64(100),
	}

	value, err := extractValueFromPath(data, "$.tranTemplateUsed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != false {
		t.Errorf("expected false for missing 'Used' field, got %v", value)
	}
}

func TestExtractValueFromPath_ComplexNestedArray(t *testing.T) {
	// Test extraction from deeply nested structures
	data := map[string]interface{}{
		"transaction": map[string]interface{}{
			"details": map[string]interface{}{
				"accounts": []interface{}{
					map[string]interface{}{
						"id":    "acc1",
						"type":  "savings",
						"value": float64(1000),
					},
					map[string]interface{}{
						"id":    "acc2",
						"type":  "checking",
						"value": float64(5000),
					},
				},
			},
		},
	}

	// Should handle nested path to array
	value, err := extractValueFromPath(data, "$.transaction.details.accounts[*].id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids, ok := value.([]interface{})
	if !ok {
		t.Errorf("expected []interface{}, got %T", value)
		return
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 ids, got %d", len(ids))
		return
	}
	if ids[0] != "acc1" || ids[1] != "acc2" {
		t.Errorf("unexpected account ids: %v", ids)
	}
}

func TestBuildPlainIdRequest_WithQueryParams(t *testing.T) {
	bodyData := map[string]interface{}{
		"name": "test",
	}

	req := RequestInfo{
		Method:  "GET",
		Path:    "/api/search",
		FullURL: "https://example.com:8443/api/search?q=test&limit=10&sort=asc",
		Headers: map[string]string{
			"X-Request-Id":  "req-123",
			"Authorization": "Bearer token",
		},
	}

	rule := FineRule{
		Body: map[string]string{
			"name": "$.name",
		},
	}

	principal := jwtauth.Principal{UserID: "u1"}

	plainIdReq, err := buildPlainIdRequest(req, principal, rule, bodyData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify query parameters were extracted
	if plainIdReq.URI.Query == nil {
		t.Errorf("expected query parameters to be extracted")
	}

	if q, ok := plainIdReq.URI.Query["q"]; !ok || q != "test" {
		t.Errorf("expected query parameter 'q=test'")
	}

	if limit, ok := plainIdReq.URI.Query["limit"]; !ok || limit != "10" {
		t.Errorf("expected query parameter 'limit=10'")
	}
}

func TestBuildPlainIdRequest_URIComponents(t *testing.T) {
	bodyData := map[string]interface{}{}

	req := RequestInfo{
		Method:  "POST",
		Path:    "/api/v1/resource/123/action",
		FullURL: "https://api.example.com:443/api/v1/resource/123/action",
		Headers: map[string]string{},
	}

	rule := FineRule{Body: map[string]string{}}
	principal := jwtauth.Principal{UserID: "u1"}

	plainIdReq, err := buildPlainIdRequest(req, principal, rule, bodyData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify schema
	if plainIdReq.URI.Schema != "https" {
		t.Errorf("expected schema 'https', got %s", plainIdReq.URI.Schema)
	}

	// Verify authority
	if plainIdReq.URI.Authority["host"] != "api.example.com" {
		t.Errorf("expected host 'api.example.com', got %s", plainIdReq.URI.Authority["host"])
	}

	if plainIdReq.URI.Authority["port"] != "443" {
		t.Errorf("expected port '443', got %s", plainIdReq.URI.Authority["port"])
	}

	// Verify path segments
	expectedPath := "/api/v1/resource/123/action"
	if plainIdReq.URI.Path[0] != expectedPath {
		t.Errorf("expected first path element '%s', got %s", expectedPath, plainIdReq.URI.Path[0])
	}

	// Should have at least the full path plus segments
	if len(plainIdReq.URI.Path) < 6 {
		t.Errorf("expected at least 6 path segments, got %d", len(plainIdReq.URI.Path))
	}
}

// Test using the TestHelper
func TestCheckPlainIdAccess_UsingTestHelper(t *testing.T) {
	helper := NewTestHelper(t)
	defer helper.Close()

	resourceMap := map[string]FineRule{
		"[/api/transfers:POST]": {
			Roles:       []string{"ROLE_USER"},
			RulesetName: "transfer-rule",
			RulesetID:   "1001",
			Body: map[string]string{
				"amount":      "$.amount",
				"recipientId": "$.recipientId",
				"currency":    "$.currency",
			},
		},
	}

	helper.SetupConfig(resourceMap)

	bodyData := map[string]interface{}{
		"amount":      float64(1000),
		"recipientId": "rec123",
		"currency":    "USD",
	}

	allowed, reason, err := helper.CheckAccess(
		"POST",
		"/api/transfers",
		"https://api.example.com/api/transfers",
		map[string]string{"X-Request-Id": "req-123"},
		bodyData,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Errorf("expected access to be allowed, reason: %s", reason)
	}

	// Verify the request sent to plainId
	helper.AssertBodyField("amount", float64(1000))
	helper.AssertBodyField("recipientId", "rec123")
	helper.AssertBodyField("currency", "USD")
	helper.AssertHeaderPresent("x-request-id")
}

func TestCheckPlainIdAccess_TestHelperWithDeny(t *testing.T) {
	helper := NewTestHelper(t)
	defer helper.Close()

	helper.SetupConfig(map[string]FineRule{
		"[/api/delete:DELETE]": {
			Body: map[string]string{"id": "$.id"},
		},
	})

	helper.SetDenyResponse("Insufficient permissions to delete")

	allowed, reason, err := helper.CheckAccess(
		"DELETE",
		"/api/delete",
		"https://api.example.com/api/delete",
		map[string]string{},
		map[string]interface{}{"id": "123"},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Errorf("expected access to be denied")
	}
	if reason != "Insufficient permissions to delete" {
		t.Errorf("expected reason 'Insufficient permissions to delete', got %q", reason)
	}
}

func TestCheckPlainIdAccess_TestHelperArrayExtraction(t *testing.T) {
	helper := NewTestHelper(t)
	defer helper.Close()

	helper.SetupConfig(map[string]FineRule{
		"[/api/batch-transfer:POST]": {
			Body: map[string]string{
				"recipients":  "$.recipients[*].id",
				"amounts":     "$.recipients[*].amount",
				"description": "$.description",
			},
		},
	})

	bodyData := map[string]interface{}{
		"description": "Batch transfer",
		"recipients": []interface{}{
			map[string]interface{}{
				"id":     "rec1",
				"amount": float64(100),
			},
			map[string]interface{}{
				"id":     "rec2",
				"amount": float64(200),
			},
		},
	}

	allowed, reason, err := helper.CheckAccess(
		"POST",
		"/api/batch-transfer",
		"https://api.example.com/api/batch-transfer",
		map[string]string{},
		bodyData,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Errorf("expected access allowed, reason: %s", reason)
	}

	// Verify array extraction
	lastReq := helper.GetLastRequest()
	if lastReq == nil {
		t.Fatal("expected request to be sent")
	}

	recipientsField, ok := lastReq.Body["recipients"].([]interface{})
	if !ok {
		t.Errorf("expected recipients to be array, got %T", lastReq.Body["recipients"])
	} else if len(recipientsField) != 2 {
		t.Errorf("expected 2 recipients, got %d", len(recipientsField))
	}
}

func TestCheckPlainIdAccess_TestHelperMultipleRequests(t *testing.T) {
	helper := NewTestHelper(t)
	defer helper.Close()

	helper.SetupConfig(map[string]FineRule{
		"[/api/test:GET]": {Body: map[string]string{"id": "$.id"}},
	})

	// Make multiple requests
	for i := 0; i < 3; i++ {
		_, _, err := helper.CheckAccess(
			"GET",
			"/api/test",
			"https://api.example.com/api/test",
			map[string]string{},
			map[string]interface{}{"id": "test-id"},
		)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
	}

	// Verify all requests were recorded
	helper.AssertRequestCount(3)
	requests := helper.GetAllRequests()
	for i, req := range requests {
		if req.Body["id"] != "test-id" {
			t.Errorf("request %d: expected id 'test-id', got %v", i, req.Body["id"])
		}
	}
}
