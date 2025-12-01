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

	// Empty scope always returns true
	record.Scopes = []string{ScopeMonitoringRead}
	if !record.HasScope("") {
		t.Fatalf("expected empty scope to always be granted")
	}

	// Explicit wildcard scope in list grants any scope
	record.Scopes = []string{ScopeWildcard}
	if !record.HasScope(ScopeSettingsWrite) {
		t.Fatalf("expected wildcard scope in list to grant %q", ScopeSettingsWrite)
	}
	if !record.HasScope(ScopeMonitoringRead) {
		t.Fatalf("expected wildcard scope in list to grant %q", ScopeMonitoringRead)
	}

	if !IsKnownScope(ScopeMonitoringRead) {
		t.Fatalf("expected %q to be known scope", ScopeMonitoringRead)
	}
	if IsKnownScope("unknown:scope") {
		t.Fatalf("unexpected scope recognised")
	}
}

func TestTokenPrefix(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{name: "longer than 6 chars", value: "abcdefghij", expected: "abcdef"},
		{name: "exactly 6 chars", value: "abcdef", expected: "abcdef"},
		{name: "shorter than 6 chars", value: "abc", expected: "abc"},
		{name: "empty string", value: "", expected: ""},
		{name: "single char", value: "x", expected: "x"},
		{name: "5 chars", value: "12345", expected: "12345"},
		{name: "7 chars", value: "1234567", expected: "123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenPrefix(tt.value)
			if result != tt.expected {
				t.Errorf("tokenPrefix(%q) = %q, want %q", tt.value, result, tt.expected)
			}
		})
	}
}

func TestTokenSuffix(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{name: "longer than 4 chars", value: "abcdefghij", expected: "ghij"},
		{name: "exactly 4 chars", value: "abcd", expected: "abcd"},
		{name: "shorter than 4 chars", value: "abc", expected: "abc"},
		{name: "empty string", value: "", expected: ""},
		{name: "single char", value: "x", expected: "x"},
		{name: "3 chars", value: "123", expected: "123"},
		{name: "5 chars", value: "12345", expected: "2345"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenSuffix(tt.value)
			if result != tt.expected {
				t.Errorf("tokenSuffix(%q) = %q, want %q", tt.value, result, tt.expected)
			}
		})
	}
}

func TestNormalizeScopes(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []string
		expected []string
	}{
		{
			name:     "nil returns wildcard",
			scopes:   nil,
			expected: []string{ScopeWildcard},
		},
		{
			name:     "empty returns wildcard",
			scopes:   []string{},
			expected: []string{ScopeWildcard},
		},
		{
			name:     "single scope preserved",
			scopes:   []string{ScopeMonitoringRead},
			expected: []string{ScopeMonitoringRead},
		},
		{
			name:     "multiple scopes preserved",
			scopes:   []string{ScopeMonitoringRead, ScopeSettingsWrite},
			expected: []string{ScopeMonitoringRead, ScopeSettingsWrite},
		},
		{
			name:     "wildcard alone preserved",
			scopes:   []string{ScopeWildcard},
			expected: []string{ScopeWildcard},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeScopes(tt.scopes)
			if len(result) != len(tt.expected) {
				t.Fatalf("normalizeScopes(%v) length = %d, want %d", tt.scopes, len(result), len(tt.expected))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("normalizeScopes(%v)[%d] = %q, want %q", tt.scopes, i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestNormalizeScopes_ReturnsCopy(t *testing.T) {
	original := []string{ScopeMonitoringRead, ScopeSettingsWrite}
	result := normalizeScopes(original)

	// Modify result and verify original is unchanged
	result[0] = "modified"
	if original[0] != ScopeMonitoringRead {
		t.Errorf("normalizeScopes did not return a copy; original was modified")
	}
}

func TestIsKnownScope(t *testing.T) {
	tests := []struct {
		name     string
		scope    string
		expected bool
	}{
		{name: "wildcard", scope: ScopeWildcard, expected: true},
		{name: "monitoring:read", scope: ScopeMonitoringRead, expected: true},
		{name: "monitoring:write", scope: ScopeMonitoringWrite, expected: true},
		{name: "docker:report", scope: ScopeDockerReport, expected: true},
		{name: "docker:manage", scope: ScopeDockerManage, expected: true},
		{name: "host-agent:report", scope: ScopeHostReport, expected: true},
		{name: "host-agent:manage", scope: ScopeHostManage, expected: true},
		{name: "settings:read", scope: ScopeSettingsRead, expected: true},
		{name: "settings:write", scope: ScopeSettingsWrite, expected: true},
		{name: "unknown scope", scope: "unknown:scope", expected: false},
		{name: "empty string", scope: "", expected: false},
		{name: "partial match", scope: "monitoring", expected: false},
		{name: "case sensitive", scope: "MONITORING:READ", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsKnownScope(tt.scope)
			if result != tt.expected {
				t.Errorf("IsKnownScope(%q) = %v, want %v", tt.scope, result, tt.expected)
			}
		})
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

func TestLoadAPITokens_ErrorPaths(t *testing.T) {
	t.Run("nonexistent file returns empty slice", func(t *testing.T) {
		dir := t.TempDir()
		persistence := NewConfigPersistence(dir)

		tokens, err := persistence.LoadAPITokens()
		if err != nil {
			t.Fatalf("expected no error for missing file, got: %v", err)
		}
		if len(tokens) != 0 {
			t.Fatalf("expected empty slice, got %d tokens", len(tokens))
		}
	})

	t.Run("empty file returns empty slice", func(t *testing.T) {
		dir := t.TempDir()
		persistence := NewConfigPersistence(dir)
		if err := persistence.EnsureConfigDir(); err != nil {
			t.Fatalf("EnsureConfigDir: %v", err)
		}

		// Write empty file
		if err := os.WriteFile(filepath.Join(dir, "api_tokens.json"), []byte{}, 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}

		tokens, err := persistence.LoadAPITokens()
		if err != nil {
			t.Fatalf("expected no error for empty file, got: %v", err)
		}
		if len(tokens) != 0 {
			t.Fatalf("expected empty slice, got %d tokens", len(tokens))
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		dir := t.TempDir()
		persistence := NewConfigPersistence(dir)
		if err := persistence.EnsureConfigDir(); err != nil {
			t.Fatalf("EnsureConfigDir: %v", err)
		}

		// Write invalid JSON
		if err := os.WriteFile(filepath.Join(dir, "api_tokens.json"), []byte("not valid json"), 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}

		_, err := persistence.LoadAPITokens()
		if err == nil {
			t.Fatal("expected error for invalid JSON, got nil")
		}
	})
}
