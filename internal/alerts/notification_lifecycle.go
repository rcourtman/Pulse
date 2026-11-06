package alerts

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

// PatrolFindingNotificationIDPrefix identifies delivery-only findings. The
// alert manager never owns these occurrences, including during router startup.
const PatrolFindingNotificationIDPrefix = "patrol-finding-"

type NotificationOccurrenceState string

const (
	NotificationOccurrenceUnavailable NotificationOccurrenceState = "unavailable"
	NotificationOccurrenceNoRecord    NotificationOccurrenceState = "no-record"
	NotificationOccurrenceActive      NotificationOccurrenceState = "active"
	NotificationOccurrenceResolved    NotificationOccurrenceState = "resolved"
)

type NotificationOccurrenceLifecycle struct {
	// CurrentInactive is positive absence from a complete current active-state
	// inventory. It rejects stale firing, but never proves historical resolution.
	CurrentInactive    bool
	State              NotificationOccurrenceState
	ResolvedAt         time.Time
	LatestActivationAt time.Time
}

// NotificationOccurrenceLifecycle supplies positive occurrence facts, preserving
// the difference between healthy history compaction and unavailable authority.
func (m *Manager) NotificationOccurrenceLifecycle(id string, start time.Time) NotificationOccurrenceLifecycle {
	unknown := NotificationOccurrenceLifecycle{State: NotificationOccurrenceUnavailable}
	if m == nil || id == "" || start.IsZero() || strings.HasPrefix(id, PatrolFindingNotificationIDPrefix) {
		return unknown
	}
	m.stopMu.RLock()
	stopping := m.stopping
	m.stopMu.RUnlock()
	if stopping {
		return unknown
	}
	m.mu.RLock()
	for _, alert := range m.activeAlerts {
		if alert != nil && (alert.ID == id || exportedAlertID(alert, alert.ID) == id) && alert.StartTime.Equal(start) {
			m.mu.RUnlock()
			return NotificationOccurrenceLifecycle{State: NotificationOccurrenceActive}
		}
	}
	m.mu.RUnlock()
	unknown.CurrentInactive = m.activeStateAuthoritative.Load()
	store := m.eventLogStore()
	if store == nil || !m.eventHistoryAuthoritative.Load() || !m.activeStateAuthoritative.Load() {
		return unknown
	}
	if !store.Healthy() {
		return unknown
	}
	result := NotificationOccurrenceLifecycle{State: NotificationOccurrenceNoRecord}
	err := store.WalkOldest(eventlog.Filter{AlertID: id, Types: []string{eventlog.TypeFired, eventlog.TypeRefired, eventlog.TypeResolved, eventlog.TypeHistoryImported}}, func(event eventlog.Event) error {
		var snapshot Alert
		if err := json.Unmarshal(event.Snapshot, &snapshot); err != nil {
			return err
		}
		if !snapshot.StartTime.Equal(start) {
			return nil
		}
		activation := result.LatestActivationAt
		if event.Type == eventlog.TypeFired || event.Type == eventlog.TypeRefired {
			activation = event.OccurredAt
		}
		result = NotificationOccurrenceLifecycle{State: NotificationOccurrenceNoRecord, LatestActivationAt: activation}
		if event.Type == eventlog.TypeResolved {
			result = NotificationOccurrenceLifecycle{State: NotificationOccurrenceResolved, ResolvedAt: event.OccurredAt, LatestActivationAt: activation}
		} else if event.Type == eventlog.TypeHistoryImported {
			// Imported history contains active snapshots too. Only an explicit
			// resolved lifecycle state supplies positive settlement authority.
			if snapshot.LatestTransition != nil && snapshot.LatestTransition.To == operationaltrust.OperationalResolved && !snapshot.LatestTransition.At.IsZero() {
				result.State, result.ResolvedAt = NotificationOccurrenceResolved, snapshot.LatestTransition.At
			} else if snapshot.OperationalRecord != nil && snapshot.OperationalRecord.State == operationaltrust.OperationalResolved {
				result.State, result.ResolvedAt = NotificationOccurrenceResolved, event.OccurredAt
			}
		}
		return nil
	})
	m.stopMu.RLock()
	stopping = m.stopping
	m.stopMu.RUnlock()
	if stopping {
		return NotificationOccurrenceLifecycle{State: NotificationOccurrenceUnavailable}
	}
	if err != nil || !store.Healthy() {
		return unknown
	}
	// A refire can publish live state while the bounded history walk runs.
	// Positive active authority overrides the older resolved snapshot.
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, alert := range m.activeAlerts {
		if alert != nil && (alert.ID == id || exportedAlertID(alert, alert.ID) == id) && alert.StartTime.Equal(start) {
			return NotificationOccurrenceLifecycle{State: NotificationOccurrenceActive}
		}
	}
	result.CurrentInactive = true
	return result
}
