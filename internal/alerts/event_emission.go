package alerts

// Alert event emission: the manager records lifecycle transitions and
// notification decisions — including suppressions, with reasons — into the
// append-only event log (docs/ALERT_ENGINE_EVOLUTION.md, Phase 0). Emission
// is additive and non-blocking: a nil or failed store records nothing and
// changes no lifecycle behavior.

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
	if err := store.Flush(); err != nil {
		return nil, err
	}
	return store.Query(filter)
}

// ReplayLifecycleEvents visits every durable lifecycle transition oldest
// first. It is the projection-repair seam for consumers such as incident and
// resource timelines: delivery callbacks are deliberately not involved.
func (m *Manager) ReplayLifecycleEvents(visit func(LifecycleEvent) error) error {
	if m == nil || visit == nil {
		return nil
	}
	store := m.eventLogStore()
	if store == nil {
		return nil
	}
	return store.WalkOldest(eventlog.Filter{Types: []string{
		eventlog.TypeFired,
		eventlog.TypeRefired,
		eventlog.TypeResolved,
		eventlog.TypeAcknowledged,
		eventlog.TypeUnacknowledged,
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
		return visit(LifecycleEvent{
			Type:       event.Type,
			OccurredAt: event.OccurredAt,
			Alert:      &snapshot,
			Details:    cloneStringMap(event.Details),
			Persisted:  true,
		})
	})
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

	persisted := false
	if store := m.eventLogStore(); store != nil {
		if eventCarriesAlertSnapshot(eventType) {
			if err := store.AppendDurable(event); err != nil {
				m.eventHistoryAuthoritative.Store(false)
				m.markActiveStateDegraded(err)
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

	if eventCarriesAlertSnapshot(eventType) && alert != nil {
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
	case eventlog.TypeFired, eventlog.TypeRefired, eventlog.TypeResolved,
		eventlog.TypeAcknowledged, eventlog.TypeUnacknowledged, eventlog.TypeEscalated:
		return true
	default:
		return false
	}
}
