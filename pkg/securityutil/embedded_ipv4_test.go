package securityutil

import (
	"context"
	"net"
	"testing"
)

func TestEmbeddedIPv4Candidates(t *testing.T) {
	tests := []struct {
		name  string
		addr  string
		want  []string
		empty bool
	}{
		{name: "nat64 well-known metadata", addr: "64:ff9b::a9fe:a9fe", want: []string{"169.254.169.254"}},
		{name: "nat64 well-known loopback", addr: "64:ff9b::7f00:1", want: []string{"127.0.0.1"}},
		{name: "nat64 well-known private", addr: "64:ff9b::a00:1", want: []string{"10.0.0.1"}},
		{name: "nat64 well-known public", addr: "64:ff9b::808:808", want: []string{"8.8.8.8"}},
		{name: "nat64 local-use 96", addr: "64:ff9b:1::a9fe:a9fe", want: []string{"169.254.169.254"}},
		{name: "nat64 local-use 48", addr: "64:ff9b:1:a9fe:a9:fe00::", want: []string{"169.254.169.254"}},
		{name: "6to4 private", addr: "2002:ac10:0001::", want: []string{"172.16.0.1"}},
		{name: "6to4 metadata", addr: "2002:a9fe:a9fe::", want: []string{"169.254.169.254"}},
		{name: "teredo server and client", addr: "2001:0:a9fe:a9fe:0:0:5601:5601", want: []string{"169.254.169.254", "169.254.169.254"}},
		{name: "isatap private", addr: "2001:db8::5efe:c0a8:101", want: []string{"192.168.1.1"}},
		{name: "isatap group bit set", addr: "2001:db8::200:5efe:c0a8:101", want: []string{"192.168.1.1"}},
		{name: "ipv4-compatible private", addr: "::192.168.1.1", want: []string{"192.168.1.1"}},
		{name: "ipv4-translated private", addr: "::ffff:0:192.168.1.1", want: []string{"192.168.1.1"}},

		{name: "plain ipv4 has nothing to unwrap", addr: "169.254.169.254", empty: true},
		{name: "ipv4-mapped is normalised by stdlib", addr: "::ffff:169.254.169.254", empty: true},
		{name: "ordinary global unicast ipv6", addr: "2606:4700:4700::1111", empty: true},
		{name: "unique local ipv6", addr: "fd00::1", empty: true},
		{name: "ipv6 loopback is not a transition address", addr: "::1", empty: true},
		{name: "unspecified is not a transition address", addr: "::", empty: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.addr)
			if ip == nil {
				t.Fatalf("failed to parse %q", tc.addr)
			}

			got := EmbeddedIPv4Candidates(ip)
			if tc.empty {
				if len(got) != 0 {
					t.Fatalf("expected no embedded candidates for %s, got %v", tc.addr, got)
				}
				return
			}

			for _, want := range tc.want {
				wantIP := net.ParseIP(want)
				found := false
				for _, candidate := range got {
					if candidate.Equal(wantIP) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected %s to embed %s, got %v", tc.addr, want, got)
				}
			}
		})
	}
}

func TestEmbeddedIPv4CandidatesSkipsUnroutableFirstOctet(t *testing.T) {
	// 0.0.0.0/8 destinations are not routable, and surfacing them would make
	// every all-zero prefix look like a transition address.
	for _, addr := range []string{"64:ff9b::0:1", "2002:0:1::", "::0.0.0.1"} {
		ip := net.ParseIP(addr)
		if ip == nil {
			t.Fatalf("failed to parse %q", addr)
		}
		if got := EmbeddedIPv4Candidates(ip); len(got) != 0 {
			t.Fatalf("expected %s to yield no candidates, got %v", addr, got)
		}
	}
}

// TestValidateOutboundIPBlocksIPv6TransitionAddresses covers the SSRF bypass
// where a NAT64, 6to4, Teredo, ISATAP or IPv4-compatible address carries a
// blocked IPv4 destination past every net.IP predicate.
func TestValidateOutboundIPBlocksIPv6TransitionAddresses(t *testing.T) {
	opts := RestrictedOutboundHTTPOptions{}

	blocked := []string{
		"64:ff9b::a9fe:a9fe",       // NAT64 -> 169.254.169.254
		"64:ff9b::7f00:1",          // NAT64 -> 127.0.0.1
		"64:ff9b::a00:1",           // NAT64 -> 10.0.0.1
		"64:ff9b:1::c0a8:1",        // NAT64 local-use -> 192.168.0.1
		"64:ff9b:1:a9fe:a9:fe00::", // NAT64 local-use /48 -> 169.254.169.254
		"2002:ac10:1::",            // 6to4 -> 172.16.0.1
		"2002:a9fe:a9fe::",         // 6to4 -> 169.254.169.254
		"2001:0:a9fe:a9fe::",       // Teredo server -> 169.254.169.254
		"2001:db8::5efe:c0a8:101",  // ISATAP -> 192.168.1.1
		"::192.168.1.1",            // IPv4-compatible -> 192.168.1.1
		"::ffff:0:10.0.0.1",        // IPv4-translated -> 10.0.0.1
	}
	for _, addr := range blocked {
		ip := net.ParseIP(addr)
		if ip == nil {
			t.Fatalf("failed to parse %q", addr)
		}
		if err := validateOutboundIP(ip, opts); err == nil {
			t.Fatalf("expected %s to be blocked as an embedded IPv4 destination", addr)
		}
	}

	allowed := []string{
		"64:ff9b::808:808",       // NAT64 -> 8.8.8.8
		"2002:808:808::",         // 6to4 -> 8.8.8.8
		"2606:4700:4700::1111",   // ordinary global unicast
		"2001:db8::5efe:808:808", // ISATAP -> 8.8.8.8
	}
	for _, addr := range allowed {
		ip := net.ParseIP(addr)
		if ip == nil {
			t.Fatalf("failed to parse %q", addr)
		}
		if err := validateOutboundIP(ip, opts); err != nil {
			t.Fatalf("expected %s to be allowed, got %v", addr, err)
		}
	}
}

func TestValidateOutboundIPTransitionRespectsOptions(t *testing.T) {
	nat64Private := net.ParseIP("64:ff9b::a00:1")
	nat64Loopback := net.ParseIP("64:ff9b::7f00:1")

	if err := validateOutboundIP(nat64Private, RestrictedOutboundHTTPOptions{AllowPrivateIPs: true}); err != nil {
		t.Fatalf("expected NAT64-wrapped private IP to be allowed when private IPs are permitted, got %v", err)
	}
	if err := validateOutboundIP(nat64Loopback, RestrictedOutboundHTTPOptions{AllowLoopback: true}); err != nil {
		t.Fatalf("expected NAT64-wrapped loopback to be allowed when loopback is permitted, got %v", err)
	}
	if err := validateOutboundIP(nat64Loopback, RestrictedOutboundHTTPOptions{AllowPrivateIPs: true}); err == nil {
		t.Fatalf("expected NAT64-wrapped loopback to stay blocked when only private IPs are permitted")
	}
}

func TestResolvePermittedOutboundIPsRejectsTransitionLiteral(t *testing.T) {
	if _, err := resolvePermittedOutboundIPs(context.Background(), "64:ff9b::a9fe:a9fe", RestrictedOutboundHTTPOptions{}); err == nil {
		t.Fatalf("expected NAT64 metadata literal to be rejected before dialling")
	}
}
