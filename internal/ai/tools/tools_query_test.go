package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestExecuteListInfrastructureAndTopology(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "node1", Name: "node1", Status: "online"}},
		VMs: []models.VM{
			{Name: "vm1", VMID: 100, Status: "running", Node: "node1"},
		},
		Containers: []models.Container{
			{Name: "ct1", VMID: 200, Status: "stopped", Node: "node1"},
		},
		DockerHosts: []models.DockerHost{
			{
				ID:          "host1",
				Hostname:    "h1",
				DisplayName: "Host 1",
				Containers: []models.DockerContainer{
					{ID: "c1", Name: "nginx", State: "running", Image: "nginx"},
				},
			},
		},
	}

	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{state: state},
		AgentServer: &mockAgentServer{
			agents: []agentexec.ConnectedAgent{{Hostname: "node1"}},
		},
		ControlLevel: ControlLevelControlled,
	})

	result, err := executor.executeListInfrastructure(context.Background(), map[string]interface{}{
		"type":   "vms",
		"status": "running",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var infra InfrastructureResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &infra); err != nil {
		t.Fatalf("decode infra: %v", err)
	}
	if len(infra.VMs) != 1 || infra.VMs[0].Name != "vm1" {
		t.Fatalf("unexpected infra response: %+v", infra)
	}

	// Topology includes derived node for VM reference if missing
	state.Nodes = nil
	executor.stateProvider = &mockStateProvider{state: state}
	topologyResult, err := executor.executeGetTopology(context.Background(), map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var topology TopologyResponse
	if err := json.Unmarshal([]byte(topologyResult.Content[0].Text), &topology); err != nil {
		t.Fatalf("decode topology: %v", err)
	}
	if topology.Summary.TotalVMs != 1 || len(topology.Proxmox.Nodes) == 0 {
		t.Fatalf("unexpected topology: %+v", topology)
	}
}

func TestExecuteGetTopologySummaryOnly(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "node1", Name: "node1", Status: "online"}},
		VMs: []models.VM{
			{Name: "vm1", VMID: 100, Status: "running", Node: "node1"},
		},
		Containers: []models.Container{
			{Name: "ct1", VMID: 200, Status: "stopped", Node: "node1"},
		},
		DockerHosts: []models.DockerHost{
			{
				Hostname: "host1",
				Containers: []models.DockerContainer{
					{ID: "c1", Name: "nginx", State: "running", Image: "nginx"},
				},
			},
		},
	}

	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{state: state},
	})

	result, err := executor.executeGetTopology(context.Background(), map[string]interface{}{
		"summary_only": true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var topology TopologyResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &topology); err != nil {
		t.Fatalf("decode topology: %v", err)
	}
	if len(topology.Proxmox.Nodes) != 0 {
		t.Fatalf("expected no proxmox nodes, got: %+v", topology.Proxmox.Nodes)
	}
	if len(topology.Docker.Hosts) != 0 {
		t.Fatalf("expected no docker hosts, got: %+v", topology.Docker.Hosts)
	}
	if len(topology.Kubernetes.Clusters) != 0 {
		t.Fatalf("expected no kubernetes clusters, got: %+v", topology.Kubernetes.Clusters)
	}
	if topology.Summary.TotalVMs != 1 || topology.Summary.TotalDockerHosts != 1 || topology.Summary.TotalDockerContainers != 1 {
		t.Fatalf("unexpected summary: %+v", topology.Summary)
	}
	if topology.Summary.RunningVMs != 1 || topology.Summary.RunningDocker != 1 {
		t.Fatalf("unexpected running summary: %+v", topology.Summary)
	}
}

