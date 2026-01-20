package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Mock implementations for testing

type mockStateProvider struct {
	state models.StateSnapshot
}

func (m *mockStateProvider) GetState() models.StateSnapshot {
	return m.state
}

type mockCommandPolicy struct {
	decisions map[string]agentexec.PolicyDecision
}

func (m *mockCommandPolicy) Evaluate(command string) agentexec.PolicyDecision {
	if decision, ok := m.decisions[command]; ok {
		return decision
	}
	return agentexec.PolicyAllow
}

type mockAgentServer struct {
	agents   []agentexec.ConnectedAgent
	execFunc func(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error)
}

func (m *mockAgentServer) GetConnectedAgents() []agentexec.ConnectedAgent {
	return m.agents
}

func (m *mockAgentServer) ExecuteCommand(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
	if m.execFunc != nil {
		return m.execFunc(ctx, agentID, cmd)
	}
	return &agentexec.CommandResultPayload{
		Stdout:   "mock output",
		ExitCode: 0,
	}, nil
}

type mockMetricsHistoryProvider struct {
	metrics map[string][]MetricPoint
	summary map[string]ResourceMetricsSummary
}

func (m *mockMetricsHistoryProvider) GetResourceMetrics(resourceID string, period time.Duration) ([]MetricPoint, error) {
	if metrics, ok := m.metrics[resourceID]; ok {
		return metrics, nil
	}
	return nil, nil
}

func (m *mockMetricsHistoryProvider) GetAllMetricsSummary(period time.Duration) (map[string]ResourceMetricsSummary, error) {
	return m.summary, nil
}

type mockAlertProvider struct {
	alerts []ActiveAlert
}

func (m *mockAlertProvider) GetActiveAlerts() []ActiveAlert {
	return m.alerts
}

type mockFindingsProvider struct {
	active    []Finding
	dismissed []Finding
}

func (m *mockFindingsProvider) GetActiveFindings() []Finding {
	return m.active
}

func (m *mockFindingsProvider) GetDismissedFindings() []Finding {
	return m.dismissed
}

// Tests

func TestNewPulseToolExecutor(t *testing.T) {
	cfg := ExecutorConfig{
		StateProvider: &mockStateProvider{},
		ControlLevel:  ControlLevelReadOnly,
	}

	executor := NewPulseToolExecutor(cfg)
	if executor == nil {
		t.Fatal("expected executor to be created")
	}

	if executor.controlLevel != ControlLevelReadOnly {
		t.Errorf("expected control level %s, got %s", ControlLevelReadOnly, executor.controlLevel)
	}
}

func TestListToolsReadOnlyMode(t *testing.T) {
	cfg := ExecutorConfig{
		StateProvider: &mockStateProvider{},
		ControlLevel:  ControlLevelReadOnly,
	}

	executor := NewPulseToolExecutor(cfg)
	tools := executor.ListTools()

	// Should not include control tools in read-only mode
	for _, tool := range tools {
		if tool.Name == "pulse_run_command" ||
			tool.Name == "pulse_control_guest" ||
			tool.Name == "pulse_control_docker" {
			t.Errorf("control tool %s should not be available in read-only mode", tool.Name)
		}
	}
}

func TestListToolsControlledMode(t *testing.T) {
	cfg := ExecutorConfig{
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
		ControlLevel:  ControlLevelControlled,
	}

	executor := NewPulseToolExecutor(cfg)
	tools := executor.ListTools()

	// Should include control tools
	controlToolsFound := make(map[string]bool)
	for _, tool := range tools {
		controlToolsFound[tool.Name] = true
	}

	if !controlToolsFound["pulse_run_command"] {
		t.Error("expected pulse_run_command in controlled mode")
	}
	if !controlToolsFound["pulse_control_guest"] {
		t.Error("expected pulse_control_guest in controlled mode")
	}
	if !controlToolsFound["pulse_control_docker"] {
		t.Error("expected pulse_control_docker in controlled mode")
	}
}

func TestExecuteToolUnknown(t *testing.T) {
	cfg := ExecutorConfig{
		StateProvider: &mockStateProvider{},
	}

	executor := NewPulseToolExecutor(cfg)
	result, err := executor.ExecuteTool(context.Background(), "unknown_tool", nil)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error result for unknown tool")
	}
}

