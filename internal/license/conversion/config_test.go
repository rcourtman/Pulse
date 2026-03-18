package conversion

import (
	"sync"
	"testing"
)

func TestCollectionConfigDefaultsToEnabled(t *testing.T) {
	cfg := NewCollectionConfig()

	if !cfg.IsEnabled() {
		t.Fatal("IsEnabled() = false, want true")
	}
	if !cfg.IsSurfaceEnabled("history_chart") {
		t.Fatal("IsSurfaceEnabled(history_chart) = false, want true")
	}

	snapshot := cfg.GetConfig()
	if !snapshot.Enabled {
		t.Fatal("snapshot.Enabled = false, want true")
	}
	if len(snapshot.DisabledSurfaces) != 0 {
		t.Fatalf("len(snapshot.DisabledSurfaces) = %d, want 0", len(snapshot.DisabledSurfaces))
	}
}

func TestCollectionConfigGlobalDisable(t *testing.T) {
	cfg := NewCollectionConfig()
	cfg.UpdateConfig(CollectionConfigSnapshot{
		Enabled: false,
	})

	if cfg.IsEnabled() {
		t.Fatal("IsEnabled() = true, want false")
	}
	if cfg.IsSurfaceEnabled("history_chart") {
		t.Fatal("IsSurfaceEnabled(history_chart) = true, want false")
	}
	if cfg.IsSurfaceEnabled("ai_intelligence") {
		t.Fatal("IsSurfaceEnabled(ai_intelligence) = true, want false")
	}
}

func TestCollectionConfigPerSurfaceDisable(t *testing.T) {
	cfg := NewCollectionConfig()
	cfg.UpdateConfig(CollectionConfigSnapshot{
		Enabled:          true,
		DisabledSurfaces: []string{"history_chart"},
	})

	if !cfg.IsEnabled() {
		t.Fatal("IsEnabled() = false, want true")
	}
	if cfg.IsSurfaceEnabled("history_chart") {
		t.Fatal("IsSurfaceEnabled(history_chart) = true, want false")
	}
	if !cfg.IsSurfaceEnabled("ai_intelligence") {
		t.Fatal("IsSurfaceEnabled(ai_intelligence) = false, want true")
	}
}

func TestCollectionConfigUpdateRoundTrip(t *testing.T) {
	cfg := NewCollectionConfig()

	input := CollectionConfigSnapshot{
		Enabled:          true,
		DisabledSurfaces: []string{"history_chart", "ai_intelligence"},
	}
	cfg.UpdateConfig(input)
	got := cfg.GetConfig()

	if got.Enabled != input.Enabled {
		t.Fatalf("snapshot.Enabled = %v, want %v", got.Enabled, input.Enabled)
	}
	if len(got.DisabledSurfaces) != len(input.DisabledSurfaces) {
		t.Fatalf("len(snapshot.DisabledSurfaces) = %d, want %d", len(got.DisabledSurfaces), len(input.DisabledSurfaces))
	}

	seen := make(map[string]bool, len(got.DisabledSurfaces))
	for _, surface := range got.DisabledSurfaces {
		seen[surface] = true
	}
	for _, surface := range input.DisabledSurfaces {
		if !seen[surface] {
			t.Fatalf("missing surface %q in snapshot", surface)
		}
	}
}

func TestCollectionConfigConcurrentAccess(t *testing.T) {
	cfg := NewCollectionConfig()
	const workers = 16
	const iterations = 300

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if (worker+j)%4 == 0 {
					cfg.UpdateConfig(CollectionConfigSnapshot{
						Enabled: (worker+j)%2 == 0,
						DisabledSurfaces: []string{
							"history_chart",
							"ai_intelligence",
						},
					})
					continue
				}

				_ = cfg.IsEnabled()
				_ = cfg.IsSurfaceEnabled("history_chart")
				_ = cfg.GetConfig()
			}
		}(i)
	}

	wg.Wait()

	snapshot := cfg.GetConfig()
	if snapshot.DisabledSurfaces == nil {
		t.Fatal("snapshot.DisabledSurfaces is nil, want non-nil slice")
	}
}
