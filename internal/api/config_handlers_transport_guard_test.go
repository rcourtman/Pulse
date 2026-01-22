package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleAddNodeAllowsTempsWithoutTransport(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DOCKER", "true")
	cfg := &config.Config{DataPath: tempDir, ConfigPath: tempDir}
	handler := newTestConfigHandlers(t, cfg)

	body := bytes.NewBufferString(`{"type":"pve","name":"node-a","host":"pve-a.local","user":"root@pam","password":"secret","temperatureMonitoringEnabled":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/config/nodes", body)
	rec := httptest.NewRecorder()

	handler.HandleAddNode(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}
	if len(cfg.PVEInstances) != 1 || cfg.PVEInstances[0].TemperatureMonitoringEnabled == nil || !*cfg.PVEInstances[0].TemperatureMonitoringEnabled {
		t.Fatalf("expected temperature monitoring to be enabled, got %+v", cfg.PVEInstances)
	}
}

func TestHandleUpdateNodeAllowsTempsWithoutTransport(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DOCKER", "true")
	cfg := &config.Config{DataPath: tempDir, ConfigPath: tempDir}
	cfg.PVEInstances = []config.PVEInstance{{
		Name: "pve-a",
		Host: "https://pve-a.local:8006",
	}}
	handler := newTestConfigHandlers(t, cfg)

	body := bytes.NewBufferString(`{"temperatureMonitoringEnabled":true}`)
	req := httptest.NewRequest(http.MethodPut, "/api/config/nodes/pve-0", body)
	rec := httptest.NewRecorder()

	handler.HandleUpdateNode(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if cfg.PVEInstances[0].TemperatureMonitoringEnabled == nil || !*cfg.PVEInstances[0].TemperatureMonitoringEnabled {
		t.Fatalf("expected temperature monitoring to be enabled, got %+v", cfg.PVEInstances[0].TemperatureMonitoringEnabled)
	}
}
