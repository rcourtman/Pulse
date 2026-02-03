package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pulsews "github.com/rcourtman/pulse-go-rewrite/internal/websocket"
)

type wsRawMessage struct {
	Type    agentexec.MessageType `json:"type"`
	Payload json.RawMessage       `json:"payload,omitempty"`
}

func newTestConfigWithTokens(t *testing.T, records ...config.APITokenRecord) *config.Config {
	t.Helper()
	tempDir := t.TempDir()
	return &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		APITokens:  records,
	}
}

func newTokenRecord(t *testing.T, raw string, scopes []string, metadata map[string]string) config.APITokenRecord {
	t.Helper()
	record, err := config.NewAPITokenRecord(raw, "test-token", scopes)
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	if metadata != nil {
		record.Metadata = metadata
	}
	return *record
}

func readRegisteredPayload(t *testing.T, conn *websocket.Conn) agentexec.RegisteredPayload {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	var msg wsRawMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal message: %v", err)
	}
	if msg.Type != agentexec.MsgTypeRegistered {
		t.Fatalf("message type = %q, want %q", msg.Type, agentexec.MsgTypeRegistered)
	}
	if msg.Payload == nil {
		t.Fatalf("registered payload missing")
	}

	var payload agentexec.RegisteredPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("unmarshal registered payload: %v", err)
	}
	return payload
}

func TestSimpleStatsRequiresAuthInAPIMode(t *testing.T) {
	rawToken := "stats-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/simple-stats", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/simple-stats", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with token, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Simple Pulse Stats") {
		t.Fatalf("expected stats page HTML, got %q", rec.Body.String())
	}
}

func TestSocketIORequiresAuthInAPIMode(t *testing.T) {
	rawToken := "socket-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/socket.io/?transport=polling", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/socket.io/?transport=polling", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with token, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=UTF-8" {
		t.Fatalf("expected text/plain content type, got %q", ct)
	}
	if body := rec.Body.String(); !strings.HasPrefix(body, "0{") {
		t.Fatalf("unexpected polling handshake body: %q", body)
	}
}

func TestSocketIOWebSocketRequiresAuthInAPIMode(t *testing.T) {
	rawToken := "socket-ws-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)

	hub := pulsews.NewHub(nil)
	go hub.Run()
	defer hub.Stop()

	router := NewRouter(cfg, nil, nil, hub, nil, "1.0.0")
	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/socket.io/?transport=websocket"

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		conn.Close()
		t.Fatalf("expected websocket auth failure without token")
	}
	if resp == nil {
		t.Fatalf("expected HTTP response for failed websocket auth")
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing token, got %d", resp.StatusCode)
	}

	headers := http.Header{}
	headers.Set("X-API-Token", rawToken)
	conn, resp, err = websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("expected websocket connection with token, got %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101 switching protocols, got %v", resp)
	}
	conn.Close()
}

func TestWebSocketRequiresMonitoringReadScope(t *testing.T) {
	rawToken := "ws-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeHostReport}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing monitoring:read scope, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), config.ScopeMonitoringRead) {
		t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeMonitoringRead, rec.Body.String())
	}
}

func TestWebSocketRequiresMonitoringReadScopeForUpgrade(t *testing.T) {
	rawToken := "ws-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeHostReport}, nil)
	cfg := newTestConfigWithTokens(t, record)

	hub := pulsews.NewHub(nil)
	go hub.Run()
	defer hub.Stop()

	router := NewRouter(cfg, nil, nil, hub, nil, "1.0.0")
	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	headers := http.Header{}
	headers.Set("X-API-Token", rawToken)
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err == nil {
		conn.Close()
		t.Fatalf("expected websocket upgrade to be rejected without monitoring:read scope")
	}
	if resp == nil {
		t.Fatalf("expected HTTP response for failed websocket upgrade")
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for missing scope, got %d", resp.StatusCode)
	}
}

func TestHostAgentManagementRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "host-manage-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeHostManage}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	cases := []struct {
		name   string
		method string
		path   string
	}{
		{name: "link", method: http.MethodPost, path: "/api/agents/host/link"},
		{name: "unlink", method: http.MethodPost, path: "/api/agents/host/unlink"},
		{name: "delete", method: http.MethodDelete, path: "/api/agents/host/agent-1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("X-API-Token", rawToken)
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403 for missing settings:write scope, got %d", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), config.ScopeSettingsWrite) {
				t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeSettingsWrite, rec.Body.String())
			}
		})
	}
}

func TestTestNotificationRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "notify-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/test-notification", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing settings:write scope, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), config.ScopeSettingsWrite) {
		t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeSettingsWrite, rec.Body.String())
	}
}

func TestAIFindingsRequiresAIExecuteScope(t *testing.T) {
	rawToken := "ai-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIChat}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/f-1/investigation", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing ai:execute scope, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), config.ScopeAIExecute) {
		t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeAIExecute, rec.Body.String())
	}
}

func TestNotificationsDLQRequiresSettingsReadScope(t *testing.T) {
	rawToken := "dlq-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/dlq", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing settings:read scope, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), config.ScopeSettingsRead) {
		t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeSettingsRead, rec.Body.String())
	}
}

func TestNotificationsDLQMutationsRequireSettingsWriteScope(t *testing.T) {
	rawToken := "dlq-write-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	cases := []string{
		"/api/notifications/dlq/retry",
		"/api/notifications/dlq/delete",
	}

	for _, path := range cases {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(`{"id":"test"}`)))
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing settings:write scope on %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeSettingsWrite) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeSettingsWrite, rec.Body.String())
		}
	}
}

func TestAgentExecTokenBindingEnforced(t *testing.T) {
	rawToken := "agent-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentExec}, map[string]string{"bound_agent_id": "agent-1"})
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	wsURL := wsURLForHTTP(ts.URL) + "/api/agent/ws"

	// Mismatched agent ID should be rejected
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if err := conn.WriteJSON(agentexec.Message{
		Type:      agentexec.MsgTypeAgentRegister,
		Timestamp: time.Now(),
		Payload: agentexec.AgentRegisterPayload{
			AgentID:  "agent-2",
			Hostname: "host-2",
			Version:  "1.0.0",
			Platform: "linux",
			Token:    rawToken,
		},
	}); err != nil {
		conn.Close()
		t.Fatalf("WriteJSON: %v", err)
	}
	reg := readRegisteredPayload(t, conn)
	if reg.Success {
		conn.Close()
		t.Fatalf("expected registration to be rejected for mismatched bound agent")
	}
	conn.Close()

	// Matching agent ID should succeed
	conn, _, err = websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if err := conn.WriteJSON(agentexec.Message{
		Type:      agentexec.MsgTypeAgentRegister,
		Timestamp: time.Now(),
		Payload: agentexec.AgentRegisterPayload{
			AgentID:  "agent-1",
			Hostname: "host-1",
			Version:  "1.0.0",
			Platform: "linux",
			Token:    rawToken,
		},
	}); err != nil {
		conn.Close()
		t.Fatalf("WriteJSON: %v", err)
	}
	reg = readRegisteredPayload(t, conn)
	if !reg.Success {
		conn.Close()
		t.Fatalf("expected registration to be accepted for matching bound agent, got %q", reg.Message)
	}
	conn.Close()
}

func TestAgentExecRequiresAgentExecScope(t *testing.T) {
	rawToken := "agent-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIChat}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/agent/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if err := conn.WriteJSON(agentexec.Message{
		Type:      agentexec.MsgTypeAgentRegister,
		Timestamp: time.Now(),
		Payload: agentexec.AgentRegisterPayload{
			AgentID:  "agent-1",
			Hostname: "host-1",
			Version:  "1.0.0",
			Platform: "linux",
			Token:    rawToken,
		},
	}); err != nil {
		conn.Close()
		t.Fatalf("WriteJSON: %v", err)
	}
	reg := readRegisteredPayload(t, conn)
	if reg.Success {
		conn.Close()
		t.Fatalf("expected registration to be rejected without agent:exec scope")
	}
	conn.Close()
}

func TestWebSocketAllowsMonitoringReadScope(t *testing.T) {
	rawToken := "ws-allow-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)

	hub := pulsews.NewHub(nil)
	go hub.Run()
	defer hub.Stop()

	router := NewRouter(cfg, nil, nil, hub, nil, "1.0.0")
	ts := httptest.NewServer(router.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	headers := http.Header{}
	headers.Set("X-API-Token", rawToken)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	conn.Close()
}
