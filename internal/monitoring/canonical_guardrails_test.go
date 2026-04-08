package monitoring

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

var bannedSnapshotResourceAccessPatterns = []struct {
	re      *regexp.Regexp
	message string
}{
	{
		re:      regexp.MustCompile(`GetState\(\)\.(VMs|Containers|Nodes|Hosts|DockerHosts|PBSInstances|Storage|PhysicalDisks)\b`),
		message: "derive canonical monitoring resource views through ReadState-backed helpers instead of GetState() resource arrays",
	},
}

type guardrailSupplementalProvider struct {
	records []unifiedresources.IngestRecord
}

func (p guardrailSupplementalProvider) SupplementalRecords(*Monitor, string) []unifiedresources.IngestRecord {
	out := make([]unifiedresources.IngestRecord, len(p.records))
	copy(out, p.records)
	return out
}

func readMonitoringRuntimeFiles(t *testing.T) map[string]string {
	t.Helper()

	files := make(map[string]string)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read monitoring directory: %v", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		files[name] = string(data)
	}

	return files
}

func TestNoGetStateResourceArrayRegression(t *testing.T) {
	for name, content := range readMonitoringRuntimeFiles(t) {
		for _, pattern := range bannedSnapshotResourceAccessPatterns {
			if matches := pattern.re.FindAllStringIndex(content, -1); len(matches) > 0 {
				for _, match := range matches {
					line := 1 + strings.Count(content[:match[0]], "\n")
					t.Errorf("%s:%d: %s", name, line, pattern.message)
				}
			}
		}
	}
}

func TestGetStateRefreshesLiveAlertSnapshots(t *testing.T) {
	data, err := os.ReadFile("monitor.go")
	if err != nil {
		t.Fatalf("failed to read monitor.go: %v", err)
	}
	source := string(data)

	for _, snippet := range []string{
		"state := m.state.GetSnapshot()",
		"state.ActiveAlerts = m.activeAlertsSnapshot()",
		"state.RecentlyResolved = m.recentlyResolvedAlertsSnapshot()",
	} {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor.go must contain %q", snippet)
		}
	}
}

func TestMonitoredSystemUsageReadinessGuardrailsRemainCanonical(t *testing.T) {
	requiredSnippets := map[string][]string{
		"monitor.go": {
			"type MonitorSupplementalInventoryReadinessProvider interface {",
			"SupplementalInventoryReadyAt(m *Monitor, orgID string) (time.Time, bool)",
		},
		"monitored_system_usage.go": {
			"MonitoredSystemUsageUnavailableMonitorState",
			"MonitoredSystemUsageUnavailableSupplementalInventoryUnsettled",
			"MonitoredSystemUsageUnavailableSupplementalInventoryRebuildPending",
			"func (m *Monitor) MonitoredSystemUsage() MonitoredSystemUsageSnapshot {",
			"readyAt, settled := m.supplementalInventoryReadyAt(orgID)",
			"if !settled {",
			"UnavailableReason: MonitoredSystemUsageUnavailableSupplementalInventoryUnsettled,",
			"if freshness.IsZero() || freshness.Before(readyAt) {",
			"UnavailableReason: MonitoredSystemUsageUnavailableSupplementalInventoryRebuildPending,",
			"Count:     unifiedresources.MonitoredSystemCount(readState),",
			"Available: true,",
		},
		"truenas_poller.go": {
			"func (p *TrueNASPoller) SupplementalInventoryReadyAt(_ *Monitor, orgID string) (time.Time, bool) {",
			"configs := p.activeConnectionConfigsForOrg(orgID)",
			"if status == nil || status.lastAttemptAt.IsZero() {",
		},
		"vmware_poller.go": {
			"func (p *VMwarePoller) SupplementalInventoryReadyAt(_ *Monitor, orgID string) (time.Time, bool) {",
			"configs := p.activeConnectionConfigsForOrg(orgID)",
			"if status == nil || status.lastAttemptAt.IsZero() {",
		},
	}

	for file, snippets := range requiredSnippets {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}
		source := string(data)
		for _, snippet := range snippets {
			if !strings.Contains(source, snippet) {
				t.Fatalf("%s must contain %q", file, snippet)
			}
		}
	}
}

func TestGuestMemoryFallbackUsesCanonicalLowTrustSelector(t *testing.T) {
	data, err := os.ReadFile("guest_memory_sources.go")
	if err != nil {
		t.Fatalf("failed to read guest_memory_sources.go: %v", err)
	}
	source := string(data)

	for _, snippet := range []string{
		"func effectiveGuestFreeMemTotal(memTotal uint64, status *proxmox.VMStatus) uint64 {",
		"if status.Balloon > 0 && status.Balloon <= memTotal && status.FreeMem <= status.Balloon {",
		"func selectGuestLowTrustUsedMemory(memTotal uint64, status *proxmox.VMStatus) (uint64, string) {",
		"if statusMemPlusFree > freeMemTotal+guestStatusMemoryMismatchTolerance {",
	} {
		if !strings.Contains(source, snippet) {
			t.Fatalf("guest_memory_sources.go must contain %q", snippet)
		}
	}
}

