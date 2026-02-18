package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
	pulsews "github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type wsRawMessage struct {
	Type    agentexec.MessageType `json:"type"`
	Payload json.RawMessage       `json:"payload,omitempty"`
}

type denyAuthorizer struct{}

func (d *denyAuthorizer) Authorize(_ context.Context, _ string, _ string) (bool, error) {
	return false, nil
}

type adminOnlyAuthorizer struct{}

func (a *adminOnlyAuthorizer) Authorize(ctx context.Context, _ string, _ string) (bool, error) {
	return auth.GetUser(ctx) == "admin", nil
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
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
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

func TestSimpleStatsAllowsBearerToken(t *testing.T) {
	rawToken := "stats-bearer-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/simple-stats", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with bearer token, got %d", rec.Code)
	}
	if rec.Header().Get("X-Auth-Method") != "api_token" {
		t.Fatalf("expected X-Auth-Method api_token, got %q", rec.Header().Get("X-Auth-Method"))
	}
}

func TestSimpleStatsRejectsInvalidBearerToken(t *testing.T) {
	rawToken := "stats-bearer-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/simple-stats", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid bearer token, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Invalid API token") {
		t.Fatalf("expected invalid token response, got %q", rec.Body.String())
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

func TestSocketIORequiresMonitoringReadScope(t *testing.T) {
	rawToken := "socket-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/socket.io/?transport=polling", nil)
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

func TestSocketIOJSRequiresAuth(t *testing.T) {
	rawToken := "socket-js-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/socket.io/socket.io.js", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/socket.io/socket.io.js", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect with token, got %d", rec.Code)
	}
	if location := rec.Header().Get("Location"); !strings.Contains(location, "socket.io.min.js") {
		t.Fatalf("expected CDN redirect, got %q", location)
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
	ts := newIPv4HTTPServer(t, router.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/socket.io/?transport=websocket&org_id=default"

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

func TestSocketIOWebSocketAllowsQueryToken(t *testing.T) {
	rawToken := "socket-ws-query-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)

	hub := pulsews.NewHub(nil)
	go hub.Run()
	defer hub.Stop()

	router := NewRouter(cfg, nil, nil, hub, nil, "1.0.0")
	ts := newIPv4HTTPServer(t, router.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/socket.io/?transport=websocket&org_id=default&token=" + rawToken
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("expected websocket connection with query token, got %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		t.Fatalf("expected 101 switching protocols, got %v", resp)
	}
	conn.Close()
}

func TestSocketIOPollingIgnoresQueryToken(t *testing.T) {
	rawToken := "socket-polling-query-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/socket.io/?transport=polling&token="+rawToken, nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when token is only in query string, got %d", rec.Code)
	}
}

func TestSchedulerHealthRequiresAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "sched-token-123.12345678", []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/monitoring/scheduler/health", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}
}

func TestChangePasswordRequiresAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "change-pass-token-123.12345678", []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/security/change-password", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestChangePasswordRejectsProxyNonAdmin(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/security/change-password", strings.NewReader(`{}`))
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set("X-Remote-User", "viewer-user")
	req.Header.Set("X-Remote-Roles", "viewer")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin proxy password change, got %d", rec.Code)
	}
}

func TestResetLockoutRequiresAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "reset-lockout-token-123.12345678", []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/security/reset-lockout", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestRequirePermissionDeniesProxyNonAdminUsers(t *testing.T) {
	prevAuthorizer := auth.GetAuthorizer()
	auth.SetAuthorizer(&adminOnlyAuthorizer{})
	defer auth.SetAuthorizer(prevAuthorizer)

	cfg := newTestConfigWithTokens(t)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/security/tokens", nil)
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set("X-Remote-User", "viewer-user")
	req.Header.Set("X-Remote-Roles", "viewer")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin proxy user, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/security/tokens", nil)
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set("X-Remote-User", "admin")
	req.Header.Set("X-Remote-Roles", "admin")
	rec = httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin proxy user, got %d", rec.Code)
	}
}

func TestLicenseFeaturesRequiresAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "license-token-123.12345678", []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/license/features", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}
}

func TestLicenseStatusRequiresAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "license-status-token-123.12345678", []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/license/status", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}
}

func TestAIStatusRequiresAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "ai-status-token-123.12345678", []string{config.ScopeAIChat}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/ai/status", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}
}

