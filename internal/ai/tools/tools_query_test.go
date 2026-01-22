package tools

import (
	"context"
	"encoding/json"
	"net"
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
	t.Setenv("PULSE_AI_ALLOW_LOOPBACK", "true")

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

func TestExecuteSetResourceURL_ClearAndMissingUpdater(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{})
	result, _ := executor.executeSetResourceURL(context.Background(), map[string]interface{}{
		"resource_type": "vm",
		"resource_id":   "100",
	})
	if result.Content[0].Text != "Metadata updater not available." {
		t.Fatalf("unexpected response: %s", result.Content[0].Text)
	}

	updater := &fakeMetadataUpdater{}
	executor.metadataUpdater = updater
	result, _ = executor.executeSetResourceURL(context.Background(), map[string]interface{}{
		"resource_type": "vm",
		"resource_id":   "100",
		"url":           "",
	})
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["action"] != "cleared" {
		t.Fatalf("unexpected response: %+v", resp)
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

func TestParseAndValidateFetchURL(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		if _, err := parseAndValidateFetchURL(context.Background(), ""); err == nil {
			t.Fatal("expected error for empty URL")
		}
	})

	t.Run("InvalidURL", func(t *testing.T) {
		if _, err := parseAndValidateFetchURL(context.Background(), "http://%"); err == nil {
			t.Fatal("expected error for invalid URL")
		}
	})

	t.Run("NotAbsolute", func(t *testing.T) {
		if _, err := parseAndValidateFetchURL(context.Background(), "example.com"); err == nil {
			t.Fatal("expected error for relative URL")
		}
	})

	t.Run("BadScheme", func(t *testing.T) {
		if _, err := parseAndValidateFetchURL(context.Background(), "ftp://example.com"); err == nil {
			t.Fatal("expected error for scheme")
		}
	})

	t.Run("Credentials", func(t *testing.T) {
		if _, err := parseAndValidateFetchURL(context.Background(), "http://user:pass@example.com"); err == nil {
			t.Fatal("expected error for credentials")
		}
	})

	t.Run("Fragment", func(t *testing.T) {
		if _, err := parseAndValidateFetchURL(context.Background(), "https://example.com/#frag"); err == nil {
			t.Fatal("expected error for fragment")
		}
	})

	t.Run("MissingHost", func(t *testing.T) {
		if _, err := parseAndValidateFetchURL(context.Background(), "http:///"); err == nil {
			t.Fatal("expected error for missing host")
		}
	})

	t.Run("BlockedHost", func(t *testing.T) {
		if _, err := parseAndValidateFetchURL(context.Background(), "http://localhost"); err == nil {
			t.Fatal("expected error for blocked host")
		}
	})

	t.Run("BlockedIP", func(t *testing.T) {
		if _, err := parseAndValidateFetchURL(context.Background(), "http://127.0.0.1"); err == nil {
			t.Fatal("expected error for blocked IP")
		}
	})

	t.Run("AllowLoopback", func(t *testing.T) {
		t.Setenv("PULSE_AI_ALLOW_LOOPBACK", "true")
		parsed, err := parseAndValidateFetchURL(context.Background(), "http://127.0.0.1:8080")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if parsed.Hostname() != "127.0.0.1" {
			t.Fatalf("unexpected host: %s", parsed.Hostname())
		}
	})
}

func TestIsBlockedFetchIP(t *testing.T) {
	if !isBlockedFetchIP(nil) {
		t.Fatal("expected nil IP to be blocked")
	}
	if !isBlockedFetchIP(net.ParseIP("0.0.0.0")) {
		t.Fatal("expected unspecified IP to be blocked")
	}
	if !isBlockedFetchIP(net.ParseIP("169.254.1.1")) {
		t.Fatal("expected link-local IP to be blocked")
	}
	if isBlockedFetchIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("expected global IP to be allowed")
	}

	t.Run("LoopbackAllowed", func(t *testing.T) {
		t.Setenv("PULSE_AI_ALLOW_LOOPBACK", "true")
		if isBlockedFetchIP(net.ParseIP("127.0.0.1")) {
			t.Fatal("expected loopback IP to be allowed")
		}
	})
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
	if res.Type != "container" || res.Name != "ct1" {
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

func TestExecuteGetURLContent_InvalidURL(t *testing.T) {
	executor := NewPulseToolExecutor(ExecutorConfig{})
	result, err := executor.executeGetURLContent(context.Background(), map[string]interface{}{
		"url": "ftp://example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var response URLFetchResponse
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == "" {
		t.Fatalf("expected error response: %+v", response)
	}
}
