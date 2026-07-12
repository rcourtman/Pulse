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
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rs/zerolog"
)

func TestCommandClientHandlesTypedHostStorageCleanup(t *testing.T) {
	before := agentexec.HostStorageCleanupSnapshot{Fingerprint: "sha256:" + strings.Repeat("a", 64), ReclaimableBytes: 512}
	after := agentexec.HostStorageCleanupSnapshot{Fingerprint: "sha256:" + strings.Repeat("b", 64), ReclaimableBytes: 12}
	snapshots := []agentexec.HostStorageCleanupSnapshot{before, after}
	manager := newStorageCleanupManager("linux", newPackageManagerLease())
	manager.lookPath = func(string) (string, error) { return "/usr/bin/apt-get", nil }
	manager.scan = func() (agentexec.HostStorageCleanupSnapshot, error) {
		snapshot := snapshots[0]
		snapshots = snapshots[1:]
		return snapshot, nil
	}
	manager.run = func(_ context.Context, _ []string, name string, args ...string) packageUpdateCommandResult {
		if name != "apt-get" || strings.Join(args, " ") != "clean" {
			t.Fatalf("unexpected cleanup command: %s %v", name, args)
		}
		return packageUpdateCommandResult{}
	}

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	resultCh := make(chan agentexec.HostStorageCleanupResultPayload, 1)
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
		payload, _ := json.Marshal(agentexec.HostStorageCleanupPayload{
			RequestID: "cleanup-1", ActionID: "action-1", Operation: agentexec.HostStorageCleanupOperationPackageCache, ExpectedFingerprint: before.Fingerprint, Timeout: 5,
		})
		if err := conn.WriteJSON(wsMessage{Type: msgTypeHostStorageCleanup, ID: "cleanup-1", Timestamp: time.Now(), Payload: payload}); err != nil {
			t.Errorf("write host storage cleanup: %v", err)
			return
		}
		var response wsMessage
		if err := conn.ReadJSON(&response); err != nil {
			t.Errorf("read host storage cleanup result: %v", err)
			return
		}
		if response.Type != msgTypeHostStorageCleanupResult {
			t.Errorf("response type = %q", response.Type)
			return
		}
		var result agentexec.HostStorageCleanupResultPayload
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
		platform: "linux", version: "6.0.6", storageCleanup: manager, logger: zerolog.Nop(), done: make(chan struct{}),
	}
	errCh := make(chan error, 1)
	go func() { errCh <- client.connectAndHandle(ctx) }()

	select {
	case result := <-resultCh:
		if !result.Success || result.Verification != agentexec.HostStorageCleanupVerificationVerified || result.ReclaimedBytes != 500 {
			t.Fatalf("result = %#v", result)
		}
		cancel()
	case <-time.After(10 * time.Second):
		cancel()
		t.Fatal("timed out waiting for typed host storage cleanup result")
	}
	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Fatal("command client did not stop")
	}
}