func TestProxmoxGuestMemoryFallbackUsesInstanceScopedCachesAndAgentMeminfo(t *testing.T) {
	requiredSnippets := map[string][]string{
		"guest_memory_agent.go": {
			"func guestMemoryCacheKey(instanceName, node string, vmid int) string {",
			"return fmt.Sprintf(\"%s/%s/%d\", instanceName, node, vmid)",
			"func (m *Monitor) getVMAgentMemAvailable(ctx context.Context, client PVEClientInterface, instanceName, node string, vmid int) (uint64, error) {",
			"requestCtx, cancel := context.WithTimeout(ctx, vmAgentMemRequestTTL)",
			"ttl := vmAgentMemCacheTTL",
			"ttl = vmAgentMemNegativeTTL",
		},
		"guest_memory_sources.go": {
			"if rrdAvailable, rrdErr := m.getVMRRDMetrics(ctx, client, instanceName, node, vmid); rrdErr == nil && rrdAvailable > 0 {",
			"if agentAvailable, agentErr := m.getVMAgentMemAvailable(ctx, client, instanceName, node, vmid); agentErr == nil && agentAvailable > 0 {",
			`memorySource = "guest-agent-meminfo"`,
			"guestRaw.GuestAgentMemAvailable = memAvailable",
		},
		"monitor.go": {
			"func (m *Monitor) getVMRRDMetrics(ctx context.Context, client PVEClientInterface, instanceName, node string, vmid int) (uint64, error) {",
			"cacheKey := guestMemoryCacheKey(instanceName, node, vmid)",
			"vmAgentMemCache            map[string]agentMemCacheEntry",
		},
		"monitor_agents.go": {
			"for key, entry := range m.vmAgentMemCache {",
			"if now.Sub(entry.fetchedAt) > vmAgentMemCleanupMaxAge {",
		},
	}

	for file, snippets := range requiredSnippets {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}
		source := string(data)
		for _, snippet := range snippets {
			if !strings.Contains(source, snippet) {
				t.Fatalf("%s must contain %q", file, snippet)
			}
		}
	}
}

func TestProxmoxGuestMemoryCarryForwardUsesCanonicalSnapshotStability(t *testing.T) {
	requiredSnippets := map[string][]string{
		"guest_memory_stability.go": {
			"func (m *Monitor) previousGuestSnapshot(instance, guestType, node string, vmid int) *GuestMemorySnapshot {",
			"func stabilizeGuestLowTrustMemory(",
			"switch CanonicalMemorySource(source) {",
			`"preserved-previous-memory-after-repeated-low-trust-pattern"`,
			`"preserved-previous-memory-for-healthy-guest-low-trust-full-usage"`,
		},
		"monitor_pve_guest_builders.go": {
			`prevSnapshot := m.previousGuestSnapshot(instanceName, "qemu", res.Node, res.VMID)`,
			"state.memUsed, state.memorySource, snapshotNotes = stabilizeGuestLowTrustMemory(",
		},
		"monitor_pve_guest_poll.go": {
			"Notes:          snapshotNotes,",
		},
		"monitor_polling_vm.go": {
			`prevSnapshot := m.previousGuestSnapshot(instanceName, "qemu", n.Node, vm.VMID)`,
			"memUsed, memorySource, snapshotNotes := stabilizeGuestLowTrustMemory(",
			"Notes:          snapshotNotes,",
		},
	}

	for file, snippets := range requiredSnippets {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}
		source := string(data)
		for _, snippet := range snippets {
			if !strings.Contains(source, snippet) {
				t.Fatalf("%s must contain %q", file, snippet)
			}
		}
	}
}

func TestProxmoxGuestDiskCarryForwardUsesCanonicalStabilityHelper(t *testing.T) {
	requiredSnippets := map[string][]string{
		"guest_disk_stability.go": {
			"func stabilizeGuestLowTrustDisk(",
			"func classifyGuestAgentDiskStatusError(err error) string {",
			`return total, used, free, prev.Disk.Usage, cloneGuestDisks(prev.Disks), "prev-" + diskStatusReason`,
		},
		"monitor_pve_guest_builders.go": {
			"state.diskTotal, state.diskUsed, state.diskFree, state.diskUsage, state.individualDisks, state.diskStatusReason = stabilizeGuestLowTrustDisk(",
			"DiskStatusReason:  state.diskStatusReason,",
			"return nil, classifyGuestAgentDiskStatusError(err), false",
		},
		"monitor_polling_vm.go": {
			"diskStatusReason = classifyGuestAgentDiskStatusError(err)",
			"diskTotal, diskUsed, diskFree, diskUsage, individualDisks, diskStatusReason = stabilizeGuestLowTrustDisk(",
		},
	}

	for file, snippets := range requiredSnippets {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}
		source := string(data)
		for _, snippet := range snippets {
			if !strings.Contains(source, snippet) {
				t.Fatalf("%s must contain %q", file, snippet)
			}
		}
	}
}

func TestStoragePollingUsesCanonicalPoolMetadataForZFSAttachment(t *testing.T) {
	data, err := os.ReadFile("monitor_polling_storage.go")
	if err != nil {
		t.Fatalf("failed to read monitor_polling_storage.go: %v", err)
	}
	source := string(data)

	for _, snippet := range []string{
		"func matchZFSPoolForStorage(storage models.Storage, zfsPoolMap map[string]*models.ZFSPool) *models.ZFSPool {",
		"storage.Pool,",
		"Pool:     storage.Pool,",
		"if modelStorage.Pool == \"\" && clusterConfig.Pool != \"\" {",
		"if pool := matchZFSPoolForStorage(modelStorage, zfsPoolMap); pool != nil {",
	} {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor_polling_storage.go must contain %q", snippet)
		}
	}
}

