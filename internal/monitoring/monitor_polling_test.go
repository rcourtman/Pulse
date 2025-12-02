package monitoring

import (
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
)

func TestBuildScheduledTasksUsesConfiguredIntervals(t *testing.T) {
	now := time.Now()
	cfg := &config.Config{
		PVEPollingInterval:          2 * time.Minute,
		PBSPollingInterval:          45 * time.Second,
		PMGPollingInterval:          90 * time.Second,
		AdaptivePollingBaseInterval: 10 * time.Second,
	}

	monitor := &Monitor{
		config:     cfg,
		pveClients: map[string]PVEClientInterface{"pve-1": nil},
		pbsClients: map[string]*pbs.Client{"pbs-1": nil},
		pmgClients: map[string]*pmg.Client{"pmg-1": nil},
	}

	tasks := monitor.buildScheduledTasks(now)
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	got := map[InstanceType]time.Duration{}
	for _, task := range tasks {
		if !task.NextRun.Equal(now) {
			t.Fatalf("expected NextRun to equal provided time, got %v", task.NextRun)
		}
		got[task.InstanceType] = task.Interval
	}

	if got[InstanceTypePVE] != cfg.PVEPollingInterval {
		t.Fatalf("expected PVE interval %v, got %v", cfg.PVEPollingInterval, got[InstanceTypePVE])
	}
	if got[InstanceTypePBS] != cfg.PBSPollingInterval {
		t.Fatalf("expected PBS interval %v, got %v", cfg.PBSPollingInterval, got[InstanceTypePBS])
	}
	if got[InstanceTypePMG] != cfg.PMGPollingInterval {
		t.Fatalf("expected PMG interval %v, got %v", cfg.PMGPollingInterval, got[InstanceTypePMG])
	}
}

func TestRescheduleTaskUsesInstanceIntervalWhenSchedulerDisabled(t *testing.T) {
	cfg := &config.Config{
		PVEPollingInterval:          75 * time.Second,
		AdaptivePollingBaseInterval: 10 * time.Second,
	}

	monitor := &Monitor{
		config:    cfg,
		taskQueue: NewTaskQueue(),
	}

	task := ScheduledTask{
		InstanceName: "pve-1",
		InstanceType: InstanceTypePVE,
		Interval:     0,
		NextRun:      time.Now(),
	}

	monitor.rescheduleTask(task)

	monitor.taskQueue.mu.Lock()
	entry, ok := monitor.taskQueue.entries[schedulerKey(task.InstanceType, task.InstanceName)]
	monitor.taskQueue.mu.Unlock()
	if !ok {
		t.Fatalf("expected task to be rescheduled in queue")
	}

	if entry.task.Interval != cfg.PVEPollingInterval {
		t.Fatalf("expected interval %v, got %v", cfg.PVEPollingInterval, entry.task.Interval)
	}

	remaining := time.Until(entry.task.NextRun)
	if remaining < cfg.PVEPollingInterval-2*time.Second || remaining > cfg.PVEPollingInterval+time.Second {
		t.Fatalf("expected NextRun about %v from now, got %v", cfg.PVEPollingInterval, remaining)
	}
}

func TestRecordTaskResult_NilMonitor(t *testing.T) {
	// Should not panic when called on nil monitor
	var m *Monitor
	m.recordTaskResult(InstanceTypePVE, "test-instance", nil)
	// If we get here without panic, the test passes
}

func TestRecordTaskResult_Success(t *testing.T) {
	m := &Monitor{
		pollStatusMap: make(map[string]*pollStatus),
		failureCounts: make(map[string]int),
		lastOutcome:   make(map[string]taskOutcome),
		circuitBreakers: make(map[string]*circuitBreaker),
	}

	// Record a success
	m.recordTaskResult(InstanceTypePVE, "test-instance", nil)

	key := schedulerKey(InstanceTypePVE, "test-instance")

	// Verify failure count is reset
	if m.failureCounts[key] != 0 {
		t.Errorf("expected failureCounts[%s] = 0, got %d", key, m.failureCounts[key])
	}

	// Verify last outcome is success
	outcome, ok := m.lastOutcome[key]
	if !ok {
		t.Fatalf("expected lastOutcome[%s] to exist", key)
	}
	if !outcome.success {
		t.Error("expected outcome.success = true")
	}

	// Verify poll status
	status, ok := m.pollStatusMap[key]
	if !ok {
		t.Fatalf("expected pollStatusMap[%s] to exist", key)
	}
	if status.ConsecutiveFailures != 0 {
		t.Errorf("expected ConsecutiveFailures = 0, got %d", status.ConsecutiveFailures)
	}
}

