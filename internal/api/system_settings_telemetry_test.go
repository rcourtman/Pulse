package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
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

func TestSecurityOperatorDocsPublishEffectiveConfigTransferPolicy(t *testing.T) {
	rootSecurity, err := os.ReadFile(filepath.Clean("../../SECURITY.md"))
	if err != nil {
		t.Fatalf("read root security guide: %v", err)
	}
	shippedSecurity, err := os.ReadFile(filepath.Clean("../../frontend-modern/public/docs/SECURITY.md"))
	if err != nil {
		t.Fatalf("read shipped security guide: %v", err)
	}
	if !bytes.Equal(rootSecurity, shippedSecurity) {
		t.Fatal("shipped security guide drifted from the canonical root guide")
	}
	guide := strings.Join(strings.Fields(string(rootSecurity)), " ")
	for _, policy := range []string{
		"only to configuration export",
		"It never enables import",
		"direct loopback connection",
		"private-network and forwarded requests are not loopback",
		"settings:read",
		"settings:write",
		"organization-bound tokens",
	} {
		if !strings.Contains(guide, policy) {
			t.Fatalf("security guide missing config transfer policy %q", policy)
		}
	}
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

func TestRuntimeDisplayPublishesOnlyEffectiveTelemetryState(t *testing.T) {
	settings := config.DefaultSystemSettings()
	persistedEnabled := true
	settings.TelemetryEnabled = &persistedEnabled
	handler := newRuntimeDisplayHandler(t, &config.Config{
		TelemetryEnabled: false,
		EnvOverrides: map[string]bool{
			"PULSE_TELEMETRY": true,
		},
	}, settings)

	display, raw := fetchRuntimeDisplay(t, handler)
	if display.TelemetryEnabled {
		t.Fatal("runtime telemetry state = enabled, want effective disabled override")
	}
	if _, ok := raw["telemetryEnabled"]; !ok {
		t.Fatal("runtime display omitted telemetryEnabled")
	}
	if _, ok := raw["telemetryPreview"]; ok {
		t.Fatal("runtime display exposed admin-only telemetry preview")
	}
}

// The monitoring cadence rides the same session-tier projection as the
// telemetry state. Publishing it must not drag the admin-only settings
// payload's neighbours along with it.
func TestRuntimeDisplayCadenceProjectionStaysNarrow(t *testing.T) {
	settings := config.DefaultSystemSettings()
	settings.PVEPollingInterval = 45
	handler := newRuntimeDisplayHandler(t, &config.Config{}, settings)

	display, raw := fetchRuntimeDisplay(t, handler)
	if display.PVEPollingInterval != 45 {
		t.Fatalf("pvePollingInterval = %d, want 45", display.PVEPollingInterval)
	}
	if _, ok := raw["pvePollingInterval"]; !ok {
		t.Fatal("runtime display omitted pvePollingInterval")
	}
	for _, adminKey := range []string{"envOverrides", "discoveryConfig", "allowedOrigins", "backupPollingInterval", "publicURL"} {
		if _, ok := raw[adminKey]; ok {
			t.Fatalf("runtime display exposed admin-only %q", adminKey)
		}
	}
}

func TestTelemetryPreview_ReturnsCurrentPayload(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:         tempDir,
		ConfigPath:       tempDir,
		TelemetryEnabled: false,
		EnvOverrides:     make(map[string]bool),
	}
	handler, _, _ := setupTelemetryTest(t, cfg)
	handler.SetTelemetryPreviewFunc(func() (telemetry.Ping, error) {
		return telemetry.Ping{
			InstallID: "preview-install-id",
			Version:   "6.0.0",
			Event:     "heartbeat",
			Platform:  "docker",
			OS:        "linux",
			Arch:      "amd64",
		}, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/api/system/settings/telemetry-preview", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetTelemetryPreview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response TelemetryPreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.Enabled {
		t.Fatal("expected telemetry preview to reflect disabled runtime state")
	}
	if response.Payload.InstallID != "preview-install-id" {
		t.Fatalf("install_id = %q, want preview-install-id", response.Payload.InstallID)
	}
	if response.Payload.Event != "heartbeat" {
		t.Fatalf("event = %q, want heartbeat", response.Payload.Event)
	}
}

func TestTelemetryReset_ReturnsUpdatedPayload(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:         tempDir,
		ConfigPath:       tempDir,
		TelemetryEnabled: true,
		EnvOverrides:     make(map[string]bool),
	}
	handler, _, token := setupTelemetryTest(t, cfg)
	handler.SetTelemetryResetFunc(func() (telemetry.Ping, error) {
		return telemetry.Ping{
			InstallID: "rotated-install-id",
			Version:   "6.0.0",
			Event:     "heartbeat",
			Platform:  "binary",
			OS:        "linux",
			Arch:      "amd64",
		}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/api/system/settings/telemetry-reset-id", nil)
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()

	handler.HandleResetTelemetryID(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response TelemetryPreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !response.Enabled {
		t.Fatal("expected telemetry reset response to reflect enabled runtime state")
	}
	if response.Payload.InstallID != "rotated-install-id" {
		t.Fatalf("install_id = %q, want rotated-install-id", response.Payload.InstallID)
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
	toggleCalled := false
	handler.SetTelemetryToggleFunc(func(bool) { toggleCalled = true })

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
		_ = os.Chmod(tempDir, 0700)    // restore so TempDir cleanup works
		_ = os.Chmod(systemFile, 0600) // restore so TempDir cleanup works
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
	if toggleCalled {
		t.Fatal("failed persistence invoked the telemetry toggle")
	}
}

func TestTelemetryUpdate_OnlyPreferenceTransitionsToggle(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath: tempDir, ConfigPath: tempDir, TelemetryEnabled: true,
		EnvOverrides: make(map[string]bool),
	}
	handler, persistence, token := setupTelemetryTest(t, cfg)
	settings := config.DefaultSystemSettings()
	enabled := true
	settings.TelemetryEnabled = &enabled
	if err := persistence.SaveSystemSettings(*settings); err != nil {
		t.Fatal(err)
	}
	var toggles []bool
	handler.SetTelemetryToggleFunc(func(value bool) {
		// The sender must observe the committed preference, not a proposed
		// value that could still fail persistence.
		persisted, err := persistence.LoadSystemSettings()
		if err != nil || persisted.TelemetryEnabled == nil || *persisted.TelemetryEnabled != value {
			t.Fatalf("toggle preceded persistence: settings=%#v error=%v", persisted, err)
		}
		if cfg.TelemetryEnabled != value {
			t.Fatalf("toggle preceded runtime mutation: enabled=%v want=%v", cfg.TelemetryEnabled, value)
		}
		toggles = append(toggles, value)
	})
	for i, value := range []bool{true, true, false, false, true, true} {
		body, err := json.Marshal(map[string]interface{}{"telemetryEnabled": value})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
		req.Header.Set("X-API-Token", token)
		rec := httptest.NewRecorder()
		handler.HandleUpdateSystemSettings(rec, req)
		if rec.Code != http.StatusOK || cfg.TelemetryEnabled != value {
			t.Fatalf("save %d returned %d, enabled=%v: %s", i, rec.Code, cfg.TelemetryEnabled, rec.Body.String())
		}
		persisted, err := persistence.LoadSystemSettings()
		if err != nil || persisted.TelemetryEnabled == nil || *persisted.TelemetryEnabled != value {
			t.Fatalf("save %d did not preserve requested preference: settings=%#v error=%v", i, persisted, err)
		}
		wantToggleCount := 0
		if i >= 2 {
			wantToggleCount++
		}
		if i >= 4 {
			wantToggleCount++
		}
		if len(toggles) != wantToggleCount {
			t.Fatalf("save %d toggles = %v, want %d transitions", i, toggles, wantToggleCount)
		}
	}
	if len(toggles) != 2 || toggles[0] != false || toggles[1] != true {
		t.Fatalf("runtime toggles = %v, want only the disable and re-enable transitions", toggles)
	}
}

func TestTelemetryUpdate_UnchangedPreferenceRepairsStaleDiskWithoutToggle(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("enabled=%v", enabled), func(t *testing.T) {
			tempDir := t.TempDir()
			cfg := &config.Config{
				DataPath: tempDir, ConfigPath: tempDir, TelemetryEnabled: enabled,
				EnvOverrides: make(map[string]bool),
			}
			handler, persistence, token := setupTelemetryTest(t, cfg)
			settings := config.DefaultSystemSettings()
			stale := !enabled
			settings.TelemetryEnabled = &stale
			if err := persistence.SaveSystemSettings(*settings); err != nil {
				t.Fatal(err)
			}
			handler.SetTelemetryToggleFunc(func(bool) { t.Fatal("unchanged effective preference invoked toggle") })
			body, err := json.Marshal(map[string]interface{}{"telemetryEnabled": enabled})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
			req.Header.Set("X-API-Token", token)
			rec := httptest.NewRecorder()
			handler.HandleUpdateSystemSettings(rec, req)
			if rec.Code != http.StatusOK || cfg.TelemetryEnabled != enabled {
				t.Fatalf("unchanged save returned %d, enabled=%v: %s", rec.Code, cfg.TelemetryEnabled, rec.Body.String())
			}
			persisted, err := persistence.LoadSystemSettings()
			if err != nil || persisted.TelemetryEnabled == nil || *persisted.TelemetryEnabled != enabled {
				t.Fatalf("unchanged save did not repair disk: settings=%#v error=%v", persisted, err)
			}
		})
	}
}

func TestTelemetryUpdate_NullPreservesEffectiveValueWithStalePreference(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath: tempDir, ConfigPath: tempDir, TelemetryEnabled: true,
		EnvOverrides: make(map[string]bool),
	}
	handler, persistence, token := setupTelemetryTest(t, cfg)
	settings := config.DefaultSystemSettings()
	stale := false
	settings.TelemetryEnabled = &stale
	if err := persistence.SaveSystemSettings(*settings); err != nil {
		t.Fatal(err)
	}
	handler.SetTelemetryToggleFunc(func(bool) { t.Fatal("null preference invoked toggle") })
	req := httptest.NewRequest(http.MethodPost, "/api/system-settings", strings.NewReader(`{"telemetryEnabled":null}`))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	handler.HandleUpdateSystemSettings(rec, req)
	if rec.Code != http.StatusOK || !cfg.TelemetryEnabled {
		t.Fatalf("null preference changed runtime: status=%d enabled=%v", rec.Code, cfg.TelemetryEnabled)
	}
	persisted, err := persistence.LoadSystemSettings()
	if err != nil || persisted.TelemetryEnabled == nil || *persisted.TelemetryEnabled {
		t.Fatalf("null preference changed disk: settings=%#v error=%v", persisted, err)
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

func TestCIDRUpdate_InvalidCIDRRejectedBeforePersist(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:         tempDir,
		ConfigPath:       tempDir,
		TelemetryEnabled: true,
		EnvOverrides:     make(map[string]bool),
	}
	handler, persistence, token := setupTelemetryTest(t, cfg)

	initial := config.DefaultSystemSettings()
	initial.WebhookAllowedPrivateCIDRs = "192.168.1.0/24"
	if err := persistence.SaveSystemSettings(*initial); err != nil {
		t.Fatal(err)
	}

	// Send an invalid CIDR value — should be rejected with 400 before persisting.
	body, _ := json.Marshal(map[string]interface{}{"webhookAllowedPrivateCIDRs": "not-a-cidr"})
	req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid CIDR, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify persisted value was NOT changed.
	saved, err := persistence.LoadSystemSettings()
	if err != nil {
		t.Fatal(err)
	}
	if saved.WebhookAllowedPrivateCIDRs != "192.168.1.0/24" {
		t.Errorf("persisted CIDRs should be unchanged, got %q", saved.WebhookAllowedPrivateCIDRs)
	}
}

func TestCIDRUpdate_ValidCIDRPersisted(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:         tempDir,
		ConfigPath:       tempDir,
		TelemetryEnabled: true,
		EnvOverrides:     make(map[string]bool),
	}
	handler, persistence, token := setupTelemetryTest(t, cfg)

	initial := config.DefaultSystemSettings()
	if err := persistence.SaveSystemSettings(*initial); err != nil {
		t.Fatal(err)
	}

	// Send a valid CIDR value.
	body, _ := json.Marshal(map[string]interface{}{"webhookAllowedPrivateCIDRs": "10.0.0.0/8, 172.16.0.0/12"})
	req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify persisted value was updated.
	saved, err := persistence.LoadSystemSettings()
	if err != nil {
		t.Fatal(err)
	}
	if saved.WebhookAllowedPrivateCIDRs != "10.0.0.0/8, 172.16.0.0/12" {
		t.Errorf("persisted CIDRs should be updated, got %q", saved.WebhookAllowedPrivateCIDRs)
	}
}

