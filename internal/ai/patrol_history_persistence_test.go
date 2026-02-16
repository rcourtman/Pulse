package ai

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// mockPatrolHistoryPersistence implements PatrolHistoryPersistence for testing
type mockPatrolHistoryPersistence struct {
	mu        sync.Mutex
	runs      []PatrolRunRecord
	saveErr   error
	loadErr   error
	saveCalls atomic.Int32
	loadCalls atomic.Int32
}

type errorPatrolHistoryPersistence struct {
	err error
}

func (e *errorPatrolHistoryPersistence) SavePatrolRunHistory(runs []PatrolRunRecord) error {
	return e.err
}

func (e *errorPatrolHistoryPersistence) LoadPatrolRunHistory() ([]PatrolRunRecord, error) {
	return nil, nil
}

func (m *mockPatrolHistoryPersistence) SavePatrolRunHistory(runs []PatrolRunRecord) error {
	m.saveCalls.Add(1)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveErr != nil {
		return m.saveErr
	}
	m.runs = runs
	return nil
}

func (m *mockPatrolHistoryPersistence) LoadPatrolRunHistory() ([]PatrolRunRecord, error) {
	m.loadCalls.Add(1)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	return m.runs, nil
}

func TestNewPatrolRunHistoryStore(t *testing.T) {
	// Test with positive maxRuns
	store := NewPatrolRunHistoryStore(50)
	if store == nil {
		t.Fatal("Expected non-nil store")
	}
	if store.maxRuns != 50 {
		t.Errorf("Expected maxRuns=50, got %d", store.maxRuns)
	}

	// Test with zero maxRuns (should use default)
	storeDefault := NewPatrolRunHistoryStore(0)
	if storeDefault.maxRuns != MaxPatrolRunHistory {
		t.Errorf("Expected maxRuns=%d (default), got %d", MaxPatrolRunHistory, storeDefault.maxRuns)
	}

	// Test with negative maxRuns (should use default)
	storeNegative := NewPatrolRunHistoryStore(-10)
	if storeNegative.maxRuns != MaxPatrolRunHistory {
		t.Errorf("Expected maxRuns=%d (default), got %d", MaxPatrolRunHistory, storeNegative.maxRuns)
	}
}

func TestPatrolRunHistoryStore_Add(t *testing.T) {
	store := NewPatrolRunHistoryStore(3)

	run1 := PatrolRunRecord{ID: "run-1", StartedAt: time.Now()}
	run2 := PatrolRunRecord{ID: "run-2", StartedAt: time.Now()}
	run3 := PatrolRunRecord{ID: "run-3", StartedAt: time.Now()}
	run4 := PatrolRunRecord{ID: "run-4", StartedAt: time.Now()}

	store.Add(run1)
	if store.Count() != 1 {
		t.Errorf("Expected count=1, got %d", store.Count())
	}

	store.Add(run2)
	store.Add(run3)
	if store.Count() != 3 {
		t.Errorf("Expected count=3, got %d", store.Count())
	}

	// Adding 4th run should trim to maxRuns
	store.Add(run4)
	if store.Count() != 3 {
		t.Errorf("Expected count=3 (trimmed), got %d", store.Count())
	}

	// Newest should be first
	runs := store.GetAll()
	if runs[0].ID != "run-4" {
		t.Errorf("Expected newest run first, got %s", runs[0].ID)
	}
}

func TestPatrolRunHistoryStore_GetAll(t *testing.T) {
	store := NewPatrolRunHistoryStore(10)

	// Empty store
	runs := store.GetAll()
	if len(runs) != 0 {
		t.Errorf("Expected empty slice, got %d runs", len(runs))
	}

	// Add runs
	store.Add(PatrolRunRecord{ID: "run-1"})
	store.Add(PatrolRunRecord{ID: "run-2"})

	runs = store.GetAll()
	if len(runs) != 2 {
		t.Errorf("Expected 2 runs, got %d", len(runs))
	}

	// Verify it returns a copy (modification shouldn't affect store)
	runs[0].ID = "modified"
	storedRuns := store.GetAll()
	if storedRuns[0].ID == "modified" {
		t.Error("GetAll should return a copy, not the original slice")
	}
}

