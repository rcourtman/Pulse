package securityutil

import (
	"net/url"
	"strings"
	"testing"
)

// mustParseURL parses raw and fails the test if it cannot be parsed. It is a
// small convenience used only to construct *url.URL fixtures for the append/
// resolve helpers; it does not re-implement any package logic.
func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("mustParseURL(%q): %v", raw, err)
	}
	return u
}

func TestBranchCovNormalizeAbsoluteHTTPURL(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantErr  string // non-empty substring expected when an error is returned
		wantOK   bool   // when true, expect success and verify resulting URL
		wantStr  string // expected String() when wantOK
		wantHost string
	}{
		{name: "empty after trim", raw: "   ", wantErr: "URL is required"},
		{name: "whitespace trimmed to success", raw: "  https://example.com/a  ", wantOK: true, wantStr: "https://example.com/a", wantHost: "example.com"},
		{name: "parse error invalid escape", raw: "http://host/%zz", wantErr: "invalid URL"},
		{name: "missing protocol scheme parse error", raw: ":badurl", wantErr: "invalid URL"},
		{name: "unsupported scheme ftp", raw: "ftp://host", wantErr: "scheme must be http or https"},
		{name: "empty host", raw: "https://", wantErr: "URL host is required"},
		{name: "userinfo rejected", raw: "https://user:pw@host", wantErr: "URL userinfo is not allowed"},
		{name: "port only hostname empty", raw: "https://:443", wantErr: "URL hostname is required"},
		{name: "https success default port", raw: "https://example.com:443/path", wantOK: true, wantStr: "https://example.com:443/path"},
		{name: "http success", raw: "http://1.2.3.4/x", wantOK: true, wantStr: "http://1.2.3.4/x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeAbsoluteHTTPURL(tt.raw)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("NormalizeAbsoluteHTTPURL(%q) = %v, want error containing %q", tt.raw, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("NormalizeAbsoluteHTTPURL(%q) err = %q, want substring %q", tt.raw, err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeAbsoluteHTTPURL(%q) unexpected error: %v", tt.raw, err)
			}
			if tt.wantOK {
				if got == nil {
					t.Fatalf("NormalizeAbsoluteHTTPURL(%q) returned nil URL", tt.raw)
				}
				if got.String() != tt.wantStr {
					t.Fatalf("NormalizeAbsoluteHTTPURL(%q) String() = %q, want %q", tt.raw, got.String(), tt.wantStr)
				}
				if tt.wantHost != "" && got.Hostname() != tt.wantHost {
					t.Fatalf("NormalizeAbsoluteHTTPURL(%q) Hostname() = %q, want %q", tt.raw, got.Hostname(), tt.wantHost)
				}
			}
		})
	}
}

