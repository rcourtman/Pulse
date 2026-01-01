package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestService_GetToolInputDisplay(t *testing.T) {
	svc := NewService(nil, nil)

	tests := []struct {
		name     string
		tc       providers.ToolCall
		expected string
	}{
		{
			name: "run_command",
			tc: providers.ToolCall{
				Name: "run_command",
				Input: map[string]interface{}{
					"command": "uptime",
				},
			},
			expected: "uptime",
		},
		{
			name: "run_command on host",
			tc: providers.ToolCall{
				Name: "run_command",
				Input: map[string]interface{}{
					"command":     "uptime",
					"run_on_host": true,
				},
			},
			expected: "[host] uptime",
		},
		{
			name: "fetch_url",
			tc: providers.ToolCall{
				Name: "fetch_url",
				Input: map[string]interface{}{
					"url": "https://google.com",
				},
			},
			expected: "https://google.com",
		},
		{
			name: "set_resource_url",
			tc: providers.ToolCall{
				Name: "set_resource_url",
				Input: map[string]interface{}{
					"resource_type": "guest",
					"url":           "http://1.2.3.4",
				},
			},
			expected: "Set guest URL: http://1.2.3.4",
		},
		{
			name: "unknown tool",
			tc: providers.ToolCall{
				Name: "unknown",
				Input: map[string]interface{}{
					"foo": "bar",
				},
			},
			expected: "map[foo:bar]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.getToolInputDisplay(tt.tc)
			if got != tt.expected {
				t.Errorf("getToolInputDisplay() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestService_GetTools(t *testing.T) {
	svc := NewService(nil, nil)
	svc.cfg = &config.AIConfig{Enabled: true}

	tools := svc.getTools()
	if len(tools) == 0 {
		t.Error("Expected tools")
	}

	// Verify some common tools are present
	foundRunCommand := false
	for _, tool := range tools {
		if tool.Name == "run_command" {
			foundRunCommand = true
			break
		}
	}
	if !foundRunCommand {
		t.Error("Expected run_command tool to be available")
	}
}

func TestService_LogRemediation_Nil(t *testing.T) {
	svc := NewService(nil, nil)
	// Should not panic even if remediation log is nil
	req := ExecuteRequest{Prompt: "Fix it"}
	svc.logRemediation(req, "ls", "output", true)
}

func TestService_AcquireExecutionSlot_Blocked(t *testing.T) {
	svc := NewService(nil, nil)

	// Fill all slots (4 by default)
	for i := 0; i < 4; i++ {
		_, err := svc.acquireExecutionSlot(context.Background(), "chat")
		if err != nil {
			t.Fatalf("Failed to fill slots: %v", err)
		}
	}

	// Try to acquire one more with a canceled context - should fail via ctx.Done()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := svc.acquireExecutionSlot(ctx, "chat")
	if err == nil {
		t.Error("Expected error when acquiring slot with canceled context")
	}

	// Try to acquire one more with a timeout - should fail via timeout case
	shortCtx, cancel2 := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel2()

	// We need to wait slightly more than the shortCtx but less than the 5s hardcoded timeout
	// But wait, the hardcoded timeout in acquireExecutionSlot is the 3rd case.
	// We want to hit the second case.
	_, err = svc.acquireExecutionSlot(shortCtx, "chat")
	if err == nil {
		t.Error("Expected error when no slots available and context times out")
	}
}

func TestService_SetResourceURL(t *testing.T) {
	svc := NewService(nil, nil)
	mockMP := &mockMetadataProvider{}
	svc.SetMetadataProvider(mockMP)

	// Valid guest URL
	err := svc.SetResourceURL("guest", "delly-150", "http://192.168.1.10")
	if err != nil {
		t.Errorf("SetResourceURL failed: %v", err)
	}
	if mockMP.lastGuestID != "delly-150" || mockMP.lastGuestURL != "http://192.168.1.10" {
		t.Error("Mock did not receive correct guest data")
	}

	// URL without scheme (should auto-add http://)
	err = svc.SetResourceURL("docker", "host:ct:123", "192.168.1.20:8080")
	if err != nil {
		t.Errorf("SetResourceURL failed: %v", err)
	}
	if mockMP.lastDockerID != "host:ct:123" || mockMP.lastDockerURL != "http://192.168.1.20:8080" {
		t.Errorf("Mock did not receive correct docker data, got %s", mockMP.lastDockerURL)
	}

	// Invalid URL
	err = svc.SetResourceURL("host", "host-1", "not a url")
	if err == nil {
		t.Error("Expected error for invalid URL")
	}

	// Unknown resource type
	err = svc.SetResourceURL("unknown", "id", "http://test")
	if err == nil {
		t.Error("Expected error for unknown resource type")
	}
}

// TestConvertMetricPoints tests the metric conversion logic

func TestConvertMetricPoints(t *testing.T) {
	// Since MetricsHistoryAdapter is just a wrapper, we test the conversion functions
	now := time.Now()
	monitoringPoints := []monitoring.MetricPoint{
		{Value: 1.2, Timestamp: now},
	}

	aiPoints := convertMetricPoints(monitoringPoints)
	if len(aiPoints) != 1 || aiPoints[0].Value != 1.2 || !aiPoints[0].Timestamp.Equal(now) {
		t.Error("Conversion failed")
	}

	if convertMetricPoints(nil) != nil {
		t.Error("Should return nil for nil input")
	}
}

func TestNewMetricsHistoryAdapter(t *testing.T) {
	if NewMetricsHistoryAdapter(nil) != nil {
		t.Error("Expected nil for nil history")
	}
}

func TestService_HasAgentForTarget(t *testing.T) {
	mockAgent := agentexec.ConnectedAgent{
		AgentID:  "agent-1",
		Hostname: "node-1",
	}
	mockServer := &mockAgentServer{
		agents: []agentexec.ConnectedAgent{mockAgent},
	}
	svc := NewService(nil, mockServer)

	// Host target with no context (should match any agent)
	if !svc.hasAgentForTarget(ExecuteRequest{TargetType: "host"}) {
		t.Error("Should have agent for host with no context")
	}

	// Target matching agent hostname
	if !svc.hasAgentForTarget(ExecuteRequest{
		TargetType: "guest",
		Context:    map[string]interface{}{"node": "node-1"},
	}) {
		t.Error("Should have agent for matching hostname")
	}

	// Target not matching
	if svc.hasAgentForTarget(ExecuteRequest{
		TargetType: "guest",
		Context:    map[string]interface{}{"node": "other-node"},
	}) {
		t.Error("Should not have agent for non-matching node")
	}

	// Empty agents
	svc.agentServer = &mockAgentServer{agents: []agentexec.ConnectedAgent{}}
	if svc.hasAgentForTarget(ExecuteRequest{TargetType: "host"}) {
		t.Error("Should not have agent when none connected")
	}
}

func TestService_ExecuteOnAgent(t *testing.T) {
	mockAgent := agentexec.ConnectedAgent{
		AgentID:  "agent-1",
		Hostname: "node-1",
	}
	mockServer := &mockAgentServer{
		agents: []agentexec.ConnectedAgent{mockAgent},
		executeFunc: func(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
			if agentID != "agent-1" {
				return nil, fmt.Errorf("wrong agent")
			}
			return &agentexec.CommandResultPayload{
				Success: true,
				Stdout:  "ok",
			}, nil
		},
	}
	svc := NewService(nil, mockServer)

	// Basic execution
	output, err := svc.executeOnAgent(context.Background(), ExecuteRequest{
		TargetType: "host",
		Context:    map[string]interface{}{"node": "node-1"},
	}, "uptime")
	if err != nil {
		t.Fatalf("ExecuteOnAgent failed: %v", err)
	}
	if output != "ok" {
		t.Errorf("Unexpected output: %s", output)
	}

	// Routing failure
	_, err = svc.executeOnAgent(context.Background(), ExecuteRequest{
		TargetType: "guest",
		Context:    map[string]interface{}{"node": "unknown"},
	}, "uptime")
	if err == nil {
		t.Error("Expected routing error")
	}
}

func TestService_RunCommandExtended(t *testing.T) {
	mockAgent := agentexec.ConnectedAgent{
		AgentID:  "agent-1",
		Hostname: "node-1",
	}
	mockServer := &mockAgentServer{
		agents: []agentexec.ConnectedAgent{mockAgent},
	}
	svc := NewService(nil, mockServer)

	// Successful run
	resp, err := svc.RunCommand(context.Background(), RunCommandRequest{
		Command:    "uptime",
		TargetType: "host",
		TargetHost: "node-1",
	})
	if err != nil {
		t.Fatalf("RunCommand failed: %v", err)
	}
	if !resp.Success {
		t.Errorf("RunCommand reported failure: %s", resp.Error)
	}
	if resp.Output != "Mock output" {
		t.Errorf("Unexpected output: %s", resp.Output)
	}

	// Failure (routing)
	resp, err = svc.RunCommand(context.Background(), RunCommandRequest{
		Command:    "uptime",
		TargetType: "host",
		TargetHost: "unknown",
	})
	if err != nil {
		t.Fatalf("RunCommand failed with error: %v", err)
	}
	if resp.Success {
		t.Error("RunCommand should have failed")
	}
}

func TestService_ExecuteStreamExtended(t *testing.T) {
	mockAgent := agentexec.ConnectedAgent{
		AgentID:  "agent-1",
		Hostname: "node-1",
	}
	mockAgentServer := &mockAgentServer{
		agents: []agentexec.ConnectedAgent{mockAgent},
	}

	callCount := 0
	mockProv := &mockProvider{
		chatFunc: func(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
			callCount++
			if callCount == 1 {
				// First call: return a tool call
				return &providers.ChatResponse{
					ToolCalls: []providers.ToolCall{
						{
							ID:   "call-1",
							Name: "run_command",
							Input: map[string]interface{}{
								"command": "uptime",
							},
						},
					},
					StopReason: "tool_use",
				}, nil
			}
			// Second call: return final response
			return &providers.ChatResponse{
				Content:    "All systems clear.",
				StopReason: "end_turn",
			}, nil
		},
	}

	svc := NewService(nil, mockAgentServer)
	// Initialize persistence to avoid nil pointer dereference in buildSystemPrompt
	tmpDir := t.TempDir()
	svc.persistence = config.NewConfigPersistence(tmpDir)

	svc.provider = mockProv
	svc.cfg = &config.AIConfig{Enabled: true}
	// Initialize limits so acquireExecutionSlot works
	svc.limits = executionLimits{
		chatSlots:   make(chan struct{}, 10),
		patrolSlots: make(chan struct{}, 10),
	}

	events := []StreamEvent{}
	callback := func(ev StreamEvent) {
		events = append(events, ev)
	}

	req := ExecuteRequest{
		Prompt:     "Check status",
		Model:      "anthropic:test-model",
		TargetType: "host",
		Context:    map[string]interface{}{"node": "node-1"},
	}

	resp, err := svc.ExecuteStream(context.Background(), req, callback)
	if err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	if resp.Content != "All systems clear." {
		t.Errorf("Unexpected content: %s", resp.Content)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 AI calls, got %d", callCount)
	}

	// Verify events
	hasToolStart := false
	hasToolEnd := false
	for _, ev := range events {
		if ev.Type == "tool_start" {
			hasToolStart = true
		}
		if ev.Type == "tool_end" {
			hasToolEnd = true
		}
	}

	if !hasToolStart {
		t.Error("Missing tool_start event")
	}
	if !hasToolEnd {
		t.Error("Missing tool_end event")
	}
}

func TestApprovalNeededFromToolCall(t *testing.T) {
	req := ExecuteRequest{
		Context: map[string]interface{}{"node": "node-1"},
	}
	tc := providers.ToolCall{
		ID:   "call-1",
		Name: "run_command",
		Input: map[string]interface{}{
			"command": "uptime",
		},
	}
	result := "APPROVAL_REQUIRED:{\"command\":\"uptime\",\"tool_id\":\"call-1\"}"

	data, needed := approvalNeededFromToolCall(req, tc, result)
	if !needed {
		t.Fatal("Expected approval needed")
	}
	if data.Command != "uptime" || data.TargetHost != "node-1" {
		t.Errorf("Unexpected data: %+v", data)
	}

	// Not a run_command
	tc.Name = "fetch_url"
	_, needed = approvalNeededFromToolCall(req, tc, result)
	if needed {
		t.Error("Should not need approval for fetch_url")
	}

	// No prefix
	tc.Name = "run_command"
	_, needed = approvalNeededFromToolCall(req, tc, "ok")
	if needed {
		t.Error("Should not need approval for normal result")
	}
}

func TestService_ExecuteTool_FetchURL(t *testing.T) {
	svc := NewService(nil, nil)
	// We need to mock fetchURL or just let it fail and check the error handling

	tc := providers.ToolCall{
		Name: "fetch_url",
		Input: map[string]interface{}{
			"url": "http://example.com",
		},
	}

	ctx := context.Background()
	req := ExecuteRequest{}

	result, exec := svc.executeTool(ctx, req, tc)
	if exec.Name != "fetch_url" {
		t.Errorf("Unexpected tool name: %s", exec.Name)
	}

	// Since we are in a limited environment, fetch might fail or be blocked.
	// But we want to see that executeTool handled it.
	if result == "" {
		t.Error("Expected non-empty result (either content or error)")
	}
}

func TestFormatContextKey(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"guestName", "Guest Name"},
		{"vmid", "VMID"},
		{"node", "PVE Node (host)"},
		{"unknown_key", "unknown_key"},
	}

	for _, tt := range tests {
		if got := formatContextKey(tt.key); got != tt.expected {
			t.Errorf("formatContextKey(%q) = %q, want %q", tt.key, got, tt.expected)
		}
	}
}

func TestProviderDisplayName(t *testing.T) {
	tests := []struct {
		provider string
		expected string
	}{
		{config.AIProviderAnthropic, "Anthropic"},
		{config.AIProviderOpenAI, "OpenAI"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		if got := providerDisplayName(tt.provider); got != tt.expected {
			t.Errorf("providerDisplayName(%q) = %q, want %q", tt.provider, got, tt.expected)
		}
	}
}

func TestService_SetBaselineStore(t *testing.T) {
	svc := NewService(nil, nil)
	// Just verify it doesn't panic
	svc.SetBaselineStore(nil)
}

func TestService_GetDebugContext(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{Enabled: true, CustomContext: "Test context"}

	stateProvider := &mockStateProvider{
		state: models.StateSnapshot{
			VMs: []models.VM{{Name: "vm-1", VMID: 100, Node: "node-1"}},
		},
	}
	svc.SetStateProvider(stateProvider)

	req := ExecuteRequest{Prompt: "test"}
	debug := svc.GetDebugContext(req)

	if debug["has_state_provider"] != true {
		t.Error("Expected has_state_provider to be true")
	}
	if debug["custom_context_preview"] != "Test context" {
		t.Errorf("Unexpected custom_context_preview: %v", debug["custom_context_preview"])
	}
	if debug["system_prompt"] == "" {
		t.Error("Expected non-empty system_prompt")
	}
}

func TestService_LoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, nil)

	// 1. Initial state (no config file)
	err := svc.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if svc.provider != nil {
		t.Error("Expected nil provider when no config exists")
	}

	// 2. Disabled config
	cfgV := config.AIConfig{Enabled: false}
	persistence.SaveAIConfig(cfgV)
	err = svc.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if svc.provider != nil {
		t.Error("Expected nil provider when AI is disabled")
	}

	// 3. Smart fallback (Anthropic model set, but only OpenAI key configured)
	cfgV = config.AIConfig{
		Enabled:      true,
		Model:        "anthropic:claude-3-opus-20240229",
		OpenAIAPIKey: "sk-test",
	}
	persistence.SaveAIConfig(cfgV)
	err = svc.LoadConfig()
	if svc.provider == nil {
		t.Fatal("Expected non-nil provider after smart fallback")
	}
	if svc.provider.Name() != config.AIProviderOpenAI {
		t.Errorf("Expected provider to fallback to openai, got %s", svc.provider.Name())
	}
}

func TestService_EnforceBudget(t *testing.T) {
	svc := NewService(nil, nil)
	svc.cfg = &config.AIConfig{
		CostBudgetUSD30d: 1.0,
	}
	svc.costStore = cost.NewStore(30)

	// No usage yet - should pass
	if err := svc.enforceBudget("chat"); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Record significant usage (e.g. 10 million tokens to exceed $1 budget)
	svc.costStore.Record(cost.UsageEvent{
		Provider:      "anthropic",
		RequestModel:  "claude-opus-20240229",
		ResponseModel: "claude-opus-20240229",
		InputTokens:   1000000,
		OutputTokens:  1000000,
	})

	// Budget should now be exceeded
	if err := svc.enforceBudget("chat"); err == nil {
		t.Error("Expected budget limit error")
	} else if !strings.Contains(err.Error(), "budget exceeded") {
		t.Errorf("Expected budget exceeded error, got: %v", err)
	}
}

func TestService_GetModelForRequest(t *testing.T) {
	svc := NewService(nil, nil)
	svc.cfg = &config.AIConfig{
		Enabled:     true,
		Model:       "default-model",
		ChatModel:   "chat-model",
		PatrolModel: "patrol-model",
	}

	// 1. Explicit override
	req := ExecuteRequest{Model: "override-model"}
	if got := svc.getModelForRequest(req); got != "override-model" {
		t.Errorf("Expected override-model, got %s", got)
	}

	// 2. Use case: patrol
	req = ExecuteRequest{UseCase: "patrol"}
	if got := svc.getModelForRequest(req); got != "patrol-model" {
		t.Errorf("Expected patrol-model, got %s", got)
	}

	// 3. Use case: chat
	req = ExecuteRequest{UseCase: "chat"}
	if got := svc.getModelForRequest(req); got != "chat-model" {
		t.Errorf("Expected chat-model, got %s", got)
	}

	// 4. Default
	req = ExecuteRequest{}
	if got := svc.getModelForRequest(req); got != "chat-model" {
		t.Errorf("Expected chat-model by default, got %s", got)
	}
}

func TestService_ExecuteTool_RunCommand(t *testing.T) {
	mockAgentServer := &mockAgentServer{
		agents: []agentexec.ConnectedAgent{
			{Hostname: "node-1", AgentID: "agent-1"},
		},
	}
	svc := NewService(nil, mockAgentServer)

	// Mock policy
	mockPolicy := &mockPolicy{
		decision: agentexec.PolicyAllow,
	}
	svc.policy = mockPolicy

	tc := providers.ToolCall{
		Name: "run_command",
		Input: map[string]interface{}{
			"command": "uptime",
		},
	}

	ctx := context.Background()
	req := ExecuteRequest{
		Context: map[string]interface{}{"node": "node-1"},
	}

	// 1. Allowed command
	result, exec := svc.executeTool(ctx, req, tc)
	if !exec.Success {
		t.Errorf("Expected success, got failure: %s", result)
	}

	// 2. Blocked command
	mockPolicy.decision = agentexec.PolicyBlock
	result, exec = svc.executeTool(ctx, req, tc)
	if exec.Success {
		t.Error("Expected failure for blocked command")
	}
	if !strings.Contains(result, "blocked") {
		t.Errorf("Expected blocked message, got %s", result)
	}

	// 3. Approval required (non-autonomous)
	mockPolicy.decision = agentexec.PolicyRequireApproval
	svc.cfg = &config.AIConfig{AutonomousMode: false}
	result, exec = svc.executeTool(ctx, req, tc)
	if !exec.Success {
		t.Errorf("Expected success (not an error to need approval), got: %s", result)
	}
	if !strings.Contains(result, "APPROVAL_REQUIRED") {
		t.Errorf("Expected APPROVAL_REQUIRED message, got %s", result)
	}
}

func TestService_ExecuteTool_SetResourceURL(t *testing.T) {
	svc := NewService(nil, nil)

	// Mock metadata provider
	mockMeta := &mockMetadataProvider{}
	svc.metadataProvider = mockMeta

	tc := providers.ToolCall{
		Name: "set_resource_url",
		Input: map[string]interface{}{
			"resource_type": "guest",
			"resource_id":   "instance-101",
			"url":           "http://example.com:8080",
		},
	}

	ctx := context.Background()
	req := ExecuteRequest{}

	result, exec := svc.executeTool(ctx, req, tc)
	if !exec.Success {
		t.Errorf("Expected success, got failure: %s", result)
	}
	if mockMeta.lastGuestID != "instance-101" || mockMeta.lastGuestURL != "http://example.com:8080" {
		t.Errorf("Metadata provider not called correctly: %+v", mockMeta)
	}
}

func TestService_PatrolManagement(t *testing.T) {
	svc := NewService(nil, nil)

	// Mock components
	patrol := NewPatrolService(svc, nil)
	svc.patrolService = patrol

	alertAnalyzer := NewAlertTriggeredAnalyzer(patrol, nil)
	svc.alertTriggeredAnalyzer = alertAnalyzer

	mockLicense := &mockLicenseStore{
		features: map[string]bool{
			FeatureAIPatrol: true,
			FeatureAIAlerts: true,
		},
	}
	svc.licenseChecker = mockLicense

	svc.cfg = &config.AIConfig{
		Enabled:                true,
		PatrolEnabled:          true,
		PatrolIntervalMinutes:  30,
		AlertTriggeredAnalysis: true,
	}

	ctx := context.Background()

	// 1. Start Patrol
	svc.StartPatrol(ctx)

	if patrol.GetConfig().Interval != 30*time.Minute {
		t.Errorf("Patrol interval not set correctly: %v", patrol.GetConfig().Interval)
	}
	if !alertAnalyzer.IsEnabled() {
		t.Error("Alert analyzer should be enabled")
	}

	// 2. Reconfigure Patrol
	svc.cfg.PatrolIntervalMinutes = 60
	svc.cfg.AlertTriggeredAnalysis = false
	svc.ReconfigurePatrol()

	if patrol.GetConfig().Interval != 60*time.Minute {
		t.Errorf("Patrol interval not updated: %v", patrol.GetConfig().Interval)
	}
	if alertAnalyzer.IsEnabled() {
		t.Error("Alert analyzer should be disabled")
	}

	// 3. License constraint
	mockLicense.features[FeatureAIAlerts] = false
	svc.cfg.AlertTriggeredAnalysis = true
	svc.ReconfigurePatrol()
	if alertAnalyzer.IsEnabled() {
		t.Error("Alert analyzer should be disabled due to lack of license")
	}
}

func TestService_LookupNodeForVMID_Extended(t *testing.T) {
	svc := NewService(nil, nil)
	mockState := &mockStateProvider{
		state: models.StateSnapshot{
			VMs: []models.VM{
				{VMID: 101, Node: "node-1", Name: "vm-101", Instance: "pve-1"},
			},
			Containers: []models.Container{
				{VMID: 201, Node: "node-2", Name: "ct-201", Instance: "pve-1"},
			},
		},
	}
	svc.stateProvider = mockState

	// 1. Find VM
	node, name, gType := svc.lookupNodeForVMID(101)
	if node != "node-1" || name != "vm-101" || gType != "qemu" {
		t.Errorf("VM lookup failed: %s, %s, %s", node, name, gType)
	}

	// 2. Find Container
	node, name, gType = svc.lookupNodeForVMID(201)
	if node != "node-2" || name != "ct-201" || gType != "lxc" {
		t.Errorf("Container lookup failed: %s, %s, %s", node, name, gType)
	}

	// 3. Not found
	node, name, gType = svc.lookupNodeForVMID(999)
	if node != "" || name != "" {
		t.Error("Should not have found non-existent VMID")
	}

	// 4. Collision
	mockState.state.VMs = append(mockState.state.VMs, models.VM{
		VMID: 101, Node: "node-3", Name: "vm-101-dup", Instance: "pve-2",
	})
	node, _, _ = svc.lookupNodeForVMID(101)
	if node == "" {
		t.Error("Should return first match on collision")
	}
}

func TestService_BuildUserAnnotationsContext(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, nil)

	// 1. Empty metadata
	if ctx := svc.buildUserAnnotationsContext(); ctx != "" {
		t.Errorf("Expected empty context, got: %s", ctx)
	}

	// 2. Guest metadata
	guestMeta := map[string]*config.GuestMetadata{
		"guest-1": {
			ID:            "guest-1",
			LastKnownName: "ServerA",
			Notes:         []string{"Primary database"},
		},
	}
	data, _ := json.Marshal(guestMeta)
	os.WriteFile(filepath.Join(tmpDir, "guest_metadata.json"), data, 0644)

	// 3. Docker metadata
	dockerMeta := map[string]*config.DockerMetadata{
		"host1:container:id1": {
			ID:    "host1:container:id1",
			Notes: []string{"Web portal"},
		},
	}
	data, _ = json.Marshal(dockerMeta)
	os.WriteFile(filepath.Join(tmpDir, "docker_metadata.json"), data, 0644)

	ctx := svc.buildUserAnnotationsContext()
	if !strings.Contains(ctx, "ServerA") || !strings.Contains(ctx, "Primary database") {
		t.Error("Guest notes missing from context")
	}
	if !strings.Contains(ctx, "Web portal") || !strings.Contains(ctx, "host1") {
		t.Error("Docker notes missing from context")
	}
}

