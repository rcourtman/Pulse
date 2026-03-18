package docker

import "fmt"

// TraefikLabels generates Docker labels for Traefik reverse-proxy routing.
// Each tenant gets a subdomain: <tenantID>.cloud.pulserelay.pro
func TraefikLabels(tenantID, baseDomain string, containerPort int) map[string]string {
	svc := "pulse-" + tenantID
	host := fmt.Sprintf("%s.%s", tenantID, baseDomain)

	return map[string]string{
		"traefik.enable": "true",

		// HTTP router
		fmt.Sprintf("traefik.http.routers.%s.rule", svc):             fmt.Sprintf("Host(`%s`)", host),
		fmt.Sprintf("traefik.http.routers.%s.entrypoints", svc):      "websecure",
		fmt.Sprintf("traefik.http.routers.%s.tls.certresolver", svc): "le",

		// Service
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", svc): fmt.Sprintf("%d", containerPort),

		// Metadata
		"pulse.tenant.id": tenantID,
	}
}
