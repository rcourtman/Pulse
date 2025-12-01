package monitoring

import (
	"testing"
	"time"
)

func TestStalenessTracker_UpdateSuccess(t *testing.T) {
	tracker := NewStalenessTracker(nil)
	now := time.Now()

	// Update success with payload
	payload := []byte("test data")
	tracker.UpdateSuccess(InstanceTypePVE, "test-instance", payload)

	// Verify entry was created
	snap, ok := tracker.snapshot(InstanceTypePVE, "test-instance")
	if !ok {
		t.Fatal("snapshot not found after UpdateSuccess")
	}

	if snap.Instance != "test-instance" {
		t.Errorf("instance = %s, want test-instance", snap.Instance)
	}
	if snap.InstanceType != InstanceTypePVE {
		t.Errorf("instanceType = %v, want %v", snap.InstanceType, InstanceTypePVE)
	}
	if snap.LastSuccess.Before(now) {
		t.Error("lastSuccess should be at or after update time")
	}
	if snap.ChangeHash == "" {
		t.Error("changeHash should be set when payload provided")
	}
}

func TestStalenessTracker_UpdateError(t *testing.T) {
	tracker := NewStalenessTracker(nil)
	now := time.Now()

	tracker.UpdateError(InstanceTypePBS, "error-instance")

	snap, ok := tracker.snapshot(InstanceTypePBS, "error-instance")
	if !ok {
		t.Fatal("snapshot not found after UpdateError")
	}

	if snap.LastError.Before(now) {
		t.Error("lastError should be at or after update time")
	}
	if snap.LastSuccess.After(now.Add(-time.Hour)) {
		t.Error("lastSuccess should not be set by UpdateError")
	}
}

func TestStalenessTracker_StalenessScore_Fresh(t *testing.T) {
	tracker := NewStalenessTracker(nil)
	tracker.SetBounds(10*time.Second, 5*time.Minute)

	// Record a recent success
	tracker.UpdateSuccess(InstanceTypePVE, "fresh-instance", nil)

	score, ok := tracker.StalenessScore(InstanceTypePVE, "fresh-instance")
	if !ok {
		t.Fatal("staleness score should be available")
	}

	// Should be near 0 for fresh data
	if score > 0.01 {
		t.Errorf("staleness score = %f, want near 0 for fresh data", score)
	}
}

func TestStalenessTracker_StalenessScore_Stale(t *testing.T) {
	tracker := NewStalenessTracker(nil)
	tracker.SetBounds(10*time.Second, 60*time.Second) // max stale is 60s

	// Record old success
	oldTime := time.Now().Add(-45 * time.Second)
	tracker.setSnapshot(FreshnessSnapshot{
		InstanceType: InstanceTypePVE,
		Instance:     "stale-instance",
		LastSuccess:  oldTime,
	})

	score, ok := tracker.StalenessScore(InstanceTypePVE, "stale-instance")
	if !ok {
		t.Fatal("staleness score should be available")
	}

	// 45s old with 60s max = 0.75 score
	expected := 45.0 / 60.0
	tolerance := 0.05
	if score < expected-tolerance || score > expected+tolerance {
		t.Errorf("staleness score = %f, want ~%f (45s / 60s)", score, expected)
	}
}

func TestStalenessTracker_StalenessScore_MaxStale(t *testing.T) {
	tracker := NewStalenessTracker(nil)
	tracker.SetBounds(10*time.Second, 60*time.Second)

	// Record very old success (beyond max)
	veryOld := time.Now().Add(-2 * time.Minute)
	tracker.setSnapshot(FreshnessSnapshot{
		InstanceType: InstanceTypePVE,
		Instance:     "very-stale",
		LastSuccess:  veryOld,
	})

	score, ok := tracker.StalenessScore(InstanceTypePVE, "very-stale")
	if !ok {
		t.Fatal("staleness score should be available")
	}

	// Should be capped at 1.0
	if score != 1.0 {
		t.Errorf("staleness score = %f, want 1.0 (capped)", score)
	}
}

