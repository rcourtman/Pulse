package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewContextStore_Defaults(t *testing.T) {
	store := NewContextStore(ContextStoreConfig{})
	if store.config.MaxMemoriesPerType != 1000 {
		t.Fatalf("expected MaxMemoriesPerType default, got %d", store.config.MaxMemoriesPerType)
	}
	if store.config.MaxResourceNotes != 20 {
		t.Fatalf("expected MaxResourceNotes default, got %d", store.config.MaxResourceNotes)
	}
	if store.config.RetentionDays != 90 {
		t.Fatalf("expected RetentionDays default, got %d", store.config.RetentionDays)
	}
	if store.config.RelevanceDecayDays != 7 {
		t.Fatalf("expected RelevanceDecayDays default, got %d", store.config.RelevanceDecayDays)
	}
}

func TestRemember_AddsMemoryAndResourceNote(t *testing.T) {
	store := NewContextStore(ContextStoreConfig{MaxResourceNotes: 2})

	id1 := store.Remember("vm-1", "note-1", "user", MemoryTypeResource)
	id2 := store.Remember("vm-1", "note-1", "user", MemoryTypeResource)
	if id1 == id2 {
		t.Fatalf("expected unique memory IDs for separate remembers")
	}

	mem := store.GetResourceMemory("vm-1")
	if mem == nil || len(mem.Notes) != 1 {
		t.Fatalf("expected duplicate notes to be ignored")
	}

	store.AddResourceNote("vm-1", "", "", "note-2")
	store.AddResourceNote("vm-1", "", "", "note-3")
	mem = store.GetResourceMemory("vm-1")
	if len(mem.Notes) != 2 {
		t.Fatalf("expected notes trimmed to max, got %d", len(mem.Notes))
	}
	if mem.Notes[0] != "note-2" {
		t.Fatalf("expected oldest note to be trimmed")
	}
}

func TestAddResourceNote_UpdatesMetadata(t *testing.T) {
	store := NewContextStore(ContextStoreConfig{})
	store.AddResourceNote("vm-1", "web-1", "vm", "note")

	mem := store.GetResourceMemory("vm-1")
	if mem == nil {
		t.Fatalf("expected resource memory to exist")
	}
	if mem.ResourceName != "web-1" || mem.ResourceType != "vm" {
		t.Fatalf("expected metadata to be stored")
	}
}

func TestAddIncidentMemory_CreatesMemory(t *testing.T) {
	store := NewContextStore(ContextStoreConfig{})

	store.AddIncidentMemory(&IncidentMemory{
		ResourceID: "vm-1",
		Summary:    "high cpu",
		RootCause:  "runaway process",
		Resolution: "restarted service",
	})

	incidents := store.GetRecentIncidents(10)
	if len(incidents) != 1 {
		t.Fatalf("expected incident to be stored")
	}

	store.mu.RLock()
	defer store.mu.RUnlock()
	if len(store.memories[MemoryTypeIncident]) != 1 {
		t.Fatalf("expected incident memory to be created")
	}
	for _, mem := range store.memories[MemoryTypeIncident] {
		if mem.Content == "" || mem.ResourceID != "vm-1" {
			t.Fatalf("expected incident memory content to be populated")
		}
	}
}

func TestAddPatternMemory_Deduplicates(t *testing.T) {
	store := NewContextStore(ContextStoreConfig{})

	store.AddPatternMemory(&PatternMemory{
		Pattern:     "cpu spike at backup",
		Description: "backup time spikes CPU",
		Occurrences: 2,
	})
	store.AddPatternMemory(&PatternMemory{
		Pattern:     "cpu spike at backup",
		Description: "same pattern",
		Occurrences: 1,
	})

	patterns := store.GetPatterns(0)
	if len(patterns) != 1 {
		t.Fatalf("expected pattern to deduplicate")
	}
	if patterns[0].Occurrences != 3 {
		t.Fatalf("expected occurrences to be incremented, got %d", patterns[0].Occurrences)
	}
	if patterns[0].Confidence == 0 {
		t.Fatalf("expected confidence to be set")
	}
}

func TestRecallAndRecallByType_SortsAndMarksUsed(t *testing.T) {
	store := NewContextStore(ContextStoreConfig{})

	store.mu.Lock()
	store.memories[MemoryTypeResource]["m1"] = &Memory{
		ID:         "m1",
		Type:       MemoryTypeResource,
		ResourceID: "vm-1",
		Relevance:  0.5,
		UseCount:   1,
		LastUsed:   time.Now().Add(-24 * time.Hour),
	}
	store.memories[MemoryTypeResource]["m2"] = &Memory{
		ID:         "m2",
		Type:       MemoryTypeResource,
		ResourceID: "vm-1",
		Relevance:  0.9,
		UseCount:   1,
		LastUsed:   time.Now().Add(-24 * time.Hour),
	}
	store.mu.Unlock()

	memories := store.Recall("vm-1")
	if len(memories) != 2 {
		t.Fatalf("expected 2 memories, got %d", len(memories))
	}
	if memories[0].ID != "m2" {
		t.Fatalf("expected higher relevance memory first")
	}

	store.mu.RLock()
	defer store.mu.RUnlock()
	if store.memories[MemoryTypeResource]["m1"].UseCount != 2 {
		t.Fatalf("expected use count increment")
	}
	if store.memories[MemoryTypeResource]["m1"].Relevance <= 0.5 {
		t.Fatalf("expected relevance to increase")
	}
}

