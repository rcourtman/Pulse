package agentexec

import (
	"sync"
	"testing"
	"time"
)

func TestActionProgressDeliveredOnlyForOwnedPendingRequest(t *testing.T) {
	srv := NewServer(func(token, agentID, hostname string) bool {
		return token == "tok"
	})
	defer srv.Shutdown()

	var mu sync.Mutex
	var received []ActionProgressPayload
	var receivedOrg, receivedAgent string
	delivered := make(chan struct{}, 4)
	srv.SetActionProgressHandler(func(orgID, agentID string, payload ActionProgressPayload) {
		mu.Lock()
		receivedOrg = orgID
		receivedAgent = agentID
		received = append(received, payload)
		mu.Unlock()
		select {
		case delivered <- struct{}{}:
		default:
		}
	})

	ts := newWSServer(t, srv)
	defer ts.Close()

	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	regMsg, _ := NewMessage(MsgTypeAgentRegister, "reg-1", AgentRegisterPayload{
		AgentID:  "agent-tower",
		Hostname: "Tower",
		Token:    "tok",
	})
	if err := conn.WriteJSON(regMsg); err != nil {
		t.Fatalf("write register: %v", err)
	}
	var regResp Message
	if err := conn.ReadJSON(&regResp); err != nil {
		t.Fatalf("read registered: %v", err)
	}

	// Progress for an unknown request_id must be ignored.
	spoofed, _ := NewMessage(MsgTypeActionProgress, "req-unknown", ActionProgressPayload{
		RequestID: "req-unknown",
		ActionID:  "act-spoofed",
		Message:   "should be dropped",
	})
	if err := conn.WriteJSON(spoofed); err != nil {
		t.Fatalf("write spoofed: %v", err)
	}

	// Register a real pending docker container update request for this connection.
	ac, ok := srv.connectionForOrganization(defaultOrganizationID, "agent-tower")
	if !ok {
		t.Fatalf("agent-tower not connected")
	}
	reqKey := pendingRequestKey(connectionAuthorityKey(ac), "req-real")
	srv.mu.Lock()
	srv.pendingDockerContainerUpdates[reqKey] = make(chan DockerContainerUpdateResultPayload, 1)
	srv.mu.Unlock()
	defer func() {
		srv.mu.Lock()
		delete(srv.pendingDockerContainerUpdates, reqKey)
		srv.mu.Unlock()
	}()

	valid, _ := NewMessage(MsgTypeActionProgress, "req-real", ActionProgressPayload{
		RequestID: "req-real",
		ActionID:  "act-123",
		Phase:     "pull",
		Message:   "Pulling image\tqmcgaw/gluetun:latest\x07\n",
	})
	if err := conn.WriteJSON(valid); err != nil {
		t.Fatalf("write valid: %v", err)
	}

	select {
	case <-delivered:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for action progress callback")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 progress payload, got %d: %+v", len(received), received)
	}
	if receivedOrg != defaultOrganizationID || receivedAgent != "agent-tower" {
		t.Fatalf("unexpected org/agent: %q / %q", receivedOrg, receivedAgent)
	}
	if received[0].ActionID != "act-123" || received[0].Message != "Pulling image qmcgaw/gluetun:latest" {
		t.Fatalf("unexpected sanitized payload: %+v", received[0])
	}
}
