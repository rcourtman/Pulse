package main

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestSanitizeCorrelationID(t *testing.T) {
	valid := sanitizeCorrelationID("550e8400-e29b-41d4-a716-446655440000")
	if valid != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("expected valid UUID to pass through, got %s", valid)
	}

	invalid := sanitizeCorrelationID("not-a-uuid")
	if invalid == "not-a-uuid" {
		t.Fatalf("expected invalid UUID to be replaced")
	}

	empty := sanitizeCorrelationID("")
	if empty == "" {
		t.Fatalf("expected empty string to be replaced")
	}

	if invalid == empty {
		t.Fatalf("expected regenerated UUIDs to differ")
	}
}

func TestValidateNodeName(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
		desc    string
	}{
		{name: "node-1", wantErr: false, desc: "alphanumeric"},
		{name: "example.com", wantErr: false, desc: "dns hostname"},
		{name: "1.2.3.4", wantErr: false, desc: "ipv4"},
		{name: "2001:db8::1", wantErr: false, desc: "ipv6 compressed"},
		{name: "[2001:db8::10]", wantErr: false, desc: "ipv6 bracketed"},
		{name: "::1", wantErr: false, desc: "ipv6 loopback"},
		{name: "::", wantErr: false, desc: "ipv6 unspecified"},
		{name: "::ffff:192.0.2.1", wantErr: false, desc: "ipv4-mapped ipv6 dual stack"},
		{name: "[::1]", wantErr: false, desc: "ipv6 loopback bracketed"},
		{name: "fe80::1%eth0", wantErr: true, desc: "ipv6 zone identifier"},
		{name: "[fe80::1%eth0]", wantErr: true, desc: "ipv6 zone identifier bracketed"},
		{name: "[2001:db8::1]:22", wantErr: true, desc: "ipv6 with port suffix"},
		{name: "[2001:db8::1", wantErr: true, desc: "missing closing bracket"},
		{name: "2001:db8::1]", wantErr: true, desc: "missing opening bracket"},
		{name: "bad host", wantErr: true, desc: "whitespace disallowed"},
		{name: "-leadinghyphen", wantErr: true, desc: "leading hyphen disallowed"},
		{name: "example.com:22", wantErr: true, desc: "dns name with port"},
		{name: "", wantErr: true, desc: "empty string"},
		{name: "example_com", wantErr: false, desc: "underscore"},
		{name: "NODE123", wantErr: false, desc: "uppercase"},
		{name: strings.Repeat("a", 64), wantErr: false, desc: "64 chars"},
		{name: strings.Repeat("a", 65), wantErr: true, desc: "65 chars"},
		{name: "senso\u200Brs", wantErr: true, desc: "zero-width space"},
		{name: "node\\name", wantErr: true, desc: "backslash"},
		{name: "/etc/passwd", wantErr: true, desc: "absolute path"},
		{name: "node\x00", wantErr: true, desc: "null byte"},
		{name: "example.com;rm", wantErr: true, desc: "semicolon"},
		{name: "node$(rm)", wantErr: true, desc: "subshell"},
	}

	for _, tc := range cases {
		tc := tc
		name := tc.desc
		if name == "" {
			name = tc.name
		}
		t.Run(name, func(t *testing.T) {
			err := validateNodeName(tc.name)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error validating %q", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.name, err)
			}
		})
	}
}

type stubResolver struct {
	ips []net.IP
	err error
}

func (s stubResolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.ips, nil
}

func TestNodeValidatorAllowlistHost(t *testing.T) {
	v := &nodeValidator{
		allowHosts:   map[string]struct{}{"node-1": {}},
		hasAllowlist: true,
		resolver:     stubResolver{},
	}

	if err := v.Validate(context.Background(), "node-1"); err != nil {
		t.Fatalf("expected node-1 to be permitted, got error: %v", err)
	}

	if err := v.Validate(context.Background(), "node-2"); err == nil {
		t.Fatalf("expected node-2 to be rejected without allow-list entry")
	}
}

func TestNodeValidatorAllowlistCIDRWithLookup(t *testing.T) {
	_, network, _ := net.ParseCIDR("10.0.0.0/24")
	v := &nodeValidator{
		allowHosts:   make(map[string]struct{}),
		allowCIDRs:   []*net.IPNet{network},
		hasAllowlist: true,
		resolver: stubResolver{
			ips: []net.IP{net.ParseIP("10.0.0.5")},
		},
	}

	if err := v.Validate(context.Background(), "worker.local"); err != nil {
		t.Fatalf("expected worker.local to resolve into allowed CIDR: %v", err)
	}
}

