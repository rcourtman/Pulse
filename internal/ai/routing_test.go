package ai

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestExtractVMIDFromTargetID(t *testing.T) {
	tests := []struct {
		name     string
		targetID string
		want     int
	}{
		// Standard formats
		{"plain vmid", "106", 106},
		{"node-vmid", "minipc-106", 106},
		{"instance-node-vmid", "pve-node-minipc-106", 106},
		{"colon-delimited", "pve-node:minipc:112", 112},
		{"slash-delimited", "cluster/node/205", 205},

		// Edge cases with hyphenated names
		{"hyphenated-node-vmid", "pve-node-01-106", 106},
		{"hyphenated-instance-node-vmid", "my-cluster-pve-node-01-106", 106},

		// Type prefixes
		{"lxc prefix", "lxc-106", 106},
		{"vm prefix", "vm-106", 106},
		{"ct prefix", "ct-106", 106},

		// Non-numeric - should return 0
		{"non-numeric", "mycontainer", 0},
		{"trailing-digits-without-separator", "mycontainer123", 0},
		{"no-vmid", "node-name", 0},
		{"empty", "", 0},

		// Large VMIDs (Proxmox uses up to 999999999)
		{"large vmid", "node-999999", 999999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractVMIDFromTargetID(tt.targetID)
			if got != tt.want {
				t.Errorf("extractVMIDFromTargetID(%q) = %d, want %d", tt.targetID, got, tt.want)
			}
		})
	}
}

func TestRoutingError(t *testing.T) {
	t.Run("with suggestion", func(t *testing.T) {
		err := &RoutingError{
			TargetNode:      "minipc",
			AvailableAgents: []string{"pve-node", "pimox"},
			Reason:          "No agent connected to node \"minipc\"",
			Suggestion:      "Install pulse-agent on minipc",
		}

		want := "No agent connected to node \"minipc\". Install pulse-agent on minipc"
		if err.Error() != want {
			t.Errorf("Error() = %q, want %q", err.Error(), want)
		}
	})

	t.Run("without suggestion", func(t *testing.T) {
		err := &RoutingError{
			Reason: "No agents connected",
		}

		want := "No agents connected"
		if err.Error() != want {
			t.Errorf("Error() = %q, want %q", err.Error(), want)
		}
	})
}

func TestRouteToAgent_NoAgents(t *testing.T) {
	s := &Service{}

	req := ExecuteRequest{
		TargetType: "container",
		TargetID:   "minipc-106",
	}

	_, err := s.routeToAgent(req, "pct exec 106 -- hostname", nil)
	if err == nil {
		t.Error("expected error for no agents, got nil")
	}

	routingErr, ok := err.(*RoutingError)
	if !ok {
		t.Fatalf("expected RoutingError, got %T", err)
	}

	if routingErr.Suggestion == "" {
		t.Error("expected suggestion in error")
	}
}

func TestRouteToAgent_ExactMatch(t *testing.T) {
	s := &Service{}

	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-1", Hostname: "pve-node"},
		{AgentID: "agent-2", Hostname: "minipc"},
		{AgentID: "agent-3", Hostname: "pimox"},
	}

	tests := []struct {
		name         string
		req          ExecuteRequest
		command      string
		wantAgentID  string
		wantHostname string
	}{
		{
			name: "route by context node",
			req: ExecuteRequest{
				TargetType: "container",
				TargetID:   "pve-node-minipc-106",
				Context:    map[string]interface{}{"node": "minipc"},
			},
			command:      "hostname",
			wantAgentID:  "agent-2",
			wantHostname: "minipc",
		},
		{
			name: "route by context hostname for host target",
			req: ExecuteRequest{
				TargetType: "agent",
				Context:    map[string]interface{}{"hostname": "pve-node"},
			},
			command:      "uptime",
			wantAgentID:  "agent-1",
			wantHostname: "pve-node",
		},
		{
			name: "route by guest_node context",
			req: ExecuteRequest{
				TargetType: "vm",
				TargetID:   "100",
				Context:    map[string]interface{}{"guest_node": "pimox"},
			},
			command:      "hostname",
			wantAgentID:  "agent-3",
			wantHostname: "pimox",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := s.routeToAgent(tt.req, tt.command, agents)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.AgentID != tt.wantAgentID {
				t.Errorf("AgentID = %q, want %q", result.AgentID, tt.wantAgentID)
			}

			if result.AgentHostname != tt.wantHostname {
				t.Errorf("AgentHostname = %q, want %q", result.AgentHostname, tt.wantHostname)
			}
		})
	}
}

