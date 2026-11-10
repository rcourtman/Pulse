package auth

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
)

// Keep the migration/diagnostic composition as an independent compatibility
// oracle: admission must not broaden just because it no longer needs their data.
func referenceRoleValidationError(role string, scopes []string) string {
	if _, _, err := CanonicalizeRoleScopes(role, scopes); err != nil {
		return err.Error()
	}
	if len(UnexpectedScopes(role, scopes)) > 0 {
		return fmt.Sprintf("%s credential has non-canonical scopes", strings.TrimSpace(role))
	}
	return ""
}

func roleValidationError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestValidateRoleScopesMatchesCanonicalAdmission(t *testing.T) {
	roles := []string{"", " \t", RuntimeRoleMonitoringCollector, " \t" + RuntimeRoleMonitoringCollector + "\n", RuntimeRoleActionRunner, " " + RuntimeRoleActionRunner + " ", RuntimeRoleLegacyFullTrust, " " + RuntimeRoleLegacyFullTrust + " ", "future-role", "MONITORING-COLLECTOR"}
	vocabulary := []string{ScopeAgentReport, ScopeAgentConfigRead, ScopeDockerReport, ScopeKubernetesReport, ScopeAgentExec, "*", "host-agent:report", "", " \t" + ScopeAgentReport + "\n", "AGENT:EXEC"}
	sequences := [][]string{nil, {}}
	var generate func([]string, int)
	generate = func(prefix []string, remaining int) {
		if remaining == 0 {
			sequences = append(sequences, slices.Clone(prefix))
			return
		}
		for _, scope := range vocabulary {
			generate(append(prefix, scope), remaining-1)
		}
	}
	for length := 1; length <= 3; length++ {
		generate(nil, length)
	}
	// Include complete collector vocabularies, duplicates, whitespace, order,
	// extras and embedded whitespace beyond the short exhaustive sequences.
	sequences = append(sequences,
		[]string{ScopeAgentReport, ScopeAgentConfigRead, ScopeDockerReport, ScopeKubernetesReport},
		[]string{ScopeKubernetesReport, ScopeDockerReport, ScopeAgentConfigRead, ScopeAgentReport, ScopeAgentReport},
		[]string{" " + ScopeAgentReport, ScopeAgentConfigRead + "\t", "\n" + ScopeDockerReport, ScopeKubernetesReport + " "},
		[]string{ScopeAgentReport, ScopeAgentConfigRead, ScopeDockerReport, ScopeKubernetesReport, ScopeSettingsWrite},
		[]string{ScopeAgentReport, ScopeAgentConfigRead, ScopeAgentExec, "agent: report"},
	)
	checks := 0
	for _, role := range roles {
		for _, scopes := range sequences {
			before := slices.Clone(scopes)
			want := referenceRoleValidationError(role, scopes)
			got := roleValidationError(ValidateRoleScopes(role, scopes))
			if got != want {
				t.Fatalf("role=%q scopes=%q: error=%q, want %q", role, scopes, got, want)
			}
			if !reflect.DeepEqual(scopes, before) {
				t.Fatalf("validation changed input for role=%q: %q -> %q", role, before, scopes)
			}
			checks++
		}
	}
	t.Logf("%d ordered role/scope/error/input comparisons", checks)
}

func TestValidateRoleScopesStableErrorsAndBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, role string
		scopes     []string
		want       string
	}{
		{"empty role", "", []string{"future:scope"}, ""},
		{"whitespace role", " \t", nil, ""},
		{"collector required report first", RuntimeRoleMonitoringCollector, []string{"*"}, "monitoring collector credential requires agent:report"},
		{"collector required config before extras", RuntimeRoleMonitoringCollector, []string{ScopeAgentReport, "*"}, "monitoring collector credential requires agent:config:read"},
		{"collector excess", RuntimeRoleMonitoringCollector, []string{ScopeAgentReport, ScopeAgentConfigRead, ScopeSettingsWrite}, "monitoring-collector credential has non-canonical scopes"},
		{"collector duplicate accepted", RuntimeRoleMonitoringCollector, []string{ScopeAgentConfigRead, ScopeAgentReport, ScopeAgentReport}, ""},
		{"collector alias not authority", RuntimeRoleMonitoringCollector, []string{"host-agent:report", ScopeAgentConfigRead}, "monitoring collector credential requires agent:report"},
		{"runner required before extras", RuntimeRoleActionRunner, []string{ScopeAgentReport}, "action runner credential requires agent:exec"},
		{"runner excess", " " + RuntimeRoleActionRunner + " ", []string{ScopeAgentExec, ScopeAgentReport}, "action-runner credential has non-canonical scopes"},
		{"runner duplicate whitespace", RuntimeRoleActionRunner, []string{" " + ScopeAgentExec + " ", ScopeAgentExec}, ""},
		{"legacy requires exec", RuntimeRoleLegacyFullTrust, nil, "legacy full-trust credential requires agent:exec"},
		{"legacy extras retained", RuntimeRoleLegacyFullTrust, []string{ScopeAgentExec, "*", "future:scope"}, ""},
		{"unknown trimmed quoted", " future-role ", nil, `unsupported agent credential role "future-role"`},
		{"case not canonicalized", "ACTION-RUNNER", []string{ScopeAgentExec}, `unsupported agent credential role "ACTION-RUNNER"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := roleValidationError(ValidateRoleScopes(tc.role, tc.scopes)); got != tc.want {
				t.Fatalf("error=%q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateRoleScopesConcurrentReadOnly(t *testing.T) {
	shared := []string{ScopeKubernetesReport, ScopeAgentReport, ScopeAgentConfigRead, ScopeDockerReport}
	before := slices.Clone(shared)
	roles := []string{RuntimeRoleMonitoringCollector, RuntimeRoleActionRunner, RuntimeRoleLegacyFullTrust, "", "future-role"}
	wants := make([]string, len(roles))
	for i, role := range roles {
		wants[i] = referenceRoleValidationError(role, shared)
	}
	var wg sync.WaitGroup
	for worker := 0; worker < 12; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for iteration := 0; iteration < 100; iteration++ {
				for i, role := range roles {
					if got := roleValidationError(ValidateRoleScopes(role, shared)); got != wants[i] {
						t.Errorf("role=%q: error=%q, want %q", role, got, wants[i])
						return
					}
				}
			}
		}()
	}
	wg.Wait()
	if !reflect.DeepEqual(shared, before) {
		t.Fatalf("concurrent validation changed scopes: %q -> %q", before, shared)
	}
}

var roleValidationBenchmarkError error

func BenchmarkValidateRoleScopes(b *testing.B) {
	for _, tc := range []struct {
		name, role string
		scopes     []string
	}{
		{"collector_minimal", RuntimeRoleMonitoringCollector, []string{ScopeAgentReport, ScopeAgentConfigRead}},
		{"collector_full", RuntimeRoleMonitoringCollector, []string{ScopeAgentReport, ScopeAgentConfigRead, ScopeDockerReport, ScopeKubernetesReport}},
		{"action_runner", RuntimeRoleActionRunner, []string{ScopeAgentExec}},
		{"legacy", RuntimeRoleLegacyFullTrust, []string{ScopeAgentExec, ScopeAgentReport}},
		{"untagged", "", []string{ScopeMonitoringRead, ScopeSettingsWrite}},
		{"collector_excess", RuntimeRoleMonitoringCollector, []string{ScopeAgentReport, ScopeAgentConfigRead, ScopeSettingsWrite}},
		{"collector_missing", RuntimeRoleMonitoringCollector, []string{ScopeAgentReport}},
		{"runner_excess", RuntimeRoleActionRunner, []string{ScopeAgentExec, ScopeAgentReport}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				roleValidationBenchmarkError = ValidateRoleScopes(tc.role, tc.scopes)
			}
		})
	}
}
