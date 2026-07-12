package agentexec

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
)

func TestStrictAPTCodecsRejectUnknownTrailingAndOpenAuthorityFields(t *testing.T) {
	hash := "sha256:" + strings.Repeat("a", 64)
	base := fmt.Sprintf(`{"request_id":"r1","action_id":"a1","operation":"install_os_updates","expected_inventory_hash":"%s","timeout":30}`, hash)
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "unknown command", body: strings.TrimSuffix(base, "}") + `,"command":"apt-get upgrade"}`},
		{name: "package selector", body: strings.TrimSuffix(base, "}") + `,"packages":["x"]}`},
		{name: "path selector", body: strings.TrimSuffix(base, "}") + `,"path":"/tmp"}`},
		{name: "removal authority", body: strings.TrimSuffix(base, "}") + `,"allow_remove":true}`},
		{name: "reboot authority", body: strings.TrimSuffix(base, "}") + `,"reboot":true}`},
		{name: "trailing json", body: base + `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeHostUpdatePayload([]byte(tc.body)); err == nil {
				t.Fatal("strict host update codec accepted open authority")
			}
		})
	}
}

func TestMalformedOrCrossTypeAPTResultCannotPoisonPendingUpdate(t *testing.T) {
	hash := "sha256:" + strings.Repeat("a", 64)
	empty := "sha256:" + strings.Repeat("b", 64)
	s := NewServer(allowAllTestTokens)
	ts := newWSServer(t, s)
	defer ts.Close()
	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{AgentID: "apt-agent", Hostname: "host", Version: "6", Platform: "linux", Token: "any", OperationReceiptVersion: operationreceipt.ProtocolVersion}))
	_ = wsReadRegisteredPayload(t, conn)

	done := make(chan error, 1)
	go func() {
		_, err := s.ExecuteHostUpdate(context.Background(), "apt-agent", HostUpdatePayload{RequestID: "attempt-1", ActionID: "action-1", Operation: HostUpdateOperationInstall, ExpectedInventoryHash: hash, Timeout: 2})
		done <- err
	}()
	msg, err := wsReadRawMessageWithTimeout(conn, time.Second)
	if err != nil || msg.Type != MsgTypeHostUpdate {
		t.Fatalf("dispatch msg=%+v err=%v", msg, err)
	}

	malformed := json.RawMessage(`{"request_id":"attempt-1","action_id":"action-1","execution_phase":"complete","verification":"verified","unknown":true}`)
	if err := conn.WriteJSON(wsMessageForTest(MsgTypeHostUpdateResult, "attempt-1", malformed)); err != nil {
		t.Fatal(err)
	}
	crossType := HostStorageCleanupResultPayload{RequestID: "attempt-1", ActionID: "action-1", ExecutionPhase: HostStorageCleanupPhaseClean, MutationStarted: true, Verification: HostStorageCleanupVerificationInconclusive, Error: "unknown"}
	if err := conn.WriteJSON(mustNewMessage(t, MsgTypeHostStorageCleanupResult, "attempt-1", crossType)); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		t.Fatalf("malformed/cross-type result terminated pending update: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	now := time.Now().UTC()
	valid := HostUpdateResultPayload{RequestID: "attempt-1", ActionID: "action-1", ExecutionPhase: HostUpdatePhaseComplete, Success: true, Verification: HostUpdateVerificationVerified,
		Before: HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: hash, PendingCount: 1, CheckedAt: now},
		After:  HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: empty, PendingCount: 0, CheckedAt: now}}
	if err := conn.WriteJSON(mustNewMessage(t, MsgTypeHostUpdateResult, "attempt-1", valid)); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("valid correlated result failed: %v", err)
	}
}

func TestCrossTypeSameRequestCollisionRefusesSecondMutation(t *testing.T) {
	hash := "sha256:" + strings.Repeat("a", 64)
	s := NewServer(allowAllTestTokens)
	ts := newWSServer(t, s)
	defer ts.Close()
	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{AgentID: "apt-agent", Hostname: "host", Version: "6", Platform: "linux", Token: "any", OperationReceiptVersion: operationreceipt.ProtocolVersion}))
	_ = wsReadRegisteredPayload(t, conn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updateDone := make(chan error, 1)
	go func() {
		_, err := s.ExecuteHostUpdate(ctx, "apt-agent", HostUpdatePayload{RequestID: "same-attempt", ActionID: "action-1", Operation: HostUpdateOperationInstall, ExpectedInventoryHash: hash, Timeout: 30})
		updateDone <- err
	}()
	if msg, err := wsReadRawMessageWithTimeout(conn, time.Second); err != nil || msg.Type != MsgTypeHostUpdate {
		t.Fatalf("first dispatch msg=%+v err=%v", msg, err)
	}
	_, cleanupErr := s.ExecuteHostStorageCleanup(context.Background(), "apt-agent", HostStorageCleanupPayload{RequestID: "same-attempt", ActionID: "action-1", Operation: HostStorageCleanupOperationPackageCache, ExpectedFingerprint: hash, Timeout: 1})
	if cleanupErr == nil || !strings.Contains(cleanupErr.Error(), "already pending") {
		t.Fatalf("cross-type collision error=%v", cleanupErr)
	}
	if _, err := wsReadRawMessageWithTimeout(conn, 100*time.Millisecond); err == nil {
		t.Fatal("cross-type collision dispatched a second mutation")
	}
	cancel()
	if err := <-updateDone; err == nil {
		t.Fatal("canceled first mutation unexpectedly succeeded")
	}
}

func TestHostUpdateContradictionRejectsWrongRequestBoundBeforeHash(t *testing.T) {
	now := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	want := "sha256:" + strings.Repeat("a", 64)
	wrong := "sha256:" + strings.Repeat("b", 64)
	req := HostUpdatePayload{RequestID: "attempt-1", ActionID: "action-1", Operation: HostUpdateOperationInstall, ExpectedInventoryHash: want}
	result := HostUpdateResultPayload{RequestID: req.RequestID, ActionID: req.ActionID, ExecutionPhase: HostUpdatePhaseVerify, MutationStarted: true, Verification: HostUpdateVerificationFailed,
		Before: HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: wrong, PendingCount: 1, CheckedAt: now.Add(-time.Minute)},
		After:  HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: wrong, PendingCount: 1, CheckedAt: now}}
	if err := ValidateHostUpdateResultForRequestAt(req, result, now); err == nil || !strings.Contains(err.Error(), "before-state") {
		t.Fatalf("wrong before hash error=%v", err)
	}
}

func TestHostStorageCleanupContradictionRejectsWrongRequestBoundBeforeFingerprint(t *testing.T) {
	now := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	want := "sha256:" + strings.Repeat("a", 64)
	wrong := "sha256:" + strings.Repeat("b", 64)
	req := HostStorageCleanupPayload{RequestID: "attempt-1", ActionID: "action-1", Operation: HostStorageCleanupOperationPackageCache, ExpectedFingerprint: want}
	result := HostStorageCleanupResultPayload{RequestID: req.RequestID, ActionID: req.ActionID, ExecutionPhase: HostStorageCleanupPhaseVerify, MutationStarted: true, Verification: HostStorageCleanupVerificationFailed,
		Before: HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: wrong, ReclaimableBytes: 100, CheckedAt: now.Add(-time.Minute)},
		After:  HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: wrong, ReclaimableBytes: 100, CheckedAt: now}}
	if err := ValidateHostStorageCleanupResultForRequestAt(req, result, now); err == nil || !strings.Contains(err.Error(), "before-state") {
		t.Fatalf("wrong before fingerprint error=%v", err)
	}
}

func TestAPTResultReceiptTimeValidationUsesControlledClock(t *testing.T) {
	receivedAt := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	hash := "sha256:" + strings.Repeat("a", 64)
	req := HostUpdatePayload{RequestID: "attempt-1", ActionID: "action-1", Operation: HostUpdateOperationInstall, ExpectedInventoryHash: hash}
	result := HostUpdateResultPayload{RequestID: req.RequestID, ActionID: req.ActionID, ExecutionPhase: HostUpdatePhaseVerify, MutationStarted: true, Verification: HostUpdateVerificationFailed,
		Before: HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: hash, PendingCount: 1, CheckedAt: receivedAt.Add(-time.Hour - time.Minute)},
		After:  HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: hash, PendingCount: 1, CheckedAt: receivedAt.Add(-time.Hour)}}
	if err := ValidateHostUpdateResultForRequestAt(req, result, receivedAt); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale result error=%v", err)
	}
}

func wsMessageForTest(messageType MessageType, id string, payload json.RawMessage) Message {
	return Message{Type: messageType, ID: id, Timestamp: time.Now().UTC(), Payload: payload}
}
