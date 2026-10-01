package alerts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
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

func TestLoadHistory_MainReadErrorWithoutBackupReturnsError(t *testing.T) {
	hm := newTestHistoryManager(t)

	// A present but unreadable primary is not equivalent to a clean first run
	// merely because no backup exists.
	if err := os.Mkdir(hm.historyFile, 0755); err != nil {
		t.Fatalf("failed to create primary directory: %v", err)
	}

	err := hm.loadHistory()
	if err == nil {
		t.Fatal("loadHistory should fail when the primary cannot be read and no backup exists")
	}
	if !strings.Contains(err.Error(), "failed to read history file") {
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
	// Exercise the real ticker and persistence worker using virtual time.
	// The old 750 ms filesystem-poll deadline measured wall-clock scheduling
	// and I/O as well as periodic-save behaviour. Wait drains each tick's real
	// file I/O before assertions, without adding a production callback or
	// letting Stop's final save satisfy the periodic-save assertion.
	synctest.Test(t, func(t *testing.T) {
		hm := newTestHistoryManager(t)
		hm.history = []HistoryEntry{
			{Alert: Alert{ID: "periodic-save-alert"}, Timestamp: time.Now()},
		}
		hm.startPeriodicSave()
		defer hm.Stop()

		synctest.Wait()
		if _, err := os.Stat(hm.historyFile); !os.IsNotExist(err) {
			t.Fatalf("history written before first tick: %v", err)
		}
		assertPersisted := func(wantIDs ...string) {
			t.Helper()
			data, err := os.ReadFile(hm.historyFile)
			if err != nil {
				t.Fatalf("read periodic history: %v", err)
			}
			var entries []HistoryEntry
			if err := json.Unmarshal(data, &entries); err != nil {
				t.Fatalf("decode periodic history: %v", err)
			}
			if len(entries) != len(wantIDs) {
				t.Fatalf("periodic history has %d entries, want %d", len(entries), len(wantIDs))
			}
			for i, id := range wantIDs {
				if entries[i].Alert.ID != id {
					t.Errorf("entry %d ID = %q, want %q", i, entries[i].Alert.ID, id)
				}
			}
		}

		time.Sleep(hm.saveInterval)
		synctest.Wait()
		assertPersisted("periodic-save-alert")

		hm.mu.Lock()
		hm.history = append(hm.history, HistoryEntry{
			Alert: Alert{ID: "second-tick-alert"}, Timestamp: time.Now(),
		})
		hm.mu.Unlock()
		time.Sleep(hm.saveInterval)
		synctest.Wait()
		assertPersisted("periodic-save-alert", "second-tick-alert")
	})
}
