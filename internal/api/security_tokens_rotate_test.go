package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
	authpkg "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestSecurityTokens_RotatePreservesBindingsAndMetadata(t *testing.T) {
	// Ensure cached bypass state does not leak into other tests (adminBypassEnabled() is memoized).
	t.Cleanup(resetAdminBypassState)

	t.Setenv("ALLOW_ADMIN_BYPASS", "1")
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("NODE_ENV", "")
	resetAdminBypassState()

	capture := &auditCaptureLogger{}
	prevLogger := audit.GetLogger()
	prevManager := GetTenantAuditManager()
	audit.SetLogger(capture)
	SetTenantAuditManager(nil)
	t.Cleanup(func() {
		audit.SetLogger(prevLogger)
		SetTenantAuditManager(prevManager)
	})

	old := newTokenRecord(t, "rotate-test-token-123.12345678", []string{"*"}, map[string]string{
		"bound_agent_id": "agent-1",
	})
	old.Name = "rotated-token"
	old.OrgID = "acme"
	old.OrgIDs = []string{"acme", "beta"}

	cfg := newTestConfigWithTokens(t, old)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	router.authorizer = &adminOnlyAuthorizer{}

	req := httptest.NewRequest(http.MethodPost, "/api/security/tokens/"+old.ID+"/rotate", nil)
	req = req.WithContext(authpkg.WithUser(req.Context(), "rotator"))
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Token  string      `json:"token"`
		Record apiTokenDTO `json:"record"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Token == "" {
		t.Fatalf("expected token in response")
	}
	if resp.Record.ID == "" || resp.Record.ID == old.ID {
		t.Fatalf("expected new token id, got %q (old=%q)", resp.Record.ID, old.ID)
	}
	if resp.Record.Name != old.Name {
		t.Fatalf("name = %q, want %q", resp.Record.Name, old.Name)
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("APITokens len = %d, want 1", len(cfg.APITokens))
	}
	if cfg.APITokens[0].ID != resp.Record.ID {
		t.Fatalf("stored token id = %q, want %q", cfg.APITokens[0].ID, resp.Record.ID)
	}
	if cfg.APITokens[0].Name != old.Name {
		t.Fatalf("stored token name = %q, want %q", cfg.APITokens[0].Name, old.Name)
	}
	if got := cfg.APITokens[0].OrgID; got != "acme" {
		t.Fatalf("stored token org = %q, want %q", got, "acme")
	}
	if got := cfg.APITokens[0].OrgIDs; len(got) != 2 || got[0] != "acme" || got[1] != "beta" {
		t.Fatalf("stored token orgIDs = %v, want [acme beta]", got)
	}
	if got := cfg.APITokens[0].Metadata["bound_agent_id"]; got != "agent-1" {
		t.Fatalf("stored metadata bound_agent_id = %q, want %q", got, "agent-1")
	}

	events, err := capture.Query(audit.QueryFilter{})
	if err != nil {
		t.Fatalf("query audit events: %v", err)
	}

	var rotateEvent *audit.Event
	for i := range events {
		if events[i].EventType == "token_rotated" && events[i].Success {
			rotateEvent = &events[i]
			break
		}
	}
	if rotateEvent == nil {
		t.Fatalf("expected successful token_rotated audit event")
	}
	if rotateEvent.User == "" {
		t.Fatalf("expected token_rotated audit event to include user")
	}
	if strings.Contains(rotateEvent.Details, resp.Token) {
		t.Fatalf("token_rotated audit details leaked raw token")
	}
}

func TestSecurityTokens_RotateRejectsScopeEscalation(t *testing.T) {
	old := newTokenRecord(t, "rotate-target-token-123.12345678", []string{"*"}, nil)
	old.Name = "target-token"
	old.OrgID = "acme"

	caller := newTokenRecord(t, "rotate-caller-token-123.12345678", []string{config.ScopeSettingsWrite}, nil)
	caller.Name = "limited-caller"
	caller.OrgID = "acme"

	cfg := newTestConfigWithTokens(t, old, caller)
	router := &Router{
		config:      cfg,
		persistence: config.NewConfigPersistence(t.TempDir()),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/tokens/"+old.ID+"/rotate", nil)
	req = req.WithContext(authpkg.WithAPIToken(req.Context(), &caller))
	rec := httptest.NewRecorder()
	router.handleRotateAPIToken(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Cannot rotate token with scope") {
		t.Fatalf("expected scope escalation denial, got %q", rec.Body.String())
	}
	if len(cfg.APITokens) != 2 {
		t.Fatalf("APITokens len = %d, want 2", len(cfg.APITokens))
	}
	tokenIDs := []string{cfg.APITokens[0].ID, cfg.APITokens[1].ID}
	slices.Sort(tokenIDs)
	expectedIDs := []string{caller.ID, old.ID}
	slices.Sort(expectedIDs)
	if !slices.Equal(tokenIDs, expectedIDs) {
		t.Fatalf("token set changed after denied rotation: %+v", cfg.APITokens)
	}
}
