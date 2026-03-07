package monitoring

import (
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

	instances, mergedAny := config.ConsolidatePVEInstances(m.config.PVEInstances)
	if !mergedAny {
		return
	}
	m.config.PVEInstances = instances

	// Persist the consolidated configuration
	if m.persistence != nil {
		if err := m.persistence.SaveNodesConfig(m.config.PVEInstances, m.config.PBSInstances, m.config.PMGInstances); err != nil {
			log.Error().Err(err).Msg("failed to persist cluster consolidation")
		} else {
			log.Info().Msg("persisted consolidated cluster configuration")
		}
	}
}
