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
		pollStatusMap:   make(map[string]*pollStatus),
		failureCounts:   make(map[string]int),
		lastOutcome:     make(map[string]taskOutcome),
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
		pollStatusMap:   make(map[string]*pollStatus),
		failureCounts:   make(map[string]int),
		lastOutcome:     make(map[string]taskOutcome),
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
		pollStatusMap:   make(map[string]*pollStatus),
		failureCounts:   make(map[string]int),
		lastOutcome:     make(map[string]taskOutcome),
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
		pollStatusMap:   make(map[string]*pollStatus),
		failureCounts:   make(map[string]int),
		lastOutcome:     make(map[string]taskOutcome),
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
		pollStatusMap:   make(map[string]*pollStatus),
		failureCounts:   nil, // nil
		lastOutcome:     nil, // nil
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

func TestDescribeInstancesForScheduler_NoClients(t *testing.T) {
	m := &Monitor{
		pveClients: make(map[string]PVEClientInterface),
		pbsClients: make(map[string]*pbs.Client),
		pmgClients: make(map[string]*pmg.Client),
	}

	descriptors := m.describeInstancesForScheduler()
	if descriptors != nil {
		t.Errorf("expected nil for empty clients, got %v", descriptors)
	}
}

func TestDescribeInstancesForScheduler_PVEOnly(t *testing.T) {
	m := &Monitor{
		pveClients: map[string]PVEClientInterface{"pve-1": nil, "pve-2": nil},
		pbsClients: make(map[string]*pbs.Client),
		pmgClients: make(map[string]*pmg.Client),
	}

	descriptors := m.describeInstancesForScheduler()
	if len(descriptors) != 2 {
		t.Fatalf("expected 2 descriptors, got %d", len(descriptors))
	}

	// Should be sorted
	if descriptors[0].Name != "pve-1" || descriptors[1].Name != "pve-2" {
		t.Errorf("expected sorted order [pve-1, pve-2], got [%s, %s]", descriptors[0].Name, descriptors[1].Name)
	}

	for _, desc := range descriptors {
		if desc.Type != InstanceTypePVE {
			t.Errorf("expected type PVE, got %v", desc.Type)
		}
	}
}

func TestDescribeInstancesForScheduler_PBSOnly(t *testing.T) {
	m := &Monitor{
		pveClients: make(map[string]PVEClientInterface),
		pbsClients: map[string]*pbs.Client{"pbs-backup": nil},
		pmgClients: make(map[string]*pmg.Client),
	}

	descriptors := m.describeInstancesForScheduler()
	if len(descriptors) != 1 {
		t.Fatalf("expected 1 descriptor, got %d", len(descriptors))
	}

	if descriptors[0].Name != "pbs-backup" {
		t.Errorf("expected name 'pbs-backup', got %q", descriptors[0].Name)
	}
	if descriptors[0].Type != InstanceTypePBS {
		t.Errorf("expected type PBS, got %v", descriptors[0].Type)
	}
}

func TestDescribeInstancesForScheduler_PMGOnly(t *testing.T) {
	m := &Monitor{
		pveClients: make(map[string]PVEClientInterface),
		pbsClients: make(map[string]*pbs.Client),
		pmgClients: map[string]*pmg.Client{"pmg-mail": nil},
	}

	descriptors := m.describeInstancesForScheduler()
	if len(descriptors) != 1 {
		t.Fatalf("expected 1 descriptor, got %d", len(descriptors))
	}

	if descriptors[0].Name != "pmg-mail" {
		t.Errorf("expected name 'pmg-mail', got %q", descriptors[0].Name)
	}
	if descriptors[0].Type != InstanceTypePMG {
		t.Errorf("expected type PMG, got %v", descriptors[0].Type)
	}
}

func TestDescribeInstancesForScheduler_AllTypes(t *testing.T) {
	m := &Monitor{
		pveClients: map[string]PVEClientInterface{"pve-1": nil},
		pbsClients: map[string]*pbs.Client{"pbs-1": nil},
		pmgClients: map[string]*pmg.Client{"pmg-1": nil},
	}

	descriptors := m.describeInstancesForScheduler()
	if len(descriptors) != 3 {
		t.Fatalf("expected 3 descriptors, got %d", len(descriptors))
	}

	// Check we have one of each type
	types := make(map[InstanceType]bool)
	for _, desc := range descriptors {
		types[desc.Type] = true
	}
	if !types[InstanceTypePVE] || !types[InstanceTypePBS] || !types[InstanceTypePMG] {
		t.Error("expected one descriptor of each type")
	}
}

