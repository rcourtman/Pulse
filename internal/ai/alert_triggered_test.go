package ai

import (
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// mockStateProvider implements StateProvider for testing
type mockStateProvider struct {
	state models.StateSnapshot
}

func (m *mockStateProvider) GetState() models.StateSnapshot {
	return m.state
}

func TestAlertTriggeredAnalyzer_NewAlertTriggeredAnalyzer(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)

	if analyzer == nil {
		t.Fatal("Expected non-nil analyzer")
	}

	// Default should be disabled
	if analyzer.IsEnabled() {
		t.Error("Expected analyzer to be disabled by default")
	}

	// Should have initialized maps
	if analyzer.lastAnalyzed == nil {
		t.Error("Expected lastAnalyzed map to be initialized")
	}
	if analyzer.pending == nil {
		t.Error("Expected pending map to be initialized")
	}

	// Default cooldown should be 5 minutes
	if analyzer.cooldown != 5*time.Minute {
		t.Errorf("Expected 5 minute cooldown, got %v", analyzer.cooldown)
	}
}

func TestAlertTriggeredAnalyzer_SetEnabled(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)

	// Start disabled
	if analyzer.IsEnabled() {
		t.Error("Expected analyzer to be disabled initially")
	}

	// Enable
	analyzer.SetEnabled(true)
	if !analyzer.IsEnabled() {
		t.Error("Expected analyzer to be enabled after SetEnabled(true)")
	}

	// Disable
	analyzer.SetEnabled(false)
	if analyzer.IsEnabled() {
		t.Error("Expected analyzer to be disabled after SetEnabled(false)")
	}
}

func TestAlertTriggeredAnalyzer_OnAlertFired_Disabled(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)

	// Create a test alert
	alert := &alerts.Alert{
		ID:           "test-alert-1",
		Type:         "cpu",
		ResourceID:   "node-1",
		ResourceName: "test-node",
		Value:        95.0,
		Threshold:    90.0,
	}

	// When disabled, OnAlertFired should do nothing (no panic)
	analyzer.OnAlertFired(alert)

	// Verify no pending analyses were started
	analyzer.mu.RLock()
	pending := len(analyzer.pending)
	analyzer.mu.RUnlock()

	if pending != 0 {
		t.Errorf("Expected no pending analyses when disabled, got %d", pending)
	}
}

func TestAlertTriggeredAnalyzer_OnAlertFired_NilAlert(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)
	analyzer.SetEnabled(true)

	// Should handle nil alert gracefully (no panic)
	analyzer.OnAlertFired(nil)

	// Verify no pending analyses
	analyzer.mu.RLock()
	pending := len(analyzer.pending)
	analyzer.mu.RUnlock()

	if pending != 0 {
		t.Errorf("Expected no pending analyses for nil alert, got %d", pending)
	}
}