func TestAIStatusRequiresAIChatScope(t *testing.T) {
	rawToken := "ai-status-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/ai/status", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing ai:chat scope, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), config.ScopeAIChat) {
		t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeAIChat, rec.Body.String())
	}
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
	ts := newIPv4HTTPServer(t, router.Handler())
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

	ts := newIPv4HTTPServer(t, router.Handler())
	defer ts.Close()

	wsURL := wsURLForHTTP(ts.URL) + "/api/agent/ws"

	// Mismatched agent ID should be rejected
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	regMsg, err := agentexec.NewMessage(agentexec.MsgTypeAgentRegister, "", agentexec.AgentRegisterPayload{
		AgentID:  "agent-2",
		Hostname: "host-2",
		Version:  "1.0.0",
		Platform: "linux",
		Token:    rawToken,
	})
	if err != nil {
		conn.Close()
		t.Fatalf("NewMessage: %v", err)
	}
	if err := conn.WriteJSON(regMsg); err != nil {
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
	regMsg, err = agentexec.NewMessage(agentexec.MsgTypeAgentRegister, "", agentexec.AgentRegisterPayload{
		AgentID:  "agent-1",
		Hostname: "host-1",
		Version:  "1.0.0",
		Platform: "linux",
		Token:    rawToken,
	})
	if err != nil {
		conn.Close()
		t.Fatalf("NewMessage: %v", err)
	}
	if err := conn.WriteJSON(regMsg); err != nil {
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

	ts := newIPv4HTTPServer(t, router.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/agent/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	regMsg, err := agentexec.NewMessage(agentexec.MsgTypeAgentRegister, "", agentexec.AgentRegisterPayload{
		AgentID:  "agent-1",
		Hostname: "host-1",
		Version:  "1.0.0",
		Platform: "linux",
		Token:    rawToken,
	})
	if err != nil {
		conn.Close()
		t.Fatalf("NewMessage: %v", err)
	}
	if err := conn.WriteJSON(regMsg); err != nil {
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
	ts := newIPv4HTTPServer(t, router.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws?org_id=default"
	headers := http.Header{}
	headers.Set("X-API-Token", rawToken)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	conn.Close()
}

func TestWebSocketAllowsBearerToken(t *testing.T) {
	rawToken := "ws-bearer-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)

	hub := pulsews.NewHub(nil)
	go hub.Run()
	defer hub.Stop()

	router := NewRouter(cfg, nil, nil, hub, nil, "1.0.0")
	ts := newIPv4HTTPServer(t, router.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws?org_id=default"
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+rawToken)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	conn.Close()
}

func TestWebSocketAllowsTokenQueryParam(t *testing.T) {
	rawToken := "ws-query-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)

	hub := pulsews.NewHub(nil)
	go hub.Run()
	defer hub.Stop()

	router := NewRouter(cfg, nil, nil, hub, nil, "1.0.0")
	ts := newIPv4HTTPServer(t, router.Handler())
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws?org_id=default&token=" + rawToken
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		t.Fatalf("expected 101 switching protocols, got %v", resp)
	}
	conn.Close()
}

func TestQueryTokenIgnoredForHTTPRequests(t *testing.T) {
	rawToken := "query-token-ignored-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/config?token="+rawToken, nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when token is only in query string, got %d", rec.Code)
	}
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

func TestLogEndpointsRequireAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "log-auth-token-123.12345678", []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/logs/stream",
		"/api/logs/download",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 without auth on %s, got %d", path, rec.Code)
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

func TestUpdateStatusRequiresAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "update-auth-token-123.12345678", []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/updates/status", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
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

func TestAIPatrolAutonomyUpdateRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "ai-patrol-autonomy-update-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPut, "/api/ai/patrol/autonomy", strings.NewReader(`{}`))
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
		{method: http.MethodPost, path: "/api/ai/findings/f-1/reapprove", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/approvals/approval-1/approve", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/approvals/approval-1/deny", body: `{}`},
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

func TestAlertMutationEndpointsAllowMonitoringWriteWithoutReadScope(t *testing.T) {
	rawToken := "alerts-write-only-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/alerts/acknowledge", strings.NewReader(`{}`))
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("expected write-only token to reach alert mutation handler, got 403 body=%q", rec.Body.String())
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected validation failure (400) after passing scope checks, got %d body=%q", rec.Code, rec.Body.String())
	}

	readReq := httptest.NewRequest(http.MethodGet, "/api/alerts/config", nil)
	readReq.Header.Set("X-API-Token", rawToken)
	readRec := httptest.NewRecorder()
	router.Handler().ServeHTTP(readRec, readReq)
	if readRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for read endpoint with write-only token, got %d", readRec.Code)
	}
	if !strings.Contains(readRec.Body.String(), config.ScopeMonitoringRead) {
		t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeMonitoringRead, readRec.Body.String())
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

func TestConfigExportRequiresSettingsReadScope(t *testing.T) {
	rawToken := "config-export-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/config/export", strings.NewReader(`{}`))
	req.RemoteAddr = "127.0.0.1:1234"
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

func TestConfigImportRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "config-import-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(`{}`))
	req.RemoteAddr = "127.0.0.1:1234"
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

func TestConfigExportRejectsShortPassphrase(t *testing.T) {
	rawToken := "config-export-pass-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/config/export", strings.NewReader(`{"passphrase":"short"}`))
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for short passphrase, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Passphrase must be at least 12 characters") {
		t.Fatalf("expected passphrase length error, got %q", rec.Body.String())
	}
}

func TestConfigExportRequiresPassphrase(t *testing.T) {
	rawToken := "config-export-missing-pass-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/config/export", strings.NewReader(`{"passphrase":""}`))
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing passphrase, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Passphrase is required") {
		t.Fatalf("expected passphrase required error, got %q", rec.Body.String())
	}
}

func TestConfigImportRejectsMissingData(t *testing.T) {
	rawToken := "config-import-data-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(`{"passphrase":"long-enough-passphrase","data":""}`))
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing data, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Import data is required") {
		t.Fatalf("expected import data error, got %q", rec.Body.String())
	}
}

func TestConfigImportRequiresPassphrase(t *testing.T) {
	rawToken := "config-import-pass-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(`{"passphrase":"","data":"encrypted"}`))
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing passphrase, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Passphrase is required") {
		t.Fatalf("expected passphrase required error, got %q", rec.Body.String())
	}
}

func TestConfigExportRequiresAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "config-export-auth-token", []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/config/export", strings.NewReader(`{}`))
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestConfigImportRequiresAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "config-import-auth-token", []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(`{}`))
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestConfigExportBlocksPublicNetworkWithoutAuth(t *testing.T) {
	cfg := &config.Config{
		DataPath:   t.TempDir(),
		ConfigPath: t.TempDir(),
	}
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/config/export", strings.NewReader(`{}`))
	req.RemoteAddr = "203.0.113.10:1234"
	ResetRateLimitForIP("203.0.113.10")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for public network without auth, got %d", rec.Code)
	}
}

func TestConfigImportBlocksPublicNetworkWithoutAuth(t *testing.T) {
	cfg := &config.Config{
		DataPath:   t.TempDir(),
		ConfigPath: t.TempDir(),
	}
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(`{}`))
	req.RemoteAddr = "203.0.113.11:1234"
	ResetRateLimitForIP("203.0.113.11")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for public network without auth, got %d", rec.Code)
	}
}

func TestAutoRegisterRequiresAuth(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	router.configHandlers.SetConfig(cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestAutoRegisterRejectsTokenMissingRequiredScope(t *testing.T) {
	rawToken := "auto-register-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	router.configHandlers.SetConfig(cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", strings.NewReader(`{}`))
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing required scope, got %d", rec.Code)
	}
}

