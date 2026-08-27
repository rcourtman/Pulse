// Package eventlog is the append-only alert event log: every alert lifecycle
// transition and notification decision — including suppressions, with the
// mechanism that held them — is recorded as one immutable event. It exists so
// "why didn't I get notified?" is answerable from durable data instead of
// being reconstructed from logs (docs/ALERT_ENGINE_EVOLUTION.md, Phase 0).
//
// The store is additive to the live alert manager: appends never block the
// evaluation path (a full buffer drops the event and counts the drop), and a
// store that fails to open degrades to a nil store whose methods are no-ops.
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
			details TEXT NOT NULL DEFAULT ''
		);
		CREATE INDEX IF NOT EXISTS idx_alert_events_alert ON alert_events(alert_id, occurred_at);
		CREATE INDEX IF NOT EXISTS idx_alert_events_time ON alert_events(occurred_at);
		CREATE INDEX IF NOT EXISTS idx_alert_events_type ON alert_events(event_type, occurred_at);
	`)
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
				log.Error().Err(err).Int("events", len(batch)).Msg("alert event log write failed")
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
						log.Error().Err(err).Msg("alert event log final drain write failed")
					}
					s.written.Add(1)
				default:
					return
				}
			}
		}
	}
}

func (s *Store) insertBatch(batch []Event) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO alert_events
			(occurred_at, event_type, alert_id, resource_id, resource_name, alert_type, level, reason, message, details)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, event := range batch {
		details := ""
		if len(event.Details) > 0 {
			encoded, err := json.Marshal(event.Details)
			if err == nil {
				details = string(encoded)
			}
		}
		if _, err := stmt.Exec(
			event.OccurredAt.UTC().Format(time.RFC3339Nano),
			event.Type,
			event.AlertID,
			event.ResourceID,
			event.ResourceName,
			event.AlertType,
			event.Level,
			event.Reason,
			event.Message,
			details,
		); err != nil {
			_ = tx.Rollback()
			return err
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

// Flush blocks until every event appended before the call has been written
// (or a short deadline passes). It exists for tests and API reads that must
// observe just-appended events.
func (s *Store) Flush() {
	if s == nil {
		return
	}
	target := s.appended.Load()
	deadline := time.Now().Add(5 * time.Second)
	for s.written.Load() < target && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
}

// Query returns matching events, newest first.
func (s *Store) Query(filter Filter) ([]Event, error) {
	if s == nil {
		return nil, nil
	}

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

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultQueryLimit
	}
	if limit > maxQueryLimit {
		limit = maxQueryLimit
	}

	query := "SELECT id, occurred_at, event_type, alert_id, resource_id, resource_name, alert_type, level, reason, message, details FROM alert_events"
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
	events := make([]Event, 0, defaultQueryLimit)
	for rows.Next() {
		var event Event
		var occurredAt, details string
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
		); err != nil {
			return nil, fmt.Errorf("scan alert event: %w", err)
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