func TestMonitoringTemperatureFallbackUsesSMARTAwareSSHSkipRule(t *testing.T) {
	requiredSnippets := map[string][]string{
		"host_agent_temps.go": {
			"func shouldSkipTemperatureSSHCollection(hostAgentTemp *models.Temperature) bool {",
			"return hostAgentTemp.HasSMART",
		},
		"monitor_polling_node_helpers.go": {
			"skipSSHCollection := shouldSkipTemperatureSSHCollection(hostAgentTemp)",
		},
	}

	for file, snippets := range requiredSnippets {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}
		source := string(data)
		for _, snippet := range snippets {
			if !strings.Contains(source, snippet) {
				t.Fatalf("%s must contain %q", file, snippet)
			}
		}
	}
}

func TestMonitoringRuntimeAvoidsLegacyMockPartialHelpers(t *testing.T) {
	forbiddenSnippets := []string{
		"mock.GetMockState(",
		"mock.GetPlatformFixtures(",
		"mock.GetMockRecoveryPoints(",
	}

	for name, content := range readMonitoringRuntimeFiles(t) {
		for _, snippet := range forbiddenSnippets {
			if strings.Contains(content, snippet) {
				t.Fatalf("%s must not depend on legacy mock partial helper %q", name, snippet)
			}
		}
	}
}

func TestMockOwnedUnifiedMetricSyncDefersToCanonicalSamplerInMockMode(t *testing.T) {
	previous := mock.IsMockEnabled()
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(previous) })

	if !shouldSkipMockOwnedUnifiedMetricSync(unifiedresources.Resource{
		Sources: []unifiedresources.DataSource{unifiedresources.SourceTrueNAS},
	}) {
		t.Fatal("expected TrueNAS mock-owned resources to skip generic unified metric sync")
	}
	if !shouldSkipMockOwnedUnifiedMetricSync(unifiedresources.Resource{
		Sources: []unifiedresources.DataSource{unifiedresources.SourceVMware},
	}) {
		t.Fatal("expected VMware mock-owned resources to skip generic unified metric sync")
	}
	if !shouldSkipMockOwnedUnifiedMetricSync(unifiedresources.Resource{
		Sources: []unifiedresources.DataSource{unifiedresources.SourceDocker},
	}) {
		t.Fatal("expected all mock-owned resources to defer to the canonical mock sampler")
	}

	mock.SetEnabled(false)
	if shouldSkipMockOwnedUnifiedMetricSync(unifiedresources.Resource{
		Sources: []unifiedresources.DataSource{unifiedresources.SourceDocker},
	}) {
		t.Fatal("expected unified metric sync to remain available outside mock mode")
	}
}

func TestConnectedInfrastructureUsesSharedTopLevelSystemResolver(t *testing.T) {
	data, err := os.ReadFile("connected_infrastructure.go")
	if err != nil {
		t.Fatalf("failed to read connected_infrastructure.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"resolver := unifiedresources.ResolveTopLevelSystems(resources)",
		"key := resolver.GroupIDForResource(resource)",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("connected_infrastructure.go must contain %q", snippet)
		}
	}
}

func TestConnectedInfrastructureKeepsPlatformConnectionsAndProjectsTrueNAS(t *testing.T) {
	data, err := os.ReadFile("connected_infrastructure.go")
	if err != nil {
		t.Fatalf("failed to read connected_infrastructure.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		`if agentSurface, ok := group.surfaces["agent"]; ok {`,
		`if dockerSurface, ok := group.surfaces["docker"]; ok {`,
		`if kubernetesSurface, ok := group.surfaces["kubernetes"]; ok {`,
		`if surface, ok := connectedInfrastructureTrueNASSurface(resource); ok {`,
		`Kind:    "truenas",`,
		`case "truenas":`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("connected_infrastructure.go must contain %q", snippet)
		}
	}

	for _, forbidden := range []string{
		`delete(group.surfaces, "proxmox")`,
		`delete(group.surfaces, "pbs")`,
		`delete(group.surfaces, "pmg")`,
		`delete(group.surfaces, "truenas")`,
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("connected_infrastructure.go must not remove platform-managed surfaces with %q", forbidden)
		}
	}
}

