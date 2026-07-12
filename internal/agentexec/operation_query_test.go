package agentexec

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
)

func testOperationIdentity(t *testing.T, agentID string) operationreceipt.Identity {
	t.Helper()
	req := HostUpdatePayload{RequestID: "action-1.dispatch.1", ActionID: "action-1", Operation: HostUpdateOperationInstall, ExpectedInventoryHash: "sha256:" + strings.Repeat("a", 64)}
	if err := BindHostUpdatePayload(&req); err != nil {
		t.Fatal(err)
	}
	return HostUpdateOperationIdentity(agentID, req)
}
func registeredTestAgent(t *testing.T, s *Server, agentID string) (*websocket.Conn, func()) {
	t.Helper()
	ts := newWSServer(t, s)
	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{AgentID: agentID, Hostname: "host", Version: "6", Platform: "linux", Token: "ok", OperationReceiptVersion: operationreceipt.ProtocolVersion}))
	if !wsReadRegisteredPayload(t, conn).Success {
		t.Fatal("registration failed")
	}
	return conn, func() { conn.Close(); ts.Close() }
}
func interruptedQueryResult(id operationreceipt.Identity, state operationreceipt.State) operationreceipt.QueryResult {
	now := time.Now().UTC()
	record := operationreceipt.Record{Identity: id, State: state, AcceptedAt: now, StartedAt: now}
	if state == operationreceipt.StateTombstone {
		record.TerminalAt = now
	}
	return operationreceipt.QueryResult{Version: operationreceipt.ProtocolVersion, Status: operationreceipt.QueryFoundInterrupted, Record: &record}
}
func hostileTerminalQueryResult(id operationreceipt.Identity, kind string, version int, payload json.RawMessage) operationreceipt.QueryResult {
	now := time.Now().UTC()
	record := operationreceipt.Record{Identity: id, State: operationreceipt.StateTerminal, AcceptedAt: now.Add(-2 * time.Second), StartedAt: now.Add(-time.Second), TerminalAt: now, ResultKind: kind, ResultVersion: version, Result: payload}
	return operationreceipt.QueryResult{Version: 1, Status: operationreceipt.QueryFoundTerminal, Record: &record}
}

func TestQueryAgentOperationRejectsHostileCorrelations(t *testing.T) {
	old := operationQueryTimeout
	operationQueryTimeout = 80 * time.Millisecond
	defer func() { operationQueryTimeout = old }()
	cases := []struct {
		name  string
		reply func(t *testing.T, conn *websocket.Conn, query wsRawMessage, id operationreceipt.Identity)
	}{
		{"wrong_message_id", func(t *testing.T, c *websocket.Conn, q wsRawMessage, id operationreceipt.Identity) {
			wsWriteMessage(t, c, mustNewMessage(t, MsgTypeOperationQueryResult, "other", interruptedQueryResult(id, operationreceipt.StateInterrupted)))
		}},
		{"wrong_identity", func(t *testing.T, c *websocket.Conn, q wsRawMessage, id operationreceipt.Identity) {
			id.ActionID = "other"
			wsWriteMessage(t, c, mustNewMessage(t, MsgTypeOperationQueryResult, q.ID, interruptedQueryResult(id, operationreceipt.StateInterrupted)))
		}},
		{"malformed_unknown", func(t *testing.T, c *websocket.Conn, q wsRawMessage, id operationreceipt.Identity) {
			raw := json.RawMessage(`{"version":1,"status":"not_found","unknown":true}`)
			wsWriteMessage(t, c, Message{Type: MsgTypeOperationQueryResult, ID: q.ID, Timestamp: time.Now(), Payload: raw})
		}},
		{"trailing", func(t *testing.T, c *websocket.Conn, q wsRawMessage, id operationreceipt.Identity) {
			raw := []byte(`{"type":"agent_operation_query_result","id":"` + q.ID + `","payload":{"version":1,"status":"not_found"} {}}`)
			if err := c.WriteMessage(websocket.TextMessage, raw); err != nil {
				t.Fatalf("write trailing result: %v", err)
			}
		}},
		{"unknown_terminal_kind", func(t *testing.T, c *websocket.Conn, q wsRawMessage, id operationreceipt.Identity) {
			wsWriteMessage(t, c, mustNewMessage(t, MsgTypeOperationQueryResult, q.ID, hostileTerminalQueryResult(id, "unknown", 1, json.RawMessage(`{"safe":true}`))))
		}},
		{"unknown_terminal_version", func(t *testing.T, c *websocket.Conn, q wsRawMessage, id operationreceipt.Identity) {
			wsWriteMessage(t, c, mustNewMessage(t, MsgTypeOperationQueryResult, q.ID, hostileTerminalQueryResult(id, HostUpdateReceiptKind, 99, json.RawMessage(`{"safe":true}`))))
		}},
		{"malformed_terminal_payload", func(t *testing.T, c *websocket.Conn, q wsRawMessage, id operationreceipt.Identity) {
			wsWriteMessage(t, c, mustNewMessage(t, MsgTypeOperationQueryResult, q.ID, hostileTerminalQueryResult(id, HostUpdateReceiptKind, 1, json.RawMessage(`{"request_id":"x","unknown":true}`))))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewServer(func(token, agent, host string) bool { return token == "ok" })
			conn, cleanup := registeredTestAgent(t, s, "agent-1")
			defer cleanup()
			id := testOperationIdentity(t, "agent-1")
			done := make(chan error, 1)
			go func() { _, err := s.QueryAgentOperation(context.Background(), "agent-1", id); done <- err }()
			query := wsReadRawMessage(t, conn)
			tc.reply(t, conn, query, id)
			if err := <-done; err == nil || !strings.Contains(err.Error(), "timed out") {
				t.Fatalf("query err=%v", err)
			}
		})
	}
}

