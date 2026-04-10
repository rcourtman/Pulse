package mock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

var (
	numberedPlatformListRE   = regexp.MustCompile("^\\d+\\.\\s+`([^`]+)`")
	currentSupportMatrixRE   = regexp.MustCompile("^\\|\\s+`([^`]+)`\\s+\\|")
	currentSupportSetupRowRE = regexp.MustCompile("^\\|\\s+`([^`]+)`\\s+\\|\\s+([^|]+?)\\s+\\|")
)

type platformSupportManifest struct {
	SchemaVersion                    int                            `json:"schema_version"`
	DefaultInfrastructureSourceOrder []string                       `json:"default_infrastructure_source_order"`
	Platforms                        []platformSupportManifestEntry `json:"platforms"`
}

type platformSupportManifestEntry struct {
	ID              string   `json:"id"`
	GovernanceState string   `json:"governance_state"`
	OnboardingPaths []string `json:"onboarding_paths"`
	UILabel         string   `json:"ui_label"`
	UITone          string   `json:"ui_tone"`
	Aliases         []string `json:"aliases"`
	DisplayTokens   []string `json:"display_tokens"`
	StorageFamily   string   `json:"storage_family"`
}

func TestPlatformSupportManifestMatchesSupportModel(t *testing.T) {
	model := loadPlatformSupportModel(t)
	manifest := loadPlatformSupportManifest(t)

	classified := parsePlatformListSection(t, model, "### First-class platforms")
	admitted := parsePlatformListSection(t, model, "### Admitted platforms (not yet supported)")
	presentationOnly := parsePlatformListSection(t, model, "### Presentation-only platform vocabulary")
	matrix := parseCurrentSupportMatrixPlatforms(t, model)

	if diff := diffPlatformSets(classified, matrix); diff != "" {
		t.Fatalf("current supported platform definitions drifted between classification and support matrix:\n%s", diff)
	}

	if diff := diffPlatformSets(classified, manifestPlatformsByState(t, manifest, "supported")); diff != "" {
		t.Fatalf("supported platform manifest drifted from the canonical support model:\n%s", diff)
	}
	if diff := diffPlatformSets(admitted, manifestPlatformsByState(t, manifest, "admitted")); diff != "" {
		t.Fatalf("admitted platform manifest drifted from the canonical support model:\n%s", diff)
	}
	if diff := diffPlatformSets(
		presentationOnly,
		manifestPlatformsByState(t, manifest, "presentation-only"),
	); diff != "" {
		t.Fatalf("presentation-only platform manifest drifted from the canonical support model:\n%s", diff)
	}
	if diff := diffPlatformFieldMap(
		parseCurrentSupportMatrixOnboardingPaths(t, model),
		manifestPlatformOnboardingPathsByState(t, manifest, "supported"),
	); diff != "" {
		t.Fatalf("supported onboarding-path manifest drifted from the canonical support model:\n%s", diff)
	}
	if diff := diffPlatformSets(classified, manifest.DefaultInfrastructureSourceOrder); diff != "" {
		t.Fatalf("default infrastructure source ordering drifted from the canonical supported platform set:\n%s", diff)
	}
}

func TestMockCoverageMatchesCurrentSupportedPlatformSet(t *testing.T) {
	manifest := loadPlatformSupportManifest(t)
	supported := manifestPlatformsByState(t, manifest, "supported")

	checkers := map[string]func(*testing.T, FixtureGraph){
		"agent":       assertAgentMockCoverage,
		"docker":      assertDockerMockCoverage,
		"kubernetes":  assertKubernetesMockCoverage,
		"proxmox-pbs": assertProxmoxPBSMockCoverage,
		"proxmox-pmg": assertProxmoxPMGMockCoverage,
		"proxmox-pve": assertProxmoxPVEMockCoverage,
		"truenas":     assertTrueNASMockCoverage,
	}

	if diff := diffPlatformSets(supported, sortedPlatformKeys(checkers)); diff != "" {
		t.Fatalf("supported platform coverage guardrail drifted from the canonical support model:\n%s", diff)
	}

	graph := buildFixtureGraph(DefaultConfig, time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC))
	for _, platform := range supported {
		checker := checkers[platform]
		checker(t, graph)
	}
}

