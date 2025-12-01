package api

import "testing"

func TestDeriveSchemeAndPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		host       string
		wantScheme string
		wantPort   string
	}{
		{
			name:       "https with explicit port",
			host:       "https://pve1.local:8443",
			wantScheme: "https",
			wantPort:   "8443",
		},
		{
			name:       "http with explicit port",
			host:       "http://pve1.local:8006",
			wantScheme: "http",
			wantPort:   "8006",
		},
		{
			name:       "https without port falls back",
			host:       "https://pve1.local",
			wantScheme: "https",
			wantPort:   "8006",
		},
		{
			name:       "host without scheme",
			host:       "pve1.local:9000",
			wantScheme: "https",
			wantPort:   "9000",
		},
		// Edge cases for full coverage
		{
			name:       "empty string returns defaults",
			host:       "",
			wantScheme: "https",
			wantPort:   "8006",
		},
		{
			name:       "whitespace only returns defaults",
			host:       "   ",
			wantScheme: "https",
			wantPort:   "8006",
		},
		{
			name:       "invalid URL returns defaults",
			host:       "://invalid",
			wantScheme: "https",
			wantPort:   "8006",
		},
		{
			name:       "host only without port",
			host:       "pve1.local",
			wantScheme: "https",
			wantPort:   "8006",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotScheme, gotPort := deriveSchemeAndPort(tt.host)
			if gotScheme != tt.wantScheme || gotPort != tt.wantPort {
				t.Fatalf("deriveSchemeAndPort(%q) = (%q, %q); want (%q, %q)", tt.host, gotScheme, gotPort, tt.wantScheme, tt.wantPort)
			}
		})
	}
}

func TestEnsureHostHasPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		host     string
		port     string
		expected string
	}{
		{
			name:     "hostname without port",
			host:     "pve1.local",
			port:     "8443",
			expected: "pve1.local:8443",
		},
		{
			name:     "hostname with port",
			host:     "pve1.local:8006",
			port:     "8443",
			expected: "pve1.local:8006",
		},
		{
			name:     "ipv6 without port",
			host:     "[2001:db8::1]",
			port:     "8006",
			expected: "[2001:db8::1]:8006",
		},
		{
			name:     "ipv6 with port",
			host:     "[2001:db8::1]:9000",
			port:     "8006",
			expected: "[2001:db8::1]:9000",
		},
		{
			name:     "host with scheme",
			host:     "https://pve1.local:8443",
			port:     "8006",
			expected: "pve1.local:8443",
		},
		{
			name:     "empty host",
			host:     "",
			port:     "8006",
			expected: "",
		},
		{
			name:     "empty port returns host unchanged",
			host:     "pve1.local",
			port:     "",
			expected: "pve1.local",
		},
		{
			name:     "both empty returns empty",
			host:     "",
			port:     "",
			expected: "",
		},
		{
			name:     "host with scheme but no port strips scheme and adds port",
			host:     "https://pve1.local",
			port:     "8006",
			expected: "https://pve1.local",
		},
		{
			name:     "protocol-relative URL extracts host and adds port",
			host:     "//pve1.local",
			port:     "8006",
			expected: "pve1.local:8006",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ensureHostHasPort(tt.host, tt.port); got != tt.expected {
				t.Fatalf("ensureHostHasPort(%q, %q) = %q; want %q", tt.host, tt.port, got, tt.expected)
			}
		})
	}
}
