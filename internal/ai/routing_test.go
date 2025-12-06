package ai

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
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
		{"instance-node-vmid", "delly-minipc-106", 106},
		
		// Edge cases with hyphenated names
		{"hyphenated-node-vmid", "pve-node-01-106", 106},
		{"hyphenated-instance-node-vmid", "my-cluster-pve-node-01-106", 106},
		
		// Type prefixes
		{"lxc prefix", "lxc-106", 106},
		{"vm prefix", "vm-106", 106},
		{"ct prefix", "ct-106", 106},
		
		// Non-numeric - should return 0
		{"non-numeric", "mycontainer", 0},
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
			AvailableAgents: []string{"delly", "pimox"},
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
		{AgentID: "agent-1", Hostname: "delly"},
		{AgentID: "agent-2", Hostname: "minipc"},
		{AgentID: "agent-3", Hostname: "pimox"},
	}
	
	tests := []struct {
		name            string
		req             ExecuteRequest
		command         string
		wantAgentID     string
		wantHostname    string
	}{
		{
			name: "route by context node",
			req: ExecuteRequest{
				TargetType: "container",
				TargetID:   "delly-minipc-106",
				Context:    map[string]interface{}{"node": "minipc"},
			},
			command:      "hostname",
			wantAgentID:  "agent-2",
			wantHostname: "minipc",
		},
		{
			name: "route by context hostname for host target",
			req: ExecuteRequest{
				TargetType: "host",
				Context:    map[string]interface{}{"hostname": "delly"},
			},
			command:      "uptime",
			wantAgentID:  "agent-1",
			wantHostname: "delly",
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
		{AgentID: "agent-1", Hostname: "delly"},
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
