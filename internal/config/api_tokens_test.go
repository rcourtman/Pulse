package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/auth"
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
		{name: "kubernetes:report", scope: ScopeKubernetesReport, expected: true},
		{name: "kubernetes:manage", scope: ScopeKubernetesManage, expected: true},
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

func TestClone(t *testing.T) {
	t.Run("clones all fields", func(t *testing.T) {
		now := time.Now().UTC()
		original := APITokenRecord{
			ID:         "test-id",
			Name:       "test-name",
			Hash:       "test-hash",
			Prefix:     "prefix",
			Suffix:     "suffix",
			CreatedAt:  now,
			LastUsedAt: &now,
			Scopes:     []string{ScopeMonitoringRead, ScopeSettingsWrite},
		}

		clone := original.Clone()

		if clone.ID != original.ID {
			t.Errorf("ID: got %q, want %q", clone.ID, original.ID)
		}
		if clone.Name != original.Name {
			t.Errorf("Name: got %q, want %q", clone.Name, original.Name)
		}
		if clone.Hash != original.Hash {
			t.Errorf("Hash: got %q, want %q", clone.Hash, original.Hash)
		}
		if clone.Prefix != original.Prefix {
			t.Errorf("Prefix: got %q, want %q", clone.Prefix, original.Prefix)
		}
		if clone.Suffix != original.Suffix {
			t.Errorf("Suffix: got %q, want %q", clone.Suffix, original.Suffix)
		}
		if !clone.CreatedAt.Equal(original.CreatedAt) {
			t.Errorf("CreatedAt: got %v, want %v", clone.CreatedAt, original.CreatedAt)
		}
		if clone.LastUsedAt == nil {
			t.Fatal("LastUsedAt should not be nil")
		}
		if !clone.LastUsedAt.Equal(*original.LastUsedAt) {
			t.Errorf("LastUsedAt: got %v, want %v", *clone.LastUsedAt, *original.LastUsedAt)
		}
	})

	t.Run("LastUsedAt is deep copied", func(t *testing.T) {
		now := time.Now().UTC()
		original := APITokenRecord{
			ID:         "test-id",
			LastUsedAt: &now,
			Scopes:     []string{ScopeMonitoringRead},
		}

		clone := original.Clone()

		// Modify the clone's LastUsedAt
		newTime := now.Add(time.Hour)
		*clone.LastUsedAt = newTime

		// Original should be unchanged
		if !original.LastUsedAt.Equal(now) {
			t.Errorf("modifying clone affected original: got %v, want %v", *original.LastUsedAt, now)
		}
	})

	t.Run("Scopes slice is deep copied", func(t *testing.T) {
		original := APITokenRecord{
			ID:     "test-id",
			Scopes: []string{ScopeMonitoringRead, ScopeSettingsWrite},
		}

		clone := original.Clone()

		// Modify the clone's Scopes
		clone.Scopes[0] = "modified"

		// Original should be unchanged
		if original.Scopes[0] != ScopeMonitoringRead {
			t.Errorf("modifying clone scopes affected original: got %q, want %q", original.Scopes[0], ScopeMonitoringRead)
		}
	})

	t.Run("nil LastUsedAt stays nil", func(t *testing.T) {
		original := APITokenRecord{
			ID:         "test-id",
			LastUsedAt: nil,
			Scopes:     []string{ScopeMonitoringRead},
		}

		clone := original.Clone()

		if clone.LastUsedAt != nil {
			t.Errorf("LastUsedAt should be nil, got %v", clone.LastUsedAt)
		}
	})

	t.Run("empty scopes normalized to wildcard", func(t *testing.T) {
		original := APITokenRecord{
			ID:     "test-id",
			Scopes: nil,
		}

		clone := original.Clone()

		if len(clone.Scopes) != 1 || clone.Scopes[0] != ScopeWildcard {
			t.Errorf("empty scopes should normalize to wildcard, got %v", clone.Scopes)
		}
	})
}

