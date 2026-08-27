package mock

import (
	"testing"
)

func withMockEnabledForTest(t *testing.T) {
	t.Helper()
	previous := IsMockEnabled()
	if err := SetEnabled(true); err != nil {
		t.Fatalf("enable mock mode: %v", err)
	}
	t.Cleanup(func() {
		if err := SetEnabled(previous); err != nil {
			t.Fatalf("restore mock mode: %v", err)
		}
	})
}

func TestFixtureDataVersionAdvancesOnMetricTick(t *testing.T) {
	withMockEnabledForTest(t)

	before := FixtureDataVersion()
	updateMetrics(GetConfig())
	after := FixtureDataVersion()

	if after <= before {
		t.Fatalf("expected FixtureDataVersion to advance on a metric tick, got %d -> %d", before, after)
	}
}

func TestUnifiedResourceSnapshotMemoizedPerDataVersion(t *testing.T) {
	withMockEnabledForTest(t)

	// This test advances the fixture explicitly below. Pause the automatic
	// metric ticker so a slow race-enabled snapshot build cannot change the
	// data version between the two reads whose memoization identity we are
	// asserting.
	stopUpdateLoop()
	t.Cleanup(func() {
		if IsMockEnabled() {
			startUpdateLoop()
		}
	})

	first, firstFreshness := UnifiedResourceSnapshot()
	if len(first) == 0 {
		t.Fatal("expected mock resources")
	}
	second, secondFreshness := UnifiedResourceSnapshot()
	if len(second) != len(first) {
		t.Fatalf("expected identical memoized result, got %d vs %d resources", len(first), len(second))
	}
	if &first[0] != &second[0] {
		t.Fatal("expected the memoized snapshot to be returned for an unchanged data version")
	}
	if !firstFreshness.Equal(secondFreshness) {
		t.Fatalf("expected identical freshness, got %v vs %v", firstFreshness, secondFreshness)
	}

	updateMetrics(GetConfig())

	third, _ := UnifiedResourceSnapshot()
	if len(third) == 0 {
		t.Fatal("expected mock resources after tick")
	}
	if &first[0] == &third[0] {
		t.Fatal("expected a rebuilt snapshot after the data version advanced")
	}
}
