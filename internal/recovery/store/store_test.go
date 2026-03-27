package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func TestStore_UpsertAndList(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "recovery.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	now := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	size := int64(1234)
	verified := true

	point := recovery.RecoveryPoint{
		ID:          "point-1",
		Provider:    recovery.ProviderKubernetes,
		Kind:        recovery.KindSnapshot,
		Mode:        recovery.ModeSnapshot,
		Outcome:     recovery.OutcomeSuccess,
		StartedAt:   &now,
		CompletedAt: &now,
		SizeBytes:   &size,
		Verified:    &verified,
		SubjectRef: &recovery.ExternalRef{
			Type:      "k8s-pvc",
			Namespace: "default",
			Name:      "data",
			UID:       "pvc-uid",
		},
		Details: map[string]any{"foo": "bar"},
	}

	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{point}); err != nil {
		t.Fatalf("UpsertPoints() error = %v", err)
	}

	got, total, err := store.ListPoints(context.Background(), recovery.ListPointsOptions{Provider: recovery.ProviderKubernetes, Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("ListPoints() error = %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(got) != 1 {
		t.Fatalf("len(points) = %d, want 1", len(got))
	}
	if got[0].ID != "point-1" {
		t.Fatalf("point id = %q, want point-1", got[0].ID)
	}
	if got[0].SubjectRef == nil || got[0].SubjectRef.Type != "k8s-pvc" {
		t.Fatalf("subjectRef = %#v, want k8s-pvc", got[0].SubjectRef)
	}
}

func TestStore_ListPoints_IgnoresMalformedJSONFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "recovery.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	now := time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC)

	point := recovery.RecoveryPoint{
		ID:          "point-bad-json",
		Provider:    recovery.ProviderKubernetes,
		Kind:        recovery.KindSnapshot,
		Mode:        recovery.ModeSnapshot,
		Outcome:     recovery.OutcomeSuccess,
		StartedAt:   &now,
		CompletedAt: &now,
		SubjectRef: &recovery.ExternalRef{
			Type:      "k8s-pvc",
			Namespace: "default",
			Name:      "data",
		},
		RepositoryRef: &recovery.ExternalRef{
			Type: "velero-backup-storage-location",
			Name: "repo-a",
		},
		Details: map[string]any{"foo": "bar"},
	}

	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{point}); err != nil {
		t.Fatalf("UpsertPoints() error = %v", err)
	}

	if _, err := store.db.ExecContext(
		context.Background(),
		`UPDATE recovery_points
		 SET subject_ref_json = '{',
		     repository_ref_json = '{',
		     details_json = '{'
		 WHERE id = ?`,
		point.ID,
	); err != nil {
		t.Fatalf("corrupt recovery point json: %v", err)
	}

	got, total, err := store.ListPoints(context.Background(), recovery.ListPointsOptions{Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("ListPoints() error = %v, want graceful degradation", err)
	}
	if total != 1 || len(got) != 1 {
		t.Fatalf("ListPoints() total=%d len=%d, want 1/1", total, len(got))
	}
	if got[0].SubjectRef != nil {
		t.Fatalf("SubjectRef = %#v, want nil after malformed json fallback", got[0].SubjectRef)
	}
	if got[0].RepositoryRef != nil {
		t.Fatalf("RepositoryRef = %#v, want nil after malformed json fallback", got[0].RepositoryRef)
	}
	if got[0].Details != nil {
		t.Fatalf("Details = %#v, want nil after malformed json fallback", got[0].Details)
	}
	if got[0].Display == nil || got[0].Display.SubjectLabel != "default/data" {
		t.Fatalf("Display = %#v, want preserved normalized subject label", got[0].Display)
	}
	if got[0].Display == nil || got[0].Display.ItemType != "pvc" {
		t.Fatalf("Display = %#v, want preserved normalized item type pvc", got[0].Display)
	}
}