func TestStalenessTracker_StalenessScore_NoData(t *testing.T) {
	tracker := NewStalenessTracker(nil)

	score, ok := tracker.StalenessScore(InstanceTypePVE, "nonexistent")
	if ok {
		t.Error("staleness score should not be available for nonexistent instance")
	}
	if score != 0 {
		t.Errorf("staleness score = %f, want 0 for nonexistent instance", score)
	}
}

func TestStalenessTracker_StalenessScore_NeverSucceeded(t *testing.T) {
	tracker := NewStalenessTracker(nil)

	// Create entry with error but no success
	tracker.UpdateError(InstanceTypePVE, "never-succeeded")

	score, ok := tracker.StalenessScore(InstanceTypePVE, "never-succeeded")
	if !ok {
		t.Fatal("staleness score should be available even without success")
	}

	// Should return max staleness (1.0) when never succeeded
	if score != 1.0 {
		t.Errorf("staleness score = %f, want 1.0 for never-succeeded instance", score)
	}
}

func TestStalenessTracker_SetChangeHash(t *testing.T) {
	tracker := NewStalenessTracker(nil)

	payload1 := []byte("data v1")
	payload2 := []byte("data v2")

	tracker.UpdateSuccess(InstanceTypePVE, "test", payload1)
	snap1, _ := tracker.snapshot(InstanceTypePVE, "test")
	hash1 := snap1.ChangeHash

	// Update hash with different payload
	tracker.SetChangeHash(InstanceTypePVE, "test", payload2)
	snap2, _ := tracker.snapshot(InstanceTypePVE, "test")
	hash2 := snap2.ChangeHash

	if hash1 == hash2 {
		t.Error("change hash should be different for different payloads")
	}
	if hash1 == "" || hash2 == "" {
		t.Error("change hashes should not be empty")
	}
}

func TestStalenessTracker_SetBounds(t *testing.T) {
	tracker := NewStalenessTracker(nil)

	// Set custom bounds
	tracker.SetBounds(30*time.Second, 10*time.Minute)

	// Verify by checking behavior
	tracker.setSnapshot(FreshnessSnapshot{
		InstanceType: InstanceTypePVE,
		Instance:     "test",
		LastSuccess:  time.Now().Add(-5 * time.Minute),
	})

	score, _ := tracker.StalenessScore(InstanceTypePVE, "test")

	// With 5min age and 10min max, score should be ~0.5
	expected := 0.5
	tolerance := 0.05
	if score < expected-tolerance || score > expected+tolerance {
		t.Errorf("staleness score = %f, want ~%f with custom bounds", score, expected)
	}
}

func TestStalenessTracker_SetBounds_ZeroValues(t *testing.T) {
	tracker := NewStalenessTracker(nil)

	// Try to set zero bounds (should be ignored)
	tracker.SetBounds(0, 0)

	// Verify defaults are still in effect
	tracker.setSnapshot(FreshnessSnapshot{
		InstanceType: InstanceTypePVE,
		Instance:     "test",
		LastSuccess:  time.Now().Add(-6 * time.Minute),
	})

	score, _ := tracker.StalenessScore(InstanceTypePVE, "test")

	// With defaults (maxStale=5min), 6min should be capped at 1.0
	if score != 1.0 {
		t.Errorf("staleness score = %f, want 1.0 (using default maxStale)", score)
	}
}

