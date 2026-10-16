package config_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestPatrolScheduleAndHistoryWritersPreserveEachOther(t *testing.T) {
	cp := config.NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()
	runs := []config.PatrolRunRecord{{ID: "kept", Type: "patrol", StartedAt: now.Add(-time.Minute), CompletedAt: now, Status: "healthy"}}
	if err := cp.SavePatrolRunHistory(runs); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		for range 20 {
			if err := cp.SavePatrolRunHistory(runs); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := range 20 {
			if err := cp.SavePatrolSchedule(now.Add(time.Duration(i)*time.Minute), 10*time.Minute); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	close(start)
	wg.Wait()
	data, err := cp.LoadPatrolRunHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Runs) != 1 || data.Runs[0].ID != "kept" || !data.NextScheduledAt.Equal(now.Add(19*time.Minute)) || data.ScheduleInterval != 10*time.Minute {
		t.Fatalf("concurrent history/schedule writes lost state: %+v", data)
	}
	if data.DailyRuns[now.Format("2006-01-02")] != 1 {
		t.Fatalf("schedule writes changed daily run tally: %v", data.DailyRuns)
	}
}

func TestPatrolScheduleWritePreservesUnreadableHistory(t *testing.T) {
	dir := t.TempDir()
	cp := config.NewConfigPersistence(dir)
	path := filepath.Join(dir, "ai_patrol_runs.json")
	before := []byte("{not valid history")
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	if err := cp.SavePatrolSchedule(time.Now(), 10*time.Minute); err == nil {
		t.Fatal("saving the deadline overwrote unreadable history")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("unreadable history changed: %q, %v", after, err)
	}
}
