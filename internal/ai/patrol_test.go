package ai

import (
	"testing"
	"time"
)


func TestDefaultPatrolThresholds(t *testing.T) {
	thresholds := DefaultPatrolThresholds()

	// Verify defaults are set
	if thresholds.NodeCPUWatch != 75 {
		t.Errorf("Expected NodeCPUWatch 75, got %f", thresholds.NodeCPUWatch)
	}
	if thresholds.NodeCPUWarning != 85 {
		t.Errorf("Expected NodeCPUWarning 85, got %f", thresholds.NodeCPUWarning)
	}
	if thresholds.StorageWatch != 70 {
		t.Errorf("Expected StorageWatch 70, got %f", thresholds.StorageWatch)
	}
}

func TestCalculatePatrolThresholds_NilProvider(t *testing.T) {
	thresholds := CalculatePatrolThresholds(nil)

	// Should return defaults when provider is nil
	defaults := DefaultPatrolThresholds()
	if thresholds.NodeCPUWatch != defaults.NodeCPUWatch {
		t.Errorf("Expected defaults when provider is nil")
	}
}

func TestCalculatePatrolThresholds_FromProvider(t *testing.T) {
	provider := &mockThresholdProvider{
		nodeCPU:    90,
		nodeMemory: 85,
		guestMem:   80,
		guestDisk:  75,
		storage:    70,
	}

	thresholds := CalculatePatrolThresholds(provider)

	// Watch thresholds should be alertThreshold - 15
	expectedNodeCPUWatch := 90 - 15
	if thresholds.NodeCPUWatch != float64(expectedNodeCPUWatch) {
		t.Errorf("Expected NodeCPUWatch %d, got %f", expectedNodeCPUWatch, thresholds.NodeCPUWatch)
	}

	// Warning thresholds should be alertThreshold - 5
	expectedNodeCPUWarning := 90 - 5
	if thresholds.NodeCPUWarning != float64(expectedNodeCPUWarning) {
		t.Errorf("Expected NodeCPUWarning %d, got %f", expectedNodeCPUWarning, thresholds.NodeCPUWarning)
	}
}

