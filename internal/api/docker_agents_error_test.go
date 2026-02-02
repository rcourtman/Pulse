package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestDockerAgentHandlers_SetMonitorAndTenant(t *testing.T) {
	handler := &DockerAgentHandlers{}
	monitor := &monitoring.Monitor{}
	handler.SetMonitor(monitor)
	if handler.legacyMonitor != monitor {
		t.Fatalf("expected legacy monitor to be set")
	}

	mtm := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{
		"default": monitor,
	})

	handler.SetMultiTenantMonitor(mtm)
	if handler.mtMonitor != mtm {
		t.Fatalf("expected multi-tenant monitor to be set")
	}
	if handler.legacyMonitor != monitor {
		t.Fatalf("expected legacy monitor to be set from multi-tenant default")
	}
}

func TestDockerAgentHandlers_GetMonitorFallback(t *testing.T) {
	legacy := &monitoring.Monitor{}
	handler := &DockerAgentHandlers{legacyMonitor: legacy}

	if got := handler.getMonitor(context.Background()); got != legacy {
		t.Fatalf("expected legacy monitor fallback")
	}
}

func TestDockerAgentHandlers_HandleReport_Errors(t *testing.T) {
	handler := &DockerAgentHandlers{}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/docker/report", nil)
	rec := httptest.NewRecorder()
	handler.HandleReport(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/agents/docker/report", bytes.NewReader([]byte("{bad")))
	rec = httptest.NewRecorder()
	handler.HandleReport(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDockerAgentHandlers_HandleCommandAck_Errors(t *testing.T) {
	handler := &DockerAgentHandlers{}

	req := httptest.NewRequest(http.MethodPost, "/api/agents/docker/commands/123", nil)
	rec := httptest.NewRecorder()
	handler.HandleCommandAck(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/agents/docker/commands//ack", nil)
	rec = httptest.NewRecorder()
	handler.HandleCommandAck(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/agents/docker/commands/cmd-1/ack", bytes.NewReader([]byte("{bad")))
	rec = httptest.NewRecorder()
	handler.HandleCommandAck(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	payload := map[string]string{
		"hostId": "host-1",
		"status": "unknown",
	}
	body, _ := json.Marshal(payload)
	req = httptest.NewRequest(http.MethodPost, "/api/agents/docker/commands/cmd-2/ack", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handler.HandleCommandAck(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
