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
//     from snapshot-owned guest arrays. Backup polling and recovery ingest
//     guest-context assembly now also derive workload node/name/type data from
//     ReadState instead of from snapshot-owned guest arrays. Storage-backup
//     preservation now also derives node/storage membership from
//     ReadState.StoragePools() instead of from snapshot-owned storage arrays.
//     Physical-disk refresh/merge paths now also derive disk, node, and linked
//     host context from ReadState instead of from snapshot-owned physical-disk
//     arrays.
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
	"reflect"
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

func TestResourceAPIUsesCanonicalTenantUnifiedSeed(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "api", "resources.go"))
	if err != nil {
		t.Fatalf("failed to read resources.go: %v", err)
	}
	source := string(data)

	if strings.Contains(source, "GetStateForTenant(") {
		t.Fatalf("internal/api/resources.go must not fall back to tenant StateSnapshot seeding")
	}
	if !strings.Contains(source, "UnifiedResourceSnapshotForTenant(orgID)") {
		t.Fatalf("internal/api/resources.go must use tenant unified resource snapshots as the canonical seed")
	}
}

func TestResourceAPIExposesDedicatedFacetReads(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "api", "resources.go"))
	if err != nil {
		t.Fatalf("failed to read resources.go: %v", err)
	}
	source := string(data)

	requiredSnippets := []string{
		"HandleGetResourceFacets",
		"HandleGetResourceTimeline",
		"unified.ParseResourceChangeFilters(r.URL.Query()[\"kind\"], r.URL.Query()[\"sourceType\"], r.URL.Query()[\"sourceAdapter\"])",
		"GetRecentChangesFiltered(resourceID, since, limit, filters)",
		"CountRecentChangesFiltered(resourceID, since, filters)",
		"CountRecentChangesByKindFiltered(resourceID, since, filters)",
		"CountRecentChangesBySourceTypeFiltered(resourceID, since, filters)",
		"sourceAdapter",
		"strings.HasSuffix(r.URL.Path, \"/facets\")",
		"strings.HasSuffix(r.URL.Path, \"/timeline\")",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("internal/api/resources.go must expose canonical facet read snippet %q", snippet)
		}
	}
}

func TestResourceChangeFilterParsingIsOwnedByUnifiedResources(t *testing.T) {
	requiredSnippets := map[string][]string{
		filepath.Join(".", "change_filters.go"): {
			"func ParseResourceChangeFilters(kinds, sourceTypes, sourceAdapters []string) (ResourceChangeFilters, error)",
			"func parseResourceChangeKinds(values []string) ([]ChangeKind, error)",
			"func parseResourceChangeSourceTypes(values []string) ([]ChangeSourceType, error)",
			"func parseResourceChangeSourceAdapters(values []string) ([]ChangeSourceAdapter, error)",
		},
		filepath.Join("..", "api", "resources.go"): {
			"unified.ParseResourceChangeFilters(r.URL.Query()[\"kind\"], r.URL.Query()[\"sourceType\"], r.URL.Query()[\"sourceAdapter\"])",
		},
	}
	for name, snippets := range requiredSnippets {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		for _, snippet := range snippets {
			if !strings.Contains(string(data), snippet) {
				t.Fatalf("%s must contain %q", name, snippet)
			}
		}
	}
}

func TestResourcePolicyPresentationUsesCanonicalLabels(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(".", "policy_presentation.go"))
	if err != nil {
		t.Fatalf("failed to read policy_presentation.go: %v", err)
	}
	source := string(data)

	requiredSnippets := []string{
		"ResourceSensitivityOrder",
		"ResourceRoutingScopeOrder",
		"ResourceRedactionHintOrder",
		"ResourceSensitivityLabel(",
		"ResourceRoutingScopeLabel(",
		"ResourceRedactionHintLabel(",
		"ResourcePolicyRedactionLabels(",
		"ResourcePolicyRedactionLabelsFromCounts(",
		"ResourcePolicySensitivitySummaryFromCounts(",
		"ResourcePolicyRoutingSummaryFromCounts(",
		"ResourcePolicySummaryLines(",
		"ResourcePolicyRedacts(",
		"ResourcePolicyUsesAISafeSummary(",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("policy_presentation.go must contain %q", snippet)
		}
	}
}

