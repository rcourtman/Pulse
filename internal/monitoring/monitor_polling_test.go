package monitoring

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
)

type testPollProvider struct {
	providerType      InstanceType
	instances         []string
	describeInstances []PollProviderInstanceInfo
	connectionStatus  map[string]bool
	connectionKey     string
	interval          time.Duration
	buildPollTask     func(instanceName string) (PollTask, error)
}

func (p testPollProvider) Type() InstanceType { return p.providerType }

func (p testPollProvider) ListInstances(_ *Monitor) []string {
	out := make([]string, len(p.instances))
	copy(out, p.instances)
	return out
}

func (p testPollProvider) DescribeInstances(_ *Monitor) []PollProviderInstanceInfo {
	out := make([]PollProviderInstanceInfo, len(p.describeInstances))
	for i := range p.describeInstances {
		out[i] = PollProviderInstanceInfo{
			Name:        p.describeInstances[i].Name,
			DisplayName: p.describeInstances[i].DisplayName,
			Connection:  p.describeInstances[i].Connection,
			Metadata:    cloneProviderMetadata(p.describeInstances[i].Metadata),
		}
	}
	return out
}

func (p testPollProvider) ConnectionStatuses(_ *Monitor) map[string]bool {
	if len(p.connectionStatus) == 0 {
		return nil
	}
	out := make(map[string]bool, len(p.connectionStatus))
	for key, healthy := range p.connectionStatus {
		out[key] = healthy
	}
	return out
}

func (p testPollProvider) ConnectionHealthKey(_ *Monitor, instanceName string) string {
	if strings.TrimSpace(p.connectionKey) != "" {
		return strings.TrimSpace(p.connectionKey)
	}
	return ""
}

func (p testPollProvider) BaseInterval(_ *Monitor) time.Duration { return p.interval }

func (p testPollProvider) BuildPollTask(_ *Monitor, instanceName string) (PollTask, error) {
	if p.buildPollTask == nil {
		return PollTask{
			InstanceName: instanceName,
			InstanceType: string(p.providerType),
		}, nil
	}
	return p.buildPollTask(instanceName)
}

type testSupplementalPollProvider struct {
	testPollProvider
	source           unifiedresources.DataSource
	ownedSources     []unifiedresources.DataSource
	recordsByOrg     map[string][]unifiedresources.IngestRecord
	lastRequestedOrg string
}

func (p *testSupplementalPollProvider) SupplementalSource() unifiedresources.DataSource {
	return p.source
}

func (p *testSupplementalPollProvider) SupplementalRecords(_ *Monitor, orgID string) []unifiedresources.IngestRecord {
	p.lastRequestedOrg = orgID
	records := p.recordsByOrg[orgID]
	out := make([]unifiedresources.IngestRecord, len(records))
	copy(out, records)
	return out
}

func (p *testSupplementalPollProvider) SnapshotOwnedSources(_ *Monitor) []unifiedresources.DataSource {
	out := make([]unifiedresources.DataSource, len(p.ownedSources))
	copy(out, p.ownedSources)
	return out
}

type testMonitorSupplementalProvider struct {
	recordsByOrg     map[string][]unifiedresources.IngestRecord
	ownedSources     []unifiedresources.DataSource
	lastRequestedOrg string
}

func (p *testMonitorSupplementalProvider) SupplementalRecords(_ *Monitor, orgID string) []unifiedresources.IngestRecord {
	p.lastRequestedOrg = orgID
	records := p.recordsByOrg[orgID]
	out := make([]unifiedresources.IngestRecord, len(records))
	copy(out, records)
	return out
}

func (p *testMonitorSupplementalProvider) SnapshotOwnedSources() []unifiedresources.DataSource {
	out := make([]unifiedresources.DataSource, len(p.ownedSources))
	copy(out, p.ownedSources)
	return out
}

type testSupplementalResourceStore struct {
	snapshotCalls   int
	lastSnapshot    models.StateSnapshot
	recordsBySource map[unifiedresources.DataSource][]unifiedresources.IngestRecord
}

func (s *testSupplementalResourceStore) ShouldSkipAPIPolling(string) bool { return false }

func (s *testSupplementalResourceStore) GetPollingRecommendations() map[string]float64 { return nil }

func (s *testSupplementalResourceStore) GetAll() []unifiedresources.Resource { return nil }

func (s *testSupplementalResourceStore) PopulateFromSnapshot(snapshot models.StateSnapshot) {
	s.lastSnapshot = snapshot
	s.snapshotCalls++
}

func (s *testSupplementalResourceStore) PopulateSupplementalRecords(source unifiedresources.DataSource, records []unifiedresources.IngestRecord) {
	if s.recordsBySource == nil {
		s.recordsBySource = make(map[unifiedresources.DataSource][]unifiedresources.IngestRecord)
	}
	cloned := make([]unifiedresources.IngestRecord, len(records))
	copy(cloned, records)
	s.recordsBySource[source] = append(s.recordsBySource[source], cloned...)
}

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

