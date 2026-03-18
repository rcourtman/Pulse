package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestQueryResponsesUseCanonicalEmptyCollections(t *testing.T) {
	tests := []struct {
		name string
		raw  any
		keys []string
	}{
		{name: "resource_search", raw: EmptyResourceSearchResponse(), keys: []string{"matches"}},
		{name: "topology_proxmox", raw: EmptyTopologyResponse(), keys: []string{"proxmox", "nodes"}},
		{name: "topology_docker", raw: EmptyTopologyResponse(), keys: []string{"docker", "hosts"}},
		{name: "topology_kubernetes", raw: EmptyTopologyResponse(), keys: []string{"kubernetes", "clusters"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(tc.raw)
			if err != nil {
				t.Fatalf("marshal %s: %v", tc.name, err)
			}

			var decoded map[string]any
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("decode %s: %v", tc.name, err)
			}

			var current any = decoded
			for _, key := range tc.keys {
				obj, ok := current.(map[string]any)
				if !ok {
					t.Fatalf("%s expected object before %s, got %T", tc.name, key, current)
				}
				current = obj[key]
			}

			values, ok := current.([]any)
			if !ok || len(values) != 0 {
				t.Fatalf("expected %s to be an empty array, got %T (%v)", tc.name, current, current)
			}
		})
	}

	payload, err := json.Marshal(TopologyResponse{
		Proxmox: ProxmoxTopology{
			Nodes: []ProxmoxNodeTopology{{
				Name:   "node1",
				Status: "online",
				VMs:    []TopologyVM{{VMID: 100, Name: "vm1", Status: "running"}},
			}},
		},
		Docker: DockerTopology{
			Hosts: []DockerHostTopology{{
				Hostname: "docker-1",
			}},
		},
		Kubernetes: KubernetesTopology{
			Clusters: []KubernetesClusterTopology{{
				Name:   "cluster1",
				Status: "online",
				Nodes: []KubernetesNodeTopology{{
					Name:   "kube-node-1",
					Status: "Ready",
					Ready:  true,
				}},
			}},
		},
	}.NormalizeCollections())
	if err != nil {
		t.Fatalf("marshal normalized topology: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode normalized topology: %v", err)
	}

	proxmox := decoded["proxmox"].(map[string]any)
	nodes := proxmox["nodes"].([]any)
	node := nodes[0].(map[string]any)
	vms, ok := node["vms"].([]any)
	if !ok || len(vms) != 1 {
		t.Fatalf("expected proxmox node vms to contain normalized items, got %T (%v)", node["vms"], node["vms"])
	}
	vm := vms[0].(map[string]any)
	if tags, ok := vm["tags"].([]any); !ok || len(tags) != 0 {
		t.Fatalf("expected proxmox node vm tags to be an empty array, got %T (%v)", vm["tags"], vm["tags"])
	}
	if containers, ok := node["containers"].([]any); !ok || len(containers) != 0 {
		t.Fatalf("expected proxmox node containers to be an empty array, got %T (%v)", node["containers"], node["containers"])
	}

	docker := decoded["docker"].(map[string]any)
	hosts := docker["hosts"].([]any)
	host := hosts[0].(map[string]any)
	if containers, ok := host["containers"].([]any); !ok || len(containers) != 0 {
		t.Fatalf("expected docker host containers to be an empty array, got %T (%v)", host["containers"], host["containers"])
	}

	kubernetes := decoded["kubernetes"].(map[string]any)
	clusters := kubernetes["clusters"].([]any)
	cluster := clusters[0].(map[string]any)
	if deployments, ok := cluster["deployments"].([]any); !ok || len(deployments) != 0 {
		t.Fatalf("expected kubernetes deployments to be an empty array, got %T (%v)", cluster["deployments"], cluster["deployments"])
	}
	if pods, ok := cluster["pods"].([]any); !ok || len(pods) != 0 {
		t.Fatalf("expected kubernetes pods to be an empty array, got %T (%v)", cluster["pods"], cluster["pods"])
	}
	kubeNodes := cluster["nodes"].([]any)
	kubeNode := kubeNodes[0].(map[string]any)
	if roles, ok := kubeNode["roles"].([]any); !ok || len(roles) != 0 {
		t.Fatalf("expected kubernetes node roles to be an empty array, got %T (%v)", kubeNode["roles"], kubeNode["roles"])
	}
}

