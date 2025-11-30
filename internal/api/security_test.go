package api

import (
	"net/http/httptest"
	"sync"
	"testing"
)

func resetTrustedProxyConfig() {
	trustedProxyCIDRs = nil
	trustedProxyOnce = sync.Once{}
}

func TestGetClientIPRejectsSpoofedLoopback(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "198.51.100.42:54321"
	req.Header.Set("X-Forwarded-For", "127.0.0.1")

	if got := GetClientIP(req); got != "198.51.100.42" {
		t.Fatalf("expected remote IP when proxy is untrusted, got %q", got)
	}
}

func TestGetClientIPUsesForwardedForTrustedProxy(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "127.0.0.1/32")
	resetTrustedProxyConfig()

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-For", "203.0.113.44")

	if got := GetClientIP(req); got != "203.0.113.44" {
		t.Fatalf("expected forwarded IP for trusted proxy, got %q", got)
	}
}

func TestIsPrivateIP(t *testing.T) {
	t.Helper()
	cases := []struct {
		name string
		ip   string
		want bool
	}{
		{"public IPv4", "198.51.100.42", false},
		{"private IPv4", "10.1.2.3", true},
		{"loopback IPv4", "127.0.0.1", true},
		{"link-local IPv6", "fe80::1", true},
		{"loopback IPv6 with port", "[::1]:8443", true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := isPrivateIP(tc.ip); got != tc.want {
				t.Fatalf("isPrivateIP(%q) = %v, want %v", tc.ip, got, tc.want)
			}
		})
	}
}

func TestIsTrustedNetwork(t *testing.T) {
	t.Helper()

	if !isTrustedNetwork("10.0.0.5", nil) {
		t.Fatal("expected private IP to be trusted when no networks configured")
	}

	if isTrustedNetwork("198.51.100.42", nil) {
		t.Fatal("expected public IP to be untrusted when no networks configured")
	}

	custom := []string{"203.0.113.0/24"}
	if !isTrustedNetwork("203.0.113.44:8080", custom) {
		t.Fatal("expected IP within custom CIDR to be trusted")
	}

	if isTrustedNetwork("198.51.100.42", custom) {
		t.Fatal("expected IP outside custom CIDR to be untrusted")
	}
}

func TestExtractRemoteIP(t *testing.T) {
	cases := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		// Empty input
		{"empty string", "", ""},

		// IPv4 with port
		{"IPv4 with port", "192.168.1.100:54321", "192.168.1.100"},
		{"localhost with port", "127.0.0.1:8080", "127.0.0.1"},
		{"public IP with port", "203.0.113.44:443", "203.0.113.44"},

		// IPv4 without port
		{"IPv4 without port", "192.168.1.100", "192.168.1.100"},
		{"localhost without port", "127.0.0.1", "127.0.0.1"},

		// IPv6 with port (bracketed)
		{"IPv6 loopback with port", "[::1]:8080", "::1"},
		{"IPv6 full with port", "[2001:db8::1]:443", "2001:db8::1"},
		{"IPv6 link-local with port", "[fe80::1]:8080", "fe80::1"},

		// IPv6 without port (bracketed)
		{"IPv6 loopback bracketed", "[::1]", "::1"},
		{"IPv6 full bracketed", "[2001:db8::1]", "2001:db8::1"},

		// IPv6 without brackets (raw)
		{"IPv6 loopback raw", "::1", "::1"},
		{"IPv6 full raw", "2001:db8::1", "2001:db8::1"},

		// Edge cases
		{"port only", ":8080", ""},
		{"brackets only", "[]", ""},
		{"whitespace", "  ", "  "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractRemoteIP(tc.remoteAddr)
			if got != tc.want {
				t.Errorf("extractRemoteIP(%q) = %q, want %q", tc.remoteAddr, got, tc.want)
			}
		})
	}
}

