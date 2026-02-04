package ai

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

func TestPatrolRunHistoryMemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory regression in short mode")
	}

	store := NewPatrolRunHistoryStore(60)

	runsPerCycle := 150
	warmupCycles := 4
	measureCycles := 12

	addCycle := func(offset int) {
		for i := 0; i < runsPerCycle; i++ {
			idx := offset + i
			started := time.Now().Add(-time.Duration(idx%120) * time.Second)
			completed := started.Add(2 * time.Second)
			store.Add(PatrolRunRecord{
				ID:               fmt.Sprintf("run-%d", idx),
				StartedAt:        started,
				CompletedAt:      completed,
				Duration:         completed.Sub(started),
				DurationMs:       int64(completed.Sub(started) / time.Millisecond),
				Type:             "patrol",
				TriggerReason:    "scheduled",
				ResourcesChecked: 10,
				NewFindings:      idx % 3,
				Status:           "healthy",
				FindingsSummary:  "All healthy",
			})
		}
		_ = store.GetRecent(10)
	}

	for i := 0; i < warmupCycles; i++ {
		addCycle(i * runsPerCycle)
	}

	runtime.GC()
	debug.FreeOSMemory()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	for i := 0; i < measureCycles; i++ {
		addCycle(100000 + i*runsPerCycle)
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