func TestVMwarePollerUsesCanonicalSupplementalIngestOwnership(t *testing.T) {
	data, err := os.ReadFile("vmware_poller.go")
	if err != nil {
		t.Fatalf("failed to read vmware_poller.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"func (p *VMwarePoller) SupplementalRecords(_ *Monitor, orgID string) []unifiedresources.IngestRecord {",
		"func (p *VMwarePoller) SnapshotOwnedSources() []unifiedresources.DataSource {",
		"return []unifiedresources.DataSource{unifiedresources.SourceVMware}",
		"return vmware.NewAPIProvider(vmware.ProviderMetadata{",
		"func (p *VMwarePoller) ConnectionSummaries(orgID string, instances []config.VMwareVCenterInstance) map[string]VMwareConnectionSummary {",
		"func (p *VMwarePoller) RecordConnectionTestSuccess(orgID, connID string, summary *vmware.InventorySummary, at time.Time) {",
		"func (p *VMwarePoller) RecordConnectionTestFailure(orgID, connID string, err error, at time.Time) {",
		"summary.Degraded = true",
		"summary.IssueCount = len(snapshot.EnrichmentIssues)",
		"summary.Issues = summarizeVMwareObservedIssues(snapshot.EnrichmentIssues)",
		"observedIssueKey    string",
		"transition := p.recordConnectionSuccessLocked(entry.orgID, entry.connectionID, at, snapshot)",
		"classifyVMwareObservedTransition(status.observed, status.observedIssueKey, nextObserved, nextIssueKey)",
		"status.observedIssueKey = nextIssueKey",
		`strings.HasPrefix(transition.action, "refresh_partial")`,
		`action:     "refresh_partial_changed",`,
		`action:  "refresh_recovered",`,
		"func summarizeVMwareObservedIssueKey(issues []vmware.InventoryEnrichmentIssue) string {",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("vmware_poller.go must contain %q", snippet)
		}
	}
}

func TestMonitorAlertTimelineWritesCanonicalAlertMetadata(t *testing.T) {
	data, err := os.ReadFile("monitor_alerts.go")
	if err != nil {
		t.Fatalf("failed to read monitor_alerts.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"change := unifiedresources.BuildAlertTimelineChange(",
		"AlertMetadata:   alert.Metadata,",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor_alerts.go must contain %q", snippet)
		}
	}
}

func TestLegacyMemorySourceAliasesRemainCanonicalized(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source    string
		canonical string
	}{
		{source: "avail-field", canonical: "available-field"},
		{source: "meminfo-available", canonical: "available-field"},
		{source: "guest-agent-meminfo", canonical: "guest-agent-meminfo"},
		{source: "node-status-available", canonical: "available-field"},
		{source: "meminfo-derived", canonical: "derived-free-buffers-cached"},
		{source: "calculated", canonical: "derived-free-buffers-cached"},
		{source: "meminfo-total-minus-used", canonical: "derived-total-minus-used"},
		{source: "rrd-available", canonical: "rrd-memavailable"},
		{source: "rrd-data", canonical: "rrd-memused"},
		{source: "listing-mem", canonical: "cluster-resources"},
		{source: "listing", canonical: "cluster-resources"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.source, func(t *testing.T) {
			t.Parallel()
			if got := CanonicalMemorySource(tt.source); got != tt.canonical {
				t.Fatalf("CanonicalMemorySource(%q) = %q, want %q", tt.source, got, tt.canonical)
			}
		})
	}
}

func TestProxmoxNodeDiskUsesCanonicalResolver(t *testing.T) {
	data, err := os.ReadFile("monitor_polling_node.go")
	if err != nil {
		t.Fatalf("failed to read monitor_polling_node.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"var nodeDiskSource string",
		"modelNode.Disk, nodeDiskSource = m.resolveNodeDisk(",
		"if resolvedDisk, diskSource := m.resolveNodeDisk(",
		"return modelNode, effectiveStatus, nodeDiskSource, nil",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor_polling_node.go must contain %q", snippet)
		}
	}
}

func TestDockerReportPreservesMetadataAcrossObservedContainerRecreation(t *testing.T) {
	migrationData, err := os.ReadFile("docker_metadata_migration.go")
	if err != nil {
		t.Fatalf("failed to read docker_metadata_migration.go: %v", err)
	}
	migrationSource := string(migrationData)
	requiredMigrationSnippets := []string{
		"func (m *Monitor) migrateDockerContainerMetadataForRecreatedContainers(",
		"normalizeDockerContainerMetadataIdentity(container.Name)",
		`strings.TrimSpace(strings.TrimPrefix(name, "/"))`,
		"m.CopyDockerContainerMetadata(hostID, previousContainer.ID, container.ID)",
	}
	for _, snippet := range requiredMigrationSnippets {
		if !strings.Contains(migrationSource, snippet) {
			t.Fatalf("docker_metadata_migration.go must contain %q", snippet)
		}
	}

	agentsData, err := os.ReadFile("monitor_agents.go")
	if err != nil {
		t.Fatalf("failed to read monitor_agents.go: %v", err)
	}
	agentsSource := string(agentsData)
	requiredAgentsSnippet := "m.migrateDockerContainerMetadataForRecreatedContainers(identifier, previous.Containers(), host.Containers)"
	if !strings.Contains(agentsSource, requiredAgentsSnippet) {
		t.Fatalf("monitor_agents.go must contain %q", requiredAgentsSnippet)
	}
}

func TestProxmoxNodeDiskFallbackPrefersCanonicalSystemStorage(t *testing.T) {
	requiredSnippets := map[string][]string{
		"node_disk_sources.go": {
			"func preferredNodeDiskFallbackRank(storage proxmox.Storage) (int, bool) {",
			`case "local-zfs":`,
			`case "local-lvm":`,
			`case "local":`,
			`supportsGuestRoots := storageContentIncludes(storage.Content, "images") || storageContentIncludes(storage.Content, "rootdir")`,
			"if storage.Shared != 0 {",
		},
		"monitor_pve.go": {
			"modelNodes, nodeEffectiveStatus, nodeDiskSources := m.pollPVENodesParallel(",
			"modelNodes = m.applyStorageFallbackAndRecordNodeMetrics(instanceName, client, modelNodes, nodeDiskSources, localStorageByNode)",
		},
		"monitor_pve_storage.go": {
			"rank, ok := preferredNodeDiskFallbackRank(storage)",
			`(modelNodes[i].Disk.Total == 0 || currentDiskSource == "" || currentDiskSource == "nodes-endpoint")`,
		},
	}

	for file, snippets := range requiredSnippets {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}
		source := string(data)
		for _, snippet := range snippets {
			if !strings.Contains(source, snippet) {
				t.Fatalf("%s must contain %q", file, snippet)
			}
		}
	}
}

