package securityutil

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestConnectionOutboundPolicyComesOnlyFromContext(t *testing.T) {
	if got := ConnectionOutboundPolicyForContext(context.Background()); got != "" {
		t.Fatalf("instance policy=%q", got)
	}
	if got := ConnectionOutboundPolicyForContext(nil); got != "" {
		t.Fatalf("nil policy=%q", got)
	}
	for _, tc := range []struct {
		private bool
		want    ConnectionOutboundPolicy
	}{{true, ConnectionOutboundTenantLAN}, {false, ConnectionOutboundTenantPublic}} {
		ctx := WithTenantOutboundPolicy(context.Background(), TenantOutboundPolicy{AllowPrivateNetworks: tc.private})
		if got := ConnectionOutboundPolicyForContext(ctx); got != tc.want {
			t.Fatalf("got %q,want %q", got, tc.want)
		}
	}
}

func TestConnectionOutboundPolicyFailsClosedWithoutNetwork(t *testing.T) {
	if dial := ConnectionOutboundPolicy("").DialContext(time.Second, nil); dial != nil {
		t.Fatal("legacy/instance changed")
	}
	for _, policy := range []ConnectionOutboundPolicy{ConnectionOutboundTenantLAN, ConnectionOutboundTenantPublic, "typo"} {
		for _, host := range []string{"127.0.0.1", "::1", "169.254.169.254", "metadata.google.internal"} {
			t.Run(string(policy)+"/"+host, func(t *testing.T) {
				dial := policy.DialContext(time.Second, func(context.Context, string) ([]net.IPAddr, error) {
					t.Fatal("blocked literal/hostname resolved")
					return nil, nil
				})
				if _, err := dial(context.Background(), "tcp", net.JoinHostPort(host, "443")); !errors.Is(err, ErrTenantOutboundDestination) {
					t.Fatalf("refusal=%v", err)
				}
			})
		}
	}
}

func TestConnectionOutboundPolicyPreservesCanceledResolution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dial := ConnectionOutboundTenantLAN.DialContext(time.Second, func(ctx context.Context, _ string) ([]net.IPAddr, error) { return nil, ctx.Err() })
	_, err := dial(ctx, "tcp", "nas.example.test:443")
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrTenantOutboundDestination) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
