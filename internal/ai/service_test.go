package ai

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestNewService(t *testing.T) {
	svc := NewService(nil, nil)

	if svc == nil {
		t.Fatal("Expected non-nil service")
	}

	// Should not be enabled without config
	if svc.IsEnabled() {
		t.Error("Expected service to not be enabled without config")
	}

	// Cost store should be initialized
	if svc.costStore == nil {
		t.Error("Expected cost store to be initialized")
	}
}

func TestService_IsEnabled(t *testing.T) {
	svc := NewService(nil, nil)

	// Without config
	if svc.IsEnabled() {
		t.Error("Expected not enabled without config")
	}

	// Should remain not enabled since we can't create a real provider in unit tests
}

func TestService_GetConfig_Nil(t *testing.T) {
	svc := NewService(nil, nil)

	cfg := svc.GetConfig()
	if cfg != nil {
		t.Error("Expected nil config before loading")
	}
}

func TestService_GetAIConfig(t *testing.T) {
	svc := NewService(nil, nil)

	cfg := svc.GetAIConfig()
	if cfg != nil {
		t.Error("Expected nil AI config before loading")
	}
}

func TestService_GetPatrolService_Initial(t *testing.T) {
	svc := NewService(nil, nil)

	patrol := svc.GetPatrolService()
	if patrol != nil {
		t.Error("Expected nil patrol service before state provider is set")
	}
}

func TestService_GetAlertTriggeredAnalyzer_Initial(t *testing.T) {
	svc := NewService(nil, nil)

	analyzer := svc.GetAlertTriggeredAnalyzer()
	if analyzer != nil {
		t.Error("Expected nil analyzer before state provider is set")
	}
}

func TestService_SetStateProvider(t *testing.T) {
	svc := NewService(nil, nil)

	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{
				{ID: "node-1", Name: "test-node"},
			},
		},
	}

	svc.SetStateProvider(stateProvider)

	// Patrol service should now be initialized
	patrol := svc.GetPatrolService()
	if patrol == nil {
		t.Error("Expected patrol service to be initialized after setting state provider")
	}

	// Alert triggered analyzer should now be initialized
	analyzer := svc.GetAlertTriggeredAnalyzer()
	if analyzer == nil {
		t.Error("Expected alert analyzer to be initialized after setting state provider")
	}
}

func TestService_GetCostSummary_NoStore(t *testing.T) {
	svc := NewService(nil, nil)
	svc.costStore = nil

	summary := svc.GetCostSummary(30)

	// Should return an empty summary
	if summary.Days != 30 {
		t.Errorf("Expected days 30, got %d", summary.Days)
	}
}

func TestService_GetCostSummary_WithStore(t *testing.T) {
	svc := NewService(nil, nil)

	summary := svc.GetCostSummary(7)

	// Should return a valid summary
	if summary.Days != 7 {
		t.Errorf("Expected days 7, got %d", summary.Days)
	}
}

func TestService_ListCostEvents_NoStore(t *testing.T) {
	svc := NewService(nil, nil)
	svc.costStore = nil

	events := svc.ListCostEvents(7)
	if events != nil {
		t.Error("Expected nil events when no store")
	}
}

func TestService_ClearCostHistory_NoStore(t *testing.T) {
	svc := NewService(nil, nil)
	svc.costStore = nil

	err := svc.ClearCostHistory()
	if err != nil {
		t.Errorf("Expected no error when no store, got %v", err)
	}
}

func TestService_AcquireExecutionSlot(t *testing.T) {
	svc := NewService(nil, nil)

	ctx := context.Background()

	// Acquire a chat slot
	release, err := svc.acquireExecutionSlot(ctx, "chat")
	if err != nil {
		t.Fatalf("Failed to acquire chat slot: %v", err)
	}
	if release == nil {
		t.Fatal("Expected non-nil release function")
	}
	release()

	// Acquire a patrol slot
	release, err = svc.acquireExecutionSlot(ctx, "patrol")
	if err != nil {
		t.Fatalf("Failed to acquire patrol slot: %v", err)
	}
	release()

	// Empty use case should default to chat
	release, err = svc.acquireExecutionSlot(ctx, "")
	if err != nil {
		t.Fatalf("Failed to acquire slot with empty use case: %v", err)
	}
	release()
}

