package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fixedTimeForTest returns a fixed time for deterministic testing
func fixedTimeForTest() time.Time {
	return time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
}

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

func TestGetClientIPEmptyRemoteAddr(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "" // Empty remote addr

	if got := GetClientIP(req); got != "" {
		t.Fatalf("expected empty string for empty RemoteAddr, got %q", got)
	}
}

func TestGetClientIPUsesXRealIPTrustedProxy(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "127.0.0.1/32")
	resetTrustedProxyConfig()

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	// No X-Forwarded-For, but has X-Real-IP
	req.Header.Set("X-Real-IP", "203.0.113.55")

	if got := GetClientIP(req); got != "203.0.113.55" {
		t.Fatalf("expected X-Real-IP for trusted proxy, got %q", got)
	}
}

func TestIsTrustedProxyIP(t *testing.T) {
	tests := []struct {
		name    string
		envCIDR string
		ipStr   string
		want    bool
	}{
		{
			name:    "empty string returns false",
			envCIDR: "127.0.0.1/32",
			ipStr:   "",
			want:    false,
		},
		{
			name:    "invalid IP returns false",
			envCIDR: "127.0.0.1/32",
			ipStr:   "not-an-ip",
			want:    false,
		},
		{
			name:    "IP not in CIDR range returns false",
			envCIDR: "10.0.0.0/8",
			ipStr:   "192.168.1.1",
			want:    false,
		},
		{
			name:    "IP in CIDR range returns true",
			envCIDR: "10.0.0.0/8",
			ipStr:   "10.1.2.3",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", tt.envCIDR)
			resetTrustedProxyConfig()

			if got := isTrustedProxyIP(tt.ipStr); got != tt.want {
				t.Errorf("isTrustedProxyIP(%q) = %v, want %v", tt.ipStr, got, tt.want)
			}
		})
	}
}

