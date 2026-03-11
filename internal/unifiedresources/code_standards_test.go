package unifiedresources

// Unified Resources Architecture — Code Standards Enforcement
//
// END STATE (State Read Consolidation — SRC):
//
// The Unified Resources Registry is the PRIMARY read surface for all
// internal business logic. StateSnapshot is the write/ingest buffer and
// frontend wire DTO only.
//
// Architecture:
//
//   1. StateSnapshot (models.StateSnapshot)
//      WRITE-ONLY ingest buffer. Monitoring/polling populates typed arrays.
//      The registry reads from it via IngestSnapshot(). Frontend reads via
//      ToFrontend()/WebSocket. Internal business logic MUST NOT read from
//      StateSnapshot directly — use the ReadState interface instead.
//
//   2. Unified Resources Registry (unifiedresources.ResourceRegistry)
//      The canonical read model. Provides cross-source identity resolution,
//      deduplication, typed views, and a normalized resource model.
//      Implements the ReadState interface with typed accessor methods
//      (VMs(), Containers(), Nodes(), etc.) backed by cached per-type
//      indexes that are O(1) to read and invalidated per ingest cycle.
//
// Consumer package policy:
//
//   - internal/ai/*, internal/api/*, internal/infradiscovery/,
//     internal/servicediscovery/: MUST use ReadState. Direct state reads
//     are banned for all migrated resource types.
//
//   - internal/monitoring/, internal/mock/, internal/models/,
//     internal/websocket/: Exempt (producer/wire-format packages).
//     Monitoring remains a producer package, but snapshot-shaped export
//     helpers are progressively derived from ReadState-backed canonical data
//     rather than treated as state-owned truth. Workload export helpers
//     (VMsSnapshot/ContainersSnapshot) now also derive from ReadState-backed
//     canonical data instead of from StateSnapshot-owned guest arrays. PBS
//     instance export helpers now follow the same rule via
//     ReadState.PBSInstances(). Backup-alert guest lookup assembly now also
//     derives VM/container identity from ReadState workload views instead of
//     from snapshot-owned guest arrays. Physical-disk refresh/merge paths now
//     also derive disk, node, and linked host context from ReadState instead
//     of from snapshot-owned physical-disk arrays.
//
//   - All state.* field access patterns and GetState() calls are
//     enforced as hard bans (SRC-04b). Migration is complete — zero
//     direct state access remains in consumer packages.
//
// See: docs/architecture/state-read-consolidation-plan-2026-02.md
// Progress: docs/architecture/state-read-consolidation-progress-2026-02.md
//
// The tests below enforce these rules by scanning consumer packages for
// banned direct-state access patterns.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// readConsumerGoFiles returns the contents of all non-test .go files in the
// specified directory (relative to the repo internal/ root).
func readConsumerGoFiles(t *testing.T, relDir string) map[string]string {
	t.Helper()

	// Walk up from unifiedresources/ to internal/
	internalDir := filepath.Join("..", relDir)

	files := make(map[string]string)
	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("failed to read %s: %v", path, readErr)
		}
		files[path] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk %s: %v", relDir, err)
	}
	return files
}

// bannedPattern defines a state access pattern that should not appear in
// consumer code because the resource type has been migrated to the registry.
type bannedPattern struct {
	re      *regexp.Regexp
	message string
}

