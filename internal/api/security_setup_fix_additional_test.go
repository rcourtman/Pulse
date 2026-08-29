package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/bootstrap"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type failingWriteFileSystem struct{}

func (failingWriteFileSystem) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (failingWriteFileSystem) WriteFile(string, []byte, os.FileMode) error {
	return errors.New("forced write failure")
}

func (failingWriteFileSystem) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func (failingWriteFileSystem) Remove(name string) error {
	return os.Remove(name)
}

func (failingWriteFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (failingWriteFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func TestDetectServiceName_Default(t *testing.T) {
	t.Setenv("PATH", "")

	if got := detectServiceName(); got != "pulse-backend" {
		t.Fatalf("expected pulse-backend, got %q", got)
	}
}

func TestResponseCaptureWrites(t *testing.T) {
	rec := httptest.NewRecorder()
	rc := &responseCapture{ResponseWriter: rec}

	rc.WriteHeader(http.StatusCreated)
	if !rc.wrote {
		t.Fatalf("expected wrote=true after WriteHeader")
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	_, _ = rc.Write([]byte("ok"))
	if !rc.wrote {
		t.Fatalf("expected wrote=true after Write")
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("expected body 'ok', got %q", rec.Body.String())
	}
}

func TestHandleRegenerateAPIToken_DoesNotRequireEnvFile(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
	}

	router := &Router{config: cfg}
	handler := http.HandlerFunc(router.HandleRegenerateAPIToken)

	authLimiter.Reset("127.0.0.1")

	req := newLoopbackRequest(http.MethodPost, "/api/security/regenerate-token", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["success"] != true {
		t.Fatalf("expected success=true, got %#v", payload["success"])
	}
	if token, ok := payload["token"].(string); !ok || strings.TrimSpace(token) == "" {
		t.Fatalf("expected non-empty token in response, got %#v", payload["token"])
	}
}

func TestHandleRegenerateAPITokenInheritsOwnerFromCallerToken(t *testing.T) {
	dataDir := t.TempDir()
	rawCallerToken := "regen-owner-token-123.12345678"
	callerRecord, err := config.NewAPITokenRecord(rawCallerToken, "admin-token", []string{config.ScopeSettingsWrite})
	if err != nil {
		t.Fatalf("new caller token record: %v", err)
	}
	callerRecord.Metadata = map[string]string{apiTokenMetadataOwnerUserID: "alice"}

	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
		APITokens:  []config.APITokenRecord{*callerRecord},
	}
	cfg.SortAPITokens()

	router := &Router{
		config:      cfg,
		persistence: config.NewConfigPersistence(cfg.DataPath),
	}
	handler := http.HandlerFunc(router.HandleRegenerateAPIToken)

	authLimiter.Reset("127.0.0.1")

	req := newLoopbackRequest(http.MethodPost, "/api/security/regenerate-token", nil)
	req.Header.Set("X-API-Token", rawCallerToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("expected one regenerated API token, got %d", len(cfg.APITokens))
	}
	if got := cfg.APITokens[0].Metadata[apiTokenMetadataOwnerUserID]; got != "alice" {
		t.Fatalf("regenerated token owner_user_id=%q, want alice", got)
	}
}

func TestHandleRegenerateAPITokenRollsBackWhenPersistenceFails(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "state")
	rawCallerToken := "regen-persistence-token-123.12345678"
	callerRecord, err := config.NewAPITokenRecord(rawCallerToken, "existing-admin-token", []string{config.ScopeSettingsWrite})
	if err != nil {
		t.Fatalf("new caller token record: %v", err)
	}

	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
		APITokens:  []config.APITokenRecord{*callerRecord},
	}
	cfg.SortAPITokens()
	previousPrimaryToken := cfg.APIToken

	// Block persistence after construction so the handler must restore the
	// complete live credential inventory rather than expose a transient token.
	persistence := config.NewConfigPersistence(dataDir)
	if err := os.RemoveAll(dataDir); err != nil {
		t.Fatalf("remove persistence directory: %v", err)
	}
	if err := os.WriteFile(dataDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create persistence blocker: %v", err)
	}
	router := &Router{config: cfg, persistence: persistence}

	authLimiter.Reset("127.0.0.1")
	req := newLoopbackRequest(http.MethodPost, "/api/security/regenerate-token", nil)
	req.Header.Set("X-API-Token", rawCallerToken)
	rec := httptest.NewRecorder()

	router.HandleRegenerateAPIToken(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != callerRecord.ID {
		t.Fatalf("live token inventory changed after failed persistence: %+v", cfg.APITokens)
	}
	if cfg.APIToken != previousPrimaryToken {
		t.Fatalf("legacy primary token = %q, want rollback to %q", cfg.APIToken, previousPrimaryToken)
	}
	if strings.Contains(rec.Body.String(), `"token"`) {
		t.Fatalf("failed regeneration exposed an unpersisted token: %q", rec.Body.String())
	}
	if !internalauth.CompareAPIToken(rawCallerToken, cfg.APITokens[0].Hash) {
		t.Fatal("previous credential no longer authenticates after failed persistence")
	}
}

func TestHandleValidateAPIToken_InvalidJSON(t *testing.T) {
	router := &Router{config: &config.Config{}}
	handler := http.HandlerFunc(router.HandleValidateAPIToken)

	authLimiter.Reset("127.0.0.1")

	req := newLoopbackRequest(http.MethodPost, "/api/security/validate-token", strings.NewReader("not-json"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleValidateAPIToken_MissingToken(t *testing.T) {
	router := &Router{config: &config.Config{}}
	handler := http.HandlerFunc(router.HandleValidateAPIToken)

	authLimiter.Reset("127.0.0.1")

	req := newLoopbackRequest(http.MethodPost, "/api/security/validate-token", strings.NewReader(`{"token":""}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Token is required" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleValidateAPIToken_NoTokensConfigured(t *testing.T) {
	router := &Router{config: &config.Config{}}
	handler := http.HandlerFunc(router.HandleValidateAPIToken)

	authLimiter.Reset("127.0.0.1")

	req := newLoopbackRequest(http.MethodPost, "/api/security/validate-token", strings.NewReader(`{"token":"abc"}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "API token authentication is not configured" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleValidateAPIToken_InvalidToken(t *testing.T) {
	hashed, err := internalauth.HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	record, err := config.NewAPITokenRecord("good-token", "token", nil)
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}

	cfg := &config.Config{
		AuthUser:  "admin",
		AuthPass:  hashed,
		APITokens: []config.APITokenRecord{*record},
	}
	cfg.SortAPITokens()

	router := &Router{config: cfg}
	handler := http.HandlerFunc(router.HandleValidateAPIToken)

	authLimiter.Reset("198.51.100.13")

	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-token", strings.NewReader(`{"token":"bad-token"}`))
	req.RemoteAddr = "198.51.100.13:54321"
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Token is invalid" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestHandleValidateAPIToken_ValidToken(t *testing.T) {
	hashed, err := internalauth.HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	record, err := config.NewAPITokenRecord("good-token", "token", nil)
	if err != nil {
		t.Fatalf("new token record: %v", err)
	}

	cfg := &config.Config{
		AuthUser:  "admin",
		AuthPass:  hashed,
		APITokens: []config.APITokenRecord{*record},
	}
	cfg.SortAPITokens()

	router := &Router{config: cfg}
	handler := http.HandlerFunc(router.HandleValidateAPIToken)

	authLimiter.Reset("198.51.100.14")

	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-token", strings.NewReader(`{"token":"good-token"}`))
	req.RemoteAddr = "198.51.100.14:54321"
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["message"] != "Token is valid" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
}

func TestQuickSecuritySetupSkipsWhenAuthConfigured(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

	dataDir := t.TempDir()
	hashed, err := internalauth.HashPassword("ExistingPassword!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	rawToken := strings.Repeat("ab", 32)
	record, err := config.NewAPITokenRecord(rawToken, "admin-token", []string{config.ScopeSettingsWrite})
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
	handler := handleQuickSecuritySetupFixed(router)

	authLimiter.Reset("198.51.100.15")

	payload := `{"username":"newadmin","password":"NewPassword!1","apiToken":"` + strings.Repeat("cd", 32) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(payload))
	req.RemoteAddr = "198.51.100.15:54321"
	req.Header.Set("X-API-Token", rawToken)

	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["skipped"] != true {
		t.Fatalf("expected skipped=true, got %#v", response["skipped"])
	}

	if cfg.AuthUser != "admin" {
		t.Fatalf("expected AuthUser to remain admin, got %q", cfg.AuthUser)
	}
	if !internalauth.CheckPasswordHash("ExistingPassword!1", cfg.AuthPass) {
		t.Fatalf("expected password hash to remain unchanged")
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("expected 1 API token, got %d", len(cfg.APITokens))
	}
	if cfg.APITokens[0].Hash != record.Hash {
		t.Fatalf("expected API token hash to remain unchanged")
	}
}

func TestQuickSecuritySetupBootstrapTokenUnavailable(t *testing.T) {
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
	handler := handleQuickSecuritySetupFixed(router)

	authLimiter.Reset("127.0.0.1")

	payload := `{"username":"bootstrap","password":"StrongPass!1","apiToken":"` + strings.Repeat("aa", 32) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(payload))
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	}
}

func TestQuickSecuritySetupAcceptsSetupTokenInBody(t *testing.T) {
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

	tokenPath := filepath.Join(cfg.DataPath, bootstrapTokenFilename)
	bootstrapToken, _, _, err := bootstrap.Load(cfg.DataPath)
	if err != nil {
		t.Fatalf("load bootstrap token: %v", err)
	}

	handler := handleQuickSecuritySetupFixed(router)

	authLimiter.Reset("127.0.0.1")

	payload := `{"username":"bootstrap","password":"StrongPass!1","apiToken":"` + strings.Repeat("aa", 32) + `","setupToken":"` + bootstrapToken + `"}` //nolint:lll
	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(payload))
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}

	if _, err := os.Stat(tokenPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected bootstrap token file to be removed after successful setup, got err=%v", err)
	}
	if router.bootstrapTokenHash != "" {
		t.Fatalf("expected bootstrap token hash to be cleared after successful setup")
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("expected one API token, got %d", len(cfg.APITokens))
	}
	if got := cfg.APITokens[0].Metadata[apiTokenMetadataOwnerUserID]; got != "bootstrap" {
		t.Fatalf("setup token owner_user_id=%q, want bootstrap", got)
	}
}

func TestQuickSecuritySetupRotatesWithBasicAuth(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

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

	router := &Router{
		config:      cfg,
		persistence: config.NewConfigPersistence(cfg.DataPath),
	}
	handler := handleQuickSecuritySetupFixed(router)

	authLimiter.Reset("198.51.100.18")

	payload := `{"username":"newadmin","password":"NewPassword!1","apiToken":"` + strings.Repeat("bb", 32) + `","force":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(payload))
	req.RemoteAddr = "198.51.100.18:54321"
	req.SetBasicAuth("admin", "OldPassword!1")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	if cfg.AuthUser != "newadmin" {
		t.Fatalf("expected AuthUser to be rotated, got %q", cfg.AuthUser)
	}
	if !internalauth.CheckPasswordHash("NewPassword!1", cfg.AuthPass) {
		t.Fatalf("stored password hash does not match new password")
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("expected one API token, got %d", len(cfg.APITokens))
	}
	if got := cfg.APITokens[0].Metadata[apiTokenMetadataOwnerUserID]; got != "newadmin" {
		t.Fatalf("rotated token owner_user_id=%q, want newadmin", got)
	}

	cookies := rec.Result().Cookies()
	sessionCookie := findCookie(cookies, sessionCookieName(false))
	if sessionCookie == nil || strings.TrimSpace(sessionCookie.Value) == "" {
		t.Fatalf("expected credential rotation to refresh the session cookie")
	}
	if !ValidateSession(sessionCookie.Value) {
		t.Fatalf("expected refreshed session to validate")
	}
	if got := GetSessionUsername(sessionCookie.Value); got != "newadmin" {
		t.Fatalf("session username=%q, want %q", got, "newadmin")
	}
}

func TestQuickSecuritySetupRestoresTokensWhenPersistenceFails(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

	dataDir := t.TempDir()
	oldPasswordHash, err := internalauth.HashPassword("ExistingPassword!1")
	if err != nil {
		t.Fatalf("hash existing password: %v", err)
	}
	rawOldToken := strings.Repeat("ab", 32)
	oldToken, err := config.NewAPITokenRecord(rawOldToken, "existing token", []string{config.ScopeSettingsWrite})
	if err != nil {
		t.Fatalf("new existing token: %v", err)
	}
	cfg := &config.Config{
		AuthUser:   "admin",
		AuthPass:   oldPasswordHash,
		DataPath:   dataDir,
		ConfigPath: dataDir,
		APITokens:  []config.APITokenRecord{*oldToken},
	}
	cfg.SortAPITokens()
	previousPrimaryToken := cfg.APIToken

	persistence := config.NewConfigPersistence(dataDir)
	if err := persistence.SaveAPITokens(cfg.APITokens); err != nil {
		t.Fatalf("save existing tokens: %v", err)
	}
	persistence.SetFileSystem(failingWriteFileSystem{})
	router := &Router{config: cfg, persistence: persistence}

	authLimiter.Reset("198.51.100.17")
	newRawToken := strings.Repeat("cd", 32)
	payload := `{"username":"newadmin","password":"NewPassword!1","apiToken":"` + newRawToken + `","force":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(payload))
	req.RemoteAddr = "198.51.100.17:54321"
	req.Header.Set("X-API-Token", rawOldToken)
	rec := httptest.NewRecorder()

	handleQuickSecuritySetupFixed(router)(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	if len(cfg.APITokens) != 1 || cfg.APITokens[0].ID != oldToken.ID {
		t.Fatalf("live token inventory changed after failed persistence: %+v", cfg.APITokens)
	}
	if cfg.APIToken != previousPrimaryToken {
		t.Fatalf("legacy primary token = %q, want rollback to %q", cfg.APIToken, previousPrimaryToken)
	}
	if !internalauth.CompareAPIToken(rawOldToken, cfg.APITokens[0].Hash) {
		t.Fatal("previous credential no longer authenticates after failed persistence")
	}
	if cfg.AuthUser != "newadmin" || !internalauth.CheckPasswordHash("NewPassword!1", cfg.AuthPass) {
		t.Fatal("persisted password authentication was not applied to live state")
	}
	if strings.Contains(rec.Body.String(), newRawToken) {
		t.Fatalf("failure response exposed API token: %q", rec.Body.String())
	}

	persisted, err := persistence.LoadAPITokens()
	if err != nil {
		t.Fatalf("load persisted tokens: %v", err)
	}
	if len(persisted) != 1 || persisted[0].ID != oldToken.ID {
		t.Fatalf("persisted token inventory changed after failed write: %+v", persisted)
	}
}

func TestQuickSecuritySetupRateLimitEnforced(t *testing.T) {
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

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

	router := &Router{
		config:      cfg,
		persistence: config.NewConfigPersistence(cfg.DataPath),
	}
	handler := handleQuickSecuritySetupFixed(router)

	ip := "203.0.113.210"
	authLimiter.Reset(ip)
	defer authLimiter.Reset(ip)

	payload := `{"username":"newadmin","password":"NewPassword!1","apiToken":"` + strings.Repeat("bb", 32) + `"}`

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(payload))
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()

		handler(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected %d, got %d (%s)", i+1, http.StatusUnauthorized, rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(payload))
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected %d, got %d (%s)", http.StatusTooManyRequests, rec.Code, rec.Body.String())
	}
}
