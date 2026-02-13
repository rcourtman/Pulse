package conversion

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	privateDirPerm  = 0o700
	privateFilePerm = 0o600
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

func ensureOwnerOnlyDir(dir string) error {
	if err := os.MkdirAll(dir, privateDirPerm); err != nil {
		return err
	}
	return os.Chmod(dir, privateDirPerm)
}

func rejectSymlinkOrNonRegular(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsafe sqlite path %q: symlink is not allowed", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsafe sqlite path %q: non-regular file is not allowed", path)
	}
	return nil
}

func hardenSQLiteFile(path string) error {
	if err := rejectSymlinkOrNonRegular(path); err != nil {
		return err
	}
	return os.Chmod(path, privateFilePerm)
}

func hardenSQLiteArtifacts(dbPath string) error {
	artifacts := []string{dbPath, dbPath + "-wal", dbPath + "-shm"}
	for _, path := range artifacts {
		if err := hardenSQLiteFile(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
	}
	return nil
}

func NewConversionStore(dbPath string) (*ConversionStore, error) {
	dbPath = filepath.Clean(strings.TrimSpace(dbPath))
	if dbPath == "" {
		return nil, fmt.Errorf("conversion db path is required")
	}

	dir := filepath.Dir(dbPath)
	if err := ensureOwnerOnlyDir(dir); err != nil {
		return nil, fmt.Errorf("failed to create conversion db directory: %w", err)
	}
	if err := rejectSymlinkOrNonRegular(dbPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
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
		initErr := fmt.Errorf("initialize conversion schema: %w", err)
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(initErr, fmt.Errorf("close conversion db after init failure: %w", closeErr))
		}
		return nil, initErr
	}
	if err := hardenSQLiteArtifacts(dbPath); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to secure conversion db files: %w", err)
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
	CREATE INDEX IF NOT EXISTS idx_conversion_events_time ON conversion_events(created_at);
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

func (s *ConversionStore) Query(orgID string, from, to time.Time, eventType string) (events []StoredConversionEvent, retErr error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("conversion store is not initialized")
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
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			wrappedCloseErr := fmt.Errorf("close conversion event rows: %w", closeErr)
			if retErr != nil {
				retErr = errors.Join(retErr, wrappedCloseErr)
				return
			}
			retErr = wrappedCloseErr
		}
	}()

	events = make([]StoredConversionEvent, 0)
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

func (s *ConversionStore) FunnelSummary(orgID string, from, to time.Time) (summary *FunnelSummary, retErr error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("conversion store is not initialized")
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
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			wrappedCloseErr := fmt.Errorf("close funnel summary rows: %w", closeErr)
			if retErr != nil {
				retErr = errors.Join(retErr, wrappedCloseErr)
				return
			}
			retErr = wrappedCloseErr
		}
	}()

	summary = &FunnelSummary{}
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
