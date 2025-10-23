package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestRequireScopeAllowsSession(t *testing.T) {
	handler := RequireScope(config.ScopeSettingsWrite, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 for session request, got %d", rr.Code)
	}
}

func TestRequireScopeRejectsMissingScope(t *testing.T) {
	handler := RequireScope(config.ScopeSettingsWrite, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	record := config.APITokenRecord{ID: "token-1", Scopes: []string{config.ScopeMonitoringRead}}
	attachAPITokenRecord(req, &record)

	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 when scope missing, got %d", rr.Code)
	}
}

func TestRequireScopeAllowsMatchingScope(t *testing.T) {
	handler := RequireScope(config.ScopeDockerReport, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	record := config.APITokenRecord{ID: "token-2", Scopes: []string{config.ScopeDockerReport}}
	attachAPITokenRecord(req, &record)

	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 when scope present, got %d", rr.Code)
	}
}
