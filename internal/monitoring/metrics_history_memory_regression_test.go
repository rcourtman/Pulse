package monitoring

import (
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

func TestMetricsHistoryMemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory regression in short mode")
	}

	history := NewMetricsHistory(64, time.Minute)
	warmupIterations := 50
	measureIterations := 200

	seed := func(iter int) {
		now := time.Now()
		value := float64(iter % 100)
		history.AddGuestMetric("guest-1", "memory", value, now)
		history.AddGuestMetric("guest-1", "cpu", value, now)
		history.AddNodeMetric("node-1", "memory", value, now)
		history.AddStorageMetric("storage-1", "usage", value, now)
		history.Cleanup()
	}

	for i := 0; i < warmupIterations; i++ {
		seed(i)
	}

	runtime.GC()
	debug.FreeOSMemory()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	for i := 0; i < measureIterations; i++ {
		seed(i)
	}

	runtime.GC()
	debug.FreeOSMemory()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	if baseline.HeapAlloc > 0 {
		allowed := baseline.HeapAlloc + 5*1024*1024
		growthRatio := float64(after.HeapAlloc) / float64(baseline.HeapAlloc)
		if after.HeapAlloc > allowed && growthRatio > 1.25 {
			t.Fatalf("heap allocation grew too much: baseline=%d final=%d ratio=%.2f", baseline.HeapAlloc, after.HeapAlloc, growthRatio)
		}
	}
}

// TestAppendMetricReleasesSeedSizedCapacity pins the demo-wedge fix: a dense
// historical seed builds a large backing array, and once retention trims the
// window down, the live append path must copy into a right-sized array
// rather than re-slicing and pinning the seed-sized one. Pre-fix, every
// series retained its full seed capacity (~2,800 points) forever while
// holding a few hundred live points, which pinned ~700MB across the demo
// estate's series and wedged the process against its memory cgroup.
func TestAppendMetricReleasesSeedSizedCapacity(t *testing.T) {
	mh := NewMetricsHistory(3500, time.Hour)

	const seedPoints = 2820
	values := make([]float64, seedPoints)
	timestamps := make([]time.Time, seedPoints)
	// Seed entirely outside the retention window so the first live append
	// trims it away, mirroring a seed that has aged out.
	start := time.Now().Add(-3 * time.Hour)
	for i := range values {
		values[i] = float64(i % 100)
		timestamps[i] = start.Add(time.Duration(i) * time.Second)
	}
	mh.addGuestMetricSeries("guest-1", "cpu", values, timestamps)

	series := mh.guestMetrics["guest-1"].CPU
	if len(series) != 0 {
		t.Fatalf("expected aged-out seed to be trimmed on backfill, got len=%d", len(series))
	}

	mh.AddGuestMetric("guest-1", "cpu", 42, time.Now())
	series = mh.guestMetrics["guest-1"].CPU
	if len(series) != 1 {
		t.Fatalf("expected exactly the live point after trim, got len=%d", len(series))
	}
	if cap(series) > 4*len(series)+64 {
		t.Fatalf("append pinned an oversized backing array: len=%d cap=%d", len(series), cap(series))
	}
}

// TestCleanupReleasesSeedSizedCapacity covers the same pinning through the
// periodic Cleanup path: a mostly-expired series must not keep its original
// backing array just because a few points survive the cutoff.
func TestCleanupReleasesSeedSizedCapacity(t *testing.T) {
	mh := NewMetricsHistory(3500, time.Hour)

	now := time.Now()
	metrics := &GuestMetrics{}
	mh.guestMetrics["guest-1"] = metrics
	// 2000 points spanning ~20h up to now; only ~100 fall inside the 1h
	// retention window, so cleanup must shed the array sized for all 2000.
	for i := 0; i < 2000; i++ {
		metrics.CPU = append(metrics.CPU, MetricPoint{
			Value:     float64(i % 100),
			Timestamp: now.Add(-20 * time.Hour).Add(time.Duration(i) * 36 * time.Second),
		})
	}
	before := cap(metrics.CPU)
	orig := metrics.CPU

	mh.Cleanup()

	series := mh.guestMetrics["guest-1"].CPU
	if len(series) == 0 {
		t.Fatal("expected some points to survive the cleanup cutoff")
	}
	if cap(series) > 4*len(series)+64 {
		t.Fatalf("cleanup pinned an oversized backing array: len=%d cap=%d (was %d)", len(series), cap(series), before)
	}
	// A front re-slice would alias the tail of the original array while
	// keeping all of it reachable; cap() alone cannot see that, so assert
	// the surviving window no longer lives inside the oversized array.
	if &series[0] == &orig[len(orig)-len(series)] {
		t.Fatalf("cleanup still aliases the seed-sized backing array (len=%d cap-was=%d)", len(series), before)
	}
}