func TestExecuteGetInfrastructureState(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{Name: "pve1", Status: "online"},
		},
		VMs: []models.VM{
			{Name: "test-vm", VMID: 100, Status: "running", Node: "pve1"},
		},
		Containers: []models.Container{
			{Name: "test-ct", VMID: 101, Status: "running", Node: "pve1"},
		},
	}

	cfg := ExecutorConfig{
		StateProvider: &mockStateProvider{state: state},
	}

	executor := NewPulseToolExecutor(cfg)
	result, err := executor.ExecuteTool(context.Background(), "pulse_get_topology", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected successful result")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	var response TopologyResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if response.Summary.TotalNodes != 1 || response.Summary.TotalVMs != 1 || response.Summary.TotalLXCContainers != 1 {
		t.Fatalf("unexpected summary totals: %+v", response.Summary)
	}
	if len(response.Proxmox.Nodes) != 1 || response.Proxmox.Nodes[0].Name != "pve1" {
		t.Fatalf("expected node pve1, got %+v", response.Proxmox.Nodes)
	}
	if len(response.Proxmox.Nodes[0].VMs) != 1 || response.Proxmox.Nodes[0].VMs[0].Name != "test-vm" {
		t.Fatalf("expected VM test-vm, got %+v", response.Proxmox.Nodes[0].VMs)
	}
	if len(response.Proxmox.Nodes[0].Containers) != 1 || response.Proxmox.Nodes[0].Containers[0].Name != "test-ct" {
		t.Fatalf("expected container test-ct, got %+v", response.Proxmox.Nodes[0].Containers)
	}
}

