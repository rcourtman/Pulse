package ai

import (
	"errors"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestPatrolConfig_GetInterval(t *testing.T) {
	tests := []struct {
		name     string
		config   PatrolConfig
		expected time.Duration
	}{
		{
			name:     "uses primary interval",
			config:   PatrolConfig{Interval: 30 * time.Minute},
			expected: 30 * time.Minute,
		},
		{
			name:     "falls back to quick check interval",
			config:   PatrolConfig{QuickCheckInterval: 20 * time.Minute},
			expected: 20 * time.Minute,
		},
		{
			name:     "defaults to 15 minutes",
			config:   PatrolConfig{},
			expected: 15 * time.Minute,
		},
		{
			name:     "primary interval takes precedence",
			config:   PatrolConfig{Interval: 45 * time.Minute, QuickCheckInterval: 10 * time.Minute},
			expected: 45 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetInterval()
			if result != tt.expected {
				t.Errorf("GetInterval() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDefaultPatrolConfig(t *testing.T) {
	cfg := DefaultPatrolConfig()

	if !cfg.Enabled {
		t.Error("Expected patrol to be enabled by default")
	}
	if cfg.Interval != 15*time.Minute {
		t.Errorf("Expected 15 minute default interval, got %v", cfg.Interval)
	}
	if !cfg.AnalyzeNodes {
		t.Error("Expected AnalyzeNodes to be true by default")
	}
	if !cfg.AnalyzeGuests {
		t.Error("Expected AnalyzeGuests to be true by default")
	}
	if !cfg.AnalyzeDocker {
		t.Error("Expected AnalyzeDocker to be true by default")
	}
	if !cfg.AnalyzeStorage {
		t.Error("Expected AnalyzeStorage to be true by default")
	}
	if !cfg.AnalyzePBS {
		t.Error("Expected AnalyzePBS to be true by default")
	}
	if !cfg.AnalyzeHosts {
		t.Error("Expected AnalyzeHosts to be true by default")
	}
	if !cfg.AnalyzeKubernetes {
		t.Error("Expected AnalyzeKubernetes to be true by default")
	}
}

func TestNewPatrolService(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	if ps == nil {
		t.Fatal("Expected non-nil patrol service")
	}

	// Should have initialized with defaults
	cfg := ps.GetConfig()
	if !cfg.Enabled {
		t.Error("Expected patrol to be enabled by default")
	}

	// Findings store should be initialized
	if ps.GetFindings() == nil {
		t.Error("Expected findings store to be initialized")
	}

	// Should not be running initially
	status := ps.GetStatus()
	if status.Running {
		t.Error("Expected patrol to not be running initially")
	}
}

func TestPatrolService_SetConfig(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	newConfig := PatrolConfig{
		Enabled:       false,
		Interval:      30 * time.Minute,
		AnalyzeNodes:  false,
		AnalyzeGuests: true,
	}

	ps.SetConfig(newConfig)
	cfg := ps.GetConfig()

	if cfg.Enabled != false {
		t.Error("Expected enabled to be false after SetConfig")
	}
	if cfg.Interval != 30*time.Minute {
		t.Errorf("Expected interval to be 30 minutes, got %v", cfg.Interval)
	}
	if cfg.AnalyzeNodes != false {
		t.Error("Expected AnalyzeNodes to be false")
	}
}

func TestPatrolService_SetThresholdProvider(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	provider := &mockThresholdProvider{
		nodeCPU:    95,
		nodeMemory: 90,
		guestMem:   85,
		guestDisk:  80,
		storage:    75,
	}

	ps.SetThresholdProvider(provider)

	// Verify thresholds were calculated (default: exact mode)
	ps.mu.RLock()
	thresholds := ps.thresholds
	ps.mu.RUnlock()

	// Watch = alert - 5 (slight buffer in exact mode)
	expectedWatch := 95.0 - 5.0
	if thresholds.NodeCPUWatch != expectedWatch {
		t.Errorf("Expected NodeCPUWatch %f, got %f", expectedWatch, thresholds.NodeCPUWatch)
	}

	// Warning = exact alert threshold (new default)
	expectedWarning := 95.0
	if thresholds.NodeCPUWarning != expectedWarning {
		t.Errorf("Expected NodeCPUWarning %f (exact threshold), got %f", expectedWarning, thresholds.NodeCPUWarning)
	}
}

func TestPatrolService_SetProactiveMode(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	provider := &mockThresholdProvider{
		nodeCPU:    95,
		nodeMemory: 90,
		guestMem:   85,
		guestDisk:  80,
		storage:    75,
	}

	ps.SetThresholdProvider(provider)

	// Enable proactive mode
	ps.SetProactiveMode(true)

	if !ps.GetProactiveMode() {
		t.Error("Expected proactive mode to be true")
	}

	// Verify thresholds were recalculated for proactive mode
	ps.mu.RLock()
	thresholds := ps.thresholds
	ps.mu.RUnlock()

	// Watch = alert - 15 in proactive mode
	expectedWatch := 95.0 - 15.0
	if thresholds.NodeCPUWatch != expectedWatch {
		t.Errorf("Expected NodeCPUWatch %f in proactive mode, got %f", expectedWatch, thresholds.NodeCPUWatch)
	}

	// Warning = alert - 5 in proactive mode
	expectedWarning := 95.0 - 5.0
	if thresholds.NodeCPUWarning != expectedWarning {
		t.Errorf("Expected NodeCPUWarning %f in proactive mode, got %f", expectedWarning, thresholds.NodeCPUWarning)
	}

	// Disable proactive mode
	ps.SetProactiveMode(false)

	if ps.GetProactiveMode() {
		t.Error("Expected proactive mode to be false")
	}

	// Verify thresholds were recalculated back to exact mode
	ps.mu.RLock()
	thresholds = ps.thresholds
	ps.mu.RUnlock()

	// Warning should be exact threshold again
	if thresholds.NodeCPUWarning != 95.0 {
		t.Errorf("Expected NodeCPUWarning 95 (exact threshold) after disabling proactive mode, got %f", thresholds.NodeCPUWarning)
	}
}

func TestPatrolService_GetStatus(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	status := ps.GetStatus()

	// Default status checks
	if status.Running {
		t.Error("Expected running to be false initially")
	}
	if !status.Enabled {
		t.Error("Expected enabled to be true by default")
	}
	if status.FindingsCount != 0 {
		t.Errorf("Expected 0 findings count, got %d", status.FindingsCount)
	}
	if !status.Healthy {
		t.Error("Expected healthy to be true with no findings")
	}
	if status.IntervalMs == 0 {
		t.Error("Expected non-zero interval")
	}
}

func TestPatrolService_filterStateByScope_DockerContainer(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				ID:       "host-1",
				Hostname: "docker-1",
				Containers: []models.DockerContainer{
					{ID: "c1", Name: "web"},
					{ID: "c2", Name: "db"},
				},
			},
		},
	}
	scope := PatrolScope{
		ResourceIDs:   []string{"c1"},
		ResourceTypes: []string{"docker_container"},
	}

	filtered := ps.filterStateByScope(state, scope)

	if len(filtered.DockerHosts) != 1 {
		t.Fatalf("expected 1 docker host, got %d", len(filtered.DockerHosts))
	}
	if len(filtered.DockerHosts[0].Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(filtered.DockerHosts[0].Containers))
	}
	if filtered.DockerHosts[0].Containers[0].ID != "c1" {
		t.Fatalf("expected container c1, got %s", filtered.DockerHosts[0].Containers[0].ID)
	}
}

func TestPatrolService_filterStateByScope_KubernetesClusterType(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{ID: "k1", Name: "cluster-1"},
		},
	}
	scope := PatrolScope{
		ResourceIDs:   []string{"k1"},
		ResourceTypes: []string{"kubernetes_cluster"},
	}

	filtered := ps.filterStateByScope(state, scope)

	if len(filtered.KubernetesClusters) != 1 {
		t.Fatalf("expected 1 kubernetes cluster, got %d", len(filtered.KubernetesClusters))
	}
	if filtered.KubernetesClusters[0].ID != "k1" {
		t.Fatalf("expected cluster k1, got %s", filtered.KubernetesClusters[0].ID)
	}
}

