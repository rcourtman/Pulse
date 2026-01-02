package ai

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestAlertTriggeredAnalyzer_AnalyzeNodeFromAlert(t *testing.T) {
	// Create a mock state provider
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{
				{
					ID:     "node/pve1",
					Name:   "pve1",
					Status: "online",
					CPU:    0.95, // 95%
					Memory: models.Memory{
						Total: 32000000000,
						Used:  30000000000,
					},
				},
			},
		},
	}

	// Create a patrol service with default thresholds
	patrolService := &PatrolService{
		thresholds: DefaultPatrolThresholds(),
	}

	analyzer := NewAlertTriggeredAnalyzer(patrolService, stateProvider)

	alert := &alerts.Alert{
		ID:           "alert-1",
		Type:         "node_cpu",
		ResourceID:   "node/pve1",
		ResourceName: "pve1",
	}

	findings := analyzer.analyzeNodeFromAlert(context.Background(), alert)

	if len(findings) == 0 {
		t.Error("Expected findings for high CPU node, got 0")
	}

	foundHighCPU := false
	for _, f := range findings {
		if f.Title == "High CPU usage" {
			foundHighCPU = true
			break
		}
	}

	if !foundHighCPU {
		t.Error("Expected 'High CPU usage' finding")
	}

	// Test with non-existent node
	alertMissing := &alerts.Alert{
		ID:         "alert-2",
		ResourceID: "non-existent",
	}
	findingsMissing := analyzer.analyzeNodeFromAlert(context.Background(), alertMissing)
	if len(findingsMissing) != 0 {
		t.Errorf("Expected 0 findings for non-existent node, got %d", len(findingsMissing))
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeNodeFromAlert_ByNodeField(t *testing.T) {
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{
				{ID: "node/pve1", Name: "pve1", Status: "online", CPU: 0.95},
			},
		},
	}
	patrolService := &PatrolService{thresholds: DefaultPatrolThresholds()}
	analyzer := NewAlertTriggeredAnalyzer(patrolService, stateProvider)

	alert := &alerts.Alert{
		ID:         "alert-1",
		Type:       "node_cpu",
		ResourceID: "node/other",
		Node:       "pve1",
	}

	findings := analyzer.analyzeNodeFromAlert(context.Background(), alert)
	if len(findings) == 0 {
		t.Error("Expected findings when node matches alert.Node")
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeNodeFromAlert_NoState(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(&PatrolService{}, nil)
	alert := &alerts.Alert{ID: "alert-1", ResourceID: "node/pve1"}
	if findings := analyzer.analyzeNodeFromAlert(context.Background(), alert); findings != nil {
		t.Errorf("Expected nil findings with no state provider, got %v", findings)
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeGuestFromAlert(t *testing.T) {
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			VMs: []models.VM{
				{
					ID:     "qemu/100",
					Name:   "test-vm",
					Status: "running",
					CPU:    0.8,
					Memory: models.Memory{
						Usage: 95.0,
					},
					Disk: models.Disk{
						Usage: 90.0,
					},
				},
			},
			Containers: []models.Container{
				{
					ID:     "lxc/200",
					Name:   "test-container",
					Status: "running",
					CPU:    0.5,
					Memory: models.Memory{
						Usage: 96.0,
					},
					Disk: models.Disk{
						Usage: 85.0,
					},
				},
			},
		},
	}

	patrolService := &PatrolService{
		thresholds: DefaultPatrolThresholds(),
	}

	analyzer := NewAlertTriggeredAnalyzer(patrolService, stateProvider)

	// Test VM
	alertVM := &alerts.Alert{
		ID:           "vm-alert",
		Type:         "vm_memory",
		ResourceID:   "qemu/100",
		ResourceName: "test-vm",
	}

	findingsVM := analyzer.analyzeGuestFromAlert(context.Background(), alertVM)
	if len(findingsVM) == 0 {
		t.Error("Expected findings for high memory VM, got 0")
	}

	// Test Container
	alertCT := &alerts.Alert{
		ID:           "ct-alert",
		Type:         "lxc_memory",
		ResourceID:   "lxc/200",
		ResourceName: "test-container",
	}

	findingsCT := analyzer.analyzeGuestFromAlert(context.Background(), alertCT)
	if len(findingsCT) == 0 {
		t.Error("Expected findings for high memory container, got 0")
	}

	// Test non-existent guest
	alertMissing := &alerts.Alert{
		ID:         "missing-alert",
		ResourceID: "qemu/999",
	}
	findingsMissing := analyzer.analyzeGuestFromAlert(context.Background(), alertMissing)
	if len(findingsMissing) != 0 {
		t.Errorf("Expected 0 findings for missing guest, got %d", len(findingsMissing))
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeGuestFromAlert_NoState(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(&PatrolService{}, nil)
	alert := &alerts.Alert{ID: "alert-1", ResourceID: "qemu/100"}
	if findings := analyzer.analyzeGuestFromAlert(context.Background(), alert); findings != nil {
		t.Errorf("Expected nil findings with no state provider, got %v", findings)
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeGuestFromAlert_ByName(t *testing.T) {
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			VMs: []models.VM{
				{
					ID:     "qemu/100",
					Name:   "vm-name",
					Node:   "pve1",
					Status: "running",
					CPU:    0.9,
					Memory: models.Memory{Usage: 95.0},
					Disk:   models.Disk{Usage: 90.0},
				},
			},
		},
	}
	patrolService := &PatrolService{thresholds: DefaultPatrolThresholds()}
	analyzer := NewAlertTriggeredAnalyzer(patrolService, stateProvider)

	alert := &alerts.Alert{
		ID:           "vm-alert",
		Type:         "vm_memory",
		ResourceName: "vm-name",
	}
	findings := analyzer.analyzeGuestFromAlert(context.Background(), alert)
	if len(findings) == 0 {
		t.Error("Expected findings when guest matches ResourceName")
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeDockerFromAlert(t *testing.T) {
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			DockerHosts: []models.DockerHost{
				{
					ID:       "dh-1",
					Hostname: "docker-host-1",
					Status:   "online",
					Containers: []models.DockerContainer{
						{
							ID:            "container-1",
							Name:          "web-server",
							State:         "running",
							MemoryPercent: 95.0,
						},
					},
				},
			},
		},
	}

	patrolService := &PatrolService{
		thresholds: DefaultPatrolThresholds(),
	}

	analyzer := NewAlertTriggeredAnalyzer(patrolService, stateProvider)

	// Test Docker Host
	alertHost := &alerts.Alert{
		ID:           "dh-alert",
		Type:         "docker_host_cpu",
		ResourceID:   "dh-1",
		ResourceName: "docker-host-1",
	}

	findingsHost := analyzer.analyzeDockerFromAlert(context.Background(), alertHost)
	// Even if everything is fine, it shouldn't return nil if the host is found (though findings might be 0)
	// But in my mock, if things are fine, findings will be 0.
	// Let's make it offline to get a finding.
	stateProvider.state.DockerHosts[0].Status = "offline"
	findingsHost = analyzer.analyzeDockerFromAlert(context.Background(), alertHost)
	if len(findingsHost) == 0 {
		t.Error("Expected findings for offline Docker host, got 0")
	}

	// Test Docker Container
	stateProvider.state.DockerHosts[0].Status = "online"
	alertContainer := &alerts.Alert{
		ID:           "container-alert",
		Type:         "docker_container_memory",
		ResourceID:   "container-1",
		ResourceName: "web-server",
	}

	findingsContainer := analyzer.analyzeDockerFromAlert(context.Background(), alertContainer)
	if len(findingsContainer) == 0 {
		t.Error("Expected findings for high memory Docker container, got 0")
	}

	// Test missing Docker resource
	alertMissing := &alerts.Alert{
		ID:         "missing-alert",
		ResourceID: "container-999",
	}
	findingsMissing := analyzer.analyzeDockerFromAlert(context.Background(), alertMissing)
	if len(findingsMissing) != 0 {
		t.Errorf("Expected 0 findings for missing Docker resource, got %d", len(findingsMissing))
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeDockerFromAlert_NoState(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(&PatrolService{}, nil)
	alert := &alerts.Alert{ID: "alert-1", ResourceID: "docker-host"}
	if findings := analyzer.analyzeDockerFromAlert(context.Background(), alert); findings != nil {
		t.Errorf("Expected nil findings with no state provider, got %v", findings)
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeStorageFromAlert(t *testing.T) {
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Storage: []models.Storage{
				{
					ID:    "storage-1",
					Name:  "local-lvm",
					Usage: 95.0,
					Total: 100000000000,
					Used:  95000000000,
				},
			},
		},
	}

	patrolService := &PatrolService{
		thresholds: DefaultPatrolThresholds(),
	}

	analyzer := NewAlertTriggeredAnalyzer(patrolService, stateProvider)

	// Test storage with high usage
	alert := &alerts.Alert{
		ID:           "storage-alert",
		Type:         "storage-usage",
		ResourceID:   "storage-1",
		ResourceName: "local-lvm",
	}

	findings := analyzer.analyzeStorageFromAlert(context.Background(), alert)
	if len(findings) == 0 {
		t.Error("Expected findings for high storage usage, got 0")
	}

	// Test missing storage
	alertMissing := &alerts.Alert{
		ID:         "missing-alert",
		ResourceID: "storage-999",
	}
	findingsMissing := analyzer.analyzeStorageFromAlert(context.Background(), alertMissing)
	if len(findingsMissing) != 0 {
		t.Errorf("Expected 0 findings for missing storage, got %d", len(findingsMissing))
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeStorageFromAlert_NoState(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(&PatrolService{}, nil)
	alert := &alerts.Alert{ID: "alert-1", ResourceID: "storage-1"}
	if findings := analyzer.analyzeStorageFromAlert(context.Background(), alert); findings != nil {
		t.Errorf("Expected nil findings with no state provider, got %v", findings)
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeGenericResourceFromAlert(t *testing.T) {
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{
				{
					ID:     "node/pve1",
					Name:   "pve1",
					Status: "online",
					CPU:    0.95,
					Memory: models.Memory{
						Total: 32000000000,
						Used:  30000000000,
					},
				},
			},
			VMs: []models.VM{
				{
					ID:     "qemu/100",
					Name:   "test-vm",
					Status: "running",
					CPU:    0.8,
					Memory: models.Memory{
						Usage: 95.0,
					},
					Disk: models.Disk{
						Usage: 90.0,
					},
				},
			},
		},
	}

	patrolService := &PatrolService{
		thresholds: DefaultPatrolThresholds(),
	}

	analyzer := NewAlertTriggeredAnalyzer(patrolService, stateProvider)

	// Test node resourceID pattern - use ResourceName to match node.Name
	alertNode := &alerts.Alert{
		ID:           "cpu-alert",
		Type:         "cpu",
		ResourceID:   "cluster1/node/pve1",
		ResourceName: "pve1",
	}
	findingsNode := analyzer.analyzeGenericResourceFromAlert(context.Background(), alertNode)
	// Should route to analyzeNodeFromAlert
	if len(findingsNode) == 0 {
		t.Error("Expected findings for node CPU, got 0")
	}

	// Test qemu resourceID pattern - use ResourceName to match vm.Name
	alertQemu := &alerts.Alert{
		ID:           "mem-alert",
		Type:         "memory",
		ResourceID:   "cluster1/qemu/100",
		ResourceName: "test-vm",
	}
	findingsQemu := analyzer.analyzeGenericResourceFromAlert(context.Background(), alertQemu)
	// Should route to analyzeGuestFromAlert
	if len(findingsQemu) == 0 {
		t.Error("Expected findings for qemu memory, got 0")
	}

	// Test docker resourceID pattern
	alertDocker := &alerts.Alert{
		ID:         "docker-alert",
		Type:       "disk",
		ResourceID: "docker-host-1",
	}
	// This won't find anything, so we expect 0 findings (no docker hosts in state)
	findingsDocker := analyzer.analyzeGenericResourceFromAlert(context.Background(), alertDocker)
	if len(findingsDocker) != 0 {
		t.Errorf("Expected 0 findings for missing docker, got %d", len(findingsDocker))
	}

	// Test fallback (tries guest first, then node)
	alertGeneric := &alerts.Alert{
		ID:           "generic-alert",
		Type:         "cpu",
		ResourceID:   "test-vm",
		ResourceName: "test-vm",
	}
	findingsGeneric := analyzer.analyzeGenericResourceFromAlert(context.Background(), alertGeneric)
	if len(findingsGeneric) == 0 {
		t.Error("Expected findings for generic CPU alert (fallback to guest), got 0")
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

func TestAlertTriggeredAnalyzer_CleanupOldCooldowns(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)

	// Add some cooldown entries - one old, one recent
	analyzer.mu.Lock()
	analyzer.lastAnalyzed["old-resource"] = time.Now().Add(-2 * time.Hour) // > 1 hour old
	analyzer.lastAnalyzed["recent-resource"] = time.Now()                  // Recent
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

func TestAlertTriggeredAnalyzer_OnAlertFired_Enabled(t *testing.T) {
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{
				{
					ID:     "node/pve1",
					Name:   "pve1",
					Status: "online",
					CPU:    0.95,
					Memory: models.Memory{
						Total: 32000000000,
						Used:  30000000000,
					},
				},
			},
		},
	}

	patrolService := &PatrolService{
		thresholds: DefaultPatrolThresholds(),
	}

	analyzer := NewAlertTriggeredAnalyzer(patrolService, stateProvider)
	analyzer.SetEnabled(true)

	alert := &alerts.Alert{
		ID:           "test-alert-1",
		Type:         "node_cpu",
		ResourceID:   "node/pve1",
		ResourceName: "pve1",
		Value:        95.0,
		Threshold:    90.0,
	}

	// Fire the alert
	analyzer.OnAlertFired(alert)

	// Give time for the goroutine to start and set pending
	time.Sleep(50 * time.Millisecond)

	// Wait for analysis to complete
	time.Sleep(100 * time.Millisecond)

	// After analysis, lastAnalyzed should be updated
	analyzer.mu.RLock()
	_, exists := analyzer.lastAnalyzed["node/pve1"]
	analyzer.mu.RUnlock()

	if !exists {
		t.Error("Expected lastAnalyzed to be updated after alert was fired")
	}
}

func TestAlertTriggeredAnalyzer_OnAlertFired_Disabled(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)
	// Analyzer is disabled by default

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

func TestAlertTriggeredAnalyzer_OnAlertFired_EmptyResourceKey(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)
	analyzer.SetEnabled(true)

	// Alert with no resource identifiers
	alert := &alerts.Alert{
		ID:   "test-alert",
		Type: "cpu",
	}

	// Should skip analysis due to empty resource key
	analyzer.OnAlertFired(alert)

	// Wait briefly
	time.Sleep(10 * time.Millisecond)

	// No pending should exist
	analyzer.mu.RLock()
	pending := len(analyzer.pending)
	analyzer.mu.RUnlock()

	if pending != 0 {
		t.Errorf("Expected no pending analyses for empty resource key, got %d", pending)
	}
}

func TestAlertTriggeredAnalyzer_OnAlertFired_Cooldown(t *testing.T) {
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

func TestAlertTriggeredAnalyzer_OnAlertFired_Deduplication(t *testing.T) {
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

func TestAlertTriggeredAnalyzer_AnalyzeResourceByAlert(t *testing.T) {
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{
				{
					ID:     "node/pve1",
					Name:   "pve1",
					Status: "online",
					CPU:    0.95,
					Memory: models.Memory{
						Total: 32000000000,
						Used:  30000000000,
					},
				},
			},
			VMs: []models.VM{
				{
					ID:     "qemu/100",
					Name:   "test-vm",
					Node:   "pve1",
					Status: "running",
					CPU:    0.8,
					Memory: models.Memory{
						Usage: 95.0,
					},
					Disk: models.Disk{
						Usage: 90.0,
					},
				},
			},
		},
	}

	patrolService := &PatrolService{
		thresholds: DefaultPatrolThresholds(),
	}

	analyzer := NewAlertTriggeredAnalyzer(patrolService, stateProvider)

	// Test node alert type
	alertNode := &alerts.Alert{
		ID:           "node-alert",
		Type:         "node_cpu",
		ResourceID:   "node/pve1",
		ResourceName: "pve1",
	}
	findingsNode := analyzer.analyzeResourceByAlert(context.Background(), alertNode)
	if len(findingsNode) == 0 {
		t.Error("Expected findings for node alert, got 0")
	}

	// Test container/VM alert type
	alertVM := &alerts.Alert{
		ID:           "vm-alert",
		Type:         "container_memory",
		ResourceID:   "qemu/100",
		ResourceName: "test-vm",
	}
	findingsVM := analyzer.analyzeResourceByAlert(context.Background(), alertVM)
	if len(findingsVM) == 0 {
		t.Error("Expected findings for container/VM alert, got 0")
	}

	// Test generic disk alert routing
	alertDisk := &alerts.Alert{
		ID:           "disk-alert",
		Type:         "disk",
		ResourceID:   "test-vm",
		ResourceName: "test-vm",
	}
	findingsDisk := analyzer.analyzeResourceByAlert(context.Background(), alertDisk)
	if len(findingsDisk) == 0 {
		t.Error("Expected findings for generic disk alert, got 0")
	}

	// Test unknown alert type
	alertUnknown := &alerts.Alert{
		ID:   "unknown-alert",
		Type: "unknown_type_xyz",
	}
	findingsUnknown := analyzer.analyzeResourceByAlert(context.Background(), alertUnknown)
	if len(findingsUnknown) != 0 {
		t.Errorf("Expected 0 findings for unknown alert type, got %d", len(findingsUnknown))
	}

	// Test Docker alert type
	stateProvider.state.DockerHosts = []models.DockerHost{
		{ID: "dh-1", Hostname: "docker-host", Status: "offline"},
	}
	alertDocker := &alerts.Alert{
		ID:           "docker-alert",
		Type:         "docker_offline",
		ResourceID:   "dh-1",
		ResourceName: "docker-host",
	}
	findingsDocker := analyzer.analyzeResourceByAlert(context.Background(), alertDocker)
	if len(findingsDocker) == 0 {
		t.Error("Expected findings for docker alert, got 0")
	}

	// Test storage alert type
	stateProvider.state.Storage = []models.Storage{
		{ID: "storage-1", Name: "local", Usage: 95.0, Total: 100, Used: 95},
	}
	alertStorage := &alerts.Alert{
		ID:           "storage-alert",
		Type:         "storage_usage",
		ResourceID:   "storage-1",
		ResourceName: "local",
	}
	findingsStorage := analyzer.analyzeResourceByAlert(context.Background(), alertStorage)
	if len(findingsStorage) == 0 {
		t.Error("Expected findings for storage alert, got 0")
	}

	// Test cpu alert with node resource ID
	alertNodeCPU := &alerts.Alert{
		ID:           "node-cpu",
		Type:         "cpu",
		ResourceID:   "cluster/node/pve1",
		ResourceName: "pve1",
	}
	findingsNodeCPU := analyzer.analyzeResourceByAlert(context.Background(), alertNodeCPU)
	if len(findingsNodeCPU) == 0 {
		t.Error("Expected findings for node cpu alert, got 0")
	}

	// Test with nil patrol service
	analyzerNoPatrol := NewAlertTriggeredAnalyzer(nil, stateProvider)
	findingsNilPatrol := analyzerNoPatrol.analyzeResourceByAlert(context.Background(), alertNode)
	if findingsNilPatrol != nil {
		t.Error("Expected nil findings when patrol service is nil")
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeResource(t *testing.T) {
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{
				{ID: "node/pve1", Name: "pve1", Status: "online", CPU: 0.95},
			},
		},
	}
	patrolService := NewPatrolService(nil, nil)
	patrolService.aiService = &Service{}

	analyzer := NewAlertTriggeredAnalyzer(patrolService, stateProvider)

	alert := &alerts.Alert{
		ID:           "alert-1",
		Type:         "node_cpu",
		ResourceID:   "node/pve1",
		ResourceName: "pve1",
	}
	analyzer.analyzeResource(alert, "node/pve1")

	if analyzer.pending["node/pve1"] {
		t.Error("Expected pending to be cleared after analysis")
	}
	if _, exists := analyzer.lastAnalyzed["node/pve1"]; !exists {
		t.Error("Expected lastAnalyzed to be updated after analysis")
	}
	if len(patrolService.findings.GetAll(nil)) == 0 {
		t.Error("Expected findings to be added after analysis")
	}
	for _, finding := range patrolService.findings.GetAll(nil) {
		if finding.AlertID != "alert-1" {
			t.Errorf("Expected AlertID to be set, got %s", finding.AlertID)
		}
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeResource_NoFindings(t *testing.T) {
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{
				{ID: "node/pve1", Name: "pve1", Status: "online", CPU: 0.05},
			},
		},
	}
	patrolService := NewPatrolService(nil, nil)
	patrolService.aiService = &Service{}

	analyzer := NewAlertTriggeredAnalyzer(patrolService, stateProvider)

	alert := &alerts.Alert{
		ID:           "alert-1",
		Type:         "node_cpu",
		ResourceID:   "node/pve1",
		ResourceName: "pve1",
	}
	analyzer.analyzeResource(alert, "node/pve1")

	if len(patrolService.findings.GetAll(nil)) != 0 {
		t.Error("Expected no findings for healthy resource")
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeGenericResourceFromAlert_FallbackToNode(t *testing.T) {
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{
				{ID: "node/pve1", Name: "pve1", Status: "online", CPU: 0.95},
			},
		},
	}
	patrolService := &PatrolService{thresholds: DefaultPatrolThresholds()}
	analyzer := NewAlertTriggeredAnalyzer(patrolService, stateProvider)

	alert := &alerts.Alert{
		ID:           "generic-alert",
		Type:         "cpu",
		ResourceID:   "pve1",
		ResourceName: "pve1",
	}
	findings := analyzer.analyzeGenericResourceFromAlert(context.Background(), alert)
	if len(findings) == 0 {
		t.Error("Expected findings from node fallback")
	}
}

type mockStateProvider struct {
	state models.StateSnapshot
}

func (m *mockStateProvider) GetState() models.StateSnapshot {
	return m.state
}

func TestAlertTriggeredAnalyzer_AnalyzeGuestFromAlert_PreservesBackup(t *testing.T) {
	// Set up a recent backup time (yesterday)
	lastBackup := time.Now().Add(-24 * time.Hour)

	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			VMs: []models.VM{
				{
					ID:     "qemu/100",
					Name:   "backup-test-vm",
					Status: "running",
					CPU:    0.1, // Low CPU, not the issue
					Memory: models.Memory{
						Usage: 20.0,
					},
					Disk: models.Disk{
						Usage: 20.0,
					},
					LastBackup: lastBackup, // <--- VM has a backup!
				},
			},
		},
	}

	patrolService := &PatrolService{
		thresholds: DefaultPatrolThresholds(),
	}

	analyzer := NewAlertTriggeredAnalyzer(patrolService, stateProvider)

	// Trigger a CPU alert (unrelated to backup)
	// This forces the analyzer to run analyzeGuestFromAlert
	alertVM := &alerts.Alert{
		ID:           "vm-cpu-alert",
		Type:         "cpu",
		ResourceID:   "qemu/100",
		ResourceName: "backup-test-vm",
		Value:        90.0,
	}

	findings := analyzer.analyzeGuestFromAlert(context.Background(), alertVM)

	// We might get CPU findings if we set CPU high enough, or none if we don't.
	// But critically, we should NOT get "Never backed up".

	for _, f := range findings {
		if f.Key == "backup-never" {
			t.Error("Found 'backup-never' finding despite VM having a valid LastBackup timestamp. Regression detected!")
		}
	}

	// Double check: if we intentionally make the backup VERY old, we SHOULD get a stale backup finding
	// This proves the timestamp is actually being passed through.
	staleBackup := time.Now().Add(-400 * 24 * time.Hour) // 400 days ago
	stateProvider.state.VMs[0].LastBackup = staleBackup

	findingsStale := analyzer.analyzeGuestFromAlert(context.Background(), alertVM)

	foundStale := false
	for _, f := range findingsStale {
		if f.Key == "backup-stale" {
			foundStale = true
			break
		}
	}

	if !foundStale {
		t.Error("Expected 'backup-stale' finding for very old backup, but didn't find it. LastBackup might not be getting passed correctly.")
	}
}

func TestAlertTriggeredAnalyzer_AnalyzeGuestFromAlert_ContainerBackup(t *testing.T) {
	lastBackup := time.Now().Add(-2 * time.Hour)

	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Containers: []models.Container{
				{
					ID:         "lxc/200",
					Name:       "backup-ct",
					Status:     "running",
					CPU:        0.5,
					Memory:     models.Memory{Usage: 95.0},
					Disk:       models.Disk{Usage: 90.0},
					LastBackup: lastBackup,
				},
			},
		},
	}

	patrolService := &PatrolService{
		thresholds: DefaultPatrolThresholds(),
	}

	analyzer := NewAlertTriggeredAnalyzer(patrolService, stateProvider)

	alertCT := &alerts.Alert{
		ID:           "ct-alert",
		Type:         "lxc_memory",
		ResourceID:   "lxc/200",
		ResourceName: "backup-ct",
	}

	findings := analyzer.analyzeGuestFromAlert(context.Background(), alertCT)
	if len(findings) == 0 {
		t.Error("Expected findings for container with backup set, got 0")
	}
}

func TestAlertTriggeredAnalyzer_StartStop(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)

	// Verify not started initially
	analyzer.mu.RLock()
	tickerBefore := analyzer.cleanupTicker
	analyzer.mu.RUnlock()

	if tickerBefore != nil {
		t.Error("Expected cleanupTicker to be nil before Start")
	}

	// Start the cleanup goroutine
	analyzer.Start()

	analyzer.mu.RLock()
	tickerAfter := analyzer.cleanupTicker
	analyzer.mu.RUnlock()

	if tickerAfter == nil {
		t.Error("Expected cleanupTicker to be set after Start")
	}

	// Calling Start again should be a no-op
	analyzer.Start()

	analyzer.mu.RLock()
	tickerAfterSecondStart := analyzer.cleanupTicker
	analyzer.mu.RUnlock()

	if tickerAfterSecondStart != tickerAfter {
		t.Error("Expected cleanupTicker to remain the same after second Start")
	}

	// Stop the cleanup goroutine
	analyzer.Stop()

	analyzer.mu.RLock()
	tickerAfterStop := analyzer.cleanupTicker
	analyzer.mu.RUnlock()

	if tickerAfterStop != nil {
		t.Error("Expected cleanupTicker to be nil after Stop")
	}
}

func TestAlertTriggeredAnalyzer_CleanupTickerRuns(t *testing.T) {
	analyzer := NewAlertTriggeredAnalyzer(nil, nil)

	// Add an old entry
	analyzer.mu.Lock()
	analyzer.lastAnalyzed["old-entry"] = time.Now().Add(-2 * time.Hour)
	analyzer.mu.Unlock()

	// Start with a very short ticker for testing (we'll manually trigger cleanup)
	analyzer.Start()
	defer analyzer.Stop()

	// Verify the entry exists
	analyzer.mu.RLock()
	_, existsBefore := analyzer.lastAnalyzed["old-entry"]
	analyzer.mu.RUnlock()

	if !existsBefore {
		t.Fatal("Expected 'old-entry' to exist before cleanup")
	}

	// Manually trigger cleanup
	analyzer.CleanupOldCooldowns()

	// Verify the entry was removed
	analyzer.mu.RLock()
	_, existsAfter := analyzer.lastAnalyzed["old-entry"]
	analyzer.mu.RUnlock()

	if existsAfter {
		t.Error("Expected 'old-entry' to be removed after cleanup")
	}
}
