package main

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/rs/zerolog"
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

func TestRunAsWindowsService_Stub(t *testing.T) {
	ran, err := runAsWindowsService(Config{}, zerolog.Nop())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if ran {
		t.Fatalf("expected ran=false on non-windows")
	}
}