var migratedResourcePatterns = []bannedPattern{
	{
		re:      regexp.MustCompile(`state\.PhysicalDisks\b`),
		message: "use unified resources registry (GetByType/ListByType with ResourceTypePhysicalDisk) instead of state.PhysicalDisks",
	},
	{
		re:      regexp.MustCompile(`state\.CephClusters\b`),
		message: "use unified resources registry (GetByType/ListByType with ResourceTypeCeph) instead of state.CephClusters",
	},
	{
		re:      regexp.MustCompile(`GetCephClusters\(\)`),
		message: "GetCephClusters() was removed — use unified resources registry instead",
	},
	{
		re:      regexp.MustCompile(`StorageProvider\b`),
		message: "StorageProvider was removed — storage pools are accessed via unified resources registry",
	},

	// SRC-04b hard bans: state.* field access patterns and GetState() calls.
	// Converted from ratchet ceilings (all reached 0) to hard bans on 2026-03-01.
	// Consumer packages must use ReadState typed accessors exclusively.
	{
		re:      regexp.MustCompile(`state\.VMs\b`),
		message: "use ReadState.VMs() instead of state.VMs — direct state access banned (SRC-04b)",
	},
	{
		re:      regexp.MustCompile(`state\.Containers\b`),
		message: "use ReadState.Containers() instead of state.Containers — direct state access banned (SRC-04b)",
	},
	{
		re:      regexp.MustCompile(`state\.Nodes\b`),
		message: "use ReadState.Nodes() instead of state.Nodes — direct state access banned (SRC-04b)",
	},
	{
		re:      regexp.MustCompile(`state\.DockerHosts\b`),
		message: "use ReadState.DockerHosts() instead of state.DockerHosts — direct state access banned (SRC-04b)",
	},
	{
		re:      regexp.MustCompile(`state\.Hosts\b`),
		message: "use ReadState.Hosts() instead of state.Hosts — direct state access banned (SRC-04b)",
	},
	{
		re:      regexp.MustCompile(`state\.Storage\b`),
		message: "use ReadState.StoragePools() instead of state.Storage — direct state access banned (SRC-04b)",
	},
	{
		re:      regexp.MustCompile(`state\.KubernetesClusters\b`),
		message: "use ReadState.K8sClusters() instead of state.KubernetesClusters — direct state access banned (SRC-04b)",
	},
	{
		re:      regexp.MustCompile(`state\.PBSInstances\b`),
		message: "use ReadState.PBSInstances() instead of state.PBSInstances — direct state access banned (SRC-04b)",
	},
	{
		re:      regexp.MustCompile(`state\.PMGInstances\b`),
		message: "use ReadState.PMGInstances() instead of state.PMGInstances — direct state access banned (SRC-04b)",
	},
	{
		re:      regexp.MustCompile(`\.GetState\(\)`),
		message: "use ReadState interface instead of GetState() — direct state access banned (SRC-04b)",
	},
}

// consumerPackage defines a package directory to scan and any files that are
// exempt from the banned patterns (e.g., adapters that bridge between layers).
type consumerPackage struct {
	dir         string
	exemptFiles map[string]bool
}

var consumerPackages = []consumerPackage{
	{dir: "ai/tools", exemptFiles: nil},
	{dir: "ai/chat", exemptFiles: nil},
	{dir: "ai", exemptFiles: nil},
	{dir: "api", exemptFiles: nil},
	{dir: "servicediscovery", exemptFiles: nil},
}

// TestNoDirectStateAccessForMigratedResources ensures that consumer packages
// do not directly access state.* fields, call GetState(), or use removed
// provider interfaces. All resource types have been migrated to the unified
// resources registry and ReadState interface (SRC-04b).
func TestNoDirectStateAccessForMigratedResources(t *testing.T) {
	// Collect all consumer file contents, deduplicating across overlapping
	// package entries (e.g., "ai" walks into "ai/tools" and "ai/chat").
	// Track exempt files so per-package exemptions are preserved.
	allFiles := make(map[string]string)
	exemptFiles := make(map[string]bool)
	for _, pkg := range consumerPackages {
		for path, content := range readConsumerGoFiles(t, pkg.dir) {
			allFiles[path] = content
			if pkg.exemptFiles[filepath.Base(path)] {
				exemptFiles[path] = true
			}
		}
	}

	for path, content := range allFiles {
		if exemptFiles[path] {
			continue
		}
		for _, bp := range migratedResourcePatterns {
			if matches := bp.re.FindAllStringIndex(content, -1); len(matches) > 0 {
				for _, m := range matches {
					line := 1 + strings.Count(content[:m[0]], "\n")
					t.Errorf("%s:%d: %s", path, line, bp.message)
				}
			}
		}
	}
}