func TestRouteToAgent_NoSubstringMatching(t *testing.T) {
	s := &Service{}

	// Agent "mini" should NOT match node "minipc"
	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-1", Hostname: "mini"},
		{AgentID: "agent-2", Hostname: "pc"},
	}

	req := ExecuteRequest{
		TargetType: "container",
		Context:    map[string]interface{}{"node": "minipc"},
	}

	_, err := s.routeToAgent(req, "hostname", agents)
	if err == nil {
		t.Error("expected error when no exact match, got nil (substring matching may be occurring)")
	}

	routingErr, ok := err.(*RoutingError)
	if !ok {
		t.Fatalf("expected RoutingError, got %T", err)
	}

	if routingErr.TargetNode != "minipc" {
		t.Errorf("TargetNode = %q, want %q", routingErr.TargetNode, "minipc")
	}
}

func TestRouteToAgent_CaseInsensitive(t *testing.T) {
	s := &Service{}

	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-1", Hostname: "MiniPC"},
	}

	req := ExecuteRequest{
		TargetType: "container",
		Context:    map[string]interface{}{"node": "minipc"}, // lowercase
	}

	result, err := s.routeToAgent(req, "hostname", agents)
	if err != nil {
		t.Fatalf("expected case-insensitive match, got error: %v", err)
	}

	if result.AgentID != "agent-1" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "agent-1")
	}
}

func TestRouteToAgent_HyphenatedNodeNames(t *testing.T) {
	s := &Service{}

	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-1", Hostname: "pve-node-01"},
		{AgentID: "agent-2", Hostname: "pve-node-02"},
	}

	req := ExecuteRequest{
		TargetType: "container",
		Context:    map[string]interface{}{"node": "pve-node-02"},
	}

	result, err := s.routeToAgent(req, "hostname", agents)
	if err != nil {
		t.Fatalf("unexpected error for hyphenated node names: %v", err)
	}

	if result.AgentID != "agent-2" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "agent-2")
	}
}

func TestRouteToAgent_ActionableErrorMessages(t *testing.T) {
	s := &Service{}

	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-1", Hostname: "pve-node"},
	}

	req := ExecuteRequest{
		TargetType: "container",
		Context:    map[string]interface{}{"node": "minipc"},
	}

	_, err := s.routeToAgent(req, "hostname", agents)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	routingErr, ok := err.(*RoutingError)
	if !ok {
		t.Fatalf("expected RoutingError, got %T", err)
	}

	// Error should mention the target node
	if routingErr.TargetNode != "minipc" {
		t.Errorf("TargetNode = %q, want %q", routingErr.TargetNode, "minipc")
	}

	// Error should list available agents
	if len(routingErr.AvailableAgents) == 0 {
		t.Error("expected available agents in error")
	}

	// Error should have actionable suggestion
	if routingErr.Suggestion == "" {
		t.Error("expected suggestion in error message")
	}
}

