package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleUpdateSystemSettingsAllowsTempsWithoutTransport(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DOCKER", "true")
	cfg := &config.Config{DataPath: tempDir, ConfigPath: tempDir, PVEInstances: []config.PVEInstance{{Name: "pve-a"}}}
	persistence := config.NewConfigPersistence(tempDir)
	handler := newTestSystemSettingsHandler(cfg, persistence, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/system/settings/update", bytes.NewBufferString(`{"temperatureMonitoringEnabled":true}`))
	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !cfg.TemperatureMonitoringEnabled {
		t.Fatalf("expected temperature monitoring enabled, got %v", cfg.TemperatureMonitoringEnabled)
	}
}

func TestHandleUpdateSystemSettingsRejectsInvalidPVEPollingInterval(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:           tempDir,
		ConfigPath:         tempDir,
		PVEPollingInterval: 10 * time.Second,
	}
	persistence := config.NewConfigPersistence(tempDir)
	handler := newTestSystemSettingsHandler(cfg, persistence, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/system/settings/update", strings.NewReader(`{"pvePollingInterval":5}`))
	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHandleUpdateSystemSettingsUpdatesPVEPollingInterval(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:           tempDir,
		ConfigPath:         tempDir,
		PVEPollingInterval: 10 * time.Second,
	}
	persistence := config.NewConfigPersistence(tempDir)
	reloaded := false
	handler := newTestSystemSettingsHandler(cfg, persistence, nil, nil, func() error {
		reloaded = true
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/api/system/settings/update", strings.NewReader(`{"pvePollingInterval":30}`))
	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if cfg.PVEPollingInterval != 30*time.Second {
		t.Fatalf("expected interval to be 30s, got %v", cfg.PVEPollingInterval)
	}
	if !reloaded {
		t.Fatal("expected reload to be triggered")
	}
}
