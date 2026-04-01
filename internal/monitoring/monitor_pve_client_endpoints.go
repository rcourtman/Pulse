package monitoring

import (
	"net"
	"net/url"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) buildClusterEndpointsForInit(pve config.PVEInstance) ([]string, map[string]string) {
	// For clusters, check if endpoints have IPs/resolvable hosts
	// If not, use the main host for all connections (Proxmox will route cluster API calls)
	hasValidEndpoints := false
	endpoints := make([]string, 0, len(pve.ClusterEndpoints))
	endpointFingerprints := make(map[string]string)
	discoveryCfg := monitorDiscoveryConfig(m)

	hasFingerprint := pve.Fingerprint != ""
	for _, ep := range pve.ClusterEndpoints {
		effectiveURL := clusterEndpointRuntimeURL(ep, pve.VerifySSL, hasFingerprint, discoveryCfg)
		if effectiveURL == "" {
			log.Warn().
				Str("node", ep.NodeName).
				Msg("Skipping cluster endpoint with no allowed host/IP")
			continue
		}

		if parsed, err := url.Parse(effectiveURL); err == nil {
			hostname := parsed.Hostname()
			if hostname != "" && (strings.Contains(hostname, ".") || net.ParseIP(hostname) != nil) {
				hasValidEndpoints = true
			}
		} else {
			hostname := normalizeEndpointHost(effectiveURL)
			if hostname != "" && (strings.Contains(hostname, ".") || net.ParseIP(hostname) != nil) {
				hasValidEndpoints = true
			}
		}

		endpoints = append(endpoints, effectiveURL)
		// Store per-endpoint fingerprint for TOFU (Trust On First Use)
		if ep.Fingerprint != "" {
			endpointFingerprints[effectiveURL] = ep.Fingerprint
		}
	}

	// If endpoints are just node names (not FQDNs or IPs), use main host only
	// This is common when cluster nodes are discovered but not directly reachable
	if !hasValidEndpoints || len(endpoints) == 0 {
		log.Info().
			Str("instance", pve.Name).
			Str("mainHost", pve.Host).
			Msg("Cluster endpoints are not resolvable, using main host for all cluster operations")
		fallback := ensureClusterEndpointURL(pve.Host)
		if fallback == "" {
			fallback = ensureClusterEndpointURL(pve.Host)
		}
		endpoints = []string{fallback}
	} else {
		// Always include the main host URL as a fallback endpoint.
		// This handles remote cluster scenarios where Proxmox reports internal IPs
		// that aren't reachable from Pulse's network. The user-provided URL is
		// reachable, so include it as a fallback for cluster API routing.
		mainHostURL := ensureClusterEndpointURL(pve.Host)
		mainHostAlreadyIncluded := false
		for _, ep := range endpoints {
			if ep == mainHostURL {
				mainHostAlreadyIncluded = true
				break
			}
		}
		if !mainHostAlreadyIncluded && mainHostURL != "" {
			log.Info().
				Str("instance", pve.Name).
				Str("mainHost", mainHostURL).
				Int("clusterEndpoints", len(endpoints)).
				Msg("Adding main host as fallback for remote cluster access")
			endpoints = append(endpoints, mainHostURL)
		}
	}

	return endpoints, endpointFingerprints
}

func (m *Monitor) buildClusterEndpointsForReconnect(pve config.PVEInstance) ([]string, map[string]string) {
	hasValidEndpoints := false
	endpoints := make([]string, 0, len(pve.ClusterEndpoints))
	endpointFingerprints := make(map[string]string)
	discoveryCfg := monitorDiscoveryConfig(m)
	hasFingerprint := pve.Fingerprint != ""

	for _, ep := range pve.ClusterEndpoints {
		host := clusterEndpointRuntimeURL(ep, pve.VerifySSL, hasFingerprint, discoveryCfg)
		if host == "" {
			continue
		}

		if parsed, err := url.Parse(host); err == nil {
			hostname := parsed.Hostname()
			if hostname != "" && (strings.Contains(hostname, ".") || net.ParseIP(hostname) != nil) {
				hasValidEndpoints = true
			}
		} else {
			hostname := normalizeEndpointHost(host)
			if hostname != "" && (strings.Contains(hostname, ".") || net.ParseIP(hostname) != nil) {
				hasValidEndpoints = true
			}
		}

		if !strings.HasPrefix(host, "http") {
			host = ensureClusterEndpointURL(host)
		}
		endpoints = append(endpoints, host)
		if ep.Fingerprint != "" {
			endpointFingerprints[host] = ep.Fingerprint
		}
	}

	if !hasValidEndpoints || len(endpoints) == 0 {
		fallback := ensureClusterEndpointURL(pve.Host)
		if fallback == "" {
			fallback = ensureClusterEndpointURL(pve.Host)
		}
		endpoints = []string{fallback}
		return endpoints, endpointFingerprints
	}

	mainHostURL := ensureClusterEndpointURL(pve.Host)
	mainHostAlreadyIncluded := false
	for _, ep := range endpoints {
		if ep == mainHostURL {
			mainHostAlreadyIncluded = true
			break
		}
	}
	if !mainHostAlreadyIncluded && mainHostURL != "" {
		endpoints = append(endpoints, mainHostURL)
	}

	return endpoints, endpointFingerprints
}