func TestDescribeInstancesForScheduler_NilSchedulerAndTracker(t *testing.T) {
	m := &Monitor{
		pveClients:       map[string]PVEClientInterface{"pve-1": nil},
		pbsClients:       make(map[string]*pbs.Client),
		pmgClients:       make(map[string]*pmg.Client),
		scheduler:        nil, // explicitly nil
		stalenessTracker: nil, // explicitly nil
	}

	// Should not panic with nil scheduler and stalenessTracker
	descriptors := m.describeInstancesForScheduler()
	if len(descriptors) != 1 {
		t.Fatalf("expected 1 descriptor, got %d", len(descriptors))
	}

	// LastScheduled and LastSuccess should be zero values
	if !descriptors[0].LastScheduled.IsZero() {
		t.Error("expected LastScheduled to be zero with nil scheduler")
	}
	if !descriptors[0].LastSuccess.IsZero() {
		t.Error("expected LastSuccess to be zero with nil stalenessTracker")
	}
}

func TestRescheduleTask_NilTaskQueue(t *testing.T) {
	m := &Monitor{
		taskQueue: nil, // nil queue
	}

	task := ScheduledTask{
		InstanceName: "pve-1",
		InstanceType: InstanceTypePVE,
		Interval:     30 * time.Second,
		NextRun:      time.Now(),
	}

	// Should not panic with nil taskQueue, just returns early
	m.rescheduleTask(task)
}

func TestRescheduleTask_SuccessfulOutcome(t *testing.T) {
	cfg := &config.Config{
		PVEPollingInterval:          30 * time.Second,
		AdaptivePollingBaseInterval: 10 * time.Second,
	}

	m := &Monitor{
		config:        cfg,
		taskQueue:     NewTaskQueue(),
		lastOutcome:   make(map[string]taskOutcome),
		failureCounts: make(map[string]int),
	}

	task := ScheduledTask{
		InstanceName: "pve-1",
		InstanceType: InstanceTypePVE,
		Interval:     30 * time.Second,
		NextRun:      time.Now(),
	}

	key := schedulerKey(task.InstanceType, task.InstanceName)

	// Record a successful outcome
	m.lastOutcome[key] = taskOutcome{success: true}

	m.rescheduleTask(task)

	// Task should be rescheduled at regular interval (no backoff)
	m.taskQueue.mu.Lock()
	entry, ok := m.taskQueue.entries[key]
	m.taskQueue.mu.Unlock()

	if !ok {
		t.Fatal("expected task to be rescheduled")
	}

	// Should use base interval since scheduler is nil
	if entry.task.Interval != cfg.PVEPollingInterval {
		t.Errorf("expected interval %v, got %v", cfg.PVEPollingInterval, entry.task.Interval)
	}
}

func TestRescheduleTask_TransientFailureWithBackoff(t *testing.T) {
	cfg := &config.Config{
		PVEPollingInterval:          30 * time.Second,
		AdaptivePollingBaseInterval: 10 * time.Second,
	}

	m := &Monitor{
		config:           cfg,
		taskQueue:        NewTaskQueue(),
		lastOutcome:      make(map[string]taskOutcome),
		failureCounts:    make(map[string]int),
		maxRetryAttempts: 5,
		backoffCfg: backoffConfig{
			Initial:    5 * time.Second,
			Multiplier: 2,
			Jitter:     0, // no jitter for predictable testing
			Max:        5 * time.Minute,
		},
	}

	// Add randomFloat method for backoff calculation
	m.rng = nil // will use default random

	task := ScheduledTask{
		InstanceName: "pve-1",
		InstanceType: InstanceTypePVE,
		Interval:     30 * time.Second,
		NextRun:      time.Now(),
	}

	key := schedulerKey(task.InstanceType, task.InstanceName)

	// Record a transient failure (1st attempt, below maxRetryAttempts)
	m.failureCounts[key] = 1
	m.lastOutcome[key] = taskOutcome{
		success:   false,
		transient: true,
		err:       errors.New("connection timeout"),
	}

	m.rescheduleTask(task)

	// Task should be rescheduled with backoff delay
	m.taskQueue.mu.Lock()
	entry, ok := m.taskQueue.entries[key]
	m.taskQueue.mu.Unlock()

	if !ok {
		t.Fatal("expected task to be rescheduled with backoff")
	}

	// With backoff, interval should be modified
	if entry.task.Interval <= 0 {
		t.Errorf("expected positive backoff interval, got %v", entry.task.Interval)
	}
}