func TestNewAPITokenRecord(t *testing.T) {
	t.Run("valid token and name", func(t *testing.T) {
		token := "my-secret-token-12345"
		name := "Test Token"
		scopes := []string{ScopeMonitoringRead}

		record, err := NewAPITokenRecord(token, name, scopes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if record.ID == "" {
			t.Error("ID should be set")
		}
		if record.Name != name {
			t.Errorf("Name: got %q, want %q", record.Name, name)
		}
		if record.Hash == "" {
			t.Error("Hash should be set")
		}
		if record.Hash == token {
			t.Error("Hash should not equal raw token")
		}
		if record.Prefix != "my-sec" {
			t.Errorf("Prefix: got %q, want %q", record.Prefix, "my-sec")
		}
		if record.Suffix != "2345" {
			t.Errorf("Suffix: got %q, want %q", record.Suffix, "2345")
		}
		if record.CreatedAt.IsZero() {
			t.Error("CreatedAt should be set")
		}
		if len(record.Scopes) != 1 || record.Scopes[0] != ScopeMonitoringRead {
			t.Errorf("Scopes: got %v, want [%s]", record.Scopes, ScopeMonitoringRead)
		}
	})

	t.Run("empty token returns ErrInvalidToken", func(t *testing.T) {
		_, err := NewAPITokenRecord("", "Test", nil)
		if err != ErrInvalidToken {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("empty scopes normalized to wildcard", func(t *testing.T) {
		record, err := NewAPITokenRecord("some-token", "Test", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(record.Scopes) != 1 || record.Scopes[0] != ScopeWildcard {
			t.Errorf("expected wildcard scope, got %v", record.Scopes)
		}
	})

	t.Run("empty slice scopes normalized to wildcard", func(t *testing.T) {
		record, err := NewAPITokenRecord("some-token", "Test", []string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(record.Scopes) != 1 || record.Scopes[0] != ScopeWildcard {
			t.Errorf("expected wildcard scope, got %v", record.Scopes)
		}
	})

	t.Run("prefix and suffix set from token", func(t *testing.T) {
		record, err := NewAPITokenRecord("abcdefghijklmnop", "Test", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if record.Prefix != "abcdef" {
			t.Errorf("Prefix: got %q, want %q", record.Prefix, "abcdef")
		}
		if record.Suffix != "mnop" {
			t.Errorf("Suffix: got %q, want %q", record.Suffix, "mnop")
		}
	})

	t.Run("short token handles prefix/suffix", func(t *testing.T) {
		record, err := NewAPITokenRecord("abc", "Test", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if record.Prefix != "abc" {
			t.Errorf("Prefix: got %q, want %q", record.Prefix, "abc")
		}
		if record.Suffix != "abc" {
			t.Errorf("Suffix: got %q, want %q", record.Suffix, "abc")
		}
	})
}

func TestNewHashedAPITokenRecord(t *testing.T) {
	t.Run("valid hash", func(t *testing.T) {
		hash := "abcdef1234567890"
		name := "Test Token"
		created := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		scopes := []string{ScopeMonitoringRead}

		record, err := NewHashedAPITokenRecord(hash, name, created, scopes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if record.ID == "" {
			t.Error("ID should be set")
		}
		if record.Name != name {
			t.Errorf("Name: got %q, want %q", record.Name, name)
		}
		if record.Hash != hash {
			t.Errorf("Hash: got %q, want %q", record.Hash, hash)
		}
		if !record.CreatedAt.Equal(created) {
			t.Errorf("CreatedAt: got %v, want %v", record.CreatedAt, created)
		}
	})

	t.Run("empty hash returns ErrInvalidToken", func(t *testing.T) {
		_, err := NewHashedAPITokenRecord("", "Test", time.Now(), nil)
		if err != ErrInvalidToken {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("zero time gets set to now", func(t *testing.T) {
		before := time.Now().UTC()
		record, err := NewHashedAPITokenRecord("somehash", "Test", time.Time{}, nil)
		after := time.Now().UTC()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if record.CreatedAt.Before(before) || record.CreatedAt.After(after) {
			t.Errorf("CreatedAt should be between %v and %v, got %v", before, after, record.CreatedAt)
		}
	})

	t.Run("scopes normalized", func(t *testing.T) {
		record, err := NewHashedAPITokenRecord("somehash", "Test", time.Now(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(record.Scopes) != 1 || record.Scopes[0] != ScopeWildcard {
			t.Errorf("expected wildcard scope, got %v", record.Scopes)
		}
	})

	t.Run("prefix and suffix set from hash", func(t *testing.T) {
		record, err := NewHashedAPITokenRecord("abcdefghijklmnop", "Test", time.Now(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if record.Prefix != "abcdef" {
			t.Errorf("Prefix: got %q, want %q", record.Prefix, "abcdef")
		}
		if record.Suffix != "mnop" {
			t.Errorf("Suffix: got %q, want %q", record.Suffix, "mnop")
		}
	})
}

func TestHasAPITokens(t *testing.T) {
	t.Run("empty slice returns false", func(t *testing.T) {
		cfg := &Config{APITokens: nil}
		if cfg.HasAPITokens() {
			t.Error("expected false for nil tokens")
		}

		cfg.APITokens = []APITokenRecord{}
		if cfg.HasAPITokens() {
			t.Error("expected false for empty tokens")
		}
	})

	t.Run("non-empty slice returns true", func(t *testing.T) {
		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "test", Hash: "hash"},
			},
		}
		if !cfg.HasAPITokens() {
			t.Error("expected true for non-empty tokens")
		}
	})
}

func TestAPITokenCount(t *testing.T) {
	t.Run("empty returns 0", func(t *testing.T) {
		cfg := &Config{APITokens: nil}
		if cfg.APITokenCount() != 0 {
			t.Errorf("expected 0, got %d", cfg.APITokenCount())
		}
	})

	t.Run("returns correct count", func(t *testing.T) {
		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "1", Hash: "hash1"},
				{ID: "2", Hash: "hash2"},
				{ID: "3", Hash: "hash3"},
			},
		}
		if cfg.APITokenCount() != 3 {
			t.Errorf("expected 3, got %d", cfg.APITokenCount())
		}
	})
}

func TestActiveAPITokenHashes(t *testing.T) {
	t.Run("empty tokens returns empty slice", func(t *testing.T) {
		cfg := &Config{APITokens: nil}
		hashes := cfg.ActiveAPITokenHashes()
		if len(hashes) != 0 {
			t.Errorf("expected empty slice, got %v", hashes)
		}
	})

	t.Run("skips records with empty hash", func(t *testing.T) {
		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "1", Hash: "hash1"},
				{ID: "2", Hash: ""},
				{ID: "3", Hash: "hash3"},
			},
		}
		hashes := cfg.ActiveAPITokenHashes()
		if len(hashes) != 2 {
			t.Fatalf("expected 2 hashes, got %d", len(hashes))
		}
		if hashes[0] != "hash1" || hashes[1] != "hash3" {
			t.Errorf("expected [hash1, hash3], got %v", hashes)
		}
	})

	t.Run("returns all valid hashes", func(t *testing.T) {
		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "1", Hash: "hash1"},
				{ID: "2", Hash: "hash2"},
			},
		}
		hashes := cfg.ActiveAPITokenHashes()
		if len(hashes) != 2 {
			t.Fatalf("expected 2 hashes, got %d", len(hashes))
		}
	})
}

func TestHasAPITokenHash(t *testing.T) {
	cfg := &Config{
		APITokens: []APITokenRecord{
			{ID: "1", Hash: "existing-hash"},
			{ID: "2", Hash: "another-hash"},
		},
	}

	t.Run("returns true when hash exists", func(t *testing.T) {
		if !cfg.HasAPITokenHash("existing-hash") {
			t.Error("expected true for existing hash")
		}
		if !cfg.HasAPITokenHash("another-hash") {
			t.Error("expected true for another existing hash")
		}
	})

	t.Run("returns false when hash doesn't exist", func(t *testing.T) {
		if cfg.HasAPITokenHash("nonexistent") {
			t.Error("expected false for nonexistent hash")
		}
	})

	t.Run("returns false for empty config", func(t *testing.T) {
		emptyCfg := &Config{}
		if emptyCfg.HasAPITokenHash("any-hash") {
			t.Error("expected false for empty config")
		}
	})
}

func TestPrimaryAPITokenHash(t *testing.T) {
	t.Run("empty tokens returns empty string", func(t *testing.T) {
		cfg := &Config{APITokens: nil}
		if cfg.PrimaryAPITokenHash() != "" {
			t.Errorf("expected empty string, got %q", cfg.PrimaryAPITokenHash())
		}
	})

	t.Run("returns first token's hash", func(t *testing.T) {
		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "1", Hash: "first-hash"},
				{ID: "2", Hash: "second-hash"},
			},
		}
		if cfg.PrimaryAPITokenHash() != "first-hash" {
			t.Errorf("expected first-hash, got %q", cfg.PrimaryAPITokenHash())
		}
	})
}

func TestPrimaryAPITokenHint(t *testing.T) {
	t.Run("empty tokens returns empty string", func(t *testing.T) {
		cfg := &Config{APITokens: nil}
		if cfg.PrimaryAPITokenHint() != "" {
			t.Errorf("expected empty string, got %q", cfg.PrimaryAPITokenHint())
		}
	})

	t.Run("returns prefix...suffix format when both exist", func(t *testing.T) {
		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "1", Prefix: "abcdef", Suffix: "wxyz"},
			},
		}
		expected := "abcdef...wxyz"
		if cfg.PrimaryAPITokenHint() != expected {
			t.Errorf("expected %q, got %q", expected, cfg.PrimaryAPITokenHint())
		}
	})

	t.Run("falls back to hash truncation when no prefix", func(t *testing.T) {
		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "1", Hash: "12345678abcdefgh", Prefix: "", Suffix: "wxyz"},
			},
		}
		expected := "1234...efgh"
		if cfg.PrimaryAPITokenHint() != expected {
			t.Errorf("expected %q, got %q", expected, cfg.PrimaryAPITokenHint())
		}
	})

	t.Run("falls back to hash truncation when no suffix", func(t *testing.T) {
		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "1", Hash: "12345678abcdefgh", Prefix: "abcdef", Suffix: ""},
			},
		}
		expected := "1234...efgh"
		if cfg.PrimaryAPITokenHint() != expected {
			t.Errorf("expected %q, got %q", expected, cfg.PrimaryAPITokenHint())
		}
	})

	t.Run("returns empty for short hash without prefix/suffix", func(t *testing.T) {
		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "1", Hash: "short", Prefix: "", Suffix: ""},
			},
		}
		if cfg.PrimaryAPITokenHint() != "" {
			t.Errorf("expected empty string for short hash, got %q", cfg.PrimaryAPITokenHint())
		}
	})
}

