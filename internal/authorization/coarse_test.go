package authorization

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"reverseProxy/internal/jwtauth"
)

func TestCheckCoarse_SkipWhenNoConfig(t *testing.T) {
	old := cfg
	cfg = nil
	t.Cleanup(func() { cfg = old })

	allow, reason, err := CheckCoarse(RequestInfo{Method: "GET", Path: "/x"}, jwtauth.Principal{UserID: "u1", Username: "alice", Email: "a@example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allow {
		t.Fatalf("expected allow when config missing")
	}
	if reason == "" {
		t.Fatalf("expected skip reason")
	}
}

// helper principal
func jwtauthPrincipalForTest() jwtauth.Principal {
	return jwtauth.Principal{UserID: "u1", Username: "alice", Email: "a@example.com"}
}

func TestCheckCoarse_AllowAndPayload(t *testing.T) {
	// setup server to validate payload and return allow=true
	var seen coarsePayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected application/json, got %s", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		_ = json.NewEncoder(w).Encode(validationResponse{Allow: true, Reason: "ok"})
	}))
	defer srv.Close()

	old := cfg
	cfg = &Config{Coarse: CoarseConfig{Enabled: true, ValidationURL: srv.URL, ResourceMap: map[string]string{
		"[/x]": "/target",
	}}}
	t.Cleanup(func() { cfg = old })

	req := RequestInfo{Method: "GET", Path: "/x"}
	p := jwtauthPrincipalForTest()
	allow, reason, err := CheckCoarse(req, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allow || reason != "ok" {
		t.Fatalf("unexpected result allow=%v reason=%q", allow, reason)
	}
	if seen.Request.Method != "GET" || seen.Request.Path != "/x" {
		t.Fatalf("payload request mismatch: %+v", seen.Request)
	}
	if seen.Principal.Username != "alice" || seen.Principal.Email != "a@example.com" {
		t.Fatalf("payload principal mismatch: %+v", seen.Principal)
	}
	if seen.Resource != "/target" {
		t.Fatalf("expected resource '/target', got %q", seen.Resource)
	}
}

func TestCheckCoarse_Deny(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(validationResponse{Allow: false, Reason: "nope"})
	}))
	defer srv.Close()

	old := cfg
	cfg = &Config{Coarse: CoarseConfig{Enabled: true, ValidationURL: srv.URL, ResourceMap: map[string]string{"[/]": "/res"}}}
	t.Cleanup(func() { cfg = old })

	allow, reason, err := CheckCoarse(RequestInfo{}, jwtauthPrincipalForTest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allow || reason != "nope" {
		t.Fatalf("expected deny with reason nope, got allow=%v reason=%q", allow, reason)
	}
}

func TestCheckCoarse_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	old := cfg
	cfg = &Config{Coarse: CoarseConfig{Enabled: true, ValidationURL: srv.URL, ResourceMap: map[string]string{"[/]": "/res"}}}
	t.Cleanup(func() { cfg = old })

	allow, reason, err := CheckCoarse(RequestInfo{}, jwtauthPrincipalForTest())
	if err == nil {
		t.Fatalf("expected error for non-2xx response")
	}
	if allow {
		t.Fatalf("expected allow=false on non-2xx")
	}
	if reason == "" {
		t.Fatalf("expected reason to be set for non-2xx")
	}
}

func TestCheckCoarse_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	old := cfg
	cfg = &Config{Coarse: CoarseConfig{Enabled: true, ValidationURL: srv.URL, ResourceMap: map[string]string{"[/]": "/res"}}}
	t.Cleanup(func() { cfg = old })

	allow, _, err := CheckCoarse(RequestInfo{}, jwtauthPrincipalForTest())
	if err == nil || allow {
		t.Fatalf("expected decode error and allow=false")
	}
}

// no extra aliasing needed when importing jwtauth in tests
