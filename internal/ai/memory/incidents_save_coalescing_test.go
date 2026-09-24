package memory

import (
	"os"
	"testing"
	"time"
)

func waitForCompletedSaves(t *testing.T, store *IncidentStore, want int64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if store.savesCompleted.Load() >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("saves completed = %d, want at least %d", store.savesCompleted.Load(), want)
}

func TestIncidentStoreSaveCoalescing(t *testing.T) {
	dir := t.TempDir()
	store := NewIncidentStore(IncidentStoreConfig{DataDir: dir})

	// Hold the save lock so the queued save cannot start: every request made
	// while one is queued must coalesce instead of queuing its own full-store
	// serialization. This is the lifecycle-replay / alert-storm shape that
	// previously produced one whole-store JSON write per event.
	store.saveMu.Lock()
	for i := 0; i < 100; i++ {
		store.RecordAnalysis("alert-coalesce", "analysis", map[string]interface{}{"pass": i})
	}
	store.saveMu.Unlock()

	waitForCompletedSaves(t, store, 1)
	if saves := store.savesCompleted.Load(); saves != 1 {
		t.Fatalf("saves completed after fully queued burst = %d, want 1", saves)
	}

	// A mutation after the coalesced save queues exactly one follow-up save.
	store.RecordAnalysis("alert-coalesce", "follow-up", nil)
	waitForCompletedSaves(t, store, 2)

	reloaded := NewIncidentStore(IncidentStoreConfig{DataDir: dir})
	timeline := reloaded.GetTimelineByAlertIdentifier("alert-coalesce")
	if timeline == nil {
		t.Fatal("coalesced saves did not persist the incident")
	}
	if len(timeline.Events) != 101 {
		t.Fatalf("persisted events = %d, want all 101 mutations captured", len(timeline.Events))
	}
}

func TestIncidentStoreFlushJoinsQueuedSave(t *testing.T) {
	dir := t.TempDir()
	store := NewIncidentStore(IncidentStoreConfig{DataDir: dir})

	// Hold the save lock so the save queued by the mutation cannot start. This
	// is the window where TempDir cleanup used to race it: the mutation has
	// returned, but the goroutine has not yet created its temp file.
	store.saveMu.Lock()
	store.RecordAnalysis("alert-flush", "analysis", nil)

	flushed := make(chan struct{})
	go func() {
		store.flush()
		close(flushed)
	}()
	select {
	case <-flushed:
		store.saveMu.Unlock()
		t.Fatal("flush returned while a queued save had not run")
	case <-time.After(50 * time.Millisecond):
	}

	store.saveMu.Unlock()
	select {
	case <-flushed:
	case <-time.After(5 * time.Second):
		t.Fatal("flush did not return after the queued save ran")
	}
	if saves := store.savesCompleted.Load(); saves != 1 {
		t.Fatalf("saves completed when flush returned = %d, want 1", saves)
	}
	if _, err := os.Stat(store.filePath + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind after flush: %v", err)
	}
}
