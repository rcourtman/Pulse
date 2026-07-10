package updates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureApplyTargetIsNewer(t *testing.T) {
	cases := []struct {
		name    string
		current string
		target  string
		wantErr bool
	}{
		{"newer patch allowed", "6.0.4", "6.0.5", false},
		{"older patch blocked", "6.0.5", "6.0.4", true},
		{"same version blocked", "6.0.5", "6.0.5", true},
		{"rc to stable of same base allowed", "6.0.5-rc.4", "6.0.5", false},
		{"stable to rc of same base blocked", "6.0.5", "6.0.5-rc.4", true},
		{"older rc blocked", "6.0.5-rc.4", "6.0.5-rc.3", true},
		{"newer rc allowed", "6.0.5-rc.4", "6.0.5-rc.5", false},
		{"v prefix handled", "v6.0.5", "v6.0.6", false},
		{"unparseable current allowed", "not-a-version", "6.0.4", false},
		{"unparseable target allowed", "6.0.5", "not-a-version", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ensureApplyTargetIsNewer(tc.current, tc.target)
			if tc.wantErr && err == nil {
				t.Fatalf("expected downgrade error for current=%s target=%s", tc.current, tc.target)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for current=%s target=%s: %v", tc.current, tc.target, err)
			}
			if err != nil && !strings.Contains(err.Error(), "not newer than the running version") {
				t.Fatalf("expected canonical downgrade message, got %q", err.Error())
			}
		})
	}
}

func TestApplyUpdateRejectsDowngradeBeforeDownload(t *testing.T) {
	t.Setenv("PULSE_ALLOW_DOCKER_UPDATES", "true")
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("PULSE_INSTALL_DIR", t.TempDir())

	oldBuildVersion := BuildVersion
	BuildVersion = "6.0.5"
	t.Cleanup(func() { BuildVersion = oldBuildVersion })

	manager := &Manager{}

	err := manager.ApplyUpdate(context.Background(), ApplyUpdateRequest{
		DownloadURL: "https://github.com/rcourtman/Pulse/releases/download/v6.0.4/pulse-v6.0.4-linux-amd64.tar.gz",
	})
	if err == nil {
		t.Fatal("expected downgrade to be rejected")
	}
	if !strings.Contains(err.Error(), "not newer than the running version") {
		t.Fatalf("expected downgrade rejection, got %v", err)
	}

	status := manager.GetStatus()
	if status.Status != "error" {
		t.Fatalf("expected error status after rejected downgrade, got %q", status.Status)
	}
}

func TestApplyUpdateAllowDowngradeSkipsGuard(t *testing.T) {
	t.Setenv("PULSE_ALLOW_DOCKER_UPDATES", "true")
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("PULSE_INSTALL_DIR", t.TempDir())

	// Serve 404s locally so the apply fails fast at the download stage
	// without touching the real release host.
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	t.Setenv("PULSE_UPDATE_SERVER", server.URL)

	oldBuildVersion := BuildVersion
	BuildVersion = "6.0.5"
	t.Cleanup(func() { BuildVersion = oldBuildVersion })

	manager := &Manager{}

	// With the opt-in set the request must get past the downgrade guard; it
	// then fails later at the download step instead.
	err := manager.ApplyUpdate(context.Background(), ApplyUpdateRequest{
		DownloadURL:    server.URL + "/releases/download/v6.0.4/pulse-v6.0.4-linux-amd64.tar.gz",
		AllowDowngrade: true,
	})
	if err == nil {
		t.Fatal("expected apply to fail at a later stage in this environment")
	}
	if strings.Contains(err.Error(), "not newer than the running version") {
		t.Fatalf("expected the downgrade guard to be skipped, got %v", err)
	}
}