func TestExecuteRunCommandReadOnly(t *testing.T) {
	cfg := ExecutorConfig{
		StateProvider: &mockStateProvider{},
		ControlLevel:  ControlLevelReadOnly,
	}

	executor := NewPulseToolExecutor(cfg)
	result, err := executor.ExecuteTool(context.Background(), "pulse_run_command", map[string]interface{}{
		"command": "ls -la",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].Text
	if !contains(text, "Control tools are disabled") {
		t.Error("expected disabled message in read-only mode")
	}
}

func TestExecuteRunCommandSuggestMode(t *testing.T) {
	cfg := ExecutorConfig{
		StateProvider: &mockStateProvider{},
		ControlLevel:  ControlLevelSuggest,
	}

	executor := NewPulseToolExecutor(cfg)
	result, err := executor.ExecuteTool(context.Background(), "pulse_run_command", map[string]interface{}{
		"command": "ls -la",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].Text
	if !contains(text, "Suggested command") {
		t.Error("expected suggestion in suggest mode")
	}
	if !contains(text, "ls -la") {
		t.Error("expected command in suggestion")
	}
}

func TestExecuteRunCommandControlled(t *testing.T) {
	cfg := ExecutorConfig{
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
		ControlLevel:  ControlLevelControlled,
	}

	executor := NewPulseToolExecutor(cfg)
	result, err := executor.ExecuteTool(context.Background(), "pulse_run_command", map[string]interface{}{
		"command": "ls -la",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].Text
	if !contains(text, "APPROVAL_REQUIRED") {
		t.Error("expected approval required in controlled mode")
	}
}

func TestExecuteRunCommandPolicyBlocked(t *testing.T) {
	cfg := ExecutorConfig{
		StateProvider: &mockStateProvider{},
		Policy: &mockCommandPolicy{
			decisions: map[string]agentexec.PolicyDecision{
				"rm -rf /": agentexec.PolicyBlock,
			},
		},
		ControlLevel: ControlLevelAutonomous,
	}

	executor := NewPulseToolExecutor(cfg)
	result, err := executor.ExecuteTool(context.Background(), "pulse_run_command", map[string]interface{}{
		"command": "rm -rf /",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].Text
	if !contains(text, "POLICY_BLOCKED") {
		t.Error("expected policy blocked message")
	}
}

func TestExecuteGetActiveAlerts(t *testing.T) {
	alerts := []ActiveAlert{
		{
			ID:           "alert-1",
			ResourceName: "test-vm",
			Type:         "cpu",
			Severity:     "warning",
			Value:        95.0,
			Threshold:    90.0,
			Message:      "CPU high",
		},
	}

	cfg := ExecutorConfig{
		StateProvider: &mockStateProvider{},
		AlertProvider: &mockAlertProvider{alerts: alerts},
	}

	executor := NewPulseToolExecutor(cfg)
	result, err := executor.ExecuteTool(context.Background(), "pulse_list_alerts", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected successful result")
	}

	var response AlertsResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if response.Count != 1 || len(response.Alerts) != 1 {
		t.Fatalf("expected 1 alert, got %+v", response)
	}
	if response.Alerts[0].ResourceName != "test-vm" || response.Alerts[0].Value != 95.0 {
		t.Fatalf("unexpected alert: %+v", response.Alerts[0])
	}
}

func TestExecuteGetFindingsWithDismissed(t *testing.T) {
	active := []Finding{
		{ID: "finding-1", Title: "Active Issue", Severity: "warning"},
	}
	dismissed := []Finding{
		{ID: "finding-2", Title: "Dismissed Issue", Severity: "info"},
	}

	cfg := ExecutorConfig{
		StateProvider:    &mockStateProvider{},
		FindingsProvider: &mockFindingsProvider{active: active, dismissed: dismissed},
	}

	executor := NewPulseToolExecutor(cfg)

	// Without dismissed
	result, _ := executor.ExecuteTool(context.Background(), "pulse_list_findings", map[string]interface{}{
		"include_dismissed": false,
	})
	var response FindingsResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(response.Active) != 1 || response.Active[0].Title != "Active Issue" {
		t.Fatalf("expected active finding, got %+v", response.Active)
	}
	if len(response.Dismissed) != 0 {
		t.Fatalf("expected no dismissed findings, got %+v", response.Dismissed)
	}

	// With dismissed
	result, _ = executor.ExecuteTool(context.Background(), "pulse_list_findings", map[string]interface{}{
		"include_dismissed": true,
	})
	response = FindingsResponse{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(response.Dismissed) != 1 || response.Dismissed[0].Title != "Dismissed Issue" {
		t.Fatalf("expected dismissed findings, got %+v", response.Dismissed)
	}
}

func TestControlGuestProtectedGuest(t *testing.T) {
	state := models.StateSnapshot{
		VMs: []models.VM{
			{Name: "protected-vm", VMID: 100, Status: "running", Node: "pve1"},
		},
	}

	cfg := ExecutorConfig{
		StateProvider:   &mockStateProvider{state: state},
		ControlLevel:    ControlLevelAutonomous,
		ProtectedGuests: []string{"100"},
	}

	executor := NewPulseToolExecutor(cfg)
	result, err := executor.ExecuteTool(context.Background(), "pulse_control_guest", map[string]interface{}{
		"guest_id": "100",
		"action":   "stop",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].Text
	if !contains(text, "protected") {
		t.Error("expected protected guest message")
	}
}

func TestSetControlLevelRuntime(t *testing.T) {
	cfg := ExecutorConfig{
		StateProvider: &mockStateProvider{},
		AgentServer:   &mockAgentServer{},
		ControlLevel:  ControlLevelReadOnly,
	}

	executor := NewPulseToolExecutor(cfg)

	// Initially read-only
	tools := executor.ListTools()
	for _, tool := range tools {
		if tool.Name == "pulse_run_command" {
			t.Error("pulse_run_command should not be available in read-only mode")
		}
	}

	// Update to controlled
	executor.SetControlLevel(ControlLevelControlled)
	tools = executor.ListTools()
	found := false
	for _, tool := range tools {
		if tool.Name == "pulse_run_command" {
			found = true
			break
		}
	}
	if !found {
		t.Error("pulse_run_command should be available after setting controlled mode")
	}
}

func TestExecuteGetAgentScope(t *testing.T) {
	state := models.StateSnapshot{
		Hosts: []models.Host{
			{ID: "agent-123", Hostname: "testhost", DisplayName: "Test Host", CommandsEnabled: true},
		},
	}

	cfg := ExecutorConfig{
		StateProvider: &mockStateProvider{state: state},
	}

	executor := NewPulseToolExecutor(cfg)
	result, err := executor.ExecuteTool(context.Background(), "pulse_get_agent_scope", map[string]interface{}{
		"hostname": "testhost",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := result.Content[0].Text
	if !contains(text, "Test Host") || !contains(text, "agent-123") {
		t.Error("expected agent info in output")
	}
}

func TestToolRegistryOrder(t *testing.T) {
	registry := NewToolRegistry()

	registry.Register(RegisteredTool{
		Definition: Tool{Name: "tool_a"},
		Handler: func(ctx context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return CallToolResult{}, nil
		},
	})
	registry.Register(RegisteredTool{
		Definition: Tool{Name: "tool_b"},
		Handler: func(ctx context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return CallToolResult{}, nil
		},
	})
	registry.Register(RegisteredTool{
		Definition: Tool{Name: "tool_c"},
		Handler: func(ctx context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return CallToolResult{}, nil
		},
	})

	tools := registry.ListTools(ControlLevelAutonomous)

	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}

	// Should preserve registration order
	if tools[0].Name != "tool_a" {
		t.Errorf("expected tool_a first, got %s", tools[0].Name)
	}
	if tools[1].Name != "tool_b" {
		t.Errorf("expected tool_b second, got %s", tools[1].Name)
	}
	if tools[2].Name != "tool_c" {
		t.Errorf("expected tool_c third, got %s", tools[2].Name)
	}
}

func TestToolRegistryControlFiltering(t *testing.T) {
	registry := NewToolRegistry()

	registry.Register(RegisteredTool{
		Definition: Tool{Name: "read_tool"},
		Handler: func(ctx context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return CallToolResult{}, nil
		},
		RequireControl: false,
	})
	registry.Register(RegisteredTool{
		Definition: Tool{Name: "control_tool"},
		Handler: func(ctx context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return CallToolResult{}, nil
		},
		RequireControl: true,
	})

	// Read-only should not include control tool
	tools := registry.ListTools(ControlLevelReadOnly)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool in read-only mode, got %d", len(tools))
	}
	if tools[0].Name != "read_tool" {
		t.Errorf("expected read_tool, got %s", tools[0].Name)
	}

	// Controlled should include both
	tools = registry.ListTools(ControlLevelControlled)
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools in controlled mode, got %d", len(tools))
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
func TestExecutor_Configuration(t *testing.T) {
	cfg := ExecutorConfig{
		StateProvider: &mockStateProvider{},
	}
	executor := NewPulseToolExecutor(cfg)

	// Test all Set* methods
	executor.SetMetadataUpdater(nil)
	executor.SetFindingsManager(nil)
	executor.SetMetricsHistory(&mockMetricsHistoryProvider{})
	executor.SetBaselineProvider(nil)
	executor.SetPatternProvider(nil)
	executor.SetAlertProvider(nil)
	executor.SetFindingsProvider(nil)
	executor.SetBackupProvider(nil)
	executor.SetStorageProvider(nil)
	executor.SetDiskHealthProvider(nil)
	executor.SetAgentProfileManager(nil)
	executor.SetUpdatesProvider(nil)
	executor.SetProtectedGuests([]string{"100", "101"})

	// Verify assignments (indirectly via no panic, and field check if we could access them)
	// Since fields are unexported, we rely on coverage to show the lines execute.
	// But we can check public getters if they existed. ListTools checks SetControlLevel side effect.
	executor.SetControlLevel(ControlLevelAutonomous)
}

func TestExecuteToolErrors(t *testing.T) {
	cfg := ExecutorConfig{
		StateProvider: &mockStateProvider{},
	}
	executor := NewPulseToolExecutor(cfg)

	// Test tool that returns error (handled by registry, but we simulate execution)
	// We need a tool that errors. Let's register one manually for this test.
	executor.registry.Register(RegisteredTool{
		Definition: Tool{Name: "error_tool"},
		Handler: func(ctx context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return CallToolResult{}, context.DeadlineExceeded
		},
	})

	_, err := executor.ExecuteTool(context.Background(), "error_tool", nil)
	if err == nil {
		t.Error("expected error from error_tool")
	}
}
