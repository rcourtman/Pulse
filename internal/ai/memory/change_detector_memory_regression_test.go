package memory

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

func TestChangeDetectorMemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory regression in short mode")
	}

	detector := NewChangeDetector(ChangeDetectorConfig{
		MaxChanges: 50,
	})

	resourcesPerCycle := 120
	warmupCycles := 4
	measureCycles := 12

	buildSnapshots := func(offset int, status string) []ResourceSnapshot {
		now := time.Now()
		snapshots := make([]ResourceSnapshot, 0, resourcesPerCycle)
		for i := 0; i < resourcesPerCycle; i++ {
			snapshots = append(snapshots, ResourceSnapshot{
				ID:           fmt.Sprintf("vm-%03d", i),
				Name:         fmt.Sprintf("vm-%03d", i),
				Type:         "vm",
				Status:       status,
				Node:         fmt.Sprintf("node-%d", i%3),
				CPUCores:     2 + ((i + offset) % 4),
				MemoryBytes:  int64(2+(i+offset)%4) * 1024 * 1024 * 1024,
				DiskBytes:    int64(20+i%10) * 1024 * 1024 * 1024,
				LastBackup:   now.Add(-time.Duration((i+offset)%5) * time.Hour),
				SnapshotTime: now,
			})
		}
		return snapshots
	}

	for i := 0; i < warmupCycles; i++ {
		status := "running"
		if i%2 == 1 {
			status = "stopped"
		}
		detector.DetectChanges(buildSnapshots(i*resourcesPerCycle, status))
	}

	runtime.GC()
	debug.FreeOSMemory()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	for i := 0; i < measureCycles; i++ {
		status := "running"
		if i%2 == 1 {
			status = "stopped"
		}
		detector.DetectChanges(buildSnapshots(100000+i*resourcesPerCycle, status))
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