// The autoUpdateCheckInterval and autoUpdateTime settings were removed as dead
// surface: nothing ever consumed them (the systemd timer schedule is rendered
// by install.sh) and no UI control set them, so accepting and persisting them
// only pretended a schedule preference existed (#1643/#1637 triage). Legacy
// clients may still send the fields; the update endpoint must ignore them
// without erroring — including values the retired validation used to reject —
// and must not write them back into system.json.
func TestSystemSettingsUpdate_LegacyAutoUpdateFieldsIgnored(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:     tempDir,
		ConfigPath:   tempDir,
		EnvOverrides: map[string]bool{},
	}
	handler, persistence, token := setupTelemetryTest(t, cfg)

	initial := config.DefaultSystemSettings()
	if err := persistence.SaveSystemSettings(*initial); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]interface{}{
		"autoUpdateCheckInterval": float64(9999), // rejected by the retired validation
		"autoUpdateTime":          "99:99",
		"connectionTimeout":       float64(30),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("legacy auto-update fields must be ignored, got %d: %s", rec.Code, rec.Body.String())
	}

	raw, err := os.ReadFile(filepath.Join(tempDir, "system.json"))
	if err != nil {
		t.Fatalf("read persisted system.json: %v", err)
	}
	var persisted map[string]interface{}
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("parse persisted system.json: %v", err)
	}
	for _, key := range []string{"autoUpdateCheckInterval", "autoUpdateTime"} {
		if _, ok := persisted[key]; ok {
			t.Fatalf("persisted system.json still carries dead setting %q:\n%s", key, raw)
		}
	}

	saved, err := persistence.LoadSystemSettings()
	if err != nil {
		t.Fatalf("load persisted settings: %v", err)
	}
	if saved.ConnectionTimeout != 30 {
		t.Fatalf("real setting alongside legacy fields was dropped: connectionTimeout=%d", saved.ConnectionTimeout)
	}
}

