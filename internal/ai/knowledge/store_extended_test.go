package knowledge

import (
	"os"
	"testing"
)

// Additional tests to improve coverage

func TestNewStore_InvalidDir(t *testing.T) {
	// Test with a file as directory (should work, creates subdir)
	store, err := NewStore("/nonexistent/path/that/should/fail")
	if store == nil && err != nil {
		// Expected - can't create in nonexistent path
		t.Log("Store creation failed as expected for nonexistent path")
	}
}

func TestStore_GetKnowledge_Empty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "knowledge-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Get knowledge for non-existent guest
	knowledge, err := store.GetKnowledge("nonexistent")
	if err != nil {
		t.Fatalf("GetKnowledge should not error for non-existent guest: %v", err)
	}

	// Should return empty knowledge
	if knowledge != nil && len(knowledge.Notes) > 0 {
		t.Error("Expected empty knowledge for non-existent guest")
	}
}

func TestStore_SaveNote_Basic(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "knowledge-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Save a note
	err = store.SaveNote("vm-100", "web-server", "vm", "config", "Database", "PostgreSQL 15")
	if err != nil {
		t.Fatalf("Failed to save note: %v", err)
	}

	// Verify it's retrievable
	knowledge, err := store.GetKnowledge("vm-100")
	if err != nil {
		t.Fatalf("Failed to get knowledge: %v", err)
	}

	if knowledge == nil {
		t.Fatal("Expected knowledge to be non-nil")
	}

	if len(knowledge.Notes) != 1 {
		t.Fatalf("Expected 1 note, got %d", len(knowledge.Notes))
	}

	note := knowledge.Notes[0]
	if note.Category != "config" {
		t.Errorf("Expected category 'config', got '%s'", note.Category)
	}
	if note.Title != "Database" {
		t.Errorf("Expected title 'Database', got '%s'", note.Title)
	}
	if note.Content != "PostgreSQL 15" {
		t.Errorf("Expected content 'PostgreSQL 15', got '%s'", note.Content)
	}
}

func TestStore_SaveNote_UpdateExisting(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "knowledge-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Save initial note
	err = store.SaveNote("vm-100", "web-server", "vm", "config", "Database", "PostgreSQL 14")
	if err != nil {
		t.Fatalf("Failed to save note: %v", err)
	}

	// Save another note with same title/category - should update
	err = store.SaveNote("vm-100", "web-server", "vm", "config", "Database", "PostgreSQL 15")
	if err != nil {
		t.Fatalf("Failed to update note: %v", err)
	}

	// Should still have only 1 note
	knowledge, err := store.GetKnowledge("vm-100")
	if err != nil {
		t.Fatalf("Failed to get knowledge: %v", err)
	}

	if len(knowledge.Notes) != 1 {
		t.Errorf("Expected 1 note after update, got %d", len(knowledge.Notes))
	}

	if knowledge.Notes[0].Content != "PostgreSQL 15" {
		t.Errorf("Expected updated content 'PostgreSQL 15', got '%s'", knowledge.Notes[0].Content)
	}
}

func TestStore_DeleteNote(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "knowledge-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Save a note
	err = store.SaveNote("vm-100", "web-server", "vm", "config", "Database", "PostgreSQL 15")
	if err != nil {
		t.Fatalf("Failed to save note: %v", err)
	}

	// Get the note ID
	knowledge, _ := store.GetKnowledge("vm-100")
	noteID := knowledge.Notes[0].ID

	// Delete the note
	err = store.DeleteNote("vm-100", noteID)
	if err != nil {
		t.Fatalf("Failed to delete note: %v", err)
	}

	// Verify it's deleted
	knowledge, _ = store.GetKnowledge("vm-100")
	if len(knowledge.Notes) != 0 {
		t.Errorf("Expected 0 notes after delete, got %d", len(knowledge.Notes))
	}
}

