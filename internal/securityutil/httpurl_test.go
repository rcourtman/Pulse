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

func TestIsLoopbackHost(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		cases := []string{"localhost", "LOCALHOST", "agent.localhost", "127.0.0.1", "::1"}
		for _, tc := range cases {
			if !IsLoopbackHost(tc) {
				t.Fatalf("expected loopback host for %q", tc)
			}
		}
	})

	t.Run("false", func(t *testing.T) {
		cases := []string{"", "example.com", "192.168.1.10", "10.0.0.5"}
		for _, tc := range cases {
			if IsLoopbackHost(tc) {
				t.Fatalf("expected non-loopback host for %q", tc)
			}
		}
	})
}

func TestNormalizePulseHTTPBaseURL(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		want      string
		wantError string
	}{
		{
			name: "normalizes secure url",
			raw:  "HTTPS://Pulse.Example.com:7655/base/",
			want: "https://pulse.example.com:7655/base",
		},
		{
			name: "allows loopback http",
			raw:  "http://LOCALHOST:7655/",
			want: "http://localhost:7655",
		},
		{
			name:      "rejects private-network http",
			raw:       "http://10.0.0.5:7655",
			wantError: "must use https unless host is loopback",
		},
		{
			name:      "rejects unsupported scheme",
			raw:       "ftp://pulse.example.com",
			wantError: "unsupported scheme",
		},
		{
			name:      "rejects query",
			raw:       "https://pulse.example.com?x=1",
			wantError: "must not include query or fragment",
		},
		{
			name:      "rejects bad port",
			raw:       "https://pulse.example.com:70000",
			wantError: "invalid port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePulseHTTPBaseURL(tt.raw)
			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("NormalizePulseHTTPBaseURL(%q) expected error", tt.raw)
				}
				if !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("NormalizePulseHTTPBaseURL(%q) error = %q, want substring %q", tt.raw, err.Error(), tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizePulseHTTPBaseURL(%q) error = %v", tt.raw, err)
			}
			if got.String() != tt.want {
				t.Fatalf("NormalizePulseHTTPBaseURL(%q) = %q, want %q", tt.raw, got.String(), tt.want)
			}
		})
	}
}

func TestNormalizePulseHTTPBaseURLWithOptions(t *testing.T) {
	got, err := NormalizePulseHTTPBaseURLWithOptions("http://10.0.0.5:7655/pulse/", PulseURLValidationOptions{
		AllowInsecureHTTP: true,
	})
	if err != nil {
		t.Fatalf("NormalizePulseHTTPBaseURLWithOptions() error = %v", err)
	}
	if got.String() != "http://10.0.0.5:7655/pulse" {
		t.Fatalf("NormalizePulseHTTPBaseURLWithOptions() = %q", got.String())
	}
}

func TestNormalizeSecureHTTPBaseURL(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		want      string
		wantError string
	}{
		{
			name: "normalizes secure url",
			raw:  "HTTPS://Billing.Example.com/api/",
			want: "https://billing.example.com/api",
		},
		{
			name: "allows loopback http",
			raw:  "http://LOCALHOST:8080/portal/",
			want: "http://localhost:8080/portal",
		},
		{
			name:      "rejects private-network http",
			raw:       "http://10.0.0.5:8080/portal",
			wantError: "must use https unless host is loopback",
		},
		{
			name:      "rejects unsupported scheme",
			raw:       "ftp://billing.example.com/portal",
			wantError: "scheme must be http or https",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeSecureHTTPBaseURL(tt.raw)
			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("NormalizeSecureHTTPBaseURL(%q) expected error", tt.raw)
				}
				if !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("NormalizeSecureHTTPBaseURL(%q) error = %q, want substring %q", tt.raw, err.Error(), tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeSecureHTTPBaseURL(%q) error = %v", tt.raw, err)
			}
			if got.String() != tt.want {
				t.Fatalf("NormalizeSecureHTTPBaseURL(%q) = %q, want %q", tt.raw, got.String(), tt.want)
			}
		})
	}
}

