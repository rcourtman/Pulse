package monitoring

import (
	"context"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) detectClusterMembership(ctx context.Context, instanceName string, instanceCfg *config.PVEInstance, client PVEClientInterface) {
	if instanceCfg.IsCluster {
		return
	}

	// Check every 5 minutes if this is actually a cluster
	if time.Since(m.lastClusterCheck[instanceName]) <= 5*time.Minute {
		return
	}

	m.lastClusterCheck[instanceName] = time.Now()

	// Try to detect if this is actually a cluster
	isActuallyCluster, checkErr := client.IsClusterMember(ctx)
	if checkErr != nil || !isActuallyCluster {
		return
	}

	// This node is actually part of a cluster!
	log.Info().
		Str("instance", instanceName).
		Msg("Detected that standalone node is actually part of a cluster - updating configuration")

	// Update the configuration
	for i := range m.config.PVEInstances {
		if m.config.PVEInstances[i].Name == instanceName {
			m.config.PVEInstances[i].IsCluster = true
			// Note: We can't get the cluster name here without direct client access
			// It will be detected on the next configuration update
			log.Info().
				Str("instance", instanceName).
				Msg("Marked node as cluster member - cluster name will be detected on next update")

			// Save the updated configuration
			if m.persistence != nil {
				if err := m.persistence.SaveNodesConfig(m.config.PVEInstances, m.config.PBSInstances, m.config.PMGInstances); err != nil {
					log.Warn().Err(err).Msg("failed to persist updated node configuration")
				}
			}
			break
		}
	}
}

func (m *Monitor) updateClusterEndpointStatus(instanceName string, instanceCfg *config.PVEInstance, client PVEClientInterface, modelNodes []models.Node) {
	if !instanceCfg.IsCluster || len(instanceCfg.ClusterEndpoints) == 0 {
		return
	}

	// Create a map of online nodes from our polling results
	onlineNodes := make(map[string]bool)
	for _, node := range modelNodes {
		// Node is online if we successfully got its data
		onlineNodes[node.Name] = node.Status == "online"
	}

	// Get Pulse connectivity status from ClusterClient if available
	var pulseHealth map[string]proxmox.EndpointHealth
	if clusterClient, ok := client.(*proxmox.ClusterClient); ok {
		pulseHealth = clusterClient.GetHealthStatusWithErrors()
	}

	// Update the online status for each cluster endpoint
	hasFingerprint := instanceCfg.Fingerprint != ""
	for i := range instanceCfg.ClusterEndpoints {
		if online, exists := onlineNodes[instanceCfg.ClusterEndpoints[i].NodeName]; exists {
			instanceCfg.ClusterEndpoints[i].Online = online
			if online {
				instanceCfg.ClusterEndpoints[i].LastSeen = time.Now()
			}
		}

		// Update Pulse connectivity status
		if pulseHealth != nil {
			// Try to find the endpoint in the health map by matching the effective URL
			endpointURL := clusterEndpointEffectiveURL(instanceCfg.ClusterEndpoints[i], instanceCfg.VerifySSL, hasFingerprint)
			if health, exists := pulseHealth[endpointURL]; exists {
				reachable := health.Healthy
				instanceCfg.ClusterEndpoints[i].PulseReachable = &reachable
				if !health.LastCheck.IsZero() {
					instanceCfg.ClusterEndpoints[i].LastPulseCheck = &health.LastCheck
				}
				instanceCfg.ClusterEndpoints[i].PulseError = health.LastError
			}
		}
	}

	// Update the config with the new online status
	// This is needed so the UI can reflect the current status
	for idx, cfg := range m.config.PVEInstances {
		if cfg.Name == instanceName {
			m.config.PVEInstances[idx].ClusterEndpoints = instanceCfg.ClusterEndpoints
			break
		}
	}
}
