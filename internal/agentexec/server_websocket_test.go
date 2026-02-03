package agentexec

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type wsRawMessage struct {
	Type      MessageType      `json:"type"`
	ID        string           `json:"id,omitempty"`
	Timestamp time.Time        `json:"timestamp"`
	Payload   *json.RawMessage `json:"payload,omitempty"`
}

func newWSServer(t *testing.T, s *Server) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.HandleWebSocket(w, r)
	}))
}

func wsURLForHTTP(serverURL string) string {
	return "ws" + strings.TrimPrefix(serverURL, "http")
}

func wsWriteMessage(t *testing.T, conn *websocket.Conn, msg Message) {
	t.Helper()
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
}

func wsReadRawMessage(t *testing.T, conn *websocket.Conn) wsRawMessage {
	t.Helper()
	msg, err := wsReadRawMessageWithTimeout(conn, 2*time.Second)
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	return msg
}

func wsReadRegisteredPayload(t *testing.T, conn *websocket.Conn) RegisteredPayload {
	t.Helper()
	msg := wsReadRawMessage(t, conn)
	if msg.Type != MsgTypeRegistered {
		t.Fatalf("message type = %q, want %q", msg.Type, MsgTypeRegistered)
	}
	if msg.Payload == nil {
		t.Fatalf("registered payload missing")
	}
	var payload RegisteredPayload
	if err := json.Unmarshal(*msg.Payload, &payload); err != nil {
		t.Fatalf("unmarshal registered payload: %v", err)
	}
	return payload
}

func wsReadRawMessageWithTimeout(conn *websocket.Conn, timeout time.Duration) (wsRawMessage, error) {
	conn.SetReadDeadline(time.Now().Add(timeout))
	_, data, err := conn.ReadMessage()
	if err != nil {
		return wsRawMessage{}, err
	}
	var msg wsRawMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return wsRawMessage{}, err
	}
	return msg, nil
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v", timeout)
}

func TestHandleWebSocket_RegistrationSuccessAndDisconnectRemovesAgent(t *testing.T) {
	s := NewServer(func(token string, agentID string) bool { return token == "ok" })
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	wsWriteMessage(t, conn, Message{
		Type:      MsgTypeAgentRegister,
		Timestamp: time.Now(),
		Payload: AgentRegisterPayload{
			AgentID:  "a1",
			Hostname: "host1",
			Version:  "1.2.3",
			Platform: "linux",
			Tags:     []string{"tag1"},
			Token:    "ok",
		},
	})

	reg := wsReadRegisteredPayload(t, conn)
	if !reg.Success {
		t.Fatalf("registration failed: %q", reg.Message)
	}

	if !s.IsAgentConnected("a1") {
		t.Fatalf("expected agent to be connected")
	}

	conn.Close()

	waitFor(t, 2*time.Second, func() bool { return !s.IsAgentConnected("a1") })
}

func TestHandleWebSocket_InvalidTokenRejected(t *testing.T) {
	s := NewServer(func(string, string) bool { return false })
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	wsWriteMessage(t, conn, Message{
		Type:      MsgTypeAgentRegister,
		Timestamp: time.Now(),
		Payload: AgentRegisterPayload{
			AgentID:  "a1",
			Hostname: "host1",
			Version:  "1.2.3",
			Platform: "linux",
			Token:    "bad",
		},
	})

	reg := wsReadRegisteredPayload(t, conn)
	if reg.Success {
		t.Fatalf("expected registration to be rejected")
	}

	waitFor(t, 2*time.Second, func() bool { return !s.IsAgentConnected("a1") })

	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatalf("expected connection to be closed by server")
	}
}

func TestHandleWebSocket_FirstMessageMustBeRegister(t *testing.T) {
	s := NewServer(nil)
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	wsWriteMessage(t, conn, Message{
		Type:      MsgTypeAgentPing,
		Timestamp: time.Now(),
	})

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatalf("expected server to close connection")
	}
}

