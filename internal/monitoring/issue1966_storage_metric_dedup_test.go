package monitoring

import (
	"testing"
	"time"
)

// These cover the guard added for rcourtman/Pulse#1966. syncUnifiedStorageMetrics runs at every
// accepted agent-report ingest boundary rather than on a storage poll cycle, so it restates every
// storage resource roughly once per agent report, while storage data refreshes on its own much
// slower schedule. On a fleet with ~20 agents and a TrueNAS system exporting ~89 datasets that was
// ~90% of everything written to metrics.db.
//
// The guard is exact, not time-based: a write is skipped only when both the value and the
// observation timestamp match what was last persisted, which given the
// (resource_type, resource_id, metric_type, tier, timestamp) row key can only ever be an upsert of
// a row to its own current contents.

func TestShouldPersistStorageMetricSuppressesIdenticalRestatements(t *testing.T) {
	m := &Monitor{}
	key := storageMetricDedupKey("storage", "system:abc/dataset:Tank", "usage")
	observedAt := time.Unix(1_700_000_000, 0)
	now := observedAt

	if !m.shouldPersistStorageMetric(key, 42.0, observedAt, now) {
		t.Fatal("first sample of a series must always be persisted")
	}

	// The storage poll hasn't refreshed, so observedAt is unchanged. Every one of these would be
	// an upsert of the row to its own contents.
	for i := 1; i <= 25; i++ {
		now = now.Add(2260 * time.Millisecond)
		if m.shouldPersistStorageMetric(key, 42.0, observedAt, now) {
			t.Fatalf("identical restatement %d was persisted; want suppressed", i)
		}
	}
}

func TestShouldPersistStorageMetricPersistsRefreshedObservation(t *testing.T) {
	m := &Monitor{}
	key := storageMetricDedupKey("storage", "system:abc/dataset:Tank", "usage")
	observedAt := time.Unix(1_700_000_000, 0)
	now := observedAt

	if !m.shouldPersistStorageMetric(key, 42.0, observedAt, now) {
		t.Fatal("first sample must be persisted")
	}
	// The poll refreshed and produced the same reading. That is a genuinely new observation at a
	// new row key, so it must still be written — this is what keeps history continuous.
	refreshed := observedAt.Add(60 * time.Second)
	if !m.shouldPersistStorageMetric(key, 42.0, refreshed, now.Add(60*time.Second)) {
		t.Fatal("same value at a refreshed observation time must be persisted")
	}
	// ...and is then itself suppressed until the next refresh.
	if m.shouldPersistStorageMetric(key, 42.0, refreshed, now.Add(62*time.Second)) {
		t.Fatal("restatement of the refreshed observation must be suppressed")
	}
}

func TestShouldPersistStorageMetricPersistsChangedValues(t *testing.T) {
	m := &Monitor{}
	key := storageMetricDedupKey("storage", "system:abc/dataset:Tank", "used")
	observedAt := time.Unix(1_700_000_000, 0)
	now := observedAt

	if !m.shouldPersistStorageMetric(key, 100, observedAt, now) {
		t.Fatal("first sample must be persisted")
	}
	// A corrected value at the same observation time still differs from what was stored, so it
	// must be written — the guard must never swallow a real change.
	for i, value := range []float64{101, 102, 103} {
		now = now.Add(time.Second)
		if !m.shouldPersistStorageMetric(key, value, observedAt, now) {
			t.Fatalf("changed value %v (step %d) was suppressed; want persisted", value, i)
		}
	}
}

func TestShouldPersistStorageMetricTracksSeriesIndependently(t *testing.T) {
	m := &Monitor{}
	observedAt := time.Unix(1_700_000_000, 0)
	now := observedAt
	usage := storageMetricDedupKey("storage", "system:abc/dataset:Tank", "usage")
	used := storageMetricDedupKey("storage", "system:abc/dataset:Tank", "used")
	other := storageMetricDedupKey("storage", "system:abc/dataset:Other", "usage")

	for _, key := range []string{usage, used, other} {
		if !m.shouldPersistStorageMetric(key, 5, observedAt, now) {
			t.Fatalf("first sample for %q must be persisted", key)
		}
	}
	now = now.Add(time.Second)
	if m.shouldPersistStorageMetric(usage, 5, observedAt, now) {
		t.Error("unchanged usage sample should be suppressed")
	}
	if !m.shouldPersistStorageMetric(used, 6, observedAt, now) {
		t.Error("changed used sample must be persisted despite usage being suppressed")
	}
	if m.shouldPersistStorageMetric(other, 5, observedAt, now) {
		t.Error("unchanged sample on a different resource should be suppressed independently")
	}
}

func TestPruneStorageMetricDedupDropsStaleSeries(t *testing.T) {
	m := &Monitor{}
	now := time.Unix(1_700_000_000, 0)
	stale := storageMetricDedupKey("storage", "system:abc/dataset:Gone", "usage")
	fresh := storageMetricDedupKey("storage", "system:abc/dataset:Live", "usage")

	m.shouldPersistStorageMetric(stale, 1, now, now)
	m.shouldPersistStorageMetric(fresh, 1, now, now.Add(storageMetricDedupTTL))

	m.pruneStorageMetricDedup(now.Add(storageMetricDedupTTL + time.Second))

	m.storageMetricDedupMu.Lock()
	_, staleFound := m.storageMetricDedup[stale]
	_, freshFound := m.storageMetricDedup[fresh]
	size := len(m.storageMetricDedup)
	m.storageMetricDedupMu.Unlock()

	if staleFound {
		t.Error("series past the TTL should have been pruned")
	}
	if !freshFound {
		t.Error("recently-written series must be retained")
	}
	if size != 1 {
		t.Errorf("dedup map size = %d, want 1", size)
	}
}

// The measured production shape: ~89 datasets x 4 metric types, restated at every agent-report
// ingest boundary (~2.26s apart as measured) while their own source refreshes only every 60s.
func TestStorageMetricDedupCollapsesRestatementsToSourceRefreshRate(t *testing.T) {
	const (
		datasets      = 89
		metricsPerSet = 4
		ingestEvery   = 2260 * time.Millisecond
		sourceRefresh = 60 * time.Second
		window        = 10 * time.Minute
	)

	m := &Monitor{}
	start := time.Unix(1_700_000_000, 0)
	writes := 0
	unguarded := 0

	for now := start; now.Before(start.Add(window)); now = now.Add(ingestEvery) {
		// The source advances its observation time only once per refresh interval.
		observedAt := start.Add(now.Sub(start).Truncate(sourceRefresh))
		for d := 0; d < datasets; d++ {
			for s := 0; s < metricsPerSet; s++ {
				unguarded++
				key := storageMetricDedupKey("storage", "ds", string(rune('a'+d%26))+string(rune('0'+d/26))+string(rune('w'+s)))
				if m.shouldPersistStorageMetric(key, float64(s), observedAt, now) {
					writes++
				}
			}
		}
	}

	// Each series should be written once per source refresh, not once per ingest event.
	expected := datasets * metricsPerSet * int(window/sourceRefresh)
	if writes != expected {
		t.Errorf("writes = %d, want %d (one per series per source refresh)", writes, expected)
	}
	if ratio := float64(writes) / float64(unguarded); ratio > 0.1 {
		t.Errorf("guard only cut writes to %.1f%% of unguarded volume (%d/%d); want <10%%",
			ratio*100, writes, unguarded)
	}
	t.Logf("writes %d vs unguarded %d (%.1f%%)", writes, unguarded, float64(writes)/float64(unguarded)*100)
}