func TestIsPrivateIP(t *testing.T) {
	cases := []struct {
		name string
		ip   string
		want bool
	}{
		// Public IPs
		{"public IPv4", "198.51.100.42", false},
		{"public IPv4 Google DNS", "8.8.8.8", false},
		{"public IPv6", "2001:4860:4860::8888", false},

		// Private IPv4 ranges (RFC1918)
		{"private IPv4 10.x.x.x", "10.1.2.3", true},
		{"private IPv4 10.0.0.0", "10.0.0.0", true},
		{"private IPv4 10.255.255.255", "10.255.255.255", true},
		{"private IPv4 172.16.x.x", "172.16.0.1", true},
		{"private IPv4 172.31.x.x", "172.31.255.255", true},
		{"private IPv4 192.168.x.x", "192.168.1.100", true},

		// Loopback
		{"loopback IPv4", "127.0.0.1", true},
		{"loopback IPv4 127.0.0.0", "127.0.0.0", true},
		{"loopback IPv4 127.255.255.255", "127.255.255.255", true},
		{"loopback IPv6", "::1", true},
		{"loopback IPv6 with port", "[::1]:8443", true},

		// Link-local
		{"link-local IPv4", "169.254.1.1", true},
		{"link-local IPv6", "fe80::1", true},

		// Link-local multicast
		{"link-local multicast IPv4", "224.0.0.1", true},
		{"link-local multicast IPv6", "ff02::1", true},

		// Unique local IPv6 (fc00::/7)
		{"unique local IPv6 fc00", "fc00::1", true},
		{"unique local IPv6 fd00", "fd00::1", true},

		// Edge cases - empty/invalid
		{"empty string", "", false},
		{"invalid IP", "not-an-ip", false},
		{"invalid format", "999.999.999.999", false},

		// With port numbers
		{"private IPv4 with port", "192.168.1.1:8080", true},
		{"public IPv4 with port", "8.8.8.8:53", false},
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
	t.Parallel()

	tests := []struct {
		name            string
		ip              string
		trustedNetworks []string
		expected        bool
	}{
		// Nil trusted networks - falls back to private IP check
		{
			name:            "private IP trusted when no networks configured",
			ip:              "10.0.0.5",
			trustedNetworks: nil,
			expected:        true,
		},
		{
			name:            "public IP untrusted when no networks configured",
			ip:              "198.51.100.42",
			trustedNetworks: nil,
			expected:        false,
		},
		// Empty trusted networks slice - falls back to private IP check
		{
			name:            "private IP trusted with empty networks",
			ip:              "192.168.1.1",
			trustedNetworks: []string{},
			expected:        true,
		},
		// Custom CIDR networks
		{
			name:            "IP within custom CIDR trusted",
			ip:              "203.0.113.44:8080",
			trustedNetworks: []string{"203.0.113.0/24"},
			expected:        true,
		},
		{
			name:            "IP outside custom CIDR untrusted",
			ip:              "198.51.100.42",
			trustedNetworks: []string{"203.0.113.0/24"},
			expected:        false,
		},
		// Edge cases - empty/invalid input
		{
			name:            "empty IP string returns false",
			ip:              "",
			trustedNetworks: []string{"10.0.0.0/8"},
			expected:        false,
		},
		{
			name:            "invalid IP returns false",
			ip:              "not-an-ip",
			trustedNetworks: []string{"10.0.0.0/8"},
			expected:        false,
		},
		// Invalid CIDR is skipped, not matched
		{
			name:            "invalid CIDR in list is skipped",
			ip:              "10.0.0.5",
			trustedNetworks: []string{"invalid-cidr", "10.0.0.0/8"},
			expected:        true,
		},
		{
			name:            "only invalid CIDRs returns false",
			ip:              "10.0.0.5",
			trustedNetworks: []string{"invalid-cidr", "also-invalid"},
			expected:        false,
		},
		// Whitespace handling in CIDRs
		{
			name:            "CIDR with whitespace is trimmed",
			ip:              "10.0.0.5",
			trustedNetworks: []string{"  10.0.0.0/8  "},
			expected:        true,
		},
		// Multiple valid CIDRs
		{
			name:            "IP matches second CIDR",
			ip:              "172.16.5.10",
			trustedNetworks: []string{"10.0.0.0/8", "172.16.0.0/12"},
			expected:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isTrustedNetwork(tt.ip, tt.trustedNetworks)
			if result != tt.expected {
				t.Errorf("isTrustedNetwork(%q, %v) = %v, want %v", tt.ip, tt.trustedNetworks, result, tt.expected)
			}
		})
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

// resetFailedLogins clears the failed login state for testing
func resetFailedLogins() {
	failedMu.Lock()
	defer failedMu.Unlock()
	failedLogins = make(map[string]*FailedLogin)
}

// resetSessionTracking clears session tracking state for testing
func resetSessionTracking() {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	allSessions = make(map[string][]string)
}

func TestRecordFailedLogin(t *testing.T) {
	resetFailedLogins()

	t.Run("increments count on each failure", func(t *testing.T) {
		resetFailedLogins()
		identifier := "test-user-1"

		RecordFailedLogin(identifier)
		attempts, _, _ := GetLockoutInfo(identifier)
		if attempts != 1 {
			t.Errorf("attempts = %d, want 1", attempts)
		}

		RecordFailedLogin(identifier)
		attempts, _, _ = GetLockoutInfo(identifier)
		if attempts != 2 {
			t.Errorf("attempts = %d, want 2", attempts)
		}

		RecordFailedLogin(identifier)
		attempts, _, _ = GetLockoutInfo(identifier)
		if attempts != 3 {
			t.Errorf("attempts = %d, want 3", attempts)
		}
	})

	t.Run("triggers lockout at max attempts", func(t *testing.T) {
		resetFailedLogins()
		identifier := "test-user-2"

		// Record up to max failed attempts
		for i := 0; i < maxFailedAttempts; i++ {
			RecordFailedLogin(identifier)
		}

		attempts, lockedUntil, isLocked := GetLockoutInfo(identifier)
		if attempts != maxFailedAttempts {
			t.Errorf("attempts = %d, want %d", attempts, maxFailedAttempts)
		}
		if !isLocked {
			t.Error("expected isLocked = true")
		}
		if lockedUntil.IsZero() {
			t.Error("expected lockedUntil to be set")
		}
	})

	t.Run("independent identifiers", func(t *testing.T) {
		resetFailedLogins()
		identifier1 := "user-a"
		identifier2 := "user-b"

		RecordFailedLogin(identifier1)
		RecordFailedLogin(identifier1)
		RecordFailedLogin(identifier2)

		attempts1, _, _ := GetLockoutInfo(identifier1)
		attempts2, _, _ := GetLockoutInfo(identifier2)

		if attempts1 != 2 {
			t.Errorf("identifier1 attempts = %d, want 2", attempts1)
		}
		if attempts2 != 1 {
			t.Errorf("identifier2 attempts = %d, want 1", attempts2)
		}
	})
}

func TestClearFailedLogins(t *testing.T) {
	resetFailedLogins()

	t.Run("clears failed login count", func(t *testing.T) {
		resetFailedLogins()
		identifier := "test-user-clear"

		RecordFailedLogin(identifier)
		RecordFailedLogin(identifier)

		attempts, _, _ := GetLockoutInfo(identifier)
		if attempts != 2 {
			t.Errorf("attempts before clear = %d, want 2", attempts)
		}

		ClearFailedLogins(identifier)

		attempts, _, _ = GetLockoutInfo(identifier)
		if attempts != 0 {
			t.Errorf("attempts after clear = %d, want 0", attempts)
		}
	})

	t.Run("clearing nonexistent identifier does not panic", func(t *testing.T) {
		resetFailedLogins()
		ClearFailedLogins("nonexistent-user")
		// Should not panic
	})

	t.Run("clears lockout state", func(t *testing.T) {
		resetFailedLogins()
		identifier := "locked-user"

		// Lock the account
		for i := 0; i < maxFailedAttempts; i++ {
			RecordFailedLogin(identifier)
		}

		_, _, isLocked := GetLockoutInfo(identifier)
		if !isLocked {
			t.Error("expected account to be locked before clear")
		}

		ClearFailedLogins(identifier)

		_, _, isLocked = GetLockoutInfo(identifier)
		if isLocked {
			t.Error("expected account to not be locked after clear")
		}
	})
}

func TestGetLockoutInfo(t *testing.T) {
	resetFailedLogins()

	t.Run("nonexistent identifier returns zeros", func(t *testing.T) {
		resetFailedLogins()
		attempts, lockedUntil, isLocked := GetLockoutInfo("unknown-user")
		if attempts != 0 {
			t.Errorf("attempts = %d, want 0", attempts)
		}
		if !lockedUntil.IsZero() {
			t.Errorf("lockedUntil = %v, want zero time", lockedUntil)
		}
		if isLocked {
			t.Error("expected isLocked = false")
		}
	})

	t.Run("returns correct attempts below lockout", func(t *testing.T) {
		resetFailedLogins()
		identifier := "partial-user"

		RecordFailedLogin(identifier)
		RecordFailedLogin(identifier)

		attempts, _, isLocked := GetLockoutInfo(identifier)
		if attempts != 2 {
			t.Errorf("attempts = %d, want 2", attempts)
		}
		if isLocked {
			t.Error("expected isLocked = false for attempts below max")
		}
	})

	t.Run("isLocked true only when attempts >= max and within lockout period", func(t *testing.T) {
		resetFailedLogins()
		identifier := "locked-user-test"

		// Record max attempts to trigger lockout
		for i := 0; i < maxFailedAttempts; i++ {
			RecordFailedLogin(identifier)
		}

		attempts, lockedUntil, isLocked := GetLockoutInfo(identifier)
		if attempts != maxFailedAttempts {
			t.Errorf("attempts = %d, want %d", attempts, maxFailedAttempts)
		}
		if !isLocked {
			t.Error("expected isLocked = true")
		}
		if lockedUntil.IsZero() {
			t.Error("expected lockedUntil to be set")
		}
	})

	t.Run("expired lockout returns zeros", func(t *testing.T) {
		resetFailedLogins()
		identifier := "expired-lockout-user"

		// Directly set an expired lockout entry
		failedMu.Lock()
		failedLogins[identifier] = &FailedLogin{
			Count:       maxFailedAttempts,
			LastAttempt: time.Now().Add(-time.Hour),
			LockedUntil: time.Now().Add(-time.Minute), // Expired
		}
		failedMu.Unlock()

		attempts, lockedUntil, isLocked := GetLockoutInfo(identifier)
		if attempts != 0 {
			t.Errorf("attempts = %d, want 0 for expired lockout", attempts)
		}
		if !lockedUntil.IsZero() {
			t.Errorf("lockedUntil = %v, want zero time for expired lockout", lockedUntil)
		}
		if isLocked {
			t.Error("expected isLocked = false for expired lockout")
		}
	})
}

func TestResetLockout(t *testing.T) {
	resetFailedLogins()

	t.Run("resets lockout state", func(t *testing.T) {
		resetFailedLogins()
		identifier := "admin-reset-test"

		// Lock the account
		for i := 0; i < maxFailedAttempts; i++ {
			RecordFailedLogin(identifier)
		}

		_, _, isLocked := GetLockoutInfo(identifier)
		if !isLocked {
			t.Error("expected account to be locked before reset")
		}

		ResetLockout(identifier)

		attempts, _, isLocked := GetLockoutInfo(identifier)
		if isLocked {
			t.Error("expected account to not be locked after reset")
		}
		if attempts != 0 {
			t.Errorf("attempts = %d, want 0 after reset", attempts)
		}
	})

	t.Run("resetting nonexistent identifier does not panic", func(t *testing.T) {
		resetFailedLogins()
		ResetLockout("nonexistent-admin-user")
		// Should not panic
	})
}

func TestTrackUserSession(t *testing.T) {
	resetSessionTracking()

	t.Run("tracks new user session", func(t *testing.T) {
		resetSessionTracking()
		TrackUserSession("alice", "session-1")

		username := GetSessionUsername("session-1")
		if username != "alice" {
			t.Errorf("username = %q, want alice", username)
		}
	})

	t.Run("tracks multiple sessions for same user", func(t *testing.T) {
		resetSessionTracking()
		TrackUserSession("bob", "session-a")
		TrackUserSession("bob", "session-b")
		TrackUserSession("bob", "session-c")

		if GetSessionUsername("session-a") != "bob" {
			t.Error("session-a should belong to bob")
		}
		if GetSessionUsername("session-b") != "bob" {
			t.Error("session-b should belong to bob")
		}
		if GetSessionUsername("session-c") != "bob" {
			t.Error("session-c should belong to bob")
		}
	})

	t.Run("tracks sessions for multiple users", func(t *testing.T) {
		resetSessionTracking()
		TrackUserSession("user1", "sess-1")
		TrackUserSession("user2", "sess-2")
		TrackUserSession("user3", "sess-3")

		if GetSessionUsername("sess-1") != "user1" {
			t.Error("sess-1 should belong to user1")
		}
		if GetSessionUsername("sess-2") != "user2" {
			t.Error("sess-2 should belong to user2")
		}
		if GetSessionUsername("sess-3") != "user3" {
			t.Error("sess-3 should belong to user3")
		}
	})
}

func TestGetSessionUsername(t *testing.T) {
	resetSessionTracking()

	t.Run("returns empty for unknown session", func(t *testing.T) {
		resetSessionTracking()
		username := GetSessionUsername("unknown-session")
		if username != "" {
			t.Errorf("username = %q, want empty string", username)
		}
	})

	t.Run("returns correct username for tracked session", func(t *testing.T) {
		resetSessionTracking()
		TrackUserSession("testuser", "test-session-id")

		username := GetSessionUsername("test-session-id")
		if username != "testuser" {
			t.Errorf("username = %q, want testuser", username)
		}
	})

	t.Run("handles multiple users and sessions", func(t *testing.T) {
		resetSessionTracking()
		TrackUserSession("alice", "alice-session-1")
		TrackUserSession("alice", "alice-session-2")
		TrackUserSession("bob", "bob-session-1")

		if GetSessionUsername("alice-session-1") != "alice" {
			t.Error("alice-session-1 should belong to alice")
		}
		if GetSessionUsername("alice-session-2") != "alice" {
			t.Error("alice-session-2 should belong to alice")
		}
		if GetSessionUsername("bob-session-1") != "bob" {
			t.Error("bob-session-1 should belong to bob")
		}
		if GetSessionUsername("unknown") != "" {
			t.Error("unknown session should return empty string")
		}
	})
}

func TestClearCSRFCookie(t *testing.T) {
	t.Run("nil writer does not panic", func(t *testing.T) {
		clearCSRFCookie(nil)
		// Should not panic
	})

	t.Run("sets cookie with maxage -1", func(t *testing.T) {
		w := httptest.NewRecorder()
		clearCSRFCookie(w)

		cookies := w.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("expected 1 cookie, got %d", len(cookies))
		}

		cookie := cookies[0]
		if cookie.Name != "pulse_csrf" {
			t.Errorf("cookie name = %q, want pulse_csrf", cookie.Name)
		}
		if cookie.Value != "" {
			t.Errorf("cookie value = %q, want empty string", cookie.Value)
		}
		if cookie.MaxAge != -1 {
			t.Errorf("cookie MaxAge = %d, want -1", cookie.MaxAge)
		}
	})
}

func TestIssueNewCSRFCookie(t *testing.T) {
	t.Run("nil writer returns empty string", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/test", nil)
		token := issueNewCSRFCookie(nil, req, "session-id")
		if token != "" {
			t.Errorf("token = %q, want empty string", token)
		}
	})

	t.Run("nil request returns empty string", func(t *testing.T) {
		w := httptest.NewRecorder()
		token := issueNewCSRFCookie(w, nil, "session-id")
		if token != "" {
			t.Errorf("token = %q, want empty string", token)
		}
	})

	t.Run("empty session ID returns empty string", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/test", nil)
		token := issueNewCSRFCookie(w, req, "")
		if token != "" {
			t.Errorf("token = %q, want empty string for empty session", token)
		}
	})

	t.Run("whitespace only session ID returns empty string", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/test", nil)
		token := issueNewCSRFCookie(w, req, "   ")
		if token != "" {
			t.Errorf("token = %q, want empty string for whitespace session", token)
		}
	})

	t.Run("valid session returns non-empty token", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/test", nil)
		token := issueNewCSRFCookie(w, req, "valid-session-id")
		if token == "" {
			t.Error("expected non-empty token for valid session")
		}

		// Check that a cookie was set
		cookies := w.Result().Cookies()
		found := false
		for _, c := range cookies {
			if c.Name == "pulse_csrf" && c.Value == token {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected pulse_csrf cookie to be set with the token value")
		}
	})
}