func TestClampThreshold(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{50, 50},   // Normal value passes through
		{5, 10},    // Below minimum, clamped to 10
		{-5, 10},   // Negative, clamped to 10
		{100, 99},  // Above maximum, clamped to 99
		{150, 99},  // Way above, clamped to 99
		{10, 10},   // Exactly at minimum
		{99, 99},   // Exactly at maximum
	}

	for _, tt := range tests {
		result := clampThreshold(tt.input)
		if result != tt.expected {
			t.Errorf("clampThreshold(%f) = %f, want %f", tt.input, result, tt.expected)
		}
	}
}

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

	// Verify thresholds were calculated
	ps.mu.RLock()
	thresholds := ps.thresholds
	ps.mu.RUnlock()

	// Watch = alert - 15
	expectedWatch := 95.0 - 15.0
	if thresholds.NodeCPUWatch != expectedWatch {
		t.Errorf("Expected NodeCPUWatch %f, got %f", expectedWatch, thresholds.NodeCPUWatch)
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

	output, phase := ps.GetCurrentStreamOutput()
	if phase != "idle" {
		t.Errorf("Expected phase 'idle', got '%s'", phase)
	}
	if output != "" {
		t.Errorf("Expected empty output after reset to idle, got '%s'", output)
	}
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
	if record.ResourcesChecked != 10 {
		t.Errorf("Expected 10 resources checked, got %d", record.ResourcesChecked)
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
	if status.FindingsCount != 3 {
		t.Errorf("Expected 3 findings, got %d", status.FindingsCount)
	}
	if status.LastPatrolAt == nil {
		t.Error("Expected LastPatrolAt to be set")
	}
	if status.IntervalMs != 900000 {
		t.Errorf("Expected interval 900000ms, got %d", status.IntervalMs)
	}
}

func TestFormatDurationPatrol(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{30 * time.Minute, "30m"},
		{59 * time.Minute, "59m"},
		{60 * time.Minute, "1h"},
		{90 * time.Minute, "1h"},    // Less than 24h, shows hours
		{2 * time.Hour, "2h"},
		{23 * time.Hour, "23h"},
		{24 * time.Hour, "1d"},
		{48 * time.Hour, "2d"},
		{7 * 24 * time.Hour, "7d"},
	}

	for _, tt := range tests {
		result := formatDurationPatrol(tt.input)
		if result != tt.expected {
			t.Errorf("formatDurationPatrol(%v) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tt := range tests {
		result := formatBytes(tt.input)
		if result != tt.expected {
			t.Errorf("formatBytes(%d) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestFormatBytesInt64(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{-100, "0 B"},     // Negative values return "0 B"
		{0, "0 B"},
		{1024, "1.0 KB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		result := formatBytesInt64(tt.input)
		if result != tt.expected {
			t.Errorf("formatBytesInt64(%d) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestPatrolService_ParseAIFindings(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Test with valid findings
	response := `Here's my analysis:

[FINDING]
SEVERITY: warning
CATEGORY: performance
RESOURCE: vm-100
RESOURCE_TYPE: vm
TITLE: High CPU usage
DESCRIPTION: VM is running at 95% CPU for extended period
RECOMMENDATION: Consider adding more vCPUs
EVIDENCE: CPU: 95%
[/FINDING]

[FINDING]
SEVERITY: critical
CATEGORY: reliability
RESOURCE: node-1
RESOURCE_TYPE: node
TITLE: Node offline
DESCRIPTION: Node is not responding to health checks
RECOMMENDATION: Check network connectivity
EVIDENCE: Status: offline
[/FINDING]

Everything else looks good.`

	findings := ps.parseAIFindings(response)

	if len(findings) != 2 {
		t.Errorf("Expected 2 findings, got %d", len(findings))
	}

	if len(findings) >= 1 {
		if findings[0].Title != "High CPU usage" {
			t.Errorf("Expected title 'High CPU usage', got '%s'", findings[0].Title)
		}
		if findings[0].Severity != FindingSeverityWarning {
			t.Errorf("Expected severity warning, got %v", findings[0].Severity)
		}
	}

	if len(findings) >= 2 {
		if findings[1].Title != "Node offline" {
			t.Errorf("Expected title 'Node offline', got '%s'", findings[1].Title)
		}
		if findings[1].Severity != FindingSeverityCritical {
			t.Errorf("Expected severity critical, got %v", findings[1].Severity)
		}
	}
}

func TestPatrolService_ParseAIFindings_NoFindings(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	response := `Everything looks healthy. No issues detected.`

	findings := ps.parseAIFindings(response)

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings, got %d", len(findings))
	}
}

func TestPatrolService_ParseFindingBlock(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	block := `
SEVERITY: warning
CATEGORY: capacity
RESOURCE: storage-1
RESOURCE_TYPE: storage
TITLE: Storage filling up
DESCRIPTION: Storage is at 90% capacity
RECOMMENDATION: Clean up old backups
EVIDENCE: Usage: 90%
`

	finding := ps.parseFindingBlock(block)

	if finding == nil {
		t.Fatal("Expected non-nil finding")
	}
	if finding.Severity != FindingSeverityWarning {
		t.Errorf("Expected severity warning, got %v", finding.Severity)
	}
	if finding.Category != FindingCategoryCapacity {
		t.Errorf("Expected category capacity, got %v", finding.Category)
	}
	if finding.Title != "Storage filling up" {
		t.Errorf("Expected title 'Storage filling up', got '%s'", finding.Title)
	}
	if finding.ResourceID != "storage-1" {
		t.Errorf("Expected resource 'storage-1', got '%s'", finding.ResourceID)
	}
}

func TestPatrolService_ParseFindingBlock_MissingRequiredFields(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Missing title and description
	block := `
SEVERITY: warning
CATEGORY: capacity
RESOURCE: storage-1
`

	finding := ps.parseFindingBlock(block)

	if finding != nil {
		t.Error("Expected nil finding when required fields are missing")
	}
}

func TestPatrolService_ParseFindingBlock_AllSeverities(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	tests := []struct {
		severity string
		expected FindingSeverity
	}{
		{"critical", FindingSeverityCritical},
		{"warning", FindingSeverityWarning},
		{"watch", FindingSeverityWatch},
		{"info", FindingSeverityInfo},
		{"unknown", FindingSeverityInfo}, // Unknown defaults to info
	}

	for _, tt := range tests {
		block := "SEVERITY: " + tt.severity + "\nTITLE: Test\nDESCRIPTION: Test description"
		finding := ps.parseFindingBlock(block)
		if finding == nil {
			t.Fatalf("Expected finding for severity %s", tt.severity)
		}
		if finding.Severity != tt.expected {
			t.Errorf("Severity %s: expected %v, got %v", tt.severity, tt.expected, finding.Severity)
		}
	}
}

func TestPatrolService_ParseFindingBlock_AllCategories(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	tests := []struct {
		category string
		expected FindingCategory
	}{
		{"performance", FindingCategoryPerformance},
		{"reliability", FindingCategoryReliability},
		{"security", FindingCategorySecurity},
		{"capacity", FindingCategoryCapacity},
		{"configuration", FindingCategoryGeneral},
		{"unknown", FindingCategoryPerformance}, // Unknown defaults to performance
	}

	for _, tt := range tests {
		block := "CATEGORY: " + tt.category + "\nTITLE: Test\nDESCRIPTION: Test description"
		finding := ps.parseFindingBlock(block)
		if finding == nil {
			t.Fatalf("Expected finding for category %s", tt.category)
		}
		if finding.Category != tt.expected {
			t.Errorf("Category %s: expected %v, got %v", tt.category, tt.expected, finding.Category)
		}
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

func TestPatrolService_SetMetricsHistoryProvider(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Set a nil provider (should not panic)
	ps.SetMetricsHistoryProvider(nil)

	// Verify it was set (field is internal, just checking no panic)
}

func TestJoinParts(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{[]string{}, ""},
		{[]string{"one"}, "one"},
		{[]string{"one", "two"}, "one and two"},
		{[]string{"one", "two", "three"}, "[one two], and three"},
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

func TestPatrolService_SetKnowledgeStore(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// Setting nil knowledge store should not panic
	ps.SetKnowledgeStore(nil)

	// Verify it was set (field is internal, just checking no panic)
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
