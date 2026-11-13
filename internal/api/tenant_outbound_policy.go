package api

import (
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

// withTenantOutboundPolicy attaches the tenant outbound policy to requests
// RequireOrgAdmin and RequireOrgManager admit without instance authority: an
// organization's own owners and admins, and tokens bound to it. Handlers that
// make this server connect to an address from the request or from that
// organization's config check it with securityutil.CheckTenantOutboundDestination.
// Instance administrators carry no policy, so single-tenant installs are never
// limited by it.
func withTenantOutboundPolicy(cfg *config.Config, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !requestCarriesInstanceAuthority(cfg, req) {
			req = req.WithContext(securityutil.WithTenantOutboundPolicy(req.Context(), tenantOutboundPolicyForRuntime()))
		}
		handler(w, req)
	}
}

// tenantOutboundPolicyForRuntime closes private networks only on Pulse Cloud.
// A provider's client runtime reaches client sites over the provider's VPN,
// and shared-process organizations monitor the operator's LAN, so both keep
// private addresses.
func tenantOutboundPolicyForRuntime() securityutil.TenantOutboundPolicy {
	pulseCloud := hostedModeEnabledFromEnv() && !isProviderHostedRuntimeFromLicensing()
	return securityutil.TenantOutboundPolicy{AllowPrivateNetworks: !pulseCloud}
}

// refuseTenantOutboundDestination writes a 403 and returns true when the
// request's tenant outbound policy excludes destination.
func refuseTenantOutboundDestination(w http.ResponseWriter, req *http.Request, destination string) bool {
	if err := securityutil.CheckTenantOutboundDestination(req.Context(), destination); err != nil {
		writeErrorResponse(w, http.StatusForbidden, "destination_not_allowed", securityutil.TenantOutboundRefusalMessage(req.Context(), err), nil)
		return true
	}
	return false
}