func TestFailedLogin_Fields(t *testing.T) {
	fl := FailedLogin{
		Count:       3,
		LastAttempt: fixedTimeForTest(),
		LockedUntil: fixedTimeForTest().Add(15 * 60 * 1000000000),
	}

	if fl.Count != 3 {
		t.Errorf("Count = %d, want 3", fl.Count)
	}
	if fl.LastAttempt.IsZero() {
		t.Error("LastAttempt should not be zero")
	}
	if fl.LockedUntil.IsZero() {
		t.Error("LockedUntil should not be zero")
	}
}

func TestAuditEvent_Fields(t *testing.T) {
	ae := AuditEvent{
		Timestamp: fixedTimeForTest(),
		Event:     "login_attempt",
		User:      "admin",
		IP:        "192.168.1.100",
		Path:      "/api/auth/login",
		Success:   true,
		Details:   "successful login",
	}

	if ae.Event != "login_attempt" {
		t.Errorf("Event = %q, want login_attempt", ae.Event)
	}
	if ae.User != "admin" {
		t.Errorf("User = %q, want admin", ae.User)
	}
	if ae.IP != "192.168.1.100" {
		t.Errorf("IP = %q, want 192.168.1.100", ae.IP)
	}
	if ae.Path != "/api/auth/login" {
		t.Errorf("Path = %q, want /api/auth/login", ae.Path)
	}
	if !ae.Success {
		t.Error("Success should be true")
	}
	if ae.Details != "successful login" {
		t.Errorf("Details = %q, want 'successful login'", ae.Details)
	}
}

