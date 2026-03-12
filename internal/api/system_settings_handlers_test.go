package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/discovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// MockMonitor implementation
type mockMonitor struct {
}

func (m *mockMonitor) GetDiscoveryService() *discovery.Service { return nil }
func (m *mockMonitor) StartDiscoveryService(ctx context.Context, wsHub *websocket.Hub, subnet string) {
}
func (m *mockMonitor) StopDiscoveryService()                                      {}
func (m *mockMonitor) EnableTemperatureMonitoring()                               {}
func (m *mockMonitor) DisableTemperatureMonitoring()                              {}
func (m *mockMonitor) GetNotificationManager() *notifications.NotificationManager { return nil }

type mockTenantMonitorProvider struct {
	orgID   string
	monitor *monitoring.Monitor
	err     error
}

func (m *mockTenantMonitorProvider) GetMonitor(orgID string) (*monitoring.Monitor, error) {
	m.orgID = orgID
	if m.err != nil {
		return nil, m.err
	}
	return m.monitor, nil
}

func newTestSystemSettingsHandler(cfg *config.Config, persistence *config.ConfigPersistence, monitor SystemSettingsMonitor, reloadSystemSettingsFunc func(), reloadMonitorFunc func() error) *SystemSettingsHandler {
	handler := NewSystemSettingsHandler(cfg, persistence, nil, nil, monitor, reloadSystemSettingsFunc, reloadMonitorFunc)
	handler.mtMonitor = nil
	return handler
}

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
	handler := newTestSystemSettingsHandler(cfg, persistence, monitor, func() {}, func() error { return nil })

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

func TestHandleGetSystemSettings_DisablesAutoUpdatesOnRC(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:          tempDir,
		ConfigPath:        tempDir,
		UpdateChannel:     "rc",
		AutoUpdateEnabled: true,
	}
	persistence := config.NewConfigPersistence(tempDir)
	monitor := &mockMonitor{}
	handler := newTestSystemSettingsHandler(cfg, persistence, monitor, func() {}, func() error { return nil })

	initialSettings := config.DefaultSystemSettings()
	initialSettings.UpdateChannel = "rc"
	initialSettings.AutoUpdateEnabled = true
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
		UpdateChannel     string `json:"updateChannel"`
		AutoUpdateEnabled bool   `json:"autoUpdateEnabled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.UpdateChannel != "rc" {
		t.Fatalf("expected updateChannel rc, got %q", response.UpdateChannel)
	}
	if response.AutoUpdateEnabled {
		t.Fatalf("expected autoUpdateEnabled to be normalized off on rc")
	}
}

func TestHandleGetSystemSettings_UsesRuntimeRCWhenPersistedChannelMissing(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:          tempDir,
		ConfigPath:        tempDir,
		UpdateChannel:     "rc",
		AutoUpdateEnabled: true,
	}
	persistence := config.NewConfigPersistence(tempDir)
	monitor := &mockMonitor{}
	handler := newTestSystemSettingsHandler(cfg, persistence, monitor, func() {}, func() error { return nil })

	initialSettings := config.DefaultSystemSettings()
	initialSettings.AutoUpdateEnabled = true
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
		UpdateChannel     string `json:"updateChannel"`
		AutoUpdateEnabled bool   `json:"autoUpdateEnabled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.UpdateChannel != "rc" {
		t.Fatalf("expected runtime updateChannel rc, got %q", response.UpdateChannel)
	}
	if response.AutoUpdateEnabled {
		t.Fatalf("expected autoUpdateEnabled to be normalized off on runtime rc")
	}
}

func TestHandleGetSystemSettings_LoadError(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{DataPath: tempDir}
	persistence := config.NewConfigPersistence(tempDir)
	handler := newTestSystemSettingsHandler(cfg, persistence, &mockMonitor{}, func() {}, func() error { return nil })

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
	handler := newTestSystemSettingsHandler(cfg, persistence, monitor, func() {}, func() error { return nil })

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

func TestHandleUpdateSystemSettings_DisablesAutoUpdatesWhenRCSelected(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	persistence := config.NewConfigPersistence(tempDir)
	monitor := &mockMonitor{}
	handler := newTestSystemSettingsHandler(cfg, persistence, monitor, func() {}, func() error { return nil })

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
		"updateChannel":     "rc",
		"autoUpdateEnabled": true,
	}
	body, _ := json.Marshal(updates)

	req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
	req.Header.Set("X-API-Token", tokenVal)

	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
	}

	loaded, err := persistence.LoadSystemSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}
	if loaded.UpdateChannel != "rc" {
		t.Fatalf("expected saved updateChannel rc, got %q", loaded.UpdateChannel)
	}
	if loaded.AutoUpdateEnabled {
		t.Fatalf("expected saved autoUpdateEnabled to be false on rc")
	}
	if cfg.AutoUpdateEnabled {
		t.Fatalf("expected runtime autoUpdateEnabled to be false on rc")
	}
}