func TestStore_DeleteNote_NonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "knowledge-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Delete non-existent note - may return error for non-existent guest
	err = store.DeleteNote("nonexistent", "nonexistent-note")
	// It's ok either way - just checking it doesn't panic
	_ = err
}

func TestStore_GetNotesByCategory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "knowledge-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Save notes in different categories
	store.SaveNote("vm-100", "web-server", "vm", "config", "Database", "PostgreSQL 15")
	store.SaveNote("vm-100", "web-server", "vm", "service", "Web Server", "nginx")
	store.SaveNote("vm-100", "web-server", "vm", "config", "Cache", "Redis")

	// Get config notes only
	notes, err := store.GetNotesByCategory("vm-100", "config")
	if err != nil {
		t.Fatalf("Failed to get notes by category: %v", err)
	}

	if len(notes) != 2 {
		t.Errorf("Expected 2 config notes, got %d", len(notes))
	}

	for _, note := range notes {
		if note.Category != "config" {
			t.Errorf("Expected category 'config', got '%s'", note.Category)
		}
	}
}

func TestStore_FormatForContext_Empty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "knowledge-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Format for non-existent guest
	result := store.FormatForContext("nonexistent")

	if result != "" {
		t.Errorf("Expected empty result for non-existent guest, got: %s", result)
	}
}

func TestStore_FormatForContext_WithData(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "knowledge-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Save some notes
	store.SaveNote("vm-100", "web-server", "vm", "config", "Database", "PostgreSQL 15")
	store.SaveNote("vm-100", "web-server", "vm", "service", "Web Server", "nginx on port 80")

	// Format for context
	result := store.FormatForContext("vm-100")

	if result == "" {
		t.Error("Expected non-empty result")
	}

	if !contains(result, "PostgreSQL") {
		t.Error("Expected result to contain PostgreSQL")
	}

	if !contains(result, "nginx") {
		t.Error("Expected result to contain nginx")
	}
}

func TestStore_ListGuests(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "knowledge-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Initially empty
	guests, err := store.ListGuests()
	if err != nil {
		t.Fatalf("Failed to list guests: %v", err)
	}

	if len(guests) != 0 {
		t.Errorf("Expected 0 guests initially, got %d", len(guests))
	}

	// Add some guests
	store.SaveNote("vm-100", "web-server", "vm", "config", "DB", "PostgreSQL")
	store.SaveNote("vm-200", "db-server", "vm", "config", "DB", "MySQL")

	// List again
	guests, err = store.ListGuests()
	if err != nil {
		t.Fatalf("Failed to list guests: %v", err)
	}

	if len(guests) != 2 {
		t.Errorf("Expected 2 guests, got %d", len(guests))
	}
}

func TestStore_Persistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "knowledge-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store and save data
	store1, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	store1.SaveNote("vm-100", "web-server", "vm", "config", "Database", "PostgreSQL 15")

	// Create new store in same dir - should load data
	store2, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create second store: %v", err)
	}

	knowledge, err := store2.GetKnowledge("vm-100")
	if err != nil {
		t.Fatalf("Failed to get knowledge from second store: %v", err)
	}

	if knowledge == nil || len(knowledge.Notes) == 0 {
		t.Error("Expected persisted note to be loaded in second store")
	}
}

func TestStore_MultipleNotes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "knowledge-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Save multiple notes with different titles
	store.SaveNote("vm-100", "web-server", "vm", "config", "Database", "PostgreSQL 15")
	store.SaveNote("vm-100", "web-server", "vm", "config", "Cache", "Redis 7")
	store.SaveNote("vm-100", "web-server", "vm", "service", "Web", "nginx")

	knowledge, err := store.GetKnowledge("vm-100")
	if err != nil {
		t.Fatalf("Failed to get knowledge: %v", err)
	}

	if len(knowledge.Notes) != 3 {
		t.Errorf("Expected 3 notes, got %d", len(knowledge.Notes))
	}
}