func TestSecurityHeadersWithConfig_EmbeddingDisabled(t *testing.T) {
	handler := SecurityHeadersWithConfig(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		false, // allowEmbedding
		"",    // allowedOrigins
	)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Check X-Frame-Options is set to DENY when embedding is disabled
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", got)
	}

	// Check CSP has frame-ancestors 'none'
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("CSP should contain frame-ancestors 'none', got: %s", csp)
	}

	// Check other security headers are present
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("X-XSS-Protection"); got != "1; mode=block" {
		t.Errorf("X-XSS-Protection = %q, want '1; mode=block'", got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Errorf("Referrer-Policy = %q, want strict-origin-when-cross-origin", got)
	}
}

func TestSecurityHeadersWithConfig_EmbeddingEnabledNoOrigins(t *testing.T) {
	handler := SecurityHeadersWithConfig(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		true, // allowEmbedding
		"",   // allowedOrigins - empty means allow all
	)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// X-Frame-Options should NOT be set when embedding is allowed
	if got := rec.Header().Get("X-Frame-Options"); got != "" {
		t.Errorf("X-Frame-Options = %q, want empty (not set)", got)
	}

	// Check CSP has frame-ancestors * (allow any)
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors *") {
		t.Errorf("CSP should contain 'frame-ancestors *', got: %s", csp)
	}
}

