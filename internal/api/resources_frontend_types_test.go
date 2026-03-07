package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestFrontendResourceType(t *testing.T) {
	tests := []struct {
		name string
		r    unified.Resource
		want unified.ResourceType
	}{
		{
			name: "proxmox node becomes node",
			r:    unified.Resource{Type: unified.ResourceTypeAgent, Proxmox: &unified.ProxmoxData{NodeName: "pve1"}},
			want: "node",
		},
		{
			name: "docker host becomes docker-host",
			r:    unified.Resource{Type: unified.ResourceTypeAgent, Docker: &unified.DockerData{Hostname: "dock1"}},
			want: "docker-host",
		},
		{
			name: "dual-mode host prefers agent over docker-host",
			r: unified.Resource{
				Type:   unified.ResourceTypeAgent,
				Agent:  &unified.AgentData{Hostname: "tower"},
				Docker: &unified.DockerData{Hostname: "tower"},
			},
			want: "agent",
		},
		{
			name: "plain host becomes agent",
			r:    unified.Resource{Type: unified.ResourceTypeAgent},
			want: "agent",
		},
		{
			name: "system-container stays system-container",
			r:    unified.Resource{Type: unified.ResourceTypeSystemContainer},
			want: "system-container",
		},
		{
			name: "app-container stays app-container",
			r:    unified.Resource{Type: unified.ResourceTypeAppContainer},
			want: "app-container",
		},
		{
			name: "ceph becomes pool",
			r:    unified.Resource{Type: unified.ResourceTypeCeph},
			want: "pool",
		},
		{
			name: "vm stays vm",
			r:    unified.Resource{Type: unified.ResourceTypeVM},
			want: unified.ResourceTypeVM,
		},
		{
			name: "storage stays storage",
			r:    unified.Resource{Type: unified.ResourceTypeStorage},
			want: unified.ResourceTypeStorage,
		},
		{
			name: "pbs stays pbs",
			r:    unified.Resource{Type: unified.ResourceTypePBS},
			want: unified.ResourceTypePBS,
		},
		{
			name: "pmg stays pmg",
			r:    unified.Resource{Type: unified.ResourceTypePMG},
			want: unified.ResourceTypePMG,
		},
		{
			name: "pod stays pod",
			r:    unified.Resource{Type: unified.ResourceTypePod},
			want: unified.ResourceTypePod,
		},
		{
			name: "proxmox node with both proxmox and docker prefers node",
			r: unified.Resource{
				Type:    unified.ResourceTypeAgent,
				Proxmox: &unified.ProxmoxData{NodeName: "pve1"},
				Docker:  &unified.DockerData{Hostname: "dock1"},
			},
			want: "node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := frontendResourceType(tt.r)
			if got != tt.want {
				t.Fatalf("frontendResourceType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplyFrontendTypes(t *testing.T) {
	resources := []unified.Resource{
		{Type: unified.ResourceTypeAgent, Proxmox: &unified.ProxmoxData{NodeName: "pve1"}},
		{Type: unified.ResourceTypeAgent, Docker: &unified.DockerData{Hostname: "dock1"}},
		{Type: unified.ResourceTypeAgent, Agent: &unified.AgentData{Hostname: "tower"}, Docker: &unified.DockerData{Hostname: "tower"}},
		{Type: unified.ResourceTypeAgent},
		{Type: unified.ResourceTypeVM},
		{Type: unified.ResourceTypeSystemContainer},
	}

	applyFrontendTypes(resources)

	expected := []unified.ResourceType{"node", "docker-host", "agent", "agent", "vm", "system-container"}
	for i, want := range expected {
		if resources[i].Type != want {
			t.Fatalf("resources[%d].Type = %q, want %q", i, resources[i].Type, want)
		}
	}
}

func TestComputeFrontendByType(t *testing.T) {
	resources := []unified.Resource{
		{Type: unified.ResourceTypeAgent, Proxmox: &unified.ProxmoxData{NodeName: "pve1"}},
		{Type: unified.ResourceTypeAgent, Proxmox: &unified.ProxmoxData{NodeName: "pve2"}},
		{Type: unified.ResourceTypeAgent, Docker: &unified.DockerData{Hostname: "dock1"}},
		{Type: unified.ResourceTypeAgent, Agent: &unified.AgentData{Hostname: "tower"}, Docker: &unified.DockerData{Hostname: "tower"}},
		{Type: unified.ResourceTypeAgent},
		{Type: unified.ResourceTypeVM},
		{Type: unified.ResourceTypeVM},
		{Type: unified.ResourceTypeSystemContainer},
	}

	byType := computeFrontendByType(resources)

	if byType["node"] != 2 {
		t.Fatalf("byType[node] = %d, want 2", byType["node"])
	}
	if byType["docker-host"] != 1 {
		t.Fatalf("byType[docker-host] = %d, want 1", byType["docker-host"])
	}
	if byType["agent"] != 2 {
		t.Fatalf("byType[agent] = %d, want 2", byType["agent"])
	}
	if byType[unified.ResourceTypeVM] != 2 {
		t.Fatalf("byType[vm] = %d, want 2", byType[unified.ResourceTypeVM])
	}
	if byType["system-container"] != 1 {
		t.Fatalf("byType[system-container] = %d, want 1", byType["system-container"])
	}

	// Verify the input slice was NOT mutated — check exact per-index type.
	originalTypes := []unified.ResourceType{
		unified.ResourceTypeAgent, unified.ResourceTypeAgent, unified.ResourceTypeAgent, unified.ResourceTypeAgent,
		unified.ResourceTypeAgent,
		unified.ResourceTypeVM, unified.ResourceTypeVM,
		unified.ResourceTypeSystemContainer,
	}
	for i, want := range originalTypes {
		if resources[i].Type != want {
			t.Fatalf("computeFrontendByType mutated input: resources[%d].Type = %q, want %q", i, resources[i].Type, want)
		}
	}
}

func TestParseResourceTypesNodeAlias(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[unified.ResourceType]struct{}
	}{
		{name: "node", input: "node", want: map[unified.ResourceType]struct{}{unified.ResourceTypeAgent: {}}},
		{name: "nodes", input: "nodes", want: map[unified.ResourceType]struct{}{unified.ResourceTypeAgent: {}}},
		{name: "docker-host", input: "docker-host", want: map[unified.ResourceType]struct{}{unified.ResourceTypeAgent: {}}},
		{name: "agent", input: "agent", want: map[unified.ResourceType]struct{}{unified.ResourceTypeAgent: {}}},
		{name: "agents", input: "agents", want: map[unified.ResourceType]struct{}{unified.ResourceTypeAgent: {}}},
		{name: "unsupported host ignored by parser", input: "host", want: map[unified.ResourceType]struct{}{}},
		{name: "unsupported lxc ignored by parser", input: "lxc", want: map[unified.ResourceType]struct{}{}},
		{name: "unsupported qemu ignored by parser", input: "qemu", want: map[unified.ResourceType]struct{}{}},
		{name: "unsupported system_container ignored by parser", input: "system_container", want: map[unified.ResourceType]struct{}{}},
		{name: "unsupported app_container ignored by parser", input: "app_container", want: map[unified.ResourceType]struct{}{}},
		{name: "unsupported container ignored by parser", input: "container", want: map[unified.ResourceType]struct{}{}},
		{name: "pod", input: "pod", want: map[unified.ResourceType]struct{}{unified.ResourceTypePod: {}}},
		{name: "pods", input: "pods", want: map[unified.ResourceType]struct{}{unified.ResourceTypePod: {}}},
		{name: "k8s cluster", input: "k8s-cluster", want: map[unified.ResourceType]struct{}{unified.ResourceTypeK8sCluster: {}}},
		{name: "k8s node", input: "k8s-node", want: map[unified.ResourceType]struct{}{unified.ResourceTypeK8sNode: {}}},
		{name: "k8s deployment", input: "k8s-deployment", want: map[unified.ResourceType]struct{}{unified.ResourceTypeK8sDeployment: {}}},
		{name: "unsupported k8s umbrella ignored by parser", input: "k8s", want: map[unified.ResourceType]struct{}{}},
		{name: "unsupported kubernetes umbrella ignored by parser", input: "kubernetes", want: map[unified.ResourceType]struct{}{}},
		{name: "unsupported k8s-pod ignored by parser", input: "k8s-pod", want: map[unified.ResourceType]struct{}{}},
		{name: "unsupported deployment alias ignored by parser", input: "deployment", want: map[unified.ResourceType]struct{}{}},
		{name: "pool", input: "pool", want: map[unified.ResourceType]struct{}{unified.ResourceTypeCeph: {}}},
		{name: "vm", input: "vm", want: map[unified.ResourceType]struct{}{unified.ResourceTypeVM: {}}},
		// CSV with multiple types
		{name: "csv node,vm", input: "node,vm", want: map[unified.ResourceType]struct{}{
			unified.ResourceTypeAgent: {},
			unified.ResourceTypeVM:    {},
		}},
		// Whitespace and empty segments are handled by splitCSV
		{name: "csv with spaces", input: " node , vm ", want: map[unified.ResourceType]struct{}{
			unified.ResourceTypeAgent: {},
			unified.ResourceTypeVM:    {},
		}},
		// Mixed case — splitCSV lowercases tokens
		{name: "mixed case NoDe,VM", input: "NoDe,VM", want: map[unified.ResourceType]struct{}{
			unified.ResourceTypeAgent: {},
			unified.ResourceTypeVM:    {},
		}},
		// Unknown tokens are silently dropped
		{name: "unknown token", input: "bogus", want: map[unified.ResourceType]struct{}{}},
		// Empty string
		{name: "empty string", input: "", want: map[unified.ResourceType]struct{}{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseResourceTypes(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("parseResourceTypes(%q) = %v (len %d), want %v (len %d)", tt.input, got, len(got), tt.want, len(tt.want))
			}
			for k := range tt.want {
				if _, ok := got[k]; !ok {
					t.Fatalf("parseResourceTypes(%q) missing %q", tt.input, k)
				}
			}
		})
	}
}

func TestUnsupportedResourceTypeFilterTokensRejectsLegacyAliases(t *testing.T) {
	unsupported := unsupportedResourceTypeFilterTokens(
		"vm,lxc,qemu,system-container,system_container,app_container,container,docker-container,docker_service,swarm_service,k8s,kubernetes,k8s-pod,deployment,deployments,k8s_pod,k8s_cluster,k8s_node,k8s_deployment,kubernetes-pod,kubernetes-cluster,kubernetes-node,kubernetes-deployment",
	)
	expected := []string{
		"lxc",
		"qemu",
		"system_container",
		"app_container",
		"container",
		"docker-container",
		"docker_service",
		"swarm_service",
		"k8s",
		"kubernetes",
		"k8s-pod",
		"deployment",
		"deployments",
		"k8s_pod",
		"k8s_cluster",
		"k8s_node",
		"k8s_deployment",
		"kubernetes-pod",
		"kubernetes-cluster",
		"kubernetes-node",
		"kubernetes-deployment",
	}
	if len(unsupported) != len(expected) {
		t.Fatalf("unsupportedResourceTypeFilterTokens returned %v, want %v", unsupported, expected)
	}
	for i := range expected {
		if unsupported[i] != expected[i] {
			t.Fatalf("unsupportedResourceTypeFilterTokens[%d] = %q, want %q", i, unsupported[i], expected[i])
		}
	}
}

// TestResourceListProxmoxNodeReturnsFrontendNodeType verifies that Proxmox nodes are
// returned with type "node" (not "agent") from the REST API, matching what the WebSocket
// path produces. This is the regression test for Known Issue #4.
//
// The fixture includes mixed host-family resources (Proxmox node + agent host + Docker host)
// to verify that ?type=node returns all three (because node/agent/docker-host resolve to the same
// backend type) and that applyFrontendTypes differentiates them correctly.
func TestResourceListProxmoxNodeReturnsFrontendNodeType(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:          "instance-pve1",
				Name:        "pve1",
				Instance:    "instance",
				Host:        "https://pve1:8006",
				Status:      "online",
				CPU:         0.15,
				ClusterName: "homelab",
				Memory:      models.Memory{Total: 1024, Used: 512, Free: 512, Usage: 0.5},
				Disk:        models.Disk{Total: 2048, Used: 1024, Free: 1024, Usage: 0.5},
				LastSeen:    now,
			},
		},
		Hosts: []models.Host{
			{
				ID:       "agent-host-1",
				Hostname: "server1",
				Status:   "online",
				Memory:   models.Memory{Total: 4096, Used: 2048, Free: 2048, Usage: 0.5},
				LastSeen: now,
			},
		},
		DockerHosts: []models.DockerHost{
			{
				ID:       "docker-host-1",
				Hostname: "docker1",
				Status:   "online",
				CPUs:     4,
				Memory:   models.Memory{Total: 8192, Used: 4096, Free: 4096, Usage: 0.5},
				LastSeen: now,
			},
		},
		VMs: []models.VM{
			{
				ID:       "vm-100",
				Name:     "test-vm",
				Node:     "pve1",
				Instance: "instance",
				VMID:     100,
				Status:   "running",
				CPU:      0.05,
				Memory:   models.Memory{Total: 2048, Used: 512, Free: 1536, Usage: 0.25},
				Disk:     models.Disk{Total: 1000, Used: 500, Free: 500, Usage: 0.5},
				LastSeen: now,
			},
		},
		Containers: []models.Container{
			{
				ID:       "ct-200",
				Name:     "test-ct",
				Node:     "pve1",
				Instance: "instance",
				VMID:     200,
				Status:   "running",
				CPU:      0.02,
				Memory:   models.Memory{Total: 512, Used: 128, Free: 384, Usage: 0.25},
				LastSeen: now,
			},
		},
	}

	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceHandlers(cfg)
	h.SetStateProvider(resourceStateProvider{snapshot: snapshot})

	// Test 1: ?type=node resolves to ResourceTypeAgent — returns all host-family resources.
	// The frontend then uses byType('node') client-side to select only Proxmox nodes.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/resources?type=node", nil)
	h.HandleListResources(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp ResourcesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// All three host-family resources are returned because "node", "agent", and
	// "docker-host" are aliases for the same backend type (ResourceTypeAgent).
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 host-family resources for ?type=node, got %d", len(resp.Data))
	}

	// Verify each resource got the correct frontend type.
	typeSet := make(map[unified.ResourceType]int)
	for _, r := range resp.Data {
		typeSet[r.Type]++
	}
	if typeSet["node"] != 1 {
		t.Fatalf("expected 1 'node' in ?type=node response, got %d (types=%v)", typeSet["node"], typeSet)
	}
	if typeSet["agent"] != 1 {
		t.Fatalf("expected 1 'agent' in ?type=node response, got %d (types=%v)", typeSet["agent"], typeSet)
	}
	if typeSet["docker-host"] != 1 {
		t.Fatalf("expected 1 'docker-host' in ?type=node response, got %d (types=%v)", typeSet["docker-host"], typeSet)
	}

	// Agent-backed host resources should publish agent metrics targets.
	var foundAgentHost *unified.Resource
	for i := range resp.Data {
		if resp.Data[i].Type == "agent" {
			foundAgentHost = &resp.Data[i]
			break
		}
	}
	if foundAgentHost == nil {
		t.Fatalf("expected agent resource in response")
	}
	if foundAgentHost.MetricsTarget == nil {
		t.Fatalf("expected metrics target on agent resource")
	}
	if foundAgentHost.MetricsTarget.ResourceType != "agent" {
		t.Fatalf("agent metrics target resourceType = %q, want %q", foundAgentHost.MetricsTarget.ResourceType, "agent")
	}

	var foundDockerHost *unified.Resource
	for i := range resp.Data {
		if resp.Data[i].Type == "docker-host" {
			foundDockerHost = &resp.Data[i]
			break
		}
	}
	if foundDockerHost == nil {
		t.Fatalf("expected docker-host resource in response")
	}
	if foundDockerHost.MetricsTarget == nil {
		t.Fatalf("expected metrics target on docker-host resource")
	}
	if foundDockerHost.MetricsTarget.ResourceType != "docker-host" {
		t.Fatalf(
			"docker-host metrics target resourceType = %q, want %q",
			foundDockerHost.MetricsTarget.ResourceType,
			"docker-host",
		)
	}

	// Find the Proxmox node resource specifically and verify its metadata.
	var foundNode *unified.Resource
	for i := range resp.Data {
		if resp.Data[i].Type == "node" {
			foundNode = &resp.Data[i]
			break
		}
	}
	if foundNode == nil {
		t.Fatalf("no resource with type 'node' found")
	}
	if foundNode.Proxmox == nil {
		t.Fatalf("expected proxmox metadata on node resource")
	}
	if foundNode.Proxmox.NodeName != "pve1" {
		t.Fatalf("proxmox.nodeName = %q, want pve1", foundNode.Proxmox.NodeName)
	}

	// Test 2: Verify ByType aggregation uses frontend type names.
	if resp.Aggregations.ByType["node"] != 1 {
		t.Fatalf("aggregations.byType[node] = %d, want 1 (got byType=%v)", resp.Aggregations.ByType["node"], resp.Aggregations.ByType)
	}
	if resp.Aggregations.ByType["agent"] != 1 {
		t.Fatalf("aggregations.byType[agent] = %d, want 1 (got byType=%v)", resp.Aggregations.ByType["agent"], resp.Aggregations.ByType)
	}
	if resp.Aggregations.ByType["docker-host"] != 1 {
		t.Fatalf("aggregations.byType[docker-host] = %d, want 1 (got byType=%v)", resp.Aggregations.ByType["docker-host"], resp.Aggregations.ByType)
	}

	// Test 3: Unfiltered response should include all resources with correct frontend types.
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/resources", nil)
	h.HandleListResources(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("unfiltered status = %d, body=%s", rec2.Code, rec2.Body.String())
	}

	var resp2 ResourcesResponse
	if err := json.NewDecoder(rec2.Body).Decode(&resp2); err != nil {
		t.Fatalf("decode unfiltered response: %v", err)
	}

	// 5 resources: node + agent + docker-host + vm + system-container
	if len(resp2.Data) != 5 {
		t.Fatalf("expected 5 resources (node+agent+docker-host+vm+ct), got %d", len(resp2.Data))
	}

	typeSet2 := make(map[unified.ResourceType]int)
	for _, r := range resp2.Data {
		typeSet2[r.Type]++
	}

	if typeSet2["node"] != 1 {
		t.Fatalf("expected 1 node in unfiltered response, got %d (types=%v)", typeSet2["node"], typeSet2)
	}
	if typeSet2["agent"] != 1 {
		t.Fatalf("expected 1 agent in unfiltered response, got %d (types=%v)", typeSet2["agent"], typeSet2)
	}
	if typeSet2["docker-host"] != 1 {
		t.Fatalf("expected 1 docker-host in unfiltered response, got %d (types=%v)", typeSet2["docker-host"], typeSet2)
	}
	if typeSet2["vm"] != 1 {
		t.Fatalf("expected 1 vm in unfiltered response, got %d (types=%v)", typeSet2["vm"], typeSet2)
	}
	if typeSet2["system-container"] != 1 {
		t.Fatalf(
			"expected 1 system-container in unfiltered response, got %d (types=%v)",
			typeSet2["system-container"],
			typeSet2,
		)
	}

	var foundVM *unified.Resource
	var foundSystemContainer *unified.Resource
	for i := range resp2.Data {
		switch resp2.Data[i].Type {
		case "vm":
			foundVM = &resp2.Data[i]
		case "system-container":
			foundSystemContainer = &resp2.Data[i]
		}
	}
	if foundVM == nil || foundVM.MetricsTarget == nil || foundVM.MetricsTarget.ResourceType != "vm" {
		t.Fatalf("expected vm metrics target resourceType vm, got %#v", foundVM)
	}
	if foundSystemContainer == nil ||
		foundSystemContainer.MetricsTarget == nil ||
		foundSystemContainer.MetricsTarget.ResourceType != "system-container" {
		t.Fatalf("expected system-container metrics target, got %#v", foundSystemContainer)
	}
}