func TestResourceAndGuestResponsesUseCanonicalEmptyCollections(t *testing.T) {
	tests := []struct {
		name string
		raw  any
		keys []string
	}{
		{name: "resource_tags", raw: EmptyResourceResponse(), keys: []string{"tags"}},
		{name: "resource_networks", raw: EmptyResourceResponse(), keys: []string{"networks"}},
		{name: "resource_ports", raw: EmptyResourceResponse(), keys: []string{"ports"}},
		{name: "resource_mounts", raw: EmptyResourceResponse(), keys: []string{"mounts"}},
		{name: "guest_mounts", raw: EmptyGuestConfigResponse(), keys: []string{"mounts"}},
		{name: "guest_disks", raw: EmptyGuestConfigResponse(), keys: []string{"disks"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(tc.raw)
			if err != nil {
				t.Fatalf("marshal %s: %v", tc.name, err)
			}

			var decoded map[string]any
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("decode %s: %v", tc.name, err)
			}

			current := decoded[tc.keys[0]]
			values, ok := current.([]any)
			if !ok || len(values) != 0 {
				t.Fatalf("expected %s to be an empty array, got %T (%v)", tc.name, current, current)
			}
		})
	}

	payload, err := json.Marshal(EmptyResourceResponse())
	if err != nil {
		t.Fatalf("marshal empty resource: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode empty resource: %v", err)
	}
	labels, ok := decoded["labels"].(map[string]any)
	if !ok || len(labels) != 0 {
		t.Fatalf("expected labels to be an empty object, got %T (%v)", decoded["labels"], decoded["labels"])
	}

	payload, err = json.Marshal(ResourceResponse{
		Type: "app-container",
		ID:   "abc",
		Name: "app",
		Networks: []NetworkInfo{{
			Name: "bridge",
		}},
	}.NormalizeCollections())
	if err != nil {
		t.Fatalf("marshal normalized resource: %v", err)
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode normalized resource: %v", err)
	}
	networks := decoded["networks"].([]any)
	network := networks[0].(map[string]any)
	addresses, ok := network["addresses"].([]any)
	if !ok || len(addresses) != 0 {
		t.Fatalf("expected network addresses to be an empty array, got %T (%v)", network["addresses"], network["addresses"])
	}

	payload, err = json.Marshal(EmptyURLFetchResponse())
	if err != nil {
		t.Fatalf("marshal empty url fetch: %v", err)
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode empty url fetch: %v", err)
	}
	headers, ok := decoded["headers"].(map[string]any)
	if !ok || len(headers) != 0 {
		t.Fatalf("expected url fetch headers to be an empty object, got %T (%v)", decoded["headers"], decoded["headers"])
	}

	payload, err = json.Marshal(EmptyGuestConfigResponse())
	if err != nil {
		t.Fatalf("marshal empty guest config: %v", err)
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode empty guest config: %v", err)
	}
	raw, ok := decoded["raw"].(map[string]any)
	if !ok || len(raw) != 0 {
		t.Fatalf("expected guest raw to be an empty object, got %T (%v)", decoded["raw"], decoded["raw"])
	}
}

type mockGuestConfigProvider struct {
	lastGuestType string
	lastInstance  string
	lastNode      string
	lastVMID      int
	config        map[string]interface{}
}

func (m *mockGuestConfigProvider) GetGuestConfig(guestType, instance, node string, vmID int) (map[string]interface{}, error) {
	m.lastGuestType = guestType
	m.lastInstance = instance
	m.lastNode = node
	m.lastVMID = vmID
	return m.config, nil
}

func TestCanonicalQueryListType_StrictV6Tokens(t *testing.T) {
	if got := canonicalQueryListType("k8s-pods"); got != "k8s-pods" {
		t.Fatalf("canonicalQueryListType(k8s-pods) = %q, want k8s-pods", got)
	}
	if got := canonicalQueryListType("k8s_pods"); got != "k8s_pods" {
		t.Fatalf("canonicalQueryListType(k8s_pods) = %q, want k8s_pods", got)
	}
	if got := canonicalQueryListType("kubernetes-clusters"); got != "kubernetes-clusters" {
		t.Fatalf("canonicalQueryListType(kubernetes-clusters) = %q, want kubernetes-clusters", got)
	}
}