// TestNoLegacyHostResourceTypeSymbol prevents reintroducing the removed
// ResourceTypeHost symbol. v6 code must use ResourceTypeAgent and
// CanonicalResourceType() for legacy normalization.
func TestNoLegacyHostResourceTypeSymbol(t *testing.T) {
	internalDir := filepath.Join("..")
	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		content := string(data)
		if !strings.Contains(content, "ResourceTypeHost") {
			return nil
		}

		normalizedPath := filepath.ToSlash(path)
		t.Errorf("%s: legacy ResourceTypeHost symbol detected; use ResourceTypeAgent instead", normalizedPath)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to scan internal packages: %v", err)
	}
}

// TestNoLegacyMigrationHintsInRuntimeCode prevents reintroducing runtime
// messages that point removed aliases at the wrong token guidance.
func TestNoLegacyMigrationHintsInRuntimeCode(t *testing.T) {
	bannedPhrases := []string{
		`no longer supported; use "agent"`,
		`no longer supported; use "agent:*"`,
		`app_container is no longer supported; use container`,
	}

	internalDir := filepath.Join("..")
	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		content := string(data)
		normalizedPath := filepath.ToSlash(path)
		for _, phrase := range bannedPhrases {
			if !strings.Contains(content, phrase) {
				continue
			}
			t.Errorf("%s: banned legacy migration hint detected: %q", normalizedPath, phrase)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to scan internal packages: %v", err)
	}
}

// TestV6AgentRegistrationArtifactsStayCanonical prevents the release-facing
// agent registration journey and eval instructions from drifting back to
// legacy /api/state.hosts or legacy agent.type="host" assumptions.
func TestV6AgentRegistrationArtifactsStayCanonical(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	integrationRoots := []string{
		filepath.Join(repoRoot, "tests", "integration", "tests"),
		filepath.Join(repoRoot, "tests", "integration", "evals"),
	}
	globalBannedPatterns := []*regexp.Regexp{
		regexp.MustCompile(`state\.hosts\b`),
		regexp.MustCompile(`hosts array`),
		regexp.MustCompile(`agent\.type\s*=\s*"host"`),
		regexp.MustCompile(`type:\s*'host'`),
		regexp.MustCompile(`"type"\s*:\s*"host"`),
		regexp.MustCompile(`resourceType"\s*:\s*"host"`),
		regexp.MustCompile(`resourceType:\s*'host'`),
		regexp.MustCompile(`/api/resources\?type=host`),
	}

	for _, root := range integrationRoots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			switch filepath.Ext(path) {
			case ".ts", ".tsx", ".md":
			default:
				return nil
			}

			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			content := string(data)
			normalizedPath := filepath.ToSlash(path)

			for _, pattern := range globalBannedPatterns {
				matches := pattern.FindAllStringIndex(content, -1)
				for _, match := range matches {
					line := 1 + strings.Count(content[:match[0]], "\n")
					t.Errorf("%s:%d: banned legacy integration/eval pattern %q", normalizedPath, line, pattern.String())
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("failed to scan %s: %v", root, err)
		}
	}

	artifacts := []struct {
		path             string
		requiredSnippets []string
		bannedPatterns   []*regexp.Regexp
	}{
		{
			path: filepath.Join(
				repoRoot,
				"tests",
				"integration",
				"tests",
				"journeys",
				"04-agent-install-registration.spec.ts",
			),
			requiredSnippets: []string{
				"state.resources",
				"type: 'unified'",
			},
			bannedPatterns: []*regexp.Regexp{
				regexp.MustCompile(`state\.hosts\b`),
				regexp.MustCompile(`hosts array`),
				regexp.MustCompile(`type:\s*'host'`),
				regexp.MustCompile(`"type"\s*:\s*"host"`),
			},
		},
		{
			path: filepath.Join(
				repoRoot,
				"tests",
				"integration",
				"evals",
				"tasks",
				"agent-registration.md",
			),
			requiredSnippets: []string{
				"resources[]",
				`agent.type = "unified"`,
			},
			bannedPatterns: []*regexp.Regexp{
				regexp.MustCompile("`hosts` array"),
				regexp.MustCompile(`agent\.type\s*=\s*"host"`),
			},
		},
	}

	for _, artifact := range artifacts {
		data, err := os.ReadFile(artifact.path)
		if err != nil {
			t.Fatalf("failed to read %s: %v", artifact.path, err)
		}
		content := string(data)
		normalizedPath := filepath.ToSlash(artifact.path)

		for _, snippet := range artifact.requiredSnippets {
			if !strings.Contains(content, snippet) {
				t.Errorf("%s: missing required canonical v6 snippet %q", normalizedPath, snippet)
			}
		}

		for _, pattern := range artifact.bannedPatterns {
			matches := pattern.FindAllStringIndex(content, -1)
			for _, match := range matches {
				line := 1 + strings.Count(content[:match[0]], "\n")
				t.Errorf(
					"%s:%d: banned legacy agent registration artifact pattern %q",
					normalizedPath,
					line,
					pattern.String(),
				)
			}
		}
	}
}

