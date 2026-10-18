package monitoring

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) buildClusterEndpointsForInit(pve config.PVEInstance) ([]string, map[string]string, []string) {
	return m.buildClusterClientEndpoints(pve)
}

func (m *Monitor) buildClusterEndpointsForReconnect(pve config.PVEInstance) ([]string, map[string]string, []string) {
	return m.buildClusterClientEndpoints(pve)
}

// buildClusterClientEndpoints preserves the configured connection as the
// cluster API authority. Proxmox can advertise corosync/member addresses that
// are valid inside the cluster but unreachable from the Pulse container; those
// addresses remain useful failover and reachability evidence, but they must
// never displace the URL the operator proved reachable when adding the cluster.
func (m *Monitor) buildClusterClientEndpoints(pve config.PVEInstance) ([]string, map[string]string, []string) {
	endpoints := make([]string, 0, len(pve.ClusterEndpoints)+1)
	endpointFingerprints := make(map[string]string)
	var conflictingEndpoints []string
	conflicts := make(map[string]bool)
	discoveryCfg := monitorDiscoveryConfig(m)
	hasFingerprint := pve.Fingerprint != ""
	seen := make(map[string]struct{}, len(pve.ClusterEndpoints)+1)

	addEndpoint := func(endpoint string) bool {
		if endpoint == "" {
			return false
		}
		if _, exists := seen[endpoint]; exists {
			return false
		}
		seen[endpoint] = struct{}{}
		endpoints = append(endpoints, endpoint)
		return true
	}

	configuredAuthority := ensureClusterEndpointURL(pve.Host)
	addEndpoint(configuredAuthority)

	failoverCount := 0
	for _, ep := range pve.ClusterEndpoints {
		host := clusterEndpointRuntimeURL(ep, pve.VerifySSL, hasFingerprint, discoveryCfg)
		if host == "" {
			log.Warn().
				Str("node", ep.NodeName).
				Msg("Skipping cluster member endpoint with no allowed host/IP")
			continue
		}
		if addEndpoint(host) {
			failoverCount++
		}
		if ep.Fingerprint != "" {
			if previous, exists := endpointFingerprints[host]; exists {
				if strings.ToLower(strings.ReplaceAll(previous, ":", "")) != strings.ToLower(strings.ReplaceAll(ep.Fingerprint, ":", "")) && !conflicts[host] {
					// The map cannot retain two values for the exact same URL.
					// Carry the conflict separately; a later repeated row must
					// not erase it or choose the certificate to trust.
					conflicts[host] = true
					conflictingEndpoints = append(conflictingEndpoints, host)
				}
			} else {
				endpointFingerprints[host] = ep.Fingerprint
			}
		}
	}

	if configuredAuthority != "" {
		log.Info().
			Str("instance", pve.Name).
			Str("authority", configuredAuthority).
			Int("memberFailovers", failoverCount).
			Msg("Using configured Proxmox cluster connection as API authority")
	} else if len(endpoints) > 0 {
		log.Warn().
			Str("instance", pve.Name).
			Str("authority", endpoints[0]).
			Int("memberFailovers", len(endpoints)-1).
			Msg("Configured Proxmox cluster URL is empty; using first discovered member as API authority")
	}

	return endpoints, endpointFingerprints, conflictingEndpoints
}
