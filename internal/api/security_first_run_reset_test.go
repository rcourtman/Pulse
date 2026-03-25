package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestResetFirstRunSecurityRequiresDevMode(t *testing.T) {
	t.Setenv("PULSE_DEV", "")
	t.Setenv("NODE_ENV", "production")

	record := newTokenRecord(t, "reset-first-run-token-123.12345678", []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/security/dev/reset-first-run", nil)
	req.Header.Set("X-API-Token", "reset-first-run-token-123.12345678")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 outside dev mode, got %d", rec.Code)
	}
}

func TestResetFirstRunSecurityClearsAuthAndReturnsBootstrapToken(t *testing.T) {
	t.Setenv("PULSE_DEV", "true")
	t.Setenv("NODE_ENV", "")

	record := newTokenRecord(t, "reset-first-run-token-234.12345678", []string{config.ScopeSettingsWrite}, nil)
	cfg := newTestConfigWithTokens(t, record)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "hashed-password"

	envPath, err := writeAuthEnvFile(cfg.ConfigPath, cfg.DataPath, []byte("PULSE_AUTH_USER='admin'\n"))
	if err != nil {
		t.Fatalf("writeAuthEnvFile: %v", err)
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	req := httptest.NewRequest(http.MethodPost, "/api/security/dev/reset-first-run", nil)
	req.Header.Set("X-API-Token", "reset-first-run-token-234.12345678")
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var payload firstRunResetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if strings.TrimSpace(payload.BootstrapToken) == "" {
		t.Fatal("expected bootstrapToken in response")
	}
	if strings.TrimSpace(payload.BootstrapTokenPath) == "" {
		t.Fatal("expected bootstrapTokenPath in response")
	}
	if router.bootstrapTokenHash == "" || !router.bootstrapTokenValid(payload.BootstrapToken) {
		t.Fatal("expected router to accept returned bootstrap token")
	}
	if cfg.AuthUser != "" || cfg.AuthPass != "" {
		t.Fatalf("expected auth credentials cleared, got user=%q pass=%q", cfg.AuthUser, cfg.AuthPass)
	}
	if cfg.HasAPITokens() || cfg.APIToken != "" {
		t.Fatalf("expected API tokens cleared, got %d tokens", len(cfg.APITokens))
	}
	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		t.Fatalf("expected auth env file removed, stat err=%v", err)
	}

	persistence := config.NewConfigPersistence(cfg.DataPath)
	tokens, err := persistence.LoadAPITokens()
	if err != nil {
		t.Fatalf("LoadAPITokens: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected persisted API tokens cleared, got %d", len(tokens))
	}

	tokenPath := filepath.Join(cfg.DataPath, bootstrapTokenFilename)
	if payload.BootstrapTokenPath != tokenPath {
		t.Fatalf("bootstrap token path = %q, want %q", payload.BootstrapTokenPath, tokenPath)
	}
}
