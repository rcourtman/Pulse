package agentexec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

type wsRawMessage struct {
	Type      MessageType      `json:"type"`
	ID        string           `json:"id,omitempty"`
	Timestamp time.Time        `json:"timestamp"`
	Payload   *json.RawMessage `json:"payload,omitempty"`
}

func TestOperationQueryInconclusiveAPTDriftPreservesAdmittedDigest(t *testing.T) {
	now := time.Now().UTC()
	updateReq := HostUpdatePayload{RequestID: "u-drift.dispatch.1", ActionID: "u-drift", Operation: HostUpdateOperationInstall, ExpectedInventoryHash: "sha256:" + strings.Repeat("a", 64)}
	if err := BindHostUpdatePayload(&updateReq); err != nil {
		t.Fatal(err)
	}
	updateIdentity := HostUpdateOperationIdentity("agent", updateReq)
	updateResult := HostUpdateResultPayload{
		RequestID: updateReq.RequestID, ActionID: updateReq.ActionID, ExecutionPhase: HostUpdatePhaseRefresh, Verification: HostUpdateVerificationInconclusive,
		Before: HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: "sha256:" + strings.Repeat("b", 64), PendingCount: 2, CheckedAt: now.Add(-time.Second)},
		After:  HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: "sha256:" + strings.Repeat("b", 64), PendingCount: 2, CheckedAt: now},
	}
	updateRaw, err := json.Marshal(updateResult)
	if err != nil {
		t.Fatal(err)
	}

	cleanupReq := HostStorageCleanupPayload{RequestID: "c-drift.dispatch.1", ActionID: "c-drift", Operation: HostStorageCleanupOperationPackageCache, ExpectedFingerprint: "sha256:" + strings.Repeat("c", 64)}
	if err := BindHostStorageCleanupPayload(&cleanupReq); err != nil {
		t.Fatal(err)
	}
	cleanupIdentity := HostStorageCleanupOperationIdentity("agent", cleanupReq)
	cleanupResult := HostStorageCleanupResultPayload{
		RequestID: cleanupReq.RequestID, ActionID: cleanupReq.ActionID, ExecutionPhase: HostStorageCleanupPhasePreflight, Verification: HostStorageCleanupVerificationInconclusive,
		Before: HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: "sha256:" + strings.Repeat("d", 64), ReclaimableBytes: 10, CheckedAt: now.Add(-time.Second)},
		After:  HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: "sha256:" + strings.Repeat("d", 64), ReclaimableBytes: 10, CheckedAt: now},
	}
	cleanupRaw, err := json.Marshal(cleanupResult)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name     string
		identity operationreceipt.Identity
		kind     string
		payload  json.RawMessage
	}{
		{name: "update inventory drift", identity: updateIdentity, kind: HostUpdateReceiptKind, payload: updateRaw},
		{name: "cleanup fingerprint drift", identity: cleanupIdentity, kind: HostStorageCleanupReceiptKind, payload: cleanupRaw},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := operationreceipt.Record{Identity: tc.identity, State: operationreceipt.StateTerminal, AcceptedAt: now.Add(-3 * time.Second), StartedAt: now.Add(-2 * time.Second), TerminalAt: now.Add(time.Second), ResultKind: tc.kind, ResultVersion: HostAPTReceiptVersion, Result: tc.payload}
			query := operationreceipt.QueryResult{Version: operationreceipt.ProtocolVersion, Status: operationreceipt.QueryFoundTerminal, Record: &record}
			if err := ValidateOperationQueryResultForIdentity(query, tc.identity, now.Add(2*time.Second)); err != nil {
				t.Fatalf("bound inconclusive drift receipt rejected: %v", err)
			}
			tampered := tc.identity
			tampered.RequestDigest = "sha256:" + strings.Repeat("e", 64)
			if err := ValidateOperationQueryResultForIdentity(query, tampered, now.Add(2*time.Second)); err == nil {
				t.Fatal("wrong request digest accepted")
			}
		})
	}
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

func wsHeadersForHTTP(t *testing.T, serverURL string) http.Header {
	t.Helper()

	origin, err := securityutil.HTTPOriginForWebSocketBaseURL(serverURL)
	if err != nil {
		t.Fatalf("failed to derive websocket origin: %v", err)
	}

	headers := http.Header{}
	headers.Set("Origin", origin)
	return headers
}

func dialAgentExecWebSocket(t *testing.T, serverURL string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	return websocket.DefaultDialer.Dial(wsURLForHTTP(serverURL), wsHeadersForHTTP(t, serverURL))
}

func wsWriteMessage(t *testing.T, conn *websocket.Conn, msg Message) {
	t.Helper()
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
}

func mustNewMessage(t *testing.T, msgType MessageType, id string, payload any) Message {
	t.Helper()
	msg, err := NewMessage(msgType, id, payload)
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	return msg
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
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
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
	s := NewServer(func(token string, agentID string, hostname string) bool { return token == "ok" })
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID:  "a1",
		Hostname: "host1",
		Version:  "1.2.3",
		Platform: "linux",
		Tags:     []string{"tag1"},
		Token:    "ok",
	}))

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

func TestActionRunnerRegistrationRequiresCredentialBoundRuntimeRoleAndCapability(t *testing.T) {
	admission := AgentAdmission{
		OrganizationID:   "org-a",
		TokenID:          "runner-token",
		AgentID:          "machine-a",
		Hostname:         "node.example",
		RuntimeRole:      RuntimeRoleActionRunner,
		ActionCapability: ActionCapabilityTypedV1,
	}
	s := NewServerWithAdmissionValidator(func(token, _, _ string) (AgentAdmission, bool) {
		return admission, token == admission.TokenID
	}, func(candidate AgentAdmission) bool { return candidate == admission })
	ts := newWSServer(t, s)
	defer ts.Close()

	register := func(role, capability string) (*websocket.Conn, RegisteredPayload) {
		t.Helper()
		conn, _, err := dialAgentExecWebSocket(t, ts.URL)
		if err != nil {
			t.Fatalf("Dial: %v", err)
		}
		wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
			AgentID: admission.AgentID, Hostname: admission.Hostname, Token: admission.TokenID,
			RuntimeRole: role, ActionCapability: capability,
		}))
		return conn, wsReadRegisteredPayload(t, conn)
	}

	for _, invalid := range []struct{ role, capability string }{
		{"", ""},
		{RuntimeRoleActionRunner, ""},
		{RuntimeRoleActionRunner, "shell.v1"},
		{RuntimeRoleLegacyFullTrust, ActionCapabilityTypedV1},
	} {
		conn, ack := register(invalid.role, invalid.capability)
		conn.Close()
		if ack.Success {
			t.Fatalf("unbound runner assertion admitted: role=%q capability=%q", invalid.role, invalid.capability)
		}
	}

	conn, ack := register(RuntimeRoleActionRunner, ActionCapabilityTypedV1)
	defer conn.Close()
	if !ack.Success {
		t.Fatalf("bound action runner registration rejected: %s", ack.Message)
	}
	connected := s.GetConnectedAgentsForOrganization("org-a")
	if len(connected) != 1 || connected[0].RuntimeRole != RuntimeRoleActionRunner || connected[0].ActionCapability != ActionCapabilityTypedV1 {
		t.Fatalf("connected action runner = %#v", connected)
	}

	ctx := WithOrganizationID(context.Background(), "org-a")
	if _, err := s.ExecuteCommand(ctx, admission.AgentID, ExecuteCommandPayload{RequestID: "shell", Command: "true", TargetType: "agent", Trusted: true}); err == nil || !strings.Contains(err.Error(), "typed action-runner") {
		t.Fatalf("action runner accepted arbitrary command: %v", err)
	}
	if _, err := s.ReadFile(ctx, admission.AgentID, ReadFilePayload{RequestID: "read", Path: "/etc/hosts", TargetType: "agent"}); err == nil || !strings.Contains(err.Error(), "typed action-runner") {
		t.Fatalf("action runner accepted unrestricted read: %v", err)
	}
	if err := s.SendDeployCancel(ctx, admission.AgentID, DeployCancelPayload{RequestID: "deploy", JobID: "job"}); err == nil || !strings.Contains(err.Error(), "typed action-runner") {
		t.Fatalf("action runner accepted deploy protocol: %v", err)
	}
}

