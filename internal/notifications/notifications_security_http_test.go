package notifications

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendAppriseViaHTTPEmptyServerURL(t *testing.T) {
	nm := NewNotificationManager("")
	defer nm.Stop()

	err := nm.sendAppriseViaHTTP(AppriseConfig{TimeoutSeconds: 1}, "title", "body", "info")
	if err == nil || !strings.Contains(err.Error(), "server URL is not configured") {
		t.Fatalf("expected missing server URL error, got %v", err)
	}
}

func TestSendAppriseViaHTTPInvalidScheme(t *testing.T) {
	nm := NewNotificationManager("")
	defer nm.Stop()

	err := nm.sendAppriseViaHTTP(AppriseConfig{ServerURL: "ftp://example.com", TimeoutSeconds: 1}, "title", "body", "info")
	if err == nil || !strings.Contains(err.Error(), "must start with http or https") {
		t.Fatalf("expected scheme validation error, got %v", err)
	}
}

func TestSendAppriseViaHTTPSkipTLSVerifyAndDefaultHeader(t *testing.T) {
	nm := NewNotificationManager("")
	defer nm.Stop()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-API-KEY"); got != "secret" {
			t.Fatalf("expected default API key header, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	if err := nm.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("allowlist: %v", err)
	}

	err := nm.sendAppriseViaHTTP(AppriseConfig{
		ServerURL:      server.URL,
		SkipTLSVerify:  true,
		APIKey:         "secret",
		TimeoutSeconds: 2,
	}, "title", "body", "info")
	if err != nil {
		t.Fatalf("expected HTTPS request to succeed, got %v", err)
	}
}

func TestSendAppriseViaHTTPIncludesTargetsAndCustomHeader(t *testing.T) {
	nm := NewNotificationManager("")
	defer nm.Stop()

	type payload struct {
		Body  string   `json:"body"`
		Title string   `json:"title"`
		Type  string   `json:"type"`
		URLs  []string `json:"urls"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Test-Key"); got != "secret" {
			t.Fatalf("expected custom API key header, got %q", got)
		}
		if !strings.Contains(r.URL.Path, "/notify/") {
			t.Fatalf("expected notify path, got %q", r.URL.Path)
		}
		var p payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(p.URLs) != 1 || p.URLs[0] != "discord://token" {
			t.Fatalf("expected urls in payload, got %#v", p.URLs)
		}
		if p.Type != "warning" {
			t.Fatalf("expected notify type warning, got %q", p.Type)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	if err := nm.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("allowlist: %v", err)
	}

	err := nm.sendAppriseViaHTTP(AppriseConfig{
		ServerURL:      server.URL,
		ConfigKey:      "config key",
		APIKey:         "secret",
		APIKeyHeader:   "X-Test-Key",
		Targets:        []string{"discord://token"},
		TimeoutSeconds: 2,
	}, "title", "body", "warning")
	if err != nil {
		t.Fatalf("expected HTTP request to succeed, got %v", err)
	}
}

func TestSendAppriseViaHTTPStatusErrorWithBody(t *testing.T) {
	nm := NewNotificationManager("")
	defer nm.Stop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	}))
	defer server.Close()

	if err := nm.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("allowlist: %v", err)
	}

	err := nm.sendAppriseViaHTTP(AppriseConfig{
		ServerURL:      server.URL,
		TimeoutSeconds: 2,
	}, "title", "body", "info")
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("expected status error with body, got %v", err)
	}
}

func TestSendAppriseViaHTTPStatusErrorWithoutBody(t *testing.T) {
	nm := NewNotificationManager("")
	defer nm.Stop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if err := nm.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("allowlist: %v", err)
	}

	err := nm.sendAppriseViaHTTP(AppriseConfig{
		ServerURL:      server.URL,
		TimeoutSeconds: 2,
	}, "title", "body", "info")
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("expected status error, got %v", err)
	}
}
