package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// The privileged-session gate only recognised cfg.AuthUser, so SSO users —
// keyed by their provider-scoped principal — could never reach settings-scoped
// routes, even with an admin group role mapping (#1533, #1535).
func TestEnsureAdminSessionSSOUsers(t *testing.T) {
	dir := t.TempDir()
	InitSessionStore(dir)

	manager, err := internalauth.NewFileManager(t.TempDir())
	if err != nil {
		t.Fatalf("new file manager: %v", err)
	}
	origManager := internalauth.GetManager()
	internalauth.SetManager(manager)
	t.Cleanup(func() { internalauth.SetManager(origManager) })

	newSessionRequest := func(t *testing.T, username string) *http.Request {
		t.Helper()
		token := generateSessionToken()
		GetSessionStore().CreateSession(token, time.Hour, "agent", "127.0.0.1", username)
		req := httptest.NewRequest(http.MethodGet, "/api/security/sso/providers", nil)
		req.AddCookie(&http.Cookie{Name: "pulse_session", Value: token})
		return req
	}

	const ssoAdmin = "sso:oidc:corp:abc123admin"
	const ssoPlain = "sso:oidc:corp:def456plain"

	if err := manager.UpdateUserRoles(ssoAdmin, []string{internalauth.RoleAdmin}); err != nil {
		t.Fatalf("assign admin role: %v", err)
	}
	if err := manager.UpdateUserRoles(ssoPlain, nil); err != nil {
		t.Fatalf("assign empty roles: %v", err)
	}

	cfgWithAdmin := &config.Config{AuthUser: "richard"}
	cfgNoAdmin := &config.Config{}

	tests := []struct {
		name string
		cfg  *config.Config
		user string
		want bool
	}{
		{name: "configured local admin passes", cfg: cfgWithAdmin, user: "richard", want: true},
		{name: "sso user with admin role passes", cfg: cfgWithAdmin, user: ssoAdmin, want: true},
		{name: "sso user without roles is rejected when local admin exists", cfg: cfgWithAdmin, user: ssoPlain, want: false},
		{name: "local non-admin user is rejected", cfg: cfgWithAdmin, user: "mallory", want: false},
		{name: "sso user passes on an instance with no local admin", cfg: cfgNoAdmin, user: ssoPlain, want: true},
		{name: "non-sso session is rejected when no local admin configured", cfg: cfgNoAdmin, user: "mallory", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := newSessionRequest(t, tc.user)
			rec := httptest.NewRecorder()
			got := ensureAdminSession(tc.cfg, rec, req)
			if got != tc.want {
				t.Fatalf("ensureAdminSession(%q) = %v, want %v (status %d)", tc.user, got, tc.want, rec.Code)
			}
			if !tc.want && rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403 for rejected session, got %d", rec.Code)
			}
		})
	}
}

func TestSessionUserHasRBACAdminGrant(t *testing.T) {
	manager, err := internalauth.NewFileManager(t.TempDir())
	if err != nil {
		t.Fatalf("new file manager: %v", err)
	}
	origManager := internalauth.GetManager()
	internalauth.SetManager(manager)
	t.Cleanup(func() { internalauth.SetManager(origManager) })

	if err := manager.UpdateUserRoles("adm", []string{internalauth.RoleAdmin}); err != nil {
		t.Fatalf("assign admin: %v", err)
	}
	if err := manager.UpdateUserRoles("viewer", []string{internalauth.RoleViewer}); err != nil {
		t.Fatalf("assign viewer: %v", err)
	}

	if !sessionUserHasRBACAdminGrant("adm") {
		t.Fatal("expected admin grant for admin-role user")
	}
	if sessionUserHasRBACAdminGrant("viewer") {
		t.Fatal("viewer role must not carry the admin grant")
	}
	if sessionUserHasRBACAdminGrant("") {
		t.Fatal("empty username must not carry the admin grant")
	}
	if sessionUserHasRBACAdminGrant("nobody") {
		t.Fatal("unassigned user must not carry the admin grant")
	}
}
