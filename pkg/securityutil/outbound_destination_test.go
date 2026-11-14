package securityutil

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

func resolveTo(ips ...string) func(context.Context, string) ([]net.IPAddr, error) {
	return func(context.Context, string) ([]net.IPAddr, error) {
		addrs := make([]net.IPAddr, 0, len(ips))
		for _, ip := range ips {
			addrs = append(addrs, net.IPAddr{IP: net.ParseIP(ip)})
		}
		return addrs, nil
	}
}

func TestValidateOutboundHostPolicy(t *testing.T) {
	privateOK := RestrictedOutboundHTTPOptions{AllowPrivateIPs: true}
	strict := RestrictedOutboundHTTPOptions{}

	for _, tc := range []struct {
		name    string
		host    string
		opts    RestrictedOutboundHTTPOptions
		blocked bool
	}{
		{"public literal", "93.184.216.34", strict, false},
		{"private literal with private allowed", "10.0.0.5", privateOK, false},
		{"private literal without private", "192.168.1.10", strict, true},
		{"unique-local IPv6 without private", "fd00::1", strict, true},
		{"loopback literal", "127.0.0.1", privateOK, true},
		{"loopback range", "127.8.9.10", privateOK, true},
		{"IPv6 loopback", "::1", privateOK, true},
		{"metadata literal", "169.254.169.254", privateOK, true},
		{"link-local", "169.254.10.1", privateOK, true},
		{"IPv6 link-local", "fe80::1", privateOK, true},
		{"unspecified", "0.0.0.0", privateOK, true},
		{"NAT64 metadata", "64:ff9b::a9fe:a9fe", privateOK, true},
		{"IPv4-mapped loopback", "::ffff:127.0.0.1", privateOK, true},
		{"metadata hostname", "metadata.google.internal", privateOK, true},
		{"metadata hostname with root dot", "Metadata.Google.Internal.", privateOK, true},
		{"empty", "  ", privateOK, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.opts.ResolveIPAddrs = func(context.Context, string) ([]net.IPAddr, error) {
				t.Fatalf("literal or refused host %q must not be resolved", tc.host)
				return nil, nil
			}
			err := ValidateOutboundHost(context.Background(), tc.host, tc.opts)
			if tc.blocked && err == nil {
				t.Fatalf("ValidateOutboundHost(%q) allowed a blocked destination", tc.host)
			}
			if !tc.blocked && err != nil {
				t.Fatalf("ValidateOutboundHost(%q) = %v, want allowed", tc.host, err)
			}
		})
	}
}

// A client that dials later is not pinned to the checked addresses, so one
// blocked address among several refuses the whole name.
func TestValidateOutboundHostRefusesNameWithAnyBlockedAddress(t *testing.T) {
	opts := RestrictedOutboundHTTPOptions{AllowPrivateIPs: true, ResolveIPAddrs: resolveTo("93.184.216.34", "127.0.0.1")}
	err := ValidateOutboundHost(context.Background(), "mixed.example.test", opts)
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("mixed resolution = %v, want a loopback refusal", err)
	}

	opts.ResolveIPAddrs = resolveTo("93.184.216.34", "10.1.2.3")
	if err := ValidateOutboundHost(context.Background(), "lan.example.test", opts); err != nil {
		t.Fatalf("public and private resolution with private allowed = %v", err)
	}
}

func TestValidateOutboundHostRefusesUnresolvableNames(t *testing.T) {
	opts := RestrictedOutboundHTTPOptions{AllowPrivateIPs: true, ResolveIPAddrs: func(context.Context, string) ([]net.IPAddr, error) {
		return nil, errors.New("no such host")
	}}
	if err := ValidateOutboundHost(context.Background(), "2130706433", opts); err == nil {
		t.Fatal("an unresolvable name was allowed; a later client may read it as an address")
	}
	opts.ResolveIPAddrs = resolveTo()
	if err := ValidateOutboundHost(context.Background(), "empty.example.test", opts); err == nil {
		t.Fatal("a name with no addresses was allowed")
	}
}
