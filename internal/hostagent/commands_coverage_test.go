package hostagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

func TestCommandClient_LoopsCoverage(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()

		// 1. Receive registration
		var msg wsMessage
		conn.ReadJSON(&msg)

		// 2. Send registered success
		payload, _ := json.Marshal(registeredPayload{Success: true})
		conn.WriteJSON(wsMessage{Type: msgTypeRegistered, Payload: payload})

		// 3. Send a pong (will be ignored)
		conn.WriteJSON(wsMessage{Type: msgTypePong})

		// 4. Send an execute_command
		cmdPayload, _ := json.Marshal(executeCommandPayload{
			RequestID: "req-1",
			Command:   "echo hi",
		})
		conn.WriteJSON(wsMessage{Type: msgTypeExecuteCmd, Payload: cmdPayload})

		// 5. Send a read_file
		readPayload, _ := json.Marshal(executeCommandPayload{
			RequestID: "req-2",
			Command:   "cat /etc/hostname",
		})
		conn.WriteJSON(wsMessage{Type: msgTypeReadFile, Payload: readPayload})

		// 6. Send unknown message type
		conn.WriteJSON(wsMessage{Type: "unknown_type"})

		// 7. Send invalid JSON for execute_command
		conn.WriteJSON(wsMessage{Type: msgTypeExecuteCmd, Payload: json.RawMessage(`{invalid}`)})

		// 8. Send invalid JSON for read_file
		conn.WriteJSON(wsMessage{Type: msgTypeReadFile, Payload: json.RawMessage(`{invalid}`)})

		// Wait a bit then close
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	logger := zerolog.Nop()
	cfg := Config{
		PulseURL: "http" + strings.TrimPrefix(server.URL, "http"),
		Logger:   &logger,
	}

	client := NewCommandClient(cfg, "agent", "host", "linux", "1.0")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Run in background and stop after a short while
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	err := client.Run(ctx)
	if err != nil && err != context.Canceled {
		t.Logf("Run returned error (expected during shutdown): %v", err)
	}
}

func TestCommandClient_PingLoop_Direct(t *testing.T) {
	// Directly test pingLoop logic for coverage
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	dialer := websocket.Dialer{}
	conn, _, _ := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	defer conn.Close()

	client := &CommandClient{
		conn:   conn,
		logger: zerolog.Nop(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	// We can't wait for the ticker easily, so we just run it briefly
	go client.pingLoop(ctx, conn, done)

	time.Sleep(50 * time.Millisecond)
	close(done)
}

func TestCommandClient_RegistrationFailure(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()

		// Receive registration
		var msg wsMessage
		conn.ReadJSON(&msg)

		// Send registered FAILURE
		payload, _ := json.Marshal(registeredPayload{Success: false, Message: "registration rejected"})
		conn.WriteJSON(wsMessage{Type: msgTypeRegistered, Payload: payload})

		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	logger := zerolog.Nop()
	cfg := Config{
		PulseURL: "http" + strings.TrimPrefix(server.URL, "http"),
		Logger:   &logger,
	}
	client := NewCommandClient(cfg, "agent", "host", "linux", "1.0")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := client.Run(ctx)
	if err == nil {
		t.Error("expected error from registration failure")
	}
}
