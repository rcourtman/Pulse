package api

import (
	"net/http/httptest"
	"testing"
)

// A proxy that appends to X-Forwarded-For keeps whatever the client sent on
// the left, so the client address is the right-most hop that no trusted proxy
// added, never the first entry.
func TestGetClientIPUsesRightMostUntrustedForwardedHop(t *testing.T) {
	cases := []struct {
		name     string
		trusted  string
		remote   string
		xff      []string
		realIP   string
		expected string
	}{
		{
			name:     "appending proxy keeps client-supplied entries on the left",
			trusted:  "127.0.0.1/32",
			remote:   "127.0.0.1:50000",
			xff:      []string{"198.51.100.66, 203.0.113.9"},
			expected: "203.0.113.9",
		},
		{
			name:     "single replaced entry",
			trusted:  "127.0.0.1/32",
			remote:   "127.0.0.1:50000",
			xff:      []string{"203.0.113.9"},
			expected: "203.0.113.9",
		},
		{
			name:     "trusted inner hops are skipped",
			trusted:  "127.0.0.1/32,10.0.0.0/8",
			remote:   "127.0.0.1:50000",
			xff:      []string{"198.51.100.66, 203.0.113.9, 10.0.0.5"},
			expected: "203.0.113.9",
		},
		{
			name:     "untrusted inner hop is the client",
			trusted:  "127.0.0.1/32",
			remote:   "127.0.0.1:50000",
			xff:      []string{"203.0.113.9, 10.0.0.5"},
			expected: "10.0.0.5",
		},
		{
			name:     "all hops trusted returns the left-most",
			trusted:  "127.0.0.1/32,10.0.0.0/8",
			remote:   "127.0.0.1:50000",
			xff:      []string{"10.0.0.7, 10.0.0.5"},
			expected: "10.0.0.7",
		},
		{
			name:     "proxy that adds a separate header line",
			trusted:  "127.0.0.1/32",
			remote:   "127.0.0.1:50000",
			xff:      []string{"198.51.100.66", "203.0.113.9"},
			expected: "203.0.113.9",
		},
		{
			name:     "entry with a port",
			trusted:  "127.0.0.1/32",
			remote:   "127.0.0.1:50000",
			xff:      []string{"198.51.100.66, 203.0.113.9:5555"},
			expected: "203.0.113.9",
		},
		{
			name:     "bracketed IPv6 entry with a port",
			trusted:  "127.0.0.1/32",
			remote:   "127.0.0.1:50000",
			xff:      []string{"198.51.100.66, [2001:db8::9]:443"},
			expected: "2001:db8::9",
		},
		{
			name:     "unreadable hop stops the walk at the proxy",
			trusted:  "127.0.0.1/32",
			remote:   "127.0.0.1:50000",
			xff:      []string{"198.51.100.66, unknown"},
			realIP:   "198.51.100.67",
			expected: "127.0.0.1",
		},
		{
			name:     "empty forwarded-for falls back to X-Real-IP",
			trusted:  "127.0.0.1/32",
			remote:   "127.0.0.1:50000",
			xff:      []string{" , "},
			realIP:   "203.0.113.9",
			expected: "203.0.113.9",
		},
		{
			name:     "untrusted peer ignores forwarded-for",
			trusted:  "127.0.0.1/32",
			remote:   "198.51.100.10:50000",
			xff:      []string{"203.0.113.9"},
			expected: "198.51.100.10",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", tc.trusted)
			resetTrustedProxyConfig()
			t.Cleanup(resetTrustedProxyConfig)

			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tc.remote
			for _, value := range tc.xff {
				req.Header.Add("X-Forwarded-For", value)
			}
			if tc.realIP != "" {
				req.Header.Set("X-Real-IP", tc.realIP)
			}

			if got := GetClientIP(req); got != tc.expected {
				t.Fatalf("GetClientIP() = %q, want %q", got, tc.expected)
			}
		})
	}
}

// Login lockout keys on the client address, so a client behind an appending
// proxy must not escape it by changing the forwarded-for value it sends.
func TestLoginLockoutIgnoresClientSuppliedForwardedEntries(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "127.0.0.1/32")
	resetTrustedProxyConfig()
	t.Cleanup(resetTrustedProxyConfig)

	seen := map[string]bool{}
	for _, spoofed := range []string{"198.51.100.1", "198.51.100.2", "198.51.100.3"} {
		req := httptest.NewRequest("POST", "/api/login", nil)
		req.RemoteAddr = "127.0.0.1:50000"
		req.Header.Set("X-Forwarded-For", spoofed+", 203.0.113.9")
		seen[GetClientIP(req)] = true
	}
	if len(seen) != 1 || !seen["203.0.113.9"] {
		t.Fatalf("client addresses = %v, want only 203.0.113.9", seen)
	}
}
