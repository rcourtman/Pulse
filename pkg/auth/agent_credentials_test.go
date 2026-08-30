package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCanonicalizeMonitoringCollectorScopes(t *testing.T) {
	t.Run("minimal collector", func(t *testing.T) {
		scopes := []string{ScopeAgentReport, ScopeAgentConfigRead}
		canonical, changed, err := CanonicalizeRoleScopes(RuntimeRoleMonitoringCollector, scopes)
		require.NoError(t, err)
		require.False(t, changed)
		require.Equal(t, scopes, canonical)
	})

	t.Run("host collector", func(t *testing.T) {
		scopes := []string{ScopeAgentReport, ScopeAgentConfigRead, ScopeDockerReport, ScopeKubernetesReport}
		canonical, changed, err := CanonicalizeRoleScopes(RuntimeRoleMonitoringCollector, scopes)
		require.NoError(t, err)
		require.False(t, changed)
		require.Equal(t, scopes, canonical)
	})

	t.Run("excess authority removed", func(t *testing.T) {
		canonical, changed, err := CanonicalizeRoleScopes(RuntimeRoleMonitoringCollector, []string{
			ScopeSettingsWrite, ScopeAgentConfigRead, ScopeAgentReport, ScopeActionsExecute,
		})
		require.NoError(t, err)
		require.True(t, changed)
		require.Equal(t, []string{ScopeAgentReport, ScopeAgentConfigRead}, canonical)
	})

	t.Run("missing baseline rejected", func(t *testing.T) {
		_, _, err := CanonicalizeRoleScopes(RuntimeRoleMonitoringCollector, []string{ScopeAgentReport})
		require.ErrorContains(t, err, ScopeAgentConfigRead)
	})
}

func TestCanonicalizeActionRunnerScopes(t *testing.T) {
	canonical, changed, err := CanonicalizeRoleScopes(RuntimeRoleActionRunner, []string{ScopeAgentExec, ScopeAgentReport})
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []string{ScopeAgentExec}, canonical)
	require.Error(t, ValidateRoleScopes(RuntimeRoleActionRunner, []string{ScopeAgentExec, ScopeAgentReport}))
}

func TestValidateRoleScopesRejectsUnknownRole(t *testing.T) {
	require.Error(t, ValidateRoleScopes("future-unreviewed-role", []string{ScopeAgentReport}))
}

func TestValidateRoleScopesAcceptsAllowedScopesInAnyOrder(t *testing.T) {
	require.NoError(t, ValidateRoleScopes(RuntimeRoleMonitoringCollector, []string{
		ScopeDockerReport, ScopeAgentConfigRead, ScopeAgentReport,
	}))
}
