package tools

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubMetadataUpdater struct{}

func (s *stubMetadataUpdater) SetResourceURL(resourceType, resourceID, url string) error {
	return nil
}

type stubAgentProfileManager struct{}

func (s *stubAgentProfileManager) ApplyAgentScope(ctx context.Context, agentID, agentLabel string, settings map[string]interface{}) (string, string, bool, error) {
	return "profile-1", "default", false, nil
}

func (s *stubAgentProfileManager) AssignProfile(ctx context.Context, agentID, profileID string) (string, error) {
	return "default", nil
}

func (s *stubAgentProfileManager) GetAgentScope(ctx context.Context, agentID string) (*AgentScope, error) {
	return &AgentScope{AgentID: agentID}, nil
}

func TestPulseToolExecutor_Setters(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{})

	exec.SetContext("vm", "101", true)
	assert.Equal(t, "vm", exec.targetType)
	assert.Equal(t, "101", exec.targetID)
	assert.True(t, exec.isAutonomous)

	exec.SetAutonomousMode(false)
	assert.Equal(t, "vm", exec.targetType)
	assert.Equal(t, "101", exec.targetID)
	assert.False(t, exec.isAutonomous)

	exec.SetControlLevel(ControlLevelControlled)
	assert.Equal(t, ControlLevelControlled, exec.controlLevel)

	exec.SetProtectedGuests([]string{"100", "101"})
	assert.Equal(t, []string{"100", "101"}, exec.protectedGuests)

	metadataUpdater := &stubMetadataUpdater{}
	exec.SetMetadataUpdater(metadataUpdater)
	assert.Equal(t, metadataUpdater, exec.metadataUpdater)

	findingsManager := &stubFindingsManager{}
	exec.SetFindingsManager(findingsManager)
	assert.Equal(t, findingsManager, exec.findingsManager)

	metricsHistory := &mockMetricsHistoryProvider{}
	exec.SetMetricsHistory(metricsHistory)
	assert.Equal(t, metricsHistory, exec.metricsHistory)

	baselineProvider := &stubBaselineProvider{}
	exec.SetBaselineProvider(baselineProvider)
	assert.Equal(t, baselineProvider, exec.baselineProvider)

	patternProvider := &stubPatternProvider{}
	exec.SetPatternProvider(patternProvider)
	assert.Equal(t, patternProvider, exec.patternProvider)

	alertProvider := &mockAlertProvider{}
	exec.SetAlertProvider(alertProvider)
	assert.Equal(t, alertProvider, exec.alertProvider)

	findingsProvider := &mockFindingsProvider{}
	exec.SetFindingsProvider(findingsProvider)
	assert.Equal(t, findingsProvider, exec.findingsProvider)

	backupProvider := &stubBackupProvider{}
	exec.SetBackupProvider(backupProvider)
	assert.Equal(t, backupProvider, exec.backupProvider)

	diskHealthProvider := &mockDiskHealthProvider{}
	exec.SetDiskHealthProvider(diskHealthProvider)
	assert.Equal(t, diskHealthProvider, exec.diskHealthProvider)

	agentProfileManager := &stubAgentProfileManager{}
	exec.SetAgentProfileManager(agentProfileManager)
	assert.Equal(t, agentProfileManager, exec.agentProfileManager)

	updatesProvider := &mockUpdatesProvider{}
	exec.SetUpdatesProvider(updatesProvider)
	assert.Equal(t, updatesProvider, exec.updatesProvider)

	actionAuditStore := unifiedresources.NewMemoryStore()
	exec.SetActionAuditStore(actionAuditStore)
	assert.Equal(t, actionAuditStore, exec.actionAuditStore)
}

func TestPulseToolExecutor_ListTools(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{})
	tools := exec.ListTools()
	// pulse_query requires state provider, so it should not be available without one
	assert.False(t, containsTool(tools, "pulse_query"))

	execWithState := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{}})
	stateTools := execWithState.ListTools()
	// With state provider, pulse_query should be available
	assert.True(t, containsTool(stateTools, "pulse_query"))
	assert.False(t, containsTool(stateTools, "pulse_kubernetes"))

	adapter := unifiedresources.NewMonitorAdapter(nil)
	adapter.PopulateFromSnapshot(models.StateSnapshot{
		Nodes:              []models.Node{{ID: "node-1", Name: "pve1", Status: "online"}},
		KubernetesClusters: []models.KubernetesCluster{{ID: "cluster-1", Name: "cluster-1"}},
	})
	execWithUnifiedReadState := NewPulseToolExecutor(ExecutorConfig{UnifiedResourceProvider: adapter})
	unifiedTools := execWithUnifiedReadState.ListTools()
	assert.True(t, containsTool(unifiedTools, "pulse_query"))
	assert.True(t, containsTool(unifiedTools, "pulse_pmg"))
	assert.True(t, containsTool(unifiedTools, "pulse_kubernetes"))
}

