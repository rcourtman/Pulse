package ai

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
	// "github.com/rcourtman/pulse-go-rewrite/internal/agentexec" // Needed if I can instantiate Server
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