func TestResourcePolicyCloneHelperUsedByAIConsumers(t *testing.T) {
	requiredFiles := []string{
		filepath.Join("..", "ai", "chat", "context_prefetch.go"),
		filepath.Join("..", "ai", "tools", "tools_query.go"),
		filepath.Join(".", "policy_metadata.go"),
	}
	requiredSnippets := map[string]string{
		filepath.Join("..", "ai", "chat", "context_prefetch.go"): "unifiedresources.CloneResourcePolicy(resolved.Resource.Policy)",
		filepath.Join("..", "ai", "tools", "tools_query.go"):     "unifiedresources.CloneResourcePolicy(resourceCopy.Policy)",
		filepath.Join(".", "policy_metadata.go"):                 "func CloneResourcePolicy(policy *ResourcePolicy) *ResourcePolicy",
	}
	for _, name := range requiredFiles {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		snippet := requiredSnippets[name]
		if !strings.Contains(string(data), snippet) {
			t.Fatalf("%s must contain %q", name, snippet)
		}
	}
}

func TestAISafeSummarySuffixHelperIsOwnedByUnifiedResources(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(".", "policy_metadata.go"))
	if err != nil {
		t.Fatalf("failed to read policy_metadata.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"func resourceAISafeSummaryPolicySuffix(sensitivity ResourceSensitivity) string",
		"return \"redacted for cloud summary\"",
		"return \"local-only context\"",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("policy_metadata.go must contain %q", snippet)
		}
	}
}

func TestCanonicalMetadataRefreshHelperUsedByConsumers(t *testing.T) {
	requiredSnippets := map[string][]string{
		filepath.Join(".", "policy_metadata.go"): {
			"func RefreshCanonicalMetadata(resource *Resource)",
			"func RefreshCanonicalMetadataSlice(resources []Resource) []Resource",
		},
		filepath.Join(".", "clone.go"): {
			"RefreshCanonicalMetadata(&out)",
		},
		filepath.Join("..", "api", "resources.go"): {
			"unified.RefreshCanonicalMetadata(&resourceCopy)",
			"unified.RefreshCanonicalMetadataSlice(paged)",
			"unified.RefreshCanonicalMetadataSlice(children)",
		},
		filepath.Join("..", "ai", "resource_context.go"): {
			"unifiedresources.RefreshCanonicalMetadataSlice(urp.GetInfrastructure())",
			"unifiedresources.RefreshCanonicalMetadataSlice(urp.GetWorkloads())",
			"unifiedresources.RefreshCanonicalMetadataSlice(urp.GetAll())",
		},
		filepath.Join("..", "ai", "intelligence.go"): {
			"unifiedresources.RefreshCanonicalMetadataSlice(unifiedResourceProvider.GetAll())",
		},
	}

	for name, snippets := range requiredSnippets {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		for _, snippet := range snippets {
			if !strings.Contains(string(data), snippet) {
				t.Fatalf("%s must contain %q", name, snippet)
			}
		}
	}
}