func TestNormalizePulseWebSocketBaseURL(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		want      string
		wantError string
	}{
		{
			name: "https becomes wss",
			raw:  "https://example.invalid/pulse/",
			want: "wss://example.invalid/pulse",
		},
		{
			name: "loopback http becomes ws",
			raw:  "http://localhost:7655",
			want: "ws://localhost:7655",
		},
		{
			name: "wss preserved",
			raw:  "wss://example.invalid",
			want: "wss://example.invalid",
		},
		{
			name:      "non-loopback http rejected",
			raw:       "http://example.invalid",
			wantError: "must use https/wss unless host is loopback",
		},
		{
			name:      "private-network ws rejected",
			raw:       "ws://10.0.0.5:7655",
			wantError: "must use https/wss unless host is loopback",
		},
		{
			name:      "unsupported scheme rejected",
			raw:       "ftp://example.invalid",
			wantError: "unsupported scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePulseWebSocketBaseURL(tt.raw)
			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("NormalizePulseWebSocketBaseURL(%q) expected error", tt.raw)
				}
				if !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("NormalizePulseWebSocketBaseURL(%q) error = %q, want substring %q", tt.raw, err.Error(), tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizePulseWebSocketBaseURL(%q) error = %v", tt.raw, err)
			}
			if got.String() != tt.want {
				t.Fatalf("NormalizePulseWebSocketBaseURL(%q) = %q, want %q", tt.raw, got.String(), tt.want)
			}
		})
	}
}

func TestNormalizePulseWebSocketBaseURLWithOptions(t *testing.T) {
	got, err := NormalizePulseWebSocketBaseURLWithOptions("http://10.0.0.5:7655/pulse", PulseURLValidationOptions{
		AllowInsecureHTTP: true,
	})
	if err != nil {
		t.Fatalf("NormalizePulseWebSocketBaseURLWithOptions() error = %v", err)
	}
	if got.String() != "ws://10.0.0.5:7655/pulse" {
		t.Fatalf("NormalizePulseWebSocketBaseURLWithOptions() = %q", got.String())
	}
}

func TestSameHostWebSocketOrigin(t *testing.T) {
	tests := []struct {
		name        string
		origin      string
		requestHost string
		want        bool
	}{
		{name: "same host", origin: "https://tenant.example.com", requestHost: "tenant.example.com", want: true},
		{name: "default port normalized", origin: "https://tenant.example.com:443", requestHost: "tenant.example.com", want: true},
		{name: "different host", origin: "https://evil.example.com", requestHost: "tenant.example.com", want: false},
		{name: "bad scheme", origin: "ws://tenant.example.com", requestHost: "tenant.example.com", want: false},
		{name: "invalid origin", origin: "://bad", requestHost: "tenant.example.com", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SameHostWebSocketOrigin(tt.origin, tt.requestHost); got != tt.want {
				t.Fatalf("SameHostWebSocketOrigin(%q, %q) = %v, want %v", tt.origin, tt.requestHost, got, tt.want)
			}
		})
	}
}

func TestHTTPOriginForWebSocketBaseURL(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		want      string
		wantError string
	}{
		{name: "wss becomes https origin", raw: "wss://example.invalid/pulse", want: "https://example.invalid"},
		{name: "ws becomes http origin", raw: "ws://localhost:7655/pulse", want: "http://localhost:7655"},
		{name: "https input becomes https origin", raw: "https://example.invalid/base", want: "https://example.invalid"},
		{name: "rejects invalid input", raw: "ftp://example.invalid", wantError: "unsupported scheme"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HTTPOriginForWebSocketBaseURL(tt.raw)
			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("HTTPOriginForWebSocketBaseURL(%q) expected error", tt.raw)
				}
				if !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("HTTPOriginForWebSocketBaseURL(%q) error = %q, want substring %q", tt.raw, err.Error(), tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("HTTPOriginForWebSocketBaseURL(%q) error = %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("HTTPOriginForWebSocketBaseURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestHTTPOriginForWebSocketBaseURLWithOptions(t *testing.T) {
	got, err := HTTPOriginForWebSocketBaseURLWithOptions("http://10.0.0.5:7655/pulse", PulseURLValidationOptions{
		AllowInsecureHTTP: true,
	})
	if err != nil {
		t.Fatalf("HTTPOriginForWebSocketBaseURLWithOptions() error = %v", err)
	}
	if got != "http://10.0.0.5:7655" {
		t.Fatalf("HTTPOriginForWebSocketBaseURLWithOptions() = %q", got)
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
