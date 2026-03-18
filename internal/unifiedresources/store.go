package unifiedresources

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	_ "modernc.org/sqlite"
)

// ResourceStore persists identity overrides and manual links.
type ResourceStore interface {
	AddLink(link ResourceLink) error
	AddExclusion(exclusion ResourceExclusion) error
	GetLinks() ([]ResourceLink, error)
	GetExclusions() ([]ResourceExclusion, error)
	RecordChange(change ResourceChange) error
	GetRecentChanges(canonicalID string, since time.Time, limit int) ([]ResourceChange, error)
	RecordActionAudit(record ActionAuditRecord) error
	GetActionAudits(canonicalID string, since time.Time, limit int) ([]ActionAuditRecord, error)
	RecordActionLifecycleEvent(event ActionLifecycleEvent) error
	GetActionLifecycleEvents(actionID string, since time.Time, limit int) ([]ActionLifecycleEvent, error)
	RecordExportAudit(record ExportAuditRecord) error
	GetExportAudits(since time.Time, limit int) ([]ExportAuditRecord, error)
	Close() error
}

// ResourceLink represents a manual merge.
type ResourceLink struct {
	ResourceA string
	ResourceB string
	PrimaryID string
	Reason    string
	CreatedBy string
	CreatedAt time.Time
}

// ResourceExclusion represents a manual split.
type ResourceExclusion struct {
	ResourceA string
	ResourceB string
	Reason    string
	CreatedBy string
	CreatedAt time.Time
}

// SQLiteResourceStore stores overrides in SQLite.
type SQLiteResourceStore struct {
	db     *sql.DB
	dbPath string
	mu     sync.Mutex
}

const (
	maxOrgIDLength     = 64
	defaultOrgID       = "default"
	resourceDBFileName = "unified_resources.db"
)

var resourceStoreOrgIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// NewSQLiteResourceStore opens or creates the SQLite store.
func NewSQLiteResourceStore(dataDir, orgID string) (*SQLiteResourceStore, error) {
	if dataDir == "" {
		dataDir = utils.GetDataDir()
	}

	resolvedOrgID, err := normalizeOrgID(orgID)
	if err != nil {
		return nil, err
	}

	path := resourceDBPath(dataDir, resolvedOrgID)
	// Resource-store metadata can include user-authored links/exclusions; keep directory owner-only.
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("failed to create resources directory: %w", err)
	}

	// Backward compatibility: migrate non-default org stores from the legacy
	// shared-path naming scheme into tenant-scoped directories on first access.
	if resolvedOrgID != defaultOrgID {
		if err := migrateLegacyResourceStore(dataDir, resolvedOrgID, path); err != nil {
			return nil, err
		}
	}

	dsn := path + "?" + url.Values{
		"_pragma": []string{
			"busy_timeout(30000)",
			"journal_mode(WAL)",
			"synchronous(NORMAL)",
			"foreign_keys(ON)",
			"cache_size(-64000)",
		},
	}.Encode()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open resources db: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	store := &SQLiteResourceStore{db: db, dbPath: path}
	if err := store.initSchema(); err != nil {
		wrappedInitErr := fmt.Errorf("initialize resource store schema for %q: %w", path, err)
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(
				wrappedInitErr,
				fmt.Errorf("close resources db %q after init failure: %w", path, closeErr),
			)
		}
		return nil, wrappedInitErr
	}
	return store, nil
}

func normalizeOrgID(orgID string) (string, error) {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return defaultOrgID, nil
	}
	if orgID == defaultOrgID {
		return defaultOrgID, nil
	}
	if !isValidResourceStoreOrgID(orgID) {
		return "", fmt.Errorf("invalid organization ID: %s", orgID)
	}
	return orgID, nil
}

func isValidResourceStoreOrgID(orgID string) bool {
	if orgID == "" || orgID == "." || orgID == ".." {
		return false
	}
	if filepath.Base(orgID) != orgID {
		return false
	}
	return resourceStoreOrgIDPattern.MatchString(orgID)
}

func resourceDBPath(dataDir, orgID string) string {
	if orgID == defaultOrgID {
		return filepath.Join(dataDir, "resources", resourceDBFileName)
	}
	return filepath.Join(dataDir, "orgs", orgID, "resources", resourceDBFileName)
}

