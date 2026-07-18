package securityutil

import (
	"context"
	"net"
	"net/url"
	"strings"
	"testing"
)

// This file raises branch/function coverage for the pure and pre-DNS error
// paths in outbound_http.go and httpurl.go. It does not exercise any network,
// filesystem, or global-resolver state: every hostname target is either an
// IP literal (which resolvePermittedOutboundIP short-circuits without DNS) or
// a value rejected before DNS resolution runs.

func TestBranchCovAllowedOutboundSchemes(t *testing.T) {
	tests := []struct {
		name string
		opts RestrictedOutboundHTTPOptions
		want []string
	}{
		{name: "empty allowed schemes defaults to http https", opts: RestrictedOutboundHTTPOptions{}, want: []string{"http", "https"}},
		{name: "nil slice defaults to http https", opts: RestrictedOutboundHTTPOptions{AllowedSchemes: nil}, want: []string{"http", "https"}},
		{name: "explicit schemes returned verbatim", opts: RestrictedOutboundHTTPOptions{AllowedSchemes: []string{"https", "ftp"}}, want: []string{"https", "ftp"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := allowedOutboundSchemes(tt.opts)
			if len(got) != len(tt.want) {
				t.Fatalf("allowedOutboundSchemes() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("allowedOutboundSchemes()[%d] = %q, want %q (full: %v)", i, got[i], tt.want[i], got)
				}
			}
		})
	}
}

func TestBranchCovIsAllowedOutboundScheme(t *testing.T) {
	tests := []struct {
		name   string
		scheme string
		allow  []string
		want   bool
	}{
		{name: "exact match http", scheme: "http", allow: []string{"http", "https"}, want: true},
		{name: "match ignores surrounding whitespace in candidate", scheme: "https", allow: []string{"  https  ", "http"}, want: true},
		{name: "match is case insensitive on candidate", scheme: "HTTPS", allow: []string{"https", "http"}, want: true},
		{name: "no match when scheme absent", scheme: "ftp", allow: []string{"http", "https"}, want: false},
		{name: "no match against empty allow list", scheme: "http", allow: []string{}, want: false},
		{name: "exact candidate differs after trim", scheme: "http", allow: []string{"httpx"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAllowedOutboundScheme(tt.scheme, tt.allow); got != tt.want {
				t.Fatalf("isAllowedOutboundScheme(%q, %v) = %v, want %v", tt.scheme, tt.allow, got, tt.want)
			}
		})
	}
}

