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
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

// newestHistoryEvidence keeps the newest evidence envelope, by ObservedAt and
// then position, in a fresh slice. A history entry carries at most this one:
// a resolved alert's full trail of up to maxOperationalEvidencePerAlert
// envelopes was most of each entry's size, and nothing reads it from history
// (#1803). The event log keeps every snapshot whole, and operationalRecord and
// transitions keep their full evidence ID lists.
func newestHistoryEvidence(evidence []operationaltrust.EvidenceEnvelope) []operationaltrust.EvidenceEnvelope {
	if len(evidence) == 0 {
		return nil
	}
	newest := 0
	for i := range evidence {
		if !evidence[i].ObservedAt.Before(evidence[newest].ObservedAt) {
			newest = i
		}
	}
	return []operationaltrust.EvidenceEnvelope{evidence[newest].Clone()}
}

// historyEventMark is what one folded lifecycle event contributed to its
// occurrence. The marks let a Since window be answered from the full fold
// instead of decoding every snapshot in the window again (#1803).
type historyEventMark struct {
	at       time.Time
	id       int64
	lastSeen time.Time // the snapshot's LastSeen, or the resolution time if later
}

// historyOccurrence accumulates one alert occurrence from its lifecycle
// events, oldest to newest.
type historyOccurrence struct {
	alert  Alert
	events []historyEventMark
	// lastAcked indexes the latest event whose snapshot was acknowledged, or
	// is -1. A window starting after it never saw the acknowledgement.
	lastAcked int
	// The latest snapshot's own acknowledgement fields, before merging carried
	// an earlier acknowledgement forward onto them.
	rawAcknowledged bool
	rawAckTime      *time.Time
	rawAckUser      string
}

func historyOccurrenceKey(alertID string, snapshot *Alert) string {
	return alertID + "\x00" + snapshot.StartTime.UTC().Format(time.RFC3339Nano)
}

// windowStart returns the index of the first event the log's Since filter
// (occurred_at >= since, compared as RFC3339Nano text) admits, or
// len(events) when none is. Events are folded in the log's (occurred_at,
// id) text order, so the comparison is monotonic.
func (occ *historyOccurrence) windowStart(sinceKey string) int {
	return sort.Search(len(occ.events), func(i int) bool {
		return occ.events[i].at.UTC().Format(time.RFC3339Nano) >= sinceKey
	})
}

// windowAck is the acknowledgement that folding events[start:] alone would
// produce: an acknowledgement is carried forward, so only one inside the
// window survives into it.
func (occ *historyOccurrence) windowAck(start int) (bool, *time.Time, string) {
	if occ.lastAcked >= start {
		return occ.alert.Acknowledged, occ.alert.AckTime, occ.alert.AckUser
	}
	return occ.rawAcknowledged, occ.rawAckTime, occ.rawAckUser
}

// windowLastSeen is the LastSeen that folding events[start:] alone would
// produce: the latest contribution inside the window.
func (occ *historyOccurrence) windowLastSeen(start int) time.Time {
	if start == 0 {
		return occ.alert.LastSeen
	}
	var latest time.Time
	for _, mark := range occ.events[start:] {
		if mark.lastSeen.After(latest) {
			latest = mark.lastSeen
		}
	}
	return latest
}

// historyRow is one candidate history entry. Rows reference the shared fold
// and are copied only when selected for output.
type historyRow struct {
	key        string
	occ        *historyOccurrence // nil for an active alert the log never saw fire
	start      int
	firstEvent time.Time
	overlay    *Alert // the live active alert shown instead of the folded state
}

func (row *historyRow) ack() (bool, *time.Time, string) {
	if row.overlay != nil {
		return row.overlay.Acknowledged, row.overlay.AckTime, row.overlay.AckUser
	}
	return row.occ.windowAck(row.start)
}

// materialize returns the row's entry. It shares the fold's maps and pointers;
// projectedHistory deep-copies every returned entry through
// canonicalizeAlertHistoryForOutput before releasing the projection lock, so
// that is the only copy a read makes.
func (row *historyRow) materialize() Alert {
	if row.overlay != nil {
		return *row.overlay
	}
	alert := row.occ.alert
	if row.start > 0 {
		alert.LastSeen = row.occ.windowLastSeen(row.start)
		alert.Acknowledged, alert.AckTime, alert.AckUser = row.occ.windowAck(row.start)
	}
	return alert
}

// AlertHistoryFromEvents projects the alert history list from the event
// log. It reports ok=false when no event log is enabled, letting callers
// fall back to the JSON-backed history manager.
func (m *Manager) AlertHistoryFromEvents(since time.Time, limit int) ([]Alert, bool) {
	store := m.eventLogStore()
	if store == nil || !m.eventHistoryAuthoritative.Load() {
		return nil, false
	}
	results, err := m.projectedHistory(store, since, limit)
	if err != nil {
		return nil, false
	}
	return m.applyCurrentNodeDisplayNames(results), true
}

// historyProjection is a disposable fold of the durable log, never a second
// source of truth. Its cursor is bounded by a committed event ID. Old imports,
// retention and store replacement rebuild it using the original chronological
// fold. A Since window is answered from this full-history fold with the same
// result as walking only the window's events (windowStart, windowAck and
// windowLastSeen), so windowed reads neither rebuild nor evict it.
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

