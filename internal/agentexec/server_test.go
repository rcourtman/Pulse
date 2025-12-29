package agentexec

import (
	"context"
	"testing"
	"time"
)

func TestExecuteCommandAgentNotConnected(t *testing.T) {
	s := NewServer(nil)
	_, err := s.ExecuteCommand(context.Background(), "missing", ExecuteCommandPayload{RequestID: "r1", Timeout: 1})
	if err == nil {
		t.Fatalf("expected error when agent not connected")
	}
}

func TestReadFileAgentNotConnected(t *testing.T) {
	s := NewServer(nil)
	_, err := s.ReadFile(context.Background(), "missing", ReadFilePayload{RequestID: "r1"})
	if err == nil {
		t.Fatalf("expected error when agent not connected")
	}
}

func TestConnectedAgentLookups(t *testing.T) {
	s := NewServer(nil)
	now := time.Now().Add(-1 * time.Minute)

	s.mu.Lock()
	s.agents["a1"] = &agentConn{agent: ConnectedAgent{AgentID: "a1", Hostname: "host1", ConnectedAt: now}}
	s.agents["a2"] = &agentConn{agent: ConnectedAgent{AgentID: "a2", Hostname: "host2", ConnectedAt: now}}
	s.mu.Unlock()

	if !s.IsAgentConnected("a1") {
		t.Fatalf("expected a1 to be connected")
	}
	if s.IsAgentConnected("missing") {
		t.Fatalf("expected missing to not be connected")
	}

	agentID, ok := s.GetAgentForHost("host2")
	if !ok || agentID != "a2" {
		t.Fatalf("expected GetAgentForHost(host2) = (a2, true), got (%q, %v)", agentID, ok)
	}
	if _, ok := s.GetAgentForHost("missing"); ok {
		t.Fatalf("expected missing host to return false")
	}

	agents := s.GetConnectedAgents()
	if len(agents) != 2 {
		t.Fatalf("expected 2 connected agents, got %d", len(agents))
	}
}