func TestCustomPollProviderIntegration(t *testing.T) {
	const customType InstanceType = "xcp"
	const customName = "xcp-cluster-1"
	customInterval := 42 * time.Second

	var executed atomic.Bool

	monitor := &Monitor{
		taskQueue: NewTaskQueue(),
	}
	if err := monitor.RegisterPollProvider(testPollProvider{
		providerType: customType,
		instances:    []string{customName},
		interval:     customInterval,
		buildPollTask: func(instanceName string) (PollTask, error) {
			return PollTask{
				InstanceName: instanceName,
				InstanceType: string(customType),
				Run: func(context.Context) {
					executed.Store(true)
				},
			}, nil
		},
	}); err != nil {
		t.Fatalf("RegisterPollProvider failed: %v", err)
	}
	monitor.SetExecutor(nil) // restore default executor

	tasks := monitor.buildScheduledTasks(time.Now())
	if len(tasks) != 1 {
		t.Fatalf("expected 1 scheduled task for custom provider, got %d", len(tasks))
	}
	task := tasks[0]
	if task.InstanceType != customType {
		t.Fatalf("expected custom instance type %q, got %q", customType, task.InstanceType)
	}
	if task.InstanceName != customName {
		t.Fatalf("expected custom instance name %q, got %q", customName, task.InstanceName)
	}
	if task.Interval != customInterval {
		t.Fatalf("expected custom interval %v, got %v", customInterval, task.Interval)
	}

	monitor.executeScheduledTask(context.Background(), task)
	if !executed.Load() {
		t.Fatal("expected custom provider poll task callback to execute")
	}
}

func TestUpdateResourceStore_IngestsSupplementalRecords(t *testing.T) {
	store := &testSupplementalResourceStore{}
	provider := &testSupplementalPollProvider{
		testPollProvider: testPollProvider{
			providerType: InstanceType("xcp"),
			instances:    []string{"xcp-cluster-1"},
			interval:     30 * time.Second,
		},
		source: unifiedresources.DataSource("xcp"),
		recordsByOrg: map[string][]unifiedresources.IngestRecord{
			"default": {
				{
					SourceID: "xcp-host-1",
					Resource: unifiedresources.Resource{
						Type:     unifiedresources.ResourceTypeHost,
						Name:     "xcp-host-1",
						Status:   unifiedresources.StatusOnline,
						LastSeen: time.Now().UTC(),
					},
					Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"xcp-host-1"}},
				},
			},
		},
	}

	monitor := &Monitor{
		resourceStore: store,
	}
	if err := monitor.RegisterPollProvider(provider); err != nil {
		t.Fatalf("RegisterPollProvider failed: %v", err)
	}

	monitor.updateResourceStore(models.StateSnapshot{})

	if store.snapshotCalls != 1 {
		t.Fatalf("expected PopulateFromSnapshot to be called once, got %d", store.snapshotCalls)
	}
	if provider.lastRequestedOrg != "default" {
		t.Fatalf("expected default org lookup, got %q", provider.lastRequestedOrg)
	}
	records := store.recordsBySource[unifiedresources.DataSource("xcp")]
	if len(records) != 1 {
		t.Fatalf("expected 1 supplemental record ingested, got %d", len(records))
	}
	if records[0].SourceID != "xcp-host-1" {
		t.Fatalf("expected supplemental source ID xcp-host-1, got %q", records[0].SourceID)
	}
}

