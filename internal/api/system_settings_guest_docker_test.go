package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// The Proxmox guest Docker inventory opt-in rides the admin-gated system
// settings endpoint: it must persist, apply to the runtime config, and fire
// the reconfigure hook exactly when its value changes — and the environment
// override must lock it out of the API entirely.

func TestGuestDockerInventoryUpdate_PersistsAppliesAndFiresHook(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:     tempDir,
		ConfigPath:   tempDir,
		EnvOverrides: make(map[string]bool),
	}
	handler, persistence, token := setupTelemetryTest(t, cfg)

	hookCalls := 0
	handler.SetGuestDockerInventoryToggleFunc(func() {
		hookCalls++
	})

	post := func(payload map[string]interface{}) *httptest.ResponseRecorder {
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
		req.Header.Set("X-API-Token", token)
		rec := httptest.NewRecorder()
		handler.HandleUpdateSystemSettings(rec, req)
		return rec
	}

	rec := post(map[string]interface{}{"enableProxmoxGuestDockerInventory": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !cfg.EnableProxmoxGuestDockerInventory {
		t.Fatal("runtime config should have the opt-in enabled after update")
	}
	if hookCalls != 1 {
		t.Fatalf("expected reconfigure hook to fire once, fired %d times", hookCalls)
	}
	saved, err := persistence.LoadSystemSettings()
	if err != nil {
		t.Fatal(err)
	}
	if saved == nil || !saved.EnableProxmoxGuestDockerInventory {
		t.Fatal("opt-in should be persisted in system settings")
	}

	// Re-sending the same value is a no-op for the hook.
	rec = post(map[string]interface{}{"enableProxmoxGuestDockerInventory": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if hookCalls != 1 {
		t.Fatalf("hook must not fire when the value did not change, fired %d times", hookCalls)
	}

	// Turning it back off fires the hook again.
	rec = post(map[string]interface{}{"enableProxmoxGuestDockerInventory": false})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if cfg.EnableProxmoxGuestDockerInventory {
		t.Fatal("runtime config should have the opt-in disabled after update")
	}
	if hookCalls != 2 {
		t.Fatalf("expected reconfigure hook to fire on disable, fired %d times", hookCalls)
	}
}

func TestGuestDockerInventoryUpdate_EnvOverrideRejected(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:                          tempDir,
		ConfigPath:                        tempDir,
		EnableProxmoxGuestDockerInventory: true,
		EnvOverrides: map[string]bool{
			"PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY": true,
			"enableProxmoxGuestDockerInventory":           true,
		},
	}
	handler, _, token := setupTelemetryTest(t, cfg)

	hookCalls := 0
	handler.SetGuestDockerInventoryToggleFunc(func() {
		hookCalls++
	})

	body, _ := json.Marshal(map[string]interface{}{"enableProxmoxGuestDockerInventory": false})
	req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for env-locked opt-in, got %d: %s", rec.Code, rec.Body.String())
	}
	if !cfg.EnableProxmoxGuestDockerInventory {
		t.Fatal("env-locked opt-in must not be changed through the API")
	}
	if hookCalls != 0 {
		t.Fatalf("hook must not fire on a rejected update, fired %d times", hookCalls)
	}
}

func TestGuestDockerInventoryUpdate_UnrelatedUpdateDoesNotClobber(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		// Runtime value differs from anything on disk (simulating an env
		// override or a pre-persistence enable): an unrelated settings save
		// must not silently flip it.
		EnableProxmoxGuestDockerInventory: true,
		EnvOverrides:                      make(map[string]bool),
	}
	handler, _, token := setupTelemetryTest(t, cfg)

	hookCalls := 0
	handler.SetGuestDockerInventoryToggleFunc(func() {
		hookCalls++
	})

	body, _ := json.Marshal(map[string]interface{}{"theme": "dark"})
	req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !cfg.EnableProxmoxGuestDockerInventory {
		t.Fatal("unrelated settings updates must not change the guest Docker opt-in")
	}
	if hookCalls != 0 {
		t.Fatalf("hook must not fire for unrelated settings updates, fired %d times", hookCalls)
	}
}

func TestGuestDockerInventoryGet_ExposesEffectiveValue(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		// Enabled by environment; disk has no opt-in recorded.
		EnableProxmoxGuestDockerInventory: true,
		EnvOverrides: map[string]bool{
			"PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY": true,
			"enableProxmoxGuestDockerInventory":           true,
		},
	}
	handler, persistence, token := setupTelemetryTest(t, cfg)

	if err := persistence.SaveSystemSettings(*config.DefaultSystemSettings()); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/system-settings", nil)
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	handler.HandleGetSystemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		EnableProxmoxGuestDockerInventory bool            `json:"enableProxmoxGuestDockerInventory"`
		EnvOverrides                      map[string]bool `json:"envOverrides"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.EnableProxmoxGuestDockerInventory {
		t.Fatal("GET must expose the effective (env-enabled) opt-in value")
	}
	if !response.EnvOverrides["enableProxmoxGuestDockerInventory"] {
		t.Fatal("GET must expose the env override lock for the opt-in")
	}
}
