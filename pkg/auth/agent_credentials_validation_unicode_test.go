package auth

import "testing"

// Supplement the unchanged before/after matrix with the review's Unicode and
// vocabulary-drift controls. TrimSpace, not ASCII trimming or alias rewriting,
// defines admission at this layer.
func TestValidateRoleScopesUnicodeCompatibility(t *testing.T) {
	roles := []string{
		"\u00a0" + RuntimeRoleMonitoringCollector + "\u0085",
		"\u2003" + RuntimeRoleActionRunner + "\u202f",
		"\u0085" + RuntimeRoleLegacyFullTrust + "\u00a0",
		"\u2003\u00a0",
		"future\"role",
		"futuré-role",
		"\ufeff" + RuntimeRoleActionRunner,
	}
	inputs := [][]string{
		nil, {},
		{"\u00a0" + ScopeAgentReport, ScopeAgentConfigRead + "\u0085"},
		{"\u2003" + ScopeAgentExec + "\u202f", ScopeAgentExec},
		{ScopeAgentReport, "agent:\u00a0config:read"},
		{ScopeAgentExec, "\ufeff" + ScopeAgentExec},
		{ScopeAgentReport, ScopeAgentConfigRead, "\u0085" + ScopeDockerReport + "\u00a0", ScopeKubernetesReport},
	}
	for _, role := range roles {
		for _, scopes := range inputs {
			if got, want := roleValidationError(ValidateRoleScopes(role, scopes)), referenceRoleValidationError(role, scopes); got != want {
				t.Fatalf("role=%q scopes=%q: error=%q, want %q", role, scopes, got, want)
			}
		}
	}
}

func TestValidateRoleScopesCollectorVocabularyStaysAligned(t *testing.T) {
	scopes := MonitoringCollectorScopes()
	if err := ValidateRoleScopes(RuntimeRoleMonitoringCollector, scopes); err != nil {
		t.Fatalf("migration's complete collector vocabulary must be admitted: %v", err)
	}
	for _, scope := range scopes {
		input := []string{ScopeAgentReport, ScopeAgentConfigRead, scope}
		if got, want := roleValidationError(ValidateRoleScopes(RuntimeRoleMonitoringCollector, input)), referenceRoleValidationError(RuntimeRoleMonitoringCollector, input); got != want {
			t.Fatalf("collector vocabulary scope %q: error=%q, want %q", scope, got, want)
		}
	}
}
