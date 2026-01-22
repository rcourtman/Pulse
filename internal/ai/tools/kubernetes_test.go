package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteGetKubernetesClusters(t *testing.T) {
	ctx := context.Background()
	exec := NewPulseToolExecutor(ExecutorConfig{})
	result, err := exec.executeGetKubernetesClusters(ctx)
	require.NoError(t, err)
	assert.Equal(t, "State provider not available.", result.Content[0].Text)

	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})
	result, err = exec.executeGetKubernetesClusters(ctx)
	require.NoError(t, err)
	assert.Equal(t, "No Kubernetes clusters found. Kubernetes monitoring may not be configured.", result.Content[0].Text)

	state := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:                "c1",
				Name:              "cluster-1",
				DisplayName:       "Cluster One",
				CustomDisplayName: "Custom One",
				Status:            "online",
				Version:           "1.28",
				Nodes: []models.KubernetesNode{
					{Name: "node1", Ready: true},
					{Name: "node2", Ready: false},
				},
			},
		},
	}
	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
	result, err = exec.executeGetKubernetesClusters(ctx)
	require.NoError(t, err)

	var resp KubernetesClustersResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	require.Len(t, resp.Clusters, 1)
	assert.Equal(t, "Custom One", resp.Clusters[0].DisplayName)
	assert.Equal(t, 1, resp.Clusters[0].ReadyNodes)
}

func TestExecuteGetKubernetesNodes(t *testing.T) {
	ctx := context.Background()
	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})
	result, err := exec.executeGetKubernetesNodes(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "cluster is required")

	result, err = exec.executeGetKubernetesNodes(ctx, map[string]interface{}{
		"cluster": "missing",
	})
	require.NoError(t, err)
	assert.Equal(t, "Kubernetes cluster 'missing' not found.", result.Content[0].Text)

	state := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:                "c1",
				Name:              "cluster-1",
				DisplayName:       "Cluster One",
				CustomDisplayName: "Custom One",
				Nodes: []models.KubernetesNode{
					{Name: "node1", Ready: true, CapacityCPU: 4, CapacityMemoryBytes: 8},
				},
			},
		},
	}
	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
	result, err = exec.executeGetKubernetesNodes(ctx, map[string]interface{}{
		"cluster": "Custom One",
	})
	require.NoError(t, err)

	var resp KubernetesNodesResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	require.Len(t, resp.Nodes, 1)
	assert.Equal(t, "node1", resp.Nodes[0].Name)
}

func TestExecuteGetKubernetesPods(t *testing.T) {
	ctx := context.Background()
	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})
	result, err := exec.executeGetKubernetesPods(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "cluster is required")

	state := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:          "c1",
				Name:        "cluster-1",
				DisplayName: "Cluster One",
				Pods: []models.KubernetesPod{
					{
						Name:      "pod-a",
						Namespace: "default",
						Phase:     "Running",
						Containers: []models.KubernetesPodContainer{
							{Name: "c1", Ready: true, State: "Running"},
						},
					},
					{
						Name:      "pod-b",
						Namespace: "kube-system",
						Phase:     "Pending",
					},
				},
			},
		},
	}
	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
	result, err = exec.executeGetKubernetesPods(ctx, map[string]interface{}{
		"cluster":   "cluster-1",
		"namespace": "default",
		"status":    "running",
	})
	require.NoError(t, err)

	var resp KubernetesPodsResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	require.Len(t, resp.Pods, 1)
	assert.Equal(t, "pod-a", resp.Pods[0].Name)
	assert.Equal(t, 2, resp.Total)
	assert.Equal(t, 1, resp.Filtered)
}

func TestExecuteGetKubernetesDeployments(t *testing.T) {
	ctx := context.Background()
	exec := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: models.StateSnapshot{}}})
	result, err := exec.executeGetKubernetesDeployments(ctx, map[string]interface{}{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "cluster is required")

	state := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:          "c1",
				Name:        "cluster-1",
				DisplayName: "Cluster One",
				Deployments: []models.KubernetesDeployment{
					{Name: "web", Namespace: "default", DesiredReplicas: 2, ReadyReplicas: 2},
					{Name: "db", Namespace: "prod", DesiredReplicas: 1, ReadyReplicas: 0},
				},
			},
		},
	}
	exec = NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{state: state}})
	result, err = exec.executeGetKubernetesDeployments(ctx, map[string]interface{}{
		"cluster":   "cluster-1",
		"namespace": "default",
	})
	require.NoError(t, err)

	var resp KubernetesDeploymentsResponse
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &resp))
	require.Len(t, resp.Deployments, 1)
	assert.Equal(t, "web", resp.Deployments[0].Name)
	assert.Equal(t, 2, resp.Total)
}