func TestCanonicalQueryTopologyInclude_StrictV6Tokens(t *testing.T) {
	if got := canonicalQueryTopologyInclude("app-container"); got != "app-containers" {
		t.Fatalf("canonicalQueryTopologyInclude(app-container) = %q, want app-containers", got)
	}
	if got := canonicalQueryTopologyInclude("app_container"); got != "app_container" {
		t.Fatalf("canonicalQueryTopologyInclude(app_container) = %q, want app_container", got)
	}
	if got := canonicalQueryTopologyInclude("docker"); got != "docker" {
		t.Fatalf("canonicalQueryTopologyInclude(docker) = %q, want docker", got)
	}
}

func TestCanonicalQuerySearchType_StrictV6Tokens(t *testing.T) {
	if got := canonicalQuerySearchType("docker-host"); got != "docker-host" {
		t.Fatalf("canonicalQuerySearchType(docker-host) = %q, want docker-host", got)
	}
	if got := canonicalQuerySearchType("docker_host"); got != "docker_host" {
		t.Fatalf("canonicalQuerySearchType(docker_host) = %q, want docker_host", got)
	}
}

func TestCanonicalQueryHelpers_DoNotTranslateLegacyAliases(t *testing.T) {
	if got := canonicalQueryResourceType("container"); got != "container" {
		t.Fatalf("canonicalQueryResourceType(container) = %q, want container", got)
	}
	if got := canonicalQueryResourceType("docker"); got != "docker" {
		t.Fatalf("canonicalQueryResourceType(docker) = %q, want docker", got)
	}
	if got := canonicalQueryListType("container"); got != "container" {
		t.Fatalf("canonicalQueryListType(container) = %q, want container", got)
	}
	if got := canonicalQuerySearchType("docker"); got != "docker" {
		t.Fatalf("canonicalQuerySearchType(docker) = %q, want docker", got)
	}
}

