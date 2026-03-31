package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type stubAppContainerReadProvider struct {
	calls  []AppContainerReadRequest
	result *AppContainerReadResult
	err    error
}

func (s *stubAppContainerReadProvider) ReadLogs(_ context.Context, req AppContainerReadRequest) (*AppContainerReadResult, error) {
	s.calls = append(s.calls, req)
	if s.err != nil {
		return nil, s.err
	}
	if s.result == nil {
		return &AppContainerReadResult{
			ResourceID:  req.ResourceID,
			ProviderUID: req.ProviderUID,
			Name:        req.Name,
			Host:        req.Host,
			Platform:    req.Platform,
			Container:   req.Container,
			Lines:       req.Lines,
			Output:      "ok",
		}, nil
	}
	result := *s.result
	return &result, nil
}

func TestPulseToolExecutor_ExecuteReadLogs_Fallbacks(t *testing.T) {
	ctx := context.Background()

	t.Run("DockerSourceWithoutContainerFallsBackToDockerPs", func(t *testing.T) {
		t.Setenv("PULSE_STRICT_RESOLUTION", "false")

		agentSrv := &mockAgentServer{}
		agentSrv.On("GetConnectedAgents").Return([]agentexec.ConnectedAgent{
			{AgentID: "agent1", Hostname: "node1"},
		})
		agentSrv.On("ExecuteCommand", mock.Anything, "agent1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
			return payload.TargetType == "agent" &&
				payload.TargetID == "" &&
				strings.Contains(payload.Command, "docker ps --format") &&
				strings.Contains(payload.Command, "head -20")
		})).Return(&agentexec.CommandResultPayload{
			Stdout:   "container-a\tUp 5h",
			ExitCode: 0,
		}, nil).Once()

		exec := NewPulseToolExecutor(ExecutorConfig{AgentServer: agentSrv})
		result, err := exec.executeReadLogs(ctx, map[string]interface{}{
			"action":      "logs",
			"source":      "docker",
			"target_host": "node1",
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
		require.NotEmpty(t, result.Content)
		assert.Contains(t, result.Content[0].Text, "container-a")
		agentSrv.AssertExpectations(t)
	})

	t.Run("JournalSourceWithoutUnitFallsBackToGlobalJournal", func(t *testing.T) {
		t.Setenv("PULSE_STRICT_RESOLUTION", "false")

		agentSrv := &mockAgentServer{}
		agentSrv.On("GetConnectedAgents").Return([]agentexec.ConnectedAgent{
			{AgentID: "agent1", Hostname: "node1"},
		})
		agentSrv.On("ExecuteCommand", mock.Anything, "agent1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
			return payload.TargetType == "agent" &&
				payload.TargetID == "" &&
				payload.Command == "journalctl --since '1h' -n 50 --no-pager"
		})).Return(&agentexec.CommandResultPayload{
			Stdout:   "journal output",
			ExitCode: 0,
		}, nil).Once()

		exec := NewPulseToolExecutor(ExecutorConfig{AgentServer: agentSrv})
		result, err := exec.executeReadLogs(ctx, map[string]interface{}{
			"action":      "logs",
			"source":      "journal",
			"target_host": "node1",
			"since":       "1h",
			"lines":       50,
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
		require.NotEmpty(t, result.Content)
		assert.Contains(t, result.Content[0].Text, "journal output")
		agentSrv.AssertExpectations(t)
	})

	t.Run("MissingSourceInfersDockerAndUnknownSourceFallsBackToJournal", func(t *testing.T) {
		t.Setenv("PULSE_STRICT_RESOLUTION", "false")

		agentSrv := &mockAgentServer{}
		agentSrv.On("GetConnectedAgents").Return([]agentexec.ConnectedAgent{
			{AgentID: "agent1", Hostname: "node1"},
		})
		agentSrv.On("ExecuteCommand", mock.Anything, "agent1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
			return payload.Command == "docker logs --tail 100 'homepage'" && payload.TargetType == "agent"
		})).Return(&agentexec.CommandResultPayload{
			Stdout:   "docker log line",
			ExitCode: 0,
		}, nil).Once()
		agentSrv.On("ExecuteCommand", mock.Anything, "agent1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
			return payload.Command == "journalctl -n 30 --no-pager" && payload.TargetType == "agent"
		})).Return(&agentexec.CommandResultPayload{
			Stdout:   "journal fallback line",
			ExitCode: 0,
		}, nil).Once()

		exec := NewPulseToolExecutor(ExecutorConfig{AgentServer: agentSrv})

		result, err := exec.executeReadLogs(ctx, map[string]interface{}{
			"action":      "logs",
			"target_host": "node1",
			"container":   "homepage",
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
		require.NotEmpty(t, result.Content)
		assert.Contains(t, result.Content[0].Text, "docker log line")

		result, err = exec.executeReadLogs(ctx, map[string]interface{}{
			"action":      "logs",
			"source":      "syslog",
			"target_host": "node1",
			"lines":       30,
		})
		require.NoError(t, err)
		assert.False(t, result.IsError)
		require.NotEmpty(t, result.Content)
		assert.Contains(t, result.Content[0].Text, "journal fallback line")

		agentSrv.AssertExpectations(t)
	})
}