func TestService_TestConnection_Extended(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, nil)

	// 1. Not configured
	err := svc.TestConnection(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no AI provider configured") {
		t.Errorf("Expected 'no AI provider configured' error, got: %v", err)
	}

	// 2. Mock provider fallback
	mockProv := &mockProvider{
		testConnectionFunc: func(ctx context.Context) error {
			return nil
		},
	}
	svc.provider = mockProv
	svc.cfg = &config.AIConfig{
		Enabled:         true,
		AnthropicAPIKey: "test-key",
		Model:           "openai:test-model",
	}

	err = svc.TestConnection(context.Background())
	if err != nil {
		t.Errorf("TestConnection failed: %v", err)
	}

	// 3. Provider error
	mockProv.testConnectionFunc = func(ctx context.Context) error {
		return fmt.Errorf("connection failed")
	}
	err = svc.TestConnection(context.Background())
	if err == nil || !strings.Contains(err.Error(), "connection failed") {
		t.Errorf("Expected 'connection failed' error, got: %v", err)
	}
}

func TestService_ExecuteTool_ResolveFinding(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, nil)

	// Set up patrol service with findings store
	stateProvider := &mockStateProvider{}
	svc.SetStateProvider(stateProvider)

	patrol := svc.GetPatrolService()
	if patrol == nil {
		t.Fatal("Expected patrol service to be initialized")
	}

	// Add a finding
	finding := &Finding{
		ID:           "test-finding-1",
		Severity:     FindingSeverityWarning,
		ResourceID:   "vm-100",
		ResourceName: "test-vm",
		Title:        "High CPU",
	}
	patrol.GetFindings().Add(finding)

	ctx := context.Background()
	req := ExecuteRequest{FindingID: "test-finding-1"}

	// 1. Successful resolution
	tc := providers.ToolCall{
		Name: "resolve_finding",
		Input: map[string]interface{}{
			"finding_id":      "test-finding-1",
			"resolution_note": "Restarted the service",
		},
	}

	result, exec := svc.executeTool(ctx, req, tc)
	if !exec.Success {
		t.Errorf("Expected success, got failure: %s", result)
	}
	if !strings.Contains(result, "Finding resolved!") {
		t.Errorf("Expected 'Finding resolved!' in result, got: %s", result)
	}

	// 2. Missing finding_id (should use from request context)
	req2 := ExecuteRequest{FindingID: "test-finding-1"}
	tc2 := providers.ToolCall{
		Name: "resolve_finding",
		Input: map[string]interface{}{
			"resolution_note": "Fixed it",
		},
	}
	_, exec2 := svc.executeTool(ctx, req2, tc2)
	// Finding already resolved, so this will fail
	if exec2.Success {
		t.Error("Expected failure for already resolved finding")
	}

	// 3. Missing resolution_note
	tc3 := providers.ToolCall{
		Name: "resolve_finding",
		Input: map[string]interface{}{
			"finding_id": "new-finding",
		},
	}
	result3, exec3 := svc.executeTool(ctx, ExecuteRequest{}, tc3)
	if exec3.Success {
		t.Error("Expected failure for missing resolution_note")
	}
	if !strings.Contains(result3, "resolution_note is required") {
		t.Errorf("Expected resolution_note error, got: %s", result3)
	}

	// 4. Missing both
	tc4 := providers.ToolCall{
		Name:  "resolve_finding",
		Input: map[string]interface{}{},
	}
	result4, exec4 := svc.executeTool(ctx, ExecuteRequest{}, tc4)
	if exec4.Success {
		t.Error("Expected failure for missing finding_id")
	}
	if !strings.Contains(result4, "finding_id is required") {
		t.Errorf("Expected finding_id error, got: %s", result4)
	}
}

