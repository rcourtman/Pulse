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
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

type wsRawMessage struct {
	Type      MessageType      `json:"type"`
	ID        string           `json:"id,omitempty"`
	Timestamp time.Time        `json:"timestamp"`
	Payload   *json.RawMessage `json:"payload,omitempty"`
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
		AgentID: "host-agent-1", Hostname: "host1", Version: "6.0.6", Platform: "linux", Token: "any",
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
			Verification:   HostUpdateVerificationVerified,
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
		AgentID: "host-agent-cleanup", Hostname: "host1", Version: "6.0.6", Platform: "linux", Token: "any",
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
}
