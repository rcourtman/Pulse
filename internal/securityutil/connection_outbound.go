package securityutil

import (
	"context"
	"fmt"
	"net"
	"time"

	pubsec "github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
)

// ConnectionOutboundPolicy retains the authenticated saving actor's outbound
// boundary for background appliance polling after the request has ended. It is
// server-owned metadata, never a grant supplied by a tenant. The empty legacy
// value retains existing instance-managed behavior: old records have no saving
// actor provenance, so this type does not guess who authorized them.
type ConnectionOutboundPolicy string

const (
	ConnectionOutboundTenantLAN    ConnectionOutboundPolicy = "tenant-lan"
	ConnectionOutboundTenantPublic ConnectionOutboundPolicy = "tenant-public"
)

// ConnectionOutboundPolicyForContext derives metadata from authenticated
// request context, not a submitted configuration's JSON fields. An explicit
// instance-authority save clears tenant policy for that connection.
func ConnectionOutboundPolicyForContext(ctx context.Context) ConnectionOutboundPolicy {
	policy, tenant := TenantOutboundPolicyFromContext(ctx)
	if !tenant {
		return ""
	}
	if policy.AllowPrivateNetworks {
		return ConnectionOutboundTenantLAN
	}
	return ConnectionOutboundTenantPublic
}

// DialContext returns nil only for the existing unrestricted/legacy value.
// Every restricted connection resolves once per dial, refuses the entire name
// if any answer is blocked, and connects only to those validated numeric IPs.
// Callers must bypass proxies and preserve the original TLS hostname/pin. A
// malformed saved policy fails closed; it is never an unrestricted fallback.
func (p ConnectionOutboundPolicy) DialContext(timeout time.Duration, resolve func(context.Context, string) ([]net.IPAddr, error)) func(context.Context, string, string) (net.Conn, error) {
	if p == "" {
		return nil
	}
	if p != ConnectionOutboundTenantLAN && p != ConnectionOutboundTenantPublic {
		return func(context.Context, string, string) (net.Conn, error) {
			return nil, fmt.Errorf("%w: unknown saved connection policy", ErrTenantOutboundDestination)
		}
	}
	dial := pubsec.NewValidatedOutboundDialer(pubsec.RestrictedOutboundHTTPOptions{
		AllowPrivateIPs: p == ConnectionOutboundTenantLAN,
		ResolveIPAddrs:  resolve,
	}, timeout)
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dial(ctx, network, address)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrTenantOutboundDestination, err)
		}
		return conn, nil
	}
}
