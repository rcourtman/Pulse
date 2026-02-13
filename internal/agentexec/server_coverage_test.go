package agentexec

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type noHijackResponseWriter struct {
	header http.Header
}

func (w *noHijackResponseWriter) Header() http.Header {
	return w.header
}

func (w *noHijackResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (w *noHijackResponseWriter) WriteHeader(int) {}

func newConnPair(t *testing.T) (*websocket.Conn, *websocket.Conn, func()) {
	t.Helper()

	serverConnCh := make(chan *websocket.Conn, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		serverConnCh <- conn
	}))

	clientConn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL), nil)
	if err != nil {
		ts.Close()
		t.Fatalf("Dial: %v", err)
	}

	var serverConn *websocket.Conn
	select {
	case serverConn = <-serverConnCh:
	case <-time.After(2 * time.Second):
		clientConn.Close()
		ts.Close()
		t.Fatal("timed out waiting for server connection")
	}

	cleanup := func() {
		clientConn.Close()
		serverConn.Close()
		ts.Close()
	}

	return serverConn, clientConn, cleanup
}

func TestHandleWebSocket_UpgradeFailureAndDeadlineErrors(t *testing.T) {
	s := NewServer(nil)
	req := httptest.NewRequest(http.MethodGet, "http://example/ws", nil)
	s.HandleWebSocket(&noHijackResponseWriter{header: make(http.Header)}, req)
}

func TestHandleWebSocket_RegistrationReadError(t *testing.T) {
	s := NewServer(nil)
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	conn.Close()
}

func TestHandleWebSocket_RegistrationMessageJSONError(t *testing.T) {
	s := NewServer(nil)
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte("{")); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatalf("expected server to close on invalid JSON")
	}
}

func TestHandleWebSocket_RegistrationPayloadMissing(t *testing.T) {
	s := NewServer(nil)
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", nil))

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatalf("expected server to close on missing payload")
	}
}