func TestPreparedActionRunnerReconnectCannotDisplaceActiveDispatchAndExactPromotionSwaps(t *testing.T) {
	active := AgentAdmission{
		OrganizationID: "org-a", TokenID: "active-token", AgentID: "machine-a", Hostname: "node.example",
		RuntimeRole: RuntimeRoleActionRunner, ActionCapability: ActionCapabilityTypedV1,
	}
	pendingOne := active
	pendingOne.TokenID = "pending-token-1"
	pendingOne.ActivationPending = true
	pendingTwo := pendingOne
	pendingTwo.TokenID = "pending-token-2"
	admissions := map[string]AgentAdmission{
		active.TokenID:     active,
		pendingOne.TokenID: pendingOne,
		pendingTwo.TokenID: pendingTwo,
	}
	s := NewServerWithAdmissionValidator(func(token, _, _ string) (AgentAdmission, bool) {
		admission, ok := admissions[token]
		return admission, ok
	}, func(AgentAdmission) bool { return true })
	ts := newWSServer(t, s)
	defer ts.Close()
	register := func(admission AgentAdmission) *websocket.Conn {
		t.Helper()
		conn, _, err := dialAgentExecWebSocket(t, ts.URL)
		if err != nil {
			t.Fatal(err)
		}
		wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
			AgentID: admission.AgentID, Hostname: admission.Hostname, Token: admission.TokenID,
			RuntimeRole: admission.RuntimeRole, ActionCapability: admission.ActionCapability,
		}))
		if ack := wsReadRegisteredPayload(t, conn); !ack.Success {
			conn.Close()
			t.Fatalf("registration for %s failed: %s", admission.TokenID, ack.Message)
		}
		return conn
	}
	requirePong := func(conn *websocket.Conn, label string) {
		t.Helper()
		wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentPing, "", nil))
		if msg := wsReadRawMessage(t, conn); msg.Type != MsgTypePong {
			t.Fatalf("%s message type = %q, want pong", label, msg.Type)
		}
	}

	activeConn := register(active)
	defer activeConn.Close()
	pendingOneConn := register(pendingOne)
	defer pendingOneConn.Close()

	if current, ok := s.connectionForOrganization(active.OrganizationID, active.AgentID); !ok || current.admission.TokenID != active.TokenID {
		t.Fatalf("pending registration displaced active dispatch: %#v, %v", current, ok)
	}
	requirePong(activeConn, "active runner while replacement pending")
	if !s.HasActionRunnerSession(pendingOne) {
		t.Fatal("first exact pending transport was not staged")
	}

	pendingTwoConn := register(pendingTwo)
	defer pendingTwoConn.Close()
	if _, err := wsReadRawMessageWithTimeout(pendingOneConn, 2*time.Second); err == nil {
		t.Fatal("replaced pending transport remained connected")
	}
	if current, ok := s.connectionForOrganization(active.OrganizationID, active.AgentID); !ok || current.admission.TokenID != active.TokenID {
		t.Fatalf("pending reconnect displaced active dispatch: %#v, %v", current, ok)
	}
	requirePong(activeConn, "active runner after pending reconnect")
	if s.HasActionRunnerSession(pendingOne) {
		t.Fatal("superseded pending transport remained promotable")
	}
	if !s.HasActionRunnerSession(pendingTwo) {
		t.Fatal("latest exact pending transport was not staged")
	}
	if cleanup, promoted := s.PromoteActionRunnerSessionForCommit(pendingOne); promoted || cleanup != nil {
		t.Fatal("stale Has/promote snapshot displaced the current transport")
	}
	cleanup, promoted := s.PromoteActionRunnerSessionForCommit(pendingTwo)
	if !promoted {
		t.Fatal("latest exact pending transport was not promoted")
	}
	if current, ok := s.connectionForOrganization(active.OrganizationID, active.AgentID); !ok || current.admission.TokenID != pendingTwo.TokenID {
		t.Fatalf("promotion did not atomically swap dispatch: %#v, %v", current, ok)
	}
	if cleanup == nil {
		t.Fatal("promotion did not return displaced active cleanup")
	}
	cleanup()
	if _, err := wsReadRawMessageWithTimeout(activeConn, 2*time.Second); err == nil {
		t.Fatal("displaced active transport remained connected after deferred cleanup")
	}
	requirePong(pendingTwoConn, "promoted runner")

	s.mu.RLock()
	activeCount := len(s.agents)
	pendingCount := len(s.pendingActionRunners)
	s.mu.RUnlock()
	if activeCount != 1 || pendingCount != 0 {
		t.Fatalf("session maps after promotion = active %d pending %d", activeCount, pendingCount)
	}
}

func TestActionRunnerInboundResultsRequireExactActiveSessionAcrossPromotion(t *testing.T) {
	active := AgentAdmission{
		OrganizationID: "org-a", TokenID: "active-token", AgentID: "machine-a", Hostname: "node.example",
		RuntimeRole: RuntimeRoleActionRunner, ActionCapability: ActionCapabilityTypedV1,
	}
	pending := active
	pending.TokenID = "pending-token"
	pending.ActivationPending = true
	admissions := map[string]AgentAdmission{active.TokenID: active, pending.TokenID: pending}
	s := NewServerWithAdmissionValidator(func(token, _, _ string) (AgentAdmission, bool) {
		admission, ok := admissions[token]
		return admission, ok
	}, func(AgentAdmission) bool { return true })
	ts := newWSServer(t, s)
	defer ts.Close()

	register := func(admission AgentAdmission) *websocket.Conn {
		t.Helper()
		conn, _, err := dialAgentExecWebSocket(t, ts.URL)
		if err != nil {
			t.Fatal(err)
		}
		wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
			AgentID: admission.AgentID, Hostname: admission.Hostname, Token: admission.TokenID,
			RuntimeRole: admission.RuntimeRole, ActionCapability: admission.ActionCapability,
			OperationReceiptVersion: operationreceipt.ProtocolVersion,
		}))
		if ack := wsReadRegisteredPayload(t, conn); !ack.Success {
			conn.Close()
			t.Fatalf("registration for %s failed: %s", admission.TokenID, ack.Message)
		}
		return conn
	}
	activeConn := register(active)
	defer activeConn.Close()
	pendingConn := register(pending)
	defer pendingConn.Close()

	type updateOutcome struct {
		result *HostUpdateResultPayload
		err    error
	}
	dispatch := func(requestID, actionID, inventoryHash string) <-chan updateOutcome {
		t.Helper()
		out := make(chan updateOutcome, 1)
		go func() {
			ctx, cancel := context.WithTimeout(WithOrganizationID(context.Background(), active.OrganizationID), 5*time.Second)
			defer cancel()
			result, err := s.ExecuteHostUpdate(ctx, active.AgentID, HostUpdatePayload{
				RequestID: requestID, ActionID: actionID, Operation: HostUpdateOperationInstall,
				ExpectedInventoryHash: inventoryHash, Timeout: 5,
			})
			out <- updateOutcome{result: result, err: err}
		}()
		return out
	}
	readRequest := func(conn *websocket.Conn) HostUpdatePayload {
		t.Helper()
		msg, err := wsReadRawMessageWithTimeout(conn, 2*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if msg.Type != MsgTypeHostUpdate || msg.Payload == nil {
			t.Fatalf("message = %#v, want typed host update", msg)
		}
		var req HostUpdatePayload
		if err := json.Unmarshal(*msg.Payload, &req); err != nil {
			t.Fatal(err)
		}
		return req
	}
	resultFor := func(req HostUpdatePayload, afterHash string) HostUpdateResultPayload {
		now := time.Now().UTC()
		return HostUpdateResultPayload{
			RequestID: req.RequestID, ActionID: req.ActionID, Success: true,
			ExecutionPhase: HostUpdatePhaseComplete,
			Before:         HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: req.ExpectedInventoryHash, PendingCount: 1, CheckedAt: now.Add(-time.Second)},
			After:          HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: afterHash, PendingCount: 0, CheckedAt: now},
			HealthChecked:  true, PackageManagerHealthy: true, Verification: HostUpdateVerificationVerified,
		}
	}
	requirePong := func(conn *websocket.Conn, label string) {
		t.Helper()
		wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentPing, "", nil))
		msg, err := wsReadRawMessageWithTimeout(conn, 2*time.Second)
		if err != nil {
			t.Fatalf("%s ping barrier: %v", label, err)
		}
		if msg.Type != MsgTypePong {
			t.Fatalf("%s ping barrier received %q, want pong", label, msg.Type)
		}
	}
	assertStillPending := func(out <-chan updateOutcome, label string) {
		t.Helper()
		select {
		case got := <-out:
			t.Fatalf("%s satisfied dispatch: result=%#v err=%v", label, got.result, got.err)
		case <-time.After(100 * time.Millisecond):
		}
	}
	assertSuccess := func(out <-chan updateOutcome, label, expectedAfterHash string) {
		t.Helper()
		select {
		case got := <-out:
			if got.err != nil || got.result == nil || !got.result.Success || got.result.After.InventoryHash != expectedAfterHash {
				t.Fatalf("%s result=%#v err=%v", label, got.result, got.err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%s did not complete dispatch", label)
		}
	}
	assertDisconnected := func(out <-chan updateOutcome, label string) {
		t.Helper()
		select {
		case got := <-out:
			if got.result != nil || got.err == nil || !strings.Contains(got.err.Error(), "disconnected") {
				t.Fatalf("%s result=%#v err=%v, want predecessor disconnect", label, got.result, got.err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%s did not terminate after predecessor cleanup", label)
		}
	}

	before := dispatch("before-promotion", "action-before", "sha256:"+strings.Repeat("a", 64))
	beforeReq := readRequest(activeConn)
	pendingForgery := resultFor(beforeReq, "sha256:"+strings.Repeat("e", 64))
	wsWriteMessage(t, pendingConn, mustNewMessage(t, MsgTypeHostUpdateResult, beforeReq.RequestID, pendingForgery))
	requirePong(pendingConn, "prepared replacement after forged result")
	assertStillPending(before, "prepared replacement")

	failedSaveTx, begun := s.BeginActionRunnerSessionPromotion(pending)
	if !begun {
		t.Fatal("failed to begin promotion fence")
	}
	fencedForgery := resultFor(beforeReq, "sha256:"+strings.Repeat("f", 64))
	wsWriteMessage(t, activeConn, mustNewMessage(t, MsgTypeHostUpdateResult, beforeReq.RequestID, fencedForgery))
	requirePong(activeConn, "predecessor during promotion fence")
	assertStillPending(before, "fenced predecessor")
	failedSaveTx.Rollback()

	beforeResult := resultFor(beforeReq, "sha256:"+strings.Repeat("b", 64))
	wsWriteMessage(t, activeConn, mustNewMessage(t, MsgTypeHostUpdateResult, beforeReq.RequestID, beforeResult))
	assertSuccess(before, "active predecessor after rollback", beforeResult.After.InventoryHash)

	across := dispatch("across-promotion", "action-across", "sha256:"+strings.Repeat("7", 64))
	acrossReq := readRequest(activeConn)
	promotion, begun := s.BeginActionRunnerSessionPromotion(pending)
	if !begun || !promotion.Commit() {
		t.Fatal("prepared replacement was not promoted")
	}
	transferredForgery := resultFor(acrossReq, "sha256:"+strings.Repeat("8", 64))
	wsWriteMessage(t, pendingConn, mustNewMessage(t, MsgTypeHostUpdateResult, acrossReq.RequestID, transferredForgery))
	requirePong(pendingConn, "promoted replacement after predecessor-request forgery")
	assertStillPending(across, "promoted replacement for predecessor request")
	promotion.Cleanup()
	assertDisconnected(across, "predecessor request across promotion")

	after := dispatch("after-promotion", "action-after", "sha256:"+strings.Repeat("c", 64))
	afterReq := readRequest(pendingConn)
	afterResult := resultFor(afterReq, "sha256:"+strings.Repeat("d", 64))
	wsWriteMessage(t, pendingConn, mustNewMessage(t, MsgTypeHostUpdateResult, afterReq.RequestID, afterResult))
	assertSuccess(after, "promoted replacement", afterResult.After.InventoryHash)
}

func TestActionRunnerPromotionVersusDisconnectNeverRetainsDeadSession(t *testing.T) {
	for attempt := 0; attempt < 20; attempt++ {
		admission := AgentAdmission{
			OrganizationID: "org-a", TokenID: fmt.Sprintf("pending-%d", attempt), AgentID: "machine-a", Hostname: "node.example",
			RuntimeRole: RuntimeRoleActionRunner, ActionCapability: ActionCapabilityTypedV1, ActivationPending: true,
		}
		s := NewServerWithAdmissionValidator(func(token, _, _ string) (AgentAdmission, bool) {
			return admission, token == admission.TokenID
		}, func(AgentAdmission) bool { return true })
		ts := newWSServer(t, s)
		conn, _, err := dialAgentExecWebSocket(t, ts.URL)
		if err != nil {
			ts.Close()
			t.Fatal(err)
		}
		wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
			AgentID: admission.AgentID, Hostname: admission.Hostname, Token: admission.TokenID,
			RuntimeRole: admission.RuntimeRole, ActionCapability: admission.ActionCapability,
		}))
		if ack := wsReadRegisteredPayload(t, conn); !ack.Success {
			conn.Close()
			ts.Close()
			t.Fatalf("attempt %d registration failed: %s", attempt, ack.Message)
		}
		start := make(chan struct{})
		promotionDone := make(chan func(), 1)
		go func() {
			<-start
			cleanup, _ := s.PromoteActionRunnerSessionForCommit(admission)
			promotionDone <- cleanup
		}()
		close(start)
		_ = conn.Close()
		cleanup := <-promotionDone
		if cleanup != nil {
			cleanup()
		}
		waitFor(t, 2*time.Second, func() bool {
			s.mu.RLock()
			defer s.mu.RUnlock()
			key := agentSessionKey(admission.OrganizationID, admission.AgentID)
			return s.agents[key] == nil && s.pendingActionRunners[key] == nil
		})
		ts.Close()
	}
}