// TestResourceGetProxmoxNodeReturnsFrontendNodeType verifies that GET /api/resources/{id}
// also returns type "node" for a Proxmox node resource.
func TestResourceGetProxmoxNodeReturnsFrontendNodeType(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:       "instance-pve1",
				Name:     "pve1",
				Instance: "instance",
				Host:     "https://pve1:8006",
				Status:   "online",
				CPU:      0.15,
				Memory:   models.Memory{Total: 1024, Used: 512, Free: 512, Usage: 0.5},
				Disk:     models.Disk{Total: 2048, Used: 1024, Free: 1024, Usage: 0.5},
				LastSeen: now,
			},
		},
	}

	cfg := &config.Config{DataPath: t.TempDir()}
	h := NewResourceHandlers(cfg)
	h.SetStateProvider(resourceStateProvider{snapshot: snapshot})

	// First get the resource ID from a list query.
	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/resources?type=node", nil)
	h.HandleListResources(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listRec.Code, listRec.Body.String())
	}

	var listResp ResourcesResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	// "node" alias maps to ResourceTypeAgent — returns all host-family resources.
	// We only have one Proxmox node in this fixture.
	if len(listResp.Data) != 1 {
		t.Fatalf("expected 1 host-family resource, got %d", len(listResp.Data))
	}

	resourceID := listResp.Data[0].ID

	// Now fetch the individual resource.
	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/resources/"+resourceID, nil)
	h.HandleGetResource(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, body=%s", getRec.Code, getRec.Body.String())
	}

	var resource unified.Resource
	if err := json.NewDecoder(getRec.Body).Decode(&resource); err != nil {
		t.Fatalf("decode resource: %v", err)
	}

	if resource.Type != "node" {
		t.Fatalf("GET resource type = %q, want \"node\"", resource.Type)
	}
	if resource.Proxmox == nil || resource.Proxmox.NodeName != "pve1" {
		t.Fatalf("expected proxmox.nodeName=pve1, got %+v", resource.Proxmox)
	}
}