func TestPulseToolExecutor_IsToolAvailable(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{})
	// pulse_metrics requires metrics provider or state provider
	assert.False(t, exec.isToolAvailable("pulse_metrics"))
	// pulse_query requires state provider
	assert.False(t, exec.isToolAvailable("pulse_query"))

	// Create new executor with state provider and metrics history
	execWithProviders := NewPulseToolExecutor(ExecutorConfig{
		StateProvider:  &mockStateProvider{},
		MetricsHistory: &mockMetricsHistoryProvider{},
	})
	// Now pulse_metrics should be available with metrics history
	assert.True(t, execWithProviders.isToolAvailable("pulse_metrics"))
	// And pulse_query should be available with state provider
	assert.True(t, execWithProviders.isToolAvailable("pulse_query"))
	assert.True(t, execWithProviders.isToolAvailable("pulse_pmg"))
	assert.False(t, execWithProviders.isToolAvailable("pulse_kubernetes"))

	adapter := unifiedresources.NewMonitorAdapter(nil)
	adapter.PopulateFromSnapshot(models.StateSnapshot{
		PMGInstances:       []models.PMGInstance{{ID: "pmg-1", Name: "pmg-1"}},
		KubernetesClusters: []models.KubernetesCluster{{ID: "cluster-1", Name: "cluster-1"}},
	})
	execWithUnifiedReadState := NewPulseToolExecutor(ExecutorConfig{
		UnifiedResourceProvider: adapter,
	})
	assert.True(t, execWithUnifiedReadState.isToolAvailable("pulse_pmg"))
	assert.True(t, execWithUnifiedReadState.isToolAvailable("pulse_kubernetes"))
	assert.False(t, execWithUnifiedReadState.isToolAvailable("pulse_read"))

	execWithNativeRead := NewPulseToolExecutor(ExecutorConfig{
		UnifiedResourceProvider:  adapter,
		ReadState:                unifiedresources.NewRegistry(nil),
		AppContainerReadProvider: &stubAppContainerReadProvider{},
	})
	assert.True(t, execWithNativeRead.isToolAvailable("pulse_read"))
}

func TestPulseToolExecutor_GetReadStatePrefersUnifiedResourceProvider(t *testing.T) {
	adapter := unifiedresources.NewMonitorAdapter(nil)
	adapter.PopulateFromSnapshot(models.StateSnapshot{
		VMs: []models.VM{{ID: "vm-unified", Name: "vm-unified", Status: "running", Node: "pve1", Instance: "cluster-a"}},
	})

	stateProvider := &mockStateProvider{}
	stateProvider.On("ReadSnapshot").Return(models.StateSnapshot{
		VMs: []models.VM{{ID: "vm-snapshot", Name: "vm-snapshot", Status: "running", Node: "pve2", Instance: "cluster-b"}},
	})

	exec := NewPulseToolExecutor(ExecutorConfig{
		StateProvider:           stateProvider,
		UnifiedResourceProvider: adapter,
	})

	readState := exec.getReadState()
	require.NotNil(t, readState)
	require.Len(t, readState.VMs(), 1)
	assert.Equal(t, "vm-unified", readState.VMs()[0].Name())
	stateProvider.AssertNotCalled(t, "ReadSnapshot")
}

func TestToolRegistry_ListTools(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(RegisteredTool{
		Definition: Tool{Name: "read"},
	})
	registry.Register(RegisteredTool{
		Definition:     Tool{Name: "control"},
		RequireControl: true,
	})

	readOnly := registry.ListTools(ControlLevelReadOnly)
	require.Len(t, readOnly, 1)
	assert.Equal(t, "read", readOnly[0].Name)

	unknown := registry.ListTools(ControlLevel("bad"))
	require.Len(t, unknown, 1)
	assert.Equal(t, "read", unknown[0].Name)

	full := registry.ListTools(ControlLevelControlled)
	require.Len(t, full, 2)
	assert.Equal(t, "read", full[0].Name)
	assert.Equal(t, "control", full[1].Name)

	governance := registry.ListToolGovernance(ControlLevelControlled)
	require.Len(t, governance, 2)
	assert.Equal(t, ToolActionRead, governance[0].ActionMode)
	assert.Equal(t, ToolActionWrite, governance[1].ActionMode)
	assert.Equal(t, ToolApprovalActionPlan, governance[1].ApprovalPolicy)
	assert.Equal(t, "hidden in read-only mode; approval required in controlled mode", governance[1].ApprovalSummary)
}

