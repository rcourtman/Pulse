package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gorillaws "github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	internalws "github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	_ "modernc.org/sqlite"
)

type integrationServer struct {
	server  *httptest.Server
	monitor *monitoring.Monitor
	hub     *internalws.Hub
	config  *config.Config
}

func newIntegrationServer(t *testing.T) *integrationServer {
	return newIntegrationServerWithConfig(t, nil)
}

func newIntegrationServerWithConfig(t *testing.T, customize func(*config.Config)) *integrationServer {
	t.Helper()

	tmpDir := t.TempDir()
	cfg := &config.Config{
		ConfigPath:     tmpDir,
		DataPath:       tmpDir,
		DemoMode:       false,
		AllowedOrigins: "*",
		EnvOverrides:   make(map[string]bool),
	}

	return newIntegrationServerWithRuntimeMode(t, cfg, true, customize)
}

func newIntegrationServerWithoutMock(t *testing.T, customize func(*config.Config)) *integrationServer {
	t.Helper()

	tmpDir := t.TempDir()
	cfg := &config.Config{
		ConfigPath:     tmpDir,
		DataPath:       tmpDir,
		DemoMode:       false,
		AllowedOrigins: "*",
		EnvOverrides:   make(map[string]bool),
	}

	return newIntegrationServerWithRuntimeMode(t, cfg, false, customize)
}

func newIntegrationServerWithRuntimeMode(
	t *testing.T,
	cfg *config.Config,
	mockMode bool,
	customize func(*config.Config),
) *integrationServer {
	t.Helper()

	if mockMode {
		t.Setenv("PULSE_MOCK_MODE", "true")
		mock.SetEnabled(true)
	} else {
		t.Setenv("PULSE_MOCK_MODE", "false")
		mock.SetEnabled(false)
	}

	if customize != nil {
		customize(cfg)
	}

	var monitor *monitoring.Monitor
	hub := internalws.NewHub(func(orgID string) interface{} {
		if monitor == nil {
			return models.StateSnapshot{}
		}
		return monitor.BuildFrontendState()
	})

	go hub.Run()

	var err error
	monitor, err = monitoring.New(cfg)
	if err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}
	monitor.SetMockMode(mockMode)

	hub.SetStateGetter(func(orgID string) interface{} {
		return monitor.BuildFrontendState()
	})

	version := readRuntimeVersion(t)
	if version == "" {
		version = "dev"
	}

	router := api.NewRouter(cfg, monitor, nil, hub, func() error {
		monitor.SyncAlertState()
		return nil
	}, version, nil)

	srv := newIPv4HTTPServer(t, router.Handler())
	t.Cleanup(func() {
		srv.Close()
		if monitor != nil {
			monitor.StopDiscoveryService()
			monitor.Stop()
		}
		if hub != nil {
			hub.Stop()
		}
		mock.SetEnabled(false)
	})

	return &integrationServer{
		server:  srv,
		monitor: monitor,
		hub:     hub,
		config:  cfg,
	}
}

