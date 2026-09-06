package tools

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestExecuteFileEditDockerContainerValidation(t *testing.T) {
	ctx := context.Background()

	t.Run("InvalidDockerContainerName", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})
		result, err := exec.executeFileEdit(ctx, map[string]interface{}{
			"action":      "read",
			"path":        "/config/test.json",
			"target_host": "tower",
			"container":   "my container", // space is invalid
		})
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "invalid character")
	})

	t.Run("ValidDockerContainerName", func(t *testing.T) {
		// This should pass validation but fail on agent lookup
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})
		result, err := exec.executeFileEdit(ctx, map[string]interface{}{
			"action":      "read",
			"path":        "/config/test.json",
			"target_host": "tower",
			"container":   "my-container_v1.2",
		})
		require.NoError(t, err)
		// Should fail with "no agent" not "invalid character"
		assert.NotContains(t, result.Content[0].Text, "invalid character")
	})
}

func TestExecuteFileReadDocker(t *testing.T) {
	ctx := context.Background()

	t.Run("ReadFromDockerContainer", func(t *testing.T) {
		agents := []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "tower"}}
		mockAgent := &mockAgentServer{}
		mockAgent.On("GetConnectedAgents").Return(agents)
		mockAgent.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			// Should wrap with docker exec
			return strings.Contains(cmd.Command, "docker exec") &&
				strings.Contains(cmd.Command, "jellyfin") &&
				strings.Contains(cmd.Command, "cat") &&
				strings.Contains(cmd.Command, "/config/settings.json")
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   `{"setting": "value"}`,
		}, nil)

		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider:    &mockStateProvider{state: models.StateSnapshot{}},
			AgentServer:      mockAgent,
			ControlLevel:     ControlLevelAutonomous,
			ActionAuditStore: unifiedresources.NewMemoryStore(),
		})
		result, err := exec.executeFileRead(ctx, "/config/settings.json", "tower", "jellyfin")
		require.NoError(t, err)

		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
		assert.True(t, resp["success"].(bool))
		assert.Equal(t, "/config/settings.json", resp["path"])
		assert.Equal(t, "jellyfin", resp["container"])
		assert.Equal(t, `{"setting": "value"}`, resp["content"])
		mockAgent.AssertExpectations(t)
	})

	t.Run("ReadFromHostWithoutDocker", func(t *testing.T) {
		agents := []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "tower"}}
		mockAgent := &mockAgentServer{}
		mockAgent.On("GetConnectedAgents").Return(agents)
		mockAgent.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			// Should NOT wrap with docker exec
			return !strings.Contains(cmd.Command, "docker exec") &&
				strings.Contains(cmd.Command, "cat") &&
				strings.Contains(cmd.Command, "/etc/hostname")
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "tower",
		}, nil)

		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider:    &mockStateProvider{state: models.StateSnapshot{}},
			AgentServer:      mockAgent,
			ControlLevel:     ControlLevelAutonomous,
			ActionAuditStore: unifiedresources.NewMemoryStore(),
		})
		result, err := exec.executeFileRead(ctx, "/etc/hostname", "tower", "") // empty container
		require.NoError(t, err)

		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
		assert.True(t, resp["success"].(bool))
		assert.Nil(t, resp["container"]) // should not be in response
		mockAgent.AssertExpectations(t)
	})

	t.Run("DockerContainerNotFound", func(t *testing.T) {
		agents := []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "tower"}}
		mockAgent := &mockAgentServer{}
		mockAgent.On("GetConnectedAgents").Return(agents)
		mockAgent.On("ExecuteCommand", mock.Anything, "agent-1", mock.Anything).Return(&agentexec.CommandResultPayload{
			ExitCode: 1,
			Stderr:   "Error: No such container: nonexistent",
		}, nil)

		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider:    &mockStateProvider{state: models.StateSnapshot{}},
			AgentServer:      mockAgent,
			ControlLevel:     ControlLevelAutonomous,
			ActionAuditStore: unifiedresources.NewMemoryStore(),
		})
		result, err := exec.executeFileRead(ctx, "/config/test.json", "tower", "nonexistent")
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "Failed to read file from container 'nonexistent'")
		assert.Contains(t, result.Content[0].Text, "No such container")
		mockAgent.AssertExpectations(t)
	})
}

