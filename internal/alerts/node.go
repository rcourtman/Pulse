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
			// Check for host agent deduplication: a usage metric that a linked host
			// agent evaluates belongs to the agent resource (CheckHost raises it), so
			// the node releases its own copy instead of alerting twice. A metric the
			// agent does not evaluate (agent alerts disabled, threshold off, usage
			// unknown) stays with the node, so the machine is never left unmonitored.
			link, _ := m.linkedHostAgentForNode(node.ID)
			input := &UnifiedResourceInput{
				ID:       node.ID,
				Type:     "node",
				Name:     node.Name,
				Node:     node.Name,
				Instance: node.Instance,
			}
			released := make([]string, 0, 3)
			if link.cpu {
				released = append(released, "cpu")
			} else {
				input.CPU = &UnifiedResourceMetric{Percent: node.CPU * 100}
			}
			if link.memory {
				released = append(released, "memory")
			} else if node.Memory.HasKnownUsage() {
				input.Memory = &UnifiedResourceMetric{Percent: node.Memory.Usage}
			}
			if link.disk {
				released = append(released, "disk")
			} else {
				input.Disk = &UnifiedResourceMetric{Percent: node.Disk.Usage}
			}
			m.releaseNodeMetricAlerts(node, released...)
			m.evaluateUnifiedMetrics(input, thresholds, nil, "temperature") // checkNodeTemperature owns its observation gaps.

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

// hostAgentNodeLink is a reporting host agent's link to a Proxmox node and the
// usage metrics the agent evaluates for that machine. The link is the monitor's
// identity decision (automatic or operator-set), so an agent whose hostname
// merely matches a node name in another instance, or one an operator unlinked,
// never makes a node release its alerts, and an agent reporting an FQDN still
// dedups its node.
type hostAgentNodeLink struct {
	agentID   string
	agentName string
	nodeID    string
	cpu       bool
	memory    bool
	disk      bool
}

// registerHostAgentNodeLink records a host agent's node link after CheckHost
// has decided which usage metrics it evaluates. A link without a node or
// without any evaluated metric is removed, so the node keeps all its alerts.
func (m *Manager) registerHostAgentNodeLink(link hostAgentNodeLink) {
	link.agentID = strings.TrimSpace(link.agentID)
	link.nodeID = strings.TrimSpace(link.nodeID)
	if link.agentID == "" {
		return
	}
	if link.nodeID == "" || !(link.cpu || link.memory || link.disk) {
		m.unregisterHostAgentNodeLink(link.agentID)
		return
	}
	m.mu.Lock()
	previous, existed := m.hostAgentNodeLinks[link.agentID]
	m.hostAgentNodeLinks[link.agentID] = link
	m.mu.Unlock()

	if !existed || previous != link {
		log.Debug().
			Str("hostID", link.agentID).
			Str("linkedNodeID", link.nodeID).
			Bool("cpu", link.cpu).
			Bool("memory", link.memory).
			Bool("disk", link.disk).
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
	link, existed := m.hostAgentNodeLinks[hostID]
	delete(m.hostAgentNodeLinks, hostID)
	m.mu.Unlock()

	if existed {
		log.Debug().
			Str("hostID", hostID).
			Str("linkedNodeID", link.nodeID).
			Msg("Removed host agent node link from deduplication")
	}
}

// linkedHostAgentForNode returns the host agent coverage for the given Proxmox
// node. A usage metric counts as owned when any agent linked to the node
// evaluates it, so a second linked agent covering a metric the first does not
// still keeps the node from alerting twice. The agent identity reported is the
// lowest agent ID among the linked agents, so the choice is stable.
func (m *Manager) linkedHostAgentForNode(nodeID string) (hostAgentNodeLink, bool) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return hostAgentNodeLink{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var found hostAgentNodeLink
	ok := false
	for _, link := range m.hostAgentNodeLinks {
		if link.nodeID != nodeID {
			continue
		}
		if !ok || link.agentID < found.agentID {
			found.agentID = link.agentID
			found.agentName = link.agentName
		}
		found.nodeID = nodeID
		found.cpu = found.cpu || link.cpu
		found.memory = found.memory || link.memory
		found.disk = found.disk || link.disk
		ok = true
	}
	return found, ok
}

// hasHostAgentForNode reports whether a reporting host agent is linked to the
// given Proxmox node and owns at least one of its usage metrics.
func (m *Manager) hasHostAgentForNode(nodeID string) bool {
	_, ok := m.linkedHostAgentForNode(nodeID)
	return ok
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
