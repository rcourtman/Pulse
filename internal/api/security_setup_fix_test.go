package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/bootstrap"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	var matched *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == name {
			matched = cookie
		}
	}
	return matched
}

func TestQuickSecuritySetupRejectsUnauthenticatedForce(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

	hashed, err := internalauth.HashPassword("OldPassword!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	cfg := &config.Config{
		AuthUser:   "admin",
		AuthPass:   hashed,
		DataPath:   t.TempDir(),
		ConfigPath: t.TempDir(),
	}

	router := &Router{config: cfg}
	handler := handleQuickSecuritySetupFixed(router)

	// Ensure rate limiter does not block test runs
	authLimiter.Reset("198.51.100.42")

	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup",
		strings.NewReader(`{"username":"newadmin","password":"NewPassword!1","apiToken":"abcd","force":true}`))
	req.RemoteAddr = "198.51.100.42:54321"

	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauthorized, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestQuickSecuritySetupRequiresBootstrapToken(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

	dataDir := t.TempDir()

	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
	}

	router := &Router{
		config:      cfg,
		persistence: config.NewConfigPersistence(cfg.DataPath),
	}
	router.initializeBootstrapToken()
	InitPersistentAuthStores(dataDir)

	tokenPath := filepath.Join(cfg.DataPath, bootstrapTokenFilename)
	bootstrapToken, _, _, err := bootstrap.Load(cfg.DataPath)
	if err != nil {
		t.Fatalf("load bootstrap token: %v", err)
	}

	handler := handleQuickSecuritySetupFixed(router)

	authLimiter.Reset("127.0.0.1")

	payload := `{"username":"bootstrap","password":"StrongPass!1","apiToken":"` + strings.Repeat("aa", 32) + `"}`

	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(payload))
	req.RemoteAddr = "127.0.0.1:54321"
	rr := httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauthorized without bootstrap token, got %d (%s)", rr.Code, rr.Body.String())
	}

	// Remote IP without a bootstrap token is rejected — loopback is the only
	// no-token fallback.
	authLimiter.Reset("198.51.100.80")

	reqRemoteNoToken := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(payload))
	reqRemoteNoToken.RemoteAddr = "198.51.100.80:54321"
	rrRemoteNoToken := httptest.NewRecorder()
	handler(rrRemoteNoToken, reqRemoteNoToken)
	if rrRemoteNoToken.Code != http.StatusForbidden {
		t.Fatalf("expected 403 forbidden for remote quick setup without bootstrap token, got %d (%s)", rrRemoteNoToken.Code, rrRemoteNoToken.Body.String())
	}

	authLimiter.Reset("127.0.0.1")

	reqLoopback := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(payload))
	reqLoopback.RemoteAddr = "127.0.0.1:54321"
	reqLoopback.Header.Set(bootstrapTokenHeader, bootstrapToken)

	rrLoopback := httptest.NewRecorder()
	handler(rrLoopback, reqLoopback)
	if rrLoopback.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with valid bootstrap token from loopback, got %d (%s)", rrLoopback.Code, rrLoopback.Body.String())
	}

	cookies := rrLoopback.Result().Cookies()
	sessionCookie := findCookie(cookies, sessionCookieName(false))
	if sessionCookie == nil || strings.TrimSpace(sessionCookie.Value) == "" {
		t.Fatalf("expected quick setup to establish a browser session cookie")
	}
	if !ValidateSession(sessionCookie.Value) {
		t.Fatalf("expected established session to validate")
	}
	if got := GetSessionUsername(sessionCookie.Value); got != "bootstrap" {
		t.Fatalf("session username=%q, want %q", got, "bootstrap")
	}

	csrfCookie := findCookie(cookies, CookieNameCSRF)
	if csrfCookie == nil || strings.TrimSpace(csrfCookie.Value) == "" {
		t.Fatalf("expected quick setup to issue a CSRF cookie")
	}

	if _, err := os.Stat(tokenPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected bootstrap token file to be removed after successful setup, got err=%v", err)
	}
	if router.bootstrapTokenHash != "" {
		t.Fatalf("expected bootstrap token hash to be cleared after successful setup")
	}
}