func TestProxmoxGuestPollersCarryPoolIntoCanonicalModels(t *testing.T) {
	requiredSnippets := map[string][]string{
		"monitor_pve_guest_builders.go": {"Pool:     strings.TrimSpace(res.Pool)"},
		"monitor_pve_guest_lxc.go":      {"Pool:     strings.TrimSpace(res.Pool)"},
		"monitor_polling_vm.go":         {"Pool:     strings.TrimSpace(vm.Pool)"},
		"monitor_polling_containers.go": {"Pool:     strings.TrimSpace(container.Pool)"},
	}

	for file, snippets := range requiredSnippets {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}
		source := string(data)
		for _, snippet := range snippets {
			if !strings.Contains(source, snippet) {
				t.Fatalf("%s must contain %q", file, snippet)
			}
		}
	}
}

func TestProxmoxGuestAgentContinuityUsesCanonicalEvidenceAndRetryPaths(t *testing.T) {
	requiredSnippets := map[string][]string{
		"guest_agent_evidence.go": {
			"func hasRecentGuestAgentEvidence(prev *models.VM, now time.Time) bool {",
			`if prev == nil || prev.Type != "qemu" {`,
			"func shouldQueryGuestAgent(vmStatus *proxmox.VMStatus, prev *models.VM, now time.Time) bool {",
			"return hasRecentGuestAgentEvidence(prev, now)",
		},
		"guest_metadata.go": {
			"guestMetadataEmptyTTL    = 30 * time.Second",
			"func guestMetadataCacheEntryTTL(entry guestMetadataCacheEntry) time.Duration {",
			"if guestMetadataCacheHasCompleteNetworkData(entry) {",
			"return guestMetadataEmptyTTL",
			"func (m *Monitor) hasRecentGuestMetadataEvidence(instanceName, nodeName string, vmid int, now time.Time) bool {",
			"func (m *Monitor) scheduleGuestMetadataFetchForEntry(key string, now time.Time, entry guestMetadataCacheEntry) {",
		},
		"monitor_previous_state.go": {
			"vmsByID            map[string]models.VM",
			"ctx.vmsByID[modelVM.ID] = modelVM",
			"guestID := makeGuestID(modelVM.Instance, modelVM.Node, modelVM.VMID)",
			"ctx.vmsByID[guestID] = modelVM",
		},
		"monitor_pve_guest_builders.go": {
			"guestAgentAvailable := shouldQueryGuestAgent(state.detailedStatus, prevVM, now) ||",
			"m.hasRecentGuestMetadataEvidence(instanceName, res.Node, res.VMID, now)",
			"if guestAgentAvailable && state.detailedStatus == nil {",
		},
		"monitor_polling_vm.go": {
			"prevVMByID := prevGuests.vmsByID",
			"guestAgentAvailable := vm.Status == \"running\" &&",
			"m.hasRecentGuestMetadataEvidence(instanceName, n.Node, vm.VMID, now)",
			"if guestAgentAvailable && diskTotal > 0 {",
		},
	}

	for file, snippets := range requiredSnippets {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}
		source := string(data)
		for _, snippet := range snippets {
			if !strings.Contains(source, snippet) {
				t.Fatalf("%s must contain %q", file, snippet)
			}
		}
	}
}

func TestUnifiedAppContainerMetricsUseCanonicalGuestHistoryPath(t *testing.T) {
	data, err := os.ReadFile("monitor.go")
	if err != nil {
		t.Fatalf("failed to read monitor.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"m.syncUnifiedAppContainerMetrics(store)",
		`if target == nil || target.ResourceType != "app-container" || strings.TrimSpace(target.ResourceID) == "" {`,
		`metricKey := fmt.Sprintf("docker:%s", targetID)`,
		`m.metricsStore.Write("dockerContainer", targetID, "cpu", value, now)`,
		`m.metricsStore.Write("dockerContainer", targetID, "diskwrite", metric.Value, now)`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor.go must contain %q", snippet)
		}
	}
}

func TestMockUnifiedStateViewUsesCanonicalMockFixtureGraph(t *testing.T) {
	data, err := os.ReadFile("monitor.go")
	if err != nil {
		t.Fatalf("failed to read monitor.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"resources, freshness := mock.UnifiedResourceSnapshot()",
		"return monitorUnifiedStateViewFromResources(resources, freshness)",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor.go must contain %q", snippet)
		}
	}
}

