package tools

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestValidateKubernetesResourceID(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid simple", "nginx", false},
		{"valid with dash", "my-app", false},
		{"valid with dot", "my.app", false},
		{"valid with numbers", "app123", false},
		{"valid complex", "my-app-v1.2.3", false},
		{"empty", "", true},
		{"uppercase", "MyApp", true},
		{"underscore", "my_app", true},
		{"space", "my app", true},
		{"special char", "my@app", true},
		{"too long", string(make([]byte, 254)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateKubernetesResourceID(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFindAgentForKubernetesCluster(t *testing.T) {
	t.Run("NoStateProvider", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{})
		agentID, cluster, err := exec.findAgentForKubernetesCluster("test")
		assert.Error(t, err)
		assert.Empty(t, agentID)
		assert.Nil(t, cluster)
		assert.Contains(t, err.Error(), "state not available")
	})

	t.Run("ClusterNotFound", func(t *testing.T) {
		state := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{
				{ID: "c1", Name: "cluster-1"},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
		agentID, cluster, err := exec.findAgentForKubernetesCluster("nonexistent")
		assert.Error(t, err)
		assert.Empty(t, agentID)
		assert.Nil(t, cluster)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("ClusterNoAgent", func(t *testing.T) {
		state := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{
				{ID: "c1", Name: "cluster-1", AgentID: ""},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
		agentID, cluster, err := exec.findAgentForKubernetesCluster("cluster-1")
		assert.Error(t, err)
		assert.Empty(t, agentID)
		assert.Nil(t, cluster)
		assert.Contains(t, err.Error(), "no agent configured")
	})

	t.Run("FoundByID", func(t *testing.T) {
		state := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{
				{ID: "c1", Name: "cluster-1", AgentID: "agent-1"},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
		agentID, cluster, err := exec.findAgentForKubernetesCluster("c1")
		assert.NoError(t, err)
		assert.Equal(t, "agent-1", agentID)
		assert.NotNil(t, cluster)
		assert.Equal(t, "cluster-1", cluster.Name)
	})

	t.Run("FoundByDisplayName", func(t *testing.T) {
		state := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{
				{ID: "c1", Name: "cluster-1", DisplayName: "Production", AgentID: "agent-1"},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
		agentID, _, err := exec.findAgentForKubernetesCluster("Production")
		assert.NoError(t, err)
		assert.Equal(t, "agent-1", agentID)
	})

	t.Run("FoundByCustomDisplayName", func(t *testing.T) {
		state := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{
				{ID: "c1", Name: "cluster-1", CustomDisplayName: "My Cluster", AgentID: "agent-1"},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
		agentID, _, err := exec.findAgentForKubernetesCluster("My Cluster")
		assert.NoError(t, err)
		assert.Equal(t, "agent-1", agentID)
	})
}

func TestExecuteKubernetesScale(t *testing.T) {
	ctx := context.Background()

	t.Run("MissingCluster", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})
		result, err := exec.executeKubernetesScale(ctx, map[string]interface{}{})
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "cluster is required")
	})

	t.Run("MissingDeployment", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})
		result, err := exec.executeKubernetesScale(ctx, map[string]interface{}{
			"cluster": "test",
		})
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "deployment is required")
	})

	t.Run("MissingReplicas", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})
		result, err := exec.executeKubernetesScale(ctx, map[string]interface{}{
			"cluster":    "test",
			"deployment": "nginx",
		})
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "replicas is required")
	})

	t.Run("InvalidNamespace", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})
		result, err := exec.executeKubernetesScale(ctx, map[string]interface{}{
			"cluster":    "test",
			"deployment": "nginx",
			"replicas":   3,
			"namespace":  "Invalid_NS",
		})
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "invalid namespace")
	})

	t.Run("ReadOnlyMode", func(t *testing.T) {
		state := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{
				{ID: "c1", Name: "cluster-1", AgentID: "agent-1"},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider: &mockStateProvider{state: state},
			ControlLevel:  ControlLevelReadOnly,
		})
		result, err := exec.executeKubernetesScale(ctx, map[string]interface{}{
			"cluster":    "cluster-1",
			"deployment": "nginx",
			"replicas":   3,
		})
		require.NoError(t, err)
		assert.Contains(t, result.Content[0].Text, "not available in read-only mode")
	})

	t.Run("ControlledRequiresApproval", func(t *testing.T) {
		state := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{
				{ID: "c1", Name: "cluster-1", AgentID: "agent-1", DisplayName: "Cluster One"},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider: &mockStateProvider{state: state},
			ControlLevel:  ControlLevelControlled,
		})
		result, err := exec.executeKubernetesScale(ctx, map[string]interface{}{
			"cluster":    "cluster-1",
			"deployment": "nginx",
			"replicas":   3,
		})
		require.NoError(t, err)
		assert.Contains(t, result.Content[0].Text, "APPROVAL_REQUIRED")
		assert.Contains(t, result.Content[0].Text, "scale")
	})

	t.Run("ExecuteSuccess", func(t *testing.T) {
		mockAgent := &mockAgentServer{
			agents: []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "k8s-host"}},
		}
		mockAgent.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return cmd.Command == "kubectl -n default scale deployment nginx --replicas=3" &&
				cmd.TargetType == "host"
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "deployment.apps/nginx scaled",
		}, nil)

		state := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{
				{ID: "c1", Name: "cluster-1", AgentID: "agent-1"},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider: &mockStateProvider{state: state},
			AgentServer:   mockAgent,
			ControlLevel:  ControlLevelAutonomous,
		})
		result, err := exec.executeKubernetesScale(ctx, map[string]interface{}{
			"cluster":    "cluster-1",
			"deployment": "nginx",
			"replicas":   3,
		})
		require.NoError(t, err)
		assert.Contains(t, result.Content[0].Text, "Successfully scaled")
		assert.Contains(t, result.Content[0].Text, "nginx")
		mockAgent.AssertExpectations(t)
	})
}

