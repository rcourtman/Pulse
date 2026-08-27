// Package eventlog is the append-only alert event log: every alert lifecycle
// transition and notification decision — including suppressions, with the
// mechanism that held them — is recorded as one immutable event. It exists so
// "why didn't I get notified?" is answerable from durable data instead of
// being reconstructed from logs (docs/ALERT_ENGINE_EVOLUTION.md, Phase 0).
//
// Delivery-diagnostic appends are additive to the live alert manager: they do
// not block evaluation and may be dropped under sustained pressure. Lifecycle
// events use AppendDurable because alert history is projected from this store.
package eventlog

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
)

// Event types recorded by the alert manager.
const (
	// TypeFired marks a new occurrence activating; TypeRefired marks a
	// reactivation that resumed a recently resolved occurrence. Both come
	// from the reducer core's explicit activation events, so persisted
	// restores are never misreported as firings.
	TypeFired                  = "fired"
	TypeRefired                = "refired"
	TypeResolved               = "resolved"
	TypeAcknowledged           = "acknowledged"
	TypeUnacknowledged         = "unacknowledged"
	TypeEscalated              = "escalated"
	TypeFlappingDetected       = "flapping_detected"
	TypeNotificationDispatched = "notification_dispatched"
	TypeNotificationDeferred   = "notification_deferred"
	TypeNotificationSuppressed = "notification_suppressed"
	// TypeShadowDivergence records the shadow reducer disagreeing with the
	// live manager on an alert's state — the always-on parity signal
	// (docs/ALERT_ENGINE_EVOLUTION.md, Phase 1).
	TypeShadowDivergence = "shadow_divergence"
	// TypeHistoryImported carries one legacy JSON-history entry migrated
	// into the log when the log becomes the history authority. Its
	// snapshot is the entry's final state.
	TypeHistoryImported = "history_imported"
	// TypeHistoryCleared is the user's clear-history action. The log stays
	// append-only; the history projection ignores lifecycle events that
	// precede the newest tombstone.
	TypeHistoryCleared = "history_cleared"
)

// Event is one immutable alert event.
type Event struct {
	ID           int64             `json:"id"`
	OccurredAt   time.Time         `json:"occurredAt"`
	Type         string            `json:"type"`
	AlertID      string            `json:"alertId"`
	ResourceID   string            `json:"resourceId,omitempty"`
	ResourceName string            `json:"resourceName,omitempty"`
	AlertType    string            `json:"alertType,omitempty"`
	Level        string            `json:"level,omitempty"`
	Reason       string            `json:"reason,omitempty"`
	Message      string            `json:"message,omitempty"`
	Details      map[string]string `json:"details,omitempty"`
	// Snapshot is the full alert state at the moment of the event, recorded
	// for lifecycle transitions (fired, refired, resolved, acknowledged,
	// unacknowledged) so alert history can be projected from the log alone.
	Snapshot json.RawMessage `json:"snapshot,omitempty"`
}

// Filter narrows a Query. Zero values mean "no constraint".
type Filter struct {
	AlertID string
	Types   []string
	Since   time.Time
	Until   time.Time
	Limit   int
}

const (
	defaultQueryLimit = 200
	maxQueryLimit     = 1000
	appendBufferSize  = 1024
	// defaultRetention keeps events long enough to explain a past incident
	// without growing without bound.
	defaultRetention = 90 * 24 * time.Hour
	pruneInterval    = time.Hour
	eventLogFileName = "events.db"
)

// Store is the SQLite-backed event log. A nil *Store is valid: Append and
// Close are no-ops and Query returns no events.
type Store struct {
	db        *sql.DB
	dbPath    string
	events    chan Event
	stop      chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once
	dropped   atomic.Int64
	appended  atomic.Int64
	written   atomic.Int64
	failed    atomic.Int64
	failureMu sync.RWMutex
	lastError error
	retention time.Duration
}

func sqliteDSN(path string) string {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + url.Values{
		"_pragma": []string{
			"busy_timeout(30000)",
			"journal_mode(WAL)",
			"synchronous(NORMAL)",
			"cache_size(-16000)",
		},
	}.Encode()
}

