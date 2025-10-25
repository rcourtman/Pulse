package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	internalauth "github.com/rcourtman/pulse-go-rewrite/internal/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func newTestConfigHandlers(t *testing.T, cfg *config.Config) *ConfigHandlers {
	t.Helper()

	h := &ConfigHandlers{
		config:               cfg,
		persistence:          config.NewConfigPersistence(cfg.DataPath),
		setupCodes:           make(map[string]*SetupCode),
		recentSetupTokens:    make(map[string]time.Time),
		lastClusterDetection: make(map[string]time.Time),
		recentAutoRegistered: make(map[string]time.Time),
	}

	return h
}

func TestHandleAutoRegisterRejectsWithoutAuth(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pam!token",
		TokenValue: "secret-token",
		ServerName: "pve.local",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleAutoRegisterAcceptsWithSetupToken(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handler := newTestConfigHandlers(t, cfg)

	const tokenValue = "TEMP-TOKEN"
	tokenHash := internalauth.HashAPIToken(tokenValue)
	handler.codeMutex.Lock()
	handler.setupCodes[tokenHash] = &SetupCode{
		ExpiresAt: time.Now().Add(5 * time.Minute),
		NodeType:  "pve",
	}
	handler.codeMutex.Unlock()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       "https://pve.local:8006",
		TokenID:    "pulse-monitor@pam!token",
		TokenValue: "secret-token",
		ServerName: "pve.local",
		AuthToken:  tokenValue,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleAutoRegister(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance stored, got %d", len(cfg.PVEInstances))
	}
}