func TestAutoRegisterAcceptsAgentToken(t *testing.T) {
	rawToken := "agent-register-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeHostReport}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	router.configHandlers.SetConfig(cfg)

	body := `{"type":"pve","host":"https://192.168.1.1:8006","tokenId":"test@pam!pulse","tokenValue":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", strings.NewReader(body))
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	// Should not be 401 — the agent token has host-agent:report which is accepted
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("expected agent token with host-agent:report to be accepted, got 401")
	}
}

func TestConfigExportRequiresProxyAdmin(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/config/export", strings.NewReader(`{}`))
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set("X-Remote-User", "viewer-user")
	req.Header.Set("X-Remote-Roles", "viewer")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin proxy user, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Admin privileges required") {
		t.Fatalf("expected admin privilege error, got %q", rec.Body.String())
	}
}

func TestConfigImportRequiresProxyAdmin(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(`{}`))
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set("X-Remote-User", "viewer-user")
	req.Header.Set("X-Remote-Roles", "viewer")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin proxy user, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Admin privileges required") {
		t.Fatalf("expected admin privilege error, got %q", rec.Body.String())
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

func TestDiscoverySettingsRejectsProxyNonAdmin(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	service := servicediscovery.NewService(nil, nil, nil, servicediscovery.DefaultConfig())
	router.SetDiscoveryService(service)

	req := httptest.NewRequest(http.MethodPost, "/api/discovery/settings", strings.NewReader(`{"max_discovery_age_days":5}`))
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set("X-Remote-User", "viewer-user")
	req.Header.Set("X-Remote-Roles", "viewer")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin proxy discovery settings, got %d", rec.Code)
	}
}

func TestDiscoveryNotesRejectsProxyUserSecrets(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	service := servicediscovery.NewService(nil, nil, nil, servicediscovery.DefaultConfig())
	router.SetDiscoveryService(service)

	payload := `{"user_notes":"note","user_secrets":{"token":"abc"}}`
	req := httptest.NewRequest(http.MethodPut, "/api/discovery/host/host-1/resource-1/notes", strings.NewReader(payload))
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set("X-Remote-User", "viewer-user")
	req.Header.Set("X-Remote-Roles", "viewer")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin proxy discovery secrets, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "user_secrets") {
		t.Fatalf("expected user_secrets error, got %q", rec.Body.String())
	}
}

func TestNotificationsRequireProxyAdmin(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/notifications/queue/stats", nil)
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set("X-Remote-User", "viewer-user")
	req.Header.Set("X-Remote-Roles", "viewer")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin proxy user, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Admin privileges required") {
		t.Fatalf("expected admin privilege error, got %q", rec.Body.String())
	}
}

func TestProxyAuthNonAdminDeniedAdminEndpoints(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	router.aiSettingsHandler.legacyConfig = cfg

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/logs/stream", body: ""},
		{method: http.MethodGet, path: "/api/logs/download", body: ""},
		{method: http.MethodGet, path: "/api/logs/level", body: ""},
		{method: http.MethodPost, path: "/api/logs/level", body: `{}`},
		{method: http.MethodGet, path: "/api/updates/check", body: ""},
		{method: http.MethodPost, path: "/api/updates/apply", body: `{}`},
		{method: http.MethodGet, path: "/api/updates/status", body: ""},
		{method: http.MethodGet, path: "/api/updates/stream", body: ""},
		{method: http.MethodGet, path: "/api/updates/plan", body: ""},
		{method: http.MethodGet, path: "/api/updates/history", body: ""},
		{method: http.MethodGet, path: "/api/updates/history/entry", body: ""},
		{method: http.MethodGet, path: "/api/diagnostics", body: ""},
		{method: http.MethodPost, path: "/api/diagnostics/docker/prepare-token", body: `{}`},
		{method: http.MethodGet, path: "/api/config/system", body: ""},
		{method: http.MethodPost, path: "/api/config/export", body: `{}`},
		{method: http.MethodPost, path: "/api/config/import", body: `{}`},
		{method: http.MethodGet, path: "/api/config/nodes", body: ""},
		{method: http.MethodPost, path: "/api/config/nodes", body: `{}`},
		{method: http.MethodPost, path: "/api/config/nodes/test-config", body: `{}`},
		{method: http.MethodPost, path: "/api/config/nodes/test-connection", body: `{}`},
		{method: http.MethodPut, path: "/api/config/nodes/node-1", body: `{}`},
		{method: http.MethodDelete, path: "/api/config/nodes/node-1", body: ``},
		{method: http.MethodPost, path: "/api/config/nodes/node-1/test", body: `{}`},
		{method: http.MethodPost, path: "/api/config/nodes/node-1/refresh-cluster", body: `{}`},
		{method: http.MethodGet, path: "/api/system/settings", body: ""},
		{method: http.MethodPost, path: "/api/system/settings/update", body: `{}`},
		{method: http.MethodPost, path: "/api/security/reset-lockout", body: `{}`},
		{method: http.MethodPost, path: "/api/security/apply-restart", body: `{}`},
		{method: http.MethodGet, path: "/api/security/tokens", body: ``},
		{method: http.MethodDelete, path: "/api/security/tokens/token-1", body: ``},
		{method: http.MethodPost, path: "/api/security/regenerate-token", body: `{}`},
		{method: http.MethodPost, path: "/api/security/validate-token", body: `{"token":"abc"}`},
		{method: http.MethodPost, path: "/api/security/oidc", body: `{}`},
		{method: http.MethodPost, path: "/api/system/verify-temperature-ssh", body: `{}`},
		{method: http.MethodPost, path: "/api/system/ssh-config", body: `{}`},
		{method: http.MethodGet, path: "/api/audit", body: ""},
		{method: http.MethodGet, path: "/api/admin/roles", body: ""},
		{method: http.MethodGet, path: "/api/admin/users", body: ""},
		{method: http.MethodGet, path: "/api/admin/reports/generate", body: ""},
		{method: http.MethodGet, path: "/api/admin/webhooks/audit", body: ""},
		{method: http.MethodGet, path: "/api/settings/ai", body: ""},
		{method: http.MethodGet, path: "/api/ai/debug/context", body: ""},
		{method: http.MethodPost, path: "/api/ai/execute", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/execute/stream", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/kubernetes/analyze", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/investigate-alert", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/run-command", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/remediation/execute", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/remediation/rollback", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/patrol/run", body: `{}`},
		{method: http.MethodGet, path: "/api/ai/patrol/autonomy", body: ""},
		{method: http.MethodPost, path: "/api/ai/cost/reset", body: `{}`},
		{method: http.MethodGet, path: "/api/ai/cost/export", body: ""},
		{method: http.MethodPost, path: "/api/ai/oauth/start", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/oauth/exchange", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/oauth/disconnect", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/test", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/test/openai", body: `{}`},
		{method: http.MethodPost, path: "/api/agents/docker/containers/update", body: `{}`},
		{method: http.MethodPost, path: "/api/agents/docker/hosts/host-1/update-all", body: ``},
		{method: http.MethodDelete, path: "/api/agents/docker/hosts/host-1", body: ``},
		{method: http.MethodDelete, path: "/api/agents/kubernetes/clusters/cluster-1", body: ``},
		{method: http.MethodPost, path: "/api/agents/host/link", body: `{}`},
		{method: http.MethodPost, path: "/api/agents/host/unlink", body: `{}`},
		{method: http.MethodPatch, path: "/api/agents/host/host-1/config", body: `{}`},
		{method: http.MethodDelete, path: "/api/agents/host/agent-1", body: ``},
		{method: http.MethodGet, path: "/api/admin/profiles/", body: ""},
		{method: http.MethodPost, path: "/api/agent-install-command", body: `{}`},
		{method: http.MethodPost, path: "/api/setup-script-url", body: `{}`},
		{method: http.MethodPost, path: "/api/test-notification", body: `{}`},
		{method: http.MethodGet, path: "/api/discover", body: ""},
		{method: http.MethodPost, path: "/api/license/activate", body: `{}`},
		{method: http.MethodPost, path: "/api/license/clear", body: `{}`},
		{method: http.MethodGet, path: "/api/license/status", body: ""},
		{method: http.MethodGet, path: "/api/notifications/queue/stats", body: ""},
		{method: http.MethodGet, path: "/api/notifications/", body: ""},
		{method: http.MethodGet, path: "/api/notifications/dlq", body: ""},
		{method: http.MethodPost, path: "/api/notifications/dlq/retry", body: `{}`},
		{method: http.MethodPost, path: "/api/notifications/dlq/delete", body: `{}`},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
		req.Header.Set("X-Remote-User", "viewer-user")
		req.Header.Set("X-Remote-Roles", "viewer")
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for non-admin proxy user on %s %s, got %d", tc.method, tc.path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Admin privileges required") {
			t.Fatalf("expected admin privilege error on %s %s, got %q", tc.method, tc.path, rec.Body.String())
		}
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

func TestHostAgentConfigPatchRequiresHostManageScope(t *testing.T) {
	rawToken := "host-config-manage-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeHostConfigRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPatch, "/api/agents/host/host-1/config", strings.NewReader(`{}`))
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for missing host:manage scope, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), config.ScopeHostManage) {
		t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeHostManage, rec.Body.String())
	}
}

func TestMonitoringReadEndpointsRequireMonitoringReadScope(t *testing.T) {
	rawToken := "monitoring-read-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/config",
		"/api/storage/host-1",
		"/api/storage-charts",
		"/api/charts",
		"/api/charts/workloads",
		"/api/metrics-store/stats",
		"/api/metrics-store/history",
		"/api/guests/metadata",
		"/api/guests/metadata/guest-1",
		"/api/docker/metadata",
		"/api/docker/metadata/container-1",
		"/api/docker/hosts/metadata",
		"/api/docker/hosts/metadata/host-1",
		"/api/hosts/metadata",
		"/api/hosts/metadata/host-1",
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

func TestMetadataMutationEndpointsRequireMonitoringWriteScope(t *testing.T) {
	rawToken := "metadata-write-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/guests/metadata/guest-1", body: `{}`},
		{method: http.MethodPut, path: "/api/guests/metadata/guest-1", body: `{}`},
		{method: http.MethodDelete, path: "/api/guests/metadata/guest-1", body: ""},
		{method: http.MethodPost, path: "/api/docker/metadata/container-1", body: `{}`},
		{method: http.MethodPut, path: "/api/docker/metadata/container-1", body: `{}`},
		{method: http.MethodDelete, path: "/api/docker/metadata/container-1", body: ""},
		{method: http.MethodPost, path: "/api/docker/hosts/metadata/host-1", body: `{}`},
		{method: http.MethodPut, path: "/api/docker/hosts/metadata/host-1", body: `{}`},
		{method: http.MethodDelete, path: "/api/docker/hosts/metadata/host-1", body: ""},
		{method: http.MethodPost, path: "/api/hosts/metadata/host-1", body: `{}`},
		{method: http.MethodPut, path: "/api/hosts/metadata/host-1", body: `{}`},
		{method: http.MethodDelete, path: "/api/hosts/metadata/host-1", body: ""},
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

func TestAISettingsReadRequiresSettingsReadScope(t *testing.T) {
	rawToken := "ai-settings-read-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/settings/ai", nil)
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

func TestAISettingsWriteRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "ai-settings-write-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/settings/ai/update",
		"/api/ai/test",
		"/api/ai/test/openai",
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

func TestAISettingsUpdateRejectsProxyNonAdmin(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	router.aiSettingsHandler.legacyConfig = cfg

	req := httptest.NewRequest(http.MethodPost, "/api/settings/ai/update", strings.NewReader(`{}`))
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set("X-Remote-User", "viewer-user")
	req.Header.Set("X-Remote-Roles", "viewer")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin proxy AI settings update, got %d", rec.Code)
	}
}

func TestAITestConnectionRejectsProxyNonAdmin(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	router.aiSettingsHandler.legacyConfig = cfg

	req := httptest.NewRequest(http.MethodPost, "/api/ai/test", strings.NewReader(`{}`))
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set("X-Remote-User", "viewer-user")
	req.Header.Set("X-Remote-Roles", "viewer")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin proxy AI test, got %d", rec.Code)
	}
}

func TestAITestProviderRejectsProxyNonAdmin(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	router.aiSettingsHandler.legacyConfig = cfg

	req := httptest.NewRequest(http.MethodPost, "/api/ai/test/openai", strings.NewReader(`{}`))
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set("X-Remote-User", "viewer-user")
	req.Header.Set("X-Remote-Roles", "viewer")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin proxy AI provider test, got %d", rec.Code)
	}
}

func TestAIChatEndpointsRequireAIChatScope(t *testing.T) {
	rawToken := "ai-chat-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/ai/models", body: ""},
		{method: http.MethodPost, path: "/api/ai/chat", body: `{}`},
		{method: http.MethodGet, path: "/api/ai/sessions", body: ""},
		{method: http.MethodGet, path: "/api/ai/sessions/session-1", body: ""},
		{method: http.MethodGet, path: "/api/ai/chat/sessions", body: ""},
		{method: http.MethodGet, path: "/api/ai/chat/sessions/session-1", body: ""},
		{method: http.MethodGet, path: "/api/ai/question/q-1", body: ""},
		{method: http.MethodGet, path: "/api/ai/knowledge", body: ""},
		{method: http.MethodPost, path: "/api/ai/knowledge/save", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/knowledge/delete", body: `{}`},
		{method: http.MethodGet, path: "/api/ai/knowledge/export", body: ""},
		{method: http.MethodPost, path: "/api/ai/knowledge/import", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/knowledge/clear", body: `{}`},
	}

	for _, tc := range paths {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing ai:chat scope on %s %s, got %d", tc.method, tc.path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), config.ScopeAIChat) {
			t.Fatalf("expected missing scope response to mention %q, got %q", config.ScopeAIChat, rec.Body.String())
		}
	}
}

func TestAuditEndpointsRequireLicenseFeature(t *testing.T) {
	rawToken := "audit-license-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 for missing audit logging license, got %d", rec.Code)
	}
}

func TestAuditVerifyRequiresLicenseFeature(t *testing.T) {
	rawToken := "audit-verify-license-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/audit/event-1/verify", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 for missing audit logging license, got %d", rec.Code)
	}
}

func TestReportingEndpointsRequireLicenseFeature(t *testing.T) {
	rawToken := "reporting-license-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/admin/reports/generate",
		"/api/admin/reports/generate-multi",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusPaymentRequired {
			t.Fatalf("expected 402 for missing reporting license on %s, got %d", path, rec.Code)
		}
	}
}

func TestRBACEndpointsRequireLicenseFeature(t *testing.T) {
	rawToken := "rbac-license-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/admin/roles",
		"/api/admin/users",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusPaymentRequired {
			t.Fatalf("expected 402 for missing RBAC license on %s, got %d", path, rec.Code)
		}
	}
}

func TestRBACMutationsRequireLicenseFeature(t *testing.T) {
	rawToken := "rbac-license-mutation-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/admin/roles", body: `{"id":"role-1","name":"Role 1"}`},
		{method: http.MethodPut, path: "/api/admin/roles/role-1", body: `{"id":"role-1","name":"Role 1"}`},
		{method: http.MethodDelete, path: "/api/admin/roles/role-1", body: ``},
		{method: http.MethodPut, path: "/api/admin/users/alice/roles", body: `{"roleIds":["role-1"]}`},
		{method: http.MethodPost, path: "/api/admin/users/alice/roles", body: `{"roleIds":["role-1"]}`},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusPaymentRequired {
			t.Fatalf("expected 402 for missing RBAC license on %s %s, got %d", tc.method, tc.path, rec.Code)
		}
	}
}

func TestAuditWebhookRequiresLicenseFeature(t *testing.T) {
	rawToken := "audit-webhook-license-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/webhooks/audit", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 for missing audit logging license, got %d", rec.Code)
	}
}

func TestSecurityTokensReadRequiresSettingsReadScope(t *testing.T) {
	rawToken := "security-tokens-read-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/security/tokens", nil)
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

func TestSecurityTokensWriteRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "security-tokens-write-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/security/tokens",
		"/api/security/tokens/token-1",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		if strings.Contains(path, "/token-") {
			req = httptest.NewRequest(http.MethodDelete, path, nil)
		}
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

func TestAgentProfilesRequireLicenseFeature(t *testing.T) {
	rawToken := "profiles-license-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/profiles/", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 for missing agent profiles license, got %d", rec.Code)
	}
}

func TestAILicensedEndpointsRequireLicenseFeature(t *testing.T) {
	rawToken := "ai-license-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIExecute}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/ai/kubernetes/analyze",
		"/api/ai/investigate-alert",
		"/api/ai/findings/f-1/reinvestigate",
		"/api/ai/findings/f-1/reapprove",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusPaymentRequired {
			t.Fatalf("expected 402 for missing AI license on %s, got %d", path, rec.Code)
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

func TestPermissionProtectedEndpointsDenyWhenAuthorizerBlocks(t *testing.T) {
	prevAuthorizer := auth.GetAuthorizer()
	auth.SetAuthorizer(&denyAuthorizer{})
	defer auth.SetAuthorizer(prevAuthorizer)

	rawToken := "perm-deny-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/audit", body: ""},
		{method: http.MethodGet, path: "/api/audit/event-1/verify", body: ""},
		{method: http.MethodGet, path: "/api/admin/roles", body: ""},
		{method: http.MethodGet, path: "/api/admin/roles/", body: ""},
		{method: http.MethodPost, path: "/api/admin/roles", body: `{"id":"role-1","name":"Role 1"}`},
		{method: http.MethodPut, path: "/api/admin/roles/role-1", body: `{"id":"role-1","name":"Role 1"}`},
		{method: http.MethodDelete, path: "/api/admin/roles/role-1", body: ""},
		{method: http.MethodGet, path: "/api/admin/users", body: ""},
		{method: http.MethodGet, path: "/api/admin/users/", body: ""},
		{method: http.MethodPut, path: "/api/admin/users/alice/roles", body: `{"roleIds":["role-1"]}`},
		{method: http.MethodPost, path: "/api/admin/users/alice/roles", body: `{"roleIds":["role-1"]}`},
		{method: http.MethodGet, path: "/api/admin/users/alice/permissions", body: ""},
		{method: http.MethodGet, path: "/api/admin/reports/generate", body: ""},
		{method: http.MethodPost, path: "/api/admin/reports/generate-multi", body: `{}`},
		{method: http.MethodGet, path: "/api/admin/webhooks/audit", body: ""},
		{method: http.MethodGet, path: "/api/security/tokens", body: ""},
		{method: http.MethodDelete, path: "/api/security/tokens/token-1", body: ""},
		{method: http.MethodGet, path: "/api/settings/ai", body: ""},
		{method: http.MethodPost, path: "/api/settings/ai/update", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/test", body: `{}`},
		{method: http.MethodPost, path: "/api/ai/test/openai", body: `{}`},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for permission denial on %s %s, got %d", tc.method, tc.path, rec.Code)
		}
	}
}

func TestPermissionEndpointsRejectProxyNonAdmin(t *testing.T) {
	prevAuthorizer := auth.GetAuthorizer()
	auth.SetAuthorizer(&auth.DefaultAuthorizer{})
	defer auth.SetAuthorizer(prevAuthorizer)

	cfg := newTestConfigWithTokens(t)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/security/tokens", nil)
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set("X-Remote-User", "viewer-user")
	req.Header.Set("X-Remote-Roles", "viewer")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin proxy on permissioned endpoint, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Admin privileges required") {
		t.Fatalf("expected admin privilege error, got %q", rec.Body.String())
	}
}

func TestApplyRestartRequiresAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "apply-restart-token-123.12345678", []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/security/apply-restart", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestApplyRestartRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "apply-restart-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/security/apply-restart", nil)
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

func TestApplyRestartRequiresProxyAdmin(t *testing.T) {
	record := newTokenRecord(t, "apply-restart-proxy-token-123.12345678", []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/security/apply-restart", nil)
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set("X-Remote-User", "viewer-user")
	req.Header.Set("X-Remote-Roles", "viewer")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin proxy user, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Admin privileges required") {
		t.Fatalf("expected admin privilege error, got %q", rec.Body.String())
	}
}

func TestVerifyTemperatureSSHRequiresAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "verify-ssh-token-123.12345678", []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/system/verify-temperature-ssh", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestVerifyTemperatureSSHRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "verify-ssh-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/system/verify-temperature-ssh", strings.NewReader(`{}`))
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

func TestSSHConfigRequiresAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "ssh-config-token-123.12345678", []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/system/ssh-config", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestSSHConfigRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "ssh-config-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/system/ssh-config", strings.NewReader(`{}`))
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

func TestQuickSetupRequiresAuthWhenConfigured(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed-password"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.20")
	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(`{"username":"admin","password":"Password!1"}`))
	req.RemoteAddr = "203.0.113.20:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestQuickSetupRejectsProxyNonAdmin(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed-password"
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.27")
	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(`{}`))
	req.RemoteAddr = "203.0.113.27:1234"
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set("X-Remote-User", "viewer-user")
	req.Header.Set("X-Remote-Roles", "viewer")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin proxy quick setup, got %d", rec.Code)
	}
}

func TestRegenerateTokenRequiresAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "regen-token-123.12345678", []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.21")
	req := httptest.NewRequest(http.MethodPost, "/api/security/regenerate-token", nil)
	req.RemoteAddr = "203.0.113.21:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestRegenerateTokenRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "regen-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.22")
	req := httptest.NewRequest(http.MethodPost, "/api/security/regenerate-token", nil)
	req.RemoteAddr = "203.0.113.22:1234"
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

func TestRegenerateTokenRejectsProxyNonAdmin(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.25")
	req := httptest.NewRequest(http.MethodPost, "/api/security/regenerate-token", nil)
	req.RemoteAddr = "203.0.113.25:1234"
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set("X-Remote-User", "viewer-user")
	req.Header.Set("X-Remote-Roles", "viewer")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin proxy regenerate-token, got %d", rec.Code)
	}
}

func TestValidateTokenRequiresAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "validate-token-123.12345678", []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.23")
	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-token", strings.NewReader(`{"token":"abc"}`))
	req.RemoteAddr = "203.0.113.23:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestValidateTokenRequiresSettingsWriteScope(t *testing.T) {
	rawToken := "validate-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.24")
	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-token", strings.NewReader(`{"token":"abc"}`))
	req.RemoteAddr = "203.0.113.24:1234"
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

func TestValidateTokenRejectsProxyNonAdmin(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.ProxyAuthSecret = "proxy-secret"
	cfg.ProxyAuthUserHeader = "X-Remote-User"
	cfg.ProxyAuthRoleHeader = "X-Remote-Roles"
	cfg.ProxyAuthAdminRole = "admin"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.26")
	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-token", strings.NewReader(`{"token":"abc"}`))
	req.RemoteAddr = "203.0.113.26:1234"
	req.Header.Set("X-Proxy-Secret", cfg.ProxyAuthSecret)
	req.Header.Set("X-Remote-User", "viewer-user")
	req.Header.Set("X-Remote-Roles", "viewer")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin proxy validate-token, got %d", rec.Code)
	}
}

func TestRecoveryEndpointRejectsRemoteWithoutToken(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.30")
	req := httptest.NewRequest(http.MethodPost, "/api/security/recovery", strings.NewReader(`{"action":"status"}`))
	req.RemoteAddr = "203.0.113.30:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for remote recovery request, got %d", rec.Code)
	}
}

func TestHealthEndpointIsPublicEvenWhenAuthConfigured(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed"
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	defer monitor.Stop()

	router := NewRouter(cfg, monitor, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.40")
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.RemoteAddr = "203.0.113.40:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for public health endpoint, got %d", rec.Code)
	}
}

func TestVersionEndpointIsPublicEvenWhenAuthConfigured(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.41")
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	req.RemoteAddr = "203.0.113.41:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for public version endpoint, got %d", rec.Code)
	}
}

func TestAgentVersionEndpointIsPublicEvenWhenAuthConfigured(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.42")
	req := httptest.NewRequest(http.MethodGet, "/api/agent/version", nil)
	req.RemoteAddr = "203.0.113.42:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for public agent version endpoint, got %d", rec.Code)
	}
}

func TestServerInfoEndpointIsPublicEvenWhenAuthConfigured(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.43")
	req := httptest.NewRequest(http.MethodGet, "/api/server/info", nil)
	req.RemoteAddr = "203.0.113.43:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for public server info endpoint, got %d", rec.Code)
	}
}

func TestSecurityStatusIsPublicEvenWhenAuthConfigured(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.44")
	req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	req.RemoteAddr = "203.0.113.44:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for public security status endpoint, got %d", rec.Code)
	}
}

func TestValidateBootstrapTokenBypassesAuth(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.45")
	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-bootstrap-token", strings.NewReader(`{}`))
	req.RemoteAddr = "203.0.113.45:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 when bootstrap token is unavailable, got %d", rec.Code)
	}
}

func TestSecurityStatusHidesBootstrapTokenWhenAuthConfigured(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.49")
	req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	req.RemoteAddr = "203.0.113.49:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for security status, got %d", rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := payload["bootstrapTokenPath"]; ok {
		t.Fatalf("expected bootstrapTokenPath to be omitted when auth is configured")
	}
}

func TestSecurityStatusIncludesBootstrapTokenWhenUnauthenticated(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.50")
	req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	req.RemoteAddr = "203.0.113.50:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for security status, got %d", rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	path, ok := payload["bootstrapTokenPath"].(string)
	if !ok || path == "" {
		t.Fatalf("expected bootstrapTokenPath to be present for unauthenticated setup")
	}
}

func TestAuditRequiresAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "audit-auth-token-123.12345678", []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestSecurityStatusIgnoresTokenQueryParam(t *testing.T) {
	rawToken := "status-query-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/security/status?token="+rawToken, nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for security status, got %d", rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if hint, ok := payload["apiTokenHint"].(string); ok && hint != "" {
		t.Fatalf("expected apiTokenHint to be empty when token passed via query param, got %q", hint)
	}
	if _, ok := payload["tokenScopes"]; ok {
		t.Fatalf("expected tokenScopes to be omitted when unauthenticated")
	}
}

func TestSecurityStatusAcceptsTokenHeader(t *testing.T) {
	rawToken := "status-header-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for security status, got %d", rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if hint, ok := payload["apiTokenHint"].(string); !ok || hint != cfg.PrimaryAPITokenHint() {
		t.Fatalf("expected apiTokenHint %q, got %v", cfg.PrimaryAPITokenHint(), payload["apiTokenHint"])
	}
	if scopes, ok := payload["tokenScopes"].([]interface{}); !ok || len(scopes) == 0 {
		t.Fatalf("expected tokenScopes to be present when authenticated via API token")
	}
}

func TestAuditVerifyRequiresAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "audit-verify-auth-token-123.12345678", []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/audit/event-1/verify", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestPathTraversalBlockedForAPIPaths(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/../api/security/status", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for path traversal on api, got %d", rec.Code)
	}
}

func TestPathTraversalBlockedForNonAPIPaths(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/../etc/passwd", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for path traversal on non-api, got %d", rec.Code)
	}
}

func TestPathTraversalBlockedForEncodedAPIPaths(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/%2e%2e/api/security/status", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for encoded path traversal on api, got %d", rec.Code)
	}
}

func TestPathTraversalBlockedForEncodedNonAPIPaths(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/%2e%2e/%2e%2e/etc/passwd", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for encoded path traversal on non-api, got %d", rec.Code)
	}
}

func TestSetupScriptIsPublicEvenWhenAuthConfigured(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/setup-script", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing params on public setup script, got %d", rec.Code)
	}
}

func TestPublicDownloadEndpointsBypassAuth(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/install-docker-agent.sh",
		"/install-container-agent.sh",
		"/download/pulse-docker-agent",
		"/install-host-agent.sh",
		"/install-host-agent.ps1",
		"/uninstall-host-agent.sh",
		"/uninstall-host-agent.ps1",
		"/download/pulse-host-agent",
		"/install.sh",
		"/install.ps1",
		"/download/pulse-agent",
	}

	for idx, path := range paths {
		ip := "203.0.113." + strconv.Itoa(70+idx)
		ResetRateLimitForIP(ip)
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 for public download endpoint %s, got %d", path, rec.Code)
		}
	}
}

func TestHostAgentChecksumRequiresAuth(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.90")
	req := httptest.NewRequest(http.MethodGet, "/download/pulse-host-agent.sha256", nil)
	req.RemoteAddr = "203.0.113.90:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for protected checksum, got %d", rec.Code)
	}
}

func TestHostAgentChecksumAllowsTokenAuth(t *testing.T) {
	rawToken := "checksum-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.91")
	req := httptest.NewRequest(http.MethodGet, "/download/pulse-host-agent.sha256", nil)
	req.RemoteAddr = "203.0.113.91:1234"
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
		t.Fatalf("expected checksum to allow token auth, got %d", rec.Code)
	}
}

func TestPublicEndpointsBypassAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "public-api-token-123.12345678", []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	defer monitor.Stop()

	router := NewRouter(cfg, monitor, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/health",
		"/api/version",
		"/api/agent/version",
		"/api/server/info",
		"/api/security/status",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for public endpoint %s, got %d", path, rec.Code)
		}
	}
}

func TestSSHKeyGenerationBlockedInContainer(t *testing.T) {
	t.Setenv("PULSE_DOCKER", "true")
	t.Setenv("PULSE_DEV_ALLOW_CONTAINER_SSH", "")
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	handler := NewConfigHandlers(nil, nil, func() error { return nil }, nil, nil, func() {})
	keys := handler.getOrGenerateSSHKeys()
	if keys.SensorsPublicKey != "" {
		t.Fatalf("expected empty key when container SSH generation is blocked")
	}

	pubKeyPath := filepath.Join(homeDir, ".ssh", "id_ed25519_sensors.pub")
	if _, err := os.Stat(pubKeyPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no key files to be written, got err=%v", err)
	}
}

func TestSetupScriptRejectsInvalidAuthToken(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve&host=https://example.com&pulse_url=https://pulse.example.com&auth_token=not-hex", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid auth_token, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Invalid auth_token parameter") {
		t.Fatalf("expected invalid auth_token error, got %q", rec.Body.String())
	}
}

func TestSetupScriptRejectsInvalidHostURL(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve&host=ftp://example.com&pulse_url=https://pulse.example.com", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid host, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Invalid host parameter") {
		t.Fatalf("expected invalid host error, got %q", rec.Body.String())
	}
}

func TestSetupScriptRejectsInvalidPulseURL(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve&host=https://example.com&pulse_url=ftp://pulse.example.com", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid pulse_url, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Invalid pulse_url parameter") {
		t.Fatalf("expected invalid pulse_url error, got %q", rec.Body.String())
	}
}

func TestOIDCLoginBypassesAuth(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.46")
	req := httptest.NewRequest(http.MethodGet, "/api/oidc/login", nil)
	req.RemoteAddr = "203.0.113.46:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect when OIDC is disabled, got %d", rec.Code)
	}
}

func TestOIDCCallbackBypassesAuth(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.51")
	req := httptest.NewRequest(http.MethodGet, config.DefaultOIDCCallbackPath, nil)
	req.RemoteAddr = "203.0.113.51:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when OIDC is disabled, got %d", rec.Code)
	}
}

func TestAIOAuthCallbackBypassesAuth(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.47")
	req := httptest.NewRequest(http.MethodGet, "/api/ai/oauth/callback", nil)
	req.RemoteAddr = "203.0.113.47:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307 redirect for OAuth callback, got %d", rec.Code)
	}
}

func TestLoginEndpointBypassesAuth(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	ResetRateLimitForIP("203.0.113.48")
	req := httptest.NewRequest(http.MethodGet, "/api/login", nil)
	req.RemoteAddr = "203.0.113.48:1234"
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for login GET, got %d", rec.Code)
	}
}

func TestInstallScriptEndpointsBypassAuth(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/install/install-docker.sh",
	}

	for idx, path := range paths {
		ip := "203.0.113." + strconv.Itoa(60+idx)
		ResetRateLimitForIP(ip)
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 for public install script %s, got %d", path, rec.Code)
		}
	}
}

func TestInstallScriptAPIRoutesRequireAuth(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed"
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []string{
		"/api/install/install.sh",
		"/api/install/install.ps1",
	}

	for idx, path := range paths {
		ip := "203.0.113." + strconv.Itoa(80+idx)
		ResetRateLimitForIP(ip)
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for protected install script %s, got %d", path, rec.Code)
		}
	}
}

func TestLogoutRequiresAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "logout-token-123.12345678", []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestStateRequiresAuthInAPIMode(t *testing.T) {
	record := newTokenRecord(t, "state-token-123.12345678", []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestStateRequiresMonitoringReadScope(t *testing.T) {
	rawToken := "state-scope-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
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

func TestVerifyTemperatureSSHAllowsSetupToken(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	token := "0123456789abcdef0123456789abcdef"
	tokenHash := auth.HashAPIToken(token)
	router.configHandlers.codeMutex.Lock()
	router.configHandlers.setupCodes[tokenHash] = &SetupCode{ExpiresAt: time.Now().Add(time.Minute)}
	router.configHandlers.codeMutex.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/api/system/verify-temperature-ssh", strings.NewReader(`{"nodes":""}`))
	req.Header.Set("X-Setup-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with setup token, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "No nodes to verify") {
		t.Fatalf("expected verify response, got %q", rec.Body.String())
	}
}

func TestSSHConfigAllowsSetupToken(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	t.Setenv("HOME", t.TempDir())

	token := "abcdef0123456789abcdef0123456789"
	tokenHash := auth.HashAPIToken(token)
	router.configHandlers.codeMutex.Lock()
	router.configHandlers.setupCodes[tokenHash] = &SetupCode{ExpiresAt: time.Now().Add(time.Minute)}
	router.configHandlers.codeMutex.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/api/system/ssh-config", strings.NewReader("Host example\nHostname example\n"))
	req.Header.Set("X-Setup-Token", token)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with setup token, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"success":true`) {
		t.Fatalf("expected success response, got %q", rec.Body.String())
	}
}

