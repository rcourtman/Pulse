package api

import (
	"crypto/tls"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestIsDirectLoopbackRequest(t *testing.T) {
	t.Helper()

	tests := []struct {
		name       string
		req        *http.Request
		remoteAddr string
		headers    map[string]string
		want       bool
	}{
		// Nil request
		{
			name: "nil request",
			req:  nil,
			want: false,
		},

		// Valid loopback IPs without proxy headers
		{
			name:       "loopback IPv4 without port",
			remoteAddr: "127.0.0.1",
			want:       true,
		},
		{
			name:       "loopback IPv4 with port",
			remoteAddr: "127.0.0.1:8080",
			want:       true,
		},
		{
			name:       "loopback IPv4 alternate",
			remoteAddr: "127.0.0.2:54321",
			want:       true,
		},
		{
			name:       "loopback IPv6 without port",
			remoteAddr: "::1",
			want:       true,
		},
		{
			name:       "loopback IPv6 with port",
			remoteAddr: "[::1]:8080",
			want:       true,
		},
		{
			name:       "loopback IPv6 with brackets no port",
			remoteAddr: "[::1]",
			want:       true,
		},

		// Loopback with proxy headers (should reject)
		{
			name:       "loopback with X-Forwarded-For",
			remoteAddr: "127.0.0.1:8080",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.1",
			},
			want: false,
		},
		{
			name:       "loopback with Forwarded",
			remoteAddr: "127.0.0.1:8080",
			headers: map[string]string{
				"Forwarded": "for=192.168.1.1",
			},
			want: false,
		},
		{
			name:       "loopback with X-Real-IP",
			remoteAddr: "127.0.0.1:8080",
			headers: map[string]string{
				"X-Real-IP": "192.168.1.1",
			},
			want: false,
		},
		{
			name:       "loopback with multiple proxy headers",
			remoteAddr: "127.0.0.1:8080",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.1",
				"X-Real-IP":       "10.0.0.1",
			},
			want: false,
		},
		{
			name:       "loopback IPv6 with X-Forwarded-For",
			remoteAddr: "[::1]:8080",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.42",
			},
			want: false,
		},

		// Non-loopback IPs (should reject)
		{
			name:       "private IPv4",
			remoteAddr: "192.168.1.1:8080",
			want:       false,
		},
		{
			name:       "private IPv4 10.x",
			remoteAddr: "10.0.0.1:54321",
			want:       false,
		},
		{
			name:       "private IPv4 172.x",
			remoteAddr: "172.16.0.1:8080",
			want:       false,
		},
		{
			name:       "public IPv4",
			remoteAddr: "203.0.113.42:8080",
			want:       false,
		},
		{
			name:       "public IPv6",
			remoteAddr: "[2001:db8::1]:8080",
			want:       false,
		},
		{
			name:       "link-local IPv6",
			remoteAddr: "[fe80::1]:8080",
			want:       false,
		},

		// Edge cases
		{
			name:       "empty RemoteAddr",
			remoteAddr: "",
			want:       false,
		},
		{
			name:       "invalid IP format",
			remoteAddr: "not-an-ip:8080",
			want:       false,
		},
		{
			name:       "invalid IP with port",
			remoteAddr: "999.999.999.999:8080",
			want:       false,
		},
		{
			name:       "malformed IPv6",
			remoteAddr: "[::g]:8080",
			want:       false,
		},
		{
			name:       "just port",
			remoteAddr: ":8080",
			want:       false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.req != nil {
				req = tt.req
			} else if tt.name != "nil request" {
				req = httptest.NewRequest("GET", "/", nil)
				req.RemoteAddr = tt.remoteAddr
				for key, value := range tt.headers {
					req.Header.Set(key, value)
				}
			}

			got := isDirectLoopbackRequest(req)
			if got != tt.want {
				t.Errorf("isDirectLoopbackRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFirstForwardedValue(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		// Empty/nil cases
		{
			name:   "empty string",
			header: "",
			want:   "",
		},

		// Single value
		{
			name:   "single IP",
			header: "192.168.1.1",
			want:   "192.168.1.1",
		},
		{
			name:   "single IP with whitespace",
			header: "  192.168.1.1  ",
			want:   "192.168.1.1",
		},
		{
			name:   "single IPv6",
			header: "2001:db8::1",
			want:   "2001:db8::1",
		},

		// Multiple values (comma-separated)
		{
			name:   "multiple IPs returns first",
			header: "192.168.1.1, 10.0.0.1, 172.16.0.1",
			want:   "192.168.1.1",
		},
		{
			name:   "multiple IPs with extra whitespace",
			header: "  203.0.113.42  ,  192.168.1.1  ",
			want:   "203.0.113.42",
		},
		{
			name:   "first value empty after split",
			header: ", 192.168.1.1",
			want:   "",
		},
		{
			name:   "only commas",
			header: ",,,",
			want:   "",
		},

		// Realistic proxy chain scenarios
		{
			name:   "proxy chain client first",
			header: "client.example.com, proxy1.example.com, proxy2.example.com",
			want:   "client.example.com",
		},
		{
			name:   "mixed IPv4 and IPv6 chain",
			header: "2001:db8::1, 192.168.1.1, 10.0.0.1",
			want:   "2001:db8::1",
		},

		// Edge cases
		{
			name:   "value with port (non-standard but seen in wild)",
			header: "192.168.1.1:8080, 10.0.0.1",
			want:   "192.168.1.1:8080",
		},
		{
			name:   "hostname instead of IP",
			header: "client.example.com",
			want:   "client.example.com",
		},
		{
			name:   "tabs and newlines stripped",
			header: "\t192.168.1.1\n",
			want:   "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstForwardedValue(tt.header)
			if got != tt.want {
				t.Errorf("firstForwardedValue(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

func TestSanitizeForwardedHost(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		wantFull     string // host with port preserved
		wantHostOnly string // host without port, brackets stripped
	}{
		// Empty cases
		{
			name:         "empty string",
			raw:          "",
			wantFull:     "",
			wantHostOnly: "",
		},
		{
			name:         "only whitespace",
			raw:          "   ",
			wantFull:     "",
			wantHostOnly: "",
		},
		{
			name:         "only scheme http",
			raw:          "http://",
			wantFull:     "",
			wantHostOnly: "",
		},
		{
			name:         "only scheme https",
			raw:          "https://",
			wantFull:     "",
			wantHostOnly: "",
		},

		// Simple hostnames
		{
			name:         "simple hostname",
			raw:          "example.com",
			wantFull:     "example.com",
			wantHostOnly: "example.com",
		},
		{
			name:         "hostname with whitespace",
			raw:          "  example.com  ",
			wantFull:     "example.com",
			wantHostOnly: "example.com",
		},
		{
			name:         "fqdn",
			raw:          "api.example.com",
			wantFull:     "api.example.com",
			wantHostOnly: "api.example.com",
		},

		// Hostnames with ports
		{
			name:         "hostname with port",
			raw:          "example.com:8080",
			wantFull:     "example.com:8080",
			wantHostOnly: "example.com",
		},
		{
			name:         "hostname with standard https port",
			raw:          "example.com:443",
			wantFull:     "example.com:443",
			wantHostOnly: "example.com",
		},
		{
			name:         "hostname with standard http port",
			raw:          "example.com:80",
			wantFull:     "example.com:80",
			wantHostOnly: "example.com",
		},

		// With scheme prefixes
		{
			name:         "http scheme stripped",
			raw:          "http://example.com",
			wantFull:     "example.com",
			wantHostOnly: "example.com",
		},
		{
			name:         "https scheme stripped",
			raw:          "https://example.com",
			wantFull:     "example.com",
			wantHostOnly: "example.com",
		},
		{
			name:         "http scheme with port",
			raw:          "http://example.com:8080",
			wantFull:     "example.com:8080",
			wantHostOnly: "example.com",
		},
		{
			name:         "https scheme with port",
			raw:          "https://example.com:9443",
			wantFull:     "example.com:9443",
			wantHostOnly: "example.com",
		},

		// Trailing slashes/paths
		{
			name:         "trailing slash stripped",
			raw:          "example.com/",
			wantFull:     "example.com",
			wantHostOnly: "example.com",
		},
		{
			name:         "scheme and trailing slash",
			raw:          "https://example.com/",
			wantFull:     "example.com",
			wantHostOnly: "example.com",
		},

		// IPv4 addresses
		{
			name:         "IPv4 address",
			raw:          "192.168.1.1",
			wantFull:     "192.168.1.1",
			wantHostOnly: "192.168.1.1",
		},
		{
			name:         "IPv4 with port",
			raw:          "192.168.1.1:8080",
			wantFull:     "192.168.1.1:8080",
			wantHostOnly: "192.168.1.1",
		},
		{
			name:         "IPv4 with scheme",
			raw:          "http://10.0.0.1",
			wantFull:     "10.0.0.1",
			wantHostOnly: "10.0.0.1",
		},

		// IPv6 addresses (key edge case - bracket handling)
		{
			name:         "IPv6 with brackets",
			raw:          "[::1]",
			wantFull:     "[::1]",
			wantHostOnly: "::1",
		},
		{
			name:         "IPv6 with brackets and port",
			raw:          "[::1]:8080",
			wantFull:     "[::1]:8080",
			wantHostOnly: "::1",
		},
		{
			name:         "IPv6 full address with brackets",
			raw:          "[2001:db8::1]",
			wantFull:     "[2001:db8::1]",
			wantHostOnly: "2001:db8::1",
		},
		{
			name:         "IPv6 full address with brackets and port",
			raw:          "[2001:db8::1]:443",
			wantFull:     "[2001:db8::1]:443",
			wantHostOnly: "2001:db8::1",
		},
		{
			name:         "IPv6 with scheme",
			raw:          "https://[::1]:9443",
			wantFull:     "[::1]:9443",
			wantHostOnly: "::1",
		},
		{
			name:         "IPv6 without brackets (raw - no port possible)",
			raw:          "::1",
			wantFull:     "::1",
			wantHostOnly: "::1",
		},

		// Realistic forwarded host headers
		{
			name:         "X-Forwarded-Host typical",
			raw:          "api.myservice.com",
			wantFull:     "api.myservice.com",
			wantHostOnly: "api.myservice.com",
		},
		{
			name:         "reverse proxy with non-standard port",
			raw:          "internal.corp.local:7655",
			wantFull:     "internal.corp.local:7655",
			wantHostOnly: "internal.corp.local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFull, gotHostOnly := sanitizeForwardedHost(tt.raw)
			if gotFull != tt.wantFull {
				t.Errorf("sanitizeForwardedHost(%q) full = %q, want %q", tt.raw, gotFull, tt.wantFull)
			}
			if gotHostOnly != tt.wantHostOnly {
				t.Errorf("sanitizeForwardedHost(%q) hostOnly = %q, want %q", tt.raw, gotHostOnly, tt.wantHostOnly)
			}
		})
	}
}

func TestIsLoopbackHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want bool
	}{
		// Empty/special cases (treated as loopback for safety)
		{
			name: "empty string",
			host: "",
			want: true,
		},

		// Localhost keyword
		{
			name: "localhost lowercase",
			host: "localhost",
			want: true,
		},
		{
			name: "localhost uppercase",
			host: "LOCALHOST",
			want: true,
		},
		{
			name: "localhost mixed case",
			host: "LocalHost",
			want: true,
		},

		// IPv4 loopback range
		{
			name: "127.0.0.1",
			host: "127.0.0.1",
			want: true,
		},
		{
			name: "127.0.0.2 (full loopback range)",
			host: "127.0.0.2",
			want: true,
		},
		{
			name: "127.255.255.255 (end of loopback range)",
			host: "127.255.255.255",
			want: true,
		},

		// IPv6 loopback
		{
			name: "::1",
			host: "::1",
			want: true,
		},

		// Unspecified addresses (treated as loopback)
		{
			name: "0.0.0.0 unspecified IPv4",
			host: "0.0.0.0",
			want: true,
		},
		{
			name: ":: unspecified IPv6",
			host: "::",
			want: true,
		},

		// Non-loopback private addresses
		{
			name: "private 192.168.x",
			host: "192.168.1.1",
			want: false,
		},
		{
			name: "private 10.x",
			host: "10.0.0.1",
			want: false,
		},
		{
			name: "private 172.16.x",
			host: "172.16.0.1",
			want: false,
		},

		// Non-loopback public addresses
		{
			name: "public IPv4",
			host: "203.0.113.42",
			want: false,
		},
		{
			name: "public IPv6",
			host: "2001:db8::1",
			want: false,
		},

		// Hostnames (not IPs)
		{
			name: "regular hostname",
			host: "example.com",
			want: false,
		},
		{
			name: "fqdn",
			host: "api.example.com",
			want: false,
		},
		{
			name: "localhost-like but not localhost",
			host: "localhost.example.com",
			want: false,
		},
		{
			name: "hostname starting with local",
			host: "local.example.com",
			want: false,
		},

		// Edge cases
		{
			name: "link-local IPv6",
			host: "fe80::1",
			want: false,
		},
		{
			name: "multicast",
			host: "224.0.0.1",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLoopbackHost(tt.host)
			if got != tt.want {
				t.Errorf("isLoopbackHost(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

func TestShouldAppendForwardedPort(t *testing.T) {
	tests := []struct {
		name   string
		port   string
		scheme string
		want   bool
	}{
		// Empty port
		{
			name:   "empty port",
			port:   "",
			scheme: "https",
			want:   false,
		},

		// Invalid port (non-numeric)
		{
			name:   "non-numeric port",
			port:   "abc",
			scheme: "https",
			want:   false,
		},
		{
			name:   "port with letters",
			port:   "80a",
			scheme: "http",
			want:   false,
		},
		{
			name:   "negative port string (Atoi accepts it)",
			port:   "-80",
			scheme: "http",
			want:   true, // strconv.Atoi parses "-80" as -80 (valid int)
		},

		// Default ports that should NOT be appended
		{
			name:   "https with 443",
			port:   "443",
			scheme: "https",
			want:   false,
		},
		{
			name:   "http with 80",
			port:   "80",
			scheme: "http",
			want:   false,
		},

		// Default ports for wrong scheme SHOULD be appended
		{
			name:   "http with 443 (unusual)",
			port:   "443",
			scheme: "http",
			want:   true,
		},
		{
			name:   "https with 80 (unusual)",
			port:   "80",
			scheme: "https",
			want:   true,
		},

		// Non-default ports should be appended
		{
			name:   "https with 8443",
			port:   "8443",
			scheme: "https",
			want:   true,
		},
		{
			name:   "http with 8080",
			port:   "8080",
			scheme: "http",
			want:   true,
		},
		{
			name:   "https with custom port",
			port:   "9443",
			scheme: "https",
			want:   true,
		},
		{
			name:   "pulse default port",
			port:   "7655",
			scheme: "https",
			want:   true,
		},

		// Edge cases
		{
			name:   "port 0",
			port:   "0",
			scheme: "http",
			want:   true,
		},
		{
			name:   "high port number",
			port:   "65535",
			scheme: "https",
			want:   true,
		},
		{
			name:   "empty scheme with non-default port",
			port:   "8080",
			scheme: "",
			want:   true,
		},
		{
			name:   "unknown scheme",
			port:   "443",
			scheme: "wss",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldAppendForwardedPort(tt.port, tt.scheme)
			if got != tt.want {
				t.Errorf("shouldAppendForwardedPort(%q, %q) = %v, want %v", tt.port, tt.scheme, got, tt.want)
			}
		})
	}
}

func TestCanCapturePublicURL_NilInputs(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	cfg := &config.Config{}

	tests := []struct {
		name string
		cfg  *config.Config
		req  *http.Request
		want bool
	}{
		{
			name: "nil config",
			cfg:  nil,
			req:  req,
			want: false,
		},
		{
			name: "nil request",
			cfg:  cfg,
			req:  nil,
			want: false,
		},
		{
			name: "both nil",
			cfg:  nil,
			req:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := canCapturePublicURL(tt.cfg, tt.req)
			if got != tt.want {
				t.Errorf("canCapturePublicURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCapturePublicURLFromRequest_NilInputs(t *testing.T) {
	t.Parallel()

	t.Run("nil router", func(t *testing.T) {
		var r *Router
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		// Should not panic
		r.capturePublicURLFromRequest(req)
	})

	t.Run("nil request", func(t *testing.T) {
		r := &Router{config: &config.Config{}}
		// Should not panic
		r.capturePublicURLFromRequest(nil)
	})

	t.Run("nil config", func(t *testing.T) {
		r := &Router{config: nil}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		// Should not panic
		r.capturePublicURLFromRequest(req)
	})
}

func TestHostAgentSearchCandidates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		platform string
		arch     string
		wantLen  int // expected number of search paths
	}{
		{
			name:     "strict mode with both params",
			platform: "linux",
			arch:     "amd64",
			wantLen:  3, // 3 paths with platform-arch suffix
		},
		{
			name:     "platform only, no arch",
			platform: "linux",
			arch:     "",
			wantLen:  3, // 3 paths with platform suffix only
		},
		{
			name:     "no params returns generic paths",
			platform: "",
			arch:     "",
			wantLen:  3, // 3 generic paths
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := hostAgentSearchCandidates(tt.platform, tt.arch)
			if len(paths) != tt.wantLen {
				t.Errorf("hostAgentSearchCandidates(%q, %q) returned %d paths, want %d",
					tt.platform, tt.arch, len(paths), tt.wantLen)
			}
		})
	}
}

func TestResolvePublicURL_ConfiguredPublicURL(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		want      string
	}{
		{
			name:      "simple URL",
			publicURL: "https://pulse.example.com",
			want:      "https://pulse.example.com",
		},
		{
			name:      "URL with trailing slash",
			publicURL: "https://pulse.example.com/",
			want:      "https://pulse.example.com",
		},
		{
			name:      "URL with multiple trailing slashes",
			publicURL: "https://pulse.example.com///",
			want:      "https://pulse.example.com",
		},
		{
			name:      "URL with port",
			publicURL: "https://pulse.example.com:8443",
			want:      "https://pulse.example.com:8443",
		},
		{
			name:      "URL with port and trailing slash",
			publicURL: "https://pulse.example.com:8443/",
			want:      "https://pulse.example.com:8443",
		},
		{
			name:      "URL with whitespace",
			publicURL: "  https://pulse.example.com  ",
			want:      "https://pulse.example.com",
		},
		{
			name:      "HTTP URL",
			publicURL: "http://internal.local:7655",
			want:      "http://internal.local:7655",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Router{
				config: &config.Config{
					PublicURL: tt.publicURL,
				},
			}

			// Request doesn't matter when PublicURL is configured
			req := httptest.NewRequest(http.MethodGet, "/", nil)

			got := r.resolvePublicURL(req)
			if got != tt.want {
				t.Errorf("resolvePublicURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolvePublicURL_FromRequest(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		useTLS        bool
		xForwardProto string
		frontendPort  int
		want          string
	}{
		{
			name:   "HTTP request with host header",
			host:   "pulse.example.com",
			useTLS: false,
			want:   "http://pulse.example.com",
		},
		{
			name:   "HTTPS request via TLS",
			host:   "pulse.example.com",
			useTLS: true,
			want:   "https://pulse.example.com",
		},
		{
			name:          "HTTP request with X-Forwarded-Proto https",
			host:          "pulse.example.com",
			useTLS:        false,
			xForwardProto: "https",
			want:          "https://pulse.example.com",
		},
		{
			name:          "X-Forwarded-Proto case insensitive",
			host:          "pulse.example.com",
			useTLS:        false,
			xForwardProto: "HTTPS",
			want:          "https://pulse.example.com",
		},
		{
			name:          "X-Forwarded-Proto http remains http",
			host:          "pulse.example.com",
			useTLS:        false,
			xForwardProto: "http",
			want:          "http://pulse.example.com",
		},
		{
			name:   "Host with port",
			host:   "pulse.example.com:8080",
			useTLS: false,
			want:   "http://pulse.example.com:8080",
		},
		{
			name:   "Host with whitespace is trimmed",
			host:   "  pulse.example.com  ",
			useTLS: false,
			want:   "http://pulse.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Router{
				config: &config.Config{
					PublicURL:    "", // not configured
					FrontendPort: tt.frontendPort,
				},
			}

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = tt.host

			if tt.useTLS {
				req.TLS = &tls.ConnectionState{} // Non-nil TLS indicates HTTPS
			}

			if tt.xForwardProto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.xForwardProto)
			}

			got := r.resolvePublicURL(req)
			if got != tt.want {
				t.Errorf("resolvePublicURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolvePublicURL_NoHostFallback(t *testing.T) {
	tests := []struct {
		name         string
		frontendPort int
		want         string
	}{
		{
			name:         "with configured frontend port",
			frontendPort: 8080,
			want:         "http://localhost:8080",
		},
		{
			name:         "with default pulse port",
			frontendPort: 7655,
			want:         "http://localhost:7655",
		},
		{
			name:         "with zero port uses default",
			frontendPort: 0,
			want:         "http://localhost:7655",
		},
		{
			name:         "with negative port uses default",
			frontendPort: -1,
			want:         "http://localhost:7655",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Router{
				config: &config.Config{
					PublicURL:    "",
					FrontendPort: tt.frontendPort,
				},
			}

			// Request with empty host
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = ""

			got := r.resolvePublicURL(req)
			if got != tt.want {
				t.Errorf("resolvePublicURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolvePublicURL_NilRequest(t *testing.T) {
	tests := []struct {
		name         string
		frontendPort int
		want         string
	}{
		{
			name:         "nil request with frontend port",
			frontendPort: 9000,
			want:         "http://localhost:9000",
		},
		{
			name:         "nil request with zero port",
			frontendPort: 0,
			want:         "http://localhost:7655",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Router{
				config: &config.Config{
					PublicURL:    "",
					FrontendPort: tt.frontendPort,
				},
			}

			got := r.resolvePublicURL(nil)
			if got != tt.want {
				t.Errorf("resolvePublicURL(nil) = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCanCapturePublicURL_Security(t *testing.T) {
	// Proxy Auth Tests
	t.Run("ProxyAuth", func(t *testing.T) {
		cfg := &config.Config{
			ProxyAuthSecret:     "secret",
			ProxyAuthUserHeader: "X-User",
			ProxyAuthRoleHeader: "X-Role",
			ProxyAuthAdminRole:  "admin",
		}

		t.Run("admin allowed", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-Proxy-Secret", "secret")
			req.Header.Set("X-User", "adminuser")
			req.Header.Set("X-Role", "admin")
			if !canCapturePublicURL(cfg, req) {
				t.Error("expected true for admin proxy user")
			}
		})

		t.Run("non-admin denied", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-Proxy-Secret", "secret")
			req.Header.Set("X-User", "user")
			req.Header.Set("X-Role", "user")
			if canCapturePublicURL(cfg, req) {
				t.Error("expected false for non-admin proxy user")
			}
		})
	})

	// API Token Tests
	t.Run("APITokens", func(t *testing.T) {
		recordRead, _ := config.NewAPITokenRecord("read-token-123.12345678", "read", []string{"start:read"}) // Low scope
		recordWrite, _ := config.NewAPITokenRecord("write-token-123.12345678", "write", []string{config.ScopeSettingsWrite})

		cfg := &config.Config{
			APITokens: []config.APITokenRecord{*recordRead, *recordWrite},
		}

		t.Run("token with settings:write allowed", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-API-Token", "write-token-123.12345678")
			if !canCapturePublicURL(cfg, req) {
				t.Error("expected true for token with settings:write")
			}
		})

		t.Run("token without settings:write denied", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-API-Token", "read-token-123.12345678")
			if canCapturePublicURL(cfg, req) {
				t.Error("expected false for token without settings:write")
			}
		})
	})

	// Basic Auth Tests
	t.Run("BasicAuth", func(t *testing.T) {
		password := "password"
		hashed, _ := auth.HashPassword(password)
		cfg := &config.Config{
			AuthUser: "admin",
			AuthPass: hashed,
		}

		t.Run("valid credentials allowed", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			creds := base64.StdEncoding.EncodeToString([]byte("admin:" + password))
			req.Header.Set("Authorization", "Basic "+creds)
			if !canCapturePublicURL(cfg, req) {
				t.Error("expected true for valid basic auth")
			}
		})
	})
}