func TestQuickSecuritySetupRejectsUnsafeUsernamesBeforeStateChanges(t *testing.T) {
	tests := []struct {
		name     string
		username string
	}{
		{name: "systemd directive injection", username: "admin\n[Service]\nExecStartPre=/bin/false"},
		{name: "environment quote injection", username: "admin'\nPULSE_AUTH_PASS='attacker"},
		{name: "leading whitespace", username: " admin"},
		{name: "unsupported punctuation", username: "admin;restart"},
		{name: "too long", username: strings.Repeat("a", maxLocalAuthUsernameRunes+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
			resetTrustedProxyConfig()

			dataDir := t.TempDir()
			cfg := &config.Config{DataPath: dataDir, ConfigPath: dataDir}
			router := &Router{
				config:      cfg,
				persistence: config.NewConfigPersistence(dataDir),
			}
			router.initializeBootstrapToken()
			InitPersistentAuthStores(dataDir)
			bootstrapToken, _, _, err := bootstrap.Load(dataDir)
			if err != nil {
				t.Fatalf("load bootstrap token: %v", err)
			}

			body, err := json.Marshal(map[string]string{
				"username": tt.username,
				"password": "StrongPass!1",
				"apiToken": strings.Repeat("ab", 24),
			})
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}

			authLimiter.Reset("198.51.100.91")
			req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(string(body)))
			req.RemoteAddr = "198.51.100.91:54321"
			req.Header.Set(bootstrapTokenHeader, bootstrapToken)
			rec := httptest.NewRecorder()

			handleQuickSecuritySetupFixed(router)(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
			}
			if !strings.Contains(strings.ToLower(rec.Body.String()), "username") {
				t.Fatalf("response = %q, want username validation error", rec.Body.String())
			}
			if cfg.AuthUser != "" || cfg.AuthPass != "" || cfg.HasAPITokens() {
				t.Fatalf("unsafe username changed runtime auth state: user=%q pass_set=%v tokens=%d", cfg.AuthUser, cfg.AuthPass != "", len(cfg.APITokens))
			}
			if _, err := os.Stat(resolveAuthEnvPath(dataDir)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unsafe username created auth env file: %v", err)
			}
		})
	}
}

func TestAuthConfigurationRenderersRejectStructuralInjection(t *testing.T) {
	if _, err := quoteAuthEnvValue("admin'\nPULSE_AUTH_PASS='attacker"); err == nil {
		t.Fatal("quoteAuthEnvValue accepted structural injection")
	}
	if _, err := quoteSystemdEnvironment("PULSE_AUTH_USER", "admin\nExecStartPre=/bin/false"); err == nil {
		t.Fatal("quoteSystemdEnvironment accepted structural injection")
	}

	generatedAt := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)
	content, err := renderSystemdAuthOverride(generatedAt, "admin@example.com", "$2a$10$abcdefghijklmnopqrstuvwxyzABCDE1234567890123456789012")
	if err != nil {
		t.Fatalf("renderSystemdAuthOverride: %v", err)
	}
	if strings.Count(string(content), "\nEnvironment=") != 3 {
		t.Fatalf("rendered override does not contain exactly three environment directives:\n%s", content)
	}
}

