package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
)

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
	if res.RequestID == "" {
		t.Error("Expected adapter to provide a non-empty request ID")
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
		TargetType: "agent",
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

	regPayload, _ := json.Marshal(agentexec.AgentRegisterPayload{
		AgentID:  "agent-1",
		Hostname: "host-1",
		Version:  "v1.0.0",
		Platform: "linux",
		Tags:     []string{"edge"},
		Token:    "ok",
	})
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := conn.WriteJSON(agentexec.Message{
		Type:      agentexec.MsgTypeAgentRegister,
		Timestamp: time.Now(),
		Payload:   regPayload,
	}); err != nil {
		t.Fatalf("write register message: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
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

func TestDiscoveryCommandAdapter_ExecuteCommandSuccess(t *testing.T) {
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

	regPayload2, _ := json.Marshal(agentexec.AgentRegisterPayload{
		AgentID:  "agent-1",
		Hostname: "host-1",
		Version:  "v1.0.0",
		Platform: "linux",
		Token:    "ok",
	})
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := conn.WriteJSON(agentexec.Message{
		Type:      agentexec.MsgTypeAgentRegister,
		Timestamp: time.Now(),
		Payload:   regPayload2,
	}); err != nil {
		t.Fatalf("write register message: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("read register ack: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			errCh <- fmt.Errorf("read execute command: %w", err)
			return
		}

		var msg struct {
			Type    agentexec.MessageType           `json:"type"`
			ID      string                          `json:"id"`
			Payload agentexec.ExecuteCommandPayload `json:"payload"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			errCh <- fmt.Errorf("unmarshal execute command: %w", err)
			return
		}

		if msg.Type != agentexec.MsgTypeExecuteCmd {
			errCh <- fmt.Errorf("execute command type = %q, want %q", msg.Type, agentexec.MsgTypeExecuteCmd)
			return
		}
		if msg.ID != "req-3" {
			errCh <- fmt.Errorf("message id = %q, want req-3", msg.ID)
			return
		}
		if msg.Payload.Command != "uname -a" || msg.Payload.TargetType != "container" || msg.Payload.TargetID != "200" || msg.Payload.Timeout != 2 {
			errCh <- fmt.Errorf("unexpected execute command payload: %#v", msg.Payload)
			return
		}

		resultPayload, _ := json.Marshal(agentexec.CommandResultPayload{
			RequestID: msg.Payload.RequestID,
			Success:   true,
			Stdout:    "ok",
			Stderr:    "warn",
			ExitCode:  0,
			Duration:  1234,
		})
		_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		err = conn.WriteJSON(agentexec.Message{
			Type:      agentexec.MsgTypeCommandResult,
			Timestamp: time.Now(),
			Payload:   resultPayload,
		})
		if err != nil {
			errCh <- fmt.Errorf("write command result: %w", err)
			return
		}
		errCh <- nil
	}()

	adapter := newDiscoveryCommandAdapter(server)
	res, err := adapter.ExecuteCommand(context.Background(), "agent-1", servicediscovery.ExecuteCommandPayload{
		RequestID:  "req-3",
		Command:    "uname -a",
		TargetType: "container",
		TargetID:   "200",
		Timeout:    2,
	})
	if err != nil {
		t.Fatalf("ExecuteCommand returned error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected command success, got failure: %q", res.Error)
	}
	if res.RequestID != "req-3" {
		t.Fatalf("request id = %q, want req-3", res.RequestID)
	}
	if res.Stdout != "ok" || res.Stderr != "warn" || res.ExitCode != 0 || res.Duration != 1234 {
		t.Fatalf("unexpected command response: %#v", res)
	}

	if asyncErr := <-errCh; asyncErr != nil {
		t.Fatal(asyncErr)
	}
}