func TestStaleActionRunnerPromotionCannotFailCloseLaterSessions(t *testing.T) {
	admission := AgentAdmission{
		OrganizationID: "org-a", TokenID: "pending-token", AgentID: "machine-a", Hostname: "node.example",
		RuntimeRole: RuntimeRoleActionRunner, ActionCapability: ActionCapabilityTypedV1, ActivationPending: true,
	}
	s := NewServerWithAdmissionValidator(func(string, string, string) (AgentAdmission, bool) {
		return AgentAdmission{}, false
	}, nil)
	key := agentSessionKey(admission.OrganizationID, admission.AgentID)
	predecessor := &agentConn{agent: ConnectedAgent{AgentID: admission.AgentID}, sessionKey: key, authorityKey: "predecessor", done: make(chan struct{})}
	pending := &agentConn{agent: ConnectedAgent{AgentID: admission.AgentID}, admission: admission, sessionKey: key, authorityKey: "pending", done: make(chan struct{})}
	s.agents[key] = predecessor
	s.pendingActionRunners[key] = pending

	tx, begun := s.BeginActionRunnerSessionPromotion(admission)
	if !begun {
		t.Fatal("failed to begin promotion")
	}
	tx.Rollback()

	laterActive := &agentConn{agent: ConnectedAgent{AgentID: admission.AgentID}, sessionKey: key, authorityKey: "later-active", done: make(chan struct{})}
	laterPending := &agentConn{agent: ConnectedAgent{AgentID: admission.AgentID}, admission: admission, sessionKey: key, authorityKey: "later-pending", done: make(chan struct{})}
	s.mu.Lock()
	s.agents[key] = laterActive
	s.pendingActionRunners[key] = laterPending
	s.mu.Unlock()

	tx.FailClosed()
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.agents[key] != laterActive || s.pendingActionRunners[key] != laterPending {
		t.Fatal("stale promotion transaction removed later session occupants")
	}
}

func TestLegacyCredentialCannotAssertActionRunnerRole(t *testing.T) {
	s := NewServerWithAdmissionValidator(func(token, _, _ string) (AgentAdmission, bool) {
		return AgentAdmission{TokenID: token, AgentID: "a1", Hostname: "host1", RuntimeRole: RuntimeRoleLegacyFullTrust}, token == "legacy"
	}, nil)
	ts := newWSServer(t, s)
	defer ts.Close()
	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID: "a1", Hostname: "host1", Token: "legacy", RuntimeRole: RuntimeRoleActionRunner, ActionCapability: ActionCapabilityTypedV1,
	}))
	if ack := wsReadRegisteredPayload(t, conn); ack.Success {
		t.Fatal("legacy credential self-promoted into action-runner role")
	}
}

func TestHandleWebSocket_RejectsMissingOrigin(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, resp, err := websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL), nil)
	if err == nil {
		conn.Close()
		t.Fatalf("expected websocket upgrade to reject missing Origin")
	}
	if resp == nil {
		t.Fatalf("expected HTTP response for rejected websocket upgrade")
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestHandleWebSocket_RejectsPerIPConnectionFlood(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	s.maxConnsPerIP = 1
	ts := newWSServer(t, s)
	defer ts.Close()

	firstConn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatalf("Dial first connection: %v", err)
	}
	defer firstConn.Close()

	wsWriteMessage(t, firstConn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID:  "a1",
		Hostname: "host1",
		Version:  "1.2.3",
		Platform: "linux",
		Token:    "any",
	}))
	reg := wsReadRegisteredPayload(t, firstConn)
	if !reg.Success {
		t.Fatalf("first registration failed: %q", reg.Message)
	}

	secondConn, resp, err := dialAgentExecWebSocket(t, ts.URL)
	if err == nil {
		secondConn.Close()
		t.Fatalf("expected second websocket upgrade to be rejected")
	}
	if resp == nil {
		t.Fatalf("expected HTTP response for rejected websocket upgrade")
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected %d, got %d", http.StatusTooManyRequests, resp.StatusCode)
	}

	firstConn.Close()
	waitFor(t, 2*time.Second, func() bool { return !s.IsAgentConnected("a1") })

	thirdConn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatalf("Dial third connection after release: %v", err)
	}
	defer thirdConn.Close()

	wsWriteMessage(t, thirdConn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID:  "a2",
		Hostname: "host2",
		Version:  "1.2.3",
		Platform: "linux",
		Token:    "any",
	}))
	reg = wsReadRegisteredPayload(t, thirdConn)
	if !reg.Success {
		t.Fatalf("third registration failed after slot release: %q", reg.Message)
	}
}

func TestHandleWebSocket_RegistrationFiresAgentRegisteredNotifier(t *testing.T) {
	s := NewServer(func(token string, agentID string, hostname string) bool { return token == "ok" })
	notified := make(chan string, 2)
	s.SetAgentRegisteredNotifier(func(agentID string) { notified <- agentID })
	ts := newWSServer(t, s)
	defer ts.Close()

	rejectedConn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer rejectedConn.Close()
	wsWriteMessage(t, rejectedConn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID: "a-rejected", Hostname: "host-rejected", Token: "bad",
	}))
	if reg := wsReadRegisteredPayload(t, rejectedConn); reg.Success {
		t.Fatalf("expected registration to be rejected")
	}

	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()
	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID: "a1", Hostname: "host1", Token: "ok",
	}))
	if reg := wsReadRegisteredPayload(t, conn); !reg.Success {
		t.Fatalf("registration failed: %q", reg.Message)
	}

	select {
	case agentID := <-notified:
		if agentID != "a1" {
			t.Fatalf("notified agent = %q, want a1 (rejected registration must not notify)", agentID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("agent-registered notifier did not fire")
	}
	select {
	case agentID := <-notified:
		t.Fatalf("unexpected second notification for agent %q", agentID)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestHandleWebSocket_InvalidTokenRejected(t *testing.T) {
	s := NewServer(func(string, string, string) bool { return false })
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID:  "a1",
		Hostname: "host1",
		Version:  "1.2.3",
		Platform: "linux",
		Token:    "bad",
	}))

	reg := wsReadRegisteredPayload(t, conn)
	if reg.Success {
		t.Fatalf("expected registration to be rejected")
	}

	waitFor(t, 2*time.Second, func() bool { return !s.IsAgentConnected("a1") })

	_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatalf("expected connection to be closed by server")
	}
}

func TestHandleWebSocket_MissingAgentIDRejected(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID:  "   ",
		Hostname: "host1",
		Version:  "1.2.3",
		Platform: "linux",
		Token:    "any",
	}))

	reg := wsReadRegisteredPayload(t, conn)
	if reg.Success {
		t.Fatalf("expected registration to be rejected")
	}

	_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatalf("expected connection to be closed by server")
	}
}