func TestStalenessTracker_MergeSnapshot(t *testing.T) {
	tracker := NewStalenessTracker(nil)
	t1 := time.Now().Add(-10 * time.Second)
	t2 := time.Now().Add(-5 * time.Second)
	t3 := time.Now()

	// Create initial snapshot
	tracker.setSnapshot(FreshnessSnapshot{
		InstanceType: InstanceTypePVE,
		Instance:     "merge-test",
		LastSuccess:  t1,
		LastError:    t2,
	})

	// Merge with newer success
	tracker.mergeSnapshot(FreshnessSnapshot{
		InstanceType: InstanceTypePVE,
		Instance:     "merge-test",
		LastSuccess:  t3,
	})

	snap, _ := tracker.snapshot(InstanceTypePVE, "merge-test")
	if !snap.LastSuccess.Equal(t3) {
		t.Error("merge should update lastSuccess with newer time")
	}
	if !snap.LastError.Equal(t2) {
		t.Error("merge should preserve lastError when not updated")
	}

	// Merge with older success (should not update)
	tracker.mergeSnapshot(FreshnessSnapshot{
		InstanceType: InstanceTypePVE,
		Instance:     "merge-test",
		LastSuccess:  t1,
	})

	snap, _ = tracker.snapshot(InstanceTypePVE, "merge-test")
	if !snap.LastSuccess.Equal(t3) {
		t.Error("merge should not update lastSuccess with older time")
	}
}

func TestStalenessTracker_Snapshot(t *testing.T) {
	tracker := NewStalenessTracker(nil)
	tracker.SetBounds(10*time.Second, 60*time.Second)

	// Add multiple entries
	tracker.UpdateSuccess(InstanceTypePVE, "pve-1", nil)
	tracker.UpdateSuccess(InstanceTypePBS, "pbs-1", nil)
	tracker.UpdateSuccess(InstanceTypePMG, "pmg-1", nil)

	// Make one stale
	tracker.setSnapshot(FreshnessSnapshot{
		InstanceType: InstanceTypePVE,
		Instance:     "pve-stale",
		LastSuccess:  time.Now().Add(-30 * time.Second),
	})

	snapshots := tracker.Snapshot()

	if len(snapshots) != 4 {
		t.Errorf("snapshot count = %d, want 4", len(snapshots))
	}

	// Verify snapshot contains expected data
	found := make(map[string]bool)
	for _, snap := range snapshots {
		found[snap.Instance] = true
		if snap.Instance == "pve-stale" {
			// Should have staleness score around 0.5 (30s / 60s)
			if snap.Score < 0.4 || snap.Score > 0.6 {
				t.Errorf("pve-stale score = %f, want ~0.5", snap.Score)
			}
		} else {
			// Fresh instances should have score near 0
			if snap.Score > 0.1 {
				t.Errorf("%s score = %f, want near 0", snap.Instance, snap.Score)
			}
		}
	}

	expectedInstances := []string{"pve-1", "pbs-1", "pmg-1", "pve-stale"}
	for _, expected := range expectedInstances {
		if !found[expected] {
			t.Errorf("snapshot missing expected instance: %s", expected)
		}
	}
}

func TestStalenessTracker_Snapshot_Empty(t *testing.T) {
	tracker := NewStalenessTracker(nil)

	snapshots := tracker.Snapshot()
	if len(snapshots) != 0 {
		t.Errorf("empty tracker snapshot count = %d, want 0", len(snapshots))
	}
}

func TestStalenessTracker_Snapshot_Nil(t *testing.T) {
	var tracker *StalenessTracker
	snapshots := tracker.Snapshot()
	if snapshots != nil {
		t.Error("nil tracker snapshot should return nil")
	}
}

func TestStalenessTracker_NilSafety(t *testing.T) {
	var tracker *StalenessTracker

	// All methods should handle nil gracefully
	tracker.UpdateSuccess(InstanceTypePVE, "test", nil)
	tracker.UpdateError(InstanceTypePVE, "test")
	tracker.SetChangeHash(InstanceTypePVE, "test", []byte("data"))

	score, ok := tracker.StalenessScore(InstanceTypePVE, "test")
	if ok {
		t.Error("nil tracker should return ok=false for staleness score")
	}
	if score != 0 {
		t.Error("nil tracker should return score=0")
	}
}

func TestStalenessTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewStalenessTracker(nil)

	// Test concurrent access doesn't panic
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			instance := "instance"
			tracker.UpdateSuccess(InstanceTypePVE, instance, []byte("data"))
			tracker.UpdateError(InstanceTypePVE, instance)
			tracker.StalenessScore(InstanceTypePVE, instance)
			tracker.Snapshot()
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestStalenessTracker_StalenessScore_ZeroMaxStaleUsesDefault(t *testing.T) {
	// Create tracker and directly set maxStale to 0 to test the defensive fallback
	tracker := &StalenessTracker{
		entries:  make(map[string]FreshnessSnapshot),
		maxStale: 0, // Force zero to test default fallback
	}

	// Set a 2.5 minute old success
	oldTime := time.Now().Add(-150 * time.Second)
	tracker.setSnapshot(FreshnessSnapshot{
		InstanceType: InstanceTypePVE,
		Instance:     "zero-max-test",
		LastSuccess:  oldTime,
	})

	score, ok := tracker.StalenessScore(InstanceTypePVE, "zero-max-test")
	if !ok {
		t.Fatal("staleness score should be available")
	}

	// With default 5 minute maxStale, 2.5 minutes should give ~0.5 score
	expected := 150.0 / 300.0 // 150s / 300s (5 min)
	tolerance := 0.05
	if score < expected-tolerance || score > expected+tolerance {
		t.Errorf("staleness score = %f, want ~%f (using default 5 min maxStale)", score, expected)
	}
}

func TestStalenessTracker_StalenessScore_FutureLastSuccess(t *testing.T) {
	tracker := NewStalenessTracker(nil)

	// Set LastSuccess in the future (clock skew scenario)
	futureTime := time.Now().Add(1 * time.Hour)
	tracker.setSnapshot(FreshnessSnapshot{
		InstanceType: InstanceTypePVE,
		Instance:     "future-instance",
		LastSuccess:  futureTime,
	})

	score, ok := tracker.StalenessScore(InstanceTypePVE, "future-instance")
	if !ok {
		t.Fatal("staleness score should be available")
	}

	// Future timestamp should return 0 (not stale)
	if score != 0 {
		t.Errorf("staleness score = %f, want 0 for future LastSuccess", score)
	}
}

func TestStalenessTracker_StalenessScore_WithMetrics(t *testing.T) {
	// Create a minimal PollMetrics with lastSuccessByKey support
	pm := &PollMetrics{
		lastSuccessByKey: make(map[metricKey]time.Time),
	}

	tracker := NewStalenessTracker(pm)
	tracker.SetBounds(10*time.Second, 60*time.Second)

	// Set an old success time in the tracker
	oldTime := time.Now().Add(-45 * time.Second)
	tracker.setSnapshot(FreshnessSnapshot{
		InstanceType: InstanceTypePVE,
		Instance:     "metrics-test",
		LastSuccess:  oldTime,
	})

	// Store a newer time in metrics
	newerTime := time.Now().Add(-15 * time.Second)
	pm.storeLastSuccess("pve", "metrics-test", newerTime)

	score, ok := tracker.StalenessScore(InstanceTypePVE, "metrics-test")
	if !ok {
		t.Fatal("staleness score should be available")
	}

	// Should use the newer time from metrics: 15s / 60s = 0.25
	expected := 15.0 / 60.0
	tolerance := 0.05
	if score < expected-tolerance || score > expected+tolerance {
		t.Errorf("staleness score = %f, want ~%f (using metrics time)", score, expected)
	}
}

func TestStalenessTracker_StalenessScore_MetricsNotUsedWhenLastSuccessZero(t *testing.T) {
	// Create a minimal PollMetrics with lastSuccessByKey support
	pm := &PollMetrics{
		lastSuccessByKey: make(map[metricKey]time.Time),
	}

	tracker := NewStalenessTracker(pm)

	// Set a snapshot with zero LastSuccess (error-only entry)
	tracker.setSnapshot(FreshnessSnapshot{
		InstanceType: InstanceTypePVE,
		Instance:     "error-only",
		LastError:    time.Now(),
		// LastSuccess is zero
	})

	// Store a time in metrics (shouldn't be used since tracker LastSuccess is zero)
	pm.storeLastSuccess("pve", "error-only", time.Now().Add(-10*time.Second))

	score, ok := tracker.StalenessScore(InstanceTypePVE, "error-only")
	if !ok {
		t.Fatal("staleness score should be available")
	}

	// Should return 1.0 because LastSuccess is zero (metrics lookup is conditional on non-zero LastSuccess)
	if score != 1.0 {
		t.Errorf("staleness score = %f, want 1.0 when tracker LastSuccess is zero", score)
	}
}