func TestValidateRetainedBackupDir(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)

	valid := filepath.Join(dataDir, "backup-20260710-010101")
	if err := os.MkdirAll(valid, 0755); err != nil {
		t.Fatalf("mkdir valid backup: %v", err)
	}

	t.Run("accepts managed backup", func(t *testing.T) {
		got, err := validateRetainedBackupDir(valid)
		if err != nil {
			t.Fatalf("expected managed backup to validate: %v", err)
		}
		if got != filepath.Clean(valid) {
			t.Fatalf("expected %q, got %q", valid, got)
		}
	})

	t.Run("rejects path outside managed roots", func(t *testing.T) {
		outside := filepath.Join(t.TempDir(), "backup-20260710-010101")
		if err := os.MkdirAll(outside, 0755); err != nil {
			t.Fatalf("mkdir outside backup: %v", err)
		}
		if _, err := validateRetainedBackupDir(outside); err == nil {
			t.Fatal("expected unmanaged path to be rejected")
		}
	})

	t.Run("rejects wrong prefix inside managed root", func(t *testing.T) {
		wrongPrefix := filepath.Join(dataDir, "snapshots-20260710-010101")
		if err := os.MkdirAll(wrongPrefix, 0755); err != nil {
			t.Fatalf("mkdir wrong prefix: %v", err)
		}
		if _, err := validateRetainedBackupDir(wrongPrefix); err == nil {
			t.Fatal("expected wrong-prefix path to be rejected")
		}
	})

	t.Run("rejects missing directory", func(t *testing.T) {
		missing := filepath.Join(dataDir, "backup-19990101-000000")
		_, err := validateRetainedBackupDir(missing)
		if err == nil || !strings.Contains(err.Error(), "no longer exists") {
			t.Fatalf("expected missing-backup error, got %v", err)
		}
	})
}

