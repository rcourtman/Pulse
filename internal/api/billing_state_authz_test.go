package api

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func newHostedAuthzTestRouter(t *testing.T) *Router {
	baseDir := t.TempDir()

	hashed, err := internalauth.HashPassword("Password!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	mtp := config.NewMultiTenantPersistence(baseDir)
	if err := mtp.SaveOrganization(&models.Organization{
		ID:          "acme",
		DisplayName: "Acme",
		OwnerUserID: "alice",
		Members: []models.OrganizationMember{
			{UserID: "alice", Role: models.OrgRoleOwner, AddedAt: time.Now()},
		},
	}); err != nil {
		t.Fatalf("seed org acme: %v", err)
	}
	if err := mtp.SaveOrganization(&models.Organization{
		ID:          "beta",
		DisplayName: "Beta",
		OwnerUserID: "bob",
		Members: []models.OrganizationMember{
			{UserID: "bob", Role: models.OrgRoleOwner, AddedAt: time.Now()},
		},
	}); err != nil {
		t.Fatalf("seed org beta: %v", err)
	}

	router := &Router{
		mux:         http.NewServeMux(),
		config:      &config.Config{DataPath: baseDir, AuthUser: "admin", AuthPass: hashed},
		multiTenant: mtp,
		hostedMode:  true,
	}
	router.registerHostedRoutes(nil, nil, nil)
	t.Cleanup(func() {
		if router.signupRateLimiter != nil {
			router.signupRateLimiter.Stop()
		}
	})
	return router
}

func newHostedAuthzNonOwnerRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: "billing-authz-alice-session"})
	// Ensure basic auth isn't accidentally used (would be platform admin).
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:wrong")))
	return req
}

func TestBillingStateAdminEndpoint_NonOwnerGets403(t *testing.T) {
	router := newHostedAuthzTestRouter(t)

	// Authenticate as alice (session user), but attempt to access bob's org.
	sessionToken := "billing-authz-alice-session"
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "alice")

	req := newHostedAuthzNonOwnerRequest(http.MethodGet, "/api/admin/orgs/beta/billing-state", "")

	rec := httptest.NewRecorder()
	router.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHostedLifecycleSuspend_NonOwnerGets403(t *testing.T) {
	router := newHostedAuthzTestRouter(t)

	sessionToken := "billing-authz-alice-session"
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "alice")

	req := newHostedAuthzNonOwnerRequest(http.MethodPost, "/api/admin/orgs/beta/suspend", `{"reason":"test"}`)
	rec := httptest.NewRecorder()
	router.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner suspend attempt, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHostedAgentInstallCommand_NonOwnerGets403(t *testing.T) {
	router := newHostedAuthzTestRouter(t)

	sessionToken := "billing-authz-alice-session"
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "alice")

	req := newHostedAuthzNonOwnerRequest(http.MethodPost, "/api/admin/orgs/beta/agent-install-command", `{"type":"pve"}`)
	rec := httptest.NewRecorder()
	router.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner agent install command attempt, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHostedOrganizationsList_SessionUserGets403(t *testing.T) {
	router := newHostedAuthzTestRouter(t)

	sessionToken := "billing-authz-alice-session"
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "alice")

	req := newHostedAuthzNonOwnerRequest(http.MethodGet, "/api/hosted/organizations", "")
	rec := httptest.NewRecorder()
	router.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-platform session on hosted organizations, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHostedOrganizationsList_ConfiguredAdminSessionGets200(t *testing.T) {
	router := newHostedAuthzTestRouter(t)

	sessionToken := "billing-authz-admin-session"
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "admin")

	req := httptest.NewRequest(http.MethodGet, "/api/hosted/organizations", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	rec := httptest.NewRecorder()
	router.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for configured admin session, got %d: %s", rec.Code, rec.Body.String())
	}
}