func TestBranchCovNormalizeHTTPBaseURL(t *testing.T) {
	tests := []struct {
		name               string
		raw                string
		defaultScheme      string
		wantErr            string
		wantPath           string
		wantScheme         string
		wantString         string
		wantDefaultApplied bool // verifies the defaultScheme prepend branch fired
	}{
		{name: "empty required", raw: "  ", defaultScheme: "https", wantErr: "base URL is required"},
		{name: "default scheme applied", raw: "example.com", defaultScheme: "https", wantScheme: "https", wantPath: "", wantString: "https://example.com", wantDefaultApplied: true},
		{name: "default scheme applied with path", raw: "example.com/pulse", defaultScheme: "http", wantScheme: "http", wantPath: "/pulse", wantString: "http://example.com/pulse", wantDefaultApplied: true},
		{name: "default scheme ignored when scheme present", raw: "https://example.com", defaultScheme: "ftp", wantScheme: "https", wantPath: "", wantString: "https://example.com"},
		{name: "no default scheme and no scheme delegates error", raw: "example.com", defaultScheme: "", wantErr: "scheme must be http or https"},
		{name: "delegates unsupported scheme", raw: "ftp://host", defaultScheme: "", wantErr: "scheme must be http or https"},
		{name: "query rejected", raw: "https://host/?q=1", defaultScheme: "", wantErr: "must not include query or fragment"},
		{name: "fragment rejected", raw: "https://host/#frag", defaultScheme: "", wantErr: "must not include query or fragment"},
		{name: "root path collapses to empty", raw: "https://host/", defaultScheme: "", wantScheme: "https", wantPath: "", wantString: "https://host"},
		{name: "dot path collapses to empty", raw: "https://host/.", defaultScheme: "", wantScheme: "https", wantPath: "", wantString: "https://host"},
		{name: "normal subpath kept", raw: "https://host/a/b", defaultScheme: "", wantScheme: "https", wantPath: "/a/b", wantString: "https://host/a/b"},
		{name: "host local backslash path rejected", raw: "https://host/\\evil", defaultScheme: "", wantErr: "base URL path must be host-local"},
		{name: "userinfo rejected via delegation", raw: "https://u@host", defaultScheme: "", wantErr: "URL userinfo is not allowed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeHTTPBaseURL(tt.raw, tt.defaultScheme)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("NormalizeHTTPBaseURL(%q,%q) = %v, want error containing %q", tt.raw, tt.defaultScheme, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("NormalizeHTTPBaseURL(%q,%q) err = %q, want substring %q", tt.raw, tt.defaultScheme, err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeHTTPBaseURL(%q,%q) unexpected error: %v", tt.raw, tt.defaultScheme, err)
			}
			if got.Scheme != tt.wantScheme {
				t.Fatalf("Scheme = %q, want %q", got.Scheme, tt.wantScheme)
			}
			if got.Path != tt.wantPath {
				t.Fatalf("Path = %q, want %q", got.Path, tt.wantPath)
			}
			if got.String() != tt.wantString {
				t.Fatalf("String() = %q, want %q", got.String(), tt.wantString)
			}
			// RawPath is always cleared by NormalizeHTTPBaseURL.
			if got.RawPath != "" {
				t.Fatalf("RawPath = %q, want empty", got.RawPath)
			}
		})
	}
}

func TestBranchCovNormalizeLocalRedirectPath(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr string
	}{
		{name: "empty required", raw: "  ", wantErr: "redirect path is required"},
		{name: "query only path empty", raw: "?foo=bar", wantErr: "redirect must be a local absolute path"},
		{name: "fragment only path empty", raw: "#frag", wantErr: "redirect must be a local absolute path"},
		{name: "del control char 0x7f", raw: "/x\x7f", wantErr: "redirect path contains control characters"},
		{name: "low control char 0x1f", raw: "/x\x1f", wantErr: "redirect path contains control characters"},
		{name: "parse error invalid escape", raw: "/%zz", wantErr: "invalid redirect path"},
		{name: "backslash in path rejected", raw: "/a\\b", wantErr: "redirect path must not contain backslashes"},
		{name: "success preserves query", raw: "/dashboard?next=/home", want: "/dashboard?next=/home"},
		{name: "success simple path", raw: "/settings/profile", want: "/settings/profile"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeLocalRedirectPath(tt.raw)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("NormalizeLocalRedirectPath(%q) = %q, want error containing %q", tt.raw, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("NormalizeLocalRedirectPath(%q) err = %q, want substring %q", tt.raw, err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeLocalRedirectPath(%q) unexpected error: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeLocalRedirectPath(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestBranchCovNormalizePulseHTTPBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
		wantStr string
	}{
		{name: "empty required", raw: "   ", wantErr: "Pulse URL is required"},
		{name: "parse error invalid escape", raw: "https://host/%zz", wantErr: "is invalid"},
		{name: "missing scheme non websocket message", raw: "example.com", wantErr: "must include scheme (https:// or loopback http://)"},
		{name: "host empty", raw: "https://", wantErr: "must include host"},
		{name: "hostname empty port only", raw: "https://:443", wantErr: "must include host"},
		{name: "user credentials rejected", raw: "https://u@host", wantErr: "must not include user credentials"},
		{name: "query rejected", raw: "https://host/?q=1", wantErr: "must not include query or fragment"},
		{name: "fragment rejected", raw: "https://host/#f", wantErr: "must not include query or fragment"},
		{name: "port zero invalid", raw: "https://host:0", wantErr: "invalid port"},
		{name: "port too high invalid", raw: "https://host:99999", wantErr: "invalid port"},
		{name: "wss scheme unsupported for http channel", raw: "wss://host", wantErr: "unsupported scheme"},
		{name: "ws scheme unsupported for http channel", raw: "ws://localhost", wantErr: "unsupported scheme"},
		{name: "ftp unsupported scheme", raw: "ftp://host", wantErr: "unsupported scheme"},
		{name: "http non loopback rejected", raw: "http://example.com", wantErr: "must use https unless host is loopback"},
		{name: "https success normalizes host and trims path", raw: "https://API.Example.COM/Pulse/", wantStr: "https://api.example.com/Pulse"},
		{name: "http loopback allowed", raw: "http://LocalHost:8080/", wantStr: "http://localhost:8080"},
		{name: "http ipv4 loopback allowed", raw: "http://127.0.0.1", wantStr: "http://127.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePulseHTTPBaseURL(tt.raw)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("NormalizePulseHTTPBaseURL(%q) = %v, want error containing %q", tt.raw, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("NormalizePulseHTTPBaseURL(%q) err = %q, want substring %q", tt.raw, err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizePulseHTTPBaseURL(%q) unexpected error: %v", tt.raw, err)
			}
			if got.String() != tt.wantStr {
				t.Fatalf("NormalizePulseHTTPBaseURL(%q) = %q, want %q", tt.raw, got.String(), tt.wantStr)
			}
		})
	}
}

func TestBranchCovNormalizeSecureHTTPBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
		wantStr string
	}{
		{name: "empty required", raw: "  ", wantErr: "base URL is required"},
		{name: "https normalizes host and trims trailing slash", raw: "https://API.Example.COM/Pulse/", wantStr: "https://api.example.com/Pulse"},
		{name: "http loopback allowed", raw: "http://127.0.0.1/path/", wantStr: "http://127.0.0.1/path"},
		{name: "http localhost allowed", raw: "http://LocalHost", wantStr: "http://localhost"},
		{name: "http non loopback rejected", raw: "http://example.com", wantErr: "must use https unless host is loopback"},
		{name: "delegates bad scheme", raw: "ftp://host", wantErr: "scheme must be http or https"},
		{name: "clears query and fragment on https", raw: "https://host/path", wantStr: "https://host/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeSecureHTTPBaseURL(tt.raw)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("NormalizeSecureHTTPBaseURL(%q) = %v, want error containing %q", tt.raw, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("NormalizeSecureHTTPBaseURL(%q) err = %q, want substring %q", tt.raw, err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeSecureHTTPBaseURL(%q) unexpected error: %v", tt.raw, err)
			}
			if got.String() != tt.wantStr {
				t.Fatalf("NormalizeSecureHTTPBaseURL(%q) = %q, want %q", tt.raw, got.String(), tt.wantStr)
			}
			// NormalizeSecureHTTPBaseURL must clear query and fragment.
			if got.RawQuery != "" || got.Fragment != "" {
				t.Fatalf("expected cleared query/fragment, got RawQuery=%q Fragment=%q", got.RawQuery, got.Fragment)
			}
		})
	}
}

func TestBranchCovNormalizePulseWebSocketBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
		wantStr string
	}{
		{name: "empty required", raw: "   ", wantErr: "Pulse URL is required"},
		{name: "missing scheme websocket message", raw: "example.com", wantErr: "must include scheme (https://, wss://, or loopback http:// / ws://)"},
		{name: "https upgraded to wss", raw: "https://LocalHost/", wantStr: "wss://localhost"},
		{name: "wss success normalizes host", raw: "wss://API.Example.COM/Pulse/", wantStr: "wss://api.example.com/Pulse"},
		{name: "http loopback downgraded to ws", raw: "http://127.0.0.1/", wantStr: "ws://127.0.0.1"},
		{name: "ws loopback kept", raw: "ws://LocalHost", wantStr: "ws://localhost"},
		{name: "http non loopback rejected websocket message", raw: "http://example.com", wantErr: "must use https/wss unless host is loopback"},
		{name: "ws non loopback rejected", raw: "ws://example.com", wantErr: "must use https/wss unless host is loopback"},
		{name: "unsupported scheme", raw: "ftp://host", wantErr: "unsupported scheme"},
		{name: "invalid port high", raw: "wss://host:99999", wantErr: "invalid port"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePulseWebSocketBaseURL(tt.raw)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("NormalizePulseWebSocketBaseURL(%q) = %v, want error containing %q", tt.raw, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("NormalizePulseWebSocketBaseURL(%q) err = %q, want substring %q", tt.raw, err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizePulseWebSocketBaseURL(%q) unexpected error: %v", tt.raw, err)
			}
			if got.String() != tt.wantStr {
				t.Fatalf("NormalizePulseWebSocketBaseURL(%q) = %q, want %q", tt.raw, got.String(), tt.wantStr)
			}
		})
	}
}

func TestBranchCovAppendURLPath(t *testing.T) {
	t.Run("nil base returns nil", func(t *testing.T) {
		if got := AppendURLPath(nil, "x"); got != nil {
			t.Fatalf("AppendURLPath(nil) = %v, want nil", got)
		}
	})

	t.Run("empty and slash only segments skipped", func(t *testing.T) {
		base := mustParseURL(t, "https://host/api")
		got := AppendURLPath(base, "", "/", "//", "///")
		// All segments trim (via strings.Trim of "/") to empty and are skipped; path unchanged.
		if got.Path != "/api" {
			t.Fatalf("Path = %q, want %q", got.Path, "/api")
		}
	})

	t.Run("appends multiple segments", func(t *testing.T) {
		base := mustParseURL(t, "https://host/api")
		got := AppendURLPath(base, "v1", "/users/", "/42")
		if got.String() != "https://host/api/v1/users/42" {
			t.Fatalf("String() = %q", got.String())
		}
	})

	t.Run("segments with parent refs collapse", func(t *testing.T) {
		base := mustParseURL(t, "https://host/a/b")
		got := AppendURLPath(base, "../c")
		// path.Join resolves .. -> /a/c
		if got.Path != "/a/c" {
			t.Fatalf("Path = %q, want %q", got.Path, "/a/c")
		}
	})

	t.Run("base with empty path gets leading slash", func(t *testing.T) {
		base := mustParseURL(t, "https://host")
		got := AppendURLPath(base, "x")
		// joined "x" has no leading slash -> normalized to "/x"
		if got.Path != "/x" {
			t.Fatalf("Path = %q, want %q", got.Path, "/x")
		}
		if got.String() != "https://host/x" {
			t.Fatalf("String() = %q", got.String())
		}
	})

	t.Run("no segments and root path collapses to empty", func(t *testing.T) {
		base := mustParseURL(t, "https://host/")
		got := AppendURLPath(base)
		if got.Path != "" {
			t.Fatalf("Path = %q, want empty", got.Path)
		}
	})

	t.Run("rawpath and fragment cleared but base query preserved", func(t *testing.T) {
		base := mustParseURL(t, "https://host/api?keep=1#/frag")
		got := AppendURLPath(base, "users")
		// AppendURLPath clears RawPath and Fragment but does NOT clear RawQuery.
		if got.RawPath != "" {
			t.Fatalf("RawPath = %q, want empty", got.RawPath)
		}
		if got.Fragment != "" {
			t.Fatalf("Fragment = %q, want empty", got.Fragment)
		}
		if got.RawQuery != "keep=1" {
			t.Fatalf("RawQuery = %q, want %q (base query is preserved)", got.RawQuery, "keep=1")
		}
	})

	t.Run("base is not mutated", func(t *testing.T) {
		base := mustParseURL(t, "https://host/api")
		_ = AppendURLPath(base, "x", "y")
		if base.Path != "/api" {
			t.Fatalf("base.Path mutated to %q", base.Path)
		}
		if base.RawPath != "" {
			t.Fatalf("base.RawPath mutated to %q", base.RawPath)
		}
	})
}

