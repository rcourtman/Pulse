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
