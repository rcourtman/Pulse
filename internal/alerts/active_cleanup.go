package alerts

import (
	"strings"
	"time"

	alertspecs "github.com/rcourtman/pulse-go-rewrite/internal/alerts/specs"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// Cleanup removes old acknowledged alerts and cleans up tracking maps.
func (m *Manager) Cleanup(maxAge time.Duration) {
	m.mu.Lock()
	now := time.Now()
	var autoAcked []*Alert

	lastSeenTooOld := func(alert *Alert, cutoff time.Duration) bool {
		if alert == nil {
			return true
		}
		lastSeen := alert.LastSeen
		if lastSeen.IsZero() {
			lastSeen = alert.StartTime
		}
		return now.Sub(lastSeen) > cutoff
	}

	if m.config.AutoAcknowledgeAfterHours > 0 {
		autoAckThreshold := time.Duration(m.config.AutoAcknowledgeAfterHours) * time.Hour
		for id, alert := range m.activeAlerts {
			if !alert.Acknowledged && now.Sub(alert.StartTime) > autoAckThreshold {
				log.Info().
					Str("alertID", id).
					Dur("age", now.Sub(alert.StartTime)).
					Msg("Auto-acknowledging old alert")
				alert.Acknowledged = true
				ackTime := now
				alert.AckTime = &ackTime
				alert.AckUser = "system-auto"
				autoAcked = append(autoAcked, alert.Clone())

				if recordAlertAcknowledged != nil {
					recordAlertAcknowledged()
				}
			}
		}
	}

	if m.config.MaxAcknowledgedAgeDays > 0 {
		acknowledgedTTL := time.Duration(m.config.MaxAcknowledgedAgeDays) * 24 * time.Hour
		for id, alert := range m.activeAlerts {
			if alert.Acknowledged && alert.AckTime != nil &&
				now.Sub(*alert.AckTime) > acknowledgedTTL &&
				lastSeenTooOld(alert, acknowledgedTTL) {
				log.Info().
					Str("alertID", id).
					Dur("age", now.Sub(*alert.AckTime)).
					Msg("Cleaning up old acknowledged alert (TTL)")
				m.removeActiveAlertNoLock(id)
			}
		}
	}

	if m.config.MaxAlertAgeDays > 0 {
		alertTTL := time.Duration(m.config.MaxAlertAgeDays) * 24 * time.Hour
		for id, alert := range m.activeAlerts {
			if !alert.Acknowledged && now.Sub(alert.StartTime) > alertTTL {
				log.Info().
					Str("alertID", id).
					Dur("age", now.Sub(alert.StartTime)).
					Msg("Cleaning up old unacknowledged alert (TTL)")
				m.removeActiveAlertNoLock(id)
			}
		}
	}

	for id, alert := range m.activeAlerts {
		if alert.Acknowledged && alert.AckTime != nil &&
			now.Sub(*alert.AckTime) > maxAge &&
			lastSeenTooOld(alert, maxAge) {
			m.removeActiveAlertNoLock(id)
		}
	}

	ackStateTTL := 1 * time.Hour
	for id, record := range m.ackState {
		if !m.hasActiveAlertNoLock(id) {
			checkTime := record.inactiveAt
			if checkTime.IsZero() {
				checkTime = record.time
			}
			if now.Sub(checkTime) > ackStateTTL {
				delete(m.ackState, id)
			}
		}
	}
	for canonicalID, record := range m.ackStateByCanonical {
		if m.hasActiveAlertTrackingKeyNoLock(canonicalID) {
			continue
		}
		checkTime := record.inactiveAt
		if checkTime.IsZero() {
			checkTime = record.time
		}
		if now.Sub(checkTime) > ackStateTTL {
			delete(m.ackStateByCanonical, canonicalID)
		}
	}

	suppressionWindow := time.Duration(m.config.SuppressionWindow) * time.Minute
	if suppressionWindow == 0 {
		suppressionWindow = 5 * time.Minute
	}
	for id, alert := range m.recentAlerts {
		if now.Sub(alert.StartTime) > suppressionWindow {
			delete(m.recentAlerts, id)
		}
	}

	for id, suppressUntil := range m.suppressedUntil {
		if now.After(suppressUntil) {
			delete(m.suppressedUntil, id)
		}
	}

	cutoff := now.Add(-1 * time.Hour)
	for alertID, times := range m.alertRateLimit {
		var recentTimes []time.Time
		for _, t := range times {
			if t.After(cutoff) {
				recentTimes = append(recentTimes, t)
			}
		}
		if len(recentTimes) == 0 {
			delete(m.alertRateLimit, alertID)
		} else {
			m.alertRateLimit[alertID] = recentTimes
		}
	}

	m.resolvedMutex.Lock()
	m.pruneRecentlyResolvedUnlocked(now)
	m.resolvedMutex.Unlock()

	maxPendingAge := 10 * time.Minute
	for id, pendingTime := range m.pendingAlerts {
		if now.Sub(pendingTime) > maxPendingAge {
			delete(m.pendingAlerts, id)
			log.Debug().
				Str("resourceID", id).
				Dur("age", now.Sub(pendingTime)).
				Msg("Cleaned up stale pending alert entry")
		}
	}

	flappingCleanupAge := 1 * time.Hour
	for alertID := range m.flappingHistory {
		if !m.hasActiveAlertTrackingKeyNoLock(alertID) {
			if suppressUntil, suppressed := m.suppressedUntil[alertID]; !suppressed || now.After(suppressUntil.Add(flappingCleanupAge)) {
				delete(m.flappingHistory, alertID)
				delete(m.flappingActive, alertID)
				log.Debug().
					Str("alertID", alertID).
					Msg("Cleaned up flapping history for inactive alert")
			}
		}
	}

	for resourceID, record := range m.dockerRestartTracking {
		if now.Sub(record.lastChecked) > 24*time.Hour {
			delete(m.dockerRestartTracking, resourceID)
			log.Debug().
				Str("resourceID", resourceID).
				Msg("Cleaned up stale Docker restart tracking entry")
		}
	}

	staleTrackerAge := 24 * time.Hour
	for pmgID, tracker := range m.pmgAnomalyTrackers {
		if tracker != nil && !tracker.LastSampleTime.IsZero() {
			if now.Sub(tracker.LastSampleTime) > staleTrackerAge {
				delete(m.pmgAnomalyTrackers, pmgID)
				log.Debug().
					Str("pmgID", pmgID).
					Time("lastSampleTime", tracker.LastSampleTime).
					Msg("Cleaned up stale PMG anomaly tracker")
			}
		}
	}

	staleHistoryAge := 7 * 24 * time.Hour
	for pmgID, snapshots := range m.pmgQuarantineHistory {
		if len(snapshots) == 0 {
			delete(m.pmgQuarantineHistory, pmgID)
			log.Debug().
				Str("pmgID", pmgID).
				Msg("Cleaned up empty PMG quarantine history")
			continue
		}

		lastSnapshot := snapshots[len(snapshots)-1]
		if now.Sub(lastSnapshot.Timestamp) > staleHistoryAge {
			delete(m.pmgQuarantineHistory, pmgID)
			log.Debug().
				Str("pmgID", pmgID).
				Time("lastSnapshot", lastSnapshot.Timestamp).
				Msg("Cleaned up stale PMG quarantine history")
		}
	}

	m.mu.Unlock()

	for _, alert := range autoAcked {
		m.safeCallAcknowledgedCallback(alert, "system-auto")
	}
}

func alertMetadataResourceType(alert *Alert) string {
	if alert == nil || alert.Metadata == nil {
		return ""
	}
	if value, ok := alert.Metadata["resourceType"].(string); ok {
		return strings.ToLower(strings.TrimSpace(value))
	}
	return ""
}

func shouldPreserveAlertOutsideNodeCleanup(alertID string, alert *Alert) bool {
	if alert == nil {
		return true
	}
	if strings.HasPrefix(alertID, "docker-") || strings.HasPrefix(alert.ResourceID, "docker:") {
		return true
	}
	if strings.HasPrefix(alertID, "pbs-") || alert.Type == "pbs-offline" {
		return true
	}
	if strings.HasPrefix(alert.ResourceID, "agent:") {
		return true
	}
	if alert.CanonicalKind == string(alertspecs.AlertSpecKindProviderIncident) {
		return true
	}

	switch alertMetadataResourceType(alert) {
	case string(unifiedresources.ResourceTypeAgent),
		string(unifiedresources.ResourceTypeAppContainer),
		string(unifiedresources.ResourceTypeDockerService),
		string(unifiedresources.ResourceTypeK8sCluster),
		string(unifiedresources.ResourceTypeK8sNode),
		string(unifiedresources.ResourceTypePod),
		string(unifiedresources.ResourceTypeK8sDeployment),
		string(unifiedresources.ResourceTypeStorage),
		string(unifiedresources.ResourceTypePBS),
		string(unifiedresources.ResourceTypePMG),
		string(unifiedresources.ResourceTypeCeph),
		string(unifiedresources.ResourceTypePhysicalDisk),
		string(unifiedresources.ResourceTypeNetworkShare),
		string(unifiedresources.ResourceTypeNetworkEndpoint),
		"docker",
		"host",
		"kubernetes",
		"k8s",
		"truenas",
		"vmware":
		return true
	default:
		return false
	}
}

// CleanupAlertsForNodes removes stale Proxmox node-scoped alerts.
func (m *Manager) CleanupAlertsForNodes(existingNodes map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Debug().
		Int("totalAlerts", len(m.activeAlerts)).
		Int("existingNodes", len(existingNodes)).
		Interface("nodes", existingNodes).
		Msg("Starting alert cleanup for non-existent nodes")

	removedCount := 0
	for storageKey, alert := range m.activeAlerts {
		alertID := effectiveAlertID(alert, storageKey)
		if alert == nil {
			continue
		}

		if alert.CanonicalKind == string(alertspecs.AlertSpecKindConnectivity) && strings.HasPrefix(alert.ResourceID, "pbs") {
			continue
		}
		if shouldPreserveAlertOutsideNodeCleanup(alertID, alert) {
			continue
		}

		node := alert.Node
		if node == "" || !existingNodes[node] {
			m.removeActiveAlertNoLock(alertID)
			removedCount++
			log.Debug().Str("alertID", alertID).Str("node", node).Msg("removed alert for non-existent node")
		}
	}

	if removedCount > 0 {
		log.Debug().Int("removed", removedCount).Int("remaining", len(m.activeAlerts)).Msg("cleaned up alerts for non-existent nodes")
		m.saveActiveAlertsAsync("node cleanup")
	} else {
		log.Debug().Msg("no alerts needed cleanup")
	}
}

// ClearActiveAlerts removes all active and pending alerts, resetting the manager state.
func (m *Manager) ClearActiveAlerts() {
	m.mu.Lock()
	if len(m.activeAlerts) == 0 && len(m.pendingAlerts) == 0 {
		m.mu.Unlock()
		return
	}
	m.activeAlerts = make(map[string]*Alert)
	m.activeAlertAlias = make(map[string]string)
	m.pendingAlerts = make(map[string]time.Time)
	m.recentAlerts = make(map[string]*Alert)
	m.suppressedUntil = make(map[string]time.Time)
	m.alertRateLimit = make(map[string][]time.Time)
	m.nodeOfflineCount = make(map[string]int)
	m.offlineConfirmations = make(map[string]int)
	m.offlineRecoveryConfirmations = make(map[string]int)
	m.dockerOfflineCount = make(map[string]int)
	m.dockerStateConfirm = make(map[string]int)
	m.dockerRestartTracking = make(map[string]*dockerRestartRecord)
	m.dockerUpdateFirstSeen = make(map[string]time.Time)
	m.dockerUpdateFirstSeenByIdentity = make(map[string]time.Time)
	m.ackState = make(map[string]ackRecord)
	m.ackStateByCanonical = make(map[string]ackRecord)
	m.mu.Unlock()

	m.resolvedMutex.Lock()
	m.recentlyResolved = make(map[string]*ResolvedAlert)
	m.resolvedAlias = make(map[string]string)
	m.resolvedMutex.Unlock()

	log.Info().Msg("cleared all active and pending alerts")

	m.saveActiveAlertsAsync("clear active alerts")
}