func TestPatrolRunHistoryStore_GetRecent(t *testing.T) {
	store := NewPatrolRunHistoryStore(10)

	store.Add(PatrolRunRecord{ID: "run-1"})
	store.Add(PatrolRunRecord{ID: "run-2"})
	store.Add(PatrolRunRecord{ID: "run-3"})

	// Get 2 recent
	runs := store.GetRecent(2)
	if len(runs) != 2 {
		t.Errorf("Expected 2 runs, got %d", len(runs))
	}
	if runs[0].ID != "run-3" {
		t.Errorf("Expected run-3 first, got %s", runs[0].ID)
	}

	// Get more than available
	runsAll := store.GetRecent(10)
	if len(runsAll) != 3 {
		t.Errorf("Expected 3 runs (all available), got %d", len(runsAll))
	}

	// Get 0 or negative
	runsZero := store.GetRecent(0)
	if len(runsZero) != 3 {
		t.Errorf("Expected 3 runs for n=0, got %d", len(runsZero))
	}

	runsNeg := store.GetRecent(-5)
	if len(runsNeg) != 3 {
		t.Errorf("Expected 3 runs for n=-5, got %d", len(runsNeg))
	}
}

func TestPatrolRunHistoryStore_Count(t *testing.T) {
	store := NewPatrolRunHistoryStore(10)

	if store.Count() != 0 {
		t.Errorf("Expected count=0, got %d", store.Count())
	}

	store.Add(PatrolRunRecord{ID: "run-1"})
	if store.Count() != 1 {
		t.Errorf("Expected count=1, got %d", store.Count())
	}

	store.Add(PatrolRunRecord{ID: "run-2"})
	store.Add(PatrolRunRecord{ID: "run-3"})
	if store.Count() != 3 {
		t.Errorf("Expected count=3, got %d", store.Count())
	}
}

func TestPatrolRunHistoryStore_SetPersistence(t *testing.T) {
	store := NewPatrolRunHistoryStore(10)

	// Create mock with existing runs
	mockPersistence := &mockPatrolHistoryPersistence{
		runs: []PatrolRunRecord{
			{ID: "persisted-1"},
			{ID: "persisted-2"},
		},
	}

	err := store.SetPersistence(mockPersistence)
	if err != nil {
		t.Fatalf("SetPersistence failed: %v", err)
	}

	// Should have loaded runs
	if store.Count() != 2 {
		t.Errorf("Expected 2 runs loaded, got %d", store.Count())
	}

	runs := store.GetAll()
	if runs[0].ID != "persisted-1" {
		t.Errorf("Expected persisted-1, got %s", runs[0].ID)
	}
}

