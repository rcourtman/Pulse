package tools

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestFileTools_Registry(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{})
	exec.registerFileTools()
	tools := exec.registry.ListTools(ControlLevelControlled)

	found := false
	for _, tool := range tools {
		if tool.Name == "pulse_file_edit" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestExecuteFileEdit_Validation(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr string
	}{
		{
			name:    "Missing Path",
			args:    map[string]interface{}{"action": "read", "target_host": "h1"},
			wantErr: "path is required",
		},
		{
			name:    "Missing TargetHost",
			args:    map[string]interface{}{"action": "read", "path": "/etc/config"},
			wantErr: "target_host is required",
		},
		{
			name:    "Relative Path",
			args:    map[string]interface{}{"action": "read", "path": "etc/config", "target_host": "h1"},
			wantErr: "path must be absolute",
		},
		{
			name:    "Invalid Docker Container",
			args:    map[string]interface{}{"action": "read", "path": "/f", "target_host": "h1", "docker_container": "bad name!"},
			wantErr: "invalid character",
		},
		{
			name:    "Unknown Action",
			args:    map[string]interface{}{"action": "dance", "path": "/f", "target_host": "h1"},
			wantErr: "unknown action",
		},
		{
			name:    "Append Missing Content",
			args:    map[string]interface{}{"action": "append", "path": "/f", "target_host": "h1"},
			wantErr: "content is required",
		},
		{
			name:    "Write Missing Content",
			args:    map[string]interface{}{"action": "write", "path": "/f", "target_host": "h1"},
			wantErr: "content is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := exec.executeFileEdit(context.Background(), tc.args)
			// Implementation returns error result (NewErrorResult), err is usually nil
			assert.NoError(t, err)
			assert.True(t, res.IsError)
			assert.NotEmpty(t, res.Content)
			assert.Contains(t, res.Content[0].Text, tc.wantErr)
		})
	}
}

func TestValidateWriteExecutionContext_Blocked(t *testing.T) {
	// This tests the security invariant: don't write to host when target is LXC

	state := models.StateSnapshot{
		Containers: []models.Container{
			{VMID: 100, Name: "my-lxc", Node: "pve1", Status: "running"},
		},
		Nodes: []models.Node{
			{Name: "pve1"},
		},
	}

	stateProvider := &mockStateProvider{state: state}
	agentServer := &mockAgentServer{}
	agentServer.agents = []agentexec.ConnectedAgent{
		{AgentID: "agent-1", Hostname: "my-lxc"}, // Agent hostname matches LXC name! Risks confusion.
	}

	exec := NewPulseToolExecutor(ExecutorConfig{
		AgentServer:   agentServer,
		StateProvider: stateProvider,
	})

	// We expect resolveTargetForCommandFull to map "my-lxc" to agent-1
	// But since agent-1 matches hostname "my-lxc" directly, it might be resolved as "direct".
	// Wait, resolveTargetForCommandFull logic logic isn't mocked, it's real code in executor.go using state.

	// If ResolveResource says it's an LXC on pve1.
	// And FindAgent says agent-1 (hostname=my-lxc) exists.
	// The fallback logic in resolveTarget might pick agent-1 if it matches hostname directly?

	// Actually, `resolveTargetForCommandFull` calls `ResolveResource`.
	// Then checks if agent exists for that resource.

	// If I want to trigger the BLOCK, I need:
	// 1. ResolveResource returns LXC.
	// 2. Routing returns "direct" transport on "host" type.

	// This happens if `findAgent` returns an agent that is NOT the node agent?
	// Or if the node agent is found via hostname match of the LXC name?

	// Let's manually invoke validateWriteExecutionContext to test logic in isolation.

	err := exec.validateWriteExecutionContext("my-lxc", CommandRoutingResult{
		Transport:     "direct",
		TargetType:    "host", // Agent matched directly, assuming it's a host
		AgentHostname: "my-lxc",
		AgentID:       "agent-1",
		ResolvedKind:  "lxc", // But resolving matched LXC
		ResolvedNode:  "pve1",
	})

	assert.NotNil(t, err)
	assert.Contains(t, err.Message, "write would execute on the Proxmox node instead of inside the lxc")
}
