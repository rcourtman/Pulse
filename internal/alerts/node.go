package alerts

import (
	"fmt"
	"strings"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// CheckNode checks a node against thresholds
func (m *Manager) CheckNode(node models.Node) {
	// Cache display name so all alerts (including guest alerts on this node) can resolve it.
	m.UpdateNodeDisplayName(node.Instance, node.Name, node.DisplayName)
	for _, alias := range node.NativeNameAliases {
		m.UpdateNodeDisplayName(node.Instance, alias, node.DisplayName)
	}

	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		return
	}
	if allDisabled, _ := m.alertPolicyTypeSwitchesNoLock("node"); allDisabled {
		m.mu.RUnlock()
		// Clear any existing node alerts when all node alerts are disabled
		m.mu.Lock()
		// Clear offline tracking
		// Clear all possible node alert types
		alertTypes := []string{"cpu", "memory", "disk", "temperature"}
		for _, alertType := range alertTypes {
			alertID := canonicalMetricStateID(node.ID, alertType)
			if m.hasActiveAlertNoLock(alertID) {
				m.clearAlertNoLock(alertID)
				log.Info().
					Str("alertID", alertID).
					Str("node", node.Name).
					Msg("Cleared node alert - all node alerts disabled")
			}
		}
		// Clear offline alert
		offlineAlertID := canonicalConnectivityStateID(node.ID)
		if m.hasActiveAlertNoLock(offlineAlertID) {
			m.clearAlertNoLock(offlineAlertID)
			log.Info().
				Str("alertID", offlineAlertID).
				Str("node", node.Name).
				Msg("Cleared offline alert - all node alerts disabled")
		}
		m.mu.Unlock()
		return
	}
	_, disableNodesOffline := m.alertPolicyTypeSwitchesNoLock("node")
	thresholds := m.resolveResourceThresholds("node", node.ID)
	m.mu.RUnlock()

	if thresholds.Disabled {
		m.mu.Lock()
		m.mu.Unlock()
		for _, alertID := range []string{
			canonicalMetricStateID(node.ID, "cpu"),
			canonicalMetricStateID(node.ID, "memory"),
			canonicalMetricStateID(node.ID, "disk"),
			canonicalMetricStateID(node.ID, "temperature"),
			canonicalConnectivityStateID(node.ID),
		} {
			m.clearAlert(alertID)
		}
		return
	}

	if disableNodesOffline || thresholds.DisableConnectivity {
		// Clear tracking and any existing offline alerts when globally disabled
		m.mu.Lock()
		m.mu.Unlock()
		m.clearAlert(canonicalConnectivityStateID(node.ID))
	} else {
		// CRITICAL: Check if node is offline first
		if node.Status == "offline" || node.ConnectionHealth == "error" || node.ConnectionHealth == "failed" {
			m.checkNodeOffline(node)

			// Clear resource alerts if node is offline/unreachable.
			// This prevents stale alerts from persisting when we can't get new data.
			metrics := []string{"cpu", "memory", "disk", "temperature"}
			for _, metric := range metrics {
				m.clearAlert(canonicalMetricStateID(node.ID, metric))
			}
		} else {
			// Clear any existing offline alert if node is back online
			m.clearNodeOfflineAlert(node)

			// Check each metric (only if node is online and reachable)
			// Check for host agent deduplication: if a host agent is linked to this node,
			// CPU, memory and disk usage alerts belong to the agent resource (CheckHost
			// evaluates them), so the node releases its own copies instead of alerting twice.
			if m.hasHostAgentForNode(node.ID) {
				m.releaseNodeMetricAlerts(node, "cpu", "memory", "disk")
			} else {
				var memoryMetric *UnifiedResourceMetric
				if node.Memory.HasKnownUsage() {
					memoryMetric = &UnifiedResourceMetric{Percent: node.Memory.Usage}
				}
				m.evaluateUnifiedMetrics(&UnifiedResourceInput{
					ID:       node.ID,
					Type:     "node",
					Name:     node.Name,
					Node:     node.Name,
					Instance: node.Instance,
					CPU:      &UnifiedResourceMetric{Percent: node.CPU * 100},
					Memory:   memoryMetric,
					Disk:     &UnifiedResourceMetric{Percent: node.Disk.Usage},
				}, thresholds, nil, "temperature") // checkNodeTemperature owns its observation gaps.
			}

			// CPU temperature stays with the node even when a host agent runs on it:
			// CheckHost has no CPU temperature metric, and the node poll already
			// merges the agent's sensor readings into node.Temperature.
			m.checkNodeTemperature(node, thresholds.Temperature)
		}
	}
}