func TestRoutingError_ForAI(t *testing.T) {
	t.Run("clarification", func(t *testing.T) {
		err := &RoutingError{
			Reason:              "Cannot determine which host should execute this command",
			AvailableAgents:     []string{"pve-node", "pimox"},
			AskForClarification: true,
		}

		msg := err.ForAI()
		if !strings.Contains(msg, "ROUTING_CLARIFICATION_NEEDED") {
			t.Errorf("expected clarification marker, got %q", msg)
		}
		if !strings.Contains(msg, "pve-node, pimox") {
			t.Errorf("expected available hosts list, got %q", msg)
		}
		if !strings.Contains(msg, err.Reason) {
			t.Errorf("expected reason to be included, got %q", msg)
		}
	})

	t.Run("fallback", func(t *testing.T) {
		err := &RoutingError{
			Reason:              "No agents connected",
			AskForClarification: true,
		}
		if err.ForAI() != err.Error() {
			t.Errorf("expected ForAI to fall back to Error, got %q", err.ForAI())
		}
	})
}

func TestRouteToAgent_VMIDRoutingWithInstance(t *testing.T) {
	// VMID 106 exists on two nodes within the same instance (cluster-b).
	// The routing code calls lookupGuestsByVMID(106, "cluster-b") which returns
	// both containers (multiple matches), then the collision resolution path
	// picks the first match whose instance matches the request context.
	snapshot := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node/node-a", Name: "node-a", Instance: "cluster-b", Status: "online"},
			{ID: "node/node-b", Name: "node-b", Instance: "cluster-b", Status: "online"},
		},
		Containers: []models.Container{
			{ID: "lxc/106-b", VMID: 106, Node: "node-b", Name: "ct-b", Instance: "cluster-b"},
			{ID: "lxc/106-a", VMID: 106, Node: "node-a", Name: "ct-a", Instance: "cluster-b"},
		},
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)

	s := &Service{readState: registry}
	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-a", Hostname: "node-a"},
		{AgentID: "agent-b", Hostname: "node-b"},
	}
	req := ExecuteRequest{
		TargetType: "container",
		Context:    map[string]interface{}{"instance": "cluster-b"},
	}

	result, err := s.routeToAgent(req, "pct exec 106 -- hostname", agents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Both containers match instance "cluster-b", so we get vmid_lookup_with_instance routing.
	// The first matching container's node determines the agent.
	if result.RoutingMethod != "vmid_lookup_with_instance" {
		t.Errorf("RoutingMethod = %q, want %q", result.RoutingMethod, "vmid_lookup_with_instance")
	}
	// The result should route to one of the agents that has a container with VMID 106
	if result.AgentID != "agent-a" && result.AgentID != "agent-b" {
		t.Errorf("AgentID = %q, want agent-a or agent-b", result.AgentID)
	}
}

func TestRouteToAgent_VMIDCollision(t *testing.T) {
	snapshot := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node/node-a", Name: "node-a", Instance: "cluster-a", Status: "online"},
			{ID: "node/node-b", Name: "node-b", Instance: "cluster-b", Status: "online"},
		},
		VMs: []models.VM{
			{ID: "qemu/200-a", VMID: 200, Node: "node-a", Name: "vm-a", Instance: "cluster-a"},
			{ID: "qemu/200-b", VMID: 200, Node: "node-b", Name: "vm-b", Instance: "cluster-b"},
		},
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)

	s := &Service{readState: registry}
	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-a", Hostname: "node-a"},
		{AgentID: "agent-b", Hostname: "node-b"},
	}
	req := ExecuteRequest{
		TargetType: "vm",
	}

	_, err := s.routeToAgent(req, "pct exec 200 -- hostname", agents)
	if err == nil {
		t.Fatal("expected error for VMID collision, got nil")
	}
	routingErr, ok := err.(*RoutingError)
	if !ok {
		t.Fatalf("expected RoutingError, got %T", err)
	}
	if routingErr.TargetVMID != 200 {
		t.Errorf("TargetVMID = %d, want %d", routingErr.TargetVMID, 200)
	}
	if !strings.Contains(routingErr.Reason, "exists on multiple nodes") {
		t.Errorf("expected collision reason, got %q", routingErr.Reason)
	}
}