func TestVerifyTemperatureSSHAllowsSetupTokenQueryParam(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	token := "abcdefabcdefabcdefabcdefabcdefab"
	tokenHash := auth.HashAPIToken(token)
	router.configHandlers.codeMutex.Lock()
	router.configHandlers.setupCodes[tokenHash] = &SetupCode{ExpiresAt: time.Now().Add(time.Minute)}
	router.configHandlers.codeMutex.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/api/system/verify-temperature-ssh?auth_token="+token, strings.NewReader(`{"nodes":""}`))
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with setup token query param, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "No nodes to verify") {
		t.Fatalf("expected verify response, got %q", rec.Body.String())
	}
}

func TestSSHConfigAllowsSetupTokenQueryParam(t *testing.T) {
	cfg := newTestConfigWithTokens(t)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	t.Setenv("HOME", t.TempDir())

	token := "deadbeefdeadbeefdeadbeefdeadbeef"
	tokenHash := auth.HashAPIToken(token)
	router.configHandlers.codeMutex.Lock()
	router.configHandlers.setupCodes[tokenHash] = &SetupCode{ExpiresAt: time.Now().Add(time.Minute)}
	router.configHandlers.codeMutex.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/api/system/ssh-config?auth_token="+token, strings.NewReader("Host example\nHostname example\n"))
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with setup token query param, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"success":true`) {
		t.Fatalf("expected success response, got %q", rec.Body.String())
	}
}
