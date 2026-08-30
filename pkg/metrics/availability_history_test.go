package metrics

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func TestAvailabilityHistorySchemaDoesNotRetainTargetOrAgentDetails(t *testing.T) {
	store, _ := newAvailabilityHistoryTestStore(t)
	rows, err := store.db.Query(`PRAGMA table_info(availability_observations)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(name)
		for _, forbidden := range []string{"address", "error", "agent_id", "agent_name"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("availability observation schema retains forbidden detail in column %q", name)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func newAvailabilityHistoryTestStore(t *testing.T) (*Store, StoreConfig) {
	t.Helper()
	config := DefaultConfig(t.TempDir())
	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, config
}

func availabilityObservation(id, target string, revision int64, outcome AvailabilityOutcome, at time.Time, validFor time.Duration, latency *int64) AvailabilityObservation {
	return AvailabilityObservation{
		ObservationID:   id,
		TargetID:        target,
		ConfigRevision:  revision,
		Outcome:         outcome,
		ObservedAt:      at,
		TimelineAt:      at,
		IngestedAt:      at,
		ValidFor:        validFor,
		ExecutionSource: AvailabilitySourceLocal,
		LatencyMillis:   latency,
	}
}

func TestAvailabilityHistoryCoverageAndLatencyRemainCategorical(t *testing.T) {
	store, _ := newAvailabilityHistoryTestStore(t)
	end := time.Now().UTC().Truncate(time.Minute)
	start := end.Add(-time.Hour)
	latency := int64(18)
	if err := store.WriteAvailabilityObservationSync(availabilityObservation("one", "target", 1, AvailabilityReachable, end.Add(-30*time.Minute), 5*time.Minute, &latency)); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteAvailabilityObservationSync(availabilityObservation("two", "target", 1, AvailabilityUnreachable, end.Add(-20*time.Minute), 5*time.Minute, nil)); err != nil {
		t.Fatal(err)
	}

	result, err := store.QueryAvailabilityHistory([]string{"target"}, start, end, 120)
	if err != nil {
		t.Fatal(err)
	}
	summary := result["target"].Summary
	if summary.ReachableSeconds != 300 || summary.UnreachableSeconds != 300 || summary.IndeterminateSeconds != 0 || summary.UnknownSeconds != 3000 {
		t.Fatalf("summary durations = %+v", summary)
	}
	if summary.AvailabilityPercent == nil || *summary.AvailabilityPercent != 50 {
		t.Fatalf("availability = %v, want 50", summary.AvailabilityPercent)
	}
	if math.Abs(summary.CoveragePercent-16.67) > 0.001 {
		t.Fatalf("coverage = %v, want 16.67", summary.CoveragePercent)
	}
	if summary.ReachableLatencyMillis == nil || summary.ReachableLatencyMillis.Average != 18 {
		t.Fatalf("reachable latency = %+v", summary.ReachableLatencyMillis)
	}
	if len(result["target"].Buckets) > 120 {
		t.Fatalf("bucket count = %d, want <= 120", len(result["target"].Buckets))
	}
}

func TestAvailabilityHistoryIndeterminateAndUnknownNeverCountAsAvailability(t *testing.T) {
	store, _ := newAvailabilityHistoryTestStore(t)
	end := time.Now().UTC().Truncate(time.Minute)
	start := end.Add(-time.Hour)
	if err := store.WriteAvailabilityObservationSync(availabilityObservation("udp", "target", 1, AvailabilityIndeterminate, end.Add(-10*time.Minute), 5*time.Minute, nil)); err != nil {
		t.Fatal(err)
	}
	result, err := store.QueryAvailabilityHistory([]string{"target"}, start, end, 120)
	if err != nil {
		t.Fatal(err)
	}
	summary := result["target"].Summary
	if summary.IndeterminateSeconds != 300 || summary.UnknownSeconds != 3300 {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.AvailabilityPercent != nil {
		t.Fatalf("availability = %v, want absent without determinate time", *summary.AvailabilityPercent)
	}
}

func TestAvailabilityHistoryDuplicateRestartRevisionAndDelete(t *testing.T) {
	dir := t.TempDir()
	config := DefaultConfig(dir)
	store, err := NewStore(config)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Truncate(time.Minute).Add(-20 * time.Minute)
	first := availabilityObservation("stable-id", "target", 1, AvailabilityReachable, base, 5*time.Minute, nil)
	if err := store.WriteAvailabilityObservationSync(first); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteAvailabilityObservationSync(first); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteAvailabilityObservationSync(availabilityObservation("revision-two", "target", 2, AvailabilityUnreachable, base.Add(10*time.Minute), 5*time.Minute, nil)); err != nil {
		t.Fatal(err)
	}
	raw, _, boundaries, err := store.AvailabilityHistoryRowCounts("target")
	if err != nil {
		t.Fatal(err)
	}
	if raw != 2 || boundaries != 2 {
		t.Fatalf("rows raw=%d boundaries=%d, want 2/2", raw, boundaries)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	restarted, err := NewStore(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = restarted.Close() })
	result, err := restarted.QueryAvailabilityHistory([]string{"target"}, base.Add(-time.Minute), base.Add(20*time.Minute), 120)
	if err != nil {
		t.Fatal(err)
	}
	if len(result["target"].RevisionBoundaries) != 2 {
		t.Fatalf("revision boundaries = %+v", result["target"].RevisionBoundaries)
	}
	restarted.DeleteAvailabilityTargetHistory("target")
	raw, buckets, boundaries, err := restarted.AvailabilityHistoryRowCounts("target")
	if err != nil {
		t.Fatal(err)
	}
	if raw != 0 || buckets != 0 || boundaries != 0 {
		t.Fatalf("rows after delete raw=%d buckets=%d boundaries=%d", raw, buckets, boundaries)
	}
}

func TestAvailabilityHistoryBatchReadIsBoundedAtTwoHundredTargets(t *testing.T) {
	store, _ := newAvailabilityHistoryTestStore(t)
	ids := make([]string, availabilityMaxBatchTargets)
	for index := range ids {
		ids[index] = fmt.Sprintf("target-%03d", index)
	}
	end := time.Now().UTC().Truncate(time.Minute)
	results, err := store.QueryAvailabilityHistory(ids, end.Add(-24*time.Hour), end, availabilityMaxBuckets)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != availabilityMaxBatchTargets {
		t.Fatalf("results = %d, want %d", len(results), availabilityMaxBatchTargets)
	}
	for _, id := range ids {
		if len(results[id].Buckets) > availabilityMaxBuckets {
			t.Fatalf("%s buckets = %d", id, len(results[id].Buckets))
		}
		if results[id].Summary.CoveragePercent != 0 || results[id].Summary.UnknownSeconds != 86400 {
			t.Fatalf("%s unknown summary = %+v", id, results[id].Summary)
		}
	}
	if _, err := store.QueryAvailabilityHistory(append(ids, "target-over-limit"), end.Add(-time.Hour), end, availabilityMaxBuckets); err == nil {
		t.Fatal("expected over-limit target batch to fail")
	}
}

func TestAvailabilityHistoryRetentionRemovesRawBucketsAndBoundaries(t *testing.T) {
	store, _ := newAvailabilityHistoryTestStore(t)
	old := time.Now().UTC().Add(-400 * 24 * time.Hour)
	if err := store.WriteAvailabilityObservationSync(availabilityObservation("old-one", "old-target", 1, AvailabilityReachable, old, 5*time.Minute, nil)); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteAvailabilityObservationSync(availabilityObservation("old-two", "old-target", 2, AvailabilityUnreachable, old.Add(10*time.Minute), 5*time.Minute, nil)); err != nil {
		t.Fatal(err)
	}
	store.runAvailabilityRetention()
	raw, buckets, boundaries, err := store.AvailabilityHistoryRowCounts("old-target")
	if err != nil {
		t.Fatal(err)
	}
	if raw != 0 || buckets != 0 || boundaries != 0 {
		t.Fatalf("retained old rows raw=%d buckets=%d boundaries=%d", raw, buckets, boundaries)
	}
}
