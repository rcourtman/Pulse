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

	"github.com/rcourtman/pulse-go-rewrite/internal/platformsupport"
)

var (
	numberedPlatformListRE     = regexp.MustCompile("^\\d+\\.\\s+`([^`]+)`")
	currentSupportMatrixRE     = regexp.MustCompile("^\\|\\s+`([^`]+)`\\s+\\|")
	currentSupportSetupRowRE   = regexp.MustCompile("^\\|\\s+`([^`]+)`\\s+\\|\\s+([^|]+?)\\s+\\|")
	currentSupportSurfaceRowRE = regexp.MustCompile("^\\|\\s+`([^`]+)`\\s+\\|\\s+`([^`]+)`\\s+\\|")
	agentHostProfileTableRowRE = regexp.MustCompile("^\\|\\s+`([^`]+)`\\s+\\|\\s+`([^`]+)`\\s+\\|\\s+`([^`]+)`\\s+\\|\\s+`([^`]+)`\\s+\\|\\s+(.+?)\\s+\\|\\s+`([^`]+)`\\s+\\|\\s+`([^`]+)`\\s+\\|")
)

type platformSupportManifest struct {
	SchemaVersion                    int                             `json:"schema_version"`
	DefaultInfrastructureSourceOrder []string                        `json:"default_infrastructure_source_order"`
	AgentHostProfiles                []agentHostProfileManifestEntry `json:"agent_host_profiles"`
	Platforms                        []platformSupportManifestEntry  `json:"platforms"`
}

type agentHostProfileManifestEntry struct {
	ID                 string            `json:"id"`
	Family             string            `json:"family"`
	GovernanceState    string            `json:"governance_state"`
	ReadinessStage     string            `json:"readiness_stage"`
	HostIdentityTokens []string          `json:"host_identity_tokens"`
	RuntimePlatform    string            `json:"runtime_platform"`
	SupportFloor       map[string]string `json:"support_floor"`
	StorageFamily      string            `json:"storage_family"`
}

type platformSupportManifestEntry struct {
	ID                   string            `json:"id"`
	SurfaceKind          string            `json:"surface_kind"`
	GovernanceState      string            `json:"governance_state"`
	ReadinessStage       string            `json:"readiness_stage"`
	PrimaryMode          string            `json:"primary_mode"`
	OnboardingPaths      []string          `json:"onboarding_paths"`
	CanonicalProjections []string          `json:"canonical_projections"`
	SupportFloor         map[string]string `json:"support_floor"`
	UILabel              string            `json:"ui_label"`
	UITone               string            `json:"ui_tone"`
	Aliases              []string          `json:"aliases"`
	DisplayTokens        []string          `json:"display_tokens"`
	StorageFamily        string            `json:"storage_family"`
}