func TestToolRegistry_ListToolsReturnsIndependentDefinitions(t *testing.T) {
	registry := NewToolRegistry()
	required := []string{"mode"}
	enum := []string{"summary", "detail"}
	properties := map[string]PropertySchema{
		"mode": {Type: "string", Enum: enum},
	}
	registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "read",
			InputSchema: InputSchema{
				Properties: properties,
				Required:   required,
			},
		},
	})

	required[0] = "source-mutated"
	enum[0] = "source-mutated"
	sourceMode := properties["mode"]
	sourceMode.Enum[1] = "source-mutated"
	properties["mode"] = sourceMode

	first := registry.ListTools(ControlLevelReadOnly)
	require.Len(t, first, 1)
	assert.Equal(t, "object", first[0].InputSchema.Type)
	assert.Equal(t, "mode", first[0].InputSchema.Required[0])
	assert.Equal(t, "summary", first[0].InputSchema.Properties["mode"].Enum[0])
	assert.Equal(t, "detail", first[0].InputSchema.Properties["mode"].Enum[1])

	first[0].InputSchema.Required[0] = "listed-mutated"
	listedMode := first[0].InputSchema.Properties["mode"]
	listedMode.Enum[0] = "listed-mutated"
	first[0].InputSchema.Properties["mode"] = listedMode

	second := registry.ListTools(ControlLevelReadOnly)
	require.Len(t, second, 1)
	assert.Equal(t, "mode", second[0].InputSchema.Required[0])
	assert.Equal(t, "summary", second[0].InputSchema.Properties["mode"].Enum[0])
}

func TestControlLevelUsesSharedAgentCapabilityVocabulary(t *testing.T) {
	assert.Equal(t, agentcapabilities.ControlLevelReadOnly, ControlLevelReadOnly)
	assert.Equal(t, agentcapabilities.ControlLevelControlled, ControlLevelControlled)
	assert.Equal(t, agentcapabilities.ControlLevelAutonomous, ControlLevelAutonomous)
	assert.False(t, agentcapabilities.ControlLevelAllowsControlTools(ControlLevelReadOnly))
	assert.True(t, agentcapabilities.ControlLevelAllowsControlTools(ControlLevelControlled))
}

func TestToolActionModeUsesSharedAgentCapabilityVocabulary(t *testing.T) {
	assert.Equal(t, agentcapabilities.ActionModeRead, ToolActionRead)
	assert.Equal(t, agentcapabilities.ActionModeMixed, ToolActionMixed)
	assert.Equal(t, agentcapabilities.ActionModeWrite, ToolActionWrite)
}

func TestToolApprovalPolicyUsesSharedAgentCapabilityVocabulary(t *testing.T) {
	assert.Equal(t, agentcapabilities.ApprovalPolicyScopeOnly, ToolApprovalScopeOnly)
	assert.Equal(t, agentcapabilities.ApprovalPolicyActionPlan, ToolApprovalActionPlan)
}

func TestToolGovernanceUsesSharedAgentCapabilityShape(t *testing.T) {
	var shared agentcapabilities.ToolGovernance = ToolGovernance{
		ActionMode:      ToolActionMixed,
		ApprovalPolicy:  ToolApprovalScopeOnly,
		ApprovalSummary: "read/list actions are safe",
		Summary:         "Reads and refreshes state.",
	}
	assert.Equal(t, agentcapabilities.ActionModeMixed, shared.ActionMode)
	assert.Equal(t, agentcapabilities.ApprovalPolicyScopeOnly, shared.ApprovalPolicy)

	var descriptor agentcapabilities.ToolGovernanceDescriptor = ToolGovernanceDescriptor{
		Name:            "pulse_discovery",
		ActionMode:      ToolActionMixed,
		ApprovalPolicy:  ToolApprovalScopeOnly,
		ApprovalSummary: "no approval required",
	}
	assert.Equal(t, "pulse_discovery", descriptor.Name)
	assert.Equal(t, agentcapabilities.ActionModeMixed, descriptor.ActionMode)
}

func TestToolRegistry_ExecuteControlToolReadOnlyUsesAssistantAndPatrolGuidance(t *testing.T) {
	registry := NewToolRegistry()
	registry.Register(RegisteredTool{
		Definition:     Tool{Name: "control"},
		RequireControl: true,
		Handler: func(ctx context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			t.Fatal("control handler should not run in read-only mode")
			return NewTextResult("unexpected"), nil
		},
	})

	exec := NewPulseToolExecutor(ExecutorConfig{})
	exec.SetControlLevel(ControlLevelReadOnly)

	result, err := registry.Execute(context.Background(), exec, "control", nil)
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	text := result.Content[0].Text
	assert.Equal(t, agentcapabilities.ControlToolsDisabledMessage, text)
	assert.Contains(t, text, "Pulse Intelligence settings")
	assert.Contains(t, text, "Pulse Assistant Permissions > Control mode")
	assert.NotContains(t, text, "Settings > Pulse Assistant")
}

