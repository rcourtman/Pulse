package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestExecuteGetCapabilities(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{
		StateProvider: &mockStateProvider{},
		AgentServer: &mockAgentServer{
			agents: []agentexec.ConnectedAgent{
				{Hostname: "host1", Version: "1.0", Platform: "linux"},
			},
		},
		MetricsHistory:   &mockMetricsHistoryProvider{},
		BaselineProvider: &BaselineMCPAdapter{},
		PatternProvider:  &PatternMCPAdapter{},
		AlertProvider:    &mockAlertProvider{},
		FindingsProvider: &mockFindingsProvider{},
		ControlLevel:     ControlLevelControlled,
		ProtectedGuests:  []string{"100"},
	})

	result, err := executor.executeGetCapabilities(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response CapabilitiesResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ControlLevel != string(ControlLevelControlled) || response.ConnectedAgents != 1 {
		t.Fatalf("unexpected response: %+v", response)
	}
	if !response.Features.Control || !response.Features.MetricsHistory {
		t.Fatalf("unexpected features: %+v", response.Features)
	}
}

func TestExecuteGetURLContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "ok")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}))
	defer server.Close()

	executor := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{}})

	if result, _ := executor.executeGetURLContent(context.Background(), map[string]interface{}{}); !result.IsError {
		t.Fatal("expected error when url missing")
	}

	result, err := executor.executeGetURLContent(context.Background(), map[string]interface{}{
		"url": server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var response URLFetchResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.StatusCode != http.StatusOK || response.Headers["X-Test"] != "ok" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

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
	if topology.Summary.TotalVMs != 1 || topology.Summary.TotalDockerHosts != 1 || topology.Summary.TotalDockerContainers != 1 {
		t.Fatalf("unexpected summary: %+v", topology.Summary)
	}
	if topology.Summary.RunningVMs != 1 || topology.Summary.RunningDocker != 1 {
		t.Fatalf("unexpected running summary: %+v", topology.Summary)
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
}

func TestExecuteSetResourceURLAndGetResource(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{StateProvider: &mockStateProvider{}})

	if result, _ := executor.executeSetResourceURL(context.Background(), map[string]interface{}{}); !result.IsError {
		t.Fatal("expected error when resource_type missing")
	}

	updater := &fakeMetadataUpdater{}
	executor.metadataUpdater = updater
	result, err := executor.executeSetResourceURL(context.Background(), map[string]interface{}{
		"resource_type": "guest",
		"resource_id":   "100",
		"url":           "http://example",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updater.resourceArgs) != 3 || updater.resourceArgs[2] != "http://example" {
		t.Fatalf("unexpected updater args: %+v", updater.resourceArgs)
	}
	var setResp map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &setResp); err != nil {
		t.Fatalf("decode set response: %v", err)
	}
	if setResp["action"] != "set" {
		t.Fatalf("unexpected set response: %+v", setResp)
	}

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
	executor.stateProvider = &mockStateProvider{state: state}

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

func TestIntArg(t *testing.T) {
	if got := intArg(map[string]interface{}{}, "limit", 10); got != 10 {
		t.Fatalf("unexpected default: %d", got)
	}
	if got := intArg(map[string]interface{}{"limit": float64(5)}, "limit", 10); got != 5 {
		t.Fatalf("unexpected value: %d", got)
	}
}
