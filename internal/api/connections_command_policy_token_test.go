package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

func commandsEnabledConfig() monitoring.HostAgentConfig {
	enabled := true
	return monitoring.HostAgentConfig{CommandsEnabled: &enabled}
}

// The desired side of the /api/connections command-policy comparison must be
// the effective config after token scope and binding checks. A host whose
// TokenID resolves to no live token can never be served commands-enabled
// config, so desired must fail closed instead of fabricating a drift the
// operator cannot fix from the profile.
func TestEffectiveConnectionAgentConfigUnresolvableTokenFailsClosed(t *testing.T) {
	execToken := config.APITokenRecord{ID: "tok-exec", Scopes: []string{config.ScopeAgentExec}}
	tokenByID := map[string]*config.APITokenRecord{execToken.ID: &execToken}

	staleHost := models.Host{ID: "agent-a", Hostname: "a", TokenID: "tok-revoked"}
	cfg := effectiveConnectionAgentConfig(commandsEnabledConfig(), staleHost, tokenByID)
	if cfg.CommandsEnabled == nil || *cfg.CommandsEnabled {
		t.Fatalf("stale TokenID must force desired commands disabled, got %+v", cfg.CommandsEnabled)
	}

	untrackedHost := models.Host{ID: "agent-b", Hostname: "b"}
	cfg = effectiveConnectionAgentConfig(commandsEnabledConfig(), untrackedHost, tokenByID)
	if cfg.CommandsEnabled == nil || *cfg.CommandsEnabled {
		t.Fatalf("untracked TokenID must force desired commands disabled, got %+v", cfg.CommandsEnabled)
	}
}

func TestEffectiveConnectionAgentConfigResolvableAndAuthOptionalPaths(t *testing.T) {
	execToken := config.APITokenRecord{
		ID:     "tok-exec",
		Scopes: []string{config.ScopeAgentExec},
		Metadata: map[string]string{
			"bound_hostname": "a",
		},
	}
	tokenByID := map[string]*config.APITokenRecord{execToken.ID: &execToken}

	boundHost := models.Host{ID: "agent-a", Hostname: "a", TokenID: execToken.ID}
	cfg := effectiveConnectionAgentConfig(commandsEnabledConfig(), boundHost, tokenByID)
	if cfg.CommandsEnabled == nil || !*cfg.CommandsEnabled {
		t.Fatalf("exec-scoped bound token must keep desired commands enabled, got %+v", cfg.CommandsEnabled)
	}

	// Auth-optional installs have no API tokens at all; the config passes
	// through untouched rather than failing closed.
	cfg = effectiveConnectionAgentConfig(commandsEnabledConfig(), models.Host{ID: "agent-c", Hostname: "c"}, nil)
	if cfg.CommandsEnabled == nil || !*cfg.CommandsEnabled {
		t.Fatalf("auth-optional path must keep desired commands enabled, got %+v", cfg.CommandsEnabled)
	}
}
