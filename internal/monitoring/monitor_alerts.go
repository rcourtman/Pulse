package monitoring

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rs/zerolog/log"
)

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

	// Always trigger AI callback, regardless of notification suppression.
	if m.alertResolvedAICallback != nil {
		if resolvedAlert == nil {
			resolvedAlert = m.alertManager.GetResolvedAlert(alertID)
		}
		if resolvedAlert != nil && resolvedAlert.Alert != nil {
			go m.alertResolvedAICallback(resolvedAlert.Alert)
		}
	}

	// Handle notifications (may be suppressed by quiet hours)
	if m.notificationMgr != nil {
		m.notificationMgr.CancelAlert(alertID)
		if m.notificationMgr.GetNotifyOnResolve() {
			if resolvedAlert == nil {
				resolvedAlert = m.alertManager.GetResolvedAlert(alertID)
			}
			if resolvedAlert != nil {
				// Check if recovery notification should be suppressed during quiet hours
				if m.alertManager.ShouldSuppressResolvedNotification(resolvedAlert.Alert) {
					log.Info().
						Str("alertID", alertID).
						Str("type", resolvedAlert.Alert.Type).
						Msg("Resolved notification skipped - suppressed by quiet hours")
					return
				}
				go m.notificationMgr.SendResolvedAlert(resolvedAlert)
			}
		} else {
			log.Info().
				Str("alertID", alertID).
				Msg("Resolved notification skipped - notifyOnResolve is disabled")
		}
	}
}

func (m *Monitor) handleAlertAcknowledged(alert *alerts.Alert, user string) {
	if m.incidentStore == nil || alert == nil {
		return
	}
	m.incidentStore.RecordAlertAcknowledged(alert, user)
}

func (m *Monitor) handleAlertUnacknowledged(alert *alerts.Alert, user string) {
	if m.incidentStore == nil || alert == nil {
		return
	}
	m.incidentStore.RecordAlertUnacknowledged(alert, user)
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

	state := m.GetState()
	frontendState := state.ToFrontend()
	m.updateResourceStore(state)
	frontendState.Resources = m.getResourcesForBroadcast()
	// Use tenant-aware broadcast method
	m.broadcastState(hub, frontendState)
}

// recordAuthFailure records an authentication failure for a node
func (m *Monitor) checkMockAlerts() {
	defer recoverFromPanic("checkMockAlerts")

	log.Info().Bool("mockEnabled", mock.IsMockEnabled()).Msg("checkMockAlerts called")
	if !mock.IsMockEnabled() {
		log.Info().Msg("mock mode not enabled, skipping mock alert check")
		return
	}

	// Get mock state
	state := mock.GetMockState()

	log.Info().
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
	log.Info().
		Int("trackedNodes", len(existingNodes)).
		Msg("Collecting resources for alert cleanup in mock mode")
	m.alertManager.CleanupAlertsForNodes(existingNodes)

	guestsByKey, guestsByVMID := buildGuestLookups(state, m.guestMetadataStore)
	pveStorage := state.Backups.PVE.StorageBackups
	if len(pveStorage) == 0 && len(state.PVEBackups.StorageBackups) > 0 {
		pveStorage = state.PVEBackups.StorageBackups
	}
	pbsBackups := state.Backups.PBS
	if len(pbsBackups) == 0 && len(state.PBSBackups) > 0 {
		pbsBackups = state.PBSBackups
	}
	pmgBackups := state.Backups.PMG
	if len(pmgBackups) == 0 && len(state.PMGBackups) > 0 {
		pmgBackups = state.PMGBackups
	}
	m.alertManager.CheckBackups(pveStorage, pbsBackups, pmgBackups, guestsByKey, guestsByVMID)

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
	log.Info().Int("storageCount", len(state.Storage)).Msg("checking storage alerts")
	for _, storage := range state.Storage {
		log.Debug().
			Str("name", storage.Name).
			Float64("usage", storage.Usage).
			Msg("Checking storage for alerts")
		m.alertManager.CheckStorage(storage)
	}

	// Check alerts for PBS instances
	log.Info().Int("pbsCount", len(state.PBSInstances)).Msg("checking PBS alerts")
	for _, pbsInst := range state.PBSInstances {
		m.alertManager.CheckPBS(pbsInst)
	}

	// Check alerts for PMG instances
	log.Info().Int("pmgCount", len(state.PMGInstances)).Msg("checking PMG alerts")
	for _, pmgInst := range state.PMGInstances {
		m.alertManager.CheckPMG(pmgInst)
	}

	// Cache the latest alert snapshots directly in the mock data so the API can serve
	// mock state without needing to grab the alert manager lock again.
	mock.UpdateAlertSnapshots(m.alertManager.GetActiveAlerts(), m.alertManager.GetRecentlyResolved())
}
