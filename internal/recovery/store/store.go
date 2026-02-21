package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rs/zerolog/log"

	_ "modernc.org/sqlite"
)

const (
	privateDirPerm  = 0o700
	privateFilePerm = 0o600

	defaultLimit = 100
	maxLimit     = 500

	defaultRetention = 90 * 24 * time.Hour
)

// Store persists recovery points in a per-tenant SQLite database.
// It is intentionally separate from unified resources (which are in-memory).
type Store struct {
	db        *sql.DB
	retention time.Duration

	pruneMu   sync.Mutex
	lastPrune time.Time
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

func Open(dbPath string) (*Store, error) {
	dbPath = filepath.Clean(strings.TrimSpace(dbPath))
	if dbPath == "" {
		return nil, fmt.Errorf("recovery db path is required")
	}

	dir := filepath.Dir(dbPath)
	if err := ensureOwnerOnlyDir(dir); err != nil {
		return nil, fmt.Errorf("failed to create recovery db directory: %w", err)
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
		return nil, fmt.Errorf("failed to open recovery db: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	store := &Store{db: db, retention: defaultRetention}
	if err := store.initSchema(); err != nil {
		initErr := fmt.Errorf("initialize recovery schema: %w", err)
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(initErr, fmt.Errorf("close recovery db after init failure: %w", closeErr))
		}
		return nil, initErr
	}
	// Best-effort backfill for deployments that wrote points before subject_key existed.
	// This should never block startup; missing backfills only affect rollup completeness.
	_ = store.BackfillKeys(context.Background())
	// Best-effort backfill for deployments that wrote points before index columns existed.
	// This should never block startup; missing backfills only affect filter UX.
	_ = store.BackfillIndex(context.Background())
	if err := hardenSQLiteArtifacts(dbPath); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to secure recovery db files: %w", err)
	}

	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) ensureInitialized() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("recovery store is not initialized")
	}
	return nil
}

