package memory

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestIncidentStoreMemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory regression in short mode")
	}

	store := NewIncidentStore(IncidentStoreConfig{
		MaxIncidents:         50,
		MaxEventsPerIncident: 8,
		MaxAgeDays:           1,
	})

	alertsPerCycle := 150
	warmupCycles := 5
	measureCycles := 15

	makeAlert := func(i int) *alerts.Alert {
		resourceID := fmt.Sprintf("vm-%02d", i%20)
		return &alerts.Alert{
			ID:           fmt.Sprintf("alert-%d", i),
			Type:         "memory",
			Level:        alerts.AlertLevelWarning,
			ResourceID:   resourceID,
			ResourceName: resourceID,
			StartTime:    time.Now().Add(-2 * time.Minute),
			Value:        90,
			Threshold:    80,
		}
	}

	runCycle := func(offset int) {
		now := time.Now()
		for i := 0; i < alertsPerCycle; i++ {
			idx := offset + i
			alert := makeAlert(idx)
			store.RecordAlertFired(alert)
			store.RecordAlertAcknowledged(alert, "admin")
			store.RecordAnalysis(alert.ID, "analysis complete", map[string]interface{}{
				"iteration": idx,
			})
			store.RecordCommand(alert.ID, "systemctl restart app", true, "ok", nil)
			store.RecordRunbook(alert.ID, fmt.Sprintf("rb-%d", idx%5), "Restart service", "resolved", true, "ok")
			store.RecordNote(alert.ID, "", "note", "admin")
			store.RecordAlertResolved(alert, now)
		}
	}

	for i := 0; i < warmupCycles; i++ {
		runCycle(i * alertsPerCycle)
	}

	runtime.GC()
	debug.FreeOSMemory()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	for i := 0; i < measureCycles; i++ {
		runCycle(100000 + i*alertsPerCycle)
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
