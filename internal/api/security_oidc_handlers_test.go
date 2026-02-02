package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestSecurityOIDCHandlers_MethodNotAllowed(t *testing.T) {
	router := &Router{config: &config.Config{}}
	req := httptest.NewRequest(http.MethodPost, "/api/security/oidc", nil)
	rr := httptest.NewRecorder()

	router.handleOIDCConfig(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestSecurityOIDCHandlers_UpdateConflictsAndValidation(t *testing.T) {
	cfg := &config.Config{PublicURL: "https://pulse.example.com", OIDC: &config.OIDCConfig{}}
	router := &Router{config: cfg}

	cfg.OIDC.EnvOverrides = map[string]bool{"OIDC_ENABLED": true}
	req := httptest.NewRequest(http.MethodPut, "/api/security/oidc", bytes.NewReader([]byte(`{"enabled":true}`)))
	rr := httptest.NewRecorder()
	router.handleOIDCConfig(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rr.Code)
	}

	cfg.OIDC.EnvOverrides = map[string]bool{}
	req = httptest.NewRequest(http.MethodPut, "/api/security/oidc", bytes.NewReader([]byte("{bad")))
	rr = httptest.NewRecorder()
	router.handleOIDCConfig(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	payload := map[string]any{
		"enabled": true,
	}
	body, _ := json.Marshal(payload)
	req = httptest.NewRequest(http.MethodPut, "/api/security/oidc", bytes.NewReader(body))
	rr = httptest.NewRecorder()
	router.handleOIDCConfig(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}
