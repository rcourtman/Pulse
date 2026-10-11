package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// Quick setup with force rotates the administrator username and password and
// replaces every API token with a full-access one. With authentication already
// configured, a token caller therefore needs instance-wide authority:
// settings:write alone must not be enough to take the instance over.
//
// The password in these requests is deliberately too weak. A caller that
// passes the authority check is then stopped by password validation with a
// 400, before any credentials are written or a session is created, so the
// tests prove which side of the authority check a caller lands on without
// touching shared session state.
func quickSetupWithTokenCaller(t *testing.T, callerScopes []string, callerOrg string) (*config.Config, *httptest.ResponseRecorder, string) {
	t.Helper()

	dir := t.TempDir()
	hash, err := internalauth.HashPassword("OriginalPass!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	const rawCaller = "quick-setup-caller-token.12345678"
	caller, err := config.NewAPITokenRecord(rawCaller, "caller", callerScopes)
	if err != nil {
		t.Fatalf("new caller record: %v", err)
	}
	if callerOrg != "" {
		caller.OrgID = callerOrg
	}
	cfg := &config.Config{
		DataPath: dir, ConfigPath: dir,
		AuthUser: "original-admin", AuthPass: hash,
		APITokens: []config.APITokenRecord{*caller},
	}
	cfg.SortAPITokens()
	router := &Router{config: cfg, persistence: config.NewConfigPersistence(dir)}

	body := `{"username":"new-admin","password":"weak","apiToken":"` + strings.Repeat("ab", 32) + `","force":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-API-Token", rawCaller)
	rec := httptest.NewRecorder()
	authLimiter.Reset("127.0.0.1")
	handleQuickSecuritySetupFixed(router)(rec, req)
	return cfg, rec, caller.ID
}

func TestQuickSecuritySetupRefusesTokenCallersWithoutInstanceWideAuthority(t *testing.T) {
	for _, tc := range []struct {
		name   string
		scopes []string
		org    string
	}{
		{"settings:write only", []string{config.ScopeSettingsWrite}, ""},
		{"settings read and write", []string{config.ScopeSettingsRead, config.ScopeSettingsWrite}, ""},
		{"full access bound to another organization", []string{config.ScopeWildcard}, "org-a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, rec, callerID := quickSetupWithTokenCaller(t, tc.scopes, tc.org)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d (%s), want 403 from the authority check", rec.Code, rec.Body.String())
			}
			if cfg.AuthUser != "original-admin" {
				t.Fatalf("administrator username changed to %q", cfg.AuthUser)
			}
			if !internalauth.CheckPasswordHash("OriginalPass!1", cfg.AuthPass) {
				t.Fatal("administrator password was replaced")
			}
			if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != callerID {
				t.Fatalf("token inventory changed: %+v", cfg.APITokens)
			}
			if _, err := os.Stat(resolveAuthEnvPath(cfg.ConfigPath)); err == nil {
				t.Fatal("a refused setup still wrote the authentication file")
			}
		})
	}
}

func TestQuickSecuritySetupLetsAnInstanceWideTokenReachValidation(t *testing.T) {
	_, rec, _ := quickSetupWithTokenCaller(t, []string{config.ScopeWildcard}, "")

	// Past the authority check, the weak password is rejected as a bad request.
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d (%s), want 400 from password validation", rec.Code, rec.Body.String())
	}
}