// Failure status must agree with the evidence returned to the model and UI.
func TestExecuteFileReadFailureStatus(t *testing.T) {
	t.Run("missing agent", func(t *testing.T) {
		agent := &mockAgentServer{}
		agent.On("GetConnectedAgents").Return([]agentexec.ConnectedAgent{})
		executor := NewPulseToolExecutor(ExecutorConfig{
			StateProvider: &mockStateProvider{state: models.StateSnapshot{}},
			AgentServer:   agent,
		})
		result, err := executor.executeFileRead(context.Background(), "/proc/meminfo", "delly2", "")
		require.NoError(t, err)
		require.True(t, result.IsError, "missing agent must not count as a successful read")
		assert.Contains(t, result.Content[0].Text, "No command connection is available")
		agent.AssertNotCalled(t, "ExecuteCommand", mock.Anything, mock.Anything, mock.Anything)
	})

	for _, container := range []string{"", "service"} {
		for _, stderr := range []bool{false, true} {
			t.Run(fmt.Sprintf("container=%s/stderr=%t", container, stderr), func(t *testing.T) {
				agent := &mockAgentServer{}
				agent.On("GetConnectedAgents").Return([]agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "tower"}})
				command := &agentexec.CommandResultPayload{ExitCode: 1}
				if stderr {
					command.Stderr = "permission denied"
				} else {
					command.Stdout = "permission denied"
				}
				agent.On("ExecuteCommand", mock.Anything, "agent-1", mock.Anything).Return(command, nil)
				executor := NewPulseToolExecutor(ExecutorConfig{
					StateProvider: &mockStateProvider{state: models.StateSnapshot{}},
					AgentServer:   agent,
				})
				result, err := executor.executeFileRead(context.Background(), "/proc/meminfo", "tower", container)
				require.NoError(t, err)
				require.True(t, result.IsError, "nonzero command exit must not count as file evidence")
				assert.Contains(t, result.Content[0].Text, "permission denied")
				assert.Contains(t, result.Content[0].Text, "exit code 1")
				agent.AssertExpectations(t)
			})
		}
	}
}

func TestExecuteFileWriteDocker(t *testing.T) {
	ctx := context.Background()

	t.Run("WriteToDockerContainer", func(t *testing.T) {
		content := `{"new": "config"}`
		encodedContent := base64.StdEncoding.EncodeToString([]byte(content))
		expectedWriteCmd := fmt.Sprintf(
			"docker exec %s sh -c %s",
			shellEscape("nginx"),
			shellEscape(fmt.Sprintf("echo %s | base64 -d > %s", shellEscape(encodedContent), shellEscape("/etc/nginx/nginx.conf"))),
		)
		expected := sha256.Sum256([]byte(content))
		expectedHex := hex.EncodeToString(expected[:])

		agents := []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "tower"}}
		mockAgent := &mockAgentServer{}
		mockAgent.On("GetConnectedAgents").Return(agents)
		mockAgent.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return cmd.Command == expectedWriteCmd
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "",
		}, nil)
		mockAgent.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return strings.Contains(cmd.Command, "docker exec") &&
				strings.Contains(cmd.Command, "nginx") &&
				strings.Contains(cmd.Command, "sha256sum") &&
				strings.Contains(cmd.Command, "/etc/nginx/nginx.conf")
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   expectedHex + "  /etc/nginx/nginx.conf\n",
		}, nil)

		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider:    &mockStateProvider{state: models.StateSnapshot{}},
			AgentServer:      mockAgent,
			ControlLevel:     ControlLevelAutonomous,
			ActionAuditStore: unifiedresources.NewMemoryStore(),
		})
		result, err := exec.executeFileWrite(ctx, "/etc/nginx/nginx.conf", content, "tower", "nginx", map[string]interface{}{})
		require.NoError(t, err)

		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
		assert.True(t, resp["success"].(bool))
		assert.Equal(t, "write", resp["action"])
		assert.Equal(t, "nginx", resp["container"])
		assert.Equal(t, float64(len(content)), resp["bytes_written"])
		if v, ok := resp["verification"].(map[string]interface{}); ok {
			assert.True(t, v["ok"].(bool))
		}
		mockAgent.AssertExpectations(t)
	})

	t.Run("WriteControlledRequiresApproval", func(t *testing.T) {
		agents := []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "tower"}}
		mockAgent := &mockAgentServer{}
		mockAgent.On("GetConnectedAgents").Return(agents)

		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider: &mockStateProvider{state: models.StateSnapshot{}},
			AgentServer:   mockAgent,
			ControlLevel:  ControlLevelControlled,
		})
		result, err := exec.executeFileWrite(ctx, "/config/test.json", "test", "tower", "mycontainer", map[string]interface{}{})
		require.NoError(t, err)
		assert.Contains(t, result.Content[0].Text, "APPROVAL_REQUIRED")
		assert.Contains(t, result.Content[0].Text, "container: mycontainer")
	})
}

