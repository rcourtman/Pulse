package api

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestRouterRouteInventory(t *testing.T) {
	literalRoutes, dynamicRoutes := parseRouterRoutes(t)

	expectedAll := sliceToSet(t, allRouteAllowlist, "all route allowlist")
	expectedPublic := sliceToSet(t, publicRouteAllowlist, "public route allowlist")
	expectedDynamic := sliceToSet(t, dynamicRouteAllowlist, "dynamic route allowlist")

	actualAll := sliceToSet(t, literalRoutes, "router routes")
	actualDynamic := sliceToSet(t, dynamicRoutes, "router dynamic routes")

	if missing := setDifference(actualAll, expectedAll); len(missing) > 0 {
		t.Fatalf("routes missing from allowlist: %s", strings.Join(sortedKeys(missing), ", "))
	}
	if stale := setDifference(expectedAll, actualAll); len(stale) > 0 {
		t.Fatalf("allowlist contains routes not in router.go: %s", strings.Join(sortedKeys(stale), ", "))
	}

	if missing := setDifference(actualDynamic, expectedDynamic); len(missing) > 0 {
		t.Fatalf("dynamic routes missing from allowlist: %s", strings.Join(sortedKeys(missing), ", "))
	}
	if stale := setDifference(expectedDynamic, actualDynamic); len(stale) > 0 {
		t.Fatalf("dynamic allowlist contains routes not in router.go: %s", strings.Join(sortedKeys(stale), ", "))
	}

	if missing := setDifference(expectedPublic, expectedAll); len(missing) > 0 {
		t.Fatalf("public routes missing from full allowlist: %s", strings.Join(sortedKeys(missing), ", "))
	}
}

func parseRouterRoutes(t *testing.T) ([]string, []string) {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to locate test file path")
	}
	routerPath := filepath.Join(filepath.Dir(file), "router.go")
	data, err := os.ReadFile(routerPath)
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}

	re := regexp.MustCompile(`r\.mux\.Handle(?:Func)?\(([^,]+),`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	if len(matches) == 0 {
		t.Fatalf("no routes found in router.go")
	}

	var literalRoutes []string
	var dynamicRoutes []string
	for _, match := range matches {
		arg := strings.TrimSpace(match[1])
		if arg == "" {
			continue
		}
		if strings.HasPrefix(arg, "\"") || strings.HasPrefix(arg, "`") || strings.HasPrefix(arg, "'") {
			unquoted, err := strconv.Unquote(arg)
			if err != nil {
				unquoted = strings.Trim(arg, "`\"'")
			}
			literalRoutes = append(literalRoutes, unquoted)
		} else {
			dynamicRoutes = append(dynamicRoutes, arg)
		}
	}

	return literalRoutes, dynamicRoutes
}

func sliceToSet(t *testing.T, items []string, name string) map[string]struct{} {
	t.Helper()
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		if _, exists := set[item]; exists {
			t.Fatalf("duplicate entry %q in %s", item, name)
		}
		set[item] = struct{}{}
	}
	return set
}

