package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
)

// MockStateProvider implements ai.StateProvider
type MockStateProvider struct {
	state models.StateSnapshot
}

func (m *MockStateProvider) GetState() models.StateSnapshot {
	return m.state
}

func TestDiscoveryStateAdapter(t *testing.T) {
	// Setup mock data
	mockState := models.StateSnapshot{
		VMs: []models.VM{
			{VMID: 100, Name: "vm-1", Node: "pve1", Status: "running", Instance: "pve1"},
		},
		Containers: []models.Container{
			{VMID: 200, Name: "lxc-1", Node: "pve1", Status: "running", Instance: "pve1"},
		},
		DockerHosts: []models.DockerHost{
			{
				AgentID:  "agent-1",
				Hostname: "docker-host",
				Containers: []models.DockerContainer{
					{
						ID:     "c1",
						Name:   "nginx",
						Image:  "nginx:latest",
						Status: "up",
						Ports:  []models.DockerContainerPort{{PublicPort: 80, PrivatePort: 80, Protocol: "tcp"}},
						Mounts: []models.DockerContainerMount{{Source: "/src", Destination: "/dest"}},
					},
				},
			},
		},
		Hosts: []models.Host{
			{ID: "host-agent-1", Hostname: "server1", Platform: "linux"},
		},
		Nodes: []models.Node{
			{ID: "node-1", Name: "pve1", LinkedHostAgentID: "host-agent-1"},
			{ID: "node-2", Name: "pve2", LinkedHostAgentID: ""}, // No linked agent
		},
	}

	provider := &MockStateProvider{state: mockState}
	adapter := newDiscoveryStateAdapter(provider)

	// Test GetState conversion
	result := adapter.GetState()

	if len(result.VMs) != 1 {
		t.Errorf("Expected 1 VM, got %d", len(result.VMs))
	} else {
		if result.VMs[0].Name != "vm-1" {
			t.Errorf("Expected VM name 'vm-1', got %s", result.VMs[0].Name)
		}
	}

	if len(result.Containers) != 1 {
		t.Errorf("Expected 1 Container, got %d", len(result.Containers))
	} else {
		if result.Containers[0].Name != "lxc-1" {
			t.Errorf("Expected Container name 'lxc-1', got %s", result.Containers[0].Name)
		}
	}

	if len(result.DockerHosts) != 1 {
		t.Errorf("Expected 1 DockerHost, got %d", len(result.DockerHosts))
	} else {
		host := result.DockerHosts[0]
		if len(host.Containers) != 1 {
			t.Errorf("Expected 1 Docker Container, got %d", len(host.Containers))
		} else {
			dc := host.Containers[0]
			if dc.Name != "nginx" {
				t.Errorf("Expected docker container name 'nginx', got %s", dc.Name)
			}
			if len(dc.Ports) != 1 {
				t.Errorf("Expected 1 port, got %d", len(dc.Ports))
			}
			if len(dc.Mounts) != 1 {
				t.Errorf("Expected 1 mount, got %d", len(dc.Mounts))
			}
		}
	}

	// Test Hosts conversion
	if len(result.Hosts) != 1 {
		t.Errorf("Expected 1 Host, got %d", len(result.Hosts))
	} else {
		if result.Hosts[0].ID != "host-agent-1" {
			t.Errorf("Expected Host ID 'host-agent-1', got %s", result.Hosts[0].ID)
		}
		if result.Hosts[0].Hostname != "server1" {
			t.Errorf("Expected Host Hostname 'server1', got %s", result.Hosts[0].Hostname)
		}
	}

	// Test Nodes conversion (critical for discovery redirection)
	if len(result.Nodes) != 2 {
		t.Errorf("Expected 2 Nodes, got %d", len(result.Nodes))
	} else {
		// Verify first node with linked agent
		if result.Nodes[0].ID != "node-1" {
			t.Errorf("Expected Node[0].ID 'node-1', got %s", result.Nodes[0].ID)
		}
		if result.Nodes[0].Name != "pve1" {
			t.Errorf("Expected Node[0].Name 'pve1', got %s", result.Nodes[0].Name)
		}
		if result.Nodes[0].LinkedHostAgentID != "host-agent-1" {
			t.Errorf("Expected Node[0].LinkedHostAgentID 'host-agent-1', got %s", result.Nodes[0].LinkedHostAgentID)
		}

		// Verify second node without linked agent
		if result.Nodes[1].Name != "pve2" {
			t.Errorf("Expected Node[1].Name 'pve2', got %s", result.Nodes[1].Name)
		}
		if result.Nodes[1].LinkedHostAgentID != "" {
			t.Errorf("Expected Node[1].LinkedHostAgentID to be empty, got %s", result.Nodes[1].LinkedHostAgentID)
		}
	}
}

