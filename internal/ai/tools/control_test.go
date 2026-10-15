package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func mustParseJSONMapControl(t *testing.T, text string) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	assert.NoError(t, json.Unmarshal([]byte(text), &out))
	return out
}

func TestPulseToolExecutor_ExecuteListBackups(t *testing.T) {
	resourceID := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, "west", 101)
	point := recovery.RecoveryPoint{ID: "backup-artifact", Provider: recovery.ProviderProxmoxPBS, Kind: recovery.KindBackup, SubjectResourceID: resourceID}
	exec := NewPulseToolExecutor(ExecutorConfig{
		RecoveryPointsProvider: inventoryPointsProviderFunc(func(ctx context.Context, opts recovery.ListPointsOptions) ([]recovery.RecoveryPoint, int, error) {
			assert.Equal(t, recovery.KindBackup, opts.Kind)
			assert.Equal(t, resourceID, opts.SubjectResourceID)
			return []recovery.RecoveryPoint{point}, 1, nil
		}),
		ControlLevel: ControlLevelReadOnly,
	})
	result, err := exec.ExecuteTool(context.Background(), "pulse_storage", map[string]interface{}{
		"type": "backups", "resource_id": resourceID,
	})
	assert.NoError(t, err)
	assert.False(t, result.IsError)
	var response BackupsResponse
	assert.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &response))
	assert.Equal(t, []recovery.RecoveryPoint{point}, response.Points)
}

func TestPulseToolExecutor_ExecuteControlGuest(t *testing.T) {
	stateProv := &mockStateProvider{}
	agentSrv := &mockAgentServer{}
	exec := NewPulseToolExecutor(ExecutorConfig{
		StateProvider:    stateProv,
		AgentServer:      agentSrv,
		ControlLevel:     ControlLevelAutonomous,
		ActionAuditStore: unifiedresources.NewMemoryStore(),
	})

	state := models.StateSnapshot{
		VMs: []models.VM{
			{VMID: 100, Name: "test-vm", Node: "node1", Status: "running", Instance: "pve1"},
		},
	}
	stateProv.On("ReadSnapshot").Return(state)

	agentSrv.On("GetConnectedAgents").Return([]agentexec.ConnectedAgent{
		{AgentID: "agent1", Hostname: "node1"},
	})

	agentSrv.On("ExecuteCommand", mock.Anything, "agent1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
		return payload.Command == "qm stop 100"
	})).Return(&agentexec.CommandResultPayload{
		Stdout:   "OK",
		ExitCode: 0,
	}, nil)

	agentSrv.On("ExecuteCommand", mock.Anything, "agent1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
		return payload.Command == "qm status 100"
	})).Return(&agentexec.CommandResultPayload{
		Stdout:   "status: stopped",
		ExitCode: 0,
	}, nil)

	// Use pulse_control tool with type: "guest"
	result, err := exec.ExecuteTool(context.Background(), "pulse_control", map[string]interface{}{
		"type":     "guest",
		"guest_id": "100",
		"action":   "stop",
	})
	assert.NoError(t, err)
	assert.Contains(t, result.Content[0].Text, "retired and denied")
	agentSrv.AssertNotCalled(t, "ExecuteCommand", mock.Anything, mock.Anything, mock.Anything)
}

func TestPulseToolExecutor_ExecuteControlDocker(t *testing.T) {
	stateProv := &mockStateProvider{}
	agentSrv := &mockAgentServer{}
	exec := NewPulseToolExecutor(ExecutorConfig{
		StateProvider:    stateProv,
		AgentServer:      agentSrv,
		ControlLevel:     ControlLevelAutonomous,
		ActionAuditStore: unifiedresources.NewMemoryStore(),
	})

	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				ID:       "host1",
				Hostname: "docker1",
				Containers: []models.DockerContainer{
					{ID: "c1", Name: "nginx"},
				},
			},
		},
	}
	stateProv.On("ReadSnapshot").Return(state)

	agentSrv.On("GetConnectedAgents").Return([]agentexec.ConnectedAgent{
		{AgentID: "agent1", Hostname: "docker1"},
	})

	agentSrv.On("ExecuteCommand", mock.Anything, "agent1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
		return payload.Command == "docker restart 'nginx'"
	})).Return(&agentexec.CommandResultPayload{
		Stdout:   "OK",
		ExitCode: 0,
	}, nil)

	agentSrv.On("ExecuteCommand", mock.Anything, "agent1", mock.MatchedBy(func(payload agentexec.ExecuteCommandPayload) bool {
		return strings.HasPrefix(payload.Command, "docker inspect -f")
	})).Return(&agentexec.CommandResultPayload{
		Stdout:   "running true",
		ExitCode: 0,
	}, nil)

	// Use pulse_docker tool with action: "control"
	result, err := exec.ExecuteTool(context.Background(), "pulse_docker", map[string]interface{}{
		"action":    "control",
		"container": "nginx",
		"operation": "restart",
	})
	assert.NoError(t, err)
	assert.Contains(t, result.Content[0].Text, "retired and denied")
	agentSrv.AssertNotCalled(t, "ExecuteCommand", mock.Anything, mock.Anything, mock.Anything)
}
