package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	gorillaws "github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	pulsews "github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const monitoringWebSocketToken = "synthetic-monitoring-websocket-token.12345678"
const monitoringWebSocketControl = "synthetic-monitoring-websocket-control.12345678"

func newMonitoringWebSocketRouter(t *testing.T) (*Router, *config.Config, *httptest.Server, *pulsews.Hub) {
	t.Helper()
	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, monitoringWebSocketToken, []string{config.ScopeMonitoringRead, config.ScopeAIExecute}, nil),
		newTokenRecord(t, monitoringWebSocketControl, []string{config.ScopeWildcard}, nil))
	hub := pulsews.NewHub(nil)
	go hub.Run()
	t.Cleanup(hub.Stop)
	router := NewRouter(cfg, nil, nil, hub, nil, "test")
	cleanupTestRouter(t, router)
	server := httptest.NewServer(router.Handler())
	t.Cleanup(server.Close)
	return router, cfg, server, hub
}

func monitoringWebSocketAddress(server *httptest.Server, raw, transport string) (string, http.Header) {
	address := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?org_id=default"
	headers := http.Header{"Origin": []string{server.URL}}
	switch transport {
	case "query":
		address += "&token=" + raw
	case "bearer":
		headers.Set("Authorization", "Bearer "+raw)
	case "session":
		headers.Set("Cookie", cookieNameSession+"="+raw)
	default:
		headers.Set("X-API-Token", raw)
	}
	return address, headers
}

func openMonitoringWebSocket(t *testing.T, server *httptest.Server, raw, transport string) *gorillaws.Conn {
	t.Helper()
	address, headers := monitoringWebSocketAddress(server, raw, transport)
	conn, _, err := gorillaws.DefaultDialer.Dial(address, headers)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	// A real application ping/pong synchronizes writer admission, without
	// waiting for the unrelated delayed initial-state builder.
	if err := conn.WriteJSON(pulsews.Message{Type: "ping"}); err != nil {
		t.Fatal(err)
	}
	if frame := nextMonitoringWebSocketFrame(t, conn); frame.Type != "pong" {
		t.Fatalf("admitted socket = %q, want pong", frame.Type)
	}
	return conn
}

func nextMonitoringWebSocketFrame(t *testing.T, conn *gorillaws.Conn) pulsews.Message {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var msg pulsews.Message
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatal(err)
	}
	return msg
}

func wantMonitoringWebSocketClientCount(t *testing.T, hub *pulsews.Hub, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if hub.GetClientCount() == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("remaining socket clients = %d, want %d", hub.GetClientCount(), want)
}

func wantMonitoringWebSocketClosed(t *testing.T, conn *gorillaws.Conn) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, body, err := conn.ReadMessage()
	var closed *gorillaws.CloseError
	if !errors.As(err, &closed) || closed.Code != gorillaws.ClosePolicyViolation || closed.Text != "Monitoring access ended" {
		t.Fatalf("withdrawn socket = %s, %v; want fixed policy close, no product frame", body, err)
	}
}

