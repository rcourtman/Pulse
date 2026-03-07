package monitoring

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

	if !m.normalizePVEConfigState() {
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

func (m *Monitor) normalizePVEConfigState() bool {
	if m == nil || m.config == nil || len(m.config.PVEInstances) < 2 {
		return false
	}

	beforeNames := make(map[string]struct{}, len(m.config.PVEInstances))
	for _, instance := range m.config.PVEInstances {
		if name := strings.TrimSpace(instance.Name); name != "" {
			beforeNames[name] = struct{}{}
		}
	}

	instances, changed := config.ConsolidatePVEInstances(m.config.PVEInstances)
	if !changed {
		return false
	}
	m.config.PVEInstances = instances

	remainingNames := make(map[string]struct{}, len(instances))
	for _, instance := range instances {
		if name := strings.TrimSpace(instance.Name); name != "" {
			remainingNames[name] = struct{}{}
		}
	}

	for name := range beforeNames {
		if _, ok := remainingNames[name]; ok {
			continue
		}
		m.retirePVEInstanceRuntime(name)
	}

	return true
}

func (m *Monitor) retirePVEInstanceRuntime(instanceName string) {
	instanceName = strings.TrimSpace(instanceName)
	if m == nil || instanceName == "" {
		return
	}

	if m.taskQueue != nil {
		m.taskQueue.Remove(InstanceTypePVE, instanceName)
	}
	if m.deadLetterQueue != nil {
		m.deadLetterQueue.Remove(InstanceTypePVE, instanceName)
	}

	key := schedulerKey(InstanceTypePVE, instanceName)

	m.mu.Lock()
	delete(m.pveClients, instanceName)
	delete(m.lastClusterCheck, instanceName)
	delete(m.lastPhysicalDiskPoll, instanceName)
	delete(m.lastPVEBackupPoll, instanceName)
	delete(m.backupPermissionWarnings, instanceName)
	delete(m.authFailures, instanceName)
	delete(m.authFailures, string(InstanceTypePVE)+"-"+instanceName)
	delete(m.lastAuthAttempt, instanceName)
	delete(m.lastAuthAttempt, string(InstanceTypePVE)+"-"+instanceName)
	delete(m.circuitBreakers, key)
	delete(m.failureCounts, key)
	delete(m.lastOutcome, key)
	delete(m.instanceInfoCache, key)
	delete(m.pollStatusMap, key)
	delete(m.dlqInsightMap, key)
	for nodeID := range m.nodeLastOnline {
		if strings.HasPrefix(nodeID, instanceName+"-") {
			delete(m.nodeLastOnline, nodeID)
		}
	}
	for nodeID := range m.nodePendingUpdatesCache {
		if strings.HasPrefix(nodeID, instanceName+"-") {
			delete(m.nodePendingUpdatesCache, nodeID)
		}
	}
	m.mu.Unlock()

	m.diagMu.Lock()
	for key := range m.nodeSnapshots {
		if strings.HasPrefix(key, instanceName+"|") {
			delete(m.nodeSnapshots, key)
		}
	}
	for key := range m.guestSnapshots {
		if strings.HasPrefix(key, instanceName+"|") {
			delete(m.guestSnapshots, key)
		}
	}
	m.diagMu.Unlock()

	m.guestMetadataMu.Lock()
	for key := range m.guestMetadataCache {
		if strings.HasPrefix(key, instanceName+"|") {
			delete(m.guestMetadataCache, key)
		}
	}
	m.guestMetadataMu.Unlock()

	m.guestMetadataLimiterMu.Lock()
	for key := range m.guestMetadataLimiter {
		if strings.HasPrefix(key, instanceName+"|") {
			delete(m.guestMetadataLimiter, key)
		}
	}
	m.guestMetadataLimiterMu.Unlock()

	if m.state != nil {
		m.state.UpdateNodesForInstance(instanceName, []models.Node{})
		m.state.UpdateVMsForInstance(instanceName, []models.VM{})
		m.state.UpdateContainersForInstance(instanceName, []models.Container{})
		m.state.UpdateStorageForInstance(instanceName, []models.Storage{})
		m.state.UpdatePhysicalDisks(instanceName, []models.PhysicalDisk{})
		m.state.UpdateCephClustersForInstance(instanceName, []models.CephCluster{})
		m.state.UpdateBackupTasksForInstance(instanceName, []models.BackupTask{})
		m.state.UpdateStorageBackupsForInstance(instanceName, []models.StorageBackup{})
		m.state.UpdateGuestSnapshotsForInstance(instanceName, []models.GuestSnapshot{})
		m.state.UpdateReplicationJobsForInstance(instanceName, []models.ReplicationJob{})
	}
	m.removeProviderConnectionHealth(InstanceTypePVE, instanceName)

	log.Info().
		Str("instance", instanceName).
		Msg("Retired consolidated PVE instance from runtime state")
}
