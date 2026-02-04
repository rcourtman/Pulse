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

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

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

func TestHandleRegenerateAPIToken_MissingEnvFile(t *testing.T) {
	dataDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
	}

	router := &Router{config: cfg}
	handler := http.HandlerFunc(router.HandleRegenerateAPIToken)

	authLimiter.Reset("198.51.100.9")

	req := httptest.NewRequest(http.MethodPost, "/api/security/regenerate-token", nil)
	req.RemoteAddr = "198.51.100.9:54321"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestHandleValidateAPIToken_InvalidJSON(t *testing.T) {
	router := &Router{config: &config.Config{}}
	handler := http.HandlerFunc(router.HandleValidateAPIToken)

	authLimiter.Reset("198.51.100.10")

	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-token", strings.NewReader("not-json"))
	req.RemoteAddr = "198.51.100.10:54321"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleValidateAPIToken_MissingToken(t *testing.T) {
	router := &Router{config: &config.Config{}}
	handler := http.HandlerFunc(router.HandleValidateAPIToken)

	authLimiter.Reset("198.51.100.11")

	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-token", strings.NewReader(`{"token":""}`))
	req.RemoteAddr = "198.51.100.11:54321"
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

	authLimiter.Reset("198.51.100.12")

	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-token", strings.NewReader(`{"token":"abc"}`))
	req.RemoteAddr = "198.51.100.12:54321"
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

	authLimiter.Reset("198.51.100.16")

	payload := `{"username":"bootstrap","password":"StrongPass!1","apiToken":"` + strings.Repeat("aa", 32) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(payload))
	req.RemoteAddr = "198.51.100.16:54321"
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
	content, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("read bootstrap token: %v", err)
	}
	bootstrapToken := strings.TrimSpace(string(content))
	if bootstrapToken == "" {
		t.Fatalf("bootstrap token is empty")
	}

	handler := handleQuickSecuritySetupFixed(router)

	authLimiter.Reset("198.51.100.17")

	payload := `{"username":"bootstrap","password":"StrongPass!1","apiToken":"` + strings.Repeat("aa", 32) + `","setupToken":"` + bootstrapToken + `"}` //nolint:lll
	req := httptest.NewRequest(http.MethodPost, "/api/security/quick-setup", strings.NewReader(payload))
	req.RemoteAddr = "198.51.100.17:54321"
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
