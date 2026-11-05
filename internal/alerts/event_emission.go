package alerts

// Alert event emission: the manager records lifecycle transitions and
// notification decisions — including suppressions, with reasons — into the
// append-only event log (docs/ALERT_ENGINE_EVOLUTION.md, Phase 0). Emission
// is fail-open: bounded journal admission precedes SQLite writes, and storage
// failures never suppress lifecycle or delivery callbacks.

import (
	"encoding/json"
	"os"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rs/zerolog/log"
)

// EnableEventLog opens the persistent alert event log under the manager's
// alerts directory. The monitoring bootstrap calls this once per manager;
// without it (tests, ephemeral managers) no events are recorded.
func (m *Manager) EnableEventLog() {
	if m == nil {
		return
	}
	if err := os.MkdirAll(m.alertsDir, 0700); err != nil {
		log.Warn().Err(err).Str("dir", m.alertsDir).Msg("alert event log disabled: cannot create alerts directory")
		return
	}
	store, err := eventlog.Open(m.alertsDir)
	if err != nil {
		log.Warn().Err(err).Msg("alert event log disabled")
		return
	}
	m.SetEventLog(store)
	m.eventHistoryAuthoritative.Store(m.importLegacyHistoryIntoEventLog(store))
	m.activeStateAuthoritative.Store(m.bootstrapActiveState(store))
}

// SetEventLog installs an event log store. Passing nil disables recording.
// The previous store, if any, is closed.
func (m *Manager) SetEventLog(store *eventlog.Store) {
	if m == nil {
		return
	}
	previous := m.eventLog.Swap(store)
	m.historyProjectionMu.Lock()
	m.historyProjection = nil
	m.historyProjectionMu.Unlock()
	authoritative := store != nil && m.historyManager != nil &&
		!m.historyManager.StorageFileExists() &&
		!m.historyManager.ImportedStorageFileExists() &&
		m.historyManager.StorageLoadError() == nil
	m.eventHistoryAuthoritative.Store(authoritative)
	if previous != nil && previous != store {
		previous.Close()
	}
}

func (m *Manager) eventLogStore() *eventlog.Store {
	if m == nil {
		return nil
	}
	return m.eventLog.Load()
}

// AlertEvents returns matching events from the persistent event log, newest
// first. It flushes buffered appends first so a caller reading right after a
// transition observes it.
func (m *Manager) AlertEvents(filter eventlog.Filter) ([]eventlog.Event, error) {
	store := m.eventLogStore()
	if store == nil {
		return nil, nil
	}
	if err := store.Settle(); err != nil {
		// Retained rows remain readable during an unsettled write or journal
		// fault. Durability status and response headers expose the missing tail.
		log.Warn().Err(err).Msg("reading retained alert events while history settlement is deferred")
	}
	return store.Query(filter)
}

// ReplayLifecycleEvents visits durable lifecycle transitions and migrated
// history snapshots with ids above afterID, in ascending durable-ID (commit)
// order. It is the projection-repair seam for consumers such as incident and
// resource timelines: delivery callbacks are deliberately not involved.
// Migrated snapshots carry a final occurrence state rather than a transition;
// consumers must expand only the facts present in that snapshot. afterID is the
// consumer's projection watermark (zero replays the full log); the visitor
// receives each event's durable id so it can advance that watermark once the
// event's projections are applied. ID order makes that watermark a
// contiguous-prefix cursor: occurrence timestamps (notably imported history)
// need not follow commit order, so a time-ordered pass interrupted after a
// checkpoint could leave lower, unvisited ids below the stored watermark. Each
// pass stops at the committed high-water id captured when it starts, and a
// visitor error ends the pass and is returned. Chronological history reads
// keep using the time-ordered walk.
func (m *Manager) ReplayLifecycleEvents(afterID int64, visit func(eventID int64, event LifecycleEvent) error) error {
	if m == nil || visit == nil {
		return nil
	}
	store := m.eventLogStore()
	if store == nil {
		return nil
	}
	return store.WalkByID(eventlog.Filter{AfterID: afterID, Types: []string{
		eventlog.TypeFired,
		eventlog.TypeRefired,
		eventlog.TypeResolved,
		eventlog.TypeAcknowledged,
		eventlog.TypeUnacknowledged,
		eventlog.TypeSnoozed,
		eventlog.TypeUnsnoozed,
		eventlog.TypeHistoryImported,
	}}, func(event eventlog.Event) error {
		if len(event.Snapshot) == 0 {
			return nil
		}
		var snapshot Alert
		if err := json.Unmarshal(event.Snapshot, &snapshot); err != nil {
			log.Warn().Err(err).
				Int64("eventID", event.ID).
				Str("alertID", event.AlertID).
				Msg("skipping invalid alert lifecycle snapshot during projection replay")
			return nil
		}
		return visit(event.ID, LifecycleEvent{
			Type:       event.Type,
			OccurredAt: event.OccurredAt,
			Alert:      &snapshot,
			Details:    cloneStringMap(event.Details),
			Persisted:  true,
		})
	})
}

