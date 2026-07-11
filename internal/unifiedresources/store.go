package unifiedresources

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	// Identity pins keep canonical IDs for merged-source hosts stable across
	// restarts. See ResourceIdentityPin.
	UpsertResourceIdentityPins(pins []ResourceIdentityPin) error
	ListResourceIdentityPins() ([]ResourceIdentityPin, error)
	RecordChange(change ResourceChange) error
	GetRecentChanges(canonicalID string, since time.Time, limit int) ([]ResourceChange, error)
	GetRecentChangesFiltered(canonicalID string, since time.Time, limit int, filters ResourceChangeFilters) ([]ResourceChange, error)
	CountRecentChanges(canonicalID string, since time.Time) (int, error)
	CountRecentChangesFiltered(canonicalID string, since time.Time, filters ResourceChangeFilters) (int, error)
	CountRecentChangesByKind(canonicalID string, since time.Time) (map[ChangeKind]int, error)
	CountRecentChangesByKindFiltered(canonicalID string, since time.Time, filters ResourceChangeFilters) (map[ChangeKind]int, error)
	CountRecentChangesBySourceType(canonicalID string, since time.Time) (map[ChangeSourceType]int, error)
	CountRecentChangesBySourceTypeFiltered(canonicalID string, since time.Time, filters ResourceChangeFilters) (map[ChangeSourceType]int, error)
	CountRecentChangesBySourceAdapter(canonicalID string, since time.Time) (map[ChangeSourceAdapter]int, error)
	CountRecentChangesBySourceAdapterFiltered(canonicalID string, since time.Time, filters ResourceChangeFilters) (map[ChangeSourceAdapter]int, error)
	CreateActionAudit(record ActionAuditRecord, initialEvents []ActionLifecycleEvent) (ActionAuditRecord, bool, error)
	RecordActionAudit(record ActionAuditRecord) error
	GetActionAudit(actionID string) (ActionAuditRecord, bool, error)
	GetActionAudits(canonicalID string, since time.Time, limit int) ([]ActionAuditRecord, error)
	RecordActionDecision(record ActionAuditRecord, event ActionLifecycleEvent) error
	RecordActionExecutionStart(record ActionAuditRecord, event ActionLifecycleEvent) error
	RecordActionPolicyExecutionStart(record ActionAuditRecord, approvalEvent, executionEvent ActionLifecycleEvent) error
	RecordActionExecutionResult(record ActionAuditRecord, event ActionLifecycleEvent) error
	RecordActionExecutionRefusal(record ActionAuditRecord, event ActionLifecycleEvent) error
	RecordActionLifecycleEvent(event ActionLifecycleEvent) error
	GetActionLifecycleEvents(actionID string, since time.Time, limit int) ([]ActionLifecycleEvent, error)
	RecordExportAudit(record ExportAuditRecord) error
	GetExportAudits(since time.Time, limit int) ([]ExportAuditRecord, error)
	// Operator-set per-resource state. See ResourceOperatorState.
	GetResourceOperatorState(canonicalID string) (ResourceOperatorState, bool, error)
	SetResourceOperatorState(state ResourceOperatorState) error
	ClearResourceOperatorState(canonicalID string) error
	// ListResourceOperatorStates returns every persisted operator-set
	// state row. Used by background loops (e.g. the maintenance
	// verification sentinel) that need to sweep the full set on each
	// tick. Order is implementation-defined; the caller sorts if it
	// cares.
	ListResourceOperatorStates() ([]ResourceOperatorState, error)
	// Loop reports — durable summaries written by background loops
	// (currently only the maintenance-verification sentinel).
	RecordLoopReport(report LoopReport) error
	GetLoopReport(reportID string) (LoopReport, bool, error)
	ListLoopReportsForResource(reportType LoopReportType, canonicalID string, limit int) ([]LoopReport, error)
	UpdateLoopReportUserOutcome(reportID string, outcome LoopReportUserOutcome, reviewedBy, note string, reviewedAt time.Time) error
	// FindLoopReportByWindow looks up an existing report by the
	// (type, scope, window-end) triple so the sentinel can dedupe
	// on each tick without scanning the whole table.
	FindLoopReportByWindow(reportType LoopReportType, canonicalID string, windowEndedAt time.Time) (LoopReport, bool, error)
	Close() error
}

// ActionAuditOriginReader is the optional origin-indexed lookup used by
// proposing surfaces to repair missed transition callbacks at read time. It is
// separate from ResourceStore so external/test stores that do not support
// broker-origin recovery remain source-compatible.
type ActionAuditOriginReader interface {
	GetLatestActionAuditByOrigin(surface, investigationID string) (ActionAuditRecord, bool, error)
}

// PendingActionAuditReader owns the indexed operator queue for canonical
// actions awaiting a decision. Mobile and desktop clients must not reconstruct
// this queue from the retired command-approval store.
type PendingActionAuditReader interface {
	GetPendingActionAudits(limit int) ([]ActionAuditRecord, error)
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
	db                                     *sql.DB
	dbPath                                 string
	resourceChangesHasTimestamp            bool
	resourceChangesHasSource               bool
	resourceChangesObservedAtNeedsFallback bool
	mu                                     sync.Mutex

	// identityPinCache caches the (tiny) resource_identities table for
	// change-journal era expansion. Invalidated on pin upserts.
	identityPinMu    sync.Mutex
	identityPinCache []ResourceIdentityPin
	identityPinFresh bool

	retentionStop chan struct{}
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
			"auto_vacuum(INCREMENTAL)",
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
		if !isCorruptionError(err) {
			if closeErr := db.Close(); closeErr != nil {
				return nil, errors.Join(
					wrappedInitErr,
					fmt.Errorf("close resources db %q after init failure: %w", path, closeErr),
				)
			}
			return nil, wrappedInitErr
		}

		log.Printf("[WARN] unified_resources: database %q appears corrupted (%v); backing up and recreating", path, err)
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(
				wrappedInitErr,
				fmt.Errorf("close corrupted resources db %q: %w", path, closeErr),
			)
		}

		if backupErr := backupCorruptedDB(path); backupErr != nil {
			return nil, errors.Join(wrappedInitErr, fmt.Errorf("backup corrupted db %q: %w", path, backupErr))
		}

		db, err = sql.Open("sqlite", dsn)
		if err != nil {
			return nil, fmt.Errorf("reopen resources db after corruption recovery: %w", err)
		}
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		db.SetConnMaxLifetime(0)

		store = &SQLiteResourceStore{db: db, dbPath: path}
		if err := store.initSchema(); err != nil {
			if closeErr := db.Close(); closeErr != nil {
				return nil, errors.Join(
					fmt.Errorf("initialize resource store schema for %q after recovery: %w", path, err),
					fmt.Errorf("close resources db %q after post-recovery init failure: %w", path, closeErr),
				)
			}
			return nil, fmt.Errorf("initialize resource store schema for %q after recovery: %w", path, err)
		}
		log.Printf("[INFO] unified_resources: database %q recreated successfully after corruption recovery", path)
	}
	store.migrateAutoVacuum()
	store.retentionStop = store.startRetentionLoop()
	return store, nil
}

// isCorruptionError reports whether the init error indicates a corrupted
// database that warrants a delete-and-recreate recovery.
func isCorruptionError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "malformed") ||
		strings.Contains(msg, "database disk image") ||
		strings.Contains(msg, "file is not a database") ||
		strings.Contains(msg, "database is not a database")
}

// backupCorruptedDB renames the corrupted database and its sidecar files
// (WAL, SHM) to *.corrupted so a fresh database can be created in their place.
func backupCorruptedDB(path string) error {
	suffix := fmt.Sprintf(".corrupted.%d", time.Now().Unix())
	sidecars := []string{path, path + "-wal", path + "-shm"}
	for _, f := range sidecars {
		if _, err := os.Stat(f); err != nil {
			continue
		}
		if err := os.Rename(f, f+suffix); err != nil {
			return fmt.Errorf("rename %q: %w", f, err)
		}
	}
	return nil
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
		result_json TEXT,
		verification_outcome_json TEXT NOT NULL DEFAULT '',
		origin_json TEXT
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

	CREATE TABLE IF NOT EXISTS resource_operator_state (
		canonical_id TEXT PRIMARY KEY,
		intentionally_offline INTEGER NOT NULL DEFAULT 0,
		never_auto_remediate INTEGER NOT NULL DEFAULT 0,
		auto_remediation_policy_json TEXT,
		maintenance_start_at DATETIME,
		maintenance_end_at DATETIME,
		maintenance_reason TEXT,
		criticality TEXT,
		note TEXT,
		set_at DATETIME NOT NULL,
		set_by TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_resource_operator_state_maintenance ON resource_operator_state(maintenance_end_at);

	CREATE TABLE IF NOT EXISTS loop_reports (
		id TEXT PRIMARY KEY,
		report_type TEXT NOT NULL,
		scope TEXT NOT NULL,
		trigger TEXT NOT NULL,
		goal TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL,
		started_at DATETIME NOT NULL,
		completed_at DATETIME NOT NULL,
		window_started_at DATETIME,
		window_ended_at DATETIME,
		evidence_json TEXT NOT NULL DEFAULT '{}',
		linked_finding_ids_json TEXT NOT NULL DEFAULT '[]',
		linked_alert_ids_json TEXT NOT NULL DEFAULT '[]',
		linked_action_ids_json TEXT NOT NULL DEFAULT '[]',
		linked_patrol_run_id TEXT NOT NULL DEFAULT '',
		recommendation TEXT NOT NULL DEFAULT '',
		user_outcome TEXT NOT NULL DEFAULT '',
		reviewed_at DATETIME,
		reviewed_by TEXT NOT NULL DEFAULT '',
		review_note TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_loop_reports_scope_type_started
		ON loop_reports(report_type, scope, started_at DESC);
	-- Look-up index for FindLoopReportByWindow. Intentionally non-unique:
	-- rerun records share (type, scope, window_ended_at) with the
	-- original under a distinct id suffix. Tick-vs-tick dedup runs at
	-- the sentinel layer (mutex + FindLoopReportByWindow check).
	CREATE INDEX IF NOT EXISTS idx_loop_reports_window_lookup
		ON loop_reports(report_type, scope, window_ended_at)
		WHERE window_ended_at IS NOT NULL;
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to initialize resource store schema: %w", err)
	}
	if err := s.migrateResourceChangesSchema(); err != nil {
		return err
	}
	if err := s.ensureResourceChangesIndexes(); err != nil {
		return err
	}
	if err := s.migrateResourceIdentitiesSchema(); err != nil {
		return err
	}
	if err := s.migrateActionAuditsSchema(); err != nil {
		return err
	}
	if err := s.migrateActionLifecycleEventsSchema(); err != nil {
		return err
	}
	if err := s.migrateResourceOperatorStateSchema(); err != nil {
		return err
	}
	if err := s.migrateActionAuditRedaction(); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteResourceStore) migrateResourceOperatorStateSchema() error {
	columns, err := s.tableColumns("resource_operator_state")
	if err != nil {
		return err
	}
	if _, ok := columns["auto_remediation_policy_json"]; !ok {
		if _, err := s.db.Exec("ALTER TABLE resource_operator_state ADD COLUMN auto_remediation_policy_json TEXT"); err != nil {
			return fmt.Errorf("add resource_operator_state.auto_remediation_policy_json column: %w", err)
		}
	}
	return nil
}

// migrateResourceIdentitiesSchema upgrades the resource_identities table from
// its original schema-only shape (no code ever wrote it before identity pins)
// to the pin layout: a cluster_name column and a unique canonical_id index so
// pins upsert one row per canonical resource.
func (s *SQLiteResourceStore) migrateResourceIdentitiesSchema() error {
	columns, err := s.tableColumns("resource_identities")
	if err != nil {
		return err
	}
	if _, ok := columns["cluster_name"]; !ok {
		if _, err := s.db.Exec("ALTER TABLE resource_identities ADD COLUMN cluster_name TEXT"); err != nil {
			return fmt.Errorf("add resource_identities.cluster_name column: %w", err)
		}
	}
	// The table shipped empty for every deployment, but stay defensive: drop
	// duplicate canonical_id rows before enforcing uniqueness.
	if _, err := s.db.Exec(`DELETE FROM resource_identities WHERE id NOT IN (
		SELECT MAX(id) FROM resource_identities GROUP BY canonical_id
	)`); err != nil {
		return fmt.Errorf("dedupe resource_identities canonical rows: %w", err)
	}
	if _, err := s.db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_resource_identities_canonical_unique ON resource_identities(canonical_id)"); err != nil {
		return fmt.Errorf("ensure resource_identities canonical unique index: %w", err)
	}
	return nil
}

