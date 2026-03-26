package monitoring

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"

	_ "modernc.org/sqlite"
)

func TestMonitor_ListBackupRollupsForAlerts_ToleratesMalformedPersistedMetadata(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	manager := recoverymanager.New(mtp)
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}

	completedAt := time.Date(2026, 2, 21, 8, 30, 0, 0, time.UTC)
	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{
		{
			ID:       "monitor-point-bad-json",
			Provider: recovery.ProviderTrueNAS,
			Kind:     recovery.KindBackup,
			Mode:     recovery.ModeRemote,
			Outcome:  recovery.OutcomeSuccess,
			SubjectRef: &recovery.ExternalRef{
				Type: "truenas-dataset",
				Name: "tank/apps",
			},
			CompletedAt: &completedAt,
		},
	}); err != nil {
		t.Fatalf("UpsertPoints(): %v", err)
	}

	persistence, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("GetPersistence(default): %v", err)
	}
	corruptMonitorRecoveryRowJSON(
		t,
		filepath.Join(persistence.DataDir(), "recovery", "recovery.db"),
		"monitor-point-bad-json",
	)

	m := &Monitor{recoveryManager: manager}
	rollups, err := m.listBackupRollupsForAlerts(context.Background())
	if err != nil {
		t.Fatalf("listBackupRollupsForAlerts() error = %v, want graceful degradation", err)
	}
	if len(rollups) != 1 {
		t.Fatalf("expected exactly 1 rollup, got %d", len(rollups))
	}
	if rollups[0].SubjectRef != nil {
		t.Fatalf("expected malformed item ref to be omitted, got %#v", rollups[0].SubjectRef)
	}
	if rollups[0].Display.SubjectLabel != "tank/apps" {
		t.Fatalf("display.subjectLabel = %q, want %q", rollups[0].Display.SubjectLabel, "tank/apps")
	}
	if rollups[0].Display.ItemType != "dataset" {
		t.Fatalf("display.itemType = %q, want %q", rollups[0].Display.ItemType, "dataset")
	}
	if rollups[0].LastOutcome != recovery.OutcomeSuccess {
		t.Fatalf("lastOutcome = %q, want %q", rollups[0].LastOutcome, recovery.OutcomeSuccess)
	}
}

func corruptMonitorRecoveryRowJSON(t *testing.T, dbPath string, rowID string) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(%q): %v", dbPath, err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.ExecContext(context.Background(), "UPDATE recovery_points SET subject_ref_json = '{' WHERE id = ?", rowID); err != nil {
		t.Fatalf("corrupt recovery row json: %v", err)
	}
}
