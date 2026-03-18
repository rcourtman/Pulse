package alerts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadHistory_BackupReadErrorReturnsError(t *testing.T) {
	hm := newTestHistoryManager(t)

	// Force backup read to fail with a non-permission, non-not-exist error.
	if err := os.Mkdir(hm.backupFile, 0755); err != nil {
		t.Fatalf("failed to create backup directory: %v", err)
	}

	err := hm.loadHistory()
	if err == nil {
		t.Fatal("loadHistory should fail when backup path cannot be read as a file")
	}
	if !strings.Contains(err.Error(), "failed to read history backup file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClearAllHistory_ReturnsJoinedErrorsWhenFilesAreDirectories(t *testing.T) {
	hm := newTestHistoryManager(t)
	hm.history = []HistoryEntry{
		{Alert: Alert{ID: "alert-1"}, Timestamp: time.Now()},
	}

	if err := os.Mkdir(hm.historyFile, 0755); err != nil {
		t.Fatalf("failed to create history directory: %v", err)
	}
	if err := os.Mkdir(hm.backupFile, 0755); err != nil {
		t.Fatalf("failed to create backup directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hm.historyFile, "keep"), []byte("x"), 0644); err != nil {
		t.Fatalf("failed to populate history directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hm.backupFile, "keep"), []byte("x"), 0644); err != nil {
		t.Fatalf("failed to populate backup directory: %v", err)
	}

	err := hm.ClearAllHistory()
	if err == nil {
		t.Fatal("ClearAllHistory should fail when history and backup paths are directories")
	}
	if len(hm.history) != 0 {
		t.Fatalf("history should be cleared in memory, got %d entries", len(hm.history))
	}

	msg := err.Error()
	if !strings.Contains(msg, "remove history file") {
		t.Fatalf("expected history removal error, got: %v", err)
	}
	if !strings.Contains(msg, "remove backup file") {
		t.Fatalf("expected backup removal error, got: %v", err)
	}
}

func TestCleanupRoutine_ReturnsImmediatelyWhenStopped(t *testing.T) {
	hm := newTestHistoryManager(t)
	close(hm.stopChan)

	done := make(chan struct{})
	go func() {
		hm.cleanupRoutine()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("cleanupRoutine did not return after stop signal")
	}
}

func TestStartPeriodicSave_PersistsHistoryOnTicker(t *testing.T) {
	hm := newTestHistoryManager(t)
	hm.saveInterval = 10 * time.Millisecond
	hm.history = []HistoryEntry{
		{Alert: Alert{ID: "periodic-save-alert"}, Timestamp: time.Now()},
	}

	hm.startPeriodicSave()
	defer func() {
		close(hm.stopChan)
		if hm.saveTicker != nil {
			hm.saveTicker.Stop()
		}
	}()

	deadline := time.Now().Add(750 * time.Millisecond)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(hm.historyFile)
		if err == nil && strings.Contains(string(data), "periodic-save-alert") {
			return
		}
		time.Sleep(15 * time.Millisecond)
	}

	t.Fatal("periodic save did not persist history data before deadline")
}