// migrateActionAuditsSchema adds the verification_outcome_json and
// origin_json columns to older action_audits tables so the
// VerificationOutcome and broker-owned Origin fields on ActionAuditRecord
// persist across restarts. Records written before the migration read back
// with the default unknown status / nil origin via the normalizer.
func (s *SQLiteResourceStore) migrateActionAuditsSchema() error {
	columns, err := s.tableColumns("action_audits")
	if err != nil {
		return err
	}
	if _, ok := columns["verification_outcome_json"]; !ok {
		if _, err := s.db.Exec("ALTER TABLE action_audits ADD COLUMN verification_outcome_json TEXT NOT NULL DEFAULT ''"); err != nil {
			return fmt.Errorf("add action_audits.verification_outcome_json column: %w", err)
		}
	}
	if _, ok := columns["origin_json"]; !ok {
		if _, err := s.db.Exec("ALTER TABLE action_audits ADD COLUMN origin_json TEXT"); err != nil {
			return fmt.Errorf("add action_audits.origin_json column: %w", err)
		}
	}
	if _, err := s.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_action_audits_origin_investigation_updated_v2
		ON action_audits(json_extract(origin_json, '$.surface'), json_extract(origin_json, '$.investigationId'), updated_at DESC)
		WHERE json_valid(origin_json)
	`); err != nil {
		return fmt.Errorf("create action audit origin investigation index: %w", err)
	}
	if _, err := s.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_action_audits_state_updated
		ON action_audits(state, updated_at ASC, created_at ASC)
	`); err != nil {
		return fmt.Errorf("create action audit state index: %w", err)
	}
	return nil
}

func (s *SQLiteResourceStore) migrateActionLifecycleEventsSchema() error {
	if _, err := s.db.Exec(`
		DELETE FROM action_lifecycle_events
		WHERE id NOT IN (
			SELECT MIN(id) FROM action_lifecycle_events GROUP BY action_id, state
		)
	`); err != nil {
		return fmt.Errorf("deduplicate action lifecycle state events: %w", err)
	}
	if _, err := s.db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_action_lifecycle_events_action_state_unique
		ON action_lifecycle_events(action_id, state)
	`); err != nil {
		return fmt.Errorf("create unique action lifecycle state index: %w", err)
	}
	return nil
}

func (s *SQLiteResourceStore) migrateActionAuditRedaction() error {
	rows, err := s.db.Query(`
		SELECT id, result_json
		FROM action_audits
		WHERE result_json IS NOT NULL AND result_json != '' AND result_json != 'null'
	`)
	if err != nil {
		return fmt.Errorf("query action audit redaction migration rows: %w", err)
	}
	defer rows.Close()

	type redactionUpdate struct {
		id         string
		resultJSON string
	}
	var updates []redactionUpdate
	for rows.Next() {
		var id, resultJSON string
		if err := rows.Scan(&id, &resultJSON); err != nil {
			return fmt.Errorf("scan action audit redaction migration row: %w", err)
		}
		var result ExecutionResult
		if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
			return fmt.Errorf("unmarshal action audit result for redaction migration %q: %w", id, err)
		}
		redactedResult := redactActionExecutionResult(&result)
		redactedJSON, err := json.Marshal(redactedResult)
		if err != nil {
			return fmt.Errorf("marshal action audit result for redaction migration %q: %w", id, err)
		}
		if string(redactedJSON) != resultJSON {
			updates = append(updates, redactionUpdate{id: id, resultJSON: string(redactedJSON)})
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate action audit redaction migration rows: %w", err)
	}
	if len(updates) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin action audit redaction migration: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	for _, update := range updates {
		if _, err := tx.Exec(`
			UPDATE action_audits
			SET result_json = ?
			WHERE id = ?
		`, update.resultJSON, update.id); err != nil {
			return fmt.Errorf("update action audit result redaction %q: %w", update.id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit action audit redaction migration: %w", err)
	}
	committed = true
	return nil
}

func (s *SQLiteResourceStore) migrateResourceChangesSchema() error {
	columns, err := s.tableColumns("resource_changes")
	if err != nil {
		return err
	}
	_, observedAtExisted := columns["observed_at"]
	if _, ok := columns["timestamp"]; ok {
		s.resourceChangesHasTimestamp = true
	}
	if _, ok := columns["source"]; ok {
		s.resourceChangesHasSource = true
	}
	if !observedAtExisted {
		s.resourceChangesObservedAtNeedsFallback = true
	} else {
		observedAtNotNull, err := s.tableColumnNotNull("resource_changes", "observed_at")
		if err != nil {
			return err
		}
		s.resourceChangesObservedAtNeedsFallback = !observedAtNotNull
	}

	if err := s.addResourceChangesColumnIfMissing(columns, "observed_at", "DATETIME"); err != nil {
		return err
	}
	if err := s.addResourceChangesColumnIfMissing(columns, "occurred_at", "DATETIME"); err != nil {
		return err
	}
	if err := s.addResourceChangesColumnIfMissing(columns, "source_type", "TEXT NOT NULL DEFAULT 'pulse_diff'"); err != nil {
		return err
	}
	if err := s.addResourceChangesColumnIfMissing(columns, "source_adapter", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.addResourceChangesColumnIfMissing(columns, "actor", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.addResourceChangesColumnIfMissing(columns, "related_resources", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	if err := s.addResourceChangesColumnIfMissing(columns, "metadata_json", "TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}

	if err := s.backfillLegacyResourceChangeObservedAt(columns); err != nil {
		return err
	}
	if err := s.normalizeResourceChangeRows(columns); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteResourceStore) ensureResourceChangesIndexes() error {
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_resource_changes_canonical_time ON resource_changes(canonical_id, observed_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_resource_changes_kind_time ON resource_changes(kind, observed_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_resource_changes_source_type_time ON resource_changes(source_type, observed_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_resource_changes_source_adapter_time ON resource_changes(source_adapter, observed_at DESC)`,
	}
	for _, stmt := range indexes {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("ensure resource_changes index: %w", err)
		}
	}
	return nil
}

func (s *SQLiteResourceStore) addResourceChangesColumnIfMissing(columns map[string]struct{}, columnName, definition string) error {
	if _, ok := columns[columnName]; ok {
		return nil
	}
	if _, err := s.db.Exec("ALTER TABLE resource_changes ADD COLUMN " + columnName + " " + definition); err != nil {
		return fmt.Errorf("add resource_changes.%s column: %w", columnName, err)
	}
	columns[columnName] = struct{}{}
	return nil
}

func (s *SQLiteResourceStore) tableColumns(tableName string) (map[string]struct{}, error) {
	rows, err := s.db.Query(`PRAGMA table_info(` + tableName + `)`)
	if err != nil {
		return nil, fmt.Errorf("inspect %s schema: %w", tableName, err)
	}
	defer rows.Close()

	columns := make(map[string]struct{})
	for rows.Next() {
		var (
			cid     int
			name    string
			typ     string
			notNull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			return nil, fmt.Errorf("scan %s schema: %w", tableName, err)
		}
		columns[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s schema: %w", tableName, err)
	}
	return columns, nil
}

func (s *SQLiteResourceStore) tableColumnNotNull(tableName, columnName string) (bool, error) {
	rows, err := s.db.Query(`PRAGMA table_info(` + tableName + `)`)
	if err != nil {
		return false, fmt.Errorf("inspect %s schema: %w", tableName, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid     int
			name    string
			typ     string
			notNull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			return false, fmt.Errorf("scan %s schema: %w", tableName, err)
		}
		if name == columnName {
			return notNull != 0, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate %s schema: %w", tableName, err)
	}
	return false, fmt.Errorf("column %s.%s not found", tableName, columnName)
}

func (s *SQLiteResourceStore) normalizeResourceChangeRows(columns map[string]struct{}) error {
	// Legacy change tables can hold millions of historical rows. Keep schema
	// migration startup-only and expose canonical defaults through read-time
	// expressions instead of rewriting history while the API is still booting.
	_ = columns
	return nil
}

func (s *SQLiteResourceStore) backfillLegacyResourceChangeObservedAt(columns map[string]struct{}) error {
	// See normalizeResourceChangeRows: legacy change history is exposed through
	// read-time fallbacks so API startup stays bounded regardless of table size.
	_ = columns
	return nil
}

func (s *SQLiteResourceStore) resourceChangesObservedAtExpr() string {
	if s.resourceChangesHasTimestamp {
		return "COALESCE(observed_at, timestamp, CURRENT_TIMESTAMP)"
	}
	if s.resourceChangesObservedAtNeedsFallback {
		return "COALESCE(observed_at, CURRENT_TIMESTAMP)"
	}
	return "observed_at"
}

func (s *SQLiteResourceStore) resourceChangesSourceTypeExpr() string {
	sourceTypeExpr := "CASE WHEN TRIM(COALESCE(source_type, '')) = '' THEN 'pulse_diff' ELSE lower(TRIM(source_type)) END"
	if !s.resourceChangesHasSource {
		return sourceTypeExpr
	}
	legacySourceExpr := "lower(TRIM(COALESCE(source, '')))"
	legacySourceTypeExpr := "CASE WHEN " + legacySourceExpr + " IN ('platform_event', 'pulse_diff', 'heuristic', 'user_action', 'agent_action') THEN " + legacySourceExpr + " ELSE 'pulse_diff' END"
	return "CASE WHEN TRIM(COALESCE(source_type, '')) = '' OR (lower(TRIM(COALESCE(source_type, ''))) = 'pulse_diff' AND " + legacySourceExpr + " IN ('platform_event', 'pulse_diff', 'heuristic', 'user_action', 'agent_action')) THEN " + legacySourceTypeExpr + " ELSE lower(TRIM(source_type)) END"
}

func (s *SQLiteResourceStore) resourceChangesSourceAdapterExpr() string {
	if s.resourceChangesHasSource {
		return "CASE WHEN TRIM(COALESCE(source_adapter, '')) = '' THEN COALESCE(NULLIF(TRIM(source), ''), '') ELSE TRIM(source_adapter) END"
	}
	return "COALESCE(NULLIF(TRIM(source_adapter), ''), '')"
}

func resourceChangesActorExpr() string {
	return "COALESCE(NULLIF(TRIM(actor), ''), '')"
}

func resourceChangesRelatedResourcesExpr() string {
	return "COALESCE(NULLIF(TRIM(related_resources), ''), '[]')"
}

func resourceChangesMetadataJSONExpr() string {
	return "COALESCE(NULLIF(TRIM(metadata_json), ''), '{}')"
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

const (
	resourceChangesRetention = 30 * 24 * time.Hour
	actionAuditsRetention    = 90 * 24 * time.Hour
	actionLifecycleRetention = 90 * 24 * time.Hour
	exportAuditsRetention    = 90 * 24 * time.Hour
	loopReportsRetention     = 30 * 24 * time.Hour
	retentionInterval        = 1 * time.Hour
	initialRetentionDelay    = 30 * time.Second
	maxUnifiedReclaimPages   = 50000
	// maxResourceChangesRows bounds resource_changes even inside the
	// retention window: time-based pruning alone cannot contain a
	// pathological writer (the demo hit 1.6M rows in three days), and an
	// unbounded table starves the store's single connection until prunes
	// themselves fail with SQLITE_BUSY.
	maxResourceChangesRows = 200000
)

// migrateAutoVacuum ensures the database uses incremental auto-vacuum so that
// deleted-row pages can be returned to the OS via PRAGMA incremental_vacuum.
// For databases created before auto_vacuum(INCREMENTAL) was in the DSN, a
// one-time VACUUM restructures the file. Without this, retention deletes rows
// but the file never shrinks (GitHub issue #1496).
func (s *SQLiteResourceStore) migrateAutoVacuum() {
	var mode int
	if err := s.db.QueryRow("PRAGMA auto_vacuum").Scan(&mode); err != nil {
		log.Printf("unifiedresources: failed to check auto_vacuum mode: %v", err)
		return
	}
	if mode == 2 {
		return
	}

	log.Printf("[INFO] unified_resources: converting database to incremental auto-vacuum (one-time migration)")
	start := time.Now()

	if _, err := s.db.Exec("PRAGMA auto_vacuum = INCREMENTAL"); err != nil {
		log.Printf("unifiedresources: failed to set auto_vacuum mode: %v", err)
		return
	}
	if _, err := s.db.Exec("VACUUM"); err != nil {
		log.Printf("unifiedresources: auto-vacuum migration VACUUM failed (will retry next restart): %v", err)
		return
	}

	log.Printf("[INFO] unified_resources: auto-vacuum migration complete in %s", time.Since(start).Round(time.Millisecond))
}

// reclaimFreePages returns freed pages to the OS. auto_vacuum is INCREMENTAL,
// so deleted-row pages sit on the freelist until reclaimed here. Capped at
// maxReclaimPages per cycle so a large backlog drains over several hourly
// cycles instead of holding the write lock for minutes at once.
func (s *SQLiteResourceStore) reclaimFreePages() {
	var freelist int64
	if err := s.db.QueryRow(`PRAGMA freelist_count`).Scan(&freelist); err != nil {
		log.Printf("unifiedresources: failed to read freelist_count: %v", err)
		return
	}
	if freelist == 0 {
		return
	}
	pages := freelist
	if pages > maxUnifiedReclaimPages {
		pages = maxUnifiedReclaimPages
	}
	if _, err := s.db.Exec(fmt.Sprintf(`PRAGMA incremental_vacuum(%d)`, pages)); err != nil {
		log.Printf("unifiedresources: incremental vacuum failed: %v", err)
	}
	if _, err := s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		log.Printf("unifiedresources: WAL checkpoint failed: %v", err)
	}
}

// startRetentionLoop launches a background goroutine that periodically
// prunes old rows from append-only tables and reclaims freed space.
// Without this, resource_changes, action_audits, and related tables grow
// without bound and the database file never shrinks (GitHub issue #1496).
// Returns a stop channel; closing it signals the goroutine to exit.
func (s *SQLiteResourceStore) startRetentionLoop() chan struct{} {
	stop := make(chan struct{})
	go func() {
		initial := time.NewTimer(initialRetentionDelay)
		defer initial.Stop()
		ticker := time.NewTicker(retentionInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-initial.C:
				s.pruneOldRecords()
			case <-ticker.C:
				s.pruneOldRecords()
			}
		}
	}()
	return stop
}

// capResourceChanges deletes the oldest resource_changes rows beyond limit,
// keeping the newest rows by observed_at.
func (s *SQLiteResourceStore) capResourceChanges(limit int) (int64, error) {
	res, err := s.db.Exec(
		`DELETE FROM resource_changes WHERE rowid IN (
			SELECT rowid FROM resource_changes
			ORDER BY observed_at DESC
			LIMIT -1 OFFSET ?
		)`,
		limit,
	)
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return affected, nil
}

func (s *SQLiteResourceStore) pruneOldRecords() {
	now := time.Now()
	changesCutoff := now.Add(-resourceChangesRetention)
	auditsCutoff := now.Add(-actionAuditsRetention)
	lifecycleCutoff := now.Add(-actionLifecycleRetention)
	exportCutoff := now.Add(-exportAuditsRetention)
	loopReportsCutoff := now.Add(-loopReportsRetention)

	tsFmt := "2006-01-02T15:04:05Z"
	var totalDeleted int64

	res, err := s.db.Exec(
		`DELETE FROM resource_changes WHERE observed_at < ? OR observed_at IS NULL`,
		changesCutoff.UTC().Format(tsFmt),
	)
	if err != nil {
		log.Printf("unifiedresources: failed to prune resource_changes: %v", err)
	} else if affected, _ := res.RowsAffected(); affected > 0 {
		totalDeleted += affected
	}

	if affected, err := s.capResourceChanges(maxResourceChangesRows); err != nil {
		log.Printf("unifiedresources: failed to cap resource_changes: %v", err)
	} else if affected > 0 {
		totalDeleted += affected
	}

	res, err = s.db.Exec(
		`DELETE FROM action_audits WHERE created_at < ?`,
		auditsCutoff.UTC().Format(tsFmt),
	)
	if err != nil {
		log.Printf("unifiedresources: failed to prune action_audits: %v", err)
	} else if affected, _ := res.RowsAffected(); affected > 0 {
		totalDeleted += affected
	}

	res, err = s.db.Exec(
		`DELETE FROM action_lifecycle_events WHERE timestamp < ?`,
		lifecycleCutoff.UTC().Format(tsFmt),
	)
	if err != nil {
		log.Printf("unifiedresources: failed to prune action_lifecycle_events: %v", err)
	} else if affected, _ := res.RowsAffected(); affected > 0 {
		totalDeleted += affected
	}

	res, err = s.db.Exec(
		`DELETE FROM export_audits WHERE timestamp < ?`,
		exportCutoff.UTC().Format(tsFmt),
	)
	if err != nil {
		log.Printf("unifiedresources: failed to prune export_audits: %v", err)
	} else if affected, _ := res.RowsAffected(); affected > 0 {
		totalDeleted += affected
	}

	res, err = s.db.Exec(
		`DELETE FROM loop_reports WHERE started_at < ?`,
		loopReportsCutoff.UTC().Format(tsFmt),
	)
	if err != nil {
		log.Printf("unifiedresources: failed to prune loop_reports: %v", err)
	} else if affected, _ := res.RowsAffected(); affected > 0 {
		totalDeleted += affected
	}

	if totalDeleted > 0 {
		log.Printf("unifiedresources: pruned %d old records (changes<%s, audits<%s, lifecycle<%s, exports<%s, loop_reports<%s)",
			totalDeleted,
			changesCutoff.Format("2006-01-02"),
			auditsCutoff.Format("2006-01-02"),
			lifecycleCutoff.Format("2006-01-02"),
			exportCutoff.Format("2006-01-02"),
			loopReportsCutoff.Format("2006-01-02"))
	}

	s.reclaimFreePages()
}

func (s *SQLiteResourceStore) Close() error {
	if s.retentionStop != nil {
		close(s.retentionStop)
	}
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return fmt.Errorf("close resources db %q: %w", s.dbPath, err)
		}
	}
	return nil
}

