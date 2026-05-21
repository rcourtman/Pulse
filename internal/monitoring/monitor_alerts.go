package monitoring

import (
	"context"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rs/zerolog/log"
)

type canonicalResourceChangeRecorder interface {
	RecordChange(change unifiedresources.ResourceChange) error
}

// GetAlertManager returns the alert manager
func (m *Monitor) GetAlertManager() *alerts.Manager {
	return m.alertManager
}

// GetIncidentStore returns the incident timeline store.
func (m *Monitor) GetIncidentStore() *memory.IncidentStore {
	return m.incidentStore
}

// SetAlertTriggeredAICallback sets an additional callback for AI analysis when alerts fire
// This enables token-efficient, real-time AI insights on specific resources
// SetAlertTriggeredAICallback sets an additional callback for AI analysis when alerts fire
// This enables token-efficient, real-time AI insights on specific resources
func (m *Monitor) SetAlertTriggeredAICallback(callback func(*alerts.Alert)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertTriggeredAICallback = callback
	log.Info().Msg("alert-triggered AI callback registered")
}

// SetAlertResolvedAICallback sets an additional callback when alerts are resolved.
// This enables AI systems (like incident recording) to stop or finalize context after resolution.
func (m *Monitor) SetAlertResolvedAICallback(callback func(*alerts.Alert)) {
	if m.alertManager == nil {
		return
	}
	m.alertResolvedAICallback = callback
	log.Info().Msg("alert-resolved AI callback registered")
}

// SetConnectionsSnapshotLister registers the closure that produces platform
// connection snapshots once per monitor poll cycle. The api layer owns the
// closure because it owns the config + persistence inputs the aggregator
// needs. Passing nil disables the connection-degraded check on this monitor.
func (m *Monitor) SetConnectionsSnapshotLister(lister func() []alerts.ConnectionSnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connectionsSnapshotLister = lister
}

// checkConnectionAlerts runs CheckConnection against every platform
// connection snapshot the registered lister returns. Invoked from the main
// poll tick so a wedged PVE / PBS / PMG / VMware / TrueNAS connection escalates
// into the top-nav alert stream instead of staying behind on the Settings page.
func (m *Monitor) checkConnectionAlerts() {
	defer recoverFromPanic("checkConnectionAlerts")

	m.mu.RLock()
	lister := m.connectionsSnapshotLister
	m.mu.RUnlock()

	if lister == nil || m.alertManager == nil {
		return
	}
	for _, snap := range lister() {
		m.alertManager.CheckConnection(snap)
	}
}

func (m *Monitor) handleAlertFired(alert *alerts.Alert) {
	if alert == nil {
		return
	}

	if m.wsHub != nil {
		m.wsHub.BroadcastAlertToTenant(m.GetOrgID(), alert)
	}

	log.Debug().
		Str("alertID", alert.ID).
		Str("level", string(alert.Level)).
		Msg("Alert raised, sending to notification manager")
	if m.notificationMgr != nil {
		go m.notificationMgr.SendAlert(alert)
	}

	if m.incidentStore != nil {
		m.incidentStore.RecordAlertFired(alert)
	}
	m.recordAlertTimelineChange(alert, unifiedresources.ChangeAlertFired, alert.StartTime, "")

	// Trigger AI analysis if callback is configured
	if m.alertTriggeredAICallback != nil {
		// Run in goroutine to avoid blocking the monitor loop
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Error().Interface("panic", r).Msg("panic in AI alert callback")
				}
			}()
			m.alertTriggeredAICallback(alert)
		}()
	}
}

