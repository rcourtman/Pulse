package memory

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// Drive the real incident mutation entrypoints without background saves, then
// checkpoint synchronously to distinguish sequential evaluations from bursts.
func TestIncidentStoreUnchangedCheckpoint(t *testing.T) {
	dir := t.TempDir()
	store := NewIncidentStore(IncidentStoreConfig{})
	alert := &alerts.Alert{ID: "pbs::connectivity", ResourceID: "pbs", Message: "offline", StartTime: time.Now().UTC()}
	checkpoint := func() {
		t.Helper()
		store.dataDir = dir
		store.filePath = filepath.Join(dir, "ai_incidents.json")
		defer func() { store.dataDir = "" }()
		if err := store.saveToDisk(); err != nil {
			t.Fatal(err)
		}
	}
	store.RecordAlertFired(alert)
	checkpoint()
	first, err := os.Stat(store.filePath)
	if err != nil {
		t.Fatal(err)
	}
	id := store.GetTimelineByAlertIdentifier(alert.ID).ID
	for i := 0; i < 20; i++ {
		store.RecordAlertFired(alert)
		checkpoint()
	}
	after, err := os.Stat(store.filePath)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(first, after) || store.savesCompleted.Load() != 1 {
		t.Errorf("unchanged evaluations rewrote JSON: writes=%d", store.savesCompleted.Load())
	}
	if len(store.incidents) != 1 || store.incidents[0].ID != id {
		t.Fatal("unchanged occurrence duplicated")
	}
	alert.Message = "still offline; new evidence"
	store.RecordAlertFired(alert)
	checkpoint()
	if store.savesCompleted.Load() != 2 {
		t.Fatal("changed metadata was not persisted")
	}
	reloaded := NewIncidentStore(IncidentStoreConfig{DataDir: dir})
	if got := reloaded.GetTimelineByAlertIdentifier(alert.ID); got == nil || got.ID != id || got.Message != alert.Message {
		t.Fatalf("restart lost occurrence or metadata: %+v", got)
	}
	// A newly loaded store also recognises an unchanged checkpoint.
	if err := reloaded.saveToDisk(); err != nil {
		t.Fatal(err)
	}
	if reloaded.savesCompleted.Load() != 0 {
		t.Error("restart rewrote unchanged JSON")
	}
	store.RecordAlertResolved(alert, alert.StartTime.Add(time.Minute))
	checkpoint()
	alert.StartTime = alert.StartTime.Add(2 * time.Minute)
	store.RecordAlertFired(alert)
	checkpoint()
	reloaded = NewIncidentStore(IncidentStoreConfig{DataDir: dir})
	if len(reloaded.incidents) != 2 || reloaded.incidents[0].OccurrenceClosedAt == nil || reloaded.incidents[1].ID == id {
		t.Fatal("resolution/recurrence did not survive restart")
	}
	// Missing files must still be restored, even when in-memory state is unchanged.
	if err := os.Rename(store.filePath, store.filePath+".previous"); err != nil {
		t.Fatal(err)
	}
	checkpoint()
	if _, err := os.Stat(store.filePath); err != nil {
		t.Fatal(err)
	}
}

func TestIncidentStoreCheckpointRetriesFailedWrite(t *testing.T) {
	dir := t.TempDir()
	store := NewIncidentStore(IncidentStoreConfig{})
	alert := &alerts.Alert{ID: "pbs::connectivity", StartTime: time.Now().UTC()}
	store.RecordAlertFired(alert)
	store.dataDir = dir
	store.filePath = filepath.Join(dir, "ai_incidents.json")
	// A directory at the temporary-file path deterministically rejects writing.
	if err := os.Mkdir(store.filePath+".tmp", 0700); err != nil {
		t.Fatal(err)
	}
	if err := store.saveToDisk(); err == nil {
		t.Fatal("expected checkpoint failure")
	}
	if store.savesCompleted.Load() != 0 {
		t.Fatal("failed write counted as successful")
	}
	if err := os.Rename(store.filePath+".tmp", store.filePath+".obstruction"); err != nil {
		t.Fatal(err)
	}
	if err := store.saveToDisk(); err != nil {
		t.Fatal(err)
	}
	if store.savesCompleted.Load() != 1 {
		t.Fatal("unchanged state was not retried")
	}
	reloaded := NewIncidentStore(IncidentStoreConfig{DataDir: dir})
	if got := reloaded.GetTimelineByAlertIdentifier(alert.ID); got == nil || got.ID != store.incidents[0].ID {
		t.Fatalf("retried occurrence not durable: %+v", got)
	}
}