func (s *SQLiteResourceStore) UpsertResourceIdentityPins(pins []ResourceIdentityPin) error {
	if len(pins) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin identity pin upsert: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	for _, pin := range pins {
		pin = pin.normalized()
		if pin.CanonicalID == "" || !pin.hasStrongKey() {
			continue
		}
		// A strong identity key can only belong to one canonical resource.
		// When a key moves (host reinstalled, cluster slot re-occupied by a
		// different machine), the new pin claims it and stale rows go.
		if _, err := tx.Exec(`DELETE FROM resource_identities WHERE canonical_id != ? AND (
				(machine_id IS NOT NULL AND machine_id = ?)
				OR (dmi_uuid IS NOT NULL AND dmi_uuid = ?)
				OR (cluster_name IS NOT NULL AND cluster_name != '' AND cluster_name = ? AND primary_hostname = ?)
			)`,
			pin.CanonicalID,
			nullIfEmptyArg(pin.MachineID),
			nullIfEmptyArg(pin.DMIUUID),
			pin.ClusterName,
			pin.Hostname,
		); err != nil {
			return fmt.Errorf("clear conflicting identity pins for %q: %w", pin.CanonicalID, err)
		}
		if _, err := tx.Exec(`INSERT INTO resource_identities (canonical_id, resource_type, machine_id, dmi_uuid, cluster_name, primary_hostname)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(canonical_id) DO UPDATE SET
				resource_type = excluded.resource_type,
				machine_id = COALESCE(excluded.machine_id, resource_identities.machine_id),
				dmi_uuid = COALESCE(excluded.dmi_uuid, resource_identities.dmi_uuid),
				cluster_name = COALESCE(NULLIF(excluded.cluster_name, ''), resource_identities.cluster_name),
				primary_hostname = COALESCE(NULLIF(excluded.primary_hostname, ''), resource_identities.primary_hostname),
				updated_at = CURRENT_TIMESTAMP`,
			pin.CanonicalID,
			string(pin.ResourceType),
			nullIfEmptyArg(pin.MachineID),
			nullIfEmptyArg(pin.DMIUUID),
			pin.ClusterName,
			pin.Hostname,
		); err != nil {
			return fmt.Errorf("upsert identity pin for %q: %w", pin.CanonicalID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit identity pin upsert: %w", err)
	}
	committed = true

	s.identityPinMu.Lock()
	s.identityPinFresh = false
	s.identityPinMu.Unlock()
	return nil
}

func nullIfEmptyArg(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func (s *SQLiteResourceStore) ListResourceIdentityPins() ([]ResourceIdentityPin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.queryResourceIdentityPins()
}

func (s *SQLiteResourceStore) queryResourceIdentityPins() ([]ResourceIdentityPin, error) {
	rows, err := s.db.Query(`SELECT canonical_id, resource_type, COALESCE(machine_id, ''), COALESCE(dmi_uuid, ''), COALESCE(cluster_name, ''), COALESCE(primary_hostname, '') FROM resource_identities`)
	if err != nil {
		return nil, fmt.Errorf("query identity pins: %w", err)
	}
	defer rows.Close()

	var pins []ResourceIdentityPin
	for rows.Next() {
		var pin ResourceIdentityPin
		var resourceType string
		if err := rows.Scan(&pin.CanonicalID, &resourceType, &pin.MachineID, &pin.DMIUUID, &pin.ClusterName, &pin.Hostname); err != nil {
			return nil, fmt.Errorf("scan identity pin row: %w", err)
		}
		pin.ResourceType = ResourceType(resourceType)
		pins = append(pins, pin.normalized())
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate identity pin rows: %w", err)
	}
	return pins, nil
}

// resourceChangeIDSet expands a canonical ID to the set of IDs its journal
// rows may have been recorded under. Canonical IDs minted before the host's
// strongest identity key was known hash a weaker key (cluster+hostname during
// a boot window vs machine ID in steady state); the pinned identity makes
// every era's ID recomputable, so reads merge the eras without rewriting
// history. Unknown IDs expand to themselves.
func (s *SQLiteResourceStore) resourceChangeIDSet(canonicalID string) []string {
	canonicalID = CanonicalResourceID(canonicalID)
	if canonicalID == "" {
		return nil
	}

	s.identityPinMu.Lock()
	if !s.identityPinFresh {
		pins, err := s.queryResourceIdentityPins()
		if err == nil {
			s.identityPinCache = pins
			s.identityPinFresh = true
		}
	}
	pins := s.identityPinCache
	s.identityPinMu.Unlock()

	return expandResourceChangeIDs(canonicalID, pins)
}

func expandResourceChangeIDs(canonicalID string, pins []ResourceIdentityPin) []string {
	for _, pin := range pins {
		eraIDs := pin.EraIDs()
		for _, eraID := range eraIDs {
			if eraID == canonicalID {
				return eraIDs
			}
		}
	}
	return []string{canonicalID}
}

func (s *SQLiteResourceStore) RecordChange(change ResourceChange) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return recordChangeSQL(s.db, change, s.resourceChangesHasTimestamp)
}

func recordChangeSQL(execer sqlExecutor, change ResourceChange, includeTimestamp bool) error {
	relJSON, _ := json.Marshal(change.RelatedResources)
	metaJSON, _ := json.Marshal(change.Metadata)

	columns := []string{"id", "canonical_id", "observed_at"}
	values := []any{change.ID, CanonicalResourceID(change.ResourceID), change.ObservedAt}
	if includeTimestamp {
		columns = append(columns, "timestamp")
		values = append(values, change.ObservedAt)
	}
	columns = append(columns,
		"occurred_at",
		"kind",
		"from_state",
		"to_state",
		"source_type",
		"source_adapter",
		"actor",
		"confidence",
		"reason",
		"related_resources",
		"metadata_json",
	)
	values = append(values,
		change.OccurredAt,
		string(change.Kind),
		change.From,
		change.To,
		change.SourceType,
		change.SourceAdapter,
		change.Actor,
		string(change.Confidence),
		change.Reason,
		string(relJSON),
		string(metaJSON),
	)

	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	_, err := execer.Exec(
		`INSERT INTO resource_changes (`+strings.Join(columns, ", ")+`) VALUES (`+strings.Join(placeholders, ", ")+`) ON CONFLICT(id) DO NOTHING`,
		values...,
	)
	if err != nil {
		return fmt.Errorf("insert resource change: %w", err)
	}
	return nil
}

func (s *SQLiteResourceStore) GetRecentChanges(canonicalID string, since time.Time, limit int) ([]ResourceChange, error) {
	return s.GetRecentChangesFiltered(canonicalID, since, limit, ResourceChangeFilters{})
}

func (s *SQLiteResourceStore) GetRecentChangesFiltered(canonicalID string, since time.Time, limit int, filters ResourceChangeFilters) ([]ResourceChange, error) {
	observedAtExpr := s.resourceChangesObservedAtExpr()
	sourceTypeExpr := s.resourceChangesSourceTypeExpr()
	sourceAdapterExpr := s.resourceChangesSourceAdapterExpr()
	query := `
		SELECT id, canonical_id, ` + observedAtExpr + `, occurred_at, COALESCE(kind, ''), COALESCE(from_state, ''), COALESCE(to_state, ''), ` + sourceTypeExpr + `, ` + sourceAdapterExpr + `, ` + resourceChangesActorExpr() + `, COALESCE(confidence, ''), COALESCE(reason, ''), ` + resourceChangesRelatedResourcesExpr() + `, ` + resourceChangesMetadataJSONExpr() + `
		FROM resource_changes`

	args := []any{}
	conditions := []string{}
	canonicalID = CanonicalResourceID(canonicalID)
	if canonicalID != "" {
		conditions, args = appendRecentChangeResourceCondition(conditions, args, s.resourceChangeIDSet(canonicalID), filters.IncludeRelated)
	} else {
		conditions = append(conditions, observedAtExpr+" >= ?")
		args = append(args, since)
	}
	if !since.IsZero() && canonicalID != "" {
		conditions = append(conditions, observedAtExpr+" >= ?")
		args = append(args, since)
	}
	if len(filters.Kinds) > 0 {
		placeholders := make([]string, 0, len(filters.Kinds))
		for _, kind := range filters.Kinds {
			placeholders = append(placeholders, "?")
			args = append(args, string(kind))
		}
		conditions = append(conditions, "kind IN ("+strings.Join(placeholders, ", ")+")")
	}
	if len(filters.SourceTypes) > 0 {
		placeholders := make([]string, 0, len(filters.SourceTypes))
		for _, sourceType := range filters.SourceTypes {
			placeholders = append(placeholders, "?")
			args = append(args, string(sourceType))
		}
		conditions = append(conditions, sourceTypeExpr+" IN ("+strings.Join(placeholders, ", ")+")")
	}
	if len(filters.SourceAdapters) > 0 {
		placeholders := make([]string, 0, len(filters.SourceAdapters))
		for _, sourceAdapter := range filters.SourceAdapters {
			placeholders = append(placeholders, "?")
			args = append(args, string(sourceAdapter))
		}
		conditions = append(conditions, sourceAdapterExpr+" IN ("+strings.Join(placeholders, ", ")+")")
	}
	if len(conditions) > 0 {
		query += "\n\t\tWHERE " + strings.Join(conditions, " AND ")
	}
	query += `
		ORDER BY ` + observedAtExpr + ` DESC`
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
		var observedAt any

		if err := rows.Scan(&c.ID, &c.ResourceID, &observedAt, &c.OccurredAt, &kindStr, &c.From, &c.To, &c.SourceType, &c.SourceAdapter, &c.Actor, &conf, &c.Reason, &relText, &metaText); err != nil {
			return nil, fmt.Errorf("scan resource change row: %w", err)
		}
		parsedObservedAt, err := parseResourceChangeTime(observedAt)
		if err != nil {
			return nil, fmt.Errorf("parse resource change observed_at for %q: %w", c.ID, err)
		}
		c.ObservedAt = parsedObservedAt
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

func parseResourceChangeTime(value any) (time.Time, error) {
	switch v := value.(type) {
	case time.Time:
		return v, nil
	case string:
		return parseResourceChangeTimeString(v)
	case []byte:
		return parseResourceChangeTimeString(string(v))
	case nil:
		return time.Time{}, nil
	default:
		return time.Time{}, fmt.Errorf("unsupported time value type %T", value)
	}
}

func parseResourceChangeTimeString(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time value %q", value)
}

func (s *SQLiteResourceStore) CountRecentChanges(canonicalID string, since time.Time) (int, error) {
	return s.CountRecentChangesFiltered(canonicalID, since, ResourceChangeFilters{})
}

func (s *SQLiteResourceStore) CountRecentChangesFiltered(canonicalID string, since time.Time, filters ResourceChangeFilters) (int, error) {
	query, args := buildRecentChangeCountQuery(
		s.resourceChangeIDSet(canonicalID),
		since,
		filters,
		"SELECT COUNT(*) FROM resource_changes",
		s.resourceChangesObservedAtExpr(),
		s.resourceChangesSourceTypeExpr(),
		s.resourceChangesSourceAdapterExpr(),
	)

	s.mu.Lock()
	defer s.mu.Unlock()

	var count int
	if err := s.db.QueryRow(query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count resource changes: %w", err)
	}
	return count, nil
}

func (s *SQLiteResourceStore) CountRecentChangesByKind(canonicalID string, since time.Time) (map[ChangeKind]int, error) {
	return s.CountRecentChangesByKindFiltered(canonicalID, since, ResourceChangeFilters{})
}

func (s *SQLiteResourceStore) CountRecentChangesByKindFiltered(canonicalID string, since time.Time, filters ResourceChangeFilters) (map[ChangeKind]int, error) {
	query, args := buildRecentChangeCountQuery(
		s.resourceChangeIDSet(canonicalID),
		since,
		filters,
		"SELECT COALESCE(kind, ''), COUNT(*) FROM resource_changes",
		s.resourceChangesObservedAtExpr(),
		s.resourceChangesSourceTypeExpr(),
		s.resourceChangesSourceAdapterExpr(),
	)
	query += ` GROUP BY kind`

	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query resource change counts by kind: %w", err)
	}
	defer rows.Close()

	counts := make(map[ChangeKind]int)
	for rows.Next() {
		var kind string
		var count int
		if err := rows.Scan(&kind, &count); err != nil {
			return nil, fmt.Errorf("scan resource change kind count row: %w", err)
		}
		if kind == "" {
			continue
		}
		counts[ChangeKind(kind)] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate resource change kind count rows: %w", err)
	}
	return counts, nil
}

func (s *SQLiteResourceStore) CountRecentChangesBySourceType(canonicalID string, since time.Time) (map[ChangeSourceType]int, error) {
	return s.CountRecentChangesBySourceTypeFiltered(canonicalID, since, ResourceChangeFilters{})
}

func (s *SQLiteResourceStore) CountRecentChangesBySourceTypeFiltered(canonicalID string, since time.Time, filters ResourceChangeFilters) (map[ChangeSourceType]int, error) {
	sourceTypeExpr := s.resourceChangesSourceTypeExpr()
	query, args := buildRecentChangeCountQuery(
		s.resourceChangeIDSet(canonicalID),
		since,
		filters,
		"SELECT "+sourceTypeExpr+", COUNT(*) FROM resource_changes",
		s.resourceChangesObservedAtExpr(),
		sourceTypeExpr,
		s.resourceChangesSourceAdapterExpr(),
	)
	query += ` GROUP BY ` + sourceTypeExpr

	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query resource change counts by source type: %w", err)
	}
	defer rows.Close()

	counts := make(map[ChangeSourceType]int)
	for rows.Next() {
		var sourceType string
		var count int
		if err := rows.Scan(&sourceType, &count); err != nil {
			return nil, fmt.Errorf("scan resource change source type count row: %w", err)
		}
		if sourceType == "" {
			continue
		}
		counts[ChangeSourceType(sourceType)] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate resource change source type count rows: %w", err)
	}
	return counts, nil
}

func (s *SQLiteResourceStore) CountRecentChangesBySourceAdapter(canonicalID string, since time.Time) (map[ChangeSourceAdapter]int, error) {
	return s.CountRecentChangesBySourceAdapterFiltered(canonicalID, since, ResourceChangeFilters{})
}

func (s *SQLiteResourceStore) CountRecentChangesBySourceAdapterFiltered(canonicalID string, since time.Time, filters ResourceChangeFilters) (map[ChangeSourceAdapter]int, error) {
	sourceAdapterExpr := s.resourceChangesSourceAdapterExpr()
	query, args := buildRecentChangeCountQuery(
		s.resourceChangeIDSet(canonicalID),
		since,
		filters,
		"SELECT "+sourceAdapterExpr+", COUNT(*) FROM resource_changes",
		s.resourceChangesObservedAtExpr(),
		s.resourceChangesSourceTypeExpr(),
		sourceAdapterExpr,
	)
	query += ` GROUP BY ` + sourceAdapterExpr

	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query resource change counts by source adapter: %w", err)
	}
	defer rows.Close()

	counts := make(map[ChangeSourceAdapter]int)
	for rows.Next() {
		var sourceAdapter string
		var count int
		if err := rows.Scan(&sourceAdapter, &count); err != nil {
			return nil, fmt.Errorf("scan resource change source adapter count row: %w", err)
		}
		if sourceAdapter == "" {
			continue
		}
		counts[ChangeSourceAdapter(sourceAdapter)] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate resource change source adapter count rows: %w", err)
	}
	return counts, nil
}

type sqlExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func actionAuditSQLArgs(record ActionAuditRecord) ([]any, error) {
	requestJSON, err := json.Marshal(record.Request)
	if err != nil {
		return nil, fmt.Errorf("marshal action request: %w", err)
	}
	planJSON, err := json.Marshal(record.Plan)
	if err != nil {
		return nil, fmt.Errorf("marshal action plan: %w", err)
	}
	approvalsJSON, err := json.Marshal(record.Approvals)
	if err != nil {
		return nil, fmt.Errorf("marshal action approvals: %w", err)
	}
	resultJSON, err := json.Marshal(record.Result)
	if err != nil {
		return nil, fmt.Errorf("marshal action result: %w", err)
	}
	verificationOutcomeJSON, err := json.Marshal(record.VerificationOutcome)
	if err != nil {
		return nil, fmt.Errorf("marshal verification outcome: %w", err)
	}
	var originJSON any
	if origin := NormalizeActionOrigin(record.Origin); origin != nil {
		encoded, err := json.Marshal(origin)
		if err != nil {
			return nil, fmt.Errorf("marshal action origin: %w", err)
		}
		originJSON = string(encoded)
	}

	return []any{record.ID, record.ID, CanonicalResourceID(record.Request.ResourceID), record.Request.RequestID, record.CreatedAt, record.UpdatedAt, string(record.State), string(requestJSON), string(planJSON), string(approvalsJSON), string(resultJSON), string(verificationOutcomeJSON), originJSON}, nil
}

func insertActionAuditSQL(exec sqlExecutor, record ActionAuditRecord) (bool, error) {
	args, err := actionAuditSQLArgs(record)
	if err != nil {
		return false, err
	}
	result, err := exec.Exec(`
		INSERT INTO action_audits (id, action_id, canonical_id, request_id, created_at, updated_at, state, request_json, plan_json, approvals_json, result_json, verification_outcome_json, origin_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING
	`, args...)
	if err != nil {
		return false, fmt.Errorf("insert action audit: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read inserted action audit rows: %w", err)
	}
	return rows == 1, nil
}

func updateActionAuditSQL(exec sqlExecutor, record ActionAuditRecord, expectedStates ...ActionState) (bool, error) {
	args, err := actionAuditSQLArgs(record)
	if err != nil {
		return false, err
	}
	setArgs := append([]any(nil), args[1:]...)
	setArgs = append(setArgs, record.ID)
	placeholders := make([]string, len(expectedStates))
	for i, state := range expectedStates {
		placeholders[i] = "?"
		setArgs = append(setArgs, string(state))
	}
	setArgs = append(setArgs, args[7], args[8], firstNonNilString(args[12]))
	result, err := exec.Exec(`
		UPDATE action_audits SET
			action_id=?, canonical_id=?, request_id=?, created_at=?, updated_at=?, state=?,
			request_json=?, plan_json=?, approvals_json=?, result_json=?, verification_outcome_json=?, origin_json=?
		WHERE id=? AND state IN (`+strings.Join(placeholders, ",")+`)
		  AND request_json=? AND plan_json=? AND COALESCE(origin_json, '')=?
	`, setArgs...)
	if err != nil {
		return false, fmt.Errorf("update action audit: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read updated action audit rows: %w", err)
	}
	return rows == 1, nil
}

func firstNonNilString(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func normalizeActionAuditCreation(record ActionAuditRecord, initialEvents []ActionLifecycleEvent) (ActionAuditRecord, []ActionLifecycleEvent, error) {
	normalized, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return ActionAuditRecord{}, nil, err
	}
	// Redact known secret shapes from operator-authored text and command
	// output before persisting. The audit log is plaintext SQL; raw
	// secrets pasted into a reason field or echoed in command output
	// must not be retained.
	record = RedactAuditRecord(normalized)
	events := make([]ActionLifecycleEvent, len(initialEvents))
	seen := make(map[ActionState]struct{}, len(initialEvents))
	for i, event := range initialEvents {
		event, err = NormalizeActionLifecycleEvent(event)
		if err != nil {
			return ActionAuditRecord{}, nil, err
		}
		if event.ActionID != record.ID {
			return ActionAuditRecord{}, nil, fmt.Errorf("initial lifecycle event id %q does not match action audit id %q", event.ActionID, record.ID)
		}
		if _, duplicate := seen[event.State]; duplicate {
			return ActionAuditRecord{}, nil, fmt.Errorf("duplicate initial lifecycle state %q", event.State)
		}
		seen[event.State] = struct{}{}
		events[i] = event
	}
	return record, events, nil
}