func TestHandleWebSocket_FirstMessageMustBeRegister(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentPing, "", nil))

	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatalf("expected server to close connection")
	}
}

func TestHandleWebSocket_RejectsOversizedRegistrationMessage(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	oversized := bytes.Repeat([]byte("x"), int(maxWebSocketMessageBytes)+1)
	if err := conn.WriteMessage(websocket.TextMessage, oversized); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatalf("expected server to close connection for oversized registration message")
	}
}

func TestHandleWebSocket_AgentPingRespondsWithPong(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID:  "a1",
		Hostname: "host1",
		Version:  "1.2.3",
		Platform: "linux",
		Token:    "any",
	}))
	_ = wsReadRegisteredPayload(t, conn)

	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentPing, "", nil))

	msg := wsReadRawMessage(t, conn)
	if msg.Type != MsgTypePong {
		t.Fatalf("message type = %q, want %q", msg.Type, MsgTypePong)
	}
}

func TestExecuteCommand_RoundTripViaWebSocket(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	callerGrant := &CommandApprovalGrant{Signature: "caller-supplied"}
	s.SetCommandAuthorizationVerifier(func(req CommandAuthorizationRequest) error {
		if req.ApprovalID != "approval-1" || req.OrgID != "org-1" || req.ActionID != "action-1" {
			return fmt.Errorf("authorization mismatch: %+v", req)
		}
		return nil
	})
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID:  "a1",
		Hostname: "host1",
		Version:  "1.2.3",
		Platform: "linux",
		Token:    "any",
	}))
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
			if payload.ApprovalGrant == nil {
				agentErr <- fmt.Errorf("missing approval grant")
				return
			}
			if payload.ApprovalGrant.Signature == callerGrant.Signature {
				agentErr <- fmt.Errorf("caller-supplied approval grant was forwarded")
				return
			}
			if err := VerifyCommandApprovalGrant("any", "a1", payload, time.Now()); err != nil {
				agentErr <- err
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			if err := conn.WriteJSON(mustNewMessage(t, MsgTypeCommandResult, "", CommandResultPayload{
				RequestID: payload.RequestID,
				Success:   true,
				Stdout:    "ok",
				ExitCode:  0,
				Duration:  1,
			})); err != nil {
				agentErr <- err
				return
			}
			agentErr <- nil
			return
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	payload := ExecuteCommandPayload{
		RequestID:     "req1",
		Command:       "echo ok",
		ApprovalID:    "approval-1",
		ApprovalGrant: callerGrant,
		Timeout:       1,
	}
	payload.BindCommandAuthorization("org-1", "action-1")
	result, err := s.ExecuteCommand(ctx, "a1", payload)
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

func TestExecuteCommand_InvalidApprovalAuthorizationNeverMintsOrDispatches(t *testing.T) {
	cases := []struct {
		name string
		err  string
	}{
		{name: "nonexistent", err: "approval not found"},
		{name: "wrong-org", err: "approval belongs to another org"},
		{name: "expired", err: "approval expired"},
		{name: "consumed", err: "approval already consumed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewServer(allowAllTestTokens)
			s.SetCommandAuthorizationVerifier(func(CommandAuthorizationRequest) error { return errors.New(tc.err) })
			grantCalls := 0
			s.newCommandApprovalGrant = func([]byte, string, ExecuteCommandPayload, time.Time, time.Duration) (*CommandApprovalGrant, error) {
				grantCalls++
				return nil, errors.New("grant must not be minted")
			}
			ts := newWSServer(t, s)
			defer ts.Close()

			conn, _, err := dialAgentExecWebSocket(t, ts.URL)
			if err != nil {
				t.Fatalf("Dial: %v", err)
			}
			defer conn.Close()
			wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
				AgentID: "a1", Hostname: "host1", Version: "1.2.3", Platform: "linux", Token: "any",
			}))
			_ = wsReadRegisteredPayload(t, conn)

			payload := ExecuteCommandPayload{
				RequestID: "req-invalid", Command: "echo rejected", ApprovalID: "approval-invalid", Timeout: 1,
			}
			payload.BindCommandAuthorization("org-1", "action-1")
			if _, err := s.ExecuteCommand(context.Background(), "a1", payload); err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("ExecuteCommand error = %v, want %q", err, tc.err)
			}
			if grantCalls != 0 {
				t.Fatalf("signed grant calls = %d, want 0", grantCalls)
			}
			if _, err := wsReadRawMessageWithTimeout(conn, 100*time.Millisecond); err == nil {
				t.Fatal("unexpected WebSocket dispatch for rejected approval")
			}
		})
	}
}

func TestExecuteHostUpdateRoundTripUsesTypedCommandFreeEnvelope(t *testing.T) {
	inventoryHash := "sha256:" + strings.Repeat("a", 64)
	emptyInventoryHash := "sha256:" + strings.Repeat("b", 64)
	s := NewServer(allowAllTestTokens)
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID: "host-agent-1", Hostname: "host1", Version: "6.0.6", Platform: "linux", Token: "any", OperationReceiptVersion: 1,
	}))
	_ = wsReadRegisteredPayload(t, conn)

	agentErr := make(chan error, 1)
	go func() {
		msg, err := wsReadRawMessageWithTimeout(conn, 2*time.Second)
		if err != nil {
			agentErr <- err
			return
		}
		if msg.Type != MsgTypeHostUpdate || msg.Payload == nil {
			agentErr <- fmt.Errorf("message = %#v, want typed host update", msg)
			return
		}
		if bytes.Contains(*msg.Payload, []byte(`"command"`)) || bytes.Contains(*msg.Payload, []byte(`"packages"`)) {
			agentErr <- fmt.Errorf("host update request exposed command or package authority: %s", string(*msg.Payload))
			return
		}
		var payload HostUpdatePayload
		if err := json.Unmarshal(*msg.Payload, &payload); err != nil {
			agentErr <- err
			return
		}
		if payload.ActionID != "action-1" || payload.Operation != HostUpdateOperationInstall {
			agentErr <- fmt.Errorf("payload = %#v", payload)
			return
		}
		response := HostUpdateResultPayload{
			RequestID:      payload.RequestID,
			ActionID:       payload.ActionID,
			Success:        true,
			ExecutionPhase: HostUpdatePhaseComplete,
			Before:         HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: inventoryHash, PendingCount: 2, CheckedAt: time.Now().UTC()},
			After:          HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: emptyInventoryHash, PendingCount: 0, RebootRequired: true, CheckedAt: time.Now().UTC()},
			HealthChecked:  true, PackageManagerHealthy: true, Verification: HostUpdateVerificationVerified,
		}
		if err := conn.WriteJSON(mustNewMessage(t, MsgTypeHostUpdateResult, payload.RequestID, response)); err != nil {
			agentErr <- err
			return
		}
		agentErr <- nil
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := s.ExecuteHostUpdate(ctx, "host-agent-1", HostUpdatePayload{
		RequestID: "request-1", ActionID: "action-1", Operation: HostUpdateOperationInstall, ExpectedInventoryHash: inventoryHash, Timeout: 1,
	})
	if err != nil {
		t.Fatalf("ExecuteHostUpdate: %v", err)
	}
	if result == nil || !result.Success || result.Verification != HostUpdateVerificationVerified || result.After.PendingCount != 0 || !result.After.RebootRequired {
		t.Fatalf("result = %#v", result)
	}
	if err := <-agentErr; err != nil {
		t.Fatalf("agent: %v", err)
	}
}

func TestActionPreflightRoundTripIsReadOnlyAndDigestBound(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	ts := newWSServer(t, s)
	defer ts.Close()
	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID: "preflight-agent", Hostname: "host1", Version: "6", Platform: "linux", Token: "any",
		OperationReceiptVersion: operationreceipt.ProtocolVersion, ActionPreflightVersion: ActionPreflightProtocolVersion,
	}))
	_ = wsReadRegisteredPayload(t, conn)
	req := boundHostUpdatePreflight(t)
	agentErr := make(chan error, 1)
	go func() {
		msg, readErr := wsReadRawMessageWithTimeout(conn, 2*time.Second)
		if readErr != nil {
			agentErr <- readErr
			return
		}
		if msg.Type != MsgTypeActionPreflight || msg.Payload == nil || bytes.Contains(*msg.Payload, []byte(`"command"`)) {
			agentErr <- fmt.Errorf("unexpected preflight envelope: %#v", msg)
			return
		}
		payload, decodeErr := DecodeActionPreflightPayload(*msg.Payload)
		if decodeErr != nil {
			agentErr <- decodeErr
			return
		}
		operation, version, digest := ActionPreflightBinding(payload)
		result := ActionPreflightResultPayload{
			RequestID: payload.RequestID, ProtocolVersion: payload.ProtocolVersion,
			Operation: operation, OperationVersion: version, RequestDigest: digest,
			ReasonCode: ActionRefusalPackageManagerUnhealthy, CheckedAt: time.Now().UTC(),
		}
		agentErr <- conn.WriteJSON(mustNewMessage(t, MsgTypeActionPreflightResult, payload.RequestID, result))
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := s.PreflightAction(ctx, "preflight-agent", req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Feasible || result.ReasonCode != ActionRefusalPackageManagerUnhealthy {
		t.Fatalf("result=%#v", result)
	}
	if err := <-agentErr; err != nil {
		t.Fatal(err)
	}
}

func TestDockerContainerObservationRoundTripIsReadOnlyAndActionBound(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	ts := newWSServer(t, s)
	defer ts.Close()
	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID: "docker-observer", Hostname: "host1", Version: "6", Platform: "linux", Token: "any",
		DockerObservationVersion: DockerContainerObservationProtocolVersion,
	}))
	_ = wsReadRegisteredPayload(t, conn)
	containerID := strings.Repeat("a", 64)
	agentErr := make(chan error, 1)
	go func() {
		msg, readErr := wsReadRawMessageWithTimeout(conn, 2*time.Second)
		if readErr != nil {
			agentErr <- readErr
			return
		}
		if msg.Type != MsgTypeDockerContainerObserve || bytes.Contains(*msg.Payload, []byte(`"command"`)) || bytes.Contains(*msg.Payload, []byte(`"operation"`)) {
			agentErr <- fmt.Errorf("unexpected docker observation envelope: %#v", msg)
			return
		}
		payload, decodeErr := DecodeDockerContainerObservationPayload(*msg.Payload)
		if decodeErr != nil {
			agentErr <- decodeErr
			return
		}
		result := DockerContainerObservationResultPayload{
			RequestID: payload.RequestID, ActionID: payload.ActionID, ProtocolVersion: payload.ProtocolVersion, RequestDigest: payload.RequestDigest,
			Observed: true,
			Snapshot: DockerContainerObservationSnapshot{ContainerID: payload.ContainerID, State: "running", Running: true, Health: DockerContainerHealthHealthy, ObservedAt: time.Now().UTC()},
		}
		agentErr <- conn.WriteJSON(mustNewMessage(t, MsgTypeDockerContainerObserveResult, payload.RequestID, result))
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := s.ObserveDockerContainer(ctx, "docker-observer", DockerContainerObservationPayload{ActionID: "action-1", Runtime: "docker", ContainerID: containerID})
	if err != nil {
		t.Fatal(err)
	}
	if result.ActionID != "action-1" || result.Snapshot.ContainerID != containerID || !result.Snapshot.Running {
		t.Fatalf("result=%#v", result)
	}
	if err := <-agentErr; err != nil {
		t.Fatal(err)
	}
}

