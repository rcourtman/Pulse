package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestHealthHandler_HealthzAlwaysOK(t *testing.T) {
	var ready atomic.Bool
	h := healthHandler(&ready)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestHealthHandler_ReadyzDependsOnReadyFlag(t *testing.T) {
	var ready atomic.Bool
	h := healthHandler(&ready)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	ready.Store(true)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHealthHandler_ReadyzExplainsModuleReadiness(t *testing.T) {
	var ready atomic.Bool
	runtimeStatus := newRuntimeHealth(&ready, map[string]bool{
		"host":       true,
		"docker":     true,
		"kubernetes": false,
	})
	h := healthHandler(&ready, runtimeStatus)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	runtimeStatus.setState("host", moduleStateRunning, nil)
	runtimeStatus.setState("docker", moduleStateRetrying, errors.New("docker socket unavailable"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 while docker retries, got %d", rec.Code)
	}
	var waiting runtimeHealthSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &waiting); err != nil {
		t.Fatalf("decode readiness response: %v", err)
	}
	if waiting.Ready || len(waiting.Modules) != 3 {
		t.Fatalf("waiting snapshot = %+v", waiting)
	}
	if waiting.Modules[0].Name != "docker" || waiting.Modules[0].State != moduleStateRetrying || waiting.Modules[0].LastError != "docker socket unavailable" {
		t.Fatalf("docker readiness evidence = %+v", waiting.Modules[0])
	}

	runtimeStatus.setState("docker", moduleStateRunning, nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !ready.Load() {
		t.Fatalf("expected ready after enabled modules run, status=%d ready=%v body=%s", rec.Code, ready.Load(), rec.Body.String())
	}
}