func TestStalenessTracker_StalenessScore_NegativeMaxStaleUsesDefault(t *testing.T) {
	// Create tracker and directly set maxStale to negative to test the defensive fallback
	tracker := &StalenessTracker{
		entries:  make(map[string]FreshnessSnapshot),
		maxStale: -5 * time.Minute, // Force negative to test default fallback
	}

	// Set a 2.5 minute old success
	oldTime := time.Now().Add(-150 * time.Second)
	tracker.setSnapshot(FreshnessSnapshot{
		InstanceType: InstanceTypePVE,
		Instance:     "neg-max-test",
		LastSuccess:  oldTime,
	})

	score, ok := tracker.StalenessScore(InstanceTypePVE, "neg-max-test")
	if !ok {
		t.Fatal("staleness score should be available")
	}

	// With default 5 minute maxStale, 2.5 minutes should give ~0.5 score
	expected := 150.0 / 300.0 // 150s / 300s (5 min)
	tolerance := 0.05
	if score < expected-tolerance || score > expected+tolerance {
		t.Errorf("staleness score = %f, want ~%f (using default 5 min maxStale)", score, expected)
	}
}

func TestStalenessTracker_StalenessScore_MetricsLookupFails(t *testing.T) {
	// Create a minimal PollMetrics with lastSuccessByKey support
	pm := &PollMetrics{
		lastSuccessByKey: make(map[metricKey]time.Time),
	}

	tracker := NewStalenessTracker(pm)
	tracker.SetBounds(10*time.Second, 60*time.Second)

	// Set snapshot with non-zero LastSuccess
	oldTime := time.Now().Add(-30 * time.Second)
	tracker.setSnapshot(FreshnessSnapshot{
		InstanceType: InstanceTypePVE,
		Instance:     "lookup-fail-test",
		LastSuccess:  oldTime,
	})

	// Don't store anything in metrics - the lookup will fail

	score, ok := tracker.StalenessScore(InstanceTypePVE, "lookup-fail-test")
	if !ok {
		t.Fatal("staleness score should be available")
	}

	// Should use the tracker's LastSuccess since metrics lookup failed: 30s / 60s = 0.5
	expected := 30.0 / 60.0
	tolerance := 0.05
	if score < expected-tolerance || score > expected+tolerance {
		t.Errorf("staleness score = %f, want ~%f (using tracker time, metrics lookup failed)", score, expected)
	}
}

func TestStalenessTracker_StalenessScore_ScoreClampedBetweenZeroAndOne(t *testing.T) {
	// Test that score is always in [0, 1] range
	tests := []struct {
		name       string
		age        time.Duration
		maxStale   time.Duration
		wantScore  float64
		wantCapped bool
	}{
		{
			name:       "age much older than maxStale is capped at 1",
			age:        10 * time.Minute,
			maxStale:   1 * time.Minute,
			wantScore:  1.0,
			wantCapped: true,
		},
		{
			name:       "age at exactly maxStale gives 1",
			age:        5 * time.Minute,
			maxStale:   5 * time.Minute,
			wantScore:  1.0,
			wantCapped: false,
		},
		{
			name:       "age at half maxStale gives 0.5",
			age:        2*time.Minute + 30*time.Second,
			maxStale:   5 * time.Minute,
			wantScore:  0.5,
			wantCapped: false,
		},
		{
			name:       "very small age gives near-zero score",
			age:        1 * time.Millisecond,
			maxStale:   5 * time.Minute,
			wantScore:  0.0,
			wantCapped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewStalenessTracker(nil)
			tracker.SetBounds(10*time.Second, tt.maxStale)

			tracker.setSnapshot(FreshnessSnapshot{
				InstanceType: InstanceTypePVE,
				Instance:     "clamp-test",
				LastSuccess:  time.Now().Add(-tt.age),
			})

			score, ok := tracker.StalenessScore(InstanceTypePVE, "clamp-test")
			if !ok {
				t.Fatal("staleness score should be available")
			}

			// Check score is in [0, 1] range
			if score < 0 || score > 1 {
				t.Errorf("score = %f, want score in [0, 1] range", score)
			}

			tolerance := 0.05
			if score < tt.wantScore-tolerance || score > tt.wantScore+tolerance {
				t.Errorf("score = %f, want ~%f", score, tt.wantScore)
			}
		})
	}
}

