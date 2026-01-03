package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	internalauth "github.com/rcourtman/pulse-go-rewrite/internal/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/discovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
)

// MockMonitor implementation
type mockMonitor struct {
	hasSocketProxy bool
}

func (m *mockMonitor) GetDiscoveryService() *discovery.Service { return nil }
func (m *mockMonitor) StartDiscoveryService(ctx context.Context, wsHub *websocket.Hub, subnet string) {
}
func (m *mockMonitor) StopDiscoveryService()                                      {}
func (m *mockMonitor) EnableTemperatureMonitoring()                               {}
func (m *mockMonitor) DisableTemperatureMonitoring()                              {}
func (m *mockMonitor) GetNotificationManager() *notifications.NotificationManager { return nil }
func (m *mockMonitor) HasSocketTemperatureProxy() bool                            { return m.hasSocketProxy }

func TestHandleGetSystemSettings(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:                     tempDir,
		ConfigPath:                   tempDir,
		PVEPollingInterval:           30 * time.Second,
		BackupPollingInterval:        1 * time.Hour,
		EnableBackupPolling:          true,
		TemperatureMonitoringEnabled: true,
	}
	persistence := config.NewConfigPersistence(tempDir)
	monitor := &mockMonitor{}
	handler := NewSystemSettingsHandler(cfg, persistence, nil, monitor, func() {}, func() error { return nil })

	// Save some settings first
	initialSettings := config.DefaultSystemSettings()
	initialSettings.Theme = "dark"
	if err := persistence.SaveSystemSettings(*initialSettings); err != nil {
		t.Fatalf("Failed to save initial settings: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/system-settings", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetSystemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response struct {
		Theme string `json:"theme"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Theme != "dark" {
		t.Errorf("Expected theme 'dark', got '%s'", response.Theme)
	}
}

func TestHandleGetSystemSettings_LoadError(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{DataPath: tempDir}
	persistence := config.NewConfigPersistence(tempDir)
	handler := NewSystemSettingsHandler(cfg, persistence, nil, &mockMonitor{}, func() {}, func() error { return nil })

	// Write invalid JSON
	systemFile := filepath.Join(tempDir, "system.json")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(systemFile, []byte("{invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/system-settings", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetSystemSettings(rec, req)

	// Should fallback to defaults
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestHandleUpdateSystemSettings_Basic(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	persistence := config.NewConfigPersistence(tempDir)
	monitor := &mockMonitor{}
	handler := NewSystemSettingsHandler(cfg, persistence, nil, monitor, func() {}, func() error { return nil })

	// Setup Authentication (API Token)
	tokenVal := "testtoken123"
	tokenHash := internalauth.HashAPIToken(tokenVal)
	cfg.APITokens = []config.APITokenRecord{
		{
			ID:   "token1",
			Hash: tokenHash,
			Name: "Test Token",
		},
	}

	updates := map[string]interface{}{
		"theme":              "light",
		"pvePollingInterval": 60,
	}
	body, _ := json.Marshal(updates)

	req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
	req.Header.Set("X-API-Token", tokenVal)

	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
	}

	// Verify persistence
	loaded, err := persistence.LoadSystemSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}
	if loaded.Theme != "light" {
		t.Errorf("Expected theme 'light', got '%s'", loaded.Theme)
	}
	if loaded.PVEPollingInterval != 60 {
		t.Errorf("Expected PVEPollingInterval 60, got %d", loaded.PVEPollingInterval)
	}

	// Verify config update
	if cfg.PVEPollingInterval != 60*time.Second {
		t.Errorf("Config was not updated. Expected 60s, got %v", cfg.PVEPollingInterval)
	}
}

func TestHandleUpdateSystemSettings_Unauthorized(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath: tempDir,
		AuthUser: "admin",
		AuthPass: "password", // Requires auth
	}
	persistence := config.NewConfigPersistence(tempDir)
	handler := NewSystemSettingsHandler(cfg, persistence, nil, &mockMonitor{}, func() {}, func() error { return nil })

	req := httptest.NewRequest(http.MethodPost, "/api/system-settings", nil)
	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestHandleUpdateSystemSettings_Validation(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	persistence := config.NewConfigPersistence(tempDir)
	handler := NewSystemSettingsHandler(cfg, persistence, nil, &mockMonitor{}, func() {}, func() error { return nil })

	// Setup Auth
	tokenVal := "testtoken123"
	tokenHash := internalauth.HashAPIToken(tokenVal)
	cfg.APITokens = []config.APITokenRecord{
		{ID: "token1", Hash: tokenHash},
	}

	updates := map[string]interface{}{
		"pvePollingInterval": -1, // Invalid
	}
	body, _ := json.Marshal(updates)

	req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
	req.Header.Set("X-API-Token", tokenVal)
	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}
