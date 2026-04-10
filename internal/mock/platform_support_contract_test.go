package mock

import (
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
	currentFirstClassPlatformRE = regexp.MustCompile("^\\d+\\.\\s+`([^`]+)`")
	currentSupportMatrixRowRE   = regexp.MustCompile("^\\|\\s+`([^`]+)`\\s+\\|")
)

func TestPlatformSupportModelCurrentClassificationMatchesSupportMatrix(t *testing.T) {
	model := loadPlatformSupportModel(t)

	classified := parseCurrentFirstClassPlatforms(t, model)
	matrix := parseCurrentSupportMatrixPlatforms(t, model)

	if diff := diffPlatformSets(classified, matrix); diff != "" {
		t.Fatalf("current supported platform definitions drifted between classification and support matrix:\n%s", diff)
	}
}

func TestMockCoverageMatchesCurrentSupportedPlatformSet(t *testing.T) {
	model := loadPlatformSupportModel(t)
	supported := parseCurrentFirstClassPlatforms(t, model)

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
	supported := parseCurrentFirstClassPlatforms(t, model)

	if containsPlatform(supported, "vmware-vsphere") {
		t.Fatal("vmware-vsphere must not appear in the current supported platform set before live proof admits it")
	}
	if !strings.Contains(model, "| `vmware-vsphere` |") {
		t.Fatal("expected platform support model to keep the vmware-vsphere admission row")
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

func parseCurrentFirstClassPlatforms(t *testing.T, model string) []string {
	t.Helper()

	var platforms []string
	inSection := false

	for _, raw := range strings.Split(model, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "### First-class platforms":
			inSection = true
			continue
		case !inSection:
			continue
		case strings.HasPrefix(line, "### ") || strings.HasPrefix(line, "## "):
			return requireNonEmptyPlatformList(t, platforms, "current first-class platform section")
		}

		matches := currentFirstClassPlatformRE.FindStringSubmatch(line)
		if len(matches) == 2 {
			platforms = append(platforms, matches[1])
		}
	}

	return requireNonEmptyPlatformList(t, platforms, "current first-class platform section")
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

		matches := currentSupportMatrixRowRE.FindStringSubmatch(line)
		if len(matches) == 2 {
			platforms = append(platforms, matches[1])
		}
	}

	return requireNonEmptyPlatformList(t, platforms, "current support matrix")
}

func requireNonEmptyPlatformList(t *testing.T, platforms []string, label string) []string {
	t.Helper()

	unique := uniqueSortedPlatforms(platforms)
	if len(unique) == 0 {
		t.Fatalf("expected %s to declare at least one platform", label)
	}
	return unique
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