func (m *Monitor) handleAlertResolved(alertID string) {
	var resolvedAlert *alerts.ResolvedAlert

	if m.wsHub != nil {
		m.wsHub.BroadcastAlertResolvedToTenant(m.GetOrgID(), alertID)
	}

	// Always record incident timeline, regardless of notification suppression.
	// This ensures we have a complete history even during quiet hours.
	if m.incidentStore != nil {
		resolvedAlert = m.alertManager.GetResolvedAlert(alertID)
		if resolvedAlert != nil && resolvedAlert.Alert != nil {
			m.incidentStore.RecordAlertResolved(resolvedAlert.Alert, resolvedAlert.ResolvedTime)
		}
	}
	if resolvedAlert == nil && m.alertManager != nil {
		resolvedAlert = m.alertManager.GetResolvedAlert(alertID)
	}
	if resolvedAlert != nil && resolvedAlert.Alert != nil {
		m.recordAlertTimelineChange(resolvedAlert.Alert, unifiedresources.ChangeAlertResolved, resolvedAlert.ResolvedTime, "")
	}

	// Always trigger AI callback, regardless of notification suppression.
	if m.alertResolvedAICallback != nil {
		if resolvedAlert == nil {
			resolvedAlert = m.alertManager.GetResolvedAlert(alertID)
		}
		if resolvedAlert != nil && resolvedAlert.Alert != nil {
			go m.alertResolvedAICallback(resolvedAlert.Alert)
		}
	}

	// Handle notifications — recovery notifications respect quiet hours.
	// If the original alert would have been suppressed during quiet hours,
	// the recovery notification is also suppressed to avoid noise.
	if m.notificationMgr != nil {
		m.notificationMgr.CancelAlert(alertID)
		if m.notificationMgr.GetNotifyOnResolve() {
			if resolvedAlert == nil {
				resolvedAlert = m.alertManager.GetResolvedAlert(alertID)
			}
			if resolvedAlert != nil && resolvedAlert.Alert != nil {
				if m.alertManager.ShouldSuppressResolvedNotification(resolvedAlert.Alert) {
					log.Info().
						Str("alertID", alertID).
						Msg("Resolved notification suppressed during quiet hours")
				} else {
					go m.notificationMgr.SendResolvedAlert(resolvedAlert)
				}
			}
		} else {
			log.Info().
				Str("alertID", alertID).
				Msg("Resolved notification skipped - notifyOnResolve is disabled")
		}
	}
}

func (m *Monitor) handleAlertEscalated(hub *websocket.Hub, alert *alerts.Alert, level int) {
	if alert == nil || m.alertManager == nil {
		return
	}

	log.Info().
		Str("alertID", alert.ID).
		Int("level", level).
		Msg("Alert escalated")

	config := m.alertManager.GetConfig()
	if level <= 0 || level > len(config.Schedule.Escalation.Levels) {
		return
	}

	if m.alertManager.ShouldSuppressNotification(alert) {
		log.Info().
			Str("alertID", alert.ID).
			Int("level", level).
			Msg("Escalated notification suppressed during quiet hours")
		m.broadcastEscalatedAlert(hub, alert)
		return
	}

	if m.notificationMgr != nil {
		escalationLevel := config.Schedule.Escalation.Levels[level-1]
		switch escalationLevel.Notify {
		case "email":
			if emailConfig := m.notificationMgr.GetEmailConfig(); emailConfig.Enabled {
				m.notificationMgr.SendAlert(alert)
			}
		case "webhook":
			for _, webhook := range m.notificationMgr.GetWebhooks() {
				if webhook.Enabled {
					m.notificationMgr.SendAlert(alert)
					break
				}
			}
		case "all":
			m.notificationMgr.SendAlert(alert)
		}
	}

	m.broadcastEscalatedAlert(hub, alert)
}

func (m *Monitor) handleAlertAcknowledged(alert *alerts.Alert, user string) {
	if m.incidentStore == nil || alert == nil {
		if alert == nil {
			return
		}
	} else {
		m.incidentStore.RecordAlertAcknowledged(alert, user)
	}
	occurredAt := time.Now()
	if alert.AckTime != nil {
		occurredAt = *alert.AckTime
	}
	m.recordAlertTimelineChange(alert, unifiedresources.ChangeAlertAcknowledged, occurredAt, user)
}

func (m *Monitor) handleAlertUnacknowledged(alert *alerts.Alert, user string) {
	if alert == nil {
		return
	}
	if m.incidentStore != nil {
		m.incidentStore.RecordAlertUnacknowledged(alert, user)
	}
	m.recordAlertTimelineChange(alert, unifiedresources.ChangeAlertUnacknowledged, time.Now(), user)
}

func (m *Monitor) recordAlertTimelineChange(alert *alerts.Alert, kind unifiedresources.ChangeKind, occurredAt time.Time, actor string) {
	if alert == nil || m == nil {
		return
	}
	recorder, ok := m.resourceStore.(canonicalResourceChangeRecorder)
	if !ok || recorder == nil {
		return
	}

	change := unifiedresources.BuildAlertTimelineChange(alert.ResourceID, kind, occurredAt, actor, unifiedresources.AlertTimelineChange{
		AlertIdentifier: alert.ID,
		AlertType:       alert.Type,
		AlertLevel:      string(alert.Level),
		AlertMessage:    alert.Message,
		AlertValue:      alert.Value,
		AlertThreshold:  alert.Threshold,
		AlertMetadata:   alert.Metadata,
	})
	if change == nil {
		return
	}
	if err := recorder.RecordChange(*change); err != nil {
		log.Warn().
			Err(err).
			Str("resource_id", alert.ResourceID).
			Str("alert_id", alert.ID).
			Str("kind", string(kind)).
			Msg("failed to record canonical alert timeline change")
	}
}