func TestExecuteGetTopology_KubernetesInclude(t *testing.T) {
	state := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:     "cluster-1",
				Name:   "prod-cluster",
				Status: "online",
				Nodes: []models.KubernetesNode{
					{Name: "worker-1", UID: "node-1", Ready: true, Roles: []string{"worker"}},
				},
				Pods: []models.KubernetesPod{
					{
						UID:       "pod-1",
						Name:      "api-6f8d5c",
						Namespace: "default",
						Phase:     "Running",
						Restarts:  2,
						OwnerKind: "Deployment",
						OwnerName: "api",
					},
				},
				Deployments: []models.KubernetesDeployment{
					{
						UID:             "deploy-1",
						Name:            "api",
						Namespace:       "default",
						DesiredReplicas: 3,
						ReadyReplicas:   2,
					},
				},
			},
		},
	}

	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{state: state},
	})

	result, err := executor.executeGetTopology(context.Background(), map[string]interface{}{
		"include": "kubernetes",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var topology TopologyResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &topology); err != nil {
		t.Fatalf("decode topology: %v", err)
	}

	if len(topology.Proxmox.Nodes) != 0 {
		t.Fatalf("expected no proxmox topology, got: %+v", topology.Proxmox.Nodes)
	}
	if len(topology.Docker.Hosts) != 0 {
		t.Fatalf("expected no docker topology, got: %+v", topology.Docker.Hosts)
	}
	if len(topology.Kubernetes.Clusters) != 1 {
		t.Fatalf("expected one kubernetes cluster, got: %+v", topology.Kubernetes.Clusters)
	}

	cluster := topology.Kubernetes.Clusters[0]
	if cluster.Name != "prod-cluster" || cluster.NodeCount != 1 || cluster.DeploymentCount != 1 || cluster.PodCount != 1 {
		t.Fatalf("unexpected cluster topology: %+v", cluster)
	}
	if len(cluster.Nodes) != 1 || cluster.Nodes[0].Name != "worker-1" || !cluster.Nodes[0].Ready {
		t.Fatalf("unexpected cluster nodes: %+v", cluster.Nodes)
	}
	if len(cluster.Deployments) != 1 || cluster.Deployments[0].Name != "api" || cluster.Deployments[0].ReadyReplicas != 2 {
		t.Fatalf("unexpected cluster deployments: %+v", cluster.Deployments)
	}
	if len(cluster.Pods) != 1 || cluster.Pods[0].Name != "api-6f8d5c" || cluster.Pods[0].OwnerName != "api" {
		t.Fatalf("unexpected cluster pods: %+v", cluster.Pods)
	}

	if topology.Summary.TotalK8sClusters != 1 || topology.Summary.TotalK8sNodes != 1 || topology.Summary.TotalK8sDeployments != 1 || topology.Summary.TotalK8sPods != 1 {
		t.Fatalf("unexpected k8s summary totals: %+v", topology.Summary)
	}
	if topology.Summary.RunningK8sPods != 1 {
		t.Fatalf("unexpected k8s running summary: %+v", topology.Summary)
	}
}

func TestExecuteSearchResources(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "node1", Name: "node1", Status: "online"}},
		VMs: []models.VM{
			{ID: "vm1", VMID: 100, Name: "web-vm", Status: "running", Node: "node1"},
		},
		Containers: []models.Container{
			{ID: "ct1", VMID: 200, Name: "db-ct", Status: "stopped", Node: "node1"},
		},
		DockerHosts: []models.DockerHost{
			{
				ID:          "host1",
				Hostname:    "dock1",
				DisplayName: "Dock 1",
				Status:      "online",
				Containers: []models.DockerContainer{
					{ID: "c1", Name: "nginx", State: "running", Image: "nginx:latest"},
				},
			},
		},
	}

	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{state: state},
	})

	result, err := executor.executeSearchResources(context.Background(), map[string]interface{}{
		"query": "nginx",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var response ResourceSearchResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Matches) != 1 || response.Matches[0].Type != "docker" || response.Matches[0].Name != "nginx" {
		t.Fatalf("unexpected search response: %+v", response)
	}

	result, err = executor.executeSearchResources(context.Background(), map[string]interface{}{
		"query": "web",
		"type":  "vm",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	response = ResourceSearchResponse{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Matches) != 1 || response.Matches[0].Type != "vm" || response.Matches[0].Name != "web-vm" {
		t.Fatalf("unexpected search response: %+v", response)
	}

	// Proxmox-style type+VMID patterns: "LXC200", "VM100", "CT200"
	for _, tc := range []struct {
		query    string
		wantType string
		wantName string
	}{
		{"LXC200", "system-container", "db-ct"},
		{"CT200", "system-container", "db-ct"},
		{"VM100", "vm", "web-vm"},
		{"vm100", "vm", "web-vm"},
	} {
		result, err = executor.executeSearchResources(context.Background(), map[string]interface{}{
			"query": tc.query,
		})
		if err != nil {
			t.Fatalf("query %q: unexpected error: %v", tc.query, err)
		}
		response = ResourceSearchResponse{}
		if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
			t.Fatalf("query %q: decode response: %v", tc.query, err)
		}
		if len(response.Matches) != 1 || response.Matches[0].Type != tc.wantType || response.Matches[0].Name != tc.wantName {
			t.Fatalf("query %q: expected 1 match (%s %s), got %+v", tc.query, tc.wantType, tc.wantName, response.Matches)
		}
	}
}

func TestExecuteSearchResources_Errors(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{state: models.StateSnapshot{}},
	})

	result, _ := executor.executeSearchResources(context.Background(), map[string]interface{}{
		"query": "",
	})
	if !result.IsError {
		t.Fatal("expected error for empty query")
	}

	result, _ = executor.executeSearchResources(context.Background(), map[string]interface{}{
		"query": "node",
		"type":  "bad",
	})
	if !result.IsError {
		t.Fatal("expected error for invalid type")
	}
}

