package patterns

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

func TestPatternDetectorMemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory regression in short mode")
	}

	detector := NewDetector(DetectorConfig{
		MaxEvents:       500,
		MinOccurrences:  2,
		PatternWindow:   24 * time.Hour,
		PredictionLimit: 7 * 24 * time.Hour,
	})

	eventsPerCycle := 600
	warmupCycles := 4
	measureCycles := 12
	alertTypes := []string{"memory_warning", "cpu_warning", "disk_warning", "oom"}

	recordCycle := func(offset int) {
		now := time.Now()
		for i := 0; i < eventsPerCycle; i++ {
			idx := offset + i
			resourceID := fmt.Sprintf("vm-%02d", idx%20)
			alertType := alertTypes[idx%len(alertTypes)]
			detector.RecordFromAlert(resourceID, alertType, now.Add(-time.Duration(idx%300)*time.Second))
		}
	}

	for i := 0; i < warmupCycles; i++ {
		recordCycle(i * eventsPerCycle)
	}

	runtime.GC()
	debug.FreeOSMemory()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	for i := 0; i < measureCycles; i++ {
		recordCycle(100000 + i*eventsPerCycle)
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
