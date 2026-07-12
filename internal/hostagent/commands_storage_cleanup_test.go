package hostagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
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
		request := agentexec.HostStorageCleanupPayload{
			RequestID: "cleanup-1", ActionID: "action-1", Operation: agentexec.HostStorageCleanupOperationPackageCache, ExpectedFingerprint: before.Fingerprint, Timeout: 5,
		}
		if err := agentexec.BindHostStorageCleanupPayload(&request); err != nil {
			t.Errorf("bind request: %v", err)
			return
		}
		payload, _ := json.Marshal(request)
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
	receipts, err := operationreceipt.Open(filepath.Join(t.TempDir(), "receipts.db"), hostOperationReceiptConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer receipts.Close()
	client := &CommandClient{
		pulseURL: strings.TrimRight(server.URL, "/"), apiToken: "token", agentID: "agent-1", hostname: "host-1",
		platform: "linux", version: "6.0.6", storageCleanup: manager, operationReceipts: receipts, logger: zerolog.Nop(), done: make(chan struct{}),
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

func TestHostStorageCleanupFingerprintDriftReceiptCompletesAndReplaysWithAdmittedIdentity(t *testing.T) {
	receipts, err := operationreceipt.Open(filepath.Join(t.TempDir(), "receipts.db"), hostOperationReceiptConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer receipts.Close()

	req := agentexec.HostStorageCleanupPayload{RequestID: "drift.dispatch.1", ActionID: "drift", Operation: agentexec.HostStorageCleanupOperationPackageCache, ExpectedFingerprint: "sha256:" + strings.Repeat("a", 64)}
	if err := agentexec.BindHostStorageCleanupPayload(&req); err != nil {
		t.Fatal(err)
	}
	identity := agentexec.HostStorageCleanupOperationIdentity("agent-1", req)
	if _, admitted, err := receipts.Admit(identity); err != nil || !admitted {
		t.Fatalf("admit=%v err=%v", admitted, err)
	}
	if _, err := receipts.MarkStarted(identity); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	result := agentexec.HostStorageCleanupResultPayload{
		RequestID: req.RequestID, ActionID: req.ActionID, ExecutionPhase: agentexec.HostStorageCleanupPhasePreflight, Verification: agentexec.HostStorageCleanupVerificationInconclusive,
		Before: agentexec.HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: "sha256:" + strings.Repeat("b", 64), ReclaimableBytes: 10, CheckedAt: now.Add(-time.Second)},
		After:  agentexec.HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: "sha256:" + strings.Repeat("b", 64), ReclaimableBytes: 10, CheckedAt: now},
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	envelope := operationreceipt.TerminalEnvelope{Kind: agentexec.HostStorageCleanupReceiptKind, Version: agentexec.HostAPTReceiptVersion, Payload: raw}
	record, err := receipts.Complete(identity, envelope)
	if err != nil || record.State != operationreceipt.StateTerminal {
		t.Fatalf("complete record=%+v err=%v", record, err)
	}
	if replay, admitted, err := receipts.Admit(identity); err != nil || admitted || replay.State != operationreceipt.StateTerminal || string(replay.Result) != string(raw) {
		t.Fatalf("replay record=%+v admitted=%v err=%v", replay, admitted, err)
	}
	wrongDigest := identity
	wrongDigest.RequestDigest = "sha256:" + strings.Repeat("c", 64)
	if _, err := receipts.Complete(wrongDigest, envelope); err == nil {
		t.Fatal("wrong admitted request digest accepted")
	}
}
