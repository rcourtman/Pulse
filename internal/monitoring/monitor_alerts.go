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
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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
	m.mu.Lock()
	m.alertResolvedAICallback = callback
	m.mu.Unlock()
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
	// The lister serves the mock connection ledger while mock mode is on.
	scope := m.mockModeFence.begin()
	for _, snap := range lister() {
		scope.run(func() { m.alertManager.CheckConnection(snap) })
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
	// Compatibility for ID-only callers. The live monitor is wired to the
	// occurrence snapshot callback, not a lookup after asynchronous dispatch.
	if m.alertManager == nil {
		return
	}
	m.handleResolvedAlert(m.alertManager.GetResolvedAlert(alertID))
}

func (m *Monitor) handleResolvedAlert(resolvedAlert *alerts.ResolvedAlert) {
	if resolvedAlert == nil || resolvedAlert.Alert == nil {
		return
	}
	alertID := resolvedAlert.Alert.ID

	if m.wsHub != nil {
		// The legacy websocket message has only an ID. Do not remove a current
		// firing occurrence that is already visible under that reusable ID.
		active := false
		if m.alertManager != nil {
			_, active = m.alertManager.DiagnoseAlertDelivery(alertID)
		}
		if !active {
			m.wsHub.BroadcastAlertResolvedToTenant(m.GetOrgID(), alertID)
		}
	}

	// Always trigger AI callback, regardless of notification suppression.
	m.mu.RLock()
	aiCallback := m.alertResolvedAICallback
	m.mu.RUnlock()
	if aiCallback != nil {
		go aiCallback(resolvedAlert.Alert.Clone())
	}

	// Handle notifications — recovery notifications respect quiet hours.
	// If the original alert would have been suppressed during quiet hours,
	// the recovery notification is also suppressed to avoid noise.
	if m.notificationMgr != nil {
		m.notificationMgr.CancelResolvedAlert(resolvedAlert.Alert)
		if m.notificationMgr.GetNotifyOnResolve() {
			if m.alertManager != nil && m.alertManager.ShouldSuppressResolvedNotification(resolvedAlert.Alert) {
				log.Info().Str("alertID", alertID).Msg("Resolved notification suppressed during quiet hours")
			} else {
				// Only occurrence/destination receipts establish who saw the
				// firing. A pending retry or lost in-memory cooldown marker
				// after restart must not suppress recovery for other recipients.
				go m.notificationMgr.SendResolvedAlert(resolvedAlert)
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

	current, escalationLevel, eligible := m.alertManager.PrepareEscalationNotification(alert, level)
	if !eligible {
		return
	}

	alert = current

	if m.alertManager.ShouldSuppressNotification(alert) {
		log.Info().
			Str("alertID", alert.ID).
			Int("level", level).
			Msg("Escalated notification suppressed during quiet hours")
		m.broadcastEscalatedAlert(hub, alert)
		return
	}

	if m.notificationMgr != nil {
		if len(escalationLevel.DestinationIDs) > 0 {
			m.notificationMgr.SendEscalatedAlertToDestinations(alert, escalationLevel.Notify, escalationLevel.DestinationIDs)
			m.broadcastEscalatedAlert(hub, alert)
			return
		}
		switch strings.ToLower(strings.TrimSpace(escalationLevel.Notify)) {
		case "", "all", "email", "webhook", "webhooks", "apprise":
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
	if strings.TrimSpace(actor) == "" {
		actor = event.Details["actor"]
	}
	timelineAlert := alert
	if alerts.IsSystemAlert(alert) && strings.TrimSpace(alert.ResourceID) == "" {
		timelineAlert = alert.Clone()
		timelineAlert.ResourceID = "pulse-system"
	}
	switch event.Type {
	case eventlog.TypeFired, eventlog.TypeRefired:
		if m.incidentStore != nil {
			if event.Type == eventlog.TypeRefired {
				m.incidentStore.RecordAlertRefired(timelineAlert, event.OccurredAt)
			} else {
				m.incidentStore.RecordAlertFired(timelineAlert)
			}
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
	case eventlog.TypeSnoozed:
		timelineAlert = timelineAlert.Clone()
		if timelineAlert.Metadata == nil {
			timelineAlert.Metadata = make(map[string]any)
		}
		if until := strings.TrimSpace(event.Details["until"]); until != "" {
			timelineAlert.Metadata["snoozedUntil"] = until
		}
		m.recordAlertTimelineChange(timelineAlert, unifiedresources.ChangeAlertSnoozed, event.OccurredAt, actor)
	case eventlog.TypeUnsnoozed:
		m.recordAlertTimelineChange(timelineAlert, unifiedresources.ChangeAlertUnsnoozed, event.OccurredAt, actor)
	case eventlog.TypeResolved:
		if m.incidentStore != nil {
			m.incidentStore.RecordAlertResolved(timelineAlert, event.OccurredAt)
		}
		m.recordAlertTimelineChange(timelineAlert, unifiedresources.ChangeAlertResolved, event.OccurredAt, "")
	case eventlog.TypeHistoryImported:
		m.materializeImportedAlertTimeline(timelineAlert, event.OccurredAt)
	}
}

func (m *Monitor) materializeImportedAlertTimeline(alert *alerts.Alert, importedAt time.Time) {
	if m == nil || alert == nil || m.incidentStore == nil {
		return
	}

	var resolvedAt *time.Time
	if !m.isActiveAlertOccurrence(alert) {
		endedAt := importedAt
		if alert.OperationalRecord != nil && alert.OperationalRecord.ResolvedAt != nil && !alert.OperationalRecord.ResolvedAt.IsZero() {
			endedAt = *alert.OperationalRecord.ResolvedAt
		} else if alert.LastSeen.After(endedAt) {
			endedAt = alert.LastSeen
		}
		if endedAt.IsZero() {
			endedAt = alert.LastSeen
		}
		if !endedAt.IsZero() {
			if !alert.StartTime.IsZero() && endedAt.Before(alert.StartTime) {
				endedAt = alert.StartTime
			}
			resolvedAt = &endedAt
		}
	}

	m.incidentStore.EnsureAlertOccurrence(alert, resolvedAt)
	firedAt := alert.StartTime
	if firedAt.IsZero() {
		firedAt = importedAt
	}
	m.recordAlertTimelineChange(alert, unifiedresources.ChangeAlertFired, firedAt, "")
	if alert.Acknowledged {
		ackAt := alert.AckTime
		if ackAt == nil || ackAt.IsZero() {
			ackAt = &firedAt
		}
		m.recordAlertTimelineChange(alert, unifiedresources.ChangeAlertAcknowledged, *ackAt, alert.AckUser)
	}
	if resolvedAt != nil {
		m.recordAlertTimelineChange(alert, unifiedresources.ChangeAlertResolved, *resolvedAt, "")
	}
}

func (m *Monitor) isActiveAlertOccurrence(candidate *alerts.Alert) bool {
	if m == nil || m.alertManager == nil || candidate == nil {
		return false
	}
	for _, active := range m.alertManager.GetActiveAlerts() {
		if active.ID != candidate.ID {
			continue
		}
		if candidate.StartTime.IsZero() || active.StartTime.IsZero() || active.StartTime.Equal(candidate.StartTime) {
			return true
		}
	}
	return false
}

const (
	// alertLifecycleProjectionConsumer names the replay watermark shared by the
	// incident-timeline and canonical resource-change projections, which one
	// replay pass applies together.
	alertLifecycleProjectionConsumer = "alert-lifecycle-timelines-v1"
	// alertProjectionCheckpointEvery bounds how much replay progress a mid-pass
	// crash can lose: the watermark is persisted after this many visited events
	// as well as at the end of a completed pass.
	alertProjectionCheckpointEvery = 512
)

// scheduleAlertProjectionCatchUp runs lifecycle projection replay and
// active-alert reconciliation in the background. The walk is bounded by the
// durable projection watermark, so it must never run synchronously on the
// serving path: a large un-projected backlog (first boot after upgrade, a
// reset watermark) would otherwise block router construction and health
// serving, and dev supervisors kill an unresponsive backend long before a full
// 200MB-log replay finishes.
func (m *Monitor) scheduleAlertProjectionCatchUp() {
	if m == nil {
		return
	}
	m.alertProjectionWG.Add(1)
	go func() {
		defer m.alertProjectionWG.Done()
		defer recoverFromPanic("alertProjectionCatchUp")
		m.replayAlertLifecycleProjections()
		m.reconcileActiveAlertTimelines()
	}()
}

func (m *Monitor) replayAlertLifecycleProjections() {
	if m == nil || m.alertManager == nil {
		return
	}
	// Serialize passes instead of skipping: a second trigger waits for the
	// in-flight pass and then walks the (now tiny) remaining tail, so callers
	// that need replay-complete semantics can rely on a finished call.
	m.alertProjectionReplayMu.Lock()
	defer m.alertProjectionReplayMu.Unlock()

	m.mu.RLock()
	_, hasRecorder := m.resourceStore.(canonicalResourceChangeRecorder)
	hasIncidents := m.incidentStore != nil
	m.mu.RUnlock()
	// The watermark only advances when the full projection surface is
	// attached. A pass that runs before the canonical resource store exists
	// repairs what it can but must not mark those events applied, or their
	// resource-timeline projections would never materialize.
	advance := hasRecorder && hasIncidents

	afterID := m.alertManager.LifecycleProjectionWatermark(alertLifecycleProjectionConsumer)
	maxApplied := afterID
	visited := 0
	err := m.alertManager.ReplayLifecycleEvents(afterID, func(eventID int64, event alerts.LifecycleEvent) error {
		m.handleAlertLifecycleEvent(event)
		if eventID > maxApplied {
			maxApplied = eventID
		}
		visited++
		if advance && visited%alertProjectionCheckpointEvery == 0 {
			m.alertManager.StoreLifecycleProjectionWatermark(alertLifecycleProjectionConsumer, maxApplied)
		}
		return nil
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to replay canonical alert lifecycle projections")
		return
	}
	if advance && maxApplied > afterID {
		m.alertManager.StoreLifecycleProjectionWatermark(alertLifecycleProjectionConsumer, maxApplied)
	}
	if visited > 0 {
		log.Info().
			Int("events", visited).
			Int64("watermark", maxApplied).
			Bool("watermarkAdvanced", advance).
			Msg("alert lifecycle projection replay completed")
	}
}

func (m *Monitor) reconcileActiveAlertTimelines() {
	if m == nil || m.alertManager == nil || m.incidentStore == nil {
		return
	}
	activeAlerts := m.alertManager.GetActiveAlerts()
	for i := range activeAlerts {
		alert := &activeAlerts[i]
		page, err := m.incidentStore.QueryIncidents(memory.IncidentQuery{AlertIdentifier: alert.ID, StartedAt: alert.StartTime, Limit: 1})
		if err != nil {
			log.Warn().Err(err).Msg("Skipping alert timeline reconciliation because canonical history is unavailable")
			continue
		}
		var timeline *memory.Incident
		if len(page.Incidents) > 0 {
			timeline = page.Incidents[0]
		}
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
	// Background catch-up replay runs concurrently with SetResourceStore, so
	// the store handle must be read under the monitor lock.
	m.mu.RLock()
	recorder, ok := m.resourceStore.(canonicalResourceChangeRecorder)
	m.mu.RUnlock()
	if !ok || recorder == nil {
		return
	}

	timelineChange := unifiedresources.AlertTimelineChange{
		AlertIdentifier: alert.ID,
		AlertStartedAt:  alert.StartTime,
		AlertType:       alert.Type,
		AlertLevel:      string(alert.Level),
		AlertMessage:    alert.Message,
		AlertValue:      alert.Value,
		AlertThreshold:  alert.Threshold,
		AlertMetadata:   alert.Metadata,
	}
	if summary := alert.Resolution.Summary(); summary != "" {
		timelineChange.ResolutionReason = string(alert.Resolution.Reason)
		timelineChange.ResolutionSummary = summary
	}
	change := unifiedresources.BuildAlertTimelineChange(alert.ResourceID, kind, occurredAt, actor, timelineChange)
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

	// Passes overlap and can still be reading the fixture when the monitor
	// leaves mock mode, so every evaluation below runs under the scope taken
	// before the mode check and stops once SetMockMode ends its epoch.
	scope := m.mockModeFence.begin()
	log.Debug().Bool("mockEnabled", mock.IsMockEnabled()).Msg("checkMockAlerts called")
	if !mock.IsMockEnabled() {
		log.Debug().Msg("mock mode not enabled, skipping mock alert check")
		return
	}

	// Get mock state
	graph, fixtureRevision := mock.CurrentFixtureGraphWithRevision()
	state := graph.State

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
	scope.run(func() { m.alertManager.CleanupAlertsForNodes(existingNodes) })

	m.checkBackupAlerts(context.Background())

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
		scope.run(func() { m.alertManager.CheckGuest(vm, "mock") })
		guestsChecked++
	}

	// Check alerts for containers (if we haven't hit the limit)
	for _, container := range state.Containers {
		if guestsChecked >= maxGuestsPerCycle {
			break
		}
		scope.run(func() { m.alertManager.CheckGuest(container, "mock") })
		guestsChecked++
	}

	// Check alerts for host agents. Live agents are evaluated as each report
	// lands, and mock mode discards reports and keeps fixture agents out of
	// the monitor state the host health sweep reads, so without this pass no
	// agent CPU, memory, disk, temperature or offline alert can open against
	// mock data. Agents go before nodes so a linked node hands its CPU, memory
	// and disk alerts to its agent on the first tick, as node-link
	// deduplication does in production.
	log.Debug().Int("hostCount", len(state.Hosts)).Msg("checking host agent alerts")
	m.evaluateMockHostAgents(scope, state.Hosts, state.Nodes, fixtureRevision)

	// Check alerts for each node
	for _, node := range state.Nodes {
		scope.run(func() { m.alertManager.CheckNode(node) })
	}

	// Check alerts for storage
	log.Debug().Int("storageCount", len(state.Storage)).Msg("checking storage alerts")
	for _, storage := range state.Storage {
		log.Debug().
			Str("name", storage.Name).
			Float64("usage", storage.Usage).
			Msg("Checking storage for alerts")
		trend := m.storageCapacityTrend(storage, time.Now())
		scope.run(func() { m.alertManager.CheckStorageWithCapacityTrend(storage, trend) })
	}

	// Check alerts for physical disks. Live disks are evaluated by the
	// physical disk poller, which only polls configured Proxmox instances, and
	// the fixture deliberately keeps a FAILED cohort and worn SSDs, so without
	// this pass the estate shows failing disks with no disk-health or
	// disk-wearout alert.
	log.Debug().Int("diskCount", len(state.PhysicalDisks)).Msg("checking physical disk alerts")
	m.checkMockPhysicalDiskAlerts(scope, state.PhysicalDisks, state.Nodes, state.Hosts)

	// Check alerts for PBS instances
	log.Debug().Int("pbsCount", len(state.PBSInstances)).Msg("checking PBS alerts")
	for _, pbsInst := range state.PBSInstances {
		scope.run(func() { m.alertManager.CheckPBS(pbsInst) })
	}

	// Check alerts for PMG instances
	log.Debug().Int("pmgCount", len(state.PMGInstances)).Msg("checking PMG alerts")
	for _, pmgInst := range state.PMGInstances {
		scope.run(func() { m.alertManager.CheckPMG(pmgInst) })
	}

	// Check alerts for Docker hosts (container state/health/metrics/updates and
	// swarm services). The mock estate deliberately includes degraded containers,
	// so skipping this loop leaves the docker alert lifecycle unexercisable
	// against mock data.
	log.Debug().Int("dockerHostCount", len(state.DockerHosts)).Msg("checking docker alerts")
	m.evaluateMockDockerHosts(scope, state.DockerHosts, fixtureRevision)

	// Cache the latest alert snapshots directly in the mock data so the API can serve
	// mock state without needing to grab the alert manager lock again.
	scope.run(func() {
		mock.UpdateAlertSnapshots(m.alertManager.GetActiveAlerts(), m.alertManager.GetRecentlyResolved())
	})
}

// evaluateMockHostAgents evaluates every fixture agent and remembers the set.
// A runtime mock config change rebuilds the estate, so an agent can leave it
// between passes; it then goes through HandleHostRemoved, as a deleted live
// agent does, or its alerts and node link would outlive it.
func (m *Monitor) evaluateMockHostAgents(scope mockModeScope, hosts []models.Host, nodes []models.Node, fixtureRevision uint64) {
	m.mockHostAgentsMu.Lock()
	defer m.mockHostAgentsMu.Unlock()

	// Both the mode epoch and structural revision must still match.
	if !scope.current() || !m.acceptMockFixturePassLocked(fixtureRevision) {
		return
	}

	current := make(map[string]models.Host, len(hosts))
	for _, host := range hosts {
		if host.ID != "" {
			current[host.ID] = host
		}
	}
	for id, host := range m.mockHostAgents {
		if _, ok := current[id]; ok {
			continue
		}
		if !scope.run(func() { m.alertManager.HandleHostRemoved(host) }) {
			// Leaving mock mode refused the removal; keep the agent so
			// forgetMockFixtureHosts still removes it.
			current[id] = host
		}
	}
	for _, host := range hosts {
		scope.run(func() { m.checkMockHostAlerts(host, nodes) })
	}
	m.mockHostAgents = current
}

// evaluateMockDockerHosts is the Docker counterpart of evaluateMockHostAgents,
// under the same lock and mode check. Nested Docker-in-LXC hosts are named
// after their guest's VMID, so a runtime mock config change can drop a host
// from the estate; it then goes through HandleDockerHostRemoved, as a deleted
// live host does. pruneStaleDockerAlerts clears the same alerts, but only when
// something reads state, so an unwatched stack would keep them active.
func (m *Monitor) evaluateMockDockerHosts(scope mockModeScope, hosts []models.DockerHost, fixtureRevision uint64) {
	m.mockHostAgentsMu.Lock()
	defer m.mockHostAgentsMu.Unlock()

	// Both the mode epoch and structural revision must still match.
	if !scope.current() || !m.acceptMockFixturePassLocked(fixtureRevision) {
		return
	}

	current := make(map[string]models.DockerHost, len(hosts))
	for _, host := range hosts {
		if host.ID != "" {
			current[host.ID] = host
		}
	}
	for id, host := range m.mockDockerHosts {
		if _, ok := current[id]; !ok {
			if !scope.run(func() { m.alertManager.HandleDockerHostRemoved(host) }) {
				// Keep refused departures for the mode switch to remove.
				current[id] = host
			}
		}
	}
	for _, host := range hosts {
		scope.run(func() { m.checkMockDockerHostAlerts(host) })
	}
	m.mockDockerHosts = current
}

// acceptMockFixturePassLocked reports whether a mock alert pass may update the
// tracked fixture hosts. A pass that took its snapshot before the monitor left
// mock mode must not evaluate fixtures after forgetMockFixtureHosts ran. Ticks
// start passes concurrently, so one that took its snapshot before an estate
// rebuild can also reach this lock after a newer pass; applying it would
// remove the new estate's hosts and re-evaluate retired ones. The fixture
// revision only advances on such rebuilds, so an older revision is rejected.
// Callers hold mockHostAgentsMu.
func (m *Monitor) acceptMockFixturePassLocked(fixtureRevision uint64) bool {
	if !mock.IsMockEnabled() || fixtureRevision < m.mockFixtureRevision {
		return false
	}
	m.mockFixtureRevision = fixtureRevision
	return true
}

// forgetMockFixtureHosts removes the fixture agents and Docker hosts when the
// monitor leaves mock mode. ClearActiveAlerts drops their alerts but not the
// agents' node links, and a leftover link would keep suppressing CPU, memory
// and disk alerts on any node that later carries the linked fixture node's ID.
// The removal also clears anything a pass already in flight
// recreated; mock mode is off before this runs, so no later pass can evaluate
// them again while it stays off. Disabling advanced the fixture revision, and
// recording it here keeps a pass paused across a disable and re-enable from
// evaluating the old estate once mock mode is back on.
func (m *Monitor) forgetMockFixtureHosts() {
	m.mockHostAgentsMu.Lock()
	defer m.mockHostAgentsMu.Unlock()

	if _, revision := mock.CurrentFixtureGraphWithRevision(); revision > m.mockFixtureRevision {
		m.mockFixtureRevision = revision
	}
	for _, host := range m.mockHostAgents {
		m.alertManager.HandleHostRemoved(host)
	}
	m.mockHostAgents = nil
	for _, host := range m.mockDockerHosts {
		m.alertManager.HandleDockerHostRemoved(host)
	}
	m.mockDockerHosts = nil
}

// checkMockHostAlerts applies the live host-agent boundary to a fixture. An
// offline fixture has no fresh telemetry, so it goes through the host
// connectivity lifecycle that evaluateHostAgents uses for a lapsed report.
// CheckHost treats its input as a fresh report and would mark the host online.
func (m *Monitor) checkMockHostAlerts(host models.Host, nodes []models.Node) {
	if strings.EqualFold(strings.TrimSpace(host.Status), "offline") {
		m.alertManager.HandleHostOfflineWithCorrelation(host, sharedSystemAlertCorrelationForHost(host, nodes))
		return
	}
	m.alertManager.CheckHost(host)
}

// checkMockDockerHostAlerts preserves the same evidence boundary as live
// agent monitoring. An explicitly offline fixture is missing fresh container
// telemetry; its last container states must not be reinterpreted as a fresh
// batch of independent exits. The host connectivity lifecycle owns that
// outage and clears child alerts once the offline confirmation floor is met.
func (m *Monitor) checkMockDockerHostAlerts(host models.DockerHost) {
	if strings.EqualFold(strings.TrimSpace(host.Status), "offline") {
		m.alertManager.HandleDockerHostOffline(host)
		return
	}
	m.alertManager.CheckDockerHost(host)
}

// checkMockPhysicalDiskAlerts applies the physical disk poller's alert
// boundary to fixture disks. The poller only evaluates disks on nodes it
// reached, so a disk on a node that is not online keeps whatever alert it
// had, and a device matched by the linked agent's --disk-exclude patterns is
// evaluated as healthy. The poller's wait for host-agent links to settle after
// a restart does not apply: fixture links and exclusions are complete from the
// first pass.
func (m *Monitor) checkMockPhysicalDiskAlerts(scope mockModeScope, disks []models.PhysicalDisk, nodes []models.Node, hosts []models.Host) {
	type nodeKey struct{ instance, name string }

	excludeByHost := make(map[string][]string, len(hosts))
	for _, host := range hosts {
		if len(host.DiskExclude) > 0 {
			excludeByHost[host.ID] = host.DiskExclude
		}
	}
	onlineNodes := make(map[nodeKey]bool, len(nodes))
	excludeByNode := make(map[nodeKey][]string)
	for _, node := range nodes {
		key := nodeKey{instance: node.Instance, name: node.Name}
		onlineNodes[key] = node.Status == "online"
		if patterns := excludeByHost[node.LinkedAgentID]; node.LinkedAgentID != "" && len(patterns) > 0 {
			excludeByNode[key] = patterns
		}
	}

	for _, disk := range disks {
		key := nodeKey{instance: disk.Instance, name: disk.Node}
		if !onlineNodes[key] {
			continue
		}
		scope.run(func() { m.checkPhysicalDiskAlerts(disk.Instance, disk, excludeByNode[key]) })
	}
}