// broadcastStateUpdate sends an immediate state update to all WebSocket clients.
// Call this after updating state with new data that should be visible immediately.
func (m *Monitor) broadcastStateUpdate() {
	m.mu.RLock()
	hub := m.wsHub
	m.mu.RUnlock()

	if hub == nil {
		return
	}

	frontendState := m.BuildBroadcastFrontendState()
	// Use tenant-aware broadcast method
	m.broadcastState(hub, frontendState)
}

// recordAuthFailure records an authentication failure for a node
func (m *Monitor) checkMockAlerts() {
	defer recoverFromPanic("checkMockAlerts")

	log.Debug().Bool("mockEnabled", mock.IsMockEnabled()).Msg("checkMockAlerts called")
	if !mock.IsMockEnabled() {
		log.Debug().Msg("mock mode not enabled, skipping mock alert check")
		return
	}

	// Get mock state
	state := mock.CurrentFixtureGraph().State

	log.Debug().
		Int("vms", len(state.VMs)).
		Int("containers", len(state.Containers)).
		Int("nodes", len(state.Nodes)).
		Msg("Checking alerts for mock data")

	// Clean up alerts for nodes that no longer exist
	existingNodes := make(map[string]bool)
	for _, node := range state.Nodes {
		existingNodes[node.Name] = true
		if node.Host != "" {
			existingNodes[node.Host] = true
		}
	}
	for _, pbsInst := range state.PBSInstances {
		existingNodes[pbsInst.Name] = true
		existingNodes["pbs-"+pbsInst.Name] = true
		if pbsInst.Host != "" {
			existingNodes[pbsInst.Host] = true
		}
	}
	log.Debug().
		Int("trackedNodes", len(existingNodes)).
		Msg("Collecting resources for alert cleanup in mock mode")
	m.alertManager.CleanupAlertsForNodes(existingNodes)

	guestsByKey, guestsByVMID := buildGuestLookupsFromReadState(m.GetUnifiedReadStateOrSnapshot(), m.guestMetadataStore)
	rollups, err := m.listBackupRollupsForAlerts(context.Background())
	if err != nil {
		log.Warn().Err(err).Msg("Failed to list recovery rollups for backup alerts")
	} else {
		m.alertManager.CheckBackupsWithInventory(rollups, guestsByKey, guestsByVMID, m.backupInventoryScopeForAlerts())
	}

	// Limit how many guests we check per cycle to prevent blocking with large datasets
	const maxGuestsPerCycle = 50
	guestsChecked := 0

	// Check alerts for VMs (up to limit)
	for _, vm := range state.VMs {
		if guestsChecked >= maxGuestsPerCycle {
			log.Debug().
				Int("checked", guestsChecked).
				Int("total", len(state.VMs)+len(state.Containers)).
				Msg("Reached guest check limit for this cycle")
			break
		}
		m.alertManager.CheckGuest(vm, "mock")
		guestsChecked++
	}

	// Check alerts for containers (if we haven't hit the limit)
	for _, container := range state.Containers {
		if guestsChecked >= maxGuestsPerCycle {
			break
		}
		m.alertManager.CheckGuest(container, "mock")
		guestsChecked++
	}

	// Check alerts for each node
	for _, node := range state.Nodes {
		m.alertManager.CheckNode(node)
	}

	// Check alerts for storage
	log.Debug().Int("storageCount", len(state.Storage)).Msg("checking storage alerts")
	for _, storage := range state.Storage {
		log.Debug().
			Str("name", storage.Name).
			Float64("usage", storage.Usage).
			Msg("Checking storage for alerts")
		m.alertManager.CheckStorage(storage)
	}

	// Check alerts for PBS instances
	log.Debug().Int("pbsCount", len(state.PBSInstances)).Msg("checking PBS alerts")
	for _, pbsInst := range state.PBSInstances {
		m.alertManager.CheckPBS(pbsInst)
	}

	// Check alerts for PMG instances
	log.Debug().Int("pmgCount", len(state.PMGInstances)).Msg("checking PMG alerts")
	for _, pmgInst := range state.PMGInstances {
		m.alertManager.CheckPMG(pmgInst)
	}

	// Cache the latest alert snapshots directly in the mock data so the API can serve
	// mock state without needing to grab the alert manager lock again.
	mock.UpdateAlertSnapshots(m.alertManager.GetActiveAlerts(), m.alertManager.GetRecentlyResolved())
}
