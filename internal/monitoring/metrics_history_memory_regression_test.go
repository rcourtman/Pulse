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