func TestRescheduleTask_NonTransientFailureGoesToDeadLetter(t *testing.T) {
	cfg := &config.Config{
		PVEPollingInterval: 30 * time.Second,
	}

	deadLetterQ := NewTaskQueue()

	m := &Monitor{
		config:           cfg,
		taskQueue:        NewTaskQueue(),
		deadLetterQueue:  deadLetterQ,
		lastOutcome:      make(map[string]taskOutcome),
		failureCounts:    make(map[string]int),
		maxRetryAttempts: 5,
	}

	task := ScheduledTask{
		InstanceName: "pve-1",
		InstanceType: InstanceTypePVE,
		Interval:     30 * time.Second,
		NextRun:      time.Now(),
	}

	key := schedulerKey(task.InstanceType, task.InstanceName)

	// Record a non-transient failure (permanent error)
	m.failureCounts[key] = 1
	m.lastOutcome[key] = taskOutcome{
		success:   false,
		transient: false, // non-transient
		err:       errors.New("authentication failed"),
	}

	m.rescheduleTask(task)

	// Task should NOT be in the main queue
	m.taskQueue.mu.Lock()
	_, inMainQueue := m.taskQueue.entries[key]
	m.taskQueue.mu.Unlock()

	if inMainQueue {
		t.Error("expected task to NOT be in main queue after non-transient failure")
	}

	// Task should be in dead letter queue
	deadLetterQ.mu.Lock()
	dlqSize := len(deadLetterQ.entries)
	deadLetterQ.mu.Unlock()

	if dlqSize != 1 {
		t.Errorf("expected 1 task in dead letter queue, got %d", dlqSize)
	}
}

func TestRescheduleTask_ExceededRetryAttemptsGoesToDeadLetter(t *testing.T) {
	cfg := &config.Config{
		PVEPollingInterval: 30 * time.Second,
	}

	deadLetterQ := NewTaskQueue()

	m := &Monitor{
		config:           cfg,
		taskQueue:        NewTaskQueue(),
		deadLetterQueue:  deadLetterQ,
		lastOutcome:      make(map[string]taskOutcome),
		failureCounts:    make(map[string]int),
		maxRetryAttempts: 3,
	}

	task := ScheduledTask{
		InstanceName: "pve-1",
		InstanceType: InstanceTypePVE,
		Interval:     30 * time.Second,
		NextRun:      time.Now(),
	}

	key := schedulerKey(task.InstanceType, task.InstanceName)

	// Exceed max retry attempts (failureCount >= maxRetryAttempts)
	m.failureCounts[key] = 3
	m.lastOutcome[key] = taskOutcome{
		success:   false,
		transient: true, // transient, but exceeded retries
		err:       errors.New("connection timeout"),
	}

	m.rescheduleTask(task)

	// Task should be in dead letter queue
	deadLetterQ.mu.Lock()
	dlqSize := len(deadLetterQ.entries)
	deadLetterQ.mu.Unlock()

	if dlqSize != 1 {
		t.Errorf("expected 1 task in dead letter queue after exceeding retries, got %d", dlqSize)
	}
}

func TestRescheduleTask_NoOutcomeUsesDefaultInterval(t *testing.T) {
	cfg := &config.Config{
		PVEPollingInterval:          45 * time.Second,
		AdaptivePollingBaseInterval: 10 * time.Second,
	}

	m := &Monitor{
		config:        cfg,
		taskQueue:     NewTaskQueue(),
		lastOutcome:   make(map[string]taskOutcome),
		failureCounts: make(map[string]int),
	}

	task := ScheduledTask{
		InstanceName: "pve-1",
		InstanceType: InstanceTypePVE,
		Interval:     0, // no interval set
		NextRun:      time.Now(),
	}

	key := schedulerKey(task.InstanceType, task.InstanceName)

	// No outcome recorded - hasOutcome will be false
	m.rescheduleTask(task)

	m.taskQueue.mu.Lock()
	entry, ok := m.taskQueue.entries[key]
	m.taskQueue.mu.Unlock()

	if !ok {
		t.Fatal("expected task to be rescheduled")
	}

	// Should use config PVE polling interval
	if entry.task.Interval != cfg.PVEPollingInterval {
		t.Errorf("expected interval %v, got %v", cfg.PVEPollingInterval, entry.task.Interval)
	}
}

func TestRescheduleTask_PBSInstance(t *testing.T) {
	cfg := &config.Config{
		PBSPollingInterval:          60 * time.Second,
		AdaptivePollingBaseInterval: 10 * time.Second,
	}

	m := &Monitor{
		config:        cfg,
		taskQueue:     NewTaskQueue(),
		lastOutcome:   make(map[string]taskOutcome),
		failureCounts: make(map[string]int),
	}

	task := ScheduledTask{
		InstanceName: "pbs-1",
		InstanceType: InstanceTypePBS,
		Interval:     0,
		NextRun:      time.Now(),
	}

	key := schedulerKey(task.InstanceType, task.InstanceName)

	m.rescheduleTask(task)

	m.taskQueue.mu.Lock()
	entry, ok := m.taskQueue.entries[key]
	m.taskQueue.mu.Unlock()

	if !ok {
		t.Fatal("expected PBS task to be rescheduled")
	}

	if entry.task.Interval != cfg.PBSPollingInterval {
		t.Errorf("expected PBS interval %v, got %v", cfg.PBSPollingInterval, entry.task.Interval)
	}
}