// Open creates or opens the event log inside dir (the manager's alerts data
// directory). The directory must already exist.
func Open(dir string) (*Store, error) {
	trimmed := strings.TrimSpace(dir)
	if trimmed == "" {
		return nil, fmt.Errorf("event log directory is required")
	}
	if info, err := os.Stat(trimmed); err != nil {
		return nil, fmt.Errorf("event log directory: %w", err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("event log directory %q is not a directory", trimmed)
	}
	dbPath := trimmed + string(os.PathSeparator) + eventLogFileName
	return openDSN(dbPath, sqliteDSN(dbPath))
}

// OpenInMemory creates a non-persistent store with identical behavior, for
// tests and as a fallback owner when on-disk bootstrap fails.
func OpenInMemory() (*Store, error) {
	return openDSN(":memory:", sqliteDSN("file::memory:?mode=memory"))
}

func openDSN(dbPath, dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open alert event log: %w", err)
	}
	// Single connection: the write path is one goroutine, and WAL keeps the
	// rare reads from blocking behind it for long.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	s := &Store{
		db:        db,
		dbPath:    dbPath,
		events:    make(chan Event, appendBufferSize),
		stop:      make(chan struct{}),
		retention: defaultRetention,
	}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init alert event log schema: %w", err)
	}

	s.wg.Add(1)
	go s.writeLoop()

	log.Info().Str("dbPath", dbPath).Msg("alert event log initialized")
	return s, nil
}

