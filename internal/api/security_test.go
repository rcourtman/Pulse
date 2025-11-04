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