func TestSecurityHeadersWithConfig_EmbeddingEnabledWithOrigins(t *testing.T) {
	handler := SecurityHeadersWithConfig(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		true,                                     // allowEmbedding
		"https://example.com, https://other.com", // allowedOrigins
	)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// X-Frame-Options should NOT be set when embedding is allowed
	if got := rec.Header().Get("X-Frame-Options"); got != "" {
		t.Errorf("X-Frame-Options = %q, want empty (not set)", got)
	}

	// Check CSP has frame-ancestors with specific origins
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors 'self' https://example.com https://other.com") {
		t.Errorf("CSP should contain specific frame-ancestors, got: %s", csp)
	}
}

func TestSecurityHeadersWithConfig_EmbeddingWithEmptyOriginEntries(t *testing.T) {
	// Test handling of origins with empty entries (e.g., trailing comma)
	handler := SecurityHeadersWithConfig(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
		true,                       // allowEmbedding
		"https://example.com, , ,", // allowedOrigins with empty entries
	)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Check CSP has frame-ancestors with only non-empty origins
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors 'self' https://example.com") {
		t.Errorf("CSP should contain frame-ancestors with filtered origins, got: %s", csp)
	}
}

func TestSecurityHeadersWithConfig_NextHandlerCalled(t *testing.T) {
	called := false
	handler := SecurityHeadersWithConfig(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}),
		false,
		"",
	)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("next handler was not called")
	}
}

