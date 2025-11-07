package main

import (
	"context"
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

func TestValidateCommand(t *testing.T) {
	type tc struct {
		name    string
		args    []string
		wantErr bool
		desc    string
	}

	cases := []tc{
		{name: "sensors", args: nil, wantErr: false, desc: "bare sensors"},
		{name: "sensors", args: []string{"-j"}, wantErr: false, desc: "json flag"},
		{name: "ipmitool", args: []string{"sdr"}, wantErr: false, desc: "safe ipmitool"},
		{name: "sensors", args: []string{"; rm -rf /"}, wantErr: true, desc: "shell metachar"},
		{name: "sensors", args: []string{"$(id)"}, wantErr: true, desc: "subshell"},
		{name: "ipmitool", args: []string{"-H", "1.2.3.4", "&&", "shutdown"}, wantErr: true, desc: "command chaining"},
		{name: "sensors", args: []string{">/tmp/out"}, wantErr: true, desc: "redirect"},
		{name: "senso\u200Brs", wantErr: true, desc: "unicode homoglyph"},
		{name: "sensors", args: []string{"-" + strings.Repeat("v", 2000)}, wantErr: true, desc: "arg too long"},
		{name: "sensors", args: []string{"test\x00"}, wantErr: true, desc: "null byte arg"},
		{name: "ipmitool", args: []string{"chassis", "power", "off"}, wantErr: true, desc: "dangerous ipmitool"},
		{name: "sensors", args: []string{"LC_ALL=C"}, wantErr: true, desc: "env prefix"},
		{name: "/usr/bin/sensors", wantErr: true, desc: "absolute path"},
		{name: "ipmitool", args: []string{"--extraneous=../../etc/passwd"}, wantErr: true, desc: "path traversal"},
	}

	for _, tc := range cases {
		tc := tc
		if tc.desc == "" {
			tc.desc = tc.name
		}
		t.Run(tc.desc, func(t *testing.T) {
			err := validateCommand(tc.name, tc.args)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %s %v", tc.name, tc.args)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %s %v: %v", tc.name, tc.args, err)
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

func TestNodeValidatorStrictNoSources(t *testing.T) {
	v := &nodeValidator{
		strict: true,
	}

	if err := v.Validate(context.Background(), "node-1"); err == nil {
		t.Fatalf("expected strict mode without sources to reject nodes")
	}
}