func TestDiscoveryStateAdapter_NilProvider(t *testing.T) {
	adapter := newDiscoveryStateAdapter(nil)
	state := adapter.GetState()
	if len(state.VMs) != 0 {
		t.Error("Expected empty state for nil provider")
	}
}

func TestDiscoveryCommandAdapter_NilServer(t *testing.T) {
	adapter := newDiscoveryCommandAdapter(nil)

	// Test ExecuteCommand
	cmd := servicediscovery.ExecuteCommandPayload{
		RequestID: "req-1",
		Command:   "ls",
	}
	res, err := adapter.ExecuteCommand(context.Background(), "agent-1", cmd)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if res.Success {
		t.Error("Expected Failure for nil server")
	}
	if res.Error != "agent server not available" {
		t.Errorf("Unexpected error message: %s", res.Error)
	}

	// Test GetConnectedAgents
	agents := adapter.GetConnectedAgents()
	if agents != nil {
		t.Error("Expected nil agents for nil server")
	}

	// Test IsAgentConnected
	if adapter.IsAgentConnected("agent-1") {
		t.Error("Expected IsAgentConnected to return false for nil server")
	}
}

func TestDiscoveryCommandAdapter_ExecuteCommandNotConnected(t *testing.T) {
	server := agentexec.NewServer(nil)
	adapter := newDiscoveryCommandAdapter(server)

	cmd := servicediscovery.ExecuteCommandPayload{
		RequestID:  "req-2",
		Command:    "hostname",
		TargetType: "host",
		Timeout:    1,
	}

	res, err := adapter.ExecuteCommand(context.Background(), "missing-agent", cmd)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if res.Success {
		t.Fatalf("expected command failure payload")
	}
	if res.RequestID != "req-2" {
		t.Fatalf("request id = %q, want %q", res.RequestID, "req-2")
	}
	if !strings.Contains(res.Error, "agent missing-agent not connected") {
		t.Fatalf("error = %q, expected missing-agent not connected", res.Error)
	}
}

func TestDiscoveryCommandAdapter_ConnectedAgentsAndLookup(t *testing.T) {
	server := agentexec.NewServer(nil)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleWebSocket(w, r)
	}))
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := conn.WriteJSON(agentexec.Message{
		Type:      agentexec.MsgTypeAgentRegister,
		Timestamp: time.Now(),
		Payload: agentexec.AgentRegisterPayload{
			AgentID:  "agent-1",
			Hostname: "host-1",
			Version:  "v1.0.0",
			Platform: "linux",
			Tags:     []string{"edge"},
			Token:    "ok",
		},
	}); err != nil {
		t.Fatalf("write register message: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read register ack: %v", err)
	}

	var ack struct {
		Type    agentexec.MessageType `json:"type"`
		Payload json.RawMessage       `json:"payload"`
	}
	if err := json.Unmarshal(data, &ack); err != nil {
		t.Fatalf("unmarshal ack: %v", err)
	}
	if ack.Type != agentexec.MsgTypeRegistered {
		t.Fatalf("ack type = %q, want %q", ack.Type, agentexec.MsgTypeRegistered)
	}

	adapter := newDiscoveryCommandAdapter(server)
	agents := adapter.GetConnectedAgents()
	if len(agents) != 1 {
		t.Fatalf("connected agents = %d, want 1", len(agents))
	}
	if agents[0].AgentID != "agent-1" {
		t.Fatalf("agent id = %q, want %q", agents[0].AgentID, "agent-1")
	}
	if agents[0].Hostname != "host-1" {
		t.Fatalf("hostname = %q, want %q", agents[0].Hostname, "host-1")
	}
	if agents[0].Version != "v1.0.0" {
		t.Fatalf("version = %q, want %q", agents[0].Version, "v1.0.0")
	}
	if agents[0].Platform != "linux" {
		t.Fatalf("platform = %q, want %q", agents[0].Platform, "linux")
	}
	if len(agents[0].Tags) != 1 || agents[0].Tags[0] != "edge" {
		t.Fatalf("tags = %#v, want [edge]", agents[0].Tags)
	}

	if !adapter.IsAgentConnected("agent-1") {
		t.Fatalf("expected agent-1 to be connected")
	}
	if adapter.IsAgentConnected("missing") {
		t.Fatalf("expected missing agent lookup to be false")
	}
}
