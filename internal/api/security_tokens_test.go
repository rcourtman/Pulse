package api

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestNormalizeRequestedScopesDefaultsToWildcard(t *testing.T) {
	scopes, err := normalizeRequestedScopes(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scopes) != 1 || scopes[0] != config.ScopeWildcard {
		t.Fatalf("expected wildcard scope, got %#v", scopes)
	}
}

func TestNormalizeRequestedScopesValidList(t *testing.T) {
	raw := []string{"docker:report", "docker:report", "monitoring:read"}
	scopes, err := normalizeRequestedScopes(&raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(scopes))
	}
	if scopes[0] != config.ScopeDockerReport || scopes[1] != config.ScopeMonitoringRead {
		t.Fatalf("unexpected scopes order: %#v", scopes)
	}
}

func TestNormalizeRequestedScopesRejectsMixedWildcard(t *testing.T) {
	raw := []string{"*", "docker:report"}
	if _, err := normalizeRequestedScopes(&raw); err == nil {
		t.Fatal("expected error when mixing wildcard with explicit scopes")
	}
}

func TestNormalizeRequestedScopesRejectsUnknown(t *testing.T) {
	raw := []string{"unknown"}
	if _, err := normalizeRequestedScopes(&raw); err == nil {
		t.Fatal("expected error for unknown scope")
	}
}

func TestNormalizeRequestedScopesRejectsEmpty(t *testing.T) {
	raw := []string{}
	if _, err := normalizeRequestedScopes(&raw); err == nil {
		t.Fatal("expected error for empty scopes array")
	}
}

func TestNormalizeRequestedScopesRejectsBlankScope(t *testing.T) {
	raw := []string{config.ScopeHostReport, "   ", config.ScopeSettingsRead}
	_, err := normalizeRequestedScopes(&raw)
	if err == nil {
		t.Fatal("expected error for blank scope identifier")
	}
	if !strings.Contains(err.Error(), "cannot be blank") {
		t.Errorf("expected blank scope error, got: %v", err)
	}
}

func TestNormalizeRequestedScopesWildcardOnly(t *testing.T) {
	raw := []string{config.ScopeWildcard}
	scopes, err := normalizeRequestedScopes(&raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scopes) != 1 || scopes[0] != config.ScopeWildcard {
		t.Fatalf("expected wildcard scope only, got %#v", scopes)
	}
}