func TestPatrolService_filterStateByScope_PBSDatastoreType(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	state := models.StateSnapshot{
		PBSInstances: []models.PBSInstance{
			{
				ID:   "pbs1",
				Name: "pbs-main",
				Datastores: []models.PBSDatastore{
					{Name: "ds1"},
				},
			},
		},
	}
	scope := PatrolScope{
		ResourceIDs:   []string{"pbs1:ds1"},
		ResourceTypes: []string{"pbs_datastore"},
	}

	filtered := ps.filterStateByScope(state, scope)

	if len(filtered.PBSInstances) != 1 {
		t.Fatalf("expected 1 PBS instance, got %d", len(filtered.PBSInstances))
	}
	if filtered.PBSInstances[0].ID != "pbs1" {
		t.Fatalf("expected PBS instance pbs1, got %s", filtered.PBSInstances[0].ID)
	}
}

func TestPatrolService_GetStatus_WithFindings(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Add a warning finding
	finding := &Finding{
		ID:           "test-finding",
		Severity:     FindingSeverityWarning,
		ResourceID:   "test-resource",
		ResourceName: "test",
		Title:        "Test Warning",
	}
	ps.findings.Add(finding)

	status := ps.GetStatus()

	if status.FindingsCount != 1 {
		t.Errorf("Expected 1 finding, got %d", status.FindingsCount)
	}
	if status.Healthy {
		t.Error("Expected healthy to be false with warning finding")
	}
}

