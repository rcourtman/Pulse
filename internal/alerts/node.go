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

	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		return
	}
	if m.config.DisableAllNodes {
		m.mu.RUnlock()
		// Clear any existing node alerts when all node alerts are disabled
		m.mu.Lock()
		// Clear offline tracking
		delete(m.nodeOfflineCount, node.ID)
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
	disableNodesOffline := m.config.DisableAllNodesOffline
	thresholds := m.resolveResourceThresholds("node", node.ID)
	m.mu.RUnlock()

	if thresholds.Disabled {
		m.mu.Lock()
		delete(m.nodeOfflineCount, node.ID)
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
		delete(m.nodeOfflineCount, node.ID)
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
			// Check for host agent deduplication: if a host agent is running on this node,
			// prefer the host agent alerts and skip node metric alerts to avoid duplicates.
			if m.hasHostAgentForNode(node.Name) {
				log.Debug().
					Str("node", node.Name).
					Msg("Skipping node metric alerts - host agent is monitoring this machine")
			} else {
				m.evaluateUnifiedMetrics(&UnifiedResourceInput{
					ID:       node.ID,
					Type:     "node",
					Name:     node.Name,
					Node:     node.Name,
					Instance: node.Instance,
					CPU:      &UnifiedResourceMetric{Percent: node.CPU * 100},
					Memory:   &UnifiedResourceMetric{Percent: node.Memory.Usage},
					Disk:     &UnifiedResourceMetric{Percent: node.Disk.Usage},
				}, thresholds, nil)

				// Check temperature if available
				// We pass the check unconditionally so that if the threshold triggers are disabled (set to 0),
				// any existing alerts will be properly cleared.
				var temp float64
				if node.Temperature != nil && node.Temperature.Available {
					// Use CPU package temp if available, otherwise use max core temp
					temp = node.Temperature.CPUPackage
					if temp == 0 {
						temp = node.Temperature.CPUMax
					}
				}
				spec, err := buildCanonicalMetricSpec(node.ID, node.Name, unifiedresources.ResourceType("node"), "temperature", thresholds.Temperature)
				if err != nil {
					log.Warn().
						Err(err).
						Str("resourceID", node.ID).
						Str("node", node.Name).
						Msg("Skipping invalid canonical node temperature metric spec")
				} else {
					m.checkMetricWithCanonicalSpec(spec, node.Name, node.Name, node.Instance, "node", temp, thresholds.Temperature, nil)
				}
			}
		}
	}
}

// RegisterHostAgentHostname registers a host agent hostname for deduplication.
// When a host agent is actively monitoring a machine, we prefer its alerts
// over Proxmox node alerts to avoid duplicate monitoring of the same machine.
func (m *Manager) RegisterHostAgentHostname(hostname string) {
	normalized := strings.ToLower(strings.TrimSpace(hostname))
	if normalized == "" {
		return
	}
	m.mu.Lock()
	m.hostAgentHostnames[normalized] = struct{}{}
	m.mu.Unlock()

	log.Debug().
		Str("hostname", hostname).
		Msg("Registered host agent hostname for deduplication")
}

// UnregisterHostAgentHostname removes a host agent hostname from deduplication tracking.
func (m *Manager) UnregisterHostAgentHostname(hostname string) {
	normalized := strings.ToLower(strings.TrimSpace(hostname))
	if normalized == "" {
		return
	}
	m.mu.Lock()
	delete(m.hostAgentHostnames, normalized)
	m.mu.Unlock()

	log.Debug().
		Str("hostname", hostname).
		Msg("Unregistered host agent hostname from deduplication")
}

// hasHostAgentForNode checks if a host agent is monitoring a machine with the same
// hostname as the given Proxmox node. If so, we should suppress node alerts to
// avoid duplicate alerting.
func (m *Manager) hasHostAgentForNode(nodeName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(nodeName))
	if normalized == "" {
		return false
	}
	m.mu.RLock()
	_, exists := m.hostAgentHostnames[normalized]
	m.mu.RUnlock()
	return exists
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

	m.mu.Lock()
	delete(m.offlineRecoveryConfirmations, canonicalConnectivityStateID(node.ID))
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
		Tracking:     m.nodeOfflineCount,
		TrackingKey:  node.ID,
		AlertID:      alertID,
		AlertType:    "connectivity",
		ResourceID:   node.ID,
		ResourceName: node.Name,
		Node:         node.Name,
		Instance:     node.Instance,
		Message:      fmt.Sprintf("Node '%s' is offline", node.Name),
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

	// Reset offline count when node comes back online
	if m.nodeOfflineCount[node.ID] > 0 {
		log.Debug().
			Str("node", node.Name).
			Int("previousCount", m.nodeOfflineCount[node.ID]).
			Msg("Node back online, resetting offline count")
		delete(m.nodeOfflineCount, node.ID)
	}

	// Check if offline alert exists
	alert, exists := m.getActiveAlertNoLock(alertID)
	if !exists {
		delete(m.offlineRecoveryConfirmations, alertID)
		return
	}

	recoveryCount, confirmed := m.confirmOfflineRecoveryNoLock(alertID, offlineRecoveryConfirmationsDefault)
	if !confirmed {
		log.Debug().
			Str("node", node.Name).
			Int("confirmations", recoveryCount).
			Int("required", offlineRecoveryConfirmationsDefault).
			Msg("Node appears back online, waiting for recovery confirmation")
		return
	}

	// Remove from active alerts
	m.removeActiveAlertNoLock(alertID)

	resolvedAlert := &ResolvedAlert{
		Alert:        alert,
		ResolvedTime: time.Now(),
	}
	m.addRecentlyResolvedWithPrimaryLock(resolvedAlert)

	// Send recovery notification (async to avoid deadlock — callback acquires m.mu.RLock
	// via ShouldSuppressResolvedNotification, and we currently hold m.mu.Lock)
	m.safeCallResolvedAlertCallback(alert, alertID, true)

	// Log recovery
	log.Info().
		Str("node", node.Name).
		Str("instance", node.Instance).
		Dur("downtime", time.Since(alert.StartTime)).
		Msg("Node is back online")
}