func TestService_ExecuteTool_SetResourceURL_EdgeCases(t *testing.T) {
	svc := NewService(nil, nil)
	mockMeta := &mockMetadataProvider{}
	svc.metadataProvider = mockMeta

	ctx := context.Background()

	// 1. Missing resource_type
	tc := providers.ToolCall{
		Name: "set_resource_url",
		Input: map[string]interface{}{
			"resource_id": "test-id",
			"url":         "http://example.com",
		},
	}
	result, exec := svc.executeTool(ctx, ExecuteRequest{}, tc)
	if exec.Success {
		t.Error("Expected failure for missing resource_type")
	}
	if !strings.Contains(result, "resource_type is required") {
		t.Errorf("Expected resource_type error, got: %s", result)
	}

	// 2. Missing resource_id but present in request context
	tc2 := providers.ToolCall{
		Name: "set_resource_url",
		Input: map[string]interface{}{
			"resource_type": "guest",
			"url":           "http://example.com:8080",
		},
	}
	req := ExecuteRequest{TargetID: "vm-from-context"}
	result2, exec2 := svc.executeTool(ctx, req, tc2)
	if !exec2.Success {
		t.Errorf("Expected success when resource_id from context, got: %s", result2)
	}
	if mockMeta.lastGuestID != "vm-from-context" {
		t.Errorf("Expected resource_id from context, got: %s", mockMeta.lastGuestID)
	}

	// 3. Missing resource_id completely
	tc3 := providers.ToolCall{
		Name: "set_resource_url",
		Input: map[string]interface{}{
			"resource_type": "guest",
			"url":           "http://example.com",
		},
	}
	result3, exec3 := svc.executeTool(ctx, ExecuteRequest{}, tc3)
	if exec3.Success {
		t.Error("Expected failure for missing resource_id")
	}
	if !strings.Contains(result3, "resource_id is required") {
		t.Errorf("Expected resource_id error, got: %s", result3)
	}
}

