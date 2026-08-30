package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
)

type actionRunnerFailingPersistenceFS struct{}

func (actionRunnerFailingPersistenceFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}
func (actionRunnerFailingPersistenceFS) WriteFile(string, []byte, os.FileMode) error {
	return errors.New("injected action-runner persistence failure")
}
func (actionRunnerFailingPersistenceFS) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}
func (actionRunnerFailingPersistenceFS) Remove(name string) error { return os.Remove(name) }
func (actionRunnerFailingPersistenceFS) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}
func (actionRunnerFailingPersistenceFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func newActionRunnerCredentialTestRouter(t *testing.T) (*Router, *config.Config, string) {
	t.Helper()
	cfg := &config.Config{DataPath: t.TempDir(), AuthUser: "admin", AuthPass: "$2a$10$dummy"}
	handlers, monitor := newUnifiedAgentHandlers(t, cfg)
	hostID := seedUnifiedAgentHost(t, monitor)
	return &Router{
		config:               cfg,
		persistence:          config.NewConfigPersistence(cfg.DataPath),
		unifiedAgentHandlers: handlers,
	}, cfg, hostID
}

func actionRunnerCredentialBody(hostID, hostname string) *bytes.Reader {
	body, _ := json.Marshal(actionRunnerCredentialRequest{AgentID: hostID, Hostname: hostname})
	return bytes.NewReader(body)
}

func issueActionRunnerCredentialForTest(t *testing.T, router *Router, hostID, hostname string) actionRunnerCredentialResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/agents/action-runner/credential", actionRunnerCredentialBody(hostID, hostname))
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "default"))
	rec := httptest.NewRecorder()
	router.handleIssueActionRunnerCredential(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("issue status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var response actionRunnerCredentialResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	return response
}

func connectActionRunnerCredentialForTest(t *testing.T, server *agentexec.Server, credential actionRunnerCredentialResponse) (*websocket.Conn, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	conn, _, err := websocket.DefaultDialer.Dial(wsURLForHTTP(ts.URL), wsHeadersForHTTP(t, ts.URL))
	if err != nil {
		ts.Close()
		t.Fatal(err)
	}
	message, err := agentexec.NewMessage(agentexec.MsgTypeAgentRegister, "", agentexec.AgentRegisterPayload{
		AgentID: credential.AgentID, Hostname: credential.Hostname, Token: credential.Token,
		RuntimeRole: credential.RuntimeRole, ActionCapability: credential.ActionCapability,
		OperationReceiptVersion: operationreceipt.ProtocolVersion,
	})
	if err != nil {
		conn.Close()
		ts.Close()
		t.Fatal(err)
	}
	if err := conn.WriteJSON(message); err != nil {
		conn.Close()
		ts.Close()
		t.Fatal(err)
	}
	if registered := readRegisteredPayload(t, conn); !registered.Success {
		conn.Close()
		ts.Close()
		t.Fatalf("runner registration failed: %s", registered.Message)
	}
	return conn, ts
}

func TestIssueActionRunnerCredentialResolvesCanonicalMonitoredHost(t *testing.T) {
	router, cfg, hostID := newActionRunnerCredentialTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/agents/action-runner/credential", actionRunnerCredentialBody(hostID, "HOST-1"))
	rec := httptest.NewRecorder()
	router.handleIssueActionRunnerCredential(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var response actionRunnerCredentialResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Token == "" || response.TokenID == "" || response.AgentID != hostID || response.Hostname != "host-1.local" {
		t.Fatalf("response = %#v", response)
	}
	if response.RuntimeRole != agenttokens.CredentialKindActionRunner || response.ActionCapability != agenttokens.ActionCapabilityTypedV1 {
		t.Fatalf("response authority = %#v", response)
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].HasScope(config.ScopeAgentReport) || !cfg.APITokens[0].HasScope(config.ScopeAgentExec) {
		t.Fatalf("persisted action credential = %#v", cfg.APITokens)
	}
}

func TestIssueActionRunnerCredentialRejectsUnknownOrMismatchedHost(t *testing.T) {
	for _, tc := range []struct {
		name     string
		agentID  string
		hostname string
	}{
		{name: "unknown id", agentID: "missing", hostname: "host-1.local"},
		{name: "mismatched hostname", agentID: "machine-1", hostname: "other.local"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router, cfg, hostID := newActionRunnerCredentialTestRouter(t)
			agentID := tc.agentID
			if agentID == "machine-1" {
				agentID = hostID
			}
			req := httptest.NewRequest(http.MethodPost, "/api/agents/action-runner/credential", actionRunnerCredentialBody(agentID, tc.hostname))
			rec := httptest.NewRecorder()
			router.handleIssueActionRunnerCredential(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}
			if len(cfg.APITokens) != 0 {
				t.Fatalf("rejected identity minted tokens: %#v", cfg.APITokens)
			}
		})
	}
}

func TestIssueActionRunnerCredentialRouteRotatesExistingHostBinding(t *testing.T) {
	router, cfg, hostID := newActionRunnerCredentialTestRouter(t)
	issue := func() actionRunnerCredentialResponse {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/agents/action-runner/credential", actionRunnerCredentialBody(hostID, "HOST-1"))
		rec := httptest.NewRecorder()
		router.handleIssueActionRunnerCredential(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
		}
		var response actionRunnerCredentialResponse
		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return response
	}
	first := issue()
	second := issue()
	if first.TokenID == second.TokenID || first.Token == second.Token {
		t.Fatalf("re-enrollment did not rotate credential: first=%#v second=%#v", first, second)
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != second.TokenID {
		t.Fatalf("route accumulated host-bound credentials: %#v", cfg.APITokens)
	}
}

func TestIssueActionRunnerCredentialRotationClosesReplacedLiveSessionAfterPersistence(t *testing.T) {
	router, _, hostID := newActionRunnerCredentialTestRouter(t)
	router.agentExecServer = agentexec.NewServerWithAdmissionValidator(router.admitAgentExecToken, router.validateAgentExecSession)
	first := issueActionRunnerCredentialForTest(t, router, hostID, "host-1.local")
	conn, ts := connectActionRunnerCredentialForTest(t, router.agentExecServer, first)
	defer conn.Close()
	defer ts.Close()
	if !router.agentExecServer.IsAgentConnectedForOrganization("default", hostID) {
		t.Fatal("runner session was not connected")
	}

	second := issueActionRunnerCredentialForTest(t, router, hostID, "host-1.local")
	if second.TokenID == first.TokenID {
		t.Fatal("credential did not rotate")
	}
	if router.agentExecServer.IsAgentConnectedForOrganization("default", hostID) {
		t.Fatal("replaced action-runner session remained connected")
	}
}

func TestIssueActionRunnerCredentialPersistenceFailureKeepsPriorLiveSession(t *testing.T) {
	router, cfg, hostID := newActionRunnerCredentialTestRouter(t)
	router.agentExecServer = agentexec.NewServerWithAdmissionValidator(router.admitAgentExecToken, router.validateAgentExecSession)
	first := issueActionRunnerCredentialForTest(t, router, hostID, "host-1.local")
	conn, ts := connectActionRunnerCredentialForTest(t, router.agentExecServer, first)
	defer conn.Close()
	defer ts.Close()

	router.persistence.SetFileSystem(actionRunnerFailingPersistenceFS{})
	req := httptest.NewRequest(http.MethodPost, "/api/agents/action-runner/credential", actionRunnerCredentialBody(hostID, "host-1.local"))
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "default"))
	rec := httptest.NewRecorder()
	router.handleIssueActionRunnerCredential(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != first.TokenID {
		t.Fatalf("prior credential was not restored: %#v", cfg.APITokens)
	}
	if !router.agentExecServer.IsAgentConnectedForOrganization("default", hostID) {
		t.Fatal("failed persistence invalidated the prior live session")
	}
}

func TestSelfRevokeActionRunnerCredentialRequiresExactBearerBindingAndClosesSession(t *testing.T) {
	router, cfg, hostID := newActionRunnerCredentialTestRouter(t)
	router.agentExecServer = agentexec.NewServerWithAdmissionValidator(router.admitAgentExecToken, router.validateAgentExecSession)
	issued := issueActionRunnerCredentialForTest(t, router, hostID, "host-1.local")
	conn, ts := connectActionRunnerCredentialForTest(t, router.agentExecServer, issued)
	defer conn.Close()
	defer ts.Close()

	handler := actionRunnerCredentialRoute(cfg, router.handleIssueActionRunnerCredential, router.handleSelfRevokeActionRunnerCredential)
	request := func(organizationID, agentID, hostname, token string) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(actionRunnerCredentialSelfRevokeRequest{AgentID: agentID, Hostname: hostname})
		req := httptest.NewRequest(http.MethodDelete, "/api/agents/action-runner/credential", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, organizationID))
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec
	}
	if rec := request("other", hostID, issued.Hostname, issued.Token); rec.Code != http.StatusForbidden {
		t.Fatalf("mismatched organization status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if rec := request("default", hostID, "other.example", issued.Token); rec.Code != http.StatusForbidden {
		t.Fatalf("mismatched hostname status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(cfg.APITokens) != 1 || !router.agentExecServer.IsAgentConnectedForOrganization("default", hostID) {
		t.Fatal("binding mismatch changed credential or session state")
	}
	if rec := request("default", hostID, issued.Hostname, issued.Token); rec.Code != http.StatusNoContent {
		t.Fatalf("self revoke status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(cfg.APITokens) != 0 {
		t.Fatalf("self-revoked credential remained stored: %#v", cfg.APITokens)
	}
	if router.agentExecServer.IsAgentConnectedForOrganization("default", hostID) {
		t.Fatal("self-revoked session remained connected")
	}
}

func TestActionRunnerCredentialRouteAuthSupportsAdminSessionAndScopedToken(t *testing.T) {
	for _, mode := range []string{"api-token", "admin-session"} {
		t.Run(mode, func(t *testing.T) {
			router, cfg, hostID := newActionRunnerCredentialTestRouter(t)
			handler := actionRunnerCredentialRoute(cfg, router.handleIssueActionRunnerCredential, router.handleSelfRevokeActionRunnerCredential)
			req := httptest.NewRequest(http.MethodPost, "/api/agents/action-runner/credential", actionRunnerCredentialBody(hostID, "host-1.local"))
			switch mode {
			case "api-token":
				raw := "runner-issuer-token-1234567890.12345678"
				record, err := config.NewAPITokenRecord(raw, "runner issuer", []string{config.ScopeSettingsWrite, config.ScopeActionsExecute})
				if err != nil {
					t.Fatal(err)
				}
				cfg.APITokens = append(cfg.APITokens, *record)
				req.Header.Set("Authorization", "Bearer "+raw)
			case "admin-session":
				resetSessionStoreForTests()
				t.Cleanup(resetSessionStoreForTests)
				InitSessionStore(t.TempDir())
				sessionToken := generateSessionToken()
				GetSessionStore().CreateSession(sessionToken, time.Hour, "browser", "127.0.0.1", "admin")
				req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
			}
			rec := httptest.NewRecorder()
			handler(rec, req)
			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestActionRunnerCredentialCannotUseCollectorReportOrConfigScopes(t *testing.T) {
	router, cfg, hostID := newActionRunnerCredentialTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/agents/action-runner/credential", actionRunnerCredentialBody(hostID, "host-1.local"))
	rec := httptest.NewRecorder()
	router.handleIssueActionRunnerCredential(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("issue status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var issued actionRunnerCredentialResponse
	if err := json.NewDecoder(rec.Body).Decode(&issued); err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{config.ScopeAgentReport, config.ScopeAgentConfigRead} {
		called := false
		handler := RequireAuth(cfg, RequireScope(scope, func(http.ResponseWriter, *http.Request) { called = true }))
		authReq := httptest.NewRequest(http.MethodGet, "/api/agents/agent/test", nil)
		authReq.Header.Set("Authorization", "Bearer "+issued.Token)
		authRec := httptest.NewRecorder()
		handler(authRec, authReq)
		if called || authRec.Code != http.StatusForbidden {
			t.Fatalf("action credential scope %q = called=%v status=%d body=%s", scope, called, authRec.Code, authRec.Body.String())
		}
	}
}