func TestLogAuditEvent_Success(t *testing.T) {
	// Should not panic and should log at Info level
	LogAuditEvent(
		"test_event",
		"testuser",
		"192.168.1.100",
		"/api/test",
		true,
		"test details",
	)
	// If we got here without panic, the test passes
}

func TestLogAuditEvent_Failure(t *testing.T) {
	// Should not panic and should log at Warn level
	LogAuditEvent(
		"failed_login",
		"attacker",
		"203.0.113.42",
		"/api/login",
		false,
		"invalid credentials",
	)
	// If we got here without panic, the test passes
}

func TestLogAuditEvent_EmptyFields(t *testing.T) {
	// Should handle empty strings gracefully
	LogAuditEvent("", "", "", "", true, "")
	LogAuditEvent("", "", "", "", false, "")
	// If we got here without panic, the test passes
}

func TestLoadTrustedProxyCIDRs_InvalidCIDR(t *testing.T) {
	// Test that invalid CIDR is logged and skipped
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "invalid/cidr, 10.0.0.0/8")
	resetTrustedProxyConfig()

	// Trigger loading
	_ = isTrustedProxyIP("10.0.0.1")

	// The valid CIDR should still work
	if !isTrustedProxyIP("10.0.0.1") {
		t.Error("10.0.0.1 should be trusted after loading valid CIDR")
	}
}

func TestLoadTrustedProxyCIDRs_InvalidIP(t *testing.T) {
	// Test that invalid IP (no CIDR notation) is logged and skipped
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "not-an-ip, 192.168.1.1")
	resetTrustedProxyConfig()

	// Trigger loading
	_ = isTrustedProxyIP("192.168.1.1")

	// The valid IP should still work
	if !isTrustedProxyIP("192.168.1.1") {
		t.Error("192.168.1.1 should be trusted after loading valid IP")
	}
}

