package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
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

func commitActionRunnerCredentialForTest(t *testing.T, router *Router, credential actionRunnerCredentialResponse) {
	t.Helper()
	if _, _, _, err := agenttokens.ActivateActionRunnerAndPersist(router.config, router.persistence, credential.TokenID, credential.AgentID, credential.Hostname); err != nil {
		t.Fatalf("activate action runner credential: %v", err)
	}
}

func requestActionRunnerActivationForTest(t *testing.T, router *Router, credential actionRunnerCredentialResponse) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(actionRunnerCredentialSelfRevokeRequest{AgentID: credential.AgentID, Hostname: credential.Hostname})
	req := httptest.NewRequest(http.MethodPatch, "/api/agents/action-runner/credential", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+credential.Token)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, credential.OrganizationID))
	rec := httptest.NewRecorder()
	actionRunnerCredentialRoute(router.config, router.handleIssueActionRunnerCredential, router.handleActivateActionRunnerCredential, router.handleSelfRevokeActionRunnerCredential)(rec, req)
	return rec
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

func TestIssueActionRunnerCredentialRoutePreparesExistingHostRotation(t *testing.T) {
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
	commitActionRunnerCredentialForTest(t, router, first)
	second := issue()
	if first.TokenID == second.TokenID || first.Token == second.Token {
		t.Fatalf("re-enrollment did not rotate credential: first=%#v second=%#v", first, second)
	}
	if !second.ActivationPending || second.ActivationDeadline == nil {
		t.Fatalf("prepared response = %#v", second)
	}
	if len(cfg.APITokens) != 2 {
		t.Fatalf("route did not retain active predecessor during prepare: %#v", cfg.APITokens)
	}
}

