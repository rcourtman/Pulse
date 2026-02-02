package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func TestKubernetesAgentHandlers_SetMultiTenantMonitor(t *testing.T) {
	handler := &KubernetesAgentHandlers{}
	monitor := &monitoring.Monitor{}

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

func TestKubernetesAgentHandlers_HandleReport_Errors(t *testing.T) {
	handler := &KubernetesAgentHandlers{}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/kubernetes/report", nil)
	rec := httptest.NewRecorder()
	handler.HandleReport(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/agents/kubernetes/report", bytes.NewReader([]byte("{bad")))
	rec = httptest.NewRecorder()
	handler.HandleReport(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestKubernetesAgentHandlers_HandleClusterActions_MethodNotAllowed(t *testing.T) {
	handler := &KubernetesAgentHandlers{}

	req := httptest.NewRequest(http.MethodGet, "/api/agents/kubernetes/clusters/cluster-1/unknown", nil)
	rec := httptest.NewRecorder()
	handler.HandleClusterActions(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestKubernetesAgentHandlers_HandleDeleteCluster_Errors(t *testing.T) {
	handler, _ := newKubernetesAgentHandlers(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/agents/kubernetes/clusters/cluster-1", nil)
	rec := httptest.NewRecorder()
	handler.HandleDeleteCluster(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/agents/kubernetes/clusters/", nil)
	rec = httptest.NewRecorder()
	handler.HandleDeleteCluster(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/agents/kubernetes/clusters/missing", nil)
	rec = httptest.NewRecorder()
	handler.HandleDeleteCluster(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestKubernetesAgentHandlers_HandleAllowReenroll_MissingID(t *testing.T) {
	handler := &KubernetesAgentHandlers{}

	req := httptest.NewRequest(http.MethodPost, "/api/agents/kubernetes/clusters//allow-reenroll", nil)
	rec := httptest.NewRecorder()
	handler.HandleAllowReenroll(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestKubernetesAgentHandlers_HandleSetCustomDisplayName_InvalidJSON(t *testing.T) {
	handler := &KubernetesAgentHandlers{}

	req := httptest.NewRequest(http.MethodPut, "/api/agents/kubernetes/clusters/cluster-1/display-name", bytes.NewReader([]byte("{bad")))
	rec := httptest.NewRecorder()
	handler.HandleSetCustomDisplayName(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