func TestUnifiedAgentMetricsUseCanonicalHostHistoryPath(t *testing.T) {
	data, err := os.ReadFile("monitor.go")
	if err != nil {
		t.Fatalf("failed to read monitor.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"m.syncUnifiedAgentMetrics(store)",
		`if target == nil || target.ResourceType != "agent" || strings.TrimSpace(target.ResourceID) == "" {`,
		`metricKey := fmt.Sprintf("agent:%s", targetID)`,
		`m.metricsStore.Write("agent", targetID, "cpu", value, now)`,
		`m.metricsStore.Write("agent", targetID, "diskwrite", metric.Value, now)`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor.go must contain %q", snippet)
		}
	}
}

func TestUnifiedVMMetricsUseCanonicalVMHistoryPath(t *testing.T) {
	data, err := os.ReadFile("monitor.go")
	if err != nil {
		t.Fatalf("failed to read monitor.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"m.syncUnifiedVMMetrics(store)",
		`if target == nil || target.ResourceType != "vm" || strings.TrimSpace(target.ResourceID) == "" {`,
		`if source == unifiedresources.SourceProxmox {`,
		`m.metricsStore.Write("vm", targetID, "cpu", value, now)`,
		`m.metricsStore.Write("vm", targetID, "diskwrite", metric.Value, now)`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor.go must contain %q", snippet)
		}
	}
}

func TestHostPhysicalDiskIOMetricsUseCanonicalDiskHistoryPath(t *testing.T) {
	data, err := os.ReadFile("monitor_agents.go")
	if err != nil {
		t.Fatalf("failed to read monitor_agents.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"m.writeHostPhysicalDiskIOMetrics(host, now)",
		"func (m *Monitor) writeHostPhysicalDiskIOMetrics(host models.Host, now time.Time) {",
		`resourceID := unifiedresources.HostSMARTDiskSourceID(host, disk)`,
		`m.metricsHistory.AddDiskMetric(resourceID, "diskread", readRate, now)`,
		`m.metricsStore.Write("disk", resourceID, "diskwrite", writeRate, now)`,
		`m.metricsStore.Write("disk", resourceID, "disk", busyPct, now)`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor_agents.go must contain %q", snippet)
		}
	}
}

func TestMockNativePollersDeferToCanonicalMockSampler(t *testing.T) {
	cases := []struct {
		file     string
		snippets []string
	}{
		{
			file: "monitor.go",
			snippets: []string{
				"func shouldSkipNativeMockStateMetricWrites() bool {",
				"return mock.IsMockEnabled()",
			},
		},
		{
			file: "monitor.go",
			snippets: []string{
				"func keepRealPollingInMockMode() bool {",
				"case \"\", \"0\", \"false\", \"no\", \"off\":",
				"return false",
			},
		},
		{
			file: "monitor.go",
			snippets: []string{
				"func shouldSkipMockOwnedUnifiedMetricSync(resource unifiedresources.Resource) bool {",
				"if !mock.IsMockEnabled() {",
				"return true",
			},
		},
		{
			file: "monitor_polling_vm.go",
			snippets: []string{
				"if !shouldSkipNativeMockStateMetricWrites() {",
				`m.metricsStore.Write("vm", vm.ID, "cpu", vm.CPU*100, now)`,
			},
		},
		{
			file: "monitor_polling_containers.go",
			snippets: []string{
				"if !shouldSkipNativeMockStateMetricWrites() {",
				`m.metricsStore.Write("container", ct.ID, "cpu", ct.CPU*100, now)`,
			},
		},
		{
			file: "monitor_pve_guest_helpers.go",
			snippets: []string{
				"func (m *Monitor) recordGuestMetric(",
				"if m == nil || shouldSkipNativeMockStateMetricWrites() {",
				`m.metricsStore.Write(resourceType, resourceID, "memory", memory, now)`,
			},
		},
		{
			file: "kubernetes_agents.go",
			snippets: []string{
				"if shouldSkipNativeMockStateMetricWrites() {",
				`m.metricsStore.Write("k8s", metricID, "cpu", pod.UsageCPUPercent, timestamp)`,
			},
		},
		{
			file: "monitor_polling_storage.go",
			snippets: []string{
				"if m.metricsHistory != nil && !shouldSkipNativeMockStateMetricWrites() {",
				`m.metricsStore.Write("storage", storage.ID, "usage", storage.Usage, timestamp)`,
			},
		},
		{
			file: "monitor_metrics.go",
			snippets: []string{
				"func (m *Monitor) nativePhysicalDiskTemperatureHistory(duration time.Duration) map[string][]MetricPoint {",
				"if mock.IsMockEnabled() {",
				"return nil",
			},
		},
		{
			file: "mock_chart_history.go",
			snippets: []string{
				"func mockCanonicalMetricSeries(resourceType, resourceID, metricType string, timestamps []time.Time) []MetricPoint {",
				"values := canonicalMetricSeries(resourceType, resourceID, metricType, timestamps)",
				"return lttb(points, chartDownsampleTarget)",
			},
		},
		{
			file: "mock_metrics_history.go",
			snippets: []string{
				`cpu := mock.SampleMetric("k8s", metricID, "cpu", ts)`,
				`memory := mock.SampleMetric("k8s", metricID, "memory", ts)`,
				`ms.Write("k8s", metricID, "memory", memory, ts)`,
			},
		},
		{
			file: "monitor_agents.go",
			snippets: []string{
				"if !shouldSkipNativeMockStateMetricWrites() {",
				`m.metricsStore.Write("dockerHost", host.ID, "cpu", host.CPUUsage, now)`,
				`m.metricsStore.Write("agent", host.ID, "cpu", host.CPUUsage, now)`,
			},
		},
		{
			file: "monitor.go",
			snippets: []string{
				"func (m *Monitor) writeSMARTMetrics(disk models.PhysicalDisk, now time.Time) {",
				"if shouldSkipNativeMockStateMetricWrites() {",
				"return",
			},
		},
	}

	for _, tc := range cases {
		data, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", tc.file, err)
		}
		source := string(data)
		for _, snippet := range tc.snippets {
			if !strings.Contains(source, snippet) {
				t.Fatalf("%s must contain %q", tc.file, snippet)
			}
		}
	}
}