func TestMonitoringWebSocketRejectsWithdrawnTokenAuthority(t *testing.T) {
	for _, transport := range []string{"header", "bearer", "query"} {
		for _, change := range []string{"revoked", "expired", "read_scope_removed", "org_rebound", "identity_replaced", "principal_replaced", "invalid_role"} {
			t.Run(transport+"/"+change, func(t *testing.T) {
				_, cfg, server, hub := newMonitoringWebSocketRouter(t)
				primary := openMonitoringWebSocket(t, server, monitoringWebSocketToken, transport)
				control := openMonitoringWebSocket(t, server, monitoringWebSocketControl, transport)
				config.Mu.Lock()
				switch change {
				case "revoked":
					cfg.RemoveAPIToken(cfg.APITokens[0].ID)
				case "expired":
					past := time.Now().Add(-time.Second)
					cfg.APITokens[0].ExpiresAt = &past
				case "read_scope_removed":
					cfg.APITokens[0].Scopes = []string{config.ScopeAgentReport}
				case "org_rebound":
					cfg.APITokens[0].OrgID = "other-org"
				case "identity_replaced":
					cfg.APITokens[0].ID = "other-credential"
				case "principal_replaced":
					cfg.APITokens[0].Metadata = map[string]string{apiTokenMetadataOwnerUserID: "other-owner"}
				case "invalid_role":
					cfg.APITokens[0].Metadata = map[string]string{internalauth.RuntimeRoleMetadataKey: internalauth.RuntimeRoleActionRunner}
				}
				config.Mu.Unlock()
				hub.BroadcastMessageToTenant("default", pulsews.Message{Type: "notification", Data: "after-withdrawal"})
				wantMonitoringWebSocketClosed(t, primary)
				wantMonitoringWebSocketClientCount(t, hub, 1)
				if frame := nextMonitoringWebSocketFrame(t, control); frame.Type != "notification" || frame.Data != "after-withdrawal" {
					t.Fatalf("independent credential lost its frame: %+v", frame)
				}
				// The normal upgrade path must also reject the withdrawn identity.
				address, headers := monitoringWebSocketAddress(server, monitoringWebSocketToken, transport)
				conn, resp, err := gorillaws.DefaultDialer.Dial(address, headers)
				if conn != nil {
					conn.Close()
				}
				if change == "identity_replaced" || change == "principal_replaced" {
					// A new request may authenticate the new identity. Only the
					// already-admitted connection is bound to its old principal.
					if err != nil {
						t.Fatalf("new valid identity could not reconnect: %v", err)
					}
				} else if err == nil || resp == nil || (resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden) {
					t.Fatalf("withdrawn reconnect: %v, %v", resp, err)
				}
			})
		}
	}
}

func TestMonitoringWebSocketPreservesAllowedTokenAndSession(t *testing.T) {
	_, cfg, server, hub := newMonitoringWebSocketRouter(t)
	primary := openMonitoringWebSocket(t, server, monitoringWebSocketToken, "query")
	const session = "synthetic-monitoring-websocket-session"
	GetSessionStore().CreateSession(session, time.Hour, "fixture", "127.0.0.1", "operator")
	t.Cleanup(func() { GetSessionStore().DeleteSession(session) })
	browser := openMonitoringWebSocket(t, server, session, "session")
	config.Mu.Lock()
	// Withdrawing execute alone must not remove wanted monitoring reads.
	cfg.APITokens[0].Scopes = []string{config.ScopeMonitoringRead}
	config.Mu.Unlock()
	hub.BroadcastMessageToTenant("default", pulsews.Message{Type: "notification", Data: "still-readable"})
	for _, conn := range []*gorillaws.Conn{primary, browser} {
		if frame := nextMonitoringWebSocketFrame(t, conn); frame.Data != "still-readable" {
			t.Fatalf("allowed read changed: %+v", frame)
		}
	}
	config.Mu.Lock()
	cfg.RemoveAPIToken(cfg.APITokens[0].ID)
	config.Mu.Unlock()
	hub.BroadcastMessageToTenant("default", pulsews.Message{Type: "notification", Data: "session-still-readable"})
	wantMonitoringWebSocketClosed(t, primary)
	if frame := nextMonitoringWebSocketFrame(t, browser); frame.Data != "session-still-readable" {
		t.Fatalf("token removal changed independent session: %+v", frame)
	}
}