func TestExecuteKubernetesRestart(t *testing.T) {
	ctx := context.Background()

	t.Run("MissingDeployment", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})
		result, err := exec.executeKubernetesRestart(ctx, map[string]interface{}{
			"cluster": "test",
		})
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "deployment is required")
	})

	t.Run("ExecuteSuccess", func(t *testing.T) {
		mockAgent := &mockAgentServer{
			agents: []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "k8s-host"}},
		}
		mockAgent.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return cmd.Command == "kubectl -n default rollout restart deployment/nginx"
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "deployment.apps/nginx restarted",
		}, nil)

		state := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{
				{ID: "c1", Name: "cluster-1", AgentID: "agent-1"},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider: &mockStateProvider{state: state},
			AgentServer:   mockAgent,
			ControlLevel:  ControlLevelAutonomous,
		})
		result, err := exec.executeKubernetesRestart(ctx, map[string]interface{}{
			"cluster":    "cluster-1",
			"deployment": "nginx",
		})
		require.NoError(t, err)
		assert.Contains(t, result.Content[0].Text, "Successfully initiated rollout restart")
		mockAgent.AssertExpectations(t)
	})
}

func TestExecuteKubernetesDeletePod(t *testing.T) {
	ctx := context.Background()

	t.Run("MissingPod", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})
		result, err := exec.executeKubernetesDeletePod(ctx, map[string]interface{}{
			"cluster": "test",
		})
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "pod is required")
	})

	t.Run("ExecuteSuccess", func(t *testing.T) {
		mockAgent := &mockAgentServer{
			agents: []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "k8s-host"}},
		}
		mockAgent.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return cmd.Command == "kubectl -n default delete pod nginx-abc123"
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "pod \"nginx-abc123\" deleted",
		}, nil)

		state := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{
				{ID: "c1", Name: "cluster-1", AgentID: "agent-1"},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider: &mockStateProvider{state: state},
			AgentServer:   mockAgent,
			ControlLevel:  ControlLevelAutonomous,
		})
		result, err := exec.executeKubernetesDeletePod(ctx, map[string]interface{}{
			"cluster": "cluster-1",
			"pod":     "nginx-abc123",
		})
		require.NoError(t, err)
		assert.Contains(t, result.Content[0].Text, "Successfully deleted pod")
		mockAgent.AssertExpectations(t)
	})
}

func TestExecuteKubernetesExec(t *testing.T) {
	ctx := context.Background()

	t.Run("MissingCommand", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})
		result, err := exec.executeKubernetesExec(ctx, map[string]interface{}{
			"cluster": "test",
			"pod":     "nginx",
		})
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "command is required")
	})

	t.Run("ExecuteWithoutContainer", func(t *testing.T) {
		mockAgent := &mockAgentServer{
			agents: []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "k8s-host"}},
		}
		mockAgent.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return cmd.Command == "kubectl -n default exec nginx-pod -- cat /etc/nginx/nginx.conf"
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "server { listen 80; }",
		}, nil)

		state := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{
				{ID: "c1", Name: "cluster-1", AgentID: "agent-1"},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider: &mockStateProvider{state: state},
			AgentServer:   mockAgent,
			ControlLevel:  ControlLevelAutonomous,
		})
		result, err := exec.executeKubernetesExec(ctx, map[string]interface{}{
			"cluster": "cluster-1",
			"pod":     "nginx-pod",
			"command": "cat /etc/nginx/nginx.conf",
		})
		require.NoError(t, err)
		assert.Contains(t, result.Content[0].Text, "Command executed")
		assert.Contains(t, result.Content[0].Text, "server { listen 80; }")
		mockAgent.AssertExpectations(t)
	})

	t.Run("ExecuteWithContainer", func(t *testing.T) {
		mockAgent := &mockAgentServer{
			agents: []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "k8s-host"}},
		}
		mockAgent.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return cmd.Command == "kubectl -n kube-system exec coredns-pod -c coredns -- cat /etc/coredns/Corefile"
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   ".:53 { forward . /etc/resolv.conf }",
		}, nil)

		state := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{
				{ID: "c1", Name: "cluster-1", AgentID: "agent-1"},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider: &mockStateProvider{state: state},
			AgentServer:   mockAgent,
			ControlLevel:  ControlLevelAutonomous,
		})
		result, err := exec.executeKubernetesExec(ctx, map[string]interface{}{
			"cluster":   "cluster-1",
			"namespace": "kube-system",
			"pod":       "coredns-pod",
			"container": "coredns",
			"command":   "cat /etc/coredns/Corefile",
		})
		require.NoError(t, err)
		assert.Contains(t, result.Content[0].Text, "Command executed")
		mockAgent.AssertExpectations(t)
	})
}