func TestValidateAPIToken(t *testing.T) {
	rawToken := "my-secret-api-token-123"
	hashedToken := auth.HashAPIToken(rawToken)

	t.Run("empty token returns nil, false", func(t *testing.T) {
		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "1", Hash: hashedToken, Scopes: []string{ScopeWildcard}},
			},
		}
		record, valid := cfg.ValidateAPIToken("")
		if record != nil || valid {
			t.Error("expected nil, false for empty token")
		}
	})

	t.Run("invalid token returns nil, false", func(t *testing.T) {
		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "1", Hash: hashedToken, Scopes: []string{ScopeWildcard}},
			},
		}
		record, valid := cfg.ValidateAPIToken("wrong-token")
		if record != nil || valid {
			t.Error("expected nil, false for invalid token")
		}
	})

	t.Run("valid token returns record, true and updates LastUsedAt", func(t *testing.T) {
		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "1", Hash: hashedToken, Scopes: []string{ScopeMonitoringRead}},
			},
		}

		before := time.Now().UTC()
		record, valid := cfg.ValidateAPIToken(rawToken)
		after := time.Now().UTC()

		if !valid {
			t.Fatal("expected valid=true")
		}
		if record == nil {
			t.Fatal("expected non-nil record")
		}
		if record.ID != "1" {
			t.Errorf("expected ID=1, got %q", record.ID)
		}
		if record.LastUsedAt == nil {
			t.Fatal("LastUsedAt should be set")
		}
		if record.LastUsedAt.Before(before) || record.LastUsedAt.After(after) {
			t.Errorf("LastUsedAt should be between %v and %v, got %v", before, after, *record.LastUsedAt)
		}

		// Verify the config's record was also updated
		if cfg.APITokens[0].LastUsedAt == nil {
			t.Error("config's record LastUsedAt should be updated")
		}
	})

	t.Run("validates against multiple tokens", func(t *testing.T) {
		token2 := "another-token-456"
		hash2 := auth.HashAPIToken(token2)

		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "1", Hash: hashedToken, Scopes: []string{ScopeWildcard}},
				{ID: "2", Hash: hash2, Scopes: []string{ScopeMonitoringRead}},
			},
		}

		record, valid := cfg.ValidateAPIToken(token2)
		if !valid {
			t.Fatal("expected valid=true for second token")
		}
		if record.ID != "2" {
			t.Errorf("expected ID=2, got %q", record.ID)
		}
	})

	t.Run("empty config returns nil, false", func(t *testing.T) {
		cfg := &Config{}
		record, valid := cfg.ValidateAPIToken(rawToken)
		if record != nil || valid {
			t.Error("expected nil, false for empty config")
		}
	})
}

