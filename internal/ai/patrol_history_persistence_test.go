package ai

import (
	"testing"
	"time"
)

// mockPatrolHistoryPersistence implements PatrolHistoryPersistence for testing
type mockPatrolHistoryPersistence struct {
	runs      []PatrolRunRecord
	saveErr   error
	loadErr   error
	saveCalls int
	loadCalls int
}

func (m *mockPatrolHistoryPersistence) SavePatrolRunHistory(runs []PatrolRunRecord) error {
	m.saveCalls++
	if m.saveErr != nil {
		return m.saveErr
	}
	m.runs = runs
	return nil
}

func (m *mockPatrolHistoryPersistence) LoadPatrolRunHistory() ([]PatrolRunRecord, error) {
	m.loadCalls++
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

func TestPatrolRunHistoryStore_FlushPersistence(t *testing.T) {
	store := NewPatrolRunHistoryStore(10)

	mockPersistence := &mockPatrolHistoryPersistence{}
	store.SetPersistence(mockPersistence)

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
	store.SetPersistence(mockPersistence)
	mockPersistence.saveCalls = 0 // Reset after SetPersistence load

	// Add a run (triggers scheduleSave)
	store.Add(PatrolRunRecord{ID: "run-1"})

	// Wait for debounce
	time.Sleep(100 * time.Millisecond)

	if mockPersistence.saveCalls < 1 {
		t.Errorf("Expected at least 1 save call after debounce, got %d", mockPersistence.saveCalls)
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
