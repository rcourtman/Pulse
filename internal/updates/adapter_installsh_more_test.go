package updates

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallShAdapter_PrepareUpdate(t *testing.T) {
	adapter := NewInstallShAdapter(nil)

	plan, err := adapter.PrepareUpdate(context.Background(), UpdateRequest{Version: "v1.2.3"})
	if err != nil {
		t.Fatalf("PrepareUpdate error: %v", err)
	}
	if !plan.CanAutoUpdate || !plan.RequiresRoot || !plan.RollbackSupport {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if len(plan.Instructions) == 0 || len(plan.Prerequisites) == 0 {
		t.Fatalf("expected instructions and prerequisites: %+v", plan)
	}
}

func TestInstallShAdapter_RollbackErrors(t *testing.T) {
	history, err := NewUpdateHistory(t.TempDir())
	if err != nil {
		t.Fatalf("NewUpdateHistory error: %v", err)
	}
	adapter := NewInstallShAdapter(history)
	ctx := context.Background()

	if err := adapter.Rollback(ctx, "missing"); err == nil {
		t.Fatal("expected error for missing history entry")
	}

	eventNoBackup, err := history.CreateEntry(ctx, UpdateHistoryEntry{
		Action:      "update",
		Status:      StatusSuccess,
		VersionFrom: "v1.0.0",
		VersionTo:   "v1.1.0",
	})
	if err != nil {
		t.Fatalf("CreateEntry error: %v", err)
	}
	if err := adapter.Rollback(ctx, eventNoBackup); err == nil || !strings.Contains(err.Error(), "no backup path") {
		t.Fatalf("expected backup path error, got %v", err)
	}

	eventMissingBackup, err := history.CreateEntry(ctx, UpdateHistoryEntry{
		Action:      "update",
		Status:      StatusSuccess,
		VersionFrom: "v1.0.0",
		VersionTo:   "v1.1.0",
		BackupPath:  filepath.Join(t.TempDir(), "missing"),
	})
	if err != nil {
		t.Fatalf("CreateEntry error: %v", err)
	}
	if err := adapter.Rollback(ctx, eventMissingBackup); err == nil || !strings.Contains(err.Error(), "backup not found") {
		t.Fatalf("expected backup not found error, got %v", err)
	}

	backupDir := t.TempDir()
	eventNoTarget, err := history.CreateEntry(ctx, UpdateHistoryEntry{
		Action:      "update",
		Status:      StatusSuccess,
		VersionFrom: "",
		VersionTo:   "v1.1.0",
		BackupPath:  backupDir,
	})
	if err != nil {
		t.Fatalf("CreateEntry error: %v", err)
	}
	if err := adapter.Rollback(ctx, eventNoTarget); err == nil || !strings.Contains(err.Error(), "no target version") {
		t.Fatalf("expected target version error, got %v", err)
	}
}