func TestUnifiedPhysicalDiskMetricsUseCanonicalDiskHistoryPath(t *testing.T) {
	data, err := os.ReadFile("monitor.go")
	if err != nil {
		t.Fatalf("failed to read monitor.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"m.syncUnifiedPhysicalDiskMetrics(store)",
		`if target == nil || target.ResourceType != "disk" || strings.TrimSpace(target.ResourceID) == "" {`,
		`if source == unifiedresources.SourceProxmox || source == unifiedresources.SourceAgent {`,
		`m.writeSMARTMetrics(disk, now)`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor.go must contain %q", snippet)
		}
	}
}

func TestProxmoxDiskAlertsRunOnMergedDiskState(t *testing.T) {
	cases := []struct {
		file     string
		snippets []string
	}{
		{
			file: "monitor.go",
			snippets: []string{
				"func mergeHostAgentSMARTIntoDisks(disks []models.PhysicalDisk, nodes []models.Node, hosts []models.Host) []models.PhysicalDisk {",
				"deriveWearoutFromSMARTAttributes(matched.Attributes)",
				`strings.EqualFold(updated[i].Health, "unknown")`,
			},
		},
		{
			file: "monitor_pve.go",
			snippets: []string{
				"allDisks = mergeHostAgentSMARTIntoDisks(allDisks, nodesFromState, hosts)",
				"m.alertManager.CheckDiskHealth(inst, disk.Node, proxmoxDiskFromPhysicalDisk(disk))",
				"func proxmoxDiskFromPhysicalDisk(disk models.PhysicalDisk) proxmox.Disk {",
			},
		},
	}

	for _, tc := range cases {
		data, err := os.ReadFile(tc.file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", tc.file, err)
		}
		source := string(data)
		for _, snippet := range tc.snippets {
			if !strings.Contains(source, snippet) {
				t.Fatalf("%s must contain %q", tc.file, snippet)
			}
		}
	}
}

func TestUnifiedPhysicalDiskMetricsAllowNativeHistoryProviders(t *testing.T) {
	monitorData, err := os.ReadFile("monitor.go")
	if err != nil {
		t.Fatalf("failed to read monitor.go: %v", err)
	}
	monitorSource := string(monitorData)
	monitorSnippets := []string{
		"type MonitorPhysicalDiskTemperatureHistoryProvider interface {",
		`PhysicalDiskTemperatureHistory(m *Monitor, orgID string, duration time.Duration) map[string][]MetricPoint`,
	}
	for _, snippet := range monitorSnippets {
		if !strings.Contains(monitorSource, snippet) {
			t.Fatalf("monitor.go must contain %q", snippet)
		}
	}

	pollerData, err := os.ReadFile("truenas_poller.go")
	if err != nil {
		t.Fatalf("failed to read truenas_poller.go: %v", err)
	}
	pollerSource := string(pollerData)
	pollerSnippets := []string{
		"func (p *TrueNASPoller) PhysicalDiskTemperatureHistory(_ *Monitor, orgID string, duration time.Duration) map[string][]MetricPoint {",
		"entry.provider.PhysicalDiskTemperatureHistory(ctx, duration)",
	}
	for _, snippet := range pollerSnippets {
		if !strings.Contains(pollerSource, snippet) {
			t.Fatalf("truenas_poller.go must contain %q", snippet)
		}
	}
}

func TestTrueNASSystemTelemetryUsesCanonicalHostTemperatureModel(t *testing.T) {
	clientData, err := os.ReadFile(filepath.Join("..", "truenas", "client.go"))
	if err != nil {
		t.Fatalf("failed to read ../truenas/client.go: %v", err)
	}
	clientSource := string(clientData)
	clientSnippets := []string{
		`"reporting.get_data"`,
		`telemetry.TemperatureCelsius = cloneTemperatureMap(temperatures)`,
		`return parseSystemTemperatures(response), nil`,
	}
	for _, snippet := range clientSnippets {
		if !strings.Contains(clientSource, snippet) {
			t.Fatalf("../truenas/client.go must contain %q", snippet)
		}
	}

	providerData, err := os.ReadFile(filepath.Join("..", "truenas", "provider.go"))
	if err != nil {
		t.Fatalf("failed to read ../truenas/provider.go: %v", err)
	}
	providerSource := string(providerData)
	providerSnippets := []string{
		`if temperature := maxTrueNASSystemTemperature(system); temperature != nil {`,
		`agent.Temperature = temperature`,
		`if sensors := sensorMetaFromTrueNASSystem(system); sensors != nil {`,
		`agent.Sensors = sensors`,
	}
	for _, snippet := range providerSnippets {
		if !strings.Contains(providerSource, snippet) {
			t.Fatalf("../truenas/provider.go must contain %q", snippet)
		}
	}
}

