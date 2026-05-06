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

func isPBSOffline(pbs models.PBSInstance) bool {
	status := strings.ToLower(strings.TrimSpace(pbs.Status))
	health := strings.ToLower(strings.TrimSpace(pbs.ConnectionHealth))
	return status == "offline" || health == "error" || health == "failed" || health == "unhealthy"
}

func (m *Manager) clearPBSMetricAlerts(pbsID string) {
	if strings.TrimSpace(pbsID) == "" {
		return
	}

	m.clearAlert(canonicalMetricStateID(pbsID, "cpu"))
	m.clearAlert(canonicalMetricStateID(pbsID, "memory"))
}

// CheckPBS checks PBS instance metrics against thresholds
func (m *Manager) CheckPBS(pbs models.PBSInstance) {
	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		return
	}
	if m.config.DisableAllPBS {
		m.mu.RUnlock()
		// Clear any existing PBS alerts when all PBS alerts are disabled
		m.mu.Lock()
		// Reset offline confirmation tracking
		delete(m.offlineConfirmations, pbs.ID)
		// Clear CPU alert
		cpuAlertID := canonicalMetricStateID(pbs.ID, "cpu")
		if m.clearActiveAlertIfPresentNoLock(cpuAlertID) {
			log.Info().
				Str("alertID", cpuAlertID).
				Str("pbs", pbs.Name).
				Msg("Cleared CPU alert - all PBS alerts disabled")
		}
		// Clear Memory alert
		memAlertID := canonicalMetricStateID(pbs.ID, "memory")
		if m.clearActiveAlertIfPresentNoLock(memAlertID) {
			log.Info().
				Str("alertID", memAlertID).
				Str("pbs", pbs.Name).
				Msg("Cleared Memory alert - all PBS alerts disabled")
		}
		// Clear offline alert
		offlineAlertID := canonicalConnectivityStateID(pbs.ID)
		if m.clearActiveAlertIfPresentNoLock(offlineAlertID) {
			log.Info().
				Str("alertID", offlineAlertID).
				Str("pbs", pbs.Name).
				Msg("Cleared offline alert - all PBS alerts disabled")
		}
		m.mu.Unlock()
		return
	}

	thresholds := m.resolveResourceThresholds("pbs", pbs.ID)
	disablePBSOffline := m.config.DisableAllPBSOffline
	m.mu.RUnlock()

	// Check override disable BEFORE offline detection to prevent spurious notifications
	if thresholds.Disabled {
		m.mu.Lock()
		// Reset offline confirmation tracking
		delete(m.offlineConfirmations, pbs.ID)
		// Clear CPU alert
		cpuAlertID := canonicalMetricStateID(pbs.ID, "cpu")
		if m.clearActiveAlertIfPresentNoLock(cpuAlertID) {
			log.Debug().
				Str("alertID", cpuAlertID).
				Str("pbs", pbs.Name).
				Msg("Cleared CPU alert - PBS has alerts disabled")
		}
		// Clear Memory alert
		memAlertID := canonicalMetricStateID(pbs.ID, "memory")
		if m.clearActiveAlertIfPresentNoLock(memAlertID) {
			log.Debug().
				Str("alertID", memAlertID).
				Str("pbs", pbs.Name).
				Msg("Cleared Memory alert - PBS has alerts disabled")
		}
		// Clear offline alert
		offlineAlertID := canonicalConnectivityStateID(pbs.ID)
		if m.clearActiveAlertIfPresentNoLock(offlineAlertID) {
			log.Debug().
				Str("alertID", offlineAlertID).
				Str("pbs", pbs.Name).
				Msg("Cleared offline alert - PBS has alerts disabled")
		}
		m.mu.Unlock()
		return
	}

	pbsOffline := isPBSOffline(pbs)

	if disablePBSOffline || thresholds.DisableConnectivity {
		// Clear tracking and any existing offline alerts when globally disabled
		m.mu.Lock()
		delete(m.offlineConfirmations, pbs.ID)
		m.mu.Unlock()
		m.clearAlert(canonicalConnectivityStateID(pbs.ID))
	} else {
		// Check if PBS is offline first (similar to nodes)
		if pbsOffline {
			m.checkPBSOffline(pbs)
		} else {
			// Clear any existing offline alert if PBS is back online
			m.clearPBSOfflineAlert(pbs)
		}
	}

	// When PBS is offline/unhealthy, clear stale metric alerts immediately.
	if pbsOffline {
		m.clearPBSMetricAlerts(pbs.ID)
		return
	}

	m.evaluateUnifiedMetrics(&UnifiedResourceInput{
		ID:       pbs.ID,
		Type:     "pbs",
		Name:     pbs.Name,
		Node:     pbs.Host,
		Instance: pbs.Name,
		CPU:      &UnifiedResourceMetric{Percent: pbs.CPU},
		Memory:   &UnifiedResourceMetric{Percent: pbs.Memory},
	}, thresholds, nil)
}

// checkPBSOffline creates an alert for offline PBS instances
func (m *Manager) checkPBSOffline(pbs models.PBSInstance) {
	m.mu.Lock()
	delete(m.offlineRecoveryConfirmations, canonicalConnectivityStateID(pbs.ID))
	m.mu.Unlock()

	thresholds := m.resolveResourceThresholds("pbs", pbs.ID)
	spec, err := buildCanonicalConnectivitySpec(pbs.ID, pbs.Name, unifiedresources.ResourceTypePBS, AlertLevelCritical, 3, thresholds.Disabled || thresholds.DisableConnectivity)
	if err != nil {
		log.Warn().
			Err(err).
			Str("pbs", pbs.Name).
			Str("pbsID", pbs.ID).
			Msg("Skipping invalid canonical PBS connectivity spec")
		return
	}

	_, _ = m.evaluateCanonicalLifecycleAlert(canonicalLifecycleAlertParams{
		Spec:         spec,
		Evidence:     alertspecs.AlertEvidence{ObservedAt: time.Now(), Connectivity: &alertspecs.ConnectivityEvidence{Signal: "status", Connected: false}},
		Tracking:     m.offlineConfirmations,
		TrackingKey:  pbs.ID,
		AlertID:      fmt.Sprintf("pbs-offline-%s", pbs.ID),
		AlertType:    "offline",
		ResourceID:   pbs.ID,
		ResourceName: pbs.Name,
		Node:         pbs.Host,
		Instance:     pbs.Name,
		Message:      fmt.Sprintf("PBS instance %s is offline", pbs.Name),
		Metadata: map[string]interface{}{
			"resourceType":     "pbs",
			"status":           pbs.Status,
			"connectionHealth": pbs.ConnectionHealth,
		},
		RateLimit:     true,
		DispatchAsync: true,
	})
}

// clearPBSOfflineAlert removes offline alert when PBS comes back online
func (m *Manager) clearPBSOfflineAlert(pbs models.PBSInstance) {
	m.clearResourceOfflineAlert(pbs.ID, pbs.Name, pbs.Host, "PBS", offlineRecoveryConfirmationsDefault)
}
