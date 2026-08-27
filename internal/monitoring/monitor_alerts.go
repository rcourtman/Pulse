package monitoring

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
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

// DeadManStatus returns the external watchdog state without exposing the
// configured secret-bearing ping URL.
func (m *Monitor) DeadManStatus() DeadManStatus {
	if m == nil || m.deadMan == nil {
		return (*deadManRuntime)(nil).statusSnapshot()
	}
	status := m.deadMan.statusSnapshot()
	if m.deadManConfigurationLoadError() != nil {
		status.Configured = true
		status.State = "configuration_unavailable"
		status.LastError = "Saved external watchdog configuration could not be read"
	} else if strings.TrimSpace(m.deadManConfigSnapshot().PingURL) == "" {
		status.Configured = false
		status.State = "disabled"
		status.LastAttemptAt = nil
		status.LastSuccessAt = nil
		status.ConsecutiveFailures = 0
		status.LastError = ""
	}
	return status
}

// DeadManConfig returns the in-memory encrypted-destination configuration.
// API callers must mask PingURL before returning it to a client.
func (m *Monitor) DeadManConfig() notifications.DeadManConfig {
	return m.deadManConfigSnapshot()
}

func (m *Monitor) deadManConfigSnapshot() notifications.DeadManConfig {
	if m == nil {
		return notifications.DeadManConfig{}
	}
	m.deadManConfigMu.RLock()
	defer m.deadManConfigMu.RUnlock()
	return m.deadManConfig
}

func (m *Monitor) deadManConfigurationLoadError() error {
	if m == nil {
		return nil
	}
	m.deadManConfigMu.RLock()
	defer m.deadManConfigMu.RUnlock()
	return m.deadManConfigLoadErr
}

// UpdateDeadManConfig persists the secret before changing live behavior, so a
// failed encrypted write can never create a runtime-only watchdog setting.
func (m *Monitor) UpdateDeadManConfig(config notifications.DeadManConfig) error {
	if m == nil || m.configPersist == nil {
		return fmt.Errorf("dead-man configuration persistence unavailable")
	}
	config = notifications.NormalizeDeadManConfig(config)
	if err := notifications.ValidateDeadManPingURL(config.PingURL); err != nil {
		return err
	}
	if err := m.configPersist.SaveDeadManConfig(config); err != nil {
		return err
	}
	m.deadManConfigMu.Lock()
	m.deadManConfig = config
	m.deadManConfigLoadErr = nil
	m.deadManConfigMu.Unlock()
	if m.alertManager != nil {
		m.alertManager.ClearSystemAlert(alerts.DeadManStateAlertType)
	}
	if m.deadMan != nil {
		m.deadMan.notifyConfigChanged()
	}
	return nil
}

func (m *Monitor) markDeadManMonitoringProgress(at time.Time) {
	if m == nil || at.IsZero() {
		return
	}
	m.deadManProgressUnixNano.Store(at.UTC().UnixNano())
}

func (m *Monitor) deadManMonitoringProgress() time.Time {
	if m == nil {
		return time.Time{}
	}
	value := m.deadManProgressUnixNano.Load()
	if value <= 0 {
		return time.Time{}
	}
	return time.Unix(0, value).UTC()
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

// SetAlertPushCallback wires best-effort mobile push delivery for canonical
// alerts. The callback is intentionally transport-agnostic; the API layer owns
// Relay and decides which alert classes are safe and useful to send.
func (m *Monitor) SetAlertPushCallback(callback func(*alerts.Alert)) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.alertPushCallback = callback
	m.mu.Unlock()
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
	m.mu.RLock()
	pushCallback := m.alertPushCallback
	m.mu.RUnlock()
	if pushCallback != nil {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Error().
						Interface("panic", recovered).
						Str("alertID", alert.ID).
						Msg("panic in alert push callback")
				}
			}()
			pushCallback(alert)
		}()
	}

}