func TestRescheduleTask_PMGInstance(t *testing.T) {
	cfg := &config.Config{
		PMGPollingInterval:          90 * time.Second,
		AdaptivePollingBaseInterval: 10 * time.Second,
	}

	m := &Monitor{
		config:        cfg,
		taskQueue:     NewTaskQueue(),
		lastOutcome:   make(map[string]taskOutcome),
		failureCounts: make(map[string]int),
	}

	task := ScheduledTask{
		InstanceName: "pmg-1",
		InstanceType: InstanceTypePMG,
		Interval:     0,
		NextRun:      time.Now(),
	}

	key := schedulerKey(task.InstanceType, task.InstanceName)

	m.rescheduleTask(task)

	m.taskQueue.mu.Lock()
	entry, ok := m.taskQueue.entries[key]
	m.taskQueue.mu.Unlock()

	if !ok {
		t.Fatal("expected PMG task to be rescheduled")
	}

	if entry.task.Interval != cfg.PMGPollingInterval {
		t.Errorf("expected PMG interval %v, got %v", cfg.PMGPollingInterval, entry.task.Interval)
	}
}

func TestRescheduleTask_AdaptivePollingMaxIntervalLimit(t *testing.T) {
	cfg := &config.Config{
		PVEPollingInterval:          30 * time.Second,
		AdaptivePollingEnabled:      true,
		AdaptivePollingMaxInterval:  10 * time.Second, // <= 15s enables capping
		AdaptivePollingBaseInterval: 5 * time.Second,
	}

	m := &Monitor{
		config:           cfg,
		taskQueue:        NewTaskQueue(),
		lastOutcome:      make(map[string]taskOutcome),
		failureCounts:    make(map[string]int),
		maxRetryAttempts: 5,
		backoffCfg: backoffConfig{
			Initial:    10 * time.Second, // would normally backoff to 10s+
			Multiplier: 2,
			Jitter:     0,
			Max:        5 * time.Minute,
		},
	}

	task := ScheduledTask{
		InstanceName: "pve-1",
		InstanceType: InstanceTypePVE,
		Interval:     30 * time.Second,
		NextRun:      time.Now(),
	}

	key := schedulerKey(task.InstanceType, task.InstanceName)

	// Simulate transient failure to trigger backoff
	m.failureCounts[key] = 1
	m.lastOutcome[key] = taskOutcome{
		success:   false,
		transient: true,
		err:       errors.New("timeout"),
	}

	m.rescheduleTask(task)

	m.taskQueue.mu.Lock()
	entry, ok := m.taskQueue.entries[key]
	m.taskQueue.mu.Unlock()

	if !ok {
		t.Fatal("expected task to be rescheduled")
	}

	// With AdaptivePollingMaxInterval <= 15s, backoff delay should be capped at 4s
	maxDelay := 4 * time.Second
	if entry.task.Interval > maxDelay {
		t.Errorf("expected backoff interval to be capped at %v, got %v", maxDelay, entry.task.Interval)
	}
}

func TestRescheduleTask_UsesExistingIntervalWhenSet(t *testing.T) {
	cfg := &config.Config{
		PVEPollingInterval:          30 * time.Second,
		AdaptivePollingBaseInterval: 10 * time.Second,
	}

	m := &Monitor{
		config:        cfg,
		taskQueue:     NewTaskQueue(),
		lastOutcome:   make(map[string]taskOutcome),
		failureCounts: make(map[string]int),
	}

	customInterval := 45 * time.Second
	task := ScheduledTask{
		InstanceName: "pve-1",
		InstanceType: InstanceTypePVE,
		Interval:     customInterval, // custom interval already set
		NextRun:      time.Now(),
	}

	key := schedulerKey(task.InstanceType, task.InstanceName)

	m.rescheduleTask(task)

	m.taskQueue.mu.Lock()
	entry, ok := m.taskQueue.entries[key]
	m.taskQueue.mu.Unlock()

	if !ok {
		t.Fatal("expected task to be rescheduled")
	}

	// Should use the existing interval when it's already set
	if entry.task.Interval != customInterval {
		t.Errorf("expected existing interval %v to be preserved, got %v", customInterval, entry.task.Interval)
	}
}