func TestGetPatterns_FilterAndSort(t *testing.T) {
	store := NewContextStore(ContextStoreConfig{})

	store.mu.Lock()
	store.patternMemories["p1"] = &PatternMemory{ID: "p1", Pattern: "a", Confidence: 0.4}
	store.patternMemories["p2"] = &PatternMemory{ID: "p2", Pattern: "b", Confidence: 0.9}
	store.patternMemories["p3"] = &PatternMemory{ID: "p3", Pattern: "c", Confidence: 0.7}
	store.mu.Unlock()

	patterns := store.GetPatterns(0.5)
	if len(patterns) != 2 {
		t.Fatalf("expected 2 patterns above threshold, got %d", len(patterns))
	}
	if patterns[0].Confidence < patterns[1].Confidence {
		t.Fatalf("expected patterns sorted by confidence desc")
	}
}

func TestDecayRelevance(t *testing.T) {
	store := NewContextStore(ContextStoreConfig{RelevanceDecayDays: 1})

	store.mu.Lock()
	store.memories[MemoryTypeResource]["m1"] = &Memory{
		ID:         "m1",
		Type:       MemoryTypeResource,
		ResourceID: "vm-1",
		Relevance:  0.2,
		LastUsed:   time.Now().Add(-30 * 24 * time.Hour),
	}
	store.mu.Unlock()

	store.DecayRelevance()
	store.mu.RLock()
	defer store.mu.RUnlock()
	if store.memories[MemoryTypeResource]["m1"].Relevance != 0.1 {
		t.Fatalf("expected relevance to decay to minimum, got %.2f", store.memories[MemoryTypeResource]["m1"].Relevance)
	}
}

func TestCleanup_RemovesOldAndTrims(t *testing.T) {
	store := NewContextStore(ContextStoreConfig{
		MaxMemoriesPerType: 1,
		RetentionDays:      1,
	})

	store.mu.Lock()
	store.memories[MemoryTypeResource]["old"] = &Memory{
		ID:        "old",
		Type:      MemoryTypeResource,
		CreatedAt: time.Now().Add(-48 * time.Hour),
		Relevance: 0.9,
	}
	store.memories[MemoryTypeResource]["low"] = &Memory{
		ID:        "low",
		Type:      MemoryTypeResource,
		CreatedAt: time.Now(),
		Relevance: 0.05,
	}
	store.memories[MemoryTypeResource]["new"] = &Memory{
		ID:        "new",
		Type:      MemoryTypeResource,
		CreatedAt: time.Now(),
		Relevance: 0.8,
	}
	store.mu.Unlock()

	removed := store.Cleanup()
	if removed == 0 {
		t.Fatalf("expected cleanup to remove memories")
	}

	store.mu.RLock()
	defer store.mu.RUnlock()
	if len(store.memories[MemoryTypeResource]) != 1 {
		t.Fatalf("expected memories trimmed to max")
	}
}

func TestFormatForPatrolAndResource(t *testing.T) {
	store := NewContextStore(ContextStoreConfig{})
	store.AddResourceNote("vm-1", "vm-one", "vm", "note-1")
	store.AddPatternMemory(&PatternMemory{
		Pattern:     "pattern",
		Description: "pattern desc",
		Occurrences: 5,
	})
	store.AddIncidentMemory(&IncidentMemory{
		ResourceID: "vm-1",
		Summary:    "disk full",
	})

	resource := store.GetResourceMemory("vm-1")
	resource.Patterns = []string{"daily spike"}
	store.mu.Lock()
	store.resourceMemories["vm-1"] = resource
	store.mu.Unlock()

	context := store.FormatForPatrol()
	if context == "" || !containsStr(context, "Resource Notes") || !containsStr(context, "Recent Incidents") {
		t.Fatalf("expected patrol context to include sections")
	}

	resourceContext := store.FormatForResource("vm-1")
	if resourceContext == "" || !containsStr(resourceContext, "Observed patterns") {
		t.Fatalf("expected resource context to include patterns")
	}
}

func TestContextStore_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewContextStore(ContextStoreConfig{DataDir: dir})

	store.Remember("vm-1", "note-1", "user", MemoryTypeResource)
	store.AddPatternMemory(&PatternMemory{
		Pattern:     "pattern",
		Description: "pattern desc",
		Occurrences: 3,
	})
	if err := store.ForceSave(); err != nil {
		t.Fatalf("force save failed: %v", err)
	}

	loaded := NewContextStore(ContextStoreConfig{DataDir: dir})
	if loaded == nil {
		t.Fatalf("expected store to load from disk")
	}
	if len(loaded.memories[MemoryTypeResource]) == 0 {
		t.Fatalf("expected memories to load")
	}
	if len(loaded.patternMemories) == 0 {
		t.Fatalf("expected patterns to load")
	}

	// Validate saved file exists and is JSON.
	data, err := os.ReadFile(filepath.Join(dir, "ai_context.json"))
	if err != nil {
		t.Fatalf("expected context file to exist: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("expected valid json, got %v", err)
	}
}