// TestIssue1638SettingsSaveResetsSSHFailureBackoff pins that saving system
// settings clears the temperature SSH failure backoff on every live monitor.
// A settings save is an operator touchpoint that often follows repairing SSH
// access, and the on-disk key-change check only notices key file replacement,
// so without this the operator waits out a backoff window that may have
// compounded to fifteen minutes before Pulse retries (#1638).
func TestIssue1638SettingsSaveResetsSSHFailureBackoff(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:     tempDir,
		ConfigPath:   tempDir,
		EnvOverrides: make(map[string]bool),
	}
	persistence := config.NewConfigPersistence(cfg.DataPath)

	tokenVal := "ssh-reset-test-token-123.12345678"
	tokenHash := internalauth.HashAPIToken(tokenVal)
	cfg.APITokens = []config.APITokenRecord{
		{ID: "tok1", Hash: tokenHash, Name: "Test Token"},
	}

	monitor := &mockMonitor{}
	handler := newTestSystemSettingsHandler(cfg, persistence, monitor, func() {}, func() error { return nil })

	initial := config.DefaultSystemSettings()
	if err := persistence.SaveSystemSettings(*initial); err != nil {
		t.Fatal(err)
	}

	// A save that touches no SSH-related field must still clear the backoff:
	// the reset is tied to the save itself, not to any particular setting.
	body, _ := json.Marshal(map[string]interface{}{"connectionTimeout": 30})
	req := httptest.NewRequest(http.MethodPost, "/api/system-settings", bytes.NewReader(body))
	req.Header.Set("X-API-Token", tokenVal)
	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if monitor.resetSSHFailureBackoffCalls != 1 {
		t.Fatalf("settings save called ResetSSHFailureBackoff %d times, want 1", monitor.resetSSHFailureBackoffCalls)
	}
}