func TestDockerContainerObservationRejectsUnsupportedAgentBeforeDispatch(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	ts := newWSServer(t, s)
	defer ts.Close()
	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID: "older-agent", Hostname: "host1", Version: "6.3", Platform: "linux", Token: "any",
		DockerObservationVersion: 1,
	}))
	_ = wsReadRegisteredPayload(t, conn)
	started := time.Now()
	_, err = s.ObserveDockerContainer(context.Background(), "older-agent", DockerContainerObservationPayload{
		ActionID: "action-1", Runtime: "docker", ContainerID: strings.Repeat("a", 64),
	})
	if err == nil || !strings.Contains(err.Error(), "does not support docker observation protocol") {
		t.Fatalf("error=%v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatalf("unsupported agent rejection was not immediate: %s", time.Since(started))
	}
}

func TestValidateHostUpdatePayloadRejectsOpenEndedAuthority(t *testing.T) {
	for _, req := range []HostUpdatePayload{
		{RequestID: "r1", ActionID: "a1", Operation: "run_command", ExpectedInventoryHash: "sha256:" + strings.Repeat("a", 64)},
		{RequestID: "r1", Operation: HostUpdateOperationInstall, ExpectedInventoryHash: "sha256:" + strings.Repeat("a", 64)},
		{RequestID: "r1", ActionID: "a1", Operation: HostUpdateOperationInstall, Timeout: 1801, ExpectedInventoryHash: "sha256:" + strings.Repeat("a", 64)},
		{RequestID: "r1", ActionID: "a1", Operation: HostUpdateOperationInstall},
	} {
		copy := req
		if err := validateHostUpdatePayload(&copy); err == nil {
			t.Fatalf("validateHostUpdatePayload(%#v) succeeded", req)
		}
	}
}

func TestValidateHostUpdateResultRejectsUnprovenVerifiedClaim(t *testing.T) {
	result := HostUpdateResultPayload{
		RequestID: "r1", Success: true, Verification: HostUpdateVerificationVerified,
		After: HostPackageUpdateSnapshot{
			Supported: true, Manager: "apt", InventoryHash: "sha256:" + strings.Repeat("a", 64), PendingCount: 1,
		},
	}
	if err := validateHostUpdateResultPayload(&result); err == nil {
		t.Fatal("verified result with pending packages must fail closed")
	}
}

func TestExecuteHostStorageCleanupRoundTripUsesPathAndCommandFreeEnvelope(t *testing.T) {
	fingerprint := "sha256:" + strings.Repeat("a", 64)
	afterFingerprint := "sha256:" + strings.Repeat("b", 64)
	s := NewServer(allowAllTestTokens)
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()
	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID: "host-agent-cleanup", Hostname: "host1", Version: "6.0.6", Platform: "linux", Token: "any", OperationReceiptVersion: 1,
	}))
	_ = wsReadRegisteredPayload(t, conn)

	agentErr := make(chan error, 1)
	go func() {
		msg, err := wsReadRawMessageWithTimeout(conn, 2*time.Second)
		if err != nil {
			agentErr <- err
			return
		}
		if msg.Type != MsgTypeHostStorageCleanup || msg.Payload == nil {
			agentErr <- fmt.Errorf("message = %#v, want typed host storage cleanup", msg)
			return
		}
		for _, forbidden := range []string{`"command"`, `"path"`, `"packages"`} {
			if bytes.Contains(*msg.Payload, []byte(forbidden)) {
				agentErr <- fmt.Errorf("storage cleanup request exposed forbidden authority %s: %s", forbidden, string(*msg.Payload))
				return
			}
		}
		var payload HostStorageCleanupPayload
		if err := json.Unmarshal(*msg.Payload, &payload); err != nil {
			agentErr <- err
			return
		}
		if payload.ActionID != "action-cleanup" || payload.Operation != HostStorageCleanupOperationPackageCache {
			agentErr <- fmt.Errorf("payload = %#v", payload)
			return
		}
		response := HostStorageCleanupResultPayload{
			RequestID:      payload.RequestID,
			ActionID:       payload.ActionID,
			ExecutionPhase: HostStorageCleanupPhaseComplete,
			Success:        true,
			Before:         HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: fingerprint, ReclaimableBytes: 500, CheckedAt: time.Now().UTC()},
			After:          HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: afterFingerprint, ReclaimableBytes: 20, CheckedAt: time.Now().UTC()},
			ReclaimedBytes: 480,
			Verification:   HostStorageCleanupVerificationVerified,
		}
		if err := conn.WriteJSON(mustNewMessage(t, MsgTypeHostStorageCleanupResult, payload.RequestID, response)); err != nil {
			agentErr <- err
			return
		}
		agentErr <- nil
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := s.ExecuteHostStorageCleanup(ctx, "host-agent-cleanup", HostStorageCleanupPayload{
		RequestID: "cleanup-1", ActionID: "action-cleanup", Operation: HostStorageCleanupOperationPackageCache, ExpectedFingerprint: fingerprint, Timeout: 1,
	})
	if err != nil {
		t.Fatalf("ExecuteHostStorageCleanup: %v", err)
	}
	if result == nil || !result.Success || result.Verification != HostStorageCleanupVerificationVerified || result.ReclaimedBytes != 480 {
		t.Fatalf("result = %#v", result)
	}
	if err := <-agentErr; err != nil {
		t.Fatalf("agent: %v", err)
	}
}

func TestValidateHostStorageCleanupRejectsOpenEndedOrUnprovenClaims(t *testing.T) {
	fingerprint := "sha256:" + strings.Repeat("a", 64)
	for _, req := range []HostStorageCleanupPayload{
		{RequestID: "r1", ActionID: "a1", Operation: "delete_path", ExpectedFingerprint: fingerprint},
		{RequestID: "r1", Operation: HostStorageCleanupOperationPackageCache, ExpectedFingerprint: fingerprint},
		{RequestID: "r1", ActionID: "a1", Operation: HostStorageCleanupOperationPackageCache, ExpectedFingerprint: "bad"},
		{RequestID: "r1", ActionID: "a1", Operation: HostStorageCleanupOperationPackageCache, ExpectedFingerprint: fingerprint, Timeout: 901},
	} {
		copy := req
		if err := validateHostStorageCleanupPayload(&copy); err == nil {
			t.Fatalf("validateHostStorageCleanupPayload(%#v) succeeded", req)
		}
	}
	result := HostStorageCleanupResultPayload{
		RequestID: "r1", Success: true, Verification: HostStorageCleanupVerificationVerified,
		Before: HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: fingerprint, ReclaimableBytes: 500},
		After:  HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: fingerprint, ReclaimableBytes: 500},
	}
	if err := validateHostStorageCleanupResultPayload(&result); err == nil {
		t.Fatal("verified result without reclaimed bytes must fail closed")
	}
}