func TestValidateBootstrapTokenEndpoint(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

	dataDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
	}

	router := &Router{config: cfg}
	router.initializeBootstrapToken()

	tokenPath := filepath.Join(cfg.DataPath, bootstrapTokenFilename)
	token, _, _, err := bootstrap.Load(cfg.DataPath)
	if err != nil {
		t.Fatalf("load bootstrap token: %v", err)
	}

	handler := http.HandlerFunc(router.handleValidateBootstrapToken)

	// GET not allowed
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/security/validate-bootstrap-token", nil)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for GET, got %d", rr.Code)
	}

	// Missing token payload
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/security/validate-bootstrap-token", strings.NewReader("{}"))
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing token, got %d (%s)", rr.Code, rr.Body.String())
	}

	// Invalid token
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/security/validate-bootstrap-token", strings.NewReader(`{"token":"deadbeef"}`))
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid token, got %d (%s)", rr.Code, rr.Body.String())
	}

	// Valid token
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/security/validate-bootstrap-token", strings.NewReader(`{"token":"`+token+`"}`))
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for valid token, got %d (%s)", rr.Code, rr.Body.String())
	}

	// Bootstrap token should remain on disk after validation
	if _, err := os.Stat(tokenPath); err != nil {
		t.Fatalf("bootstrap token should remain after validation, got err=%v", err)
	}

	// Once token removed, endpoint should report conflict
	router.clearBootstrapToken()
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/security/validate-bootstrap-token", strings.NewReader(`{"token":"`+token+`"}`))
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 when bootstrap token unavailable, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestValidateBootstrapTokenEndpoint_RateLimited(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

	dataDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
	}

	router := &Router{
		config:                          cfg,
		bootstrapTokenValidationLimiter: NewRateLimiter(1, time.Hour),
	}
	t.Cleanup(router.bootstrapTokenValidationLimiter.Stop)
	router.initializeBootstrapToken()

	handler := http.HandlerFunc(router.handleValidateBootstrapToken)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-bootstrap-token", strings.NewReader(`{"token":"deadbeef"}`))
	req.RemoteAddr = "127.0.0.1:1234"
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for first invalid token, got %d (%s)", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/security/validate-bootstrap-token", strings.NewReader(`{"token":"deadbeef"}`))
	req.RemoteAddr = "127.0.0.1:1234"
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after exhausting bootstrap token validation limit, got %d (%s)", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on bootstrap token validation rate limit")
	}
}

