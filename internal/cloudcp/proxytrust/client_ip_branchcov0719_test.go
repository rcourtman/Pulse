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

func TestForwardedClientIP(t *testing.T) {
	tests := []struct {
		name        string
		primary     string
		fallback    string
		values      []string
		want        string
		wantPresent bool
	}{
		{"no header", "10.0.0.0/8", "", nil, "", false},
		{"empty header", "10.0.0.0/8", "", []string{""}, "", false},
		{"only separators", "10.0.0.0/8", "", []string{" , ,"}, "", false},
		{"single untrusted hop", "10.0.0.0/8", "", []string{"203.0.113.5"}, "203.0.113.5", true},
		{"single trusted hop returns self", "10.0.0.0/8", "", []string{"10.0.0.1"}, "10.0.0.1", true},
		{"rightmost untrusted wins over trusted", "10.0.0.0/8", "", []string{"203.0.113.5, 10.0.0.1"}, "203.0.113.5", true},
		{"client claim left of appended hop ignored", "10.0.0.0/8", "", []string{"198.51.100.99, 203.0.113.5"}, "203.0.113.5", true},
		{"all trusted returns leftmost", "10.0.0.0/8", "", []string{"10.0.0.1, 10.0.0.2"}, "10.0.0.1", true},
		{"unreadable hop stops the walk", "10.0.0.0/8", "", []string{"198.51.100.99, unknown"}, "", true},
		{"unreadable hop after trusted hop stops the walk", "10.0.0.0/8", "", []string{"198.51.100.99, unknown, 10.0.0.2"}, "", true},
		{"client garbage left of appended hop ignored", "10.0.0.0/8", "", []string{"garbage, 203.0.113.5"}, "203.0.113.5", true},
		{"all invalid", "10.0.0.0/8", "", []string{"garbage, also-bad"}, "", true},
		{"multi hop mixed picks rightmost untrusted", "10.0.0.0/8", "", []string{"203.0.113.5, 10.0.0.1, 10.0.0.2"}, "203.0.113.5", true},
		{"bracketed entries stripped", "10.0.0.0/8", "", []string{"[203.0.113.5], [10.0.0.1]"}, "203.0.113.5", true},
		{"ipv4 with port", "10.0.0.0/8", "", []string{"198.51.100.99, 203.0.113.5:4711"}, "203.0.113.5", true},
		{"bracketed ipv6 with port", "10.0.0.0/8", "", []string{"198.51.100.99, [2001:db8::5]:4711"}, "2001:db8::5", true},
		{"trusted hop with port skipped", "10.0.0.0/8", "", []string{"203.0.113.5, 10.0.0.2:80"}, "203.0.113.5", true},
		{"ipv6 normalised", "10.0.0.0/8", "", []string{"2001:DB8:0:0::5"}, "2001:db8::5", true},
		{"header lines read as one list", "10.0.0.0/8", "", []string{"198.51.100.99", "203.0.113.5"}, "203.0.113.5", true},
		{"later line trusted hops skipped", "10.0.0.0/8", "", []string{"198.51.100.99, 203.0.113.5", "10.0.0.2"}, "203.0.113.5", true},
		{"no config treats all as untrusted rightmost wins", "", "", []string{"1.2.3.4, 5.6.7.8"}, "5.6.7.8", true},
		{"fallback env provides trust set", "", "10.0.0.0/8", []string{"203.0.113.5, 10.0.0.1"}, "203.0.113.5", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setupTrustEnv(t, tc.primary, tc.fallback)
			got, present := forwardedClientIP(tc.values)
			if got != tc.want || present != tc.wantPresent {
				t.Errorf("forwardedClientIP(%q) primary=%q = (%q, %v), want (%q, %v)",
					tc.values, tc.primary, got, present, tc.want, tc.wantPresent)
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
			name:     "trusted remote unreadable xff returns proxy not xreal ip",
			req:      makeReq(t, "10.0.0.1:1234", map[string]string{"X-Forwarded-For": "not-an-ip", "X-Real-IP": "203.0.113.7"}),
			want:     "10.0.0.1",
			setupEnv: true,
		},
		{
			name:     "trusted remote empty xff value falls back to xreal ip",
			req:      makeReq(t, "10.0.0.1:1234", map[string]string{"X-Forwarded-For": " ", "X-Real-IP": "203.0.113.7"}),
			want:     "203.0.113.7",
			setupEnv: true,
		},
		{
			name: "trusted remote reads the proxy's own xff line",
			req: &http.Request{RemoteAddr: "10.0.0.1:1234", Header: http.Header{
				"X-Forwarded-For": {"198.51.100.99", "203.0.113.5"},
			}},
			want:     "203.0.113.5",
			setupEnv: true,
		},
		{
			name:     "trusted remote valid xff wins over conflicting xreal ip",
			req:      makeReq(t, "10.0.0.1:1234", map[string]string{"X-Forwarded-For": "203.0.113.5", "X-Real-IP": "198.51.100.99"}),
			want:     "203.0.113.5",
			setupEnv: true,
		},
		{
			name:     "untrusted cdn hop becomes the client",
			req:      makeReq(t, "10.0.0.1:1234", map[string]string{"X-Forwarded-For": "203.0.113.5, 192.0.2.10"}),
			want:     "192.0.2.10",
			setupEnv: true,
		},
		{
			name:     "trusted remote xff hop with port",
			req:      makeReq(t, "10.0.0.1:1234", map[string]string{"X-Forwarded-For": "198.51.100.99, 203.0.113.5:4711"}),
			want:     "203.0.113.5",
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
