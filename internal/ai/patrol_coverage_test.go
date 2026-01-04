package ai

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// TestPatrolService_RunPatrol_FullCoverage tests the main patrol loop logic including AI analysis
func TestPatrolService_RunPatrol_FullCoverage(t *testing.T) {
	// Setup dependencies
	persistence := config.NewConfigPersistence(t.TempDir())
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{Enabled: true}

	// Mock provider for AI analysis
	mockP := &mockProvider{
		chatFunc: func(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
			// Return a response with findings to verify parsing
			// ...
			response := `
[FINDING]
SEVERITY: warning
CATEGORY: performance
RESOURCE: vm-100
RESOURCE_TYPE: vm
TITLE: High CPU
DESCRIPTION: CPU usage is high
RECOMMENDATION: Check processes
EVIDENCE: CPU > 90%
[/FINDING]
`
			return &providers.ChatResponse{
				Content: response,
				Model:   "mock-model",
			}, nil
		},
	}
	svc.provider = mockP

	// Set model that will fail creation (missing API key) to ensure fallback to mock provider
	svc.cfg.PatrolModel = "anthropic:mock-model"

	// Mock state provider
	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			VMs: []models.VM{
				{ID: "vm-100", VMID: 100, Name: "web-server", Node: "pve-1", Status: "running", CPU: 0.95},
			},
			Nodes: []models.Node{
				{ID: "node-1", Name: "pve-1", Status: "online"},
			},
		},
	}

	// Create patrol service with mocked dependencies
	ps := NewPatrolService(svc, stateProvider)

	// Configure patrol to ensure it runs
	ps.SetConfig(PatrolConfig{
		Enabled:       true,
		Interval:      100 * time.Millisecond,
		AnalyzeNodes:  true,
		AnalyzeGuests: true,
	})
	ps.SetProactiveMode(true)

	// Since Start() runs in a goroutine, we want to test runPatrol directly to be deterministic
	// But we also want to test that dependencies are wired up correctly.
	// runPatrol checks cfg.Enabled and aiService.IsEnabled()

	ctx := context.Background()
	ps.runPatrol(ctx)

	// Check findings
	findings := ps.GetFindings().GetActive(FindingSeverityInfo)
	if len(findings) == 0 {
		t.Error("Expected findings from patrol run, got 0")
	} else {
		if findings[0].Title != "High CPU" {
			t.Errorf("Expected finding title 'High CPU', got '%s'", findings[0].Title)
		}
	}

	// Check coverage for GetStatus fields updated during run
	status := ps.GetStatus()
	if status.FindingsCount != 1 {
		t.Errorf("Expected 1 finding in status, got %d", status.FindingsCount)
	}
	if status.ResourcesChecked == 0 {
		t.Error("Expected resources checked > 0")
	}
}

// TestPatrolService_StartStop verifies the startup and shutdown sequence covering patrolLoop
func TestPatrolService_StartStop(t *testing.T) {
	svc := NewService(nil, nil)
	stateProvider := &mockStateProvider{}
	ps := NewPatrolService(svc, stateProvider)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Capture logs or side effects if needed, but mainly ensuring no panic and state transitions
	ps.Start(ctx)

	time.Sleep(10 * time.Millisecond) // Give it a moment to start

	// Call Start again should return early coverage
	ps.Start(ctx)

	ps.Stop()

	// Call Stop again should return early coverage
	ps.Stop()
}

// TestPatrolService_Setters_Coverage tests setter methods not covered in existing tests
func TestPatrolService_Setters_Coverage(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	// SetConfig trigger channel logic
	ps.Start(context.Background())

	// We need to wait a bit for listeners
	time.Sleep(10 * time.Millisecond)

	// Create a new config with different interval to trigger update
	newCfg := PatrolConfig{
		Enabled:  true,
		Interval: 5 * time.Minute,
	}
	ps.SetConfig(newCfg)

	// Give it time to process channel
	time.Sleep(10 * time.Millisecond)

	ps.Stop()

	// Verify SetIncidentStore getter/setter
	ps.SetIncidentStore(nil)
	if ps.GetIncidentStore() != nil {
		t.Error("Expected nil incident store")
	}
}

// TestPatrol_RunHeuristicAnalysis_Coverage tests logic branches in heuristic analysis
func TestPatrol_RunHeuristicAnalysis_Coverage(t *testing.T) {
	// Heuristic analysis relies on state and thresholds.
	// We can set thresholds low to trigger heuristics without AI.

	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{
				{ID: "node-high-cpu", Name: "node1", Status: "online", CPU: 95.0, Memory: models.Memory{Usage: 80.0}},
				{ID: "node-high-mem", Name: "node2", Status: "online", CPU: 10.0, Memory: models.Memory{Usage: 95.0}},
			},
			VMs: []models.VM{
				{ID: "vm-high-mem", Name: "vm1", Status: "running", Memory: models.Memory{Usage: 95.0}},
			},
			Storage: []models.Storage{
				{ID: "storage-full", Name: "store1", Status: "active", Usage: 95.0},
			},
		},
	}

	ps := NewPatrolService(nil, stateProvider)

	// Set low thresholds to ensure triggers
	provider := &mockThresholdProvider{
		nodeCPU:    50,
		nodeMemory: 50,
		guestMem:   50,
		guestDisk:  50,
		storage:    50,
	}
	ps.SetThresholdProvider(provider)

	// runHeuristics is private, but called by runPatrol.
	// However, without AI service, runPatrol might skip or return early?
	// check runPatrol implementation:
	// if p.aiService == nil ... skipping patrol

	// So we typically need AI service for part of it.
	// But `Start()` checks AI enabled.
	// Let's create an AI service that is enabled but mock provider returns error or dummy,
	// to see if heuristics run BEFORE AI?
	// Looking at patrol.go (not visible fully), typically heuristics might run as part of analysis.
	// Generally, heuristic findings are added to `findings`.

	// If runPatrol requires AI service, we provide one.
	svc := NewService(nil, nil)

	// Set model that will fail creation (missing API key) to ensure fallback to mock provider
	svc.cfg = &config.AIConfig{
		Enabled:     true,
		PatrolModel: "anthropic:mock-model",
	}

	// Mock provider can return empty or error, finding generation dependent on logic
	mockP := &mockProvider{
		chatFunc: func(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
			return &providers.ChatResponse{Content: "Nothing"}, nil
		},
	}
	svc.provider = mockP

	// Link them
	ps.aiService = svc

	// Disable AI patrol feature to force heuristic analysis
	licenseChecker := &mockLicenseStore{
		features: map[string]bool{
			"ai_patrol": false, // explicitly false
		},
		state: "active",
		valid: true,
	}
	svc.SetLicenseChecker(licenseChecker)

	ctx := context.Background()
	ps.runPatrol(ctx)

	// Check if heuristics generated findings
	// With low thresholds and high usage in state, we expect findings
	findings := ps.GetFindings().GetActive(FindingSeverityWarning)
	// We expect at least one finding (CPU, Memory, or Storage)
	if len(findings) == 0 {
		// Try Info severity (maybe watch/heuristic produces lower severity?)
		findings = ps.GetFindings().GetActive(FindingSeverityInfo)
		if len(findings) == 0 {
			t.Error("Expected findings from heuristic analysis")
		}
	}
}