func TestQuickSecuritySetupAllowsRecoveryTokenRotation(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()
	resetRecoveryStore()

	dataDir := t.TempDir()

	hashed, err := internalauth.HashPassword("OldPassword!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	cfg := &config.Config{
		AuthUser:   "admin",
		AuthPass:   hashed,
		DataPath:   dataDir,
		ConfigPath: dataDir,
	}

	persistence := config.NewConfigPersistence(cfg.DataPath)

	router := &Router{
		config:      cfg,
		persistence: persistence,
	}

	InitRecoveryTokenStore(cfg.DataPath)
	token, err := GetRecoveryTokenStore().GenerateRecoveryToken(5*time.Minute, "127.0.0.1")
	if err != nil {
		t.Fatalf("generate recovery token: %v", err)
	}

	authLimiter.Reset("127.0.0.1")

	body := `{"username":"rotated","password":"RotatedPassword!1","apiToken":"deadbeef","force":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(body))
	req.Header.Set("X-Recovery-Token", token)
	req.RemoteAddr = "127.0.0.1:54321"

	rr := httptest.NewRecorder()
	handler := handleQuickSecuritySetupFixed(router)
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rr.Code, rr.Body.String())
	}

	if cfg.AuthUser != "rotated" {
		t.Fatalf("expected AuthUser to be rotated, got %q", cfg.AuthUser)
	}
	if !internalauth.CheckPasswordHash("RotatedPassword!1", cfg.AuthPass) {
		t.Fatalf("stored password hash does not match new password")
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("expected one API token, got %d", len(cfg.APITokens))
	}
}

func TestQuickSecuritySetupRequiresSettingsScopeForTokens(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

	dataDir := t.TempDir()
	hashed, err := internalauth.HashPassword("ExistingPassword!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	rawToken := strings.Repeat("ab", 32) // 64 hex chars
	record, err := config.NewAPITokenRecord(rawToken, "limited", []string{config.ScopeAgentReport})
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}

	cfg := &config.Config{
		AuthUser:   "admin",
		AuthPass:   hashed,
		DataPath:   dataDir,
		ConfigPath: dataDir,
		APITokens:  []config.APITokenRecord{*record},
	}
	cfg.SortAPITokens()

	router := &Router{
		config:      cfg,
		persistence: config.NewConfigPersistence(cfg.DataPath),
	}

	authLimiter.Reset("203.0.113.10")

	body := `{"username":"rotated","password":"RotatedPassword!1","apiToken":"deadbeef","force":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(body))
	req.RemoteAddr = "203.0.113.10:1234"
	req.Header.Set("X-API-Token", rawToken)

	rr := httptest.NewRecorder()
	handler := handleQuickSecuritySetupFixed(router)
	handler(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 forbidden for missing scope, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestRegenerateAPITokenRequiresSettingsScope(t *testing.T) {
	dataDir := t.TempDir()
	hashed, err := internalauth.HashPassword("ExistingPassword!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	rawToken := strings.Repeat("cd", 32)
	record, err := config.NewAPITokenRecord(rawToken, "limited", []string{config.ScopeAgentReport})
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}

	cfg := &config.Config{
		AuthUser:   "admin",
		AuthPass:   hashed,
		DataPath:   dataDir,
		ConfigPath: dataDir,
		APITokens:  []config.APITokenRecord{*record},
	}
	cfg.SortAPITokens()

	router := &Router{
		config:      cfg,
		persistence: config.NewConfigPersistence(cfg.DataPath),
	}

	authLimiter.Reset("198.51.100.7")
	req := httptest.NewRequest(http.MethodPost, "/api/security/regenerate-token", nil)
	req.RemoteAddr = "198.51.100.7:44321"
	req.Header.Set("X-API-Token", rawToken)

	rr := httptest.NewRecorder()
	router.HandleRegenerateAPIToken(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 forbidden for missing scope, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestValidateAPITokenRequiresSettingsScope(t *testing.T) {
	dataDir := t.TempDir()
	hashed, err := internalauth.HashPassword("ExistingPassword!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	rawToken := strings.Repeat("ef", 32)
	record, err := config.NewAPITokenRecord(rawToken, "limited", []string{config.ScopeAgentReport})
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}

	cfg := &config.Config{
		AuthUser:   "admin",
		AuthPass:   hashed,
		DataPath:   dataDir,
		ConfigPath: dataDir,
		APITokens:  []config.APITokenRecord{*record},
	}
	cfg.SortAPITokens()

	router := &Router{
		config:      cfg,
		persistence: config.NewConfigPersistence(cfg.DataPath),
	}

	authLimiter.Reset("198.51.100.8")
	body := `{"token":"abcdef"}`
	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-token", strings.NewReader(body))
	req.RemoteAddr = "198.51.100.8:54321"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", rawToken)

	rr := httptest.NewRecorder()
	router.HandleValidateAPIToken(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 forbidden for missing scope, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestResetLockoutRequiresSettingsScope(t *testing.T) {
	dataDir := t.TempDir()
	hashed, err := internalauth.HashPassword("ExistingPassword!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	rawToken := strings.Repeat("12", 32)
	record, err := config.NewAPITokenRecord(rawToken, "limited", []string{config.ScopeAgentReport})
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}

	cfg := &config.Config{
		AuthUser:   "admin",
		AuthPass:   hashed,
		DataPath:   dataDir,
		ConfigPath: dataDir,
		APITokens:  []config.APITokenRecord{*record},
	}
	cfg.SortAPITokens()

	router := &Router{
		config: cfg,
	}

	authLimiter.Reset("198.51.100.9")
	body := `{"identifier":"admin"}`
	req := httptest.NewRequest(http.MethodPost, "/api/security/reset-lockout", strings.NewReader(body))
	req.RemoteAddr = "198.51.100.9:54321"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", rawToken)

	rr := httptest.NewRecorder()
	router.handleResetLockout(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 forbidden for missing scope, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestEnsureSettingsWriteScopeWithValidScope(t *testing.T) {
	dataDir := t.TempDir()

	rawToken := strings.Repeat("ab", 32)
	record, err := config.NewAPITokenRecord(rawToken, "admin-token", []string{config.ScopeSettingsWrite})
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}

	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
		APITokens:  []config.APITokenRecord{*record},
	}
	cfg.SortAPITokens()

	req := httptest.NewRequest(http.MethodPost, "/api/security/test", nil)
	attachAPITokenRecord(req, record)

	rr := httptest.NewRecorder()
	result := ensureSettingsWriteScope(cfg, rr, req)

	if !result {
		t.Error("ensureSettingsWriteScope should return true when token has settings:write scope")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected no error response, got status %d", rr.Code)
	}
}

func TestEnsureSettingsWriteScopeRejectsNonAdminSession(t *testing.T) {
	cfg := &config.Config{
		AuthUser: "admin",
	}

	sessionToken := "settings-write-member-" + time.Now().Format("150405.000000000")
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "member")

	req := httptest.NewRequest(http.MethodPost, "/api/security/test", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})

	rr := httptest.NewRecorder()
	result := ensureSettingsWriteScope(cfg, rr, req)

	if result {
		t.Fatal("ensureSettingsWriteScope should reject non-admin session users")
	}
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin session, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Admin privileges required") {
		t.Fatalf("expected admin privileges error, got %q", rr.Body.String())
	}
}

func TestEnsureSettingsWriteScopeAllowsConfiguredAdminSession(t *testing.T) {
	cfg := &config.Config{
		AuthUser: "admin",
	}

	sessionToken := "settings-write-admin-" + time.Now().Format("150405.000000001")
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "admin")

	req := httptest.NewRequest(http.MethodPost, "/api/security/test", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})

	rr := httptest.NewRecorder()
	result := ensureSettingsWriteScope(cfg, rr, req)

	if !result {
		t.Fatal("ensureSettingsWriteScope should allow configured admin session users")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin session, got %d", rr.Code)
	}
}

func TestEnsureSettingsWriteScopeAllowsOrgManagerSession(t *testing.T) {
	cfg := &config.Config{
		AuthUser: "platform-admin",
	}

	sessionToken := "settings-write-owner-" + time.Now().Format("150405.000000002")
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "legacy-owner")

	req := httptest.NewRequest(http.MethodPost, "/api/security/test", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	req = req.WithContext(context.WithValue(req.Context(), OrgContextKey, &models.Organization{
		ID:          "org-a",
		DisplayName: "Org A",
		OwnerUserID: "operator-owner",
		Members: []models.OrganizationMember{
			{UserID: "legacy-owner", Role: models.OrgRoleOwner},
			{UserID: "operator-owner", Role: models.OrgRoleOwner},
		},
	}))

	rr := httptest.NewRecorder()
	result := ensureSettingsWriteScope(cfg, rr, req)

	if !result {
		t.Fatal("ensureSettingsWriteScope should allow org manager session users")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for org manager session, got %d", rr.Code)
	}
}

func TestValidateBcryptHash(t *testing.T) {
	tests := []struct {
		name    string
		hash    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid $2a$ hash",
			hash:    "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
			wantErr: false,
		},
		{
			name:    "valid $2b$ hash",
			hash:    "$2b$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
			wantErr: false,
		},
		{
			name:    "valid $2y$ hash",
			hash:    "$2y$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
			wantErr: false,
		},
		{
			name:    "truncated hash (59 chars)",
			hash:    "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhW",
			wantErr: true,
			errMsg:  "expected 60 characters, got 59",
		},
		{
			name:    "too long hash (61 chars)",
			hash:    "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWyX",
			wantErr: true,
			errMsg:  "expected 60 characters, got 61",
		},
		{
			name:    "empty hash",
			hash:    "",
			wantErr: true,
			errMsg:  "expected 60 characters, got 0",
		},
		{
			name:    "wrong prefix (md5 style, 60 chars)",
			hash:    "$1$saltsalt$WabcdEFGhijKLMnopQRstuV0123456789012345123456789",
			wantErr: true,
			errMsg:  "must start with $2a$, $2b$, or $2y$",
		},
		{
			name:    "wrong prefix ($2x$)",
			hash:    "$2x$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",
			wantErr: true,
			errMsg:  "must start with $2a$, $2b$, or $2y$",
		},
		{
			name:    "no prefix at all (60 chars)",
			hash:    "N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy1234567",
			wantErr: true,
			errMsg:  "must start with $2a$, $2b$, or $2y$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBcryptHash(tt.hash)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateBcryptHash() expected error, got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validateBcryptHash() error = %q, want substring %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateBcryptHash() unexpected error = %v", err)
				}
			}
		})
	}
}
