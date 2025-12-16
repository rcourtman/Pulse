package ai

import (
	"testing"
	"time"
)

// mockThresholdProvider implements ThresholdProvider for testing
type mockThresholdProvider struct {
	nodeCPU    float64
	nodeMemory float64
	guestMem   float64
	guestDisk  float64
	storage    float64
}

func (m *mockThresholdProvider) GetNodeCPUThreshold() float64    { return m.nodeCPU }
func (m *mockThresholdProvider) GetNodeMemoryThreshold() float64 { return m.nodeMemory }
func (m *mockThresholdProvider) GetGuestMemoryThreshold() float64 { return m.guestMem }
func (m *mockThresholdProvider) GetGuestDiskThreshold() float64   { return m.guestDisk }
func (m *mockThresholdProvider) GetStorageThreshold() float64     { return m.storage }

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
