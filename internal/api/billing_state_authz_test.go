package api

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestBillingStateAdminEndpoint_NonOwnerGets403(t *testing.T) {
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

	// Authenticate as alice (session user), but attempt to access bob's org.
	sessionToken := "billing-authz-alice-session"
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "alice")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/orgs/beta/billing-state", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	// Ensure basic auth isn't accidentally used (would be platform admin).
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("admin:wrong")))

	rec := httptest.NewRecorder()
	router.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner, got %d: %s", rec.Code, rec.Body.String())
	}
}