func TestResourcePolicyLabelHelpersUsedByAIConsumers(t *testing.T) {
	requiredSnippets := map[string][]string{
		filepath.Join("..", "ai", "intelligence.go"): {
			"func (i *Intelligence) HasCorrelationsSource() bool",
			"func (i *Intelligence) GetCorrelations(resourceID string) []*correlation.Correlation",
			"func (i *Intelligence) FormatCorrelationsContext(resourceID string) string",
		},
		filepath.Join("..", "ai", "service.go"): {
			"intel.FormatCorrelationsContext(resourceID)",
		},
		filepath.Join("..", "ai", "patrol_ai.go"): {
			"intelFacade.GetCorrelations(\"\")",
		},
		filepath.Join("..", "api", "ai_intelligence_handlers.go"): {
			"intel.HasCorrelationsSource()",
			"intel.GetCorrelations(resourceID)",
		},
		filepath.Join("..", "ai", "chat", "knowledge_extractor.go"): {
			"unifiedresources.ResourcePolicyLabel(",
			"unifiedresources.ResourcePolicyRedactedValue(",
		},
		filepath.Join("..", "ai", "chat", "context_prefetch.go"): {
			"tools.CanonicalDiscoveryResourceType(",
			"tools.DiscoveryProviderResourceType(",
			"tools.CanonicalDiscoveryTargetID(",
			"unifiedresources.ResourcePolicyRequiresGovernedSummary(mention.Policy)",
			"unifiedresources.FormatResourcePolicyGovernedSummary(mention.AISafeSummary, mention.Policy)",
		},
		filepath.Join("..", "ai", "tools", "tools_discovery.go"): {
			"func CanonicalDiscoveryResourceType(raw string) string",
			"func DiscoveryProviderResourceType(canonical string) string",
			"func CanonicalDiscoveryTargetID(discovery *ResourceDiscoveryInfo, fallbackTargetID string) string",
		},
		filepath.Join("..", "ai", "resource_export.go"): {
			"unifiedresources.ResourceRedactionLabelsFromHints(redactionHints)",
		},
		filepath.Join("..", "ai", "resource_context.go"): {
			"unifiedresources.ResourcePolicyLabel(",
			"unifiedresources.ResourceDisplayName(",
			"unifiedresources.ResourceClusterName(",
			"unifiedresources.ResourceIPSummary(",
		},
		filepath.Join("..", "unifiedresources", "unified_ai_adapter.go"): {
			"ResourceDisplayName(results[i])",
			"ResourceDisplayName(results[j])",
		},
		filepath.Join(".", "policy_presentation.go"): {
			"func ResourcePolicyLabel(name, aiSafeSummary string, policy *ResourcePolicy) string",
			"if ResourcePolicyRequiresGovernedSummary(policy) {",
			"return ResourcePolicyRedactedLabel",
			"func ResourcePolicyRedactedValue(value string, policy *ResourcePolicy, hints ...ResourceRedactionHint) string",
			"const ResourcePolicyRedactedLabel = \"redacted by policy\"",
			"func ResourceRedactionLabelsFromHints(hints []ResourceRedactionHint) []string",
			"func ResourceClusterName(resource Resource) string",
			"func ResourceIPSummary(resource Resource, limit int) string",
			"func ResourcePolicyRequiresGovernedSummary(policy *ResourcePolicy) bool",
			"func ResourcePolicyGovernedSummaryPreamble() string",
			"func ResourcePolicyGovernedSummaryFooter() string",
			"func FormatResourcePolicyGovernedSummary(summary string, policy *ResourcePolicy) string",
			"func ResourceDisplayName(resource Resource) string",
		},
	}
	for name, snippets := range requiredSnippets {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		for _, snippet := range snippets {
			if !strings.Contains(string(data), snippet) {
				t.Fatalf("%s must contain %q", name, snippet)
			}
		}
	}
}

func TestPolicyPostureSummaryIsOwnedByUnifiedResources(t *testing.T) {
	requiredSnippets := map[string][]string{
		filepath.Join(".", "policy_posture.go"): {
			"type PolicyPostureSummary struct {",
			"func SummarizePolicyPosture(resources []Resource) *PolicyPostureSummary",
		},
		filepath.Join("..", "ai", "intelligence.go"): {
			"unifiedresources.PolicyPostureSummary",
			"unifiedresources.SummarizePolicyPosture(",
		},
		filepath.Join("..", "ai", "resource_context.go"): {
			"unifiedresources.SummarizePolicyPosture(allResources)",
		},
	}

	for name, snippets := range requiredSnippets {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		for _, snippet := range snippets {
			if !strings.Contains(string(data), snippet) {
				t.Fatalf("%s must contain %q", name, snippet)
			}
		}
	}
}

func TestExportDecisionHelpersUsedByAIConsumers(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "ai", "resource_export.go"))
	if err != nil {
		t.Fatalf("failed to read resource_export.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"unifiedresources.ExportSensitivityFloor(sensitivityCounts)",
		"unifiedresources.ExportDecisionForContext(sensitivityFloor, localOnlyCount, len(redactions))",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("internal/ai/resource_export.go must pin canonical export decision snippet %q", snippet)
		}
	}
}

func TestExportDecisionHelpersCanonicalInUnifiedResources(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("privacy.go"))
	if err != nil {
		t.Fatalf("failed to read privacy.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"func ExportSensitivityFloor(counts map[ResourceSensitivity]int) DataSensitivity",
		"func ExportDecisionForContext(sensitivityFloor DataSensitivity, localOnlyCount int, redactionCount int) (ExportDecision, string)",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("internal/unifiedresources/privacy.go must pin canonical export helper snippet %q", snippet)
		}
	}
}

