package learning

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

func TestLearningStoreMemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory regression in short mode")
	}

	store := NewLearningStore(LearningStoreConfig{
		MaxRecords:    200,
		RetentionDays: 1,
	})

	recordsPerCycle := 300
	warmupCycles := 5
	measureCycles := 15

	recordFeedback := func(offset int) {
		now := time.Now()
		for i := 0; i < recordsPerCycle; i++ {
			idx := offset + i
			record := FeedbackRecord{
				FindingID:  fmt.Sprintf("finding-%d", idx),
				FindingKey: fmt.Sprintf("key-%d", idx%10),
				ResourceID: fmt.Sprintf("vm-%02d", idx%20),
				Category:   "performance",
				Severity:   "high",
				Action:     ActionAcknowledge,
				Timestamp:  now.Add(-time.Duration(idx%60) * time.Minute),
				UserNote:   "acknowledged",
			}
			if idx%3 == 0 {
				record.Action = ActionDismissNotAnIssue
			}
			store.RecordFeedback(record)
		}
		store.Cleanup()
		store.GetStatistics()
	}

	for i := 0; i < warmupCycles; i++ {
		recordFeedback(i * recordsPerCycle)
	}

	runtime.GC()
	debug.FreeOSMemory()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	for i := 0; i < measureCycles; i++ {
		recordFeedback(100000 + i*recordsPerCycle)
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
