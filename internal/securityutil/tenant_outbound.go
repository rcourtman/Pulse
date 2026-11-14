package securityutil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	pubsec "github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
)

// TenantOutboundPolicy limits where a tenant organization's administrator may
// point this server's outbound connections: infrastructure connections,
// availability checks, address probes and the SMTP server. A tenant
// administrator is not the instance operator, so the server's own loopback,
// link-local and cloud metadata addresses are never theirs to reach. Private
// networks stay open where tenants' systems legitimately live behind them
// (shared-process organizations on the operator's LAN, provider client
// runtimes reaching client sites over VPN), and close on Pulse Cloud, where a
// private address can only be the hosting provider's own network.
type TenantOutboundPolicy struct {
	AllowPrivateNetworks bool
}

type tenantOutboundPolicyKey struct{}

// WithTenantOutboundPolicy marks ctx as acting for a tenant administrator.
func WithTenantOutboundPolicy(ctx context.Context, policy TenantOutboundPolicy) context.Context {
	return context.WithValue(ctx, tenantOutboundPolicyKey{}, policy)
}

// TenantOutboundPolicyFromContext returns the policy WithTenantOutboundPolicy
// attached, if any.
func TenantOutboundPolicyFromContext(ctx context.Context) (TenantOutboundPolicy, bool) {
	if ctx == nil {
		return TenantOutboundPolicy{}, false
	}
	policy, ok := ctx.Value(tenantOutboundPolicyKey{}).(TenantOutboundPolicy)
	return policy, ok
}

// ErrTenantOutboundDestination marks a destination refused by the tenant
// outbound policy.
var ErrTenantOutboundDestination = errors.New("destination is not available to organization administrators")

// CheckTenantOutboundDestination refuses destination when ctx carries a
// tenant outbound policy and the destination names or resolves to an address
// that policy excludes. Without a policy (instance administrators, and every
// single-tenant install) it allows everything. destination may be a host, an
// IP, host:port, [IPv6]:port or an absolute URL.
func CheckTenantOutboundDestination(ctx context.Context, destination string) error {
	policy, ok := TenantOutboundPolicyFromContext(ctx)
	if !ok {
		return nil
	}
	host := outboundDestinationHost(destination)
	if host == "" {
		return fmt.Errorf("%w: address has no host", ErrTenantOutboundDestination)
	}
	opts := RestrictedOutboundHTTPOptions{AllowPrivateIPs: policy.AllowPrivateNetworks}
	if err := pubsec.ValidateOutboundHost(ctx, host, opts); err != nil {
		return fmt.Errorf("%w: %v", ErrTenantOutboundDestination, err)
	}
	return nil
}

// TenantOutboundRefusalMessage explains a CheckTenantOutboundDestination
// refusal to the person who made the request.
func TenantOutboundRefusalMessage(ctx context.Context, err error) string {
	scope := "loopback, link-local or cloud metadata addresses"
	if policy, ok := TenantOutboundPolicyFromContext(ctx); ok && !policy.AllowPrivateNetworks {
		scope = "private, loopback, link-local or cloud metadata addresses"
	}
	return fmt.Sprintf("Organization administrators cannot connect Pulse to %s. An instance administrator can add this connection. (%v)", scope, err)
}

func outboundDestinationHost(destination string) string {
	destination = strings.TrimSpace(destination)
	if destination == "" {
		return ""
	}
	if ip := net.ParseIP(strings.Trim(destination, "[]")); ip != nil {
		return ip.String()
	}
	if strings.Contains(destination, "://") {
		parsed, err := url.Parse(destination)
		if err != nil {
			return ""
		}
		return parsed.Hostname()
	}
	if host, _, err := net.SplitHostPort(destination); err == nil {
		return strings.Trim(host, "[]")
	}
	parsed, err := url.Parse("//" + destination)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}
