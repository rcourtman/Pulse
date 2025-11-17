package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleUpdateSystemSettingsRejectsTempsWithoutTransport(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DOCKER", "true")
	cfg := &config.Config{DataPath: tempDir, ConfigPath: tempDir, PVEInstances: []config.PVEInstance{{Name: "pve-a"}}}
	persistence := config.NewConfigPersistence(tempDir)
	handler := NewSystemSettingsHandler(cfg, persistence, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/system/settings/update", bytes.NewBufferString(`{"temperatureMonitoringEnabled":true}`))
	rec := httptest.NewRecorder()

	handler.HandleUpdateSystemSettings(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}
