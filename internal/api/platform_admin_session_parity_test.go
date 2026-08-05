package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"golang.org/x/crypto/bcrypt"
)

func platformAdminSession(t *testing.T, user string) *http.Cookie {
	t.Helper()
	tok := "platform-admin-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	GetSessionStore().CreateSession(tok, time.Hour, "browser", "127.0.0.1", user)
	return &http.Cookie{Name: sessionCookieName(false), Value: tok}
}

func platformAdminConfig(t *testing.T, adminUser string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{DataPath: dir, ConfigPath: dir}
	if adminUser != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte("platform-admin-password"), bcrypt.DefaultCost)
		if err != nil {
			t.Fatalf("bcrypt: %v", err)
		}
		cfg.AuthUser = adminUser
		cfg.AuthPass = string(hashed)
	}
	return cfg
}

func billingAdminCapability(t *testing.T, router *Router, user string) bool {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/security/status", nil)
	req.AddCookie(platformAdminSession(t, user))
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("security status: %v", err)
	}
	caps, ok := payload["settingsCapabilities"].(map[string]any)
	if !ok {
		t.Fatalf("settingsCapabilities missing from %s", rec.Body.String())
	}
	return caps["billingAdmin"] == true
}

// The billingAdmin capability is published from canAccessPlatformAdminSurface,
// which treats any instance administrator as a platform admin. The route behind
// it compared the session user against cfg.AuthUser alone, so on an instance
// whose only administrators are SSO principals the UI offered the surface and
// the route refused it.
func TestPlatformAdminRouteAgreesWithBillingAdminCapability(t *testing.T) {
	prev := auth.GetAuthorizer()
	auth.SetAuthorizer(&auth.DefaultAuthorizer{})
	defer auth.SetAuthorizer(prev)

	cfg := platformAdminConfig(t, "")
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	ssoOwner := "sso:owner@example.com"

	if !billingAdminCapability(t, router, ssoOwner) {
		t.Fatal("precondition: billingAdmin must be advertised to the OIDC-only administrator")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/hosted/organizations", nil)
	req.AddCookie(platformAdminSession(t, ssoOwner))
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("platform admin route refused the caller its own capability advertised (body %s)", rec.Body.String())
	}
}

// An org-scoped tenant session is not an instance administrator, whatever its
// username. Without this the SSO fallback in sessionUserCarriesAdminPrivileges
// would make every tenant on a control plane with no local admin a platform
// admin, which is a far worse failure than the one being fixed.
func TestPlatformAdminRouteRefusesOrgScopedTenantSession(t *testing.T) {
	prev := auth.GetAuthorizer()
	auth.SetAuthorizer(&auth.DefaultAuthorizer{})
	defer auth.SetAuthorizer(prev)

	cfg := platformAdminConfig(t, "")
	tenant := "sso:tenant@example.com"

	handlerReached := false
	guarded := RequirePlatformAdmin(cfg, func(w http.ResponseWriter, r *http.Request) {
		handlerReached = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/hosted/organizations", nil)
	req.AddCookie(platformAdminSession(t, tenant))
	req = req.WithContext(context.WithValue(req.Context(), OrgContextKey, &models.Organization{
		ID:          "tenant-org",
		DisplayName: "Tenant Org",
	}))
	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)

	if handlerReached || rec.Code != http.StatusForbidden {
		t.Fatalf("org-scoped tenant session reached the platform admin surface (code %d)", rec.Code)
	}

	// The same principal without an org binding is the instance administrator.
	req2 := httptest.NewRequest(http.MethodGet, "/api/hosted/organizations", nil)
	req2.AddCookie(platformAdminSession(t, tenant))
	rec2 := httptest.NewRecorder()
	guarded.ServeHTTP(rec2, req2)
	if rec2.Code == http.StatusForbidden {
		t.Fatal("instance-scoped SSO administrator must still reach the platform admin surface")
	}
}

// An unrelated SSO principal on an instance that does configure a local admin
// is not an administrator and must stay refused.
func TestPlatformAdminRouteRefusesUnrelatedSSOUser(t *testing.T) {
	prev := auth.GetAuthorizer()
	auth.SetAuthorizer(&auth.DefaultAuthorizer{})
	defer auth.SetAuthorizer(prev)

	cfg := platformAdminConfig(t, "admin")
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	outsider := "sso:outsider@example.com"

	if billingAdminCapability(t, router, outsider) {
		t.Fatal("precondition: billingAdmin must not be advertised to a non-admin")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/hosted/organizations", nil)
	req.AddCookie(platformAdminSession(t, outsider))
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unrelated SSO user on the platform admin route = %d, want 403", rec.Code)
	}
}
