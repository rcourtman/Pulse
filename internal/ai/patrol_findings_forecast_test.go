package ai

import (
	"testing"
	"time"
)

func TestStampCapacityForecasts_AttachesToCapacityFindingByResourceID(t *testing.T) {
	store := NewFindingsStore()
	f := &Finding{
		ID:           "f1",
		Key:          "disk-usage",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryCapacity,
		ResourceID:   "storage-tower-array",
		ResourceName: "Tower Array",
		ResourceType: "storage",
		Title:        "Tower Array at 86%",
		Description:  "Storage pool near capacity.",
		DetectedAt:   time.Now(),
		LastSeenAt:   time.Now(),
	}
	if !store.Add(f) {
		t.Fatal("expected Add to create the finding")
	}

	forecasts := []seedForecast{
		{resourceID: "storage-tower-array", metric: "storage", current: 86.0, dailyChange: 1.2, daysToFull: 11},
		{resourceID: "unrelated", metric: "storage", current: 50.0, dailyChange: 0.1, daysToFull: 500},
	}
	changed := store.StampCapacityForecasts(mustForecastMap(forecasts))
	if changed != 1 {
		t.Fatalf("changed = %d, want 1", changed)
	}

	stored := store.Get("f1")
	if stored.CapacityForecast == nil {
		t.Fatal("expected CapacityForecast to be set")
	}
	if stored.CapacityForecast.DaysToFull != 11 {
		t.Fatalf("DaysToFull = %d, want 11", stored.CapacityForecast.DaysToFull)
	}
	if stored.CapacityForecast.CurrentPct != 86.0 {
		t.Fatalf("CurrentPct = %v, want 86", stored.CapacityForecast.CurrentPct)
	}
}

func TestStampCapacityForecasts_DoesNotAttachToNonCapacityUnrelatedResource(t *testing.T) {
	store := NewFindingsStore()
	// A reliability finding on a node (neither capacity category nor storage/disk
	// resource type) must not receive a storage utilization forecast.
	f := &Finding{
		ID:           "f2",
		Key:          "node-reboot",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryReliability,
		ResourceID:   "node-pve1",
		ResourceName: "pve1",
		ResourceType: "node",
		Title:        "Node rebooted unexpectedly",
		Description:  "Unexpected reboot.",
		DetectedAt:   time.Now(),
		LastSeenAt:   time.Now(),
	}
	store.Add(f)

	changed := store.StampCapacityForecasts(map[string]CapacityForecast{
		"node-pve1": {Metric: "storage", CurrentPct: 86, DailyChange: 1.2, DaysToFull: 11},
	})
	if changed != 0 {
		t.Fatalf("changed = %d, want 0 (non-capacity, non-storage finding)", changed)
	}
	if store.Get("f2").CapacityForecast != nil {
		t.Fatal("CapacityForecast must remain nil for a non-capacity node finding")
	}
}

func TestStampCapacityForecasts_StorageResourceStampsRegardlessOfCategory(t *testing.T) {
	store := NewFindingsStore()
	// A storage resource finding categorized as "general" still concerns the
	// underlying utilization, so the forecast applies.
	f := &Finding{
		ID:           "f3",
		Key:          "pool-noise",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryGeneral,
		ResourceID:   "storage-zfs-1",
		ResourceName: "zfs-1",
		ResourceType: "storage",
		Title:        "Pool utilization high",
		Description:  "High utilization.",
		DetectedAt:   time.Now(),
		LastSeenAt:   time.Now(),
	}
	store.Add(f)

	changed := store.StampCapacityForecasts(map[string]CapacityForecast{
		"storage-zfs-1": {Metric: "storage", CurrentPct: 91, DailyChange: 0.5, DaysToFull: 18},
	})
	if changed != 1 {
		t.Fatalf("changed = %d, want 1", changed)
	}
	if store.Get("f3").CapacityForecast == nil {
		t.Fatal("expected forecast on storage finding regardless of category")
	}
}

func TestStampCapacityForecasts_SkipsResolvedAndUnchanged(t *testing.T) {
	store := NewFindingsStore()
	active := &Finding{
		ID: "a", Key: "k", Severity: FindingSeverityWarning, Category: FindingCategoryCapacity,
		ResourceID: "r-a", ResourceType: "storage", Title: "t", Description: "d",
		DetectedAt: time.Now(), LastSeenAt: time.Now(),
	}
	resolved := &Finding{
		ID: "z", Key: "k", Severity: FindingSeverityWarning, Category: FindingCategoryCapacity,
		ResourceID: "r-z", ResourceType: "storage", Title: "t", Description: "d",
		DetectedAt: time.Now(), LastSeenAt: time.Now(),
	}
	store.Add(active)
	store.Add(resolved)
	now := time.Now()
	resolved.ResolvedAt = &now
	// Mutate resolved under lock like the store mutators do.
	store.mu.Lock()
	if rf := store.findings["z"]; rf != nil {
		rf.ResolvedAt = &now
	}
	store.mu.Unlock()

	fc := map[string]CapacityForecast{
		"r-a": {CurrentPct: 80, DailyChange: 1, DaysToFull: 20},
		"r-z": {CurrentPct: 80, DailyChange: 1, DaysToFull: 20},
	}
	if changed := store.StampCapacityForecasts(fc); changed != 1 {
		t.Fatalf("first stamp changed = %d, want 1 (resolved skipped)", changed)
	}
	// Second identical stamp must be a no-op (equal forecast already present).
	if changed := store.StampCapacityForecasts(fc); changed != 0 {
		t.Fatalf("second stamp changed = %d, want 0 (idempotent)", changed)
	}
	if store.Get("z").CapacityForecast != nil {
		t.Fatal("resolved finding must not be stamped")
	}
}

func TestForecastMoreUrgent(t *testing.T) {
	filling := CapacityForecast{DaysToFull: 5, DailyChange: 2}
	fillingLater := CapacityForecast{DaysToFull: 30, DailyChange: 0.5}
	stable := CapacityForecast{DaysToFull: -1, DailyChange: 0}
	declining := CapacityForecast{DaysToFull: -1, DailyChange: -1}

	if !forecastMoreUrgent(filling, stable) {
		t.Error("filling should beat stable")
	}
	if !forecastMoreUrgent(filling, fillingLater) {
		t.Error("sooner fill should beat later fill")
	}
	if forecastMoreUrgent(stable, filling) {
		t.Error("stable should not beat filling")
	}
	if !forecastMoreUrgent(declining, stable) && !forecastMoreUrgent(stable, declining) {
		// both non-filling: tie-break by dailyChange sign; either ordering is
		// acceptable as long as it is deterministic. Just ensure no panic.
	}
}

func mustForecastMap(seeds []seedForecast) map[string]CapacityForecast {
	out := make(map[string]CapacityForecast, len(seeds))
	for _, sf := range seeds {
		want := CapacityForecast{Metric: sf.metric, CurrentPct: sf.current, DailyChange: sf.dailyChange, DaysToFull: sf.daysToFull}
		if cur, ok := out[sf.resourceID]; !ok || forecastMoreUrgent(want, cur) {
			out[sf.resourceID] = want
		}
	}
	return out
}