func TestHandleWebSocket_ReconnectSameAgentIDClosesOldConnection(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	ts := newWSServer(t, s)
	defer ts.Close()

	dial := func() *websocket.Conn {
		t.Helper()
		conn, _, err := dialAgentExecWebSocket(t, ts.URL)
		if err != nil {
			t.Fatalf("Dial: %v", err)
		}
		return conn
	}

	c1 := dial()
	defer c1.Close()
	wsWriteMessage(t, c1, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID:  "a1",
		Hostname: "host1",
		Version:  "1.2.3",
		Platform: "linux",
		Token:    "any",
	}))
	_ = wsReadRegisteredPayload(t, c1)

	progressCh := s.SubscribeDeployProgress("a1", "job-reconnect", 1)
	defer s.UnsubscribeDeployProgress("a1", "job-reconnect")
	commandDone := make(chan error, 1)
	go func() {
		_, err := s.ExecuteCommand(context.Background(), "a1", ExecuteCommandPayload{
			RequestID: "command-before-reconnect",
			Command:   "true",
			Timeout:   10,
			Trusted:   true,
		})
		commandDone <- err
	}()
	if command := wsReadRawMessage(t, c1); command.Type != MsgTypeExecuteCmd {
		t.Fatalf("old session received %q, want %q", command.Type, MsgTypeExecuteCmd)
	}

	c2 := dial()
	defer c2.Close()
	wsWriteMessage(t, c2, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID:  "a1",
		Hostname: "host1",
		Version:  "1.2.3",
		Platform: "linux",
		Token:    "any",
	}))
	_ = wsReadRegisteredPayload(t, c2)

	_ = c1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, _, err := c1.ReadMessage()
	if err == nil {
		t.Fatalf("expected old connection to be closed")
	}

	select {
	case commandErr := <-commandDone:
		if commandErr == nil || !strings.Contains(commandErr.Error(), "disconnected") {
			t.Fatalf("in-flight command reconnect result = %v, want disconnected", commandErr)
		}
	case <-time.After(time.Second):
		t.Fatal("in-flight command did not stop when its session was replaced")
	}

	progress := DeployProgressPayload{
		RequestID: "deploy-after-reconnect",
		JobID:     "job-reconnect",
		Phase:     DeployPhasePreflightSSH,
		Status:    DeployStepOK,
	}
	wsWriteMessage(t, c2, mustNewMessage(t, MsgTypeDeployProgress, progress.RequestID, progress))
	select {
	case received, ok := <-progressCh:
		if !ok {
			t.Fatal("replacement cleanup closed the active deploy subscription")
		}
		if received.RequestID != progress.RequestID {
			t.Fatalf("deploy progress request id = %q, want %q", received.RequestID, progress.RequestID)
		}
	case <-time.After(time.Second):
		t.Fatal("replacement session did not retain deploy progress subscription")
	}
}

func TestCommandSessionsAreTenantScopedAndDuplicateIdentityFailsClosed(t *testing.T) {
	admissions := map[string]AgentAdmission{
		"token-a": {OrganizationID: "org-a", TokenID: "token-a", AgentID: "shared", Hostname: "host-a"},
		"token-b": {OrganizationID: "org-b", TokenID: "token-b", AgentID: "shared", Hostname: "host-b"},
		"token-c": {OrganizationID: "org-a", TokenID: "token-c", AgentID: "shared", Hostname: "other-host"},
		"token-d": {OrganizationID: "org-a", TokenID: "token-d", AgentID: "other-id", Hostname: "host-a"},
	}
	s := NewServerWithAdmissionValidator(func(token, _, _ string) (AgentAdmission, bool) {
		admission, ok := admissions[token]
		return admission, ok
	}, func(AgentAdmission) bool { return true })
	ts := newWSServer(t, s)
	defer ts.Close()

	register := func(token, agentID, hostname string) (*websocket.Conn, RegisteredPayload) {
		t.Helper()
		conn, _, err := dialAgentExecWebSocket(t, ts.URL)
		if err != nil {
			t.Fatalf("Dial: %v", err)
		}
		wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
			AgentID: agentID, Hostname: hostname, Token: token,
		}))
		return conn, wsReadRegisteredPayload(t, conn)
	}

	orgA, ack := register("token-a", "shared", "host-a")
	defer orgA.Close()
	if !ack.Success {
		t.Fatalf("org-a registration failed: %s", ack.Message)
	}
	orgB, ack := register("token-b", "shared", "host-b")
	defer orgB.Close()
	if !ack.Success {
		t.Fatalf("org-b registration failed: %s", ack.Message)
	}
	if !s.IsAgentConnectedForOrganization("org-a", "shared") ||
		!s.IsAgentConnectedForOrganization("org-b", "shared") {
		t.Fatal("same agent id must remain independently connected in both organizations")
	}
	if s.IsAgentConnected("shared") {
		t.Fatal("tenant-scoped sessions must not leak into the default organization")
	}
	orgAView := s.ForOrganization("org-a")
	orgAAgents := orgAView.GetConnectedAgents()
	if len(orgAAgents) != 1 || orgAAgents[0].Hostname != "host-a" {
		t.Fatalf("org-a server view leaked another tenant: %#v", orgAAgents)
	}
	orgBAgents := s.ForOrganization("org-b").GetConnectedAgents()
	if len(orgBAgents) != 1 || orgBAgents[0].Hostname != "host-b" {
		t.Fatalf("org-b server view leaked another tenant: %#v", orgBAgents)
	}

	duplicate, ack := register("token-c", "shared", "other-host")
	defer duplicate.Close()
	if ack.Success {
		t.Fatal("same-tenant duplicate identity from another hostname was admitted")
	}
	if !s.IsAgentConnectedForOrganization("org-a", "shared") {
		t.Fatal("rejected duplicate identity evicted the original session")
	}

	duplicateHost, ack := register("token-d", "other-id", "host-a")
	defer duplicateHost.Close()
	if ack.Success {
		t.Fatal("same-tenant hostname was admitted under a second identity")
	}
	if !s.IsAgentConnectedForOrganization("org-a", "shared") {
		t.Fatal("rejected duplicate hostname evicted the original session")
	}
}

func TestRevokedAdmissionInvalidatesStaleSocketBeforeDispatch(t *testing.T) {
	valid := true
	admission := AgentAdmission{
		OrganizationID: "org-a",
		TokenID:        "token-a",
		AgentID:        "agent-a",
		Hostname:       "host-a",
	}
	s := NewServerWithAdmissionValidator(func(token, _, _ string) (AgentAdmission, bool) {
		return admission, token == admission.TokenID
	}, func(candidate AgentAdmission) bool {
		return valid && candidate == admission
	})
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()
	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID: admission.AgentID, Hostname: admission.Hostname, Token: admission.TokenID,
	}))
	if ack := wsReadRegisteredPayload(t, conn); !ack.Success {
		t.Fatalf("registration failed: %s", ack.Message)
	}
	if !s.IsAgentConnectedForOrganization("org-a", "agent-a") {
		t.Fatal("expected admitted session")
	}

	valid = false
	ctx := WithOrganizationID(context.Background(), "org-a")
	if _, err := s.ExecuteCommand(ctx, "agent-a", ExecuteCommandPayload{
		RequestID:  "after-revocation",
		Command:    "true",
		TargetType: "agent",
		Trusted:    true,
	}); err == nil || !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("revoked stale socket remained dispatchable: %v", err)
	}
	if s.IsAgentConnectedForOrganization("org-a", "agent-a") {
		t.Fatal("revoked session remained visible as connected")
	}
}

func TestInvalidateActionRunnerSessionClosesExactSessionAndUnblocksInflightDispatch(t *testing.T) {
	admission := AgentAdmission{
		OrganizationID:   "org-a",
		TokenID:          "runner-token",
		AgentID:          "agent-a",
		Hostname:         "node.example",
		RuntimeRole:      RuntimeRoleActionRunner,
		ActionCapability: ActionCapabilityTypedV1,
	}
	s := NewServerWithAdmissionValidator(func(token, _, _ string) (AgentAdmission, bool) {
		return admission, token == admission.TokenID
	}, func(candidate AgentAdmission) bool { return candidate == admission })
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID: admission.AgentID, Hostname: admission.Hostname, Token: admission.TokenID,
		RuntimeRole: admission.RuntimeRole, ActionCapability: admission.ActionCapability,
		OperationReceiptVersion: operationreceipt.ProtocolVersion,
	}))
	if ack := wsReadRegisteredPayload(t, conn); !ack.Success {
		t.Fatalf("registration failed: %s", ack.Message)
	}

	ctx := WithOrganizationID(context.Background(), admission.OrganizationID)
	result := make(chan error, 1)
	go func() {
		_, dispatchErr := s.ExecuteHostUpdate(ctx, admission.AgentID, HostUpdatePayload{
			RequestID: "rotation-inflight", ActionID: "action-a", Operation: HostUpdateOperationInstall,
			ExpectedInventoryHash: "sha256:" + strings.Repeat("a", 64), Timeout: 30,
		})
		result <- dispatchErr
	}()
	message, err := wsReadRawMessageWithTimeout(conn, 2*time.Second)
	if err != nil || message.Type != MsgTypeHostUpdate {
		t.Fatalf("in-flight dispatch = %#v, %v", message, err)
	}

	wrong := admission
	wrong.TokenID = "replacement-token"
	if s.InvalidateActionRunnerSession(wrong) {
		t.Fatal("mismatched replacement identity evicted the current session")
	}
	if !s.InvalidateActionRunnerSession(admission) {
		t.Fatal("exact action-runner admission was not invalidated")
	}
	select {
	case dispatchErr := <-result:
		if dispatchErr == nil || !strings.Contains(dispatchErr.Error(), "disconnected") {
			t.Fatalf("in-flight dispatch error = %v", dispatchErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight dispatch did not unblock after session invalidation")
	}
	if _, err := s.ExecuteHostUpdate(ctx, admission.AgentID, HostUpdatePayload{
		RequestID: "after-rotation", ActionID: "action-b", Operation: HostUpdateOperationInstall,
		ExpectedInventoryHash: "sha256:" + strings.Repeat("b", 64), Timeout: 1,
	}); err == nil || !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("new dispatch after invalidation = %v", err)
	}
}