func TestVMwareFixturesRemainAdmittedButNotSupported(t *testing.T) {
	model := loadPlatformSupportModel(t)
	manifest := loadPlatformSupportManifest(t)
	supported := manifestPlatformsByState(t, manifest, "supported")
	admitted := manifestPlatformsByState(t, manifest, "admitted")
	presentationOnly := manifestPlatformsByState(t, manifest, "presentation-only")

	if containsPlatform(supported, "vmware-vsphere") {
		t.Fatal("vmware-vsphere must not appear in the current supported platform set before live proof admits it")
	}
	if !containsPlatform(admitted, "vmware-vsphere") {
		t.Fatal("expected vmware-vsphere to remain admitted while it is outside the supported platform set")
	}
	if containsPlatform(presentationOnly, "vmware-vsphere") {
		t.Fatal("vmware-vsphere must not regress into presentation-only vocabulary once admitted")
	}
	if !strings.Contains(model, "| `vmware-vsphere` |") {
		t.Fatal("expected platform support model to keep the vmware-vsphere admission row")
	}
	vmwareOnboardingPaths, ok := manifestPlatformOnboardingPaths(manifest)["vmware-vsphere"]
	if !ok {
		t.Fatal("expected vmware-vsphere onboarding-path manifest entry")
	}
	if diff := diffPlatformSets([]string{"platform-connections"}, vmwareOnboardingPaths); diff != "" {
		t.Fatalf("vmware-vsphere onboarding path drifted from the admission model:\n%s", diff)
	}
	if !strings.Contains(model, "| `vmware-vsphere` | platform connections to `vCenter` only |") {
		t.Fatal("expected platform support model to keep the vmware-vsphere platform-connections admission floor")
	}

	graph := buildFixtureGraph(DefaultConfig, time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC))
	if len(graph.PlatformFixtures.VMware.Hosts) == 0 {
		t.Fatal("expected VMware mock fixtures to include hosts")
	}
	if len(graph.PlatformFixtures.VMware.VMs) == 0 {
		t.Fatal("expected VMware mock fixtures to include VMs")
	}
	if len(graph.PlatformFixtures.VMware.Datastores) == 0 {
		t.Fatal("expected VMware mock fixtures to include datastores")
	}

	fixture := DefaultVMwareConnectionFixture()
	if fixture.CollectedAt.IsZero() {
		t.Fatal("expected VMware mock connection fixture freshness")
	}
	if fixture.Hosts == 0 || fixture.VMs == 0 || fixture.Datastores == 0 {
		t.Fatalf("expected VMware mock connection fixture to describe canonical inventory, got %+v", fixture)
	}
}

func assertAgentMockCoverage(t *testing.T, graph FixtureGraph) {
	t.Helper()

	if len(graph.State.Hosts) == 0 {
		t.Fatal("expected host-agent fixtures for the supported agent platform")
	}
	if len(graph.State.PhysicalDisks) == 0 {
		t.Fatal("expected physical-disk fixtures for the supported agent platform")
	}
}

func assertDockerMockCoverage(t *testing.T, graph FixtureGraph) {
	t.Helper()

	if len(graph.State.DockerHosts) == 0 {
		t.Fatal("expected docker host fixtures for the supported docker platform")
	}

	containerCount := 0
	for _, host := range graph.State.DockerHosts {
		containerCount += len(host.Containers)
	}
	if containerCount == 0 {
		t.Fatal("expected docker container fixtures for the supported docker platform")
	}
}

func assertKubernetesMockCoverage(t *testing.T, graph FixtureGraph) {
	t.Helper()

	if len(graph.State.KubernetesClusters) == 0 {
		t.Fatal("expected kubernetes cluster fixtures for the supported kubernetes platform")
	}

	nodeCount := 0
	deploymentCount := 0
	podCount := 0
	for _, cluster := range graph.State.KubernetesClusters {
		nodeCount += len(cluster.Nodes)
		deploymentCount += len(cluster.Deployments)
		podCount += len(cluster.Pods)
	}
	if nodeCount == 0 || deploymentCount == 0 || podCount == 0 {
		t.Fatalf(
			"expected kubernetes fixtures to include nodes, deployments, and pods; got nodes=%d deployments=%d pods=%d",
			nodeCount,
			deploymentCount,
			podCount,
		)
	}
}