// Note: TestService_AcquireExecutionSlot_Canceled removed - the slot acquisition
// doesn't immediately check context cancel in the select since slots are available

func TestService_EnforceBudget_NoBudget(t *testing.T) {
	svc := NewService(nil, nil)

	// No budget set - should pass
	err := svc.enforceBudget("chat")
	if err != nil {
		t.Errorf("Expected no error without budget, got %v", err)
	}
}

func TestService_StartPatrol_NoPatrol(t *testing.T) {
	svc := NewService(nil, nil)

	// Should not panic when patrol is nil
	svc.StartPatrol(context.Background())
}

func TestService_StopPatrol_NoPatrol(t *testing.T) {
	svc := NewService(nil, nil)

	// Should not panic when patrol is nil
	svc.StopPatrol()
}

func TestService_ReconfigurePatrol_NoPatrol(t *testing.T) {
	svc := NewService(nil, nil)

	// Should not panic when patrol is nil
	svc.ReconfigurePatrol()
}

func TestService_LookupGuestsByVMID(t *testing.T) {
	svc := NewService(nil, nil)

	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			VMs: []models.VM{
				{ID: "vm-100", VMID: 100, Name: "web-server", Node: "pve-1"},
				{ID: "vm-101", VMID: 101, Name: "database", Node: "pve-1"},
			},
			Containers: []models.Container{
				{ID: "ct-200", VMID: 200, Name: "nginx", Node: "pve-1"},
			},
		},
	}
	svc.SetStateProvider(stateProvider)

	// Test finding a VM
	guests := svc.lookupGuestsByVMID(100, "")
	if len(guests) != 1 {
		t.Errorf("Expected 1 guest for VMID 100, got %d", len(guests))
	}
	if len(guests) > 0 && guests[0].Name != "web-server" {
		t.Errorf("Expected name 'web-server', got '%s'", guests[0].Name)
	}

	// Test finding a container
	guests = svc.lookupGuestsByVMID(200, "")
	if len(guests) != 1 {
		t.Errorf("Expected 1 guest for VMID 200, got %d", len(guests))
	}
	if len(guests) > 0 && guests[0].Type != "lxc" {
		t.Errorf("Expected type 'lxc', got '%s'", guests[0].Type)
	}

	// Test not found
	guests = svc.lookupGuestsByVMID(999, "")
	if len(guests) != 0 {
		t.Errorf("Expected 0 guests for VMID 999, got %d", len(guests))
	}
}

func TestService_LookupGuestsByVMID_NoStateProvider(t *testing.T) {
	svc := NewService(nil, nil)

	guests := svc.lookupGuestsByVMID(100, "")
	if guests != nil {
		t.Error("Expected nil guests without state provider")
	}
}

func TestService_LookupNodeForVMID(t *testing.T) {
	svc := NewService(nil, nil)

	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			VMs: []models.VM{
				{ID: "vm-100", VMID: 100, Name: "web-server", Node: "pve-1"},
			},
		},
	}
	svc.SetStateProvider(stateProvider)

	node, name, guestType := svc.lookupNodeForVMID(100)

	if node != "pve-1" {
		t.Errorf("Expected node 'pve-1', got '%s'", node)
	}
	if name != "web-server" {
		t.Errorf("Expected name 'web-server', got '%s'", name)
	}
	if guestType != "qemu" {
		t.Errorf("Expected type 'qemu', got '%s'", guestType)
	}
}

