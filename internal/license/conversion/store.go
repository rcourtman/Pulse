package conversion

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type ConversionStore struct {
	db *sql.DB
}

type StoredConversionEvent struct {
	ID             int64
	OrgID          string
	EventType      string
	Surface        string
	Capability     string
	IdempotencyKey string
	CreatedAt      time.Time
}

type FunnelSummary struct {
	PaywallViewed     int64 `json:"paywall_viewed"`
	TrialStarted      int64 `json:"trial_started"`
	UpgradeClicked    int64 `json:"upgrade_clicked"`
	CheckoutCompleted int64 `json:"checkout_completed"`
	Period            struct {
		From time.Time `json:"from"`
		To   time.Time `json:"to"`
	} `json:"period"`
}

func NewConversionStore(dbPath string) (*ConversionStore, error) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		return nil, fmt.Errorf("conversion db path is required")
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create conversion db directory: %w", err)
	}

	dsn := dbPath + "?" + url.Values{
		"_pragma": []string{
			"busy_timeout(30000)",
			"journal_mode(WAL)",
			"synchronous(NORMAL)",
			"foreign_keys(ON)",
		},
	}.Encode()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open conversion db: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	store := &ConversionStore{db: db}
	if err := store.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *ConversionStore) ensureInitialized() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("conversion store is not initialized")
	}
	return nil
}

func formatTimeForDB(t time.Time) string {
	return t.UTC().Truncate(time.Second).Format(time.RFC3339)
}

func (s *ConversionStore) initSchema() error {
	if err := s.ensureInitialized(); err != nil {
		return err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS conversion_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		org_id TEXT NOT NULL,
		event_type TEXT NOT NULL,
		surface TEXT NOT NULL DEFAULT '',
		capability TEXT NOT NULL DEFAULT '',
		idempotency_key TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(idempotency_key)
	);
	CREATE INDEX IF NOT EXISTS idx_conversion_events_org_time ON conversion_events(org_id, created_at);
	CREATE INDEX IF NOT EXISTS idx_conversion_events_type ON conversion_events(event_type, created_at);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to initialize conversion schema: %w", err)
	}

	return nil
}

func (s *ConversionStore) Record(event StoredConversionEvent) error {
	if err := s.ensureInitialized(); err != nil {
		return err
	}

	orgID := strings.TrimSpace(event.OrgID)
	if orgID == "" {
		return fmt.Errorf("org_id is required")
	}
	eventType := strings.TrimSpace(event.EventType)
	if eventType == "" {
		return fmt.Errorf("event_type is required")
	}
	idempotencyKey := strings.TrimSpace(event.IdempotencyKey)
	if idempotencyKey == "" {
		return fmt.Errorf("idempotency_key is required")
	}

	createdAt := event.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	createdAtValue := formatTimeForDB(createdAt)

	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO conversion_events (org_id, event_type, surface, capability, idempotency_key, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		orgID,
		eventType,
		strings.TrimSpace(event.Surface),
		strings.TrimSpace(event.Capability),
		idempotencyKey,
		createdAtValue,
	)
	if err != nil {
		return fmt.Errorf("failed to insert conversion event: %w", err)
	}
	return nil
}

func (s *ConversionStore) Query(orgID string, from, to time.Time, eventType string) ([]StoredConversionEvent, error) {
	if err := s.ensureInitialized(); err != nil {
		return nil, err
	}

	where := make([]string, 0, 8)
	args := make([]any, 0, 8)

	orgID = strings.TrimSpace(orgID)
	if orgID != "" {
		where = append(where, "org_id = ?")
		args = append(args, orgID)
	}
	eventType = strings.TrimSpace(eventType)
	if eventType != "" {
		where = append(where, "event_type = ?")
		args = append(args, eventType)
	}
	if !from.IsZero() {
		where = append(where, "created_at >= ?")
		args = append(args, formatTimeForDB(from))
	}
	if !to.IsZero() {
		where = append(where, "created_at < ?")
		args = append(args, formatTimeForDB(to))
	}

	query := `
		SELECT
			id,
			org_id,
			event_type,
			surface,
			capability,
			idempotency_key,
			CAST(strftime('%s', created_at) AS INTEGER) AS created_at_unix
		FROM conversion_events
	`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY created_at ASC, id ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query conversion events: %w", err)
	}
	defer rows.Close()

	events := make([]StoredConversionEvent, 0)
	for rows.Next() {
		var ev StoredConversionEvent
		var createdAtUnix int64
		if err := rows.Scan(
			&ev.ID,
			&ev.OrgID,
			&ev.EventType,
			&ev.Surface,
			&ev.Capability,
			&ev.IdempotencyKey,
			&createdAtUnix,
		); err != nil {
			return nil, fmt.Errorf("failed to scan conversion event: %w", err)
		}
		ev.CreatedAt = time.Unix(createdAtUnix, 0).UTC()
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate conversion events: %w", err)
	}
	return events, nil
}

func (s *ConversionStore) FunnelSummary(orgID string, from, to time.Time) (*FunnelSummary, error) {
	if err := s.ensureInitialized(); err != nil {
		return nil, err
	}
	if from.IsZero() || to.IsZero() {
		return nil, fmt.Errorf("from/to are required")
	}

	orgID = strings.TrimSpace(orgID)
	where := []string{"created_at >= ?", "created_at < ?"}
	args := []any{
		formatTimeForDB(from),
		formatTimeForDB(to),
	}
	if orgID != "" {
		where = append(where, "org_id = ?")
		args = append(args, orgID)
	}

	query := `
		SELECT event_type, COUNT(1)
		FROM conversion_events
		WHERE ` + strings.Join(where, " AND ") + `
		GROUP BY event_type
	`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query funnel summary: %w", err)
	}
	defer rows.Close()

	summary := &FunnelSummary{}
	summary.Period.From = from.UTC()
	summary.Period.To = to.UTC()

	for rows.Next() {
		var eventType string
		var count int64
		if err := rows.Scan(&eventType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan funnel summary row: %w", err)
		}
		switch strings.TrimSpace(eventType) {
		case EventPaywallViewed:
			summary.PaywallViewed = count
		case EventTrialStarted:
			summary.TrialStarted = count
		case EventUpgradeClicked:
			summary.UpgradeClicked = count
		case EventCheckoutCompleted:
			summary.CheckoutCompleted = count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate funnel summary rows: %w", err)
	}
	return summary, nil
}

func (s *ConversionStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
