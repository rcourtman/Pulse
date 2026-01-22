package providers

import (
	"context"
	"testing"
	"time"
)

func TestNotableModelsCache_Refresh(t *testing.T) {
	// Test that we can successfully fetch and parse models.dev API
	cache := NewNotableModelsCache(modelsDevAPIURL, 1*time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := cache.Refresh(ctx)
	if err != nil {
		t.Fatalf("Failed to refresh cache: %v", err)
	}

	if len(cache.data) == 0 {
		t.Fatal("Cache is empty after refresh")
	}

	t.Logf("Cache contains %d models", len(cache.data))

	// Check for some known models
	testCases := []struct {
		provider string
		modelID  string
	}{
		{"anthropic", "claude-opus-4-5"},
		{"anthropic", "claude-sonnet-4-5"},
		{"openai", "gpt-4o"},
	}

	for _, tc := range testCases {
		key := normalizeKey(tc.provider, tc.modelID)
		if info, found := cache.data[key]; found {
			t.Logf("Found %s: release_date=%s, last_updated=%s", key, info.ReleaseDate, info.LastUpdated)
		} else {
			t.Logf("Model %s not found in cache (key=%s)", tc.modelID, key)
			// Try fuzzy match
			fuzzyKey := normalizeModelID(tc.modelID)
			if info, found := cache.data[fuzzyKey]; found {
				t.Logf("Found via fuzzy match %s: release_date=%s", fuzzyKey, info.ReleaseDate)
			}
		}
	}
}

func TestIsNotable_Integration(t *testing.T) {
	cache := GetNotableCache()

	// Test Ollama - should always be notable
	if !cache.IsNotable("ollama", "llama3", 0) {
		t.Error("Ollama models should always be notable")
	}

	// Test Anthropic models
	// claude-opus-4-5 was released 2025-11-21, should be notable
	notable := cache.IsNotable("anthropic", "claude-opus-4-5", 0)
	t.Logf("claude-opus-4-5 notable: %v", notable)

	// claude-3-5-sonnet-20241022 was released 2024-10-22, should NOT be notable (>6 months old)
	notable = cache.IsNotable("anthropic", "claude-3-5-sonnet-20241022", 0)
	t.Logf("claude-3-5-sonnet-20241022 notable: %v", notable)

	// Print cache size
	cache.mu.RLock()
	t.Logf("Cache size: %d", len(cache.data))
	cache.mu.RUnlock()
}

func TestGetModelFamilyAliases(t *testing.T) {
	tests := []struct {
		model string
		want  []string
	}{
		{"claude-3-5-haiku", []string{"claude-haiku-4-5", "claude-3-5-haiku-latest"}},
		{"claude-3-5-sonnet", []string{"claude-sonnet-4-5", "claude-3-5-sonnet-latest"}},
		{"claude-3-opus", []string{"claude-opus-4-5", "claude-opus-4-1", "claude-opus-4-0"}},
		{"gpt-4o", []string{"gpt-4o", "gpt-4o-2024-11-20"}},
		{"o1-mini", []string{"o1", "o1-pro", "o3-mini"}},
		{"unknown", nil},
	}

	for _, tt := range tests {
		got := getModelFamilyAliases(tt.model)
		if len(tt.want) == 0 && len(got) != 0 {
			t.Fatalf("model %s: expected no aliases, got %+v", tt.model, got)
		}
		if len(tt.want) > 0 && len(got) != len(tt.want) {
			t.Fatalf("model %s: expected %d aliases, got %d", tt.model, len(tt.want), len(got))
		}
	}
}

func TestParseFlexibleDate_Invalid(t *testing.T) {
	_, err := parseFlexibleDate("not-a-date")
	if err == nil {
		t.Fatal("expected error for invalid date")
	}
}

func TestIsRecentlyReleased(t *testing.T) {
	if !isRecentlyReleased("2100-01-01", "") {
		t.Fatal("expected future release date to be notable")
	}
	if isRecentlyReleased("2000-01-01", "") {
		t.Fatal("expected old release date to be not notable")
	}
}
