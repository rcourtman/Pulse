package alerts

import (
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// trackingMapCleanup periodically cleans up stale entries from tracking maps
// to prevent unbounded memory growth from deleted/decommissioned resources.
func (m *Manager) trackingMapCleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupStaleMaps()
		case <-m.cleanupStop:
			return
		}
	}
}

// cleanupStaleMaps removes stale entries from tracking maps.
// Entries are considered stale if they haven't been updated in 24 hours
// and don't correspond to any active alert.
func (m *Manager) cleanupStaleMaps() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	staleThreshold := StaleTrackingThreshold
	cleaned := 0

	for alertID, history := range m.flappingHistory {
		if !m.hasActiveAlertNoLock(alertID) {
			if len(history) == 0 || now.Sub(history[len(history)-1]) > staleThreshold {
				delete(m.flappingHistory, alertID)
				delete(m.flappingActive, alertID)
				cleaned++
			}
		}
	}

	for alertID, suppressUntil := range m.suppressedUntil {
		if now.After(suppressUntil) {
			delete(m.suppressedUntil, alertID)
			cleaned++
		}
	}

	for alertID, pendingTime := range m.pendingAlerts {
		if !m.hasActiveAlertNoLock(alertID) {
			if now.Sub(pendingTime) > staleThreshold {
				delete(m.pendingAlerts, alertID)
				cleaned++
			}
		}
	}

	for resourceID := range m.offlineConfirmations {
		hasRelatedAlert := false
		for storageKey, alert := range m.activeAlerts {
			alertID := effectiveAlertID(alert, storageKey)
			if strings.Contains(alertID, resourceID) {
				hasRelatedAlert = true
				break
			}
		}
		if !hasRelatedAlert {
			delete(m.offlineConfirmations, resourceID)
			cleaned++
		}
	}

	for alertID := range m.offlineRecoveryConfirmations {
		if !m.hasActiveAlertNoLock(alertID) {
			delete(m.offlineRecoveryConfirmations, alertID)
			cleaned++
		}
	}

	for nodeID := range m.nodeOfflineCount {
		hasRelatedAlert := false
		for storageKey, alert := range m.activeAlerts {
			alertID := effectiveAlertID(alert, storageKey)
			if strings.Contains(alertID, nodeID) {
				hasRelatedAlert = true
				break
			}
		}
		if !hasRelatedAlert {
			delete(m.nodeOfflineCount, nodeID)
			cleaned++
		}
	}

	for containerID := range m.dockerStateConfirm {
		hasRelatedAlert := false
		for storageKey, alert := range m.activeAlerts {
			alertID := effectiveAlertID(alert, storageKey)
			if strings.Contains(alertID, containerID) {
				hasRelatedAlert = true
				break
			}
		}
		if !hasRelatedAlert {
			delete(m.dockerStateConfirm, containerID)
			cleaned++
		}
	}

	for hostID := range m.dockerOfflineCount {
		hasRelatedAlert := false
		for storageKey, alert := range m.activeAlerts {
			alertID := effectiveAlertID(alert, storageKey)
			if strings.Contains(alertID, hostID) {
				hasRelatedAlert = true
				break
			}
		}
		if !hasRelatedAlert {
			delete(m.dockerOfflineCount, hostID)
			cleaned++
		}
	}

	for containerID, record := range m.dockerRestartTracking {
		if record != nil && now.Sub(record.lastChecked) > staleThreshold {
			delete(m.dockerRestartTracking, containerID)
			delete(m.dockerLastExitCode, containerID)
			cleaned++
		}
	}

	for containerID, firstSeen := range m.dockerUpdateFirstSeen {
		if now.Sub(firstSeen) > staleThreshold {
			delete(m.dockerUpdateFirstSeen, containerID)
			cleaned++
		}
	}
	for containerID, firstSeen := range m.dockerUpdateFirstSeenByIdentity {
		if now.Sub(firstSeen) > staleThreshold {
			delete(m.dockerUpdateFirstSeenByIdentity, containerID)
			cleaned++
		}
	}

	rateLimitThreshold := RateLimitCleanupWindow
	for resourceID, times := range m.alertRateLimit {
		var recent []time.Time
		for _, t := range times {
			if now.Sub(t) < rateLimitThreshold {
				recent = append(recent, t)
			}
		}
		if len(recent) == 0 {
			delete(m.alertRateLimit, resourceID)
			cleaned++
		} else if len(recent) < len(times) {
			m.alertRateLimit[resourceID] = recent
		}
	}

	suppressWindow := time.Duration(m.config.SuppressionWindow) * time.Minute
	if suppressWindow <= 0 {
		suppressWindow = 5 * time.Minute
	}
	for alertID, alert := range m.recentAlerts {
		if now.Sub(alert.LastSeen) > suppressWindow {
			delete(m.recentAlerts, alertID)
			cleaned++
		}
	}

	for alertID, record := range m.ackState {
		if !m.hasActiveAlertNoLock(alertID) {
			checkTime := record.inactiveAt
			if checkTime.IsZero() {
				checkTime = record.time
			}
			if now.Sub(checkTime) > staleThreshold {
				delete(m.ackState, alertID)
				cleaned++
			}
		}
	}
	for canonicalID, record := range m.ackStateByCanonical {
		checkTime := record.inactiveAt
		if checkTime.IsZero() {
			checkTime = record.time
		}
		if now.Sub(checkTime) > staleThreshold {
			delete(m.ackStateByCanonical, canonicalID)
			cleaned++
		}
	}

	staleAlerts := make([]string, 0)
	for storageKey, alert := range m.activeAlerts {
		alertID := effectiveAlertID(alert, storageKey)
		if alert != nil && now.Sub(alert.LastSeen) > staleThreshold {
			staleAlerts = append(staleAlerts, alertID)
		}
	}
	staleResolved := 0
	for _, alertID := range staleAlerts {
		alert, exists := m.getActiveAlertNoLock(alertID)
		if !exists || alert == nil {
			continue
		}
		log.Info().
			Str("alertID", alertID).
			Str("resourceName", alert.ResourceName).
			Time("lastSeen", alert.LastSeen).
			Dur("staleFor", now.Sub(alert.LastSeen)).
			Msg("Auto-resolving stale alert - resource no longer being monitored")
		m.clearAlertNoLock(alertID)
		cleaned++
		staleResolved++
	}

	if staleResolved > 0 {
		m.saveActiveAlertsAsync("stale cleanup")
		log.Info().
			Int("count", staleResolved).
			Msg("Auto-resolved stale alerts")
	}

	if cleaned > 0 {
		log.Debug().
			Int("entriesCleaned", cleaned).
			Msg("Cleaned stale entries from alert tracking maps")
	}
}