func TestBranchCovValidateOutboundIP(t *testing.T) {
	allOpts := RestrictedOutboundHTTPOptions{AllowPrivateIPs: true, AllowLoopback: true}
	strictOpts := RestrictedOutboundHTTPOptions{}

	tests := []struct {
		name    string
		ip      net.IP
		opts    RestrictedOutboundHTTPOptions
		wantErr string // empty means expect nil
	}{
		{name: "nil ip rejected", ip: net.IP(nil), opts: allOpts, wantErr: "invalid IP address"},
		{name: "loopback blocked when not allowed", ip: net.ParseIP("127.0.0.1"), opts: strictOpts, wantErr: "loopback addresses are not allowed"},
		{name: "loopback permitted when allowed", ip: net.ParseIP("127.0.0.1"), opts: allOpts, wantErr: ""},
		{name: "metadata service ip always rejected", ip: net.ParseIP("169.254.169.254"), opts: allOpts, wantErr: "metadata service address is not allowed"},
		{name: "link local unicast rejected", ip: net.ParseIP("169.254.10.20"), opts: allOpts, wantErr: "link-local addresses are not allowed"},
		{name: "link local multicast rejected as link local", ip: net.ParseIP("224.0.0.1"), opts: allOpts, wantErr: "link-local addresses are not allowed"},
		{name: "non-link-local multicast rejected", ip: net.ParseIP("239.0.0.1"), opts: allOpts, wantErr: "multicast addresses are not allowed"},
		{name: "unspecified ipv4 rejected", ip: net.ParseIP("0.0.0.0"), opts: allOpts, wantErr: "unspecified addresses are not allowed"},
		{name: "unspecified ipv6 rejected", ip: net.ParseIP("::"), opts: allOpts, wantErr: "unspecified addresses are not allowed"},
		{name: "private blocked when not allowed", ip: net.ParseIP("192.168.1.1"), opts: strictOpts, wantErr: "private addresses are not allowed"},
		{name: "private permitted when allowed", ip: net.ParseIP("10.0.0.1"), opts: allOpts, wantErr: ""},
		{name: "public documentation ip accepted", ip: net.ParseIP("203.0.113.1"), opts: strictOpts, wantErr: ""},
		{name: "public ipv6 accepted", ip: net.ParseIP("2001:db8::1"), opts: strictOpts, wantErr: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOutboundIP(tt.ip, tt.opts)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateOutboundIP(%v, %+v) unexpected error: %v", tt.ip, tt.opts, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateOutboundIP(%v, %+v) = nil, want error containing %q", tt.ip, tt.opts, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateOutboundIP(%v, %+v) err = %q, want substring %q", tt.ip, tt.opts, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestBranchCovCanonicalOriginHost(t *testing.T) {
	tests := []struct {
		name string
		u    func(t *testing.T) *url.URL // nil returns nil *url.URL
		want string
	}{
		{name: "nil url returns empty", u: func(t *testing.T) *url.URL { return nil }, want: ""},
		{name: "http default port 80", u: func(t *testing.T) *url.URL { return mustParseURL(t, "http://example.com/p") }, want: "example.com:80"},
		{name: "https default port 443", u: func(t *testing.T) *url.URL { return mustParseURL(t, "https://example.com/p") }, want: "example.com:443"},
		{name: "host uppercased normalized", u: func(t *testing.T) *url.URL { return mustParseURL(t, "http://EXAMPLE.com/p") }, want: "example.com:80"},
		{name: "explicit port preserved", u: func(t *testing.T) *url.URL { return mustParseURL(t, "https://example.com:8443/p") }, want: "example.com:8443"},
		{name: "ipv6 default http port", u: func(t *testing.T) *url.URL { return mustParseURL(t, "http://[::1]/p") }, want: "[::1]:80"},
		{name: "ipv6 explicit port", u: func(t *testing.T) *url.URL { return mustParseURL(t, "http://[::1]:9000/p") }, want: "[::1]:9000"},
		{name: "unknown scheme no port returns raw host", u: func(t *testing.T) *url.URL { return mustParseURL(t, "ftp://example.com/p") }, want: "example.com"},
		{name: "empty host falls back to raw host", u: func(t *testing.T) *url.URL { return mustParseURL(t, "http://") }, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canonicalOriginHost(tt.u(t))
			if got != tt.want {
				t.Fatalf("canonicalOriginHost() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBranchCovValidateOutboundFetchURL(t *testing.T) {
	httpsOnly := RestrictedOutboundHTTPOptions{AllowedSchemes: []string{"https"}}
	permissive := RestrictedOutboundHTTPOptions{AllowedSchemes: []string{"http", "https"}, AllowPrivateIPs: true, AllowLoopback: true}

	tests := []struct {
		name    string
		raw     string
		opts    RestrictedOutboundHTTPOptions
		wantErr string
		wantStr string
	}{
		{name: "empty raw rejected", raw: "   ", opts: permissive, wantErr: "URL is required"},
		{name: "malformed raw rejected", raw: ":not-a-url", opts: permissive, wantErr: "invalid URL"},
		{name: "default host required", raw: "https://", opts: permissive, wantErr: "URL host is required"},
		{name: "disallowed scheme rejected with configured list", raw: "http://203.0.113.1/x", opts: httpsOnly, wantErr: "URL scheme must be one of: https"},
		{name: "fragment rejected before dns", raw: "https://203.0.113.1/x#frag", opts: permissive, wantErr: "URL fragments are not allowed"},
		{name: "ip literal metadata blocked without dns", raw: "http://169.254.169.254/x", opts: permissive, wantErr: "metadata service address is not allowed"},
		{name: "ip literal private blocked without dns", raw: "http://192.168.1.1/x", opts: RestrictedOutboundHTTPOptions{AllowedSchemes: []string{"http", "https"}}, wantErr: "private addresses are not allowed"},
		{name: "ip literal public success without dns", raw: "http://203.0.113.1/path?q=1", opts: permissive, wantStr: "http://203.0.113.1/path?q=1"},
		{name: "https ip literal success", raw: "https://203.0.113.1/p", opts: httpsOnly, wantStr: "https://203.0.113.1/p"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateOutboundFetchURL(context.Background(), tt.raw, tt.opts)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("ValidateOutboundFetchURL(%q) = %v, want error containing %q", tt.raw, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ValidateOutboundFetchURL(%q) err = %q, want substring %q", tt.raw, err.Error(), tt.wantErr)
				}
				if got != nil {
					t.Fatalf("ValidateOutboundFetchURL(%q) returned non-nil URL with error: %v", tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateOutboundFetchURL(%q) unexpected error: %v", tt.raw, err)
			}
			if got == nil {
				t.Fatalf("ValidateOutboundFetchURL(%q) returned nil URL", tt.raw)
			}
			if got.String() != tt.wantStr {
				t.Fatalf("ValidateOutboundFetchURL(%q) = %q, want %q", tt.raw, got.String(), tt.wantStr)
			}
		})
	}
}

func TestBranchCovNewValidatedRequestWithContext(t *testing.T) {
	t.Run("nil target errors", func(t *testing.T) {
		req, err := NewValidatedRequestWithContext(context.Background(), "GET", nil, nil)
		if err == nil || !strings.Contains(err.Error(), "target URL is required") {
			t.Fatalf("err = %v, want target URL is required", err)
		}
		if req != nil {
			t.Fatalf("req = %v, want nil", req)
		}
	})

	t.Run("invalid method surfaces new request error", func(t *testing.T) {
		target := mustParseURL(t, "https://example.com/p")
		req, err := NewValidatedRequestWithContext(context.Background(), "GET X", target, nil)
		if err == nil || !strings.Contains(err.Error(), "invalid method") {
			t.Fatalf("err = %v, want invalid method", err)
		}
		if req != nil {
			t.Fatalf("req = %v, want nil", req)
		}
	})

	t.Run("valid target clones url and sets host", func(t *testing.T) {
		target := mustParseURL(t, "https://api.example.com:8443/v1?k=v")
		req, err := NewValidatedRequestWithContext(context.Background(), "GET", target, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req == nil {
			t.Fatal("req is nil")
		}
		if req.Method != "GET" {
			t.Fatalf("Method = %q, want GET", req.Method)
		}
		if req.URL == nil || req.URL.String() != "https://api.example.com:8443/v1?k=v" {
			t.Fatalf("URL = %v, want the target URL", req.URL)
		}
		if req.URL == target {
			t.Fatal("URL is the same pointer as target; expected a clone")
		}
		if req.Host != "api.example.com:8443" {
			t.Fatalf("Host = %q, want api.example.com:8443", req.Host)
		}
		if req.RequestURI != "" {
			t.Fatalf("RequestURI = %q, want empty", req.RequestURI)
		}
	})

	t.Run("post method preserved with body", func(t *testing.T) {
		target := mustParseURL(t, "https://example.com/ingest")
		req, err := NewValidatedRequestWithContext(context.Background(), "POST", target, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Method != "POST" {
			t.Fatalf("Method = %q, want POST", req.Method)
		}
	})
}

func TestBranchCovNewRelativeRequestWithContext(t *testing.T) {
	t.Run("nil base errors before method validation", func(t *testing.T) {
		req, err := NewRelativeRequestWithContext(context.Background(), "GET", nil, "/x", nil)
		if err == nil || !strings.Contains(err.Error(), "base URL is required") {
			t.Fatalf("err = %v, want base URL is required", err)
		}
		if req != nil {
			t.Fatalf("req = %v, want nil", req)
		}
	})

	t.Run("empty relative path errors", func(t *testing.T) {
		base := mustParseURL(t, "https://api.example.com/v1")
		_, err := NewRelativeRequestWithContext(context.Background(), "GET", base, "   ", nil)
		if err == nil || !strings.Contains(err.Error(), "relative path is required") {
			t.Fatalf("err = %v, want relative path is required", err)
		}
	})

	t.Run("backslash in relative path errors", func(t *testing.T) {
		base := mustParseURL(t, "https://api.example.com/v1")
		_, err := NewRelativeRequestWithContext(context.Background(), "GET", base, `/a\b`, nil)
		if err == nil || !strings.Contains(err.Error(), "must not contain backslashes") {
			t.Fatalf("err = %v, want backslash rejection", err)
		}
	})

	t.Run("absolute url in relative path errors", func(t *testing.T) {
		base := mustParseURL(t, "https://api.example.com/v1")
		_, err := NewRelativeRequestWithContext(context.Background(), "GET", base, "https://evil.example/x", nil)
		if err == nil || !strings.Contains(err.Error(), "must not include scheme or host") {
			t.Fatalf("err = %v, want scheme/host rejection", err)
		}
	})

	t.Run("relative path without leading slash errors", func(t *testing.T) {
		base := mustParseURL(t, "https://api.example.com/v1")
		_, err := NewRelativeRequestWithContext(context.Background(), "GET", base, "users", nil)
		if err == nil || !strings.Contains(err.Error(), "must start with '/'") {
			t.Fatalf("err = %v, want leading slash rejection", err)
		}
	})

	t.Run("success builds request with joined url", func(t *testing.T) {
		base := mustParseURL(t, "https://api.example.com/v1")
		req, err := NewRelativeRequestWithContext(context.Background(), "POST", base, "/users?active=1", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req == nil {
			t.Fatal("req is nil")
		}
		if req.Method != "POST" {
			t.Fatalf("Method = %q, want POST", req.Method)
		}
		if req.URL == nil || req.URL.String() != "https://api.example.com/v1/users?active=1" {
			t.Fatalf("URL = %v, want joined relative URL", req.URL)
		}
		if req.Host != "api.example.com" {
			t.Fatalf("Host = %q, want api.example.com", req.Host)
		}
		if req.RequestURI != "" {
			t.Fatalf("RequestURI = %q, want empty", req.RequestURI)
		}
	})
}