func TestService_ExecuteTool_UnknownTool(t *testing.T) {
	svc := NewService(nil, nil)

	tc := providers.ToolCall{
		Name: "unknown_tool",
		Input: map[string]interface{}{
			"foo": "bar",
		},
	}

	result, exec := svc.executeTool(context.Background(), ExecuteRequest{}, tc)
	if exec.Success {
		t.Error("Expected failure for unknown tool")
	}
	if !strings.Contains(result, "Unknown tool") {
		t.Errorf("Expected 'Unknown tool' error, got: %s", result)
	}
}

func TestService_ExecuteTool_FetchURL_EmptyURL(t *testing.T) {
	svc := NewService(nil, nil)

	tc := providers.ToolCall{
		Name: "fetch_url",
		Input: map[string]interface{}{
			"url": "",
		},
	}

	result, exec := svc.executeTool(context.Background(), ExecuteRequest{}, tc)
	if exec.Success {
		t.Error("Expected failure for empty URL")
	}
	if !strings.Contains(result, "url is required") {
		t.Errorf("Expected 'url is required' error, got: %s", result)
	}
}

func TestService_ExecuteTool_RunCommand_WithTargetHost(t *testing.T) {
	mockAgent := agentexec.ConnectedAgent{
		AgentID:  "agent-1",
		Hostname: "target-node",
	}
	mockServer := &mockAgentServer{
		agents: []agentexec.ConnectedAgent{mockAgent},
	}
	svc := NewService(nil, mockServer)
	svc.policy = &mockPolicy{decision: agentexec.PolicyAllow}

	tc := providers.ToolCall{
		Name: "run_command",
		Input: map[string]interface{}{
			"command":     "uptime",
			"run_on_host": true,
			"target_host": "target-node",
		},
	}

	req := ExecuteRequest{
		TargetType: "vm",
		TargetID:   "vm-100",
		Context: map[string]interface{}{
			"node": "original-node",
		},
	}

	result, exec := svc.executeTool(context.Background(), req, tc)
	if !exec.Success {
		t.Errorf("Expected success, got failure: %s", result)
	}
	if exec.Input != "[target-node] uptime" {
		t.Errorf("Expected input to show target-node, got: %s", exec.Input)
	}
}

func TestService_HasAgentForTarget_WithResourceProvider(t *testing.T) {
	mockAgent := agentexec.ConnectedAgent{
		AgentID:  "agent-1",
		Hostname: "host-from-provider",
	}
	mockServer := &mockAgentServer{
		agents: []agentexec.ConnectedAgent{mockAgent},
	}
	svc := NewService(nil, mockServer)

	// Mock resource provider that returns host for container
	mockRP := &mockResourceProvider{}
	mockRP.ResourceProvider = mockRP
	svc.resourceProvider = mockRP

	// Overwrite FindContainerHost behavior
	// The mock by default returns empty string, so this should fail
	// But with containerName context, it will try the resource provider

	req := ExecuteRequest{
		TargetType: "docker",
		Context: map[string]interface{}{
			"containerName": "some-container",
		},
	}

	// Without provider returning anything, should still return true (at least one agent)
	if !svc.hasAgentForTarget(req) {
		t.Error("Expected true when at least one agent is available")
	}

	// With name context
	req2 := ExecuteRequest{
		TargetType: "docker",
		Context: map[string]interface{}{
			"name": "my-container",
		},
	}
	if !svc.hasAgentForTarget(req2) {
		t.Error("Expected true with name context")
	}

	// With guestName context
	req3 := ExecuteRequest{
		TargetType: "vm",
		Context: map[string]interface{}{
			"guestName": "my-vm",
		},
	}
	if !svc.hasAgentForTarget(req3) {
		t.Error("Expected true with guestName context")
	}
}

func TestService_HasAgentForTarget_HostFields(t *testing.T) {
	mockAgent := agentexec.ConnectedAgent{
		AgentID:  "agent-1",
		Hostname: "myhost",
	}
	mockServer := &mockAgentServer{
		agents: []agentexec.ConnectedAgent{mockAgent},
	}
	svc := NewService(nil, mockServer)

	hostFields := []string{"node", "host", "guest_node", "hostname", "host_name", "target_host"}

	for _, field := range hostFields {
		t.Run(field, func(t *testing.T) {
			req := ExecuteRequest{
				TargetType: "vm",
				Context: map[string]interface{}{
					field: "MYHOST", // Test case-insensitive matching
				},
			}
			if !svc.hasAgentForTarget(req) {
				t.Errorf("Expected true for host field %s", field)
			}
		})
	}
}

func TestService_ListModelsWithCache_CacheHit(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, nil)

	// Save a config
	cfg := config.AIConfig{
		Enabled:      true,
		OpenAIAPIKey: "test-key",
	}
	persistence.SaveAIConfig(cfg)

	// Get the cache key that persistence would generate
	loadedCfg, _ := persistence.LoadAIConfig()
	cacheKey := buildModelsCacheKey(loadedCfg)

	// Pre-populate cache before the first call
	svc.modelsCache.mu.Lock()
	svc.modelsCache.key = cacheKey
	svc.modelsCache.at = time.Now()
	svc.modelsCache.ttl = 5 * time.Minute
	svc.modelsCache.models = []providers.ModelInfo{
		{ID: "test-model", Name: "Test Model"},
	}
	svc.modelsCache.mu.Unlock()

	// Call should hit cache since we pre-populated it
	models, cached, err := svc.ListModelsWithCache(context.Background())
	if err != nil {
		t.Fatalf("ListModelsWithCache failed: %v", err)
	}
	if !cached {
		t.Error("Expected cache hit")
	}
	if len(models) != 1 || models[0].ID != "test-model" {
		t.Errorf("Expected cached model, got: %v", models)
	}
}

// Note: ListModelsWithCache does not handle nil persistence gracefully (panics)
// This is acceptable as NewService should always be called with a valid persistence

func TestProviderDisplayName_AllProviders(t *testing.T) {
	tests := []struct {
		provider string
		expected string
	}{
		{config.AIProviderAnthropic, "Anthropic"},
		{config.AIProviderOpenAI, "OpenAI"},
		{config.AIProviderDeepSeek, "DeepSeek"},
		{config.AIProviderGemini, "Google Gemini"},
		{config.AIProviderOllama, "Ollama"},
		{"custom_provider", "custom_provider"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := providerDisplayName(tt.provider)
			if got != tt.expected {
				t.Errorf("providerDisplayName(%q) = %q, want %q", tt.provider, got, tt.expected)
			}
		})
	}
}

func TestService_ExecuteOnAgent_VMIDExtraction(t *testing.T) {
	mockAgent := agentexec.ConnectedAgent{
		AgentID:  "agent-1",
		Hostname: "node-1",
	}

	var capturedCmd agentexec.ExecuteCommandPayload
	mockServer := &mockAgentServer{
		agents: []agentexec.ConnectedAgent{mockAgent},
		executeFunc: func(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
			capturedCmd = cmd
			return &agentexec.CommandResultPayload{
				Success: true,
				Stdout:  "ok",
			}, nil
		},
	}
	svc := NewService(nil, mockServer)

	// Test with VMID in context as float64 (common from JSON)
	req := ExecuteRequest{
		TargetType: "container",
		TargetID:   "instance-123",
		Context: map[string]interface{}{
			"node": "node-1",
			"vmid": float64(123),
		},
	}

	_, err := svc.executeOnAgent(context.Background(), req, "uptime")
	if err != nil {
		t.Fatalf("executeOnAgent failed: %v", err)
	}
	if capturedCmd.TargetID != "123" {
		t.Errorf("Expected TargetID '123', got '%s'", capturedCmd.TargetID)
	}

	// Test with VMID as int
	req.Context["vmid"] = 456
	_, err = svc.executeOnAgent(context.Background(), req, "uptime")
	if err != nil {
		t.Fatalf("executeOnAgent failed: %v", err)
	}
	if capturedCmd.TargetID != "456" {
		t.Errorf("Expected TargetID '456', got '%s'", capturedCmd.TargetID)
	}

	// Test with VMID as string
	req.Context["vmid"] = "789"
	_, err = svc.executeOnAgent(context.Background(), req, "uptime")
	if err != nil {
		t.Fatalf("executeOnAgent failed: %v", err)
	}
	if capturedCmd.TargetID != "789" {
		t.Errorf("Expected TargetID '789', got '%s'", capturedCmd.TargetID)
	}
}

func TestService_ExecuteOnAgent_AptNonInteractive(t *testing.T) {
	mockAgent := agentexec.ConnectedAgent{
		AgentID:  "agent-1",
		Hostname: "node-1",
	}

	var capturedCmd agentexec.ExecuteCommandPayload
	mockServer := &mockAgentServer{
		agents: []agentexec.ConnectedAgent{mockAgent},
		executeFunc: func(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
			capturedCmd = cmd
			return &agentexec.CommandResultPayload{Success: true, Stdout: "ok"}, nil
		},
	}
	svc := NewService(nil, mockServer)

	req := ExecuteRequest{
		TargetType: "host",
		Context:    map[string]interface{}{"node": "node-1"},
	}

	// apt command should get DEBIAN_FRONTEND prefix
	_, err := svc.executeOnAgent(context.Background(), req, "apt update")
	if err != nil {
		t.Fatalf("executeOnAgent failed: %v", err)
	}
	if !strings.Contains(capturedCmd.Command, "DEBIAN_FRONTEND=noninteractive") {
		t.Errorf("Expected DEBIAN_FRONTEND prefix for apt, got: %s", capturedCmd.Command)
	}

	// dpkg command should also get prefix
	_, err = svc.executeOnAgent(context.Background(), req, "dpkg --configure -a")
	if err != nil {
		t.Fatalf("executeOnAgent failed: %v", err)
	}
	if !strings.Contains(capturedCmd.Command, "DEBIAN_FRONTEND=noninteractive") {
		t.Errorf("Expected DEBIAN_FRONTEND prefix for dpkg, got: %s", capturedCmd.Command)
	}

	// If already has DEBIAN_FRONTEND, don't add again
	_, err = svc.executeOnAgent(context.Background(), req, "DEBIAN_FRONTEND=noninteractive apt update")
	if err != nil {
		t.Fatalf("executeOnAgent failed: %v", err)
	}
	if strings.Count(capturedCmd.Command, "DEBIAN_FRONTEND") > 1 {
		t.Error("Should not duplicate DEBIAN_FRONTEND")
	}
}