func TestQueryAgentOperationWrongAgentLateDuplicateAndInterruptedAreInert(t *testing.T) {
	old := operationQueryTimeout
	operationQueryTimeout = 80 * time.Millisecond
	defer func() { operationQueryTimeout = old }()
	s := NewServer(func(token, agent, host string) bool { return token == "ok" })
	first, cleanupFirst := registeredTestAgent(t, s, "agent-1")
	defer cleanupFirst()
	second, cleanupSecond := registeredTestAgent(t, s, "agent-2")
	defer cleanupSecond()
	id := testOperationIdentity(t, "agent-1")
	done := make(chan error, 1)
	go func() { _, err := s.QueryAgentOperation(context.Background(), "agent-1", id); done <- err }()
	query := wsReadRawMessage(t, first)
	wsWriteMessage(t, second, mustNewMessage(t, MsgTypeOperationQueryResult, query.ID, interruptedQueryResult(id, operationreceipt.StateInterrupted)))
	if err := <-done; err == nil {
		t.Fatal("wrong-agent response completed query")
	}
	wsWriteMessage(t, first, mustNewMessage(t, MsgTypeOperationQueryResult, query.ID, interruptedQueryResult(id, operationreceipt.StateInterrupted)))
	// A fresh query accepts interrupted/tombstone state but never turns it into a terminal result.
	resultCh := make(chan operationreceipt.QueryResult, 1)
	errCh := make(chan error, 1)
	go func() {
		r, err := s.QueryAgentOperation(context.Background(), "agent-1", id)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- r
	}()
	q2 := wsReadRawMessage(t, first)
	response := interruptedQueryResult(id, operationreceipt.StateTombstone)
	wsWriteMessage(t, first, mustNewMessage(t, MsgTypeOperationQueryResult, q2.ID, response))
	wsWriteMessage(t, first, mustNewMessage(t, MsgTypeOperationQueryResult, q2.ID, response))
	select {
	case err := <-errCh:
		t.Fatal(err)
	case got := <-resultCh:
		if got.Status != operationreceipt.QueryFoundInterrupted || got.Record.State != operationreceipt.StateTombstone {
			t.Fatalf("result=%+v", got)
		}
	}
}