func TestUpsertAPIToken(t *testing.T) {
	t.Run("insert new token", func(t *testing.T) {
		cfg := &Config{}
		record := APITokenRecord{
			ID:        "new-id",
			Name:      "New Token",
			Hash:      "new-hash",
			CreatedAt: time.Now().UTC(),
		}

		cfg.UpsertAPIToken(record)

		if len(cfg.APITokens) != 1 {
			t.Fatalf("expected 1 token, got %d", len(cfg.APITokens))
		}
		if cfg.APITokens[0].ID != "new-id" {
			t.Errorf("expected ID=new-id, got %q", cfg.APITokens[0].ID)
		}
		// Should have normalized scopes
		if len(cfg.APITokens[0].Scopes) != 1 || cfg.APITokens[0].Scopes[0] != ScopeWildcard {
			t.Errorf("expected wildcard scope, got %v", cfg.APITokens[0].Scopes)
		}
	})

	t.Run("update existing token by ID", func(t *testing.T) {
		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "existing", Name: "Old Name", Hash: "old-hash", CreatedAt: time.Now().UTC()},
			},
		}

		updated := APITokenRecord{
			ID:        "existing",
			Name:      "Updated Name",
			Hash:      "updated-hash",
			CreatedAt: time.Now().UTC(),
			Scopes:    []string{ScopeMonitoringRead},
		}
		cfg.UpsertAPIToken(updated)

		if len(cfg.APITokens) != 1 {
			t.Fatalf("expected 1 token after update, got %d", len(cfg.APITokens))
		}
		if cfg.APITokens[0].Name != "Updated Name" {
			t.Errorf("expected name to be updated, got %q", cfg.APITokens[0].Name)
		}
		if cfg.APITokens[0].Hash != "updated-hash" {
			t.Errorf("expected hash to be updated, got %q", cfg.APITokens[0].Hash)
		}
	})

	t.Run("sorting happens after upsert", func(t *testing.T) {
		older := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		newer := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)

		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "older", Name: "Older", Hash: "hash1", CreatedAt: older},
			},
		}

		cfg.UpsertAPIToken(APITokenRecord{
			ID:        "newer",
			Name:      "Newer",
			Hash:      "hash2",
			CreatedAt: newer,
		})

		if len(cfg.APITokens) != 2 {
			t.Fatalf("expected 2 tokens, got %d", len(cfg.APITokens))
		}
		// Newest should be first
		if cfg.APITokens[0].ID != "newer" {
			t.Errorf("expected newest token first, got %q", cfg.APITokens[0].ID)
		}
		if cfg.APITokens[1].ID != "older" {
			t.Errorf("expected older token second, got %q", cfg.APITokens[1].ID)
		}
	})
}

