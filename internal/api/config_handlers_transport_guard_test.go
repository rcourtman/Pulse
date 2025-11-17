package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleAddNodeRejectsTempsWithoutTransport(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DOCKER", "true")
	cfg := &config.Config{DataPath: tempDir, ConfigPath: tempDir}
	handler := newTestConfigHandlers(t, cfg)

	body := bytes.NewBufferString(`{"type":"pve","name":"node-a","host":"pve-a.local","user":"root@pam","password":"secret","temperatureMonitoringEnabled":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/config/nodes", body)
	rec := httptest.NewRecorder()

	handler.HandleAddNode(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "proxy") {
		t.Fatalf("expected proxy error, got %s", rec.Body.String())
	}
}

func TestHandleUpdateNodeRejectsTempsWithoutTransport(t *testing.T) {
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

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "proxy") {
		t.Fatalf("expected proxy error, got %s", rec.Body.String())
	}
}