func TestStore_OpenMigratesLegacySchemaMissingItemType(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "recovery.db")

	now := time.Date(2026, 2, 24, 12, 0, 0, 0, time.UTC)
	legacyPoint := recovery.RecoveryPoint{
		ID:          "legacy-point-no-item-type",
		Provider:    recovery.ProviderKubernetes,
		Kind:        recovery.KindSnapshot,
		Mode:        recovery.ModeSnapshot,
		Outcome:     recovery.OutcomeSuccess,
		StartedAt:   &now,
		CompletedAt: &now,
		SubjectRef: &recovery.ExternalRef{
			Type:      "k8s-pvc",
			Namespace: "default",
			Name:      "data",
		},
		RepositoryRef: &recovery.ExternalRef{
			Type: "velero-backup-storage-location",
			Name: "repo-a",
		},
		Details: map[string]any{
			"foo": "bar",
		},
	}

	createLegacyRecoveryDBWithoutItemType(t, dbPath, legacyPoint)

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	assertRecoveryColumnExists(t, dbPath, "item_type")

	points, total, err := store.ListPoints(context.Background(), recovery.ListPointsOptions{Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("ListPoints() error = %v", err)
	}
	if total != 1 || len(points) != 1 {
		t.Fatalf("ListPoints() total=%d len=%d, want 1/1", total, len(points))
	}
	if points[0].Display == nil || points[0].Display.ItemType != "pvc" {
		t.Fatalf("ListPoints() display = %#v, want item type pvc", points[0].Display)
	}

	rollups, rollupTotal, err := store.ListRollups(context.Background(), recovery.ListPointsOptions{Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("ListRollups() error = %v", err)
	}
	if rollupTotal != 1 || len(rollups) != 1 {
		t.Fatalf("ListRollups() total=%d len=%d, want 1/1", rollupTotal, len(rollups))
	}
	if rollups[0].Display == nil || rollups[0].Display.ItemType != "pvc" {
		t.Fatalf("ListRollups() display = %#v, want item type pvc", rollups[0].Display)
	}
}

func createLegacyRecoveryDBWithoutItemType(t *testing.T, dbPath string, point recovery.RecoveryPoint) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(%q): %v", dbPath, err)
	}
	defer db.Close()

	schema := `
		CREATE TABLE recovery_points (
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

		CREATE INDEX idx_recovery_points_completed
		ON recovery_points(completed_at_ms);

		CREATE INDEX idx_recovery_points_provider_completed
		ON recovery_points(provider, completed_at_ms);

		CREATE INDEX idx_recovery_points_subject_completed
		ON recovery_points(subject_resource_id, completed_at_ms);

		CREATE INDEX idx_recovery_points_subject_key_completed
		ON recovery_points(subject_key, completed_at_ms);

		CREATE INDEX idx_recovery_points_cluster_completed
		ON recovery_points(cluster_label, completed_at_ms);

		CREATE INDEX idx_recovery_points_node_completed
		ON recovery_points(node_host_label, completed_at_ms);

		CREATE INDEX idx_recovery_points_namespace_completed
		ON recovery_points(namespace_label, completed_at_ms);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create legacy recovery schema: %v", err)
	}

	var (
		subjectRefJSON    sql.NullString
		repositoryRefJSON sql.NullString
		detailsJSON       sql.NullString
		sizeBytes         sql.NullInt64
		verified          sql.NullInt64
		encrypted         sql.NullInt64
		immutable         sql.NullInt64
		subjectRID        sql.NullString
		repositoryRID     sql.NullString
		startedAtMs       sql.NullInt64
		completedAtMs     sql.NullInt64
	)

	if point.SubjectRef != nil {
		data, err := json.Marshal(point.SubjectRef)
		if err != nil {
			t.Fatalf("marshal subject ref: %v", err)
		}
		subjectRefJSON = sql.NullString{String: string(data), Valid: true}
	}
	if point.RepositoryRef != nil {
		data, err := json.Marshal(point.RepositoryRef)
		if err != nil {
			t.Fatalf("marshal repository ref: %v", err)
		}
		repositoryRefJSON = sql.NullString{String: string(data), Valid: true}
	}
	if len(point.Details) > 0 {
		data, err := json.Marshal(point.Details)
		if err != nil {
			t.Fatalf("marshal details: %v", err)
		}
		detailsJSON = sql.NullString{String: string(data), Valid: true}
	}
	if point.SizeBytes != nil {
		sizeBytes = sql.NullInt64{Int64: *point.SizeBytes, Valid: true}
	}
	if point.Verified != nil {
		if *point.Verified {
			verified = sql.NullInt64{Int64: 1, Valid: true}
		} else {
			verified = sql.NullInt64{Int64: 0, Valid: true}
		}
	}
	if point.Encrypted != nil {
		if *point.Encrypted {
			encrypted = sql.NullInt64{Int64: 1, Valid: true}
		} else {
			encrypted = sql.NullInt64{Int64: 0, Valid: true}
		}
	}
	if point.Immutable != nil {
		if *point.Immutable {
			immutable = sql.NullInt64{Int64: 1, Valid: true}
		} else {
			immutable = sql.NullInt64{Int64: 0, Valid: true}
		}
	}
	if point.SubjectResourceID != "" {
		subjectRID = sql.NullString{String: point.SubjectResourceID, Valid: true}
	}
	if point.RepositoryResourceID != "" {
		repositoryRID = sql.NullString{String: point.RepositoryResourceID, Valid: true}
	}
	if point.StartedAt != nil {
		startedAtMs = sql.NullInt64{Int64: point.StartedAt.UTC().UnixMilli(), Valid: true}
	}
	if point.CompletedAt != nil {
		completedAtMs = sql.NullInt64{Int64: point.CompletedAt.UTC().UnixMilli(), Valid: true}
	}

	idx := recovery.DeriveIndex(point)
	isWorkload := 0
	if idx.IsWorkload {
		isWorkload = 1
	}

	createdAtMs := time.Date(2026, 2, 26, 12, 0, 0, 0, time.UTC).UnixMilli()
	updatedAtMs := createdAtMs

	if _, err := db.ExecContext(context.Background(), `
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
	`,
		point.ID,
		string(point.Provider),
		string(point.Kind),
		string(point.Mode),
		string(point.Outcome),
		startedAtMs,
		completedAtMs,
		sizeBytes,
		verified,
		encrypted,
		immutable,
		recovery.SubjectKeyForPoint(point),
		nil,
		subjectRID,
		repositoryRID,
		subjectRefJSON,
		repositoryRefJSON,
		detailsJSON,
		idx.SubjectLabel,
		idx.SubjectType,
		isWorkload,
		idx.ClusterLabel,
		idx.NodeHostLabel,
		idx.NamespaceLabel,
		idx.EntityIDLabel,
		idx.RepositoryLabel,
		idx.DetailsSummary,
		createdAtMs,
		updatedAtMs,
	); err != nil {
		t.Fatalf("insert legacy recovery point: %v", err)
	}
}

func assertRecoveryColumnExists(t *testing.T, dbPath string, column string) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(%q): %v", dbPath, err)
	}
	defer db.Close()

	rows, err := db.QueryContext(context.Background(), `PRAGMA table_info(recovery_points)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(recovery_points): %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			colType    string
			notNull    int
			defaultVal sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultVal, &primaryKey); err != nil {
			t.Fatalf("scan table_info row: %v", err)
		}
		if name == column {
			return
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("table_info rows err: %v", err)
	}
	t.Fatalf("expected recovery_points column %q to exist after migration", column)
}