// Note on coverage: The `if score < 0 { score = 0 }` branch (line 145) is mathematically
// unreachable because:
// 1. If age <= 0, we return early with score=0 (line 133-135)
// 2. If max <= 0, we default to 5 minutes making max positive (line 138-140)
// 3. Therefore score = age.Seconds() / max.Seconds() is always non-negative
// This is defensive code that guards against future refactoring mistakes.

func TestTrackerKey(t *testing.T) {
	tests := []struct {
		instanceType InstanceType
		instance     string
		want         string
	}{
		{InstanceTypePVE, "test1", "pve::test1"},
		{InstanceTypePBS, "test2", "pbs::test2"},
		{InstanceTypePMG, "pmg-host", "pmg::pmg-host"},
		{InstanceTypePVE, "", "pve::"},
	}

	for _, tt := range tests {
		got := trackerKey(tt.instanceType, tt.instance)
		if got != tt.want {
			t.Errorf("trackerKey(%v, %q) = %q, want %q", tt.instanceType, tt.instance, got, tt.want)
		}
	}
}

func TestStalenessTracker_MergeSnapshot_TableDriven(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	olderTime := baseTime.Add(-10 * time.Second)
	newerTime := baseTime.Add(10 * time.Second)

	tests := []struct {
		name            string
		existing        *FreshnessSnapshot // nil means no existing entry
		merge           FreshnessSnapshot
		wantLastSuccess time.Time
		wantLastError   time.Time
		wantLastMutated time.Time
		wantChangeHash  string
	}{
		{
			name:     "merge into non-existent entry creates new entry",
			existing: nil,
			merge: FreshnessSnapshot{
				InstanceType: InstanceTypePVE,
				Instance:     "new-instance",
				LastSuccess:  baseTime,
				LastError:    baseTime,
				LastMutated:  baseTime,
				ChangeHash:   "abc123",
			},
			wantLastSuccess: baseTime,
			wantLastError:   baseTime,
			wantLastMutated: baseTime,
			wantChangeHash:  "abc123",
		},
		{
			name: "newer LastSuccess updates existing",
			existing: &FreshnessSnapshot{
				InstanceType: InstanceTypePVE,
				Instance:     "test",
				LastSuccess:  baseTime,
			},
			merge: FreshnessSnapshot{
				InstanceType: InstanceTypePVE,
				Instance:     "test",
				LastSuccess:  newerTime,
			},
			wantLastSuccess: newerTime,
		},
		{
			name: "older LastSuccess does not update existing",
			existing: &FreshnessSnapshot{
				InstanceType: InstanceTypePVE,
				Instance:     "test",
				LastSuccess:  baseTime,
			},
			merge: FreshnessSnapshot{
				InstanceType: InstanceTypePVE,
				Instance:     "test",
				LastSuccess:  olderTime,
			},
			wantLastSuccess: baseTime,
		},
		{
			name: "newer LastError updates existing",
			existing: &FreshnessSnapshot{
				InstanceType: InstanceTypePVE,
				Instance:     "test",
				LastError:    baseTime,
			},
			merge: FreshnessSnapshot{
				InstanceType: InstanceTypePVE,
				Instance:     "test",
				LastError:    newerTime,
			},
			wantLastError: newerTime,
		},
		{
			name: "older LastError does not update existing",
			existing: &FreshnessSnapshot{
				InstanceType: InstanceTypePVE,
				Instance:     "test",
				LastError:    baseTime,
			},
			merge: FreshnessSnapshot{
				InstanceType: InstanceTypePVE,
				Instance:     "test",
				LastError:    olderTime,
			},
			wantLastError: baseTime,
		},
		{
			name: "newer LastMutated updates existing",
			existing: &FreshnessSnapshot{
				InstanceType: InstanceTypePVE,
				Instance:     "test",
				LastMutated:  baseTime,
			},
			merge: FreshnessSnapshot{
				InstanceType: InstanceTypePVE,
				Instance:     "test",
				LastMutated:  newerTime,
			},
			wantLastMutated: newerTime,
		},
		{
			name: "older LastMutated does not update existing",
			existing: &FreshnessSnapshot{
				InstanceType: InstanceTypePVE,
				Instance:     "test",
				LastMutated:  baseTime,
			},
			merge: FreshnessSnapshot{
				InstanceType: InstanceTypePVE,
				Instance:     "test",
				LastMutated:  olderTime,
			},
			wantLastMutated: baseTime,
		},
		{
			name: "non-empty ChangeHash updates existing",
			existing: &FreshnessSnapshot{
				InstanceType: InstanceTypePVE,
				Instance:     "test",
				ChangeHash:   "old-hash",
			},
			merge: FreshnessSnapshot{
				InstanceType: InstanceTypePVE,
				Instance:     "test",
				ChangeHash:   "new-hash",
			},
			wantChangeHash: "new-hash",
		},
		{
			name: "empty ChangeHash does not overwrite existing",
			existing: &FreshnessSnapshot{
				InstanceType: InstanceTypePVE,
				Instance:     "test",
				ChangeHash:   "existing-hash",
			},
			merge: FreshnessSnapshot{
				InstanceType: InstanceTypePVE,
				Instance:     "test",
				ChangeHash:   "",
			},
			wantChangeHash: "existing-hash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewStalenessTracker(nil)

			// Set up existing entry if specified
			if tt.existing != nil {
				tracker.setSnapshot(*tt.existing)
			}

			// Perform merge
			tracker.mergeSnapshot(tt.merge)

			// Get result
			snap, ok := tracker.snapshot(tt.merge.InstanceType, tt.merge.Instance)
			if !ok {
				t.Fatal("snapshot not found after merge")
			}

			// Verify instance metadata is always set
			if snap.InstanceType != tt.merge.InstanceType {
				t.Errorf("InstanceType = %v, want %v", snap.InstanceType, tt.merge.InstanceType)
			}
			if snap.Instance != tt.merge.Instance {
				t.Errorf("Instance = %q, want %q", snap.Instance, tt.merge.Instance)
			}

			// Verify timestamps
			if !tt.wantLastSuccess.IsZero() && !snap.LastSuccess.Equal(tt.wantLastSuccess) {
				t.Errorf("LastSuccess = %v, want %v", snap.LastSuccess, tt.wantLastSuccess)
			}
			if !tt.wantLastError.IsZero() && !snap.LastError.Equal(tt.wantLastError) {
				t.Errorf("LastError = %v, want %v", snap.LastError, tt.wantLastError)
			}
			if !tt.wantLastMutated.IsZero() && !snap.LastMutated.Equal(tt.wantLastMutated) {
				t.Errorf("LastMutated = %v, want %v", snap.LastMutated, tt.wantLastMutated)
			}
			if tt.wantChangeHash != "" && snap.ChangeHash != tt.wantChangeHash {
				t.Errorf("ChangeHash = %q, want %q", snap.ChangeHash, tt.wantChangeHash)
			}
		})
	}
}
