package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	authpkg "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// An organization manager may pair a phone, but the relay mobile scope stands
// in for the action scopes, so the token must not carry action authority its
// minter's own session does not have.
func TestRelayMobileTokenCarriesNoMoreActionAuthorityThanItsMinter(t *testing.T) {
	defer SetMultiTenantEnabled(false)
	SetMultiTenantEnabled(true)
	t.Setenv("PULSE_DEV", "true")

	dataDir := t.TempDir()
	hashed, err := authpkg.HashPassword("Password!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
		AuthUser:   "platform-admin",
		AuthPass:   hashed,
	}
	org := &models.Organization{
		ID:          "org-a",
		DisplayName: "Org A",
		OwnerUserID: "org-owner",
		Members: []models.OrganizationMember{
			{UserID: "org-owner", Role: models.OrgRoleOwner, AddedAt: time.Now()},
		},
	}
	if err := config.NewMultiTenantPersistence(dataDir).SaveOrganization(org); err != nil {
		t.Fatalf("save organization: %v", err)
	}

	router := newMultiTenantRouter(t, cfg)
	setLicenseTierForHandlersForTests(t, router.licenseHandlers, "org-a", pkglicensing.TierRelay)
	setLicenseTierForHandlersForTests(t, router.licenseHandlers, "default", pkglicensing.TierRelay)

	mint := func(t *testing.T, user, orgID string) (string, config.APITokenRecord) {
		t.Helper()
		sessionToken := "relay-mobile-" + user + "-" + strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339Nano), ":", "-")
		GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", user)

		req := httptest.NewRequest(http.MethodPost, "/api/security/tokens/relay-mobile", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", generateCSRFToken(sessionToken))
		req.Header.Set("X-Pulse-Org-ID", orgID)
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: sessionToken})
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("mint as %s: status = %d, want %d: %s", user, rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			Token  string      `json:"token"`
			Record apiTokenDTO `json:"record"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
			t.Fatalf("decode mint response: %v", err)
		}
		config.Mu.RLock()
		defer config.Mu.RUnlock()
		for _, stored := range cfg.APITokens {
			if stored.ID == payload.Record.ID {
				return payload.Token, stored.Clone()
			}
		}
		t.Fatalf("minted token %s is not stored", payload.Record.ID)
		return "", config.APITokenRecord{}
	}
	request := func(rawToken, orgID, method, path string) int {
		req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Token", rawToken)
		req.Header.Set("X-Pulse-Org-ID", orgID)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		return rec.Code
	}
	actionRoutes := []struct{ method, path string }{
		{http.MethodGet, "/api/actions"},
		{http.MethodGet, "/api/actions/pending"},
		{http.MethodGet, "/api/actions/action-1"},
		{http.MethodPost, "/api/actions/action-1/decision"},
		{http.MethodPost, "/api/actions/action-1/execute"},
		{http.MethodGet, "/api/ai/approvals"},
		{http.MethodPost, "/api/ai/approvals/approval-1/approve"},
		{http.MethodPost, "/api/ai/approvals/approval-1/deny"},
	}

	managerToken, managerRecord := mint(t, "org-owner", "org-a")
	if !tokenActionsWithheld(&managerRecord) {
		t.Fatalf("organization manager's mobile token is not marked: %v", managerRecord.Metadata)
	}
	for _, capability := range []string{authpkg.ActionPlan, authpkg.ActionApprove, authpkg.ActionExecute} {
		if tokenHasActionCapabilityScope(&managerRecord, capability) {
			t.Fatalf("organization manager's mobile token grants the %s capability", capability)
		}
	}
	for _, route := range actionRoutes {
		if code := request(managerToken, "org-a", route.method, route.path); code != http.StatusForbidden {
			t.Fatalf("organization manager's mobile token on %s %s = %d, want %d", route.method, route.path, code, http.StatusForbidden)
		}
	}
	// The rest of the mobile surface stays with the paired phone.
	if code := request(managerToken, "org-a", http.MethodGet, "/api/ai/patrol/attention"); code == http.StatusForbidden || code == http.StatusUnauthorized {
		t.Fatalf("organization manager's mobile token on GET /api/ai/patrol/attention = %d, want it admitted", code)
	}

	adminToken, adminRecord := mint(t, "platform-admin", "default")
	if tokenActionsWithheld(&adminRecord) {
		t.Fatalf("administrator's mobile token is marked: %v", adminRecord.Metadata)
	}
	if !tokenHasActionCapabilityScope(&adminRecord, authpkg.ActionApprove) || !tokenHasActionCapabilityScope(&adminRecord, authpkg.ActionExecute) {
		t.Fatal("administrator's mobile token lost its action capability")
	}
	for _, route := range actionRoutes {
		if code := request(adminToken, "default", route.method, route.path); code == http.StatusForbidden || code == http.StatusUnauthorized {
			t.Fatalf("administrator's mobile token on %s %s = %d, want it admitted", route.method, route.path, code)
		}
	}
}