// PurgeStalePVEPBSBackups removes legacy PVE-proxied PBS backup entries.
// It is safe to call repeatedly.
func (s *Store) PurgeStalePVEPBSBackups(ctx context.Context) error {
	if err := s.ensureInitialized(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	res, err := s.db.ExecContext(ctx, `
		DELETE FROM recovery_points
		WHERE id LIKE 'pve-backup:%'
		  AND json_extract(details_json, '$.isPBS') = 1
	`)
	if err != nil {
		return err
	}

	deleted, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if deleted > 0 {
		log.Info().
			Int64("deleted", deleted).
			Msg("Purged stale PVE-sourced PBS backup entries - PBS direct is now authoritative")
	}

	return nil
}

func (s *Store) initSchema() error {
	if err := s.ensureInitialized(); err != nil {
		return err
	}

	schema := `
		CREATE TABLE IF NOT EXISTS recovery_points (
			id TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			kind TEXT NOT NULL,
			mode TEXT NOT NULL,
			outcome TEXT NOT NULL,
			started_at_ms INTEGER,
			completed_at_ms INTEGER,
			size_bytes INTEGER,
			verified INTEGER,
			encrypted INTEGER,
			immutable INTEGER,
			subject_key TEXT,
			repository_key TEXT,
			subject_resource_id TEXT,
			repository_resource_id TEXT,
			subject_ref_json TEXT,
			repository_ref_json TEXT,
			details_json TEXT,
			subject_label TEXT,
			subject_type TEXT,
			is_workload INTEGER,
			cluster_label TEXT,
			node_host_label TEXT,
			namespace_label TEXT,
			entity_id_label TEXT,
			repository_label TEXT,
			details_summary TEXT,
			created_at_ms INTEGER NOT NULL,
			updated_at_ms INTEGER NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_recovery_points_completed
		ON recovery_points(completed_at_ms);

		CREATE INDEX IF NOT EXISTS idx_recovery_points_provider_completed
		ON recovery_points(provider, completed_at_ms);

		CREATE INDEX IF NOT EXISTS idx_recovery_points_subject_completed
		ON recovery_points(subject_resource_id, completed_at_ms);

		CREATE INDEX IF NOT EXISTS idx_recovery_points_subject_key_completed
		ON recovery_points(subject_key, completed_at_ms);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Migration: subject_key/repository_key were added after initial rollout.
	if err := s.ensureColumn("recovery_points", "subject_key", "TEXT"); err != nil {
		return err
	}
	if err := s.ensureColumn("recovery_points", "repository_key", "TEXT"); err != nil {
		return err
	}

	// Migration: normalized index columns for efficient filtering/searching and consistent UI display.
	for _, col := range []struct {
		name string
		typ  string
	}{
		{"subject_label", "TEXT"},
		{"subject_type", "TEXT"},
		{"is_workload", "INTEGER"},
		{"cluster_label", "TEXT"},
		{"node_host_label", "TEXT"},
		{"namespace_label", "TEXT"},
		{"entity_id_label", "TEXT"},
		{"repository_label", "TEXT"},
		{"details_summary", "TEXT"},
	} {
		if err := s.ensureColumn("recovery_points", col.name, col.typ); err != nil {
			return err
		}
	}

	// Indexes on migrated columns — must run AFTER ensureColumn so pre-existing
	// databases that lack these columns don't crash with "no such column".
	postMigrationIndexes := `
		CREATE INDEX IF NOT EXISTS idx_recovery_points_cluster_completed
		ON recovery_points(cluster_label, completed_at_ms);

		CREATE INDEX IF NOT EXISTS idx_recovery_points_node_completed
		ON recovery_points(node_host_label, completed_at_ms);

		CREATE INDEX IF NOT EXISTS idx_recovery_points_namespace_completed
		ON recovery_points(namespace_label, completed_at_ms);
	`
	if _, err := s.db.Exec(postMigrationIndexes); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureColumn(table string, column string, typeSQL string) error {
	if err := s.ensureInitialized(); err != nil {
		return err
	}
	table = strings.TrimSpace(table)
	column = strings.TrimSpace(column)
	typeSQL = strings.TrimSpace(typeSQL)
	if table == "" || column == "" || typeSQL == "" {
		return fmt.Errorf("invalid column migration: table=%q column=%q type=%q", table, column, typeSQL)
	}

	rows, err := s.db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			colType    string
			notNull    int
			dfltValue  sql.NullString
			isPKColumn int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &isPKColumn); err != nil {
			return err
		}
		if strings.EqualFold(strings.TrimSpace(name), column) {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = s.db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + typeSQL)
	return err
}

// BackfillKeys populates subject_key and repository_key for rows that predate those columns.
// It is safe to call multiple times.
func (s *Store) BackfillKeys(ctx context.Context) error {
	if err := s.ensureInitialized(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// Fast path: resource-linked keys can be derived directly in SQL.
	_, _ = s.db.ExecContext(ctx, `
		UPDATE recovery_points
		SET subject_key = 'res:' || subject_resource_id
		WHERE (subject_key IS NULL OR subject_key = '')
		  AND subject_resource_id IS NOT NULL
		  AND TRIM(subject_resource_id) != ''
	`)
	_, _ = s.db.ExecContext(ctx, `
		UPDATE recovery_points
		SET repository_key = 'res:' || repository_resource_id
		WHERE (repository_key IS NULL OR repository_key = '')
		  AND repository_resource_id IS NOT NULL
		  AND TRIM(repository_resource_id) != ''
	`)

	// External refs require JSON decode + hashing in Go. Keep it bounded.
	const maxBackfillRows = 2000

	backfillExternal := func(selectSQL string, targetCol string, keyFn func(provider recovery.Provider, rid string, ref *recovery.ExternalRef) string) error {
		rows, err := s.db.QueryContext(ctx, selectSQL)
		if err != nil {
			return err
		}
		defer rows.Close()

		type row struct {
			id       string
			provider string
			rid      string
			refRaw   sql.NullString
		}

		items := make([]row, 0, 256)
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.id, &r.provider, &r.rid, &r.refRaw); err != nil {
				return err
			}
			items = append(items, r)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}

		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()

		stmt, err := tx.PrepareContext(ctx, "UPDATE recovery_points SET "+targetCol+" = ? WHERE id = ?")
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, item := range items {
			var ref recovery.ExternalRef
			_ = unmarshalJSON(item.refRaw, &ref) // best-effort
			var refPtr *recovery.ExternalRef
			if strings.TrimSpace(ref.Type) != "" {
				refPtr = &ref
			}

			key := keyFn(recovery.Provider(strings.TrimSpace(item.provider)), strings.TrimSpace(item.rid), refPtr)
			if strings.TrimSpace(key) == "" {
				continue
			}
			if _, err := stmt.ExecContext(ctx, key, item.id); err != nil {
				return err
			}
		}

		if err := tx.Commit(); err != nil {
			return err
		}
		return nil
	}

	_ = backfillExternal(
		`
			SELECT id, provider, subject_resource_id, subject_ref_json
			FROM recovery_points
			WHERE (subject_key IS NULL OR subject_key = '')
			LIMIT `+fmt.Sprint(maxBackfillRows)+`
		`,
		"subject_key",
		func(provider recovery.Provider, rid string, ref *recovery.ExternalRef) string {
			return recovery.SubjectKey(provider, rid, ref)
		},
	)

	_ = backfillExternal(
		`
			SELECT id, provider, repository_resource_id, repository_ref_json
			FROM recovery_points
			WHERE (repository_key IS NULL OR repository_key = '')
			LIMIT `+fmt.Sprint(maxBackfillRows)+`
		`,
		"repository_key",
		func(provider recovery.Provider, rid string, ref *recovery.ExternalRef) string {
			return recovery.RepositoryKey(provider, rid, ref)
		},
	)

	return nil
}

func boolPtrToDB(v *bool) any {
	if v == nil {
		return nil
	}
	if *v {
		return 1
	}
	return 0
}

func dbToBoolPtr(v sql.NullInt64) *bool {
	if !v.Valid {
		return nil
	}
	b := v.Int64 != 0
	return &b
}

func timePtrToMillis(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC().UnixMilli()
}

func millisToTimePtr(v sql.NullInt64) *time.Time {
	if !v.Valid || v.Int64 <= 0 {
		return nil
	}
	t := time.UnixMilli(v.Int64).UTC()
	return &t
}

func marshalJSON(v any) (sql.NullString, error) {
	if v == nil {
		return sql.NullString{}, nil
	}
	switch typed := v.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return sql.NullString{}, nil
		}
		return sql.NullString{String: typed, Valid: true}, nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return sql.NullString{}, err
		}
		if len(b) == 0 {
			return sql.NullString{}, nil
		}
		return sql.NullString{String: string(b), Valid: true}, nil
	}
}

func unmarshalJSON[T any](raw sql.NullString, target *T) error {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw.String), target)
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func normalizePage(page int) int {
	if page <= 0 {
		return 1
	}
	return page
}

// UpsertPoints inserts or updates recovery points by ID.
func (s *Store) UpsertPoints(ctx context.Context, points []recovery.RecoveryPoint) error {
	if err := s.ensureInitialized(); err != nil {
		return err
	}
	if len(points) == 0 {
		return nil
	}

	s.maybePrune(ctx)

	nowMs := time.Now().UTC().UnixMilli()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO recovery_points (
				id, provider, kind, mode, outcome,
				started_at_ms, completed_at_ms, size_bytes,
				verified, encrypted, immutable,
				subject_key, repository_key,
				subject_resource_id, repository_resource_id,
				subject_ref_json, repository_ref_json, details_json,
				subject_label, subject_type, is_workload,
				cluster_label, node_host_label, namespace_label, entity_id_label,
				repository_label, details_summary,
				created_at_ms, updated_at_ms
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				provider=excluded.provider,
				kind=excluded.kind,
				mode=excluded.mode,
				outcome=excluded.outcome,
				started_at_ms=excluded.started_at_ms,
				completed_at_ms=excluded.completed_at_ms,
				size_bytes=excluded.size_bytes,
				verified=excluded.verified,
				encrypted=excluded.encrypted,
				immutable=excluded.immutable,
				subject_key=excluded.subject_key,
				repository_key=excluded.repository_key,
				subject_resource_id=excluded.subject_resource_id,
				repository_resource_id=excluded.repository_resource_id,
				subject_ref_json=excluded.subject_ref_json,
				repository_ref_json=excluded.repository_ref_json,
				details_json=excluded.details_json,
				subject_label=excluded.subject_label,
				subject_type=excluded.subject_type,
				is_workload=excluded.is_workload,
				cluster_label=excluded.cluster_label,
				node_host_label=excluded.node_host_label,
				namespace_label=excluded.namespace_label,
				entity_id_label=excluded.entity_id_label,
				repository_label=excluded.repository_label,
				details_summary=excluded.details_summary,
				updated_at_ms=excluded.updated_at_ms
		`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range points {
		id := strings.TrimSpace(p.ID)
		if id == "" {
			continue
		}
		provider := strings.TrimSpace(string(p.Provider))
		kind := strings.TrimSpace(string(p.Kind))
		mode := strings.TrimSpace(string(p.Mode))
		outcome := strings.TrimSpace(string(p.Outcome))
		if provider == "" || kind == "" || mode == "" || outcome == "" {
			continue
		}

		subjectRef, err := marshalJSON(p.SubjectRef)
		if err != nil {
			return err
		}
		repoRef, err := marshalJSON(p.RepositoryRef)
		if err != nil {
			return err
		}
		details, err := marshalJSON(p.Details)
		if err != nil {
			return err
		}

		var size any
		if p.SizeBytes != nil {
			size = *p.SizeBytes
		}

		subjectRID := strings.TrimSpace(p.SubjectResourceID)
		repoRID := strings.TrimSpace(p.RepositoryResourceID)
		subjectKey := recovery.SubjectKey(p.Provider, subjectRID, p.SubjectRef)
		repoKey := recovery.RepositoryKey(p.Provider, repoRID, p.RepositoryRef)
		idx := recovery.DeriveIndex(p)

		if _, err := stmt.ExecContext(
			ctx,
			id,
			provider,
			kind,
			mode,
			outcome,
			timePtrToMillis(p.StartedAt),
			timePtrToMillis(p.CompletedAt),
			size,
			boolPtrToDB(p.Verified),
			boolPtrToDB(p.Encrypted),
			boolPtrToDB(p.Immutable),
			strings.TrimSpace(subjectKey),
			strings.TrimSpace(repoKey),
			subjectRID,
			repoRID,
			nullStringToAny(subjectRef),
			nullStringToAny(repoRef),
			nullStringToAny(details),
			strings.TrimSpace(idx.SubjectLabel),
			strings.TrimSpace(idx.SubjectType),
			func() any {
				if idx.IsWorkload {
					return 1
				}
				// Persist 0 rather than NULL so filters are deterministic.
				return 0
			}(),
			strings.TrimSpace(idx.ClusterLabel),
			strings.TrimSpace(idx.NodeHostLabel),
			strings.TrimSpace(idx.NamespaceLabel),
			strings.TrimSpace(idx.EntityIDLabel),
			strings.TrimSpace(idx.RepositoryLabel),
			strings.TrimSpace(idx.DetailsSummary),
			nowMs,
			nowMs,
		); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) maybePrune(ctx context.Context) {
	if s == nil || s.db == nil || s.retention <= 0 {
		return
	}

	// Keep pruning overhead low: at most once per hour per store.
	const minInterval = 1 * time.Hour

	s.pruneMu.Lock()
	defer s.pruneMu.Unlock()

	now := time.Now().UTC()
	if !s.lastPrune.IsZero() && now.Sub(s.lastPrune) < minInterval {
		return
	}
	s.lastPrune = now

	cutoffMs := now.Add(-s.retention).UnixMilli()
	// Best-effort cleanup; never block writes if pruning fails.
	_, _ = s.db.ExecContext(ctx, `
		DELETE FROM recovery_points
		WHERE COALESCE(completed_at_ms, started_at_ms, updated_at_ms) < ?
	`, cutoffMs)
}

func nullStringToAny(s sql.NullString) any {
	if !s.Valid {
		return nil
	}
	return s.String
}

// ListPoints returns recovery points matching filters, ordered by completed_at desc (NULLS last).
func (s *Store) ListPoints(ctx context.Context, opts recovery.ListPointsOptions) ([]recovery.RecoveryPoint, int, error) {
	if err := s.ensureInitialized(); err != nil {
		return nil, 0, err
	}

	limit := normalizeLimit(opts.Limit)
	page := normalizePage(opts.Page)
	offset := (page - 1) * limit

	where := make([]string, 0, 8)
	args := make([]any, 0, 16)

	if strings.TrimSpace(string(opts.Provider)) != "" {
		where = append(where, "provider = ?")
		args = append(args, string(opts.Provider))
	}
	if strings.TrimSpace(string(opts.Kind)) != "" {
		where = append(where, "kind = ?")
		args = append(args, string(opts.Kind))
	}
	if strings.TrimSpace(string(opts.Mode)) != "" {
		where = append(where, "mode = ?")
		args = append(args, string(opts.Mode))
	}
	if strings.TrimSpace(string(opts.Outcome)) != "" {
		where = append(where, "outcome = ?")
		args = append(args, string(opts.Outcome))
	}
	if strings.TrimSpace(opts.RollupID) != "" {
		rid := strings.TrimSpace(opts.RollupID)
		// Accept raw unified resource IDs by mapping them to the rollup key format.
		if !strings.HasPrefix(rid, "res:") && !strings.HasPrefix(rid, "ext:") {
			rid = "res:" + rid
		}
		where = append(where, "subject_key = ?")
		args = append(args, rid)
	} else if strings.TrimSpace(opts.SubjectResourceID) != "" {
		where = append(where, "subject_resource_id = ?")
		args = append(args, strings.TrimSpace(opts.SubjectResourceID))
	}
	if opts.From != nil && !opts.From.IsZero() {
		where = append(where, "(completed_at_ms IS NULL OR completed_at_ms >= ?)")
		args = append(args, opts.From.UTC().UnixMilli())
	}
	if opts.To != nil && !opts.To.IsZero() {
		where = append(where, "(completed_at_ms IS NULL OR completed_at_ms <= ?)")
		args = append(args, opts.To.UTC().UnixMilli())
	}
	if strings.TrimSpace(opts.ClusterLabel) != "" {
		where = append(where, "cluster_label = ?")
		args = append(args, strings.TrimSpace(opts.ClusterLabel))
	}
	if strings.TrimSpace(opts.NodeHostLabel) != "" {
		where = append(where, "node_host_label = ?")
		args = append(args, strings.TrimSpace(opts.NodeHostLabel))
	}
	if strings.TrimSpace(opts.NamespaceLabel) != "" {
		where = append(where, "namespace_label = ?")
		args = append(args, strings.TrimSpace(opts.NamespaceLabel))
	}
	if opts.WorkloadOnly {
		where = append(where, "is_workload = 1")
	}
	if v := strings.ToLower(strings.TrimSpace(opts.Verification)); v != "" {
		switch v {
		case "verified":
			where = append(where, "verified = 1")
		case "unverified":
			where = append(where, "verified = 0")
		case "unknown":
			where = append(where, "verified IS NULL")
		}
	}
	if q := strings.ToLower(strings.TrimSpace(opts.Query)); q != "" {
		// Best-effort search across normalized columns (keep it deterministic).
		needle := "%" + q + "%"
		where = append(where, `
			(
				LOWER(id) LIKE ? OR
				LOWER(subject_label) LIKE ? OR
				LOWER(subject_type) LIKE ? OR
				LOWER(cluster_label) LIKE ? OR
				LOWER(node_host_label) LIKE ? OR
				LOWER(namespace_label) LIKE ? OR
				LOWER(entity_id_label) LIKE ? OR
				LOWER(repository_label) LIKE ? OR
				LOWER(details_summary) LIKE ?
			)
		`)
		for i := 0; i < 9; i++ {
			args = append(args, needle)
		}
	}

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(where, " AND ")
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM recovery_points " + whereSQL
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT
			id, provider, kind, mode, outcome,
			started_at_ms, completed_at_ms, size_bytes,
			verified, encrypted, immutable,
			subject_resource_id, repository_resource_id,
			subject_ref_json, repository_ref_json, details_json
			, subject_label, subject_type, is_workload,
			cluster_label, node_host_label, namespace_label, entity_id_label,
			repository_label, details_summary
		FROM recovery_points
	` + whereSQL + `
		ORDER BY (completed_at_ms IS NULL) ASC, completed_at_ms DESC, updated_at_ms DESC
		LIMIT ? OFFSET ?
	`

	argsWithPaging := append(append([]any(nil), args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, query, argsWithPaging...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	points := make([]recovery.RecoveryPoint, 0, limit)
	for rows.Next() {
		var p recovery.RecoveryPoint
		var provider, kind, mode, outcome string
		var startedMs, completedMs sql.NullInt64
		var sizeBytes sql.NullInt64
		var verified, encrypted, immutable sql.NullInt64
		var subjectRID, repoRID sql.NullString
		var subjectRefRaw, repoRefRaw, detailsRaw sql.NullString
		var subjectLabel, subjectType sql.NullString
		var isWorkload sql.NullInt64
		var clusterLabel, nodeHostLabel, namespaceLabel, entityIDLabel sql.NullString
		var repositoryLabel, detailsSummary sql.NullString

		if err := rows.Scan(
			&p.ID,
			&provider,
			&kind,
			&mode,
			&outcome,
			&startedMs,
			&completedMs,
			&sizeBytes,
			&verified,
			&encrypted,
			&immutable,
			&subjectRID,
			&repoRID,
			&subjectRefRaw,
			&repoRefRaw,
			&detailsRaw,
			&subjectLabel,
			&subjectType,
			&isWorkload,
			&clusterLabel,
			&nodeHostLabel,
			&namespaceLabel,
			&entityIDLabel,
			&repositoryLabel,
			&detailsSummary,
		); err != nil {
			return nil, 0, err
		}

		p.Provider = recovery.Provider(provider)
		p.Kind = recovery.Kind(kind)
		p.Mode = recovery.Mode(mode)
		p.Outcome = recovery.Outcome(outcome)

		p.StartedAt = millisToTimePtr(startedMs)
		p.CompletedAt = millisToTimePtr(completedMs)

		if sizeBytes.Valid {
			v := sizeBytes.Int64
			p.SizeBytes = &v
		}
		p.Verified = dbToBoolPtr(verified)
		p.Encrypted = dbToBoolPtr(encrypted)
		p.Immutable = dbToBoolPtr(immutable)

		if subjectRID.Valid {
			p.SubjectResourceID = subjectRID.String
		}
		if repoRID.Valid {
			p.RepositoryResourceID = repoRID.String
		}

		var subjectRef recovery.ExternalRef
		if err := unmarshalJSON(subjectRefRaw, &subjectRef); err != nil {
			return nil, 0, err
		}
		if subjectRef.Type != "" {
			p.SubjectRef = &subjectRef
		}
		var repoRef recovery.ExternalRef
		if err := unmarshalJSON(repoRefRaw, &repoRef); err != nil {
			return nil, 0, err
		}
		if repoRef.Type != "" {
			p.RepositoryRef = &repoRef
		}

		var details map[string]any
		if err := unmarshalJSON(detailsRaw, &details); err != nil {
			return nil, 0, err
		}
		if len(details) > 0 {
			p.Details = details
		}

		idx := recovery.PointIndex{
			SubjectLabel:    strings.TrimSpace(subjectLabel.String),
			SubjectType:     strings.TrimSpace(subjectType.String),
			IsWorkload:      isWorkload.Valid && isWorkload.Int64 != 0,
			ClusterLabel:    strings.TrimSpace(clusterLabel.String),
			NodeHostLabel:   strings.TrimSpace(nodeHostLabel.String),
			NamespaceLabel:  strings.TrimSpace(namespaceLabel.String),
			EntityIDLabel:   strings.TrimSpace(entityIDLabel.String),
			RepositoryLabel: strings.TrimSpace(repositoryLabel.String),
			DetailsSummary:  strings.TrimSpace(detailsSummary.String),
		}
		p.Display = idx.ToDisplay()

		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return points, total, nil
}

// ListRollups returns per-subject protection rollups derived from recovery points.
// Rollups are grouped by a stable subject key:
// - linked resources: "res:<resource-id>" (returns resourceId without the prefix)
// - external refs: "ext:<hash>" (returns the ext key as resourceId)
func (s *Store) ListRollups(ctx context.Context, opts recovery.ListPointsOptions) ([]recovery.ProtectionRollup, int, error) {
	if err := s.ensureInitialized(); err != nil {
		return nil, 0, err
	}

	limit := normalizeLimit(opts.Limit)
	page := normalizePage(opts.Page)
	offset := (page - 1) * limit

	where := make([]string, 0, 10)
	args := make([]any, 0, 16)

	where = append(where, "subject_key IS NOT NULL", "subject_key != ''")

	if strings.TrimSpace(string(opts.Provider)) != "" {
		where = append(where, "provider = ?")
		args = append(args, string(opts.Provider))
	}
	if strings.TrimSpace(string(opts.Kind)) != "" {
		where = append(where, "kind = ?")
		args = append(args, string(opts.Kind))
	}
	if strings.TrimSpace(string(opts.Mode)) != "" {
		where = append(where, "mode = ?")
		args = append(args, string(opts.Mode))
	}
	if strings.TrimSpace(string(opts.Outcome)) != "" {
		where = append(where, "outcome = ?")
		args = append(args, string(opts.Outcome))
	}
	if strings.TrimSpace(opts.RollupID) != "" {
		rid := strings.TrimSpace(opts.RollupID)
		if !strings.HasPrefix(rid, "res:") && !strings.HasPrefix(rid, "ext:") {
			rid = "res:" + rid
		}
		where = append(where, "subject_key = ?")
		args = append(args, rid)
	} else if strings.TrimSpace(opts.SubjectResourceID) != "" {
		where = append(where, "subject_resource_id = ?")
		args = append(args, strings.TrimSpace(opts.SubjectResourceID))
	}

	// Rollup time window is based on the best-known timestamp for a point.
	// Prefer completed, then started, then created.
	tsExpr := "COALESCE(completed_at_ms, started_at_ms, created_at_ms)"
	if opts.From != nil && !opts.From.IsZero() {
		where = append(where, tsExpr+" >= ?")
		args = append(args, opts.From.UTC().UnixMilli())
	}
	if opts.To != nil && !opts.To.IsZero() {
		where = append(where, tsExpr+" <= ?")
		args = append(args, opts.To.UTC().UnixMilli())
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")

	countQuery := `
		WITH filtered AS (
			SELECT subject_key
			FROM recovery_points
			` + whereSQL + `
		)
		SELECT COUNT(*) FROM (SELECT subject_key FROM filtered GROUP BY subject_key)
	`

	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []recovery.ProtectionRollup{}, 0, nil
	}

	query := `
		WITH filtered AS (
			SELECT
				subject_key,
				subject_resource_id,
				subject_ref_json,
				provider,
				outcome,
				` + tsExpr + ` AS ts_ms,
				updated_at_ms,
				id
			FROM recovery_points
			` + whereSQL + `
		),
		agg AS (
			SELECT
				subject_key,
				MAX(ts_ms) AS last_attempt_ms,
				MAX(CASE WHEN outcome = 'success' THEN ts_ms END) AS last_success_ms
			FROM filtered
			GROUP BY subject_key
		),
		ranked AS (
			SELECT
				subject_key,
				outcome,
				ROW_NUMBER() OVER (PARTITION BY subject_key ORDER BY ts_ms DESC, updated_at_ms DESC, id DESC) AS rn
			FROM filtered
		),
		latest AS (
			SELECT subject_key, subject_resource_id, subject_ref_json
			FROM (
				SELECT
					subject_key,
					subject_resource_id,
					subject_ref_json,
					ROW_NUMBER() OVER (PARTITION BY subject_key ORDER BY ts_ms DESC, updated_at_ms DESC, id DESC) AS rn
				FROM filtered
			) x
			WHERE x.rn = 1
		),
		providers AS (
			SELECT subject_key, GROUP_CONCAT(DISTINCT provider) AS providers_csv
			FROM filtered
			GROUP BY subject_key
		)
		SELECT
			agg.subject_key,
			latest.subject_resource_id,
			latest.subject_ref_json,
			agg.last_attempt_ms,
			agg.last_success_ms,
			(SELECT outcome FROM ranked r WHERE r.subject_key = agg.subject_key AND r.rn = 1) AS last_outcome,
			providers.providers_csv
		FROM agg
		LEFT JOIN latest USING(subject_key)
		LEFT JOIN providers USING(subject_key)
		ORDER BY (agg.last_attempt_ms IS NULL) ASC, agg.last_attempt_ms DESC, agg.subject_key ASC
		LIMIT ? OFFSET ?
	`

	argsWithPaging := append(append([]any(nil), args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, query, argsWithPaging...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]recovery.ProtectionRollup, 0, limit)
	for rows.Next() {
		var subjectKey string
		var subjectRID sql.NullString
		var subjectRefRaw sql.NullString
		var lastAttemptMs sql.NullInt64
		var lastSuccessMs sql.NullInt64
		var lastOutcome string
		var providersRaw sql.NullString

		if err := rows.Scan(&subjectKey, &subjectRID, &subjectRefRaw, &lastAttemptMs, &lastSuccessMs, &lastOutcome, &providersRaw); err != nil {
			return nil, 0, err
		}

		var lastAttemptAt *time.Time
		if lastAttemptMs.Valid && lastAttemptMs.Int64 > 0 {
			t := time.UnixMilli(lastAttemptMs.Int64).UTC()
			lastAttemptAt = &t
		}
		var lastSuccessAt *time.Time
		if lastSuccessMs.Valid && lastSuccessMs.Int64 > 0 {
			t := time.UnixMilli(lastSuccessMs.Int64).UTC()
			lastSuccessAt = &t
		}

		var subjectRef recovery.ExternalRef
		if err := unmarshalJSON(subjectRefRaw, &subjectRef); err != nil {
			return nil, 0, err
		}
		var subjectRefPtr *recovery.ExternalRef
		if subjectRef.Type != "" {
			subjectRefPtr = &subjectRef
		}

		providers := make([]recovery.Provider, 0, 2)
		if providersRaw.Valid && strings.TrimSpace(providersRaw.String) != "" {
			for _, part := range strings.Split(providersRaw.String, ",") {
				p := strings.TrimSpace(part)
				if p == "" {
					continue
				}
				providers = append(providers, recovery.Provider(p))
			}
		}

		outcome := recovery.Outcome(strings.TrimSpace(lastOutcome))
		if outcome == "" {
			outcome = recovery.OutcomeUnknown
		}

		out = append(out, recovery.ProtectionRollup{
			RollupID:          strings.TrimSpace(subjectKey),
			SubjectResourceID: strings.TrimSpace(subjectRID.String),
			SubjectRef:        subjectRefPtr,
			LastAttemptAt:     lastAttemptAt,
			LastSuccessAt:     lastSuccessAt,
			LastOutcome:       outcome,
			Providers:         providers,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return out, total, nil
}
