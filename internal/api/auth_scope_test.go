package api

import (
	"context"
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

func TestRespondMissingScopeNilWriter(t *testing.T) {
	// Should not panic when called with nil writer
	respondMissingScope(nil, "some-scope")
}

func TestAttachAPITokenRecordNilRecord(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	originalCtx := req.Context()

	// Should not panic and should not modify context when record is nil
	attachAPITokenRecord(req, nil)

	// Context should remain unchanged
	if req.Context() != originalCtx {
		t.Fatal("context should not change when attaching nil record")
	}
}

func TestGetAPITokenRecordFromRequestWrongType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	// Attach a value of wrong type to the context
	ctx := req.Context()
	ctx = context.WithValue(ctx, contextKeyAPIToken, "not-a-record")
	req = req.WithContext(ctx)

	// Should return nil when type assertion fails
	record := getAPITokenRecordFromRequest(req)
	if record != nil {
		t.Fatal("expected nil when context value is wrong type")
	}
}

func TestGetAPITokenRecordFromRequestNoValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	// No token attached - should return nil
	record := getAPITokenRecordFromRequest(req)
	if record != nil {
		t.Fatal("expected nil when no token in context")
	}
}