func TestHandleUpdateSystemSettings_UsesRuntimeRCWhenChannelOmitted(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:      tempDir,
		ConfigPath:    tempDir,
		UpdateChannel: "rc",
	}
	persistence := config.NewConfigPersistence(tempDir)
	monitor := &mockMonitor{}
	handler := newTestSystemSettingsHandler(cfg, persistence, monitor, func() {}, func() error { return nil })

	tokenVal := "testtoken123"
	tokenHash := internalauth.HashAPIToken(tokenVal)
	cfg.APITokens = []config.APITokenRecord{
		{
			ID:   "token1",
			Hash: tokenHash,
			Name: "Test Token",
		},
	}

	initialSettings := config.DefaultSystemSettings()
	if err := persistence.SaveSystemSettings(*initialSettings); err != nil {
		t.Fatalf("Failed to save initial settings: %v", err)
	}

	updates := map[string]interface{}{
		"autoUpdateEnabled": true,
	}
	body, _ := json.Marshal(updates)

	req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
	req.Header.Set("X-API-Token", tokenVal)
	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
	}

	loaded, err := persistence.LoadSystemSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}
	if loaded.UpdateChannel != "rc" {
		t.Fatalf("expected saved updateChannel rc from runtime fallback, got %q", loaded.UpdateChannel)
	}
	if loaded.AutoUpdateEnabled {
		t.Fatalf("expected saved autoUpdateEnabled to remain false on runtime rc")
	}
	if cfg.AutoUpdateEnabled {
		t.Fatalf("expected runtime autoUpdateEnabled to remain false on runtime rc")
	}
}

func TestHandleUpdateSystemSettings_DiscoveryConfigAppliedFromCamelCasePayload(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		Discovery:  config.DefaultDiscoveryConfig(),
	}
	persistence := config.NewConfigPersistence(tempDir)
	monitor := &mockMonitor{}
	handler := newTestSystemSettingsHandler(cfg, persistence, monitor, func() {}, func() error { return nil })

	tokenVal := "testtoken123"
	tokenHash := internalauth.HashAPIToken(tokenVal)
	cfg.APITokens = []config.APITokenRecord{
		{ID: "token1", Hash: tokenHash, Name: "Test Token"},
	}

	updates := map[string]interface{}{
		"discoveryConfig": map[string]interface{}{
			"environmentOverride": "docker-bridge",
			"subnetAllowlist":     []string{"10.0.0.0/8"},
			"subnetBlocklist":     []string{"169.254.0.0/16"},
			"maxHostsPerScan":     77,
			"maxConcurrent":       11,
			"enableReverseDns":    false,
			"scanGateways":        false,
			"dialTimeoutMs":       1500,
			"httpTimeoutMs":       2300,
		},
	}
	body, _ := json.Marshal(updates)

	req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
	req.Header.Set("X-API-Token", tokenVal)
	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
	}

	loaded, err := persistence.LoadSystemSettings()
	if err != nil {
		t.Fatalf("Failed to load settings: %v", err)
	}
	got := loaded.DiscoveryConfig
	if got.EnvironmentOverride != "docker-bridge" {
		t.Fatalf("EnvironmentOverride = %q, want %q", got.EnvironmentOverride, "docker-bridge")
	}
	if got.MaxHostsPerScan != 77 {
		t.Fatalf("MaxHostsPerScan = %d, want 77", got.MaxHostsPerScan)
	}
	if got.MaxConcurrent != 11 {
		t.Fatalf("MaxConcurrent = %d, want 11", got.MaxConcurrent)
	}
	if got.EnableReverseDNS {
		t.Fatalf("EnableReverseDNS = true, want false")
	}
	if got.ScanGateways {
		t.Fatalf("ScanGateways = true, want false")
	}
	if got.DialTimeout != 1500 {
		t.Fatalf("DialTimeout = %d, want 1500", got.DialTimeout)
	}
	if got.HTTPTimeout != 2300 {
		t.Fatalf("HTTPTimeout = %d, want 2300", got.HTTPTimeout)
	}
	if len(got.SubnetAllowlist) != 1 || got.SubnetAllowlist[0] != "10.0.0.0/8" {
		t.Fatalf("SubnetAllowlist = %v, want [10.0.0.0/8]", got.SubnetAllowlist)
	}
	if len(got.SubnetBlocklist) != 1 || got.SubnetBlocklist[0] != "169.254.0.0/16" {
		t.Fatalf("SubnetBlocklist = %v, want [169.254.0.0/16]", got.SubnetBlocklist)
	}

	if cfg.Discovery.EnvironmentOverride != "docker-bridge" {
		t.Fatalf("runtime config EnvironmentOverride = %q, want %q", cfg.Discovery.EnvironmentOverride, "docker-bridge")
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
	handler := newTestSystemSettingsHandler(cfg, persistence, &mockMonitor{}, func() {}, func() error { return nil })

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
	handler := newTestSystemSettingsHandler(cfg, persistence, &mockMonitor{}, func() {}, func() error { return nil })

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

func TestSystemSettingsHandler_GetMonitor_UsesTenantMonitorInterface(t *testing.T) {
	tenantMonitor := &monitoring.Monitor{}
	defaultMonitor := &mockMonitor{}
	provider := &mockTenantMonitorProvider{monitor: tenantMonitor}

	handler := &SystemSettingsHandler{
		mtMonitor:      provider,
		defaultMonitor: defaultMonitor,
	}

	got := handler.getMonitor(context.Background())
	if got != tenantMonitor {
		t.Fatalf("expected tenant monitor, got %#v", got)
	}
	if provider.orgID != "default" {
		t.Fatalf("expected org lookup for default context org, got %q", provider.orgID)
	}
}

func TestSystemSettingsHandler_GetMonitor_FallsBackToLegacyOnTenantError(t *testing.T) {
	defaultMonitor := &mockMonitor{}
	provider := &mockTenantMonitorProvider{err: errors.New("boom")}

	handler := &SystemSettingsHandler{
		mtMonitor:      provider,
		defaultMonitor: defaultMonitor,
	}

	got := handler.getMonitor(context.WithValue(context.Background(), OrgIDContextKey, "acme"))
	if got != defaultMonitor {
		t.Fatalf("expected legacy monitor fallback, got %#v", got)
	}
	if provider.orgID != "acme" {
		t.Fatalf("expected tenant org lookup for acme, got %q", provider.orgID)
	}
}
