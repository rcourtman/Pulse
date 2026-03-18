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
	origDelay := reconnectDelay
	reconnectDelay = 0
	t.Cleanup(func() { reconnectDelay = origDelay })

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()

		// 1. Receive registration
		var msg wsMessage
		_ = conn.ReadJSON(&msg)

		// 2. Send registered success
		payload, _ := json.Marshal(registeredPayload{Success: true})
		_ = conn.WriteJSON(wsMessage{Type: msgTypeRegistered, Payload: payload})

		// 3. Send a pong (will be ignored)
		_ = conn.WriteJSON(wsMessage{Type: msgTypePong})

		// 4. Send an execute_command
		cmdPayload, _ := json.Marshal(executeCommandPayload{
			RequestID: "req-1",
			Command:   "echo hi",
		})
		_ = conn.WriteJSON(wsMessage{Type: msgTypeExecuteCmd, Payload: cmdPayload})

		// 5. Send a read_file
		readPayload, _ := json.Marshal(executeCommandPayload{
			RequestID: "req-2",
			Command:   "cat /etc/hostname",
		})
		_ = conn.WriteJSON(wsMessage{Type: msgTypeReadFile, Payload: readPayload})

		// 6. Send unknown message type
		_ = conn.WriteJSON(wsMessage{Type: "unknown_type"})

		// 7. Send invalid JSON for execute_command
		_ = conn.WriteJSON(wsMessage{Type: msgTypeExecuteCmd, Payload: json.RawMessage(`{invalid}`)})

		// 8. Send invalid JSON for read_file
		_ = conn.WriteJSON(wsMessage{Type: msgTypeReadFile, Payload: json.RawMessage(`{invalid}`)})
	}))
	defer server.Close()

	logger := zerolog.Nop()
	cfg := Config{
		PulseURL: "http" + strings.TrimPrefix(server.URL, "http"),
		Logger:   &logger,
	}

	client := NewCommandClient(cfg, "agent", "host", "linux", "1.0")

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

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
	// We can't wait for the ticker easily; this is just a cancellation/exit-path coverage check.
	go client.pingLoop(ctx, conn, done)
	close(done)
}

func TestCommandClient_RegistrationFailure(t *testing.T) {
	origDelay := reconnectDelay
	reconnectDelay = 0
	t.Cleanup(func() { reconnectDelay = origDelay })

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()

		// Receive registration
		var msg wsMessage
		_ = conn.ReadJSON(&msg)

		// Send registered FAILURE
		payload, _ := json.Marshal(registeredPayload{Success: false, Message: "registration rejected"})
		_ = conn.WriteJSON(wsMessage{Type: msgTypeRegistered, Payload: payload})
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

func TestCommandClient_RunStopsAfterClose(t *testing.T) {
	origDelay := reconnectDelay
	reconnectDelay = 5 * time.Millisecond
	t.Cleanup(func() { reconnectDelay = origDelay })

	client := &CommandClient{
		pulseURL: "http://127.0.0.1:1",
		logger:   zerolog.Nop(),
		done:     make(chan struct{}),
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.Run(context.Background())
	}()

	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run() error = %v, want nil after Close()", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not stop after Close()")
	}
}
