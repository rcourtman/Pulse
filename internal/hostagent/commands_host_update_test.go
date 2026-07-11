package hostagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rs/zerolog"
)

func TestCommandClientHandlesTypedHostUpdateWithoutExecuteCommand(t *testing.T) {
	manager := newPackageUpdateManager("linux")
	manager.lookPath = func(string) (string, error) { return "/usr/bin/apt-get", nil }
	manager.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	simulations := 0
	manager.run = func(_ context.Context, _ []string, name string, args ...string) packageUpdateCommandResult {
		if name != "apt-get" {
			t.Fatalf("executable = %q, want apt-get", name)
		}
		if strings.Contains(strings.Join(args, " "), "-s") {
			simulations++
			if simulations <= 2 {
				return packageUpdateCommandResult{stdout: "Inst openssl [1.0] (1.1 repo [amd64])\n"}
			}
		}
		return packageUpdateCommandResult{}
	}
	expectedInventoryHash := aptUpgradeInventoryHash("Inst openssl [1.0] (1.1 repo [amd64])\n")

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	resultCh := make(chan agentexec.HostUpdateResultPayload, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		var registration wsMessage
		if err := conn.ReadJSON(&registration); err != nil {
			t.Errorf("read registration: %v", err)
			return
		}
		registered, _ := json.Marshal(registeredPayload{Success: true, Message: "Registered"})
		if err := conn.WriteJSON(wsMessage{Type: msgTypeRegistered, Timestamp: time.Now(), Payload: registered}); err != nil {
			t.Errorf("write registered: %v", err)
			return
		}
		payload, _ := json.Marshal(agentexec.HostUpdatePayload{
			RequestID: "request-1", ActionID: "action-1", Operation: agentexec.HostUpdateOperationInstall, ExpectedInventoryHash: expectedInventoryHash, Timeout: 5,
		})
		if err := conn.WriteJSON(wsMessage{Type: msgTypeHostUpdate, ID: "request-1", Timestamp: time.Now(), Payload: payload}); err != nil {
			t.Errorf("write host update: %v", err)
			return
		}
		var response wsMessage
		if err := conn.ReadJSON(&response); err != nil {
			t.Errorf("read host update result: %v", err)
			return
		}
		if response.Type != msgTypeHostUpdateResult {
			t.Errorf("response type = %q", response.Type)
			return
		}
		var result agentexec.HostUpdateResultPayload
		if err := json.Unmarshal(response.Payload, &result); err != nil {
			t.Errorf("decode result: %v", err)
			return
		}
		resultCh <- result
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	client := &CommandClient{
		pulseURL: strings.TrimRight(server.URL, "/"), apiToken: "token", agentID: "agent-1", hostname: "host-1",
		platform: "linux", version: "6.0.6", packageUpdates: manager, logger: zerolog.Nop(), done: make(chan struct{}),
	}
	errCh := make(chan error, 1)
	go func() { errCh <- client.connectAndHandle(ctx) }()

	select {
	case result := <-resultCh:
		if !result.Success || result.Verification != agentexec.HostUpdateVerificationVerified || result.Before.PendingCount != 1 || result.After.PendingCount != 0 {
			t.Fatalf("result = %#v", result)
		}
		cancel()
	case <-time.After(10 * time.Second):
		cancel()
		t.Fatal("timed out waiting for typed host update result")
	}
	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Fatal("command client did not stop")
	}
}
