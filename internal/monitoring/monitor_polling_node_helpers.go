package monitoring

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

func resolveNodeConnectionInfo(instanceCfg *config.PVEInstance, nodeName string) (string, string) {
	connectionHost := instanceCfg.Host
	guestURL := instanceCfg.GuestURL
	if instanceCfg.IsCluster && len(instanceCfg.ClusterEndpoints) > 0 {
		hasFingerprint := instanceCfg.Fingerprint != ""
		for _, ep := range instanceCfg.ClusterEndpoints {
			if strings.EqualFold(ep.NodeName, nodeName) {
				if effective := clusterEndpointEffectiveURL(ep, instanceCfg.VerifySSL, hasFingerprint); effective != "" {
					connectionHost = effective
				}
				if ep.GuestURL != "" {
					guestURL = ep.GuestURL
				}
				break
			}
		}
	}

	return connectionHost, guestURL
}

func (m *Monitor) determineNodeIDAndStatus(instanceName string, instanceCfg *config.PVEInstance, node proxmox.Node) (string, string) {
	// Apply grace period for node status to prevent flapping
	// For clustered nodes, use clusterName-nodeName as the ID to deduplicate
	// when the same cluster is registered via multiple entry points
	// (e.g., agent installed with --enable-proxmox on multiple cluster nodes)
	var nodeID string
	if instanceCfg.IsCluster && instanceCfg.ClusterName != "" {
		nodeID = instanceCfg.ClusterName + "-" + node.Node
	} else {
		nodeID = instanceName + "-" + node.Node
	}
	effectiveStatus := node.Status
	now := time.Now()

	m.mu.Lock()
	if strings.ToLower(node.Status) == "online" {
		// Node is online - update last-online timestamp
		m.nodeLastOnline[nodeID] = now
	} else {
		// Node is reported as offline - check grace period
		lastOnline, exists := m.nodeLastOnline[nodeID]
		if exists && now.Sub(lastOnline) < nodeOfflineGracePeriod {
			// Still within grace period - preserve online status
			effectiveStatus = "online"
			log.Debug().
				Str("instance", instanceName).
				Str("node", node.Node).
				Dur("timeSinceOnline", now.Sub(lastOnline)).
				Dur("gracePeriod", nodeOfflineGracePeriod).
				Msg("Node offline but within grace period - preserving online status")
		} else {
			// Grace period expired or never seen online - mark as offline
			if exists {
				log.Info().
					Str("instance", instanceName).
					Str("node", node.Node).
					Dur("timeSinceOnline", now.Sub(lastOnline)).
					Msg("Node offline and grace period expired - marking as offline")
			}
		}
	}
	m.mu.Unlock()

	return nodeID, effectiveStatus
}