func TestRecoveryRollupsEndpointToleratesMalformedPersistedMetadata(t *testing.T) {
	srv := newIntegrationServerWithoutMock(t, nil)

	dbPath := seedRecoveryPointForIntegration(t, srv.config, recovery.RecoveryPoint{
		ID:          "router-rollup-bad-json",
		Provider:    recovery.ProviderTrueNAS,
		Kind:        recovery.KindSnapshot,
		Mode:        recovery.ModeSnapshot,
		Outcome:     recovery.OutcomeSuccess,
		StartedAt:   timePtrIntegration(time.Date(2026, 2, 22, 12, 0, 0, 0, time.UTC)),
		CompletedAt: timePtrIntegration(time.Date(2026, 2, 22, 12, 0, 0, 0, time.UTC)),
		SubjectRef: &recovery.ExternalRef{
			Type: "truenas-dataset",
			Name: "tank/apps",
			ID:   "tank/apps",
		},
	})
	corruptRecoveryIntegrationRowJSON(t, dbPath, "router-rollup-bad-json", true, false, false)

	res, err := http.Get(srv.server.URL + "/api/recovery/rollups?limit=500")
	if err != nil {
		t.Fatalf("rollups request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("unexpected status: got %d want %d, body=%s", res.StatusCode, http.StatusOK, string(body))
	}

	var payload struct {
		Data []struct {
			RollupID    string         `json:"rollupId"`
			ItemRef     map[string]any `json:"itemRef"`
			SubjectRef  map[string]any `json:"subjectRef"`
			LastOutcome string         `json:"lastOutcome"`
			Display     struct {
				SubjectLabel string `json:"subjectLabel"`
				ItemType     string `json:"itemType"`
			} `json:"display"`
		} `json:"data"`
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read rollups response: %v", err)
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal rollups response: %v", err)
	}
	if len(payload.Data) != 1 {
		t.Fatalf("expected exactly 1 rollup, got %d body=%s", len(payload.Data), string(body))
	}
	rollup := payload.Data[0]
	if rollup.RollupID == "" {
		t.Fatalf("expected non-empty rollup id, body=%s", string(body))
	}
	if rollup.ItemRef != nil || rollup.SubjectRef != nil {
		t.Fatalf("expected malformed item refs to be omitted, got itemRef=%#v subjectRef=%#v", rollup.ItemRef, rollup.SubjectRef)
	}
	if rollup.Display.SubjectLabel != "tank/apps" {
		t.Fatalf("display.subjectLabel = %q, want %q", rollup.Display.SubjectLabel, "tank/apps")
	}
	if rollup.Display.ItemType != "dataset" {
		t.Fatalf("display.itemType = %q, want %q", rollup.Display.ItemType, "dataset")
	}
	if rollup.LastOutcome != "success" {
		t.Fatalf("lastOutcome = %q, want %q", rollup.LastOutcome, "success")
	}
}

func TestRecoveryPointsEndpointToleratesMalformedPersistedMetadata(t *testing.T) {
	srv := newIntegrationServerWithoutMock(t, nil)

	dbPath := seedRecoveryPointForIntegration(t, srv.config, recovery.RecoveryPoint{
		ID:          "router-point-bad-json",
		Provider:    recovery.ProviderKubernetes,
		Kind:        recovery.KindSnapshot,
		Mode:        recovery.ModeSnapshot,
		Outcome:     recovery.OutcomeSuccess,
		StartedAt:   timePtrIntegration(time.Date(2026, 2, 23, 12, 0, 0, 0, time.UTC)),
		CompletedAt: timePtrIntegration(time.Date(2026, 2, 23, 12, 0, 0, 0, time.UTC)),
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
	})
	corruptRecoveryIntegrationRowJSON(t, dbPath, "router-point-bad-json", true, true, true)

	res, err := http.Get(srv.server.URL + "/api/recovery/points?limit=500")
	if err != nil {
		t.Fatalf("points request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("unexpected status: got %d want %d, body=%s", res.StatusCode, http.StatusOK, string(body))
	}

	var payload struct {
		Data []struct {
			ID            string         `json:"id"`
			ItemRef       map[string]any `json:"itemRef"`
			SubjectRef    map[string]any `json:"subjectRef"`
			RepositoryRef map[string]any `json:"repositoryRef"`
			Details       map[string]any `json:"details"`
			Display       struct {
				SubjectLabel string `json:"subjectLabel"`
				ItemType     string `json:"itemType"`
			} `json:"display"`
		} `json:"data"`
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read points response: %v", err)
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal points response: %v", err)
	}
	if len(payload.Data) != 1 {
		t.Fatalf("expected exactly 1 point, got %d body=%s", len(payload.Data), string(body))
	}
	point := payload.Data[0]
	if point.ID != "router-point-bad-json" {
		t.Fatalf("id = %q, want %q", point.ID, "router-point-bad-json")
	}
	if point.ItemRef != nil || point.SubjectRef != nil || point.RepositoryRef != nil || point.Details != nil {
		t.Fatalf("expected malformed metadata fields to be omitted, got itemRef=%#v subjectRef=%#v repositoryRef=%#v details=%#v", point.ItemRef, point.SubjectRef, point.RepositoryRef, point.Details)
	}
	if point.Display.SubjectLabel != "default/data" {
		t.Fatalf("display.subjectLabel = %q, want %q", point.Display.SubjectLabel, "default/data")
	}
	if point.Display.ItemType != "pvc" {
		t.Fatalf("display.itemType = %q, want %q", point.Display.ItemType, "pvc")
	}
}

func TestRecoveryPointsEndpointMigratesLegacySchemaMissingItemType(t *testing.T) {
	srv := newIntegrationServerWithoutMock(t, nil)

	dbPath := seedLegacyRecoveryPointWithoutItemTypeForIntegration(t, srv.config, recovery.RecoveryPoint{
		ID:          "legacy-point-no-item-type",
		Provider:    recovery.ProviderKubernetes,
		Kind:        recovery.KindSnapshot,
		Mode:        recovery.ModeSnapshot,
		Outcome:     recovery.OutcomeSuccess,
		StartedAt:   timePtrIntegration(time.Date(2026, 2, 24, 12, 0, 0, 0, time.UTC)),
		CompletedAt: timePtrIntegration(time.Date(2026, 2, 24, 12, 0, 0, 0, time.UTC)),
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
	})

	res, err := http.Get(srv.server.URL + "/api/recovery/points?limit=500")
	if err != nil {
		t.Fatalf("points request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("unexpected status: got %d want %d, body=%s", res.StatusCode, http.StatusOK, string(body))
	}

	assertRecoveryIntegrationColumnExists(t, dbPath, "item_type")

	var payload struct {
		Data []struct {
			ID      string `json:"id"`
			Display struct {
				SubjectLabel string `json:"subjectLabel"`
				ItemType     string `json:"itemType"`
			} `json:"display"`
		} `json:"data"`
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read points response: %v", err)
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal points response: %v", err)
	}
	if len(payload.Data) != 1 {
		t.Fatalf("expected exactly 1 point, got %d body=%s", len(payload.Data), string(body))
	}
	if payload.Data[0].ID != "legacy-point-no-item-type" {
		t.Fatalf("point id = %q, want %q", payload.Data[0].ID, "legacy-point-no-item-type")
	}
	if payload.Data[0].Display.SubjectLabel != "default/data" {
		t.Fatalf("display.subjectLabel = %q, want %q", payload.Data[0].Display.SubjectLabel, "default/data")
	}
	if payload.Data[0].Display.ItemType != "pvc" {
		t.Fatalf("display.itemType = %q, want %q", payload.Data[0].Display.ItemType, "pvc")
	}
}

func TestRecoveryRollupsEndpointMigratesLegacySchemaMissingItemType(t *testing.T) {
	srv := newIntegrationServerWithoutMock(t, nil)

	dbPath := seedLegacyRecoveryPointWithoutItemTypeForIntegration(t, srv.config, recovery.RecoveryPoint{
		ID:          "legacy-rollup-no-item-type",
		Provider:    recovery.ProviderTrueNAS,
		Kind:        recovery.KindSnapshot,
		Mode:        recovery.ModeSnapshot,
		Outcome:     recovery.OutcomeSuccess,
		StartedAt:   timePtrIntegration(time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)),
		CompletedAt: timePtrIntegration(time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)),
		SubjectRef: &recovery.ExternalRef{
			Type: "truenas-dataset",
			Name: "tank/apps",
			ID:   "tank/apps",
		},
	})

	res, err := http.Get(srv.server.URL + "/api/recovery/rollups?limit=500")
	if err != nil {
		t.Fatalf("rollups request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("unexpected status: got %d want %d, body=%s", res.StatusCode, http.StatusOK, string(body))
	}

	assertRecoveryIntegrationColumnExists(t, dbPath, "item_type")

	var payload struct {
		Data []struct {
			RollupID string `json:"rollupId"`
			Display  struct {
				SubjectLabel string `json:"subjectLabel"`
				ItemType     string `json:"itemType"`
			} `json:"display"`
		} `json:"data"`
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read rollups response: %v", err)
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal rollups response: %v", err)
	}
	if len(payload.Data) != 1 {
		t.Fatalf("expected exactly 1 rollup, got %d body=%s", len(payload.Data), string(body))
	}
	if payload.Data[0].RollupID == "" {
		t.Fatalf("expected non-empty rollup id body=%s", string(body))
	}
	if payload.Data[0].Display.SubjectLabel != "tank/apps" {
		t.Fatalf("display.subjectLabel = %q, want %q", payload.Data[0].Display.SubjectLabel, "tank/apps")
	}
	if payload.Data[0].Display.ItemType != "dataset" {
		t.Fatalf("display.itemType = %q, want %q", payload.Data[0].Display.ItemType, "dataset")
	}
}

func seedRecoveryPointForIntegration(t *testing.T, cfg *config.Config, point recovery.RecoveryPoint) string {
	t.Helper()

	mtp := config.NewMultiTenantPersistence(cfg.DataPath)
	manager := recoverymanager.New(mtp)
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}
	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{point}); err != nil {
		t.Fatalf("UpsertPoints(): %v", err)
	}

	return filepath.Join(cfg.DataPath, "recovery", "recovery.db")
}

func seedLegacyRecoveryPointWithoutItemTypeForIntegration(t *testing.T, cfg *config.Config, point recovery.RecoveryPoint) string {
	t.Helper()

	dbPath := filepath.Join(cfg.DataPath, "recovery", "recovery.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatalf("MkdirAll(recovery dir): %v", err)
	}

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

	return dbPath
}

