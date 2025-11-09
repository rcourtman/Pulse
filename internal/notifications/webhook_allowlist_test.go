package notifications

import (
	"net"
	"testing"
)

func TestUpdateAllowedPrivateCIDRs(t *testing.T) {
	nm := NewNotificationManager("")

	tests := []struct {
		name    string
		cidrs   string
		wantErr bool
	}{
		{
			name:    "empty string clears allowlist",
			cidrs:   "",
			wantErr: false,
		},
		{
			name:    "single valid CIDR",
			cidrs:   "192.168.1.0/24",
			wantErr: false,
		},
		{
			name:    "multiple valid CIDRs",
			cidrs:   "192.168.1.0/24,10.0.0.0/8",
			wantErr: false,
		},
		{
			name:    "CIDR with spaces",
			cidrs:   "192.168.1.0/24, 10.0.0.0/8",
			wantErr: false,
		},
		{
			name:    "bare IPv4 address",
			cidrs:   "192.168.1.1",
			wantErr: false,
		},
		{
			name:    "bare IPv6 address",
			cidrs:   "fe80::1",
			wantErr: false,
		},
		{
			name:    "invalid CIDR",
			cidrs:   "not-a-cidr",
			wantErr: true,
		},
		{
			name:    "invalid IP address",
			cidrs:   "999.999.999.999",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := nm.UpdateAllowedPrivateCIDRs(tt.cidrs)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateAllowedPrivateCIDRs() error = %v, wantErr %v", err, tt.wantErr)
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