func TestPulseToolExecutor_ExecuteReadRejectsLegacyAppContainerArg(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{})

	result, err := exec.executeReadExec(context.Background(), map[string]interface{}{
		"action":        "exec",
		"command":       "uptime",
		"target_host":   "node1",
		"app_container": "homepage",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	require.NotEmpty(t, result.Content)
	assert.Contains(t, result.Content[0].Text, "app_container is no longer supported; use app-container")
}

func TestPulseToolExecutor_ListTools_IncludesPulseReadForNativeAppReadProvider(t *testing.T) {
	provider := newTrueNASUnifiedQueryProvider(t)
	exec := NewPulseToolExecutor(ExecutorConfig{
		UnifiedResourceProvider:  provider,
		ReadState:                provider.ResourceRegistry,
		AppContainerReadProvider: &stubAppContainerReadProvider{},
	})

	tools := exec.ListTools()
	assert.True(t, containsTool(tools, "pulse_read"))
}

func TestExecuteReadLogs_TrueNASAppUsesNativeReadProvider(t *testing.T) {
	provider := newTrueNASUnifiedQueryProvider(t)
	resolved := &mockResolvedContext{
		resources: make(map[string]ResolvedResourceInfo),
		aliases:   make(map[string]ResolvedResourceInfo),
	}
	readProvider := &stubAppContainerReadProvider{
		result: &AppContainerReadResult{
			ResourceID:  "app-container:truenas-main:nextcloud",
			ProviderUID: "nextcloud",
			Name:        "Nextcloud",
			Host:        "truenas-main",
			Platform:    "truenas",
			Container:   "nextcloud",
			Lines:       25,
			Output:      "2026-03-29T18:00:00Z ready\n2026-03-29T18:01:00Z serving",
		},
	}

	exec := NewPulseToolExecutor(ExecutorConfig{
		UnifiedResourceProvider:  provider,
		ReadState:                provider.ResourceRegistry,
		AppContainerReadProvider: readProvider,
	})
	exec.SetResolvedContext(resolved)

	if _, err := exec.executeGetResource(context.Background(), map[string]interface{}{
		"resource_type": "app-container",
		"resource_id":   "nextcloud",
	}); err != nil {
		t.Fatalf("seed resolved context: unexpected error: %v", err)
	}

	result, err := exec.executeReadLogs(context.Background(), map[string]interface{}{
		"action":      "logs",
		"resource_id": "Nextcloud",
		"container":   "nextcloud",
		"lines":       25,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	require.NotEmpty(t, result.Content)
	assert.Contains(t, result.Content[0].Text, "Logs from app 'Nextcloud' (container 'nextcloud') (last 25 lines):")
	assert.Contains(t, result.Content[0].Text, "serving")

	if len(readProvider.calls) != 1 {
		t.Fatalf("expected one native app read call, got %+v", readProvider.calls)
	}
	call := readProvider.calls[0]
	if call.OrgID != "default" || call.ProviderUID != "nextcloud" || call.Host != "truenas-main" || call.Container != "nextcloud" || call.Lines != 25 {
		t.Fatalf("unexpected native app read request: %+v", call)
	}
}

func TestExecuteReadLogs_VMwareResourcesReturnStructuredQueryGuidance(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		resourceID   string
		resourceRef  string
		wantKind     string
	}{
		{name: "vm", resourceType: "vm", resourceID: "app-01", resourceRef: "app-01", wantKind: "vm"},
		{name: "agent", resourceType: "agent", resourceID: "esxi-01.lab.local", resourceRef: "esxi-01.lab.local", wantKind: "agent"},
		{name: "storage", resourceType: "storage", resourceID: "nvme-primary", resourceRef: "nvme-primary", wantKind: "storage"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := newVMwareUnifiedQueryProvider(t)
			resolved := &mockResolvedContext{
				resources: make(map[string]ResolvedResourceInfo),
				aliases:   make(map[string]ResolvedResourceInfo),
			}
			exec := NewPulseToolExecutor(ExecutorConfig{
				UnifiedResourceProvider: provider,
				ReadState:               provider.ResourceRegistry,
			})
			exec.SetResolvedContext(resolved)

			_, err := exec.executeGetResource(context.Background(), map[string]interface{}{
				"resource_type": tc.resourceType,
				"resource_id":   tc.resourceID,
			})
			require.NoError(t, err)

			result, err := exec.executeReadLogs(context.Background(), map[string]interface{}{
				"action":      "logs",
				"resource_id": tc.resourceRef,
			})
			require.NoError(t, err)
			assert.True(t, result.IsError)

			payload := decodeToolResponse(t, result)
			require.NotNil(t, payload.Error)
			assert.Equal(t, ErrCodeActionNotAllowed, payload.Error.Code)
			assert.True(t, payload.Error.Blocked)
			assert.Contains(t, payload.Error.Message, "does not expose native logs through pulse_read")
			assert.Equal(t, tc.wantKind, payload.Error.Details["resource_type"])
			assert.Equal(t, "vmware-vsphere", payload.Error.Details["adapter"])

			hint, ok := payload.Error.Details["recovery_hint"].(string)
			require.True(t, ok)
			assert.Contains(t, hint, `pulse_query action=get`)
			assert.Contains(t, hint, `resource_type="`+tc.wantKind+`"`)
		})
	}
}

func TestExecuteReadLogs_NonNativeAppContainersReturnStructuredTargetHostGuidance(t *testing.T) {
	resolved := &mockResolvedContext{
		resources: make(map[string]ResolvedResourceInfo),
		aliases:   make(map[string]ResolvedResourceInfo),
	}
	resource := &mockResource{
		resourceID:     "app-container:docker-host:homepage",
		resourceType:   "app-container",
		targetHost:     "docker-host",
		adapter:        "docker",
		allowedActions: []string{"query", "get", "start", "stop", "restart"},
		providerUID:    "homepage",
		kind:           "app-container",
		aliases:        []string{"homepage"},
	}
	resolved.resources[resource.resourceID] = resource
	resolved.aliases["homepage"] = resource

	exec := NewPulseToolExecutor(ExecutorConfig{})
	exec.SetResolvedContext(resolved)

	result, err := exec.executeReadLogs(context.Background(), map[string]interface{}{
		"action":      "logs",
		"resource_id": "homepage",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)

	payload := decodeToolResponse(t, result)
	require.NotNil(t, payload.Error)
	assert.Equal(t, ErrCodeActionNotAllowed, payload.Error.Code)
	assert.True(t, payload.Error.Blocked)
	assert.Contains(t, payload.Error.Message, "does not expose native app logs through pulse_read resource_id")
	assert.Equal(t, "app-container", payload.Error.Details["resource_type"])
	assert.Equal(t, "docker-host", payload.Error.Details["target_host"])
	assert.Equal(t, "homepage", payload.Error.Details["container"])

	hint, ok := payload.Error.Details["recovery_hint"].(string)
	require.True(t, ok)
	assert.Contains(t, hint, `target_host="docker-host"`)
	assert.Contains(t, hint, `container="homepage"`)
}

func decodeToolResponse(t *testing.T, result CallToolResult) ToolResponse {
	t.Helper()
	require.NotEmpty(t, result.Content)

	var payload ToolResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &payload))
	return payload
}