func TestHostAgentRemovalGuardUsesResolvedIdentifier(t *testing.T) {
	data, err := os.ReadFile("monitor_agents.go")
	if err != nil {
		t.Fatalf("failed to read monitor_agents.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"removedAt, wasRemoved := m.lookupRemovedHostAgent(identifier, hostname)",
		`Str("hostID", identifier)`,
		`fmt.Errorf("host agent %q had monitoring stopped at %v and cannot report again.`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor_agents.go must contain %q", snippet)
		}
	}
	if strings.Contains(source, "m.lookupRemovedHostAgent(baseIdentifier, hostname)") {
		t.Fatal("monitor_agents.go must not check removed host-agent state against pre-resolution baseIdentifier")
	}
}

func TestAlertLifecycleCanonicalChangesRemainWritable(t *testing.T) {
	store := unifiedresources.NewMemoryStore()
	incidentStore := memory.NewIncidentStore(memory.IncidentStoreConfig{})
	monitor := &Monitor{
		incidentStore: incidentStore,
	}
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store)))

	startedAt := time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC)
	alert := &alerts.Alert{
		ID:         "alert-guardrail-1",
		Type:       "cpu",
		Level:      alerts.AlertLevelCritical,
		ResourceID: "vm-guardrail",
		Message:    "CPU threshold exceeded",
		StartTime:  startedAt,
	}

	monitor.handleAlertFired(alert)

	changes, err := store.GetRecentChanges("vm-guardrail", time.Time{}, 10)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 canonical alert change, got %d", len(changes))
	}
	if changes[0].Kind != unifiedresources.ChangeAlertFired {
		t.Fatalf("Kind = %q, want %q", changes[0].Kind, unifiedresources.ChangeAlertFired)
	}

	timeline := incidentStore.GetTimelineByAlertIdentifier(alert.ID)
	if timeline == nil {
		t.Fatal("expected incident timeline")
	}
	if len(timeline.Events) != 1 || timeline.Events[0].Type != memory.IncidentEventAlertFired {
		t.Fatalf("expected projected alert-fired event, got %#v", timeline.Events)
	}
}

func TestSupplementalProviderChangesRefreshCanonicalReadState(t *testing.T) {
	now := time.Now().UTC()
	state := models.NewState()
	state.UpsertHost(models.Host{
		ID:       "host-1",
		Hostname: "seed-host",
		Status:   "online",
		LastSeen: now,
	})

	store := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	monitor := &Monitor{state: state}
	monitor.SetResourceStore(store)
	monitor.SetSupplementalRecordsProvider(unifiedresources.SourceVMware, guardrailSupplementalProvider{
		records: []unifiedresources.IngestRecord{
			{
				SourceID: "vmware:esxi-01",
				Resource: unifiedresources.Resource{
					Type:      unifiedresources.ResourceTypeAgent,
					Name:      "esxi-01.lab.local",
					Status:    unifiedresources.StatusOnline,
					LastSeen:  now,
					UpdatedAt: now,
				},
				Identity: unifiedresources.ResourceIdentity{
					Hostnames: []string{"esxi-01.lab.local"},
				},
			},
		},
	})

	resources := store.GetAll()
	for _, resource := range resources {
		for _, source := range resource.Sources {
			if source == unifiedresources.SourceVMware {
				return
			}
		}
	}
	t.Fatalf("expected resource store refresh to surface vmware supplemental records, got %#v", resources)
}

func TestTelemetrySnapshotAggregationUsesProvisionedTenantSet(t *testing.T) {
	baseDir := t.TempDir()
	persistence := config.NewMultiTenantPersistence(baseDir)
	for _, orgID := range []string{"default", "org-a"} {
		if _, err := persistence.GetPersistence(orgID); err != nil {
			t.Fatalf("GetPersistence(%s): %v", orgID, err)
		}
	}

	rm, err := NewReloadableMonitor(&config.Config{DataPath: baseDir}, persistence, nil)
	if err != nil {
		t.Fatalf("NewReloadableMonitor: %v", err)
	}

	mtm := rm.GetMultiTenantMonitor()
	if mtm == nil {
		t.Fatal("expected multi-tenant monitor")
	}
	mtm.monitors["default"] = testTelemetryMonitor(
		nil,
		[]models.VM{{ID: "vm-default", VMID: 101, Name: "vm-default", Instance: "pve-default"}},
		nil,
		nil,
		nil,
		nil,
		nil,
		1,
	)
	mtm.monitors["org-a"] = testTelemetryMonitor(
		nil,
		[]models.VM{{ID: "vm-a", VMID: 201, Name: "vm-a", Instance: "pve-a"}},
		nil,
		nil,
		nil,
		nil,
		nil,
		2,
	)

	counts := rm.AggregateInstallSnapshotCounts()
	if counts.VMs != 2 {
		t.Fatalf("VMs = %d, want 2 across provisioned tenants", counts.VMs)
	}
	if counts.ActiveAlerts != 3 {
		t.Fatalf("ActiveAlerts = %d, want 3 across provisioned tenants", counts.ActiveAlerts)
	}
}
