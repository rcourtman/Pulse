package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func TestHandleMetricsStoreStats_MethodNotAllowed(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodPost, "/api/metrics/store/stats", nil)
	rec := httptest.NewRecorder()

	router.handleMetricsStoreStats(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleMetricsStoreStats_NoMonitor(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/api/metrics/store/stats", nil)
	rec := httptest.NewRecorder()

	router.handleMetricsStoreStats(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestHandleMetricsStoreStats_NoStore(t *testing.T) {
	monitor, _, _ := newTestMonitor(t)
	router := &Router{monitor: monitor}

	req := httptest.NewRequest(http.MethodGet, "/api/metrics/store/stats", nil)
	rec := httptest.NewRecorder()

	router.handleMetricsStoreStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["enabled"] != false {
		t.Fatalf("expected enabled=false, got %#v", payload["enabled"])
	}
}

func TestHandleMetricsStoreStats_WithStore(t *testing.T) {
	monitor, _, _ := newTestMonitor(t)
	store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("metrics.NewStore error: %v", err)
	}
	defer store.Close()

	setUnexportedField(t, monitor, "metricsStore", store)
	router := &Router{monitor: monitor}

	req := httptest.NewRequest(http.MethodGet, "/api/metrics/store/stats", nil)
	rec := httptest.NewRecorder()

	router.handleMetricsStoreStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["enabled"] != true {
		t.Fatalf("expected enabled=true, got %#v", payload["enabled"])
	}
}

func TestHandleDiagnosticsDockerPrepareToken_MethodNotAllowed(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/api/diagnostics/docker/prepare-token", nil)
	rec := httptest.NewRecorder()

	router.handleDiagnosticsDockerPrepareToken(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleDiagnosticsDockerPrepareToken_InvalidJSON(t *testing.T) {
	router := &Router{monitor: &monitoring.Monitor{}, config: &config.Config{}}
	req := httptest.NewRequest(http.MethodPost, "/api/diagnostics/docker/prepare-token", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	router.handleDiagnosticsDockerPrepareToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleDiagnosticsDockerPrepareToken_MissingHostID(t *testing.T) {
	router := &Router{monitor: &monitoring.Monitor{}, config: &config.Config{}}
	body := bytes.NewBufferString(`{"hostId":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/diagnostics/docker/prepare-token", body)
	rec := httptest.NewRecorder()

	router.handleDiagnosticsDockerPrepareToken(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleDiagnosticsDockerPrepareToken_HostNotFound(t *testing.T) {
	monitor, _, _ := newTestMonitor(t)
	router := &Router{monitor: monitor, config: &config.Config{}}
	body := bytes.NewBufferString(`{"hostId":"missing"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/diagnostics/docker/prepare-token", body)
	rec := httptest.NewRecorder()

	router.handleDiagnosticsDockerPrepareToken(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleDiagnosticsDockerPrepareToken_Success(t *testing.T) {
	monitor, state, _ := newTestMonitor(t)
	state.DockerHosts = []models.DockerHost{{ID: "host-1", DisplayName: "Docker Host"}}

	router := &Router{monitor: monitor, config: &config.Config{PublicURL: "https://pulse.example.com"}}
	body := bytes.NewBufferString(`{"hostId":"host-1","tokenName":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/diagnostics/docker/prepare-token", body)
	rec := httptest.NewRecorder()

	router.handleDiagnosticsDockerPrepareToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ok, _ := payload["success"].(bool); !ok {
		t.Fatalf("expected success true, got %#v", payload["success"])
	}
	if payload["token"] == "" {
		t.Fatalf("expected token in response")
	}
	host, _ := payload["host"].(map[string]interface{})
	if host["id"] != "host-1" {
		t.Fatalf("unexpected host id: %#v", host["id"])
	}
	if !strings.Contains(payload["installCommand"].(string), "https://pulse.example.com") {
		t.Fatalf("expected install command to include base URL")
	}
	if len(router.config.APITokens) == 0 {
		t.Fatalf("expected API token to be recorded")
	}
}

func TestHandleDownloadDockerInstallerScript_MethodNotAllowed(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodPost, "/download/install-docker.sh", nil)
	rec := httptest.NewRecorder()

	router.handleDownloadDockerInstallerScript(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleDownloadDockerInstallerScript_ServesFile(t *testing.T) {
	root := t.TempDir()
	scriptPath := filepath.Join(root, "scripts", "install-docker.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0o755); err != nil {
		t.Fatalf("mkdir scripts dir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho docker\n"), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	router := &Router{projectRoot: root}
	req := httptest.NewRequest(http.MethodGet, "/download/install-docker.sh", nil)
	rec := httptest.NewRecorder()

	router.handleDownloadDockerInstallerScript(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/x-shellscript" {
		t.Fatalf("expected text/x-shellscript, got %q", ct)
	}
}

func TestKnowledgeStoreProviderWrapper(t *testing.T) {
	wrapper := &knowledgeStoreProviderWrapper{}
	if err := wrapper.SaveNote("res-1", "note", "service"); err == nil {
		t.Fatalf("expected error when store is nil")
	}
	if got := wrapper.GetKnowledge("res-1", ""); got != nil {
		t.Fatalf("expected nil knowledge when store is nil")
	}

	store, err := knowledge.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("knowledge.NewStore error: %v", err)
	}
	wrapper.store = store

	if err := wrapper.SaveNote("res-1", "hello", "service"); err != nil {
		t.Fatalf("SaveNote error: %v", err)
	}

	entries := wrapper.GetKnowledge("res-1", "service")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Category != "service" || entries[0].Note != "hello" {
		t.Fatalf("unexpected entry: %#v", entries[0])
	}

	all := wrapper.GetKnowledge("res-1", "")
	if len(all) != 1 {
		t.Fatalf("expected 1 entry from full query, got %d", len(all))
	}
}
