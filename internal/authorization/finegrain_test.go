package authorization

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"reverseProxy/internal/jwtauth"
)

func TestCheckFineGrain_SkipWhenNoConfig(t *testing.T) {
	old := cfg
	cfg = nil
	t.Cleanup(func() { cfg = old })

	allow, reason, err := CheckFineGrainAccess(RequestInfo{Method: "GET", Path: "/x"}, jwtauth.Principal{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allow || reason == "" {
		t.Fatalf("expected allow with skip reason when no config")
	}
}

func TestCheckFineGrain_SkipWhenNoURL(t *testing.T) {
	old := cfg
	cfg = &Config{FineGrain: FineGrainConfig{Enabled: true, ValidationURL: ""}}
	t.Cleanup(func() { cfg = old })
	allow, reason, err := CheckFineGrainAccess(RequestInfo{}, jwtauth.Principal{})
	if err != nil || !allow || reason == "" {
		t.Fatalf("expected skip allow when URL empty, got allow=%v reason=%q err=%v", allow, reason, err)
	}
}

func TestCheckFineGrain_Allow(t *testing.T) {
	var seen finePayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		_ = json.NewEncoder(w).Encode(validationResponse{Allow: true, Reason: "ok"})
	}))
	defer srv.Close()

	old := cfg
	cfg = &Config{FineGrain: FineGrainConfig{Enabled: true, ValidationURL: srv.URL, ResourceMap: map[string]FineRule{
		"[/items:POST]": {Roles: []string{"ROLE_USER"}, RulesetName: "rs", RulesetID: "1", Body: map[string]string{"username": "$.username"}},
	}}}
	t.Cleanup(func() { cfg = old })

	req := RequestInfo{Method: "POST", Path: "/items"}
	p := jwtauth.Principal{UserID: "u1", Username: "alice", Email: "a@example.com"}
	allow, reason, err := CheckFineGrainAccess(req, p)
	if err != nil || !allow || reason != "ok" {
		t.Fatalf("unexpected result allow=%v reason=%q err=%v", allow, reason, err)
	}
	if seen.Request.Path != "/items" || seen.Request.Method != "POST" {
		t.Fatalf("unexpected payload request: %+v", seen.Request)
	}
}

func TestCheckFineGrain_Deny(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(validationResponse{Allow: false, Reason: "blocked"})
	}))
	defer srv.Close()

	old := cfg
	cfg = &Config{FineGrain: FineGrainConfig{Enabled: true, ValidationURL: srv.URL, ResourceMap: map[string]FineRule{"[/]": {}}}}
	t.Cleanup(func() { cfg = old })

	allow, reason, err := CheckFineGrainAccess(RequestInfo{}, jwtauth.Principal{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allow || reason != "blocked" {
		t.Fatalf("expected deny with reason blocked, got allow=%v reason=%q", allow, reason)
	}
}

func TestCheckFineGrain_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer srv.Close()

	old := cfg
	cfg = &Config{FineGrain: FineGrainConfig{Enabled: true, ValidationURL: srv.URL, ResourceMap: map[string]FineRule{"[/]": {}}}}
	t.Cleanup(func() { cfg = old })

	allow, reason, err := CheckFineGrainAccess(RequestInfo{}, jwtauth.Principal{})
	if err == nil || allow || reason == "" {
		t.Fatalf("expected error, allow=false, and non-empty reason for non-2xx")
	}
}

func TestCheckFineGrain_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	old := cfg
	cfg = &Config{FineGrain: FineGrainConfig{Enabled: true, ValidationURL: srv.URL, ResourceMap: map[string]FineRule{"[/]": {}}}}
	t.Cleanup(func() { cfg = old })

	allow, _, err := CheckFineGrainAccess(RequestInfo{}, jwtauth.Principal{})
	if err == nil || allow {
		t.Fatalf("expected decode error and allow=false")
	}
}