// Clearing browser trust must be an explicit patch, not a falsy-value no-op.
func TestAllowedOriginsPatchBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, saved, effective, body, wantSaved, wantEffective string
		override                                               string
		wantStatus                                             int
	}{
		{"clear wildcard", "*", "*", `{"allowedOrigins":""}`, "", "", "", http.StatusOK},
		{"clear exact origins", "https://old.example", "https://old.example", `{"allowedOrigins":""}`, "", "", "", http.StatusOK},
		{"omission keeps effective policy", "https://saved.example", "https://effective.example", `{"theme":"dark"}`, "https://saved.example", "https://effective.example", "", http.StatusOK},
		{"null is not a clear", "*", "*", `{"allowedOrigins":null}`, "*", "*", "", http.StatusBadRequest},
		{"deployment lock uppercase", "https://saved.example", "https://deploy.example", `{"allowedOrigins":""}`, "https://saved.example", "https://deploy.example", "ALLOWED_ORIGINS", http.StatusConflict},
		{"deployment lock field name", "https://saved.example", "https://deploy.example", `{"allowedOrigins":"*"}`, "https://saved.example", "https://deploy.example", "allowedOrigins", http.StatusConflict},
		{"unchanged locked form saves other fields", "https://saved.example", "https://deploy.example", `{"allowedOrigins":"https://deploy.example","theme":"dark"}`, "https://saved.example", "https://deploy.example", "allowedOrigins", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := &config.Config{DataPath: dir, ConfigPath: dir, AllowedOrigins: tc.effective, EnvOverrides: map[string]bool{}}
			if tc.override != "" {
				cfg.EnvOverrides[tc.override] = true
			}
			h, persistence, token := setupTelemetryTest(t, cfg)
			initial := config.DefaultSystemSettings()
			initial.AllowedOrigins = tc.saved
			if err := persistence.SaveSystemSettings(*initial); err != nil {
				t.Fatal(err)
			}
			reloads := 0
			h.reloadSystemSettingsFunc = func() { reloads++ }
			req := httptest.NewRequest(http.MethodPost, "/api/system/settings/update", strings.NewReader(tc.body))
			req.Header.Set("X-API-Token", token)
			rec := httptest.NewRecorder()
			h.HandleUpdateSystemSettings(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			saved, err := persistence.LoadSystemSettings()
			if err != nil || saved == nil {
				t.Fatalf("read saved policy: %v", err)
			}
			if saved.AllowedOrigins != tc.wantSaved || cfg.AllowedOrigins != tc.wantEffective {
				t.Fatal("saved/effective policy did not honour field presence and deployment precedence")
			}
			if tc.wantStatus != http.StatusOK && reloads != 0 {
				t.Fatal("rejected patch reloaded runtime state")
			}
			if tc.wantStatus == http.StatusOK && reloads != 1 {
				t.Fatal("successful patch did not reload settings")
			}
			if tc.name == "unchanged locked form saves other fields" && saved.Theme != "dark" {
				t.Fatal("locked value prevented an unrelated settings save")
			}
		})
	}
}