func TestActionRunnerAdmissionTombstoneFencesDelayedCanonicalRegistrationAndIsExact(t *testing.T) {
	cancelled := AgentAdmission{OrganizationID: "org-a", TokenID: "pending-token", AgentID: "agent-a", Hostname: "node.example", RuntimeRole: RuntimeRoleActionRunner, ActionCapability: ActionCapabilityTypedV1, ActivationPending: true}
	other := cancelled
	other.TokenID = "other-token"
	admitted := make(chan struct{})
	release := make(chan struct{})
	s := NewServerWithAdmissionValidator(func(token, _, _ string) (AgentAdmission, bool) {
		if token == cancelled.TokenID {
			close(admitted)
			<-release
			return cancelled, true
		}
		return other, token == other.TokenID
	}, func(AgentAdmission) bool { return true })
	ts := newWSServer(t, s)
	defer ts.Close()
	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{AgentID: cancelled.AgentID, Hostname: "NODE", Token: cancelled.TokenID, RuntimeRole: cancelled.RuntimeRole, ActionCapability: cancelled.ActionCapability}))
	select {
	case <-admitted:
	case <-time.After(time.Second):
		t.Fatal("registration did not reach pre-insertion admission point")
	}
	if !s.TombstoneActionRunnerAdmission(cancelled, time.Now().Add(time.Minute)) {
		t.Fatal("exact pending admission was not tombstoned")
	}
	close(release)
	if ack := wsReadRegisteredPayload(t, conn); ack.Success {
		t.Fatal("pre-admitted cancelled socket registered after durable cancellation")
	}
	otherConn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer otherConn.Close()
	wsWriteMessage(t, otherConn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{AgentID: other.AgentID, Hostname: "NODE", Token: other.TokenID, RuntimeRole: other.RuntimeRole, ActionCapability: other.ActionCapability}))
	if ack := wsReadRegisteredPayload(t, otherConn); !ack.Success {
		t.Fatalf("exact tombstone rejected another credential: %s", ack.Message)
	}
}

func TestActionRunnerAdmissionTombstoneKeyIsolatesEveryBoundField(t *testing.T) {
	base := AgentAdmission{
		OrganizationID: "org-a", TokenID: "pending-token", AgentID: "agent-a", Hostname: "node.a.example",
		RuntimeRole: RuntimeRoleActionRunner, ActionCapability: ActionCapabilityTypedV1,
	}
	baseKey := actionRunnerAdmissionTombstoneKey(base)
	canonicalSpelling := base
	canonicalSpelling.Hostname = " NODE.A.EXAMPLE. "
	if got := actionRunnerAdmissionTombstoneKey(canonicalSpelling); got != baseKey {
		t.Fatalf("equivalent canonical hostname produced a distinct key: %q != %q", got, baseKey)
	}

	mutations := map[string]func(*AgentAdmission){
		"organization": func(candidate *AgentAdmission) { candidate.OrganizationID = "org-b" },
		"token":        func(candidate *AgentAdmission) { candidate.TokenID = "other-token" },
		"agent":        func(candidate *AgentAdmission) { candidate.AgentID = "agent-b" },
		"full hostname": func(candidate *AgentAdmission) {
			// The short label is intentionally unchanged: separate FQDNs must
			// never share a cancellation fence.
			candidate.Hostname = "node.b.example"
		},
		"runtime role": func(candidate *AgentAdmission) { candidate.RuntimeRole = RuntimeRoleLegacyFullTrust },
		"capability":   func(candidate *AgentAdmission) { candidate.ActionCapability = "typed_actions.v2" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := base
			mutate(&candidate)
			if got := actionRunnerAdmissionTombstoneKey(candidate); got == baseKey {
				t.Fatalf("%s change did not isolate tombstone key", name)
			}
		})
	}
}

func TestActionRunnerAdmissionTombstoneExpiresAndClosesCurrentExactSession(t *testing.T) {
	now := time.Now().UTC()
	admission := AgentAdmission{OrganizationID: "org-a", TokenID: "pending-token", AgentID: "agent-a", Hostname: "node.example", RuntimeRole: RuntimeRoleActionRunner, ActionCapability: ActionCapabilityTypedV1, ActivationPending: true}
	s := NewServerWithAdmissionValidator(func(token, _, _ string) (AgentAdmission, bool) { return admission, token == admission.TokenID }, func(AgentAdmission) bool { return true })
	s.now = func() time.Time { return now }
	ts := newWSServer(t, s)
	defer ts.Close()
	register := func() *websocket.Conn {
		conn, _, err := dialAgentExecWebSocket(t, ts.URL)
		if err != nil {
			t.Fatal(err)
		}
		wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{AgentID: admission.AgentID, Hostname: admission.Hostname, Token: admission.TokenID, RuntimeRole: admission.RuntimeRole, ActionCapability: admission.ActionCapability}))
		return conn
	}
	current := register()
	defer current.Close()
	if ack := wsReadRegisteredPayload(t, current); !ack.Success {
		t.Fatalf("initial registration failed: %s", ack.Message)
	}
	if !s.TombstoneActionRunnerAdmission(admission, now.Add(time.Second)) {
		t.Fatal("current exact session was not tombstoned")
	}
	if s.IsAgentConnectedForOrganization(admission.OrganizationID, admission.AgentID) {
		t.Fatal("tombstoned current session remained connected")
	}
	if err := current.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := current.ReadMessage(); err == nil {
		t.Fatal("tombstoned current websocket remained open")
	}
	now = now.Add(2 * time.Second)
	afterExpiry := register()
	defer afterExpiry.Close()
	if ack := wsReadRegisteredPayload(t, afterExpiry); !ack.Success {
		t.Fatalf("expired tombstone rejected registration: %s", ack.Message)
	}
	s.mu.RLock()
	tombstoneCount := len(s.actionRunnerAdmissionTombstones)
	s.mu.RUnlock()
	if tombstoneCount != 0 {
		t.Fatalf("expired admission tombstone was not pruned: %d entries remain", tombstoneCount)
	}
}

func TestInvalidateAgentSessionRequiresExactLegacyAdmission(t *testing.T) {
	admission := AgentAdmission{
		OrganizationID: "org-a", TokenID: "collector-token", AgentID: "agent-a",
		Hostname: "node.example", RuntimeRole: RuntimeRoleLegacyFullTrust,
	}
	s := NewServerWithAdmissionValidator(func(token, _, _ string) (AgentAdmission, bool) {
		return admission, token == admission.TokenID
	}, func(candidate AgentAdmission) bool { return candidate == admission })
	ts := newWSServer(t, s)
	defer ts.Close()
	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID: admission.AgentID, Hostname: admission.Hostname, Token: admission.TokenID,
	}))
	if ack := wsReadRegisteredPayload(t, conn); !ack.Success {
		t.Fatalf("registration failed: %s", ack.Message)
	}
	wrong := admission
	wrong.TokenID = "other-token"
	if s.InvalidateAgentSession(wrong) {
		t.Fatal("mismatched token evicted legacy session")
	}
	if !s.InvalidateAgentSession(admission) {
		t.Fatal("exact legacy admission was not invalidated")
	}
	if s.IsAgentConnectedForOrganization("org-a", "agent-a") {
		t.Fatal("invalidated legacy session remained connected")
	}
}

// registerCancelTestAgent registers agent "a1" over the websocket harness and
// returns after the registration ack has been read.
func registerCancelTestAgent(t *testing.T, s *Server, tsURL string) *cancelTestConn {
	t.Helper()
	conn, _, err := dialAgentExecWebSocket(t, tsURL)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID:  "a1",
		Hostname: "host1",
		Version:  "1.2.3",
		Platform: "linux",
		Token:    "any",
	}))
	_ = wsReadRegisteredPayload(t, conn)
	return &cancelTestConn{t: t, conn: conn}
}

func registerTypedCancelTestAgent(t *testing.T, s *Server, tsURL string) *cancelTestConn {
	t.Helper()
	conn, _, err := dialAgentExecWebSocket(t, tsURL)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID: "typed-a1", Hostname: "typed-host1", Version: "1.2.3", Platform: "linux", Token: "typed-token",
		RuntimeRole: RuntimeRoleActionRunner, ActionCapability: ActionCapabilityTypedV1,
		OperationReceiptVersion: operationreceipt.ProtocolVersion,
	}))
	if ack := wsReadRegisteredPayload(t, conn); !ack.Success {
		t.Fatalf("typed action runner registration failed: %s", ack.Message)
	}
	return &cancelTestConn{t: t, conn: conn}
}

type cancelTestConn struct {
	t    *testing.T
	conn *websocket.Conn
}

// nextMessage returns the next message from the server, or ok=false when
// nothing arrives before the timeout.
func (c *cancelTestConn) nextMessage(timeout time.Duration) (wsRawMessage, bool) {
	c.t.Helper()
	msg, err := wsReadRawMessageWithTimeout(c.conn, timeout)
	if err != nil {
		return wsRawMessage{}, false
	}
	return msg, true
}