func assertRecoveryIntegrationColumnExists(t *testing.T, dbPath string, column string) {
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

func corruptRecoveryIntegrationRowJSON(t *testing.T, dbPath string, rowID string, corruptSubject bool, corruptRepository bool, corruptDetails bool) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(%q): %v", dbPath, err)
	}
	t.Cleanup(func() { _ = db.Close() })

	assignments := make([]string, 0, 3)
	if corruptSubject {
		assignments = append(assignments, "subject_ref_json = '{'")
	}
	if corruptRepository {
		assignments = append(assignments, "repository_ref_json = '{'")
	}
	if corruptDetails {
		assignments = append(assignments, "details_json = '{'")
	}
	if len(assignments) == 0 {
		t.Fatal("corruptRecoveryIntegrationRowJSON called without any fields to corrupt")
	}

	query := "UPDATE recovery_points SET " + joinCSVIntegration(assignments) + " WHERE id = ?"
	if _, err := db.ExecContext(context.Background(), query, rowID); err != nil {
		t.Fatalf("corrupt recovery row json: %v", err)
	}
}

func joinCSVIntegration(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	default:
		out := parts[0]
		for _, part := range parts[1:] {
			out += ", " + part
		}
		return out
	}
}

func timePtrIntegration(t time.Time) *time.Time {
	return &t
}

func TestHealthEndpoint(t *testing.T) {
	srv := newIntegrationServer(t)

	res, err := http.Get(srv.server.URL + "/api/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", res.StatusCode, http.StatusOK)
	}

	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	if payload["status"] != "healthy" {
		t.Fatalf("expected status=healthy, got %v", payload["status"])
	}

	dependencies, ok := payload["dependencies"].(map[string]any)
	if !ok {
		t.Fatalf("expected dependencies map in health response, got %#v", payload["dependencies"])
	}
	if dependencies["monitor"] != true {
		t.Fatalf("expected monitor dependency to be true, got %#v", dependencies["monitor"])
	}
	if dependencies["scheduler"] != true {
		t.Fatalf("expected scheduler dependency to be true, got %#v", dependencies["scheduler"])
	}
	if dependencies["websocket"] != true {
		t.Fatalf("expected websocket dependency to be true, got %#v", dependencies["websocket"])
	}
}

func TestVersionEndpointUsesRepoVersion(t *testing.T) {
	srv := newIntegrationServer(t)

	releaseVersion := readVersionFile(t)
	runtimeVersion := readRuntimeVersion(t)

	res, err := http.Get(srv.server.URL + "/api/version")
	if err != nil {
		t.Fatalf("version request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", res.StatusCode, http.StatusOK)
	}

	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode version response: %v", err)
	}

	actual, ok := payload["version"].(string)
	if !ok {
		t.Fatalf("version field missing or not a string: %v", payload["version"])
	}

	if strings.HasPrefix(actual, "0.0.0-") {
		// Development builds normalize to 0.0.0-<branch>[...], which is expected.
		return
	}

	normalizedActual := normalizeVersion(actual)
	if releaseVersion != "" && normalizedActual == normalizeVersion(releaseVersion) {
		return
	}

	if normalizedActual == normalizeVersion(runtimeVersion) {
		return
	}

	t.Fatalf("expected version to match release %q or runtime %q, got %s", releaseVersion, runtimeVersion, actual)
}

func TestAlertAcknowledge_AllowsPrintableAlertIDs(t *testing.T) {
	srv := newIntegrationServer(t)

	// This ID includes parentheses which were rejected in v5 RC builds due to overly strict
	// validation. The request should make it to the alert manager, which returns 404 because
	// the alert does not exist in this test environment.
	alertID := "docker(host)-container-unhealthy"
	body := bytes.NewBufferString(`{"alertIdentifier":"` + alertID + `"}`)
	req, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/alerts/acknowledge", body)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("ack request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("unexpected status: got %d want %d, body=%s", res.StatusCode, http.StatusNotFound, string(body))
	}
}

func TestStateEndpointReturnsMockData(t *testing.T) {
	srv := newIntegrationServer(t)

	res, err := http.Get(srv.server.URL + "/api/state")
	if err != nil {
		t.Fatalf("state request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", res.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read state response: %v", err)
	}

	var snapshot map[string]any
	if err := json.Unmarshal(body, &snapshot); err != nil {
		t.Fatalf("unmarshal state response: %v", err)
	}

	if _, ok := snapshot["nodes"]; ok {
		t.Fatalf("state response should not include legacy nodes key: %s", string(body))
	}

	hasNonLegacyData := false
	for _, key := range []string{"resources", "connectedInfrastructure"} {
		if values, ok := snapshot[key].([]any); ok && len(values) > 0 {
			hasNonLegacyData = true
			break
		}
	}
	if !hasNonLegacyData {
		t.Fatalf("expected non-legacy state data (resources/kubernetesClusters/pbs/pmg), got: %s", string(body))
	}
}

func TestRecoveryRollupsEndpointReturnsMockData(t *testing.T) {
	srv := newIntegrationServer(t)

	res, err := http.Get(srv.server.URL + "/api/recovery/rollups?limit=500")
	if err != nil {
		t.Fatalf("rollups request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("unexpected status: got %d want %d, body=%s", res.StatusCode, http.StatusOK, string(body))
	}

	var payload struct {
		Data []map[string]any `json:"data"`
		Meta map[string]any   `json:"meta"`
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read rollups response: %v", err)
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal rollups response: %v", err)
	}

	if len(payload.Data) == 0 {
		t.Fatalf("expected rollups data, got none: %s", string(body))
	}

	hasK8s := false
	hasTrueNAS := false
	for _, item := range payload.Data {
		raw, ok := item["platforms"]
		if !ok {
			continue
		}
		arr, ok := raw.([]any)
		if !ok {
			continue
		}
		for _, v := range arr {
			s, _ := v.(string)
			if s == "kubernetes" {
				hasK8s = true
			}
			if s == "truenas" {
				hasTrueNAS = true
			}
		}
	}
	if !hasK8s || !hasTrueNAS {
		t.Fatalf("expected rollups to include kubernetes and truenas platforms, got k8s=%v truenas=%v", hasK8s, hasTrueNAS)
	}
}

func TestProtectedEndpointsRequireAuthentication(t *testing.T) {
	passwordHash, err := internalauth.HashPassword("supersecret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	srv := newIntegrationServerWithConfig(t, func(cfg *config.Config) {
		cfg.AuthUser = "admin"
		cfg.AuthPass = passwordHash
	})

	client := &http.Client{}

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/state"},
		{"GET", "/api/storage/test-storage"},
		{"GET", "/api/updates/status"},
		{"POST", "/api/updates/apply"},
		{"GET", "/api/alerts/active"},
		{"GET", "/api/notifications/email"},
	}

	for _, ep := range endpoints {
		req, err := http.NewRequest(ep.method, srv.server.URL+ep.path, nil)
		if err != nil {
			t.Fatalf("build request for %s %s: %v", ep.method, ep.path, err)
		}

		res, err := client.Do(req)
		if err != nil {
			t.Fatalf("request for %s %s failed: %v", ep.method, ep.path, err)
		}
		_ = res.Body.Close()

		if res.StatusCode != http.StatusUnauthorized && res.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 401/403 for %s %s, got %d", ep.method, ep.path, res.StatusCode)
		}
	}
}