func TestOperationQueryBeforeStateDigestMismatchFailsClosed(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name     string
		identity operationreceipt.Identity
		record   operationreceipt.Record
	}{
		func() struct {
			name     string
			identity operationreceipt.Identity
			record   operationreceipt.Record
		} {
			req := HostUpdatePayload{RequestID: "u.dispatch.1", ActionID: "u", Operation: HostUpdateOperationInstall, ExpectedInventoryHash: "sha256:" + strings.Repeat("a", 64)}
			_ = BindHostUpdatePayload(&req)
			id := HostUpdateOperationIdentity("agent", req)
			result := HostUpdateResultPayload{RequestID: req.RequestID, ActionID: req.ActionID, Success: true, ExecutionPhase: HostUpdatePhaseComplete, MutationStarted: true, Before: HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: "sha256:" + strings.Repeat("b", 64), PendingCount: 1, CheckedAt: now.Add(-time.Second)}, After: HostPackageUpdateSnapshot{Supported: true, Manager: "apt", InventoryHash: "sha256:" + strings.Repeat("c", 64), PendingCount: 0, CheckedAt: now}, Verification: HostUpdateVerificationVerified}
			raw, _ := json.Marshal(result)
			return struct {
				name     string
				identity operationreceipt.Identity
				record   operationreceipt.Record
			}{"update", id, operationreceipt.Record{Identity: id, State: operationreceipt.StateTerminal, AcceptedAt: now.Add(-2 * time.Second), StartedAt: now.Add(-time.Second), TerminalAt: now, ResultKind: HostUpdateReceiptKind, ResultVersion: 1, Result: raw}}
		}(),
		func() struct {
			name     string
			identity operationreceipt.Identity
			record   operationreceipt.Record
		} {
			req := HostStorageCleanupPayload{RequestID: "c.dispatch.1", ActionID: "c", Operation: HostStorageCleanupOperationPackageCache, ExpectedFingerprint: "sha256:" + strings.Repeat("a", 64)}
			_ = BindHostStorageCleanupPayload(&req)
			id := HostStorageCleanupOperationIdentity("agent", req)
			result := HostStorageCleanupResultPayload{RequestID: req.RequestID, ActionID: req.ActionID, Success: true, ExecutionPhase: HostStorageCleanupPhaseComplete, MutationStarted: true, Before: HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: "sha256:" + strings.Repeat("b", 64), ReclaimableBytes: 10, CheckedAt: now.Add(-time.Second)}, After: HostStorageCleanupSnapshot{Supported: true, Provider: "apt-package-cache", Fingerprint: "sha256:" + strings.Repeat("c", 64), ReclaimableBytes: 1, CheckedAt: now}, ReclaimedBytes: 9, Verification: HostStorageCleanupVerificationVerified}
			raw, _ := json.Marshal(result)
			return struct {
				name     string
				identity operationreceipt.Identity
				record   operationreceipt.Record
			}{"cleanup", id, operationreceipt.Record{Identity: id, State: operationreceipt.StateTerminal, AcceptedAt: now.Add(-2 * time.Second), StartedAt: now.Add(-time.Second), TerminalAt: now, ResultKind: HostStorageCleanupReceiptKind, ResultVersion: 1, Result: raw}}
		}(),
	} {
		t.Run(tc.name, func(t *testing.T) {
			query := operationreceipt.QueryResult{Version: 1, Status: operationreceipt.QueryFoundTerminal, Record: &tc.record}
			if err := ValidateOperationQueryResultForIdentity(query, tc.identity, now); err == nil {
				t.Fatal("mismatched before-state digest accepted")
			}
		})
	}
}

func TestLegacyAgentWithoutReceiptProtocolRemainsConnectedButTypedMutationFailsClosed(t *testing.T) {
	s := NewServer(func(token, agent, host string) bool { return token == "ok" })
	conn, cleanup := registeredTestAgentLegacy(t, s, "legacy-agent")
	defer cleanup()
	req := HostUpdatePayload{RequestID: "legacy.dispatch.1", ActionID: "legacy", Operation: HostUpdateOperationInstall, ExpectedInventoryHash: "sha256:" + strings.Repeat("a", 64), Timeout: 1}
	if _, err := s.ExecuteHostUpdate(context.Background(), "legacy-agent", req); err == nil || !strings.Contains(err.Error(), "does not support durable operation receipts") {
		t.Fatalf("err=%v", err)
	}
	if _, err := wsReadRawMessageWithTimeout(conn, 50*time.Millisecond); err == nil {
		t.Fatal("legacy agent received typed mutation")
	}
}

func registeredTestAgentLegacy(t *testing.T, s *Server, agentID string) (*websocket.Conn, func()) {
	t.Helper()
	ts := newWSServer(t, s)
	conn, _, err := dialAgentExecWebSocket(t, ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{AgentID: agentID, Hostname: "host", Version: "5", Platform: "linux", Token: "ok"}))
	if !wsReadRegisteredPayload(t, conn).Success {
		t.Fatal("registration failed")
	}
	return conn, func() { conn.Close(); ts.Close() }
}

var _ = errors.Is
