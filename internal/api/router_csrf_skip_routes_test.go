package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

func TestCSRFSkippedForValidateBootstrapToken(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

	dataDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   dataDir,
		ConfigPath: dataDir,
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	if router.configHandlers != nil {
		router.configHandlers.SetConfig(cfg)
	}

	tokenPath := filepath.Join(cfg.DataPath, bootstrapTokenFilename)
	content, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("read bootstrap token: %v", err)
	}
	token := strings.TrimSpace(string(content))
	if token == "" {
		t.Fatalf("bootstrap token should not be empty")
	}

	// Create a session to ensure CSRF would be required if not skipped.
	sessionToken := generateSessionToken()
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "admin")

	body := map[string]string{"token": token}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/security/validate-bootstrap-token", bytes.NewReader(payload))
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	rec := httptest.NewRecorder()

	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusNoContent, rec.Code, rec.Body.String())
	}
}

func TestCSRFSkippedForSetupScriptURL(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	t.Setenv("PULSE_TRUSTED_PROXY_CIDRS", "")
	resetTrustedProxyConfig()

	dataDir := t.TempDir()
	hashed, err := internalauth.HashPassword("Password!1")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	cfg := &config.Config{
		AuthUser:   "admin",
		AuthPass:   hashed,
		DataPath:   dataDir,
		ConfigPath: dataDir,
	}

	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	if router.configHandlers != nil {
		router.configHandlers.SetConfig(cfg)
	}

	sessionToken := generateSessionToken()
	GetSessionStore().CreateSession(sessionToken, time.Hour, "agent", "127.0.0.1", "admin")

	body := `{"type":"pve"}`
	req := httptest.NewRequest(http.MethodPost, "/api/setup-script-url", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "pulse_session", Value: sessionToken})
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d (%s)", http.StatusOK, rec.Code, rec.Body.String())
	}
}
