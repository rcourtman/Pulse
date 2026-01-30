package tools

import (
	"context"
	"testing"

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

	storageProvider := &stubStorageProvider{}
	exec.SetStorageProvider(storageProvider)
	assert.Equal(t, storageProvider, exec.storageProvider)

	diskHealthProvider := &mockDiskHealthProvider{}
	exec.SetDiskHealthProvider(diskHealthProvider)
	assert.Equal(t, diskHealthProvider, exec.diskHealthProvider)

	agentProfileManager := &stubAgentProfileManager{}
	exec.SetAgentProfileManager(agentProfileManager)
	assert.Equal(t, agentProfileManager, exec.agentProfileManager)

	updatesProvider := &mockUpdatesProvider{}
	exec.SetUpdatesProvider(updatesProvider)
	assert.Equal(t, updatesProvider, exec.updatesProvider)
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

	full := registry.ListTools(ControlLevelControlled)
	require.Len(t, full, 2)
	assert.Equal(t, "read", full[0].Name)
	assert.Equal(t, "control", full[1].Name)
}

func containsTool(tools []Tool, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}