func (m *Monitor) collectNodeTemperatureData(
	ctx context.Context,
	instanceName string,
	instanceCfg *config.PVEInstance,
	node proxmox.Node,
	modelNode *models.Node,
	prevInstanceNodes []models.Node,
	effectiveStatus string,
) {
	if modelNode == nil {
		return
	}

	// Collect temperature data via SSH (non-blocking, best effort)
	// Only attempt for online nodes when temperature monitoring is enabled
	// Check per-node setting first, fall back to global setting
	tempMonitoringEnabled := m.config.TemperatureMonitoringEnabled
	if instanceCfg.TemperatureMonitoringEnabled != nil {
		tempMonitoringEnabled = *instanceCfg.TemperatureMonitoringEnabled
	}
	if effectiveStatus == "online" && tempMonitoringEnabled {
		// First, check if there's a matching host agent with temperature data.
		// Host agent temperatures are preferred because they don't require SSH access.
		// Use getHostAgentTemperatureByID with the unique node ID to correctly handle
		// duplicate hostname scenarios (e.g., two "px1" nodes on different IPs).
		hostAgentTemp := m.getHostAgentTemperatureByID(modelNode.ID, node.Node)
		if hostAgentTemp != nil {
			log.Debug().
				Str("node", node.Node).
				Float64("cpuPackage", hostAgentTemp.CPUPackage).
				Float64("cpuMax", hostAgentTemp.CPUMax).
				Int("nvmeCount", len(hostAgentTemp.NVMe)).
				Msg("Using temperature data from host agent")
		}

		// If no host agent temp or we need additional data (SMART), try SSH/proxy collection
		var sshTemp *models.Temperature
		var err error
		if m.tempCollector != nil {
			// Temperature collection is best-effort - use a short timeout to avoid blocking node polling
			// Use context.Background() so the timeout is truly independent of the parent polling context
			// If SSH is slow or unresponsive, we'll preserve previous temperature data
			tempCtx, tempCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer tempCancel()

			// Determine SSH hostname to use (most robust approach):
			// Prefer the resolved host for this node, with cluster overrides when available.
			sshHost := modelNode.Host
			foundNodeEndpoint := false

			if modelNode.IsClusterMember && instanceCfg.IsCluster {
				// Try to find specific endpoint configuration for this node
				if len(instanceCfg.ClusterEndpoints) > 0 {
					hasFingerprint := instanceCfg.Fingerprint != ""
					for _, ep := range instanceCfg.ClusterEndpoints {
						if strings.EqualFold(ep.NodeName, node.Node) {
							if effective := clusterEndpointEffectiveURL(ep, instanceCfg.VerifySSL, hasFingerprint); effective != "" {
								sshHost = effective
								foundNodeEndpoint = true
							}
							break
						}
					}
				}

				// If no specific endpoint found, fall back to node name
				if !foundNodeEndpoint {
					sshHost = node.Node
					log.Debug().
						Str("node", node.Node).
						Str("instance", instanceCfg.Name).
						Msg("Node endpoint not found in cluster metadata - falling back to node name for temperature collection")
				}
			}

			if strings.TrimSpace(sshHost) == "" {
				sshHost = node.Node
			}

			// Skip SSH collection if we already have host agent data.
			skipSSHCollection := hostAgentTemp != nil

			if !skipSSHCollection {
				sshTemp, err = m.tempCollector.CollectTemperature(tempCtx, sshHost, node.Node)
				if err != nil && hostAgentTemp == nil {
					log.Debug().
						Str("node", node.Node).
						Str("sshHost", sshHost).
						Bool("isCluster", modelNode.IsClusterMember).
						Int("endpointCount", len(instanceCfg.ClusterEndpoints)).
						Msg("Temperature collection failed - check SSH access")
				}
			}

			// Debug: log SSH temp details before merge
			if sshTemp != nil {
				log.Debug().
					Str("node", node.Node).
					Bool("sshTempAvailable", sshTemp.Available).
					Bool("sshHasSMART", sshTemp.HasSMART).
					Int("sshSMARTCount", len(sshTemp.SMART)).
					Bool("sshHasNVMe", sshTemp.HasNVMe).
					Int("sshNVMeCount", len(sshTemp.NVMe)).
					Msg("SSH temperature data before merge")
			} else {
				log.Debug().
					Str("node", node.Node).
					Msg("SSH temperature data is nil")
			}
		}

		// Merge host agent and SSH temperatures
		temp := mergeTemperatureData(hostAgentTemp, sshTemp)

		if temp != nil && temp.Available {
			// Get the current CPU temperature (prefer package, fall back to max)
			currentTemp := temp.CPUPackage
			if currentTemp == 0 && temp.CPUMax > 0 {
				currentTemp = temp.CPUMax
			}

			// Find previous temperature data for this node to preserve min/max
			var prevTemp *models.Temperature
			for _, prevNode := range prevInstanceNodes {
				if prevNode.ID == modelNode.ID && prevNode.Temperature != nil {
					prevTemp = prevNode.Temperature
					break
				}
			}

			// Initialize or update min/max tracking
			if prevTemp != nil && prevTemp.CPUMin > 0 {
				// Preserve existing min/max and update if necessary
				temp.CPUMin = prevTemp.CPUMin
				temp.CPUMaxRecord = prevTemp.CPUMaxRecord
				temp.MinRecorded = prevTemp.MinRecorded
				temp.MaxRecorded = prevTemp.MaxRecorded

				// Update min if current is lower
				if currentTemp > 0 && currentTemp < temp.CPUMin {
					temp.CPUMin = currentTemp
					temp.MinRecorded = time.Now()
				}

				// Update max if current is higher
				if currentTemp > temp.CPUMaxRecord {
					temp.CPUMaxRecord = currentTemp
					temp.MaxRecorded = time.Now()
				}
			} else if currentTemp > 0 {
				// First reading - initialize min/max to current value
				temp.CPUMin = currentTemp
				temp.CPUMaxRecord = currentTemp
				temp.MinRecorded = time.Now()
				temp.MaxRecorded = time.Now()
			}

			modelNode.Temperature = temp

			// Determine source for logging
			tempSource := "ssh"
			if hostAgentTemp != nil && sshTemp == nil {
				tempSource = "host-agent"
			} else if hostAgentTemp != nil && sshTemp != nil {
				tempSource = "host-agent+ssh"
			}

			log.Debug().
				Str("node", node.Node).
				Str("source", tempSource).
				Float64("cpuPackage", temp.CPUPackage).
				Float64("cpuMax", temp.CPUMax).
				Float64("cpuMin", temp.CPUMin).
				Float64("cpuMaxRecord", temp.CPUMaxRecord).
				Int("nvmeCount", len(temp.NVMe)).
				Msg("Collected temperature data")
		} else {
			// Temperature data returned but not available (temp != nil && !temp.Available)
			// OR no temperature data from any source - preserve previous temperature if available
			// This prevents the temperature column from flickering when collection temporarily fails
			var prevTemp *models.Temperature
			for _, prevNode := range prevInstanceNodes {
				if prevNode.ID == modelNode.ID && prevNode.Temperature != nil && prevNode.Temperature.Available {
					prevTemp = prevNode.Temperature
					break
				}
			}

			if prevTemp != nil {
				// Clone the previous temperature to avoid modifying historical data
				preserved := *prevTemp
				preserved.LastUpdate = prevTemp.LastUpdate // Keep original update time to indicate staleness
				modelNode.Temperature = &preserved
				log.Debug().
					Str("node", node.Node).
					Bool("isCluster", modelNode.IsClusterMember).
					Float64("cpuPackage", preserved.CPUPackage).
					Time("lastUpdate", preserved.LastUpdate).
					Msg("Preserved previous temperature data (current collection failed or unavailable)")
			} else {
				log.Debug().
					Str("node", node.Node).
					Bool("isCluster", modelNode.IsClusterMember).
					Msg("No temperature data available (collection failed, no previous data to preserve)")
			}
		}
	}
}

