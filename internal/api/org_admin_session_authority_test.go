package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// ensureAdminSession guards instance-wide state: credentials, API tokens, SSO,
// system settings, updates and the rest of instance administration. An
// organization's owner or admin manages that organization, not the instance,
// so only routes that opt in with ensureOrgAdminSession / RequireOrgAdmin admit
// them. Before this split every admin route admitted an org manager who named
// their organization in the request, which let them rotate the instance
// administrator's API token and receive its secret.

const (
	orgAuthorityAdmin  = "instance-admin"
	orgAuthorityOwner  = "tenant-owner"
	orgAuthorityAdmin2 = "tenant-admin"
	orgAuthorityMember = "tenant-member"
	orgAuthorityRBAC   = "sso:oidc:corp:rbac-admin"
)

func orgAuthorityConfig(t *testing.T) *config.Config {
	t.Helper()
	InitSessionStore(t.TempDir())
	manager := installTestRBACManager(t)
	if err := manager.UpdateUserRoles(orgAuthorityRBAC, []string{internalauth.RoleAdmin}); err != nil {
		t.Fatalf("assign admin role: %v", err)
	}
	return &config.Config{AuthUser: orgAuthorityAdmin}
}

func orgAuthorityRequest(t *testing.T, method, target, username, orgID string) *http.Request {
	t.Helper()
	token := generateSessionToken()
	GetSessionStore().CreateSession(token, time.Hour, "agent", "127.0.0.1", username)
	req := newLoopbackRequest(method, target, nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: token})
	if orgID != "" {
		org := &models.Organization{
			ID:          orgID,
			DisplayName: orgID,
			OwnerUserID: orgAuthorityOwner,
			Members: []models.OrganizationMember{
				{UserID: orgAuthorityAdmin2, Role: models.OrgRoleAdmin},
				{UserID: orgAuthorityMember, Role: models.OrgRoleViewer},
			},
		}
		req = req.WithContext(context.WithValue(req.Context(), apicontext.OrgContextKey, org))
	}
	return req
}

type orgAuthorityCase struct {
	name     string
	user     string
	org      string
	instance bool // admitted by ensureAdminSession / RequireAdmin
	orgBound bool // admitted by ensureOrgAdminSession / RequireOrgAdmin
}

var orgAuthorityCases = []orgAuthorityCase{
	{"configured admin, instance context", orgAuthorityAdmin, "", true, true},
	{"RBAC admin, instance context", orgAuthorityRBAC, "", true, true},
	{"configured admin inside a tenant org", orgAuthorityAdmin, "org-a", true, true},
	{"RBAC admin inside a tenant org", orgAuthorityRBAC, "org-a", true, true},
	{"org owner inside their org", orgAuthorityOwner, "org-a", false, true},
	{"org admin inside their org", orgAuthorityAdmin2, "org-a", false, true},
	{"org member inside their org", orgAuthorityMember, "org-a", false, false},
	{"org owner without an org", orgAuthorityOwner, "", false, false},
	{"unknown user without an org", "mallory", "", false, false},
}

func TestAdminSessionChecksSplitInstanceAndOrgAuthority(t *testing.T) {
	cfg := orgAuthorityConfig(t)
	for _, tc := range orgAuthorityCases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			if got := ensureAdminSession(cfg, rec, orgAuthorityRequest(t, http.MethodGet, "/api/x", tc.user, tc.org)); got != tc.instance {
				t.Fatalf("ensureAdminSession = %v, want %v", got, tc.instance)
			}
			if !tc.instance && rec.Code != http.StatusForbidden {
				t.Fatalf("refused instance check answered %d, want 403", rec.Code)
			}
			rec = httptest.NewRecorder()
			if got := ensureOrgAdminSession(cfg, rec, orgAuthorityRequest(t, http.MethodGet, "/api/x", tc.user, tc.org)); got != tc.orgBound {
				t.Fatalf("ensureOrgAdminSession = %v, want %v", got, tc.orgBound)
			}
		})
	}
}

func TestRequireAdminAndRequireOrgAdminFollowTheSessionSplit(t *testing.T) {
	cfg := orgAuthorityConfig(t)
	reached := func(wrap func(*config.Config, http.HandlerFunc) http.HandlerFunc, req *http.Request) (bool, int) {
		hit := false
		rec := httptest.NewRecorder()
		wrap(cfg, func(w http.ResponseWriter, r *http.Request) { hit = true })(rec, req)
		return hit, rec.Code
	}
	for _, tc := range orgAuthorityCases {
		t.Run(tc.name, func(t *testing.T) {
			if hit, code := reached(RequireAdmin, orgAuthorityRequest(t, http.MethodGet, "/api/x", tc.user, tc.org)); hit != tc.instance {
				t.Fatalf("RequireAdmin reached handler = %v (status %d), want %v", hit, code, tc.instance)
			}
			if hit, code := reached(RequireOrgAdmin, orgAuthorityRequest(t, http.MethodGet, "/api/x", tc.user, tc.org)); hit != tc.orgBound {
				t.Fatalf("RequireOrgAdmin reached handler = %v (status %d), want %v", hit, code, tc.orgBound)
			}
		})
	}
}

// Token management, SSO, system settings and the other instance surfaces
// reach their handlers through the settings scope helpers. An org owner's
// session must not pass them, whatever organization it names.
func TestSettingsScopeRefusesOrgOwnerSession(t *testing.T) {
	cfg := orgAuthorityConfig(t)
	for _, check := range []struct {
		name string
		fn   func(*config.Config, http.ResponseWriter, *http.Request) bool
	}{
		{"settings read", ensureSettingsReadScope},
		{"settings write", ensureSettingsWriteScope},
	} {
		t.Run(check.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := orgAuthorityRequest(t, http.MethodPost, "/api/security/tokens/some-id/rotate", orgAuthorityOwner, "org-a")
			if check.fn(cfg, rec, req) {
				t.Fatal("an org owner's session passed the instance settings scope check")
			}
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", rec.Code)
			}
		})
	}
}
