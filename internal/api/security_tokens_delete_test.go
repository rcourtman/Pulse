package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

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
	assertAPITokenIDs(t, cfg.APITokens, "newest", "oldest")
	persisted, err := persistence.LoadAPITokens()
	if err != nil {
		t.Fatalf("load persisted tokens: %v", err)
	}
	assertAPITokenIDs(t, persisted, "newest", "oldest")
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
	assertAPITokenIDs(t, cfg.APITokens, "keep", "target")
	if cfg.APIToken != "hash-keep" {
		t.Fatalf("legacy primary token = %q, want rollback to %q", cfg.APIToken, "hash-keep")
	}
}

func assertAPITokenIDs(t *testing.T, tokens []config.APITokenRecord, want ...string) {
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
