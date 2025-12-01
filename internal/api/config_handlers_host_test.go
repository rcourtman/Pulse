package api

import (
	"testing"
)

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

func TestValidateIPAddress(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		// Valid IPv4 addresses
		{"valid IPv4 localhost", "127.0.0.1", true},
		{"valid IPv4 private", "192.168.1.1", true},
		{"valid IPv4 public", "8.8.8.8", true},
		{"valid IPv4 zeros", "0.0.0.0", true},
		{"valid IPv4 broadcast", "255.255.255.255", true},
		{"valid IPv4 class A", "10.0.0.1", true},
		{"valid IPv4 class B", "172.16.0.1", true},

		// Valid IPv6 addresses
		{"valid IPv6 loopback", "::1", true},
		{"valid IPv6 unspecified", "::", true},
		{"valid IPv6 full", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", true},
		{"valid IPv6 compressed", "2001:db8:85a3::8a2e:370:7334", true},
		{"valid IPv6 link-local", "fe80::1", true},
		{"valid IPv6 multicast", "ff02::1", true},

		// Invalid addresses
		{"invalid empty", "", false},
		{"invalid hostname", "localhost", false},
		{"invalid domain", "example.com", false},
		{"invalid IPv4 with port", "192.168.1.1:8080", false},
		{"invalid IPv4 out of range", "256.256.256.256", false},
		{"invalid IPv4 too many octets", "192.168.1.1.1", false},
		{"invalid IPv4 too few octets", "192.168.1", false},
		{"invalid IPv4 negative", "-1.0.0.0", false},
		{"invalid IPv4 with letters", "192.168.a.1", false},
		{"invalid IPv6 with port", "[::1]:8080", false},
		{"invalid IPv6 brackets only", "[::1]", false},
		{"invalid random string", "not-an-ip", false},
		{"invalid whitespace", " 192.168.1.1 ", false},
		{"invalid URL", "https://192.168.1.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateIPAddress(tt.ip)
			if got != tt.want {
				t.Errorf("validateIPAddress(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		portStr string
		want    bool
	}{
		// Valid ports
		{"valid minimum port", "1", true},
		{"valid maximum port", "65535", true},
		{"valid common HTTP", "80", true},
		{"valid HTTPS", "443", true},
		{"valid Proxmox PVE", "8006", true},
		{"valid Proxmox PBS", "8007", true},
		{"valid SSH", "22", true},
		{"valid high port", "49152", true},

		// Invalid ports
		{"invalid zero", "0", false},
		{"invalid negative", "-1", false},
		{"invalid too high", "65536", false},
		{"invalid way too high", "100000", false},
		{"invalid empty string", "", false},
		{"invalid non-numeric", "abc", false},
		{"invalid float", "80.5", false},
		{"invalid with spaces", " 80 ", false},
		{"invalid mixed", "80abc", false},
		{"invalid hex", "0x50", false},
		{"invalid with leading zeros that parse ok", "0080", true}, // strconv.Atoi handles leading zeros
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validatePort(tt.portStr)
			if got != tt.want {
				t.Errorf("validatePort(%q) = %v, want %v", tt.portStr, got, tt.want)
			}
		})
	}
}

func TestDefaultPortForNodeType(t *testing.T) {
	tests := []struct {
		name     string
		nodeType string
		want     string
	}{
		{"PVE node", "pve", "8006"},
		{"PMG node", "pmg", "8006"},
		{"PBS node", "pbs", "8007"},
		{"docker node returns empty", "docker", ""},
		{"unknown type returns empty", "unknown", ""},
		{"empty type returns empty", "", ""},
		{"uppercase PVE returns empty", "PVE", ""},     // case sensitive
		{"mixed case returns empty", "Pve", ""},        // case sensitive
		{"similar but wrong returns empty", "pvee", ""}, // exact match only
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultPortForNodeType(tt.nodeType)
			if got != tt.want {
				t.Errorf("defaultPortForNodeType(%q) = %q, want %q", tt.nodeType, got, tt.want)
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
		wantErr  bool
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
		// Error cases
		{
			name:     "empty host returns error",
			rawHost:  "",
			nodeType: "pve",
			wantErr:  true,
		},
		{
			name:     "whitespace-only host returns error",
			rawHost:  "   ",
			nodeType: "pve",
			wantErr:  true,
		},
		{
			name:     "control character in host returns error",
			rawHost:  "\x00invalid",
			nodeType: "pve",
			wantErr:  true,
		},
		// Unknown node type (no default port added)
		{
			name:     "unknown node type keeps host without port",
			rawHost:  "https://example.com",
			nodeType: "unknown",
			want:     "https://example.com",
		},
		{
			name:     "docker node type keeps host without port",
			rawHost:  "https://docker.lan",
			nodeType: "docker",
			want:     "https://docker.lan",
		},
		// IPv4 addresses
		{
			name:     "IPv4 address gets default port",
			rawHost:  "192.168.1.100",
			nodeType: "pve",
			want:     "https://192.168.1.100:8006",
		},
		{
			name:     "IPv4 with custom port preserved",
			rawHost:  "192.168.1.100:9000",
			nodeType: "pve",
			want:     "https://192.168.1.100:9000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeNodeHost(tt.rawHost, tt.nodeType)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("normalizeNodeHost(%q, %q) expected error, got %q", tt.rawHost, tt.nodeType, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeNodeHost(%q, %q) returned error: %v", tt.rawHost, tt.nodeType, err)
			}
			if got != tt.want {
				t.Fatalf("normalizeNodeHost(%q, %q) = %q, want %q", tt.rawHost, tt.nodeType, got, tt.want)
			}
		})
	}
}

func TestGenerateNodeID(t *testing.T) {
	tests := []struct {
		name     string
		nodeType string
		index    int
		want     string
	}{
		// Standard node types
		{"pve node index 0", "pve", 0, "pve-0"},
		{"pve node index 1", "pve", 1, "pve-1"},
		{"pbs node index 0", "pbs", 0, "pbs-0"},
		{"pmg node index 5", "pmg", 5, "pmg-5"},
		{"docker node index 2", "docker", 2, "docker-2"},

		// Edge cases
		{"empty node type", "", 0, "-0"},
		{"negative index", "pve", -1, "pve--1"},
		{"large index", "pve", 999, "pve-999"},
		{"very large index", "pve", 2147483647, "pve-2147483647"},

		// Custom/unknown node types
		{"custom type", "custom-type", 3, "custom-type-3"},
		{"type with spaces", "my node", 1, "my node-1"},
		{"type with special chars", "node_v2", 0, "node_v2-0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateNodeID(tt.nodeType, tt.index)
			if got != tt.want {
				t.Errorf("generateNodeID(%q, %d) = %q, want %q", tt.nodeType, tt.index, got, tt.want)
			}
		})
	}
}
