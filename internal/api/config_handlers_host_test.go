package api

import "testing"

func TestExtractHostAndPort(t *testing.T) {
	tests := []struct {
		name     string
		hostStr  string
		wantHost string
		wantPort string
		wantErr  bool
	}{
		// Basic host:port
		{
			name:     "simple host with port",
			hostStr:  "example.com:8006",
			wantHost: "example.com",
			wantPort: "8006",
		},
		{
			name:     "host without port",
			hostStr:  "example.com",
			wantHost: "example.com",
			wantPort: "",
		},

		// HTTP/HTTPS scheme stripping
		{
			name:     "https scheme stripped",
			hostStr:  "https://example.com:8006",
			wantHost: "example.com",
			wantPort: "8006",
		},
		{
			name:     "http scheme stripped",
			hostStr:  "http://example.com:8006",
			wantHost: "example.com",
			wantPort: "8006",
		},
		{
			name:     "https scheme without port",
			hostStr:  "https://example.com",
			wantHost: "example.com",
			wantPort: "",
		},
		{
			name:     "http scheme without port",
			hostStr:  "http://example.com",
			wantHost: "example.com",
			wantPort: "",
		},

		// Path stripping
		{
			name:     "path stripped from host:port",
			hostStr:  "example.com:8006/api/v1",
			wantHost: "example.com",
			wantPort: "8006",
		},
		{
			name:     "path stripped from host without port",
			hostStr:  "example.com/api/v1",
			wantHost: "example.com",
			wantPort: "",
		},
		{
			name:     "full URL with scheme path and port",
			hostStr:  "https://example.com:8443/some/path",
			wantHost: "example.com",
			wantPort: "8443",
		},
		{
			name:     "trailing slash stripped",
			hostStr:  "example.com/",
			wantHost: "example.com",
			wantPort: "",
		},

		// IPv4 addresses
		{
			name:     "IPv4 with port",
			hostStr:  "192.168.1.100:8006",
			wantHost: "192.168.1.100",
			wantPort: "8006",
		},
		{
			name:     "IPv4 without port",
			hostStr:  "192.168.1.100",
			wantHost: "192.168.1.100",
			wantPort: "",
		},
		{
			name:     "IPv4 with scheme and port",
			hostStr:  "https://192.168.1.100:8006",
			wantHost: "192.168.1.100",
			wantPort: "8006",
		},

		// IPv6 addresses with brackets
		{
			name:     "IPv6 with brackets and port",
			hostStr:  "[2001:db8::1]:8006",
			wantHost: "2001:db8::1",
			wantPort: "8006",
		},
		{
			name:     "IPv6 with brackets no port returns error",
			hostStr:  "[2001:db8::1]",
			wantErr:  true, // net.SplitHostPort fails on bracketed IPv6 without port
		},
		{
			name:     "IPv6 with scheme brackets and port",
			hostStr:  "https://[2001:db8::1]:8006",
			wantHost: "2001:db8::1",
			wantPort: "8006",
		},
		{
			name:     "IPv6 loopback with brackets",
			hostStr:  "[::1]:8006",
			wantHost: "::1",
			wantPort: "8006",
		},

		// IPv6 without brackets (no port case)
		{
			name:     "IPv6 without brackets returns as-is",
			hostStr:  "2001:db8::1",
			wantHost: "2001:db8::1",
			wantPort: "",
		},
		{
			name:     "IPv6 full address without brackets",
			hostStr:  "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			wantHost: "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			wantPort: "",
		},
		{
			name:     "IPv6 loopback without brackets",
			hostStr:  "::1",
			wantHost: "::1",
			wantPort: "",
		},

		// Edge cases
		{
			name:     "empty string",
			hostStr:  "",
			wantHost: "",
			wantPort: "",
		},
		{
			name:     "only port colon",
			hostStr:  ":8006",
			wantHost: "",
			wantPort: "8006",
		},
		{
			name:     "localhost with port",
			hostStr:  "localhost:8006",
			wantHost: "localhost",
			wantPort: "8006",
		},
		{
			name:     "localhost without port",
			hostStr:  "localhost",
			wantHost: "localhost",
			wantPort: "",
		},

		// Error cases
		{
			name:     "malformed brackets",
			hostStr:  "[2001:db8::1:8006",
			wantErr:  true,
		},
		{
			name:     "brackets without closing with port attempt",
			hostStr:  "[invalid:8006",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHost, gotPort, err := extractHostAndPort(tt.hostStr)
			if tt.wantErr {
				if err == nil {
					t.Errorf("extractHostAndPort(%q) expected error, got host=%q port=%q", tt.hostStr, gotHost, gotPort)
				}
				return
			}
			if err != nil {
				t.Errorf("extractHostAndPort(%q) unexpected error: %v", tt.hostStr, err)
				return
			}
			if gotHost != tt.wantHost {
				t.Errorf("extractHostAndPort(%q) host = %q, want %q", tt.hostStr, gotHost, tt.wantHost)
			}
			if gotPort != tt.wantPort {
				t.Errorf("extractHostAndPort(%q) port = %q, want %q", tt.hostStr, gotPort, tt.wantPort)
			}
		})
	}
}

func TestNormalizeNodeHost(t *testing.T) {
	tests := []struct {
		name     string
		rawHost  string
		nodeType string
		want     string
	}{
		{
			name:     "adds default port to explicit https without port",
			rawHost:  "https://example.com",
			nodeType: "pve",
			want:     "https://example.com:8006",
		},
		{
			name:     "adds default port for bare pve host",
			rawHost:  "pve.lan",
			nodeType: "pve",
			want:     "https://pve.lan:8006",
		},
		{
			name:     "adds default port for bare pbs host",
			rawHost:  "pbs.lan",
			nodeType: "pbs",
			want:     "https://pbs.lan:8007",
		},
		{
			name:     "preserves custom port",
			rawHost:  "https://example.com:8443",
			nodeType: "pve",
			want:     "https://example.com:8443",
		},
		{
			name:     "supports ipv6 without scheme",
			rawHost:  "2001:db8::1",
			nodeType: "pmg",
			want:     "https://[2001:db8::1]:8006",
		},
		{
			name:     "drops path segments",
			rawHost:  "https://example.com/api",
			nodeType: "pve",
			want:     "https://example.com:8006",
		},
		{
			name:     "adds default port to explicit http scheme",
			rawHost:  "http://example.com",
			nodeType: "pve",
			want:     "http://example.com:8006",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeNodeHost(tt.rawHost, tt.nodeType)
			if err != nil {
				t.Fatalf("normalizeNodeHost returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeNodeHost(%q, %q) = %q, want %q", tt.rawHost, tt.nodeType, got, tt.want)
			}
		})
	}
}
