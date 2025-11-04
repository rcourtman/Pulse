package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleSetupScriptRejectsUnsafeAuthToken(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve&host=https://example.com&auth_token=$(touch%20/tmp/pwned)", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 bad request for unsafe auth token, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestHandleSetupScriptRejectsUnsafePulseURL(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve&host=https://example.com&pulse_url=http://example.com%5C%0Aecho%20oops", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 bad request for unsafe pulse_url, got %d (%s)", rr.Code, rr.Body.String())
	}
}