func TestAPIOnlyModeRequiresToken(t *testing.T) {
	const rawToken = "apitoken-test-1234567890"

	tokenRecord, err := config.NewAPITokenRecord(rawToken, "test token", []string{config.ScopeMonitoringRead})
	if err != nil {
		t.Fatalf("create token record: %v", err)
	}

	srv := newIntegrationServerWithConfig(t, func(cfg *config.Config) {
		cfg.AuthUser = ""
		cfg.AuthPass = ""
		cfg.APITokens = []config.APITokenRecord{*tokenRecord}
	})

	client := &http.Client{}

	// Without token should be rejected.
	req, err := http.NewRequest("GET", srv.server.URL+"/api/state", nil)
	if err != nil {
		t.Fatalf("build unauthenticated request: %v", err)
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("unauthenticated request failed: %v", err)
	}
	_ = res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without API token, got %d", res.StatusCode)
	}

	// With the correct token should succeed.
	req, err = http.NewRequest("GET", srv.server.URL+"/api/state", nil)
	if err != nil {
		t.Fatalf("build authenticated request: %v", err)
	}
	req.Header.Set("X-API-Token", rawToken)

	res, err = client.Do(req)
	if err != nil {
		t.Fatalf("authenticated request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with valid API token, got %d", res.StatusCode)
	}
}

func TestServerInfoEndpointReportsDevelopment(t *testing.T) {
	srv := newIntegrationServer(t)

	res, err := http.Get(srv.server.URL + "/api/server/info")
	if err != nil {
		t.Fatalf("server info request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", res.StatusCode, http.StatusOK)
	}

	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode server info response: %v", err)
	}

	isDev, ok := payload["isDevelopment"].(bool)
	if !ok {
		t.Fatalf("isDevelopment missing or not bool: %v", payload["isDevelopment"])
	}
	if !isDev {
		t.Fatalf("expected development mode to be true")
	}

	version, ok := payload["version"].(string)
	if !ok {
		t.Fatalf("version missing or not string: %v", payload["version"])
	}
	if version == "" {
		t.Fatalf("expected non-empty version string")
	}
}

