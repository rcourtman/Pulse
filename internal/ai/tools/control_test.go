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

// A read refused because commands were never enabled is not a dropped
// connection. The agent reports, nothing is broken, and the model must be told
// so instead of being sent to repair a connection.
func TestReadRefusalSeparatesCommandsOffFromDisconnected(t *testing.T) {
	t.Setenv("PULSE_STRICT_RESOLUTION", "false")
	const containerID = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	snapshot := func(commandsEnabled bool) models.StateSnapshot {
		host := routingFixtureHost("agent-docker01", "docker01", "")
		host.CommandsEnabled = commandsEnabled
		docker := routingFixtureDocker("agent-docker01", "docker01", "")
		docker.Containers = []models.DockerContainer{{ID: containerID, Name: "web", State: "running", Status: "Up 2 hours"}}
		return models.StateSnapshot{Hosts: []models.Host{host}, DockerHosts: []models.DockerHost{docker}}
	}
	refusal := func(t *testing.T, commandsEnabled bool, operation, target string) ToolResponse {
		t.Helper()
		executor := NewPulseToolExecutor(ExecutorConfig{
			StateProvider: &mockStateProvider{state: snapshot(commandsEnabled)},
			AgentServer:   &mockAgentServer{},
		})
		var result CallToolResult
		var err error
		switch operation {
		case "file read":
			result, err = executor.executeFileRead(context.Background(), "/proc/meminfo", target, "")
		default:
			result, err = executor.executeReadExec(context.Background(), map[string]interface{}{"command": "uptime", "target_host": target})
		}
		assert.NoError(t, err)
		assert.True(t, result.IsError)
		var response ToolResponse
		assert.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &response))
		if response.Error == nil {
			t.Fatalf("%s on %s returned no error: %s", operation, target, result.Content[0].Text)
		}
		return response
	}

	for _, operation := range []string{"file read", "read exec"} {
		for _, target := range []string{"docker01", "web"} {
			t.Run("commands off/"+operation+"/"+target, func(t *testing.T) {
				response := refusal(t, false, operation, target)
				assert.Equal(t, "COMMANDS_NOT_ENABLED", response.Error.Code)
				assert.True(t, response.Error.Failed)
				assert.Equal(t, "host_commands_disabled", response.Error.Details["reason_code"])
				assert.Equal(t, "docker01", response.Error.Details["host"])
				assert.Contains(t, response.Error.Message, "Commands are not enabled on the Pulse agent on docker01.")
				assert.Contains(t, response.Error.Message, "Nothing is disconnected or broken")
				assert.Contains(t, response.Error.Message, "did not run")
				assert.NotContains(t, response.Error.Message, "No command connection")
			})
			t.Run("commands on, session down/"+operation+"/"+target, func(t *testing.T) {
				response := refusal(t, true, operation, target)
				assert.Equal(t, ErrCodeNoAgent, response.Error.Code)
				assert.Equal(t, "command_agent_disconnected", response.Error.Details["reason_code"])
				assert.NotContains(t, response.Error.Message, "not enabled")
				assert.NotContains(t, response.Error.Details, "how_to_enable")
			})
		}
	}

	// The answer names only a host a command would use. An agent that stopped
	// reporting, or a target no reporting agent serves, is not called commands-off.
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot(false))
	if host, disabled := CommandsDisabledHostForTarget(registry, "docker01"); !disabled || host.Name != "docker01" || host.ResourceID == "" {
		t.Fatalf("docker01 = %#v, %v, want commands-off", host, disabled)
	}
	if _, disabled := CommandsDisabledHostForTarget(registry, "no-such-host"); disabled {
		t.Fatal("an unknown target must not read as commands-off")
	}
	if _, disabled := CommandsDisabledHostForTarget(nil, "docker01"); disabled {
		t.Fatal("no read state must not read as commands-off")
	}
}

// A read for a Proxmox guest goes through its node's agent. The read fact
// names that host only when its agent reports commands on, and says the read
// runs inside the guest.
func TestCommandReadHostForTargetFollowsGuestRouting(t *testing.T) {
	snapshot := func(commandsEnabled bool) models.StateSnapshot {
		host := routingFixtureHost("agent-pve1", "pve1", "")
		host.CommandsEnabled = commandsEnabled
		return models.StateSnapshot{
			Nodes:      []models.Node{{ID: "lab-pve1", Instance: "lab", Name: "pve1", Status: "online"}},
			Hosts:      []models.Host{host},
			Containers: []models.Container{{ID: "lab:pve1:111", Instance: "lab", Node: "pve1", VMID: 111, Name: "nextcloud", Type: "lxc", Status: "running"}},
			VMs:        []models.VM{{ID: "lab:pve1:160", Instance: "lab", Node: "pve1", VMID: 160, Name: "builder", Type: "qemu", Status: "running"}},
		}
	}
	on := unifiedresources.NewRegistry(nil)
	on.IngestSnapshot(snapshot(true))
	for target, guest := range map[string]string{"nextcloud": "container", "builder": "vm"} {
		read, ok := CommandReadHostForTarget(on, target)
		if !ok || read.Name != "pve1" || read.ResourceID == "" || read.Guest != guest {
			t.Errorf("%s = %#v, %v, want reads inside the %s through pve1", target, read, ok, guest)
		}
		if _, disabled := CommandsDisabledHostForTarget(on, target); disabled {
			t.Errorf("%s reads as commands-off on an agent with commands on", target)
		}
	}

	off := unifiedresources.NewRegistry(nil)
	off.IngestSnapshot(snapshot(false))
	if read, ok := CommandReadHostForTarget(off, "nextcloud"); ok {
		t.Errorf("commands off still reads: %#v", read)
	}
	if host, disabled := CommandsDisabledHostForTarget(off, "nextcloud"); !disabled || host.Name != "pve1" {
		t.Errorf("commands off = %#v, %v, want pve1", host, disabled)
	}
	if _, ok := CommandReadHostForTarget(on, "no-such-guest"); ok {
		t.Error("an unknown target must not read")
	}
	if _, ok := CommandReadHostForTarget(nil, "nextcloud"); ok {
		t.Error("no read state must not read")
	}
}