func TestNodeValidatorClusterCaching(t *testing.T) {
	current := time.Now()
	fetches := 0

	v := &nodeValidator{
		clusterEnabled: true,
		clusterFetcher: func() ([]string, error) {
			fetches++
			return []string{"10.0.0.9"}, nil
		},
		cacheTTL: nodeValidatorCacheTTL,
		clock: func() time.Time {
			return current
		},
	}

	if err := v.Validate(context.Background(), "10.0.0.9"); err != nil {
		t.Fatalf("expected node to be allowed via cluster membership: %v", err)
	}
	if fetches != 1 {
		t.Fatalf("expected initial cluster fetch, got %d", fetches)
	}

	current = current.Add(30 * time.Second)
	if err := v.Validate(context.Background(), "10.0.0.9"); err != nil {
		t.Fatalf("expected cached cluster membership to allow node: %v", err)
	}
	if fetches != 1 {
		t.Fatalf("expected cache hit to avoid new fetch, got %d fetches", fetches)
	}

	current = current.Add(nodeValidatorCacheTTL + time.Second)
	if err := v.Validate(context.Background(), "10.0.0.9"); err != nil {
		t.Fatalf("expected refreshed cluster membership to allow node: %v", err)
	}
	if fetches != 2 {
		t.Fatalf("expected cache expiry to trigger new fetch, got %d", fetches)
	}
}

func TestNodeValidatorClusterResolvesHostIPs(t *testing.T) {
	v := &nodeValidator{
		clusterEnabled: true,
		clusterFetcher: func() ([]string, error) {
			return []string{"worker.local"}, nil
		},
		resolver: stubResolver{
			ips: []net.IP{net.ParseIP("10.0.0.5")},
		},
	}

	if err := v.Validate(context.Background(), "10.0.0.5"); err != nil {
		t.Fatalf("expected cluster hostname resolution to permit node: %v", err)
	}
}

func TestNodeValidatorStrictNoSources(t *testing.T) {
	v := &nodeValidator{
		strict: true,
	}

	if err := v.Validate(context.Background(), "node-1"); err == nil {
		t.Fatalf("expected strict mode without sources to reject nodes")
	}
}

func TestStripNodeDelimiters(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ipv6 with brackets",
			input:    "[2001:db8::1]",
			expected: "2001:db8::1",
		},
		{
			name:     "ipv6 loopback bracketed",
			input:    "[::1]",
			expected: "::1",
		},
		{
			name:     "ipv4 no brackets",
			input:    "192.168.1.1",
			expected: "192.168.1.1",
		},
		{
			name:     "hostname no brackets",
			input:    "node-1.example.com",
			expected: "node-1.example.com",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only opening bracket",
			input:    "[2001:db8::1",
			expected: "[2001:db8::1",
		},
		{
			name:     "only closing bracket",
			input:    "2001:db8::1]",
			expected: "2001:db8::1]",
		},
		{
			name:     "brackets with single char",
			input:    "[a]",
			expected: "a",
		},
		{
			name:     "empty brackets",
			input:    "[]",
			expected: "[]",
		},
		{
			name:     "single bracket char",
			input:    "[",
			expected: "[",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := stripNodeDelimiters(tc.input)
			if result != tc.expected {
				t.Errorf("stripNodeDelimiters(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestParseNodeIP(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expectNil  bool
		expectedIP string
	}{
		{
			name:       "ipv4",
			input:      "192.168.1.1",
			expectNil:  false,
			expectedIP: "192.168.1.1",
		},
		{
			name:       "ipv4 with whitespace",
			input:      "  10.0.0.1  ",
			expectNil:  false,
			expectedIP: "10.0.0.1",
		},
		{
			name:       "ipv6",
			input:      "2001:db8::1",
			expectNil:  false,
			expectedIP: "2001:db8::1",
		},
		{
			name:       "ipv6 bracketed",
			input:      "[2001:db8::1]",
			expectNil:  false,
			expectedIP: "2001:db8::1",
		},
		{
			name:       "ipv6 loopback",
			input:      "::1",
			expectNil:  false,
			expectedIP: "::1",
		},
		{
			name:       "ipv6 loopback bracketed",
			input:      "[::1]",
			expectNil:  false,
			expectedIP: "::1",
		},
		{
			name:      "hostname",
			input:     "node-1.example.com",
			expectNil: true,
		},
		{
			name:      "empty string",
			input:     "",
			expectNil: true,
		},
		{
			name:      "invalid ip",
			input:     "999.999.999.999",
			expectNil: true,
		},
		{
			name:      "partial ipv4",
			input:     "192.168.1",
			expectNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parseNodeIP(tc.input)
			if tc.expectNil {
				if result != nil {
					t.Errorf("parseNodeIP(%q) = %v, want nil", tc.input, result)
				}
			} else {
				if result == nil {
					t.Errorf("parseNodeIP(%q) = nil, want %s", tc.input, tc.expectedIP)
				} else if result.String() != tc.expectedIP {
					t.Errorf("parseNodeIP(%q) = %v, want %s", tc.input, result, tc.expectedIP)
				}
			}
		})
	}
}

