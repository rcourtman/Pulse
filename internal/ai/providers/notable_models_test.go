package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// Exercise the real HTTP decoder without depending on external availability,
// today's catalogue, or the singleton's state from a preceding test.
func notableModelsTestCache(t *testing.T) (*NotableModelsCache, map[string]ModelsDevInfo, *atomic.Int32) {
	t.Helper()
	recent := time.Now().UTC().AddDate(0, -1, 0).Format("2006-01-02")
	old := time.Now().UTC().AddDate(-1, 0, 0).Format("2006-01-02")
	providers := map[string]ModelsDevProvider{
		"anthropic": {ID: "anthropic", Models: map[string]ModelsDevInfo{
			"claude-opus-4-5":            {ID: "claude-opus-4-5", ReleaseDate: recent, LastUpdated: recent},
			"claude-sonnet-4-5":          {ID: "claude-sonnet-4-5", ReleaseDate: recent, LastUpdated: recent},
			"claude-3-5-sonnet-20241022": {ID: "claude-3-5-sonnet-20241022", ReleaseDate: old, LastUpdated: old},
		}},
		"openai": {ID: "openai", Models: map[string]ModelsDevInfo{
			"gpt-4o": {ID: "gpt-4o", ReleaseDate: recent, LastUpdated: recent},
		}},
	}
	want := make(map[string]ModelsDevInfo)
	for provider, data := range providers {
		for model, info := range data.Models {
			want[normalizeKey(provider, model)] = info
			want[normalizeModelID(model)] = info
		}
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodGet || r.URL.RequestURI() != "/api.json" {
			t.Errorf("request = %s %s, want GET /api.json", r.Method, r.URL.RequestURI())
		}
		if got := r.Header.Get("User-Agent"); got != "Pulse/1.0" {
			t.Errorf("User-Agent = %q, want Pulse/1.0", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(providers); err != nil {
			t.Errorf("encode fixture: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return NewNotableModelsCache(server.URL+"/api.json", time.Hour), want, &calls
}

func TestNotableModelsCache_Refresh(t *testing.T) {
	cache, want, calls := notableModelsTestCache(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := cache.Refresh(ctx); err != nil {
		t.Fatalf("Failed to refresh cache: %v", err)
	}
	if len(cache.data) != len(want) {
		t.Fatalf("cache contains %d entries, want %d", len(cache.data), len(want))
	}
	for key, info := range want {
		if got, found := cache.data[key]; !found || got != info {
			t.Errorf("cache[%q] = %+v (found=%t), want %+v", key, got, found, info)
		}
	}
	if cache.lastFetched.IsZero() {
		t.Fatal("successful refresh did not record freshness")
	}
	if err := cache.Refresh(ctx); err != nil {
		t.Fatalf("fresh cached refresh: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("fresh cache sent %d HTTP requests, want one", got)
	}
}

func TestIsNotable_Integration(t *testing.T) {
	cache, _, calls := notableModelsTestCache(t)

	// Test Ollama - should always be notable
	if !cache.IsNotable("ollama", "llama3", 0) {
		t.Error("Ollama models should always be notable")
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("Ollama fetched metadata %d times, want none", got)
	}
	for _, tc := range []struct {
		provider string
		modelID  string
		want     bool
	}{
		{"anthropic", "claude-opus-4-5", true},
		{"anthropic", "claude-3-5-sonnet-20241022", false},
		{"openai", "gpt-4o", true},
	} {
		if got := cache.IsNotable(tc.provider, tc.modelID, 0); got != tc.want {
			t.Errorf("IsNotable(%q, %q) = %t, want %t", tc.provider, tc.modelID, got, tc.want)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("cached notable checks sent %d HTTP requests, want one", got)
	}
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
