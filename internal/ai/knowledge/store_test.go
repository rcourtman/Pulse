package knowledge

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFormatAllForContext_LimitsOutput(t *testing.T) {
	// Create a temp directory for the test
	tmpDir, err := os.MkdirTemp("", "knowledge-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store without encryption for simplicity
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Add more than maxGuests (10) guests with notes
	for i := 0; i < 15; i++ {
		guestID := filepath.Base(tmpDir) + "-guest-" + string(rune('A'+i))
		err := store.SaveNote(guestID, "Guest-"+string(rune('A'+i)), "vm", "service", "Web Server", "http://example.com:8080")
		if err != nil {
			t.Fatalf("Failed to save note: %v", err)
		}
		// Small delay to ensure different UpdatedAt times
		time.Sleep(10 * time.Millisecond)
	}

	// Get the formatted context
	result := store.FormatAllForContext()

	if result == "" {
		t.Fatal("Expected non-empty result")
	}

	// Should mention it's truncated (15 guests but only 10 included)
	if !contains(result, "/") {
		t.Error("Expected truncation indicator (e.g., '10/15 notes') in output")
	}

	// Count how many guest sections we have (look for "### Guest-")
	guestCount := countOccurrences(result, "### Guest-")
	if guestCount > 10 {
		t.Errorf("Expected at most 10 guests, got %d", guestCount)
	}

	// Should be under 8KB
	if len(result) > 8500 {
		t.Errorf("Result too large: %d bytes (expected < 8500)", len(result))
	}
}

func TestFormatAllForContext_PrioritizesRecent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "knowledge-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Create an old guest first
	err = store.SaveNote("old-guest", "OldGuest", "vm", "config", "Setting", "old value")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)

	// Create a newer guest
	err = store.SaveNote("new-guest", "NewGuest", "vm", "config", "Setting", "new value")
	if err != nil {
		t.Fatal(err)
	}

	result := store.FormatAllForContext()

	// NewGuest should appear before OldGuest (more recently updated)
	newIdx := indexOf(result, "NewGuest")
	oldIdx := indexOf(result, "OldGuest")

	if newIdx == -1 || oldIdx == -1 {
		t.Fatalf("Both guests should be in result. newIdx=%d, oldIdx=%d", newIdx, oldIdx)
	}

	if newIdx > oldIdx {
		t.Error("Expected NewGuest to appear before OldGuest (more recent)")
	}
}

func TestFormatAllForContext_ByteLimit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "knowledge-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Create notes with very large content to trigger byte limit
	largeContent := make([]byte, 2000)
	for i := range largeContent {
		largeContent[i] = 'x'
	}

	for i := 0; i < 10; i++ {
		guestID := "large-guest-" + string(rune('A'+i))
		err := store.SaveNote(guestID, "LargeGuest-"+string(rune('A'+i)), "vm", "learning", "Big Note", string(largeContent))
		if err != nil {
			t.Fatal(err)
		}
	}

	result := store.FormatAllForContext()

	// Should be capped at ~8KB (with some tolerance for headers)
	if len(result) > 9000 {
		t.Errorf("Result should be capped at ~8KB, got %d bytes", len(result))
	}
}

// Helper functions
func contains(s, substr string) bool {
	return indexOf(s, substr) != -1
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func countOccurrences(s, substr string) int {
	count := 0
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			count++
		}
	}
	return count
}

func TestNewStore(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	if store == nil {
		t.Fatal("Expected non-nil store")
	}
}

func TestGetKnowledge_NotExists(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Get knowledge for non-existent guest
	knowledge, err := store.GetKnowledge("non-existent-guest")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should return empty knowledge (not nil)
	if knowledge == nil {
		t.Fatal("Expected empty knowledge, got nil")
	}
	if len(knowledge.Notes) != 0 {
		t.Errorf("Expected 0 notes, got %d", len(knowledge.Notes))
	}
}

func TestGetKnowledge_AfterSave(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Save a note
	err = store.SaveNote("test-guest", "TestGuest", "vm", "service", "WebServer", "http://localhost")
	if err != nil {
		t.Fatal(err)
	}

	// Get knowledge
	knowledge, err := store.GetKnowledge("test-guest")
	if err != nil {
		t.Fatal(err)
	}

	if len(knowledge.Notes) != 1 {
		t.Errorf("Expected 1 note, got %d", len(knowledge.Notes))
	}
	if knowledge.GuestName != "TestGuest" {
		t.Errorf("Expected guest name 'TestGuest', got '%s'", knowledge.GuestName)
	}
}

func TestFormatForContext_SingleGuest(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Save notes
	err = store.SaveNote("test-guest", "TestGuest", "vm", "service", "WebServer", "nginx on port 80")
	if err != nil {
		t.Fatal(err)
	}

	context := store.FormatForContext("test-guest")

	if context == "" {
		t.Error("Expected non-empty context")
	}
	if !contains(context, "nginx") {
		t.Errorf("Expected context to contain 'nginx', got: %s", context)
	}
}

func TestDeleteNote(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Save a note
	err = store.SaveNote("test-guest", "TestGuest", "vm", "config", "Setting", "value")
	if err != nil {
		t.Fatal(err)
	}

	// Get knowledge to find note ID
	knowledge, _ := store.GetKnowledge("test-guest")
	if len(knowledge.Notes) == 0 {
		t.Fatal("Expected notes to be saved")
	}
	noteID := knowledge.Notes[0].ID

	// Delete the note
	err = store.DeleteNote("test-guest", noteID)
	if err != nil {
		t.Fatalf("Failed to delete note: %v", err)
	}

	// Verify note is deleted
	knowledge, _ = store.GetKnowledge("test-guest")
	if len(knowledge.Notes) != 0 {
		t.Errorf("Expected 0 notes after delete, got %d", len(knowledge.Notes))
	}
}

func TestGetNotesByCategory(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Save multiple notes with different categories
	_ = store.SaveNote("test-guest", "TestGuest", "vm", "service", "WebServer", "nginx")
	_ = store.SaveNote("test-guest", "TestGuest", "vm", "config", "Setting", "value")
	_ = store.SaveNote("test-guest", "TestGuest", "vm", "service", "Database", "postgres")

	// Get service notes
	serviceNotes, err := store.GetNotesByCategory("test-guest", "service")
	if err != nil {
		t.Fatal(err)
	}
	if len(serviceNotes) != 2 {
		t.Errorf("Expected 2 service notes, got %d", len(serviceNotes))
	}

	// Get config notes
	configNotes, err := store.GetNotesByCategory("test-guest", "config")
	if err != nil {
		t.Fatal(err)
	}
	if len(configNotes) != 1 {
		t.Errorf("Expected 1 config note, got %d", len(configNotes))
	}
}

func TestListGuests(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Save notes for different guests
	_ = store.SaveNote("guest-1", "Guest1", "vm", "service", "Note", "content")
	_ = store.SaveNote("guest-2", "Guest2", "vm", "service", "Note", "content")

	guests, err := store.ListGuests()
	if err != nil {
		t.Fatal(err)
	}
	if len(guests) != 2 {
		t.Errorf("Expected 2 guests, got %d", len(guests))
	}
}
