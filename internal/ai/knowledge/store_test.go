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
