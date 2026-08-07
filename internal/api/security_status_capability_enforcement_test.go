package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"golang.org/x/crypto/bcrypt"
)

func newCapabilityConfig(t *testing.T, adminUser string) *config.Config {
	t.Helper()
	tempDir := t.TempDir()
	cfg := &config.Config{DataPath: tempDir, ConfigPath: tempDir}
	if adminUser != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte("capability-password"), bcrypt.DefaultCost)
		if err != nil {
			t.Fatalf("bcrypt: %v", err)
		}
		cfg.AuthUser = adminUser
		cfg.AuthPass = string(hashed)
	}
	return cfg
}

func capabilitySessionCookie(t *testing.T, username string) *http.Cookie {
	t.Helper()
	token := "capability-session-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	GetSessionStore().CreateSession(token, time.Hour, "browser", "127.0.0.1", username)
	return &http.Cookie{Name: sessionCookieName(false), Value: token}
}

func fetchSettingsCapabilities(t *testing.T, router *Router, cookie *http.Cookie) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("security status = %d, want 200", rec.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal security status: %v", err)
	}
	caps, ok := payload["settingsCapabilities"].(map[string]any)
	if !ok {
		t.Fatalf("settingsCapabilities missing from %s", rec.Body.String())
	}
	return caps
}

// A settings capability is a promise the routes have to keep. Without an RBAC
// licence Authorize allows everything, so capabilities derived from it alone
// reported true while ensureSettingsScope refused the matching request, and the
// API Access and Single Sign-On tabs rendered for users who then got a 403 and
// an error toast. Refs discussion #1672.
func TestSettingsCapabilitiesMatchRouteEnforcementWithoutRBAC(t *testing.T) {
	prevAuthorizer := auth.GetAuthorizer()
	auth.SetAuthorizer(&auth.DefaultAuthorizer{})
	defer auth.SetAuthorizer(prevAuthorizer)

	cfg := newCapabilityConfig(t, "admin")
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	caps := fetchSettingsCapabilities(t, router, capabilitySessionCookie(t, "sso:outsider@example.com"))
	for _, key := range []string{
		"apiAccessRead",
		"apiAccessWrite",
		"singleSignOnRead",
		"singleSignOnWrite",
		"authenticationRead",
		"authenticationWrite",
		"infrastructureRead",
		// Sibling of infrastructureRead for the admin-only System tabs. Same
		// derivation today, but each names its own surface so tightening one
		// gate cannot silently hide the other's tabs.
		"systemSettingsRead",
	} {
		if caps[key] != false {
			t.Fatalf("%s = %v for a non-admin session, want false", key, caps[key])
		}
	}

	// Every capability the non-admin was refused must in fact be refused by the
	// route, otherwise the fix has swung into hiding a usable surface.
	for _, probe := range []struct{ method, path string }{
		{http.MethodGet, "/api/security/tokens"},
		{http.MethodGet, "/api/security/sso/providers"},
		// Settings → Infrastructure reads these on mount and then polls two of
		// them, so a withheld infrastructureRead has to line up with a refusal.
		{http.MethodGet, "/api/config/nodes"},
		{http.MethodGet, "/api/system/settings"},
	} {
		req := httptest.NewRequest(probe.method, probe.path, nil)
		req.AddCookie(capabilitySessionCookie(t, "sso:outsider@example.com"))
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s = %d, want 403 to match the withheld capability", probe.method, probe.path, rec.Code)
		}
	}
}

// The configured admin must keep every surface, or the fix hides settings from
// the only account that can reach them.
func TestSettingsCapabilitiesGrantConfiguredAdminWithoutRBAC(t *testing.T) {
	prevAuthorizer := auth.GetAuthorizer()
	auth.SetAuthorizer(&auth.DefaultAuthorizer{})
	defer auth.SetAuthorizer(prevAuthorizer)

	cfg := newCapabilityConfig(t, "admin")
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	caps := fetchSettingsCapabilities(t, router, capabilitySessionCookie(t, "admin"))
	for _, key := range []string{"apiAccessRead", "apiAccessWrite", "singleSignOnRead", "singleSignOnWrite", "infrastructureRead", "systemSettingsRead"} {
		if caps[key] != true {
			t.Fatalf("%s = %v for the configured admin, want true", key, caps[key])
		}
	}
}

// On the OIDC-only pattern there is no local admin, so SSO principals are the
// instance's administrators. They must keep the tabs, and the routes agree.
func TestSettingsCapabilitiesGrantSSOAdminWhenNoLocalAdminConfigured(t *testing.T) {
	prevAuthorizer := auth.GetAuthorizer()
	auth.SetAuthorizer(&auth.DefaultAuthorizer{})
	defer auth.SetAuthorizer(prevAuthorizer)

	cfg := newCapabilityConfig(t, "")
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	caps := fetchSettingsCapabilities(t, router, capabilitySessionCookie(t, "sso:owner@example.com"))
	if caps["apiAccessRead"] != true {
		t.Fatalf("apiAccessRead = %v on an OIDC-only instance, want true", caps["apiAccessRead"])
	}

	req := httptest.NewRequest(http.MethodGet, "/api/security/tokens", nil)
	req.AddCookie(capabilitySessionCookie(t, "sso:owner@example.com"))
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code == http.StatusForbidden {
		t.Fatal("OIDC-only instance must not refuse its own SSO administrator")
	}
}
