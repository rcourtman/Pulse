package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
	authpkg "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestSecurityTokens_UpdateScopesPreservesCredentialAndAuditsTransition(t *testing.T) {
	capture := &auditCaptureLogger{}
	prevLogger := audit.GetLogger()
	prevManager := GetTenantAuditManager()
	audit.SetLogger(capture)
	SetTenantAuditManager(nil)
	t.Cleanup(func() {
		audit.SetLogger(prevLogger)
		SetTenantAuditManager(prevManager)
	})

	record := newTokenRecord(t, "update-target-token-123.12345678", []string{config.ScopeSettingsRead}, map[string]string{
		"bound_agent_id": "agent-1",
	})
	record.Name = "automation"
	record.OrgID = "acme"
	record.OrgIDs = []string{"acme", "beta"}
	originalHash := record.Hash
	originalCreatedAt := record.CreatedAt

	cfg := newTestConfigWithTokens(t, record)
	router := &Router{
		config:      cfg,
		persistence: config.NewConfigPersistence(t.TempDir()),
	}

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/security/tokens/"+record.ID,
		strings.NewReader(`{"scopes":["monitoring:write","monitoring:read","monitoring:write"]}`),
	)
	req = req.WithContext(authpkg.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()
	router.handleUpdateAPIToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("APITokens len = %d, want 1", len(cfg.APITokens))
	}

	updated := cfg.APITokens[0]
	if updated.ID != record.ID || updated.Hash != originalHash || !updated.CreatedAt.Equal(originalCreatedAt) {
		t.Fatalf("credential identity changed during scope update: %+v", updated)
	}
	if updated.Name != "automation" || updated.OrgID != "acme" || len(updated.OrgIDs) != 2 {
		t.Fatalf("token metadata or org bindings changed during scope update: %+v", updated)
	}
	if updated.Metadata["bound_agent_id"] != "agent-1" {
		t.Fatalf("token metadata changed during scope update: %+v", updated.Metadata)
	}
	if got := strings.Join(updated.Scopes, ","); got != "monitoring:read,monitoring:write" {
		t.Fatalf("stored scopes = %q, want %q", got, "monitoring:read,monitoring:write")
	}

	var response struct {
		Record apiTokenDTO `json:"record"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if response.Record.ID != record.ID {
		t.Fatalf("response token ID = %q, want %q", response.Record.ID, record.ID)
	}
	if got := strings.Join(response.Record.Scopes, ","); got != "monitoring:read,monitoring:write" {
		t.Fatalf("response scopes = %q, want canonical scopes", got)
	}

	events, err := capture.Query(audit.QueryFilter{})
	if err != nil {
		t.Fatalf("query audit events: %v", err)
	}
	var updateEvent *audit.Event
	for i := range events {
		if events[i].EventType == "token_updated" && events[i].Success {
			updateEvent = &events[i]
			break
		}
	}
	if updateEvent == nil {
		t.Fatal("expected successful token_updated audit event")
	}
	if updateEvent.User != "alice" {
		t.Fatalf("audit user = %q, want alice", updateEvent.User)
	}
	if !strings.Contains(updateEvent.Details, "old_scopes=settings:read") ||
		!strings.Contains(updateEvent.Details, "new_scopes=monitoring:read,monitoring:write") {
		t.Fatalf("audit details do not record scope transition: %q", updateEvent.Details)
	}
	if strings.Contains(updateEvent.Details, originalHash) {
		t.Fatal("audit details leaked token hash")
	}
}

func TestSecurityTokens_UpdateScopesValidatesRequest(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "wrong method",
			method:     http.MethodPost,
			path:       "/api/security/tokens/token-1",
			body:       `{"scopes":["monitoring:read"]}`,
			wantStatus: http.StatusMethodNotAllowed,
			wantBody:   "Method not allowed",
		},
		{
			name:       "missing token id",
			method:     http.MethodPatch,
			path:       "/api/security/tokens/",
			body:       `{"scopes":["monitoring:read"]}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "Token ID required",
		},
		{
			name:       "invalid body",
			method:     http.MethodPatch,
			path:       "/api/security/tokens/token-1",
			body:       `{bad`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid request body",
		},
		{
			name:       "missing scopes",
			method:     http.MethodPatch,
			path:       "/api/security/tokens/token-1",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "Scopes field required",
		},
		{
			name:       "empty scopes",
			method:     http.MethodPatch,
			path:       "/api/security/tokens/token-1",
			body:       `{"scopes":[]}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "select at least one scope",
		},
		{
			name:       "unknown scope",
			method:     http.MethodPatch,
			path:       "/api/security/tokens/token-1",
			body:       `{"scopes":["unknown:scope"]}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   `unknown scope "unknown:scope"`,
		},
		{
			name:       "wildcard combination",
			method:     http.MethodPatch,
			path:       "/api/security/tokens/token-1",
			body:       `{"scopes":["*","monitoring:read"]}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "wildcard '*' cannot be combined",
		},
		{
			name:       "missing token",
			method:     http.MethodPatch,
			path:       "/api/security/tokens/missing",
			body:       `{"scopes":["monitoring:read"]}`,
			wantStatus: http.StatusNotFound,
			wantBody:   "Token not found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &config.Config{
				APITokens: []config.APITokenRecord{{
					ID:     "token-1",
					Name:   "target",
					Hash:   "hash",
					Scopes: []string{config.ScopeSettingsRead},
					OrgID:  "default",
				}},
			}
			router := &Router{config: cfg}
			req := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			rec := httptest.NewRecorder()

			router.handleUpdateAPIToken(rec, req)

			if rec.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d (body=%q)", rec.Code, test.wantStatus, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), test.wantBody) {
				t.Fatalf("body = %q, want fragment %q", rec.Body.String(), test.wantBody)
			}
			if got := strings.Join(cfg.APITokens[0].Scopes, ","); got != config.ScopeSettingsRead {
				t.Fatalf("scopes changed after invalid request: %q", got)
			}
		})
	}
}

func TestSecurityTokens_UpdateScopesRejectsScopeEscalationForTokenCaller(t *testing.T) {
	t.Run("caller cannot mutate broader existing token", func(t *testing.T) {
		target := newTokenRecord(t, "update-broad-target-123.12345678", []string{config.ScopeWildcard}, nil)
		caller := newTokenRecord(t, "update-limited-caller-123.12345678", []string{
			config.ScopeSettingsWrite,
			config.ScopeMonitoringRead,
		}, nil)
		cfg := newTestConfigWithTokens(t, target, caller)
		router := &Router{config: cfg}

		req := httptest.NewRequest(
			http.MethodPatch,
			"/api/security/tokens/"+target.ID,
			strings.NewReader(`{"scopes":["monitoring:read"]}`),
		)
		req = req.WithContext(authpkg.WithAPIToken(req.Context(), &caller))
		rec := httptest.NewRecorder()
		router.handleUpdateAPIToken(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusForbidden, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `Cannot update token with scope "*"`) {
			t.Fatalf("unexpected response: %q", rec.Body.String())
		}
		if got := strings.Join(findAPITokenByID(t, cfg.APITokens, target.ID).Scopes, ","); got != config.ScopeWildcard {
			t.Fatalf("target scopes changed after denied update: %q", got)
		}
	})

	t.Run("caller cannot grant scope it does not hold", func(t *testing.T) {
		target := newTokenRecord(t, "update-target-123456.12345678", []string{config.ScopeMonitoringRead}, nil)
		caller := newTokenRecord(t, "update-caller-123456.12345678", []string{
			config.ScopeSettingsWrite,
			config.ScopeMonitoringRead,
		}, nil)
		cfg := newTestConfigWithTokens(t, target, caller)
		router := &Router{config: cfg}

		req := httptest.NewRequest(
			http.MethodPatch,
			"/api/security/tokens/"+target.ID,
			strings.NewReader(`{"scopes":["monitoring:read","monitoring:write"]}`),
		)
		req = req.WithContext(authpkg.WithAPIToken(req.Context(), &caller))
		rec := httptest.NewRecorder()
		router.handleUpdateAPIToken(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusForbidden, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `Cannot grant scope "monitoring:write"`) {
			t.Fatalf("unexpected response: %q", rec.Body.String())
		}
		if got := strings.Join(findAPITokenByID(t, cfg.APITokens, target.ID).Scopes, ","); got != config.ScopeMonitoringRead {
			t.Fatalf("target scopes changed after denied update: %q", got)
		}
	})

	t.Run("caller can narrow itself", func(t *testing.T) {
		target := newTokenRecord(t, "update-self-token-123.12345678", []string{
			config.ScopeSettingsWrite,
			config.ScopeMonitoringRead,
		}, nil)
		cfg := newTestConfigWithTokens(t, target)
		router := &Router{config: cfg}

		req := httptest.NewRequest(
			http.MethodPatch,
			"/api/security/tokens/"+target.ID,
			strings.NewReader(`{"scopes":["settings:write"]}`),
		)
		req = req.WithContext(authpkg.WithAPIToken(req.Context(), &target))
		rec := httptest.NewRecorder()
		router.handleUpdateAPIToken(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		if got := strings.Join(cfg.APITokens[0].Scopes, ","); got != config.ScopeSettingsWrite {
			t.Fatalf("self-narrowed scopes = %q, want %q", got, config.ScopeSettingsWrite)
		}
	})
}

func TestSecurityTokens_UpdateScopesRollsBackWhenPersistenceFails(t *testing.T) {
	record := newTokenRecord(t, "update-rollback-token-123.12345678", []string{config.ScopeSettingsRead}, nil)
	cfg := newTestConfigWithTokens(t, record)

	stateDir := filepath.Join(t.TempDir(), "state")
	persistence := config.NewConfigPersistence(stateDir)
	if err := os.RemoveAll(stateDir); err != nil {
		t.Fatalf("remove persistence directory: %v", err)
	}
	if err := os.WriteFile(stateDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create persistence blocker: %v", err)
	}
	router := &Router{config: cfg, persistence: persistence}

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/security/tokens/"+record.ID,
		strings.NewReader(`{"scopes":["monitoring:read"]}`),
	)
	rec := httptest.NewRecorder()
	router.handleUpdateAPIToken(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	if got := strings.Join(cfg.APITokens[0].Scopes, ","); got != config.ScopeSettingsRead {
		t.Fatalf("scopes after failed persistence = %q, want rollback to %q", got, config.ScopeSettingsRead)
	}
}

func findAPITokenByID(t *testing.T, tokens []config.APITokenRecord, tokenID string) config.APITokenRecord {
	t.Helper()
	for _, token := range tokens {
		if token.ID == tokenID {
			return token
		}
	}
	t.Fatalf("token %q not found", tokenID)
	return config.APITokenRecord{}
}