func migrateLegacyResourceStore(dataDir, orgID, newPath string) error {
	legacyPath := filepath.Join(dataDir, "resources", legacyResourceStoreFileName(orgID))
	if legacyPath == newPath {
		return nil
	}

	if err := copyFileIfMissing(legacyPath, newPath, 0600); err != nil {
		return fmt.Errorf("migrate legacy resource store %q -> %q: %w", legacyPath, newPath, err)
	}
	// Copy SQLite sidecar files when present to preserve uncheckpointed state.
	for _, suffix := range []string{"-wal", "-shm"} {
		legacyCompanion := legacyPath + suffix
		newCompanion := newPath + suffix
		if err := copyFileIfMissing(legacyCompanion, newCompanion, 0600); err != nil {
			return fmt.Errorf("migrate legacy resource store companion %q -> %q: %w", legacyCompanion, newCompanion, err)
		}
	}
	return nil
}

func copyFileIfMissing(src, dst string, perm os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}

	_, copyErr := io.Copy(dstFile, srcFile)
	closeErr := dstFile.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dst)
		return closeErr
	}
	return nil
}

func (s *SQLiteResourceStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS resource_identities (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		canonical_id TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		machine_id TEXT,
		dmi_uuid TEXT,
		primary_hostname TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_resource_identities_machine_id ON resource_identities(machine_id) WHERE machine_id IS NOT NULL;
	CREATE UNIQUE INDEX IF NOT EXISTS idx_resource_identities_dmi_uuid ON resource_identities(dmi_uuid) WHERE dmi_uuid IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_resource_identities_canonical ON resource_identities(canonical_id);
	CREATE INDEX IF NOT EXISTS idx_resource_identities_hostname ON resource_identities(primary_hostname);

	CREATE TABLE IF NOT EXISTS resource_hostnames (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		canonical_id TEXT NOT NULL,
		hostname TEXT NOT NULL,
		source TEXT NOT NULL,
		is_primary BOOLEAN DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(hostname, source)
	);
	CREATE INDEX IF NOT EXISTS idx_resource_hostnames_canonical ON resource_hostnames(canonical_id);
	CREATE INDEX IF NOT EXISTS idx_resource_hostnames_hostname ON resource_hostnames(hostname);

	CREATE TABLE IF NOT EXISTS resource_ips (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		canonical_id TEXT NOT NULL,
		ip_address TEXT NOT NULL,
		source TEXT NOT NULL,
		last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(ip_address, source)
	);
	CREATE INDEX IF NOT EXISTS idx_resource_ips_canonical ON resource_ips(canonical_id);
	CREATE INDEX IF NOT EXISTS idx_resource_ips_ip ON resource_ips(ip_address);

	CREATE TABLE IF NOT EXISTS resource_links (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		resource_a TEXT NOT NULL,
		resource_b TEXT NOT NULL,
		primary_id TEXT NOT NULL,
		reason TEXT,
		created_by TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(resource_a, resource_b)
	);
	CREATE INDEX IF NOT EXISTS idx_resource_links_primary ON resource_links(primary_id);

	CREATE TABLE IF NOT EXISTS resource_exclusions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		resource_a TEXT NOT NULL,
		resource_b TEXT NOT NULL,
		reason TEXT,
		created_by TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(resource_a, resource_b)
	);
	CREATE INDEX IF NOT EXISTS idx_resource_exclusions_a ON resource_exclusions(resource_a);
	CREATE INDEX IF NOT EXISTS idx_resource_exclusions_b ON resource_exclusions(resource_b);

	CREATE TABLE IF NOT EXISTS resource_metadata (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		canonical_id TEXT NOT NULL UNIQUE,
		custom_url TEXT,
		custom_name TEXT,
		notes TEXT,
		tags TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_by TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_resource_metadata_canonical ON resource_metadata(canonical_id);

	CREATE TABLE IF NOT EXISTS resource_changes (
		id TEXT PRIMARY KEY,
		canonical_id TEXT NOT NULL,
		observed_at DATETIME NOT NULL,
		occurred_at DATETIME,
		kind TEXT NOT NULL,
		from_state TEXT,
		to_state TEXT,
		source_type TEXT NOT NULL,
		source_adapter TEXT,
		actor TEXT,
		confidence TEXT NOT NULL,
		reason TEXT,
		related_resources TEXT,
		metadata_json TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_resource_changes_canonical_time ON resource_changes(canonical_id, observed_at DESC);

	CREATE TABLE IF NOT EXISTS action_audits (
		id TEXT PRIMARY KEY,
		action_id TEXT NOT NULL,
		canonical_id TEXT NOT NULL,
		request_id TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		state TEXT NOT NULL,
		request_json TEXT NOT NULL,
		plan_json TEXT NOT NULL,
		approvals_json TEXT,
		result_json TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_action_audits_canonical_created ON action_audits(canonical_id, created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_action_audits_action_id ON action_audits(action_id);

	CREATE TABLE IF NOT EXISTS action_lifecycle_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		action_id TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		state TEXT NOT NULL,
		actor TEXT,
		message TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_action_lifecycle_events_action ON action_lifecycle_events(action_id, timestamp DESC);

	CREATE TABLE IF NOT EXISTS export_audits (
		id TEXT PRIMARY KEY,
		timestamp DATETIME NOT NULL,
		actor TEXT NOT NULL,
		envelope_hash TEXT NOT NULL,
		decision TEXT NOT NULL,
		destination TEXT NOT NULL,
		redactions_json TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_export_audits_timestamp ON export_audits(timestamp DESC);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to initialize resource store schema: %w", err)
	}
	return nil
}

func (s *SQLiteResourceStore) AddLink(link ResourceLink) error {
	if link.ResourceA == "" || link.ResourceB == "" {
		return fmt.Errorf("resource IDs required")
	}
	link.ResourceA = CanonicalResourceID(link.ResourceA)
	link.ResourceB = CanonicalResourceID(link.ResourceB)
	link.PrimaryID = CanonicalResourceID(link.PrimaryID)
	if link.PrimaryID == "" {
		link.PrimaryID = link.ResourceA
	}
	if link.CreatedAt.IsZero() {
		link.CreatedAt = time.Now().UTC()
	}

	a, b := normalizePair(link.ResourceA, link.ResourceB)
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO resource_links (resource_a, resource_b, primary_id, reason, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(resource_a, resource_b) DO UPDATE SET
			primary_id=excluded.primary_id,
			reason=excluded.reason,
			created_by=excluded.created_by,
			created_at=excluded.created_at
	`, a, b, link.PrimaryID, link.Reason, link.CreatedBy, link.CreatedAt)
	if err != nil {
		return fmt.Errorf("upsert resource link %q<->%q: %w", a, b, err)
	}
	return nil
}

func (s *SQLiteResourceStore) AddExclusion(exclusion ResourceExclusion) error {
	if exclusion.ResourceA == "" || exclusion.ResourceB == "" {
		return fmt.Errorf("resource IDs required")
	}
	exclusion.ResourceA = CanonicalResourceID(exclusion.ResourceA)
	exclusion.ResourceB = CanonicalResourceID(exclusion.ResourceB)
	if exclusion.CreatedAt.IsZero() {
		exclusion.CreatedAt = time.Now().UTC()
	}
	a, b := normalizePair(exclusion.ResourceA, exclusion.ResourceB)
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO resource_exclusions (resource_a, resource_b, reason, created_by, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(resource_a, resource_b) DO UPDATE SET
			reason=excluded.reason,
			created_by=excluded.created_by,
			created_at=excluded.created_at
	`, a, b, exclusion.Reason, exclusion.CreatedBy, exclusion.CreatedAt)
	if err != nil {
		return fmt.Errorf("upsert resource exclusion %q<->%q: %w", a, b, err)
	}
	return nil
}

func (s *SQLiteResourceStore) GetLinks() (links []ResourceLink, err error) {
	rows, err := s.db.Query(`SELECT resource_a, resource_b, primary_id, reason, created_by, created_at FROM resource_links`)
	if err != nil {
		return nil, fmt.Errorf("query resource links: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			wrappedCloseErr := fmt.Errorf("close resource links rows: %w", closeErr)
			if err != nil {
				err = errors.Join(err, wrappedCloseErr)
				return
			}
			err = wrappedCloseErr
		}
	}()

	for rows.Next() {
		var link ResourceLink
		if scanErr := rows.Scan(&link.ResourceA, &link.ResourceB, &link.PrimaryID, &link.Reason, &link.CreatedBy, &link.CreatedAt); scanErr != nil {
			return nil, fmt.Errorf("scan resource link row: %w", scanErr)
		}
		link.ResourceA = CanonicalResourceID(link.ResourceA)
		link.ResourceB = CanonicalResourceID(link.ResourceB)
		link.PrimaryID = CanonicalResourceID(link.PrimaryID)
		links = append(links, link)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("iterate resource links rows: %w", rowsErr)
	}
	return links, nil
}

func (s *SQLiteResourceStore) GetExclusions() (exclusions []ResourceExclusion, err error) {
	rows, err := s.db.Query(`SELECT resource_a, resource_b, reason, created_by, created_at FROM resource_exclusions`)
	if err != nil {
		return nil, fmt.Errorf("query resource exclusions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			wrappedCloseErr := fmt.Errorf("close resource exclusions rows: %w", closeErr)
			if err != nil {
				err = errors.Join(err, wrappedCloseErr)
				return
			}
			err = wrappedCloseErr
		}
	}()

	for rows.Next() {
		var exclusion ResourceExclusion
		if scanErr := rows.Scan(&exclusion.ResourceA, &exclusion.ResourceB, &exclusion.Reason, &exclusion.CreatedBy, &exclusion.CreatedAt); scanErr != nil {
			return nil, fmt.Errorf("scan resource exclusion row: %w", scanErr)
		}
		exclusion.ResourceA = CanonicalResourceID(exclusion.ResourceA)
		exclusion.ResourceB = CanonicalResourceID(exclusion.ResourceB)
		exclusions = append(exclusions, exclusion)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("iterate resource exclusions rows: %w", rowsErr)
	}
	return exclusions, nil
}

func (s *SQLiteResourceStore) Close() error {
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return fmt.Errorf("close resources db %q: %w", s.dbPath, err)
		}
	}
	return nil
}

func (s *SQLiteResourceStore) RecordChange(change ResourceChange) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	relJSON, _ := json.Marshal(change.RelatedResources)
	metaJSON, _ := json.Marshal(change.Metadata)

	_, err := s.db.Exec(`
		INSERT INTO resource_changes (id, canonical_id, observed_at, occurred_at, kind, from_state, to_state, source_type, source_adapter, actor, confidence, reason, related_resources, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, change.ID, CanonicalResourceID(change.ResourceID), change.ObservedAt, change.OccurredAt, string(change.Kind), change.From, change.To, change.SourceType, change.SourceAdapter, change.Actor, string(change.Confidence), change.Reason, string(relJSON), string(metaJSON))
	if err != nil {
		return fmt.Errorf("insert resource change: %w", err)
	}
	return nil
}

func (s *SQLiteResourceStore) GetRecentChanges(canonicalID string, since time.Time, limit int) ([]ResourceChange, error) {
	query := `
		SELECT id, canonical_id, observed_at, occurred_at, kind, from_state, to_state, source_type, source_adapter, actor, confidence, reason, related_resources, metadata_json 
		FROM resource_changes`

	args := []any{}
	canonicalID = CanonicalResourceID(canonicalID)
	if canonicalID != "" {
		query += `
		WHERE canonical_id = ? AND observed_at >= ?`
		args = append(args, canonicalID, since)
	} else {
		query += `
		WHERE observed_at >= ?`
		args = append(args, since)
	}
	query += `
		ORDER BY observed_at DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query resource changes: %w", err)
	}
	defer rows.Close()

	var changes []ResourceChange
	for rows.Next() {
		var c ResourceChange
		var conf, kindStr string
		var relText, metaText sql.NullString

		if err := rows.Scan(&c.ID, &c.ResourceID, &c.ObservedAt, &c.OccurredAt, &kindStr, &c.From, &c.To, &c.SourceType, &c.SourceAdapter, &c.Actor, &conf, &c.Reason, &relText, &metaText); err != nil {
			return nil, fmt.Errorf("scan resource change row: %w", err)
		}
		c.ResourceID = CanonicalResourceID(c.ResourceID)

		c.Kind = ChangeKind(kindStr)
		c.Confidence = ChangeConfidence(conf)

		if relText.Valid && relText.String != "" {
			_ = json.Unmarshal([]byte(relText.String), &c.RelatedResources)
		}
		if metaText.Valid && metaText.String != "" {
			_ = json.Unmarshal([]byte(metaText.String), &c.Metadata)
		}

		changes = append(changes, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate resource change rows: %w", err)
	}
	return changes, nil
}

func (s *SQLiteResourceStore) RecordActionAudit(record ActionAuditRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	requestJSON, err := json.Marshal(record.Request)
	if err != nil {
		return fmt.Errorf("marshal action request: %w", err)
	}
	planJSON, err := json.Marshal(record.Plan)
	if err != nil {
		return fmt.Errorf("marshal action plan: %w", err)
	}
	approvalsJSON, err := json.Marshal(record.Approvals)
	if err != nil {
		return fmt.Errorf("marshal action approvals: %w", err)
	}
	resultJSON, err := json.Marshal(record.Result)
	if err != nil {
		return fmt.Errorf("marshal action result: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO action_audits (id, action_id, canonical_id, request_id, created_at, updated_at, state, request_json, plan_json, approvals_json, result_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			action_id=excluded.action_id,
			canonical_id=excluded.canonical_id,
			request_id=excluded.request_id,
			created_at=excluded.created_at,
			updated_at=excluded.updated_at,
			state=excluded.state,
			request_json=excluded.request_json,
			plan_json=excluded.plan_json,
			approvals_json=excluded.approvals_json,
			result_json=excluded.result_json
	`, record.ID, record.ID, CanonicalResourceID(record.Request.ResourceID), record.Request.RequestID, record.CreatedAt, record.UpdatedAt, string(record.State), string(requestJSON), string(planJSON), string(approvalsJSON), string(resultJSON))
	if err != nil {
		return fmt.Errorf("insert action audit: %w", err)
	}
	return nil
}

func (s *SQLiteResourceStore) GetActionAudits(canonicalID string, since time.Time, limit int) ([]ActionAuditRecord, error) {
	query := `
		SELECT id, action_id, request_id, created_at, updated_at, state, request_json, plan_json, approvals_json, result_json
		FROM action_audits`

	args := []any{}
	canonicalID = CanonicalResourceID(canonicalID)
	if canonicalID != "" {
		query += `
		WHERE canonical_id = ? AND created_at >= ?`
		args = append(args, canonicalID, since)
	} else {
		query += `
		WHERE created_at >= ?`
		args = append(args, since)
	}
	query += `
		ORDER BY created_at DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query action audits: %w", err)
	}
	defer rows.Close()

	var records []ActionAuditRecord
	for rows.Next() {
		var record ActionAuditRecord
		var stateStr string
		var actionID, requestID string
		var requestJSON, planJSON, approvalsJSON, resultJSON sql.NullString
		if err := rows.Scan(&record.ID, &actionID, &requestID, &record.CreatedAt, &record.UpdatedAt, &stateStr, &requestJSON, &planJSON, &approvalsJSON, &resultJSON); err != nil {
			return nil, fmt.Errorf("scan action audit row: %w", err)
		}

		record.State = ActionState(stateStr)
		if requestJSON.Valid && requestJSON.String != "" {
			if err := json.Unmarshal([]byte(requestJSON.String), &record.Request); err != nil {
				return nil, fmt.Errorf("unmarshal action request: %w", err)
			}
		}
		if planJSON.Valid && planJSON.String != "" {
			if err := json.Unmarshal([]byte(planJSON.String), &record.Plan); err != nil {
				return nil, fmt.Errorf("unmarshal action plan: %w", err)
			}
		}
		if approvalsJSON.Valid && approvalsJSON.String != "" {
			if err := json.Unmarshal([]byte(approvalsJSON.String), &record.Approvals); err != nil {
				return nil, fmt.Errorf("unmarshal action approvals: %w", err)
			}
		}
		if resultJSON.Valid && resultJSON.String != "" && resultJSON.String != "null" {
			var result ExecutionResult
			if err := json.Unmarshal([]byte(resultJSON.String), &result); err != nil {
				return nil, fmt.Errorf("unmarshal action result: %w", err)
			}
			record.Result = &result
		}
		record.Request.RequestID = requestID
		record.Request.ResourceID = CanonicalResourceID(record.Request.ResourceID)
		_ = actionID
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate action audit rows: %w", err)
	}
	return records, nil
}

func (s *SQLiteResourceStore) RecordActionLifecycleEvent(event ActionLifecycleEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO action_lifecycle_events (action_id, timestamp, state, actor, message)
		VALUES (?, ?, ?, ?, ?)
	`, event.ActionID, event.Timestamp, string(event.State), event.Actor, event.Message)
	if err != nil {
		return fmt.Errorf("insert action lifecycle event: %w", err)
	}
	return nil
}

func (s *SQLiteResourceStore) GetActionLifecycleEvents(actionID string, since time.Time, limit int) ([]ActionLifecycleEvent, error) {
	query := `
		SELECT action_id, timestamp, state, actor, message
		FROM action_lifecycle_events`

	args := []any{}
	actionID = strings.TrimSpace(actionID)
	if actionID != "" {
		query += `
		WHERE action_id = ? AND timestamp >= ?`
		args = append(args, actionID, since)
	} else {
		query += `
		WHERE timestamp >= ?`
		args = append(args, since)
	}
	query += `
		ORDER BY timestamp DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query action lifecycle events: %w", err)
	}
	defer rows.Close()

	var events []ActionLifecycleEvent
	for rows.Next() {
		var event ActionLifecycleEvent
		var stateStr string
		if err := rows.Scan(&event.ActionID, &event.Timestamp, &stateStr, &event.Actor, &event.Message); err != nil {
			return nil, fmt.Errorf("scan action lifecycle row: %w", err)
		}
		event.State = ActionState(stateStr)
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate action lifecycle rows: %w", err)
	}
	return events, nil
}

func (s *SQLiteResourceStore) RecordExportAudit(record ExportAuditRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	redactionsJSON, err := json.Marshal(record.Redactions)
	if err != nil {
		return fmt.Errorf("marshal export redactions: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO export_audits (id, timestamp, actor, envelope_hash, decision, destination, redactions_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			timestamp=excluded.timestamp,
			actor=excluded.actor,
			envelope_hash=excluded.envelope_hash,
			decision=excluded.decision,
			destination=excluded.destination,
			redactions_json=excluded.redactions_json
	`, record.ID, record.Timestamp, record.Actor, record.EnvelopeHash, string(record.Decision), record.Destination, string(redactionsJSON))
	if err != nil {
		return fmt.Errorf("insert export audit: %w", err)
	}
	return nil
}

func (s *SQLiteResourceStore) GetExportAudits(since time.Time, limit int) ([]ExportAuditRecord, error) {
	query := `
		SELECT id, timestamp, actor, envelope_hash, decision, destination, redactions_json
		FROM export_audits
		WHERE timestamp >= ?
		ORDER BY timestamp DESC`

	args := []any{since}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query export audits: %w", err)
	}
	defer rows.Close()

	var records []ExportAuditRecord
	for rows.Next() {
		var record ExportAuditRecord
		var decisionStr string
		var redactionsJSON sql.NullString
		if err := rows.Scan(&record.ID, &record.Timestamp, &record.Actor, &record.EnvelopeHash, &decisionStr, &record.Destination, &redactionsJSON); err != nil {
			return nil, fmt.Errorf("scan export audit row: %w", err)
		}
		record.Decision = ExportDecision(decisionStr)
		if redactionsJSON.Valid && redactionsJSON.String != "" {
			if err := json.Unmarshal([]byte(redactionsJSON.String), &record.Redactions); err != nil {
				return nil, fmt.Errorf("unmarshal export redactions: %w", err)
			}
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate export audit rows: %w", err)
	}
	return records, nil
}

// MemoryStore is an in-memory implementation for tests.
type MemoryStore struct {
	mu                    sync.RWMutex
	links                 []ResourceLink
	exclusions            []ResourceExclusion
	changes               []ResourceChange
	actionAudits          []ActionAuditRecord
	actionLifecycleEvents []ActionLifecycleEvent
	exportAudits          []ExportAuditRecord
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (m *MemoryStore) AddLink(link ResourceLink) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	link.ResourceA = CanonicalResourceID(link.ResourceA)
	link.ResourceB = CanonicalResourceID(link.ResourceB)
	link.PrimaryID = CanonicalResourceID(link.PrimaryID)
	m.links = append(m.links, link)
	return nil
}

func (m *MemoryStore) AddExclusion(exclusion ResourceExclusion) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	exclusion.ResourceA = CanonicalResourceID(exclusion.ResourceA)
	exclusion.ResourceB = CanonicalResourceID(exclusion.ResourceB)
	m.exclusions = append(m.exclusions, exclusion)
	return nil
}

func (m *MemoryStore) GetLinks() ([]ResourceLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ResourceLink, len(m.links))
	copy(out, m.links)
	return out, nil
}

func (m *MemoryStore) GetExclusions() ([]ResourceExclusion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ResourceExclusion, len(m.exclusions))
	copy(out, m.exclusions)
	return out, nil
}

func (m *MemoryStore) Close() error {
	return nil
}

func (m *MemoryStore) RecordChange(change ResourceChange) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.changes = append(m.changes, change)
	return nil
}

func (m *MemoryStore) GetRecentChanges(canonicalID string, since time.Time, limit int) ([]ResourceChange, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	canonicalID = CanonicalResourceID(canonicalID)
	var out []ResourceChange
	for i := len(m.changes) - 1; i >= 0; i-- {
		change := m.changes[i]
		if canonicalID != "" && CanonicalResourceID(change.ResourceID) != canonicalID {
			continue
		}
		if !since.IsZero() && change.ObservedAt.Before(since) {
			continue
		}
		out = append(out, change)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *MemoryStore) RecordActionAudit(record ActionAuditRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.actionAudits {
		if m.actionAudits[i].ID == record.ID {
			m.actionAudits[i] = record
			return nil
		}
	}
	m.actionAudits = append(m.actionAudits, record)
	return nil
}

func (m *MemoryStore) GetActionAudits(canonicalID string, since time.Time, limit int) ([]ActionAuditRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	canonicalID = CanonicalResourceID(canonicalID)
	var out []ActionAuditRecord
	for i := len(m.actionAudits) - 1; i >= 0; i-- {
		record := m.actionAudits[i]
		if canonicalID != "" && CanonicalResourceID(record.Request.ResourceID) != canonicalID {
			continue
		}
		if !since.IsZero() && record.CreatedAt.Before(since) {
			continue
		}
		out = append(out, record)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *MemoryStore) RecordActionLifecycleEvent(event ActionLifecycleEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.actionLifecycleEvents = append(m.actionLifecycleEvents, event)
	return nil
}

func (m *MemoryStore) GetActionLifecycleEvents(actionID string, since time.Time, limit int) ([]ActionLifecycleEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []ActionLifecycleEvent
	for i := len(m.actionLifecycleEvents) - 1; i >= 0; i-- {
		event := m.actionLifecycleEvents[i]
		if actionID != "" && event.ActionID != actionID {
			continue
		}
		if !since.IsZero() && event.Timestamp.Before(since) {
			continue
		}
		out = append(out, event)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *MemoryStore) RecordExportAudit(record ExportAuditRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.exportAudits = append(m.exportAudits, record)
	return nil
}

func (m *MemoryStore) GetExportAudits(since time.Time, limit int) ([]ExportAuditRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []ExportAuditRecord
	for i := len(m.exportAudits) - 1; i >= 0; i-- {
		record := m.exportAudits[i]
		if !since.IsZero() && record.Timestamp.Before(since) {
			continue
		}
		out = append(out, record)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func normalizePair(a, b string) (string, string) {
	a = CanonicalResourceID(a)
	b = CanonicalResourceID(b)
	if a > b {
		return b, a
	}
	return a, b
}

func sanitizeOrgID(orgID string) string {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(orgID))
	lastWasUnderscore := false
	for _, r := range orgID {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' ||
			r == '_' {
			if b.Len() >= maxOrgIDLength {
				break
			}
			b.WriteRune(r)
			lastWasUnderscore = false
			continue
		}

		if !lastWasUnderscore {
			if b.Len() >= maxOrgIDLength {
				break
			}
			b.WriteByte('_')
			lastWasUnderscore = true
		}
	}

	sanitized := strings.Trim(b.String(), "_-")
	if len(sanitized) > maxOrgIDLength {
		sanitized = sanitized[:maxOrgIDLength]
	}
	return sanitized
}

func legacyResourceStoreFileName(orgID string) string {
	orgPart := sanitizeOrgID(orgID)
	if orgPart != "" && orgPart != defaultOrgID {
		return fmt.Sprintf("unified_resources_%s.db", orgPart)
	}
	return resourceDBFileName
}
