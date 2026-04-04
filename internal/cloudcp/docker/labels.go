package docker

import (
	"fmt"
	"strings"
)

// TenantRuntimeRoutingContract is the canonical hosted-runtime addressing
// surface derived from a tenant ID and base domain.
type TenantRuntimeRoutingContract struct {
	Host      string
	PublicURL string
}

// CanonicalTenantRuntimeRouting derives the canonical hosted route host and
// public URL. Hosted routing must stay lowercase regardless of how the tenant
// ID is cased in control-plane storage.
func CanonicalTenantRuntimeRouting(tenantID, baseDomain string) TenantRuntimeRoutingContract {
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	baseDomain = strings.ToLower(strings.TrimSpace(baseDomain))
	if tenantID == "" || baseDomain == "" {
		return TenantRuntimeRoutingContract{}
	}
	host := fmt.Sprintf("%s.%s", tenantID, baseDomain)
	return TenantRuntimeRoutingContract{
		Host:      host,
		PublicURL: "https://" + host,
	}
}

// TraefikLabels generates Docker labels for Traefik reverse-proxy routing.
// Each tenant gets a subdomain: <tenantID>.cloud.pulserelay.pro
func TraefikLabels(tenantID, baseDomain string, containerPort int) map[string]string {
	svc := "pulse-" + tenantID
	routing := CanonicalTenantRuntimeRouting(tenantID, baseDomain)

	return map[string]string{
		"traefik.enable": "true",

		// HTTP router
		fmt.Sprintf("traefik.http.routers.%s.rule", svc):             fmt.Sprintf("Host(`%s`)", routing.Host),
		fmt.Sprintf("traefik.http.routers.%s.entrypoints", svc):      "websecure",
		fmt.Sprintf("traefik.http.routers.%s.tls.certresolver", svc): "le",

		// Service
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", svc): fmt.Sprintf("%d", containerPort),

		// Metadata
		"pulse.tenant.id": tenantID,
	}
}
