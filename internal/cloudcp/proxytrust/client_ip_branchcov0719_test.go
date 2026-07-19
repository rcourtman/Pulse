package proxytrust

import (
	"net/http"
	"testing"
)

// setupTrustEnv resets the package-level sync.Once and trusted CIDRs, sets both
// the primary and fallback env vars for the duration of the subtest, and
// schedules a final reset so global state never leaks to sibling tests.
//
// loadTrustedProxyCIDRs reads env inside trustedProxyOnce.Do, so the reset MUST
// happen before t.Setenv and before the call under test.
func setupTrustEnv(t *testing.T, primary, fallback string) {
	t.Helper()
	ResetForTesting()
	t.Setenv("CP_TRUSTED_PROXY_CIDRS", primary)
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", fallback)
	t.Cleanup(ResetForTesting)
}

func makeReq(t *testing.T, remote string, headers map[string]string) *http.Request {
	t.Helper()
	h := http.Header{}
	for k, v := range headers {
		h.Set(k, v)
	}
	return &http.Request{RemoteAddr: remote, Header: h}
}

func TestExtractRemoteIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{"ipv4 host port", "1.2.3.4:5678", "1.2.3.4"},
		{"bare ipv4 no port", "1.2.3.4", "1.2.3.4"},
		{"empty string", "", ""},
		{"ipv6 bracketed with port", "[::1]:80", "::1"},
		{"ipv6 bracketed no port", "[::1]", "::1"},
		{"ipv6 bare no port", "::1", "::1"},
		{"whitespace only", "  ", "  "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractRemoteIP(tc.remoteAddr)
			if got != tc.want {
				t.Errorf("ExtractRemoteIP(%q) = %q, want %q", tc.remoteAddr, got, tc.want)
			}
		})
	}
}

func TestIsTrustedProxyIP(t *testing.T) {
	tests := []struct {
		name     string
		primary  string
		fallback string
		rawIP    string
		want     bool
	}{
		{"inside v4 cidr", "10.0.0.0/8", "", "10.5.6.7", true},
		{"outside v4 cidr", "10.0.0.0/8", "", "192.168.1.1", false},
		{"bare v4 exact match", "10.0.0.5", "", "10.0.0.5", true},
		{"bare v4 different host", "10.0.0.5", "", "10.0.0.6", false},
		{"bare v6 exact match", "::1", "", "::1", true},
		{"v6 inside cidr", "fc00::/7", "", "fc00::1234", true},
		{"bracketed raw input stripped", "10.0.0.0/8", "", "[10.0.0.5]", true},
		{"unparseable input", "10.0.0.0/8", "", "not-an-ip", false},
		{"empty input", "10.0.0.0/8", "", "", false},
		{"no config returns false", "", "", "10.0.0.5", false},
		{"invalid cidr entry ignored valid one used", "not-a-cidr,10.0.0.0/8", "", "10.0.0.5", true},
		{"all invalid entries yields false", "not-a-cidr,also-bad", "", "10.0.0.5", false},
		{"fallback env used when primary empty", "", "10.0.0.0/8", "10.0.0.5", true},
		{"multiple cidrs any match", "10.0.0.0/8,192.168.0.0/16", "", "192.168.50.1", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setupTrustEnv(t, tc.primary, tc.fallback)
			got := IsTrustedProxyIP(tc.rawIP)
			if got != tc.want {
				t.Errorf("IsTrustedProxyIP(%q) primary=%q fallback=%q = %v, want %v",
					tc.rawIP, tc.primary, tc.fallback, got, tc.want)
			}
		})
	}
}

func TestRightMostUntrustedForwardedIP(t *testing.T) {
	tests := []struct {
		name     string
		primary  string
		fallback string
		header   string
		want     string
	}{
		{"empty header", "10.0.0.0/8", "", "", ""},
		{"single untrusted hop", "10.0.0.0/8", "", "203.0.113.5", "203.0.113.5"},
		{"single trusted hop returns self", "10.0.0.0/8", "", "10.0.0.1", "10.0.0.1"},
		{"rightmost untrusted wins over trusted", "10.0.0.0/8", "", "203.0.113.5, 10.0.0.1", "203.0.113.5"},
		{"all trusted returns leftmost valid", "10.0.0.0/8", "", "10.0.0.1, 10.0.0.2", "10.0.0.1"},
		{"invalid entries skipped returns leftmost valid", "10.0.0.0/8", "", "10.0.0.1, not-an-ip", "10.0.0.1"},
		{"all invalid returns empty", "10.0.0.0/8", "", "garbage, also-bad", ""},
		{"multi hop mixed picks rightmost untrusted", "10.0.0.0/8", "", "203.0.113.5, 10.0.0.1, 10.0.0.2", "203.0.113.5"},
		{"bracketed entries stripped", "10.0.0.0/8", "", "[203.0.113.5], [10.0.0.1]", "203.0.113.5"},
		{"no config treats all as untrusted rightmost wins", "", "", "1.2.3.4, 5.6.7.8", "5.6.7.8"},
		{"no config single ip returns it", "", "", "1.2.3.4", "1.2.3.4"},
		{"fallback env provides trust set", "", "10.0.0.0/8", "203.0.113.5, 10.0.0.1", "203.0.113.5"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setupTrustEnv(t, tc.primary, tc.fallback)
			got := rightMostUntrustedForwardedIP(tc.header)
			if got != tc.want {
				t.Errorf("rightMostUntrustedForwardedIP(%q) primary=%q = %q, want %q",
					tc.header, tc.primary, got, tc.want)
			}
		})
	}
}

