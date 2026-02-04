package memory

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

func TestRemediationLogMemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory regression in short mode")
	}

	log := NewRemediationLog(RemediationLogConfig{
		MaxRecords: 80,
	})

	recordsPerCycle := 200
	warmupCycles := 4
	measureCycles := 12

	recordCycle := func(offset int) {
		for i := 0; i < recordsPerCycle; i++ {
			idx := offset + i
			err := log.Log(RemediationRecord{
				ResourceID:   fmt.Sprintf("vm-%02d", idx%20),
				ResourceType: "vm",
				ResourceName: fmt.Sprintf("vm-%02d", idx%20),
				Problem:      "memory spike",
				Summary:      "restart service",
				Action:       "systemctl restart app",
				Output:       "ok",
				Outcome:      OutcomeResolved,
				Duration:     2 * time.Second,
				Automatic:    idx%2 == 0,
				Timestamp:    time.Now().Add(-time.Duration(idx%60) * time.Second),
			})
			if err != nil {
				t.Fatalf("Log: %v", err)
			}
		}
		_ = log.GetForResource("vm-01", 5)
		_ = log.GetSimilar("memory spike", 5)
	}

	for i := 0; i < warmupCycles; i++ {
		recordCycle(i * recordsPerCycle)
	}

	runtime.GC()
	debug.FreeOSMemory()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	for i := 0; i < measureCycles; i++ {
		recordCycle(100000 + i*recordsPerCycle)
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
