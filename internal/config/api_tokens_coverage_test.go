package config

import (
	"testing"
	"time"
)

func TestConfigAPITokenHelpersAndMutations(t *testing.T) {
	cfg := &Config{}
	if cfg.APITokenCount() != 0 {
		t.Fatalf("APITokenCount() = %d, want 0", cfg.APITokenCount())
	}
	if cfg.PrimaryAPITokenHash() != "" {
		t.Fatalf("PrimaryAPITokenHash() = %q, want empty", cfg.PrimaryAPITokenHash())
	}
	if cfg.PrimaryAPITokenHint() != "" {
		t.Fatalf("PrimaryAPITokenHint() = %q, want empty", cfg.PrimaryAPITokenHint())
	}

	cfg.SuppressEnvMigration("hash-a")
	cfg.SuppressEnvMigration("hash-a") // duplicate should be ignored
	if !cfg.IsEnvMigrationSuppressed("hash-a") {
		t.Fatalf("expected hash-a to be suppressed")
	}
	if cfg.IsEnvMigrationSuppressed("hash-b") {
		t.Fatalf("expected hash-b to not be suppressed")
	}
	if len(cfg.SuppressedEnvMigrations) != 1 {
		t.Fatalf("SuppressedEnvMigrations len = %d, want 1", len(cfg.SuppressedEnvMigrations))
	}

	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	cfg.UpsertAPIToken(APITokenRecord{
		ID:        "old-id",
		Hash:      "old-token-hash-1234",
		CreatedAt: base,
	})
	if cfg.APITokenCount() != 1 {
		t.Fatalf("APITokenCount() = %d, want 1", cfg.APITokenCount())
	}
	if len(cfg.APITokens[0].Scopes) != 1 || cfg.APITokens[0].Scopes[0] != ScopeWildcard {
		t.Fatalf("expected default wildcard scope after upsert, got %v", cfg.APITokens[0].Scopes)
	}

	cfg.UpsertAPIToken(APITokenRecord{
		ID:        "new-id",
		Hash:      "new-token-hash-5678",
		Prefix:    "newtok",
		Suffix:    "5678",
		CreatedAt: base.Add(time.Hour),
	})
	if cfg.APITokenCount() != 2 {
		t.Fatalf("APITokenCount() = %d, want 2", cfg.APITokenCount())
	}
	if cfg.PrimaryAPITokenHash() != "new-token-hash-5678" {
		t.Fatalf("PrimaryAPITokenHash() = %q", cfg.PrimaryAPITokenHash())
	}
	if cfg.PrimaryAPITokenHint() != "newtok...5678" {
		t.Fatalf("PrimaryAPITokenHint() = %q", cfg.PrimaryAPITokenHint())
	}

	// Replace existing ID to hit upsert-replace branch and fallback hash-based hint logic.
	cfg.UpsertAPIToken(APITokenRecord{
		ID:        "new-id",
		Hash:      "replacehashABCDEFGH",
		CreatedAt: base.Add(2 * time.Hour),
		Scopes:    []string{ScopeSettingsRead},
	})
	if cfg.APITokenCount() != 2 {
		t.Fatalf("APITokenCount() = %d, want 2 after replace", cfg.APITokenCount())
	}
	if cfg.PrimaryAPITokenHash() != "replacehashABCDEFGH" {
		t.Fatalf("PrimaryAPITokenHash() = %q", cfg.PrimaryAPITokenHash())
	}
	if cfg.PrimaryAPITokenHint() != "repl...EFGH" {
		t.Fatalf("PrimaryAPITokenHint() = %q", cfg.PrimaryAPITokenHint())
	}
	if len(cfg.APITokens[0].Scopes) != 1 || cfg.APITokens[0].Scopes[0] != ScopeSettingsRead {
		t.Fatalf("unexpected scopes after replace: %v", cfg.APITokens[0].Scopes)
	}
	cfg.APITokens[0].Hash = "short"
	if cfg.PrimaryAPITokenHint() != "" {
		t.Fatalf("PrimaryAPITokenHint() with short hash = %q, want empty", cfg.PrimaryAPITokenHint())
	}

	removed := cfg.RemoveAPIToken("new-id")
	if removed == nil || removed.ID != "new-id" {
		t.Fatalf("expected RemoveAPIToken to remove new-id, got %#v", removed)
	}
	if cfg.RemoveAPIToken("does-not-exist") != nil {
		t.Fatalf("expected RemoveAPIToken on unknown id to return nil")
	}
}

func TestAPITokenScopesHelpers(t *testing.T) {
	record := &APITokenRecord{}
	if !record.HasScope("") {
		t.Fatalf("empty requested scope should be allowed")
	}
	if !record.HasScope(ScopeSettingsRead) {
		t.Fatalf("default wildcard scopes should allow non-empty scopes")
	}

	record = &APITokenRecord{Scopes: []string{ScopeMonitoringRead}}
	if !record.HasScope(ScopeMonitoringRead) {
		t.Fatalf("expected direct scope match to be allowed")
	}
	if record.HasScope(ScopeSettingsWrite) {
		t.Fatalf("unexpected scope allowed")
	}

	if !IsKnownScope(ScopeWildcard) {
		t.Fatalf("wildcard should be recognized as known scope")
	}
	if !IsKnownScope(ScopeMonitoringWrite) {
		t.Fatalf("known scope should be recognized")
	}
	if IsKnownScope("not:a:scope") {
		t.Fatalf("unknown scope should not be recognized")
	}
}
