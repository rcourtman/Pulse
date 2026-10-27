package monitoring

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
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
	return BuildPVEClusterClientEndpoints(pve, monitorDiscoveryConfig(m))
}

// PVEClusterEndpointURL exposes the same permitted dial target used by polling.
// It neither probes nor resolves a different address for the editor/test flow.
func PVEClusterEndpointURL(endpoint config.ClusterEndpoint, verifySSL bool, discoveryCfg config.DiscoveryConfig) string {
	return clusterEndpointRuntimeURL(endpoint, verifySSL, false, discoveryCfg)
}

// BuildPVEClusterClientEndpoints constructs polling trust, including explicit
// decisions for exact endpoint identities. It performs no network operation.
func BuildPVEClusterClientEndpoints(pve config.PVEInstance, discoveryCfg config.DiscoveryConfig) ([]string, map[string]string, []string) {
	endpoints := make([]string, 0, len(pve.ClusterEndpoints)+1)
	endpointFingerprints := make(map[string]string)
	var conflictingEndpoints []string
	conflicts := make(map[string]bool)
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

	// An explicit save reconciles only this exact authority/base path, including
	// equivalent URL spellings. Keep other member pins and conflicts untouched.
	for _, endpoint := range endpoints {
		fingerprint, known, err := proxmox.ResolveEndpointFingerprint(endpoint, pve.TLSFingerprintOverrides)
		if !known {
			continue
		}
		key, valid := proxmox.ClusterEndpointIdentity(endpoint)
		if !valid {
			continue
		}
		if err != nil {
			conflictingEndpoints = append(conflictingEndpoints, endpoint)
			continue
		}
		for saved := range endpointFingerprints {
			if savedKey, ok := proxmox.ClusterEndpointIdentity(saved); ok && savedKey == key {
				delete(endpointFingerprints, saved)
			}
		}
		filtered := conflictingEndpoints[:0]
		for _, conflict := range conflictingEndpoints {
			if conflictKey, ok := proxmox.ClusterEndpointIdentity(conflict); !ok || conflictKey != key {
				filtered = append(filtered, conflict)
			}
		}
		conflictingEndpoints = filtered
		endpointFingerprints[endpoint] = fingerprint
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