func TestExecuteListInfrastructureAndTopology(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{{ID: "node1", Name: "node1", Instance: "pve1", Status: "online"}},
		VMs: []models.VM{
			{ID: "qemu/pve1/node1/100", Name: "vm1", VMID: 100, Instance: "pve1", Status: "running", Node: "node1"},
		},
		Containers: []models.Container{
			{ID: "lxc/pve1/node1/200", Name: "ct1", VMID: 200, Instance: "pve1", Status: "stopped", Node: "node1"},
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
		Nodes: []models.Node{{ID: "node1", Name: "node1", Instance: "pve1", Status: "online"}},
		VMs: []models.VM{
			{ID: "qemu/pve1/node1/100", Name: "vm1", VMID: 100, Instance: "pve1", Status: "running", Node: "node1"},
		},
		Containers: []models.Container{
			{ID: "lxc/pve1/node1/200", Name: "ct1", VMID: 200, Instance: "pve1", Status: "stopped", Node: "node1"},
		},
		DockerHosts: []models.DockerHost{
			{
				ID:       "host1",
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

func TestExecuteGetTopology_UsesCanonicalMaxProxmoxNodesInput(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node-1", Name: "node-1", Instance: "pve1", Status: "online"},
			{ID: "node-2", Name: "node-2", Instance: "pve1", Status: "online"},
		},
	}

	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{state: state},
	})

	result, err := executor.executeGetTopology(context.Background(), map[string]interface{}{
		"max_proxmox_nodes": 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var topology TopologyResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &topology); err != nil {
		t.Fatalf("decode topology: %v", err)
	}
	if len(topology.Proxmox.Nodes) != 1 {
		t.Fatalf("expected one proxmox node after max_proxmox_nodes cap, got %+v", topology.Proxmox.Nodes)
	}
}

func TestExecuteGetTopology_RejectsLegacyDockerIncludeAlias(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{})
	result, _ := executor.executeGetTopology(context.Background(), map[string]interface{}{
		"include": "docker",
	})
	if !result.IsError {
		t.Fatal("expected error for legacy include alias")
	}
	if !strings.Contains(result.Content[0].Text, "invalid include") {
		t.Fatalf("unexpected error text: %s", result.Content[0].Text)
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
	if len(response.Matches) != 1 || response.Matches[0].Type != "app-container" || response.Matches[0].Name != "nginx" {
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

	result, err = executor.executeSearchResources(context.Background(), map[string]interface{}{
		"query": "dock",
		"type":  "docker-host",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	response = ResourceSearchResponse{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Matches) != 1 || response.Matches[0].Type != "docker-host" || response.Matches[0].Name != "Dock 1" {
		t.Fatalf("unexpected docker-host search response: %+v", response)
	}

	// Canonical type+VMID patterns: "VM100", "system-container200"
	for _, tc := range []struct {
		query    string
		wantType string
		wantName string
	}{
		{"system-container200", "system-container", "db-ct"},
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

	for _, legacyQuery := range []string{"LXC200", "qemu100", "CT200"} {
		result, err = executor.executeSearchResources(context.Background(), map[string]interface{}{
			"query": legacyQuery,
		})
		if err != nil {
			t.Fatalf("legacy query %q: unexpected error: %v", legacyQuery, err)
		}
		response = ResourceSearchResponse{}
		if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
			t.Fatalf("legacy query %q: decode response: %v", legacyQuery, err)
		}
		if len(response.Matches) != 0 {
			t.Fatalf("legacy query %q: expected no matches, got %+v", legacyQuery, response.Matches)
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

	result, _ = executor.executeSearchResources(context.Background(), map[string]interface{}{
		"query": "node",
		"type":  "host",
	})
	if !result.IsError {
		t.Fatal("expected error for legacy host type")
	}
	if !strings.Contains(result.Content[0].Text, "invalid type: host") {
		t.Fatalf("unexpected legacy host type error: %s", result.Content[0].Text)
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
		"resource_type": "app-container",
		"resource_id":   "abc",
	})
	if err := json.Unmarshal([]byte(resource.Content[0].Text), &res); err != nil {
		t.Fatalf("decode docker resource: %v", err)
	}
	if res.Type != "app-container" || res.Name != "nginx" {
		t.Fatalf("unexpected docker resource: %+v", res)
	}
}

func TestExecuteQuerySurfacesIncludeGovernedMetadata(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node1", Name: "node1.internal", Status: "online"},
		},
		VMs: []models.VM{
			{ID: "qemu/pve1/node1/100", VMID: 100, Name: "finance-vm", Status: "running", Node: "node1.internal"},
		},
	}

	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{state: state},
	})

	getResult, _ := executor.executeGetResource(context.Background(), map[string]interface{}{
		"resource_type": "vm",
		"resource_id":   "100",
	})
	var resource ResourceResponse
	if err := json.Unmarshal([]byte(getResult.Content[0].Text), &resource); err != nil {
		t.Fatalf("decode resource response: %v", err)
	}
	if resource.Policy == nil {
		t.Fatal("expected governed policy metadata on get response")
	}
	if resource.Policy.Sensitivity != "sensitive" {
		t.Fatalf("unexpected get response sensitivity: %+v", resource.Policy)
	}
	if resource.AISafeSummary == "" {
		t.Fatal("expected aiSafeSummary on get response")
	}
	if strings.Contains(resource.AISafeSummary, "finance-vm") {
		t.Fatalf("aiSafeSummary leaked raw VM name: %q", resource.AISafeSummary)
	}

	searchResult, _ := executor.executeSearchResources(context.Background(), map[string]interface{}{
		"query": "finance",
		"type":  "vm",
	})
	var search ResourceSearchResponse
	if err := json.Unmarshal([]byte(searchResult.Content[0].Text), &search); err != nil {
		t.Fatalf("decode search response: %v", err)
	}
	if len(search.Matches) != 1 {
		t.Fatalf("expected 1 search match, got %+v", search.Matches)
	}
	if search.Matches[0].Policy == nil {
		t.Fatal("expected governed policy metadata on search match")
	}
	if search.Matches[0].AISafeSummary == "" {
		t.Fatal("expected aiSafeSummary on search match")
	}

	topologyResult, _ := executor.executeGetTopology(context.Background(), map[string]interface{}{
		"include": "proxmox",
	})
	var topology TopologyResponse
	if err := json.Unmarshal([]byte(topologyResult.Content[0].Text), &topology); err != nil {
		t.Fatalf("decode topology response: %v", err)
	}
	if len(topology.Proxmox.Nodes) != 1 {
		t.Fatalf("expected 1 proxmox node, got %+v", topology.Proxmox.Nodes)
	}
	if topology.Proxmox.Nodes[0].Policy == nil {
		t.Fatal("expected governed policy metadata on topology node")
	}
	if len(topology.Proxmox.Nodes[0].VMs) != 1 || topology.Proxmox.Nodes[0].VMs[0].Policy == nil {
		t.Fatalf("expected governed VM metadata in topology response, got %+v", topology.Proxmox.Nodes[0].VMs)
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
		"resource_type": "app-container",
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
		"type":                           "app-containers",
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
		"type":         "k8s-pods",
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
		"type":      "k8s-deployments",
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
		"resource_type": "system-container",
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

	result, _ = executor.executeGetResource(context.Background(), map[string]interface{}{
		"resource_type": "host",
		"resource_id":   "1",
	})
	if !result.IsError {
		t.Fatal("expected error for legacy host resource_type")
	}
	if !strings.Contains(result.Content[0].Text, "invalid resource_type: host") {
		t.Fatalf("unexpected legacy host resource_type error: %s", result.Content[0].Text)
	}
}

func TestExecuteListInfrastructure_NoStateProvider(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{})
	result, _ := executor.executeListInfrastructure(context.Background(), map[string]interface{}{})
	if !result.IsError {
		t.Fatal("expected error without state provider")
	}
}

func TestInfrastructureResponse_UsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(EmptyInfrastructureResponse())
	if err != nil {
		t.Fatalf("marshal empty infrastructure response: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode empty infrastructure response: %v", err)
	}

	for _, key := range []string{"nodes", "vms", "containers", "docker_hosts", "k8s_clusters", "k8s_nodes", "k8s_pods", "k8s_deployments"} {
		values, ok := decoded[key].([]any)
		if !ok || len(values) != 0 {
			t.Fatalf("expected %s to serialize as an empty array, got %T (%v)", key, decoded[key], decoded[key])
		}
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

func TestExecuteGetGuestConfig_SystemContainerUsesCanonicalResolution(t *testing.T) {
	guestCfg := &mockGuestConfigProvider{
		config: map[string]interface{}{
			"hostname": "ct1",
			"ostype":   "debian",
			"onboot":   "1",
			"rootfs":   "local-lvm:vm-200-disk-0,size=8G",
			"mp0":      "local:200/vm-200-disk-1.raw,mp=/data",
		},
	}

	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{state: models.StateSnapshot{
			Containers: []models.Container{
				{ID: "ct1", VMID: 200, Name: "ct1", Status: "running", Node: "node1", Instance: "pve1"},
			},
		}},
		GuestConfigProvider: guestCfg,
	})

	result, _ := executor.executeGetGuestConfig(context.Background(), map[string]interface{}{
		"resource_type": "system-container",
		"resource_id":   "ct1",
	})
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content[0].Text)
	}

	var response GuestConfigResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("decode guest config response: %v", err)
	}

	if response.GuestType != "system-container" || response.VMID != 200 || response.Node != "node1" {
		t.Fatalf("unexpected guest config response: %+v", response)
	}
	if response.Policy == nil {
		t.Fatal("expected governed policy metadata on guest config response")
	}
	if response.AISafeSummary == "" {
		t.Fatal("expected aiSafeSummary on guest config response")
	}
	if guestCfg.lastGuestType != "container" {
		t.Fatalf("expected provider guestType=container, got %q", guestCfg.lastGuestType)
	}
	if guestCfg.lastInstance != "pve1" || guestCfg.lastNode != "node1" || guestCfg.lastVMID != 200 {
		t.Fatalf("unexpected provider call context: instance=%q node=%q vmid=%d", guestCfg.lastInstance, guestCfg.lastNode, guestCfg.lastVMID)
	}
}

func TestExecuteGetGuestConfig_RejectsLegacyResourceTypes(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{state: models.StateSnapshot{
			Containers: []models.Container{
				{ID: "ct1", VMID: 200, Name: "ct1", Status: "running", Node: "node1", Instance: "pve1"},
			},
		}},
		GuestConfigProvider: &mockGuestConfigProvider{config: map[string]interface{}{}},
	})

	for _, resourceType := range []string{"lxc", "host"} {
		t.Run(resourceType, func(t *testing.T) {
			result, _ := executor.executeGetGuestConfig(context.Background(), map[string]interface{}{
				"resource_type": resourceType,
				"resource_id":   "ct1",
			})
			if !result.IsError {
				t.Fatalf("expected error for legacy %s resource_type", resourceType)
			}
			if !strings.Contains(result.Content[0].Text, "invalid resource_type: "+resourceType) {
				t.Fatalf("unexpected error text for %s: %s", resourceType, result.Content[0].Text)
			}
		})
	}
}
