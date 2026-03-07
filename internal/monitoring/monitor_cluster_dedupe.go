package monitoring

import (
	"net"
	"net/url"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// consolidateDuplicateClusters detects and merges duplicate cluster instances.
// When multiple PVE instances belong to the same Proxmox cluster (determined by ClusterName),
// they should be merged into a single instance with all endpoints combined.
// It also removes standalone PVE instances whose configured endpoint is already
// represented by a cluster endpoint. This prevents duplicate polling upstream.
func (m *Monitor) consolidateDuplicateClusters() {
	if m == nil || m.config == nil || len(m.config.PVEInstances) < 2 {
		return
	}

	// Group instances by cluster name
	clusterGroups := make(map[string][]int) // clusterName -> indices of instances
	for i, instance := range m.config.PVEInstances {
		if instance.IsCluster && instance.ClusterName != "" {
			clusterGroups[instance.ClusterName] = append(clusterGroups[instance.ClusterName], i)
		}
	}

	// Find clusters that have duplicates.
	var mergedAny bool
	for clusterName, indices := range clusterGroups {
		if len(indices) < 2 {
			continue // No duplicates for this cluster
		}

		log.Warn().
			Str("cluster", clusterName).
			Int("duplicates", len(indices)).
			Msg("Detected duplicate cluster instances - consolidating")

		// Keep the first instance and merge all others into it
		primaryIdx := indices[0]
		primary := &m.config.PVEInstances[primaryIdx]

		// Build a set of existing endpoint node names
		existingEndpoints := make(map[string]bool)
		for _, ep := range primary.ClusterEndpoints {
			existingEndpoints[ep.NodeName] = true
		}

		// Merge endpoints from all duplicate instances
		for _, dupIdx := range indices[1:] {
			duplicate := m.config.PVEInstances[dupIdx]
			log.Info().
				Str("cluster", clusterName).
				Str("primary", primary.Name).
				Str("duplicate", duplicate.Name).
				Msg("Merging duplicate cluster instance")

			for _, ep := range duplicate.ClusterEndpoints {
				if !existingEndpoints[ep.NodeName] {
					primary.ClusterEndpoints = append(primary.ClusterEndpoints, ep)
					existingEndpoints[ep.NodeName] = true
					log.Info().
						Str("cluster", clusterName).
						Str("endpoint", ep.NodeName).
						Msg("Added endpoint from duplicate instance")
				}
			}
		}

		mergedAny = true
	}

	if mergedAny {
		m.config.PVEInstances = dedupeClusterInstances(m.config.PVEInstances)
	}

	// Remove standalone instances that explicitly target a node already covered
	// by a cluster endpoint.
	instancesAfterStandaloneMerge, standaloneMerged := mergeStandaloneInstancesIntoClusters(m.config.PVEInstances)
	if standaloneMerged {
		m.config.PVEInstances = instancesAfterStandaloneMerge
		mergedAny = true
	}

	if !mergedAny {
		return
	}

	// Persist the consolidated configuration
	if m.persistence != nil {
		if err := m.persistence.SaveNodesConfig(m.config.PVEInstances, m.config.PBSInstances, m.config.PMGInstances); err != nil {
			log.Error().Err(err).Msg("failed to persist cluster consolidation")
		} else {
			log.Info().Msg("persisted consolidated cluster configuration")
		}
	}
}

func dedupeClusterInstances(instances []config.PVEInstance) []config.PVEInstance {
	var out []config.PVEInstance
	seenClusters := make(map[string]bool)

	for _, instance := range instances {
		if instance.IsCluster && instance.ClusterName != "" {
			if seenClusters[instance.ClusterName] {
				log.Info().
					Str("cluster", instance.ClusterName).
					Str("instance", instance.Name).
					Msg("Removing duplicate cluster instance")
				continue
			}
			seenClusters[instance.ClusterName] = true
		}
		out = append(out, instance)
	}

	return out
}

func normalizePVEEndpointIdentity(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	if !strings.HasPrefix(strings.ToLower(value), "http://") && !strings.HasPrefix(strings.ToLower(value), "https://") {
		value = "https://" + value
	}

	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return ""
	}

	host := strings.TrimSpace(strings.ToLower(parsed.Hostname()))
	if host == "" {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	}

	port := parsed.Port()
	if port == "" {
		port = config.DefaultPVEPort
	}

	return net.JoinHostPort(host, port)
}