func TestRecoveryPointsEndpointReturnsMockData(t *testing.T) {
	srv := newIntegrationServer(t)

	res, err := http.Get(srv.server.URL + "/api/recovery/points?limit=500")
	if err != nil {
		t.Fatalf("recovery points request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("unexpected status: got %d want %d; body=%s", res.StatusCode, http.StatusOK, string(body))
	}

	var payload struct {
		Data []struct {
			Platform string `json:"platform"`
		} `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode recovery points response: %v", err)
	}

	if payload.Meta.Total <= 0 {
		t.Fatalf("expected meta.total > 0, got %d", payload.Meta.Total)
	}

	var hasK8s, hasTrueNAS bool
	for _, p := range payload.Data {
		switch p.Platform {
		case "kubernetes":
			hasK8s = true
		case "truenas":
			hasTrueNAS = true
		}
	}
	if !hasK8s {
		t.Fatalf("expected at least one kubernetes recovery point in response")
	}
	if !hasTrueNAS {
		t.Fatalf("expected at least one truenas recovery point in response")
	}
}

func TestServerInfoEndpointMethodNotAllowed(t *testing.T) {
	srv := newIntegrationServer(t)

	req, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/server/info", nil)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("server info POST request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected status: got %d want %d", res.StatusCode, http.StatusMethodNotAllowed)
	}
}

func TestHealthEndpointMethodNotAllowed(t *testing.T) {
	srv := newIntegrationServer(t)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req, err := http.NewRequest(method, srv.server.URL+"/api/health", nil)
		if err != nil {
			t.Fatalf("create request failed: %v", err)
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("health %s request failed: %v", method, err)
		}
		res.Body.Close()

		if res.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s: unexpected status: got %d want %d", method, res.StatusCode, http.StatusMethodNotAllowed)
		}
	}
}

func TestHealthEndpointHeadAllowed(t *testing.T) {
	srv := newIntegrationServer(t)

	req, err := http.NewRequest(http.MethodHead, srv.server.URL+"/api/health", nil)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("health HEAD request failed: %v", err)
	}
	res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("HEAD: unexpected status: got %d want %d", res.StatusCode, http.StatusOK)
	}
}

func TestConfigNodesUsesMockTopology(t *testing.T) {
	srv := newIntegrationServer(t)

	res, err := http.Get(srv.server.URL + "/api/config/nodes")
	if err != nil {
		t.Fatalf("config nodes request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", res.StatusCode, http.StatusOK)
	}

	var nodes []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&nodes); err != nil {
		t.Fatalf("decode config nodes response: %v", err)
	}

	if len(nodes) == 0 {
		t.Fatalf("expected at least one mock node definition")
	}
}

func TestMockModeToggleEndpoint(t *testing.T) {
	srv := newIntegrationServer(t)

	if !mock.IsMockEnabled() {
		t.Fatalf("mock mode should be enabled at start of test")
	}

	disablePayload := bytes.NewBufferString(`{"enabled": false}`)
	res, err := http.Post(srv.server.URL+"/api/system/mock-mode", "application/json", disablePayload)
	if err != nil {
		t.Fatalf("disable mock mode request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status disabling mock mode: got %d want %d", res.StatusCode, http.StatusOK)
	}

	var response struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("decode mock mode response: %v", err)
	}
	if response.Enabled {
		t.Fatalf("expected mock mode to be disabled")
	}
	if mock.IsMockEnabled() {
		t.Fatalf("mock mode global flag not disabled")
	}

	enablePayload := bytes.NewBufferString(`{"enabled": true}`)
	resEnable, err := http.Post(srv.server.URL+"/api/system/mock-mode", "application/json", enablePayload)
	if err != nil {
		t.Fatalf("enable mock mode request failed: %v", err)
	}
	defer resEnable.Body.Close()

	if resEnable.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status enabling mock mode: got %d want %d", resEnable.StatusCode, http.StatusOK)
	}
	if err := json.NewDecoder(resEnable.Body).Decode(&response); err != nil {
		t.Fatalf("decode enable response: %v", err)
	}
	if !response.Enabled {
		t.Fatalf("expected mock mode to be enabled after re-enable call")
	}
}

func TestMockModeToggleFromRealDataRebindsPlatformSupplementalProviders(t *testing.T) {
	srv := newIntegrationServerWithoutMock(t, nil)

	if mock.IsMockEnabled() {
		t.Fatalf("mock mode should be disabled at start of test")
	}

	enablePayload := bytes.NewBufferString(`{"enabled": true}`)
	resEnable, err := http.Post(srv.server.URL+"/api/system/mock-mode", "application/json", enablePayload)
	if err != nil {
		t.Fatalf("enable mock mode request failed: %v", err)
	}
	defer resEnable.Body.Close()

	if resEnable.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status enabling mock mode: got %d want %d", resEnable.StatusCode, http.StatusOK)
	}

	checkResourceSource := func(path string, wantSource string) {
		t.Helper()

		deadline := time.Now().Add(5 * time.Second)
		for {
			res, err := http.Get(srv.server.URL + path)
			if err != nil {
				t.Fatalf("GET %s failed: %v", path, err)
			}

			var payload api.ResourcesResponse
			decodeErr := json.NewDecoder(res.Body).Decode(&payload)
			res.Body.Close()
			if decodeErr != nil {
				t.Fatalf("decode %s response: %v", path, decodeErr)
			}
			if res.StatusCode != http.StatusOK {
				t.Fatalf("unexpected %s status: got %d want %d", path, res.StatusCode, http.StatusOK)
			}

			for _, resource := range payload.Data {
				for _, source := range resource.Sources {
					if string(source) == wantSource {
						return
					}
				}
			}

			if time.Now().After(deadline) {
				t.Fatalf("expected %s resources after mock toggle, got %#v", wantSource, payload.Data)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	checkResourceSource("/api/resources?source=truenas", "truenas")
	checkResourceSource("/api/resources?source=vmware-vsphere", "vmware")
}

func TestAuthenticatedEndpointsRequireToken(t *testing.T) {
	const apiToken = "test-token"

	srv := newIntegrationServerWithConfig(t, func(cfg *config.Config) {
		record, err := config.NewAPITokenRecord(apiToken, "Integration test token", nil)
		if err != nil {
			t.Fatalf("create API token record: %v", err)
		}
		cfg.APITokens = []config.APITokenRecord{*record}
		cfg.SortAPITokens()
		hashedPass, err := internalauth.HashPassword("super-secure-pass")
		if err != nil {
			t.Fatalf("hash password: %v", err)
		}
		cfg.AuthUser = "admin"
		cfg.AuthPass = hashedPass
	})

	url := srv.server.URL + "/api/config/nodes"

	// Without token -> unauthorized
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("unauthenticated request failed: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", res.StatusCode)
	}

	// With wrong token -> still unauthorized
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("X-API-Token", "wrong-token")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request with wrong token failed: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong token, got %d", res.StatusCode)
	}

	// With correct token -> success
	req, err = http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("create authenticated request: %v", err)
	}
	req.Header.Set("X-API-Token", apiToken)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authenticated request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with valid token, got %d", res.StatusCode)
	}

	// Admin endpoint should reject without token and accept with token
	postURL := srv.server.URL + "/api/config/nodes"

	req, err = http.NewRequest(http.MethodPost, postURL, bytes.NewBufferString("{}"))
	if err != nil {
		t.Fatalf("create POST request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unauthenticated POST failed: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for POST without token, got %d", res.StatusCode)
	}

	req, err = http.NewRequest(http.MethodPost, postURL, bytes.NewBufferString("{}"))
	if err != nil {
		t.Fatalf("create authenticated POST request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", apiToken)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authenticated POST failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized {
		t.Fatalf("expected POST to require auth but got 401 even with valid token")
	}
	if res.StatusCode != http.StatusBadRequest && res.StatusCode != http.StatusForbidden && res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status for authenticated POST: %d", res.StatusCode)
	}
}

func TestAPITokenQueryAndHeaderAuth(t *testing.T) {
	const apiToken = "query-token-1234567890"

	srv := newIntegrationServerWithConfig(t, func(cfg *config.Config) {
		record, err := config.NewAPITokenRecord(apiToken, "Query token test", nil)
		if err != nil {
			t.Fatalf("create API token record: %v", err)
		}
		cfg.APITokens = []config.APITokenRecord{*record}
		cfg.SortAPITokens()
	})

	// Query-string tokens must be rejected for regular HTTP requests to prevent
	// token leakage via logs, referrer headers, and browser history.
	queryURL := srv.server.URL + "/api/state?token=" + apiToken
	res, err := http.Get(queryURL)
	if err != nil {
		t.Fatalf("query parameter request failed: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 when query-string token used without WebSocket upgrade, got %d", res.StatusCode)
	}

	// Query-string tokens must be accepted for WebSocket upgrade requests.
	wsReq, err := http.NewRequest(http.MethodGet, srv.server.URL+"/api/state?token="+apiToken, nil)
	if err != nil {
		t.Fatalf("create websocket upgrade request: %v", err)
	}
	wsReq.Header.Set("Upgrade", "websocket")
	wsReq.Header.Set("Connection", "Upgrade")
	wsRes, err := http.DefaultClient.Do(wsReq)
	if err != nil {
		t.Fatalf("websocket upgrade request failed: %v", err)
	}
	wsRes.Body.Close()
	// The server won't complete the WebSocket handshake (no real upgrader),
	// but the auth layer should accept the token — anything other than 401 is fine.
	if wsRes.StatusCode == http.StatusUnauthorized {
		t.Fatalf("expected query-string token to be accepted on WebSocket upgrade, got 401")
	}

	// Header-based token auth must still work for regular requests.
	req, err := http.NewRequest(http.MethodGet, srv.server.URL+"/api/state", nil)
	if err != nil {
		t.Fatalf("create header-auth request: %v", err)
	}
	req.Header.Set("X-API-Token", apiToken)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("header-auth request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with header token, got %d", res.StatusCode)
	}
}

func TestRecoveryEndpointRequiresDirectLoopback(t *testing.T) {
	srv := newIntegrationServer(t)

	generateBody := strings.NewReader(`{"action":"generate_token"}`)
	req, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/security/recovery", generateBody)
	if err != nil {
		t.Fatalf("create generate token request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("generate token request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 generating token from loopback, got %d", res.StatusCode)
	}

	forwardedBody := strings.NewReader(`{"action":"generate_token"}`)
	reqForwarded, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/security/recovery", forwardedBody)
	if err != nil {
		t.Fatalf("create forwarded request: %v", err)
	}
	reqForwarded.Header.Set("Content-Type", "application/json")
	reqForwarded.Header.Set("X-Forwarded-For", "198.51.100.42")

	resForwarded, err := http.DefaultClient.Do(reqForwarded)
	if err != nil {
		t.Fatalf("forwarded recovery request failed: %v", err)
	}
	defer resForwarded.Body.Close()
	if resForwarded.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 when forwarded headers present, got %d", resForwarded.StatusCode)
	}
}

func TestWebSocketSendsInitialState(t *testing.T) {
	srv := newIntegrationServer(t)

	wsURL := "ws" + strings.TrimPrefix(srv.server.URL, "http") + "/ws?org_id=default"

	conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	readMsg := func() (string, map[string]any) {
		t.Helper()
		if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("set deadline: %v", err)
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read message: %v", err)
		}
		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("decode message: %v", err)
		}
		typeVal, _ := msg["type"].(string)
		payload := map[string]any{}
		if raw, ok := msg["data"].(map[string]any); ok {
			payload = raw
		}
		return typeVal, payload
	}

	msgType, _ := readMsg()
	if msgType != "welcome" {
		t.Fatalf("expected welcome message, got %q", msgType)
	}

	msgType, payload := readMsg()
	if msgType != "initialState" {
		t.Fatalf("expected initialState message, got %q", msgType)
	}

	legacyKeys := []string{
		"nodes",
		"vms",
		"containers",
		"dockerHosts",
		"hosts",
		"storage",
	}
	for _, key := range legacyKeys {
		if _, ok := payload[key]; ok {
			t.Fatalf("initial state should not include legacy key %q", key)
		}
	}

	// Broadcast an additional state update and ensure clients receive it
	state := srv.monitor.BuildFrontendState()
	srv.hub.BroadcastState(state)

	msgType, payload = readMsg()
	if msgType != "rawData" {
		t.Fatalf("expected rawData broadcast, got %q", msgType)
	}
	for _, key := range legacyKeys {
		if _, ok := payload[key]; ok {
			t.Fatalf("broadcast payload should not include legacy key %q", key)
		}
	}
}

func TestWebsocketPayloadContractShape(t *testing.T) {
	srv := newIntegrationServer(t)

	wsURL := "ws" + strings.TrimPrefix(srv.server.URL, "http") + "/ws?org_id=default"

	conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	readMsg := func() (string, map[string]any) {
		t.Helper()
		if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("set deadline: %v", err)
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read message: %v", err)
		}
		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("decode message: %v", err)
		}
		typeVal, _ := msg["type"].(string)
		payload := map[string]any{}
		if raw, ok := msg["data"].(map[string]any); ok {
			payload = raw
		}
		return typeVal, payload
	}

	readType := func(expected string) map[string]any {
		t.Helper()
		for i := 0; i < 6; i++ {
			msgType, payload := readMsg()
			if msgType == expected {
				return payload
			}
		}
		t.Fatalf("timed out waiting for %q websocket message", expected)
		return nil
	}

	readType("welcome")
	readType("initialState")

	contractState := models.StateFrontend{
		Resources: []models.ResourceFrontend{
			{
				ID:           "resource-1",
				Type:         "node",
				Name:         "node-1",
				DisplayName:  "Node 1",
				PlatformID:   "platform-1",
				PlatformType: "proxmox",
				SourceType:   "pve",
				Status:       "online",
				LastSeen:     1,
			},
		},
		ConnectedInfrastructure: []models.ConnectedInfrastructureItemFrontend{
			{
				ID:     "resource-1",
				Name:   "Node 1",
				Status: "active",
				Surfaces: []models.ConnectedInfrastructureSurfaceFrontend{
					{ID: "agent:host-1", Kind: "agent", Label: "Host telemetry"},
				},
			},
		},
	}

	srv.hub.BroadcastState(contractState)
	payload := readType("rawData")

	requiredArrayKeys := []string{"resources", "connectedInfrastructure"}
	for _, key := range requiredArrayKeys {
		val, ok := payload[key]
		if !ok {
			t.Fatalf("websocket payload missing required %q key", key)
		}
		if values, ok := val.([]any); !ok || len(values) == 0 {
			t.Fatalf("websocket payload key %q must be a non-empty array, got %T (%v)", key, val, val)
		}
	}

	legacyKeys := []string{
		"nodes",
		"vms",
		"containers",
		"dockerHosts",
		"hosts",
		"storage",
		"removedDockerHosts",
		"removedHostAgents",
		"removedKubernetesClusters",
	}
	for _, key := range legacyKeys {
		if _, ok := payload[key]; ok {
			t.Fatalf("expected websocket payload to exclude legacy key %q", key)
		}
	}

	if _, ok := payload["backups"]; ok {
		t.Fatalf("websocket payload must not include legacy backups map")
	}
}

func TestWebsocketPayloadUsesCanonicalStateContract(t *testing.T) {
	srv := newIntegrationServer(t)

	wsURL := "ws" + strings.TrimPrefix(srv.server.URL, "http") + "/ws?org_id=default"

	conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	readMsg := func() (string, map[string]any) {
		t.Helper()
		if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("set deadline: %v", err)
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read message: %v", err)
		}
		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("decode message: %v", err)
		}
		typeVal, _ := msg["type"].(string)
		payload := map[string]any{}
		if raw, ok := msg["data"].(map[string]any); ok {
			payload = raw
		}
		return typeVal, payload
	}

	readType := func(expected string) map[string]any {
		t.Helper()
		for i := 0; i < 6; i++ {
			msgType, payload := readMsg()
			if msgType == expected {
				return payload
			}
		}
		t.Fatalf("timed out waiting for %q websocket message", expected)
		return nil
	}

	readType("welcome")
	readType("initialState")

	testState := models.StateFrontend{
		Resources: []models.ResourceFrontend{
			{
				ID:           "resource-1",
				Type:         "node",
				Name:         "node-1",
				DisplayName:  "Node 1",
				PlatformID:   "platform-1",
				PlatformType: "proxmox",
				SourceType:   "pve",
				Status:       "online",
				LastSeen:     1,
			},
		},
		ConnectedInfrastructure: []models.ConnectedInfrastructureItemFrontend{
			{
				ID:     "resource-1",
				Name:   "Node 1",
				Status: "active",
				Surfaces: []models.ConnectedInfrastructureSurfaceFrontend{
					{ID: "agent:host-1", Kind: "agent", Label: "Host telemetry"},
				},
			},
		},
	}

	srv.hub.BroadcastState(testState)
	canonicalPayload := readType("rawData")

	legacyKeys := []string{
		"nodes",
		"vms",
		"containers",
		"dockerHosts",
		"hosts",
		"storage",
		"removedDockerHosts",
		"removedHostAgents",
		"removedKubernetesClusters",
	}
	for _, key := range legacyKeys {
		if _, ok := canonicalPayload[key]; ok {
			t.Fatalf("expected canonical websocket payload to exclude %s", key)
		}
	}

	resources, ok := canonicalPayload["resources"].([]any)
	if !ok || len(resources) == 0 {
		t.Fatalf("expected resources in canonical websocket payload: %v", canonicalPayload["resources"])
	}
	if connectedInfra, ok := canonicalPayload["connectedInfrastructure"].([]any); !ok || len(connectedInfra) == 0 {
		t.Fatalf(
			"expected connectedInfrastructure in canonical websocket payload: %v",
			canonicalPayload["connectedInfrastructure"],
		)
	}
	if _, ok := canonicalPayload["backups"]; ok {
		t.Fatalf("expected canonical websocket payload to omit backups")
	}
}

func TestSessionCookieAllowsAuthenticatedAccess(t *testing.T) {
	srv := newIntegrationServerWithConfig(t, func(cfg *config.Config) {
		hashedPass, err := internalauth.HashPassword("super-secure-pass")
		if err != nil {
			t.Fatalf("hash password: %v", err)
		}
		cfg.AuthUser = "admin"
		cfg.AuthPass = hashedPass
	})

	noCookieResp, err := http.Get(srv.server.URL + "/api/config/nodes")
	if err != nil {
		t.Fatalf("unauthenticated request failed: %v", err)
	}
	noCookieResp.Body.Close()
	if noCookieResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", noCookieResp.StatusCode)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}

	body, err := json.Marshal(map[string]string{
		"username": "admin",
		"password": "super-secure-pass",
	})
	if err != nil {
		t.Fatalf("marshal login payload: %v", err)
	}

	loginReq, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/login", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create login request: %v", err)
	}
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := client.Do(loginReq)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d", loginResp.StatusCode)
	}

	sessionURL, _ := url.Parse(srv.server.URL)
	cookies := jar.Cookies(sessionURL)
	var hasSessionCookie bool
	for _, c := range cookies {
		if c.Name == "pulse_session" && c.Value != "" {
			hasSessionCookie = true
			break
		}
	}
	if !hasSessionCookie {
		t.Fatalf("expected pulse_session cookie after login")
	}

	authedResp, err := client.Get(srv.server.URL + "/api/config/nodes")
	if err != nil {
		t.Fatalf("authenticated request failed: %v", err)
	}
	defer authedResp.Body.Close()
	if authedResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with session cookie, got %d", authedResp.StatusCode)
	}
}

func TestRevokedAPITokenImmediatelyLosesAccess(t *testing.T) {
	srv := newIntegrationServerWithConfig(t, func(cfg *config.Config) {
		hashedPass, err := internalauth.HashPassword("super-secure-pass")
		if err != nil {
			t.Fatalf("hash password: %v", err)
		}
		cfg.AuthUser = "admin"
		cfg.AuthPass = hashedPass
	})

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	sessionClient := &http.Client{Jar: jar}
	tokenClient := &http.Client{}

	loginBody, err := json.Marshal(map[string]string{
		"username": "admin",
		"password": "super-secure-pass",
	})
	if err != nil {
		t.Fatalf("marshal login payload: %v", err)
	}

	loginReq, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/login", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("create login request: %v", err)
	}
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := sessionClient.Do(loginReq)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d", loginResp.StatusCode)
	}

	sessionURL, err := url.Parse(srv.server.URL)
	if err != nil {
		t.Fatalf("parse session URL: %v", err)
	}
	var csrfToken string
	for _, cookie := range jar.Cookies(sessionURL) {
		if cookie.Name == "pulse_csrf" {
			csrfToken = cookie.Value
			break
		}
	}
	if csrfToken == "" {
		t.Fatalf("expected pulse_csrf cookie after login")
	}

	createReq, err := http.NewRequest(
		http.MethodPost,
		srv.server.URL+"/api/security/tokens",
		bytes.NewBufferString(`{"name":"revocation-proof","scopes":["monitoring:read"]}`),
	)
	if err != nil {
		t.Fatalf("create token request: %v", err)
	}
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-CSRF-Token", csrfToken)
	createResp, err := sessionClient.Do(createReq)
	if err != nil {
		t.Fatalf("create token request failed: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(createResp.Body)
		t.Fatalf("expected 200 creating token, got %d: %s", createResp.StatusCode, string(body))
	}

	var tokenPayload struct {
		Token  string `json:"token"`
		Record struct {
			ID string `json:"id"`
		} `json:"record"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&tokenPayload); err != nil {
		t.Fatalf("decode token create response: %v", err)
	}
	if tokenPayload.Token == "" || tokenPayload.Record.ID == "" {
		t.Fatalf("expected token and record ID, got %+v", tokenPayload)
	}

	assertStateAuth := func(headerName, headerValue string, wantStatus int) {
		t.Helper()

		req, err := http.NewRequest(http.MethodGet, srv.server.URL+"/api/state", nil)
		if err != nil {
			t.Fatalf("create state request: %v", err)
		}
		req.Header.Set(headerName, headerValue)

		res, err := tokenClient.Do(req)
		if err != nil {
			t.Fatalf("state request failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != wantStatus {
			body, _ := io.ReadAll(res.Body)
			t.Fatalf("expected %d for %s auth, got %d: %s", wantStatus, headerName, res.StatusCode, string(body))
		}
	}

	assertStateAuth("X-API-Token", tokenPayload.Token, http.StatusOK)
	assertStateAuth("Authorization", "Bearer "+tokenPayload.Token, http.StatusOK)

	deleteReq, err := http.NewRequest(
		http.MethodDelete,
		srv.server.URL+"/api/security/tokens/"+tokenPayload.Record.ID,
		nil,
	)
	if err != nil {
		t.Fatalf("create delete request: %v", err)
	}
	deleteReq.Header.Set("X-CSRF-Token", csrfToken)
	deleteResp, err := sessionClient.Do(deleteReq)
	if err != nil {
		t.Fatalf("delete token request failed: %v", err)
	}
	deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 deleting token, got %d", deleteResp.StatusCode)
	}

	assertStateAuth("X-API-Token", tokenPayload.Token, http.StatusUnauthorized)
	assertStateAuth("Authorization", "Bearer "+tokenPayload.Token, http.StatusUnauthorized)
}

func TestLimitedAPITokenCannotCreateBroaderToken(t *testing.T) {
	srv := newIntegrationServerWithConfig(t, func(cfg *config.Config) {
		hashedPass, err := internalauth.HashPassword("super-secure-pass")
		if err != nil {
			t.Fatalf("hash password: %v", err)
		}
		cfg.AuthUser = "admin"
		cfg.AuthPass = hashedPass
	})

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	sessionClient := &http.Client{Jar: jar}
	tokenClient := &http.Client{}

	loginBody, err := json.Marshal(map[string]string{
		"username": "admin",
		"password": "super-secure-pass",
	})
	if err != nil {
		t.Fatalf("marshal login payload: %v", err)
	}

	loginReq, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/login", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("create login request: %v", err)
	}
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := sessionClient.Do(loginReq)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d", loginResp.StatusCode)
	}

	sessionURL, err := url.Parse(srv.server.URL)
	if err != nil {
		t.Fatalf("parse session URL: %v", err)
	}
	var csrfToken string
	for _, cookie := range jar.Cookies(sessionURL) {
		if cookie.Name == "pulse_csrf" {
			csrfToken = cookie.Value
			break
		}
	}
	if csrfToken == "" {
		t.Fatalf("expected pulse_csrf cookie after login")
	}

	createLimitedReq, err := http.NewRequest(
		http.MethodPost,
		srv.server.URL+"/api/security/tokens",
		bytes.NewBufferString(`{"name":"limited-token","scopes":["settings:write"]}`),
	)
	if err != nil {
		t.Fatalf("create limited token request: %v", err)
	}
	createLimitedReq.Header.Set("Content-Type", "application/json")
	createLimitedReq.Header.Set("X-CSRF-Token", csrfToken)
	createLimitedResp, err := sessionClient.Do(createLimitedReq)
	if err != nil {
		t.Fatalf("limited token creation request failed: %v", err)
	}
	defer createLimitedResp.Body.Close()
	if createLimitedResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(createLimitedResp.Body)
		t.Fatalf("expected 200 creating limited token, got %d: %s", createLimitedResp.StatusCode, string(body))
	}

	var limitedTokenPayload struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(createLimitedResp.Body).Decode(&limitedTokenPayload); err != nil {
		t.Fatalf("decode limited token create response: %v", err)
	}
	if limitedTokenPayload.Token == "" {
		t.Fatalf("expected limited token value in response")
	}

	assertScopeEscalationDenied := func(name, body, wantFragment string) {
		t.Helper()

		req, err := http.NewRequest(http.MethodPost, srv.server.URL+"/api/security/tokens", bytes.NewBufferString(body))
		if err != nil {
			t.Fatalf("create denied token request %s: %v", name, err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+limitedTokenPayload.Token)

		res, err := tokenClient.Do(req)
		if err != nil {
			t.Fatalf("denied token request %s failed: %v", name, err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusForbidden {
			body, _ := io.ReadAll(res.Body)
			t.Fatalf("expected 403 for %s, got %d: %s", name, res.StatusCode, string(body))
		}

		payload, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatalf("read denied response for %s: %v", name, err)
		}
		if !strings.Contains(string(payload), wantFragment) {
			t.Fatalf("expected %s response to contain %q, got %q", name, wantFragment, string(payload))
		}
	}

	assertScopeEscalationDenied(
		"explicit broader scope",
		`{"name":"broader-token","scopes":["settings:write","monitoring:read"]}`,
		`Cannot grant scope "monitoring:read"`,
	)
	assertScopeEscalationDenied(
		"implicit wildcard scope",
		`{"name":"wildcard-token"}`,
		`Cannot grant scope "*"`,
	)

	listReq, err := http.NewRequest(http.MethodGet, srv.server.URL+"/api/security/tokens", nil)
	if err != nil {
		t.Fatalf("create list tokens request: %v", err)
	}
	listReq.Header.Set("X-CSRF-Token", csrfToken)
	listResp, err := sessionClient.Do(listReq)
	if err != nil {
		t.Fatalf("list tokens request failed: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(listResp.Body)
		t.Fatalf("expected 200 listing tokens, got %d: %s", listResp.StatusCode, string(body))
	}

	var listPayload struct {
		Tokens []struct {
			Name string `json:"name"`
		} `json:"tokens"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode list tokens response: %v", err)
	}
	if len(listPayload.Tokens) != 1 || listPayload.Tokens[0].Name != "limited-token" {
		t.Fatalf("expected only limited token to remain after denied requests, got %+v", listPayload.Tokens)
	}
}

func TestPublicURLDetectionUsesForwardedHeaders(t *testing.T) {
	const apiToken = "public-url-detection-token-12345"

	// Configure 127.0.0.1 as trusted proxy so X-Forwarded-* headers are read
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "127.0.0.1/32")
	api.ResetTrustedProxyConfigForTests()

	srv := newIntegrationServerWithConfig(t, func(cfg *config.Config) {
		record, err := config.NewAPITokenRecord(apiToken, "Public URL detection test", nil)
		if err != nil {
			t.Fatalf("create API token record: %v", err)
		}
		cfg.APITokens = []config.APITokenRecord{*record}
		cfg.SortAPITokens()
	})

	req, err := http.NewRequest(http.MethodGet, srv.server.URL+"/api/health", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "pulse.example.com")
	req.Header.Set("X-Forwarded-Port", "8443")
	req.Header.Set("X-API-Token", apiToken)

	res, err := srv.server.Client().Do(req)
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	res.Body.Close()

	expected := "https://pulse.example.com:8443"
	if got := srv.config.PublicURL; got != expected {
		t.Fatalf("expected config public URL %q, got %q", expected, got)
	}

	if mgr := srv.monitor.GetNotificationManager(); mgr != nil {
		if actual := mgr.GetPublicURL(); actual != expected {
			t.Fatalf("expected notification manager public URL %q, got %q", expected, actual)
		}
	}
}

func TestPublicURLDetectionRespectsEnvOverride(t *testing.T) {
	const overrideURL = "https://from-env.example.com"

	srv := newIntegrationServerWithConfig(t, func(cfg *config.Config) {
		cfg.PublicURL = overrideURL
		cfg.EnvOverrides["publicURL"] = true
	})

	req, err := http.NewRequest(http.MethodGet, srv.server.URL+"/api/health", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "ignored.example.org")

	res, err := srv.server.Client().Do(req)
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	res.Body.Close()

	if got := srv.config.PublicURL; got != overrideURL {
		t.Fatalf("expected config public URL to remain %q, got %q", overrideURL, got)
	}

	if mgr := srv.monitor.GetNotificationManager(); mgr != nil {
		if actual := mgr.GetPublicURL(); actual != overrideURL {
			t.Fatalf("expected notification manager public URL %q, got %q", overrideURL, actual)
		}
	}
}

func readVersionFile(t *testing.T) string {
	t.Helper()

	versionPath := filepath.Join("..", "..", "VERSION")
	data, err := os.ReadFile(versionPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func readRuntimeVersion(t *testing.T) string {
	t.Helper()

	info, err := updates.GetCurrentVersion()
	if err != nil {
		t.Fatalf("failed to determine current version: %v", err)
	}
	return strings.TrimSpace(info.Version)
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimSuffix(v, "-dirty")
	// Strip pre-release metadata (after '-')
	if idx := strings.IndexByte(v, '-'); idx >= 0 {
		v = v[:idx]
	}
	// Strip build metadata (after '+')
	if idx := strings.IndexByte(v, '+'); idx >= 0 {
		v = v[:idx]
	}
	return v
}