func (s *SQLiteResourceStore) CreateActionAudit(record ActionAuditRecord, initialEvents []ActionLifecycleEvent) (ActionAuditRecord, bool, error) {
	record, events, err := normalizeActionAuditCreation(record, initialEvents)
	if err != nil {
		return ActionAuditRecord{}, false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return ActionAuditRecord{}, false, fmt.Errorf("begin action audit creation transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	created, err := insertActionAuditSQL(tx, record)
	if err != nil {
		return ActionAuditRecord{}, false, err
	}
	if !created {
		current, found, err := getActionAuditFrom(tx, record.ID)
		if err != nil {
			return ActionAuditRecord{}, false, err
		}
		if !found {
			return ActionAuditRecord{}, false, fmt.Errorf("action audit %q disappeared during creation", record.ID)
		}
		if !ActionAuditIdentityMatches(current, record) {
			return current, false, ErrActionIdentityConflict
		}
		if err := tx.Commit(); err != nil {
			return ActionAuditRecord{}, false, fmt.Errorf("commit action audit replay transaction: %w", err)
		}
		committed = true
		return current, false, nil
	}
	for _, event := range events {
		if err := recordActionLifecycleEventSQL(tx, event); err != nil {
			return ActionAuditRecord{}, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ActionAuditRecord{}, false, fmt.Errorf("commit action audit creation transaction: %w", err)
	}
	committed = true
	return record, true, nil
}

// RecordActionAudit is retained as an insert-only compatibility boundary.
// Production lifecycle code must use CreateActionAudit or a typed transition;
// replacing an existing record would permit state rewind.
func (s *SQLiteResourceStore) RecordActionAudit(record ActionAuditRecord) error {
	_, created, err := s.CreateActionAudit(record, nil)
	if err != nil {
		return err
	}
	if !created {
		return ErrActionAuditAlreadyExists
	}
	return nil
}

type actionAuditScanner interface {
	Scan(dest ...any) error
}

func scanActionAuditRecord(scanner actionAuditScanner) (ActionAuditRecord, error) {
	var record ActionAuditRecord
	var stateStr string
	var actionID, requestID string
	var requestJSON, planJSON, approvalsJSON, resultJSON, verificationOutcomeJSON, originJSON sql.NullString
	if err := scanner.Scan(&record.ID, &actionID, &requestID, &record.CreatedAt, &record.UpdatedAt, &stateStr, &requestJSON, &planJSON, &approvalsJSON, &resultJSON, &verificationOutcomeJSON, &originJSON); err != nil {
		return ActionAuditRecord{}, err
	}

	record.State = ActionState(stateStr)
	if requestJSON.Valid && requestJSON.String != "" {
		if err := json.Unmarshal([]byte(requestJSON.String), &record.Request); err != nil {
			return ActionAuditRecord{}, fmt.Errorf("unmarshal action request: %w", err)
		}
	}
	if planJSON.Valid && planJSON.String != "" {
		if err := json.Unmarshal([]byte(planJSON.String), &record.Plan); err != nil {
			return ActionAuditRecord{}, fmt.Errorf("unmarshal action plan: %w", err)
		}
	}
	if approvalsJSON.Valid && approvalsJSON.String != "" {
		if err := json.Unmarshal([]byte(approvalsJSON.String), &record.Approvals); err != nil {
			return ActionAuditRecord{}, fmt.Errorf("unmarshal action approvals: %w", err)
		}
	}
	if resultJSON.Valid && resultJSON.String != "" && resultJSON.String != "null" {
		var result ExecutionResult
		if err := json.Unmarshal([]byte(resultJSON.String), &result); err != nil {
			return ActionAuditRecord{}, fmt.Errorf("unmarshal action result: %w", err)
		}
		record.Result = &result
	}
	if verificationOutcomeJSON.Valid && verificationOutcomeJSON.String != "" && verificationOutcomeJSON.String != "null" {
		var outcome VerificationOutcome
		if err := json.Unmarshal([]byte(verificationOutcomeJSON.String), &outcome); err != nil {
			return ActionAuditRecord{}, fmt.Errorf("unmarshal verification outcome: %w", err)
		}
		record.VerificationOutcome = outcome
	}
	record.VerificationOutcome = NormalizeVerificationOutcome(record.VerificationOutcome)
	if originJSON.Valid && originJSON.String != "" && originJSON.String != "null" {
		var origin ActionOrigin
		if err := json.Unmarshal([]byte(originJSON.String), &origin); err != nil {
			return ActionAuditRecord{}, fmt.Errorf("unmarshal action origin: %w", err)
		}
		record.Origin = NormalizeActionOrigin(&origin)
	}
	record.Request.RequestID = requestID
	record.Request.ResourceID = CanonicalResourceID(record.Request.ResourceID)
	_ = actionID
	return redactActionAuditRecordFromStore(record), nil
}

func redactActionAuditRecordFromStore(record ActionAuditRecord) ActionAuditRecord {
	return RedactAuditRecord(normalizeActionAuditRecordFromStore(record))
}

func normalizeActionAuditRecordFromStore(record ActionAuditRecord) ActionAuditRecord {
	normalized, err := NormalizeActionAuditRecord(record)
	if err == nil {
		return normalized
	}

	if record.Result != nil {
		result := *record.Result
		result.Output = strings.TrimSpace(result.Output)
		result.ErrorMessage = strings.TrimSpace(result.ErrorMessage)
		result.Verification = NormalizeActionVerificationResult(result.Verification)
		record.Result = &result
	}
	record.Verification = NormalizeActionVerificationResult(record.Verification)
	if record.Verification == nil && record.Result != nil {
		record.Verification = cloneActionVerificationResult(record.Result.Verification)
	}
	if record.Verification != nil && record.Result != nil {
		result := *record.Result
		result.Verification = cloneActionVerificationResult(record.Verification)
		record.Result = &result
	}
	record.VerificationOutcome = NormalizeVerificationOutcome(record.VerificationOutcome)
	return record
}

func (s *SQLiteResourceStore) GetActionAudit(actionID string) (ActionAuditRecord, bool, error) {
	return s.getActionAudit(actionID)
}

func (s *SQLiteResourceStore) GetLatestActionAuditByOrigin(surface, investigationID string) (ActionAuditRecord, bool, error) {
	surface = strings.TrimSpace(surface)
	investigationID = strings.TrimSpace(investigationID)
	if surface == "" || investigationID == "" {
		return ActionAuditRecord{}, false, nil
	}
	row := s.db.QueryRow(`
		SELECT id, action_id, request_id, created_at, updated_at, state, request_json, plan_json, approvals_json, result_json, verification_outcome_json, origin_json
		FROM action_audits
		WHERE json_valid(origin_json)
		  AND json_extract(origin_json, '$.surface') = ?
		  AND json_extract(origin_json, '$.investigationId') = ?
		ORDER BY updated_at DESC, created_at DESC
		LIMIT 1
	`, surface, investigationID)
	record, err := scanActionAuditRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ActionAuditRecord{}, false, nil
	}
	if err != nil {
		return ActionAuditRecord{}, false, fmt.Errorf("query action audit by origin: %w", err)
	}
	return record, true, nil
}

func (s *SQLiteResourceStore) GetPendingActionAudits(limit int) ([]ActionAuditRecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(`
		SELECT id, action_id, request_id, created_at, updated_at, state, request_json, plan_json, approvals_json, result_json, verification_outcome_json, origin_json
		FROM action_audits
		WHERE state = ?
		ORDER BY updated_at ASC, created_at ASC
		LIMIT ?
	`, ActionStatePending, limit)
	if err != nil {
		return nil, fmt.Errorf("query pending action audits: %w", err)
	}
	defer rows.Close()
	records := make([]ActionAuditRecord, 0)
	for rows.Next() {
		record, err := scanActionAuditRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("scan pending action audit row: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending action audit rows: %w", err)
	}
	return records, nil
}

func (s *SQLiteResourceStore) GetActionAudits(canonicalID string, since time.Time, limit int) ([]ActionAuditRecord, error) {
	query := `
		SELECT id, action_id, request_id, created_at, updated_at, state, request_json, plan_json, approvals_json, result_json, verification_outcome_json, origin_json
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
		record, err := scanActionAuditRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("scan action audit row: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate action audit rows: %w", err)
	}
	return records, nil
}

func recordActionLifecycleEventSQL(exec sqlExecutor, event ActionLifecycleEvent) error {
	_, err := exec.Exec(`
		INSERT INTO action_lifecycle_events (action_id, timestamp, state, actor, message)
		VALUES (?, ?, ?, ?, ?)
	`, event.ActionID, event.Timestamp, string(event.State), event.Actor, event.Message)
	if err != nil {
		return fmt.Errorf("insert action lifecycle event: %w", err)
	}
	return nil
}

func actionTransitionConflict(current ActionAuditRecord, desired ActionAuditRecord, fallback error) error {
	if !ActionAuditIdentityMatches(current, desired) {
		return ErrActionIdentityConflict
	}
	switch current.State {
	case ActionStateRejected, ActionStateCompleted, ActionStateFailed:
		return ErrActionExecutionFinal
	case ActionStateExecuting:
		if desired.State == ActionStateExecuting {
			return ErrActionAlreadyExecuting
		}
		return ErrActionNotExecuting
	default:
		return fallback
	}
}

func (s *SQLiteResourceStore) recordActionTransition(record ActionAuditRecord, event ActionLifecycleEvent, expectedStates []ActionState, fallback error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin action transition transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	updated, err := updateActionAuditSQL(tx, record, expectedStates...)
	if err != nil {
		return err
	}
	if !updated {
		current, found, err := getActionAuditFrom(tx, record.ID)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("action audit %q not found", record.ID)
		}
		return actionTransitionConflict(current, record, fallback)
	}
	if err := recordActionLifecycleEventSQL(tx, event); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit action transition transaction: %w", err)
	}
	committed = true
	return nil
}

func (s *SQLiteResourceStore) RecordActionDecision(record ActionAuditRecord, event ActionLifecycleEvent) error {
	normalizedRecord, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return err
	}
	normalizedEvent, err := NormalizeActionLifecycleEvent(event)
	if err != nil {
		return err
	}
	if normalizedEvent.ActionID != normalizedRecord.ID {
		return fmt.Errorf("action decision event id %q does not match action audit id %q", normalizedEvent.ActionID, normalizedRecord.ID)
	}

	return s.recordActionTransition(normalizedRecord, normalizedEvent, []ActionState{ActionStatePending}, ErrActionNotPending)
}

func (s *SQLiteResourceStore) RecordActionExecutionStart(record ActionAuditRecord, event ActionLifecycleEvent) error {
	normalizedRecord, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return err
	}
	normalizedEvent, err := NormalizeActionLifecycleEvent(event)
	if err != nil {
		return err
	}
	if normalizedRecord.State != ActionStateExecuting || normalizedEvent.State != ActionStateExecuting {
		return fmt.Errorf("action execution start must persist executing state")
	}
	if normalizedEvent.ActionID != normalizedRecord.ID {
		return fmt.Errorf("action execution start event id %q does not match action audit id %q", normalizedEvent.ActionID, normalizedRecord.ID)
	}
	// Redact secret shapes from operator-authored fields and command
	// output before persisting; see RecordActionAudit for the contract.
	normalizedRecord = RedactAuditRecord(normalizedRecord)

	if normalizedRecord.Plan.ApprovalPolicy == ApprovalDryRun {
		return ErrActionDryRunOnly
	}
	if !normalizedRecord.Plan.ExpiresAt.IsZero() && !normalizedEvent.Timestamp.Before(normalizedRecord.Plan.ExpiresAt) {
		return ErrActionPlanExpired
	}
	expected := []ActionState{ActionStateApproved}
	if normalizedRecord.Plan.Allowed && !normalizedRecord.Plan.RequiresApproval {
		expected = append(expected, ActionStatePlanned)
	}
	return s.recordActionTransition(normalizedRecord, normalizedEvent, expected, ErrActionNotApproved)
}

// RecordActionPolicyExecutionStart is the single automatic-admission CAS. It
// persists the server-owned policy approval and executing transition together,
// so no reusable approved policy state can escape the transaction.
func (s *SQLiteResourceStore) RecordActionPolicyExecutionStart(record ActionAuditRecord, approvalEvent, executionEvent ActionLifecycleEvent) error {
	record, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return err
	}
	approvalEvent, err = NormalizeActionLifecycleEvent(approvalEvent)
	if err != nil {
		return err
	}
	executionEvent, err = NormalizeActionLifecycleEvent(executionEvent)
	if err != nil {
		return err
	}
	if record.State != ActionStateExecuting || approvalEvent.State != ActionStateApproved || executionEvent.State != ActionStateExecuting || approvalEvent.ActionID != record.ID || executionEvent.ActionID != record.ID {
		return errors.New("policy admission must persist matching approved and executing events")
	}
	if len(record.Approvals) == 0 || record.Approvals[len(record.Approvals)-1].Method != MethodPolicy || record.Approvals[len(record.Approvals)-1].PolicyLease == nil {
		return ErrActionPolicyAuthorizationInvalid
	}
	record = RedactAuditRecord(record)
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin policy admission transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	updated, err := updateActionAuditSQL(tx, record, ActionStatePlanned, ActionStatePending)
	if err != nil {
		return err
	}
	if !updated {
		current, found, queryErr := getActionAuditFrom(tx, record.ID)
		if queryErr != nil {
			return queryErr
		}
		if !found {
			return fmt.Errorf("action audit %q not found", record.ID)
		}
		return actionTransitionConflict(current, record, ErrActionNotApproved)
	}
	if err := recordActionLifecycleEventSQL(tx, approvalEvent); err != nil {
		return err
	}
	if err := recordActionLifecycleEventSQL(tx, executionEvent); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit policy admission transaction: %w", err)
	}
	committed = true
	return nil
}

func (s *SQLiteResourceStore) RecordActionExecutionResult(record ActionAuditRecord, event ActionLifecycleEvent) error {
	normalizedRecord, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return err
	}
	normalizedEvent, err := NormalizeActionLifecycleEvent(event)
	if err != nil {
		return err
	}
	if normalizedRecord.State != ActionStateCompleted && normalizedRecord.State != ActionStateFailed {
		return fmt.Errorf("action execution result must persist terminal execution state")
	}
	if normalizedEvent.State != normalizedRecord.State {
		return fmt.Errorf("action execution result event state %q does not match audit state %q", normalizedEvent.State, normalizedRecord.State)
	}
	if normalizedEvent.ActionID != normalizedRecord.ID {
		return fmt.Errorf("action execution result event id %q does not match action audit id %q", normalizedEvent.ActionID, normalizedRecord.ID)
	}
	// Redact secret shapes from operator-authored fields and command
	// output before persisting; see RecordActionAudit for the contract.
	normalizedRecord = RedactAuditRecord(normalizedRecord)

	return s.recordActionTransition(normalizedRecord, normalizedEvent, []ActionState{ActionStateExecuting}, ErrActionNotExecuting)
}

func (s *SQLiteResourceStore) RecordActionExecutionRefusal(record ActionAuditRecord, event ActionLifecycleEvent) error {
	normalizedRecord, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return err
	}
	normalizedEvent, err := NormalizeActionLifecycleEvent(event)
	if err != nil {
		return err
	}
	if normalizedRecord.State != ActionStateFailed || normalizedEvent.State != ActionStateFailed || normalizedEvent.ActionID != normalizedRecord.ID {
		return fmt.Errorf("action execution refusal must persist matching failed state")
	}
	normalizedRecord = RedactAuditRecord(normalizedRecord)
	return s.recordActionTransition(normalizedRecord, normalizedEvent, []ActionState{ActionStatePlanned, ActionStatePending, ActionStateApproved}, ErrActionExecutionFinal)
}

func (s *SQLiteResourceStore) getActionAudit(actionID string) (ActionAuditRecord, bool, error) {
	return getActionAuditFrom(s.db, actionID)
}

type actionAuditQueryRower interface {
	QueryRow(query string, args ...any) *sql.Row
}

func getActionAuditFrom(queryer actionAuditQueryRower, actionID string) (ActionAuditRecord, bool, error) {
	actionID = strings.TrimSpace(actionID)
	if actionID == "" {
		return ActionAuditRecord{}, false, nil
	}
	row := queryer.QueryRow(`
		SELECT id, action_id, request_id, created_at, updated_at, state, request_json, plan_json, approvals_json, result_json, verification_outcome_json, origin_json
		FROM action_audits
		WHERE id = ?
	`, actionID)
	record, err := scanActionAuditRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ActionAuditRecord{}, false, nil
	}
	if err != nil {
		return ActionAuditRecord{}, false, fmt.Errorf("query action audit: %w", err)
	}
	return record, true, nil
}

func (s *SQLiteResourceStore) RecordActionLifecycleEvent(event ActionLifecycleEvent) error {
	normalized, err := NormalizeActionLifecycleEvent(event)
	if err != nil {
		return err
	}
	event = normalized

	s.mu.Lock()
	defer s.mu.Unlock()

	return recordActionLifecycleEventSQL(s.db, event)
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

// GetResourceOperatorState reads the operator-set state for the given
// canonical ID. Returns (zero, false, nil) on no entry; (state, true,
// nil) on a hit; (zero, false, error) on a query failure. The found
// signal is meaningful for the API GET path which returns 404 vs
// returning the default no-state record.
func (s *SQLiteResourceStore) GetResourceOperatorState(canonicalID string) (ResourceOperatorState, bool, error) {
	return getResourceOperatorStateSQL(s.db, canonicalID)
}

type resourceOperatorStateQueryRower interface {
	QueryRow(query string, args ...any) *sql.Row
}

type resourceOperatorStateScanner interface {
	Scan(dest ...any) error
}

func getResourceOperatorStateSQL(queryer resourceOperatorStateQueryRower, canonicalID string) (ResourceOperatorState, bool, error) {
	canonicalID = strings.TrimSpace(canonicalID)
	if canonicalID == "" {
		return ResourceOperatorState{}, false, nil
	}
	row := queryer.QueryRow(`
		SELECT canonical_id, intentionally_offline, never_auto_remediate,
		       auto_remediation_policy_json,
		       maintenance_start_at, maintenance_end_at, maintenance_reason,
		       criticality, note, set_at, set_by
		FROM resource_operator_state WHERE canonical_id = ?`, canonicalID)
	state, err := scanResourceOperatorState(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ResourceOperatorState{}, false, nil
		}
		return ResourceOperatorState{}, false, fmt.Errorf("query resource operator state: %w", err)
	}
	return state, true, nil
}

func scanResourceOperatorState(scanner resourceOperatorStateScanner) (ResourceOperatorState, error) {
	var state ResourceOperatorState
	var (
		intentional    int
		neverRemediate int
		autoPolicyJSON sql.NullString
		startAt, endAt sql.NullTime
		reason         sql.NullString
		criticality    sql.NullString
		note           sql.NullString
		setBy          sql.NullString
	)
	if err := scanner.Scan(
		&state.CanonicalID,
		&intentional,
		&neverRemediate,
		&autoPolicyJSON,
		&startAt,
		&endAt,
		&reason,
		&criticality,
		&note,
		&state.SetAt,
		&setBy,
	); err != nil {
		return ResourceOperatorState{}, err
	}
	state.IntentionallyOffline = intentional != 0
	state.NeverAutoRemediate = neverRemediate != 0
	if autoPolicyJSON.Valid && strings.TrimSpace(autoPolicyJSON.String) != "" {
		if err := json.Unmarshal([]byte(autoPolicyJSON.String), &state.AutoRemediationPolicy); err != nil {
			return ResourceOperatorState{}, fmt.Errorf("unmarshal auto remediation policy: %w", err)
		}
		state.AutoRemediationPolicy = NormalizeAutoRemediationPolicy(state.AutoRemediationPolicy)
	}
	if startAt.Valid {
		t := startAt.Time
		state.MaintenanceStartAt = &t
	}
	if endAt.Valid {
		t := endAt.Time
		state.MaintenanceEndAt = &t
	}
	if reason.Valid {
		state.MaintenanceReason = reason.String
	}
	if criticality.Valid {
		state.Criticality = ResourceCriticality(criticality.String)
	}
	if note.Valid {
		state.Note = note.String
	}
	if setBy.Valid {
		state.SetBy = setBy.String
	}
	return state, nil
}

// SetResourceOperatorState upserts the state row. Validates and
// normalizes before persistence; rejects malformed records (empty
// canonical ID, mismatched maintenance window, unknown criticality)
// with ErrResourceOperatorStateInvalid so the API boundary can return
// 400 with a stable error code.
func (s *SQLiteResourceStore) SetResourceOperatorState(state ResourceOperatorState) error {
	state = NormalizeResourceOperatorState(state)
	if err := ValidateResourceOperatorState(state); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return setResourceOperatorStateSQL(s.db, state)
}

func setResourceOperatorStateSQL(execer sqlExecutor, state ResourceOperatorState) error {
	intentional := 0
	if state.IntentionallyOffline {
		intentional = 1
	}
	neverRemediate := 0
	if state.NeverAutoRemediate {
		neverRemediate = 1
	}
	var (
		startAt, endAt sql.NullTime
		reason         sql.NullString
		criticality    sql.NullString
		note           sql.NullString
		setBy          sql.NullString
		autoPolicyJSON sql.NullString
	)
	if policy := NormalizeAutoRemediationPolicy(state.AutoRemediationPolicy); policy.Enabled || len(policy.CapabilityNames) > 0 || policy.Window != nil {
		encoded, err := json.Marshal(policy)
		if err != nil {
			return fmt.Errorf("marshal auto remediation policy: %w", err)
		}
		autoPolicyJSON.String = string(encoded)
		autoPolicyJSON.Valid = true
	}
	if state.MaintenanceStartAt != nil {
		startAt.Time = *state.MaintenanceStartAt
		startAt.Valid = true
	}
	if state.MaintenanceEndAt != nil {
		endAt.Time = *state.MaintenanceEndAt
		endAt.Valid = true
	}
	if state.MaintenanceReason != "" {
		reason.String = state.MaintenanceReason
		reason.Valid = true
	}
	if state.Criticality != "" {
		criticality.String = string(state.Criticality)
		criticality.Valid = true
	}
	if state.Note != "" {
		note.String = state.Note
		note.Valid = true
	}
	if state.SetBy != "" {
		setBy.String = state.SetBy
		setBy.Valid = true
	}
	_, err := execer.Exec(`
		INSERT INTO resource_operator_state (
			canonical_id, intentionally_offline, never_auto_remediate,
			auto_remediation_policy_json,
			maintenance_start_at, maintenance_end_at, maintenance_reason,
			criticality, note, set_at, set_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(canonical_id) DO UPDATE SET
			intentionally_offline = excluded.intentionally_offline,
			never_auto_remediate = excluded.never_auto_remediate,
			auto_remediation_policy_json = excluded.auto_remediation_policy_json,
			maintenance_start_at = excluded.maintenance_start_at,
			maintenance_end_at = excluded.maintenance_end_at,
			maintenance_reason = excluded.maintenance_reason,
			criticality = excluded.criticality,
			note = excluded.note,
			set_at = excluded.set_at,
			set_by = excluded.set_by`,
		state.CanonicalID,
		intentional,
		neverRemediate,
		autoPolicyJSON,
		startAt,
		endAt,
		reason,
		criticality,
		note,
		state.SetAt,
		setBy,
	)
	if err != nil {
		return fmt.Errorf("upsert resource operator state: %w", err)
	}
	return nil
}

// SetResourceOperatorStateWithMaintenanceLifecycle persists the operator-state
// source row and the derived maintenance-window timeline projection in a single
// SQLite transaction.
func (s *SQLiteResourceStore) SetResourceOperatorStateWithMaintenanceLifecycle(state ResourceOperatorState) (ResourceOperatorState, error) {
	state = NormalizeResourceOperatorState(state)
	if err := ValidateResourceOperatorState(state); err != nil {
		return ResourceOperatorState{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return ResourceOperatorState{}, fmt.Errorf("begin resource operator state transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	previous, previousFound, err := getResourceOperatorStateSQL(tx, state.CanonicalID)
	if err != nil {
		return ResourceOperatorState{}, err
	}
	if err := setResourceOperatorStateSQL(tx, state); err != nil {
		return ResourceOperatorState{}, err
	}
	if change, ok := BuildMaintenanceWindowLifecycleChange(previous, previousFound, state, true, state.SetAt, state.SetBy); ok {
		if err := recordChangeSQL(tx, change, s.resourceChangesHasTimestamp); err != nil {
			return ResourceOperatorState{}, fmt.Errorf("record maintenance window lifecycle change: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return ResourceOperatorState{}, fmt.Errorf("commit resource operator state transaction: %w", err)
	}
	committed = true
	return state, nil
}

// ClearResourceOperatorState removes the row for the given canonical
// ID. Idempotent — returns nil whether or not a row existed.
func (s *SQLiteResourceStore) ClearResourceOperatorState(canonicalID string) error {
	canonicalID = strings.TrimSpace(canonicalID)
	if canonicalID == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return clearResourceOperatorStateSQL(s.db, canonicalID)
}

func clearResourceOperatorStateSQL(execer sqlExecutor, canonicalID string) error {
	_, err := execer.Exec(`DELETE FROM resource_operator_state WHERE canonical_id = ?`, canonicalID)
	if err != nil {
		return fmt.Errorf("delete resource operator state: %w", err)
	}
	return nil
}

// ClearResourceOperatorStateWithMaintenanceLifecycle deletes the operator-state
// source row and the derived maintenance-window timeline projection in a single
// SQLite transaction.
func (s *SQLiteResourceStore) ClearResourceOperatorStateWithMaintenanceLifecycle(canonicalID string, observedAt time.Time, actor string) error {
	canonicalID = strings.TrimSpace(canonicalID)
	if canonicalID == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin resource operator state clear transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	previous, previousFound, err := getResourceOperatorStateSQL(tx, canonicalID)
	if err != nil {
		return err
	}
	if err := clearResourceOperatorStateSQL(tx, canonicalID); err != nil {
		return err
	}
	current := ResourceOperatorState{CanonicalID: canonicalID}
	if change, ok := BuildMaintenanceWindowLifecycleChange(previous, previousFound, current, false, observedAt, actor); ok {
		if err := recordChangeSQL(tx, change, s.resourceChangesHasTimestamp); err != nil {
			return fmt.Errorf("record maintenance window lifecycle change: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit resource operator state clear transaction: %w", err)
	}
	committed = true
	return nil
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
	resourceOperatorState map[string]ResourceOperatorState
	loopReports           map[string]LoopReport
	identityPins          map[string]ResourceIdentityPin
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		resourceOperatorState: make(map[string]ResourceOperatorState),
		loopReports:           make(map[string]LoopReport),
		identityPins:          make(map[string]ResourceIdentityPin),
	}
}

func (m *MemoryStore) UpsertResourceIdentityPins(pins []ResourceIdentityPin) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, pin := range pins {
		pin = pin.normalized()
		if pin.CanonicalID == "" || !pin.hasStrongKey() {
			continue
		}
		// Mirror the SQLite conflict semantics: a strong identity key belongs
		// to exactly one canonical resource.
		for canonicalID, existing := range m.identityPins {
			if canonicalID == pin.CanonicalID {
				continue
			}
			if (pin.MachineID != "" && existing.MachineID == pin.MachineID) ||
				(pin.DMIUUID != "" && existing.DMIUUID == pin.DMIUUID) ||
				(pin.ClusterName != "" && existing.ClusterName == pin.ClusterName && existing.Hostname == pin.Hostname) {
				delete(m.identityPins, canonicalID)
			}
		}
		if existing, ok := m.identityPins[pin.CanonicalID]; ok {
			if pin.MachineID == "" {
				pin.MachineID = existing.MachineID
			}
			if pin.DMIUUID == "" {
				pin.DMIUUID = existing.DMIUUID
			}
			if pin.ClusterName == "" {
				pin.ClusterName = existing.ClusterName
			}
			if pin.Hostname == "" {
				pin.Hostname = existing.Hostname
			}
		}
		m.identityPins[pin.CanonicalID] = pin
	}
	return nil
}

func (m *MemoryStore) ListResourceIdentityPins() ([]ResourceIdentityPin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pins := make([]ResourceIdentityPin, 0, len(m.identityPins))
	for _, pin := range m.identityPins {
		pins = append(pins, pin)
	}
	return pins, nil
}

func (m *MemoryStore) resourceChangeIDSetLocked(canonicalID string) []string {
	canonicalID = CanonicalResourceID(canonicalID)
	if canonicalID == "" {
		return nil
	}
	pins := make([]ResourceIdentityPin, 0, len(m.identityPins))
	for _, pin := range m.identityPins {
		pins = append(pins, pin)
	}
	return expandResourceChangeIDs(canonicalID, pins)
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

	return m.recordChangeLocked(change)
}

func (m *MemoryStore) recordChangeLocked(change ResourceChange) error {
	for _, existing := range m.changes {
		if existing.ID == change.ID && change.ID != "" {
			return nil
		}
	}
	m.changes = append(m.changes, change)
	return nil
}

func (m *MemoryStore) GetRecentChanges(canonicalID string, since time.Time, limit int) ([]ResourceChange, error) {
	return m.GetRecentChangesFiltered(canonicalID, since, limit, ResourceChangeFilters{})
}

func (m *MemoryStore) GetRecentChangesFiltered(canonicalID string, since time.Time, limit int, filters ResourceChangeFilters) ([]ResourceChange, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	canonicalID = CanonicalResourceID(canonicalID)
	idSet := m.resourceChangeIDSetLocked(canonicalID)
	var out []ResourceChange
	for i := len(m.changes) - 1; i >= 0; i-- {
		change := m.changes[i]
		if canonicalID != "" && !changeMatchesResource(change, idSet, filters.IncludeRelated) {
			continue
		}
		if !since.IsZero() && change.ObservedAt.Before(since) {
			continue
		}
		if !filters.matches(change) {
			continue
		}
		out = append(out, change)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *MemoryStore) CountRecentChanges(canonicalID string, since time.Time) (int, error) {
	return m.CountRecentChangesFiltered(canonicalID, since, ResourceChangeFilters{})
}

func (m *MemoryStore) CountRecentChangesFiltered(canonicalID string, since time.Time, filters ResourceChangeFilters) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	canonicalID = CanonicalResourceID(canonicalID)
	idSet := m.resourceChangeIDSetLocked(canonicalID)
	count := 0
	for _, change := range m.changes {
		if canonicalID != "" && !changeMatchesResource(change, idSet, filters.IncludeRelated) {
			continue
		}
		if !since.IsZero() && change.ObservedAt.Before(since) {
			continue
		}
		if !filters.matches(change) {
			continue
		}
		count++
	}
	return count, nil
}

func (m *MemoryStore) CountRecentChangesByKind(canonicalID string, since time.Time) (map[ChangeKind]int, error) {
	return m.CountRecentChangesByKindFiltered(canonicalID, since, ResourceChangeFilters{})
}

func (m *MemoryStore) CountRecentChangesByKindFiltered(canonicalID string, since time.Time, filters ResourceChangeFilters) (map[ChangeKind]int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	canonicalID = CanonicalResourceID(canonicalID)
	idSet := m.resourceChangeIDSetLocked(canonicalID)
	counts := make(map[ChangeKind]int)
	for _, change := range m.changes {
		if canonicalID != "" && !changeMatchesResource(change, idSet, filters.IncludeRelated) {
			continue
		}
		if !since.IsZero() && change.ObservedAt.Before(since) {
			continue
		}
		if !filters.matches(change) {
			continue
		}
		counts[change.Kind]++
	}
	if len(counts) == 0 {
		return nil, nil
	}
	return counts, nil
}

func (m *MemoryStore) CountRecentChangesBySourceType(canonicalID string, since time.Time) (map[ChangeSourceType]int, error) {
	return m.CountRecentChangesBySourceTypeFiltered(canonicalID, since, ResourceChangeFilters{})
}

func (m *MemoryStore) CountRecentChangesBySourceTypeFiltered(canonicalID string, since time.Time, filters ResourceChangeFilters) (map[ChangeSourceType]int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	canonicalID = CanonicalResourceID(canonicalID)
	idSet := m.resourceChangeIDSetLocked(canonicalID)
	counts := make(map[ChangeSourceType]int)
	for _, change := range m.changes {
		if canonicalID != "" && !changeMatchesResource(change, idSet, filters.IncludeRelated) {
			continue
		}
		if !since.IsZero() && change.ObservedAt.Before(since) {
			continue
		}
		if !filters.matches(change) {
			continue
		}
		counts[change.SourceType]++
	}
	if len(counts) == 0 {
		return nil, nil
	}
	return counts, nil
}

func (m *MemoryStore) CountRecentChangesBySourceAdapter(canonicalID string, since time.Time) (map[ChangeSourceAdapter]int, error) {
	return m.CountRecentChangesBySourceAdapterFiltered(canonicalID, since, ResourceChangeFilters{})
}

func (m *MemoryStore) CountRecentChangesBySourceAdapterFiltered(canonicalID string, since time.Time, filters ResourceChangeFilters) (map[ChangeSourceAdapter]int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	canonicalID = CanonicalResourceID(canonicalID)
	idSet := m.resourceChangeIDSetLocked(canonicalID)
	counts := make(map[ChangeSourceAdapter]int)
	for _, change := range m.changes {
		if canonicalID != "" && !changeMatchesResource(change, idSet, filters.IncludeRelated) {
			continue
		}
		if !since.IsZero() && change.ObservedAt.Before(since) {
			continue
		}
		if !filters.matches(change) {
			continue
		}
		counts[change.SourceAdapter]++
	}
	if len(counts) == 0 {
		return nil, nil
	}
	return counts, nil
}

func buildRecentChangeCountQuery(canonicalIDs []string, since time.Time, filters ResourceChangeFilters, selectClause string, observedAtExpr string, sourceTypeExpr string, sourceAdapterExpr string) (string, []any) {
	query := selectClause
	args := []any{}
	conditions := []string{observedAtExpr + " >= ?"}
	args = append(args, since)
	if len(canonicalIDs) > 0 {
		conditions, args = appendRecentChangeResourceCondition(conditions, args, canonicalIDs, filters.IncludeRelated)
	}
	if len(filters.Kinds) > 0 {
		placeholders := make([]string, 0, len(filters.Kinds))
		for _, kind := range filters.Kinds {
			placeholders = append(placeholders, "?")
			args = append(args, string(kind))
		}
		conditions = append(conditions, "kind IN ("+strings.Join(placeholders, ", ")+")")
	}
	if len(filters.SourceTypes) > 0 {
		placeholders := make([]string, 0, len(filters.SourceTypes))
		for _, sourceType := range filters.SourceTypes {
			placeholders = append(placeholders, "?")
			args = append(args, string(sourceType))
		}
		conditions = append(conditions, sourceTypeExpr+" IN ("+strings.Join(placeholders, ", ")+")")
	}
	if len(filters.SourceAdapters) > 0 {
		placeholders := make([]string, 0, len(filters.SourceAdapters))
		for _, sourceAdapter := range filters.SourceAdapters {
			placeholders = append(placeholders, "?")
			args = append(args, string(sourceAdapter))
		}
		conditions = append(conditions, sourceAdapterExpr+" IN ("+strings.Join(placeholders, ", ")+")")
	}
	query += ` WHERE ` + strings.Join(conditions, " AND ")
	return query, args
}

func appendRecentChangeResourceCondition(conditions []string, args []any, canonicalIDs []string, includeRelated bool) ([]string, []any) {
	placeholders := make([]string, len(canonicalIDs))
	for i, id := range canonicalIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	inClause := "(" + strings.Join(placeholders, ", ") + ")"
	if !includeRelated {
		return append(conditions, "canonical_id IN "+inClause), args
	}
	for _, id := range canonicalIDs {
		args = append(args, id)
	}
	return append(conditions, `(canonical_id IN `+inClause+` OR EXISTS (
			SELECT 1
			FROM json_each(CASE WHEN json_valid(resource_changes.related_resources) THEN resource_changes.related_resources ELSE '[]' END)
			WHERE TRIM(json_each.value) IN `+inClause+`
		))`), args
}

func changeMatchesResource(change ResourceChange, canonicalIDs []string, includeRelated bool) bool {
	matches := func(id string) bool {
		id = CanonicalResourceID(id)
		for _, canonicalID := range canonicalIDs {
			if id == canonicalID {
				return true
			}
		}
		return false
	}
	if matches(change.ResourceID) {
		return true
	}
	if !includeRelated {
		return false
	}
	for _, relatedID := range change.RelatedResources {
		if matches(relatedID) {
			return true
		}
	}
	return false
}

func (m *MemoryStore) CreateActionAudit(record ActionAuditRecord, initialEvents []ActionLifecycleEvent) (ActionAuditRecord, bool, error) {
	record, events, err := normalizeActionAuditCreation(record, initialEvents)
	if err != nil {
		return ActionAuditRecord{}, false, err
	}
	// Redact secret shapes from operator-authored fields and command
	// output before persisting in-memory too. The MemoryStore is used in
	// tests and contract examples; redaction must apply uniformly so test
	// fixtures cannot accidentally exercise an unredacted persistence
	// path that production never sees.
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.actionAudits {
		if m.actionAudits[i].ID == record.ID {
			current := m.actionAudits[i]
			if !ActionAuditIdentityMatches(current, record) {
				return current, false, ErrActionIdentityConflict
			}
			return current, false, nil
		}
	}
	for _, event := range events {
		for _, existing := range m.actionLifecycleEvents {
			if existing.ActionID == event.ActionID && existing.State == event.State {
				return ActionAuditRecord{}, false, fmt.Errorf("action lifecycle state %q already recorded for %q", event.State, event.ActionID)
			}
		}
	}
	m.actionAudits = append(m.actionAudits, record)
	m.actionLifecycleEvents = append(m.actionLifecycleEvents, events...)
	return record, true, nil
}

func (m *MemoryStore) RecordActionAudit(record ActionAuditRecord) error {
	_, created, err := m.CreateActionAudit(record, nil)
	if err != nil {
		return err
	}
	if !created {
		return ErrActionAuditAlreadyExists
	}
	return nil
}

func (m *MemoryStore) GetActionAudit(actionID string) (ActionAuditRecord, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	actionID = strings.TrimSpace(actionID)
	if actionID == "" {
		return ActionAuditRecord{}, false, nil
	}
	for i := len(m.actionAudits) - 1; i >= 0; i-- {
		if m.actionAudits[i].ID == actionID {
			return m.actionAudits[i], true, nil
		}
	}
	return ActionAuditRecord{}, false, nil
}

func (m *MemoryStore) GetLatestActionAuditByOrigin(surface, investigationID string) (ActionAuditRecord, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	surface = strings.TrimSpace(surface)
	investigationID = strings.TrimSpace(investigationID)
	if surface == "" || investigationID == "" {
		return ActionAuditRecord{}, false, nil
	}
	var latest ActionAuditRecord
	found := false
	for _, record := range m.actionAudits {
		if record.Origin == nil || strings.TrimSpace(record.Origin.Surface) != surface || strings.TrimSpace(record.Origin.InvestigationID) != investigationID {
			continue
		}
		if !found || record.UpdatedAt.After(latest.UpdatedAt) || (record.UpdatedAt.Equal(latest.UpdatedAt) && record.CreatedAt.After(latest.CreatedAt)) {
			latest = record
			found = true
		}
	}
	return latest, found, nil
}

func (m *MemoryStore) GetPendingActionAudits(limit int) ([]ActionAuditRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	out := make([]ActionAuditRecord, 0)
	for _, record := range m.actionAudits {
		if record.State == ActionStatePending {
			out = append(out, record)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].UpdatedAt.Before(out[j].UpdatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *MemoryStore) RecordActionDecision(record ActionAuditRecord, event ActionLifecycleEvent) error {
	normalizedRecord, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return err
	}
	normalizedEvent, err := NormalizeActionLifecycleEvent(event)
	if err != nil {
		return err
	}
	if normalizedEvent.ActionID != normalizedRecord.ID {
		return fmt.Errorf("action decision event id %q does not match action audit id %q", normalizedEvent.ActionID, normalizedRecord.ID)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	replaced := false
	for i := range m.actionAudits {
		if m.actionAudits[i].ID == normalizedRecord.ID {
			if m.actionAudits[i].State != ActionStatePending {
				return actionTransitionConflict(m.actionAudits[i], normalizedRecord, ErrActionNotPending)
			}
			if !ActionAuditIdentityMatches(m.actionAudits[i], normalizedRecord) {
				return ErrActionIdentityConflict
			}
			m.actionAudits[i] = normalizedRecord
			replaced = true
			break
		}
	}
	if !replaced {
		return fmt.Errorf("action audit %q not found", normalizedRecord.ID)
	}
	m.actionLifecycleEvents = append(m.actionLifecycleEvents, normalizedEvent)
	return nil
}

func (m *MemoryStore) RecordActionExecutionStart(record ActionAuditRecord, event ActionLifecycleEvent) error {
	normalizedRecord, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return err
	}
	normalizedEvent, err := NormalizeActionLifecycleEvent(event)
	if err != nil {
		return err
	}
	if normalizedRecord.State != ActionStateExecuting || normalizedEvent.State != ActionStateExecuting {
		return fmt.Errorf("action execution start must persist executing state")
	}
	if normalizedEvent.ActionID != normalizedRecord.ID {
		return fmt.Errorf("action execution start event id %q does not match action audit id %q", normalizedEvent.ActionID, normalizedRecord.ID)
	}
	// Redact secret shapes from operator-authored fields and command
	// output before persisting; see RecordActionAudit for the contract.
	normalizedRecord = RedactAuditRecord(normalizedRecord)

	m.mu.Lock()
	defer m.mu.Unlock()
	replaced := false
	for i := range m.actionAudits {
		if m.actionAudits[i].ID == normalizedRecord.ID {
			if !ActionAuditIdentityMatches(m.actionAudits[i], normalizedRecord) {
				return ErrActionIdentityConflict
			}
			if err := ValidateActionExecutionStart(m.actionAudits[i], normalizedEvent.Timestamp); err != nil {
				return err
			}
			m.actionAudits[i] = normalizedRecord
			replaced = true
			break
		}
	}
	if !replaced {
		return fmt.Errorf("action audit %q not found", normalizedRecord.ID)
	}
	m.actionLifecycleEvents = append(m.actionLifecycleEvents, normalizedEvent)
	return nil
}

func (m *MemoryStore) RecordActionPolicyExecutionStart(record ActionAuditRecord, approvalEvent, executionEvent ActionLifecycleEvent) error {
	record, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return err
	}
	approvalEvent, err = NormalizeActionLifecycleEvent(approvalEvent)
	if err != nil {
		return err
	}
	executionEvent, err = NormalizeActionLifecycleEvent(executionEvent)
	if err != nil {
		return err
	}
	if record.State != ActionStateExecuting || approvalEvent.State != ActionStateApproved || executionEvent.State != ActionStateExecuting || approvalEvent.ActionID != record.ID || executionEvent.ActionID != record.ID {
		return errors.New("policy admission must persist matching approved and executing events")
	}
	if len(record.Approvals) == 0 || record.Approvals[len(record.Approvals)-1].Method != MethodPolicy || record.Approvals[len(record.Approvals)-1].PolicyLease == nil {
		return ErrActionPolicyAuthorizationInvalid
	}
	record = RedactAuditRecord(record)
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.actionAudits {
		if m.actionAudits[i].ID != record.ID {
			continue
		}
		current := m.actionAudits[i]
		if !ActionAuditIdentityMatches(current, record) {
			return ErrActionIdentityConflict
		}
		if current.State != ActionStatePlanned && current.State != ActionStatePending {
			return actionTransitionConflict(current, record, ErrActionNotApproved)
		}
		m.actionAudits[i] = record
		m.actionLifecycleEvents = append(m.actionLifecycleEvents, approvalEvent, executionEvent)
		return nil
	}
	return fmt.Errorf("action audit %q not found", record.ID)
}

func (m *MemoryStore) RecordActionExecutionResult(record ActionAuditRecord, event ActionLifecycleEvent) error {
	normalizedRecord, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return err
	}
	normalizedEvent, err := NormalizeActionLifecycleEvent(event)
	if err != nil {
		return err
	}
	if normalizedRecord.State != ActionStateCompleted && normalizedRecord.State != ActionStateFailed {
		return fmt.Errorf("action execution result must persist terminal execution state")
	}
	if normalizedEvent.State != normalizedRecord.State {
		return fmt.Errorf("action execution result event state %q does not match audit state %q", normalizedEvent.State, normalizedRecord.State)
	}
	if normalizedEvent.ActionID != normalizedRecord.ID {
		return fmt.Errorf("action execution result event id %q does not match action audit id %q", normalizedEvent.ActionID, normalizedRecord.ID)
	}
	// Redact secret shapes from operator-authored fields and command
	// output before persisting; see RecordActionAudit for the contract.
	normalizedRecord = RedactAuditRecord(normalizedRecord)

	m.mu.Lock()
	defer m.mu.Unlock()
	replaced := false
	for i := range m.actionAudits {
		if m.actionAudits[i].ID == normalizedRecord.ID {
			if m.actionAudits[i].State != ActionStateExecuting {
				return actionTransitionConflict(m.actionAudits[i], normalizedRecord, ErrActionNotExecuting)
			}
			if !ActionAuditIdentityMatches(m.actionAudits[i], normalizedRecord) {
				return ErrActionIdentityConflict
			}
			m.actionAudits[i] = normalizedRecord
			replaced = true
			break
		}
	}
	if !replaced {
		return fmt.Errorf("action audit %q not found", normalizedRecord.ID)
	}
	m.actionLifecycleEvents = append(m.actionLifecycleEvents, normalizedEvent)
	return nil
}

func (m *MemoryStore) RecordActionExecutionRefusal(record ActionAuditRecord, event ActionLifecycleEvent) error {
	normalizedRecord, err := NormalizeActionAuditRecord(record)
	if err != nil {
		return err
	}
	normalizedEvent, err := NormalizeActionLifecycleEvent(event)
	if err != nil {
		return err
	}
	if normalizedRecord.State != ActionStateFailed || normalizedEvent.State != ActionStateFailed || normalizedEvent.ActionID != normalizedRecord.ID {
		return fmt.Errorf("action execution refusal must persist matching failed state")
	}
	normalizedRecord = RedactAuditRecord(normalizedRecord)
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.actionAudits {
		if m.actionAudits[i].ID != normalizedRecord.ID {
			continue
		}
		current := m.actionAudits[i]
		if !ActionAuditIdentityMatches(current, normalizedRecord) {
			return ErrActionIdentityConflict
		}
		switch current.State {
		case ActionStatePlanned, ActionStatePending, ActionStateApproved:
			m.actionAudits[i] = normalizedRecord
			m.actionLifecycleEvents = append(m.actionLifecycleEvents, normalizedEvent)
			return nil
		default:
			return actionTransitionConflict(current, normalizedRecord, ErrActionExecutionFinal)
		}
	}
	return fmt.Errorf("action audit %q not found", normalizedRecord.ID)
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
	normalized, err := NormalizeActionLifecycleEvent(event)
	if err != nil {
		return err
	}
	event = normalized

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.actionLifecycleEvents {
		if existing.ActionID == event.ActionID && existing.State == event.State {
			return fmt.Errorf("action lifecycle state %q already recorded for %q", event.State, event.ActionID)
		}
	}
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

// GetResourceOperatorState returns the operator-set state for the given
// canonical resource ID. The found bool distinguishes "no entry" (the
// default no-state posture) from "explicit empty entry the operator
// cleared" — both have the same effective behavior but the latter may
// be surfaced differently on the audit timeline.
func (m *MemoryStore) GetResourceOperatorState(canonicalID string) (ResourceOperatorState, bool, error) {
	canonicalID = strings.TrimSpace(canonicalID)
	if canonicalID == "" {
		return ResourceOperatorState{}, false, nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.resourceOperatorState[canonicalID]
	if !ok {
		return ResourceOperatorState{}, false, nil
	}
	return state, true, nil
}

// SetResourceOperatorState upserts the supplied state, validating it
// before persistence. Callers should pass a fully-populated record;
// per-field merge is not supported, so reading the existing state and
// modifying it is the operator's responsibility.
func (m *MemoryStore) SetResourceOperatorState(state ResourceOperatorState) error {
	state = NormalizeResourceOperatorState(state)
	if err := ValidateResourceOperatorState(state); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.resourceOperatorState == nil {
		m.resourceOperatorState = make(map[string]ResourceOperatorState)
	}
	m.resourceOperatorState[state.CanonicalID] = state
	return nil
}

// SetResourceOperatorStateWithMaintenanceLifecycle updates the in-memory
// source row and derived timeline projection under one lock.
func (m *MemoryStore) SetResourceOperatorStateWithMaintenanceLifecycle(state ResourceOperatorState) (ResourceOperatorState, error) {
	state = NormalizeResourceOperatorState(state)
	if err := ValidateResourceOperatorState(state); err != nil {
		return ResourceOperatorState{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.resourceOperatorState == nil {
		m.resourceOperatorState = make(map[string]ResourceOperatorState)
	}
	previous, previousFound := m.resourceOperatorState[state.CanonicalID]
	m.resourceOperatorState[state.CanonicalID] = state
	if change, ok := BuildMaintenanceWindowLifecycleChange(previous, previousFound, state, true, state.SetAt, state.SetBy); ok {
		if err := m.recordChangeLocked(change); err != nil {
			return ResourceOperatorState{}, fmt.Errorf("record maintenance window lifecycle change: %w", err)
		}
	}
	return state, nil
}

// ClearResourceOperatorState removes any operator-set state for the
// given canonical ID. Returns nil whether or not an entry was present —
// the operation is idempotent so the API surface can issue
// fire-and-forget DELETE without round-tripping the existence check.
func (m *MemoryStore) ClearResourceOperatorState(canonicalID string) error {
	canonicalID = strings.TrimSpace(canonicalID)
	if canonicalID == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.resourceOperatorState, canonicalID)
	return nil
}

// ClearResourceOperatorStateWithMaintenanceLifecycle clears the in-memory
// source row and derived timeline projection under one lock.
func (m *MemoryStore) ClearResourceOperatorStateWithMaintenanceLifecycle(canonicalID string, observedAt time.Time, actor string) error {
	canonicalID = strings.TrimSpace(canonicalID)
	if canonicalID == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	previous, previousFound := m.resourceOperatorState[canonicalID]
	delete(m.resourceOperatorState, canonicalID)
	current := ResourceOperatorState{CanonicalID: canonicalID}
	if change, ok := BuildMaintenanceWindowLifecycleChange(previous, previousFound, current, false, observedAt, actor); ok {
		if err := m.recordChangeLocked(change); err != nil {
			return fmt.Errorf("record maintenance window lifecycle change: %w", err)
		}
	}
	return nil
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