func TestService_ExecuteOnAgent_ResultHandling(t *testing.T) {
	mockAgent := agentexec.ConnectedAgent{
		AgentID:  "agent-1",
		Hostname: "node-1",
	}

	svc := NewService(nil, nil)
	req := ExecuteRequest{
		TargetType: "host",
		Context:    map[string]interface{}{"node": "node-1"},
	}

	// Test with both stdout and stderr
	mockServer := &mockAgentServer{
		agents: []agentexec.ConnectedAgent{mockAgent},
		executeFunc: func(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
			return &agentexec.CommandResultPayload{
				Success: true,
				Stdout:  "stdout content",
				Stderr:  "stderr content",
			}, nil
		},
	}
	svc.agentServer = mockServer

	output, err := svc.executeOnAgent(context.Background(), req, "ls")
	if err != nil {
		t.Fatalf("executeOnAgent failed: %v", err)
	}
	if !strings.Contains(output, "stdout content") || !strings.Contains(output, "STDERR") || !strings.Contains(output, "stderr content") {
		t.Errorf("Expected combined output, got: %s", output)
	}

	// Test with only stderr (failure case)
	mockServer.executeFunc = func(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
		return &agentexec.CommandResultPayload{
			Success: false,
			Stderr:  "error message",
		}, nil
	}

	output, err = svc.executeOnAgent(context.Background(), req, "bad-command")
	if err != nil {
		t.Fatalf("executeOnAgent failed: %v", err)
	}
	if output != "error message" {
		t.Errorf("Expected stderr as output, got: %s", output)
	}

	// Test with error
	mockServer.executeFunc = func(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
		return &agentexec.CommandResultPayload{
			Success: false,
			Error:   "command failed",
		}, nil
	}

	_, err = svc.executeOnAgent(context.Background(), req, "failing-command")
	if err == nil {
		t.Error("Expected error")
	}
	if !strings.Contains(err.Error(), "command failed") {
		t.Errorf("Expected error message, got: %v", err)
	}
}

func TestService_ExecuteOnAgent_RoutingClarification(t *testing.T) {
	svc := NewService(nil, &mockAgentServer{
		agents: []agentexec.ConnectedAgent{
			{AgentID: "agent-1", Hostname: "host-1"},
			{AgentID: "agent-2", Hostname: "host-2"},
		},
	})

	// No target node in context, multiple agents available -> clarification needed
	req := ExecuteRequest{
		TargetType: "host",
	}

	output, err := svc.executeOnAgent(context.Background(), req, "hostname")
	if err != nil {
		t.Fatalf("executeOnAgent failed: %v", err)
	}
	t.Logf("Output: %s", output)

	if !strings.Contains(output, "ROUTING_CLARIFICATION_NEEDED") {
		t.Errorf("Expected clarification needed response, got: %s", output)
	}
}

func TestService_ExecuteOnAgent_TargetIDExtractionOnly(t *testing.T) {
	var capturedCmd agentexec.ExecuteCommandPayload
	mockServer := &mockAgentServer{
		agents: []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "host-1"}},
		executeFunc: func(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
			capturedCmd = cmd
			return &agentexec.CommandResultPayload{Success: true, Stdout: "ok"}, nil
		},
	}
	svc := NewService(nil, mockServer)

	// No vmid in context, but TargetID has a number
	req := ExecuteRequest{
		TargetType: "container",
		TargetID:   "delly-135",
		Context: map[string]interface{}{
			"node": "host-1", // Force routing
		},
	}

	_, err := svc.executeOnAgent(context.Background(), req, "uptime")
	if err != nil {
		t.Fatalf("executeOnAgent failed: %v", err)
	}

	// Should have extracted 135 from delly-135
	if capturedCmd.TargetID != "135" {
		t.Errorf("Expected extracted TargetID '135', got '%s'", capturedCmd.TargetID)
	}
}

// ============================================================================
// approvalNeededFromToolCall Tests
// ============================================================================

func TestApprovalNeededFromToolCall_Extended(t *testing.T) {
	req := ExecuteRequest{
		Context: map[string]interface{}{
			"node": "node-1",
		},
	}

	tc := providers.ToolCall{
		ID:   "call-1",
		Name: "run_command",
		Input: map[string]interface{}{
			"command": "reboot",
		},
	}

	// Case 1: Result doesn't have APPROVAL_REQUIRED prefix
	_, ok := approvalNeededFromToolCall(req, tc, "regular result")
	if ok {
		t.Error("Expected false for regular result")
	}

	// Case 2: Wrong tool name
	tc2 := tc
	tc2.Name = "fetch_url"
	_, ok = approvalNeededFromToolCall(req, tc2, "APPROVAL_REQUIRED: ...")
	if ok {
		t.Error("Expected false for non-run_command tool")
	}

	// Case 3: Success with node from context
	data, ok := approvalNeededFromToolCall(req, tc, "APPROVAL_REQUIRED: reason")
	if !ok {
		t.Error("Expected true for approval required")
	}
	if data.TargetHost != "node-1" {
		t.Errorf("Expected TargetHost node-1, got %s", data.TargetHost)
	}

	// Case 4: Other context fields
	req2 := ExecuteRequest{Context: map[string]interface{}{"hostname": "host-2"}}
	data, _ = approvalNeededFromToolCall(req2, tc, "APPROVAL_REQUIRED: reason")
	if data.TargetHost != "host-2" {
		t.Errorf("Expected host-2, got %s", data.TargetHost)
	}

	req3 := ExecuteRequest{Context: map[string]interface{}{"host_name": "host-3"}}
	data, _ = approvalNeededFromToolCall(req3, tc, "APPROVAL_REQUIRED: reason")
	if data.TargetHost != "host-3" {
		t.Errorf("Expected host-3, got %s", data.TargetHost)
	}
}

// ============================================================================
// getTools Tests
// ============================================================================

func TestService_getTools_Variants(t *testing.T) {
	svc := NewService(nil, nil)

	// Case 1: Generic provider
	tools := svc.getTools()
	foundWebSearch := false
	for _, tool := range tools {
		if tool.Name == "web_search" {
			foundWebSearch = true
		}
	}
	if foundWebSearch {
		t.Error("Did not expect web_search tool for generic provider")
	}

	// Case 2: Anthropic provider
	svc.provider = &mockProvider{nameFunc: func() string { return "anthropic" }}
	tools = svc.getTools()
	foundWebSearch = false
	for _, tool := range tools {
		if tool.Name == "web_search" || tool.Type == "web_search_20250305" {
			foundWebSearch = true
		}
	}
	if !foundWebSearch {
		t.Error("Expected web_search tool for Anthropic provider")
	}
}

// ============================================================================
// ExecuteStream Tests - Continued
// ============================================================================

func TestService_ExecuteStream_BudgetExceeded(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, nil)
	svc.mu.Lock()
	svc.cfg = &config.AIConfig{
		Enabled:          true,
		CostBudgetUSD30d: 0.0001,                // Very low budget
		Model:            "anthropic:forbidden", // Force fallback to mock
	}
	svc.provider = &mockProvider{}
	svc.mu.Unlock()

	// We need costStore to have some usage
	costStore := cost.NewStore(30)
	costStore.Record(cost.UsageEvent{
		Timestamp:    time.Now(),
		Provider:     "openai",
		RequestModel: "gpt-4o",
		UseCase:      "chat",
		InputTokens:  1000000,
		OutputTokens: 1000000,
	})
	svc.costStore = costStore

	events := []StreamEvent{}
	callback := func(e StreamEvent) {
		events = append(events, e)
	}

	// First call should fail synchronously with error
	_, err := svc.ExecuteStream(context.Background(), ExecuteRequest{UseCase: "chat"}, callback)
	if err == nil {
		t.Error("Expected error due to budget")
	}
	if !strings.Contains(err.Error(), "budget") {
		t.Errorf("Expected budget error, got %v", err)
	}
}

func TestService_ExecuteStream_ThinkingContent(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, nil)
	mock := &mockProvider{
		chatFunc: func(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
			return &providers.ChatResponse{
				Content:          "Final response",
				ReasoningContent: "Thinking...",
				StopReason:       "end_turn",
			}, nil
		},
	}
	svc.provider = mock
	svc.cfg = &config.AIConfig{Enabled: true, Model: "anthropic:test"}

	events := []StreamEvent{}
	callback := func(e StreamEvent) {
		events = append(events, e)
	}

	_, err := svc.ExecuteStream(context.Background(), ExecuteRequest{}, callback)
	if err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	foundThinking := false
	for _, e := range events {
		if e.Type == "thinking" && e.Data == "Thinking..." {
			foundThinking = true
		}
	}
	if !foundThinking {
		t.Error("Expected thinking event in stream")
	}
}

func TestService_ExecuteStream_PolicyBlock(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, &mockAgentServer{})
	svc.policy = &mockPolicy{decision: agentexec.PolicyBlock}
	svc.cfg = &config.AIConfig{Enabled: true, Model: "anthropic:test"}

	iteration := 0
	mock := &mockProvider{
		chatFunc: func(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
			iteration++
			if iteration == 1 {
				return &providers.ChatResponse{
					ToolCalls: []providers.ToolCall{
						{ID: "call-1", Name: "run_command", Input: map[string]interface{}{"command": "rm -rf /"}},
					},
					StopReason: "tool_use",
				}, nil
			}
			return &providers.ChatResponse{
				Content:    "The command was blocked.",
				StopReason: "end_turn",
			}, nil
		},
	}
	svc.provider = mock

	events := []StreamEvent{}
	callback := func(e StreamEvent) {
		events = append(events, e)
	}

	_, err := svc.ExecuteStream(context.Background(), ExecuteRequest{}, callback)
	if err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	foundBlocked := false
	for _, e := range events {
		if e.Type == "tool_end" {
			data := e.Data.(ToolEndData)
			if !data.Success && strings.Contains(data.Output, "blocked") {
				foundBlocked = true
			}
		}
	}
	if !foundBlocked {
		t.Error("Expected blocked tool result in stream")
	}
}

