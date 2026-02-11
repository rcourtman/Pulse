package config

import (
	"testing"
	"time"
)

func TestAPITokenRecord_IsLegacyToken(t *testing.T) {
	record := &APITokenRecord{}
	if !record.IsLegacyToken() {
		t.Fatalf("expected legacy token when no org bindings")
	}

	record.OrgID = "org-1"
	if record.IsLegacyToken() {
		t.Fatalf("expected non-legacy token when OrgID is set")
	}

	record.OrgID = ""
	record.OrgIDs = []string{"org-2"}
	if record.IsLegacyToken() {
		t.Fatalf("expected non-legacy token when OrgIDs is set")
	}
}

func TestNewAPITokenRecord(t *testing.T) {
	if _, err := NewAPITokenRecord("", "name", nil); err == nil {
		t.Fatalf("expected error for empty raw token")
	}

	record, err := NewAPITokenRecord("token-abcdef123456", "name", nil)
	if err != nil {
		t.Fatalf("NewAPITokenRecord error: %v", err)
	}
	if record.Name != "name" {
		t.Fatalf("Name = %q", record.Name)
	}
	if record.Hash == "" {
		t.Fatalf("expected hash to be set")
	}
	if record.Prefix != "token-" {
		t.Fatalf("Prefix = %q", record.Prefix)
	}
	if record.Suffix != "3456" {
		t.Fatalf("Suffix = %q", record.Suffix)
	}
	if len(record.Scopes) != 1 || record.Scopes[0] != ScopeWildcard {
		t.Fatalf("Scopes = %v", record.Scopes)
	}
}

func TestNewHashedAPITokenRecord(t *testing.T) {
	if _, err := NewHashedAPITokenRecord("", "name", time.Time{}, nil); err == nil {
		t.Fatalf("expected error for empty hashed token")
	}

	record, err := NewHashedAPITokenRecord("hashed-token-1234", "name", time.Time{}, []string{ScopeSettingsRead})
	if err != nil {
		t.Fatalf("NewHashedAPITokenRecord error: %v", err)
	}
	if record.Hash != "hashed-token-1234" {
		t.Fatalf("Hash = %q", record.Hash)
	}
	if record.Prefix != "hashed" {
		t.Fatalf("Prefix = %q", record.Prefix)
	}
	if record.Suffix != "1234" {
		t.Fatalf("Suffix = %q", record.Suffix)
	}
	if len(record.Scopes) != 1 || record.Scopes[0] != ScopeSettingsRead {
		t.Fatalf("Scopes = %v", record.Scopes)
	}
	if record.CreatedAt.IsZero() {
		t.Fatalf("expected CreatedAt to be set")
	}
}

func TestAPITokenRecord_IsExpired(t *testing.T) {
	record := &APITokenRecord{}
	if record.IsExpired() {
		t.Fatalf("expected token with nil ExpiresAt to not be expired")
	}

	past := time.Now().UTC().Add(-time.Minute)
	record.ExpiresAt = &past
	if !record.IsExpired() {
		t.Fatalf("expected token to be expired when ExpiresAt is in the past")
	}

	future := time.Now().UTC().Add(time.Minute)
	record.ExpiresAt = &future
	if record.IsExpired() {
		t.Fatalf("expected token to not be expired when ExpiresAt is in the future")
	}
}

func TestConfig_APITokenExpiration_Enforced(t *testing.T) {
	rawToken := "token-abcdef123456"
	record, err := NewAPITokenRecord(rawToken, "name", nil)
	if err != nil {
		t.Fatalf("NewAPITokenRecord error: %v", err)
	}

	expiredAt := time.Now().UTC().Add(-time.Minute)
	record.ExpiresAt = &expiredAt

	cfg := &Config{
		APITokens: []APITokenRecord{*record},
	}

	if got, ok := cfg.ValidateAPIToken(rawToken); ok || got != nil {
		t.Fatalf("expected expired token to be rejected by ValidateAPIToken")
	}
	if cfg.APITokens[0].LastUsedAt != nil {
		t.Fatalf("expected LastUsedAt to remain nil when token is expired")
	}
	if cfg.IsValidAPIToken(rawToken) {
		t.Fatalf("expected expired token to be rejected by IsValidAPIToken")
	}

	// Move expiration to the future and ensure the token is accepted.
	notExpiredAt := time.Now().UTC().Add(time.Minute)
	cfg.APITokens[0].ExpiresAt = &notExpiredAt

	if got, ok := cfg.ValidateAPIToken(rawToken); !ok || got == nil {
		t.Fatalf("expected non-expired token to be accepted by ValidateAPIToken")
	}
	if cfg.APITokens[0].LastUsedAt == nil {
		t.Fatalf("expected LastUsedAt to be updated for valid token")
	}
	if !cfg.IsValidAPIToken(rawToken) {
		t.Fatalf("expected non-expired token to be accepted by IsValidAPIToken")
	}
}