func assertProxmoxPVEMockCoverage(t *testing.T, graph FixtureGraph) {
	t.Helper()

	if len(graph.State.Nodes) == 0 {
		t.Fatal("expected proxmox-pve node fixtures")
	}
	if len(graph.State.VMs) == 0 {
		t.Fatal("expected proxmox-pve VM fixtures")
	}
	if len(graph.State.Containers) == 0 {
		t.Fatal("expected proxmox-pve system-container fixtures")
	}
	if len(graph.State.Storage) == 0 {
		t.Fatal("expected proxmox-pve storage fixtures")
	}
	if len(graph.State.PhysicalDisks) == 0 {
		t.Fatal("expected proxmox-pve physical-disk fixtures")
	}
	if len(graph.State.PVEBackups.StorageBackups) == 0 {
		t.Fatal("expected proxmox-pve recovery fixtures")
	}
}

func assertProxmoxPBSMockCoverage(t *testing.T, graph FixtureGraph) {
	t.Helper()

	if len(graph.State.PBSInstances) == 0 {
		t.Fatal("expected proxmox-pbs instance fixtures")
	}
	if len(graph.State.PBSBackups) == 0 {
		t.Fatal("expected proxmox-pbs backup fixtures")
	}
}

func assertProxmoxPMGMockCoverage(t *testing.T, graph FixtureGraph) {
	t.Helper()

	if len(graph.State.PMGInstances) == 0 {
		t.Fatal("expected proxmox-pmg instance fixtures")
	}
	if len(graph.State.PMGBackups) == 0 {
		t.Fatal("expected proxmox-pmg backup fixtures")
	}
}

func assertTrueNASMockCoverage(t *testing.T, graph FixtureGraph) {
	t.Helper()

	snapshot := graph.PlatformFixtures.TrueNAS
	if strings.TrimSpace(snapshot.System.Hostname) == "" {
		t.Fatal("expected truenas system fixture")
	}
	if len(snapshot.Apps) == 0 {
		t.Fatal("expected truenas app fixtures")
	}
	if len(snapshot.Pools) == 0 || len(snapshot.Datasets) == 0 {
		t.Fatal("expected truenas storage fixtures")
	}
	if len(snapshot.Disks) == 0 {
		t.Fatal("expected truenas disk fixtures")
	}
	if len(snapshot.ZFSSnapshots)+len(snapshot.ReplicationTasks) == 0 {
		t.Fatal("expected truenas recovery fixtures")
	}

	fixture := DefaultTrueNASConnectionFixture()
	if fixture.CollectedAt.IsZero() {
		t.Fatal("expected truenas mock connection fixture freshness")
	}
	if fixture.Systems == 0 || fixture.StoragePools == 0 || fixture.Apps == 0 || fixture.Disks == 0 {
		t.Fatalf("expected truenas mock connection fixture to describe canonical inventory, got %+v", fixture)
	}
}

func loadPlatformSupportManifest(t *testing.T) platformSupportManifest {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate current test file for platform support manifest lookup")
	}

	manifestPath := filepath.Join(
		filepath.Dir(currentFile),
		"..",
		"..",
		"docs",
		"release-control",
		"v6",
		"internal",
		"PLATFORM_SUPPORT_MANIFEST.json",
	)
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read platform support manifest: %v", err)
	}

	var manifest platformSupportManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("unmarshal platform support manifest: %v", err)
	}
	if manifest.SchemaVersion < 1 {
		t.Fatalf("expected positive platform support manifest schema version, got %d", manifest.SchemaVersion)
	}
	if len(manifest.Platforms) == 0 {
		t.Fatal("expected platform support manifest to declare at least one platform")
	}
	if len(manifest.DefaultInfrastructureSourceOrder) == 0 {
		t.Fatal("expected platform support manifest to declare a default infrastructure source order")
	}

	return manifest
}

func loadPlatformSupportModel(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate current test file for platform support model lookup")
	}

	modelPath := filepath.Join(
		filepath.Dir(currentFile),
		"..",
		"..",
		"docs",
		"release-control",
		"v6",
		"internal",
		"PLATFORM_SUPPORT_MODEL.md",
	)
	modelBytes, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("read platform support model: %v", err)
	}
	return string(modelBytes)
}