// lifecycleIDOrderWatermarkNamespace versions lifecycle replay cursors for the
// durable-ID ordered walk. Releases that replayed in occurrence-time order
// stored their cursor under the bare consumer name, and an interrupted pass
// there could persist an ID above retained events it never projected. That
// legacy cursor is not a contiguous ID prefix, so it is never read: the first
// pass after upgrade starts this namespace at zero and replays the retained
// log once. Projections key their canonical writes by event identity, so the
// facts already applied are rewritten, not duplicated. The legacy key is left
// untouched and unused. Change the version only together with a change to the
// replay order or cursor meaning.
const lifecycleIDOrderWatermarkNamespace = "lifecycle-id-order-v1:"

// lifecycleWatermarkName maps a consumer to its ID-ordered cursor. An empty
// consumer stays empty, so the event log's name validation still applies.
func lifecycleWatermarkName(name string) string {
	if name == "" {
		return ""
	}
	return lifecycleIDOrderWatermarkNamespace + name
}

// LifecycleProjectionWatermark returns the durable ID-ordered replay
// watermark for a named projection consumer, or zero when no event log is
// enabled or the consumer has no cursor in the current namespace.
func (m *Manager) LifecycleProjectionWatermark(name string) int64 {
	if m == nil {
		return 0
	}
	return m.eventLogStore().ProjectionWatermark(lifecycleWatermarkName(name))
}

// StoreLifecycleProjectionWatermark durably advances (or resets) the
// ID-ordered replay watermark for a named projection consumer. Failures are
// logged, not fatal: a stale watermark only means the next replay revisits
// already-idempotent projections.
func (m *Manager) StoreLifecycleProjectionWatermark(name string, eventID int64) {
	if m == nil {
		return
	}
	if err := m.eventLogStore().SetProjectionWatermark(lifecycleWatermarkName(name), eventID); err != nil {
		log.Warn().Err(err).
			Str("consumer", name).
			Int64("eventID", eventID).
			Msg("failed to persist lifecycle projection watermark")
	}
}

