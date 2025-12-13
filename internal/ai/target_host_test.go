package ai

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
)

// TestRouteToAgent_TargetHostExplicit tests that explicit target_host in context
// takes priority for routing decisions
func TestRouteToAgent_TargetHostExplicit(t *testing.T) {
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
		wantMethod      string
	}{
		{
			name: "explicit node in context routes correctly",
			req: ExecuteRequest{
				TargetType: "host", // run_on_host=true sets this
				TargetID:   "",     // run_on_host clears this
				Context:    map[string]interface{}{"node": "minipc"},
			},
			command:      "pct exec 106 -- hostname",
			wantAgentID:  "agent-2",
			wantHostname: "minipc",
			wantMethod:   "context_node",
		},
		{
			name: "guest_node also routes correctly for host commands",
			req: ExecuteRequest{
				TargetType: "host",
				TargetID:   "",
				Context:    map[string]interface{}{"guest_node": "pimox"},
			},
			command:      "qm guest exec 100 hostname",
			wantAgentID:  "agent-3",
			wantHostname: "pimox",
			wantMethod:   "context_guest_node",
		},
		{
			name: "node takes priority over guest_node",
			req: ExecuteRequest{
				TargetType: "host",
				TargetID:   "",
				Context: map[string]interface{}{
					"node":       "delly",
					"guest_node": "minipc", // Should be ignored when node is set
				},
			},
			command:      "uptime",
			wantAgentID:  "agent-1",
			wantHostname: "delly",
			wantMethod:   "context_node",
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

			if result.RoutingMethod != tt.wantMethod {
				t.Errorf("RoutingMethod = %q, want %q", result.RoutingMethod, tt.wantMethod)
			}
		})
	}
}

// TestRouteToAgent_SingleAgentFallback tests that with only one agent,
// we fall back to it with a warning
func TestRouteToAgent_SingleAgentFallback(t *testing.T) {
	s := &Service{}

	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-1", Hostname: "delly"},
	}

	req := ExecuteRequest{
		TargetType: "host",
		TargetID:   "",
		Context:    nil, // No context at all
	}

	result, err := s.routeToAgent(req, "uptime", agents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.AgentID != "agent-1" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "agent-1")
	}

	if result.RoutingMethod != "single_agent_fallback" {
		t.Errorf("RoutingMethod = %q, want %q", result.RoutingMethod, "single_agent_fallback")
	}

	// Should have a warning about the fallback
	if len(result.Warnings) == 0 {
		t.Error("expected warning about fallback routing")
	}
}

// TestRouteToAgent_MultiAgentNoContext tests that with multiple agents
// and no context, we get a clear error
func TestRouteToAgent_MultiAgentNoContext(t *testing.T) {
	s := &Service{}

	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-1", Hostname: "delly"},
		{AgentID: "agent-2", Hostname: "minipc"},
	}

	req := ExecuteRequest{
		TargetType: "host",
		TargetID:   "",
		Context:    nil, // No context
	}

	_, err := s.routeToAgent(req, "uptime", agents)
	if err == nil {
		t.Fatal("expected error when no context with multiple agents")
	}

	routingErr, ok := err.(*RoutingError)
	if !ok {
		t.Fatalf("expected RoutingError, got %T", err)
	}

	// Should mention target_host in the suggestion
	if routingErr.Suggestion == "" {
		t.Error("expected suggestion in error")
	}

	// Should list available agents
	if len(routingErr.AvailableAgents) != 2 {
		t.Errorf("expected 2 available agents, got %d", len(routingErr.AvailableAgents))
	}
}

// TestRouteToAgent_VMIDRoutingWithContext tests that VMID-based routing
// from context works correctly for pct/qm commands
func TestRouteToAgent_VMIDInCommandWithContext(t *testing.T) {
	s := &Service{}

	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-1", Hostname: "delly"},
		{AgentID: "agent-2", Hostname: "minipc"},
	}

	// Even with a VMID in the command, if we have node context, use it
	req := ExecuteRequest{
		TargetType: "host",
		TargetID:   "",
		Context:    map[string]interface{}{"node": "minipc"},
	}

	result, err := s.routeToAgent(req, "pct exec 106 -- hostname", agents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.AgentHostname != "minipc" {
		t.Errorf("AgentHostname = %q, want %q", result.AgentHostname, "minipc")
	}
}