func TestRootCauseEngineUsesCanonicalRelationshipModel(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "ai", "correlation", "rootcause.go"))
	if err != nil {
		t.Fatalf("failed to read rootcause.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"type RelationshipType = unifiedresources.RelationshipType",
		"type ResourceRelationship = unifiedresources.ResourceRelationship",
		"GetRelationships(resourceID string) []ResourceRelationship",
		"score += relationshipScore(rel.Type)",
		"func relationshipScore(t RelationshipType) float64",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("internal/ai/correlation/rootcause.go must pin canonical relationship snippet %q", snippet)
		}
	}
}

func TestResourceDisplayNameUsedByInfrastructureConsumers(t *testing.T) {
	requiredSnippets := map[string][]string{
		filepath.Join("..", "monitoring", "connected_infrastructure.go"): {
			"unifiedresources.ResourceDisplayName(resource)",
		},
		filepath.Join(".", "monitored_systems.go"): {
			"if name := ResourceDisplayName(*resource); name != \"\" {",
		},
	}
	for name, snippets := range requiredSnippets {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		for _, snippet := range snippets {
			if !strings.Contains(string(data), snippet) {
				t.Fatalf("%s must contain %q", name, snippet)
			}
		}
	}
}

func TestResourceTimelineStoreIndexesSupportFilteredReads(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("store.go"))
	if err != nil {
		t.Fatalf("failed to read store.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"idx_resource_changes_kind_time",
		"idx_resource_changes_source_type_time",
		"idx_resource_changes_source_adapter_time",
		"ensureResourceChangesIndexes",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("internal/unifiedresources/store.go must pin filtered timeline index snippet %q", snippet)
		}
	}
}

func TestResourceChangeEmissionCoversRelationshipAndCapabilityChanges(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("change_emission.go"))
	if err != nil {
		t.Fatalf("failed to read change_emission.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"change.RelatedResources = relatedResourceIDs(change.ResourceID, before, after)",
		"case resourceRestartChanged(before, after):",
		"case resourceIncidentChanged(before, after):",
		"if !reflect.DeepEqual(before.Relationships, after.Relationships) {",
		"changed = append(changed, \"relationships\")",
		"if resourceIncidentChanged(before, after) {",
		"changed = append(changed, \"incidents\")",
		"if dockerRestartChanged(before, after) {",
		"changed = append(changed, \"docker.restartCount\", \"docker.uptimeSeconds\")",
		"if kubernetesRestartChanged(before, after) {",
		"changed = append(changed, \"kubernetes.restarts\", \"kubernetes.uptimeSeconds\")",
		"if !reflect.DeepEqual(before.Capabilities, after.Capabilities) {",
		"changed = append(changed, \"capabilities\")",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("internal/unifiedresources/change_emission.go must pin canonical relationship/capability change detection snippet %q", snippet)
		}
	}
}

func TestResourceChangePresentationUsesCanonicalLabels(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("change_presentation.go"))
	if err != nil {
		t.Fatalf("failed to read change_presentation.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"func ChangeKindLabel(kind ChangeKind) string",
		"func DescribeChange(change ResourceChange) ChangePresentation",
		"func FormatResourceChangeSummary(change ResourceChange) string",
		"func resourceRelationshipSummary(relationships []ResourceRelationship) string",
		"resourceStateSummary(resource Resource) string",
		"resourceRestartSummary(resource Resource) string",
		"resourceIncidentSummary(resource Resource) string",
		"resourceIncidentSummaryFromSlice(incidents []ResourceIncident) string",
		"resourceIncidentLabel(incident ResourceIncident) string",
		"resourceConfigSummary(resource Resource) string",
		"KindLabel: ChangeKindLabel(change.Kind)",
		"presentation.SourceType = strings.TrimSpace(string(change.SourceType))",
		"presentation.SourceAdapter = strings.TrimSpace(string(change.SourceAdapter))",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("internal/unifiedresources/change_presentation.go must pin canonical change presentation snippet %q", snippet)
		}
	}
}

func TestResourceRelationshipContextUsesCanonicalRelationshipPresentation(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "ai", "service.go"))
	if err != nil {
		t.Fatalf("failed to read service.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"func (s *Service) buildResourceRelationshipContext(resourceID string) string",
		"if relationshipContext := s.buildResourceRelationshipContext(resourceID); relationshipContext != \"\" {",
		"Get canonical relationship context from unified resources.",
		"unifiedresources.FormatResourceRelationshipContext(resource, 3)",
		"unifiedresources.FormatResourceRecentChangesContext(changes, false, \"###\")",
		"type canonicalResourceGetter interface {",
		"intel.FormatCorrelationsContext(resourceID)",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("internal/ai/service.go must pin canonical relationship presentation snippet %q", snippet)
		}
	}
}