func TestPatrolService_StreamSubscription(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Subscribe
	ch := ps.SubscribeToStream()
	if ch == nil {
		t.Fatal("Expected non-nil channel")
	}

	// Verify it's tracked
	ps.streamMu.RLock()
	_, exists := ps.streamSubscribers[ch]
	ps.streamMu.RUnlock()

	if !exists {
		t.Error("Expected channel to be in subscribers")
	}

	// Unsubscribe
	ps.UnsubscribeFromStream(ch)

	ps.streamMu.RLock()
	_, stillExists := ps.streamSubscribers[ch]
	ps.streamMu.RUnlock()

	if stillExists {
		t.Error("Expected channel to be removed from subscribers")
	}
}

func TestPatrolService_Broadcast(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	ch := ps.SubscribeToStream()

	// Broadcast an event
	event := PatrolStreamEvent{
		Type:    "test",
		Content: "test content",
	}
	ps.broadcast(event)

	// Check for the event
	select {
	case received := <-ch:
		if received.Type != "test" {
			t.Errorf("Expected type 'test', got '%s'", received.Type)
		}
		if received.Content != "test content" {
			t.Errorf("Expected content 'test content', got '%s'", received.Content)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected to receive broadcast event")
	}

	ps.UnsubscribeFromStream(ch)
}

func TestPatrolService_SetStreamPhase(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Default phase
	ps.streamMu.RLock()
	initialPhase := ps.streamPhase
	ps.streamMu.RUnlock()

	if initialPhase != "idle" {
		t.Errorf("Expected initial phase 'idle', got '%s'", initialPhase)
	}

	// Change phase
	ps.setStreamPhase("analyzing")

	ps.streamMu.RLock()
	newPhase := ps.streamPhase
	ps.streamMu.RUnlock()

	if newPhase != "analyzing" {
		t.Errorf("Expected phase 'analyzing', got '%s'", newPhase)
	}

	// Reset to idle should clear output
	ps.streamMu.Lock()
	ps.currentOutput.WriteString("some content")
	ps.streamMu.Unlock()

	ps.setStreamPhase("idle")

	_, phase := ps.GetCurrentStreamOutput()
	if phase != "idle" {
		t.Errorf("Expected phase 'idle', got '%s'", phase)
	}
	// Output is no longer cleared on idle; it is cleared when a new run starts.
}

func TestPatrolService_GetCurrentStreamOutput(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	ps.setStreamPhase("analyzing")
	ps.appendStreamContent("test output 1")
	ps.appendStreamContent("test output 2")

	output, phase := ps.GetCurrentStreamOutput()

	if phase != "analyzing" {
		t.Errorf("Expected phase 'analyzing', got '%s'", phase)
	}
	if output != "test output 1test output 2" {
		t.Errorf("Expected concatenated output, got '%s'", output)
	}
}

func TestPatrolService_SetMemoryProviders(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Test SetChangeDetector
	changeDetector := &ChangeDetector{} // Would need proper initialization
	ps.mu.Lock()
	ps.changeDetector = changeDetector
	ps.mu.Unlock()

	if ps.GetChangeDetector() != changeDetector {
		t.Error("Expected change detector to be set")
	}

	// Test SetRemediationLog
	remLog := &RemediationLog{} // Would need proper initialization
	ps.mu.Lock()
	ps.remediationLog = remLog
	ps.mu.Unlock()

	if ps.GetRemediationLog() != remLog {
		t.Error("Expected remediation log to be set")
	}
}

func TestPatrolRunRecord(t *testing.T) {
	now := time.Now()
	record := PatrolRunRecord{
		ID:               "test-run-1",
		StartedAt:        now,
		CompletedAt:      now.Add(5 * time.Second),
		Duration:         5 * time.Second,
		Type:             "patrol",
		ResourcesChecked: 10,
		NodesChecked:     2,
		GuestsChecked:    5,
		NewFindings:      1,
		Status:           "issues_found",
	}

	if record.ID != "test-run-1" {
		t.Errorf("Expected ID 'test-run-1', got '%s'", record.ID)
	}
	if record.CompletedAt != now.Add(5*time.Second) {
		t.Error("Expected CompletedAt to match now + 5s")
	}
	if record.Duration != 5*time.Second {
		t.Errorf("Expected duration 5s, got %v", record.Duration)
	}
	if record.ResourcesChecked != 10 {
		t.Errorf("Expected 10 resources checked, got %d", record.ResourcesChecked)
	}
	if record.StartedAt != now {
		t.Error("Expected StartedAt to match now")
	}
	if record.Type != "patrol" {
		t.Errorf("Expected type 'patrol', got '%s'", record.Type)
	}
	if record.NodesChecked != 2 {
		t.Errorf("Expected 2 nodes checked, got %d", record.NodesChecked)
	}
	if record.GuestsChecked != 5 {
		t.Errorf("Expected 5 guests checked, got %d", record.GuestsChecked)
	}
	if record.NewFindings != 1 {
		t.Errorf("Expected 1 new finding, got %d", record.NewFindings)
	}
	if record.Status != "issues_found" {
		t.Errorf("Expected status 'issues_found', got '%s'", record.Status)
	}
}

func TestPatrolStatus_Fields(t *testing.T) {
	now := time.Now()
	next := now.Add(15 * time.Minute)

	status := PatrolStatus{
		Running:          true,
		Enabled:          true,
		LastPatrolAt:     &now,
		NextPatrolAt:     &next,
		LastDuration:     5 * time.Second,
		ResourcesChecked: 25,
		FindingsCount:    3,
		ErrorCount:       0,
		Healthy:          false,
		IntervalMs:       900000,
	}

	if !status.Running {
		t.Error("Expected running to be true")
	}
	if !status.Enabled {
		t.Error("Expected enabled to be true")
	}
	if status.FindingsCount != 3 {
		t.Errorf("Expected 3 findings, got %d", status.FindingsCount)
	}
	if status.LastPatrolAt == nil {
		t.Error("Expected LastPatrolAt to be set")
	}
	if *status.NextPatrolAt != next {
		t.Error("NextPatrolAt value mismatch")
	}
	if status.LastDuration != 5*time.Second {
		t.Errorf("Expected last duration 5s, got %v", status.LastDuration)
	}
	if status.ResourcesChecked != 25 {
		t.Errorf("Expected 25 resources checked, got %d", status.ResourcesChecked)
	}
	if status.FindingsCount != 3 {
		t.Errorf("Expected 3 findings, got %d", status.FindingsCount)
	}
	if status.ErrorCount != 0 {
		t.Errorf("Expected 0 errors, got %d", status.ErrorCount)
	}
	if status.Healthy {
		t.Error("Expected Healthy to be false")
	}
	if status.IntervalMs != 900000 {
		t.Errorf("Expected interval 900000ms, got %d", status.IntervalMs)
	}
}

func TestPatrolService_GetFindingsForResource(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Add findings for specific resources
	f1 := &Finding{
		ID:           "f1",
		ResourceID:   "res-1",
		ResourceName: "Resource 1",
		Severity:     FindingSeverityWarning,
		Title:        "Finding 1",
	}
	f2 := &Finding{
		ID:           "f2",
		ResourceID:   "res-1",
		ResourceName: "Resource 1",
		Severity:     FindingSeverityCritical,
		Title:        "Finding 2",
	}
	f3 := &Finding{
		ID:           "f3",
		ResourceID:   "res-2",
		ResourceName: "Resource 2",
		Severity:     FindingSeverityWarning,
		Title:        "Finding 3",
	}

	ps.findings.Add(f1)
	ps.findings.Add(f2)
	ps.findings.Add(f3)

	// Get findings for res-1
	res1Findings := ps.GetFindingsForResource("res-1")
	if len(res1Findings) != 2 {
		t.Errorf("Expected 2 findings for res-1, got %d", len(res1Findings))
	}

	// Get findings for res-2
	res2Findings := ps.GetFindingsForResource("res-2")
	if len(res2Findings) != 1 {
		t.Errorf("Expected 1 finding for res-2, got %d", len(res2Findings))
	}
}

func TestPatrolService_GetFindingsSummary(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Add findings
	ps.findings.Add(&Finding{ID: "f1", Severity: FindingSeverityCritical, Title: "Critical"})
	ps.findings.Add(&Finding{ID: "f2", Severity: FindingSeverityWarning, Title: "Warning"})
	ps.findings.Add(&Finding{ID: "f3", Severity: FindingSeverityWatch, Title: "Watch"})

	summary := ps.GetFindingsSummary()

	if summary.Critical != 1 {
		t.Errorf("Expected 1 critical, got %d", summary.Critical)
	}
	if summary.Warning != 1 {
		t.Errorf("Expected 1 warning, got %d", summary.Warning)
	}
	if summary.Watch != 1 {
		t.Errorf("Expected 1 watch, got %d", summary.Watch)
	}
}

func TestPatrolService_GetRunHistory(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Add some run records
	ps.runHistoryStore.Add(PatrolRunRecord{ID: "run-1", Status: "completed"})
	ps.runHistoryStore.Add(PatrolRunRecord{ID: "run-2", Status: "completed"})
	ps.runHistoryStore.Add(PatrolRunRecord{ID: "run-3", Status: "completed"})

	// Get all
	allRuns := ps.GetRunHistory(0)
	if len(allRuns) != 3 {
		t.Errorf("Expected 3 runs, got %d", len(allRuns))
	}

	// Get limited
	limitedRuns := ps.GetRunHistory(2)
	if len(limitedRuns) != 2 {
		t.Errorf("Expected 2 runs (limited), got %d", len(limitedRuns))
	}
}

func TestPatrolService_GetPatternDetector(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Initially nil
	if ps.GetPatternDetector() != nil {
		t.Error("Expected nil PatternDetector initially")
	}

	// Set pattern detector
	detector := NewPatternDetector(DefaultPatternConfig())
	ps.SetPatternDetector(detector)

	if ps.GetPatternDetector() != detector {
		t.Error("Expected GetPatternDetector to return the set detector")
	}
}

func TestPatrolService_GetCorrelationDetector(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Initially nil
	if ps.GetCorrelationDetector() != nil {
		t.Error("Expected nil CorrelationDetector initially")
	}

	// Set correlation detector
	detector := NewCorrelationDetector(DefaultCorrelationConfig())
	ps.SetCorrelationDetector(detector)

	if ps.GetCorrelationDetector() != detector {
		t.Error("Expected GetCorrelationDetector to return the set detector")
	}
}

func TestPatrolService_GetBaselineStore(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Initially nil
	if ps.GetBaselineStore() != nil {
		t.Error("Expected nil BaselineStore initially")
	}

	// Set baseline store
	store := NewBaselineStore(DefaultBaselineConfig())
	ps.SetBaselineStore(store)

	if ps.GetBaselineStore() != store {
		t.Error("Expected GetBaselineStore to return the set store")
	}
}

func TestJoinParts(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{[]string{}, ""},
		{[]string{"one"}, "one"},
		{[]string{"one", "two"}, "one and two"},
		{[]string{"one", "two", "three"}, "one, two, and three"},
	}

	for _, tt := range tests {
		result := joinParts(tt.input)
		if result != tt.expected {
			t.Errorf("joinParts(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestPatrolService_GetAllFindings(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Add findings with different severities
	// Note: GetAllFindings now filters out info/watch findings (only returns warning+)
	ps.findings.Add(&Finding{
		ID:         "f1",
		Severity:   FindingSeverityInfo,
		Title:      "Info finding",
		DetectedAt: time.Now().Add(-3 * time.Hour),
	})
	ps.findings.Add(&Finding{
		ID:         "f2",
		Severity:   FindingSeverityCritical,
		Title:      "Critical finding",
		DetectedAt: time.Now().Add(-1 * time.Hour),
	})
	ps.findings.Add(&Finding{
		ID:         "f3",
		Severity:   FindingSeverityWarning,
		Title:      "Warning finding",
		DetectedAt: time.Now().Add(-2 * time.Hour),
	})

	findings := ps.GetAllFindings()

	// GetAllFindings filters out info/watch - only returns critical and warning
	if len(findings) != 2 {
		t.Fatalf("Expected 2 findings (critical+warning only), got %d", len(findings))
	}

	// Should be sorted by severity (critical first)
	if findings[0].Severity != FindingSeverityCritical {
		t.Errorf("Expected first finding to be critical, got %s", findings[0].Severity)
	}
	if findings[1].Severity != FindingSeverityWarning {
		t.Errorf("Expected second finding to be warning, got %s", findings[1].Severity)
	}
}

func TestPatrolService_GetFindingsHistory(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	now := time.Now()

	// Add findings at different times
	ps.findings.Add(&Finding{
		ID:         "f1",
		Title:      "Old finding",
		DetectedAt: now.Add(-48 * time.Hour),
	})
	ps.findings.Add(&Finding{
		ID:         "f2",
		Title:      "Recent finding",
		DetectedAt: now.Add(-1 * time.Hour),
	})

	// Get all findings history
	allHistory := ps.GetFindingsHistory(nil)
	if len(allHistory) != 2 {
		t.Errorf("Expected 2 findings in history, got %d", len(allHistory))
	}

	// Should be sorted by detected time (newest first)
	if allHistory[0].ID != "f2" {
		t.Errorf("Expected newest finding first, got %s", allHistory[0].ID)
	}

	// Get filtered history (only last 24 hours)
	startTime := now.Add(-24 * time.Hour)
	filteredHistory := ps.GetFindingsHistory(&startTime)
	if len(filteredHistory) != 1 {
		t.Errorf("Expected 1 finding in filtered history, got %d", len(filteredHistory))
	}
}

func TestPatrolService_ResolveFinding_Errors(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Test empty ID
	err := ps.ResolveFinding("", "resolved")
	if err == nil {
		t.Error("Expected error for empty finding ID")
	}

	// Test non-existent finding
	err = ps.ResolveFinding("nonexistent", "resolved")
	if err == nil {
		t.Error("Expected error for non-existent finding")
	}
}

func TestPatrolService_SetFindingsPersistence_Nil(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Setting nil persistence should not error
	err := ps.SetFindingsPersistence(nil)
	if err != nil {
		t.Errorf("Expected no error with nil persistence, got: %v", err)
	}
}

func TestPatrolService_SetRunHistoryPersistence_Nil(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Setting nil persistence should not error
	err := ps.SetRunHistoryPersistence(nil)
	if err != nil {
		t.Errorf("Expected no error with nil persistence, got: %v", err)
	}
}

type mockFindingsPersistence struct {
	loadErr error
}

func (m *mockFindingsPersistence) SaveFindings(findings map[string]*Finding) error {
	return nil
}

func (m *mockFindingsPersistence) LoadFindings() (map[string]*Finding, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	return make(map[string]*Finding), nil
}

func TestPatrolService_SetFindingsPersistence(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	err := ps.SetFindingsPersistence(&mockFindingsPersistence{})
	if err != nil {
		t.Errorf("Expected no error with persistence, got: %v", err)
	}
}

func TestPatrolService_SetFindingsPersistence_Error(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	err := ps.SetFindingsPersistence(&mockFindingsPersistence{loadErr: errors.New("load failed")})
	if err == nil {
		t.Error("Expected error when persistence load fails")
	}
}

func TestPatrolService_SetRunHistoryPersistence(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	mockPersistence := &mockPatrolHistoryPersistence{
		runs: []PatrolRunRecord{{ID: "run-1"}},
	}

	err := ps.SetRunHistoryPersistence(mockPersistence)
	if err != nil {
		t.Errorf("Expected no error with persistence, got: %v", err)
	}
}

func TestPatrolService_SetRunHistoryPersistence_Error(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	mockPersistence := &mockPatrolHistoryPersistence{
		loadErr: errors.New("load failed"),
	}

	err := ps.SetRunHistoryPersistence(mockPersistence)
	if err == nil {
		t.Error("Expected error when run history persistence load fails")
	}
}

func TestPatrolService_IncidentStore(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	store := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	ps.SetIncidentStore(store)

	if got := ps.GetIncidentStore(); got != store {
		t.Errorf("Expected incident store to match")
	}
}

func TestPatrolService_GetThresholds(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	thresholds := ps.GetThresholds()
	if thresholds.StorageWarning == 0 {
		t.Errorf("Expected thresholds to be initialized")
	}
}

func TestPatrolService_GetIntelligence(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.stateProvider = &mockStateProvider{}

	intel := ps.GetIntelligence()
	if intel == nil {
		t.Fatal("Expected intelligence facade to be created")
	}
	if intel != ps.GetIntelligence() {
		t.Fatal("Expected GetIntelligence to return cached instance")
	}
}

func TestPatrolService_Stop(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Stop should no-op when not running
	ps.Stop()

	ps.running = true
	ps.stopCh = make(chan struct{})
	ps.Stop()

	ps.mu.RLock()
	running := ps.running
	ps.mu.RUnlock()
	if running {
		t.Error("Expected patrol service to be stopped")
	}

	select {
	case <-ps.stopCh:
	default:
		t.Error("Expected stop channel to be closed")
	}
}

// ========================================
// normalizeFindingKey tests
// ========================================

func TestNormalizeFindingKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: "",
		},
		{
			name:     "simple lowercase",
			input:    "high-cpu-usage",
			expected: "high-cpu-usage",
		},
		{
			name:     "uppercase to lowercase",
			input:    "High-CPU-Usage",
			expected: "high-cpu-usage",
		},
		{
			name:     "underscores to dashes",
			input:    "high_cpu_usage",
			expected: "high-cpu-usage",
		},
		{
			name:     "spaces to dashes",
			input:    "high cpu usage",
			expected: "high-cpu-usage",
		},
		{
			name:     "mixed separators",
			input:    "high_cpu usage-warning",
			expected: "high-cpu-usage-warning",
		},
		{
			name:     "special characters removed",
			input:    "cpu@100%!warning",
			expected: "cpu100warning",
		},
		{
			name:     "leading/trailing whitespace",
			input:    "  high-cpu  ",
			expected: "high-cpu",
		},
		{
			name:     "with numbers",
			input:    "vm-123-cpu-high",
			expected: "vm-123-cpu-high",
		},
		{
			name:     "leading/trailing dashes trimmed",
			input:    "-high-cpu-",
			expected: "high-cpu",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeFindingKey(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeFindingKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