func TestHandleWebSocket_RegistrationPayloadUnmarshalError(t *testing.T) {
	s := NewServer(nil)
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"agent_register","payload":"oops"}`)); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatalf("expected server to close on invalid payload")
	}
}

func TestHandleWebSocket_PongHandler(t *testing.T) {
	s := NewServer(nil)
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID:  "a1",
		Hostname: "host1",
		Token:    "any",
	}))
	_ = wsReadRegisteredPayload(t, conn)

	if err := conn.WriteControl(websocket.PongMessage, []byte("pong"), time.Now().Add(time.Second)); err != nil {
		t.Fatalf("WriteControl pong: %v", err)
	}

	conn.Close()
	waitFor(t, 2*time.Second, func() bool { return !s.IsAgentConnected("a1") })
}

func TestReadLoopDone(t *testing.T) {
	s := NewServer(nil)
	serverConn, clientConn, cleanup := newConnPair(t)
	defer cleanup()

	ac := &agentConn{
		conn:  serverConn,
		agent: ConnectedAgent{AgentID: "a1"},
		done:  make(chan struct{}),
	}
	close(ac.done)

	s.mu.Lock()
	s.agents["a1"] = ac
	s.mu.Unlock()

	s.readLoop(ac)

	if s.IsAgentConnected("a1") {
		t.Fatalf("expected agent to be removed")
	}
	clientConn.Close()
}

func TestReadLoopUnexpectedCloseError(t *testing.T) {
	s := NewServer(nil)
	serverConn, clientConn, cleanup := newConnPair(t)
	defer cleanup()

	ac := &agentConn{
		conn:  serverConn,
		agent: ConnectedAgent{AgentID: "a1"},
		done:  make(chan struct{}),
	}

	s.mu.Lock()
	s.agents["a1"] = ac
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		s.readLoop(ac)
		close(done)
	}()

	_ = clientConn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseProtocolError, "bye"),
		time.Now().Add(time.Second),
	)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("readLoop did not exit")
	}
}

func TestReadLoopCommandResultBranches(t *testing.T) {
	s := NewServer(nil)
	serverConn, clientConn, cleanup := newConnPair(t)
	defer cleanup()

	ac := &agentConn{
		conn:  serverConn,
		agent: ConnectedAgent{AgentID: "a1"},
		done:  make(chan struct{}),
	}

	s.mu.Lock()
	s.agents["a1"] = ac
	s.pendingReqs[pendingRequestKey("a1", "req-full")] = make(chan CommandResultPayload)
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		s.readLoop(ac)
		close(done)
	}()

	_ = clientConn.WriteMessage(websocket.TextMessage, []byte("{"))
	_ = clientConn.WriteMessage(websocket.TextMessage, []byte(`{"type":"command_result","payload":{"request_id":123}}`))
	_ = clientConn.WriteMessage(websocket.TextMessage, []byte(`{"type":"command_result","payload":{"request_id":"req-full","success":true}}`))
	_ = clientConn.WriteMessage(websocket.TextMessage, []byte(`{"type":"command_result","payload":{"request_id":"req-missing","success":true}}`))

	clientConn.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("readLoop did not exit")
	}

	s.mu.Lock()
	delete(s.pendingReqs, pendingRequestKey("a1", "req-full"))
	s.mu.Unlock()
}

func TestReadLoopCommandResultWrongAgentIsolation(t *testing.T) {
	s := NewServer(nil)
	serverConn, clientConn, cleanup := newConnPair(t)
	defer cleanup()

	ac := &agentConn{
		conn:  serverConn,
		agent: ConnectedAgent{AgentID: "a1"},
		done:  make(chan struct{}),
	}

	foreignCh := make(chan CommandResultPayload, 1)

	s.mu.Lock()
	s.agents["a1"] = ac
	s.pendingReqs[pendingRequestKey("a2", "req-shared")] = foreignCh
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		s.readLoop(ac)
		close(done)
	}()

	_ = clientConn.WriteMessage(websocket.TextMessage, []byte(`{"type":"command_result","payload":{"request_id":"req-shared","success":true}}`))
	clientConn.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("readLoop did not exit")
	}

	select {
	case <-foreignCh:
		t.Fatalf("expected result not to be delivered across agents")
	default:
	}
}

func TestPingLoopSuccessAndStop(t *testing.T) {
	origInterval := pingInterval
	t.Cleanup(func() { pingInterval = origInterval })
	pingInterval = 5 * time.Millisecond

	s := NewServer(nil)
	serverConn, _, cleanup := newConnPair(t)
	defer cleanup()

	ac := &agentConn{
		conn:  serverConn,
		agent: ConnectedAgent{AgentID: "a1"},
		done:  make(chan struct{}),
	}

	stop := make(chan struct{})
	exited := make(chan struct{})
	go func() {
		s.pingLoop(ac, stop)
		close(exited)
	}()

	time.Sleep(2 * pingInterval)
	close(stop)

	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatalf("pingLoop did not exit")
	}
}

func TestPingLoopFailuresClose(t *testing.T) {
	origInterval := pingInterval
	t.Cleanup(func() { pingInterval = origInterval })
	pingInterval = 5 * time.Millisecond

	s := NewServer(nil)
	serverConn, _, cleanup := newConnPair(t)
	defer cleanup()

	ac := &agentConn{
		conn:  serverConn,
		agent: ConnectedAgent{AgentID: "a1"},
		done:  make(chan struct{}),
	}

	serverConn.Close()

	stop := make(chan struct{})
	exited := make(chan struct{})
	go func() {
		s.pingLoop(ac, stop)
		close(exited)
	}()

	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatalf("pingLoop did not exit after failures")
	}
}

func TestSendMessageMarshalError(t *testing.T) {
	s := NewServer(nil)
	if err := s.sendMessage(nil, Message{Payload: json.RawMessage("{")}); err == nil {
		t.Fatalf("expected marshal error")
	}
}

func TestExecuteCommandSendError(t *testing.T) {
	s := NewServer(nil)
	serverConn, _, cleanup := newConnPair(t)
	defer cleanup()

	serverConn.Close()

	ac := &agentConn{
		conn:  serverConn,
		agent: ConnectedAgent{AgentID: "a1"},
		done:  make(chan struct{}),
	}
	s.mu.Lock()
	s.agents["a1"] = ac
	s.mu.Unlock()

	_, err := s.ExecuteCommand(context.Background(), "a1", ExecuteCommandPayload{
		RequestID: "r1",
		Command:   "echo ok",
		Timeout:   1,
	})
	if err == nil {
		t.Fatalf("expected send error")
	}
}

func TestExecuteCommandTimeoutAndCancel(t *testing.T) {
	s := NewServer(nil)
	serverConn, _, cleanup := newConnPair(t)
	defer cleanup()

	ac := &agentConn{
		conn:  serverConn,
		agent: ConnectedAgent{AgentID: "a1"},
		done:  make(chan struct{}),
	}
	s.mu.Lock()
	s.agents["a1"] = ac
	s.mu.Unlock()

	_, err := s.ExecuteCommand(context.Background(), "a1", ExecuteCommandPayload{
		RequestID: "r-timeout",
		Command:   "echo ok",
		Timeout:   1,
	})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = s.ExecuteCommand(ctx, "a1", ExecuteCommandPayload{
		RequestID: "r-cancel",
		Command:   "echo ok",
		Timeout:   1,
	})
	if err == nil {
		t.Fatalf("expected cancel error")
	}
}

func TestExecuteCommandDefaultTimeout(t *testing.T) {
	s := NewServer(nil)
	serverConn, _, cleanup := newConnPair(t)
	defer cleanup()

	ac := &agentConn{
		conn:  serverConn,
		agent: ConnectedAgent{AgentID: "a1"},
		done:  make(chan struct{}),
	}
	s.mu.Lock()
	s.agents["a1"] = ac
	s.mu.Unlock()

	go func() {
		for {
			s.mu.RLock()
			ch := s.pendingReqs[pendingRequestKey("a1", "r-default")]
			s.mu.RUnlock()
			if ch != nil {
				ch <- CommandResultPayload{RequestID: "r-default", Success: true}
				return
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()

	result, err := s.ExecuteCommand(context.Background(), "a1", ExecuteCommandPayload{
		RequestID: "r-default",
		Command:   "echo ok",
	})
	if err != nil || result == nil || !result.Success {
		t.Fatalf("expected success, got result=%v err=%v", result, err)
	}
}

func TestReadFileRoundTrip(t *testing.T) {
	s := NewServer(nil)
	ts := newWSServer(t, s)
	defer ts.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL), nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	wsWriteMessage(t, conn, mustNewMessage(t, MsgTypeAgentRegister, "", AgentRegisterPayload{
		AgentID:  "a1",
		Hostname: "host1",
		Token:    "any",
	}))
	_ = wsReadRegisteredPayload(t, conn)

	agentDone := make(chan error, 1)
	go func() {
		for {
			msg, err := wsReadRawMessageWithTimeout(conn, 2*time.Second)
			if err != nil {
				agentDone <- err
				return
			}
			if msg.Type != MsgTypeReadFile || msg.Payload == nil {
				continue
			}
			var payload ReadFilePayload
			if err := json.Unmarshal(*msg.Payload, &payload); err != nil {
				agentDone <- err
				return
			}
			agentDone <- conn.WriteJSON(mustNewMessage(t, MsgTypeCommandResult, "", CommandResultPayload{
				RequestID: payload.RequestID,
				Success:   true,
				Stdout:    "data",
				ExitCode:  0,
			}))
			return
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := s.ReadFile(ctx, "a1", ReadFilePayload{RequestID: "read-1", Path: "/etc/hosts"})
	if err != nil || result == nil || result.Stdout != "data" {
		t.Fatalf("unexpected read file result=%v err=%v", result, err)
	}

	if err := <-agentDone; err != nil {
		t.Fatalf("agent error: %v", err)
	}
}

func TestReadFileTimeoutCancelAndSendError(t *testing.T) {
	origTimeout := readFileTimeout
	t.Cleanup(func() { readFileTimeout = origTimeout })
	readFileTimeout = 10 * time.Millisecond

	s := NewServer(nil)
	serverConn, _, cleanup := newConnPair(t)
	defer cleanup()

	ac := &agentConn{
		conn:  serverConn,
		agent: ConnectedAgent{AgentID: "a1"},
		done:  make(chan struct{}),
	}
	s.mu.Lock()
	s.agents["a1"] = ac
	s.mu.Unlock()

	if _, err := s.ReadFile(context.Background(), "a1", ReadFilePayload{
		RequestID: "read-timeout",
		Path:      "/etc/hosts",
	}); err == nil {
		t.Fatalf("expected timeout error")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.ReadFile(ctx, "a1", ReadFilePayload{
		RequestID: "read-cancel",
		Path:      "/etc/hosts",
	}); err == nil {
		t.Fatalf("expected cancel error")
	}

	serverConn.Close()
	if _, err := s.ReadFile(context.Background(), "a1", ReadFilePayload{
		RequestID: "read-send",
		Path:      "/etc/hosts",
	}); err == nil {
		t.Fatalf("expected send error")
	}
}