func TestExecuteGetResource(t *testing.T) {
	state := models.StateSnapshot{
		VMs:        []models.VM{{ID: "vm1", VMID: 100, Name: "vm1", Status: "running", Node: "node1"}},
		Containers: []models.Container{{ID: "ct1", VMID: 200, Name: "ct1", Status: "running", Node: "node1"}},
		DockerHosts: []models.DockerHost{{
			Hostname: "host",
			Containers: []models.DockerContainer{{
				ID:    "abc123",
				Name:  "nginx",
				State: "running",
				Image: "nginx",
			}},
		}},
	}
	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{state: state},
	})

	resource, _ := executor.executeGetResource(context.Background(), map[string]interface{}{
		"resource_type": "vm",
		"resource_id":   "100",
	})
	var res ResourceResponse
	if err := json.Unmarshal([]byte(resource.Content[0].Text), &res); err != nil {
		t.Fatalf("decode resource: %v", err)
	}
	if res.Type != "vm" || res.Name != "vm1" {
		t.Fatalf("unexpected resource: %+v", res)
	}

	resource, _ = executor.executeGetResource(context.Background(), map[string]interface{}{
		"resource_type": "docker",
		"resource_id":   "abc",
	})
	if err := json.Unmarshal([]byte(resource.Content[0].Text), &res); err != nil {
		t.Fatalf("decode docker resource: %v", err)
	}
	if res.Type != "docker" || res.Name != "nginx" {
		t.Fatalf("unexpected docker resource: %+v", res)
	}
}

func TestExecuteGetResource_DockerDetails(t *testing.T) {
	state := models.StateSnapshot{
		DockerHosts: []models.DockerHost{{
			Hostname: "dock1",
			Containers: []models.DockerContainer{{
				ID:            "abcd1234",
				Name:          "web",
				State:         "running",
				Image:         "nginx:latest",
				Health:        "healthy",
				CPUPercent:    1.2,
				MemoryPercent: 3.4,
				MemoryUsage:   1024,
				MemoryLimit:   2048,
				RestartCount:  2,
				Labels: map[string]string{
					"service": "web",
				},
				UpdateStatus: &models.DockerContainerUpdateStatus{
					UpdateAvailable: true,
				},
				Ports: []models.DockerContainerPort{
					{PrivatePort: 80, PublicPort: 8080, Protocol: "tcp", IP: "0.0.0.0"},
				},
				Networks: []models.DockerContainerNetworkLink{
					{Name: "bridge", IPv4: "172.17.0.2"},
				},
				Mounts: []models.DockerContainerMount{
					{Source: "/src", Destination: "/dst", RW: true},
				},
			}},
		}},
	}

	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{state: state},
	})

	result, _ := executor.executeGetResource(context.Background(), map[string]interface{}{
		"resource_type": "docker",
		"resource_id":   "web",
	})
	var res ResourceResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &res); err != nil {
		t.Fatalf("decode docker resource: %v", err)
	}
	if !res.UpdateAvailable || len(res.Ports) != 1 || len(res.Networks) != 1 || len(res.Mounts) != 1 {
		t.Fatalf("unexpected docker details: %+v", res)
	}
}