// TestV6AIEvalPromptsStayCanonical prevents internal AI eval scenarios from
// teaching legacy pulse_query list types after the v6 canonicalization.
func TestV6AIEvalPromptsStayCanonical(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	path := filepath.Join(repoRoot, "internal", "ai", "eval", "scenarios.go")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	content := string(data)
	normalizedPath := filepath.ToSlash(path)

	requiredSnippets := []string{
		`type=system-containers`,
		`type=app-containers`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(content, snippet) {
			t.Errorf("%s: missing required canonical AI eval snippet %q", normalizedPath, snippet)
		}
	}

	bannedSnippets := []string{
		`type=containers`,
		`type=docker`,
	}
	for _, snippet := range bannedSnippets {
		if !strings.Contains(content, snippet) {
			continue
		}
		t.Errorf("%s: banned legacy AI eval snippet %q", normalizedPath, snippet)
	}
}

// TestV6AlertConfigAliasesStayStripped keeps alert-config compatibility tests
// pinned on dropping removed legacy resource-type aliases and host-era keys.
func TestV6AlertConfigAliasesStayStripped(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	path := filepath.Join(repoRoot, "internal", "alerts", "config_aliases_test.go")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	content := string(data)
	normalizedPath := filepath.ToSlash(path)

	requiredSnippets := []string{
		`TestAlertConfigUnmarshal_LegacyHostAliasesIgnored`,
		`expected legacy timeThresholds.host to be removed`,
		`expected legacy timeThresholds.docker to be removed`,
		`expected legacy timeThresholds.k8s to be removed`,
		`expected legacy metricTimeThresholds.host to be removed`,
		`expected legacy metricTimeThresholds.dockerhost to be removed`,
		`expected legacy metricTimeThresholds.kubernetes-cluster to be removed`,
		`TestAlertConfigUnmarshal_CanonicalKeysTakePrecedence`,
		`did not expect legacy metricTimeThresholds.docker to remain`,
		`TestAlertConfigMarshal_UsesCanonicalAgentKeys`,
		`did not expect legacy hostDefaults in output`,
		`did not expect legacy disableAllHosts in output`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(content, snippet) {
			t.Errorf("%s: missing required alert-config alias stripping snippet %q", normalizedPath, snippet)
		}
	}
}