func setDifference(a, b map[string]struct{}) map[string]struct{} {
	diff := make(map[string]struct{})
	for key := range a {
		if _, ok := b[key]; !ok {
			diff[key] = struct{}{}
		}
	}
	return diff
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

var dynamicRouteAllowlist = []string{
	"config.DefaultOIDCCallbackPath",
}

var publicRouteAllowlist = []string{
	"/api/health",
	"/api/version",
	"/api/agent/version",
	"/api/server/info",
	"/api/security/status",
	"/api/security/validate-bootstrap-token",
	"/api/security/quick-setup",
	"/api/security/recovery",
	"/api/login",
	"/api/logout",
	"/api/oidc/login",
	"/api/ai/oauth/callback",
	"/api/setup-script",
	"/api/install/install-docker.sh",
	"/api/install/install.sh",
	"/api/install/install.ps1",
	"/install-docker-agent.sh",
	"/install-container-agent.sh",
	"/download/pulse-docker-agent",
	"/install-host-agent.sh",
	"/install-host-agent.ps1",
	"/uninstall-host-agent.sh",
	"/uninstall-host-agent.ps1",
	"/download/pulse-host-agent",
	"/download/pulse-host-agent.sha256",
	"/install.sh",
	"/install.ps1",
	"/download/pulse-agent",
}

var allRouteAllowlist = []string{
	"/api/health",
	"/api/monitoring/scheduler/health",
	"/api/state",
	"/api/logs/stream",
	"/api/logs/download",
	"/api/logs/level",
	"/api/agents/docker/report",
	"/api/agents/kubernetes/report",
	"/api/agents/host/report",
	"/api/agents/host/lookup",
	"/api/agents/host/uninstall",
	"/api/agents/host/unlink",
	"/api/agents/host/link",
	"/api/agents/host/",
	"/api/agents/docker/commands/",
	"/api/agents/docker/hosts/",
	"/api/agents/docker/containers/update",
	"/api/agents/kubernetes/clusters/",
	"/api/version",
	"/api/storage/",
	"/api/storage-charts",
	"/api/charts",
	"/api/metrics-store/stats",
	"/api/metrics-store/history",
	"/api/diagnostics",
	"/api/diagnostics/docker/prepare-token",
	"/api/install/install-docker.sh",
	"/api/install/install.sh",
	"/api/install/install.ps1",
	"/api/config",
	"/api/backups",
	"/api/backups/",
	"/api/backups/unified",
	"/api/backups/pve",
	"/api/backups/pbs",
	"/api/snapshots",
	"/api/resources",
	"/api/resources/stats",
	"/api/resources/",
	"/api/guests/metadata",
	"/api/guests/metadata/",
	"/api/docker/metadata",
	"/api/docker/metadata/",
	"/api/docker/hosts/metadata",
	"/api/docker/hosts/metadata/",
	"/api/hosts/metadata",
	"/api/hosts/metadata/",
	"/api/updates/check",
	"/api/updates/apply",
	"/api/updates/status",
	"/api/updates/stream",
	"/api/updates/plan",
	"/api/updates/history",
	"/api/updates/history/entry",
	"/api/infra-updates",
	"/api/infra-updates/summary",
	"/api/infra-updates/check",
	"/api/infra-updates/host/",
	"/api/infra-updates/",
	"/api/config/nodes",
	"/api/security/validate-bootstrap-token",
	"/api/config/nodes/test-config",
	"/api/config/nodes/test-connection",
	"/api/config/nodes/",
	"/api/admin/profiles/",
	"/api/config/system",
	"/api/system/mock-mode",
	"/api/license/status",
	"/api/license/features",
	"/api/license/activate",
	"/api/license/clear",
	"GET /api/audit",
	"GET /api/audit/",
	"GET /api/audit/{id}/verify",
	"/api/admin/roles",
	"/api/admin/roles/",
	"/api/admin/users",
	"/api/admin/users/",
	"/api/admin/reports/generate",
	"/api/admin/reports/generate-multi",
	"/api/admin/webhooks/audit",
	"/api/security/change-password",
	"/api/logout",
	"/api/login",
	"/api/security/reset-lockout",
	"/api/security/oidc",
	"/api/oidc/login",
	"/api/security/tokens",
	"/api/security/tokens/",
	"/api/security/status",
	"/api/security/quick-setup",
	"/api/security/regenerate-token",
	"/api/security/validate-token",
	"/api/security/apply-restart",
	"/api/security/recovery",
	"/api/config/export",
	"/api/config/import",
	"/api/setup-script",
	"/api/setup-script-url",
	"/api/agent-install-command",
	"/api/auto-register",
	"/api/discover",
	"/api/test-notification",
	"/api/alerts/",
	"/api/notifications/",
	"/api/notifications/dlq",
	"/api/notifications/queue/stats",
	"/api/notifications/dlq/retry",
	"/api/notifications/dlq/delete",
	"/api/system/settings",
	"/api/system/settings/update",
	"/api/system/ssh-config",
	"/api/system/verify-temperature-ssh",
	"/api/settings/ai",
	"/api/settings/ai/update",
	"/api/ai/test",
	"/api/ai/test/{provider}",
	"/api/ai/models",
	"/api/ai/execute",
	"/api/ai/execute/stream",
	"/api/ai/kubernetes/analyze",
	"/api/ai/investigate-alert",
	"/api/ai/run-command",
	"/api/ai/knowledge",
	"/api/ai/knowledge/save",
	"/api/ai/knowledge/delete",
	"/api/ai/knowledge/export",
	"/api/ai/knowledge/import",
	"/api/ai/knowledge/clear",
	"/api/ai/debug/context",
	"/api/ai/agents",
	"/api/ai/cost/summary",
	"/api/ai/cost/reset",
	"/api/ai/cost/export",
	"/api/ai/oauth/start",
	"/api/ai/oauth/exchange",
	"/api/ai/oauth/callback",
	"/api/ai/oauth/disconnect",
	"/api/ai/patrol/status",
	"/api/ai/patrol/stream",
	"/api/ai/patrol/findings",
	"/api/ai/patrol/history",
	"/api/ai/patrol/run",
	"/api/ai/patrol/acknowledge",
	"/api/ai/patrol/dismiss",
	"/api/ai/patrol/findings/note",
	"/api/ai/patrol/suppress",
	"/api/ai/patrol/snooze",
	"/api/ai/patrol/resolve",
	"/api/ai/patrol/runs",
	"/api/ai/patrol/suppressions",
	"/api/ai/patrol/suppressions/",
	"/api/ai/patrol/dismissed",
	"/api/ai/patrol/autonomy",
	"/api/ai/findings/",
	"/api/ai/intelligence",
	"/api/ai/intelligence/patterns",
	"/api/ai/intelligence/predictions",
	"/api/ai/intelligence/correlations",
	"/api/ai/intelligence/changes",
	"/api/ai/intelligence/baselines",
	"/api/ai/intelligence/remediations",
	"/api/ai/intelligence/anomalies",
	"/api/ai/intelligence/learning",
	"/api/ai/unified/findings",
	"/api/ai/forecast",
	"/api/ai/forecasts/overview",
	"/api/ai/learning/preferences",
	"/api/ai/proxmox/events",
	"/api/ai/proxmox/correlations",
	"/api/ai/remediation/plans",
	"/api/ai/remediation/plan",
	"/api/ai/remediation/approve",
	"/api/ai/remediation/execute",
	"/api/ai/remediation/rollback",
	"/api/ai/circuit/status",
	"/api/ai/incidents",
	"/api/ai/incidents/",
	"/api/ai/chat/sessions",
	"/api/ai/chat/sessions/",
	"/api/ai/status",
	"/api/ai/chat",
	"/api/ai/sessions",
	"/api/ai/sessions/",
	"/api/ai/approvals",
	"/api/ai/approvals/",
	"/api/ai/question/",
	"/api/discovery",
	"/api/discovery/status",
	"/api/discovery/settings",
	"/api/discovery/info/",
	"/api/discovery/type/",
	"/api/discovery/host/",
	"/api/discovery/",
	"/api/agent/ws",
	"/install-docker-agent.sh",
	"/install-container-agent.sh",
	"/download/pulse-docker-agent",
	"/install-host-agent.sh",
	"/install-host-agent.ps1",
	"/uninstall-host-agent.sh",
	"/uninstall-host-agent.ps1",
	"/download/pulse-host-agent",
	"/download/pulse-host-agent.sha256",
	"/install.sh",
	"/install.ps1",
	"/download/pulse-agent",
	"/api/agent/version",
	"/api/server/info",
	"/ws",
	"/socket.io/",
	"/simple-stats",
}