func TestResourceRelationshipModelUsesCanonicalEdgeComment(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("relationships.go"))
	if err != nil {
		t.Fatalf("failed to read relationships.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"// ResourceRelationship represents a typed relationship edge between two unified resources.",
		"type ResourceRelationship struct {",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("internal/unifiedresources/relationships.go must pin canonical relationship edge snippet %q", snippet)
		}
	}
}

func TestPatrolSeedCorrelationContextUsesCanonicalSummaryFormatter(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "ai", "patrol_ai.go"))
	if err != nil {
		t.Fatalf("failed to read patrol_ai.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"# Known Resource Correlations",
		"correlation.FormatCorrelationSummary(c)",
		"memory.ChangeFromUnifiedResourceChange(change)",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("internal/ai/patrol_ai.go must pin canonical correlation presentation snippet %q", snippet)
		}
	}
}

func TestIntelligenceRecentChangesUseCanonicalSummaryFormatter(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "ai", "intelligence.go"))
	if err != nil {
		t.Fatalf("failed to read intelligence.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"func (i *Intelligence) GetRecentChanges(since time.Time, limit int) []unifiedresources.ResourceChange",
		"func (i *Intelligence) DescribeResource(resourceID string) (string, string)",
		"func (i *Intelligence) HasRecentChangesSource() bool",
		"unifiedresources.FormatResourceRecentChangesContext(recent, includeResourcePrefix, \"##\")",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("internal/ai/intelligence.go must pin canonical recent-change snippet %q", snippet)
		}
	}
}

func TestMemoryChangeConversionHelpersAreSharedAcrossAIConsumers(t *testing.T) {
	requiredFiles := map[string][]string{
		filepath.Join("..", "ai", "patrol_ai.go"): {
			"memory.ChangeFromUnifiedResourceChange(change)",
		},
		filepath.Join("..", "ai", "intelligence.go"): {
			"memory.ResourceChangeFromMemoryChange(change)",
		},
		filepath.Join("..", "ai", "memory", "presentation.go"): {
			"func ChangeFromUnifiedResourceChange(change unifiedresources.ResourceChange) Change",
			"func ResourceChangeFromMemoryChange(change Change) unifiedresources.ResourceChange",
		},
	}

	for path, snippets := range requiredFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read %s: %v", path, err)
		}
		source := string(data)
		for _, snippet := range snippets {
			if !strings.Contains(source, snippet) {
				t.Fatalf("%s must pin canonical memory conversion snippet %q", path, snippet)
			}
		}
	}
}

func TestAIRecentChangesHandlerUsesCanonicalIntelligencePath(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "api", "ai_intelligence_handlers.go"))
	if err != nil {
		t.Fatalf("failed to read ai_intelligence_handlers.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"intel := patrol.GetIntelligence()",
		"intel.HasRecentChangesSource()",
		"intel.GetRecentChanges(since, 100)",
		"intel.DescribeResource(change.ResourceID)",
		"unifiedresources.FormatResourceChangeSummary(change)",
		"Recent changes not initialized",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("internal/api/ai_intelligence_handlers.go must pin canonical recent-changes snippet %q", snippet)
		}
	}
}

func TestResourcePresentationsUseSharedDurationHelper(t *testing.T) {
	requiredFiles := []string{
		"change_presentation.go",
		"relationship_presentation.go",
	}
	for _, name := range requiredFiles {
		data, err := os.ReadFile(filepath.Join(name))
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		if !strings.Contains(string(data), "utils.FormatDurationAgo(") {
			t.Fatalf("%s must use the shared utils.FormatDurationAgo helper", name)
		}
	}
}