func TestIntArg(t *testing.T) {
	if got := intArg(map[string]interface{}{}, "limit", 10); got != 10 {
		t.Fatalf("unexpected default: %d", got)
	}
	if got := intArg(map[string]interface{}{"limit": float64(5)}, "limit", 10); got != 5 {
		t.Fatalf("unexpected value: %d", got)
	}
}

func TestExecuteListInfrastructurePaginationAndDockerFilter(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node1", Name: "node1", Status: "online"},
			{ID: "node2", Name: "node2", Status: "offline"},
		},
		DockerHosts: []models.DockerHost{
			{
				ID:       "host1",
				Hostname: "dock1",
				Containers: []models.DockerContainer{
					{ID: "c1", Name: "app", State: "running"},
					{ID: "c2", Name: "db", State: "stopped"},
				},
			},
			{
				ID:       "host2",
				Hostname: "dock2",
				Containers: []models.DockerContainer{
					{ID: "c3", Name: "cache", State: "stopped"},
				},
			},
		},
	}

	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{state: state},
	})

	result, err := executor.executeListInfrastructure(context.Background(), map[string]interface{}{
		"type":   "nodes",
		"limit":  1,
		"offset": 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var nodesResp InfrastructureResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &nodesResp); err != nil {
		t.Fatalf("decode nodes response: %v", err)
	}
	if len(nodesResp.Nodes) != 1 || nodesResp.Nodes[0].Name != "node2" {
		t.Fatalf("unexpected nodes response: %+v", nodesResp.Nodes)
	}
	if nodesResp.Pagination == nil || nodesResp.Pagination.Total != 2 || nodesResp.Pagination.Offset != 1 {
		t.Fatalf("unexpected pagination: %+v", nodesResp.Pagination)
	}

	result, err = executor.executeListInfrastructure(context.Background(), map[string]interface{}{
		"type":                           "docker",
		"status":                         "running",
		"max_docker_containers_per_host": 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var dockerResp InfrastructureResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &dockerResp); err != nil {
		t.Fatalf("decode docker response: %v", err)
	}
	if len(dockerResp.DockerHosts) != 1 || dockerResp.DockerHosts[0].Hostname != "dock1" {
		t.Fatalf("unexpected docker hosts: %+v", dockerResp.DockerHosts)
	}
	if len(dockerResp.DockerHosts[0].Containers) != 1 || dockerResp.DockerHosts[0].Containers[0].State != "running" {
		t.Fatalf("unexpected docker containers: %+v", dockerResp.DockerHosts[0].Containers)
	}
}

func TestExecuteListInfrastructure_KubernetesFilters(t *testing.T) {
	state := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:     "cluster-1",
				Name:   "prod-cluster",
				Status: "online",
				Nodes: []models.KubernetesNode{
					{Name: "worker-1", UID: "node-1", Ready: true, Roles: []string{"worker"}},
				},
				Pods: []models.KubernetesPod{
					{
						UID:       "pod-1",
						Name:      "api-123",
						Namespace: "default",
						Phase:     "Running",
						OwnerKind: "Deployment",
						OwnerName: "api",
					},
					{
						UID:       "pod-2",
						Name:      "job-123",
						Namespace: "batch",
						Phase:     "Succeeded",
						OwnerKind: "Job",
						OwnerName: "cleanup",
					},
				},
				Deployments: []models.KubernetesDeployment{
					{
						UID:               "dep-1",
						Name:              "api",
						Namespace:         "default",
						DesiredReplicas:   3,
						ReadyReplicas:     2,
						AvailableReplicas: 2,
					},
				},
			},
			{
				ID:     "cluster-2",
				Name:   "dev-cluster",
				Status: "warning",
				Nodes: []models.KubernetesNode{
					{Name: "dev-worker-1", UID: "node-2", Ready: false},
				},
			},
		},
	}

	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{state: state},
	})

	result, err := executor.executeListInfrastructure(context.Background(), map[string]interface{}{
		"type":         "kubernetes",
		"cluster_name": "prod-cluster",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var clustersResp InfrastructureResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &clustersResp); err != nil {
		t.Fatalf("decode kubernetes clusters response: %v", err)
	}
	if len(clustersResp.K8sClusters) != 1 || clustersResp.K8sClusters[0].Name != "prod-cluster" {
		t.Fatalf("unexpected k8s clusters: %+v", clustersResp.K8sClusters)
	}
	if clustersResp.K8sClusters[0].NodeCount != 1 || clustersResp.K8sClusters[0].DeploymentCount != 1 || clustersResp.K8sClusters[0].PodCount != 2 {
		t.Fatalf("unexpected k8s cluster counts: %+v", clustersResp.K8sClusters[0])
	}
	if clustersResp.Total.K8sClusters != 2 || clustersResp.Total.K8sNodes != 2 || clustersResp.Total.K8sPods != 2 || clustersResp.Total.K8sDeployments != 1 {
		t.Fatalf("unexpected k8s totals: %+v", clustersResp.Total)
	}

	result, err = executor.executeListInfrastructure(context.Background(), map[string]interface{}{
		"type":         "k8s_pods",
		"status":       "running",
		"cluster_name": "prod-cluster",
		"namespace":    "default",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var podsResp InfrastructureResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &podsResp); err != nil {
		t.Fatalf("decode k8s pods response: %v", err)
	}
	if len(podsResp.K8sPods) != 1 || podsResp.K8sPods[0].Name != "api-123" {
		t.Fatalf("unexpected k8s pods: %+v", podsResp.K8sPods)
	}

	result, err = executor.executeListInfrastructure(context.Background(), map[string]interface{}{
		"type":      "k8s_deployments",
		"namespace": "default",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var deploymentsResp InfrastructureResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &deploymentsResp); err != nil {
		t.Fatalf("decode k8s deployments response: %v", err)
	}
	if len(deploymentsResp.K8sDeployments) != 1 || deploymentsResp.K8sDeployments[0].Name != "api" {
		t.Fatalf("unexpected k8s deployments: %+v", deploymentsResp.K8sDeployments)
	}
}

func TestExecuteGetResourceErrorsAndContainer(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{})
	result, _ := executor.executeGetResource(context.Background(), map[string]interface{}{
		"resource_type": "vm",
		"resource_id":   "100",
	})
	if result.Content[0].Text != "State information not available." {
		t.Fatalf("unexpected response: %s", result.Content[0].Text)
	}

	executor.stateProvider = &mockStateProvider{state: models.StateSnapshot{
		Containers: []models.Container{
			{ID: "ct1", VMID: 200, Name: "ct1", Status: "running", Node: "node1"},
		},
	}}

	result, _ = executor.executeGetResource(context.Background(), map[string]interface{}{
		"resource_type": "container",
		"resource_id":   "ct1",
	})
	var res ResourceResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &res); err != nil {
		t.Fatalf("decode container response: %v", err)
	}
	if res.Type != "system-container" || res.Name != "ct1" {
		t.Fatalf("unexpected container response: %+v", res)
	}

	result, _ = executor.executeGetResource(context.Background(), map[string]interface{}{
		"resource_type": "vm",
		"resource_id":   "999",
	})
	var notFound map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &notFound); err != nil {
		t.Fatalf("decode not found response: %v", err)
	}
	if notFound["error"] != "not_found" {
		t.Fatalf("unexpected not found response: %+v", notFound)
	}

	result, _ = executor.executeGetResource(context.Background(), map[string]interface{}{
		"resource_type": "bad",
		"resource_id":   "1",
	})
	if !result.IsError {
		t.Fatal("expected error for invalid resource_type")
	}
}

func TestExecuteListInfrastructure_NoStateProvider(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{})
	result, _ := executor.executeListInfrastructure(context.Background(), map[string]interface{}{})
	if !result.IsError {
		t.Fatal("expected error without state provider")
	}
}

func TestExecuteGetResource_MissingArgs(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{state: models.StateSnapshot{}},
	})

	result, _ := executor.executeGetResource(context.Background(), map[string]interface{}{})
	if !result.IsError {
		t.Fatal("expected error for missing resource_type")
	}

	result, _ = executor.executeGetResource(context.Background(), map[string]interface{}{
		"resource_type": "vm",
	})
	if !result.IsError {
		t.Fatal("expected error for missing resource_id")
	}
}