func TestRemoveAPIToken(t *testing.T) {
	t.Run("remove existing returns record", func(t *testing.T) {
		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "keep", Hash: "hash1"},
				{ID: "remove", Hash: "hash2"},
				{ID: "also-keep", Hash: "hash3"},
			},
		}

		removed := cfg.RemoveAPIToken("remove")
		if removed == nil {
			t.Error("expected record when removing existing token")
		}
		if removed.ID != "remove" || removed.Hash != "hash2" {
			t.Errorf("unexpected removed record: %+v", removed)
		}
		if len(cfg.APITokens) != 2 {
			t.Fatalf("expected 2 tokens remaining, got %d", len(cfg.APITokens))
		}
		// Verify the correct token was removed
		for _, token := range cfg.APITokens {
			if token.ID == "remove" {
				t.Error("removed token should not be in list")
			}
		}
	})

	t.Run("remove non-existing returns nil", func(t *testing.T) {
		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "existing", Hash: "hash1"},
			},
		}

		removed := cfg.RemoveAPIToken("nonexistent")
		if removed != nil {
			t.Error("expected nil when removing nonexistent token")
		}
		if len(cfg.APITokens) != 1 {
			t.Errorf("token count should be unchanged, got %d", len(cfg.APITokens))
		}
	})

	t.Run("remove from empty returns nil", func(t *testing.T) {
		cfg := &Config{}
		removed := cfg.RemoveAPIToken("any")
		if removed != nil {
			t.Error("expected nil when removing from empty config")
		}
	})
}

