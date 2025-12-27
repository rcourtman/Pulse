package hostagent

import "testing"

func TestSelectBestIP(t *testing.T) {
	tests := []struct {
		name     string
		ips      []string
		expected string
	}{
		{
			name:     "prefers 192.168.x.x over corosync 172.20.x.x",
			ips:      []string{"172.20.0.80", "192.168.1.100"},
			expected: "192.168.1.100",
		},
		{
			name:     "prefers 192.168.x.x even when listed second",
			ips:      []string{"10.0.0.1", "192.168.0.1"},
			expected: "192.168.0.1",
		},
		{
			name:     "prefers 10.x.x.x over 172.16-31.x.x",
			ips:      []string{"172.20.0.1", "10.1.10.5"},
			expected: "10.1.10.5",
		},
		{
			name:     "handles single IP",
			ips:      []string{"192.168.1.1"},
			expected: "192.168.1.1",
		},
		{
			name:     "skips loopback",
			ips:      []string{"127.0.0.1", "192.168.1.1"},
			expected: "192.168.1.1",
		},
		{
			name:     "skips IPv6 loopback",
			ips:      []string{"::1", "10.0.0.1"},
			expected: "10.0.0.1",
		},
		{
			name:     "skips link-local IPv6",
			ips:      []string{"fe80::1", "192.168.1.1"},
			expected: "192.168.1.1",
		},
		{
			name:     "skips link-local IPv4",
			ips:      []string{"169.254.1.1", "10.0.0.1"},
			expected: "10.0.0.1",
		},
		{
			name:     "returns corosync IP if only option",
			ips:      []string{"127.0.0.1", "172.20.0.80"},
			expected: "172.20.0.80",
		},
		{
			name:     "empty list returns empty",
			ips:      []string{},
			expected: "",
		},
		{
			name:     "only loopback returns empty",
			ips:      []string{"127.0.0.1", "::1"},
			expected: "",
		},
		{
			name:     "common 10.1.x.x LAN preferred over 172.x.x",
			ips:      []string{"172.16.0.1", "10.1.10.50"},
			expected: "10.1.10.50",
		},
		{
			name:     "prefers 10.0.x.x to 10.100.x.x (common ranges first)",
			ips:      []string{"10.100.0.1", "10.0.0.1"},
			expected: "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectBestIP(tt.ips)
			if result != tt.expected {
				t.Errorf("selectBestIP(%v) = %q, want %q", tt.ips, result, tt.expected)
			}
		})
	}
}

func TestScoreIPv4(t *testing.T) {
	tests := []struct {
		ip            string
		expectedScore int
	}{
		// 192.168.x.x - highest priority (100)
		{"192.168.1.1", 100},
		{"192.168.0.100", 100},
		{"192.168.255.255", 100},

		// 10.0-31.x.x - common corporate (90)
		{"10.0.0.1", 90},
		{"10.1.10.5", 90},
		{"10.31.255.255", 90},

		// 10.32+.x.x - less common (70)
		{"10.32.0.1", 70},
		{"10.100.0.1", 70},
		{"10.255.255.255", 70},

		// 172.16-31.x.x - private but often cluster (50)
		{"172.16.0.1", 50},
		{"172.20.0.80", 50}, // Corosync typical
		{"172.31.255.255", 50},

		// 169.254.x.x - link-local (0)
		{"169.254.1.1", 0},

		// Other/public (30)
		{"8.8.8.8", 30},
		{"1.1.1.1", 30},
		{"203.0.113.1", 30},

		// Invalid
		{"not-an-ip", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			result := scoreIPv4(tt.ip)
			if result != tt.expectedScore {
				t.Errorf("scoreIPv4(%q) = %d, want %d", tt.ip, result, tt.expectedScore)
			}
		})
	}
}

func TestStateFileForType(t *testing.T) {
	setup := &ProxmoxSetup{}

	tests := []struct {
		ptype    string
		expected string
	}{
		{"pve", stateFilePVE},
		{"pbs", stateFilePBS},
		{"unknown", stateFilePath}, // fallback to legacy
	}

	for _, tt := range tests {
		t.Run(tt.ptype, func(t *testing.T) {
			result := setup.stateFileForType(tt.ptype)
			if result != tt.expected {
				t.Errorf("stateFileForType(%q) = %q, want %q", tt.ptype, result, tt.expected)
			}
		})
	}
}
