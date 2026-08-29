package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/agenttokens"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

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

func TestActionRunnerCredentialRouteAuthSupportsAdminSessionAndScopedToken(t *testing.T) {
	for _, mode := range []string{"api-token", "admin-session"} {
		t.Run(mode, func(t *testing.T) {
			router, cfg, hostID := newActionRunnerCredentialTestRouter(t)
			handler := RequireAdmin(cfg, RequireScope(config.ScopeSettingsWrite, RequireScope(config.ScopeActionsExecute, router.handleIssueActionRunnerCredential)))
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
