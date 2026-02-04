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

func TestLogEndpointsRequireSettingsReadScope(t *testing.T) {
	rawToken := "logs-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/logs/stream",
		"/api/logs/download",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing settings:read scope on %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeSettingsRead) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeSettingsRead, rec.Body.String())
		}
	}
}

func TestLogLevelReadRequiresSettingsReadScope(t *testing.T) {
	rawToken := "log-level-read-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/logs/level", nil)
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

func TestLogLevelUpdateRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "log-level-write-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/logs/level", strings.NewReader(`{"level":"info"}`))
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

func TestUpdateReadEndpointsRequireSettingsReadScope(t *testing.T) {
	rawToken := "updates-read-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/updates/check",
		"/api/updates/status",
		"/api/updates/plan",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing settings:read scope on %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeSettingsRead) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeSettingsRead, rec.Body.String())
		}
	}
}

func TestUpdateApplyRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "updates-write-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/updates/apply", strings.NewReader(`{}`))
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

func TestLicenseMutationsRequireSettingsWriteScope(t *testing.T) {
	rawToken := "license-write-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/license/activate",
		"/api/license/clear",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
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

func TestSetupScriptURLRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "setup-script-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/setup-script-url", strings.NewReader(`{}`))
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

func TestAgentInstallCommandRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "agent-install-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/agent-install-command", strings.NewReader(`{}`))
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

func TestDiscoverRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "discover-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []struct {
		name   string
		method string
		body   string
	}{
		{name: "get", method: http.MethodGet, body: ""},
		{name: "post", method: http.MethodPost, body: `{}`},
	}

	for _, tc := range paths {
		req := httptest.NewRequest(tc.method, "/api/discover", strings.NewReader(tc.body))
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing settings:write scope on %s, got %d", tc.name, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeSettingsWrite) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeSettingsWrite, rec.Body.String())
		}
	}
}

func TestAIOAuthEndpointsRequireSettingsWriteScope(t *testing.T) {
	rawToken := "ai-oauth-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/ai/oauth/start",
		"/api/ai/oauth/exchange",
		"/api/ai/oauth/disconnect",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
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

func TestAIExecuteEndpointsRequireAIExecuteScope(t *testing.T) {
	rawToken := "ai-exec-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIChat}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/ai/execute",
		"/api/ai/execute/stream",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing ai:execute scope on %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeAIExecute) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeAIExecute, rec.Body.String())
		}
	}
}

func TestAIRemediationMutationsRequireAIExecuteScope(t *testing.T) {
	rawToken := "ai-remediate-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIChat}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/ai/remediation/execute",
		"/api/ai/remediation/rollback",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing ai:execute scope on %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeAIExecute) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeAIExecute, rec.Body.String())
		}
	}
}

func TestAIAgentsRequiresAIExecuteScope(t *testing.T) {
	rawToken := "ai-agents-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIChat}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/ai/agents", nil)
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

func TestAICostEndpointsRequireSettingsScopes(t *testing.T) {
	rawToken := "ai-cost-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	// Summary requires settings:read
	req := httptest.NewRequest(http.MethodGet, "/api/ai/cost/summary", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing settings:read scope, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), config.ScopeSettingsRead) {
		t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeSettingsRead, rec.Body.String())
	}

	// Reset requires settings:write
	req = httptest.NewRequest(http.MethodPost, "/api/ai/cost/reset", strings.NewReader(`{}`))
	req.Header.Set("X-API-Token", rawToken)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing settings:write scope, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), config.ScopeSettingsWrite) {
		t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeSettingsWrite, rec.Body.String())
	}

	// Export requires settings:read
	req = httptest.NewRequest(http.MethodGet, "/api/ai/cost/export", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing settings:read scope, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), config.ScopeSettingsRead) {
		t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeSettingsRead, rec.Body.String())
	}
}

func TestAIDebugContextRequiresSettingsReadScope(t *testing.T) {
	rawToken := "ai-debug-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/ai/debug/context", nil)
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

func TestAIRunCommandRequiresAIExecuteScope(t *testing.T) {
	rawToken := "ai-run-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIChat}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/ai/run-command", strings.NewReader(`{}`))
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

func TestAIPatrolRunRequiresAIExecuteScope(t *testing.T) {
	rawToken := "ai-patrol-run-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIChat}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/ai/patrol/run", strings.NewReader(`{}`))
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

func TestAIPatrolAutonomyRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "ai-patrol-autonomy-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/ai/patrol/autonomy", nil)
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

