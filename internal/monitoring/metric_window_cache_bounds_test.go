package monitoring

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// These controls use only the in-process cache. No database, collector,
// listener, process profile, forced GC or installed workload is involved.
func TestMetricWindowCacheBoundsLiveEntries(t *testing.T) {
	history := NewMetricsHistory(32, time.Hour)
	now := time.Now()
	for i := 0; i < 1100; i++ {
		history.cacheMetricWindow(fmt.Sprintf("vm-%04d", i), []MetricPoint{{Timestamp: now, Value: float64(i)}}, now)
	}
	if got := len(history.metricWindowCache); got > 1024 {
		t.Fatalf("live cache entries = %d, want at most 1024; expiry is not a capacity bound", got)
	}
	if got := len(history.metricWindowCache); got != metricWindowCacheMaxEntries {
		t.Fatalf("retained %d entries, want the admitted working set %d", got, metricWindowCacheMaxEntries)
	}
	// Refusing a new copy must not evict or corrupt an unexpired accepted one.
	if got, ok := history.cachedMetricWindow("vm-0000", now); !ok || len(got) != 1 || got[0].Value != 0 {
		t.Fatalf("capacity refusal lost an accepted measured-zero window: %+v, hit=%v", got, ok)
	}
	if _, ok := history.cachedMetricWindow("vm-1099", now); ok {
		t.Fatal("a refused window was retained")
	}
	assertMetricWindowCacheAccounting(t, history)
}

func TestMetricWindowCacheBoundsDenseWindows(t *testing.T) {
	history := NewMetricsHistory(32, time.Hour)
	now := time.Now()
	points := make([]MetricPoint, 4096)
	for i := range points {
		points[i] = MetricPoint{Timestamp: now.Add(time.Duration(i) * time.Millisecond), Value: float64(i)}
	}
	for i := 0; i < 200; i++ {
		history.cacheMetricWindow(fmt.Sprintf("vm-%04d", i), points, now)
	}
	retained := 0
	for _, entry := range history.metricWindowCache {
		retained += len(entry.points)
	}
	// Charge points conservatively at 64 bytes, plus keys/map/entry overhead.
	if retained*64 > 16<<20 {
		t.Fatalf("retained %d dense points (%d charged point bytes), want at most 16 MiB", retained, retained*64)
	}
	if retained == 0 {
		t.Fatal("budget disabled the ordinary dense working set")
	}
	assertMetricWindowCacheAccounting(t, history)
}

func assertMetricWindowCacheAccounting(t *testing.T, history *MetricsHistory) {
	t.Helper()
	history.metricWindowMu.Lock()
	defer history.metricWindowMu.Unlock()
	sum := 0
	for key, entry := range history.metricWindowCache {
		want := metricWindowCacheEntryBytes + len(key) + len(entry.points)*metricWindowCachePointBytes
		if entry.bytes != want || cap(entry.points) != len(entry.points) {
			t.Fatalf("entry charge/backing capacity diverged: bytes=%d want=%d len=%d cap=%d", entry.bytes, want, len(entry.points), cap(entry.points))
		}
		sum += entry.bytes
	}
	if sum != history.metricWindowBytes || sum > metricWindowCacheMaxBytes || len(history.metricWindowCache) > metricWindowCacheMaxEntries {
		t.Fatalf("cache budget diverged: summed=%d accounted=%d entries=%d", sum, history.metricWindowBytes, len(history.metricWindowCache))
	}
}

func TestMetricWindowCacheBudgetIncludesKeysAndEmptyWindows(t *testing.T) {
	if metricWindowCachePointBytes < int(unsafe.Sizeof(alerts.MetricWindowPoint{})) {
		t.Fatal("point budget no longer covers the native point representation")
	}
	history := NewMetricsHistory(32, time.Hour)
	now := time.Now()
	// No point bytes: large keys alone must be bounded, too.
	prefix := strings.Repeat("x", 32<<10)
	for i := 0; i < 1024; i++ {
		history.cacheMetricWindow(fmt.Sprintf("%s-%04d", prefix, i), nil, now)
	}
	assertMetricWindowCacheAccounting(t, history)
	if len(history.metricWindowCache) == 0 || len(history.metricWindowCache) >= metricWindowCacheMaxEntries {
		t.Fatal("the byte budget did not admit/bound long empty-window keys")
	}
}

func TestMetricWindowCacheOversizedReplacementDoesNotReviveOldReadings(t *testing.T) {
	now := time.Now()
	for _, kind := range []string{"points", "key"} {
		t.Run(kind, func(t *testing.T) {
			history := NewMetricsHistory(32, time.Hour)
			key := "vm-1"
			points := []MetricPoint{{Timestamp: now, Value: 5}}
			if kind == "key" {
				key = strings.Repeat("x", metricWindowCacheMaxBytes)
			} else {
				history.cacheMetricWindow(key, points, now)
				points = make([]MetricPoint, metricWindowCacheMaxBytes/metricWindowCachePointBytes+1)
			}
			history.cacheMetricWindow(key, points, now)
			if _, ok := history.cachedMetricWindow(key, now); ok {
				t.Fatal("an oversized window revived a previous reading or kept its extra copy")
			}
			if history.metricWindowCache != nil || history.metricWindowBytes != 0 {
				t.Fatal("oversized admission allocated or retained the cache working set")
			}
		})
	}
}