// TestV6BroadLegacyAliasCoverage keeps the broader removed alias set pinned in
// central API, AI, and alerts tests so coverage does not regress back to host-only.
func TestV6BroadLegacyAliasCoverage(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	artifacts := []struct {
		path             string
		requiredSnippets []string
	}{
		{
			path: filepath.Join(repoRoot, "internal", "api", "ai_handlers_test.go"),
			requiredSnippets: []string{
				`legacy guest rejected`,
				`legacy docker rejected`,
				`legacy container rejected`,
				`legacy k8s alias rejected`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "alert_adapter_test.go"),
			requiredSnippets: []string{
				`vm qemu rejected`,
				`system container lxc rejected`,
				`legacy system_container alias rejected`,
				`legacy docker_container alias rejected`,
				`legacy docker_service alias rejected`,
				`legacy kubernetes_cluster alias rejected`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "patrol_run_test.go"),
			requiredSnippets: []string{
				`expected legacy 'qemu' alias to be rejected`,
				`expected legacy 'container' alias to be rejected`,
				`expected legacy 'system_container' alias to be rejected`,
				`expected legacy 'docker_container' alias to be rejected`,
				`expected legacy 'kubernetes_cluster' alias to be rejected`,
				`expected legacy 'app_container' alias to be rejected`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "tools", "tools_metrics_alerts_test.go"),
			requiredSnippets: []string{
				`expected error for legacy system_container resource_type`,
				`expected error for legacy container resource_type`,
				`expected error for legacy app_container resource_type`,
				`expected error for legacy docker resource_type`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "alerts", "utility_test.go"),
			requiredSnippets: []string{
				`legacy host alias type key is dropped`,
				`legacy docker alias type key is dropped`,
			},
		},
	}

	for _, artifact := range artifacts {
		data, err := os.ReadFile(artifact.path)
		if err != nil {
			t.Fatalf("failed to read %s: %v", artifact.path, err)
		}
		content := string(data)
		normalizedPath := filepath.ToSlash(artifact.path)

		for _, snippet := range artifact.requiredSnippets {
			if !strings.Contains(content, snippet) {
				t.Errorf("%s: missing required broad legacy-alias coverage snippet %q", normalizedPath, snippet)
			}
		}
	}
}

// TestV6ReleaseFacingAPITestsCoverLegacyHostRejection keeps release-facing API
// contract tests pinned on strict v6 behavior for removed host aliases.
func TestV6ReleaseFacingAPITestsCoverLegacyHostRejection(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	artifacts := []struct {
		path             string
		requiredSnippets []string
	}{
		{
			path: filepath.Join(repoRoot, "internal", "api", "ai_handler_test.go"),
			requiredSnippets: []string{
				`canonicalizeChatMentionType("host")`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "api", "ai_handlers_test.go"),
			requiredSnippets: []string{
				`"target_type":"host"`,
				`unsupported resource_type "host"`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "api", "org_handlers_test.go"),
			requiredSnippets: []string{
				`"resourceType":"host"`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "api", "resources_test.go"),
			requiredSnippets: []string{
				`/api/resources?type=host`,
				`unsupported type filter token(s): host`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "api", "discovery_handlers_info_test.go"),
			requiredSnippets: []string{
				`/api/discovery/info/host`,
				`unsupported resource type "host"`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "api", "discovery_handlers_test.go"),
			requiredSnippets: []string{
				`/api/discovery/type/host`,
				`/api/discovery/host/host-1/host-1`,
				`unsupported resource type "host"`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "api", "docker_agents_routes_more_test.go"),
			requiredSnippets: []string{
				`legacy hosts alias status = %d, want 400`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "api", "reporting_handlers_test.go"),
			requiredSnippets: []string{
				`/api/reporting?format=pdf&resourceType=host&resourceId=h-1`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "api", "router_version_tenant_metrics_test.go"),
			requiredSnippets: []string{
				`/api/metrics-store/history?resourceType=host&resourceId=agent-1&metric=cpu&range=1h`,
				`unsupported resourceType "host"`,
			},
		},
	}

	for _, artifact := range artifacts {
		data, err := os.ReadFile(artifact.path)
		if err != nil {
			t.Fatalf("failed to read %s: %v", artifact.path, err)
		}
		content := string(data)
		normalizedPath := filepath.ToSlash(artifact.path)

		for _, snippet := range artifact.requiredSnippets {
			if !strings.Contains(content, snippet) {
				t.Errorf("%s: missing required legacy-host rejection snippet %q", normalizedPath, snippet)
			}
		}
	}
}

