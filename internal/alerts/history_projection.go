package alerts

// Alert history projected from the event log
// (docs/ALERT_ENGINE_EVOLUTION.md): lifecycle events carry full alert
// snapshots, so the history list — one row per alert occurrence, newest
// first — can be rebuilt from the append-only log instead of the
// separately maintained JSON snapshot file. While the JSON file remains
// authoritative, the parity suite in history_projection_parity_test.go
// holds this projection equal to it; the cutover retires the file.

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
)

// historyOccurrence accumulates one alert occurrence from its lifecycle
// events, oldest to newest.
type historyOccurrence struct {
	alert      Alert
	firstEvent time.Time
	lastEvent  time.Time
	resolved   bool
}

func historyOccurrenceKey(alertID string, snapshot *Alert) string {
	return alertID + "\x00" + snapshot.StartTime.UTC().Format(time.RFC3339Nano)
}

// AlertHistoryFromEvents projects the alert history list from the event
// log. It reports ok=false when no event log is enabled, letting callers
// fall back to the JSON-backed history manager.
func (m *Manager) AlertHistoryFromEvents(since time.Time, limit int) ([]Alert, bool) {
	store := m.eventLogStore()
	if store == nil {
		return nil, false
	}
	store.Flush()

	// Fold oldest to newest through the store's bounded-page walker. The
	// ordinary Query API intentionally caps responses at 1,000 rows, which is
	// appropriate for callers but not for reconstructing a complete history
	// window on a noisy installation.
	occurrences := make(map[string]*historyOccurrence)
	order := make([]string, 0)
	err := store.WalkOldest(eventlog.Filter{
		Types: []string{
			eventlog.TypeFired,
			eventlog.TypeRefired,
			eventlog.TypeResolved,
			eventlog.TypeAcknowledged,
			eventlog.TypeUnacknowledged,
			eventlog.TypeEscalated,
			eventlog.TypeHistoryImported,
			eventlog.TypeHistoryCleared,
		},
		Since: since,
	}, func(event eventlog.Event) error {
		if event.Type == eventlog.TypeHistoryCleared {
			// The user cleared history: everything before the tombstone
			// leaves the projection. The log itself stays append-only.
			occurrences = make(map[string]*historyOccurrence)
			order = order[:0]
			return nil
		}
		if len(event.Snapshot) == 0 {
			return nil
		}
		var snapshot Alert
		if err := json.Unmarshal(event.Snapshot, &snapshot); err != nil {
			return nil
		}
		key := historyOccurrenceKey(event.AlertID, &snapshot)
		occ, exists := occurrences[key]
		if !exists {
			occ = &historyOccurrence{alert: snapshot, firstEvent: event.OccurredAt}
			occurrences[key] = occ
			order = append(order, key)
		} else {
			occ.alert = mergeHistoryAlertSnapshots(occ.alert, snapshot)
		}
		occ.lastEvent = event.OccurredAt
		if event.Type == eventlog.TypeResolved {
			occ.resolved = true
			// The JSON history's resolve path stamps the entry's LastSeen
			// with the resolution time so the row reflects the true
			// duration; mirror that.
			if event.OccurredAt.After(occ.alert.LastSeen) {
				occ.alert.LastSeen = event.OccurredAt
			}
		} else {
			occ.resolved = false
		}
		return nil
	})
	if err != nil {
		return nil, false
	}

	// Overlay the live active alerts: an occurrence still firing shows its
	// current state (fresh LastSeen and value), exactly as the JSON
	// history's continuously updated entries do. Active alerts the log
	// never saw fire (log enabled mid-flight) are appended.
	m.mu.RLock()
	active := make([]*Alert, 0, len(m.activeAlerts))
	for storageKey, alert := range m.activeAlerts {
		if alert == nil {
			continue
		}
		exported := cloneAlertForOutput(alert)
		if exported == nil {
			continue
		}
		if exported.ID == "" {
			exported.ID = effectiveAlertID(alert, storageKey)
		}
		active = append(active, exported)
	}
	m.mu.RUnlock()
	for _, alert := range active {
		key := historyOccurrenceKey(alert.ID, alert)
		if occ, exists := occurrences[key]; exists {
			ack := occ.alert.Acknowledged
			ackTime, ackUser := occ.alert.AckTime, occ.alert.AckUser
			occ.alert = *alert
			if ack && !occ.alert.Acknowledged {
				occ.alert.Acknowledged = true
				occ.alert.AckTime = ackTime
				occ.alert.AckUser = ackUser
			}
			occ.resolved = false
		} else if since.IsZero() || alert.StartTime.After(since) {
			occurrences[key] = &historyOccurrence{
				alert:      *alert,
				firstEvent: alert.StartTime,
				lastEvent:  alert.LastSeen,
			}
			order = append(order, key)
		}
	}

	sort.SliceStable(order, func(i, j int) bool {
		return occurrences[order[i]].firstEvent.Before(occurrences[order[j]].firstEvent)
	})

	results := make([]Alert, 0, len(order))
	for i := len(order) - 1; i >= 0; i-- {
		if limit > 0 && len(results) >= limit {
			break
		}
		results = append(results, occurrences[order[i]].alert)
	}
	return m.applyCurrentNodeDisplayNames(canonicalizeAlertHistoryForOutput(results)), true
}
