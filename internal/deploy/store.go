package deploy

import (
	"context"
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

// Store provides SQLite-backed persistence for deployment jobs, targets, and events.
type Store struct {
	db *sql.DB
}

// Open creates or opens the deploy SQLite database at dbPath.
func Open(dbPath string) (*Store, error) {
	dbPath = filepath.Clean(strings.TrimSpace(dbPath))
	if dbPath == "" {
		return nil, fmt.Errorf("deploy db path is required")
	}

	dir := filepath.Dir(dbPath)
	if err := ensureOwnerOnlyDir(dir); err != nil {
		return nil, fmt.Errorf("failed to create deploy db directory: %w", err)
	}

	if err := rejectSymlinkOrNonRegular(dbPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	dsn := dbPath + "?" + url.Values{
		"_pragma": {
			"busy_timeout(30000)",
			"journal_mode(WAL)",
			"synchronous(NORMAL)",
			"foreign_keys(ON)",
		},
	}.Encode()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open deploy db: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		initErr := fmt.Errorf("initialize deploy schema: %w", err)
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(initErr, closeErr)
		}
		return nil, initErr
	}

	if err := hardenSQLiteArtifacts(dbPath); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to secure deploy db files: %w", err)
	}

	return s, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) initSchema() error {
	schema := `
CREATE TABLE IF NOT EXISTS deploy_jobs (
    id             TEXT PRIMARY KEY,
    cluster_id     TEXT NOT NULL,
    cluster_name   TEXT NOT NULL,
    source_agent_id TEXT NOT NULL,
    source_node_id TEXT NOT NULL,
    org_id         TEXT NOT NULL DEFAULT 'default',
    status         TEXT NOT NULL,
    max_parallel   INTEGER NOT NULL DEFAULT 2,
    retry_max      INTEGER NOT NULL DEFAULT 3,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    completed_at   TEXT
);

CREATE TABLE IF NOT EXISTS deploy_targets (
    id            TEXT PRIMARY KEY,
    job_id        TEXT NOT NULL REFERENCES deploy_jobs(id) ON DELETE CASCADE,
    node_id       TEXT NOT NULL,
    node_name     TEXT NOT NULL,
    node_ip       TEXT NOT NULL,
    arch          TEXT,
    status        TEXT NOT NULL,
    error_message TEXT,
    attempts      INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS deploy_events (
    id         TEXT PRIMARY KEY,
    job_id     TEXT NOT NULL REFERENCES deploy_jobs(id) ON DELETE CASCADE,
    target_id  TEXT,
    type       TEXT NOT NULL,
    message    TEXT NOT NULL,
    data       TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_deploy_targets_job ON deploy_targets(job_id);
CREATE INDEX IF NOT EXISTS idx_deploy_targets_job_created ON deploy_targets(job_id, created_at);
CREATE INDEX IF NOT EXISTS idx_deploy_events_job ON deploy_events(job_id);
CREATE INDEX IF NOT EXISTS idx_deploy_events_job_created ON deploy_events(job_id, created_at);
CREATE INDEX IF NOT EXISTS idx_deploy_events_target ON deploy_events(target_id);
CREATE INDEX IF NOT EXISTS idx_deploy_events_target_created ON deploy_events(target_id, created_at);
CREATE INDEX IF NOT EXISTS idx_deploy_jobs_org ON deploy_jobs(org_id);
CREATE INDEX IF NOT EXISTS idx_deploy_jobs_org_created ON deploy_jobs(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_deploy_jobs_status ON deploy_jobs(status);
CREATE INDEX IF NOT EXISTS idx_deploy_jobs_completed ON deploy_jobs(completed_at);
`
	_, err := s.db.Exec(schema)
	return err
}

// --- Jobs ---