func TestHandleWebSocket_AgentPingRespondsWithPong(t *testing.T) {
	s := NewServer(nil)
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	wsWriteMessage(t, conn, Message{
		Type:      MsgTypeAgentRegister,
		Timestamp: time.Now(),
		Payload: AgentRegisterPayload{
			AgentID:  "a1",
			Hostname: "host1",
			Version:  "1.2.3",
			Platform: "linux",
			Token:    "any",
		},
	})
	_ = wsReadRegisteredPayload(t, conn)

	wsWriteMessage(t, conn, Message{
		Type:      MsgTypeAgentPing,
		Timestamp: time.Now(),
	})

	msg := wsReadRawMessage(t, conn)
	if msg.Type != MsgTypePong {
		t.Fatalf("message type = %q, want %q", msg.Type, MsgTypePong)
	}
}

func TestExecuteCommand_RoundTripViaWebSocket(t *testing.T) {
	s := NewServer(nil)
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	wsWriteMessage(t, conn, Message{
		Type:      MsgTypeAgentRegister,
		Timestamp: time.Now(),
		Payload: AgentRegisterPayload{
			AgentID:  "a1",
			Hostname: "host1",
			Version:  "1.2.3",
			Platform: "linux",
			Token:    "any",
		},
	})
	_ = wsReadRegisteredPayload(t, conn)

	agentDone := make(chan struct{})
	agentErr := make(chan error, 1)
	go func() {
		defer close(agentDone)
		for {
			msg, err := wsReadRawMessageWithTimeout(conn, 2*time.Second)
			if err != nil {
				agentErr <- err
				return
			}
			if msg.Type != MsgTypeExecuteCmd {
				continue
			}
			if msg.Payload == nil {
				agentErr <- nil
				return
			}
			var payload ExecuteCommandPayload
			if err := json.Unmarshal(*msg.Payload, &payload); err != nil {
				agentErr <- err
				return
			}
			conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			if err := conn.WriteJSON(Message{
				Type:      MsgTypeCommandResult,
				Timestamp: time.Now(),
				Payload: CommandResultPayload{
					RequestID: payload.RequestID,
					Success:   true,
					Stdout:    "ok",
					ExitCode:  0,
					Duration:  1,
				},
			}); err != nil {
				agentErr <- err
				return
			}
			agentErr <- nil
			return
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := s.ExecuteCommand(ctx, "a1", ExecuteCommandPayload{
		RequestID: "req1",
		Command:   "echo ok",
		Timeout:   1,
	})
	if err != nil {
		t.Fatalf("ExecuteCommand: %v", err)
	}
	if result == nil || !result.Success || result.Stdout != "ok" || result.ExitCode != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}

	select {
	case <-agentDone:
	case <-time.After(2 * time.Second):
		t.Fatalf("agent goroutine did not finish")
	}

	if err := <-agentErr; err != nil {
		t.Fatalf("agent error: %v", err)
	}
}

func TestHandleWebSocket_ReconnectSameAgentIDClosesOldConnection(t *testing.T) {
	s := NewServer(nil)
	ts := newWSServer(t, s)
	defer ts.Close()

	dial := func() *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL), nil)
		if err != nil {
			t.Fatalf("Dial: %v", err)
		}
		return conn
	}

	c1 := dial()
	defer c1.Close()
	wsWriteMessage(t, c1, Message{
		Type:      MsgTypeAgentRegister,
		Timestamp: time.Now(),
		Payload: AgentRegisterPayload{
			AgentID:  "a1",
			Hostname: "host1",
			Version:  "1.2.3",
			Platform: "linux",
			Token:    "any",
		},
	})
	_ = wsReadRegisteredPayload(t, c1)

	c2 := dial()
	defer c2.Close()
	wsWriteMessage(t, c2, Message{
		Type:      MsgTypeAgentRegister,
		Timestamp: time.Now(),
		Payload: AgentRegisterPayload{
			AgentID:  "a1",
			Hostname: "host1",
			Version:  "1.2.3",
			Platform: "linux",
			Token:    "any",
		},
	})
	_ = wsReadRegisteredPayload(t, c2)

	c1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err := c1.ReadMessage()
	if err == nil {
		t.Fatalf("expected old connection to be closed")
	}
}