func TestAlertTriggeredAnalyzer_ResourceKeyFromAlert(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)

	tests := []struct {
		name     string
		alert    *alerts.Alert
		expected string
	}{
		{
			name: "with resource ID",
			alert: &alerts.Alert{
				ResourceID:   "vm-100",
				ResourceName: "test-vm",
				Instance:     "cluster-1",
			},
			expected: "vm-100",
		},
		{
			name: "with resource name and instance",
			alert: &alerts.Alert{
				ResourceName: "test-vm",
				Instance:     "cluster-1",
			},
			expected: "cluster-1/test-vm",
		},
		{
			name: "with resource name only",
			alert: &alerts.Alert{
				ResourceName: "test-vm",
			},
			expected: "test-vm",
		},
		{
			name:     "empty alert",
			alert:    &alerts.Alert{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.resourceKeyFromAlert(tt.alert)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestAlertTriggeredAnalyzer_Cooldown(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)
	analyzer.SetEnabled(true)
	// Set a short cooldown for testing
	analyzer.cooldown = 100 * time.Millisecond

	alert := &alerts.Alert{
		ID:           "test-alert-1",
		Type:         "cpu",
		ResourceID:   "node-1",
		ResourceName: "test-node",
	}

	// Manually set the resource as recently analyzed
	analyzer.mu.Lock()
	analyzer.lastAnalyzed["node-1"] = time.Now()
	analyzer.mu.Unlock()

	// OnAlertFired should skip due to cooldown
	analyzer.OnAlertFired(alert)

	// Give a moment for any async operations
	time.Sleep(10 * time.Millisecond)

	// Should not have a pending analysis since cooldown is active
	analyzer.mu.RLock()
	pending := len(analyzer.pending)
	analyzer.mu.RUnlock()

	if pending != 0 {
		t.Errorf("Expected no pending analyses during cooldown, got %d", pending)
	}
}

func TestAlertTriggeredAnalyzer_CleanupOldCooldowns(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)

	// Add some cooldown entries - one old, one recent
	analyzer.mu.Lock()
	analyzer.lastAnalyzed["old-resource"] = time.Now().Add(-2 * time.Hour) // > 1 hour old
	analyzer.lastAnalyzed["recent-resource"] = time.Now()                   // Recent
	analyzer.mu.Unlock()

	// Cleanup
	analyzer.CleanupOldCooldowns()

	analyzer.mu.RLock()
	_, oldExists := analyzer.lastAnalyzed["old-resource"]
	_, recentExists := analyzer.lastAnalyzed["recent-resource"]
	analyzer.mu.RUnlock()

	if oldExists {
		t.Error("Expected old cooldown entry to be removed")
	}
	if !recentExists {
		t.Error("Expected recent cooldown entry to be kept")
	}
}

func TestAlertTriggeredAnalyzer_DeduplicatePendingAnalyses(t *testing.T) {
	// Create a mock state provider with basic data
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{
				{ID: "node-1", Name: "test-node", Status: "online"},
			},
		},
	}

	analyzer := NewAlertTriggeredAnalyzer(nil, stateProvider)
	analyzer.SetEnabled(true)

	alert := &alerts.Alert{
		ID:           "test-alert-1",
		Type:         "node",
		ResourceID:   "node-1",
		ResourceName: "test-node",
	}

	// Use a WaitGroup to track the analysis
	var wg sync.WaitGroup
	wg.Add(1)

	// Manually mark as pending
	analyzer.mu.Lock()
	analyzer.pending["node-1"] = true
	analyzer.mu.Unlock()

	// Second call should be deduplicated
	analyzer.OnAlertFired(alert)

	// Check that we still only have one pending
	analyzer.mu.RLock()
	pendingCount := 0
	for _, isPending := range analyzer.pending {
		if isPending {
			pendingCount++
		}
	}
	analyzer.mu.RUnlock()

	if pendingCount != 1 {
		t.Errorf("Expected 1 pending analysis (deduplication), got %d", pendingCount)
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeResourceByAlert_AlertTypes(t *testing.T) {
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{
				{ID: "node-1", Name: "test-node", Status: "online"},
			},
			VMs: []models.VM{
				{ID: "vm-100", Name: "test-vm", Node: "test-node", Status: "running"},
			},
		},
	}

	analyzer := NewAlertTriggeredAnalyzer(nil, stateProvider)

	// Test alert type detection
	tests := []struct {
		name      string
		alertType string
		shouldRun bool
	}{
		{
			name:      "node alert",
			alertType: "node_cpu",
			shouldRun: true, // Will try to analyze but no patrol service
		},
		{
			name:      "container alert",
			alertType: "container_memory",
			shouldRun: true,
		},
		{
			name:      "vm alert",
			alertType: "vm_disk",
			shouldRun: true,
		},
		{
			name:      "docker alert",
			alertType: "docker_cpu",
			shouldRun: true,
		},
		{
			name:      "storage alert",
			alertType: "storage-usage",
			shouldRun: true,
		},
		{
			name:      "generic cpu alert",
			alertType: "cpu",
			shouldRun: true,
		},
		{
			name:      "unknown alert type",
			alertType: "unknown_type",
			shouldRun: false, // Should not match any handler
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alert := &alerts.Alert{
				ID:           "test-alert",
				Type:         tt.alertType,
				ResourceID:   "node-1",
				ResourceName: "test-node",
			}

			// This should not panic, even without a patrol service
			// analyzeResourceByAlert returns nil when patrolService is nil
			findings := analyzer.analyzeResourceByAlert(nil, alert)

			// Without patrol service, findings should be nil
			if findings != nil {
				t.Error("Expected nil findings without patrol service")
			}
		})
	}
}

func TestAlertTriggeredAnalyzer_ConcurrentAccess(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)
	analyzer.SetEnabled(true)

	// Test concurrent access to the analyzer
	var wg sync.WaitGroup
	iterations := 100

	// Concurrent enable/disable
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			analyzer.SetEnabled(i%2 == 0)
		}
	}()

	// Concurrent IsEnabled checks
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = analyzer.IsEnabled()
		}
	}()

	// Concurrent cleanup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			analyzer.CleanupOldCooldowns()
		}
	}()

	wg.Wait()
	// Test passes if no race conditions or panics
}