func (s *Store) initSchema() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS alert_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			occurred_at TEXT NOT NULL,
			event_type TEXT NOT NULL,
			alert_id TEXT NOT NULL,
			resource_id TEXT NOT NULL DEFAULT '',
			resource_name TEXT NOT NULL DEFAULT '',
			alert_type TEXT NOT NULL DEFAULT '',
			level TEXT NOT NULL DEFAULT '',
			reason TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL DEFAULT '',
			details TEXT NOT NULL DEFAULT '',
			snapshot TEXT NOT NULL DEFAULT ''
		);
		CREATE INDEX IF NOT EXISTS idx_alert_events_alert ON alert_events(alert_id, occurred_at);
		CREATE INDEX IF NOT EXISTS idx_alert_events_time ON alert_events(occurred_at);
		CREATE INDEX IF NOT EXISTS idx_alert_events_type ON alert_events(event_type, occurred_at);
	`)
	if err != nil {
		return err
	}
	return s.ensureSnapshotColumn()
}

// ensureSnapshotColumn upgrades a pre-snapshot database in place. ALTER TABLE
// ADD COLUMN on SQLite is metadata-only, so the upgrade is cheap and existing
// rows read back with an empty snapshot.
func (s *Store) ensureSnapshotColumn() error {
	rows, err := s.db.Query(`PRAGMA table_info(alert_events)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return err
		}
		if name == "snapshot" {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE alert_events ADD COLUMN snapshot TEXT NOT NULL DEFAULT ''`)
	return err
}

// Append records one event without blocking: when the buffer is full the
// event is dropped and counted, never stalling alert evaluation. Safe to
// call on a nil store and while holding manager locks.
func (s *Store) Append(event Event) {
	if s == nil {
		return
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now()
	}
	select {
	case s.events <- event:
		s.appended.Add(1)
	default:
		s.dropped.Add(1)
	}
}

// AppendDurable commits one lifecycle event before returning. Lifecycle
// volume is low, and this explicit durability boundary prevents alert history
// from silently losing state when the diagnostic append buffer is saturated.
func (s *Store) AppendDurable(event Event) error {
	if s == nil {
		return fmt.Errorf("event log is not enabled")
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now()
	}
	if err := s.insertBatch([]Event{event}); err != nil {
		s.recordWriteFailure(err)
		return err
	}
	return nil
}

// ImportEvents writes events synchronously, bypassing the droppable append
// buffer. It exists for the one-time legacy-history migration, where losing
// an entry to a full buffer would silently lose user data.
func (s *Store) ImportEvents(events []Event) error {
	if s == nil {
		return fmt.Errorf("event log is not enabled")
	}
	for i := range events {
		if events[i].OccurredAt.IsZero() {
			events[i].OccurredAt = time.Now()
		}
	}
	return s.insertBatch(events)
}

// Dropped reports how many events were discarded because the append buffer
// was full. A non-zero value means the log is incomplete for that window.
func (s *Store) Dropped() int64 {
	if s == nil {
		return 0
	}
	return s.dropped.Load()
}

func (s *Store) writeLoop() {
	defer s.wg.Done()
	pruneTicker := time.NewTicker(pruneInterval)
	defer pruneTicker.Stop()
	s.pruneOld()

	for {
		select {
		case event := <-s.events:
			batch := []Event{event}
		drain:
			for len(batch) < 128 {
				select {
				case next := <-s.events:
					batch = append(batch, next)
				default:
					break drain
				}
			}
			if err := s.insertBatch(batch); err != nil {
				s.recordWriteFailure(err)
				log.Error().Err(err).Int("events", len(batch)).Msg("alert event log write failed")
				continue
			}
			s.written.Add(int64(len(batch)))
		case <-pruneTicker.C:
			s.pruneOld()
		case <-s.stop:
			// Final drain so a clean shutdown does not lose buffered events.
			for {
				select {
				case event := <-s.events:
					if err := s.insertBatch([]Event{event}); err != nil {
						s.recordWriteFailure(err)
						log.Error().Err(err).Msg("alert event log final drain write failed")
						continue
					}
					s.written.Add(1)
				default:
					return
				}
			}
		}
	}
}

func (s *Store) recordWriteFailure(err error) {
	if s == nil || err == nil {
		return
	}
	s.failureMu.Lock()
	s.lastError = err
	s.failureMu.Unlock()
	s.failed.Add(1)
}

func (s *Store) writeError() error {
	if s == nil || s.failed.Load() == 0 {
		return nil
	}
	s.failureMu.RLock()
	err := s.lastError
	s.failureMu.RUnlock()
	if err == nil {
		return fmt.Errorf("alert event log write failed")
	}
	return fmt.Errorf("alert event log write failed: %w", err)
}

func (s *Store) insertBatch(batch []Event) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO alert_events
			(occurred_at, event_type, alert_id, resource_id, resource_name, alert_type, level, reason, message, details, snapshot)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	var importStmt *sql.Stmt
	for _, event := range batch {
		if event.Type != TypeHistoryImported {
			continue
		}
		importStmt, err = tx.Prepare(`
			INSERT INTO alert_events
				(occurred_at, event_type, alert_id, resource_id, resource_name, alert_type, level, reason, message, details, snapshot)
			SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
			WHERE NOT EXISTS (
				SELECT 1 FROM alert_events
				WHERE event_type = ? AND alert_id = ? AND occurred_at = ? AND snapshot = ?
			)
		`)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		defer importStmt.Close()
		break
	}
	for _, event := range batch {
		details := ""
		if len(event.Details) > 0 {
			encoded, err := json.Marshal(event.Details)
			if err == nil {
				details = string(encoded)
			}
		}
		occurredAt := event.OccurredAt.UTC().Format(time.RFC3339Nano)
		snapshot := string(event.Snapshot)
		args := []any{
			occurredAt,
			event.Type,
			event.AlertID,
			event.ResourceID,
			event.ResourceName,
			event.AlertType,
			event.Level,
			event.Reason,
			event.Message,
			details,
			snapshot,
		}
		var insertErr error
		if event.Type == TypeHistoryImported {
			_, insertErr = importStmt.Exec(append(args,
				TypeHistoryImported,
				event.AlertID,
				occurredAt,
				snapshot,
			)...)
		} else {
			_, insertErr = stmt.Exec(args...)
		}
		if insertErr != nil {
			_ = tx.Rollback()
			return insertErr
		}
	}
	return tx.Commit()
}

func (s *Store) pruneOld() {
	if s.retention <= 0 {
		return
	}
	cutoff := time.Now().Add(-s.retention).UTC().Format(time.RFC3339Nano)
	if _, err := s.db.Exec(`DELETE FROM alert_events WHERE occurred_at < ?`, cutoff); err != nil {
		log.Error().Err(err).Msg("alert event log prune failed")
	}
}

// Flush blocks until every diagnostic event appended before the call has been
// written. It reports failed writes and timeouts rather than claiming a lost
// batch was successfully flushed.
func (s *Store) Flush() error {
	if s == nil {
		return nil
	}
	target := s.appended.Load()
	deadline := time.Now().Add(5 * time.Second)
	for s.written.Load() < target && time.Now().Before(deadline) {
		if err := s.writeError(); err != nil {
			return err
		}
		time.Sleep(time.Millisecond)
	}
	if err := s.writeError(); err != nil {
		return err
	}
	if written := s.written.Load(); written < target {
		return fmt.Errorf("alert event log flush timed out: wrote %d of %d events", written, target)
	}
	return nil
}

// Query returns matching events, newest first.
func (s *Store) Query(filter Filter) ([]Event, error) {
	if s == nil {
		return nil, nil
	}

	where, args := eventFilterWhere(filter)

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultQueryLimit
	}
	if limit > maxQueryLimit {
		limit = maxQueryLimit
	}

	query := "SELECT id, occurred_at, event_type, alert_id, resource_id, resource_name, alert_type, level, reason, message, details, snapshot FROM alert_events"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY occurred_at DESC, id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query alert event log: %w", err)
	}
	defer rows.Close()

	// Do not use the request-derived limit as an allocation hint. The SQL
	// query is capped above, but keeping the result slice allocation independent
	// of caller input makes that memory-safety boundary explicit and prevents a
	// future query refactor from turning an oversized limit into an eager
	// allocation.
	return scanEventRows(rows, defaultQueryLimit)
}

// WalkOldest visits every matching event oldest first. It uses bounded keyset
// pages so a complete history projection is not truncated by Query's public
// safety cap and does not hold the store's single database connection for the
// full scan. A positive filter Limit caps the total visits; zero walks all
// matching rows.
func (s *Store) WalkOldest(filter Filter, visit func(Event) error) error {
	if s == nil {
		return nil
	}
	if visit == nil {
		return fmt.Errorf("event visitor is required")
	}

	baseWhere, baseArgs := eventFilterWhere(filter)
	visited := 0
	cursorOccurredAt := ""
	var cursorID int64

	for {
		pageLimit := maxQueryLimit
		if filter.Limit > 0 {
			remaining := filter.Limit - visited
			if remaining <= 0 {
				return nil
			}
			if remaining < pageLimit {
				pageLimit = remaining
			}
		}

		where := append([]string(nil), baseWhere...)
		args := append([]any(nil), baseArgs...)
		if cursorOccurredAt != "" {
			where = append(where, "(occurred_at > ? OR (occurred_at = ? AND id > ?))")
			args = append(args, cursorOccurredAt, cursorOccurredAt, cursorID)
		}

		query := "SELECT id, occurred_at, event_type, alert_id, resource_id, resource_name, alert_type, level, reason, message, details, snapshot FROM alert_events"
		if len(where) > 0 {
			query += " WHERE " + strings.Join(where, " AND ")
		}
		query += " ORDER BY occurred_at ASC, id ASC LIMIT ?"
		args = append(args, pageLimit)

		rows, err := s.db.Query(query, args...)
		if err != nil {
			return fmt.Errorf("walk alert event log: %w", err)
		}
		page, scanErr := scanEventRows(rows, pageLimit)
		closeErr := rows.Close()
		if scanErr != nil {
			return scanErr
		}
		if closeErr != nil {
			return fmt.Errorf("close alert event page: %w", closeErr)
		}

		for _, event := range page {
			if err := visit(event); err != nil {
				return err
			}
			visited++
		}
		if len(page) < pageLimit {
			return nil
		}

		last := page[len(page)-1]
		cursorOccurredAt = last.OccurredAt.UTC().Format(time.RFC3339Nano)
		cursorID = last.ID
	}
}

func eventFilterWhere(filter Filter) ([]string, []any) {
	where := make([]string, 0, 4)
	args := make([]any, 0, 6)
	if trimmed := strings.TrimSpace(filter.AlertID); trimmed != "" {
		where = append(where, "alert_id = ?")
		args = append(args, trimmed)
	}
	if len(filter.Types) > 0 {
		placeholders := make([]string, 0, len(filter.Types))
		for _, eventType := range filter.Types {
			trimmed := strings.TrimSpace(eventType)
			if trimmed == "" {
				continue
			}
			placeholders = append(placeholders, "?")
			args = append(args, trimmed)
		}
		if len(placeholders) > 0 {
			where = append(where, "event_type IN ("+strings.Join(placeholders, ",")+")")
		}
	}
	if !filter.Since.IsZero() {
		where = append(where, "occurred_at >= ?")
		args = append(args, filter.Since.UTC().Format(time.RFC3339Nano))
	}
	if !filter.Until.IsZero() {
		where = append(where, "occurred_at <= ?")
		args = append(args, filter.Until.UTC().Format(time.RFC3339Nano))
	}
	return where, args
}

func scanEventRows(rows *sql.Rows, capacity int) ([]Event, error) {
	if capacity < 0 || capacity > maxQueryLimit {
		capacity = maxQueryLimit
	}
	events := make([]Event, 0, capacity)
	for rows.Next() {
		var event Event
		var occurredAt, details, snapshot string
		if err := rows.Scan(
			&event.ID,
			&occurredAt,
			&event.Type,
			&event.AlertID,
			&event.ResourceID,
			&event.ResourceName,
			&event.AlertType,
			&event.Level,
			&event.Reason,
			&event.Message,
			&details,
			&snapshot,
		); err != nil {
			return nil, fmt.Errorf("scan alert event: %w", err)
		}
		if snapshot != "" {
			event.Snapshot = json.RawMessage(snapshot)
		}
		if parsed, err := time.Parse(time.RFC3339Nano, occurredAt); err == nil {
			event.OccurredAt = parsed
		}
		if details != "" {
			decoded := map[string]string{}
			if err := json.Unmarshal([]byte(details), &decoded); err == nil && len(decoded) > 0 {
				event.Details = decoded
			}
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// Close drains buffered events and closes the database.
func (s *Store) Close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		close(s.stop)
		s.wg.Wait()
		if err := s.db.Close(); err != nil {
			log.Error().Err(err).Msg("alert event log close failed")
		}
	})
}