// The socket uses the same current-owner resolution as normal authentication,
// without starting a monitored tenant or opening an appliance/database runtime.
func TestMonitoringWebSocketAuthorizationUsesLiveTenantWithoutMutatingRequest(t *testing.T) {
	const orgID = "socket-tenant"
	token := newTokenRecord(t, monitoringWebSocketToken, []string{config.ScopeMonitoringRead}, nil)
	token.OrgID = orgID
	base := &config.Config{}
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	if _, err := mtp.GetPersistence(orgID); err != nil {
		t.Fatal(err)
	}
	makeManager := func(cfg *config.Config) *monitoring.MultiTenantMonitor {
		manager := monitoring.NewMultiTenantMonitor(base, mtp, nil)
		monitor := &monitoring.Monitor{}
		setUnexportedField(t, monitor, "config", cfg)
		setUnexportedField(t, manager, "monitors", map[string]*monitoring.Monitor{orgID: monitor})
		t.Cleanup(manager.Stop)
		return manager
	}
	owner := makeManager(&config.Config{APITokens: []config.APITokenRecord{token}})
	empty := makeManager(&config.Config{})
	replacement := makeManager(&config.Config{APITokens: []config.APITokenRecord{token.Clone()}})
	req := httptest.NewRequest(http.MethodGet, "/ws?token="+monitoringWebSocketToken, nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("X-Pulse-Org-ID", orgID)
	req = extractAndStoreAuthContext(base, owner, req)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, orgID))
	source := req.Context().Value(tenantTokenSourceContextKey).(tenantTokenSource)
	var current atomic.Pointer[monitoring.MultiTenantMonitor]
	current.Store(owner)
	source.resolveManager = current.Load
	req = req.WithContext(context.WithValue(req.Context(), tenantTokenSourceContextKey, source))
	originalContext := req.Context()
	originalToken := getAPITokenRecordFromRequest(req).Clone()
	originalHeader := req.Header.Clone()
	authorize := monitoringWebSocketAuthorization(base, req)
	if authorize == nil || !authorize() {
		t.Fatal("admitted tenant rejected")
	}
	// A copied request remains safe after the HTTP handler returns and another
	// observer reads its original context/headers concurrently with refreshes.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 64 {
			if !authorize() {
				t.Error("unchanged tenant authority rejected")
			}
		}
	}()
	for range 64 {
		if req.Context() != originalContext || !reflect.DeepEqual(req.Header, originalHeader) || !reflect.DeepEqual(getAPITokenRecordFromRequest(req).Clone(), originalToken) {
			t.Fatal("socket validation mutated its admitted HTTP request")
		}
	}
	<-done
	current.Store(replacement)
	if !authorize() {
		t.Fatal("same valid credential in current tenant owner was rejected")
	}
	config.Mu.Lock()
	base.APITokens = []config.APITokenRecord{token.Clone()}
	config.Mu.Unlock()
	for _, manager := range []*monitoring.MultiTenantMonitor{empty, nil} {
		current.Store(manager)
		if authorize() {
			t.Fatal("removed tenant authority fell back to a stale or global inventory")
		}
	}
}

func deleteMonitoringWebSocketTokenThroughHTTP(t *testing.T) {
	t.Helper()
	router, cfg, server, hub := newMonitoringWebSocketRouter(t)
	primary := openMonitoringWebSocket(t, server, monitoringWebSocketToken, "query")
	id := cfg.APITokens[0].ID
	req, _ := http.NewRequest(http.MethodDelete, server.URL+"/api/security/tokens/"+id, nil)
	req.Header.Set("X-API-Token", monitoringWebSocketControl)
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("token deletion = %d: %s", resp.StatusCode, body)
	}
	persisted, err := router.persistence.LoadAPITokens()
	if err != nil || len(persisted) != 1 || persisted[0].ID == id {
		t.Fatalf("token revocation not persisted: %v", err)
	}
	hub.BroadcastMessageToTenant("default", pulsews.Message{Type: "notification", Data: "after-persisted-deletion"})
	wantMonitoringWebSocketClosed(t, primary)
	wantMonitoringWebSocketClientCount(t, hub, 0)
	wire, _ := json.Marshal(map[string]any{"code": gorillaws.ClosePolicyViolation, "reason": "Monitoring access ended"})
	t.Logf("actual registered-route policy close after persisted HTTP deletion: %s", wire)
}
