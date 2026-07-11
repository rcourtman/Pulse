package mutationregistry

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestInfrastructureAPIRoutesResolveToRegistry scans the real router sources,
// not a duplicate ID inventory. Its scope is deliberately limited to the
// infrastructure mutation namespaces below; settings/metadata CRUD is outside
// M1 and is not claimed as infrastructure enumeration.
func TestInfrastructureAPIRoutesResolveToRegistry(t *testing.T) {
	root := repoRoot(t)
	files := []string{
		"internal/api/router_routes_monitoring.go",
		"internal/api/router_routes_registration.go",
		"internal/api/router_routes_ai_relay.go",
	}
	catalog := map[string]string{
		"/api/actions/plan":                    "action.api.plan",
		"/api/actions/pending":                 "",
		"/api/actions/{id}/decision":           "action.api.decision",
		"/api/actions/{id}/execute":            "action.api.execute",
		"/api/agents/docker/report":            "",
		"/api/agents/docker/commands/":         "",
		"/api/agents/docker/runtimes/":         "docker.api.runtime-stop",
		"/api/agents/docker/containers/update": "docker.api.update-container",
		"/api/updates/check":                   "",
		"/api/updates/apply":                   "admin.pulse.self-update",
		"/api/updates/rollback":                "admin.pulse.self-update",
		"/api/updates/status":                  "",
		"/api/updates/stream":                  "",
		"/api/updates/plan":                    "",
		"/api/updates/history":                 "",
		"/api/updates/history/entry":           "",
		"/api/ai/run-command":                  "legacy.api.run-command",
		"/api/ai/remediation/plans":            "",
		"/api/ai/remediation/plan":             "",
		"/api/ai/remediation/approve":          "legacy.enterprise.remediation-approve",
		"/api/ai/remediation/execute":          "legacy.enterprise.remediation-execute",
		"/api/ai/remediation/rollback":         "legacy.enterprise.remediation-rollback",
	}
	routeRE := regexp.MustCompile(`"(?:GET |POST |PUT |DELETE )?(/api/[^"]+)"`)
	found := map[string]bool{}
	for _, relative := range files {
		data, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range routeRE.FindAllStringSubmatch(string(data), -1) {
			route := match[1]
			if !auditedInfrastructurePrefix(route) {
				continue
			}
			found[route] = true
			id, classified := catalog[route]
			if !classified {
				t.Errorf("runtime infrastructure route %q is unclassified", route)
				continue
			}
			if id != "" {
				if err := AuditRuntimeCandidates([]RuntimeCandidate{{Surface: "http:" + route, MutationID: id}}); err != nil {
					t.Error(err)
				}
			}
		}
	}
	for route := range catalog {
		if !found[route] {
			t.Errorf("classified route %q is no longer registered", route)
		}
	}
	// The shared Docker gateway contains a second mutation sub-route.
	dockerSource, _ := os.ReadFile(filepath.Join(root, "internal/api/docker_agents.go"))
	if !strings.Contains(string(dockerSource), `"/update-all"`) {
		t.Fatal("Docker update-all handler candidate not found")
	}
	if err := AuditRuntimeCandidates([]RuntimeCandidate{{Surface: "http:/api/agents/docker/runtimes/#update-all", MutationID: "docker.api.update-all"}}); err != nil {
		t.Fatal(err)
	}
}

func auditedInfrastructurePrefix(route string) bool {
	for _, prefix := range []string{"/api/actions/", "/api/agents/docker/", "/api/updates/", "/api/ai/remediation/", "/api/ai/run-command"} {
		if strings.HasPrefix(route, prefix) {
			return true
		}
	}
	return false
}

func TestTransportCommandCatalogsResolveToRegistry(t *testing.T) {
	root := repoRoot(t)
	docker := scanNamedStringConstants(t, filepath.Join(root, "internal/monitoring/docker_commands.go"), `DockerCommandType\w+`)
	dockerCatalog := map[string]string{
		"DockerCommandTypeStop":            "transport.docker.runtime-stop",
		"DockerCommandTypeUpdateContainer": "transport.docker.update-container",
		"DockerCommandTypeUpdateAll":       "transport.docker.update-all",
		"DockerCommandTypeCheckUpdates":    "",
	}
	auditConstantCatalog(t, docker, dockerCatalog, nil)

	agent := scanNamedStringConstants(t, filepath.Join(root, "internal/agentexec/types.go"), `MsgType\w+`)
	agentCatalog := map[string]string{
		"MsgTypeAgentRegister": "", "MsgTypeAgentPing": "", "MsgTypeCommandResult": "",
		"MsgTypeHostStorageCleanupResult": "", "MsgTypeHostUpdateResult": "", "MsgTypeRegistered": "", "MsgTypePong": "",
		"MsgTypeExecuteCmd": "transport.agent.raw-command", "MsgTypeHostStorageCleanup": "transport.agent.host-package-cache-cleanup",
		"MsgTypeReadFile": "", "MsgTypeHostUpdate": "transport.agent.host-package-update",
		"MsgTypeDeployPreflight": "transport.agent.deploy-preflight", "MsgTypeDeployInstall": "transport.agent.deploy-install",
		"MsgTypeDeployCancelJob": "transport.agent.deploy-cancel", "MsgTypeDeployProgress": "",
	}
	authority := map[string]string{
		"MsgTypeExecuteCmd":         "assistant.resource-action",
		"MsgTypeHostStorageCleanup": "resource.host.package-cache-cleanup",
		"MsgTypeHostUpdate":         "resource.host.package-update",
	}
	auditConstantCatalog(t, agent, agentCatalog, authority)
}

func TestPatrolJobRegistrationResolvesToRegistry(t *testing.T) {
	root := repoRoot(t)
	router, _ := os.ReadFile(filepath.Join(root, "internal/api/router.go"))
	broker, _ := os.ReadFile(filepath.Join(root, "internal/api/patrol_action_broker.go"))
	if !strings.Contains(string(router), "SetActionBrokerFactory") || !strings.Contains(string(broker), "PlanWithOptions") {
		t.Fatal("Patrol action job registration is no longer mechanically discoverable")
	}
	if err := AuditRuntimeCandidates([]RuntimeCandidate{{Surface: "job:patrol-action-broker", MutationID: "patrol.typed-action"}}); err != nil {
		t.Fatal(err)
	}
}

func scanNamedStringConstants(t *testing.T, path, namePattern string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?m)^\s*(` + namePattern + `)(?:\s+\w+)?\s*=\s*"([^"]+)"`)
	out := map[string]string{}
	for _, match := range re.FindAllStringSubmatch(string(data), -1) {
		out[match[1]] = match[2]
	}
	return out
}

func auditConstantCatalog(t *testing.T, actual, catalog, authority map[string]string) {
	t.Helper()
	for name, value := range actual {
		id, ok := catalog[name]
		if !ok {
			t.Errorf("transport command %s=%q is unclassified", name, value)
			continue
		}
		if id == "" {
			continue
		}
		candidate := RuntimeCandidate{Surface: "transport:" + value, MutationID: id, Transport: true, DurableAuthorityID: authority[name]}
		if err := AuditRuntimeCandidates([]RuntimeCandidate{candidate}); err != nil {
			t.Error(err)
		}
	}
	var stale []string
	for name := range catalog {
		if _, ok := actual[name]; !ok {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("classified transport constants no longer exist: %v", stale)
	}
}
