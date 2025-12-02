package notifications

import (
	"net"
	"strings"
	"testing"
)

func TestUpdateAllowedPrivateCIDRs(t *testing.T) {
	tests := []struct {
		name    string
		cidrs   string
		wantErr string // empty string means no error expected
	}{
		// Success cases
		{
			name:    "empty string clears allowlist",
			cidrs:   "",
			wantErr: "",
		},
		{
			name:    "single valid CIDR",
			cidrs:   "192.168.1.0/24",
			wantErr: "",
		},
		{
			name:    "multiple valid CIDRs",
			cidrs:   "192.168.1.0/24,10.0.0.0/8",
			wantErr: "",
		},
		{
			name:    "CIDR with spaces",
			cidrs:   "192.168.1.0/24, 10.0.0.0/8",
			wantErr: "",
		},
		{
			name:    "bare IPv4 address",
			cidrs:   "192.168.1.1",
			wantErr: "",
		},
		{
			name:    "bare IPv6 address",
			cidrs:   "fe80::1",
			wantErr: "",
		},
		{
			name:    "valid IPv6 CIDR",
			cidrs:   "fe80::/10",
			wantErr: "",
		},
		{
			name:    "loopback IPv6",
			cidrs:   "::1",
			wantErr: "",
		},
		{
			name:    "multiple valid CIDRs with mixed IP versions",
			cidrs:   "192.168.1.0/24, 10.0.0.0/8, fe80::/10",
			wantErr: "",
		},
		{
			name:    "mixed bare IPs and CIDRs",
			cidrs:   "192.168.1.1, 10.0.0.0/8",
			wantErr: "",
		},
		{
			name:    "whitespace handling",
			cidrs:   "  192.168.1.0/24  ,  10.0.0.1  ",
			wantErr: "",
		},
		{
			name:    "empty entries skipped",
			cidrs:   "192.168.1.0/24,,10.0.0.1",
			wantErr: "",
		},

		// Error cases - invalid IP addresses (bare IPs without CIDR notation)
		{
			name:    "invalid IP address - garbage text",
			cidrs:   "not-a-cidr",
			wantErr: "invalid IP address: not-a-cidr",
		},
		{
			name:    "invalid IP address - out of range octets",
			cidrs:   "999.999.999.999",
			wantErr: "invalid IP address: 999.999.999.999",
		},
		{
			name:    "invalid IP address in list",
			cidrs:   "192.168.1.0/24, invalid, 10.0.0.1",
			wantErr: "invalid IP address: invalid",
		},
		{
			name:    "IP with too many octets",
			cidrs:   "192.168.1.1.1",
			wantErr: "invalid IP address: 192.168.1.1.1",
		},
		{
			name:    "IP with negative octet",
			cidrs:   "192.168.-1.0",
			wantErr: "invalid IP address: 192.168.-1.0",
		},
		{
			name:    "IP with octet out of range",
			cidrs:   "192.168.256.0",
			wantErr: "invalid IP address: 192.168.256.0",
		},

		// Error cases - invalid CIDR notation
		{
			name:    "CIDR prefix too large for IPv4",
			cidrs:   "192.168.1.0/33",
			wantErr: "invalid CIDR range 192.168.1.0/33",
		},
		{
			name:    "CIDR prefix too large for IPv6",
			cidrs:   "fe80::/129",
			wantErr: "invalid CIDR range fe80::/129",
		},
		{
			name:    "CIDR prefix way too large",
			cidrs:   "192.168.1.0/999",
			wantErr: "invalid CIDR range 192.168.1.0/999",
		},
		{
			name:    "CIDR with negative prefix",
			cidrs:   "192.168.1.0/-1",
			wantErr: "invalid CIDR range 192.168.1.0/-1",
		},
		{
			name:    "CIDR with non-numeric prefix",
			cidrs:   "192.168.1.0/abc",
			wantErr: "invalid CIDR range 192.168.1.0/abc",
		},
		{
			name:    "CIDR with empty prefix",
			cidrs:   "192.168.1.0/",
			wantErr: "invalid CIDR range 192.168.1.0/",
		},
		{
			name:    "CIDR with floating point prefix",
			cidrs:   "192.168.1.0/24.5",
			wantErr: "invalid CIDR range 192.168.1.0/24.5",
		},

		// Error cases - malformed strings
		{
			name:    "double slash in CIDR",
			cidrs:   "192.168.1.0//24",
			wantErr: "invalid CIDR range 192.168.1.0//24",
		},
		{
			name:    "CIDR with invalid IP part",
			cidrs:   "999.999.999.999/24",
			wantErr: "invalid CIDR range 999.999.999.999/24",
		},
		{
			name:    "valid CIDR followed by invalid CIDR",
			cidrs:   "192.168.1.0/24, bad/cidr",
			wantErr: "invalid CIDR range bad/cidr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nm := NewNotificationManager("")
			defer nm.Stop()

			err := nm.UpdateAllowedPrivateCIDRs(tt.cidrs)

			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("UpdateAllowedPrivateCIDRs() unexpected error = %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("UpdateAllowedPrivateCIDRs() expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("UpdateAllowedPrivateCIDRs() error = %v, want error containing %q", err, tt.wantErr)
				}
			}
		})
	}
}