// CreateJob inserts a new deployment job.
func (s *Store) CreateJob(ctx context.Context, job *Job) error {
	var completedAt *string
	if job.CompletedAt != nil {
		v := job.CompletedAt.UTC().Format(time.RFC3339)
		completedAt = &v
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO deploy_jobs (id, cluster_id, cluster_name, source_agent_id, source_node_id, org_id, status, max_parallel, retry_max, created_at, updated_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.ClusterID, job.ClusterName, job.SourceAgentID, job.SourceNodeID,
		job.OrgID, string(job.Status), job.MaxParallel, job.RetryMax,
		job.CreatedAt.UTC().Format(time.RFC3339), job.UpdatedAt.UTC().Format(time.RFC3339),
		completedAt,
	)
	return err
}

// GetJob retrieves a deployment job by ID.
func (s *Store) GetJob(ctx context.Context, id string) (*Job, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, cluster_id, cluster_name, source_agent_id, source_node_id, org_id, status, max_parallel, retry_max, created_at, updated_at, completed_at
		 FROM deploy_jobs WHERE id = ?`, id)
	return scanJob(row)
}

// UpdateJobStatus sets the status (and completed_at for terminal states) of a job.
func (s *Store) UpdateJobStatus(ctx context.Context, id string, status JobStatus) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var completedAt *string
	if isJobTerminal(status) {
		completedAt = &now
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE deploy_jobs SET status = ?, updated_at = ?, completed_at = COALESCE(?, completed_at) WHERE id = ?`,
		string(status), now, completedAt, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("deploy job %q not found", id)
	}
	return nil
}

// ListJobs returns the most recent jobs for an org, ordered by creation time descending.
func (s *Store) ListJobs(ctx context.Context, orgID string, limit int) ([]Job, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, cluster_id, cluster_name, source_agent_id, source_node_id, org_id, status, max_parallel, retry_max, created_at, updated_at, completed_at
		 FROM deploy_jobs WHERE org_id = ? ORDER BY created_at DESC LIMIT ?`, orgID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		j, err := scanJobRow(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// --- Targets ---

// GetTarget retrieves a single deployment target by ID.
func (s *Store) GetTarget(ctx context.Context, targetID string) (*Target, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, job_id, node_id, node_name, node_ip, arch, status, error_message, attempts, created_at, updated_at
		 FROM deploy_targets WHERE id = ?`, targetID)
	var t Target
	var status string
	var arch, errMsg sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(&t.ID, &t.JobID, &t.NodeID, &t.NodeName, &t.NodeIP,
		&arch, &status, &errMsg, &t.Attempts, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.Status = TargetStatus(status)
	t.Arch = arch.String
	t.ErrorMessage = errMsg.String
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &t, nil
}