func TestLoadTrustedProxyCIDRs_IPv6(t *testing.T) {
	// Test IPv6 address handling (uses 128 bits for mask)
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "::1, 2001:db8::1")
	resetTrustedProxyConfig()

	// Trigger loading
	_ = isTrustedProxyIP("::1")

	// IPv6 addresses should be trusted
	if !isTrustedProxyIP("::1") {
		t.Error("::1 should be trusted")
	}
	if !isTrustedProxyIP("2001:db8::1") {
		t.Error("2001:db8::1 should be trusted")
	}
}

func TestLoadTrustedProxyCIDRs_EmptyEntries(t *testing.T) {
	// Test that empty entries in the list are skipped
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "10.0.0.0/8, , ,  , 192.168.0.0/16")
	resetTrustedProxyConfig()

	// Trigger loading
	_ = isTrustedProxyIP("10.0.0.1")

	// Both valid CIDRs should work
	if !isTrustedProxyIP("10.0.0.1") {
		t.Error("10.0.0.1 should be trusted")
	}
	if !isTrustedProxyIP("192.168.1.1") {
		t.Error("192.168.1.1 should be trusted")
	}
}

func TestLoadTrustedProxyCIDRs_MixedValidInvalid(t *testing.T) {
	// Test mix of valid CIDRs, valid IPs, invalid CIDRs, and invalid IPs
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "10.0.0.0/8, bad-cidr/99, 172.16.0.1, not-valid, ::1")
	resetTrustedProxyConfig()

	// Trigger loading
	_ = isTrustedProxyIP("10.0.0.1")

	// Valid entries should work
	if !isTrustedProxyIP("10.0.0.1") {
		t.Error("10.0.0.1 should be trusted (from valid CIDR)")
	}
	if !isTrustedProxyIP("172.16.0.1") {
		t.Error("172.16.0.1 should be trusted (from valid IP)")
	}
	if !isTrustedProxyIP("::1") {
		t.Error("::1 should be trusted (from valid IPv6)")
	}

	// Invalid entries should not cause problems
	if isTrustedProxyIP("192.168.1.1") {
		t.Error("192.168.1.1 should not be trusted (not in any valid entry)")
	}
}

// resetAdminBypassState resets the admin bypass state for testing
func resetAdminBypassState() {
	adminBypassState.once = sync.Once{}
	adminBypassState.enabled = false
	adminBypassState.declined = false
}

func TestAdminBypassEnabled_NotRequested(t *testing.T) {
	// When ALLOW_ADMIN_BYPASS is not set to "1", bypass should be disabled
	t.Setenv("ALLOW_ADMIN_BYPASS", "")
	resetAdminBypassState()

	if adminBypassEnabled() {
		t.Error("adminBypassEnabled() should return false when ALLOW_ADMIN_BYPASS is not '1'")
	}
}

func TestAdminBypassEnabled_WithPulseDev(t *testing.T) {
	// When ALLOW_ADMIN_BYPASS=1 and PULSE_DEV=true, bypass should be enabled
	t.Setenv("ALLOW_ADMIN_BYPASS", "1")
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("NODE_ENV", "")
	resetAdminBypassState()

	if !adminBypassEnabled() {
		t.Error("adminBypassEnabled() should return true when ALLOW_ADMIN_BYPASS=1 and PULSE_DEV=true")
	}
}

func TestAdminBypassEnabled_WithNodeEnvDevelopment(t *testing.T) {
	// When ALLOW_ADMIN_BYPASS=1 and NODE_ENV=development, bypass should be enabled
	t.Setenv("ALLOW_ADMIN_BYPASS", "1")
	t.Setenv("PULSE_DEV", "")
	t.Setenv("NODE_ENV", "development")
	resetAdminBypassState()

	if !adminBypassEnabled() {
		t.Error("adminBypassEnabled() should return true when ALLOW_ADMIN_BYPASS=1 and NODE_ENV=development")
	}
}

func TestAdminBypassEnabled_NodeEnvCaseInsensitive(t *testing.T) {
	// NODE_ENV comparison should be case-insensitive
	t.Setenv("ALLOW_ADMIN_BYPASS", "1")
	t.Setenv("PULSE_DEV", "")
	t.Setenv("NODE_ENV", "DEVELOPMENT")
	resetAdminBypassState()

	if !adminBypassEnabled() {
		t.Error("adminBypassEnabled() should return true when NODE_ENV=DEVELOPMENT (case-insensitive)")
	}
}

