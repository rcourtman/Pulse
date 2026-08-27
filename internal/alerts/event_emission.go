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
	m.importLegacyHistoryIntoEventLog(store)
}

// SetEventLog installs an event log store. Passing nil disables recording.
// The previous store, if any, is closed.
func (m *Manager) SetEventLog(store *eventlog.Store) {
	if m == nil {
		return
	}
	previous := m.eventLog.Swap(store)
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
	store.Flush()
	return store.Query(filter)
}

// recordAlertEvent appends one event for an alert. alertID is the identity
// fallback for callers whose *Alert may be nil (some resolve paths); when the
// alert is present its exported ID wins. Never blocks; safe under m.mu.
func (m *Manager) recordAlertEvent(eventType string, alert *Alert, alertID, reason, message string, details map[string]string) {
	store := m.eventLogStore()
	if store == nil {
		return
	}

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
	store.Append(event)
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
