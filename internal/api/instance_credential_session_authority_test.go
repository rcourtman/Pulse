package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// Regenerating every API token and force-running quick setup replace the
// instance's own credentials. An organization owner manages their own
// organization's settings, but naming that organization in the request must not
// let them rotate the instance administrator login or mint a full-access
// default-organization token.

const (
	credentialTestAdmin    = "instance-admin"
	credentialTestOrgOwner = "tenant-owner"
)

func credentialAuthorityConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	InitSessionStore(dir)
	hash, err := internalauth.HashPassword("OriginalPass!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	existing, err := config.NewAPITokenRecord("existing-admin-token.12345678", "existing", nil)
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}
	return &config.Config{
		DataPath: dir, ConfigPath: dir,
		AuthUser: credentialTestAdmin, AuthPass: hash,
		APITokens: []config.APITokenRecord{*existing},
	}
}

// sessionCredentialRequest builds a session request; a non-empty orgID binds it
// to that organization, owned by credentialTestOrgOwner, as the tenant
// middleware would for an X-Pulse-Org-ID header.
func sessionCredentialRequest(t *testing.T, target, body, username, orgID string) *http.Request {
	t.Helper()
	token := generateSessionToken()
	GetSessionStore().CreateSession(token, time.Hour, "agent", "127.0.0.1", username)
	req := newLoopbackRequest(http.MethodPost, target, strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: token})
	if orgID != "" {
		org := &models.Organization{ID: orgID, DisplayName: orgID, OwnerUserID: credentialTestOrgOwner}
		req = req.WithContext(context.WithValue(req.Context(), apicontext.OrgContextKey, org))
	}
	authLimiter.Reset("127.0.0.1")
	return req
}

func assertInstanceCredentialsUnchanged(t *testing.T, cfg *config.Config, tokenID string) {
	t.Helper()
	if cfg.AuthUser != credentialTestAdmin {
		t.Fatalf("administrator username changed to %q", cfg.AuthUser)
	}
	if !internalauth.CheckPasswordHash("OriginalPass!1", cfg.AuthPass) {
		t.Fatal("administrator password was replaced")
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != tokenID {
		t.Fatalf("token inventory changed: %+v", cfg.APITokens)
	}
}

func TestRegenerateAPITokenRefusesOrganizationOwnerSession(t *testing.T) {
	cfg := credentialAuthorityConfig(t)
	tokenID := cfg.APITokens[0].ID
	router := &Router{config: cfg, persistence: config.NewConfigPersistence(cfg.DataPath)}

	req := sessionCredentialRequest(t, "/api/security/regenerate-token", "", credentialTestOrgOwner, "org-a")
	rec := httptest.NewRecorder()
	router.HandleRegenerateAPIToken(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d (%s), want 403", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "pulse_") {
		t.Fatalf("refused response carried a token: %s", rec.Body.String())
	}
	assertInstanceCredentialsUnchanged(t, cfg, tokenID)
}

func TestRegenerateAPITokenAllowsInstanceAdminSession(t *testing.T) {
	cfg := credentialAuthorityConfig(t)
	router := &Router{config: cfg, persistence: config.NewConfigPersistence(cfg.DataPath)}

	req := sessionCredentialRequest(t, "/api/security/regenerate-token", "", credentialTestAdmin, "")
	rec := httptest.NewRecorder()
	router.HandleRegenerateAPIToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", rec.Code, rec.Body.String())
	}
}

func TestQuickSecuritySetupForceRefusesOrganizationOwnerSession(t *testing.T) {
	cfg := credentialAuthorityConfig(t)
	tokenID := cfg.APITokens[0].ID
	router := &Router{config: cfg, persistence: config.NewConfigPersistence(cfg.DataPath)}

	// A strong password: only the authority check stands between this caller
	// and the credential write.
	body := `{"username":"attacker","password":"Attacker-Pass-2026!x","apiToken":"` + strings.Repeat("ab", 32) + `","force":true}`
	req := sessionCredentialRequest(t, "/api/security/quick-setup", body, credentialTestOrgOwner, "org-a")
	rec := httptest.NewRecorder()
	handleQuickSecuritySetupFixed(router)(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d (%s), want 403", rec.Code, rec.Body.String())
	}
	assertInstanceCredentialsUnchanged(t, cfg, tokenID)
	if _, err := os.Stat(resolveAuthEnvPath(cfg.ConfigPath)); err == nil {
		t.Fatal("a refused setup still wrote the authentication file")
	}
}

func TestQuickSecuritySetupForceLetsInstanceAdminSessionReachValidation(t *testing.T) {
	cfg := credentialAuthorityConfig(t)
	router := &Router{config: cfg, persistence: config.NewConfigPersistence(cfg.DataPath)}

	// Past the authority check, the weak password is rejected as a bad request
	// before anything is written.
	body := `{"username":"new-admin","password":"weak","apiToken":"` + strings.Repeat("ab", 32) + `","force":true}`
	req := sessionCredentialRequest(t, "/api/security/quick-setup", body, credentialTestAdmin, "")
	rec := httptest.NewRecorder()
	handleQuickSecuritySetupFixed(router)(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d (%s), want 400 from password validation", rec.Code, rec.Body.String())
	}
}