func TestFirstValidForwardedIP(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   string
	}{
		// Empty input
		{"empty string", "", ""},
		{"whitespace only", "   ", ""},

		// Single IP
		{"single IPv4", "192.168.1.100", "192.168.1.100"},
		{"single IPv4 with whitespace", "  192.168.1.100  ", "192.168.1.100"},
		{"single IPv6", "2001:db8::1", "2001:db8::1"},
		{"single IPv6 bracketed", "[2001:db8::1]", "2001:db8::1"},
		{"single IPv6 loopback", "::1", "::1"},

		// Multiple IPs (comma-separated)
		{"two IPs first valid", "192.168.1.100, 10.0.0.1", "192.168.1.100"},
		{"two IPs with spaces", "  192.168.1.100  ,  10.0.0.1  ", "192.168.1.100"},
		{"three IPs", "203.0.113.1, 10.0.0.1, 172.16.0.1", "203.0.113.1"},

		// Invalid first, valid second
		{"invalid first then valid", "not-an-ip, 192.168.1.100", "192.168.1.100"},
		{"empty first then valid", ", 192.168.1.100", "192.168.1.100"},
		{"garbage then valid", "garbage, foobar, 10.0.0.1", "10.0.0.1"},

		// All invalid
		{"all invalid", "not-an-ip, also-invalid", ""},
		{"hostnames not IPs", "example.com, localhost", ""},

		// Mixed IPv4 and IPv6
		{"IPv6 first then IPv4", "2001:db8::1, 192.168.1.1", "2001:db8::1"},
		{"IPv4 first then IPv6", "192.168.1.1, 2001:db8::1", "192.168.1.1"},

		// Edge cases
		{"IP with port rejected", "192.168.1.100:8080", ""},
		{"bracketed IPv6 with port rejected", "[2001:db8::1]:443", ""},
		{"multiple commas", "192.168.1.1,,,10.0.0.1", "192.168.1.1"},
		{"only commas", ",,,", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := firstValidForwardedIP(tc.header)
			if got != tc.want {
				t.Errorf("firstValidForwardedIP(%q) = %q, want %q", tc.header, got, tc.want)
			}
		})
	}
}

func TestIsPrivateIPExtended(t *testing.T) {
	// Extended test cases beyond the basic ones in TestIsPrivateIP
	cases := []struct {
		name string
		ip   string
		want bool
	}{
		// RFC 1918 private ranges
		{"10.x.x.x range start", "10.0.0.0", true},
		{"10.x.x.x range middle", "10.128.64.32", true},
		{"10.x.x.x range end", "10.255.255.255", true},
		{"172.16-31.x.x start", "172.16.0.0", true},
		{"172.16-31.x.x middle", "172.24.128.64", true},
		{"172.16-31.x.x end", "172.31.255.255", true},
		{"172.15.x.x outside range", "172.15.255.255", false},
		{"172.32.x.x outside range", "172.32.0.0", false},
		{"192.168.x.x start", "192.168.0.0", true},
		{"192.168.x.x end", "192.168.255.255", true},
		{"192.169.x.x outside range", "192.169.0.0", false},

		// Loopback
		{"loopback start", "127.0.0.0", true},
		{"loopback middle", "127.128.64.32", true},
		{"loopback end", "127.255.255.255", true},

		// IPv6 private/local
		{"IPv6 loopback", "::1", true},
		{"IPv6 unique local fc00::/7 start", "fc00::1", true},
		{"IPv6 unique local fd00::", "fd00::1234", true},
		{"IPv6 link-local fe80::/10", "fe80::abcd:1234", true},

		// Public IPs
		{"Google DNS", "8.8.8.8", false},
		{"Cloudflare DNS", "1.1.1.1", false},
		{"documentation range 192.0.2.x", "192.0.2.1", false},
		{"documentation range 198.51.100.x", "198.51.100.1", false},
		{"documentation range 203.0.113.x", "203.0.113.1", false},
		{"IPv6 public", "2001:4860:4860::8888", false},

		// With ports
		{"private with port", "192.168.1.1:8080", true},
		{"public with port", "8.8.8.8:53", false},
		{"IPv6 loopback with port", "[::1]:443", true},
		{"IPv6 public with port", "[2001:4860:4860::8888]:443", false},

		// Invalid inputs
		{"empty string", "", false},
		{"invalid IP", "not-an-ip", false},
		{"hostname", "example.com", false},
		{"partial IP", "192.168", false},
		{"IPv4 with extra octet", "192.168.1.1.1", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isPrivateIP(tc.ip)
			if got != tc.want {
				t.Errorf("isPrivateIP(%q) = %v, want %v", tc.ip, got, tc.want)
			}
		})
	}
}