func TestBranchCovResolveRelativeURL(t *testing.T) {
	t.Run("nil base errors", func(t *testing.T) {
		_, err := ResolveRelativeURL(nil, "/x")
		if err == nil || !strings.Contains(err.Error(), "base URL is required") {
			t.Fatalf("err = %v, want base URL is required", err)
		}
	})

	t.Run("empty relative after trim errors", func(t *testing.T) {
		base := mustParseURL(t, "https://host/api")
		_, err := ResolveRelativeURL(base, "   ")
		if err == nil || !strings.Contains(err.Error(), "relative path is required") {
			t.Fatalf("err = %v, want relative path is required", err)
		}
	})

	t.Run("backslash rejected", func(t *testing.T) {
		base := mustParseURL(t, "https://host/api")
		_, err := ResolveRelativeURL(base, `/a\b`)
		if err == nil || !strings.Contains(err.Error(), "must not contain backslashes") {
			t.Fatalf("err = %v, want backslash rejection", err)
		}
	})

	t.Run("parse error invalid escape", func(t *testing.T) {
		base := mustParseURL(t, "https://host/api")
		_, err := ResolveRelativeURL(base, "/%zz")
		if err == nil || !strings.Contains(err.Error(), "invalid relative path") {
			t.Fatalf("err = %v, want invalid relative path", err)
		}
	})

	t.Run("absolute url rejected", func(t *testing.T) {
		base := mustParseURL(t, "https://host/api")
		_, err := ResolveRelativeURL(base, "https://evil/x")
		if err == nil || !strings.Contains(err.Error(), "must not include scheme or host") {
			t.Fatalf("err = %v, want scheme/host rejection", err)
		}
	})

	t.Run("scheme relative host rejected", func(t *testing.T) {
		base := mustParseURL(t, "https://host/api")
		_, err := ResolveRelativeURL(base, "//evil/x")
		if err == nil || !strings.Contains(err.Error(), "must not include scheme or host") {
			t.Fatalf("err = %v, want host rejection", err)
		}
	})

	t.Run("relative without leading slash rejected", func(t *testing.T) {
		base := mustParseURL(t, "https://host/api")
		_, err := ResolveRelativeURL(base, "users")
		if err == nil || !strings.Contains(err.Error(), "must start with '/'") {
			t.Fatalf("err = %v, want leading slash rejection", err)
		}
	})

	t.Run("success joins and clears fragment", func(t *testing.T) {
		base := mustParseURL(t, "https://host/api")
		got, err := ResolveRelativeURL(base, "/users")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.String() != "https://host/api/users" {
			t.Fatalf("String() = %q", got.String())
		}
		// No encoding in path -> RawPath collapsed to empty.
		if got.RawPath != "" {
			t.Fatalf("RawPath = %q, want empty", got.RawPath)
		}
		if got.Fragment != "" {
			t.Fatalf("Fragment = %q, want empty", got.Fragment)
		}
		if got.RawQuery != "" {
			t.Fatalf("RawQuery = %q, want empty", got.RawQuery)
		}
	})

	t.Run("success propagates relative query", func(t *testing.T) {
		base := mustParseURL(t, "https://host/api")
		got, err := ResolveRelativeURL(base, "/users?active=1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.RawQuery != "active=1" {
			t.Fatalf("RawQuery = %q, want %q", got.RawQuery, "active=1")
		}
	})

	t.Run("encoded path retains rawpath", func(t *testing.T) {
		base := mustParseURL(t, "https://host/api")
		got, err := ResolveRelativeURL(base, "/us%20ers")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Path != "/api/us ers" {
			t.Fatalf("Path = %q, want %q", got.Path, "/api/us ers")
		}
		if got.RawPath != "/api/us%20ers" {
			t.Fatalf("RawPath = %q, want %q (retained because it differs from Path)", got.RawPath, "/api/us%20ers")
		}
	})

	t.Run("base is not mutated", func(t *testing.T) {
		base := mustParseURL(t, "https://host/api")
		_, _ = ResolveRelativeURL(base, "/users")
		if base.Path != "/api" {
			t.Fatalf("base.Path mutated to %q", base.Path)
		}
	})
}
