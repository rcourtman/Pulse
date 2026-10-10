package tools

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newConfiguredKubernetesExecutor(snapshot models.StateSnapshot, mutate func(*ExecutorConfig)) *PulseToolExecutor {
	adapter := unifiedresources.NewMonitorAdapter(nil)
	adapter.PopulateFromSnapshot(snapshot)

	// Autonomous dispatches fail closed when no audit store is wired
	// (unknown remediation-lock state), so these routing/control tests
	// always get an in-memory store.
	cfg := ExecutorConfig{
		UnifiedResourceProvider: adapter,
		ActionAuditStore:        unifiedresources.NewMemoryStore(),
	}
	if mutate != nil {
		mutate(&cfg)
	}
	return NewPulseToolExecutor(cfg)
}

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
		exec := newConfiguredKubernetesExecutor(state, nil)
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
		exec := newConfiguredKubernetesExecutor(state, nil)
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
		exec := newConfiguredKubernetesExecutor(state, nil)
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
		exec := newConfiguredKubernetesExecutor(state, nil)
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
		exec := newConfiguredKubernetesExecutor(state, nil)
		agentID, _, err := exec.findAgentForKubernetesCluster("My Cluster")
		assert.NoError(t, err)
		assert.Equal(t, "agent-1", agentID)
	})

	t.Run("FoundWithUnifiedReadStateOnly", func(t *testing.T) {
		snapshot := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{
				{ID: "c1", Name: "cluster-1", CustomDisplayName: "My Cluster", AgentID: "agent-1", Server: "https://k8s.example", Context: "prod"},
			},
		}
		exec := newConfiguredKubernetesExecutor(snapshot, nil)
		agentID, cluster, err := exec.findAgentForKubernetesCluster("My Cluster")
		assert.NoError(t, err)
		assert.Equal(t, "agent-1", agentID)
		require.NotNil(t, cluster)
		assert.NotEmpty(t, cluster.ID)
		assert.Equal(t, "cluster-1", cluster.Name)
		assert.Equal(t, "My Cluster", cluster.DisplayName)
		assert.Equal(t, "https://k8s.example", cluster.Server)
		assert.Equal(t, "prod", cluster.Context)
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
			return cmd.Command == "kubectl -n 'default' logs 'nginx-pod' --tail=50"
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "2024-01-01 10:00:00 Request received\n2024-01-01 10:00:01 Response sent",
		}, nil)

		state := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{
				{ID: "c1", Name: "cluster-1", AgentID: "agent-1"},
			},
		}
		exec := newConfiguredKubernetesExecutor(state, func(cfg *ExecutorConfig) {
			cfg.AgentServer = mockAgent
			cfg.ControlLevel = ControlLevelControlled
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
			return cmd.Command == "kubectl -n 'default' logs 'nginx-pod' -c 'sidecar' --tail=100"
		})).Return(&agentexec.CommandResultPayload{
			ExitCode: 0,
			Stdout:   "Sidecar logs here",
		}, nil)

		state := models.StateSnapshot{
			KubernetesClusters: []models.KubernetesCluster{
				{ID: "c1", Name: "cluster-1", AgentID: "agent-1"},
			},
		}
		exec := newConfiguredKubernetesExecutor(state, func(cfg *ExecutorConfig) {
			cfg.AgentServer = mockAgent
			cfg.ControlLevel = ControlLevelAutonomous
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
		exec := newConfiguredKubernetesExecutor(state, func(cfg *ExecutorConfig) {
			cfg.AgentServer = mockAgent
			cfg.ControlLevel = ControlLevelAutonomous
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