func TestExecuteKubernetesLogs(t *testing.T) {
	ctx := context.Background()

	t.Run("MissingPod", func(t *testing.T) {
		exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})
		result, err := exec.executeKubernetesLogs(ctx, map[string]interface{}{
			"cluster": "test",
		})
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "pod is required")
	})

	t.Run("LogsNoApprovalNeeded", func(t *testing.T) {
		// Logs should work even in controlled mode without approval
		mockAgent := &mockAgentServer{
			agents: []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "k8s-host"}},
		}
		mockAgent.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return cmd.Command == "kubectl -n default logs nginx-pod --tail=50"
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "2024-01-01 10:00:00 Request received\n2024-01-01 10:00:01 Response sent",
		}, nil)

		state := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{
				{ID: "c1", Name: "cluster-1", AgentID: "agent-1"},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider: &mockStateProvider{state: state},
			AgentServer:   mockAgent,
			ControlLevel:  ControlLevelControlled, // Even in controlled mode
		})
		result, err := exec.executeKubernetesLogs(ctx, map[string]interface{}{
			"cluster": "cluster-1",
			"pod":     "nginx-pod",
			"lines":   50,
		})
		require.NoError(t, err)
		// Should NOT require approval since logs is read-only
		assert.NotContains(t, result.Content[0].Text, "APPROVAL_REQUIRED")
		assert.Contains(t, result.Content[0].Text, "Logs from pod")
		mockAgent.AssertExpectations(t)
	})

	t.Run("LogsWithContainer", func(t *testing.T) {
		mockAgent := &mockAgentServer{
			agents: []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "k8s-host"}},
		}
		mockAgent.On("ExecuteCommand", mock.Anything, "agent-1", mock.MatchedBy(func(cmd agentexec.ExecuteCommandPayload) bool {
			return cmd.Command == "kubectl -n default logs nginx-pod -c sidecar --tail=100"
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "Sidecar logs here",
		}, nil)

		state := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{
				{ID: "c1", Name: "cluster-1", AgentID: "agent-1"},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider: &mockStateProvider{state: state},
			AgentServer:   mockAgent,
			ControlLevel:  ControlLevelAutonomous,
		})
		result, err := exec.executeKubernetesLogs(ctx, map[string]interface{}{
			"cluster":   "cluster-1",
			"pod":       "nginx-pod",
			"container": "sidecar",
		})
		require.NoError(t, err)
		assert.Contains(t, result.Content[0].Text, "Logs from pod")
		mockAgent.AssertExpectations(t)
	})

	t.Run("EmptyLogs", func(t *testing.T) {
		mockAgent := &mockAgentServer{
			agents: []agentexec.ConnectedAgent{{AgentID: "agent-1", Hostname: "k8s-host"}},
		}
		mockAgent.On("ExecuteCommand", mock.Anything, "agent-1", mock.Anything).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "",
		}, nil)

		state := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{
				{ID: "c1", Name: "cluster-1", AgentID: "agent-1"},
			},
		}
		exec := NewPulseToolExecutor(ExecutorConfig{
			StateProvider: &mockStateProvider{state: state},
			AgentServer:   mockAgent,
			ControlLevel:  ControlLevelAutonomous,
		})
		result, err := exec.executeKubernetesLogs(ctx, map[string]interface{}{
			"cluster": "cluster-1",
			"pod":     "nginx-pod",
		})
		require.NoError(t, err)
		assert.Contains(t, result.Content[0].Text, "No logs found")
		mockAgent.AssertExpectations(t)
	})
}

func TestFormatKubernetesApprovalNeeded(t *testing.T) {
	result := formatKubernetesApprovalNeeded("scale", "nginx", "default", "production", "kubectl scale...", "approval-123")
	assert.Contains(t, result, "APPROVAL_REQUIRED")
	assert.Contains(t, result, "scale")
	assert.Contains(t, result, "nginx")
	assert.Contains(t, result, "default")
	assert.Contains(t, result, "production")
	assert.Contains(t, result, "approval-123")
}