func (m *Monitor) handleAlertResolved(alertID string) {
	var resolvedAlert *alerts.ResolvedAlert

	if m.wsHub != nil {
		m.wsHub.BroadcastAlertResolvedToTenant(m.GetOrgID(), alertID)
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
		firingNeverDelivered := m.notificationMgr.CancelAlert(alertID)
		if m.notificationMgr.GetNotifyOnResolve() {
			if resolvedAlert == nil {
				resolvedAlert = m.alertManager.GetResolvedAlert(alertID)
			}
			if resolvedAlert != nil && resolvedAlert.Alert != nil {
				if firingNeverDelivered {
					// The firing notification was still in the grouping window
					// or waiting in the queue (e.g. quiet-hours replay) when the
					// alert resolved, and CancelAlert just cancelled it. A
					// recovery for an alert the user never saw fire is noise.
					log.Info().
						Str("alertID", alertID).
						Msg("Resolved notification suppressed because the firing notification was cancelled before delivery")
				} else if m.alertManager.ShouldSuppressResolvedNotification(resolvedAlert.Alert) {
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
		switch strings.ToLower(strings.TrimSpace(escalationLevel.Notify)) {
		case "", "all", "email", "webhook", "webhooks":
			m.notificationMgr.SendEscalatedAlert(alert, escalationLevel.Notify)
		default:
			log.Warn().
				Str("alertID", alert.ID).
				Int("level", level).
				Str("notify", escalationLevel.Notify).
				Msg("Skipping alert escalation with unknown notification target")
		}
	}

	m.broadcastEscalatedAlert(hub, alert)
}

func (m *Monitor) handleAlertLifecycleEvent(event alerts.LifecycleEvent) {
	alert := event.Alert
	if m == nil || alert == nil {
		return
	}

	actor := event.Details["user"]
	timelineAlert := alert
	if alerts.IsSystemAlert(alert) && strings.TrimSpace(alert.ResourceID) == "" {
		timelineAlert = alert.Clone()
		timelineAlert.ResourceID = "pulse-system"
	}
	switch event.Type {
	case eventlog.TypeFired, eventlog.TypeRefired:
		if m.incidentStore != nil {
			m.incidentStore.RecordAlertFired(timelineAlert)
		}
		occurredAt := event.OccurredAt
		if event.Type == eventlog.TypeFired && !alert.StartTime.IsZero() {
			occurredAt = alert.StartTime
		}
		m.recordAlertTimelineChange(timelineAlert, unifiedresources.ChangeAlertFired, occurredAt, "")
	case eventlog.TypeAcknowledged:
		if m.incidentStore != nil {
			m.incidentStore.RecordAlertAcknowledged(timelineAlert, actor)
		}
		occurredAt := event.OccurredAt
		if alert.AckTime != nil && !alert.AckTime.IsZero() {
			occurredAt = *alert.AckTime
		}
		m.recordAlertTimelineChange(timelineAlert, unifiedresources.ChangeAlertAcknowledged, occurredAt, actor)
	case eventlog.TypeUnacknowledged:
		if m.incidentStore != nil {
			m.incidentStore.RecordAlertUnacknowledged(timelineAlert, actor)
		}
		m.recordAlertTimelineChange(timelineAlert, unifiedresources.ChangeAlertUnacknowledged, event.OccurredAt, actor)
	case eventlog.TypeResolved:
		if m.incidentStore != nil {
			m.incidentStore.RecordAlertResolved(timelineAlert, event.OccurredAt)
		}
		m.recordAlertTimelineChange(timelineAlert, unifiedresources.ChangeAlertResolved, event.OccurredAt, "")
	}
}

func (m *Monitor) replayAlertLifecycleProjections() {
	if m == nil || m.alertManager == nil {
		return
	}
	if err := m.alertManager.ReplayLifecycleEvents(func(event alerts.LifecycleEvent) error {
		m.handleAlertLifecycleEvent(event)
		return nil
	}); err != nil {
		log.Error().Err(err).Msg("failed to replay canonical alert lifecycle projections")
	}
}

func (m *Monitor) reconcileActiveAlertTimelines() {
	if m == nil || m.alertManager == nil || m.incidentStore == nil {
		return
	}
	activeAlerts := m.alertManager.GetActiveAlerts()
	for i := range activeAlerts {
		alert := &activeAlerts[i]
		timeline := m.incidentStore.GetTimelineByAlertAt(alert.ID, alert.StartTime)
		if timeline != nil {
			hasFired := false
			for _, event := range timeline.Events {
				if event.Type == memory.IncidentEventAlertFired {
					hasFired = true
					break
				}
			}
			if hasFired {
				continue
			}
		}
		m.handleAlertLifecycleEvent(alerts.LifecycleEvent{
			Type:       eventlog.TypeFired,
			OccurredAt: alert.StartTime,
			Alert:      alert,
		})
	}
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
	change.ID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join([]string{
		"pulse-alert-lifecycle-v1",
		strings.TrimSpace(alert.ID),
		strings.TrimSpace(alert.ResourceID),
		string(kind),
		occurredAt.UTC().Format(time.RFC3339Nano),
	}, "\x00"))).String()
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
	m.broadcastCurrentState(hub)
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

	// Check alerts for Docker hosts (container state/health/metrics/updates and
	// swarm services). The mock estate deliberately includes degraded containers,
	// so skipping this loop leaves the docker alert lifecycle unexercisable
	// against mock data.
	log.Debug().Int("dockerHostCount", len(state.DockerHosts)).Msg("checking docker alerts")
	for _, dockerHost := range state.DockerHosts {
		m.alertManager.CheckDockerHost(dockerHost)
	}

	// Cache the latest alert snapshots directly in the mock data so the API can serve
	// mock state without needing to grab the alert manager lock again.
	mock.UpdateAlertSnapshots(m.alertManager.GetActiveAlerts(), m.alertManager.GetRecentlyResolved())
}
