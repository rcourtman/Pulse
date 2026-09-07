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
	"errors"
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
	if store == nil || !m.eventHistoryAuthoritative.Load() {
		return nil, false
	}

	occurrences, order, err := m.historyOccurrences(store, since)
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

// historyProjection is a disposable fold of the durable log, never a second
// source of truth. Its cursor is bounded by a committed event ID. Old imports,
// retention and store replacement rebuild it using the original chronological
// fold. A moving Since window keeps its original event-filter semantics and
// does not evict the full-history fold used by attention and normal polling.
type historyProjection struct {
	store       *eventlog.Store
	boundary    eventlog.ReplayBoundary
	lastTimeKey string
	occurrences map[string]*historyOccurrence
	order       []string
}

var errHistoryEventBeforeCursor = errors.New("new history event precedes the folded chronology")

func newHistoryProjection(store *eventlog.Store) *historyProjection {
	return &historyProjection{store: store, occurrences: make(map[string]*historyOccurrence)}
}

func (p *historyProjection) fold(since time.Time, throughID int64) error {
	if throughID == 0 || throughID == p.boundary.LastID {
		return nil
	}
	return p.store.WalkOldest(eventlog.Filter{
		Types: []string{
			eventlog.TypeFired, eventlog.TypeRefired, eventlog.TypeResolved,
			eventlog.TypeAcknowledged, eventlog.TypeUnacknowledged,
			eventlog.TypeSnoozed, eventlog.TypeUnsnoozed, eventlog.TypeEscalated,
			eventlog.TypeHistoryImported, eventlog.TypeHistoryCleared,
		},
		Since: since, AfterID: p.boundary.LastID, ThroughID: throughID,
	}, func(event eventlog.Event) error {
		// Match the log's (occurred_at, id) ordering. Equal timestamps are
		// safe because all appended IDs exceed the preceding boundary.
		timeKey := event.OccurredAt.UTC().Format(time.RFC3339Nano)
		if timeKey < p.lastTimeKey {
			return errHistoryEventBeforeCursor
		}
		p.lastTimeKey = timeKey
		if event.Type == eventlog.TypeHistoryCleared {
			// The user cleared history: everything before the tombstone
			// leaves the projection. The log itself stays append-only.
			p.occurrences = make(map[string]*historyOccurrence)
			p.order = p.order[:0]
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
		occ, exists := p.occurrences[key]
		if !exists {
			occ = &historyOccurrence{alert: snapshot, firstEvent: event.OccurredAt}
			p.occurrences[key] = occ
			p.order = append(p.order, key)
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
}

func (m *Manager) historyOccurrences(store *eventlog.Store, since time.Time) (map[string]*historyOccurrence, []string, error) {
	m.historyProjectionMu.Lock()
	defer m.historyProjectionMu.Unlock()
	for {
		boundary, err := store.ReplayBoundary()
		if err != nil {
			m.historyProjection = nil
			return nil, nil, err
		}
		p := m.historyProjection
		if !since.IsZero() || p == nil || p.store != store ||
			p.boundary.RetentionRevision != boundary.RetentionRevision || p.boundary.LastID > boundary.LastID {
			p = newHistoryProjection(store)
		}
		err = p.fold(since, boundary.LastID)
		if errors.Is(err, errHistoryEventBeforeCursor) {
			p = newHistoryProjection(store)
			err = p.fold(since, boundary.LastID)
		}
		if err != nil {
			m.historyProjection = nil
			return nil, nil, err
		}
		after, err := store.ReplayBoundary()
		if err != nil {
			m.historyProjection = nil
			return nil, nil, err
		}
		if after.RetentionRevision != boundary.RetentionRevision {
			m.historyProjection = nil
			continue
		}
		p.boundary = boundary
		if since.IsZero() {
			m.historyProjection = p
		}
		// The live-state overlay and callers may mutate their result. Never
		// let those changes contaminate the durable fold or another reader.
		occurrences := make(map[string]*historyOccurrence, len(p.occurrences))
		for key, occurrence := range p.occurrences {
			copy := *occurrence
			copy.alert = *occurrence.alert.Clone()
			occurrences[key] = &copy
		}
		return occurrences, append([]string(nil), p.order...), nil
	}
}
