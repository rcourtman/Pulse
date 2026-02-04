package monitoring

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

func TestTaskQueueMemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory regression in short mode")
	}

	queue := NewTaskQueue()
	taskCount := 250
	warmupCycles := 10
	measureCycles := 50

	makeTask := func(i int) ScheduledTask {
		return ScheduledTask{
			InstanceName: fmt.Sprintf("node-%03d", i),
			InstanceType: InstanceTypePVE,
			NextRun:      time.Now().Add(time.Duration(i) * time.Second),
			Interval:     10 * time.Second,
			Priority:     1,
		}
	}

	cycle := func() {
		for i := 0; i < taskCount; i++ {
			queue.Upsert(makeTask(i))
		}
		for i := 0; i < taskCount; i++ {
			queue.Remove(InstanceTypePVE, fmt.Sprintf("node-%03d", i))
		}
	}

	for i := 0; i < warmupCycles; i++ {
		cycle()
	}

	runtime.GC()
	debug.FreeOSMemory()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	for i := 0; i < measureCycles; i++ {
		cycle()
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
