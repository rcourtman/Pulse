package api

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"golang.org/x/crypto/bcrypt"
)

func adminParitySession(t *testing.T, user string) *http.Cookie {
	t.Helper()
	tok := "admin-parity-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	GetSessionStore().CreateSession(tok, time.Hour, "browser", "127.0.0.1", user)
	return &http.Cookie{Name: sessionCookieName(false), Value: tok}
}

func adminParityConfig(t *testing.T, adminUser string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	// An API token is present so the export/import path treats auth as required
	// on the OIDC-only instance too, which is the shape that actually 403s.
	record, err := config.NewAPITokenRecord("admin-parity-token-123.12345678", "parity", []string{config.ScopeSettingsWrite})
	if err != nil {
		t.Fatalf("NewAPITokenRecord: %v", err)
	}
	cfg := &config.Config{DataPath: dir, ConfigPath: dir, APITokens: []config.APITokenRecord{*record}}
	if adminUser != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte("admin-parity-password"), bcrypt.DefaultCost)
		if err != nil {
			t.Fatalf("bcrypt: %v", err)
		}
		cfg.AuthUser = adminUser
		cfg.AuthPass = string(hashed)
	}
	return cfg
}

func exportStatusFor(t *testing.T, router *Router, user string) int {
	t.Helper()
	cookie := adminParitySession(t, user)
	req := httptest.NewRequest(http.MethodPost, "/api/config/export",
		strings.NewReader(`{"passphrase":"long-enough-passphrase"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	req.Header.Set("X-CSRF-Token", generateCSRFToken(cookie.Value))
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	return rec.Code
}

// SSO-only deployments retain a usable administration path through an
// explicit RBAC administrator grant. Authentication without that grant is not
// instance administration.
func TestSSOOnlyExplicitAdminReachesEveryAdminGuard(t *testing.T) {
	prev := auth.GetAuthorizer()
	auth.SetAuthorizer(&auth.DefaultAuthorizer{})
	defer auth.SetAuthorizer(prev)

	cfg := adminParityConfig(t, "")
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	manager := installTestRBACManager(t)
	ssoAdmin := "sso:owner@example.com"
	if err := manager.UpdateUserRoles(ssoAdmin, []string{auth.RoleAdmin}); err != nil {
		t.Fatalf("assign SSO administrator role: %v", err)
	}

	if !sessionUserCarriesAdminPrivileges(cfg, ssoAdmin) {
		t.Fatal("precondition: the canonical helper must treat this principal as an admin")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.AddCookie(adminParitySession(t, ssoAdmin))
	if !canCapturePublicURL(cfg, req) {
		t.Error("canCapturePublicURL refused the explicit SSO administrator")
	}

	h := &DiscoveryHandlers{config: cfg}
	req2 := httptest.NewRequest(http.MethodGet, "/api/discovery/status", nil)
	req2.AddCookie(adminParitySession(t, ssoAdmin))
	if !h.isAdminRequest(req2) {
		t.Error("discovery isAdminRequest refused the explicit SSO administrator")
	}

	if code := exportStatusFor(t, router, ssoAdmin); code == http.StatusForbidden {
		t.Error("config export refused the explicitly authorized SSO administrator")
	}

	ssoViewer := "sso:viewer@example.com"
	if sessionUserCarriesAdminPrivileges(cfg, ssoViewer) {
		t.Fatal("unassigned SSO user inherited instance administration")
	}
	req3 := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req3.AddCookie(adminParitySession(t, ssoViewer))
	if canCapturePublicURL(cfg, req3) {
		t.Error("public URL capture admitted an unassigned SSO user")
	}
	req4 := httptest.NewRequest(http.MethodGet, "/api/discovery/status", nil)
	req4.AddCookie(adminParitySession(t, ssoViewer))
	if h.isAdminRequest(req4) {
		t.Error("discovery admitted an unassigned SSO user")
	}
	if code := exportStatusFor(t, router, ssoViewer); code != http.StatusForbidden {
		t.Errorf("config export for an unassigned SSO user = %d, want 403", code)
	}
}

// The parity fix must not widen anything. On an instance that does configure a
// local admin, an unrelated SSO principal is not an administrator and every one
// of these guards must still refuse them.
func TestUnrelatedSSOUserStillRefusedWhenLocalAdminConfigured(t *testing.T) {
	prev := auth.GetAuthorizer()
	auth.SetAuthorizer(&auth.DefaultAuthorizer{})
	defer auth.SetAuthorizer(prev)

	cfg := adminParityConfig(t, "admin")
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	outsider := "sso:outsider@example.com"

	if sessionUserCarriesAdminPrivileges(cfg, outsider) {
		t.Fatal("precondition: an unrelated SSO user must not be an admin when a local admin is configured")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.AddCookie(adminParitySession(t, outsider))
	if canCapturePublicURL(cfg, req) {
		t.Error("canCapturePublicURL admitted a non-admin SSO user")
	}

	h := &DiscoveryHandlers{config: cfg}
	req2 := httptest.NewRequest(http.MethodGet, "/api/discovery/status", nil)
	req2.AddCookie(adminParitySession(t, outsider))
	if h.isAdminRequest(req2) {
		t.Error("discovery isAdminRequest admitted a non-admin SSO user")
	}

	if code := exportStatusFor(t, router, outsider); code != http.StatusForbidden {
		t.Errorf("config export for a non-admin SSO user = %d, want 403", code)
	}

	// The configured admin is unaffected.
	if code := exportStatusFor(t, router, "admin"); code == http.StatusForbidden {
		t.Error("config export refused the configured local admin")
	}
}