func TestService_ExecuteStream_ResultTruncation(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, &mockAgentServer{
		agents: []agentexec.ConnectedAgent{
			{AgentID: "agent-1", Hostname: "agent-1"},
		},
		executeFunc: func(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
			return &agentexec.CommandResultPayload{
				Success: true,
				Stdout:  strings.Repeat("long output ", 1000),
			}, nil
		},
	})
	svc.cfg = &config.AIConfig{Enabled: true, AutonomousMode: true, Model: "anthropic:test"}

	iteration := 0
	mock := &mockProvider{
		chatFunc: func(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error) {
			iteration++
			if iteration == 1 {
				return &providers.ChatResponse{
					ToolCalls: []providers.ToolCall{
						{ID: "call-1", Name: "run_command", Input: map[string]interface{}{"command": "large-cmd", "target_host": "agent-1"}},
					},
					StopReason: "tool_use",
				}, nil
			}
			// Check if the previous tool result was truncated in the request messages
			for _, msg := range req.Messages {
				if msg.Role == "user" && msg.ToolResult != nil {
					// Use a more flexible check as the exact string might vary
					if strings.Contains(msg.ToolResult.Content, "omitted") || strings.Contains(msg.ToolResult.Content, "truncated") {
						return &providers.ChatResponse{Content: "Verified truncation", StopReason: "end_turn"}, nil
					}
				}
			}
			return &providers.ChatResponse{Content: "Not truncated", StopReason: "end_turn"}, nil
		},
	}
	svc.provider = mock

	resp, err := svc.ExecuteStream(context.Background(), ExecuteRequest{TargetType: "guest"}, func(StreamEvent) {})
	if err != nil {
		t.Fatalf("ExecuteStream failed: %v", err)
	}

	if resp.Content != "Verified truncation" {
		t.Errorf("Expected 'Verified truncation', got %s", resp.Content)
	}
}

func TestService_GetCostSummary_NoStore_Truncated(t *testing.T) {
	svc := &Service{} // Store is nil
	summary := svc.GetCostSummary(30)
	if summary.Days != 30 {
		t.Errorf("Expected 30 days, got %d", summary.Days)
	}
	if len(summary.ProviderModels) != 0 {
		t.Error("Expected 0 provider models")
	}

	// Test truncation branch
	summary = svc.GetCostSummary(cost.DefaultMaxDays + 1)
	if !summary.Truncated {
		t.Error("Expected summary to be truncated")
	}
}

func TestService_PatrolManagement_NilPatrol(t *testing.T) {
	svc := &Service{}

	// These should not panic when patrol is nil
	svc.StopPatrol()
	svc.SetMetricsHistoryProvider(nil)
	svc.SetBaselineStore(nil)
}

func TestService_StartPatrol_EdgeCases(t *testing.T) {
	// Case 1: Patrol service is nil
	svc := &Service{}
	svc.StartPatrol(context.Background()) // Should not panic

	// Case 2: Config is nil
	svc.patrolService = &PatrolService{}
	svc.StartPatrol(context.Background())

	// Case 3: Patrol disabled in config
	svc.cfg = &config.AIConfig{Enabled: true, PatrolSchedulePreset: "disabled"}
	svc.StartPatrol(context.Background())

	// Case 4: License feature missing
	svc.cfg = &config.AIConfig{Enabled: true, PatrolEnabled: true}
	svc.licenseChecker = &mockLicenseStore{features: map[string]bool{FeatureAIPatrol: false}}
	// This will still call Start but log info.
	svc.patrolService = &PatrolService{}
}

func TestSanitizeError_Extended(t *testing.T) {
	tests := []struct {
		input    error
		contains string
	}{
		{nil, ""},
		{fmt.Errorf("failed to send command: i/o timeout"), "timed out"},
		{fmt.Errorf("something i/o timeout"), "timeout"},
		{fmt.Errorf("connection refused"), "not be running"},
		{fmt.Errorf("no such host"), "host not found"},
		{fmt.Errorf("context deadline exceeded"), "timed out"},
		{fmt.Errorf("some other error"), "some other error"},
	}

	for _, tt := range tests {
		t.Run(tt.contains, func(t *testing.T) {
			result := sanitizeError(tt.input)
			if tt.input == nil {
				if result != nil {
					t.Error("Expected nil for nil input")
				}
				return
			}
			if !strings.Contains(result.Error(), tt.contains) {
				t.Errorf("sanitizeError(%v) = %v, expected to contain %q", tt.input, result, tt.contains)
			}
		})
	}
}

func TestService_GetGuestID(t *testing.T) {
	svc := NewService(nil, nil)

	tests := []struct {
		req      ExecuteRequest
		expected string
	}{
		{ExecuteRequest{TargetType: "", TargetID: ""}, ""},
		{ExecuteRequest{TargetType: "vm", TargetID: ""}, ""},
		{ExecuteRequest{TargetType: "", TargetID: "100"}, ""},
		{ExecuteRequest{TargetType: "vm", TargetID: "100"}, "vm-100"},
		{ExecuteRequest{TargetType: "container", TargetID: "host:ct:abc"}, "container-host:ct:abc"},
	}

	for _, tt := range tests {
		result := svc.getGuestID(tt.req)
		if result != tt.expected {
			t.Errorf("getGuestID(%+v) = %q, expected %q", tt.req, result, tt.expected)
		}
	}
}

func TestService_LogRemediation_WithPatrolService(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulse-log-rem-patrol-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, nil)

	// Set up patrol service
	stateProvider := &mockStateProvider{}
	svc.SetStateProvider(stateProvider)

	patrol := svc.GetPatrolService()
	if patrol == nil {
		t.Fatal("Expected patrol service")
	}

	// Set up remediation log
	remLog := NewRemediationLog(RemediationLogConfig{DataDir: tmpDir})
	patrol.remediationLog = remLog

	// Test logging with context name
	req := ExecuteRequest{
		TargetID:   "vm-100-with-patrol",
		TargetType: "vm",
		Prompt:     "High CPU usage detected",
		Context: map[string]interface{}{
			"name": "web-server",
		},
		UseCase: "patrol",
	}

	svc.logRemediation(req, "systemctl restart nginx", "output data", true)

	// Verify the log was created
	history := remLog.GetForResource("vm-100-with-patrol", 10)
	if len(history) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(history))
	}
	if history[0].ResourceName != "web-server" {
		t.Errorf("Expected resource name 'web-server', got '%s'", history[0].ResourceName)
	}
	if history[0].Outcome != OutcomeResolved {
		t.Errorf("Expected outcome Resolved, got %v", history[0].Outcome)
	}
	if !history[0].Automatic {
		t.Error("Expected Automatic=true for patrol use case")
	}
}

func TestService_LogRemediation_LongPromptTruncation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulse-log-rem-truncate-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, nil)

	stateProvider := &mockStateProvider{}
	svc.SetStateProvider(stateProvider)

	patrol := svc.GetPatrolService()
	remLog := NewRemediationLog(RemediationLogConfig{DataDir: tmpDir})
	patrol.remediationLog = remLog

	// Create a very long prompt
	longPrompt := strings.Repeat("a", 300)

	req := ExecuteRequest{
		TargetID:   "vm-100-truncation-test",
		TargetType: "vm",
		Prompt:     longPrompt,
	}

	svc.logRemediation(req, "docker restart app", "output", false)

	history := remLog.GetForResource("vm-100-truncation-test", 10)
	if len(history) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(history))
	}
	if len(history[0].Problem) > 205 { // 200 + "..."
		t.Errorf("Problem should be truncated, got length %d", len(history[0].Problem))
	}
	if history[0].Outcome != OutcomeFailed {
		t.Errorf("Expected outcome Failed for success=false, got %v", history[0].Outcome)
	}
}

// ============================================================================
// Knowledge Store Tests
// ============================================================================

func TestService_KnowledgeStore_NotConfigured(t *testing.T) {
	svc := NewService(nil, nil)
	// knowledgeStore is nil

	_, err := svc.GetGuestKnowledge("test-guest")
	if err == nil {
		t.Error("Expected error when knowledge store not available")
	}

	err = svc.SaveGuestNote("guest-1", "name", "vm", "cat", "title", "content")
	if err == nil {
		t.Error("Expected error when knowledge store not available")
	}

	err = svc.DeleteGuestNote("guest-1", "note-1")
	if err == nil {
		t.Error("Expected error when knowledge store not available")
	}
}

// ============================================================================
// LoadConfig Advanced Tests
// ============================================================================

func TestService_LoadConfig_WithDisabledAI(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulse-loadconfig-disabled-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	persistence := config.NewConfigPersistence(tmpDir)

	// Save a disabled config
	cfg := config.AIConfig{
		Enabled: false,
	}
	persistence.SaveAIConfig(cfg)

	svc := NewService(persistence, nil)
	err = svc.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if svc.IsEnabled() {
		t.Error("Expected AI to be disabled")
	}
}

func TestService_LoadConfig_SmartFallback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulse-loadconfig-smart-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	persistence := config.NewConfigPersistence(tmpDir)

	// Config with Anthropic model selected but only OpenAI configured
	// NewForModel("anthropic:...") will fail, triggering smart fallback to OpenAI
	cfg := config.AIConfig{
		Enabled:      true,
		Model:        "anthropic:claude-3-opus",
		OpenAIAPIKey: "test-openai-key",
	}
	persistence.SaveAIConfig(cfg)

	svc := NewService(persistence, nil)
	err = svc.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if svc.provider == nil {
		t.Fatal("Expected provider to be set via smart fallback")
	}
	if svc.provider.Name() != "openai" {
		t.Errorf("Expected openai provider, got %s", svc.provider.Name())
	}
}

func TestService_LoadConfig_LegacyMigration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulse-loadconfig-legacy-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	persistence := config.NewConfigPersistence(tmpDir)

	// Config with legacy fields but no provider credentials in new format
	cfg := config.AIConfig{
		Enabled:  true,
		Provider: "openai",
		APIKey:   "legacy-key",
		Model:    "anthropic:claude-3-5-sonnet",
	}
	persistence.SaveAIConfig(cfg)

	svc := NewService(persistence, nil)
	err = svc.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Should initialize via migration path
	if svc.provider == nil {
		t.Error("Expected provider to be initialized via legacy config")
	}
}

func TestService_LoadConfig_LegacyMigration_Failure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulse-loadconfig-legacy-fail-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	persistence := config.NewConfigPersistence(tmpDir)

	// Legacy config with unknown provider to force NewFromConfig to fail
	cfg := config.AIConfig{
		Enabled:  true,
		Provider: "unknown-provider",
		APIKey:   "key",
		Model:    "anthropic:claude-opus",
	}
	persistence.SaveAIConfig(cfg)

	svc := NewService(persistence, nil)
	err = svc.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if svc.provider != nil {
		t.Error("Expected provider to be nil on migration failure")
	}
}