// TestV6DirectHostAliasValidatorCoverage keeps direct validator-level tests in
// place for the highest-risk host-alias rejection paths.
func TestV6DirectHostAliasValidatorCoverage(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	artifacts := []struct {
		path             string
		requiredSnippets []string
	}{
		{
			path: filepath.Join(repoRoot, "internal", "api", "ai_handlers_test.go"),
			requiredSnippets: []string{
				`TestNormalizeAndValidateAIExecuteTargetType_StrictCanonicalV6`,
				`legacy host rejected", in: "host"`,
				`TestNormalizeInvestigateAlertTargetType_StrictCanonicalV6`,
				`host target type rejected`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "api", "org_handlers_test.go"),
			requiredSnippets: []string{
				`TestIsUnsupportedOrganizationShareResourceType`,
				`host unsupported`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "api", "discovery_handlers_test.go"),
			requiredSnippets: []string{
				`TestParseDiscoveryResourceType_RejectsLegacyHostAlias`,
				`legacy host rejected`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "resource_type_legacy_test.go"),
			requiredSnippets: []string{
				`TestIsUnsupportedLegacyAIResourceTypeToken`,
				`legacy host rejected`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "tools", "tools_patrol_test.go"),
			requiredSnippets: []string{
				`TestHandlePatrolReportFinding_RejectsLegacyResourceTypeAliases`,
				`TestHandlePatrolReportFinding_AcceptsPhysicalDiskResourceType`,
				`"resource_type"] = "physical_disk"`,
				`[]string{"host", "container", "docker"`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "tools", "tools_discovery_test.go"),
			requiredSnippets: []string{
				`TestIsUnsupportedDiscoveryLegacyResourceTypeToken`,
				`"host", "lxc"`,
				`TestExecuteListDiscoveries_RejectsLegacyTypeAlias`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "tools", "tools_query_test.go"),
			requiredSnippets: []string{
				`TestExecuteGetTopology_RejectsLegacyDockerIncludeAlias`,
				`expected error for legacy include alias`,
				`invalid include`,
				`"type":  "host"`,
				`invalid type: host`,
				`"resource_type": "host"`,
				`invalid resource_type: host`,
				`[]string{"lxc", "host"}`,
				`TestExecuteGetGuestConfig_RejectsLegacyResourceTypes`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "alert_adapter_test.go"),
			requiredSnippets: []string{
				`with_metadata_host_legacy_ignored`,
				`agent host alias rejected`,
				`input: "host"`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "findings_resource_type_test.go"),
			requiredSnippets: []string{
				`{in: "host", want: ""}`,
				`TestNormalizeFindingResourceTypes_RejectsLegacyAndInfers`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "patrol_triggers_test.go"),
			requiredSnippets: []string{
				`AnomalyDetectedPatrolScope("res-host", "host", "cpu", 95, 50)`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "patrol_run_test.go"),
			requiredSnippets: []string{
				`expected legacy 'host' alias to be rejected`,
				`expected non-canonical 'docker' alias to be rejected`,
				`expected non-canonical 'agent_raid' alias to be rejected`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "patrol_triage_test.go"),
			requiredSnippets: []string{
				`triageResourceType("host", "qemu/100")`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "adapters", "adapters_additional_test.go"),
			requiredSnippets: []string{
				`expected unsupported host resource ID to be rejected`,
				`expected unsupported host query alias to be rejected`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "knowledge", "store_extended_test.go"),
			requiredSnippets: []string{
				`expected unsupported host guest ID to be rejected`,
				`expected unsupported host guest ID query to be rejected`,
				`expected unsupported host guest type to be rejected`,
				`expected unsupported host file to remain unchanged`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "approval", "store_test.go"),
			requiredSnippets: []string{
				`expected unsupported host target type to be rejected`,
				`expected error for unsupported host target type input`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "metadata_provider_test.go"),
			requiredSnippets: []string{
				`[]string{"host", "guest", "docker", "container", "lxc", "qemu", "docker_container", "docker_service"}`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "incident_coordinator_additional_test.go"),
			requiredSnippets: []string{
				`TestIncidentCoordinator_OnAnomalyDetected_CanonicalizesLegacyHostAlias`,
				`expected anomaly recording resource type to be canonicalized to agent`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "tools", "tools_metrics_alerts_test.go"),
			requiredSnippets: []string{
				`expected error for legacy host resource_type`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "tools", "tools_read_test.go"),
			requiredSnippets: []string{
				`TestPulseToolExecutor_ExecuteReadRejectsLegacyAppContainerArg`,
				`app_container is no longer supported; use app-container`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "tools", "tools_file_test.go"),
			requiredSnippets: []string{
				`Legacy AppContainer Rejected`,
				`app_container is no longer supported; use app-container`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "ai", "chat", "context_prefetch_additional_test.go"),
			requiredSnippets: []string{
				`expected legacy host mention to be ignored`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "alerts", "utility_test.go"),
			requiredSnippets: []string{
				`legacy host alias rejected`,
				`legacy container alias rejected`,
				`legacy docker alias rejected`,
				`legacy k8s alias rejected`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "metrics", "incident_recorder_test.go"),
			requiredSnippets: []string{
				`TestStartRecordingCanonicalizesLegacyHostAlias`,
				`expected legacy host alias to canonicalize to agent`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "api", "resources_test.go"),
			requiredSnippets: []string{
				`/api/resources?type=host`,
				`unsupported type filter token(s): host`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "api", "resources_frontend_types_test.go"),
			requiredSnippets: []string{
				`unsupported host ignored by parser`,
				`TestUnsupportedResourceTypeFilterTokensRejectsLegacyAliases`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "api", "router_helpers_additional_test.go"),
			requiredSnippets: []string{
				`metadata legacy resource type ignored`,
				`Metadata:   map[string]interface{}{"resourceType": "host"}`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "api", "reporting_handlers_test.go"),
			requiredSnippets: []string{
				`TestNormalizeReportResourceType_RejectsLegacyAliases`,
				`{"host", "container"}`,
			},
		},
		{
			path: filepath.Join(repoRoot, "internal", "api", "router_misc_additional_test.go"),
			requiredSnippets: []string{
				`TestNormalizeMetricsHistoryResourceType_RejectsLegacyAliases`,
				`[]string{"host", "guest", "docker", "dockerhost", "dockercontainer", "system_container"}`,
			},
		},
	}

	for _, artifact := range artifacts {
		data, err := os.ReadFile(artifact.path)
		if err != nil {
			t.Fatalf("failed to read %s: %v", artifact.path, err)
		}
		content := string(data)
		normalizedPath := filepath.ToSlash(artifact.path)

		for _, snippet := range artifact.requiredSnippets {
			if !strings.Contains(content, snippet) {
				t.Errorf("%s: missing required direct host-alias validator snippet %q", normalizedPath, snippet)
			}
		}
	}
}

// SRC-04b: Ratchet-to-hard-ban conversion completed 2026-03-01.
//
// All state.* field access patterns and GetState() calls in consumer packages
// reached ceiling 0 through SRC-03a → SRC-04g migration work. They are now
// enforced as hard bans via migratedResourcePatterns above (per-file, per-line
// error reporting). The legacy ratchet infrastructure (legacyStateRatchet type,
// legacyStateRatchets slice, TestLegacyStateAccessRatchet) has been removed.
//
// Migration changelog preserved in git history (see commits SRC-03f → SRC-04g).
