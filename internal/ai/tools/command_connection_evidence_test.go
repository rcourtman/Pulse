package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func commandEvidenceSnapshot() models.StateSnapshot {
	return models.StateSnapshot{
		Nodes: []models.Node{{ID: "node-one", Name: "node-one", Status: "online"}},
		VMs:   []models.VM{{ID: "vm-one", VMID: 101, Name: "guest-one", Node: "node-one", Status: "running"}},
		DockerHosts: []models.DockerHost{{
			ID: "host-one", Hostname: "command-host", Status: "online", LastSeen: time.Now(),
			Containers: []models.DockerContainer{{ID: "container-one", Name: "observed-service", State: "running", CPUPercent: 12.5}},
		}},
	}
}

func commandEvidenceJSON(t *testing.T, value any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCommandConnectivityDoesNotReplaceMonitoringEvidence(t *testing.T) {
	for _, tc := range []struct {
		name                                      string
		connected, guestConnected, controlEnabled bool
	}{
		{name: "no_command_connection"},
		{name: "connected_read_only", connected: true},
		{name: "guest_connection_only", guestConnected: true},
		{name: "connected_control_enabled", connected: true, controlEnabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := &mockAgentServer{}
			if tc.connected {
				server.agents = []agentexec.ConnectedAgent{{Hostname: "command-host"}, {Hostname: "node-one"}}
			}
			if tc.guestConnected {
				server.agents = append(server.agents, agentexec.ConnectedAgent{Hostname: "guest-one"})
			}
			controlLevel := ControlLevelReadOnly
			if tc.controlEnabled {
				controlLevel = ControlLevelControlled
			}
			registry := unifiedresources.NewRegistry(nil)
			registry.IngestSnapshot(commandEvidenceSnapshot())
			executor := NewPulseToolExecutor(ExecutorConfig{ReadState: registry, UnifiedResourceProvider: &registryUnifiedQueryProvider{registry}, AgentServer: server, ControlLevel: controlLevel})
			query := func(args map[string]interface{}) map[string]any {
				t.Helper()
				result, err := executor.executeQuery(context.Background(), args)
				if err != nil || result.IsError {
					t.Fatalf("query failed: %v %+v", err, result)
				}
				var decoded map[string]any
				if err := json.Unmarshal([]byte(result.Content[0].Text), &decoded); err != nil {
					t.Fatal(err)
				}
				capture, _ := json.Marshal(map[string]any{"case": tc.name, "input": args, "output": decoded})
				t.Logf("COMMAND_EVIDENCE %s", capture)
				return decoded
			}
			list := query(map[string]interface{}{"action": "list", "type": "docker-hosts"})
			host := list["docker_hosts"].([]any)[0].(map[string]any)
			if host["command_agent_connected"] != tc.connected {
				t.Fatalf("connection must name command transport: %+v", host)
			}
			if _, exists := host["agent_connected"]; exists {
				t.Fatal("ambiguous connection field remains")
			}
			container := host["containers"].([]any)[0].(map[string]any)
			resource := query(map[string]interface{}{"action": "get", "resource_type": "app-container", "resource_id": container["id"]})
			if resource["status"] != "running" || resource["cpu"].(map[string]any)["percent"] != 12.5 {
				t.Fatalf("command state replaced monitored evidence: %+v", resource)
			}
			topology := query(map[string]interface{}{"action": "topology", "include": "all"})
			docker := topology["docker"].(map[string]any)["hosts"].([]any)[0].(map[string]any)
			if docker["command_agent_connected"] != tc.connected || docker["can_execute"] != (tc.connected && tc.controlEnabled) {
				t.Fatalf("transport/control hint changed: %+v", docker)
			}
			search := query(map[string]interface{}{"action": "search", "query": "guest-one"})
			guest := search["matches"].([]any)[0].(map[string]any)
			if guest["node_command_agent_connected"] != tc.connected {
				t.Fatalf("parent transport not identified: %+v", guest)
			}
			if guest["command_agent_connected"] != tc.guestConnected {
				t.Fatalf("parent connection became a direct guest connection: %+v", guest)
			}
			server.AssertNotCalled(t, "ExecuteCommand")
		})
	}
}

func TestTopologyOmitsUnobservedCommandConnections(t *testing.T) {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(commandEvidenceSnapshot())
	for _, observed := range []bool{false, true} {
		options := TopologyBuildOptions{Include: "all", ControlEnabled: true}
		if observed {
			options.ConnectedAgentHostnames = map[string]bool{}
		}
		result := commandEvidenceJSON(t, BuildTopologyResponseFromReadState(registry, options))
		caseName := "unobserved_topology"
		if observed {
			caseName = "observed_empty_topology"
		}
		capture, err := json.Marshal(map[string]any{"case": caseName, "input": map[string]any{"action": "topology", "include": "all"}, "output": result})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("COMMAND_EVIDENCE %s", capture)
		for _, group := range []struct{ family, collection string }{{"docker", "hosts"}, {"proxmox", "nodes"}} {
			item := result[group.family].(map[string]any)[group.collection].([]any)[0].(map[string]any)
			for _, field := range []string{"command_agent_connected", "can_execute"} {
				value, exists := item[field]
				if exists != observed || (exists && value != false) {
					t.Fatalf("observed=%t field=%s: %+v", observed, field, item)
				}
			}
			if _, exists := item["agent_connected"]; exists {
				t.Fatal("ambiguous connection field remains")
			}
		}
		for _, field := range []string{"nodes_with_command_agents", "docker_hosts_with_command_agents"} {
			value, exists := result["summary"].(map[string]any)[field]
			if exists != observed || (exists && value != float64(0)) {
				t.Fatalf("unobserved connections became a count: %+v", result["summary"])
			}
		}
	}
}