func TestUpdateResourceStore_SuppressesProviderOwnedSnapshotSources(t *testing.T) {
	store := &testSupplementalResourceStore{}
	provider := &testSupplementalPollProvider{
		testPollProvider: testPollProvider{
			providerType: InstanceType("xcp"),
			instances:    []string{"xcp-cluster-1"},
			interval:     30 * time.Second,
		},
		source:       unifiedresources.SourceProxmox,
		ownedSources: []unifiedresources.DataSource{unifiedresources.SourceProxmox},
		recordsByOrg: map[string][]unifiedresources.IngestRecord{
			"default": {
				{
					SourceID: "xcp-host-1",
					Resource: unifiedresources.Resource{
						Type:     unifiedresources.ResourceTypeHost,
						Name:     "xcp-host-1",
						Status:   unifiedresources.StatusOnline,
						LastSeen: time.Now().UTC(),
					},
					Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"xcp-host-1"}},
				},
			},
		},
	}

	monitor := &Monitor{
		resourceStore: store,
	}
	if err := monitor.RegisterPollProvider(provider); err != nil {
		t.Fatalf("RegisterPollProvider failed: %v", err)
	}

	monitor.updateResourceStore(models.StateSnapshot{
		Nodes:         []models.Node{{}},
		VMs:           []models.VM{{}},
		Containers:    []models.Container{{}},
		Storage:       []models.Storage{{}},
		PhysicalDisks: []models.PhysicalDisk{{}},
		CephClusters:  []models.CephCluster{{}},
		Hosts:         []models.Host{{}},
	})

	if store.snapshotCalls != 1 {
		t.Fatalf("expected PopulateFromSnapshot to be called once, got %d", store.snapshotCalls)
	}
	if len(store.lastSnapshot.Nodes) != 0 || len(store.lastSnapshot.VMs) != 0 || len(store.lastSnapshot.Containers) != 0 {
		t.Fatalf("expected proxmox compute slices to be suppressed before snapshot ingest")
	}
	if len(store.lastSnapshot.Storage) != 0 || len(store.lastSnapshot.PhysicalDisks) != 0 || len(store.lastSnapshot.CephClusters) != 0 {
		t.Fatalf("expected proxmox storage slices to be suppressed before snapshot ingest")
	}
	if len(store.lastSnapshot.Hosts) != 1 {
		t.Fatalf("expected host-agent slice to remain in snapshot ingest")
	}

	records := store.recordsBySource[unifiedresources.SourceProxmox]
	if len(records) != 1 {
		t.Fatalf("expected 1 provider-owned supplemental record, got %d", len(records))
	}
}

func TestUpdateResourceStore_IngestsRegisteredSupplementalProvider(t *testing.T) {
	store := &testSupplementalResourceStore{}
	provider := &testMonitorSupplementalProvider{
		recordsByOrg: map[string][]unifiedresources.IngestRecord{
			"default": {
				{
					SourceID: "tn-host-1",
					Resource: unifiedresources.Resource{
						Type:     unifiedresources.ResourceTypeHost,
						Name:     "tn-host-1",
						Status:   unifiedresources.StatusOnline,
						LastSeen: time.Now().UTC(),
					},
					Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"tn-host-1"}},
				},
			},
		},
	}

	monitor := &Monitor{
		resourceStore: store,
	}
	monitor.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, provider)
	monitor.updateResourceStore(models.StateSnapshot{})

	if store.snapshotCalls != 1 {
		t.Fatalf("expected PopulateFromSnapshot to be called once, got %d", store.snapshotCalls)
	}
	if provider.lastRequestedOrg != "default" {
		t.Fatalf("expected default org lookup, got %q", provider.lastRequestedOrg)
	}
	records := store.recordsBySource[unifiedresources.SourceTrueNAS]
	if len(records) != 1 {
		t.Fatalf("expected 1 supplemental record from direct provider, got %d", len(records))
	}
	if records[0].SourceID != "tn-host-1" {
		t.Fatalf("expected source ID tn-host-1, got %q", records[0].SourceID)
	}
}

func TestUpdateResourceStore_SuppressesSnapshotForRegisteredSupplementalOwnership(t *testing.T) {
	store := &testSupplementalResourceStore{}
	provider := &testMonitorSupplementalProvider{
		ownedSources: []unifiedresources.DataSource{unifiedresources.SourceProxmox},
		recordsByOrg: map[string][]unifiedresources.IngestRecord{
			"default": {
				{
					SourceID: "tn-host-1",
					Resource: unifiedresources.Resource{
						Type:     unifiedresources.ResourceTypeHost,
						Name:     "tn-host-1",
						Status:   unifiedresources.StatusOnline,
						LastSeen: time.Now().UTC(),
					},
					Identity: unifiedresources.ResourceIdentity{Hostnames: []string{"tn-host-1"}},
				},
			},
		},
	}

	monitor := &Monitor{
		resourceStore: store,
	}
	monitor.SetSupplementalRecordsProvider(unifiedresources.SourceTrueNAS, provider)
	monitor.updateResourceStore(models.StateSnapshot{
		Nodes:      []models.Node{{}},
		VMs:        []models.VM{{}},
		Containers: []models.Container{{}},
		Hosts:      []models.Host{{}},
	})

	if len(store.lastSnapshot.Nodes) != 0 || len(store.lastSnapshot.VMs) != 0 || len(store.lastSnapshot.Containers) != 0 {
		t.Fatalf("expected proxmox slices to be suppressed for direct provider ownership")
	}
	if len(store.lastSnapshot.Hosts) != 1 {
		t.Fatalf("expected non-owned host slice to remain")
	}
	records := store.recordsBySource[unifiedresources.SourceTrueNAS]
	if len(records) != 1 {
		t.Fatalf("expected 1 supplemental record from direct provider, got %d", len(records))
	}
}