func (p *historyProjection) fold(throughID int64) error {
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
		AfterID: p.boundary.LastID, ThroughID: throughID,
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
		// History keeps only the newest evidence envelope. Merging takes the
		// incoming snapshot's evidence, so trimming here is all the fold holds.
		snapshot.Evidence = newestHistoryEvidence(snapshot.Evidence)
		key := historyOccurrenceKey(event.AlertID, &snapshot)
		occ, exists := p.occurrences[key]
		if !exists {
			occ = &historyOccurrence{alert: snapshot, lastAcked: -1}
			p.occurrences[key] = occ
			p.order = append(p.order, key)
		} else {
			occ.alert = mergeHistoryAlertSnapshots(occ.alert, snapshot)
		}
		mark := historyEventMark{at: event.OccurredAt, id: event.ID, lastSeen: snapshot.LastSeen}
		if event.Type == eventlog.TypeResolved {
			// The JSON history's resolve path stamps the entry's LastSeen
			// with the resolution time so the row reflects the true
			// duration; mirror that.
			if event.OccurredAt.After(occ.alert.LastSeen) {
				occ.alert.LastSeen = event.OccurredAt
			}
			if event.OccurredAt.After(mark.lastSeen) {
				mark.lastSeen = event.OccurredAt
			}
		}
		occ.events = append(occ.events, mark)
		if snapshot.Acknowledged {
			occ.lastAcked = len(occ.events) - 1
		}
		occ.rawAcknowledged, occ.rawAckTime, occ.rawAckUser = snapshot.Acknowledged, snapshot.AckTime, snapshot.AckUser
		return nil
	})
}

// currentHistoryProjection folds the log through its current boundary and
// returns the shared projection. The caller must hold historyProjectionMu.
func (m *Manager) currentHistoryProjection(store *eventlog.Store) (*historyProjection, error) {
	for {
		boundary, err := store.ReplayBoundary()
		if err != nil {
			m.historyProjection = nil
			return nil, err
		}
		p := m.historyProjection
		if p == nil || p.store != store ||
			p.boundary.RetentionRevision != boundary.RetentionRevision || p.boundary.LastID > boundary.LastID {
			p = newHistoryProjection(store)
		}
		err = p.fold(boundary.LastID)
		if errors.Is(err, errHistoryEventBeforeCursor) {
			p = newHistoryProjection(store)
			err = p.fold(boundary.LastID)
		}
		if err != nil {
			m.historyProjection = nil
			return nil, err
		}
		after, err := store.ReplayBoundary()
		if err != nil {
			m.historyProjection = nil
			return nil, err
		}
		if after.RetentionRevision != boundary.RetentionRevision {
			m.historyProjection = nil
			continue
		}
		p.boundary = boundary
		m.historyProjection = p
		return p, nil
	}
}

// projectedHistory selects the history list, newest first, under the
// projection lock, and copies only the entries it returns. Callers may mutate
// the result without touching the shared fold or another reader's result.
func (m *Manager) projectedHistory(store *eventlog.Store, since time.Time, limit int) ([]Alert, error) {
	m.historyProjectionMu.Lock()
	defer m.historyProjectionMu.Unlock()
	p, err := m.currentHistoryProjection(store)
	if err != nil {
		return nil, err
	}

	sinceKey := ""
	if !since.IsZero() {
		sinceKey = since.UTC().Format(time.RFC3339Nano)
	}
	rows := make([]historyRow, 0, len(p.order))
	for _, key := range p.order {
		occ := p.occurrences[key]
		start := 0
		if sinceKey != "" {
			if start = occ.windowStart(sinceKey); start == len(occ.events) {
				continue
			}
		}
		rows = append(rows, historyRow{key: key, occ: occ, start: start, firstEvent: occ.events[start].at})
	}
	if sinceKey != "" {
		// Walking only the window appends each occurrence at its first event
		// inside the window, in the log's (occurred_at, id) order.
		keys := make([]string, len(rows))
		for i := range rows {
			keys[i] = rows[i].firstEvent.UTC().Format(time.RFC3339Nano)
		}
		order := make([]int, len(rows))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(a, b int) bool {
			ra, rb := order[a], order[b]
			if keys[ra] != keys[rb] {
				return keys[ra] < keys[rb]
			}
			return rows[ra].occ.events[rows[ra].start].id < rows[rb].occ.events[rows[rb].start].id
		})
		sorted := make([]historyRow, len(rows))
		for i, row := range order {
			sorted[i] = rows[row]
		}
		rows = sorted
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
	index := make(map[string]int, len(rows))
	for i := range rows {
		index[rows[i].key] = i
	}
	for _, alert := range active {
		key := historyOccurrenceKey(alert.ID, alert)
		if i, exists := index[key]; exists {
			acknowledged, ackTime, ackUser := rows[i].ack()
			overlay := *alert
			if acknowledged && !overlay.Acknowledged {
				overlay.Acknowledged, overlay.AckTime, overlay.AckUser = true, ackTime, ackUser
			}
			rows[i].overlay = &overlay
		} else if since.IsZero() || alert.StartTime.After(since) {
			overlay := *alert
			index[key] = len(rows)
			rows = append(rows, historyRow{key: key, firstEvent: alert.StartTime, overlay: &overlay})
		}
	}

	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].firstEvent.Before(rows[j].firstEvent)
	})

	size := len(rows)
	if limit > 0 && limit < size {
		size = limit
	}
	results := make([]Alert, 0, size)
	for i := len(rows) - 1; i >= 0 && len(results) < size; i-- {
		results = append(results, rows[i].materialize())
	}
	return canonicalizeAlertHistoryForOutput(results), nil
}