func TestEnvMigrationSuppression(t *testing.T) {
	t.Run("IsEnvMigrationSuppressed returns false for empty list", func(t *testing.T) {
		cfg := &Config{}
		if cfg.IsEnvMigrationSuppressed("any-hash") {
			t.Error("expected false for empty suppression list")
		}
	})

	t.Run("IsEnvMigrationSuppressed returns true for suppressed hash", func(t *testing.T) {
		cfg := &Config{
			SuppressedEnvMigrations: []string{"hash1", "hash2"},
		}
		if !cfg.IsEnvMigrationSuppressed("hash1") {
			t.Error("expected true for suppressed hash")
		}
		if !cfg.IsEnvMigrationSuppressed("hash2") {
			t.Error("expected true for suppressed hash")
		}
	})

	t.Run("IsEnvMigrationSuppressed returns false for non-suppressed hash", func(t *testing.T) {
		cfg := &Config{
			SuppressedEnvMigrations: []string{"hash1", "hash2"},
		}
		if cfg.IsEnvMigrationSuppressed("hash3") {
			t.Error("expected false for non-suppressed hash")
		}
	})

	t.Run("SuppressEnvMigration adds new hash", func(t *testing.T) {
		cfg := &Config{}
		cfg.SuppressEnvMigration("hash1")
		if len(cfg.SuppressedEnvMigrations) != 1 {
			t.Errorf("expected 1 suppressed hash, got %d", len(cfg.SuppressedEnvMigrations))
		}
		if !cfg.IsEnvMigrationSuppressed("hash1") {
			t.Error("hash1 should be suppressed")
		}
	})

	t.Run("SuppressEnvMigration is idempotent", func(t *testing.T) {
		cfg := &Config{}
		cfg.SuppressEnvMigration("hash1")
		cfg.SuppressEnvMigration("hash1")
		if len(cfg.SuppressedEnvMigrations) != 1 {
			t.Errorf("expected 1 suppressed hash after duplicate add, got %d", len(cfg.SuppressedEnvMigrations))
		}
	})
}

func TestSortAPITokens(t *testing.T) {
	t.Run("sorts newest first", func(t *testing.T) {
		oldest := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		middle := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
		newest := time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC)

		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "middle", Hash: "hash2", CreatedAt: middle},
				{ID: "oldest", Hash: "hash1", CreatedAt: oldest},
				{ID: "newest", Hash: "hash3", CreatedAt: newest},
			},
		}

		cfg.SortAPITokens()

		if cfg.APITokens[0].ID != "newest" {
			t.Errorf("expected newest first, got %q", cfg.APITokens[0].ID)
		}
		if cfg.APITokens[1].ID != "middle" {
			t.Errorf("expected middle second, got %q", cfg.APITokens[1].ID)
		}
		if cfg.APITokens[2].ID != "oldest" {
			t.Errorf("expected oldest last, got %q", cfg.APITokens[2].ID)
		}
	})

	t.Run("updates legacy APIToken field", func(t *testing.T) {
		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "1", Hash: "primary-hash", CreatedAt: time.Now().UTC()},
			},
		}

		cfg.SortAPITokens()

		if cfg.APIToken != "primary-hash" {
			t.Errorf("expected APIToken to be set to primary hash, got %q", cfg.APIToken)
		}
		if !cfg.APITokenEnabled {
			t.Error("expected APITokenEnabled to be true")
		}
	})

	t.Run("empty tokens clears APIToken", func(t *testing.T) {
		cfg := &Config{
			APIToken:        "old-value",
			APITokenEnabled: true,
			APITokens:       []APITokenRecord{},
		}

		cfg.SortAPITokens()

		if cfg.APIToken != "" {
			t.Errorf("expected APIToken to be cleared, got %q", cfg.APIToken)
		}
	})

	t.Run("normalizes scopes for all tokens", func(t *testing.T) {
		cfg := &Config{
			APITokens: []APITokenRecord{
				{ID: "1", Hash: "hash1", CreatedAt: time.Now().UTC(), Scopes: nil},
				{ID: "2", Hash: "hash2", CreatedAt: time.Now().UTC(), Scopes: []string{}},
			},
		}

		cfg.SortAPITokens()

		for i, token := range cfg.APITokens {
			if len(token.Scopes) != 1 || token.Scopes[0] != ScopeWildcard {
				t.Errorf("token %d: expected wildcard scope, got %v", i, token.Scopes)
			}
		}
	})
}