func TestRouteToAgent_VMIDNotFoundFallsBackToContext(t *testing.T) {
	registry := unifiedresources.NewRegistry(nil)
	s := &Service{readState: registry}

	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-1", Hostname: "minipc"},
	}
	req := ExecuteRequest{
		TargetType: "container",
		Context:    map[string]interface{}{"node": "minipc"},
	}

	result, err := s.routeToAgent(req, "pct exec 106 -- hostname", agents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RoutingMethod != "context_node" {
		t.Errorf("RoutingMethod = %q, want %q", result.RoutingMethod, "context_node")
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "VMID 106 not found") {
		t.Errorf("expected warning about missing VMID, got %v", result.Warnings)
	}
}

func TestRouteToAgent_UnifiedProviderContexts(t *testing.T) {
	s := &Service{}
	mockURP := &mockUnifiedResourceProvider{
		findContainerHostFunc: func(containerNameOrID string) string {
			if containerNameOrID == "" {
				return ""
			}
			return "rp-host"
		},
	}
	s.unifiedResourceProvider = mockURP

	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-1", Hostname: "rp-host"},
	}

	tests := []struct {
		name string
		key  string
	}{
		{name: "containerName", key: "containerName"},
		{name: "name", key: "name"},
		{name: "guestName", key: "guestName"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ExecuteRequest{
				TargetType: "container",
				Context:    map[string]interface{}{tt.key: "workload"},
			}

			result, err := s.routeToAgent(req, "docker ps", agents)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.RoutingMethod != "resource_provider_lookup" {
				t.Errorf("RoutingMethod = %q, want %q", result.RoutingMethod, "resource_provider_lookup")
			}
		})
	}
}

func TestRouteToAgent_TargetIDLookup(t *testing.T) {
	snapshot := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node/node-vm", Name: "node-vm", Instance: "cluster-a", Status: "online"},
		},
		VMs: []models.VM{
			{ID: "qemu/222", VMID: 222, Node: "node-vm", Name: "vm-222", Instance: "cluster-a"},
		},
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)

	s := &Service{readState: registry}
	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-1", Hostname: "node-vm"},
	}
	req := ExecuteRequest{
		TargetType: "vm",
		TargetID:   "vm-222",
		Context:    map[string]interface{}{"instance": "cluster-a"},
	}

	result, err := s.routeToAgent(req, "status", agents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RoutingMethod != "target_id_vmid_lookup" {
		t.Errorf("RoutingMethod = %q, want %q", result.RoutingMethod, "target_id_vmid_lookup")
	}
	if result.AgentID != "agent-1" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "agent-1")
	}
}

func TestRouteToAgent_VMIDLookupSingleMatch(t *testing.T) {
	snapshot := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "node/node-1", Name: "node-1", Instance: "default", Status: "online"},
		},
		VMs: []models.VM{
			{ID: "qemu/101", VMID: 101, Node: "node-1", Name: "vm-one", Instance: "default"},
		},
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)

	s := &Service{readState: registry}
	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-1", Hostname: "node-1"},
	}
	req := ExecuteRequest{
		TargetType: "vm",
	}

	result, err := s.routeToAgent(req, "qm start 101", agents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RoutingMethod != "vmid_lookup" {
		t.Errorf("RoutingMethod = %q, want %q", result.RoutingMethod, "vmid_lookup")
	}
	if result.AgentID != "agent-1" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "agent-1")
	}
}

func TestRouteToAgent_ClusterPeer(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	err := persistence.SaveNodesConfig([]config.PVEInstance{
		{
			Name:        "node-a",
			IsCluster:   true,
			ClusterName: "cluster-a",
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "node-a"},
				{NodeName: "node-b"},
			},
		},
	}, nil, nil)
	if err != nil {
		t.Fatalf("SaveNodesConfig: %v", err)
	}

	s := &Service{persistence: persistence}
	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-b", Hostname: "node-b"},
	}
	req := ExecuteRequest{
		TargetType: "vm",
		Context:    map[string]interface{}{"node": "node-a"},
	}

	result, err := s.routeToAgent(req, "hostname", agents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.ClusterPeer {
		t.Error("expected cluster peer routing")
	}
	if result.AgentID != "agent-b" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "agent-b")
	}
}