func TestIssueActionRunnerCredentialRotationCommitsOnlyAfterReplacementHealthHandshake(t *testing.T) {
	router, cfg, hostID := newActionRunnerCredentialTestRouter(t)
	router.agentExecServer = agentexec.NewServerWithAdmissionValidator(router.admitAgentExecToken, router.validateAgentExecSession)
	first := issueActionRunnerCredentialForTest(t, router, hostID, "host-1.local")
	commitActionRunnerCredentialForTest(t, router, first)
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
	if !router.agentExecServer.IsAgentConnectedForOrganization("default", hostID) {
		t.Fatal("prepare step revoked the prior runner before replacement activation")
	}
	if len(cfg.APITokens) != 2 {
		t.Fatalf("prepared rotation inventory = %#v", cfg.APITokens)
	}
	pendingConn, pendingServer := connectActionRunnerCredentialForTest(t, router.agentExecServer, second)
	defer pendingConn.Close()
	defer pendingServer.Close()
	if !router.agentExecServer.IsAgentConnectedForOrganization("default", hostID) {
		t.Fatal("pending replacement interrupted active dispatch before activation")
	}
	if rec := requestActionRunnerActivationForTest(t, router, second); rec.Code != http.StatusNoContent {
		t.Fatalf("activation status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !router.agentExecServer.IsAgentConnectedForOrganization("default", hostID) {
		t.Fatal("activated replacement did not become dispatchable")
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != second.TokenID || cfg.APITokens[0].ExpiresAt != nil {
		t.Fatalf("committed rotation inventory = %#v", cfg.APITokens)
	}
	if _, ok := router.admitAgentExecToken(first.Token, hostID, first.Hostname); ok {
		t.Fatal("committed rotation left prior secret valid")
	}
}

func TestIssueActionRunnerCredentialPersistenceFailureKeepsPriorLiveSession(t *testing.T) {
	router, cfg, hostID := newActionRunnerCredentialTestRouter(t)
	router.agentExecServer = agentexec.NewServerWithAdmissionValidator(router.admitAgentExecToken, router.validateAgentExecSession)
	first := issueActionRunnerCredentialForTest(t, router, hostID, "host-1.local")
	commitActionRunnerCredentialForTest(t, router, first)
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

func TestCancelPendingActionRunnerCredentialActivationIsSelfOnlyAnd204IsRollbackAuthority(t *testing.T) {
	router, cfg, hostID := newActionRunnerCredentialTestRouter(t)
	router.agentExecServer = agentexec.NewServerWithAdmissionValidator(router.admitAgentExecToken, router.validateAgentExecSession)
	handler := RequireAuth(cfg, RequireScope(config.ScopeAgentExec, router.handleCancelPendingActionRunnerCredentialActivation))
	request := func(token, organizationID, body string) *httptest.ResponseRecorder {
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		req := httptest.NewRequest(http.MethodDelete, "/api/agents/action-runner/credential/activation", reader)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, organizationID))
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec
	}

	pending := issueActionRunnerCredentialForTest(t, router, hostID, "host-1.local")
	if rec := request("", "default", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing bearer status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if rec := request(pending.Token, "other", ""); rec.Code == http.StatusNoContent {
		t.Fatal("cross-organization bearer returned rollback-authorizing 204")
	}
	if rec := request(pending.Token, "default", `{"agentId":"other","hostname":"other"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("selector body status = %d, body=%s", rec.Code, rec.Body.String())
	}
	for index := range cfg.APITokens {
		if cfg.APITokens[index].ID == pending.TokenID {
			cfg.APITokens[index].Scopes = append(cfg.APITokens[index].Scopes, config.ScopeSettingsWrite)
		}
	}
	if rec := request(pending.Token, "default", ""); rec.Code == http.StatusNoContent {
		t.Fatal("excess-scope runner bearer returned rollback-authorizing 204")
	}
	for index := range cfg.APITokens {
		if cfg.APITokens[index].ID == pending.TokenID {
			cfg.APITokens[index].Scopes = []string{config.ScopeAgentExec}
		}
	}
	legacyRaw := "legacy-cancel-token-1234567890.12345678"
	legacy, err := config.NewAPITokenRecord(legacyRaw, "legacy exec", []string{config.ScopeAgentExec})
	if err != nil {
		t.Fatal(err)
	}
	legacy.OrgID = "default"
	legacy.Metadata = map[string]string{agenttokens.RuntimeRoleMetadataKey: agenttokens.CredentialKindLegacyFullTrust, "bound_agent_id": hostID, "bound_hostname": "host-1.local"}
	cfg.APITokens = append(cfg.APITokens, *legacy)
	if rec := request(legacyRaw, "default", ""); rec.Code == http.StatusNoContent {
		t.Fatal("wrong-role exec bearer returned rollback-authorizing 204")
	}
	for index := range cfg.APITokens {
		if cfg.APITokens[index].ID == legacy.ID {
			cfg.APITokens = append(cfg.APITokens[:index], cfg.APITokens[index+1:]...)
			break
		}
	}
	if rec := request(pending.Token, "default", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("pending cancel status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(cfg.APITokens) != 0 {
		t.Fatalf("cancelled pending credential remained: %#v", cfg.APITokens)
	}
	if rec := request(pending.Token, "default", ""); rec.Code == http.StatusNoContent {
		t.Fatal("removed credential incorrectly returned rollback-authorizing 204")
	}

	active := issueActionRunnerCredentialForTest(t, router, hostID, "host-1.local")
	commitActionRunnerCredentialForTest(t, router, active)
	if rec := request(active.Token, "default", ""); rec.Code != http.StatusConflict {
		t.Fatalf("committed cancel status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != active.TokenID {
		t.Fatalf("committed cancel changed active credential: %#v", cfg.APITokens)
	}
}

func TestActivateActionRunnerCredentialPersistenceFailureKeepsBothCredentialsAndPendingSession(t *testing.T) {
	router, cfg, hostID := newActionRunnerCredentialTestRouter(t)
	router.agentExecServer = agentexec.NewServerWithAdmissionValidator(router.admitAgentExecToken, router.validateAgentExecSession)
	first := issueActionRunnerCredentialForTest(t, router, hostID, "host-1.local")
	commitActionRunnerCredentialForTest(t, router, first)
	second := issueActionRunnerCredentialForTest(t, router, hostID, "host-1.local")
	pendingConn, pendingServer := connectActionRunnerCredentialForTest(t, router.agentExecServer, second)
	defer pendingConn.Close()
	defer pendingServer.Close()
	router.persistence.SetFileSystem(actionRunnerFailingPersistenceFS{})

	rec := requestActionRunnerActivationForTest(t, router, second)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("activation status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(cfg.APITokens) != 2 {
		t.Fatalf("failed activation inventory = %#v", cfg.APITokens)
	}
	if _, ok := router.admitAgentExecToken(first.Token, hostID, first.Hostname); !ok {
		t.Fatal("failed activation revoked the prior credential")
	}
	secondAdmission, ok := router.admitAgentExecToken(second.Token, hostID, second.Hostname)
	if !ok || !secondAdmission.ActivationPending {
		t.Fatalf("failed activation did not restore pending replacement: %#v, %v", secondAdmission, ok)
	}
	if router.agentExecServer.IsAgentConnectedForOrganization("default", hostID) {
		t.Fatal("failed activation made the pending runner dispatchable")
	}
}

func TestActivateActionRunnerCredentialRequiresExactRegisteredSession(t *testing.T) {
	router, cfg, hostID := newActionRunnerCredentialTestRouter(t)
	router.agentExecServer = agentexec.NewServerWithAdmissionValidator(router.admitAgentExecToken, router.validateAgentExecSession)
	issued := issueActionRunnerCredentialForTest(t, router, hostID, "host-1.local")
	rec := requestActionRunnerActivationForTest(t, router, issued)
	if rec.Code != http.StatusConflict {
		t.Fatalf("activation status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != issued.TokenID || cfg.APITokens[0].ExpiresAt == nil || cfg.APITokens[0].Metadata[agenttokens.ActionRunnerActivationPendingMetadataKey] != "true" {
		t.Fatalf("unregistered activation changed inventory = %#v", cfg.APITokens)
	}
}

func TestActivateActionRunnerCredentialRequiresDurablePersistenceWithoutChangingTokenOrSession(t *testing.T) {
	router, cfg, hostID := newActionRunnerCredentialTestRouter(t)
	router.agentExecServer = agentexec.NewServerWithAdmissionValidator(router.admitAgentExecToken, router.validateAgentExecSession)
	issued := issueActionRunnerCredentialForTest(t, router, hostID, "host-1.local")
	pendingConn, pendingServer := connectActionRunnerCredentialForTest(t, router.agentExecServer, issued)
	defer pendingConn.Close()
	defer pendingServer.Close()
	router.persistence = nil
	if rec := requestActionRunnerActivationForTest(t, router, issued); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil persistence activation status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != issued.TokenID || cfg.APITokens[0].ExpiresAt == nil || cfg.APITokens[0].Metadata[agenttokens.ActionRunnerActivationPendingMetadataKey] != "true" {
		t.Fatalf("nil persistence activation changed inventory = %#v", cfg.APITokens)
	}
	admission, ok := router.admitAgentExecToken(issued.Token, issued.AgentID, issued.Hostname)
	if !ok || !admission.ActivationPending || !router.agentExecServer.HasActionRunnerSession(admission) {
		t.Fatalf("nil persistence activation changed pending session = %#v, %v", admission, ok)
	}
}

func TestSelfRevokeActionRunnerCredentialRequiresExactBearerBindingAndClosesSession(t *testing.T) {
	router, cfg, hostID := newActionRunnerCredentialTestRouter(t)
	router.agentExecServer = agentexec.NewServerWithAdmissionValidator(router.admitAgentExecToken, router.validateAgentExecSession)
	issued := issueActionRunnerCredentialForTest(t, router, hostID, "host-1.local")
	commitActionRunnerCredentialForTest(t, router, issued)
	conn, ts := connectActionRunnerCredentialForTest(t, router.agentExecServer, issued)
	defer conn.Close()
	defer ts.Close()

	handler := actionRunnerCredentialRoute(cfg, router.handleIssueActionRunnerCredential, router.handleActivateActionRunnerCredential, router.handleSelfRevokeActionRunnerCredential)
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

func TestSelfRevokeActionRunnerCredentialRequiresDurablePersistence(t *testing.T) {
	router, cfg, hostID := newActionRunnerCredentialTestRouter(t)
	issued := issueActionRunnerCredentialForTest(t, router, hostID, "host-1.local")
	request := func() *httptest.ResponseRecorder {
		body, _ := json.Marshal(actionRunnerCredentialSelfRevokeRequest{AgentID: issued.AgentID, Hostname: issued.Hostname})
		req := httptest.NewRequest(http.MethodDelete, "/api/agents/action-runner/credential", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+issued.Token)
		req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "default"))
		rec := httptest.NewRecorder()
		actionRunnerCredentialRoute(cfg, router.handleIssueActionRunnerCredential, router.handleActivateActionRunnerCredential, router.handleSelfRevokeActionRunnerCredential)(rec, req)
		return rec
	}
	originalPersistence := router.persistence
	router.persistence = nil
	if rec := request(); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil persistence status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != issued.TokenID {
		t.Fatalf("nil persistence mutated inventory: %#v", cfg.APITokens)
	}
	router.persistence = originalPersistence
	router.persistence.SetFileSystem(actionRunnerFailingPersistenceFS{})
	if rec := request(); rec.Code != http.StatusInternalServerError {
		t.Fatalf("failed persistence status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != issued.TokenID {
		t.Fatalf("failed persistence mutated inventory: %#v", cfg.APITokens)
	}
}

func TestActionRunnerCredentialRouteAuthSupportsAdminSessionAndScopedToken(t *testing.T) {
	for _, mode := range []string{"api-token", "admin-session"} {
		t.Run(mode, func(t *testing.T) {
			router, cfg, hostID := newActionRunnerCredentialTestRouter(t)
			handler := actionRunnerCredentialRoute(cfg, router.handleIssueActionRunnerCredential, router.handleActivateActionRunnerCredential, router.handleSelfRevokeActionRunnerCredential)
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

func TestActionRunnerRotationProductionRouterTLSPersistenceRestart(t *testing.T) {
	dataPath := t.TempDir()
	hashedPassword, err := internalauth.HashPassword("production-router-test-password")
	if err != nil {
		t.Fatal(err)
	}
	newRuntime := func(cfg *config.Config) (*Router, *monitoring.Monitor, *httptest.Server) {
		t.Helper()
		monitor, err := monitoring.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		router := NewRouter(cfg, monitor, nil, nil, func() error { return nil }, "test")
		return router, monitor, httptest.NewTLSServer(router.Handler())
	}
	shutdown := func(router *Router, monitor *monitoring.Monitor, server *httptest.Server) {
		server.Close()
		if router.agentExecServer != nil {
			router.agentExecServer.Shutdown()
		}
		router.shutdownBackgroundWorkers()
		router.ShutdownResourceStores()
		router.ShutdownRBAC()
		monitor.StopDiscoveryService()
		monitor.Stop()
	}
	dial := func(server *httptest.Server, credential actionRunnerCredentialResponse) (*websocket.Conn, agentexec.RegisteredPayload) {
		t.Helper()
		transport, ok := server.Client().Transport.(*http.Transport)
		if !ok || transport.TLSClientConfig == nil {
			t.Fatal("TLS test server client has no TLS transport")
		}
		dialer := websocket.Dialer{TLSClientConfig: transport.TLSClientConfig.Clone()}
		conn, _, err := dialer.Dial(wsURLForHTTP(server.URL)+"/api/agent/ws", wsHeadersForHTTP(t, server.URL))
		if err != nil {
			t.Fatal(err)
		}
		message, err := agentexec.NewMessage(agentexec.MsgTypeAgentRegister, "", agentexec.AgentRegisterPayload{
			AgentID: credential.AgentID, Hostname: credential.Hostname, Token: credential.Token,
			RuntimeRole: credential.RuntimeRole, ActionCapability: credential.ActionCapability,
			OperationReceiptVersion: operationreceipt.ProtocolVersion,
		})
		if err != nil {
			conn.Close()
			t.Fatal(err)
		}
		if err := conn.WriteJSON(message); err != nil {
			conn.Close()
			t.Fatal(err)
		}
		return conn, readRegisteredPayload(t, conn)
	}
	issue := func(server *httptest.Server, hostID string) actionRunnerCredentialResponse {
		t.Helper()
		body, _ := json.Marshal(actionRunnerCredentialRequest{AgentID: hostID, Hostname: "host-1.local"})
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/agents/action-runner/credential", bytes.NewReader(body))
		req.SetBasicAuth("admin", "production-router-test-password")
		req.Header.Set("Content-Type", "application/json")
		response, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusCreated {
			payload, _ := io.ReadAll(response.Body)
			t.Fatalf("issue status = %d, body=%s", response.StatusCode, payload)
		}
		var credential actionRunnerCredentialResponse
		if err := json.NewDecoder(response.Body).Decode(&credential); err != nil {
			t.Fatal(err)
		}
		return credential
	}
	activate := func(server *httptest.Server, credential actionRunnerCredentialResponse) {
		t.Helper()
		body, _ := json.Marshal(actionRunnerCredentialSelfRevokeRequest{AgentID: credential.AgentID, Hostname: credential.Hostname})
		req, _ := http.NewRequest(http.MethodPatch, server.URL+"/api/agents/action-runner/credential", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+credential.Token)
		req.Header.Set("Content-Type", "application/json")
		response, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			payload, _ := io.ReadAll(response.Body)
			t.Fatalf("activation status = %d, body=%s", response.StatusCode, payload)
		}
	}
	selfRevoke := func(server *httptest.Server, credential actionRunnerCredentialResponse) {
		t.Helper()
		body, _ := json.Marshal(actionRunnerCredentialSelfRevokeRequest{AgentID: credential.AgentID, Hostname: credential.Hostname})
		req, _ := http.NewRequest(http.MethodDelete, server.URL+"/api/agents/action-runner/credential", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+credential.Token)
		req.Header.Set("Content-Type", "application/json")
		response, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusNoContent {
			payload, _ := io.ReadAll(response.Body)
			t.Fatalf("self-revoke status = %d, body=%s", response.StatusCode, payload)
		}
	}
	requireSocketClosed := func(conn *websocket.Conn, label string) {
		t.Helper()
		if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("set %s socket deadline: %v", label, err)
		}
		if _, _, err := conn.ReadMessage(); err == nil {
			t.Fatalf("%s socket remained readable after exact credential invalidation", label)
		}
	}

	cfg := &config.Config{DataPath: dataPath, ConfigPath: dataPath, AuthUser: "admin", AuthPass: hashedPassword, AllowedOrigins: "*", EnvOverrides: map[string]bool{}}
	router, monitor, server := newRuntime(cfg)
	hostID := seedUnifiedAgentHost(t, monitor)
	first := issue(server, hostID)
	firstConn, registered := dial(server, first)
	if !registered.Success || router.agentExecServer.IsAgentConnectedForOrganization("default", hostID) {
		firstConn.Close()
		shutdown(router, monitor, server)
		t.Fatalf("prepared first registration = %#v", registered)
	}
	activate(server, first)
	if !router.agentExecServer.IsAgentConnectedForOrganization("default", hostID) {
		firstConn.Close()
		shutdown(router, monitor, server)
		t.Fatal("first credential did not become active")
	}

	router.persistence.SetFileSystem(actionRunnerFailingPersistenceFS{})
	failureBody, _ := json.Marshal(actionRunnerCredentialRequest{AgentID: hostID, Hostname: "host-1.local"})
	failureRequest, _ := http.NewRequest(http.MethodPost, server.URL+"/api/agents/action-runner/credential", bytes.NewReader(failureBody))
	failureRequest.SetBasicAuth("admin", "production-router-test-password")
	failureRequest.Header.Set("Content-Type", "application/json")
	failureResponse, err := server.Client().Do(failureRequest)
	if err != nil {
		firstConn.Close()
		shutdown(router, monitor, server)
		t.Fatal(err)
	}
	failurePayload, readErr := io.ReadAll(failureResponse.Body)
	failureResponse.Body.Close()
	if readErr != nil {
		firstConn.Close()
		shutdown(router, monitor, server)
		t.Fatal(readErr)
	}
	if failureResponse.StatusCode != http.StatusInternalServerError || bytes.Contains(failurePayload, []byte(`"token"`)) {
		firstConn.Close()
		shutdown(router, monitor, server)
		t.Fatalf("failed rotation response = status %d body %q", failureResponse.StatusCode, failurePayload)
	}
	persistedAfterFailure, err := config.NewConfigPersistence(dataPath).LoadAPITokens()
	if err != nil {
		firstConn.Close()
		shutdown(router, monitor, server)
		t.Fatal(err)
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != first.TokenID ||
		len(persistedAfterFailure) != 1 || persistedAfterFailure[0].ID != first.TokenID ||
		!router.agentExecServer.IsAgentConnectedForOrganization("default", hostID) {
		firstConn.Close()
		shutdown(router, monitor, server)
		t.Fatalf("failed HTTPS rotation changed durable credential or live session: %#v", persistedAfterFailure)
	}
	router.persistence = config.NewConfigPersistence(dataPath)

	second := issue(server, hostID)
	if _, ok := cfg.ValidateAPIToken(first.Token); !ok {
		firstConn.Close()
		shutdown(router, monitor, server)
		t.Fatal("rotation prepare rejected the first secret")
	}
	secondConn, registered := dial(server, second)
	if !registered.Success || !router.agentExecServer.IsAgentConnectedForOrganization("default", hostID) {
		secondConn.Close()
		firstConn.Close()
		shutdown(router, monitor, server)
		t.Fatalf("prepared second registration = %#v", registered)
	}
	activate(server, second)
	requireSocketClosed(firstConn, "superseded runner")
	if _, ok := cfg.ValidateAPIToken(first.Token); ok {
		secondConn.Close()
		firstConn.Close()
		shutdown(router, monitor, server)
		t.Fatal("activation retained first secret")
	}
	persisted, err := config.NewConfigPersistence(dataPath).LoadAPITokens()
	if err != nil {
		secondConn.Close()
		firstConn.Close()
		shutdown(router, monitor, server)
		t.Fatal(err)
	}
	if len(persisted) != 1 || persisted[0].ID != second.TokenID || persisted[0].ExpiresAt != nil {
		secondConn.Close()
		firstConn.Close()
		shutdown(router, monitor, server)
		t.Fatalf("durably committed rotation inventory = %#v", persisted)
	}
	secondConn.Close()
	shutdown(router, monitor, server)

	restartedConfig := &config.Config{DataPath: dataPath, ConfigPath: dataPath, AuthUser: "admin", AuthPass: hashedPassword, AllowedOrigins: "*", EnvOverrides: map[string]bool{}, APITokens: persisted}
	restartedRouter, restartedMonitor, restartedServer := newRuntime(restartedConfig)
	oldConn, oldRegistration := dial(restartedServer, first)
	oldConn.Close()
	if oldRegistration.Success {
		t.Fatal("durably revoked predecessor registered after server restart")
	}
	newConn, newRegistration := dial(restartedServer, second)
	if !newRegistration.Success || !restartedRouter.agentExecServer.IsAgentConnectedForOrganization("default", hostID) {
		newConn.Close()
		shutdown(restartedRouter, restartedMonitor, restartedServer)
		t.Fatalf("durably activated replacement after restart = %#v", newRegistration)
	}

	selfRevoke(restartedServer, second)
	requireSocketClosed(newConn, "self-revoked runner")
	persisted, err = config.NewConfigPersistence(dataPath).LoadAPITokens()
	if err != nil {
		shutdown(restartedRouter, restartedMonitor, restartedServer)
		t.Fatal(err)
	}
	if len(persisted) != 0 {
		shutdown(restartedRouter, restartedMonitor, restartedServer)
		t.Fatalf("self-revoked credential remained durable: %#v", persisted)
	}
	shutdown(restartedRouter, restartedMonitor, restartedServer)

	finalConfig := &config.Config{DataPath: dataPath, ConfigPath: dataPath, AuthUser: "admin", AuthPass: hashedPassword, AllowedOrigins: "*", EnvOverrides: map[string]bool{}, APITokens: persisted}
	finalRouter, finalMonitor, finalServer := newRuntime(finalConfig)
	defer shutdown(finalRouter, finalMonitor, finalServer)
	revokedConn, revokedRegistration := dial(finalServer, second)
	revokedConn.Close()
	if revokedRegistration.Success {
		t.Fatal("durably self-revoked replacement registered after second server restart")
	}
}
