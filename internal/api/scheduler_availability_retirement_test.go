package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// A target that has not polled yet owns queue work but no cached result.
// Delete it through the ordinary handler and read the real diagnostic route.
func TestDeleteAvailabilityTargetRetiresScheduledPoll(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(dir, "absent-legacy.key"))
	persistence := config.NewConfigPersistence(dir)
	synctest.Test(t, func(t *testing.T) {
		checkAvailabilityRetirementHTTP(t, persistence)
	})
}

func checkAvailabilityRetirementHTTP(t *testing.T, persistence *config.ConfigPersistence) {
	t.Helper()
	const deletedID = "fixture-deleted-check"
	if err := persistence.SaveAvailabilityTargets([]config.AvailabilityTarget{{
		ID: deletedID, Name: "Deleted check", Address: "192.0.2.10",
		Protocol: config.AvailabilityProbeICMP, Enabled: true, PollIntervalSecs: 60,
	}}); err != nil {
		t.Fatal(err)
	}
	monitor := &monitoring.Monitor{}
	queue := monitoring.NewTaskQueue()
	setUnexportedField(t, monitor, "taskQueue", queue)
	setUnexportedField(t, monitor, "state", models.NewState())
	setUnexportedField(t, monitor, "configPersist", persistence)
	// Intentionally use the same ID to check that provider scope is respected.
	queue.Upsert(monitoring.ScheduledTask{
		InstanceName: deletedID, InstanceType: monitoring.InstanceTypePVE,
		NextRun: time.Now().Add(20 * time.Minute),
	})
	monitor.RefreshAvailabilityTargets()
	time.Sleep(5 * time.Second)

	router := &Router{monitor: monitor}
	handler := NewAvailabilityHandlers(
		func(context.Context) *config.ConfigPersistence { return persistence },
		func(context.Context) *monitoring.Monitor { return monitor },
		nil,
	)
	snapshot := func(phase string) monitoring.SchedulerHealthResponse {
		t.Helper()
		w := httptest.NewRecorder()
		router.handleSchedulerHealth(w, httptest.NewRequest(http.MethodGet, "/api/monitoring/scheduler/health", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("Scheduler Health returned HTTP %d: %s", w.Code, w.Body)
		}
		t.Logf("SCHEDULER_HEALTH_%s %s", phase, w.Body.String())
		var result monitoring.SchedulerHealthResponse
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	before := snapshot("BEFORE_DELETE")
	if before.Queue.Depth != 2 || before.Queue.PerType["availability"] != 1 || before.Queue.PerType["pve"] != 1 ||
		before.Queue.DueWithinSeconds != 1 || before.Queue.OldestOverdueSeconds != 5 {
		t.Fatalf("fixture did not schedule both check and peer: %+v", before.Queue)
	}
	peer, ok := queue.Get(monitoring.InstanceTypePVE, deletedID)
	if !ok {
		t.Fatal("missing PVE peer before delete")
	}
	w := httptest.NewRecorder()
	handler.HandleDelete(w, httptest.NewRequest(http.MethodDelete, "/api/availability-targets/"+deletedID, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("delete returned HTTP %d: %s", w.Code, w.Body)
	}
	if remaining, err := persistence.LoadAvailabilityTargets(); len(remaining) != 0 || err != nil {
		t.Fatalf("delete did not persist, targets=%d err=%v", len(remaining), err)
	}
	after := snapshot("AFTER_DELETE")
	if after.Queue.Depth != 1 || after.Queue.PerType["availability"] != 0 || after.Queue.PerType["pve"] != 1 ||
		after.Queue.DueWithinSeconds != 0 || after.Queue.OldestOverdueSeconds != 0 {
		t.Errorf("deleted check still affects scheduler queue: %+v", after.Queue)
	}
	if current, ok := queue.Get(monitoring.InstanceTypePVE, deletedID); !ok || current != peer {
		t.Errorf("delete changed the unrelated PVE peer: %+v %v", current, ok)
	}
}