// checkNodeTemperature evaluates the node CPU temperature alert.
// A disabled threshold (nil or trigger 0) is always passed through so any
// existing alert is cleared. Otherwise a missing CPU reading is not evidence:
// feeding 0°C would resolve an open alert without proving the node cooled down,
// so the open alert is kept and only its timing runs are interrupted.
func (m *Manager) checkNodeTemperature(node models.Node, threshold *HysteresisThreshold) {
	var temp float64
	if node.Temperature != nil && node.Temperature.Available {
		// Use CPU package temp if available, otherwise use max core temp
		temp = node.Temperature.CPUPackage
		if temp == 0 {
			temp = node.Temperature.CPUMax
		}
	}
	spec, err := buildCanonicalMetricSpec(node.ID, node.Name, unifiedresources.ResourceType("node"), "temperature", threshold)
	if err != nil {
		log.Warn().
			Err(err).
			Str("resourceID", node.ID).
			Str("node", node.Name).
			Msg("Skipping invalid canonical node temperature metric spec")
		return
	}
	if temp <= 0 && !spec.Disabled {
		m.interruptMetricRun(spec)
		return
	}
	m.checkMetricWithCanonicalSpec(spec, node.Name, node.Name, node.Instance, "node", temp, threshold, nil)
}

// releaseNodeMetricAlerts stops node-side evaluation of the given metrics the
// same way a disabled threshold does: any pending run is dropped and any open
// node alert is resolved. Without this, a node alert that was open when a host
// agent registered would stay frozen until the agent went offline.
func (m *Manager) releaseNodeMetricAlerts(node models.Node, metrics ...string) {
	for _, metric := range metrics {
		spec, err := buildCanonicalMetricSpec(node.ID, node.Name, unifiedresources.ResourceType("node"), metric, nil)
		if err != nil {
			log.Warn().
				Err(err).
				Str("resourceID", node.ID).
				Str("node", node.Name).
				Str("metric", metric).
				Msg("Skipping invalid canonical node metric spec")
			continue
		}
		m.checkMetricWithCanonicalSpec(spec, node.Name, node.Name, node.Instance, "node", 0, nil, nil)
	}
}

// registerHostAgentNodeLink records which Proxmox node a reporting host agent
// is linked to. While the link holds, the agent resource owns that machine's
// CPU, memory and disk usage alerts. The link is the monitor's identity decision
// (automatic or operator-set), so an agent whose hostname merely matches a node
// name in another instance, or one an operator unlinked, never makes a node
// release its alerts, and an agent reporting an FQDN still dedups its node.
func (m *Manager) registerHostAgentNodeLink(host models.Host) {
	hostID := strings.TrimSpace(host.ID)
	if hostID == "" {
		return
	}
	linkedNodeID := strings.TrimSpace(host.LinkedNodeID)
	m.mu.Lock()
	previous, existed := m.hostAgentNodeLinks[hostID]
	if linkedNodeID == "" {
		delete(m.hostAgentNodeLinks, hostID)
	} else {
		m.hostAgentNodeLinks[hostID] = linkedNodeID
	}
	m.mu.Unlock()

	if !existed || previous != linkedNodeID {
		log.Debug().
			Str("hostID", hostID).
			Str("hostname", host.Hostname).
			Str("linkedNodeID", linkedNodeID).
			Msg("Updated host agent node link for deduplication")
	}
}

// unregisterHostAgentNodeLink removes a host agent from deduplication tracking,
// so its linked node evaluates its own usage alerts again.
func (m *Manager) unregisterHostAgentNodeLink(hostID string) {
	hostID = strings.TrimSpace(hostID)
	if hostID == "" {
		return
	}
	m.mu.Lock()
	linkedNodeID, existed := m.hostAgentNodeLinks[hostID]
	delete(m.hostAgentNodeLinks, hostID)
	m.mu.Unlock()

	if existed {
		log.Debug().
			Str("hostID", hostID).
			Str("linkedNodeID", linkedNodeID).
			Msg("Removed host agent node link from deduplication")
	}
}

