package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleConfig_MethodNotAllowed(t *testing.T) {
	router := &Router{config: &config.Config{}}
	req := httptest.NewRequest(http.MethodPost, "/api/config", nil)
	rec := httptest.NewRecorder()

	router.handleConfig(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleConfig_Success(t *testing.T) {
	router := &Router{config: &config.Config{AutoUpdateEnabled: true, UpdateChannel: "beta"}}
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()

	router.handleConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["csrfProtection"] != false {
		t.Fatalf("expected csrfProtection=false, got %#v", payload["csrfProtection"])
	}
	if payload["autoUpdateEnabled"] != true {
		t.Fatalf("expected autoUpdateEnabled=true, got %#v", payload["autoUpdateEnabled"])
	}
	if payload["updateChannel"] != "stable" {
		t.Fatalf("expected updateChannel=stable, got %#v", payload["updateChannel"])
	}
}

func TestHandleConfig_DisablesAutoUpdatesOnRC(t *testing.T) {
	router := &Router{config: &config.Config{AutoUpdateEnabled: true, UpdateChannel: "rc"}}
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()

	router.handleConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["updateChannel"] != "rc" {
		t.Fatalf("expected updateChannel=rc, got %#v", payload["updateChannel"])
	}
	if payload["autoUpdateEnabled"] != false {
		t.Fatalf("expected autoUpdateEnabled=false on rc, got %#v", payload["autoUpdateEnabled"])
	}
}

func TestHandleSimpleStats(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/simple-stats", nil)
	rec := httptest.NewRecorder()

	router.handleSimpleStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("expected text/html content type, got %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "Simple Pulse Stats") {
		t.Fatalf("expected stats page HTML, got %q", rec.Body.String())
	}
}