// recordAlertEvent emits one canonical lifecycle transition and appends it to
// the durable event log. alertID is the identity fallback for callers whose
// *Alert may be nil (some resolve paths); when the alert is present its
// exported ID wins. Lifecycle subscribers run even when the diagnostic event
// log is unavailable, so incident history never depends on notification or
// event-log availability.
func (m *Manager) recordAlertEvent(eventType string, alert *Alert, alertID, reason, message string, details map[string]string) {
	event := eventlog.Event{
		OccurredAt: m.policyNow(),
		Type:       eventType,
		AlertID:    alertID,
		Reason:     reason,
		Message:    message,
		Details:    details,
	}
	if alert != nil {
		if id := exportedAlertID(alert, alertID); id != "" {
			event.AlertID = id
		}
		event.ResourceID = alert.ResourceID
		event.ResourceName = alert.ResourceName
		event.AlertType = alert.Type
		event.Level = string(alert.Level)
		if eventCarriesAlertSnapshot(eventType) {
			if snapshot, err := json.Marshal(cloneAlertForOutput(alert)); err == nil {
				event.Snapshot = snapshot
			}
		}
	}
	if event.AlertID == "" {
		return
	}
	if eventCarriesAlertSnapshot(eventType) {
		if eventType == eventlog.TypeResolved || eventType == eventlog.TypeActiveRetired {
			m.removeActiveRecoveryAlert(alert, event.AlertID)
		} else if alert != nil {
			if eventType == eventlog.TypeFired || eventType == eventlog.TypeRefired {
				m.setActiveRecoveryAlert(alert, event.AlertID)
			} else {
				m.setActiveRecoveryAlert(alert, event.AlertID, eventType)
			}
		}
	}

	persisted := false
	if store := m.eventLogStore(); store != nil {
		if eventCarriesAlertSnapshot(eventType) {
			if err := store.AppendDurable(event); err != nil {
				m.activeStateFailureEpoch.Add(1)
				if mirrorErr := m.checkpointActiveRecoveryAfterDurableFailure(err); mirrorErr != nil {
					log.Error().Err(mirrorErr).
						Str("alertID", event.AlertID).
						Str("eventType", eventType).
						Msg("failed to checkpoint active alert recovery after lifecycle write failure")
				}
				log.Error().Err(err).
					Str("alertID", event.AlertID).
					Str("eventType", eventType).
					Msg("durable alert lifecycle event write failed")
			} else {
				persisted = true
			}
		} else {
			store.Append(event)
		}
	}

	if eventCarriesAlertSnapshot(eventType) && eventType != eventlog.TypeActiveRetired && eventType != eventlog.TypeActiveMoved && alert != nil {
		lifecycleEvent := LifecycleEvent{
			Type:       eventType,
			OccurredAt: event.OccurredAt,
			Alert:      cloneAlertForOutput(alert),
			Details:    cloneStringMap(details),
			Persisted:  persisted,
		}
		for _, callback := range m.getLifecycleCallbacks() {
			func(cb func(LifecycleEvent)) {
				defer func() {
					if recovered := recover(); recovered != nil {
						log.Error().
							Interface("panic", recovered).
							Str("alertID", event.AlertID).
							Str("eventType", eventType).
							Msg("panic in alert lifecycle callback")
					}
				}()
				cb(lifecycleEvent)
			}(callback)
		}
	}
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

// eventCarriesAlertSnapshot reports whether an event type records the full
// alert state alongside the transition. Lifecycle transitions do — they are
// what alert history is projected from — while high-frequency notification
// decisions stay lean.
func eventCarriesAlertSnapshot(eventType string) bool {
	switch eventType {
	case eventlog.TypeActiveRetired, eventlog.TypeActiveMoved:
		return true
	case eventlog.TypeFired, eventlog.TypeRefired, eventlog.TypeResolved,
		eventlog.TypeAcknowledged, eventlog.TypeUnacknowledged, eventlog.TypeSnoozed,
		eventlog.TypeUnsnoozed, eventlog.TypeEscalated:
		return true
	default:
		return false
	}
}

// EventHistoryDurability reports retained obligations and explicit missing-fact
// evidence independently of migration authority and delivery callbacks.
func (m *Manager) EventHistoryDurability() eventlog.DurabilityStatus {
	if m == nil {
		return (*eventlog.Store)(nil).DurabilityStatus()
	}
	status := m.eventLogStore().DurabilityStatus()
	if !m.eventHistoryAuthoritative.Load() && status.Error == "" {
		status.Error = "legacy alert history migration is incomplete"
	}
	return status
}

// AcknowledgeEventHistoryGaps accepts missing facts at an exact observed gap
// revision. Pending retained events are never discarded by this operation.
func (m *Manager) AcknowledgeEventHistoryGaps(revision uint64, reason string) error {
	if m == nil {
		return (*eventlog.Store)(nil).AcknowledgeGaps(revision, reason)
	}
	return m.eventLogStore().AcknowledgeGaps(revision, reason)
}