// Exercise the production middleware and handlers, disk, settings-cache reload
// and the production config loader; no simulated setter stands in for policy.
func TestAllowedOriginsSavedEffectiveLifecycle(t *testing.T) {
	for _, original := range []string{"*", "https://old.example"} {
		t.Run(original, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("PULSE_DATA_DIR", dir)
			t.Setenv("ALLOWED_ORIGINS", "")
			t.Setenv("PULSE_DEV", "false")
			t.Setenv("NODE_ENV", "production")
			InitPersistentAuthStores(dir)
			t.Cleanup(resetSessionStoreForTests)
			t.Cleanup(resetCSRFStoreForTests)
			cfg := &config.Config{DataPath: dir, ConfigPath: dir, AllowedOrigins: original, PublicURL: "https://pulse.example", AuthUser: "admin", AuthPass: "synthetic-unused-hash", EnvOverrides: map[string]bool{}, TLSCertFile: "synthetic-cert", TLSKeyFile: "synthetic-key"}
			h, persistence, token := setupTelemetryTest(t, cfg)
			initial := config.DefaultSystemSettings()
			initial.AllowedOrigins = original
			initial.AllowedEmbedOrigins = "https://frame.example"
			if err := persistence.SaveSystemSettings(*initial); err != nil {
				t.Fatal(err)
			}
			router := &Router{config: cfg, persistence: persistence, mux: http.NewServeMux()}
			h.reloadSystemSettingsFunc = router.reloadSystemSettings
			router.mux.HandleFunc("/api/system/settings", RequireAdmin(cfg, RequireScope(config.ScopeSettingsRead, h.HandleGetSystemSettings)))
			router.mux.HandleFunc("/api/system/settings/update", RequireAdmin(cfg, RequireScope(config.ScopeSettingsWrite, h.HandleUpdateSystemSettings)))
			router.reloadSystemSettings()
			request := func(method, path, origin, body, credential string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(method, path, strings.NewReader(body))
				if origin != "" {
					req.Header.Set("Origin", origin)
				}
				if credential != "" {
					req.Header.Set("X-API-Token", credential)
				}
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				return rec
			}
			assertOrigin := func(origin, want string) {
				t.Helper()
				rec := request(http.MethodGet, "/api/system/settings", origin, "", token)
				if rec.Code != http.StatusOK || rec.Header().Get("Access-Control-Allow-Origin") != want {
					t.Fatal("effective CORS response does not match saved policy")
				}
				if rec.Header().Get("Access-Control-Allow-Credentials") != "" && want == "" {
					t.Fatal("removed trust still enables credentialed CORS")
				}
				if want != "" && want != "*" && (rec.Header().Get("Access-Control-Allow-Credentials") != "true" || !strings.Contains(rec.Header().Get("Vary"), "Origin")) {
					t.Fatal("exact-origin credential/Vary contract changed")
				}
				if want == "*" && rec.Header().Get("Access-Control-Allow-Credentials") != "" {
					t.Fatal("wildcard gained credentialed browser access")
				}
			}
			origin := "https://old.example"
			assertOrigin(origin, original)
			rec := request(http.MethodPost, "/api/system/settings/update", "", `{"allowedOrigins":""}`, token)
			if rec.Code != http.StatusOK {
				t.Fatalf("clear: status %d", rec.Code)
			}
			for _, candidate := range []string{origin, "https://untrusted.example", ""} {
				assertOrigin(candidate, "")
			}
			preflight := request(http.MethodOptions, "/api/system/settings", origin, "", "")
			if preflight.Code != http.StatusOK || preflight.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("cleared policy still grants preflight access")
			}
			get := request(http.MethodGet, "/api/system/settings", "", "", token)
			var visible SystemSettingsResponse
			if err := json.Unmarshal(get.Body.Bytes(), &visible); err != nil || visible.AllowedOrigins != "" {
				t.Fatal("GET does not expose the effective clear")
			}
			saved, err := persistence.LoadSystemSettings()
			if err != nil || saved == nil || saved.AllowedOrigins != "" || saved.AllowedEmbedOrigins != initial.AllowedEmbedOrigins || saved.AllowEmbedding {
				t.Fatal("clear lost saved policy or widened embedding")
			}
			if cfg.TLSCertFile != "synthetic-cert" || cfg.TLSKeyFile != "synthetic-key" {
				t.Fatal("CORS edit altered TLS configuration")
			}
			restarted, err := config.LoadWithoutLoggingInit()
			if err != nil || restarted.AllowedOrigins != "" {
				t.Fatal("production config reload revived removed trust")
			}
			if rec := request(http.MethodGet, "/api/system/settings", origin, "", ""); rec.Code != http.StatusUnauthorized {
				t.Fatal("same-origin policy bypassed authentication")
			}
			// CORS grants never replace the session mutation's CSRF requirement.
			session := generateSessionToken()
			GetSessionStore().CreateSession(session, time.Hour, "cors-fixture", "192.0.2.1", "admin")
			t.Cleanup(func() { GetSessionStore().DeleteSession(session) })
			req := httptest.NewRequest(http.MethodPost, "/api/system/settings/update", strings.NewReader(`{"allowedOrigins":"*"}`))
			req.Header.Set("User-Agent", "cors-fixture")
			req.AddCookie(&http.Cookie{Name: sessionCookieName(false), Value: session})
			csrf := httptest.NewRecorder()
			router.ServeHTTP(csrf, req)
			if csrf.Code != http.StatusForbidden || cfg.AllowedOrigins != "" {
				t.Fatal("CORS clear weakened CSRF")
			}
			const readToken = "cors-read-synthetic.12345678"
			cfg.APITokens = append(cfg.APITokens, config.APITokenRecord{ID: "read-only", Hash: internalauth.HashAPIToken(readToken), Scopes: []string{config.ScopeSettingsRead}})
			if rec := request(http.MethodPost, "/api/system/settings/update", "", `{"allowedOrigins":"*"}`, readToken); rec.Code != http.StatusForbidden || cfg.AllowedOrigins != "" {
				t.Fatal("settings:read token widened trust")
			}
			// An explicit new exact list works; prefixes and paths do not match.
			rec = request(http.MethodPost, "/api/system/settings/update", "", `{"allowedOrigins":"https://trusted.example, https://other.example"}`, token)
			if rec.Code != http.StatusOK {
				t.Fatalf("set exact list: %d", rec.Code)
			}
			assertOrigin("https://trusted.example", "https://trusted.example")
			assertOrigin("https://other.example", "https://other.example")
			assertOrigin("https://trusted.example.evil", "")
			assertOrigin("https://trusted.example/", "")
			assertOrigin("", "")
		})
	}
}