func TestAdminBypassEnabled_DeclinedOutsideDevMode(t *testing.T) {
	// When ALLOW_ADMIN_BYPASS=1 but not in dev mode, bypass should be declined
	t.Setenv("ALLOW_ADMIN_BYPASS", "1")
	t.Setenv("PULSE_DEV", "")
	t.Setenv("NODE_ENV", "production")
	resetAdminBypassState()

	if adminBypassEnabled() {
		t.Error("adminBypassEnabled() should return false when ALLOW_ADMIN_BYPASS=1 but not in dev mode")
	}

	// Verify the declined flag was set
	if !adminBypassState.declined {
		t.Error("adminBypassState.declined should be true when bypass is ignored outside dev mode")
	}
}

func TestCheckCSRF_SafeMethods(t *testing.T) {
	tests := []struct {
		method string
	}{
		{"GET"},
		{"HEAD"},
		{"OPTIONS"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, "/api/test", nil)

			// Safe methods should always return true regardless of CSRF state
			result := CheckCSRF(w, req)
			if !result {
				t.Errorf("CheckCSRF(%s) = false, want true for safe method", tt.method)
			}
		})
	}
}

func TestCheckCSRF_APITokenAuth(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/test", nil)
	req.Header.Set("X-API-Token", "some-api-token")

	// API token auth bypasses CSRF check
	result := CheckCSRF(w, req)
	if !result {
		t.Error("CheckCSRF should return true when X-API-Token is present")
	}
}

func TestCheckCSRF_BasicAuth(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/test", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	// Basic auth bypasses CSRF check
	result := CheckCSRF(w, req)
	if !result {
		t.Error("CheckCSRF should return true when Authorization header is present")
	}
}

func TestCheckCSRF_NoSessionCookie(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/test", nil)
	// No session cookie set

	// Without session cookie, CSRF check is not needed
	result := CheckCSRF(w, req)
	if !result {
		t.Error("CheckCSRF should return true when no session cookie is present")
	}
}

func TestCheckCSRF_MissingCSRFToken(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "pulse_session",
		Value: "test-session-id-1234567890",
	})
	// No CSRF token set

	// Missing CSRF token should fail
	result := CheckCSRF(w, req)
	if result {
		t.Error("CheckCSRF should return false when CSRF token is missing")
	}

	// Should set X-CSRF-Token header with new token
	newToken := w.Header().Get("X-CSRF-Token")
	if newToken == "" {
		t.Error("CheckCSRF should issue new CSRF token in header when missing")
	}
}

func TestCheckCSRF_InvalidCSRFToken(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "pulse_session",
		Value: "test-session-id-1234567890",
	})
	req.Header.Set("X-CSRF-Token", "invalid-csrf-token")

	// Invalid CSRF token should fail
	result := CheckCSRF(w, req)
	if result {
		t.Error("CheckCSRF should return false when CSRF token is invalid")
	}

	// Should set X-CSRF-Token header with new token
	newToken := w.Header().Get("X-CSRF-Token")
	if newToken == "" {
		t.Error("CheckCSRF should issue new CSRF token in header when invalid")
	}
}

func TestCheckCSRF_CSRFTokenFromFormValue(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/test?csrf_token=form-token-value", nil)
	req.AddCookie(&http.Cookie{
		Name:  "pulse_session",
		Value: "test-session-id-1234567890",
	})
	// csrf_token is set as query param which is read by FormValue

	// The token won't validate, but we're testing that FormValue is checked
	result := CheckCSRF(w, req)
	// Will fail because token doesn't match session
	if result {
		t.Error("CheckCSRF should still validate the token from FormValue")
	}
}

func TestCheckCSRF_UnsafeMethods(t *testing.T) {
	methods := []string{"POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/api/test", nil)
			req.AddCookie(&http.Cookie{
				Name:  "pulse_session",
				Value: "test-session-id-1234567890",
			})
			// No CSRF token

			// Unsafe methods without valid CSRF should fail
			result := CheckCSRF(w, req)
			if result {
				t.Errorf("CheckCSRF(%s) should return false without valid CSRF token", method)
			}
		})
	}
}
