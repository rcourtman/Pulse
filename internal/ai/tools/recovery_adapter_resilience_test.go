package tools

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

func TestRecoveryPointsMCPAdapter_ListPointsToleratesMalformedPersistedMetadata(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	manager := recoverymanager.New(mtp)
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}

	completedAt := time.Date(2026, 2, 22, 7, 45, 0, 0, time.UTC)
	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{
		{
			ID:                "adapter-point-bad-json",
			Provider:          recovery.ProviderProxmoxPBS,
			Kind:              recovery.KindBackup,
			Mode:              recovery.ModeRemote,
			Outcome:           recovery.OutcomeSuccess,
			SubjectResourceID: "vm-321",
			SubjectRef: &recovery.ExternalRef{
				Type: "proxmox-vm",
				Name: "321",
			},
			RepositoryRef: &recovery.ExternalRef{
				Type: "pbs-datastore",
				Name: "archive-store",
			},
			CompletedAt: &completedAt,
			Details: map[string]any{
				"comment": "nightly archive",
			},
		},
	}); err != nil {
		t.Fatalf("UpsertPoints(): %v", err)
	}

	persistence, err := mtp.GetPersistence("default")
	if err != nil {
		t.Fatalf("GetPersistence(default): %v", err)
	}
	corruptRecoveryAdapterRowJSON(
		t,
		filepath.Join(persistence.DataDir(), "recovery", "recovery.db"),
		"adapter-point-bad-json",
	)

	adapter := NewRecoveryPointsMCPAdapter(manager, "default")
	points, total, err := adapter.ListPoints(context.Background(), recovery.ListPointsOptions{
		Page:  1,
		Limit: 50,
	})
	if err != nil {
		t.Fatalf("ListPoints() error = %v, want graceful degradation", err)
	}
	if total != 1 || len(points) != 1 {
		t.Fatalf("ListPoints() total=%d len=%d, want 1/1", total, len(points))
	}
	if points[0].SubjectRef != nil {
		t.Fatalf("expected malformed item ref to be omitted, got %#v", points[0].SubjectRef)
	}
	if points[0].RepositoryRef != nil {
		t.Fatalf("expected malformed repository ref to be omitted, got %#v", points[0].RepositoryRef)
	}
	if points[0].Details != nil {
		t.Fatalf("expected malformed details to be omitted, got %#v", points[0].Details)
	}
	if points[0].Display.SubjectLabel != "nightly archive" {
		t.Fatalf("display.subjectLabel = %q, want %q", points[0].Display.SubjectLabel, "nightly archive")
	}
	if points[0].Display.ItemType != "vm" {
		t.Fatalf("display.itemType = %q, want %q", points[0].Display.ItemType, "vm")
	}
}

func corruptRecoveryAdapterRowJSON(t *testing.T, dbPath string, rowID string) {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(%q): %v", dbPath, err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.ExecContext(
		context.Background(),
		"UPDATE recovery_points SET subject_ref_json = '{', repository_ref_json = '{', details_json = '{' WHERE id = ?",
		rowID,
	); err != nil {
		t.Fatalf("corrupt recovery row json: %v", err)
	}
}