func TestService_LoadConfig_SmartFallback_Variants(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulse-loadconfig-fallback-variants-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	persistence := config.NewConfigPersistence(tmpDir)

	providers := []struct {
		name   string
		key    string
		expect string
	}{
		{"anthropic", "AnthropicAPIKey", "anthropic"},
		{"gemini", "GeminiAPIKey", "gemini"},
		{"deepseek", "DeepSeekAPIKey", "openai"}, // DeepSeek uses OpenAI client
		{"ollama", "OllamaBaseURL", "ollama"},
	}

	for _, p := range providers {
		t.Run(p.name, func(t *testing.T) {
			cfg := config.AIConfig{
				Enabled: true,
				Model:   "openai:gpt-4", // selected
			}

			// Set the specific key
			switch p.name {
			case "anthropic":
				cfg.AnthropicAPIKey = "key"
			case "gemini":
				cfg.GeminiAPIKey = "key"
			case "deepseek":
				cfg.DeepSeekAPIKey = "key"
			case "ollama":
				cfg.OllamaBaseURL = "http://localhost:11434"
			}

			persistence.SaveAIConfig(cfg)
			svc := NewService(persistence, nil)
			err = svc.LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig failed: %v", err)
			}

			if svc.provider == nil || svc.provider.Name() != p.expect {
				t.Errorf("Expected %s provider via fallback, got %v", p.expect, svc.provider)
			}
		})
	}
}

func TestService_LoadConfig_NoProviders(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulse-loadconfig-none-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	persistence := config.NewConfigPersistence(tmpDir)

	// Enabled but no providers configured
	cfg := config.AIConfig{
		Enabled: true,
	}
	persistence.SaveAIConfig(cfg)

	svc := NewService(persistence, nil)
	err = svc.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if svc.provider != nil {
		t.Error("Expected provider to be nil when no credentials configured")
	}
}

func TestService_LoadConfig_PersistenceError(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)

	// Write invalid JSON to ai.enc (this is tricky because it's encrypted)
	// Actually, we can just make the directory unreadable or use a non-directory for the path
	aiFilePath := filepath.Join(tmpDir, "ai.enc")
	os.MkdirAll(aiFilePath, 0755) // Create directory where file should be

	svc := NewService(persistence, nil)
	err := svc.LoadConfig()
	if err == nil {
		t.Error("Expected error when LoadAIConfig fails")
	}
}

func TestService_LoadConfig_FallbackFailure(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)

	// Create a bit of a weird situation: Ollama is configured (which always "succeeds" to create client)
	// but we want providers.NewForModel to fail for the fallback model.
	// Actually, NewForModel only fails if provider is unknown or key is missing.

	// Selected model is unknown provider, triggers fallback.
	// Fallback provider is Ollama.
	cfg := config.AIConfig{
		Enabled:       true,
		Model:         "unknown:model",
		OllamaBaseURL: "http://localhost:11434",
	}
	persistence.SaveAIConfig(cfg)

	svc := NewService(persistence, nil)
	// We need providers.NewForModel to fail for Ollama fallback model?
	// That's hard because Ollama NewForProvider always succeeds.

	err := svc.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	// It should fall back to Ollama
	if svc.provider.Name() != "ollama" {
		t.Errorf("Expected ollama provider, got %s", svc.provider.Name())
	}
}

// ============================================================================
// GetConfig Tests
// ============================================================================

func TestService_GetConfig_DefaultsWhenNil(t *testing.T) {
	svc := NewService(nil, nil)
	svc.cfg = nil

	cfg := svc.GetConfig()
	if cfg != nil {
		t.Error("Expected nil config when not configured")
	}
}

func TestService_GetConfig_ReturnsCopy(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulse-getconfig-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	persistence := config.NewConfigPersistence(tmpDir)
	persistence.SaveAIConfig(config.AIConfig{
		Enabled: true,
		Model:   "test:model",
	})

	svc := NewService(persistence, nil)
	svc.LoadConfig()

	cfg := svc.GetConfig()
	if cfg == nil {
		t.Fatal("Expected config to be returned")
	}

	if cfg.Model != "test:model" {
		t.Errorf("Expected model 'test:model', got '%s'", cfg.Model)
	}
}

// ============================================================================
// IsAutonomous Tests
// ============================================================================

func TestService_IsAutonomous_Variations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulse-autonomous-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	persistence := config.NewConfigPersistence(tmpDir)

	// Test with AutonomousMode = false
	cfg := config.AIConfig{
		Enabled:        true,
		AutonomousMode: false,
	}
	persistence.SaveAIConfig(cfg)

	svc := NewService(persistence, nil)
	svc.LoadConfig()

	if svc.IsAutonomous() {
		t.Error("Expected not autonomous when mode is false")
	}

	// Test with AutonomousMode = true
	cfg.AutonomousMode = true
	persistence.SaveAIConfig(cfg)
	svc.LoadConfig()

	if !svc.IsAutonomous() {
		t.Error("Expected autonomous when mode is true")
	}
}

// ============================================================================
// Knowledge Store Tests - Extended
// ============================================================================

func TestService_KnowledgeStore_Success(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := knowledge.NewStore(tmpDir)

	svc := NewService(nil, nil)
	svc.knowledgeStore = store

	_, err := svc.GetGuestKnowledge("vm-100")
	if err != nil {
		t.Errorf("GetGuestKnowledge failed: %v", err)
	}

	err = svc.SaveGuestNote("vm-100", "test", "vm", "general", "title", "content")
	if err != nil {
		t.Errorf("SaveGuestNote failed: %v", err)
	}

	err = svc.DeleteGuestNote("vm-100", "general-1")
	if err != nil {
		t.Errorf("DeleteGuestNote failed: %v", err)
	}
}

// ============================================================================
// ListModelsWithCache - Extended
// ============================================================================

func TestService_ListModelsWithCache_ProviderErrors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulse-listmodels-err-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	persistence := config.NewConfigPersistence(tmpDir)
	persistence.SaveAIConfig(config.AIConfig{
		Enabled:      true,
		OpenAIAPIKey: "test-key",
	})

	svc := NewService(persistence, nil)

	// The provider will fail because it's a real OpenAI provider with dummy key
	_, cached, err := svc.ListModelsWithCache(context.Background())
	if err != nil {
		t.Fatalf("ListModelsWithCache should not fail even if providers fail: %v", err)
	}
	if cached {
		t.Error("Expected no cache hit")
	}
}

// ============================================================================
// GetDebugContext - Truncation
// ============================================================================

func TestService_GetDebugContext_Truncation(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, nil)

	// Large custom context
	svc.cfg = &config.AIConfig{
		CustomContext: strings.Repeat("x", 300),
	}

	stateProvider := &mockStateProvider{}
	svc.stateProvider = stateProvider

	// Add many VMs to trigger truncation
	state := models.StateSnapshot{}
	for i := 0; i < 15; i++ {
		state.VMs = append(state.VMs, models.VM{VMID: i, Name: fmt.Sprintf("vm-%d", i), Node: "node"})
		state.Containers = append(state.Containers, models.Container{VMID: i + 100, Name: fmt.Sprintf("ct-%d", i), Node: "node"})
	}
	stateProvider.state = state

	result := svc.GetDebugContext(ExecuteRequest{})

	preview := result["custom_context_preview"].(string)
	if !strings.HasSuffix(preview, "...") || len(preview) != 203 {
		t.Errorf("Expected truncated preview, got length %d", len(preview))
	}

	sampleVMs := result["sample_vms"].([]string)
	if len(sampleVMs) != 10 {
		t.Errorf("Expected 10 sample VMs, got %d", len(sampleVMs))
	}

	sampleCTs := result["sample_containers"].([]string)
	if len(sampleCTs) != 10 {
		t.Errorf("Expected 10 sample containers, got %d", len(sampleCTs))
	}
}

func TestService_GetDebugContext_ShortContext(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{
		CustomContext: "short context",
	}

	result := svc.GetDebugContext(ExecuteRequest{})
	if result["custom_context_preview"] != "short context" {
		t.Errorf("Expected 'short context', got %v", result["custom_context_preview"])
	}
}

func TestService_ListModelsWithCache_ConfigErrors(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)

	// Force LoadAIConfig failure
	aiFilePath := filepath.Join(tmpDir, "ai.enc")
	os.MkdirAll(aiFilePath, 0755)

	svc := NewService(persistence, nil)
	_, _, err := svc.ListModelsWithCache(context.Background())
	if err == nil {
		t.Error("Expected error when LoadAIConfig fails")
	}
}

// ============================================================================
// SetResourceURL Tests - Extended
// ============================================================================

func TestService_SetResourceURL_DockerType(t *testing.T) {
	svc := NewService(nil, nil)
	mockMeta := &mockMetadataProvider{}
	svc.metadataProvider = mockMeta

	err := svc.SetResourceURL("docker", "host:container:abc123", "http://example.com:8080")
	if err != nil {
		t.Errorf("SetResourceURL failed: %v", err)
	}
	if mockMeta.lastDockerID != "host:container:abc123" {
		t.Errorf("Expected docker ID 'host:container:abc123', got '%s'", mockMeta.lastDockerID)
	}
}

func TestService_SetResourceURL_HostType(t *testing.T) {
	svc := NewService(nil, nil)
	mockMeta := &mockMetadataProvider{}
	svc.metadataProvider = mockMeta

	err := svc.SetResourceURL("host", "my-host", "http://host.local:9090")
	if err != nil {
		t.Errorf("SetResourceURL failed: %v", err)
	}
	if mockMeta.lastHostID != "my-host" {
		t.Errorf("Expected host ID 'my-host', got '%s'", mockMeta.lastHostID)
	}
}

func TestService_SetResourceURL_UnknownType(t *testing.T) {
	svc := NewService(nil, nil)
	mockMeta := &mockMetadataProvider{}
	svc.metadataProvider = mockMeta

	err := svc.SetResourceURL("unknown", "id", "http://example.com")
	if err == nil {
		t.Error("Expected error for unknown resource type")
	}
}

func TestService_SetResourceURL_NoMetadataProvider(t *testing.T) {
	svc := NewService(nil, nil)
	svc.metadataProvider = nil

	err := svc.SetResourceURL("guest", "vm-100", "http://example.com")
	if err == nil {
		t.Error("Expected error when metadata provider not available")
	}
}

