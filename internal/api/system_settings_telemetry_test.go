package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// setupTelemetryTest creates a handler with API token auth configured.
func setupTelemetryTest(t *testing.T, cfg *config.Config) (*SystemSettingsHandler, *config.ConfigPersistence, string) {
	t.Helper()
	persistence := config.NewConfigPersistence(cfg.DataPath)

	tokenVal := "telemetry-test-token-123.12345678"
	tokenHash := internalauth.HashAPIToken(tokenVal)
	cfg.APITokens = []config.APITokenRecord{
		{ID: "tok1", Hash: tokenHash, Name: "Test Token"},
	}

	handler := newTestSystemSettingsHandler(cfg, persistence, &mockMonitor{}, func() {}, func() error { return nil })
	return handler, persistence, tokenVal
}

func TestTelemetryUpdate_EnvLockRejects(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:         tempDir,
		ConfigPath:       tempDir,
		TelemetryEnabled: true,
		EnvOverrides:     map[string]bool{"PULSE_TELEMETRY": true, "telemetryEnabled": true},
	}
	handler, persistence, token := setupTelemetryTest(t, cfg)

	initial := config.DefaultSystemSettings()
	if err := persistence.SaveSystemSettings(*initial); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]interface{}{"telemetryEnabled": false})
	req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict when env-locked, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify in-memory config was NOT changed.
	if !cfg.TelemetryEnabled {
		t.Error("TelemetryEnabled should still be true after env-lock rejection")
	}
}

func TestTelemetryUpdate_NullIsIgnored(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:         tempDir,
		ConfigPath:       tempDir,
		TelemetryEnabled: true,
		EnvOverrides:     make(map[string]bool),
	}
	handler, persistence, token := setupTelemetryTest(t, cfg)

	initial := config.DefaultSystemSettings()
	enabled := true
	initial.TelemetryEnabled = &enabled
	if err := persistence.SaveSystemSettings(*initial); err != nil {
		t.Fatal(err)
	}

	// Send telemetryEnabled: null via raw JSON.
	body := []byte(`{"telemetryEnabled": null}`)
	req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// In-memory should remain true (null should not flip it).
	if !cfg.TelemetryEnabled {
		t.Error("TelemetryEnabled should still be true after null update")
	}

	// Verify persisted value is still set (not nil).
	saved, err := persistence.LoadSystemSettings()
	if err != nil {
		t.Fatal(err)
	}
	if saved.TelemetryEnabled == nil {
		t.Error("persisted TelemetryEnabled should not be nil after null update")
	} else if !*saved.TelemetryEnabled {
		t.Error("persisted TelemetryEnabled should still be true")
	}
}

func TestTelemetryUpdate_PersistBeforeMutate(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:           tempDir,
		ConfigPath:         tempDir,
		TelemetryEnabled:   true,
		PVEPollingInterval: 10 * time.Second,
		EnvOverrides:       make(map[string]bool),
	}

	saveCalled := false
	persistence := config.NewConfigPersistence(cfg.DataPath)
	tokenVal := "telemetry-test-token-123.12345678"
	tokenHash := internalauth.HashAPIToken(tokenVal)
	cfg.APITokens = []config.APITokenRecord{
		{ID: "tok1", Hash: tokenHash, Name: "Test Token"},
	}
	handler := newTestSystemSettingsHandler(cfg, persistence, &mockMonitor{}, func() {
		saveCalled = true
	}, func() error { return nil })

	initial := config.DefaultSystemSettings()
	if err := persistence.SaveSystemSettings(*initial); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]interface{}{"telemetryEnabled": false})
	req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
	req.Header.Set("X-API-Token", tokenVal)
	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if !saveCalled {
		t.Error("expected reload (post-save) to be called")
	}

	// In-memory should now be false (applied after successful save).
	if cfg.TelemetryEnabled {
		t.Error("TelemetryEnabled should be false after successful update")
	}
}

func TestTelemetryUpdate_GetReturnsEffectiveValue(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:         tempDir,
		ConfigPath:       tempDir,
		TelemetryEnabled: true,
		EnvOverrides:     map[string]bool{"PULSE_TELEMETRY": true},
	}
	handler, persistence, _ := setupTelemetryTest(t, cfg)

	// Persist with telemetry disabled (simulating a stale disk value).
	initial := config.DefaultSystemSettings()
	disabled := false
	initial.TelemetryEnabled = &disabled
	if err := persistence.SaveSystemSettings(*initial); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/system-settings", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetSystemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var response struct {
		TelemetryEnabled *bool `json:"telemetryEnabled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if response.TelemetryEnabled == nil {
		t.Fatal("telemetryEnabled should be present in response")
	}
	if !*response.TelemetryEnabled {
		t.Error("GET should return effective runtime value (true), not stale disk value (false)")
	}
}

func TestTelemetryUpdate_NoMutationOnPersistFailure(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:           tempDir,
		ConfigPath:         tempDir,
		TelemetryEnabled:   true,
		PVEPollingInterval: 10 * time.Second,
		EnvOverrides:       make(map[string]bool),
	}
	handler, persistence, token := setupTelemetryTest(t, cfg)

	initial := config.DefaultSystemSettings()
	if err := persistence.SaveSystemSettings(*initial); err != nil {
		t.Fatal(err)
	}

	// Make the config directory read-only so SaveSystemSettings fails.
	systemFile := filepath.Join(tempDir, "system.json")
	if err := os.Chmod(systemFile, 0400); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(tempDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chmod(tempDir, 0700)
		os.Chmod(systemFile, 0600)
	})

	body, _ := json.Marshal(map[string]interface{}{"telemetryEnabled": false})
	req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when persistence fails, got %d: %s", rec.Code, rec.Body.String())
	}

	// In-memory config must NOT have been mutated.
	if !cfg.TelemetryEnabled {
		t.Error("TelemetryEnabled should still be true after persistence failure")
	}
}

func TestTelemetryUpdate_UnrelatedUpdateDoesNotToggle(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:         tempDir,
		ConfigPath:       tempDir,
		TelemetryEnabled: true,
		EnvOverrides:     make(map[string]bool),
	}
	handler, persistence, token := setupTelemetryTest(t, cfg)

	// Persist with telemetry enabled.
	initial := config.DefaultSystemSettings()
	enabled := true
	initial.TelemetryEnabled = &enabled
	if err := persistence.SaveSystemSettings(*initial); err != nil {
		t.Fatal(err)
	}

	// Track whether toggle callback fires.
	toggleCalled := false
	handler.SetTelemetryToggleFunc(func(en bool) {
		toggleCalled = true
	})

	// Send an update that does NOT include telemetryEnabled.
	body, _ := json.Marshal(map[string]interface{}{"theme": "dark"})
	req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if toggleCalled {
		t.Error("telemetry toggle callback should NOT fire for unrelated settings updates")
	}
}