func (m *Monitor) applyNodePendingUpdates(ctx context.Context, instanceName string, client PVEClientInterface, node proxmox.Node, nodeID string, effectiveStatus string, modelNode *models.Node) {
	if modelNode == nil {
		return
	}

	// Poll pending apt updates (less frequently - every 30 minutes)
	// Only for online nodes to avoid wasting API calls on offline nodes
	if effectiveStatus == "online" {
		now := time.Now()
		m.mu.RLock()
		if m.nodePendingUpdatesCache == nil {
			m.mu.RUnlock()
			m.mu.Lock()
			if m.nodePendingUpdatesCache == nil {
				m.nodePendingUpdatesCache = make(map[string]pendingUpdatesCache)
			}
			m.mu.Unlock()
			m.mu.RLock()
		}
		cached, hasCached := m.nodePendingUpdatesCache[nodeID]
		m.mu.RUnlock()

		if !hasCached || now.Sub(cached.checkedAt) >= pendingUpdatesCacheTTL {
			// Time to check for updates
			pendingPkgs, err := client.GetNodePendingUpdates(ctx, node.Node)
			if err != nil {
				// API call failed - preserve cached value if available, don't spam logs
				log.Debug().
					Err(err).
					Str("node", node.Node).
					Str("instance", instanceName).
					Msg("Could not check pending apt updates (may require Sys.Audit permission)")
				if hasCached {
					modelNode.PendingUpdates = cached.count
					modelNode.PendingUpdatesCheckedAt = cached.checkedAt
				}
			} else {
				updateCount := len(pendingPkgs)
				modelNode.PendingUpdates = updateCount
				modelNode.PendingUpdatesCheckedAt = now

				// Cache the result
				m.mu.Lock()
				m.nodePendingUpdatesCache[nodeID] = pendingUpdatesCache{
					count:     updateCount,
					checkedAt: now,
				}
				m.mu.Unlock()

				log.Debug().
					Str("node", node.Node).
					Str("instance", instanceName).
					Int("pendingUpdates", updateCount).
					Msg("Checked pending apt updates")
			}
		} else {
			// Use cached value
			modelNode.PendingUpdates = cached.count
			modelNode.PendingUpdatesCheckedAt = cached.checkedAt
		}
	}
}

func (m *Monitor) recordNodePollMetrics(instanceName string, node proxmox.Node, modelNode *models.Node, nodeStart time.Time) {
	if m.pollMetrics == nil || modelNode == nil {
		return
	}

	nodeNameLabel := strings.TrimSpace(node.Node)
	if nodeNameLabel == "" {
		nodeNameLabel = strings.TrimSpace(modelNode.DisplayName)
	}
	if nodeNameLabel == "" {
		nodeNameLabel = "unknown-node"
	}

	success := true
	nodeErrReason := ""
	health := strings.ToLower(strings.TrimSpace(modelNode.ConnectionHealth))
	if health != "" && health != "healthy" {
		success = false
		nodeErrReason = fmt.Sprintf("connection health %s", health)
	}

	status := strings.ToLower(strings.TrimSpace(modelNode.Status))
	if success && status != "" && status != "online" {
		success = false
		nodeErrReason = fmt.Sprintf("status %s", status)
	}

	var nodeErr error
	if !success {
		if nodeErrReason == "" {
			nodeErrReason = "unknown node error"
		}
		nodeErr = stderrors.New(nodeErrReason)
	}

	m.pollMetrics.RecordNodeResult(NodePollResult{
		InstanceName: instanceName,
		InstanceType: "pve",
		NodeName:     nodeNameLabel,
		Success:      success,
		Error:        nodeErr,
		StartTime:    nodeStart,
		EndTime:      time.Now(),
	})
}