func parsePlatformListSection(t *testing.T, model string, heading string) []string {
	t.Helper()

	var platforms []string
	inSection := false

	for _, raw := range strings.Split(model, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == heading:
			inSection = true
			continue
		case !inSection:
			continue
		case strings.HasPrefix(line, "### ") || strings.HasPrefix(line, "## "):
			return requireNonEmptyPlatformList(t, platforms, heading)
		}

		matches := numberedPlatformListRE.FindStringSubmatch(line)
		if len(matches) == 2 {
			platforms = append(platforms, matches[1])
		}
	}

	return requireNonEmptyPlatformList(t, platforms, heading)
}

func parseCurrentSupportMatrixPlatforms(t *testing.T, model string) []string {
	t.Helper()

	var platforms []string
	inSection := false

	for _, raw := range strings.Split(model, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "## Current Support Matrix":
			inSection = true
			continue
		case !inSection:
			continue
		case strings.HasPrefix(line, "## "):
			return requireNonEmptyPlatformList(t, platforms, "current support matrix")
		}

		matches := currentSupportMatrixRE.FindStringSubmatch(line)
		if len(matches) == 2 {
			platforms = append(platforms, matches[1])
		}
	}

	return requireNonEmptyPlatformList(t, platforms, "current support matrix")
}

func parseCurrentSupportMatrixOnboardingPaths(t *testing.T, model string) map[string][]string {
	t.Helper()

	rows := make(map[string][]string)
	inSetupTable := false

	for _, raw := range strings.Split(model, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "| Platform") && strings.Contains(line, "| Setup"):
			inSetupTable = true
			continue
		case !inSetupTable:
			continue
		case strings.HasPrefix(line, "## "):
			return requireNonEmptyPlatformFieldMap(t, rows, "current support matrix onboarding paths")
		case line == "" || strings.HasPrefix(line, "| ---"):
			continue
		}

		matches := currentSupportSetupRowRE.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		rows[matches[1]] = parseSetupCellOnboardingPaths(t, matches[2])
	}

	return requireNonEmptyPlatformFieldMap(t, rows, "current support matrix onboarding paths")
}

func manifestPlatformsByState(
	t *testing.T,
	manifest platformSupportManifest,
	governanceState string,
) []string {
	t.Helper()

	platforms := make([]string, 0)
	for _, platform := range manifest.Platforms {
		if strings.TrimSpace(platform.ID) == "" {
			t.Fatal("expected platform support manifest platform ids to be non-empty")
		}
		if strings.TrimSpace(platform.GovernanceState) == "" {
			t.Fatalf("expected platform support manifest to classify %s", platform.ID)
		}
		if platform.GovernanceState == governanceState {
			platforms = append(platforms, platform.ID)
		}
	}

	return requireNonEmptyPlatformList(t, platforms, fmt.Sprintf("manifest platforms with state %s", governanceState))
}

func manifestPlatformOnboardingPathsByState(
	t *testing.T,
	manifest platformSupportManifest,
	governanceState string,
) map[string][]string {
	t.Helper()

	rows := make(map[string][]string)
	for _, platform := range manifest.Platforms {
		if platform.GovernanceState != governanceState {
			continue
		}
		rows[platform.ID] = validateManifestOnboardingPaths(t, platform)
	}

	return requireNonEmptyPlatformFieldMap(
		t,
		rows,
		fmt.Sprintf("manifest onboarding paths with state %s", governanceState),
	)
}

func manifestPlatformOnboardingPaths(manifest platformSupportManifest) map[string][]string {
	rows := make(map[string][]string, len(manifest.Platforms))
	for _, platform := range manifest.Platforms {
		rows[platform.ID] = append([]string(nil), platform.OnboardingPaths...)
	}
	return rows
}

func requireNonEmptyPlatformList(t *testing.T, platforms []string, label string) []string {
	t.Helper()

	unique := uniqueSortedPlatforms(platforms)
	if len(unique) == 0 {
		t.Fatalf("expected %s to declare at least one platform", label)
	}
	return unique
}

func requireNonEmptyPlatformFieldMap(
	t *testing.T,
	values map[string][]string,
	label string,
) map[string][]string {
	t.Helper()

	if len(values) == 0 {
		t.Fatalf("expected %s to declare at least one platform", label)
	}
	return values
}

