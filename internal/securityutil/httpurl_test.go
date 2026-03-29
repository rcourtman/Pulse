package securityutil

import (
	"context"
	"strings"
	"testing"
)

func TestNormalizeHTTPBaseURLAddsDefaultScheme(t *testing.T) {
	parsed, err := NormalizeHTTPBaseURL("pbs.example.com:8007", "https")
	if err != nil {
		t.Fatalf("NormalizeHTTPBaseURL() error = %v", err)
	}
	if got := parsed.String(); got != "https://pbs.example.com:8007" {
		t.Fatalf("NormalizeHTTPBaseURL() = %q", got)
	}
}

func TestNormalizeHTTPBaseURLRejectsQuery(t *testing.T) {
	if _, err := NormalizeHTTPBaseURL("https://example.com/path?x=1", ""); err == nil {
		t.Fatal("expected query-bearing base URL to be rejected")
	}
}

func TestResolveRelativeURLRejectsAbsoluteURL(t *testing.T) {
	base, err := NormalizeHTTPBaseURL("https://example.com/api", "")
	if err != nil {
		t.Fatalf("NormalizeHTTPBaseURL() error = %v", err)
	}

	if _, err := ResolveRelativeURL(base, "https://evil.example.com"); err == nil {
		t.Fatal("expected absolute URL to be rejected")
	}
}

func TestAppendURLPathPreservesBasePath(t *testing.T) {
	base, err := NormalizeHTTPBaseURL("https://issuer.example.com/realms/pulse", "")
	if err != nil {
		t.Fatalf("NormalizeHTTPBaseURL() error = %v", err)
	}

	appended := AppendURLPath(base, ".well-known", "openid-configuration")
	if appended == nil {
		t.Fatal("AppendURLPath() returned nil")
	}
	if got := appended.String(); got != "https://issuer.example.com/realms/pulse/.well-known/openid-configuration" {
		t.Fatalf("AppendURLPath() = %q", got)
	}
}

func TestNewRelativeRequestWithContextUsesValidatedURL(t *testing.T) {
	base, err := NormalizeHTTPBaseURL("https://example.com/api2/json", "")
	if err != nil {
		t.Fatalf("NormalizeHTTPBaseURL() error = %v", err)
	}

	req, err := NewRelativeRequestWithContext(context.Background(), "GET", base, "/nodes", nil)
	if err != nil {
		t.Fatalf("NewRelativeRequestWithContext() error = %v", err)
	}
	if got := req.URL.String(); got != "https://example.com/api2/json/nodes" {
		t.Fatalf("request URL = %q", got)
	}
	if strings.Contains(req.URL.String(), "pulse.invalid") {
		t.Fatalf("placeholder URL leaked into request: %q", req.URL.String())
	}
}

func TestNewRelativeRequestWithContextPreservesEscapedPath(t *testing.T) {
	base, err := NormalizeHTTPBaseURL("https://example.com/api2/json", "")
	if err != nil {
		t.Fatalf("NormalizeHTTPBaseURL() error = %v", err)
	}

	req, err := NewRelativeRequestWithContext(context.Background(), "GET", base, "/nodes/node%2F1/backup", nil)
	if err != nil {
		t.Fatalf("NewRelativeRequestWithContext() error = %v", err)
	}
	if got := req.URL.Path; got != "/api2/json/nodes/node/1/backup" {
		t.Fatalf("request path = %q", got)
	}
	if got := req.URL.EscapedPath(); got != "/api2/json/nodes/node%2F1/backup" {
		t.Fatalf("request escaped path = %q", got)
	}
}