func TestRecordTaskResult_Failure(t *testing.T) {
	m := &Monitor{
		pollStatusMap: make(map[string]*pollStatus),
		failureCounts: make(map[string]int),
		lastOutcome:   make(map[string]taskOutcome),
		circuitBreakers: make(map[string]*circuitBreaker),
	}

	testErr := errors.New("connection refused")

	// Record a failure
	m.recordTaskResult(InstanceTypePVE, "test-instance", testErr)

	key := schedulerKey(InstanceTypePVE, "test-instance")

	// Verify failure count is incremented
	if m.failureCounts[key] != 1 {
		t.Errorf("expected failureCounts[%s] = 1, got %d", key, m.failureCounts[key])
	}

	// Verify last outcome is failure
	outcome, ok := m.lastOutcome[key]
	if !ok {
		t.Fatalf("expected lastOutcome[%s] to exist", key)
	}
	if outcome.success {
		t.Error("expected outcome.success = false")
	}
	if outcome.err != testErr {
		t.Errorf("expected outcome.err = %v, got %v", testErr, outcome.err)
	}

	// Verify poll status
	status, ok := m.pollStatusMap[key]
	if !ok {
		t.Fatalf("expected pollStatusMap[%s] to exist", key)
	}
	if status.ConsecutiveFailures != 1 {
		t.Errorf("expected ConsecutiveFailures = 1, got %d", status.ConsecutiveFailures)
	}
	if status.LastErrorMessage != "connection refused" {
		t.Errorf("expected LastErrorMessage = 'connection refused', got %q", status.LastErrorMessage)
	}
}

func TestRecordTaskResult_ConsecutiveFailures(t *testing.T) {
	m := &Monitor{
		pollStatusMap: make(map[string]*pollStatus),
		failureCounts: make(map[string]int),
		lastOutcome:   make(map[string]taskOutcome),
		circuitBreakers: make(map[string]*circuitBreaker),
	}

	testErr := errors.New("timeout")

	// Record multiple failures
	m.recordTaskResult(InstanceTypePBS, "pbs-server", testErr)
	m.recordTaskResult(InstanceTypePBS, "pbs-server", testErr)
	m.recordTaskResult(InstanceTypePBS, "pbs-server", testErr)

	key := schedulerKey(InstanceTypePBS, "pbs-server")

	// Verify consecutive failures count
	status := m.pollStatusMap[key]
	if status.ConsecutiveFailures != 3 {
		t.Errorf("expected ConsecutiveFailures = 3, got %d", status.ConsecutiveFailures)
	}

	// FirstFailureAt should be set on first failure and not change
	if status.FirstFailureAt.IsZero() {
		t.Error("expected FirstFailureAt to be set")
	}
}

func TestRecordTaskResult_SuccessResetsFailures(t *testing.T) {
	m := &Monitor{
		pollStatusMap: make(map[string]*pollStatus),
		failureCounts: make(map[string]int),
		lastOutcome:   make(map[string]taskOutcome),
		circuitBreakers: make(map[string]*circuitBreaker),
	}

	testErr := errors.New("error")
	key := schedulerKey(InstanceTypePMG, "pmg-server")

	// Record some failures first
	m.recordTaskResult(InstanceTypePMG, "pmg-server", testErr)
	m.recordTaskResult(InstanceTypePMG, "pmg-server", testErr)

	if m.pollStatusMap[key].ConsecutiveFailures != 2 {
		t.Fatalf("expected 2 failures before reset")
	}

	// Now record a success
	m.recordTaskResult(InstanceTypePMG, "pmg-server", nil)

	// Verify everything is reset
	if m.failureCounts[key] != 0 {
		t.Errorf("expected failureCounts to be reset to 0, got %d", m.failureCounts[key])
	}
	if m.pollStatusMap[key].ConsecutiveFailures != 0 {
		t.Errorf("expected ConsecutiveFailures to be reset to 0, got %d", m.pollStatusMap[key].ConsecutiveFailures)
	}
	if !m.pollStatusMap[key].FirstFailureAt.IsZero() {
		t.Error("expected FirstFailureAt to be reset to zero")
	}
}

func TestRecordTaskResult_NilMaps(t *testing.T) {
	// Monitor with nil internal maps - should not panic
	m := &Monitor{
		pollStatusMap: make(map[string]*pollStatus),
		failureCounts: nil, // nil
		lastOutcome:   nil, // nil
		circuitBreakers: make(map[string]*circuitBreaker),
	}

	// Should not panic
	m.recordTaskResult(InstanceTypePVE, "test", nil)
	m.recordTaskResult(InstanceTypePVE, "test", errors.New("error"))

	// pollStatusMap should still be updated
	key := schedulerKey(InstanceTypePVE, "test")
	if _, ok := m.pollStatusMap[key]; !ok {
		t.Error("expected pollStatusMap to be updated even with nil failureCounts/lastOutcome")
	}
}