func TestPatrolRunHistoryStore_PersistenceStatusAndErrors(t *testing.T) {
	store := NewPatrolRunHistoryStore(5)
	store.saveDebounce = 0

	saveErr := errors.New("save failed")
	errPersistence := &errorPatrolHistoryPersistence{err: saveErr}

	if err := store.SetPersistence(errPersistence); err != nil {
		t.Fatalf("SetPersistence failed: %v", err)
	}

	errCh := make(chan error, 1)
	store.SetOnSaveError(func(err error) {
		errCh <- err
	})

	store.Add(PatrolRunRecord{ID: "run-1"})

	select {
	case err := <-errCh:
		if err == nil || err.Error() != "save failed" {
			t.Fatalf("unexpected error callback: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for save error callback")
	}

	lastErr, lastSaveTime, hasPersistence := store.GetPersistenceStatus()
	if lastErr == nil {
		t.Fatal("expected last save error to be recorded")
	}
	if !lastSaveTime.IsZero() {
		t.Fatalf("expected last save time to be zero on failure, got %v", lastSaveTime)
	}
	if !hasPersistence {
		t.Fatal("expected persistence to be configured")
	}
}

func TestConvertPatrolToolCalls(t *testing.T) {
	aiCalls := []ToolCallRecord{
		{
			ID:        "call-1",
			ToolName:  "pulse_query",
			Input:     `{"action":"get"}`,
			Output:    `{"status":"ok"}`,
			Success:   true,
			StartTime: 123,
			EndTime:   456,
			Duration:  333,
		},
	}

	cfgCalls := convertAIToolCallsToConfig(aiCalls)
	if len(cfgCalls) != 1 {
		t.Fatalf("expected 1 config tool call, got %d", len(cfgCalls))
	}
	if cfgCalls[0].ToolName != "pulse_query" || cfgCalls[0].Duration != 333 {
		t.Fatalf("unexpected config tool call: %+v", cfgCalls[0])
	}

	roundTrip := convertConfigToolCallsToAI(cfgCalls)
	if len(roundTrip) != 1 {
		t.Fatalf("expected 1 AI tool call, got %d", len(roundTrip))
	}
	if roundTrip[0].ID != "call-1" || roundTrip[0].Output != `{"status":"ok"}` {
		t.Fatalf("unexpected AI tool call: %+v", roundTrip[0])
	}
}

func TestPatrolRunHistoryStore_SetPersistence_Nil(t *testing.T) {
	store := NewPatrolRunHistoryStore(10)

	// Setting nil persistence should not error
	err := store.SetPersistence(nil)
	if err != nil {
		t.Fatalf("SetPersistence(nil) should not error: %v", err)
	}
}

func TestPatrolRunHistoryStore_SetPersistence_TrimToMax(t *testing.T) {
	store := NewPatrolRunHistoryStore(2) // Only allow 2 runs

	// Create mock with more runs than maxRuns
	mockPersistence := &mockPatrolHistoryPersistence{
		runs: []PatrolRunRecord{
			{ID: "run-1"},
			{ID: "run-2"},
			{ID: "run-3"},
			{ID: "run-4"},
		},
	}

	err := store.SetPersistence(mockPersistence)
	if err != nil {
		t.Fatalf("SetPersistence failed: %v", err)
	}

	// Should have trimmed to max
	if store.Count() != 2 {
		t.Errorf("Expected 2 runs (trimmed), got %d", store.Count())
	}
}

func TestPatrolRunHistoryStore_SetPersistence_LoadError(t *testing.T) {
	store := NewPatrolRunHistoryStore(10)
	mockPersistence := &mockPatrolHistoryPersistence{
		loadErr: errors.New("load failed"),
	}

	err := store.SetPersistence(mockPersistence)
	if err == nil {
		t.Fatal("expected error from SetPersistence when load fails")
	}
}

func TestPatrolRunHistoryStore_FlushPersistence(t *testing.T) {
	store := NewPatrolRunHistoryStore(10)

	mockPersistence := &mockPatrolHistoryPersistence{}
	_ = store.SetPersistence(mockPersistence)

	store.Add(PatrolRunRecord{ID: "run-1"})
	store.Add(PatrolRunRecord{ID: "run-2"})

	err := store.FlushPersistence()
	if err != nil {
		t.Fatalf("FlushPersistence failed: %v", err)
	}

	// Should have saved
	if len(mockPersistence.runs) != 2 {
		t.Errorf("Expected 2 runs saved, got %d", len(mockPersistence.runs))
	}
}

func TestPatrolRunHistoryStore_FlushPersistence_NoPersistence(t *testing.T) {
	store := NewPatrolRunHistoryStore(10)

	store.Add(PatrolRunRecord{ID: "run-1"})

	// Should not error when no persistence is set
	err := store.FlushPersistence()
	if err != nil {
		t.Fatalf("FlushPersistence without persistence should not error: %v", err)
	}
}

func TestPatrolRunHistoryStore_ScheduleSave(t *testing.T) {
	store := NewPatrolRunHistoryStore(10)
	store.saveDebounce = 50 * time.Millisecond // Short debounce for testing

	mockPersistence := &mockPatrolHistoryPersistence{}
	_ = store.SetPersistence(mockPersistence)
	mockPersistence.saveCalls.Store(0) // Reset after SetPersistence load

	// Add a run (triggers scheduleSave)
	store.Add(PatrolRunRecord{ID: "run-1"})

	// Wait for debounce
	time.Sleep(100 * time.Millisecond)

	if mockPersistence.saveCalls.Load() < 1 {
		t.Errorf("Expected at least 1 save call after debounce, got %d", mockPersistence.saveCalls.Load())
	}
}

func TestPatrolRunHistoryStore_ScheduleSave_StopsExistingTimer(t *testing.T) {
	store := NewPatrolRunHistoryStore(10)
	store.saveDebounce = 50 * time.Millisecond

	mockPersistence := &mockPatrolHistoryPersistence{}
	_ = store.SetPersistence(mockPersistence)
	mockPersistence.saveCalls.Store(0)

	store.Add(PatrolRunRecord{ID: "run-1"})
	store.Add(PatrolRunRecord{ID: "run-2"})

	time.Sleep(120 * time.Millisecond)

	if mockPersistence.saveCalls.Load() != 1 {
		t.Errorf("expected 1 save call after reschedule, got %d", mockPersistence.saveCalls.Load())
	}
}

func TestPatrolRunHistoryStore_ScheduleSave_Cancelled(t *testing.T) {
	store := NewPatrolRunHistoryStore(10)
	store.saveDebounce = 50 * time.Millisecond

	mockPersistence := &mockPatrolHistoryPersistence{}
	_ = store.SetPersistence(mockPersistence)
	mockPersistence.saveCalls.Store(0)

	store.Add(PatrolRunRecord{ID: "run-1"})

	store.mu.Lock()
	store.savePending = false
	store.mu.Unlock()

	time.Sleep(120 * time.Millisecond)

	if mockPersistence.saveCalls.Load() != 0 {
		t.Errorf("expected no save calls after cancellation, got %d", mockPersistence.saveCalls.Load())
	}
}

func TestPatrolRunHistoryStore_ScheduleSave_Error(t *testing.T) {
	store := NewPatrolRunHistoryStore(10)
	store.saveDebounce = 50 * time.Millisecond

	mockPersistence := &mockPatrolHistoryPersistence{saveErr: errors.New("save failed")}
	_ = store.SetPersistence(mockPersistence)
	mockPersistence.saveCalls.Store(0)

	store.Add(PatrolRunRecord{ID: "run-1"})

	time.Sleep(120 * time.Millisecond)

	if mockPersistence.saveCalls.Load() < 1 {
		t.Errorf("expected save to be attempted, got %d calls", mockPersistence.saveCalls.Load())
	}
}

func TestPatrolRunHistoryStore_ScheduleSave_NoPersistence(t *testing.T) {
	store := NewPatrolRunHistoryStore(10)

	// Add without persistence - should not panic
	store.Add(PatrolRunRecord{ID: "run-1"})

	// Give time for any potential async operation
	time.Sleep(10 * time.Millisecond)

	// No error or panic is success
}

func TestPatrolHistoryPersistenceAdapter_SaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)

	adapter := NewPatrolHistoryPersistenceAdapter(persistence)
	if adapter == nil {
		t.Fatal("expected adapter to be created")
	}

	runs := []PatrolRunRecord{
		{
			ID:               "run-1",
			StartedAt:        time.Now().Add(-2 * time.Minute),
			CompletedAt:      time.Now().Add(-1 * time.Minute),
			Duration:         time.Minute,
			Type:             "manual",
			ResourcesChecked: 10,
			NodesChecked:     2,
			GuestsChecked:    5,
			DockerChecked:    1,
			StorageChecked:   1,
			HostsChecked:     0,
			PBSChecked:       0,
			NewFindings:      1,
			ExistingFindings: 2,
			ResolvedFindings: 1,
			AutoFixCount:     0,
			FindingsSummary:  "summary",
			FindingIDs:       []string{"f1", "f2"},
			ErrorCount:       0,
			Status:           "ok",
			AIAnalysis:       "analysis",
			InputTokens:      100,
			OutputTokens:     200,
		},
	}

	if err := adapter.SavePatrolRunHistory(runs); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := adapter.LoadPatrolRunHistory()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(loaded) != 1 || loaded[0].ID != "run-1" {
		t.Fatalf("unexpected loaded runs: %+v", loaded)
	}
	if loaded[0].Duration != runs[0].Duration {
		t.Fatalf("expected duration %v, got %v", runs[0].Duration, loaded[0].Duration)
	}
}

func TestPatrolHistoryPersistenceAdapter_LoadError(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	adapter := NewPatrolHistoryPersistenceAdapter(persistence)

	badPath := filepath.Join(tmp, "ai_patrol_runs.json")
	if err := os.Mkdir(badPath, 0700); err != nil {
		t.Fatalf("failed to create directory at %s: %v", badPath, err)
	}

	if _, err := adapter.LoadPatrolRunHistory(); err == nil {
		t.Fatal("expected error when patrol runs path is a directory")
	}
}