func TestMetricWindowCacheReplacementAndIndependentOwnership(t *testing.T) {
	history := NewMetricsHistory(32, time.Hour)
	now := time.Now()
	points := []MetricPoint{{Timestamp: now, Value: 42}, {Timestamp: now.Add(time.Second), Value: 99}}
	history.cacheMetricWindow("vm-1", points, now)
	points[0].Value = -1
	got, ok := history.cachedMetricWindow("vm-1", now)
	if !ok || len(got) != 2 || got[0].Value != 42 || got[1].Value != 99 {
		t.Fatalf("admitted window changed with query storage: %+v, hit=%v", got, ok)
	}
	got[0].Value = -2
	again, _ := history.cachedMetricWindow("vm-1", now)
	if again[0].Value != 42 {
		t.Fatal("a reader changed another reader's retained window")
	}
	history.cacheMetricWindow("vm-1", []MetricPoint{{Timestamp: now, Value: 0}}, now.Add(time.Second))
	assertMetricWindowCacheAccounting(t, history)
	if len(history.metricWindowCache) != 1 || len(history.metricWindowCache["vm-1"].points) != 1 {
		t.Fatal("replacement double-counted or retained the old backing array")
	}
}

func TestMetricWindowCacheExpiryReleasesAndReadmitsAtCapacity(t *testing.T) {
	history := NewMetricsHistory(32, time.Hour)
	now := time.Now()
	for i := 0; i < metricWindowCacheMaxEntries; i++ {
		history.cacheMetricWindow(fmt.Sprintf("vm-%04d", i), nil, now)
	}
	expiry := now.Add(metricWindowPersistentCacheTTL)
	if _, ok := history.cachedMetricWindow("vm-0000", expiry); ok {
		t.Fatal("exact-expiry lookup renewed stale evidence")
	}
	// An ordinary insert sweeps the other expired windows and frees budget.
	history.cacheMetricWindow("new", []MetricPoint{{Timestamp: expiry, Value: 7}}, expiry)
	assertMetricWindowCacheAccounting(t, history)
	if len(history.metricWindowCache) != 1 || history.metricWindowSweepAt != expiry.Add(metricWindowPersistentCacheTTL) {
		t.Fatal("expired windows prevented ordinary re-admission or reset the wrong expiry")
	}
	if _, ok := history.cachedMetricWindow("new", expiry.Add(metricWindowPersistentCacheTTL)); ok {
		t.Fatal("replacement outlived its own TTL")
	}
	if history.metricWindowCache != nil || history.metricWindowBytes != 0 || !history.metricWindowSweepAt.IsZero() {
		t.Fatal("the last expired entry kept map storage or accounting")
	}
}

func TestMetricWindowCacheSweepKeepsUnexpiredStaggeredWindows(t *testing.T) {
	history := NewMetricsHistory(32, time.Hour)
	now := time.Now()
	history.cacheMetricWindow("old", nil, now)
	history.cacheMetricWindow("later", nil, now.Add(10*time.Second))
	expiry := now.Add(metricWindowPersistentCacheTTL)
	history.cacheMetricWindow("new", nil, expiry)
	if _, ok := history.cachedMetricWindow("old", expiry); ok {
		t.Fatal("sweep retained expired history")
	}
	if _, ok := history.cachedMetricWindow("later", expiry); !ok {
		t.Fatal("sweep discarded an unexpired independent window")
	}
	if history.metricWindowSweepAt != now.Add(10*time.Second+metricWindowPersistentCacheTTL) {
		t.Fatal("sweep did not follow the earliest live expiry")
	}
	assertMetricWindowCacheAccounting(t, history)
}

func TestMetricWindowCacheResetAndConcurrentReadersWriters(t *testing.T) {
	history := NewMetricsHistory(32, time.Hour)
	now := time.Now()
	var workers sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := 0; i < 128; i++ {
				key := fmt.Sprintf("worker-%02d-%03d", worker, i)
				history.cacheMetricWindow(key, []MetricPoint{{Timestamp: now, Value: float64(worker)}}, now)
				if got, ok := history.cachedMetricWindow(key, now); ok {
					if len(got) != 1 || got[0].Value != float64(worker) {
						t.Error("concurrent caller received another window's values")
						continue
					}
					got[0].Value = -1
				}
				if i%31 == 0 {
					history.Reset()
				}
			}
		}()
	}
	workers.Wait()
	assertMetricWindowCacheAccounting(t, history)
	history.Reset()
	if history.metricWindowCache != nil || history.metricWindowBytes != 0 || !history.metricWindowSweepAt.IsZero() {
		t.Fatal("reset retained history cache storage/accounting")
	}
}
