package mutationregistry

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type routeClassification struct {
	MutationID string
}

var infrastructureRouteCatalog = map[string]routeClassification{
	"POST /api/actions/plan":               {MutationID: "action.api.plan"},
	"GET /api/actions/pending":             {},
	"GET /api/actions":                     {},
	"GET /api/actions/{id}":                {},
	"POST /api/actions/{id}/decision":      {MutationID: "action.api.decision"},
	"POST /api/actions/{id}/execute":       {MutationID: "action.api.execute"},
	"/api/agents/docker/report":            {},
	"/api/agents/docker/commands/":         {},
	"/api/agents/docker/runtimes/":         {MutationID: "docker.api.runtime-stop"},
	"/api/agents/docker/containers/update": {MutationID: "docker.api.update-container"},
	"/api/updates/check":                   {},
	"/api/updates/apply":                   {MutationID: "admin.pulse.self-update"},
	"/api/updates/rollback":                {MutationID: "admin.pulse.self-update"},
	"/api/updates/status":                  {},
	"/api/updates/stream":                  {},
	"/api/updates/plan":                    {},
	"/api/updates/history":                 {},
	"/api/updates/history/entry":           {},
	"/api/ai/run-command":                  {MutationID: "legacy.api.run-command"},
	"/api/ai/remediation/plans":            {},
	"/api/ai/remediation/plan":             {},
	"/api/ai/remediation/approve":          {MutationID: "legacy.enterprise.remediation-approve"},
	"/api/ai/remediation/execute":          {MutationID: "legacy.enterprise.remediation-execute"},
	"/api/ai/remediation/rollback":         {MutationID: "legacy.enterprise.remediation-rollback"},
}

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
	routeRE := regexp.MustCompile(`"(?:(GET|POST|PUT|DELETE) )?(/api/[^"]+)"`)
	found := map[string]bool{}
	for _, relative := range files {
		data, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range routeRE.FindAllStringSubmatch(string(data), -1) {
			method, route := match[1], match[2]
			if !auditedInfrastructurePrefix(route) {
				continue
			}
			key := routeCatalogKey(method, route)
			found[key] = true
			classification, classified := infrastructureRouteCatalog[key]
			if !classified {
				t.Errorf("runtime infrastructure route %q is unclassified", key)
				continue
			}
			if classification.MutationID != "" {
				if err := AuditRuntimeCandidates([]RuntimeCandidate{{Surface: "http:" + key, MutationID: classification.MutationID}}); err != nil {
					t.Error(err)
				}
			}
		}
	}
	for key := range infrastructureRouteCatalog {
		if !found[key] {
			t.Errorf("classified route %q is no longer registered", key)
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

func routeCatalogKey(method, route string) string {
	if strings.TrimSpace(method) == "" {
		return route
	}
	return method + " " + route
}

func auditedInfrastructurePrefix(route string) bool {
	if route == "/api/actions" {
		return true
	}
	for _, prefix := range []string{"/api/actions/", "/api/agents/docker/", "/api/updates/", "/api/ai/remediation/", "/api/ai/run-command"} {
		if strings.HasPrefix(route, prefix) {
			return true
		}
	}
	return false
}

func TestActionRouteMethodAuthorityIsExactAndLookalikesFailClosed(t *testing.T) {
	tests := []struct {
		method, route, mutationID string
		classified                bool
	}{
		{method: "GET", route: "/api/actions/{id}", classified: true},
		{method: "POST", route: "/api/actions/{id}/execute", mutationID: "action.api.execute", classified: true},
		{method: "DELETE", route: "/api/actions/{id}"},
		{method: "POST", route: "/api/actions/{id}"},
		{method: "GET", route: "/api/actions/{id}/execute"},
		{method: "POST", route: "/api/actions/{id}/execute-copy"},
	}
	for _, tt := range tests {
		key := routeCatalogKey(tt.method, tt.route)
		classification, ok := infrastructureRouteCatalog[key]
		if ok != tt.classified {
			t.Errorf("route %q classified=%v, want %v", key, ok, tt.classified)
			continue
		}
		if ok && classification.MutationID != tt.mutationID {
			t.Errorf("route %q mutation=%q, want %q", key, classification.MutationID, tt.mutationID)
		}
	}
}

type transportClassification struct {
	Role               TransportRole
	MutationID         string
	DurableAuthorityID string
}

func TestTransportCommandCatalogsResolveToRegistry(t *testing.T) {
	root := repoRoot(t)
	docker := scanNamedStringConstants(t, filepath.Join(root, "internal/monitoring/docker_commands.go"), `DockerCommandType\w+`)
	dockerCatalog := map[string]transportClassification{
		"DockerCommandTypeStop":            {Role: TransportRoleMutationRequest, MutationID: "transport.docker.runtime-stop"},
		"DockerCommandTypeUpdateContainer": {Role: TransportRoleMutationRequest, MutationID: "transport.docker.update-container"},
		"DockerCommandTypeUpdateAll":       {Role: TransportRoleMutationRequest, MutationID: "transport.docker.update-all"},
		"DockerCommandTypeCheckUpdates":    {Role: TransportRoleProtocol},
	}
	auditConstantCatalog(t, docker, dockerCatalog)

	agent := scanNamedStringConstants(t, filepath.Join(root, "internal/agentexec/types.go"), `MsgType\w+`)
	agentCatalog := map[string]transportClassification{
		"MsgTypeAgentRegister": {Role: TransportRoleProtocol}, "MsgTypeAgentPing": {Role: TransportRoleProtocol},
		"MsgTypeCommandResult":            {Role: TransportRoleOperationResult},
		"MsgTypeHostStorageCleanupResult": {Role: TransportRoleOperationResult}, "MsgTypeHostUpdateResult": {Role: TransportRoleOperationResult},
		"MsgTypeDockerContainerLifecycleResult": {Role: TransportRoleOperationResult},
		"MsgTypeOperationQueryResult":           {Role: TransportRoleOperationReceipt},
		"MsgTypeRegistered":                     {Role: TransportRoleProtocol}, "MsgTypePong": {Role: TransportRoleProtocol},
		"MsgTypeExecuteCmd":               {Role: TransportRoleMutationRequest, MutationID: "transport.agent.raw-command", DurableAuthorityID: "assistant.resource-action"},
		"MsgTypeHostStorageCleanup":       {Role: TransportRoleMutationRequest, MutationID: "transport.agent.host-package-cache-cleanup", DurableAuthorityID: "resource.host.package-cache-cleanup"},
		"MsgTypeReadFile":                 {Role: TransportRoleProtocol},
		"MsgTypeHostUpdate":               {Role: TransportRoleMutationRequest, MutationID: "transport.agent.host-package-update", DurableAuthorityID: "resource.host.package-update"},
		"MsgTypeDockerContainerLifecycle": {Role: TransportRoleMutationRequest, MutationID: "transport.agent.docker-container-lifecycle", DurableAuthorityID: "resource.docker.container-lifecycle"},
		"MsgTypeOperationQuery":           {Role: TransportRoleOperationQuery},
		"MsgTypeDeployPreflight":          {Role: TransportRoleAdministrativeRequest, MutationID: "transport.agent.deploy-preflight"},
		"MsgTypeDeployInstall":            {Role: TransportRoleAdministrativeRequest, MutationID: "transport.agent.deploy-install"},
		"MsgTypeDeployCancelJob":          {Role: TransportRoleAdministrativeRequest, MutationID: "transport.agent.deploy-cancel"},
		"MsgTypeDeployProgress":           {Role: TransportRoleOperationResult},
	}
	auditConstantCatalog(t, agent, agentCatalog)
}

func TestNonAdmittingTransportMessagesCannotCarryDispatchAuthority(t *testing.T) {
	for _, role := range []TransportRole{TransportRoleOperationQuery, TransportRoleOperationResult, TransportRoleOperationReceipt, TransportRoleProtocol} {
		err := AuditTransportSurfaces([]TransportSurface{{
			Name: "lookalike", WireValue: "docker_container_lifecycle_result_retry", Role: role,
			MutationID: "transport.agent.docker-container-lifecycle", DurableAuthorityID: "resource.docker.container-lifecycle",
		}})
		if err == nil {
			t.Errorf("non-admitting role %q accepted dispatch authority", role)
		}
	}
}

func TestUnknownTransportLookalikeFailsClosed(t *testing.T) {
	actual := map[string]string{"MsgTypeDockerContainerLifecycle": "docker_container_lifecycle", "MsgTypeDockerContainerLifecycleRetry": "docker_container_lifecycle_retry"}
	catalog := map[string]transportClassification{
		"MsgTypeDockerContainerLifecycle": {Role: TransportRoleMutationRequest, MutationID: "transport.agent.docker-container-lifecycle", DurableAuthorityID: "resource.docker.container-lifecycle"},
	}
	if unknown := unclassifiedTransportConstants(actual, catalog); len(unknown) != 1 || unknown[0] != `MsgTypeDockerContainerLifecycleRetry="docker_container_lifecycle_retry"` {
		t.Fatalf("unknown lookalike classification = %v", unknown)
	}
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

func auditConstantCatalog(t *testing.T, actual map[string]string, catalog map[string]transportClassification) {
	t.Helper()
	var surfaces []TransportSurface
	for name, value := range actual {
		classification, ok := catalog[name]
		if !ok {
			t.Errorf("transport command %s=%q is unclassified", name, value)
			continue
		}
		surfaces = append(surfaces, TransportSurface{Name: name, WireValue: value, Role: classification.Role, MutationID: classification.MutationID, DurableAuthorityID: classification.DurableAuthorityID})
	}
	if err := AuditTransportSurfaces(surfaces); err != nil {
		t.Error(err)
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

func unclassifiedTransportConstants(actual map[string]string, catalog map[string]transportClassification) []string {
	var unknown []string
	for name, value := range actual {
		if _, ok := catalog[name]; !ok {
			unknown = append(unknown, fmt.Sprintf(`%s=%q`, name, value))
		}
	}
	sort.Strings(unknown)
	return unknown
}