func TestAIExecuteReadEndpointsRequireAIExecuteScope(t *testing.T) {
	rawToken := "ai-exec-read-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIChat}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/ai/patrol/status",
		"/api/ai/patrol/stream",
		"/api/ai/patrol/findings",
		"/api/ai/patrol/history",
		"/api/ai/patrol/runs",
		"/api/ai/patrol/dismissed",
		"/api/ai/patrol/suppressions",
		"/api/ai/approvals",
		"/api/ai/approvals/approval-1",
		"/api/ai/intelligence",
		"/api/ai/intelligence/patterns",
		"/api/ai/intelligence/predictions",
		"/api/ai/intelligence/correlations",
		"/api/ai/intelligence/changes",
		"/api/ai/intelligence/baselines",
		"/api/ai/intelligence/remediations",
		"/api/ai/intelligence/anomalies",
		"/api/ai/intelligence/learning",
		"/api/ai/unified/findings",
		"/api/ai/forecast",
		"/api/ai/forecasts/overview",
		"/api/ai/learning/preferences",
		"/api/ai/proxmox/events",
		"/api/ai/proxmox/correlations",
		"/api/ai/remediation/plans",
		"/api/ai/remediation/plan",
		"/api/ai/circuit/status",
		"/api/ai/incidents",
		"/api/ai/incidents/incident-1",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing ai:execute scope on %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeAIExecute) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeAIExecute, rec.Body.String())
		}
	}
}

func TestAIExecuteMutationEndpointsRequireAIExecuteScope(t *testing.T) {
	rawToken := "ai-exec-mutate-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIChat}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/ai/patrol/acknowledge", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/patrol/dismiss", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/patrol/findings/note", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/patrol/suppress", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/patrol/snooze", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/patrol/resolve", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/patrol/suppressions", body: `{}`},
		{method: http.MethodDelete, path: "/api/ai/patrol/suppressions/rule-1", body: ""},
		{method: http.MethodPost, path: "/api/ai/remediation/approve", body: `{}`},
	}

	for _, tc := range paths {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing ai:execute scope on %s %s, got %d", tc.method, tc.path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeAIExecute) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeAIExecute, rec.Body.String())
		}
	}
}

func TestInfraUpdateReadEndpointsRequireMonitoringReadScope(t *testing.T) {
	rawToken := "infra-read-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/infra-updates",
		"/api/infra-updates/summary",
		"/api/infra-updates/host/host-1",
		"/api/infra-updates/docker:host-1/c1",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing monitoring:read scope on %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeMonitoringRead) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeMonitoringRead, rec.Body.String())
		}
	}
}

func TestInfraUpdateCheckRequiresMonitoringWriteScope(t *testing.T) {
	rawToken := "infra-write-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/infra-updates/check", strings.NewReader(`{}`))
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing monitoring:write scope, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), config.ScopeMonitoringWrite) {
		t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeMonitoringWrite, rec.Body.String())
	}
}

func TestAlertReadEndpointsRequireMonitoringReadScope(t *testing.T) {
	rawToken := "alerts-read-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/alerts/config",
		"/api/alerts/active",
		"/api/alerts/history",
		"/api/alerts/incidents",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing monitoring:read scope on %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeMonitoringRead) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeMonitoringRead, rec.Body.String())
		}
	}
}

func TestAlertMutationEndpointsRequireMonitoringWriteScope(t *testing.T) {
	rawToken := "alerts-write-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPut, path: "/api/alerts/config", body: `{}`},
		{method: http.MethodPost, path: "/api/alerts/activate", body: `{}`},
		{method: http.MethodDelete, path: "/api/alerts/history", body: ""},
		{method: http.MethodPost, path: "/api/alerts/bulk/acknowledge", body: `{}`},
		{method: http.MethodPost, path: "/api/alerts/bulk/clear", body: `{}`},
		{method: http.MethodPost, path: "/api/alerts/acknowledge", body: `{}`},
		{method: http.MethodPost, path: "/api/alerts/unacknowledge", body: `{}`},
		{method: http.MethodPost, path: "/api/alerts/clear", body: `{}`},
		{method: http.MethodPost, path: "/api/alerts/alert-1/acknowledge", body: `{}`},
		{method: http.MethodPost, path: "/api/alerts/alert-1/unacknowledge", body: `{}`},
		{method: http.MethodPost, path: "/api/alerts/alert-1/clear", body: `{}`},
		{method: http.MethodPost, path: "/api/alerts/incidents/note", body: `{}`},
	}

	for _, tc := range paths {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing monitoring:write scope on %s %s, got %d", tc.method, tc.path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeMonitoringWrite) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeMonitoringWrite, rec.Body.String())
		}
	}
}

func TestNotificationQueueStatsRequireSettingsReadScope(t *testing.T) {
	rawToken := "queue-stats-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/queue/stats", nil)
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

func TestConfigSystemRequiresSettingsReadScope(t *testing.T) {
	rawToken := "config-system-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/config/system", nil)
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

func TestSystemSettingsRequiresSettingsReadScope(t *testing.T) {
	rawToken := "system-settings-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/system/settings", nil)
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

func TestSystemSettingsUpdateRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "system-settings-write-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/system/settings/update", strings.NewReader(`{}`))
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

func TestMockModeReadRequiresSettingsReadScope(t *testing.T) {
	rawToken := "mock-mode-read-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/system/mock-mode", nil)
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

func TestMockModeWriteRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "mock-mode-write-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/system/mock-mode", strings.NewReader(`{}`))
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