func uniqueSortedPlatforms(platforms []string) []string {
	seen := make(map[string]struct{}, len(platforms))
	for _, platform := range platforms {
		platform = strings.TrimSpace(platform)
		if platform == "" {
			continue
		}
		seen[platform] = struct{}{}
	}

	out := make([]string, 0, len(seen))
	for platform := range seen {
		out = append(out, platform)
	}
	sort.Strings(out)
	return out
}

func sortedPlatformKeys(checkers map[string]func(*testing.T, FixtureGraph)) []string {
	keys := make([]string, 0, len(checkers))
	for key := range checkers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func containsPlatform(platforms []string, want string) bool {
	for _, platform := range platforms {
		if platform == want {
			return true
		}
	}
	return false
}

func diffPlatformSets(want []string, got []string) string {
	wantSet := make(map[string]struct{}, len(want))
	for _, platform := range want {
		wantSet[platform] = struct{}{}
	}

	gotSet := make(map[string]struct{}, len(got))
	for _, platform := range got {
		gotSet[platform] = struct{}{}
	}

	missing := make([]string, 0)
	for _, platform := range want {
		if _, ok := gotSet[platform]; !ok {
			missing = append(missing, platform)
		}
	}

	extra := make([]string, 0)
	for _, platform := range got {
		if _, ok := wantSet[platform]; !ok {
			extra = append(extra, platform)
		}
	}

	if len(missing) == 0 && len(extra) == 0 {
		return ""
	}

	parts := make([]string, 0, 2)
	if len(missing) > 0 {
		parts = append(parts, fmt.Sprintf("missing: %s", strings.Join(missing, ", ")))
	}
	if len(extra) > 0 {
		parts = append(parts, fmt.Sprintf("extra: %s", strings.Join(extra, ", ")))
	}
	return strings.Join(parts, "\n")
}

func diffPlatformFieldMap(want map[string][]string, got map[string][]string) string {
	platforms := make(map[string]struct{}, len(want)+len(got))
	for platform := range want {
		platforms[platform] = struct{}{}
	}
	for platform := range got {
		platforms[platform] = struct{}{}
	}

	keys := make([]string, 0, len(platforms))
	for platform := range platforms {
		keys = append(keys, platform)
	}
	sort.Strings(keys)

	messages := make([]string, 0)
	for _, platform := range keys {
		wantValue, wantOK := want[platform]
		gotValue, gotOK := got[platform]
		switch {
		case !wantOK:
			messages = append(messages, fmt.Sprintf("unexpected platform %s", platform))
		case !gotOK:
			messages = append(messages, fmt.Sprintf("missing platform %s", platform))
		default:
			if diff := diffPlatformSets(wantValue, gotValue); diff != "" {
				messages = append(
					messages,
					fmt.Sprintf("%s onboarding paths drifted:\n%s", platform, indentDiff(diff)),
				)
			}
		}
	}

	return strings.Join(messages, "\n")
}

func parseSetupCellOnboardingPaths(t *testing.T, value string) []string {
	t.Helper()

	normalized := strings.ToLower(strings.TrimSpace(value))
	paths := make([]string, 0, 2)
	if strings.Contains(normalized, "install workspace") {
		paths = append(paths, "install-workspace")
	}
	if strings.Contains(normalized, "platform connections") {
		paths = append(paths, "platform-connections")
	}
	return requireNonEmptyPlatformList(t, paths, fmt.Sprintf("setup cell %q", value))
}

func validateManifestOnboardingPaths(t *testing.T, platform platformSupportManifestEntry) []string {
	t.Helper()

	switch platform.GovernanceState {
	case "supported", "admitted":
		return requireNonEmptyPlatformList(
			t,
			platform.OnboardingPaths,
			fmt.Sprintf("manifest onboarding paths for %s", platform.ID),
		)
	case "presentation-only":
		if len(platform.OnboardingPaths) != 0 {
			t.Fatalf(
				"presentation-only platform %s must not declare onboarding paths, got %v",
				platform.ID,
				platform.OnboardingPaths,
			)
		}
		return []string{}
	default:
		t.Fatalf("unexpected governance state %s for %s", platform.GovernanceState, platform.ID)
		return nil
	}
}

func indentDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}
