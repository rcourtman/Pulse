package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func newRuntimeDisplayHandler(t *testing.T, cfg *config.Config, settings *config.SystemSettings) *SystemSettingsHandler {
	t.Helper()
	tempDir := t.TempDir()
	if cfg == nil {
		cfg = &config.Config{}
	}
	cfg.DataPath = tempDir
	cfg.ConfigPath = tempDir
	persistence := config.NewConfigPersistence(tempDir)
	if settings != nil {
		if err := persistence.SaveSystemSettings(*settings); err != nil {
			t.Fatalf("save system settings: %v", err)
		}
	}
	return newTestSystemSettingsHandler(cfg, persistence, &mockMonitor{}, func() {}, func() error { return nil })
}

func fetchRuntimeDisplay(t *testing.T, handler *SystemSettingsHandler) (RuntimeDisplayResponse, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/display", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetRuntimeDisplay(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("runtime display = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	var typed RuntimeDisplayResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &typed); err != nil {
		t.Fatalf("decode runtime display: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode runtime display as object: %v", err)
	}
	return typed, raw
}

func TestHandleGetRuntimeDisplay_ProjectsPersistedPresentationValues(t *testing.T) {
	settings := config.DefaultSystemSettings()
	settings.Theme = "dark"
	settings.FullWidthMode = true
	settings.ReduceProUpsellNoise = true

	handler := newRuntimeDisplayHandler(t, &config.Config{}, settings)
	got, _ := fetchRuntimeDisplay(t, handler)

	want := RuntimeDisplayResponse{Theme: "dark", FullWidthMode: true, ReduceProUpsellNoise: true}
	if got != want {
		t.Fatalf("runtime display = %+v, want %+v", got, want)
	}
}

// Assert on the serialized response so a future embedded settings struct cannot
// silently widen this session-tier contract.
func TestHandleGetRuntimeDisplay_PublishesOnlyPresentationFields(t *testing.T) {
	settings := config.DefaultSystemSettings()
	settings.Theme = "light"
	settings.AllowedOrigins = "https://pulse.internal"
	settings.PublicURL = "https://pulse.example.com"

	handler := newRuntimeDisplayHandler(t, &config.Config{}, settings)
	_, raw := fetchRuntimeDisplay(t, handler)

	allowed := map[string]struct{}{
		"theme":                      {},
		"fullWidthMode":              {},
		"disableDockerUpdateActions": {},
		"reduceProUpsellNoise":       {},
	}
	for key := range raw {
		if _, ok := allowed[key]; !ok {
			t.Fatalf("runtime display published %q; the session-tier projection is a whitelist", key)
		}
	}
	for key := range allowed {
		if _, ok := raw[key]; !ok {
			t.Fatalf("runtime display omitted %q", key)
		}
	}
}

func TestHandleGetRuntimeDisplay_UsesEffectiveDockerUpdateActionsSetting(t *testing.T) {
	settings := config.DefaultSystemSettings()
	settings.DisableDockerUpdateActions = false

	handler := newRuntimeDisplayHandler(t, &config.Config{DisableDockerUpdateActions: true}, settings)
	got, _ := fetchRuntimeDisplay(t, handler)

	if !got.DisableDockerUpdateActions {
		t.Fatal("disableDockerUpdateActions = false, want the effective config override to win")
	}
}

func TestHandleGetRuntimeDisplay_RejectsNonGET(t *testing.T) {
	handler := newRuntimeDisplayHandler(t, &config.Config{}, config.DefaultSystemSettings())
	req := httptest.NewRequest(http.MethodPost, "/api/runtime/display", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetRuntimeDisplay(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/runtime/display = %d, want 405", rec.Code)
	}
}
