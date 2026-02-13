package monitoring

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// consolidateDuplicateClusters detects and merges duplicate cluster instances.
// When multiple PVE instances belong to the same Proxmox cluster (determined by ClusterName),
// they should be merged into a single instance with all endpoints combined.
// This prevents duplicate VMs/containers in the UI.
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

	// Find clusters that have duplicates
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

	if !mergedAny {
		return
	}

	// Remove duplicate instances (keeping only the primary for each cluster)
	var newInstances []config.PVEInstance
	seenClusters := make(map[string]bool)

	for _, instance := range m.config.PVEInstances {
		if instance.IsCluster && instance.ClusterName != "" {
			if seenClusters[instance.ClusterName] {
				log.Info().
					Str("cluster", instance.ClusterName).
					Str("instance", instance.Name).
					Msg("Removing duplicate cluster instance")
				continue // Skip duplicates
			}
			seenClusters[instance.ClusterName] = true
		}
		newInstances = append(newInstances, instance)
	}

	m.config.PVEInstances = newInstances

	// Persist the consolidated configuration
	if m.persistence != nil {
		if err := m.persistence.SaveNodesConfig(m.config.PVEInstances, m.config.PBSInstances, m.config.PMGInstances); err != nil {
			log.Error().Err(err).Msg("failed to persist cluster consolidation")
		} else {
			log.Info().Msg("persisted consolidated cluster configuration")
		}
	}
}
