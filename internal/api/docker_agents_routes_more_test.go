package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestNewDockerAgentHandlers_DefaultMonitorFromMultiTenant(t *testing.T) {
	monitor := &monitoring.Monitor{}
	mtm := &monitoring.MultiTenantMonitor{}
	setUnexportedField(t, mtm, "monitors", map[string]*monitoring.Monitor{
		"default": monitor,
	})

	handler := NewDockerAgentHandlers(mtm, nil, nil, nil)
	if handler.legacyMonitor != monitor {
		t.Fatalf("expected legacy monitor to be set from multi-tenant default")
	}
}

func TestDockerAgentHandlers_HandleDockerHostActions_Routes(t *testing.T) {
	handler, monitor := newDockerAgentHandlers(t, nil)
	hostID := seedDockerHost(t, monitor)

	req := httptest.NewRequest(http.MethodPost, "/api/agents/docker/hosts/"+hostID+"/allow-reenroll", nil)
	rec := httptest.NewRecorder()
	handler.HandleDockerHostActions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("allow-reenroll status = %d, want 200", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/agents/docker/hosts/"+hostID+"/unhide", nil)
	rec = httptest.NewRecorder()
	handler.HandleDockerHostActions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unhide status = %d, want 200", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/agents/docker/hosts/"+hostID+"/pending-uninstall", nil)
	rec = httptest.NewRecorder()
	handler.HandleDockerHostActions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("pending-uninstall status = %d, want 200", rec.Code)
	}

	body := []byte(`{"displayName":"New Name"}`)
	req = httptest.NewRequest(http.MethodPut, "/api/agents/docker/hosts/"+hostID+"/display-name", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handler.HandleDockerHostActions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("display-name status = %d, want 200", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/agents/docker/hosts/"+hostID+"/check-updates", nil)
	rec = httptest.NewRecorder()
	handler.HandleDockerHostActions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("check-updates status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Check for updates") {
		t.Fatalf("expected check updates message")
	}
}

func TestDockerAgentHandlers_HandleDockerHostActions_DeleteRoute(t *testing.T) {
	handler, monitor := newDockerAgentHandlers(t, nil)
	hostID := seedDockerHost(t, monitor)

	req := httptest.NewRequest(http.MethodDelete, "/api/agents/docker/hosts/"+hostID, nil)
	rec := httptest.NewRecorder()
	handler.HandleDockerHostActions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200", rec.Code)
	}
}

func TestDockerAgentHandlers_HandleDockerHostActions_MethodNotAllowed(t *testing.T) {
	handler := &DockerAgentHandlers{}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/docker/hosts/host-1/unknown", nil)
	rec := httptest.NewRecorder()
	handler.HandleDockerHostActions(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestDockerAgentHandlers_HandleDeleteHost_Errors(t *testing.T) {
	handler, _ := newDockerAgentHandlers(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/docker/hosts/host-1", nil)
	rec := httptest.NewRecorder()
	handler.HandleDeleteHost(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/agents/docker/hosts/", nil)
	rec = httptest.NewRecorder()
	handler.HandleDeleteHost(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/agents/docker/hosts/missing?hide=true", nil)
	rec = httptest.NewRecorder()
	handler.HandleDeleteHost(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/agents/docker/hosts/missing?force=true", nil)
	rec = httptest.NewRecorder()
	handler.HandleDeleteHost(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