func TestExecuteFileAppendDocker(t *testing.T) {
	ctx := context.Background()

	t.Run("AppendToDockerContainer", func(t *testing.T) {
		content := "\nnew line"
		encodedContent := base64.StdEncoding.EncodeToString([]byte(content))
		expectedAppendCmd := fmt.Sprintf(
			"docker exec %s sh -c %s",
			shellEscape("logcontainer"),
			shellEscape(fmt.Sprintf("echo %s | base64 -d >> %s", shellEscape(encodedContent), shellEscape("/var/log/app.log"))),
		)
		expected := sha256.Sum256([]byte(content))
		expectedHex := hex.EncodeToString(expected[:])

		agents := []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "tower"}}
		mockAgent := &mockAgentServer{}
		mockAgent.On("GetConnectedAgents").Return(agents)
		mockAgent.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return cmd.Command == expectedAppendCmd
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "",
		}, nil)
		mockAgent.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return strings.Contains(cmd.Command, "docker exec") &&
				strings.Contains(cmd.Command, "logcontainer") &&
				strings.Contains(cmd.Command, "tail -c") &&
				(strings.Contains(cmd.Command, "sha256sum") || strings.Contains(cmd.Command, "shasum"))
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   expectedHex + "  -\n",
		}, nil)

		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider:    &mockStateProvider{state: models.StateSnapshot{}},
			AgentServer:      mockAgent,
			ControlLevel:     ControlLevelAutonomous,
			ActionAuditStore: unifiedresources.NewMemoryStore(),
		})
		result, err := exec.executeFileAppend(ctx, "/var/log/app.log", content, "tower", "logcontainer", map[string]interface{}{})
		require.NoError(t, err)

		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
		assert.True(t, resp["success"].(bool))
		assert.Equal(t, "append", resp["action"])
		assert.Equal(t, "logcontainer", resp["container"])
		if v, ok := resp["verification"].(map[string]interface{}); ok {
			assert.True(t, v["ok"].(bool))
		}
		mockAgent.AssertExpectations(t)
	})
}

