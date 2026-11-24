package securityutil

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCheckTenantOutboundDestinationWithoutPolicyAllowsEverything(t *testing.T) {
	for _, destination := range []string{"127.0.0.1", "https://169.254.169.254/latest", "[::1]:8006", ""} {
		if err := CheckTenantOutboundDestination(context.Background(), destination); err != nil {
			t.Fatalf("no policy: %q refused: %v", destination, err)
		}
	}
}

func TestCheckTenantOutboundDestinationHostForms(t *testing.T) {
	selfHosted := WithTenantOutboundPolicy(context.Background(), TenantOutboundPolicy{AllowPrivateNetworks: true})
	cloud := WithTenantOutboundPolicy(context.Background(), TenantOutboundPolicy{})

	for _, tc := range []struct {
		destination string
		ctx         context.Context
		blocked     bool
	}{
		{"127.0.0.1", selfHosted, true},
		{"127.0.0.1:7655", selfHosted, true},
		{"https://127.0.0.1:8006", selfHosted, true},
		{"http://user@127.0.0.1:7655/api/security/recovery", selfHosted, true},
		{"[::1]:8006", selfHosted, true},
		{"[::1]", selfHosted, true},
		{"::1", selfHosted, true},
		{"https://[fe80::1]:8006", selfHosted, true},
		{"169.254.169.254", selfHosted, true},
		{"http://169.254.169.254/latest/meta-data/", selfHosted, true},
		{"0.0.0.0:22", selfHosted, true},
		{"metadata.google.internal:80", selfHosted, true},
		{"", selfHosted, true},
		{"https://", selfHosted, true},
		{"10.0.0.5", selfHosted, false},
		{"https://192.168.1.20:8006", selfHosted, false},
		{"192.168.1.20:8006/api2/json", selfHosted, false},
		{"fd12::5", selfHosted, false},
		{"93.184.216.34", selfHosted, false},
		{"10.0.0.5", cloud, true},
		{"https://192.168.1.20:8006", cloud, true},
		{"fd12::5", cloud, true},
		{"https://93.184.216.34:8006", cloud, false},
	} {
		err := CheckTenantOutboundDestination(tc.ctx, tc.destination)
		if tc.blocked {
			if err == nil {
				t.Errorf("%q allowed, want refused", tc.destination)
			} else if !errors.Is(err, ErrTenantOutboundDestination) {
				t.Errorf("%q refusal %v does not wrap ErrTenantOutboundDestination", tc.destination, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q refused: %v", tc.destination, err)
		}
	}
}

// A destination is dialed as written (SMTP, TCP probes) or behind a default
// scheme (API clients). One that names a different host in each reading must
// not be judged by the reading its caller does not use.
func TestCheckTenantOutboundDestinationRefusesAmbiguousHosts(t *testing.T) {
	selfHosted := WithTenantOutboundPolicy(context.Background(), TenantOutboundPolicy{AllowPrivateNetworks: true})

	for _, destination := range []string{
		// Dialed as written: an IPv6 literal whose zone is "25@93.184.216.34".
		"::1%25@93.184.216.34",
		"::ffff:127.0.0.1%25@93.184.216.34",
		"::ffff:169.254.169.254%25@93.184.216.34",
		"fe80::1%25@93.184.216.34",
		// Behind a default scheme: user information, then the host.
		"93.184.216.34:1@127.0.0.1",
		"93.184.216.34:8006@127.0.0.1:8006",
		"93.184.216.34@127.0.0.1",
		"user:pass@93.184.216.34",
		// Behind a default scheme: host "localhost" with an empty port.
		"localhost://93.184.216.34",
		"localhost://93.184.216.34:8006/",
		"tcp://93.184.216.34:25",
		// Zones without user information still name the loopback address.
		"::1%25:25",
		"::1%2525:25",
		"[::1%25eth0]:25",
		"[::1%25eth0]",
	} {
		err := CheckTenantOutboundDestination(selfHosted, destination)
		if err == nil {
			t.Errorf("%q allowed, want refused", destination)
		} else if !errors.Is(err, ErrTenantOutboundDestination) {
			t.Errorf("%q refusal %v does not wrap ErrTenantOutboundDestination", destination, err)
		}
	}

	for _, destination := range []string{
		"HTTPS://93.184.216.34:8006",
		"http://93.184.216.34/path@segment",
		"93.184.216.34:8006/api2/json",
	} {
		if err := CheckTenantOutboundDestination(selfHosted, destination); err != nil {
			t.Errorf("%q refused: %v", destination, err)
		}
	}
}

func TestTenantOutboundRefusalMessageNamesTheRule(t *testing.T) {
	selfHosted := WithTenantOutboundPolicy(context.Background(), TenantOutboundPolicy{AllowPrivateNetworks: true})
	err := CheckTenantOutboundDestination(selfHosted, "127.0.0.1")
	message := TenantOutboundRefusalMessage(selfHosted, err)
	if !strings.Contains(message, "loopback") || strings.Contains(message, "private") {
		t.Fatalf("self-hosted refusal message = %q", message)
	}
	cloud := WithTenantOutboundPolicy(context.Background(), TenantOutboundPolicy{})
	if message := TenantOutboundRefusalMessage(cloud, err); !strings.Contains(message, "private") {
		t.Fatalf("cloud refusal message = %q", message)
	}
}
