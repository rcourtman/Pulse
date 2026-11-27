package monitoring

import (
	"math"
	"runtime"
	"runtime/debug"
	"slices"
	"testing"
	"testing/synctest"
	"time"
)

func TestMetricsHistoryFullWindowReusesBackingArray(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const window = 1000 // The live monitor's in-memory point limit.
		mh := NewMetricsHistory(window, 24*time.Hour)
		now := time.Now()
		full := make([]MetricPoint, window)
		for i := range full {
			full[i] = MetricPoint{Value: float64(i), Timestamp: now.Add(time.Duration(i-window) * time.Second)}
		}
		want := append(slices.Clone(full[1:]), MetricPoint{Value: 1001, Timestamp: now})
		mh.guestMetrics["vm"] = &GuestMetrics{CPU: full}

		mh.AddGuestMetric("vm", "cpu", 1001, now)

		got := mh.guestMetrics["vm"].CPU
		if !slices.Equal(got, want) {
			t.Fatal("rolling the full window changed the retained samples")
		}
		if cap(got) != window || &got[0] != &full[0] {
			t.Errorf("full window allocated a new backing array: capacity %d -> %d points", window, cap(got))
		}
	})
}

func TestMetricsHistoryFullWindowReusesExpiredSlot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mh := NewMetricsHistory(1000, 24*time.Hour)
		now := time.Now()
		for i := 0; ; i++ {
			// Fill a naturally grown live buffer between 500 and 999 points.
			// When its first observation expires the next sample fits.
			const cadence = 150 * time.Second
			at := now.Add(-24 * time.Hour).Add(time.Duration(i+1) * cadence)
			mh.AddGuestMetric("vm", "cpu", float64(i), at)
			series := mh.guestMetrics["vm"].CPU
			if len(series) >= 500 && len(series) == cap(series) {
				time.Sleep(time.Until(series[0].Timestamp.Add(24 * time.Hour)))
				next := MetricPoint{Value: 1001, Timestamp: time.Now()}
				want := append(slices.Clone(series[1:]), next)
				mh.AddGuestMetric("vm", "cpu", next.Value, next.Timestamp)
				got := mh.guestMetrics["vm"].CPU
				if !slices.Equal(got, want) {
					t.Fatal("retention trim changed the samples kept on append")
				}
				if cap(got) != cap(series) || &got[0] != &series[0] {
					t.Errorf("expired slot allocated a new backing array: capacity %d -> %d points", cap(series), cap(got))
				}
				break
			}
			if i >= 998 {
				t.Fatal("fixture did not reach a full buffer before the point limit")
			}
		}
	})
}

func TestMetricsHistoryFullWindowPreservesTrimOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		now := time.Now()
		cutoff := now.Add(-time.Hour)
		fresh := MetricPoint{Value: 1, Timestamp: now.Add(-time.Minute)}
		expired := MetricPoint{Value: 2, Timestamp: cutoff}
		incoming := MetricPoint{Value: 3, Timestamp: now}
		for _, tc := range []struct {
			name      string
			old, want []MetricPoint
			point     MetricPoint
		}{
			{"single slot", []MetricPoint{fresh}, []MetricPoint{incoming}, incoming},
			{"late sample previously kept behind fresh head", []MetricPoint{fresh, expired}, []MetricPoint{expired, incoming}, incoming},
			{"retention trims expired prefix at cutoff", []MetricPoint{expired, expired, fresh}, []MetricPoint{fresh, incoming}, incoming},
			{"expired series accepts fresh sample", []MetricPoint{expired, expired}, []MetricPoint{incoming}, incoming},
			{"no retained samples", []MetricPoint{expired}, nil, MetricPoint{Value: 3, Timestamp: cutoff.Add(-time.Minute)}},
			{"duplicate tail replaces instead of evicting", []MetricPoint{fresh, incoming}, []MetricPoint{fresh, {Value: 4, Timestamp: now}}, MetricPoint{Value: 4, Timestamp: now}},
			{"invalid sample leaves entire buffer untouched", []MetricPoint{fresh, incoming}, []MetricPoint{fresh, incoming}, MetricPoint{Value: math.NaN(), Timestamp: now}},
		} {
			full := make([]MetricPoint, len(tc.old))
			copy(full, tc.old)
			mh := NewMetricsHistory(len(full), time.Hour)
			if got := mh.appendMetric(full, tc.point); !slices.Equal(got, tc.want) {
				t.Errorf("%s: retained samples = %v, want %v", tc.name, got, tc.want)
			}
		}
	})
}

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

func TestCleanupUsedMemoryReleasesSeedSizedCapacity(t *testing.T) {
	for _, scope := range []string{"guest", "node"} {
		t.Run(scope, func(t *testing.T) {
			mh := NewMetricsHistory(3500, 4*time.Hour)
			add := mh.AddGuestMetric
			entries := mh.guestMetrics
			if scope == "node" {
				add, entries = mh.AddNodeMetric, mh.nodeMetrics
			}
			now := time.Now()
			for i := 0; i < 2000; i++ {
				add("resource", "memoryused", float64(i), now.Add(-2*time.Hour).Add(time.Duration(i)*time.Second))
			}
			add("resource", "memoryused", 4096, now.Add(-time.Minute))
			add("resource", "cpu", 25, now)
			original := entries["resource"].MemoryUsed
			mh.retentionTime = time.Hour
			mh.Cleanup()
			series := entries["resource"].MemoryUsed
			if len(series) != 1 || series[0].Value != 4096 || !series[0].Timestamp.Equal(now.Add(-time.Minute)) {
				t.Fatalf("used-memory retention kept %d points, want the unchanged live point", len(series))
			}
			if cap(series) > 4*len(series)+64 {
				t.Fatalf("used-memory cleanup pinned an oversized array: len=%d cap=%d", len(series), cap(series))
			}
			if &series[0] == &original[0] || &series[0] == &original[len(original)-1] {
				t.Fatal("used-memory cleanup still aliases the seed-sized array")
			}
			t.Logf("used-memory samples %d -> %d; backing capacity %d -> %d", len(original), len(series), cap(original), cap(series))
		})
	}
}