func TestConfigNodesReadRequiresSettingsReadScope(t *testing.T) {
	rawToken := "config-nodes-read-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/config/nodes", nil)
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

func TestConfigNodesWriteRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "config-nodes-write-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/config/nodes", strings.NewReader(`{}`))
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

func TestConfigNodeMutationsRequireSettingsWriteScope(t *testing.T) {
	rawToken := "config-node-mutate-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/config/nodes/test-config", body: `{}`},
		{method: http.MethodPost, path: "/api/config/nodes/test-connection", body: `{}`},
		{method: http.MethodPut, path: "/api/config/nodes/node-1", body: `{}`},
		{method: http.MethodDelete, path: "/api/config/nodes/node-1", body: ""},
		{method: http.MethodPost, path: "/api/config/nodes/node-1/test", body: `{}`},
		{method: http.MethodPost, path: "/api/config/nodes/node-1/refresh-cluster", body: `{}`},
	}

	for _, tc := range paths {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing settings:write scope on %s %s, got %d", tc.method, tc.path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeSettingsWrite) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeSettingsWrite, rec.Body.String())
		}
	}
}

func TestDiscoveryReadEndpointsRequireMonitoringReadScope(t *testing.T) {
	rawToken := "discovery-read-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/discovery",
		"/api/discovery/status",
		"/api/discovery/info/host-1",
		"/api/discovery/type/pve",
		"/api/discovery/host/host-1",
		"/api/discovery/host/host-1/resource-1",
		"/api/discovery/host/host-1/resource-1/progress",
		"/api/discovery/resource-1",
		"/api/discovery/resource-1/progress",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing monitoring:read scope on %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeMonitoringRead) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeMonitoringRead, rec.Body.String())
		}
	}
}

func TestDiscoveryMutationEndpointsRequireMonitoringWriteScope(t *testing.T) {
	rawToken := "discovery-write-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/discovery/host/host-1/resource-1", body: `{}`},
		{method: http.MethodPut, path: "/api/discovery/host/host-1/resource-1/notes", body: `{}`},
		{method: http.MethodDelete, path: "/api/discovery/host/host-1/resource-1", body: ""},
		{method: http.MethodPost, path: "/api/discovery/resource-1", body: `{}`},
		{method: http.MethodPut, path: "/api/discovery/resource-1/notes", body: `{}`},
		{method: http.MethodDelete, path: "/api/discovery/resource-1", body: ""},
	}

	for _, tc := range paths {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing monitoring:write scope on %s %s, got %d", tc.method, tc.path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeMonitoringWrite) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeMonitoringWrite, rec.Body.String())
		}
	}
}

func TestDiscoverySettingsRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "discovery-settings-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/discovery/settings", strings.NewReader(`{}`))
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

func TestDockerAgentEndpointsRequireDockerReportScope(t *testing.T) {
	rawToken := "docker-report-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/agents/docker/report",
		"/api/agents/docker/commands/command-1",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing docker:report scope on %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeDockerReport) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeDockerReport, rec.Body.String())
		}
	}
}

func TestDockerManageEndpointsRequireDockerManageScope(t *testing.T) {
	rawToken := "docker-manage-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeDockerReport}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/agents/docker/hosts/host-1",
		"/api/agents/docker/containers/update",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing docker:manage scope on %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeDockerManage) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeDockerManage, rec.Body.String())
		}
	}
}

func TestKubernetesAgentEndpointsRequireKubernetesReportScope(t *testing.T) {
	rawToken := "kube-report-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/agents/kubernetes/report", strings.NewReader(`{}`))
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing kubernetes:report scope, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), config.ScopeKubernetesReport) {
		t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeKubernetesReport, rec.Body.String())
	}
}

func TestKubernetesManageEndpointsRequireKubernetesManageScope(t *testing.T) {
	rawToken := "kube-manage-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeKubernetesReport}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/agents/kubernetes/clusters/cluster-1", strings.NewReader(`{}`))
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing kubernetes:manage scope, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), config.ScopeKubernetesManage) {
		t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeKubernetesManage, rec.Body.String())
	}
}

func TestHostAgentEndpointsRequireHostReportScope(t *testing.T) {
	rawToken := "host-report-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/agents/host/report",
		"/api/agents/host/lookup",
		"/api/agents/host/uninstall",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing host:report scope on %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeHostReport) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeHostReport, rec.Body.String())
		}
	}
}

func TestSecurityOIDCRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "security-oidc-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/security/oidc", strings.NewReader(`{}`))
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

func TestUpdateHistoryEndpointsRequireSettingsReadScope(t *testing.T) {
	rawToken := "updates-history-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/updates/history",
		"/api/updates/history/entry",
		"/api/updates/stream",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing settings:read scope on %s, got %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeSettingsRead) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeSettingsRead, rec.Body.String())
		}
	}
}

func TestDiagnosticsRequireSettingsReadScope(t *testing.T) {
	rawToken := "diag-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/diagnostics", nil)
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

func TestDiagnosticsPrepareTokenRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "diag-write-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/diagnostics/docker/prepare-token", strings.NewReader(`{}`))
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