// hasHostAgentForNode reports whether a reporting host agent is linked to the
// given Proxmox node, in which case the node's usage alerts belong to the agent.
func (m *Manager) hasHostAgentForNode(nodeID string) bool {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, linkedNodeID := range m.hostAgentNodeLinks {
		if linkedNodeID == nodeID {
			return true
		}
	}
	return false
}

// UpdateNodeDisplayName caches the display name for a node/host so alerts
// can resolve it without needing the full model object.
func nodeDisplayNameCacheKey(instance, name string) string {
	return strings.TrimSpace(instance) + "\x00" + strings.TrimSpace(name)
}

func (m *Manager) UpdateNodeDisplayName(parts ...string) {
	var instance, name, displayName string
	switch len(parts) {
	case 2:
		name, displayName = parts[0], parts[1]
	case 3:
		instance, name, displayName = parts[0], parts[1], parts[2]
	default:
		return
	}

	instance = strings.TrimSpace(instance)
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	displayName = strings.TrimSpace(displayName)
	m.mu.Lock()
	if instance != "" {
		key := nodeDisplayNameCacheKey(instance, name)
		if displayName != "" && displayName != name {
			m.instanceNodeDisplayNames[key] = displayName
		} else {
			delete(m.instanceNodeDisplayNames, key)
		}
	} else {
		if displayName != "" && displayName != name {
			m.nodeDisplayNames[name] = displayName
		} else {
			delete(m.nodeDisplayNames, name)
		}
	}
	m.mu.Unlock()
}

// resolveNodeDisplayName returns the cached display name for a node, or empty
// string if none is set. Caller must hold m.mu (read or write).
func (m *Manager) resolveNodeDisplayName(instance, node string) string {
	if instance = strings.TrimSpace(instance); instance != "" {
		if displayName, ok := m.instanceNodeDisplayNames[nodeDisplayNameCacheKey(instance, node)]; ok {
			return displayName
		}
	}
	return m.nodeDisplayNames[node]
}

// checkNodeOffline creates an alert for offline nodes after confirmation
func (m *Manager) checkNodeOffline(node models.Node) {
	alertID := fmt.Sprintf("node-offline-%s", node.ID)
	correlation := NewSharedSystemAlertCorrelation(
		"pve:"+strings.TrimSpace(node.Instance),
		AlertCorrelationRoleSupporting,
		"proxmox-node-membership",
	)

	m.mu.Lock()
	m.mu.Unlock()

	thresholds := m.resolveResourceThresholds("node", node.ID)
	spec, err := buildCanonicalConnectivitySpec(node.ID, node.Name, unifiedresources.ResourceType("node"), AlertLevelCritical, 3, thresholds.Disabled || thresholds.DisableConnectivity)
	if err != nil {
		log.Warn().
			Err(err).
			Str("node", node.Name).
			Str("nodeID", node.ID).
			Msg("Skipping invalid canonical node connectivity spec")
		return
	}

	_, _ = m.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
		Spec:         spec,
		Evidence:     alertspecs.AlertEvidence{ObservedAt: time.Now(), Connectivity: &alertspecs.ConnectivityEvidence{Signal: "status", Connected: false}},
		AlertID:      alertID,
		AlertType:    "connectivity",
		ResourceID:   node.ID,
		ResourceName: node.Name,
		Node:         node.Name,
		Instance:     node.Instance,
		Message:      fmt.Sprintf("Node '%s' is offline", node.Name),
		Correlation:  correlation,
		Metadata: map[string]interface{}{
			"resourceType":     "node",
			"status":           node.Status,
			"connectionHealth": node.ConnectionHealth,
		},
		AddToRecent:   true,
		AddToHistory:  true,
		RateLimit:     true,
		DispatchAsync: false,
	})
}

// clearNodeOfflineAlert removes offline alert when node comes back online
func (m *Manager) clearNodeOfflineAlert(node models.Node) {
	alertID := canonicalConnectivityStateID(node.ID)

	m.mu.Lock()
	defer m.mu.Unlock()
	defer m.shadowObserveRecoveryNoLock(node.ID, canonicalConnectivitySpecID(node.ID), alertID, offlineRecoveryConfirmationsDefault)

	// Reset the legacy offline counter; the reducer core owns the
	// confirmation run itself.

	m.resolveDiscreteRecoveryNoLock(node.ID, canonicalConnectivitySpecID(node.ID), alertID, offlineRecoveryConfirmationsDefault, "Node", node.Name, node.Instance)
}