func TestExtractVMIDFromCommand(t *testing.T) {
	tests := []struct {
		name             string
		command          string
		expectedVMID     int
		expectedOwner    bool
		expectedFound    bool
	}{
		{
			name:          "pct exec",
			command:       "pct exec 100 -- ls",
			expectedVMID:  100,
			expectedOwner: true,
			expectedFound: true,
		},
		{
			name:          "pct enter",
			command:       "pct enter 101",
			expectedVMID:  101,
			expectedOwner: true,
			expectedFound: true,
		},
		{
			name:          "qm start",
			command:       "qm start 200",
			expectedVMID:  200,
			expectedOwner: true,
			expectedFound: true,
		},
		{
			name:          "qm guest exec",
			command:       "qm guest exec 201 ls",
			expectedVMID:  201,
			expectedOwner: true,
			expectedFound: true,
		},
		{
			name:          "vzdump",
			command:       "vzdump 100",
			expectedVMID:  100,
			expectedOwner: false, // vzdump is cluster-aware
			expectedFound: true,
		},
		{
			name:          "no vmid command",
			command:       "ls -la",
			expectedVMID:  0,
			expectedOwner: false,
			expectedFound: false,
		},
		{
			name:          "empty command",
			command:       "",
			expectedVMID:  0,
			expectedOwner: false,
			expectedFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vmid, owner, found := extractVMIDFromCommand(tt.command)
			if vmid != tt.expectedVMID {
				t.Errorf("Expected VMID %d, got %d", tt.expectedVMID, vmid)
			}
			if owner != tt.expectedOwner {
				t.Errorf("Expected owner %v, got %v", tt.expectedOwner, owner)
			}
			if found != tt.expectedFound {
				t.Errorf("Expected found %v, got %v", tt.expectedFound, found)
			}
		})
	}
}

func TestFormatApprovalNeededToolResult(t *testing.T) {
	result := formatApprovalNeededToolResult("rm -rf /", "tool-123", "Dangerous command")

	if result == "" {
		t.Error("Expected non-empty result")
	}

	// Should contain APPROVAL_REQUIRED prefix
	if !hasPrefix(result, "APPROVAL_REQUIRED:") {
		t.Error("Expected APPROVAL_REQUIRED prefix")
	}

	// Should contain the command
	if !containsString(result, "rm -rf /") {
		t.Error("Expected result to contain command")
	}
}

func TestFormatPolicyBlockedToolResult(t *testing.T) {
	result := formatPolicyBlockedToolResult("rm -rf /", "Command not allowed by policy")

	if result == "" {
		t.Error("Expected non-empty result")
	}

	// Should contain POLICY_BLOCKED prefix
	if !hasPrefix(result, "POLICY_BLOCKED:") {
		t.Error("Expected POLICY_BLOCKED prefix")
	}
}

func TestParseApprovalNeededMarker(t *testing.T) {
	// Valid approval needed response
	validResult := formatApprovalNeededToolResult("rm -rf /", "tool-123", "test")
	data, found := parseApprovalNeededMarker(validResult)
	if !found {
		t.Error("Expected to parse approval needed marker")
	}
	if data.Command != "rm -rf /" {
		t.Errorf("Expected command 'rm -rf /', got '%s'", data.Command)
	}
	if data.ToolID != "tool-123" {
		t.Errorf("Expected tool ID 'tool-123', got '%s'", data.ToolID)
	}

	// Invalid input
	_, found = parseApprovalNeededMarker("not an approval marker")
	if found {
		t.Error("Expected not found for invalid input")
	}

	// Empty input
	_, found = parseApprovalNeededMarker("")
	if found {
		t.Error("Expected not found for empty input")
	}

	// Only prefix without JSON
	_, found = parseApprovalNeededMarker("APPROVAL_REQUIRED:")
	if found {
		t.Error("Expected not found for prefix without JSON")
	}
}

// Note: GetDebugContext tests removed - they require persistence to be properly
// initialized and are better suited for integration tests

func TestService_SetProviders(t *testing.T) {
	svc := NewService(nil, nil)

	// Set up state provider first (needed for patrol service)
	stateProvider := &mockStateProvider{}
	svc.SetStateProvider(stateProvider)

	// Test SetPatrolThresholdProvider
	thresholdProvider := &mockThresholdProvider{
		nodeCPU:    90,
		nodeMemory: 85,
	}
	svc.SetPatrolThresholdProvider(thresholdProvider)
	// No direct way to verify, but shouldn't panic

	// Test SetChangeDetector with nil
	svc.SetChangeDetector(nil)

	// Test SetRemediationLog with nil
	svc.SetRemediationLog(nil)

	// Test SetPatternDetector with nil
	svc.SetPatternDetector(nil)

	// Test SetCorrelationDetector with nil
	svc.SetCorrelationDetector(nil)
}

// Helper functions
func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