func TestRouteToAgent_SingleAgentFallbackForHost(t *testing.T) {
	s := &Service{}
	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-1", Hostname: "only"},
	}
	req := ExecuteRequest{
		TargetType: "agent",
	}

	result, err := s.routeToAgent(req, "uptime", agents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RoutingMethod != "single_agent_fallback" {
		t.Errorf("RoutingMethod = %q, want %q", result.RoutingMethod, "single_agent_fallback")
	}
	if len(result.Warnings) != 1 {
		t.Errorf("expected warning for single agent fallback, got %v", result.Warnings)
	}
}

func TestRouteToAgent_AsksForClarification(t *testing.T) {
	s := &Service{}
	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-1", Hostname: "pve-node"},
		{AgentID: "agent-2", Hostname: "pimox"},
	}

	req := ExecuteRequest{
		TargetType: "vm",
	}

	_, err := s.routeToAgent(req, "hostname", agents)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	routingErr, ok := err.(*RoutingError)
	if !ok {
		t.Fatalf("expected RoutingError, got %T", err)
	}
	if !routingErr.AskForClarification {
		t.Error("expected AskForClarification to be true")
	}
	if len(routingErr.AvailableAgents) != 2 {
		t.Errorf("expected available agents, got %v", routingErr.AvailableAgents)
	}
}

func TestFindClusterPeerAgent_NoPersistence(t *testing.T) {
	s := &Service{}
	if got := s.findClusterPeerAgent("node-a", nil); got != "" {
		t.Errorf("expected empty result, got %q", got)
	}
}

func TestFindClusterPeerAgent_LoadError(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	nodesPath := filepath.Join(tmp, "nodes.enc")
	if err := os.Mkdir(nodesPath, 0700); err != nil {
		t.Fatalf("Mkdir nodes.enc: %v", err)
	}

	s := &Service{persistence: persistence}
	if got := s.findClusterPeerAgent("node-a", nil); got != "" {
		t.Errorf("expected empty result, got %q", got)
	}
}

func TestFindClusterPeerAgent_NotCluster(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	err := persistence.SaveNodesConfig([]config.PVEInstance{
		{Name: "node-a"},
	}, nil, nil)
	if err != nil {
		t.Fatalf("SaveNodesConfig: %v", err)
	}

	s := &Service{persistence: persistence}
	if got := s.findClusterPeerAgent("node-a", nil); got != "" {
		t.Errorf("expected empty result, got %q", got)
	}
}

func TestFindClusterPeerAgent_NoAgentMatch(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	err := persistence.SaveNodesConfig([]config.PVEInstance{
		{
			Name:        "node-a",
			IsCluster:   true,
			ClusterName: "cluster-a",
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "node-a"},
				{NodeName: "node-b"},
			},
		},
	}, nil, nil)
	if err != nil {
		t.Fatalf("SaveNodesConfig: %v", err)
	}

	s := &Service{persistence: persistence}
	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-1", Hostname: "node-c"},
	}
	if got := s.findClusterPeerAgent("node-a", agents); got != "" {
		t.Errorf("expected empty result, got %q", got)
	}
}

func TestFindClusterPeerAgent_EndpointMatch(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	err := persistence.SaveNodesConfig([]config.PVEInstance{
		{
			Name:        "cluster-master",
			IsCluster:   true,
			ClusterName: "cluster-a",
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "node-a"},
				{NodeName: "node-b"},
			},
		},
	}, nil, nil)
	if err != nil {
		t.Fatalf("SaveNodesConfig: %v", err)
	}

	s := &Service{persistence: persistence}
	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-b", Hostname: "node-b"},
	}
	if got := s.findClusterPeerAgent("node-a", agents); got != "agent-b" {
		t.Errorf("expected peer agent, got %q", got)
	}
}
