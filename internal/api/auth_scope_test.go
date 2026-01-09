package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
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

func TestRequireScopeEmptyScopeAllowsAll(t *testing.T) {
	// Empty scope should allow all requests through without checking token
	handler := RequireScope("", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Attach a token with no scopes - should still be allowed through
	record := config.APITokenRecord{ID: "token-empty", Scopes: []string{}}
	attachAPITokenRecord(req, &record)

	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 when scope is empty, got %d", rr.Code)
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
	ctx = context.WithValue(ctx, auth.GetAPITokenContextKey(), "not-a-record")
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

// ensureScope tests

func TestEnsureScope_EmptyScopeAllowsAll(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	// Empty scope should always return true
	result := ensureScope(rr, req, "")
	if !result {
		t.Fatal("expected ensureScope to return true for empty scope")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected no status written (200 default), got %d", rr.Code)
	}
}

func TestEnsureScope_NoTokenAllowsAccess(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	// No token attached - should allow access (session-based request)
	result := ensureScope(rr, req, config.ScopeSettingsWrite)
	if !result {
		t.Fatal("expected ensureScope to return true when no token (session request)")
	}
}

func TestEnsureScope_TokenWithMatchingScopeAllowsAccess(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	record := config.APITokenRecord{ID: "token-1", Scopes: []string{config.ScopeMonitoringRead}}
	attachAPITokenRecord(req, &record)
	rr := httptest.NewRecorder()

	result := ensureScope(rr, req, config.ScopeMonitoringRead)
	if !result {
		t.Fatal("expected ensureScope to return true when token has matching scope")
	}
}

func TestEnsureScope_TokenWithMultipleScopesAllowsAccess(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	record := config.APITokenRecord{
		ID:     "token-multi",
		Scopes: []string{config.ScopeMonitoringRead, config.ScopeSettingsWrite, config.ScopeDockerReport},
	}
	attachAPITokenRecord(req, &record)
	rr := httptest.NewRecorder()

	result := ensureScope(rr, req, config.ScopeSettingsWrite)
	if !result {
		t.Fatal("expected ensureScope to return true when token has required scope among multiple")
	}
}

func TestEnsureScope_TokenMissingScopeRejects(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	record := config.APITokenRecord{ID: "token-2", Scopes: []string{config.ScopeMonitoringRead}}
	attachAPITokenRecord(req, &record)
	rr := httptest.NewRecorder()

	result := ensureScope(rr, req, config.ScopeSettingsWrite)
	if result {
		t.Fatal("expected ensureScope to return false when token missing required scope")
	}
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rr.Code)
	}
}

func TestEnsureScope_TokenWithEmptyScopesDefaultsToWildcard(t *testing.T) {
	// Note: Empty scopes defaults to wildcard access via ensureScopes()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	record := config.APITokenRecord{ID: "token-empty", Scopes: []string{}}
	attachAPITokenRecord(req, &record)
	rr := httptest.NewRecorder()

	// Empty scopes defaults to ScopeWildcard, so any scope should be allowed
	result := ensureScope(rr, req, config.ScopeDockerReport)
	if !result {
		t.Fatal("expected ensureScope to return true (empty scopes defaults to wildcard)")
	}
}

func TestEnsureScope_RejectsWithProperJSONResponse(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	record := config.APITokenRecord{ID: "token-3", Scopes: []string{"other:scope"}}
	attachAPITokenRecord(req, &record)
	rr := httptest.NewRecorder()

	result := ensureScope(rr, req, config.ScopeSettingsWrite)
	if result {
		t.Fatal("expected ensureScope to return false")
	}

	// Check response format
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", ct)
	}

	body := rr.Body.String()
	if body == "" {
		t.Fatal("expected JSON body in response")
	}
	// Body should contain error and requiredScope
	if !strings.Contains(body, "missing_scope") {
		t.Fatalf("expected 'missing_scope' in body, got: %s", body)
	}
	if !strings.Contains(body, config.ScopeSettingsWrite) {
		t.Fatalf("expected required scope in body, got: %s", body)
	}
}