// The probe-storm incident (minipc, 2026-08-20) started with the server
// dispatching commands under a parent context that had already expired: the
// send succeeded, ExecuteCommand returned "context deadline exceeded,
// duration 0.05", and the agent was left running the command. An expired
// context must fail the call before anything reaches the agent.
func TestExecuteCommand_ExpiredContextNeverDispatches(t *testing.T) {
	s := NewServer(allowAllTestTokens)
	ts := newWSServer(t, s)
	defer ts.Close()

	agent := registerCancelTestAgent(t, s, ts.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := s.ExecuteCommand(ctx, "a1", ExecuteCommandPayload{
		RequestID: "req-expired",
		Command:   "echo hi",
		Timeout:   5,
		Trusted:   true,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ExecuteCommand error = %v, want context.Canceled", err)
	}
	if !strings.Contains(err.Error(), "not dispatched") {
		t.Fatalf("error should say the command was not dispatched, got %q", err)
	}

	// The agent must never see the command.
	if msg, ok := agent.nextMessage(500 * time.Millisecond); ok && msg.Type == MsgTypeExecuteCmd {
		t.Fatalf("agent received execute_command despite expired context")
	}
}

// When the server stops waiting for a dispatched command (its own timeout or
// the caller's context), it must tell the agent to abort the execution so the
// process tree is reaped instead of running on for minutes.
func TestExecuteCommand_AbandonedCommandSendsCancel(t *testing.T) {
	cases := []struct {
		name    string
		timeout int // ExecuteCommandPayload.Timeout in seconds
		abandon func(cancel context.CancelFunc)
		wantErr string
	}{
		{
			name:    "server timeout",
			timeout: 1,
			abandon: func(context.CancelFunc) {}, // let the 1s timer fire
			wantErr: "timed out",
		},
		{
			name:    "caller context canceled",
			timeout: 30,
			abandon: func(cancel context.CancelFunc) {
				time.Sleep(200 * time.Millisecond)
				cancel()
			},
			wantErr: "context canceled",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewServer(allowAllTestTokens)
			ts := newWSServer(t, s)
			defer ts.Close()

			agent := registerCancelTestAgent(t, s, ts.URL)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go tc.abandon(cancel)

			requestID := "req-" + strings.ReplaceAll(tc.name, " ", "-")
			_, err := s.ExecuteCommand(ctx, "a1", ExecuteCommandPayload{
				RequestID: requestID,
				Command:   "sleep 300",
				Timeout:   tc.timeout,
				Trusted:   true,
			})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ExecuteCommand error = %v, want containing %q", err, tc.wantErr)
			}

			// The agent first receives the command, then the cancellation.
			sawExecute := false
			sawCancel := false
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) && !sawCancel {
				msg, ok := agent.nextMessage(time.Until(deadline))
				if !ok {
					break
				}
				switch msg.Type {
				case MsgTypeExecuteCmd:
					sawExecute = true
				case MsgTypeCancelCmd:
					if msg.Payload == nil {
						t.Fatalf("cancel_command payload missing")
					}
					var payload CancelCommandPayload
					if err := json.Unmarshal(*msg.Payload, &payload); err != nil {
						t.Fatalf("unmarshal cancel_command payload: %v", err)
					}
					if payload.RequestID != requestID {
						t.Fatalf("cancel_command request_id = %q, want %q", payload.RequestID, requestID)
					}
					sawCancel = true
				}
			}
			if !sawExecute {
				t.Fatalf("agent never received execute_command")
			}
			if !sawCancel {
				t.Fatalf("agent never received cancel_command after the server abandoned the request")
			}
		})
	}
}

func TestTypedOperations_AbandonedDispatchSendsExactlyOneCancel(t *testing.T) {
	type dispatchFunc func(context.Context, *Server) error
	cases := []struct {
		name        string
		messageType MessageType
		dispatch    dispatchFunc
	}{
		{
			name: "host update", messageType: MsgTypeHostUpdate,
			dispatch: func(ctx context.Context, s *Server) error {
				req := HostUpdatePayload{RequestID: "update.cancel.1", ActionID: "update", Operation: HostUpdateOperationInstall, ExpectedInventoryHash: "sha256:" + strings.Repeat("a", 64), Timeout: 30}
				if err := BindHostUpdatePayload(&req); err != nil {
					return err
				}
				_, err := s.ExecuteHostUpdate(ctx, "typed-a1", req)
				return err
			},
		},
		{
			name: "storage cleanup", messageType: MsgTypeHostStorageCleanup,
			dispatch: func(ctx context.Context, s *Server) error {
				req := HostStorageCleanupPayload{RequestID: "cleanup.cancel.1", ActionID: "cleanup", Operation: HostStorageCleanupOperationPackageCache, ExpectedFingerprint: "sha256:" + strings.Repeat("b", 64), Timeout: 30}
				if err := BindHostStorageCleanupPayload(&req); err != nil {
					return err
				}
				_, err := s.ExecuteHostStorageCleanup(ctx, "typed-a1", req)
				return err
			},
		},
		{
			name: "Proxmox guest lifecycle", messageType: MsgTypeProxmoxGuestLifecycle,
			dispatch: func(ctx context.Context, s *Server) error {
				req := ProxmoxGuestLifecyclePayload{RequestID: "pve.cancel.1", ActionID: "pve", Operation: "shutdown", GuestKind: "vm", VMID: 101, ExpectedStatus: "running", Timeout: 30}
				if err := BindProxmoxGuestLifecyclePayload(&req); err != nil {
					return err
				}
				_, err := s.ExecuteProxmoxGuestLifecycle(ctx, "typed-a1", req)
				return err
			},
		},
		{
			name: "container lifecycle", messageType: MsgTypeDockerContainerLifecycle,
			dispatch: func(ctx context.Context, s *Server) error {
				req := DockerContainerLifecyclePayload{RequestID: "docker.cancel.1", ActionID: "docker", Operation: DockerContainerOperationRestart, Runtime: "docker", ContainerID: strings.Repeat("c", 12), ExpectedState: "running", Timeout: 30}
				if err := BindDockerContainerLifecyclePayload(&req); err != nil {
					return err
				}
				_, err := s.ExecuteDockerContainerLifecycle(ctx, "typed-a1", req)
				return err
			},
		},
		{
			name: "container update", messageType: MsgTypeDockerContainerUpdate,
			dispatch: func(ctx context.Context, s *Server) error {
				req := DockerContainerUpdatePayload{RequestID: "docker-update.cancel.1", ActionID: "docker-update", Runtime: "docker", ContainerID: strings.Repeat("d", 12), ExpectedImageDigest: "sha256:" + strings.Repeat("e", 64), Timeout: 30}
				if err := BindDockerContainerUpdatePayload(&req); err != nil {
					return err
				}
				_, err := s.ExecuteDockerContainerUpdate(ctx, "typed-a1", req)
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			admission := AgentAdmission{TokenID: "typed-token", AgentID: "typed-a1", Hostname: "typed-host1", RuntimeRole: RuntimeRoleActionRunner, ActionCapability: ActionCapabilityTypedV1}
			s := NewServerWithAdmissionValidator(func(token, _, _ string) (AgentAdmission, bool) {
				return admission, token == admission.TokenID
			}, func(AgentAdmission) bool { return true })
			ts := newWSServer(t, s)
			defer ts.Close()
			agent := registerTypedCancelTestAgent(t, s, ts.URL)

			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- tc.dispatch(ctx, s) }()

			request, ok := agent.nextMessage(3 * time.Second)
			if !ok || request.Type != tc.messageType {
				t.Fatalf("dispatched message = %+v, want %s", request, tc.messageType)
			}
			cancel()
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Fatalf("dispatch error = %v, want context.Canceled", err)
			}

			cancelMessage, ok := agent.nextMessage(3 * time.Second)
			if !ok || cancelMessage.Type != MsgTypeCancelCmd {
				t.Fatalf("cancellation message = %+v, want %s", cancelMessage, MsgTypeCancelCmd)
			}
			var payload CancelCommandPayload
			if cancelMessage.Payload == nil || json.Unmarshal(*cancelMessage.Payload, &payload) != nil || payload.RequestID != request.ID {
				t.Fatalf("cancellation payload = %+v, want request_id %q", payload, request.ID)
			}
			if duplicate, ok := agent.nextMessage(250 * time.Millisecond); ok && duplicate.Type == MsgTypeCancelCmd {
				t.Fatalf("received duplicate cancellation: %+v", duplicate)
			}
		})
	}
}

func TestTypedOperation_TimeoutSendsCancelAndExpiredContextNeverDispatches(t *testing.T) {
	newTypedServer := func(t *testing.T) (*Server, *httptest.Server, *cancelTestConn) {
		t.Helper()
		admission := AgentAdmission{TokenID: "typed-token", AgentID: "typed-a1", Hostname: "typed-host1", RuntimeRole: RuntimeRoleActionRunner, ActionCapability: ActionCapabilityTypedV1}
		s := NewServerWithAdmissionValidator(func(token, _, _ string) (AgentAdmission, bool) {
			return admission, token == admission.TokenID
		}, func(AgentAdmission) bool { return true })
		ts := newWSServer(t, s)
		t.Cleanup(ts.Close)
		return s, ts, registerTypedCancelTestAgent(t, s, ts.URL)
	}

	t.Run("expired caller context", func(t *testing.T) {
		s, _, agent := newTypedServer(t)
		expired, expire := context.WithCancel(context.Background())
		expire()
		notDispatched := ProxmoxGuestLifecyclePayload{RequestID: "pve.expired.1", ActionID: "pve", Operation: "shutdown", GuestKind: "vm", VMID: 101, ExpectedStatus: "running", Timeout: 30}
		if err := BindProxmoxGuestLifecyclePayload(&notDispatched); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ExecuteProxmoxGuestLifecycle(expired, "typed-a1", notDispatched); !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "not dispatched") {
			t.Fatalf("expired dispatch error = %v", err)
		}
		if unexpected, ok := agent.nextMessage(250 * time.Millisecond); ok {
			t.Fatalf("expired context reached action runner: %+v", unexpected)
		}
	})

	t.Run("server timeout", func(t *testing.T) {
		s, _, agent := newTypedServer(t)
		timed := ProxmoxGuestLifecyclePayload{RequestID: "pve.timeout.1", ActionID: "pve", Operation: "shutdown", GuestKind: "vm", VMID: 101, ExpectedStatus: "running", Timeout: 1}
		if err := BindProxmoxGuestLifecyclePayload(&timed); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { _, err := s.ExecuteProxmoxGuestLifecycle(context.Background(), "typed-a1", timed); done <- err }()
		request, ok := agent.nextMessage(3 * time.Second)
		if !ok || request.Type != MsgTypeProxmoxGuestLifecycle {
			t.Fatalf("timeout request = %+v", request)
		}
		if err := <-done; err == nil || !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("timeout error = %v", err)
		}
		cancelMessage, ok := agent.nextMessage(3 * time.Second)
		if !ok || cancelMessage.Type != MsgTypeCancelCmd {
			t.Fatalf("timeout cancellation = %+v", cancelMessage)
		}
	})
}
