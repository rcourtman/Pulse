package hostagent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

func wsURLForHTTP(serverURL string) string {
	return "ws" + strings.TrimPrefix(serverURL, "http")
}

func TestCommandClient_sendRegistration_WritesExpectedPayload(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

	gotMsgCh := make(chan wsMessage, 1)
	gotPayloadCh := make(chan registerPayload, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()

		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var msg wsMessage
		if err := conn.ReadJSON(&msg); err != nil {
			t.Errorf("ReadJSON: %v", err)
			return
		}

		var payload registerPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			t.Errorf("unmarshal payload: %v", err)
			return
		}

		gotMsgCh <- msg
		gotPayloadCh <- payload
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(server.URL), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	client := &CommandClient{
		apiToken: "token-1",
		agentID:  "agent-1",
		hostname: "host-1",
		platform: "linux",
		version:  "1.2.3",
		logger:   zerolog.Nop(),
	}

	if err := client.sendRegistration(conn); err != nil {
		t.Fatalf("sendRegistration: %v", err)
	}

	msg := <-gotMsgCh
	if msg.Type != msgTypeAgentRegister {
		t.Fatalf("msg.Type = %q, want %q", msg.Type, msgTypeAgentRegister)
	}
	if msg.Timestamp.IsZero() {
		t.Fatalf("expected non-zero Timestamp")
	}

	payload := <-gotPayloadCh
	if payload.AgentID != "agent-1" {
		t.Fatalf("payload.AgentID = %q, want %q", payload.AgentID, "agent-1")
	}
	if payload.Hostname != "host-1" {
		t.Fatalf("payload.Hostname = %q, want %q", payload.Hostname, "host-1")
	}
	if payload.Platform != "linux" {
		t.Fatalf("payload.Platform = %q, want %q", payload.Platform, "linux")
	}
	if payload.Version != "1.2.3" {
		t.Fatalf("payload.Version = %q, want %q", payload.Version, "1.2.3")
	}
	if payload.Token != "token-1" {
		t.Fatalf("payload.Token = %q, want %q", payload.Token, "token-1")
	}
}

func TestCommandClient_waitForRegistration_AcceptsSuccess(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()

		_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		payload, _ := json.Marshal(registeredPayload{Success: true, Message: "Registered"})
		_ = conn.WriteJSON(wsMessage{Type: msgTypeRegistered, Timestamp: time.Now(), Payload: payload})
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(server.URL), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	client := &CommandClient{logger: zerolog.Nop()}
	if err := client.waitForRegistration(conn); err != nil {
		t.Fatalf("waitForRegistration: %v", err)
	}
}

func TestCommandClient_waitForRegistration_RejectsFailure(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()

		_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		payload, _ := json.Marshal(registeredPayload{Success: false, Message: "Invalid token"})
		_ = conn.WriteJSON(wsMessage{Type: msgTypeRegistered, Timestamp: time.Now(), Payload: payload})
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(server.URL), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	client := &CommandClient{logger: zerolog.Nop()}
	if err := client.waitForRegistration(conn); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCommandClient_waitForRegistration_UnexpectedMessageType(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()

		_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_ = conn.WriteJSON(wsMessage{Type: msgTypePong, Timestamp: time.Now()})
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(server.URL), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	client := &CommandClient{logger: zerolog.Nop()}
	if err := client.waitForRegistration(conn); err == nil {
		t.Fatalf("expected error")
	}
}
