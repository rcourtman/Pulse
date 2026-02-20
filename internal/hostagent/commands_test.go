package hostagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

func TestCommandClient_Run(t *testing.T) {
	// Setup mock WebSocket server
	upgrader := websocket.Upgrader{}

	// Channels to verify interaction
	registerReceived := make(chan bool)
	commandSent := make(chan bool)
	resultReceived := make(chan bool)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		// 1. Expect Registration
		var msg wsMessage
		if err := conn.ReadJSON(&msg); err != nil {
			t.Errorf("read registration failed: %v", err)
			return
		}
		if msg.Type != msgTypeAgentRegister {
			t.Errorf("expected register msg, got %s", msg.Type)
			return
		}
		registerReceived <- true

		// 2. Send Registration Success
		respPayload, _ := json.Marshal(registeredPayload{Success: true})
		_ = conn.WriteJSON(wsMessage{
			Type:      msgTypeRegistered,
			Timestamp: time.Now(),
			Payload:   respPayload,
		})

		// 3. Send Execute Command
		cmdPayload, _ := json.Marshal(executeCommandPayload{
			RequestID: "cmd-1",
			Command:   "echo hello",
			Timeout:   5,
		})
		_ = conn.WriteJSON(wsMessage{
			Type:      msgTypeExecuteCmd,
			Timestamp: time.Now(),
			Payload:   cmdPayload,
		})
		commandSent <- true

		// 4. Expect Result
		if err := conn.ReadJSON(&msg); err != nil {
			t.Errorf("read result failed: %v", err)
			return
		}
		if msg.Type != msgTypeCommandResult {
			t.Errorf("expected result msg, got %s", msg.Type)
			return
		}
		resultReceived <- true
	}))
	defer server.Close()

	// Mock execCommandContext
	origExec := execCommandContext
	defer func() { execCommandContext = origExec }()

	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Just run real echo for simplicity, or mock using test helper
		// echo is safe and standard
		return exec.CommandContext(ctx, name, args...)
	}

	logger := zerolog.Nop()
	cfg := Config{
		PulseURL: server.URL,
		APIToken: "test-token",
		Logger:   &logger,
	}
	client := NewCommandClient(cfg, "agent-1", "host-1", "linux", "v1")

	// Run client in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = client.Run(ctx)
	}()

	// Verify sequence
	select {
	case <-registerReceived:
		t.Log("Registration received")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for registration")
	}

	select {
	case <-commandSent:
		t.Log("Command sent")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for command send")
	}

	select {
	case <-resultReceived:
		t.Log("Result received")
	case <-time.After(5 * time.Second): // Allow execution time
		t.Fatal("Timeout waiting for result")
	}

	// Terminate
	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestWrapCommand(t *testing.T) {
	tests := []struct {
		name    string
		payload executeCommandPayload
		wantCmd string
		checkFn func(string) bool
	}{
		{
			name: "HostCommandPassedThrough",
			payload: executeCommandPayload{
				Command:    "ls -la",
				TargetType: "host",
				TargetID:   "",
			},
			wantCmd: "ls -la",
		},
		{
			name: "LXCCommandWrappedInShC",
			payload: executeCommandPayload{
				Command:    "grep pattern /var/log/*.log",
				TargetType: "container",
				TargetID:   "141",
			},
			checkFn: func(cmd string) bool {
				// Should be: pct exec 141 -- sh -c 'grep pattern /var/log/*.log'
				return strings.HasPrefix(cmd, "pct exec 141 -- sh -c '") &&
					strings.Contains(cmd, "grep pattern /var/log/*.log")
			},
		},
		{
			name: "VMCommandWrappedInShC",
			payload: executeCommandPayload{
				Command:    "cat /etc/hostname",
				TargetType: "vm",
				TargetID:   "100",
			},
			checkFn: func(cmd string) bool {
				// Should be: qm guest exec 100 -- sh -c 'cat /etc/hostname'
				return strings.HasPrefix(cmd, "qm guest exec 100 -- sh -c '") &&
					strings.Contains(cmd, "cat /etc/hostname")
			},
		},
		{
			name: "LXCCommandWithSingleQuotes",
			payload: executeCommandPayload{
				Command:    "echo \"it's working\"",
				TargetType: "container",
				TargetID:   "141",
			},
			checkFn: func(cmd string) bool {
				// Single quotes should be escaped: it's -> it'"'"'s
				return strings.HasPrefix(cmd, "pct exec 141 -- sh -c '") &&
					strings.Contains(cmd, `it'"'"'s`)
			},
		},
		{
			name: "LXCCommandWithPipeline",
			payload: executeCommandPayload{
				Command:    "echo 'test' | base64 -d > /tmp/file",
				TargetType: "container",
				TargetID:   "108",
			},
			checkFn: func(cmd string) bool {
				// Pipeline should be wrapped so it runs inside LXC
				return strings.HasPrefix(cmd, "pct exec 108 -- sh -c '") &&
					strings.Contains(cmd, "| base64 -d > /tmp/file")
			},
		},
		{
			name: "InvalidTargetIDReturnsError",
			payload: executeCommandPayload{
				Command:    "ls",
				TargetType: "container",
				TargetID:   "141; rm -rf /", // injection attempt
			},
			checkFn: func(cmd string) bool {
				return strings.Contains(cmd, "invalid target ID")
			},
		},
		{
			name: "EmptyTargetIDReturnsError",
			payload: executeCommandPayload{
				Command:    "ls",
				TargetType: "container",
				TargetID:   "",
			},
			checkFn: func(cmd string) bool {
				return strings.Contains(cmd, "missing target ID")
			},
		},
		{
			name: "OptionLikeTargetIDReturnsError",
			payload: executeCommandPayload{
				Command:    "ls",
				TargetType: "vm",
				TargetID:   "-1",
			},
			checkFn: func(cmd string) bool {
				return strings.Contains(cmd, "invalid target ID")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapCommand(tt.payload)
			if tt.wantCmd != "" {
				if got != tt.wantCmd {
					t.Errorf("wrapCommand() = %q, want %q", got, tt.wantCmd)
				}
			}
			if tt.checkFn != nil {
				if !tt.checkFn(got) {
					t.Errorf("wrapCommand() = %q, check failed", got)
				}
			}
		})
	}
}