func TestExecuteFileWriteLXCVMTargets(t *testing.T) {
	ctx := context.Background()

	t.Run("WriteToLXCRoutedCorrectly", func(t *testing.T) {
		// Test that file writes to LXC are routed with correct target type/ID
		// Agent handles sh -c wrapping, so tool sends raw pipeline command
		content := "test content"
		encodedContent := base64.StdEncoding.EncodeToString([]byte(content))
		expected := sha256.Sum256([]byte(content))
		expectedHex := hex.EncodeToString(expected[:])

		agents := []agentexec.ConnectedAgent{{AgentID: "proxmox-agent", Hostname: "pve-node"}}
		mockAgent := &mockAgentServer{}
		mockAgent.On("GetConnectedAgents").Return(agents)
		mockAgent.On("ExecuteCommand", mock.Anything, "proxmox-agent", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			// Tool sends raw pipeline, agent wraps in sh -c for LXC
			return cmd.TargetType == "container" &&
				cmd.TargetID == "141" &&
				strings.Contains(cmd.Command, encodedContent) &&
				strings.Contains(cmd.Command, "| base64 -d >") &&
				!strings.Contains(cmd.Command, "docker exec")
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "",
		}, nil)
		mockAgent.On("ExecuteCommand", mock.Anything, "proxmox-agent", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return cmd.TargetType == "container" &&
				cmd.TargetID == "141" &&
				strings.Contains(cmd.Command, "sha256sum") &&
				strings.Contains(cmd.Command, "/opt/test/config.yaml") &&
				!strings.Contains(cmd.Command, "docker exec")
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   expectedHex + "  /opt/test/config.yaml\n",
		}, nil)

		state := models.StateSnapshot{
			Containers: []models.Container{
				{VMID: 141, Name: "homepage-docker", Node: "pve-node"},
			},
		}

		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider:    &mockStateProvider{state: state},
			AgentServer:      mockAgent,
			ControlLevel:     ControlLevelAutonomous,
			ActionAuditStore: unifiedresources.NewMemoryStore(),
		})
		result, err := exec.executeFileWrite(ctx, "/opt/test/config.yaml", content, "homepage-docker", "", map[string]interface{}{})
		require.NoError(t, err)

		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
		assert.True(t, resp["success"].(bool))
		assert.Equal(t, "write", resp["action"])
		assert.Nil(t, resp["container"]) // No Docker container
		if v, ok := resp["verification"].(map[string]interface{}); ok {
			assert.True(t, v["ok"].(bool))
		}
		mockAgent.AssertExpectations(t)
	})

	t.Run("WriteToVMRoutedCorrectly", func(t *testing.T) {
		// Test that file writes to VMs are routed with correct target type/ID
		content := "vm config"
		encodedContent := base64.StdEncoding.EncodeToString([]byte(content))
		expected := sha256.Sum256([]byte(content))
		expectedHex := hex.EncodeToString(expected[:])

		agents := []agentexec.ConnectedAgent{{AgentID: "proxmox-agent", Hostname: "pve-node"}}
		mockAgent := &mockAgentServer{}
		mockAgent.On("GetConnectedAgents").Return(agents)
		mockAgent.On("ExecuteCommand", mock.Anything, "proxmox-agent", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return cmd.TargetType == "vm" &&
				cmd.TargetID == "100" &&
				strings.Contains(cmd.Command, encodedContent)
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "",
		}, nil)
		mockAgent.On("ExecuteCommand", mock.Anything, "proxmox-agent", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return cmd.TargetType == "vm" &&
				cmd.TargetID == "100" &&
				strings.Contains(cmd.Command, "sha256sum") &&
				strings.Contains(cmd.Command, "/etc/test.conf") &&
				!strings.Contains(cmd.Command, "docker exec")
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   expectedHex + "  /etc/test.conf\n",
		}, nil)

		state := models.StateSnapshot{
			VMs: []models.VM{
				{VMID: 100, Name: "test-vm", Node: "pve-node"},
			},
		}

		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider:    &mockStateProvider{state: state},
			AgentServer:      mockAgent,
			ControlLevel:     ControlLevelAutonomous,
			ActionAuditStore: unifiedresources.NewMemoryStore(),
		})
		result, err := exec.executeFileWrite(ctx, "/etc/test.conf", content, "test-vm", "", map[string]interface{}{})
		require.NoError(t, err)

		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
		assert.True(t, resp["success"].(bool))
		if v, ok := resp["verification"].(map[string]interface{}); ok {
			assert.True(t, v["ok"].(bool))
		}
		mockAgent.AssertExpectations(t)
	})

	t.Run("WriteToDirectHost", func(t *testing.T) {
		// Direct host writes use raw pipeline command
		content := "host config"
		encodedContent := base64.StdEncoding.EncodeToString([]byte(content))
		expected := sha256.Sum256([]byte(content))
		expectedHex := hex.EncodeToString(expected[:])

		agents := []agentexec.ConnectedAgent{{AgentID: "agent", Hostname: "tower"}}
		mockAgent := &mockAgentServer{}
		mockAgent.On("GetConnectedAgents").Return(agents)
		mockAgent.On("ExecuteCommand", mock.Anything, "agent", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return cmd.TargetType == "agent" &&
				strings.Contains(cmd.Command, encodedContent) &&
				strings.Contains(cmd.Command, "| base64 -d >")
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "",
		}, nil)
		mockAgent.On("ExecuteCommand", mock.Anything, "agent", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return cmd.TargetType == "agent" &&
				strings.Contains(cmd.Command, "sha256sum") &&
				strings.Contains(cmd.Command, "/tmp/test.txt") &&
				!strings.Contains(cmd.Command, "docker exec")
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   expectedHex + "  /tmp/test.txt\n",
		}, nil)

		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider:    &mockStateProvider{state: models.StateSnapshot{}},
			AgentServer:      mockAgent,
			ControlLevel:     ControlLevelAutonomous,
			ActionAuditStore: unifiedresources.NewMemoryStore(),
		})
		result, err := exec.executeFileWrite(ctx, "/tmp/test.txt", content, "tower", "", map[string]interface{}{})
		require.NoError(t, err)

		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
		assert.True(t, resp["success"].(bool))
		if v, ok := resp["verification"].(map[string]interface{}); ok {
			assert.True(t, v["ok"].(bool))
		}
		mockAgent.AssertExpectations(t)
	})

	t.Run("AppendToLXCRoutedCorrectly", func(t *testing.T) {
		// Append operations to LXC are routed with correct target type/ID
		content := "\nnew line"
		encodedContent := base64.StdEncoding.EncodeToString([]byte(content))
		expected := sha256.Sum256([]byte(content))
		expectedHex := hex.EncodeToString(expected[:])

		agents := []agentexec.ConnectedAgent{{AgentID: "proxmox-agent", Hostname: "pve-node"}}
		mockAgent := &mockAgentServer{}
		mockAgent.On("GetConnectedAgents").Return(agents)
		mockAgent.On("ExecuteCommand", mock.Anything, "proxmox-agent", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return cmd.TargetType == "container" &&
				cmd.TargetID == "141" &&
				strings.Contains(cmd.Command, encodedContent) &&
				strings.Contains(cmd.Command, ">>") // append uses >>
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "",
		}, nil)
		mockAgent.On("ExecuteCommand", mock.Anything, "proxmox-agent", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return cmd.TargetType == "container" &&
				cmd.TargetID == "141" &&
				strings.Contains(cmd.Command, "tail -c") &&
				strings.Contains(cmd.Command, "/var/log/app.log") &&
				(strings.Contains(cmd.Command, "sha256sum") || strings.Contains(cmd.Command, "shasum")) &&
				!strings.Contains(cmd.Command, "docker exec")
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   expectedHex + "  -\n",
		}, nil)

		state := models.StateSnapshot{
			Containers: []models.Container{
				{VMID: 141, Name: "homepage-docker", Node: "pve-node"},
			},
		}

		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider:    &mockStateProvider{state: state},
			AgentServer:      mockAgent,
			ControlLevel:     ControlLevelAutonomous,
			ActionAuditStore: unifiedresources.NewMemoryStore(),
		})
		result, err := exec.executeFileAppend(ctx, "/var/log/app.log", content, "homepage-docker", "", map[string]interface{}{})
		require.NoError(t, err)

		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
		assert.True(t, resp["success"].(bool))
		assert.Equal(t, "append", resp["action"])
		if v, ok := resp["verification"].(map[string]interface{}); ok {
			assert.True(t, v["ok"].(bool))
		}
		mockAgent.AssertExpectations(t)
	})
}