// CreateTarget inserts a new deployment target.
func (s *Store) CreateTarget(ctx context.Context, target *Target) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO deploy_targets (id, job_id, node_id, node_name, node_ip, arch, status, error_message, attempts, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		target.ID, target.JobID, target.NodeID, target.NodeName, target.NodeIP,
		nilIfEmpty(target.Arch), string(target.Status), nilIfEmpty(target.ErrorMessage),
		target.Attempts,
		target.CreatedAt.UTC().Format(time.RFC3339), target.UpdatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// GetTargetsForJob returns all targets for a given job.
func (s *Store) GetTargetsForJob(ctx context.Context, jobID string) ([]Target, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, job_id, node_id, node_name, node_ip, arch, status, error_message, attempts, created_at, updated_at
		 FROM deploy_targets WHERE job_id = ? ORDER BY created_at ASC`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []Target
	for rows.Next() {
		t, err := scanTargetRow(rows)
		if err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

// UpdateTargetStatus sets the status and optional error message of a target.
func (s *Store) UpdateTargetStatus(ctx context.Context, id string, status TargetStatus, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		`UPDATE deploy_targets SET status = ?, error_message = ?, updated_at = ? WHERE id = ?`,
		string(status), nilIfEmpty(errMsg), now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("deploy target %q not found", id)
	}
	return nil
}

// IncrementTargetAttempts bumps the attempt counter for a target.
func (s *Store) IncrementTargetAttempts(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		`UPDATE deploy_targets SET attempts = attempts + 1, updated_at = ? WHERE id = ?`, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("deploy target %q not found", id)
	}
	return nil
}

// UpdateTargetArch sets the architecture discovered during preflight for a target.
func (s *Store) UpdateTargetArch(ctx context.Context, id, arch string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		`UPDATE deploy_targets SET arch = ?, updated_at = ? WHERE id = ?`,
		nilIfEmpty(arch), now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("deploy target %q not found", id)
	}
	return nil
}

// ResetTargetsForRetry resets targets in failed states back to pending.
// Attempt counters are NOT incremented here — they are incremented by
// updateTargetFromInstallProgress when the next failure occurs. Returns the
// number of targets actually reset.
func (s *Store) ResetTargetsForRetry(ctx context.Context, targetIDs []string) (int, error) {
	if len(targetIDs) == 0 {
		return 0, nil
	}
	now := time.Now().UTC().Format(time.RFC3339)

	// Build placeholders and args for IN clause.
	placeholders := make([]string, len(targetIDs))
	args := make([]any, 0, len(targetIDs)+1)
	args = append(args, now)
	for i, id := range targetIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := fmt.Sprintf(
		`UPDATE deploy_targets SET status = 'pending', error_message = NULL, updated_at = ?
		 WHERE id IN (%s) AND status IN ('failed_retryable', 'failed_permanent')`,
		strings.Join(placeholders, ","))

	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// --- Events ---

// AppendEvent inserts an immutable audit event.
func (s *Store) AppendEvent(ctx context.Context, event *Event) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO deploy_events (id, job_id, target_id, type, message, data, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.JobID, nilIfEmpty(event.TargetID),
		string(event.Type), event.Message, nilIfEmpty(event.Data),
		event.CreatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// GetEventsForJob returns all events for a given job, ordered chronologically.
func (s *Store) GetEventsForJob(ctx context.Context, jobID string) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, job_id, target_id, type, message, data, created_at
		 FROM deploy_events WHERE job_id = ? ORDER BY created_at ASC`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		e, err := scanEventRow(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetEventsForTarget returns all events for a given target, ordered chronologically.
func (s *Store) GetEventsForTarget(ctx context.Context, targetID string) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, job_id, target_id, type, message, data, created_at
		 FROM deploy_events WHERE target_id = ? ORDER BY created_at ASC`, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		e, err := scanEventRow(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// --- Cleanup ---

// PruneOldJobs deletes completed jobs older than the given duration, cascading to targets and events.
func (s *Store) PruneOldJobs(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-olderThan).Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM deploy_jobs WHERE completed_at IS NOT NULL AND completed_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// --- scan helpers ---

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (*Job, error) {
	var j Job
	var status string
	var createdAt, updatedAt string
	var completedAt sql.NullString
	if err := row.Scan(&j.ID, &j.ClusterID, &j.ClusterName, &j.SourceAgentID, &j.SourceNodeID,
		&j.OrgID, &status, &j.MaxParallel, &j.RetryMax, &createdAt, &updatedAt, &completedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	j.Status = JobStatus(status)
	j.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	j.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		j.CompletedAt = &t
	}
	return &j, nil
}

func scanJobRow(rows *sql.Rows) (Job, error) {
	j, err := scanJob(rows)
	if err != nil {
		return Job{}, err
	}
	if j == nil {
		return Job{}, fmt.Errorf("unexpected nil job from row scan")
	}
	return *j, nil
}

func scanTargetRow(rows *sql.Rows) (Target, error) {
	var t Target
	var status string
	var arch, errMsg sql.NullString
	var createdAt, updatedAt string
	if err := rows.Scan(&t.ID, &t.JobID, &t.NodeID, &t.NodeName, &t.NodeIP,
		&arch, &status, &errMsg, &t.Attempts, &createdAt, &updatedAt); err != nil {
		return Target{}, err
	}
	t.Status = TargetStatus(status)
	t.Arch = arch.String
	t.ErrorMessage = errMsg.String
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return t, nil
}

func scanEventRow(rows *sql.Rows) (Event, error) {
	var e Event
	var typ string
	var targetID, data sql.NullString
	var createdAt string
	if err := rows.Scan(&e.ID, &e.JobID, &targetID, &typ, &e.Message, &data, &createdAt); err != nil {
		return Event{}, err
	}
	e.Type = EventType(typ)
	e.TargetID = targetID.String
	e.Data = data.String
	e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return e, nil
}

// --- filesystem helpers (matching recovery store pattern) ---

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
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return os.Chmod(path, privateFilePerm)
}

func hardenSQLiteArtifacts(dbPath string) error {
	for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if err := hardenSQLiteFile(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
	}
	return nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