func TestIsIPInAllowlist(t *testing.T) {
	nm := NewNotificationManager("")

	// Set up allowlist
	err := nm.UpdateAllowedPrivateCIDRs("192.168.1.0/24,10.0.0.0/8")
	if err != nil {
		t.Fatalf("Failed to setup allowlist: %v", err)
	}

	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{
			name:     "IP in first CIDR range",
			ip:       "192.168.1.100",
			expected: true,
		},
		{
			name:     "IP in second CIDR range",
			ip:       "10.5.10.20",
			expected: true,
		},
		{
			name:     "IP not in any range",
			ip:       "172.16.1.1",
			expected: false,
		},
		{
			name:     "IP at network boundary",
			ip:       "192.168.1.0",
			expected: true,
		},
		{
			name:     "IP at broadcast boundary",
			ip:       "192.168.1.255",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Invalid test IP: %s", tt.ip)
			}
			result := nm.isIPInAllowlist(ip)
			if result != tt.expected {
				t.Errorf("isIPInAllowlist(%s) = %v, expected %v", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestIsIPInAllowlistEmptyList(t *testing.T) {
	nm := NewNotificationManager("")

	// Empty allowlist should block all IPs
	ip := net.ParseIP("192.168.1.1")
	if nm.isIPInAllowlist(ip) {
		t.Error("Empty allowlist should block all private IPs")
	}
}

func TestValidateWebhookURLWithAllowlist(t *testing.T) {
	nm := NewNotificationManager("")

	// Test without allowlist (should block private IPs)
	err := nm.ValidateWebhookURL("http://192.168.1.100/webhook")
	if err == nil {
		t.Error("Expected error for private IP without allowlist")
	}

	// Set up allowlist
	err = nm.UpdateAllowedPrivateCIDRs("192.168.1.0/24")
	if err != nil {
		t.Fatalf("Failed to setup allowlist: %v", err)
	}

	// Should now allow the private IP in the allowlist
	err = nm.ValidateWebhookURL("http://192.168.1.100/webhook")
	if err != nil {
		t.Errorf("Expected no error for private IP in allowlist, got: %v", err)
	}

	// Should still block private IPs not in allowlist
	err = nm.ValidateWebhookURL("http://10.0.0.1/webhook")
	if err == nil {
		t.Error("Expected error for private IP not in allowlist")
	}

	// Should always block localhost regardless of allowlist
	err = nm.ValidateWebhookURL("http://localhost/webhook")
	if err == nil {
		t.Error("Expected error for localhost even with allowlist")
	}

	// Should always block link-local regardless of allowlist
	err = nm.ValidateWebhookURL("http://169.254.169.254/webhook")
	if err == nil {
		t.Error("Expected error for link-local even with allowlist")
	}
}

func TestValidateWebhookURL_DNSResolutionFailure(t *testing.T) {
	nm := NewNotificationManager("")

	// Use a hostname that will definitely not resolve
	err := nm.ValidateWebhookURL("http://this-hostname-definitely-does-not-exist-12345.invalid/webhook")
	if err == nil {
		t.Error("Expected error for unresolvable hostname")
	}
	if !strings.Contains(err.Error(), "failed to resolve webhook hostname") {
		t.Errorf("Expected DNS resolution error, got: %v", err)
	}
}

func TestValidateWebhookURL_EmptyURL(t *testing.T) {
	nm := NewNotificationManager("")

	err := nm.ValidateWebhookURL("")
	if err == nil {
		t.Error("Expected error for empty URL")
	}
	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("Expected 'cannot be empty' error, got: %v", err)
	}
}

func TestValidateWebhookURL_InvalidScheme(t *testing.T) {
	nm := NewNotificationManager("")

	// Test FTP scheme
	err := nm.ValidateWebhookURL("ftp://example.com/webhook")
	if err == nil {
		t.Error("Expected error for FTP scheme")
	}
	if !strings.Contains(err.Error(), "must use http or https") {
		t.Errorf("Expected scheme error, got: %v", err)
	}

	// Test file scheme
	err = nm.ValidateWebhookURL("file:///etc/passwd")
	if err == nil {
		t.Error("Expected error for file scheme")
	}
}

func TestValidateWebhookURL_MissingHostname(t *testing.T) {
	nm := NewNotificationManager("")

	// URL with no hostname
	err := nm.ValidateWebhookURL("http:///path")
	if err == nil {
		t.Error("Expected error for missing hostname")
	}
	if !strings.Contains(err.Error(), "missing hostname") {
		t.Errorf("Expected 'missing hostname' error, got: %v", err)
	}
}

func TestValidateWebhookURL_CloudMetadataEndpoints(t *testing.T) {
	nm := NewNotificationManager("")

	metadataEndpoints := []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://metadata.google.internal/computeMetadata/v1/",
		"http://metadata.goog/computeMetadata/v1/",
	}

	for _, endpoint := range metadataEndpoints {
		err := nm.ValidateWebhookURL(endpoint)
		if err == nil {
			t.Errorf("Expected error for cloud metadata endpoint: %s", endpoint)
		}
	}
}

func TestValidateWebhookURL_LoopbackVariants(t *testing.T) {
	nm := NewNotificationManager("")

	loopbackURLs := []string{
		"http://127.0.0.1/webhook",
		"http://127.0.0.2/webhook",
		"http://127.255.255.255/webhook",
		"http://[::1]/webhook",
	}

	for _, url := range loopbackURLs {
		err := nm.ValidateWebhookURL(url)
		if err == nil {
			t.Errorf("Expected error for loopback URL: %s", url)
		}
	}
}

func TestValidateWebhookURL_LinkLocalIPv6(t *testing.T) {
	nm := NewNotificationManager("")

	err := nm.ValidateWebhookURL("http://[fe80::1]/webhook")
	if err == nil {
		t.Error("Expected error for IPv6 link-local address")
	}
	if !strings.Contains(err.Error(), "link-local") {
		t.Errorf("Expected link-local error, got: %v", err)
	}
}