func TestResourceFacetCountsAreCanonicalResourceFields(t *testing.T) {
	typesData, err := os.ReadFile(filepath.Join("types.go"))
	if err != nil {
		t.Fatalf("failed to read types.go: %v", err)
	}
	typesSource := string(typesData)
	if !strings.Contains(typesSource, "FacetCounts           ResourceFacetCounts") {
		t.Fatalf("internal/unifiedresources/types.go must expose Resource.FacetCounts on the canonical resource model")
	}
	if !strings.Contains(typesSource, "json:\"facetCounts,omitempty\"") {
		t.Fatalf("internal/unifiedresources/types.go must keep the facetCounts JSON contract")
	}
	if !strings.Contains(typesSource, "RecentChangeKinds          map[ChangeKind]int") {
		t.Fatalf("internal/unifiedresources/types.go must expose grouped timeline counts on the canonical facet model")
	}
	if !strings.Contains(typesSource, "RecentChangeSourceTypes    map[ChangeSourceType]int") {
		t.Fatalf("internal/unifiedresources/types.go must expose grouped timeline source-type counts on the canonical facet model")
	}
	if !strings.Contains(typesSource, "RecentChangeSourceAdapters map[ChangeSourceAdapter]int") {
		t.Fatalf("internal/unifiedresources/types.go must expose grouped timeline source-adapter counts on the canonical facet model")
	}

	cloneData, err := os.ReadFile(filepath.Join("clone.go"))
	if err != nil {
		t.Fatalf("failed to read clone.go: %v", err)
	}
	cloneSource := string(cloneData)
	requiredSnippets := []string{
		"out.FacetCounts = resourceFacetCounts(out)",
		"func resourceFacetCounts(resource Resource) ResourceFacetCounts",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(cloneSource, snippet) {
			t.Fatalf("internal/unifiedresources/clone.go must derive canonical facet counts via %q", snippet)
		}
	}
}

func TestCanonicalIdentityIsCanonicalResourceField(t *testing.T) {
	typesData, err := os.ReadFile(filepath.Join("types.go"))
	if err != nil {
		t.Fatalf("failed to read types.go: %v", err)
	}
	typesSource := string(typesData)
	requiredSnippets := []string{
		"json:\"canonicalIdentity,omitempty\"",
		"type CanonicalIdentity struct {",
		"DisplayName string   `json:\"displayName,omitempty\"`",
		"Aliases     []string `json:\"aliases,omitempty\"`",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(typesSource, snippet) {
			t.Fatalf("internal/unifiedresources/types.go must keep the canonical identity contract snippet %q", snippet)
		}
	}

	cloneData, err := os.ReadFile(filepath.Join("clone.go"))
	if err != nil {
		t.Fatalf("failed to read clone.go: %v", err)
	}
	cloneSource := string(cloneData)
	requiredCloneSnippets := []string{
		"RefreshCanonicalMetadata(&out)",
	}
	for _, snippet := range requiredCloneSnippets {
		if !strings.Contains(cloneSource, snippet) {
			t.Fatalf("internal/unifiedresources/clone.go must preserve canonical identity via %q", snippet)
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

func TestResourceParentBySourceStateRemainsInternal(t *testing.T) {
	resourceType := reflect.TypeOf(Resource{})

	parentIDField, ok := resourceType.FieldByName("ParentID")
	if !ok {
		t.Fatalf("expected Resource.ParentID field")
	}
	if got := parentIDField.Tag.Get("json"); got != "parentId,omitempty" {
		t.Fatalf("Resource.ParentID json tag = %q, want %q", got, "parentId,omitempty")
	}

	parentBySourceField, ok := resourceType.FieldByName("parentBySource")
	if !ok {
		t.Fatalf("expected Resource.parentBySource field")
	}
	if parentBySourceField.IsExported() {
		t.Fatalf("expected Resource.parentBySource to remain internal-only")
	}
	if got := parentBySourceField.Tag.Get("json"); got != "" {
		t.Fatalf("expected Resource.parentBySource to have no JSON contract, got %q", got)
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

func TestResourceAPIHotPathUsesSingleRegistryListSnapshot(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	path := filepath.Join(repoRoot, "internal", "api", "resources.go")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}

	source := string(data)
	normalizedPath := filepath.ToSlash(path)

	if strings.Count(source, "allResources := registry.List()") != 2 {
		t.Fatalf("%s: expected HandleListResources and HandleStats to each seed exactly one registry list snapshot", normalizedPath)
	}
	if strings.Count(source, "computeResourceContractByType(allResources)") != 2 {
		t.Fatalf("%s: expected canonical by-type aggregations to reuse the seeded registry snapshot in both handlers", normalizedPath)
	}
	if strings.Contains(source, "computeResourceContractByType(registry.List())") {
		t.Fatalf("%s: duplicate registry.List() hot-path aggregation detected", normalizedPath)
	}
}