func TestRollbackToBackupRestoresFilesAndRecordsHistory(t *testing.T) {
	t.Setenv("PULSE_ALLOW_DOCKER_UPDATES", "true")
	dataDir := t.TempDir()
	installDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)
	t.Setenv("PULSE_INSTALL_DIR", installDir)

	// Backup contents captured before the recorded update. The backup holds
	// no "pulse" binary on purpose: restoring one would overwrite the running
	// test executable via os.Executable.
	backupDir := filepath.Join(dataDir, "backup-20260710-020202")
	if err := os.MkdirAll(filepath.Join(backupDir, "config"), 0755); err != nil {
		t.Fatalf("mkdir backup config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "config", "system.json"), []byte(`{"restored":true}`), 0600); err != nil {
		t.Fatalf("write backup config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, ".env"), []byte("RESTORED=1\n"), 0600); err != nil {
		t.Fatalf("write backup env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "VERSION"), []byte("6.0.4\n"), 0644); err != nil {
		t.Fatalf("write backup VERSION: %v", err)
	}

	// Current install state that the rollback must replace.
	if err := os.MkdirAll(filepath.Join(installDir, "config"), 0755); err != nil {
		t.Fatalf("mkdir install config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "config", "system.json"), []byte(`{"restored":false}`), 0600); err != nil {
		t.Fatalf("write install config: %v", err)
	}

	history, err := NewUpdateHistory(t.TempDir())
	if err != nil {
		t.Fatalf("NewUpdateHistory: %v", err)
	}
	sourceEventID, err := history.CreateEntry(context.Background(), UpdateHistoryEntry{
		Action:      "update",
		Status:      StatusSuccess,
		VersionFrom: "6.0.4",
		VersionTo:   "6.0.5",
		BackupPath:  backupDir,
	})
	if err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}

	manager := &Manager{history: history}

	if err := manager.RollbackToBackup(context.Background(), RollbackRequest{EventID: sourceEventID}); err != nil {
		t.Fatalf("RollbackToBackup: %v", err)
	}

	restoredConfig, err := os.ReadFile(filepath.Join(installDir, "config", "system.json"))
	if err != nil {
		t.Fatalf("read restored config: %v", err)
	}
	if !strings.Contains(string(restoredConfig), `"restored":true`) {
		t.Fatalf("expected config to be restored from backup, got %s", restoredConfig)
	}
	restoredEnv, err := os.ReadFile(filepath.Join(installDir, ".env"))
	if err != nil {
		t.Fatalf("read restored .env: %v", err)
	}
	if !strings.Contains(string(restoredEnv), "RESTORED=1") {
		t.Fatalf("expected .env to be restored, got %s", restoredEnv)
	}

	source, err := history.GetEntry(sourceEventID)
	if err != nil {
		t.Fatalf("GetEntry source: %v", err)
	}
	if source.Status != StatusRolledBack {
		t.Fatalf("expected source entry to be marked rolled_back, got %q", source.Status)
	}

	entries := history.ListEntries(HistoryFilter{Action: "rollback"})
	if len(entries) != 1 {
		t.Fatalf("expected one rollback history entry, got %d", len(entries))
	}
	rollback := entries[0]
	if rollback.Status != StatusSuccess {
		t.Fatalf("expected rollback entry success, got %q", rollback.Status)
	}
	if rollback.VersionTo != "6.0.4" {
		t.Fatalf("expected rollback target version 6.0.4, got %q", rollback.VersionTo)
	}
	if rollback.RelatedEventID != sourceEventID {
		t.Fatalf("expected rollback to reference the source update entry")
	}

	status := manager.GetStatus()
	if status.Status != "completed" {
		t.Fatalf("expected completed status after rollback, got %q", status.Status)
	}
}

func TestRollbackToBackupRejectsBadRequests(t *testing.T) {
	t.Setenv("PULSE_ALLOW_DOCKER_UPDATES", "true")
	dataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)
	t.Setenv("PULSE_INSTALL_DIR", t.TempDir())

	history, err := NewUpdateHistory(t.TempDir())
	if err != nil {
		t.Fatalf("NewUpdateHistory: %v", err)
	}
	manager := &Manager{history: history}

	t.Run("no history sink", func(t *testing.T) {
		bare := &Manager{}
		if err := bare.RollbackToBackup(context.Background(), RollbackRequest{EventID: "x"}); err == nil {
			t.Fatal("expected error without history")
		}
	})

	t.Run("empty event ID", func(t *testing.T) {
		err := manager.RollbackToBackup(context.Background(), RollbackRequest{EventID: "  "})
		if err == nil || !strings.Contains(err.Error(), "event ID is required") {
			t.Fatalf("expected event ID error, got %v", err)
		}
	})

	t.Run("unknown entry", func(t *testing.T) {
		err := manager.RollbackToBackup(context.Background(), RollbackRequest{EventID: "does-not-exist"})
		if err == nil || !strings.Contains(err.Error(), "history entry not found") {
			t.Fatalf("expected not-found error, got %v", err)
		}
	})

	t.Run("entry without backup", func(t *testing.T) {
		eventID, err := history.CreateEntry(context.Background(), UpdateHistoryEntry{
			Action: "update",
			Status: StatusSuccess,
		})
		if err != nil {
			t.Fatalf("CreateEntry: %v", err)
		}
		rollbackErr := manager.RollbackToBackup(context.Background(), RollbackRequest{EventID: eventID})
		if rollbackErr == nil || !strings.Contains(rollbackErr.Error(), "no retained backup") {
			t.Fatalf("expected no-backup error, got %v", rollbackErr)
		}
	})

	t.Run("pruned backup directory", func(t *testing.T) {
		eventID, err := history.CreateEntry(context.Background(), UpdateHistoryEntry{
			Action:     "update",
			Status:     StatusSuccess,
			BackupPath: filepath.Join(dataDir, "backup-20200101-000000"),
		})
		if err != nil {
			t.Fatalf("CreateEntry: %v", err)
		}
		rollbackErr := manager.RollbackToBackup(context.Background(), RollbackRequest{EventID: eventID})
		if rollbackErr == nil || !strings.Contains(rollbackErr.Error(), "no longer exists") {
			t.Fatalf("expected missing-backup error, got %v", rollbackErr)
		}
	})

	t.Run("concurrent update in flight", func(t *testing.T) {
		backupDir := filepath.Join(dataDir, "backup-20260710-030303")
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			t.Fatalf("mkdir backup: %v", err)
		}
		eventID, err := history.CreateEntry(context.Background(), UpdateHistoryEntry{
			Action:     "update",
			Status:     StatusSuccess,
			BackupPath: backupDir,
		})
		if err != nil {
			t.Fatalf("CreateEntry: %v", err)
		}

		manager.updateMu.Lock()
		manager.updateInFlight = true
		manager.updateMu.Unlock()
		t.Cleanup(func() {
			manager.updateMu.Lock()
			manager.updateInFlight = false
			manager.updateMu.Unlock()
		})

		rollbackErr := manager.RollbackToBackup(context.Background(), RollbackRequest{EventID: eventID})
		if rollbackErr == nil || !strings.Contains(rollbackErr.Error(), "already in progress") {
			t.Fatalf("expected in-progress error, got %v", rollbackErr)
		}
	})
}