func TestNormalizeAllowlistEntry(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ipv4",
			input:    "192.168.1.1",
			expected: "192.168.1.1",
		},
		{
			name:     "ipv4 with whitespace",
			input:    "  10.0.0.1  ",
			expected: "10.0.0.1",
		},
		{
			name:     "ipv6",
			input:    "2001:db8::1",
			expected: "2001:db8::1",
		},
		{
			name:     "ipv6 bracketed",
			input:    "[2001:db8::1]",
			expected: "2001:db8::1",
		},
		{
			name:     "hostname lowercase",
			input:    "node-1.example.com",
			expected: "node-1.example.com",
		},
		{
			name:     "hostname uppercase normalized",
			input:    "NODE-1.EXAMPLE.COM",
			expected: "node-1.example.com",
		},
		{
			name:     "hostname mixed case",
			input:    "Node-1.Example.Com",
			expected: "node-1.example.com",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: "",
		},
		{
			name:     "ipv6 loopback",
			input:    "::1",
			expected: "::1",
		},
		{
			name:     "ipv6 full form normalized",
			input:    "2001:0db8:0000:0000:0000:0000:0000:0001",
			expected: "2001:db8::1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizeAllowlistEntry(tc.input)
			if result != tc.expected {
				t.Errorf("normalizeAllowlistEntry(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIPAllowed(t *testing.T) {
	// Setup test CIDRs
	_, cidr10, _ := net.ParseCIDR("10.0.0.0/8")
	_, cidr192, _ := net.ParseCIDR("192.168.1.0/24")
	_, cidr172, _ := net.ParseCIDR("172.16.0.0/12")

	tests := []struct {
		name     string
		ip       net.IP
		hosts    map[string]struct{}
		cidrs    []*net.IPNet
		expected bool
	}{
		{
			name:     "nil ip",
			ip:       nil,
			hosts:    map[string]struct{}{"10.0.0.1": {}},
			cidrs:    nil,
			expected: false,
		},
		{
			name:     "ip in hosts map",
			ip:       net.ParseIP("192.168.1.100"),
			hosts:    map[string]struct{}{"192.168.1.100": {}},
			cidrs:    nil,
			expected: true,
		},
		{
			name:     "ip not in hosts map",
			ip:       net.ParseIP("192.168.1.100"),
			hosts:    map[string]struct{}{"192.168.1.200": {}},
			cidrs:    nil,
			expected: false,
		},
		{
			name:     "ip in cidr range",
			ip:       net.ParseIP("10.1.2.3"),
			hosts:    nil,
			cidrs:    []*net.IPNet{cidr10},
			expected: true,
		},
		{
			name:     "ip not in cidr range",
			ip:       net.ParseIP("11.0.0.1"),
			hosts:    nil,
			cidrs:    []*net.IPNet{cidr10},
			expected: false,
		},
		{
			name:     "ip in second cidr",
			ip:       net.ParseIP("192.168.1.50"),
			hosts:    nil,
			cidrs:    []*net.IPNet{cidr10, cidr192},
			expected: true,
		},
		{
			name:     "ip matches hosts not cidrs",
			ip:       net.ParseIP("8.8.8.8"),
			hosts:    map[string]struct{}{"8.8.8.8": {}},
			cidrs:    []*net.IPNet{cidr10},
			expected: true,
		},
		{
			name:     "ip matches cidrs not hosts",
			ip:       net.ParseIP("172.20.0.1"),
			hosts:    map[string]struct{}{"8.8.8.8": {}},
			cidrs:    []*net.IPNet{cidr172},
			expected: true,
		},
		{
			name:     "empty hosts and cidrs",
			ip:       net.ParseIP("192.168.1.1"),
			hosts:    nil,
			cidrs:    nil,
			expected: false,
		},
		{
			name:     "ipv6 in hosts",
			ip:       net.ParseIP("2001:db8::1"),
			hosts:    map[string]struct{}{"2001:db8::1": {}},
			cidrs:    nil,
			expected: true,
		},
		{
			name:     "nil hosts map with ip match in cidr",
			ip:       net.ParseIP("10.255.255.255"),
			hosts:    nil,
			cidrs:    []*net.IPNet{cidr10},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ipAllowed(tc.ip, tc.hosts, tc.cidrs)
			if result != tc.expected {
				t.Errorf("ipAllowed(%v, hosts, cidrs) = %v, want %v", tc.ip, result, tc.expected)
			}
		})
	}
}

func TestDefaultHostResolver(t *testing.T) {
	r := defaultHostResolver{}
	// Test with localhost
	ips, err := r.LookupIP(context.Background(), "localhost")
	if err != nil {
		t.Logf("localhost lookup failed (might be expected in some environments): %v", err)
	} else if len(ips) == 0 {
		t.Error("expected at least one IP for localhost")
	}

	// Test with nil context
	_, _ = r.LookupIP(nil, "localhost")

	// Test with invalid host
	_, err = r.LookupIP(context.Background(), "invalid.host.local.test")
	if err == nil {
		t.Error("expected error for invalid host")
	}
}

func TestNewNodeValidator(t *testing.T) {
	if _, err := newNodeValidator(nil, nil); err == nil {
		t.Error("expected error for nil config")
	}

	cfg := &Config{
		AllowedNodes:         []string{"node1", "10.0.0.0/24", ""},
		StrictNodeValidation: true,
	}
	v, err := newNodeValidator(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !v.hasAllowlist {
		t.Error("expected hasAllowlist to be true")
	}
	if len(v.allowHosts) != 1 {
		t.Errorf("expected 1 host, got %d", len(v.allowHosts))
	}
	if len(v.allowCIDRs) != 1 {
		t.Errorf("expected 1 CIDR, got %d", len(v.allowCIDRs))
	}
}

func TestNodeValidator_UpdateAllowlist(t *testing.T) {
	v := &nodeValidator{}
	v.UpdateAllowlist([]string{"node1"})
	if len(v.allowHosts) != 1 {
		t.Error("expected 1 allowed host")
	}

	// Update to empty
	v.UpdateAllowlist([]string{})
	if v.hasAllowlist {
		t.Error("expected hasAllowlist to be false")
	}

	// Nil validator
	var nilV *nodeValidator
	nilV.UpdateAllowlist([]string{"node1"})
}

func TestNodeValidator_Validate_Errors(t *testing.T) {
	v := &nodeValidator{
		hasAllowlist: true,
		resolver: stubResolver{
			err: errors.New("resolution failed"),
		},
		allowCIDRs: []*net.IPNet{{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(24, 32)}},
	}

	// matchesAllowlist returns error
	err := v.Validate(context.Background(), "hostname")
	if err == nil || !strings.Contains(err.Error(), "resolution failed") {
		t.Errorf("expected resolution failed error, got %v", err)
	}

	// Cluster enabled but fetcher fails
	v2 := &nodeValidator{
		clusterEnabled: true,
		clusterFetcher: func() ([]string, error) {
			return nil, errors.New("fetch failed")
		},
		metrics: NewProxyMetrics("test"),
	}
	// Note: validateAsLocalhost will be called, which might succeed or fail depending on env
	_ = v2.Validate(context.Background(), "some-node")
}

func TestNodeValidator_ValidateAsLocalhost(t *testing.T) {
	v := &nodeValidator{}
	// Test with node that is likely NOT localhost
	err := v.validateAsLocalhost(context.Background(), "not-localhost-host")
	if err == nil {
		t.Error("expected error for non-localhost node")
	}

	// Test with "127.0.0.1"
	err = v.validateAsLocalhost(context.Background(), "127.0.0.1")
	if err != nil {
		t.Logf("127.0.0.1 validation failed (env dependent): %v", err)
	}
}

func TestGetClusterMembers_Error(t *testing.T) {
	v := &nodeValidator{
		clusterFetcher: func() ([]string, error) {
			return nil, errors.New("fetch failed")
		},
	}
	_, err := v.getClusterMembers(context.Background())
	if err == nil {
		t.Error("expected error from clusterFetcher")
	}
}

func TestGetClusterMembers_ResolutionFails(t *testing.T) {
	v := &nodeValidator{
		clusterFetcher: func() ([]string, error) {
			return []string{"valid-node", "invalid-node", "10.0.0.1", ""}, nil
		},
		resolver: stubResolver{
			err: errors.New("resolution failed"),
		},
	}
	members, err := v.getClusterMembers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// "valid-node", "invalid-node" (both normalized), and "10.0.0.1" should be in members
	// Hostnames are added even if resolution fails.
	if len(members) < 3 {
		t.Errorf("expected at least 3 members, got %d: %v", len(members), members)
	}
}

func TestNodeValidator_Validate_Nil(t *testing.T) {
	var v *nodeValidator
	if err := v.Validate(context.Background(), "node"); err != nil {
		t.Error("expected nil error for nil validator")
	}
}