func TestToolRegistryExecuteNormalizesSharedToolCallParams(t *testing.T) {
	registry := NewToolRegistry()
	var gotArgs map[string]interface{}
	var gotArgsLenBeforeMutation int
	registry.Register(RegisteredTool{
		Definition: Tool{Name: "test_tool"},
		Handler: func(ctx context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			gotArgs = args
			gotArgsLenBeforeMutation = len(args)
			args["resource_id"] = "vm/101"
			args["body"].(map[string]interface{})["note"] = "changed"
			return NewTextResult("ok"), nil
		},
	})
	exec := NewPulseToolExecutor(ExecutorConfig{})

	sourceArgs := map[string]interface{}{
		"resource_id": "vm/100",
		"body":        map[string]interface{}{"note": "maintenance"},
	}
	result, err := registry.Execute(context.Background(), exec, " test_tool ", sourceArgs)
	require.NoError(t, err)
	assert.Equal(t, "ok", agentcapabilities.ToolResultText(result))
	assert.Equal(t, "vm/100", sourceArgs["resource_id"], "registry execution must not expose the caller's argument map to handlers")
	assert.Equal(t, "maintenance", sourceArgs["body"].(map[string]interface{})["note"], "registry execution must not expose nested caller arguments to handlers")
	require.NotNil(t, gotArgs)
	assert.Equal(t, "vm/101", gotArgs["resource_id"])
	assert.Equal(t, "changed", gotArgs["body"].(map[string]interface{})["note"])

	gotArgs = nil
	result, err = registry.Execute(context.Background(), exec, "  ", nil)
	require.NoError(t, err)
	interpreted := agentcapabilities.InterpretToolResult(result)
	assert.True(t, interpreted.IsError)
	assert.Contains(t, interpreted.Text, "invalid tools/call params: tool name is required")

	registry.Register(RegisteredTool{
		Definition: Tool{Name: "empty_args"},
		Handler: func(ctx context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			gotArgs = args
			gotArgsLenBeforeMutation = len(args)
			return NewTextResult("ok"), nil
		},
	})
	result, err = registry.Execute(context.Background(), exec, "empty_args", nil)
	require.NoError(t, err)
	assert.Equal(t, "ok", agentcapabilities.ToolResultText(result))
	require.NotNil(t, gotArgs)
	assert.Equal(t, 0, gotArgsLenBeforeMutation)
}

func TestToolRegistryExecuteNormalizesSharedToolResult(t *testing.T) {
	registry := NewToolRegistry()
	handlerContent := []Content{{Type: "text", Text: "ok"}}
	registry.Register(RegisteredTool{
		Definition: Tool{Name: "read"},
		Handler: func(ctx context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return CallToolResult{Content: handlerContent}, nil
		},
	})
	registry.Register(RegisteredTool{
		Definition: Tool{Name: "empty"},
		Handler: func(ctx context.Context, e *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return CallToolResult{}, nil
		},
	})

	exec := NewPulseToolExecutor(ExecutorConfig{})
	result, err := registry.Execute(context.Background(), exec, "read", nil)
	require.NoError(t, err)
	assert.Equal(t, "ok", agentcapabilities.ToolResultText(result))
	result.Content[0].Text = "changed"
	assert.Equal(t, "ok", handlerContent[0].Text)

	empty, err := registry.Execute(context.Background(), exec, "empty", nil)
	require.NoError(t, err)
	assert.NotNil(t, empty.Content)
	assert.Empty(t, empty.Content)
}

func TestToolRegistryExecuteUsesSharedRegistryExecutionResults(t *testing.T) {
	src, err := os.ReadFile("registry.go")
	require.NoError(t, err)
	text := string(src)

	for _, fragment := range []string{
		"agentcapabilities.PrepareToolRegistryExecution(name, args)",
		"agentcapabilities.NewUnknownToolResult(name)",
		"agentcapabilities.NewControlToolsDisabledToolResult()",
	} {
		assert.Contains(t, text, fragment)
	}
	for _, fragment := range []string{
		"PrepareMCPToolRegistryExecution",
		"NewUnknownMCPToolResult",
		"NewControlToolsDisabledMCPToolResult",
	} {
		if strings.Contains(text, fragment) {
			t.Fatalf("native registry execution must use neutral shared helpers; found %s", fragment)
		}
	}
	for _, fragment := range []string{
		"invalid tools/call params:",
		"unknown tool: %s",
		"agentcapabilities.ControlToolsDisabledMessage",
	} {
		if strings.Contains(text, fragment) {
			t.Fatalf("registry execution must use shared registry result helpers; found %s", fragment)
		}
	}
}

func containsTool(tools []Tool, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}