func TestSchedulerHealth_UsesProviderInstanceDescriptions(t *testing.T) {
	const customType InstanceType = "xcp"
	monitor := &Monitor{
		config:            &config.Config{},
		instanceInfoCache: make(map[string]*instanceInfo),
	}

	if err := monitor.RegisterPollProvider(testPollProvider{
		providerType: customType,
		instances:    []string{"xcp-a"},
		describeInstances: []PollProviderInstanceInfo{
			{
				Name:        "xcp-a",
				DisplayName: "XCP Cluster A",
				Connection:  "https://xcp-a.example",
			},
		},
		interval: 30 * time.Second,
	}); err != nil {
		t.Fatalf("RegisterPollProvider failed: %v", err)
	}

	resp := monitor.SchedulerHealth()

	key := schedulerKey(customType, "xcp-a")
	for _, inst := range resp.Instances {
		if inst.Key != key {
			continue
		}
		if inst.DisplayName != "XCP Cluster A" {
			t.Fatalf("expected display name %q, got %q", "XCP Cluster A", inst.DisplayName)
		}
		if inst.Connection != "https://xcp-a.example" {
			t.Fatalf("expected connection %q, got %q", "https://xcp-a.example", inst.Connection)
		}
		return
	}

	t.Fatalf("expected instance %q in scheduler health", key)
}

func TestGetConnectionStatuses_CustomProviderStatuses(t *testing.T) {
	const customType InstanceType = "xcp"
	monitor := &Monitor{
		state: models.NewState(),
	}
	if err := monitor.RegisterPollProvider(testPollProvider{
		providerType: customType,
		instances:    []string{"xcp-a"},
		connectionStatus: map[string]bool{
			"xcp-xcp-a": true,
		},
	}); err != nil {
		t.Fatalf("RegisterPollProvider failed: %v", err)
	}

	statuses := monitor.GetConnectionStatuses()
	if connected, ok := statuses["xcp-xcp-a"]; !ok || !connected {
		t.Fatalf("expected xcp-xcp-a to be connected, got exists=%v value=%v", ok, connected)
	}
}

func TestGetConnectionStatuses_CustomProviderFallbackToState(t *testing.T) {
	const customType InstanceType = "xcp"
	monitor := &Monitor{
		state: models.NewState(),
	}
	monitor.state.SetConnectionHealth("xcp-xcp-a", true)

	if err := monitor.RegisterPollProvider(pollProviderAdapter{
		instanceType: customType,
		listInstances: func(*Monitor) []string {
			return []string{"xcp-a"}
		},
		baseInterval: func(*Monitor) time.Duration { return 30 * time.Second },
		buildPollTask: func(*Monitor, string) (PollTask, error) {
			return PollTask{}, nil
		},
	}); err != nil {
		t.Fatalf("RegisterPollProvider failed: %v", err)
	}

	statuses := monitor.GetConnectionStatuses()
	if connected, ok := statuses["xcp-xcp-a"]; !ok || !connected {
		t.Fatalf("expected xcp-xcp-a to be connected via fallback, got exists=%v value=%v", ok, connected)
	}
}

func TestGetConnectionStatuses_BuiltInPMGSupport(t *testing.T) {
	monitor := &Monitor{
		config: &config.Config{
			PMGInstances: []config.PMGInstance{
				{Name: "pmg-1"},
				{Name: "pmg-2"},
			},
		},
		state:      models.NewState(),
		pmgClients: map[string]*pmg.Client{"pmg-1": {}},
	}
	monitor.state.SetConnectionHealth("pmg-pmg-1", true)

	statuses := monitor.GetConnectionStatuses()
	if connected, ok := statuses["pmg-pmg-1"]; !ok || !connected {
		t.Fatalf("expected pmg-pmg-1 connected, got exists=%v value=%v", ok, connected)
	}
	if connected, ok := statuses["pmg-pmg-2"]; !ok || connected {
		t.Fatalf("expected pmg-pmg-2 disconnected, got exists=%v value=%v", ok, connected)
	}
}

func TestSetProviderConnectionHealth_UsesProviderConnectionKey(t *testing.T) {
	const customType InstanceType = "xcp"
	const instanceName = "xcp-a"
	const providerKey = "provider/xcp-a"

	monitor := &Monitor{
		state: models.NewState(),
	}
	if err := monitor.RegisterPollProvider(testPollProvider{
		providerType:  customType,
		instances:     []string{instanceName},
		connectionKey: providerKey,
	}); err != nil {
		t.Fatalf("RegisterPollProvider failed: %v", err)
	}

	monitor.setProviderConnectionHealth(customType, instanceName, true)

	if !monitor.state.ConnectionHealth[providerKey] {
		t.Fatalf("expected provider key %q to be marked healthy", providerKey)
	}
	if _, exists := monitor.state.ConnectionHealth["xcp-"+instanceName]; exists {
		t.Fatalf("did not expect fallback key %q when provider key override is set", "xcp-"+instanceName)
	}
}