func TestAllowedOriginsSaveFailurePreservesPolicy(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{DataPath: dir, ConfigPath: dir, AllowedOrigins: "https://trusted.example"}
	h, persistence, token := setupTelemetryTest(t, cfg)
	initial := config.DefaultSystemSettings()
	initial.AllowedOrigins = cfg.AllowedOrigins
	if err := persistence.SaveSystemSettings(*initial); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "system.json"))
	if err != nil {
		t.Fatal(err)
	}
	persistence.SetFileSystem(failingWriteFileSystem{})
	reloads := 0
	h.reloadSystemSettingsFunc = func() { reloads++ }
	req := httptest.NewRequest(http.MethodPost, "/api/system/settings/update", strings.NewReader(`{"allowedOrigins":""}`))
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	h.HandleUpdateSystemSettings(rec, req)
	after, err := os.ReadFile(filepath.Join(dir, "system.json"))
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusInternalServerError || cfg.AllowedOrigins != initial.AllowedOrigins || reloads != 0 || !bytes.Equal(before, after) {
		t.Fatal("failed save changed durable/effective browser trust")
	}
}

func TestAllowedOriginsGetUsesDeploymentPolicy(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{DataPath: dir, ConfigPath: dir, AllowedOrigins: "https://deploy.example", EnvOverrides: map[string]bool{"allowedOrigins": true}}
	h, persistence, token := setupTelemetryTest(t, cfg)
	initial := config.DefaultSystemSettings()
	initial.AllowedOrigins = "https://saved.example"
	if err := persistence.SaveSystemSettings(*initial); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/system/settings", nil)
	req.Header.Set("X-API-Token", token)
	rec := httptest.NewRecorder()
	h.HandleGetSystemSettings(rec, req)
	var response SystemSettingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil || rec.Code != http.StatusOK || response.AllowedOrigins != cfg.AllowedOrigins || !response.EnvOverrides["allowedOrigins"] {
		t.Fatal("GET misrepresented deployment-owned browser trust")
	}
}
