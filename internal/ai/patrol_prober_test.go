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
)

type agentExecRawMessage struct {
	Type    agentexec.MessageType `json:"type"`
	Payload json.RawMessage       `json:"payload,omitempty"`
}

type pingAgentResult struct {
	command string
	err     error
}

func dialAndRegisterAgent(t *testing.T, serverURL, agentID, hostname string) *websocket.Conn {
	t.Helper()

	conn, _, err := websocket.DefaultDialer.Dial(agentExecWSURLForHTTP(serverURL), agentExecWSHeadersForHTTP(t, serverURL))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	regPayload, _ := json.Marshal(agentexec.AgentRegisterPayload{
		AgentID:  agentID,
		Hostname: hostname,
		Token:    "any",
	})
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := conn.WriteJSON(agentexec.Message{
		Type:      agentexec.MsgTypeAgentRegister,
		Timestamp: time.Now(),
		Payload:   regPayload,
	}); err != nil {
		conn.Close()
		t.Fatalf("WriteJSON register: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var ack agentExecRawMessage
	if err := conn.ReadJSON(&ack); err != nil {
		conn.Close()
		t.Fatalf("ReadJSON registration ack: %v", err)
	}
	if ack.Type != agentexec.MsgTypeRegistered {
		conn.Close()
		t.Fatalf("ack type = %q, want %q", ack.Type, agentexec.MsgTypeRegistered)
	}
	var payload agentexec.RegisteredPayload
	if err := json.Unmarshal(ack.Payload, &payload); err != nil {
		conn.Close()
		t.Fatalf("Unmarshal registration payload: %v", err)
	}
	if !payload.Success {
		conn.Close()
		t.Fatalf("registration failed: %q", payload.Message)
	}

	return conn
}

func TestAgentExecProber_PingGuestsNilServer(t *testing.T) {
	prober := NewAgentExecProber(nil)
	_, err := prober.PingGuests(context.Background(), "agent-1", []string{"10.0.0.1"})
	if err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("expected nil-server error, got: %v", err)
	}
}

func TestAgentExecProber_PingGuestsEmptyIPs(t *testing.T) {
	prober := NewAgentExecProber(agentexec.NewServer(func(string, string, string) bool { return true }))
	results, err := prober.PingGuests(context.Background(), "agent-1", nil)
	if err != nil {
		t.Fatalf("PingGuests with empty ips returned error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty result map, got %d entries", len(results))
	}
}

func TestAgentExecProber_RoundTripViaAgentExecServer(t *testing.T) {
	server := agentexec.NewServer(func(string, string, string) bool { return true })
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.HandleWebSocket(w, r)
	}))
	defer ts.Close()

	conn := dialAndRegisterAgent(t, ts.URL, "agent-1", "node-a")
	defer conn.Close()

	prober := NewAgentExecProber(server)
	if agentID, ok := prober.GetAgentForHost("node-a"); !ok || agentID != "agent-1" {
		t.Fatalf("GetAgentForHost(node-a) = (%q, %v), want (agent-1, true)", agentID, ok)
	}
	if _, ok := prober.GetAgentForHost("missing-node"); ok {
		t.Fatalf("expected missing host lookup to return false")
	}

	done := make(chan pingAgentResult, 1)
	go func() {
		commands := make([]string, 0, 2)
		for i := 0; i < 2; i++ {
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			var msg agentExecRawMessage
			if err := conn.ReadJSON(&msg); err != nil {
				done <- pingAgentResult{err: err}
				return
			}
			if msg.Type != agentexec.MsgTypeExecuteCmd {
				done <- pingAgentResult{err: fmt.Errorf("message type = %q, want %q", msg.Type, agentexec.MsgTypeExecuteCmd)}
				return
			}

			var payload agentexec.ExecuteCommandPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				done <- pingAgentResult{err: err}
				return
			}
			commands = append(commands, payload.Command)
			if strings.ContainsAny(payload.Command, ";&|`$<>") {
				done <- pingAgentResult{err: fmt.Errorf("ping command uses shell control: %q", payload.Command)}
				return
			}

			exitCode := 1
			success := false
			switch payload.Command {
			case "ping -c 1 -W 1 10.0.0.1":
				exitCode = 0
				success = true
			case "ping -c 1 -W 1 10.0.0.2":
			default:
				done <- pingAgentResult{err: fmt.Errorf("unexpected ping command: %q", payload.Command)}
				return
			}

			cmdResult, _ := json.Marshal(agentexec.CommandResultPayload{
				RequestID: payload.RequestID,
				Success:   success,
				ExitCode:  exitCode,
			})
			_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			if err := conn.WriteJSON(agentexec.Message{
				Type:      agentexec.MsgTypeCommandResult,
				Timestamp: time.Now(),
				Payload:   cmdResult,
			}); err != nil {
				done <- pingAgentResult{err: err}
				return
			}
		}
		done <- pingAgentResult{command: strings.Join(commands, "\n")}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	results, err := prober.PingGuests(ctx, "agent-1", []string{"10.0.0.1", "10.0.0.2"})
	if err != nil {
		t.Fatalf("PingGuests returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 ping results, got %d", len(results))
	}
	if !results["10.0.0.1"].Reachable {
		t.Fatalf("expected 10.0.0.1 to be reachable")
	}
	if results["10.0.0.2"].Reachable {
		t.Fatalf("expected 10.0.0.2 to be unreachable")
	}

	agent := <-done
	if agent.err != nil {
		t.Fatalf("agent loop error: %v", agent.err)
	}
	if !strings.Contains(agent.command, "ping -c 1 -W 1 10.0.0.1") || !strings.Contains(agent.command, "ping -c 1 -W 1 10.0.0.2") {
		t.Fatalf("commands did not include expected IPs: %s", agent.command)
	}
}

func TestAgentExecProber_RejectsInvalidPingTargets(t *testing.T) {
	prober := NewAgentExecProber(agentexec.NewServer(func(string, string, string) bool { return true }))

	_, err := prober.PingGuests(context.Background(), "agent-1", []string{"10.0.0.1; rm -rf /"})
	if err == nil || !strings.Contains(err.Error(), "invalid ping target") {
		t.Fatalf("expected invalid ping target error, got: %v", err)
	}
}
