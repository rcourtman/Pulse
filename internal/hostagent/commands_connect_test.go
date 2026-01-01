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

func TestCommandClient_connectAndHandle_ExecutesCommandAndReturnsResult(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping websocket integration test in short mode")
	}

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

	serverDone := make(chan struct{})
	gotResult := make(chan commandResultPayload, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()

		conn.SetReadDeadline(time.Now().Add(5 * time.Second))

		var regMsg wsMessage
		if err := conn.ReadJSON(&regMsg); err != nil {
			t.Errorf("read registration: %v", err)
			return
		}
		if regMsg.Type != msgTypeAgentRegister {
			t.Errorf("registration type = %q, want %q", regMsg.Type, msgTypeAgentRegister)
			return
		}

		conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		registeredBytes, _ := json.Marshal(registeredPayload{Success: true, Message: "Registered"})
		if err := conn.WriteJSON(wsMessage{Type: msgTypeRegistered, Timestamp: time.Now(), Payload: registeredBytes}); err != nil {
			t.Errorf("write registered: %v", err)
			return
		}

		execPayloadBytes, _ := json.Marshal(executeCommandPayload{
			RequestID:  "req-1",
			Command:    "echo hello",
			TargetType: "host",
			Timeout:    5,
		})
		if err := conn.WriteJSON(wsMessage{Type: msgTypeExecuteCmd, ID: "req-1", Timestamp: time.Now(), Payload: execPayloadBytes}); err != nil {
			t.Errorf("write execute_command: %v", err)
			return
		}

		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		var resultMsg wsMessage
		if err := conn.ReadJSON(&resultMsg); err != nil {
			t.Errorf("read command_result: %v", err)
			return
		}
		if resultMsg.Type != msgTypeCommandResult {
			t.Errorf("result type = %q, want %q", resultMsg.Type, msgTypeCommandResult)
			return
		}

		var result commandResultPayload
		if err := json.Unmarshal(resultMsg.Payload, &result); err != nil {
			t.Errorf("unmarshal command_result payload: %v", err)
			return
		}
		gotResult <- result

		<-serverDone
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := &CommandClient{
		pulseURL: strings.TrimRight(server.URL, "/"),
		apiToken: "token",
		agentID:  "agent-1",
		hostname: "host-1",
		platform: "linux",
		version:  "1.2.3",
		logger:   zerolog.Nop(),
		done:     make(chan struct{}),
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.connectAndHandle(ctx)
	}()

	select {
	case result := <-gotResult:
		if result.RequestID != "req-1" {
			t.Fatalf("result.RequestID = %q, want %q", result.RequestID, "req-1")
		}
		if !result.Success || result.ExitCode != 0 {
			t.Fatalf("unexpected result: %#v", result)
		}
		if !strings.Contains(result.Stdout, "hello") {
			t.Fatalf("stdout = %q, expected to contain %q", result.Stdout, "hello")
		}

		cancel()
		close(serverDone)
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for command result")
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatalf("expected error due to context cancellation")
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for connectAndHandle to return")
	}
}
