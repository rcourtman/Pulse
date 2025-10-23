package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAPITokenRecordHasScope(t *testing.T) {
	record := APITokenRecord{Scopes: []string{ScopeMonitoringRead}}

	if !record.HasScope(ScopeMonitoringRead) {
		t.Fatalf("expected scope %q to be granted", ScopeMonitoringRead)
	}
	if record.HasScope(ScopeSettingsWrite) {
		t.Fatalf("did not expect scope %q to be granted", ScopeSettingsWrite)
	}

	record.Scopes = nil // legacy tokens with no scopes should default to wildcard
	if !record.HasScope(ScopeSettingsWrite) {
		t.Fatalf("expected wildcard to grant %q", ScopeSettingsWrite)
	}

	if !IsKnownScope(ScopeMonitoringRead) {
		t.Fatalf("expected %q to be known scope", ScopeMonitoringRead)
	}
	if IsKnownScope("unknown:scope") {
		t.Fatalf("unexpected scope recognised")
	}
}

func TestLoadAPITokensAppliesLegacyScopes(t *testing.T) {
	if len(AllKnownScopes) == 0 {
		t.Fatal("expected known scopes to be defined")
	}

	dir := t.TempDir()
	persistence := NewConfigPersistence(dir)
	if err := persistence.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	payload := `[{"id":"legacy","name":"legacy","hash":"abc","createdAt":"2024-01-01T00:00:00Z"}]`
	if err := os.WriteFile(filepath.Join(dir, "api_tokens.json"), []byte(payload), 0600); err != nil {
		t.Fatalf("write api_tokens.json: %v", err)
	}

	tokens, err := persistence.LoadAPITokens()
	if err != nil {
		t.Fatalf("LoadAPITokens: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if len(tokens[0].Scopes) != 1 || tokens[0].Scopes[0] != ScopeWildcard {
		t.Fatalf("expected legacy token to default to wildcard scope, got %#v", tokens[0].Scopes)
	}
}