func TestClientIP(t *testing.T) {
	const trustCIDR = "10.0.0.0/8"

	tests := []struct {
		name     string
		req      *http.Request
		want     string
		setupEnv bool // when false, still sets env to trustCIDR for determinism
	}{
		{
			name:     "nil request returns empty",
			req:      nil,
			want:     "",
			setupEnv: true,
		},
		{
			name:     "empty remote addr returns empty",
			req:      makeReq(t, "", nil),
			want:     "",
			setupEnv: true,
		},
		{
			name:     "untrusted remote ignores xff",
			req:      makeReq(t, "192.168.1.1:1234", map[string]string{"X-Forwarded-For": "203.0.113.5"}),
			want:     "192.168.1.1",
			setupEnv: true,
		},
		{
			name:     "untrusted remote ignores xreal ip",
			req:      makeReq(t, "192.168.1.1:1234", map[string]string{"X-Real-IP": "203.0.113.5"}),
			want:     "192.168.1.1",
			setupEnv: true,
		},
		{
			name:     "untrusted remote no headers returns remote",
			req:      makeReq(t, "192.168.1.1:1234", nil),
			want:     "192.168.1.1",
			setupEnv: true,
		},
		{
			name:     "trusted remote single untrusted xff hop",
			req:      makeReq(t, "10.0.0.1:1234", map[string]string{"X-Forwarded-For": "203.0.113.5"}),
			want:     "203.0.113.5",
			setupEnv: true,
		},
		{
			name:     "trusted remote xff chain mixed trust",
			req:      makeReq(t, "10.0.0.1:1234", map[string]string{"X-Forwarded-For": "203.0.113.5, 10.0.0.2"}),
			want:     "203.0.113.5",
			setupEnv: true,
		},
		{
			name:     "trusted remote all trusted xff returns leftmost",
			req:      makeReq(t, "10.0.0.1:1234", map[string]string{"X-Forwarded-For": "10.0.0.7, 10.0.0.8"}),
			want:     "10.0.0.7",
			setupEnv: true,
		},
		{
			name:     "trusted remote invalid xff falls back to valid xreal ip",
			req:      makeReq(t, "10.0.0.1:1234", map[string]string{"X-Forwarded-For": "not-an-ip", "X-Real-IP": "203.0.113.7"}),
			want:     "203.0.113.7",
			setupEnv: true,
		},
		{
			name:     "trusted remote empty xff falls back to xreal ip",
			req:      makeReq(t, "10.0.0.1:1234", map[string]string{"X-Real-IP": "203.0.113.9"}),
			want:     "203.0.113.9",
			setupEnv: true,
		},
		{
			name:     "trusted remote invalid xreal ip returns remote",
			req:      makeReq(t, "10.0.0.1:1234", map[string]string{"X-Real-IP": "not-an-ip"}),
			want:     "10.0.0.1",
			setupEnv: true,
		},
		{
			name:     "trusted remote bracketed xreal ip stripped",
			req:      makeReq(t, "10.0.0.1:1234", map[string]string{"X-Real-IP": "[203.0.113.10]"}),
			want:     "203.0.113.10",
			setupEnv: true,
		},
		{
			name:     "trusted remote empty xff and empty xreal ip returns remote",
			req:      makeReq(t, "10.0.0.1:1234", nil),
			want:     "10.0.0.1",
			setupEnv: true,
		},
		{
			name:     "trusted remote unparseable remote addr falls through",
			req:      makeReq(t, "garbage-no-port", map[string]string{"X-Forwarded-For": "203.0.113.5"}),
			want:     "garbage-no-port",
			setupEnv: true,
		},
		{
			name:     "no trust config treats remote as untrusted returns remote",
			req:      makeReq(t, "10.0.0.1:1234", map[string]string{"X-Forwarded-For": "203.0.113.5"}),
			want:     "10.0.0.1",
			setupEnv: false, // explicitly empty config below
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupEnv {
				setupTrustEnv(t, trustCIDR, "")
			} else {
				// Explicit empty config: no CIDRs loaded.
				setupTrustEnv(t, "", "")
			}
			got := ClientIP(tc.req)
			if got != tc.want {
				t.Errorf("ClientIP() = %q, want %q", got, tc.want)
			}
		})
	}
}