func clusterEndpointIdentityKeys(endpoint config.ClusterEndpoint) []string {
	keys := make([]string, 0, 3)
	if key := normalizePVEEndpointIdentity(endpoint.Host); key != "" {
		keys = append(keys, key)
	}
	if key := normalizePVEEndpointIdentity(endpoint.IP); key != "" {
		keys = append(keys, key)
	}
	if key := normalizePVEEndpointIdentity(endpoint.IPOverride); key != "" {
		keys = append(keys, key)
	}
	return keys
}

func mergeStandaloneInstancesIntoClusters(instances []config.PVEInstance) ([]config.PVEInstance, bool) {
	type endpointRef struct {
		clusterIdx  int
		endpointIdx int
	}

	endpointOwners := make(map[string]endpointRef)
	ambiguousKeys := make(map[string]struct{})
	registerKey := func(key string, ref endpointRef) {
		if key == "" {
			return
		}
		if _, ambiguous := ambiguousKeys[key]; ambiguous {
			return
		}
		if existing, ok := endpointOwners[key]; ok && existing != ref {
			delete(endpointOwners, key)
			ambiguousKeys[key] = struct{}{}
			return
		}
		endpointOwners[key] = ref
	}

	for clusterIdx := range instances {
		if !instances[clusterIdx].IsCluster {
			continue
		}
		for endpointIdx, endpoint := range instances[clusterIdx].ClusterEndpoints {
			for _, key := range clusterEndpointIdentityKeys(endpoint) {
				registerKey(key, endpointRef{clusterIdx: clusterIdx, endpointIdx: endpointIdx})
			}
		}
	}

	if len(endpointOwners) == 0 {
		return instances, false
	}

	out := make([]config.PVEInstance, len(instances))
	copy(out, instances)
	removeStandalone := make([]bool, len(out))
	mergedAny := false
	for idx, instance := range out {
		if instance.IsCluster {
			continue
		}

		key := normalizePVEEndpointIdentity(instance.Host)
		if key == "" {
			continue
		}
		if _, ambiguous := ambiguousKeys[key]; ambiguous {
			continue
		}

		ref, ok := endpointOwners[key]
		if !ok {
			continue
		}

		cluster := &out[ref.clusterIdx]
		if ref.endpointIdx >= len(cluster.ClusterEndpoints) {
			continue
		}

		endpoint := &cluster.ClusterEndpoints[ref.endpointIdx]
		if endpoint.GuestURL == "" && strings.TrimSpace(instance.GuestURL) != "" {
			endpoint.GuestURL = strings.TrimSpace(instance.GuestURL)
		}
		if endpoint.Fingerprint == "" && strings.TrimSpace(instance.Fingerprint) != "" {
			endpoint.Fingerprint = strings.TrimSpace(instance.Fingerprint)
		}
		if endpoint.Host == "" && strings.TrimSpace(instance.Host) != "" {
			endpoint.Host = strings.TrimSpace(instance.Host)
		}

		log.Warn().
			Str("standalone", instance.Name).
			Str("standaloneHost", instance.Host).
			Str("cluster", cluster.Name).
			Str("node", endpoint.NodeName).
			Msg("Detected standalone PVE instance already covered by cluster endpoint - consolidating")
		removeStandalone[idx] = true
		mergedAny = true
	}

	if !mergedAny {
		return instances, false
	}

	keep := make([]config.PVEInstance, 0, len(out))
	for idx, instance := range out {
		if removeStandalone[idx] {
			continue
		}
		keep = append(keep, instance)
	}
	return keep, true
}