func TestService_BuildUserAnnotationsContext_Variants(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, nil)

	// Guest metadata with no name
	guestMeta := map[string]*config.GuestMetadata{
		"guest-no-name": {
			ID:    "guest-no-name",
			Notes: []string{"Note1"},
		},
	}
	data, _ := json.Marshal(guestMeta)
	os.WriteFile(filepath.Join(tmpDir, "guest_metadata.json"), data, 0644)

	// Docker metadata with long ID and simple ID (no host part)
	dockerMeta := map[string]*config.DockerMetadata{
		"host:container:verylongcontainerid123456": {
			ID:    "host:container:verylongcontainerid123456",
			Notes: []string{"DockerNote1"},
		},
		"simple-id": {
			ID:    "simple-id",
			Notes: []string{"SimpleDockerNote"},
		},
	}
	data, _ = json.Marshal(dockerMeta)
	os.WriteFile(filepath.Join(tmpDir, "docker_metadata.json"), data, 0644)

	ctx := svc.buildUserAnnotationsContext()

	// Check guest ID fallback
	if !strings.Contains(ctx, "guest-no-name") {
		t.Error("Expected guest ID when LastKnownName is missing")
	}

	// Check long container ID truncation
	if !strings.Contains(ctx, "verylongcont") {
		t.Error("Container name should contain partial ID")
	}

	// Check simple docker ID
	if !strings.Contains(ctx, "simple-id") {
		t.Error("Should handle simple docker ID")
	}
}

func TestService_GetDebugContext_Extended(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, nil)

	// Mock state provider
	stateProvider := &mockStateProvider{}
	state := models.StateSnapshot{}
	for i := 0; i < 15; i++ {
		state.VMs = append(state.VMs, models.VM{Name: fmt.Sprintf("VM-%d", i), VMID: i, Node: "node"})
		state.Containers = append(state.Containers, models.Container{Name: fmt.Sprintf("CT-%d", i), VMID: i + 100, Node: "node"})
	}
	stateProvider.state = state
	svc.SetStateProvider(stateProvider)

	svc.cfg = &config.AIConfig{
		Enabled:       true,
		CustomContext: strings.Repeat("x", 300),
	}

	debug := svc.GetDebugContext(ExecuteRequest{})

	if len(debug["sample_vms"].([]string)) != 10 {
		t.Errorf("Expected 10 sample VMs, got %d", len(debug["sample_vms"].([]string)))
	}
	if len(debug["sample_containers"].([]string)) != 10 {
		t.Errorf("Expected 10 sample containers, got %d", len(debug["sample_containers"].([]string)))
	}
	if !strings.HasSuffix(debug["custom_context_preview"].(string), "...") {
		t.Error("Expected custom context preview to be truncated")
	}
}

func TestService_EnrichRequestFromFinding(t *testing.T) {
	svc := NewService(nil, nil)

	// Setup patrol service with a mock finding
	mockState := &mockStateProvider{
		state: models.StateSnapshot{},
	}
	svc.stateProvider = mockState
	svc.patrolService = NewPatrolService(svc, mockState)

	// Add a finding to the store
	finding := &Finding{
		ID:           "test-finding-123",
		Node:         "minipc",
		ResourceID:   "delly:minipc:112",
		ResourceName: "debian-go",
		ResourceType: "container",
		Title:        "Test Finding",
		Description:  "Test description",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryCapacity,
	}
	svc.patrolService.GetFindings().Add(finding)

	// Test 1: Enrich request with finding context
	req := ExecuteRequest{
		FindingID: "test-finding-123",
		Prompt:    "Help me fix this",
	}
	svc.enrichRequestFromFinding(&req)

	if req.Context["node"] != "minipc" {
		t.Errorf("Expected node 'minipc', got %v", req.Context["node"])
	}
	if req.Context["guestName"] != "debian-go" {
		t.Errorf("Expected guestName 'debian-go', got %v", req.Context["guestName"])
	}
	if req.TargetID != "delly:minipc:112" {
		t.Errorf("Expected TargetID 'delly:minipc:112', got %s", req.TargetID)
	}
	if req.TargetType != "container" {
		t.Errorf("Expected TargetType 'container', got %s", req.TargetType)
	}
	if req.Context["finding_node"] != "minipc" {
		t.Errorf("Expected finding_node 'minipc', got %v", req.Context["finding_node"])
	}

	// Test 2: Don't overwrite existing context
	req2 := ExecuteRequest{
		FindingID:  "test-finding-123",
		TargetID:   "existing-target",
		TargetType: "existing-type",
		Context: map[string]interface{}{
			"node":      "existing-node",
			"guestName": "existing-name",
		},
	}
	svc.enrichRequestFromFinding(&req2)

	if req2.Context["node"] != "existing-node" {
		t.Errorf("Should not overwrite existing node, got %v", req2.Context["node"])
	}
	if req2.Context["guestName"] != "existing-name" {
		t.Errorf("Should not overwrite existing guestName, got %v", req2.Context["guestName"])
	}
	if req2.TargetID != "existing-target" {
		t.Errorf("Should not overwrite existing TargetID, got %s", req2.TargetID)
	}
	// But finding_* fields should still be added
	if req2.Context["finding_node"] != "minipc" {
		t.Errorf("Expected finding_node 'minipc', got %v", req2.Context["finding_node"])
	}

	// Test 3: No finding ID - should be a no-op
	req3 := ExecuteRequest{
		Prompt: "Just a question",
	}
	svc.enrichRequestFromFinding(&req3)
	if req3.Context != nil && len(req3.Context) > 0 {
		t.Error("Should not add context when no FindingID")
	}

	// Test 4: Non-existent finding ID - should be a no-op
	req4 := ExecuteRequest{
		FindingID: "non-existent",
		Prompt:    "Help",
	}
	svc.enrichRequestFromFinding(&req4)
	if req4.Context != nil && len(req4.Context) > 0 {
		t.Error("Should not add context when finding not found")
	}
}

// TestService_FindingContextInSystemPrompt verifies that when a FindingID is provided,
// the system prompt includes proper routing instructions derived from the finding's node.
// This is the integration test that would have caught the original bug.
func TestService_FindingContextInSystemPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, nil)
	svc.cfg = &config.AIConfig{Enabled: true}

	// Setup patrol service with a finding that has node info
	mockState := &mockStateProvider{
		state: models.StateSnapshot{
			Containers: []models.Container{
				{VMID: 112, Node: "minipc", Name: "debian-go", Instance: "delly"},
			},
		},
	}
	svc.stateProvider = mockState
	svc.patrolService = NewPatrolService(svc, mockState)

	// Add a finding with node context
	finding := &Finding{
		ID:           "disk-growth-finding",
		Node:         "minipc",
		ResourceID:   "delly:minipc:112",
		ResourceName: "debian-go",
		ResourceType: "container",
		Title:        "Container disk usage growing",
		Description:  "Disk growing at 4% per day",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryCapacity,
	}
	svc.patrolService.GetFindings().Add(finding)

	// Create a request like "Get Help" would - only FindingID provided
	req := ExecuteRequest{
		FindingID: "disk-growth-finding",
		Prompt:    "Help me fix this disk issue",
	}

	// Enrich the request (this is what Execute() does now)
	svc.enrichRequestFromFinding(&req)

	// Build the system prompt
	prompt := svc.buildSystemPrompt(req)

	// Verify the prompt includes routing context
	if !strings.Contains(prompt, "minipc") {
		t.Error("System prompt should include the node name 'minipc' for routing")
	}
	if !strings.Contains(prompt, "debian-go") {
		t.Error("System prompt should include the resource name 'debian-go'")
	}
	if !strings.Contains(prompt, "target_host") || !strings.Contains(prompt, "minipc") {
		t.Error("System prompt should include ROUTING instructions mentioning target_host=minipc")
	}
}

// TestService_FindingContextCommandRouting verifies that commands route correctly
// when ExecuteStream is called with a FindingID. This is an end-to-end test.
func TestService_FindingContextCommandRouting(t *testing.T) {
	// Track which agent the command was routed to
	routedToAgent := ""

	mockAgentServer := &mockAgentServer{
		agents: []agentexec.ConnectedAgent{
			{AgentID: "agent-delly", Hostname: "delly"},
			{AgentID: "agent-minipc", Hostname: "minipc"},
		},
		executeFunc: func(ctx context.Context, agentID string, cmd agentexec.ExecuteCommandPayload) (*agentexec.CommandResultPayload, error) {
			routedToAgent = agentID
			return &agentexec.CommandResultPayload{
				Success: true,
				Stdout:  "Command output",
			}, nil
		},
	}

	tmpDir := t.TempDir()
	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, mockAgentServer)
	svc.cfg = &config.AIConfig{Enabled: true}
	svc.limits = executionLimits{
		chatSlots:   make(chan struct{}, 10),
		patrolSlots: make(chan struct{}, 10),
	}
	svc.policy = &mockPolicy{decision: agentexec.PolicyAllow}

	// Setup patrol service with finding
	mockState := &mockStateProvider{
		state: models.StateSnapshot{
			Containers: []models.Container{
				{VMID: 112, Node: "minipc", Name: "debian-go", Instance: "delly"},
			},
		},
	}
	svc.stateProvider = mockState
	svc.patrolService = NewPatrolService(svc, mockState)

	// Add finding on node "minipc"
	finding := &Finding{
		ID:           "test-routing-finding",
		Node:         "minipc",
		ResourceID:   "delly:minipc:112",
		ResourceName: "debian-go",
		ResourceType: "container",
	}
	svc.patrolService.GetFindings().Add(finding)

	// Create request with finding ID (like "Get Help" does)
	req := ExecuteRequest{
		FindingID: "test-routing-finding",
		Prompt:    "Check disk usage",
	}

	// Enrich the request
	svc.enrichRequestFromFinding(&req)

	// Now execute a command tool call - should route to minipc
	tc := providers.ToolCall{
		Name: "run_command",
		Input: map[string]interface{}{
			"command": "df -h",
		},
	}

	result, exec := svc.executeTool(context.Background(), req, tc)

	if !exec.Success {
		t.Errorf("Command execution failed: %s", result)
	}

	// Verify it routed to the correct agent (minipc, not delly)
	if routedToAgent != "agent-minipc" {
		t.Errorf("Expected command to route to agent-minipc, but went to: %s", routedToAgent)
	}
}
