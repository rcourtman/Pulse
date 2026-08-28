package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestSecurityTokensCreateRollsBackCompleteInventoryWhenPersistenceFails(t *testing.T) {
	now := time.Now().UTC()
	tokens := []config.APITokenRecord{
		{ID: "newest", Name: "newest", Hash: "hash-newest", CreatedAt: now, Scopes: []string{config.ScopeWildcard}},
		{ID: "oldest", Name: "oldest", Hash: "hash-oldest", CreatedAt: now.Add(-time.Minute), Scopes: []string{config.ScopeWildcard}},
	}
	cfg := &config.Config{APITokens: append([]config.APITokenRecord(nil), tokens...)}
	cfg.SortAPITokens()

	stateDir := filepath.Join(t.TempDir(), "state")
	persistence := config.NewConfigPersistence(stateDir)
	if err := os.RemoveAll(stateDir); err != nil {
		t.Fatalf("remove persistence directory: %v", err)
	}
	if err := os.WriteFile(stateDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create persistence blocker: %v", err)
	}
	router := &Router{config: cfg, persistence: persistence}

	req := httptest.NewRequest(http.MethodPost, "/api/security/tokens", bytes.NewBufferString(`{"name":"must-not-survive"}`))
	rec := httptest.NewRecorder()
	router.handleCreateAPIToken(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	assertLifecycleAPITokenIDs(t, cfg.APITokens, "newest", "oldest")
	if cfg.APIToken != "hash-newest" {
		t.Fatalf("legacy primary token = %q, want rollback to %q", cfg.APIToken, "hash-newest")
	}
	for _, token := range cfg.APITokens {
		if token.Name == "must-not-survive" {
			t.Fatalf("failed creation left generated token active: %+v", token)
		}
	}
}

func TestSecurityTokensDeletePersistsOnlyRequestedRemoval(t *testing.T) {
	now := time.Now().UTC()
	tokens := []config.APITokenRecord{
		{ID: "newest", Name: "newest", Hash: "hash-newest", CreatedAt: now, Scopes: []string{config.ScopeWildcard}},
		{ID: "target", Name: "target", Hash: "hash-target", CreatedAt: now.Add(-time.Minute), Scopes: []string{config.ScopeWildcard}},
		{ID: "oldest", Name: "oldest", Hash: "hash-oldest", CreatedAt: now.Add(-2 * time.Minute), Scopes: []string{config.ScopeWildcard}},
	}
	persistence := config.NewConfigPersistence(t.TempDir())
	if err := persistence.SaveAPITokens(tokens); err != nil {
		t.Fatalf("save initial tokens: %v", err)
	}
	cfg := &config.Config{APITokens: append([]config.APITokenRecord(nil), tokens...)}
	router := &Router{config: cfg, persistence: persistence}

	req := httptest.NewRequest(http.MethodDelete, "/api/security/tokens/target", nil)
	rec := httptest.NewRecorder()
	router.handleDeleteAPIToken(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	assertLifecycleAPITokenIDs(t, cfg.APITokens, "newest", "oldest")
	persisted, err := persistence.LoadAPITokens()
	if err != nil {
		t.Fatalf("load persisted tokens: %v", err)
	}
	assertLifecycleAPITokenIDs(t, persisted, "newest", "oldest")
}

func TestSecurityTokensDeleteRollsBackWhenPersistenceFails(t *testing.T) {
	now := time.Now().UTC()
	tokens := []config.APITokenRecord{
		{ID: "keep", Name: "keep", Hash: "hash-keep", CreatedAt: now, Scopes: []string{config.ScopeWildcard}},
		{ID: "target", Name: "target", Hash: "hash-target", CreatedAt: now.Add(-time.Minute), Scopes: []string{config.ScopeWildcard}},
	}
	cfg := &config.Config{APITokens: append([]config.APITokenRecord(nil), tokens...)}
	cfg.SortAPITokens()

	stateDir := filepath.Join(t.TempDir(), "state")
	persistence := config.NewConfigPersistence(stateDir)
	if err := os.RemoveAll(stateDir); err != nil {
		t.Fatalf("remove persistence directory: %v", err)
	}
	if err := os.WriteFile(stateDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create persistence blocker: %v", err)
	}
	router := &Router{config: cfg, persistence: persistence}

	req := httptest.NewRequest(http.MethodDelete, "/api/security/tokens/target", nil)
	rec := httptest.NewRecorder()
	router.handleDeleteAPIToken(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (body=%q)", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	assertLifecycleAPITokenIDs(t, cfg.APITokens, "keep", "target")
	if cfg.APIToken != "hash-keep" {
		t.Fatalf("legacy primary token = %q, want rollback to %q", cfg.APIToken, "hash-keep")
	}
}

func assertLifecycleAPITokenIDs(t *testing.T, tokens []config.APITokenRecord, want ...string) {
	t.Helper()
	if len(tokens) != len(want) {
		t.Fatalf("token count = %d, want %d: %+v", len(tokens), len(want), tokens)
	}
	for index, id := range want {
		if tokens[index].ID != id {
			t.Fatalf("token[%d].ID = %q, want %q", index, tokens[index].ID, id)
		}
	}
}

// TestSecurityTokens_DeleteFailsAuthImmediately verifies that after deleting an API token,
// subsequent requests using that token receive 401 Unauthorized.
// Covers acceptance test checklist item 2.4: "Delete token — verify it no longer authenticates".
func TestSecurityTokens_DeleteFailsAuthImmediately(t *testing.T) {
	rawToken := "lifecycle-delete-token.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)

	// Keep a second token so the system stays in API-token mode after
	// deleting the first (otherwise HasAPITokens() returns false and
	// auth falls through to session/password or anonymous).
	keepToken := "lifecycle-keep-token.87654321"
	keepRecord := newTokenRecord(t, keepToken, []string{config.ScopeMonitoringRead}, nil)

	cfg := newTestConfigWithTokens(t, record, keepRecord)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	handler := router.Handler()

	// 1. Token works before deletion.
	req := httptest.NewRequest(http.MethodGet, "/simple-stats", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 before deletion, got %d", rec.Code)
	}

	// 2. Delete the target token from config (simulates handler delete).
	removed := cfg.RemoveAPIToken(record.ID)
	if removed == nil {
		t.Fatal("expected token to be removed from config")
	}

	// 3. Deleted token now fails with 401 via X-API-Token.
	req = httptest.NewRequest(http.MethodGet, "/simple-stats", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after deletion, got %d", rec.Code)
	}

	// 4. Deleted token also fails via Bearer.
	req = httptest.NewRequest(http.MethodGet, "/simple-stats", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for Bearer after deletion, got %d", rec.Code)
	}

	// 5. The kept token still works (proving auth itself isn't broken).
	req = httptest.NewRequest(http.MethodGet, "/simple-stats", nil)
	req.Header.Set("X-API-Token", keepToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for kept token, got %d", rec.Code)
	}
}

// TestSecurityTokens_ExpiredTokenRejectedAtHTTPLayer verifies that an API token whose
// ExpiresAt is in the past is rejected at the HTTP authentication layer
// with 401 Unauthorized.
// Covers acceptance test checklist item 2.4: "Expired token is rejected".
func TestSecurityTokens_ExpiredTokenRejectedAtHTTPLayer(t *testing.T) {
	rawToken := "lifecycle-expired-token.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)

	// Set expiration 1 second in the past so the token is already expired.
	past := time.Now().UTC().Add(-1 * time.Second)
	record.ExpiresAt = &past

	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	handler := router.Handler()

	// X-API-Token header path.
	req := httptest.NewRequest(http.MethodGet, "/simple-stats", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token via X-API-Token, got %d", rec.Code)
	}

	// Bearer token path.
	req = httptest.NewRequest(http.MethodGet, "/simple-stats", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token via Bearer, got %d", rec.Code)
	}

	// Verify LastUsedAt was NOT updated (expired tokens must not update stats).
	for _, storedRecord := range cfg.APITokens {
		if storedRecord.ID == record.ID && storedRecord.LastUsedAt != nil {
			t.Fatal("expired token should not have LastUsedAt updated in config")
		}
	}
}

// TestSecurityTokens_ValidTokenUpdatesLastUsedAt verifies that making an API call with a
// valid token updates the token's LastUsedAt timestamp in the config.
// Covers acceptance test checklist item 2.4: "lastUsedAt updates after token is used".
func TestSecurityTokens_ValidTokenUpdatesLastUsedAt(t *testing.T) {
	rawToken := "lifecycle-lastused-token.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	handler := router.Handler()

	// Confirm LastUsedAt is nil before first use.
	if cfg.APITokens[0].LastUsedAt != nil {
		t.Fatal("expected LastUsedAt to be nil before first use")
	}

	before := time.Now().UTC()

	// Make a request with the token.
	req := httptest.NewRequest(http.MethodGet, "/simple-stats", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	after := time.Now().UTC()

	// Verify LastUsedAt was set and is recent.
	if cfg.APITokens[0].LastUsedAt == nil {
		t.Fatal("expected LastUsedAt to be set after token use")
	}
	lastUsed := *cfg.APITokens[0].LastUsedAt
	if lastUsed.Before(before) || lastUsed.After(after.Add(time.Second)) {
		t.Fatalf("LastUsedAt %v not in expected range [%v, %v]", lastUsed, before, after)
	}

	// Second request should update the timestamp again.
	time.Sleep(2 * time.Millisecond) // ensure measurable time difference
	beforeSecond := time.Now().UTC()

	req = httptest.NewRequest(http.MethodGet, "/simple-stats", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on second call, got %d", rec.Code)
	}

	secondLastUsed := *cfg.APITokens[0].LastUsedAt
	if secondLastUsed.Before(beforeSecond) {
		t.Fatalf("LastUsedAt did not advance after second request: first=%v second=%v", lastUsed, secondLastUsed)
	}
}

// TestSecurityTokens_NonExpiredTokenAllowedThenExpiredRejected verifies the lifecycle
// transition: a token with a future expiration works, and once the
// expiration passes, the same token is rejected.
// Uses direct ExpiresAt mutation instead of time.Sleep to avoid slow/flaky tests.
func TestSecurityTokens_NonExpiredTokenAllowedThenExpiredRejected(t *testing.T) {
	rawToken := "lifecycle-expiry-window.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)

	// Set expiration far enough in the future that it won't expire during the test.
	future := time.Now().UTC().Add(1 * time.Hour)
	record.ExpiresAt = &future

	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	handler := router.Handler()

	// Token works while not expired.
	req := httptest.NewRequest(http.MethodGet, "/simple-stats", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 while token is valid, got %d", rec.Code)
	}

	// Move expiration into the past to simulate natural expiry.
	past := time.Now().UTC().Add(-1 * time.Second)
	cfg.APITokens[0].ExpiresAt = &past

	// Same token now rejected.
	req = httptest.NewRequest(http.MethodGet, "/simple-stats", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after expiration, got %d", rec.Code)
	}
}

// TestSecurityTokens_InvalidTokenHeaderReturns401 verifies that a completely wrong token
// value in the X-API-Token header returns 401 with the correct error message.
// Covers acceptance test checklist item 2.4: "API call with wrong token returns 401".
func TestSecurityTokens_InvalidTokenHeaderReturns401(t *testing.T) {
	rawToken := "lifecycle-valid-token.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")
	handler := router.Handler()

	// Wrong X-API-Token header.
	req := httptest.NewRequest(http.MethodGet, "/simple-stats", nil)
	req.Header.Set("X-API-Token", "completely-wrong-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong X-API-Token, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Invalid API token") {
		t.Fatalf("expected 'Invalid API token' body, got %q", rec.Body.String())
	}

	// Verify LastUsedAt NOT updated for invalid tokens.
	if cfg.APITokens[0].LastUsedAt != nil {
		t.Fatal("invalid token should not update LastUsedAt")
	}
}