func TestExecuteFileEditDockerNestedRouting(t *testing.T) {
	ctx := context.Background()

	t.Run("DockerInsideLXC", func(t *testing.T) {
		// Test case: Docker running inside an LXC container
		// target_host="homepage-docker" (LXC), container="nginx"
		// Command should route through Proxmox node agent with LXC target type

		agents := []agentexec.ConnectedAgent{{AgentID: "proxmox-agent", Hostname: "pve-node"}}
		mockAgent := &mockAgentServer{}
		mockAgent.On("GetConnectedAgents").Return(agents)
		mockAgent.On("ExecuteCommand", mock.Anything, "proxmox-agent", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			// Should have container target type for LXC routing
			// and command should include docker exec
			return cmd.TargetType == "container" &&
				cmd.TargetID == "141" &&
				strings.Contains(cmd.Command, "docker exec") &&
				strings.Contains(cmd.Command, "nginx")
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "file content",
		}, nil)

		state := models.StateSnapshot{
			Containers: []models.Container{
				{VMID: 141, Name: "homepage-docker", Node: "pve-node"},
			},
		}

		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider:    &mockStateProvider{state: state},
			AgentServer:      mockAgent,
			ControlLevel:     ControlLevelAutonomous,
			ActionAuditStore: unifiedresources.NewMemoryStore(),
		})
		result, err := exec.executeFileRead(ctx, "/config/test.json", "homepage-docker", "nginx")
		require.NoError(t, err)

		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
		assert.True(t, resp["success"].(bool))
		assert.Equal(t, "nginx", resp["container"])
		mockAgent.AssertExpectations(t)
	})
}