func TestPlatformSupportManifestMatchesSupportModel(t *testing.T) {
	model := loadPlatformSupportModel(t)
	manifest := loadPlatformSupportManifest(t)

	firstClassPlatforms := parsePlatformListSection(t, model, "### First-class platforms")
	runtimeLenses := parsePlatformListSection(t, model, "### Runtime lenses")
	supportedSurfaces := uniqueSortedPlatforms(append(
		append([]string(nil), firstClassPlatforms...),
		runtimeLenses...,
	))
	admitted := parsePlatformListSection(t, model, "### Admitted platforms (not yet supported)")
	presentationOnly := parsePlatformListSection(t, model, "### Presentation-only platform vocabulary")
	agentHostProfiles := parseAgentHostProfileSection(t, model, "### Agent Host Profiles")
	matrix := parseCurrentSupportMatrixPlatforms(t, model)

	if diff := diffPlatformSets(supportedSurfaces, matrix); diff != "" {
		t.Fatalf("current supported platform definitions drifted between classification and support matrix:\n%s", diff)
	}

	if diff := diffPlatformSets(supportedSurfaces, manifestPlatformsByState(t, manifest, "supported")); diff != "" {
		t.Fatalf("supported platform manifest drifted from the canonical support model:\n%s", diff)
	}
	if diff := diffPlatformSets(
		firstClassPlatforms,
		manifestPlatformsBySurfaceKindAndState(t, manifest, "platform", "supported"),
	); diff != "" {
		t.Fatalf("supported platform surface-kind manifest drifted from the canonical support model:\n%s", diff)
	}
	if diff := diffPlatformSets(
		runtimeLenses,
		manifestPlatformsBySurfaceKindAndState(t, manifest, "runtime-lens", "supported"),
	); diff != "" {
		t.Fatalf("supported runtime-lens manifest drifted from the canonical support model:\n%s", diff)
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
	if diff := diffPlatformSets(
		presentationOnly,
		manifestPlatformsBySurfaceKindAndState(t, manifest, "presentation-only", "presentation-only"),
	); diff != "" {
		t.Fatalf("presentation-only surface-kind manifest drifted from the canonical support model:\n%s", diff)
	}
	if diff := diffPlatformFieldMap(
		parseCurrentSupportRowSurfaceKinds(t, model),
		manifestPlatformSurfaceKindsByState(t, manifest, "supported"),
	); diff != "" {
		t.Fatalf("supported surface-kind manifest drifted from the canonical support rows:\n%s", diff)
	}
	if diff := diffPlatformFieldMap(
		parseCurrentSupportMatrixOnboardingPaths(t, model),
		manifestPlatformOnboardingPathsByState(t, manifest, "supported"),
	); diff != "" {
		t.Fatalf("supported onboarding-path manifest drifted from the canonical support model:\n%s", diff)
	}
	if diff := diffPlatformSets(
		sortedAgentHostProfileIDs(agentHostProfiles),
		manifestAgentHostProfileIDsByState(t, manifest, "supported"),
	); diff != "" {
		t.Fatalf("agent host profile manifest drifted from the canonical support model:\n%s", diff)
	}
	if diff := diffPlatformFieldMap(
		agentHostProfileFieldMap(agentHostProfiles, func(profile agentHostProfileSectionEntry) []string {
			return []string{profile.Family}
		}),
		manifestAgentHostProfileFamiliesByState(t, manifest, "supported"),
	); diff != "" {
		t.Fatalf("agent host profile family drifted from the canonical support model:\n%s", diff)
	}
	if diff := diffPlatformFieldMap(
		agentHostProfileFieldMap(agentHostProfiles, func(profile agentHostProfileSectionEntry) []string {
			return []string{profile.GovernanceState}
		}),
		manifestAgentHostProfileGovernanceStatesByState(t, manifest, "supported"),
	); diff != "" {
		t.Fatalf("agent host profile governance drifted from the canonical support model:\n%s", diff)
	}
	if diff := diffPlatformFieldMap(
		agentHostProfileFieldMap(agentHostProfiles, func(profile agentHostProfileSectionEntry) []string {
			return []string{profile.ReadinessStage}
		}),
		manifestAgentHostProfileReadinessStagesByState(t, manifest, "supported"),
	); diff != "" {
		t.Fatalf("agent host profile readiness drifted from the canonical support model:\n%s", diff)
	}
	if diff := diffPlatformFieldMap(
		agentHostProfileFieldMap(agentHostProfiles, func(profile agentHostProfileSectionEntry) []string {
			return append([]string(nil), profile.HostIdentityTokens...)
		}),
		manifestAgentHostProfileHostIdentityTokensByState(t, manifest, "supported"),
	); diff != "" {
		t.Fatalf("agent host profile host-identity tokens drifted from the canonical support model:\n%s", diff)
	}
	if diff := diffPlatformFieldMap(
		agentHostProfileFieldMap(agentHostProfiles, func(profile agentHostProfileSectionEntry) []string {
			return []string{profile.RuntimePlatform}
		}),
		manifestAgentHostProfileRuntimePlatformsByState(t, manifest, "supported"),
	); diff != "" {
		t.Fatalf("agent host profile runtime-platform fallback drifted from the canonical support model:\n%s", diff)
	}
	if diff := diffPlatformFieldMap(
		agentHostProfileFieldMap(agentHostProfiles, func(profile agentHostProfileSectionEntry) []string {
			return []string{profile.StorageFamily}
		}),
		manifestAgentHostProfileStorageFamiliesByState(t, manifest, "supported"),
	); diff != "" {
		t.Fatalf("agent host profile storage-family drifted from the canonical support model:\n%s", diff)
	}
	defaultInfrastructureSources := uniqueSortedPlatforms(append(
		append([]string(nil), supportedSurfaces...),
		admitted...,
	))
	if diff := diffPlatformSets(defaultInfrastructureSources, manifest.DefaultInfrastructureSourceOrder); diff != "" {
		t.Fatalf("default infrastructure source ordering drifted from the canonical supported/admitted platform set:\n%s", diff)
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

func TestVMwareFixturesRemainAdmittedAtPhase1Floor(t *testing.T) {
	model := loadPlatformSupportModel(t)
	manifest := loadPlatformSupportManifest(t)
	supported := manifestPlatformsByState(t, manifest, "supported")
	admitted := manifestPlatformsByState(t, manifest, "admitted")
	presentationOnly := manifestPlatformsByState(t, manifest, "presentation-only")

	if containsPlatform(supported, "vmware-vsphere") {
		t.Fatal("vmware-vsphere must not appear in the current supported platform set before live vCenter proof")
	}
	if !containsPlatform(admitted, "vmware-vsphere") {
		t.Fatal("vmware-vsphere must remain in the admitted set until live vCenter proof promotes it")
	}
	if containsPlatform(presentationOnly, "vmware-vsphere") {
		t.Fatal("vmware-vsphere must not regress into presentation-only vocabulary while admitted")
	}
	if !strings.Contains(model, "| `vmware-vsphere` | VMware | `vCenter` only in phase 1 |") {
		t.Fatal("expected platform support model to keep the vmware-vsphere admission row")
	}
	vmwareOnboardingPaths, ok := manifestPlatformOnboardingPaths(manifest)["vmware-vsphere"]
	if !ok {
		t.Fatal("expected vmware-vsphere onboarding-path manifest entry")
	}
	if diff := diffPlatformSets([]string{"platform-connections"}, vmwareOnboardingPaths); diff != "" {
		t.Fatalf("vmware-vsphere onboarding path drifted from the support model:\n%s", diff)
	}
	vmwareManifest := requireManifestPlatform(t, manifest, "vmware-vsphere")
	if vmwareManifest.GovernanceState != "admitted" {
		t.Fatalf("vmware governance state = %q, want admitted", vmwareManifest.GovernanceState)
	}
	if vmwareManifest.ReadinessStage != "first-lab-ready" {
		t.Fatalf("vmware readiness stage = %q, want first-lab-ready", vmwareManifest.ReadinessStage)
	}
	if vmwareManifest.PrimaryMode != "api-backed" {
		t.Fatalf("vmware primary mode = %q, want api-backed", vmwareManifest.PrimaryMode)
	}
	if diff := diffPlatformSets([]string{"agent", "network", "storage", "vm"}, vmwareManifest.CanonicalProjections); diff != "" {
		t.Fatalf("vmware canonical projections drifted from the support model:\n%s", diff)
	}
	if got := vmwareManifest.SupportFloor["recovery"]; got != "n/a" {
		t.Fatalf("vmware recovery support floor = %q, want n/a", got)
	}
	if got := vmwareManifest.SupportFloor["assistant_control"]; got != "read-only" {
		t.Fatalf("vmware assistant control support floor = %q, want read-only", got)
	}

	graph := buildFixtureGraph(DefaultConfig, time.Date(2026, time.April, 10, 12, 0, 0, 0, time.UTC))
	assertVMwareMockCoverage(t, graph)

	fixture := DefaultVMwareConnectionFixture()
	if fixture.CollectedAt.IsZero() {
		t.Fatal("expected VMware mock connection fixture freshness")
	}
	if fixture.Hosts == 0 || fixture.VMs == 0 || fixture.Datastores == 0 || fixture.Networks == 0 {
		t.Fatalf("expected VMware mock connection fixture to describe canonical inventory, got %+v", fixture)
	}
}

func TestUnraidRemainsInPresentationOnlyVocabularyAndAgentHostProfiles(t *testing.T) {
	model := loadPlatformSupportModel(t)
	manifest := loadPlatformSupportManifest(t)
	supported := manifestPlatformsByState(t, manifest, "supported")
	admitted := manifestPlatformsByState(t, manifest, "admitted")
	presentationOnly := manifestPlatformsByState(t, manifest, "presentation-only")
	profiles := parseAgentHostProfileSection(t, model, "### Agent Host Profiles")

	if containsPlatform(supported, "unraid") {
		t.Fatal("unraid must not appear in the current supported platform set")
	}
	if containsPlatform(admitted, "unraid") {
		t.Fatal("unraid must not appear in the admitted platform set")
	}
	if !containsPlatform(presentationOnly, "unraid") {
		t.Fatal("expected unraid to remain presentation-only platform vocabulary")
	}
	if !containsPlatform(sortedAgentHostProfileIDs(profiles), "unraid") {
		t.Fatal("expected unraid to remain a supported agent host profile")
	}

	profile := requireAgentHostProfileManifestEntry(t, manifest, "unraid")
	if profile.GovernanceState != "supported" {
		t.Fatalf("unraid governance state = %q, want supported", profile.GovernanceState)
	}
	if profile.ReadinessStage != "supported" {
		t.Fatalf("unraid readiness stage = %q, want supported", profile.ReadinessStage)
	}
	if diff := diffPlatformSets([]string{"unraid", "unraid-os", "unraid os"}, profile.HostIdentityTokens); diff != "" {
		t.Fatalf("unraid host identity tokens drifted from the host-profile model:\n%s", diff)
	}
	if profile.RuntimePlatform != "linux" {
		t.Fatalf("unraid runtime platform = %q, want linux", profile.RuntimePlatform)
	}
	if profile.StorageFamily != "onprem" {
		t.Fatalf("unraid storage family = %q, want onprem", profile.StorageFamily)
	}
	if got := profile.SupportFloor["assistant_control"]; got != "read-only" {
		t.Fatalf("unraid assistant control support floor = %q, want read-only", got)
	}
	if got := profile.SupportFloor["storage"]; got != "supported" {
		t.Fatalf("unraid storage support floor = %q, want supported", got)
	}
	if !strings.Contains(model, "### Agent Host Profiles") {
		t.Fatal("expected platform support model to keep an agent host profiles section")
	}
	if !strings.Contains(
		model,
		"| `unraid` | `Unraid` | `supported` | `supported` | `unraid`, `unraid-os`, `unraid os` | `linux` | `onprem` |",
	) {
		t.Fatal("expected platform support model to keep the unraid host-profile row")
	}
}

func TestPlatformSupportBackendProjectionMatchesSupportManifest(t *testing.T) {
	manifest := loadPlatformSupportManifest(t)
	profiles := platformsupport.AgentHostProfiles()

	gotIDs := make([]string, 0, len(profiles))
	gotTokens := make(map[string][]string, len(profiles))
	gotRuntimePlatforms := make(map[string][]string, len(profiles))
	for _, profile := range profiles {
		if profile.GovernanceState != "supported" {
			continue
		}
		gotIDs = append(gotIDs, profile.ID)
		gotTokens[profile.ID] = append([]string(nil), profile.HostIdentityTokens...)
		gotRuntimePlatforms[profile.ID] = []string{profile.RuntimePlatform}
	}

	if diff := diffPlatformSets(manifestAgentHostProfileIDsByState(t, manifest, "supported"), gotIDs); diff != "" {
		t.Fatalf("backend agent host profile ids drifted from manifest:\n%s", diff)
	}
	if diff := diffPlatformFieldMap(
		manifestAgentHostProfileHostIdentityTokensByState(t, manifest, "supported"),
		gotTokens,
	); diff != "" {
		t.Fatalf("backend agent host profile identity tokens drifted from manifest:\n%s", diff)
	}
	if diff := diffPlatformFieldMap(
		manifestAgentHostProfileRuntimePlatformsByState(t, manifest, "supported"),
		gotRuntimePlatforms,
	); diff != "" {
		t.Fatalf("backend agent host profile runtime platforms drifted from manifest:\n%s", diff)
	}

	profile, ok := platformsupport.AgentHostProfileForIdentity("Unraid")
	if !ok {
		t.Fatal("expected backend projection to resolve Unraid identity token")
	}
	if profile.ID != "unraid" || profile.RuntimePlatform != "linux" {
		t.Fatalf("backend Unraid profile = %+v, want id=unraid runtime=linux", profile)
	}
	if got := platformsupport.NormalizeRuntimePlatformForAgentHostProfile("unraid", "unraid"); got != "linux" {
		t.Fatalf("backend runtime normalization = %q, want linux", got)
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
	imageCount := 0
	volumeCount := 0
	networkCount := 0
	storageUsageCount := 0
	swarmNodeCount := 0
	for _, host := range graph.State.DockerHosts {
		containerCount += len(host.Containers)
		imageCount += len(host.Images)
		volumeCount += len(host.Volumes)
		networkCount += len(host.Networks)
		swarmNodeCount += len(host.Nodes)
		if host.StorageUsage != nil {
			storageUsageCount++
		}
	}
	if containerCount == 0 {
		t.Fatal("expected docker container fixtures for the supported docker platform")
	}
	if imageCount == 0 || volumeCount == 0 || networkCount == 0 || storageUsageCount == 0 || swarmNodeCount == 0 {
		t.Fatalf(
			"expected docker fixtures to include images, volumes, networks, storage usage, and swarm nodes; got images=%d volumes=%d networks=%d storage=%d swarmNodes=%d",
			imageCount,
			volumeCount,
			networkCount,
			storageUsageCount,
			swarmNodeCount,
		)
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
	serviceCount := 0
	ingressCount := 0
	endpointSliceCount := 0
	storageClassCount := 0
	persistentVolumeCount := 0
	persistentVolumeClaimCount := 0
	for _, cluster := range graph.State.KubernetesClusters {
		nodeCount += len(cluster.Nodes)
		deploymentCount += len(cluster.Deployments)
		podCount += len(cluster.Pods)
		serviceCount += len(cluster.Services)
		ingressCount += len(cluster.Ingresses)
		endpointSliceCount += len(cluster.EndpointSlices)
		storageClassCount += len(cluster.StorageClasses)
		persistentVolumeCount += len(cluster.PersistentVolumes)
		persistentVolumeClaimCount += len(cluster.PersistentVolumeClaims)
	}
	if nodeCount == 0 || deploymentCount == 0 || podCount == 0 {
		t.Fatalf(
			"expected kubernetes fixtures to include nodes, deployments, and pods; got nodes=%d deployments=%d pods=%d",
			nodeCount,
			deploymentCount,
			podCount,
		)
	}
	if serviceCount == 0 || ingressCount == 0 || endpointSliceCount == 0 {
		t.Fatalf(
			"expected kubernetes fixtures to include services, ingresses, and endpoint slices; got services=%d ingresses=%d endpointSlices=%d",
			serviceCount,
			ingressCount,
			endpointSliceCount,
		)
	}
	if storageClassCount == 0 || persistentVolumeCount == 0 || persistentVolumeClaimCount == 0 {
		t.Fatalf(
			"expected kubernetes fixtures to include storage classes, persistent volumes, and persistent volume claims; got classes=%d pvs=%d pvcs=%d",
			storageClassCount,
			persistentVolumeCount,
			persistentVolumeClaimCount,
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

func assertVMwareMockCoverage(t *testing.T, graph FixtureGraph) {
	t.Helper()

	if len(graph.PlatformFixtures.VMware.Hosts) == 0 {
		t.Fatal("expected VMware mock fixtures to include hosts")
	}
	if len(graph.PlatformFixtures.VMware.VMs) == 0 {
		t.Fatal("expected VMware mock fixtures to include VMs")
	}
	if len(graph.PlatformFixtures.VMware.Datastores) == 0 {
		t.Fatal("expected VMware mock fixtures to include datastores")
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
	// The admitted-platform section is allowed to be empty once the last
	// admitted platform graduates to supported; every other section must
	// declare at least one platform.
	allowEmpty := strings.Contains(heading, "Admitted platforms")

	for _, raw := range strings.Split(model, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == heading:
			inSection = true
			continue
		case !inSection:
			continue
		case strings.HasPrefix(line, "### ") || strings.HasPrefix(line, "## "):
			if allowEmpty {
				return allowEmptyPlatformList(platforms)
			}
			return requireNonEmptyPlatformList(t, platforms, heading)
		}

		matches := numberedPlatformListRE.FindStringSubmatch(line)
		if len(matches) == 2 {
			platforms = append(platforms, matches[1])
		}
	}

	if allowEmpty {
		return allowEmptyPlatformList(platforms)
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

func parseCurrentSupportRowSurfaceKinds(t *testing.T, model string) map[string][]string {
	t.Helper()

	rows := make(map[string][]string)
	inRows := false

	for _, raw := range strings.Split(model, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "### Current support rows":
			inRows = true
			continue
		case !inRows:
			continue
		case strings.HasPrefix(line, "### ") || strings.HasPrefix(line, "## "):
			return requireNonEmptyPlatformFieldMap(t, rows, "current support row surface kinds")
		case line == "" || strings.HasPrefix(line, "| ---") || strings.HasPrefix(line, "| Platform"):
			continue
		}

		matches := currentSupportSurfaceRowRE.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		rows[matches[1]] = []string{matches[2]}
	}

	return requireNonEmptyPlatformFieldMap(t, rows, "current support row surface kinds")
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

	if governanceState == "admitted" {
		return allowEmptyPlatformList(platforms)
	}
	return requireNonEmptyPlatformList(t, platforms, fmt.Sprintf("manifest platforms with state %s", governanceState))
}

func manifestPlatformsBySurfaceKindAndState(
	t *testing.T,
	manifest platformSupportManifest,
	surfaceKind string,
	governanceState string,
) []string {
	t.Helper()

	platforms := make([]string, 0)
	for _, platform := range manifest.Platforms {
		if platform.GovernanceState == governanceState && platform.SurfaceKind == surfaceKind {
			platforms = append(platforms, platform.ID)
		}
	}

	if governanceState == "admitted" {
		return allowEmptyPlatformList(platforms)
	}
	return requireNonEmptyPlatformList(
		t,
		platforms,
		fmt.Sprintf("manifest platforms with surface kind %s and state %s", surfaceKind, governanceState),
	)
}

func manifestPlatformSurfaceKindsByState(
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
		if strings.TrimSpace(platform.SurfaceKind) == "" {
			t.Fatalf("expected platform support manifest to classify surface kind for %s", platform.ID)
		}
		rows[platform.ID] = []string{platform.SurfaceKind}
	}

	return requireNonEmptyPlatformFieldMap(
		t,
		rows,
		fmt.Sprintf("manifest surface kinds with state %s", governanceState),
	)
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

type agentHostProfileSectionEntry struct {
	Family             string
	GovernanceState    string
	ReadinessStage     string
	HostIdentityTokens []string
	RuntimePlatform    string
	StorageFamily      string
}

func parseAgentHostProfileSection(
	t *testing.T,
	model string,
	heading string,
) map[string]agentHostProfileSectionEntry {
	t.Helper()

	entries := make(map[string]agentHostProfileSectionEntry)
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
			return requireNonEmptyAgentHostProfileMap(t, entries, heading)
		case line == "" || strings.HasPrefix(line, "| ---"):
			continue
		}

		matches := agentHostProfileTableRowRE.FindStringSubmatch(line)
		if len(matches) != 8 {
			continue
		}

		entries[matches[1]] = agentHostProfileSectionEntry{
			Family:             matches[2],
			GovernanceState:    matches[3],
			ReadinessStage:     matches[4],
			HostIdentityTokens: parseInlineTokenList(matches[5]),
			RuntimePlatform:    matches[6],
			StorageFamily:      matches[7],
		}
	}

	return requireNonEmptyAgentHostProfileMap(t, entries, heading)
}

func parseInlineTokenList(value string) []string {
	tokens := make([]string, 0, 1)
	for _, token := range strings.Split(value, ",") {
		token = strings.TrimSpace(token)
		token = strings.Trim(token, "`")
		if token == "" {
			continue
		}
		tokens = append(tokens, token)
	}
	return tokens
}

func sortedAgentHostProfileIDs(profiles map[string]agentHostProfileSectionEntry) []string {
	ids := make([]string, 0, len(profiles))
	for id := range profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func agentHostProfileFieldMap(
	profiles map[string]agentHostProfileSectionEntry,
	selector func(agentHostProfileSectionEntry) []string,
) map[string][]string {
	rows := make(map[string][]string, len(profiles))
	for id, profile := range profiles {
		rows[id] = append([]string(nil), selector(profile)...)
	}
	return rows
}

func requireNonEmptyAgentHostProfileMap(
	t *testing.T,
	entries map[string]agentHostProfileSectionEntry,
	heading string,
) map[string]agentHostProfileSectionEntry {
	t.Helper()

	if len(entries) == 0 {
		t.Fatalf("expected %s to declare at least one agent host profile", heading)
	}
	return entries
}

func manifestAgentHostProfileIDsByState(
	t *testing.T,
	manifest platformSupportManifest,
	governanceState string,
) []string {
	t.Helper()

	ids := make([]string, 0)
	for _, profile := range manifest.AgentHostProfiles {
		if strings.TrimSpace(profile.ID) == "" {
			t.Fatal("expected agent host profile ids to be non-empty")
		}
		if strings.TrimSpace(profile.GovernanceState) == "" {
			t.Fatalf("expected agent host profile manifest to classify %s", profile.ID)
		}
		if profile.GovernanceState == governanceState {
			ids = append(ids, profile.ID)
		}
	}

	return requireNonEmptyPlatformList(
		t,
		ids,
		fmt.Sprintf("manifest agent host profiles with state %s", governanceState),
	)
}

func manifestAgentHostProfileFieldMap(
	t *testing.T,
	manifest platformSupportManifest,
	governanceState string,
	selector func(agentHostProfileManifestEntry) []string,
	label string,
) map[string][]string {
	t.Helper()

	rows := make(map[string][]string)
	for _, profile := range manifest.AgentHostProfiles {
		if profile.GovernanceState != governanceState {
			continue
		}
		rows[profile.ID] = append([]string(nil), selector(profile)...)
	}

	return requireNonEmptyPlatformFieldMap(t, rows, label)
}

func manifestAgentHostProfileFamiliesByState(
	t *testing.T,
	manifest platformSupportManifest,
	governanceState string,
) map[string][]string {
	return manifestAgentHostProfileFieldMap(
		t,
		manifest,
		governanceState,
		func(profile agentHostProfileManifestEntry) []string { return []string{profile.Family} },
		fmt.Sprintf("manifest agent host profile families with state %s", governanceState),
	)
}

func manifestAgentHostProfileGovernanceStatesByState(
	t *testing.T,
	manifest platformSupportManifest,
	governanceState string,
) map[string][]string {
	return manifestAgentHostProfileFieldMap(
		t,
		manifest,
		governanceState,
		func(profile agentHostProfileManifestEntry) []string { return []string{profile.GovernanceState} },
		fmt.Sprintf("manifest agent host profile governance with state %s", governanceState),
	)
}

func manifestAgentHostProfileReadinessStagesByState(
	t *testing.T,
	manifest platformSupportManifest,
	governanceState string,
) map[string][]string {
	return manifestAgentHostProfileFieldMap(
		t,
		manifest,
		governanceState,
		func(profile agentHostProfileManifestEntry) []string { return []string{profile.ReadinessStage} },
		fmt.Sprintf("manifest agent host profile readiness with state %s", governanceState),
	)
}

func manifestAgentHostProfileHostIdentityTokensByState(
	t *testing.T,
	manifest platformSupportManifest,
	governanceState string,
) map[string][]string {
	return manifestAgentHostProfileFieldMap(
		t,
		manifest,
		governanceState,
		func(profile agentHostProfileManifestEntry) []string {
			return append([]string(nil), profile.HostIdentityTokens...)
		},
		fmt.Sprintf("manifest agent host profile identity tokens with state %s", governanceState),
	)
}

func manifestAgentHostProfileRuntimePlatformsByState(
	t *testing.T,
	manifest platformSupportManifest,
	governanceState string,
) map[string][]string {
	return manifestAgentHostProfileFieldMap(
		t,
		manifest,
		governanceState,
		func(profile agentHostProfileManifestEntry) []string { return []string{profile.RuntimePlatform} },
		fmt.Sprintf("manifest agent host profile runtime platforms with state %s", governanceState),
	)
}

func manifestAgentHostProfileStorageFamiliesByState(
	t *testing.T,
	manifest platformSupportManifest,
	governanceState string,
) map[string][]string {
	return manifestAgentHostProfileFieldMap(
		t,
		manifest,
		governanceState,
		func(profile agentHostProfileManifestEntry) []string { return []string{profile.StorageFamily} },
		fmt.Sprintf("manifest agent host profile storage families with state %s", governanceState),
	)
}

func requireAgentHostProfileManifestEntry(
	t *testing.T,
	manifest platformSupportManifest,
	profileID string,
) agentHostProfileManifestEntry {
	t.Helper()

	for _, profile := range manifest.AgentHostProfiles {
		if profile.ID == profileID {
			return profile
		}
	}
	t.Fatalf("expected agent host profile manifest entry for %s", profileID)
	return agentHostProfileManifestEntry{}
}

func requireManifestPlatform(t *testing.T, manifest platformSupportManifest, platformID string) platformSupportManifestEntry {
	t.Helper()

	for _, platform := range manifest.Platforms {
		if platform.ID == platformID {
			return platform
		}
	}
	t.Fatalf("expected platform support manifest entry for %s", platformID)
	return platformSupportManifestEntry{}
}

func requireNonEmptyPlatformList(t *testing.T, platforms []string, label string) []string {
	t.Helper()

	unique := uniqueSortedPlatforms(platforms)
	if len(unique) == 0 {
		t.Fatalf("expected %s to declare at least one platform", label)
	}
	return unique
}

// allowEmptyPlatformList mirrors requireNonEmptyPlatformList but tolerates an
// empty result. Used for governance sets that can legitimately be empty
// (e.g. the admitted-platform staging area after the last admitted platform
// has been promoted to supported).
func allowEmptyPlatformList(platforms []string) []string {
	return uniqueSortedPlatforms(platforms)
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
					fmt.Sprintf("%s field values drifted:\n%s", platform, indentDiff(diff)),
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