// Missing access is a failed operation, independent of whether monitoring knows
// the target. Exercise the actual tool handlers so writes cannot report success.
func TestCommandToolsPreserveMonitoringWhenAccessUnavailable(t *testing.T) {
	t.Setenv("PULSE_STRICT_RESOLUTION", "false")
	for _, target := range []string{"known-vm", "unknown-target"} {
		for _, noServer := range []bool{false, true} {
			for _, operation := range []string{"file read", "file write", "file append", "read exec", "legacy command"} {
				t.Run(fmt.Sprintf("%s/noServer=%t/%s", target, noServer, operation), func(t *testing.T) {
					server := &mockAgentServer{}
					config := ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{
						VMs: []models.VM{{ID: "vm-101", VMID: 101, Name: "known-vm", Node: "known-node", Status: "running"}},
					}}}
					if !noServer {
						config.AgentServer = server
					}
					executor := NewPulseToolExecutor(config)
					ctx := context.Background()
					var result CallToolResult
					var err error
					switch operation {
					case "file read":
						result, err = executor.executeFileRead(ctx, "/proc/meminfo", target, "")
					case "file write":
						result, err = executor.executeFileWrite(ctx, "/tmp/access-proof", "value", target, "", nil)
					case "file append":
						result, err = executor.executeFileAppend(ctx, "/tmp/access-proof", "value", target, "", nil)
					case "read exec":
						result, err = executor.executeReadExec(ctx, map[string]interface{}{"command": "uptime", "target_host": target})
					case "legacy command":
						result, err = executor.executeRunCommand(ctx, map[string]interface{}{"command": "uptime", "target_host": target})
					}
					require.NoError(t, err)
					require.True(t, result.IsError)
					require.Len(t, result.Content, 1)
					var response ToolResponse
					require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &response))
					require.False(t, response.OK)
					require.NotNil(t, response.Error)
					require.Equal(t, ErrCodeNoAgent, response.Error.Code)
					require.True(t, response.Error.Failed)
					require.False(t, response.Error.Blocked, "an absent connection does not prove a policy denial")
					require.Equal(t, target, response.Error.Details["target"])
					require.Contains(t, response.Error.Message, "did not run")
					require.NotContains(t, response.Error.Message, "Install")
					if target == "known-vm" {
						require.Equal(t, "vm", response.Error.Details["resource_kind"])
						require.Equal(t, "known-node", response.Error.Details["parent_node"])
						require.Contains(t, response.Error.Message, "Pulse monitoring knows")
					} else {
						require.NotContains(t, response.Error.Details, "resource_kind")
						require.NotContains(t, response.Error.Message, "Pulse monitoring knows")
					}
					server.AssertNotCalled(t, "ExecuteCommand", mock.Anything, mock.Anything, mock.Anything)
				})
			}
		}
	}
}
