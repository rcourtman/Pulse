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
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
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

func TestPVETagStyleRefreshStaysPerInstance(t *testing.T) {
	data, err := os.ReadFile("monitor_pve.go")
	if err != nil {
		t.Fatalf("failed to read monitor_pve.go: %v", err)
	}
	source := string(data)

	for _, snippet := range []string{
		"func (m *Monitor) refreshPVETagColors(ctx context.Context, instanceName string, client PVEClientInterface)",
		"tagStyle := proxmox.ParseTagStyle(opts.TagStyle)",
		"m.state.MergePVETagStyle(instanceName, models.PVETagStyle{",
	} {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor_pve.go must contain %q", snippet)
		}
	}
	if strings.Contains(source, "ParseTagColorMap(opts.TagStyle)") {
		t.Fatal("PVE tag refresh must preserve case-sensitive style metadata, not only the legacy color map")
	}
	if strings.Contains(source, "m.state.MergeTagColors(") {
		t.Fatal("PVE tag refresh must merge style by instance so cleared color maps do not leave stale aggregate colors")
	}
}

func TestBroadcastResourceDiskIOUsesUnifiedResourceMetrics(t *testing.T) {
	hasDiskIO, readRate, writeRate := monitorDiskIOMetricInput(&unifiedresources.ResourceMetrics{
		DiskRead:  &unifiedresources.MetricValue{Value: 4096.4, Unit: "bytes/s", Source: unifiedresources.SourceAgent},
		DiskWrite: &unifiedresources.MetricValue{Value: 8191.6, Unit: "bytes/s", Source: unifiedresources.SourceAgent},
	})
	if !hasDiskIO {
		t.Fatal("expected disk I/O metrics to be projected")
	}
	if readRate != 4096 || writeRate != 8192 {
		t.Fatalf("unexpected projected disk I/O rates: read=%d write=%d", readRate, writeRate)
	}
}

func TestStandaloneInactiveDockerSwarmMetadataIsNotCapabilityEvidence(t *testing.T) {
	got := convertDockerSwarmInfo(&agentsdocker.SwarmInfo{
		NodeRole:   "worker",
		LocalState: "inactive",
		Scope:      "node",
	})
	if got != nil {
		t.Fatalf("expected inactive standalone Docker Swarm metadata to be omitted, got %+v", got)
	}
}

func TestDockerInventoryConvertersPreserveNativeRuntimeFields(t *testing.T) {
	createdAt := time.Date(2026, 5, 24, 8, 0, 0, 0, time.UTC)

	images := convertDockerImages([]agentsdocker.Image{{
		ID:          " sha256:image1 ",
		RepoTags:    []string{"repo/app:latest"},
		RepoDigests: []string{"repo/app@sha256:abc"},
		SizeBytes:   1024,
		CreatedAt:   createdAt,
		Labels:      map[string]string{"tier": "web"},
	}})
	if len(images) != 1 || images[0].ID != "sha256:image1" || images[0].Labels["tier"] != "web" {
		t.Fatalf("unexpected image conversion: %+v", images)
	}

	volumes := convertDockerVolumes([]agentsdocker.Volume{{
		Name: " app-data ", Driver: " local ", SizeBytes: 2048, RefCount: 3, Labels: map[string]string{"backup": "true"},
	}})
	if len(volumes) != 1 || volumes[0].Name != "app-data" || volumes[0].SizeBytes != 2048 || volumes[0].Labels["backup"] != "true" {
		t.Fatalf("unexpected volume conversion: %+v", volumes)
	}

	networks := convertDockerNetworks([]agentsdocker.Network{{
		ID: " net1 ", Name: " app-net ", Driver: " bridge ", EnableIPv4: true, Subnets: []agentsdocker.NetworkSubnet{{Subnet: " 10.88.0.0/24 ", Gateway: " 10.88.0.1 "}},
	}})
	if len(networks) != 1 || networks[0].Name != "app-net" || networks[0].Subnets[0].Subnet != "10.88.0.0/24" {
		t.Fatalf("unexpected network conversion: %+v", networks)
	}

	usage := convertDockerStorageUsage(&agentsdocker.StorageUsage{
		Images: agentsdocker.StorageUsageBucket{TotalCount: 3, ActiveCount: 2, TotalSizeBytes: 4096, ReclaimableBytes: 512},
	})
	if usage == nil || usage.Images.TotalCount != 3 || usage.Images.ReclaimableBytes != 512 {
		t.Fatalf("unexpected storage usage conversion: %+v", usage)
	}

	nodes := convertDockerNodes([]agentsdocker.Node{{
		ID:                  " node-manager ",
		Hostname:            " manager-1 ",
		Role:                " manager ",
		Availability:        " active ",
		State:               " ready ",
		Address:             " 192.0.2.10 ",
		ManagerReachability: " reachable ",
		ManagerAddress:      " 192.0.2.10:2377 ",
		Leader:              true,
		EngineVersion:       " 27.5.1 ",
		OS:                  " linux ",
		Architecture:        " amd64 ",
		NanoCPUs:            8_000_000_000,
		MemoryBytes:         32 * 1024 * 1024 * 1024,
		Labels:              map[string]string{"zone": "rack-a"},
		EngineLabels:        map[string]string{"engine": "primary"},
		CreatedAt:           createdAt,
	}})
	if len(nodes) != 1 || nodes[0].ID != "node-manager" || nodes[0].ManagerReachability != "reachable" || nodes[0].EngineLabels["engine"] != "primary" {
		t.Fatalf("unexpected swarm node conversion: %+v", nodes)
	}
}

func TestPVEBackupPermissionWarningsPreserveTokenACLRepair(t *testing.T) {
	warning := pveBackupPermissionWarning(&config.PVEInstance{
		TokenName: "pulse-monitor@pve!pulse-example",
	})

	for _, snippet := range []string{
		"pveum aclmod /storage -user pulse-monitor@pve -role PVEDatastoreAdmin",
		"pveum aclmod /storage -token 'pulse-monitor@pve!pulse-example' -role PVEDatastoreAdmin",
	} {
		if !strings.Contains(warning, snippet) {
			t.Fatalf("expected warning to contain %q, got %q", snippet, warning)
		}
	}
}

func TestLegacySSHTemperatureUsesPulseSensorWrapperContract(t *testing.T) {
	if !strings.Contains(pulseSensorsSSHCommand, "/usr/local/sbin/pulse-sensors") {
		t.Fatalf("legacy SSH temperature command must prefer Pulse sensor wrapper, got %q", pulseSensorsSSHCommand)
	}
	if !strings.Contains(pulseSensorsSSHCommand, "sensors -j") {
		t.Fatalf("legacy SSH temperature command must preserve sensors -j fallback for old forced keys, got %q", pulseSensorsSSHCommand)
	}
	if strings.Contains(pulseSensorsSSHCommand, "smartctl") {
		t.Fatalf("SMART collection belongs inside the remote wrapper, not the local SSH command: %q", pulseSensorsSSHCommand)
	}
}

func TestPBSBackupsSnapshotPreservesSourceArtifactFields(t *testing.T) {
	state := models.NewState()
	backupTime := time.Date(2026, 5, 25, 1, 34, 25, 0, time.UTC)
	state.UpdatePBSBackups("pbs-main", []models.PBSBackup{{
		ID:         "pbs-main/main/minipc/ct/112/2026-05-25T01:34:25Z",
		Instance:   "pbs-main",
		Datastore:  "main",
		Namespace:  "minipc",
		BackupType: "ct",
		VMID:       "112",
		BackupTime: backupTime,
		Size:       8_589_934_592,
		Protected:  true,
		Verified:   true,
		Comment:    "debian-go",
		Files:      []string{"index.json.blob", "root.pxar.didx"},
		Owner:      "backup@pbs",
	}})
	monitor := &Monitor{state: state}

	got := monitor.PBSBackupsSnapshot()
	if len(got) != 1 {
		t.Fatalf("expected one PBS backup, got %d", len(got))
	}
	if got[0].Datastore != "main" || got[0].Namespace != "minipc" || got[0].VMID != "112" {
		t.Fatalf("unexpected PBS artifact identity: %+v", got[0])
	}
	if got[0].Size != 8_589_934_592 || !got[0].Protected || !got[0].Verified {
		t.Fatalf("PBS artifact source fields were not preserved: %+v", got[0])
	}
	if got[0].Owner != "backup@pbs" || len(got[0].Files) != 2 {
		t.Fatalf("PBS artifact owner/files were not preserved: %+v", got[0])
	}
}

func TestGuestSnapshotPollingStaysBoundedConcurrent(t *testing.T) {
	data, err := os.ReadFile("monitor_backups.go")
	if err != nil {
		t.Fatalf("failed to read monitor_backups.go: %v", err)
	}
	source := string(data)

	for _, snippet := range []string{
		"maxConcurrentGuestSnapshotPolls = 8",
		"targets := make([]guestSnapshotPollTarget, 0, activeGuests)",
		"results := make(chan guestSnapshotPollResult, len(targets))",
		"sem := make(chan struct{}, maxConcurrentGuestSnapshotPolls)",
		"polledGuestKeys[result.target.key] = struct{}{}",
	} {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor_backups.go must contain %q", snippet)
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

func TestAlertStateSyncDoesNotLogNormalResolvedStateAtInfo(t *testing.T) {
	data, err := os.ReadFile("monitor_alert_sync.go")
	if err != nil {
		t.Fatalf("failed to read monitor_alert_sync.go: %v", err)
	}
	source := string(data)

	if !strings.Contains(source, `log.Debug().Int("count", len(recentlyResolved)).Msg("syncing recently resolved alerts")`) {
		t.Fatal("recently resolved alert sync must stay at debug level to avoid info-log churn")
	}
	if strings.Contains(source, `log.Info().Int("count", len(recentlyResolved)).Msg("syncing recently resolved alerts")`) {
		t.Fatal("recently resolved alert sync must not log normal polling state at info level")
	}
}

func TestUnifiedResourceAlertSyncEvaluatesMetricsBeforeIncidents(t *testing.T) {
	data, err := os.ReadFile("monitor_alert_sync.go")
	if err != nil {
		t.Fatalf("failed to read monitor_alert_sync.go: %v", err)
	}
	source := string(data)

	metricIndex := strings.Index(source, "CheckUnifiedResourceMetrics(resources)")
	incidentIndex := strings.Index(source, "SyncUnifiedResourceIncidents(resources)")
	if metricIndex < 0 {
		t.Fatalf("monitor alert sync must run unified resource metric evaluation")
	}
	if incidentIndex < 0 {
		t.Fatalf("monitor alert sync must preserve unified resource incident sync")
	}
	if metricIndex > incidentIndex {
		t.Fatalf("unified resource metric evaluation must run before incident sync")
	}
}

func TestStateBroadcastTreatsTypedNilHubAsAbsent(t *testing.T) {
	data, err := os.ReadFile("monitor.go")
	if err != nil {
		t.Fatalf("failed to read monitor.go: %v", err)
	}
	source := string(data)

	for _, snippet := range []string{
		"func isNilStateBroadcaster(hub stateBroadcaster) bool {",
		"if isNilStateBroadcaster(hub) {",
		"value := reflect.ValueOf(hub)",
		"case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:",
		"return value.IsNil()",
	} {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor.go must contain %q", snippet)
		}
	}
}

func TestBroadcastResourceProjectionPreservesCanonicalHealthContext(t *testing.T) {
	data, err := os.ReadFile("monitor.go")
	if err != nil {
		t.Fatalf("failed to read monitor.go: %v", err)
	}
	source := string(data)

	for _, snippet := range []string{
		"if resource.DiscoveryTarget == nil {",
		"if resource.MetricsTarget == nil {",
		"unifiedresources.RefreshCanonicalMetadata(&resource)",
		"IncidentSummary:       resource.IncidentSummary",
		"Canonical:             monitorRawJSON(resource.Canonical)",
		"DiscoveryTarget:       monitorRawJSON(resource.DiscoveryTarget)",
		"MetricsTarget:         monitorRawJSON(resource.MetricsTarget)",
		"Storage:               monitorRawJSON(resource.Storage)",
		"Agent:                 monitorRawJSON(resource.Agent)",
		"Incidents:             monitorRawJSON(resource.Incidents)",
	} {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor.go must contain %q", snippet)
		}
	}
}

func TestHostUnraidReadStateProjectionPreservesNativeDiskFields(t *testing.T) {
	projected := hostUnraidFromReadStateView(&unifiedresources.HostUnraidMeta{
		ArrayStarted: true,
		ArrayState:   "STARTED",
		Disks: []unifiedresources.HostUnraidDiskMeta{
			{
				Name:        "disk1",
				Device:      "/dev/sdc",
				Role:        "data",
				Status:      "online",
				RawStatus:   "DISK_OK",
				Model:       "WDC WD60EFRX",
				Serial:      "SERIAL-DATA",
				Filesystem:  "xfs",
				Transport:   "sata",
				SizeBytes:   6_000_000_000_000,
				UsedBytes:   4_000,
				FreeBytes:   2_000,
				Temperature: 31,
				SpunDown:    true,
				ReadCount:   11,
				WriteCount:  12,
				ErrorCount:  16,
				Slot:        1,
			},
		},
	})

	if projected == nil || len(projected.Disks) != 1 {
		t.Fatalf("expected projected Unraid disk metadata, got %+v", projected)
	}
	disk := projected.Disks[0]
	if disk.Model != "WDC WD60EFRX" || disk.Transport != "sata" || disk.SizeBytes != 6_000_000_000_000 {
		t.Fatalf("expected native metadata to survive read-state projection, got %+v", disk)
	}
	if disk.UsedBytes != 4_000 || disk.FreeBytes != 2_000 || disk.Temperature != 31 || !disk.SpunDown {
		t.Fatalf("expected native capacity and state fields to survive projection, got %+v", disk)
	}
	if disk.ReadCount != 11 || disk.WriteCount != 12 || disk.ErrorCount != 16 {
		t.Fatalf("expected native counters to survive projection, got %+v", disk)
	}
}

func TestBroadcastResourceProjectionCoalescesSplitHostIdentities(t *testing.T) {
	data, err := os.ReadFile("monitor.go")
	if err != nil {
		t.Fatalf("failed to read monitor.go: %v", err)
	}
	source := string(data)

	for _, snippet := range []string{
		"metricsTargetResolver := broadcastMetricsTargetResolver(unifiedView.readState)",
		"broadcastResources := unifiedresources.CoalescePresentationHostResources(unifiedView.resources)",
		"frontendState.Resources = convertResourcesForBroadcast(broadcastResources, metricsTargetResolver)",
		"frontendState.ConnectedInfrastructure = buildConnectedInfrastructure(broadcastResources, snapshot)",
		"func attachBroadcastMetricsTargets(",
		"allResources = unifiedresources.CoalescePresentationHostResources(allResources)",
	} {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor.go must contain %q", snippet)
		}
	}
}

func TestMetricsStoreRuntimeOverridesStayOnMonitorBoundary(t *testing.T) {
	data, err := os.ReadFile("monitor.go")
	if err != nil {
		t.Fatalf("failed to read monitor.go: %v", err)
	}
	source := string(data)

	for _, snippet := range []string{
		"metricsStoreConfig := metrics.DefaultConfig(cfg.DataPath)",
		"if strings.TrimSpace(cfg.MetricsDBPath) != \"\" {",
		"metricsStoreConfig.DBPath = cfg.MetricsDBPath",
		"if cfg.MetricsRollupInterval > 0 {",
		"metricsStoreConfig.RollupInterval = cfg.MetricsRollupInterval",
	} {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor.go must contain %q", snippet)
		}
	}
}

func TestEscalationDeliveryDefersToCanonicalAlertSuppression(t *testing.T) {
	requiredSnippets := map[string][]string{
		"monitor.go": {
			"m.alertManager.SetEscalateCallback(func(alert *alerts.Alert, level int) {",
			"m.handleAlertEscalated(wsHub, alert, level)",
		},
		"monitor_alerts.go": {
			"func (m *Monitor) handleAlertEscalated(hub *websocket.Hub, alert *alerts.Alert, level int) {",
			"if m.alertManager.ShouldSuppressNotification(alert) {",
			"m.notificationMgr.SendEscalatedAlert(alert, escalationLevel.Notify)",
			"m.broadcastEscalatedAlert(hub, alert)",
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

func TestAvailabilityProviderStaysOnCanonicalMonitoringPath(t *testing.T) {
	requiredSnippets := map[string][]string{
		"scheduler.go": {
			`InstanceTypeAvailability InstanceType = "availability"`,
		},
		"monitor.go": {
			"availabilityStatuses       map[string]AvailabilityProbeStatus",
			"availabilityStatuses:       make(map[string]AvailabilityProbeStatus)",
		},
		"poll_providers.go": {
			"_ = m.RegisterPollProvider(newAvailabilityPollProvider())",
			"newAvailabilityPollProvider(),",
			"case InstanceTypeAvailability:",
			"return newAvailabilityPollProvider()",
		},
		"availability_poller.go": {
			"func newAvailabilityPollProvider() PollProvider {",
			"func (availabilityPollProvider) SupplementalSource() unifiedresources.DataSource {",
			"return unifiedresources.SourceAvailability",
			"func (availabilityPollProvider) SupplementalRecords(m *Monitor, orgID string) []unifiedresources.IngestRecord {",
			"Type:         unifiedresources.ResourceTypeNetworkEndpoint,",
			"Sources:      []unifiedresources.DataSource{unifiedresources.SourceAvailability},",
			"Availability: data,",
			"m.recordTaskResult(InstanceTypeAvailability, target.ID, nil)",
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

func TestMonitoredSystemUsageReadinessGuardrailsRemainCanonical(t *testing.T) {
	requiredSnippets := map[string][]string{
		"monitor.go": {
			"type MonitorSupplementalInventoryReadinessProvider interface {",
			"SupplementalInventoryReadyAt(m *Monitor, orgID string) (time.Time, bool)",
			"hostContinuityStore        *config.HostContinuityStore",
			"hostContinuityStore:        config.NewHostContinuityStore(cfg.DataPath, nil),",
			"func (m *Monitor) HostsSnapshot() []models.Host {",
			"readState = m.readStateWithStandaloneHostContinuity(readState)",
			"func (m *Monitor) unifiedStateViewWithStandaloneHostContinuity(view monitorUnifiedStateView) monitorUnifiedStateView {",
			"view.readState = readState",
			"view.resources = resources",
			"func latestUnifiedResourceLastSeen(resources []unifiedresources.Resource) time.Time {",
			"return m.unifiedStateViewWithStandaloneHostContinuity(monitorUnifiedStateView{",
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
			"readState = m.readStateWithStandaloneHostContinuity(readState)",
			"Count:     unifiedresources.MonitoredSystemCount(readState),",
			"Available: true,",
			"func (m *Monitor) readStateWithStandaloneHostContinuity(",
			"return unifiedresources.ReadStateWithRecords(readState, unifiedresources.SourceAgent, records)",
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

func TestInstallTelemetrySnapshotCountsStayOnMonitoringBoundary(t *testing.T) {
	data, err := os.ReadFile("reload.go")
	if err != nil {
		t.Fatalf("failed to read reload.go: %v", err)
	}
	source := string(data)

	for _, snippet := range []string{
		"AgentHosts            int",
		"DockerContainers      int",
		"KubernetesPods        int",
		"TrueNASSystems        int",
		"VMwareDatastores      int",
		"AvailabilityTargets   int",
		"resources, _ := monitor.UnifiedResourceSnapshot()",
		"accumulateInstallSnapshotUnifiedResourceCounts(counts, resources)",
		"case unifiedresources.ResourceTypeNetworkShare:",
		"resource.Availability != nil && resource.Availability.Enabled",
	} {
		if !strings.Contains(source, snippet) {
			t.Fatalf("reload.go must contain %q", snippet)
		}
	}
}

func TestMonitoredSystemUsageStaysInventoryOnly(t *testing.T) {
	data, err := os.ReadFile("monitored_system_usage.go")
	if err != nil {
		t.Fatalf("failed to read monitored_system_usage.go: %v", err)
	}
	source := string(data)

	for _, forbidden := range []string{
		"max_monitored_systems",
		"license_limit",
		"would_exceed_limit",
		"monitored_system_capacity",
		"admission",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("monitored_system_usage.go must not contain retired cap/admission token %q", forbidden)
		}
	}
}

func TestDockerHostIdentityUsesCanonicalHostnameEquivalence(t *testing.T) {
	data, err := os.ReadFile("docker_host_identity.go")
	if err != nil {
		t.Fatalf("failed to read docker_host_identity.go: %v", err)
	}
	source := string(data)

	for _, snippet := range []string{
		"unifiedresources.HostnamesEquivalent(host.Hostname(), hostname)",
	} {
		if !strings.Contains(source, snippet) {
			t.Fatalf("docker_host_identity.go must contain %q", snippet)
		}
	}
}

func TestDockerTokenBindingUsesCanonicalHostIdentity(t *testing.T) {
	requiredSnippets := map[string][]string{
		"docker_host_identity.go": {
			"func resolveDockerTokenBindingIdentity(",
			"preferred = dockerHostStableID(previous)",
			"previous.AgentID()",
			"func dockerTokenBindingMatches(",
		},
		"monitor_agents.go": {
			"resolveDockerTokenBindingIdentity(identifier, report, previous, hasPrevious)",
			"dockerTokenBindingMatches(boundAgentID, tokenBindingAliases)",
			"Bound Docker / Podman module token to host identity",
		},
		"monitor.go": {
			"Docker host identity bindings",
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
		"func guestStatusFreeMem(status *proxmox.VMStatus) uint64 {",
		"if status.BalloonInfo != nil {",
		"if status.Balloon > 0 && status.Balloon <= memTotal && freeMem <= status.Balloon {",
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
			"ttl = m.vmAgentMemNegativeCacheTTL(instanceName, node, vmid)",
			"vmAgentMemNegativeKnownGuestTTL = 30 * time.Second",
		},
		"guest_memory_sources.go": {
			"func shouldPreferGuestAgentMemAvailable(status *proxmox.VMStatus, memTotal uint64) bool {",
			"func (m *Monitor) tryGuestAgentMemAvailable(",
			"if memAvailable == 0 && shouldPreferGuestAgentMemAvailable(status, memTotal) {",
			"if rrdAvailable, rrdErr := m.getVMRRDMetrics(ctx, client, instanceName, node, vmid); rrdErr == nil && rrdAvailable > 0 {",
			"if agentAvailable, ok := m.tryGuestAgentMemAvailable(ctx, client, instanceName, guestName, node, vmid, memTotal, guestRaw); ok {",
			`memorySource = "guest-agent-meminfo"`,
			"guestRaw.GuestAgentMemAvailable = agentAvailable",
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

func TestLXCStatusCountersStayOnCanonicalMergePath(t *testing.T) {
	requiredSnippets := map[string][]string{
		"monitor_pve.go": {
			"func mergeContainerRuntimeCounters(current IOMetrics, status *proxmox.Container) IOMetrics {",
			"func (m *Monitor) enrichContainerMetadata(ctx context.Context, client PVEClientInterface, instanceName, nodeName string, container *models.Container, prefetchedStatus ...*proxmox.Container) {",
		},
		"monitor_pve_guest_lxc.go": {
			"currentMetrics = mergeContainerRuntimeCounters(currentMetrics, statusSnapshot)",
			"m.enrichContainerMetadata(ctx, client, instanceName, res.Node, &container, statusSnapshot)",
		},
		"monitor_polling_containers.go": {
			"currentMetrics = mergeContainerRuntimeCounters(currentMetrics, statusSnapshot)",
			"m.enrichContainerMetadata(",
			"statusSnapshot,",
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

func TestDockerMutatingCommandsRespectCanonicalSecurityPosture(t *testing.T) {
	monitor := newTestMonitorForCommands(t)
	host := models.DockerHost{
		ID:       "docker-secure-host",
		Hostname: "docker-secure-host",
		Status:   "online",
		Containers: []models.DockerContainer{
			{
				ID:   "container-a",
				Name: "container-a",
				UpdateStatus: &models.DockerContainerUpdateStatus{
					UpdateAvailable: true,
					LastChecked:     time.Now().UTC(),
				},
			},
		},
		Security: &models.DockerHostSecurity{
			AuthorizationPlugins:          []string{"opa"},
			MutatingCommandsBlocked:       true,
			MutatingCommandsBlockedReason: "blocked for test",
		},
	}
	monitor.state.UpsertDockerHost(host)

	if _, err := monitor.QueueDockerContainerUpdateCommand(host.ID, "container-a", "container-a"); err == nil || !strings.Contains(err.Error(), "blocked for test") {
		t.Fatalf("expected container update to be blocked by canonical security posture, got %v", err)
	}

	if _, err := monitor.QueueDockerUpdateAllCommand(host.ID); err == nil || !strings.Contains(err.Error(), "blocked for test") {
		t.Fatalf("expected update-all to be blocked by canonical security posture, got %v", err)
	}

	status, err := monitor.QueueDockerCheckUpdatesCommand(host.ID)
	if err != nil {
		t.Fatalf("expected check-updates to remain allowed, got %v", err)
	}
	if status.Type != DockerCommandTypeCheckUpdates {
		t.Fatalf("expected check-updates command, got %q", status.Type)
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
			"prevVMByID := prevGuests.vmsByID",
			"m.pollNodeVMsWithClusterResourceBuilder(ctx, instanceName, n.Node, vms, client, prevVMByID, vmIDToHostAgent)",
		},
		"monitor_pve_node_vm_builder.go": {
			"return m.collectClusterVMResources(ctx, instanceName, resources, client, prevVMByID, vmIDToHostAgent), templateSubjects",
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
			"m.pollNodeVMsWithClusterResourceBuilder(ctx, instanceName, n.Node, vms, client, prevVMByID, vmIDToHostAgent)",
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

func TestProxmoxGuestDiskInventoryPrefersCanonicalLinkedHostAgentSource(t *testing.T) {
	requiredSnippets := map[string][]string{
		"guest_host_agent_fallback.go": {
			"func resolveGuestDiskFromLinkedHostAgent(guestID string, vmIDToHostAgent map[string]models.Host) (models.Disk, []models.Disk, bool) {",
			"summary, ok := models.AggregateDisk(host.Disks)",
			"func preferLinkedHostAgentDiskInventory(",
		},
		"monitor_pve_guest_builders.go": {
			"preferLinkedHostAgentDiskInventory(",
			`Msg("QEMU disk: preferring linked Pulse host agent disk inventory")`,
		},
		"monitor_polling_vm.go": {
			"vmIDToHostAgent := prevGuests.hostAgentsByVMID",
			"m.pollNodeVMsWithClusterResourceBuilder(ctx, instanceName, n.Node, vms, client, prevVMByID, vmIDToHostAgent)",
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

func TestProxmoxGuestDockerInventoryUsesCanonicalReportIngestPath(t *testing.T) {
	requiredSnippets := map[string][]string{
		"docker_detection.go": {
			"type DockerInventoryCollector interface {",
			"proxmoxGuestDockerSocketMarker",
			"proxmoxGuestDockerNegativeRecheckAfter",
			"dockerCheckerConfiguredAt",
			"checker_reconfigured",
			"func (m *Monitor) CollectProxmoxGuestDockerInventory(ctx context.Context, containers []models.Container) {",
			"m.hasOnlineHostAgentForContainer(ct.ID)",
			"m.ApplyDockerReport(report, nil)",
			"pct exec %d -- sh -c",
			"container docker socket probe failed",
			`docker ps -a --no-trunc --format 'CONTAINER\\t{{json .ID}}`,
			`docker stats --no-stream --no-trunc --format 'STAT\\t{{json .ID}}`,
			"ParseProxmoxGuestDockerInventoryVMIDs",
		},
		"monitor_pve_guest_poll.go": {
			"m.CollectProxmoxGuestDockerInventory(ctx, allContainers)",
			"m.state.UpdateContainersForInstance(instanceName, allContainers)",
		},
		"monitor_polling_containers.go": {
			"m.CollectProxmoxGuestDockerInventory(ctx, allContainers)",
			"m.state.UpdateContainersForInstance(instanceName, allContainers)",
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
		if file == "docker_detection.go" && strings.Contains(source, "&& echo yes || echo no") {
			t.Fatalf("%s must not mask host-side pct exec failures with a host-shell echo fallback", file)
		}
	}
}

func TestProxmoxGuestDockerDetectionRechecksNegativeCacheAfterCheckerSetup(t *testing.T) {
	configuredAt := time.Now()
	monitor := &Monitor{}
	container := models.Container{
		ID:     "pve-a:node-a:101",
		VMID:   101,
		Name:   "docker-lxc",
		Node:   "node-a",
		Status: "running",
		Uptime: 7200,
	}

	reason := monitor.containerNeedsDockerCheck(container, map[string]models.Container{
		container.ID: {
			ID:              container.ID,
			Status:          "running",
			Uptime:          3600,
			HasDocker:       false,
			DockerCheckedAt: configuredAt.Add(-time.Minute),
		},
	}, configuredAt)
	if reason != "checker_reconfigured" {
		t.Fatalf("reason = %q, want checker_reconfigured", reason)
	}

	reason = monitor.containerNeedsDockerCheck(container, map[string]models.Container{
		container.ID: {
			ID:              container.ID,
			Status:          "running",
			Uptime:          3600,
			HasDocker:       false,
			DockerCheckedAt: configuredAt.Add(time.Minute),
		},
	}, configuredAt)
	if reason != "" {
		t.Fatalf("fresh negative cache reason = %q, want empty", reason)
	}
}

func TestProxmoxGuestDockerLXCProbingRequiresExplicitOptIn(t *testing.T) {
	requiredSnippets := map[string][]string{
		"../config/config.go": {
			"EnableProxmoxGuestDockerDetection bool",
			`envconfig:"PULSE_ENABLE_PROXMOX_GUEST_DOCKER_DETECTION" default:"false"`,
			"EnableProxmoxGuestDockerInventory bool",
			`envconfig:"PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY" default:"false"`,
		},
		"../api/router.go": {
			"inventoryEnabled := r.config != nil && r.config.EnableProxmoxGuestDockerInventory",
			"detectionEnabled := r.config != nil && (r.config.EnableProxmoxGuestDockerDetection || inventoryEnabled)",
			"m.SetDockerChecker(nil)",
			"m.SetDockerInventoryCollector(nil)",
			"if inventoryEnabled {",
		},
		"../../docs/UNIFIED_AGENT.md": {
			"Pulse does not use a Proxmox node agent to look inside LXCs by default.",
			"`PULSE_ENABLE_PROXMOX_GUEST_DOCKER_DETECTION=true`",
			"`PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY=true`",
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

func TestBackupOrphanDetectionUsesCanonicalInventoryReadinessScope(t *testing.T) {
	requiredSnippets := map[string][]string{
		"monitor.go": {
			"pveBackupInventoryReady    map[string]map[string]bool",
			"pveBackupTemplateSubjects  map[string]map[string]struct{}",
		},
		"monitor_backups.go": {
			"func (m *Monitor) updatePVEBackupTemplateSubjectsForType(instanceName, guestType string, subjects map[string]struct{}) {",
			"func (m *Monitor) updatePVEBackupTemplateSubjectsFromClusterResources(instanceName string, resources []proxmox.ClusterResource) {",
			"func (m *Monitor) backupInventoryScopeForAlerts() *alerts.BackupInventoryScope {",
			"m.alertManager.CheckBackupsWithInventory(rollups, guestsByKey, guestsByVMID, m.backupInventoryScopeForAlerts())",
		},
		"monitor_pve_guest_poll.go": {
			"m.updatePVEBackupTemplateSubjectsFromClusterResources(instanceName, resources)",
		},
		"monitor_pve_node_vm_builder.go": {
			`pveBackupTemplateSubjectKey(instanceName, "qemu", node, vm.VMID)`,
		},
		"monitor_polling_vm.go": {
			`m.updatePVEBackupTemplateSubjectsForType(instanceName, "qemu", qemuTemplateSubjects)`,
		},
		"monitor_polling_containers.go": {
			`pveBackupTemplateSubjectKey(instanceName, "lxc", n.Node, int(container.VMID))`,
			`m.updatePVEBackupTemplateSubjectsForType(instanceName, "lxc", lxcTemplateSubjects)`,
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

func TestStoragePollingKeepsSharedStorageClusterStatusCanonical(t *testing.T) {
	data, err := os.ReadFile("monitor_polling_storage.go")
	if err != nil {
		t.Fatalf("failed to read monitor_polling_storage.go: %v", err)
	}
	source := string(data)

	for _, snippet := range []string{
		"storage.Status = normalizeSharedStorageStatus(storage)",
		"entry.storage.Status = mergeSharedStorageStatus(entry.storage.Status, storage.Status)",
		"entry.storage.Enabled = entry.storage.Enabled || storage.Enabled",
		"entry.storage.Active = entry.storage.Active || storage.Active",
		"func normalizeSharedStorageStatus(storage models.Storage) string {",
		"func mergeSharedStorageStatus(current, candidate string) string {",
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
			"return hasUsableSMARTTemperature(hostAgentTemp)",
			"func hasUsableSMARTTemperature(temp *models.Temperature) bool {",
			"disk.Temperature > 0 && !disk.StandbySkipped",
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
	mustSetMockEnabled(t, true)
	t.Cleanup(func() { mustSetMockEnabled(t, previous) })

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

	mustSetMockEnabled(t, false)
	if shouldSkipMockOwnedUnifiedMetricSync(unifiedresources.Resource{
		Sources: []unifiedresources.DataSource{unifiedresources.SourceDocker},
	}) {
		t.Fatal("expected unified metric sync to remain available outside mock mode")
	}
}

func TestMonitoringBroadcastCarriesCanonicalSourceAndStoragePlatformIdentity(t *testing.T) {
	input := monitorResourceToConvertInput(unifiedresources.Resource{
		ID:      "storage-tower-array",
		Type:    unifiedresources.ResourceTypeStorage,
		Name:    "Tower Array",
		Status:  unifiedresources.StatusWarning,
		Sources: []unifiedresources.DataSource{unifiedresources.SourceAgent},
		Storage: &unifiedresources.StorageMeta{
			Type:     "unraid-array",
			Platform: "unraid",
			Topology: "array",
			Enabled:  true,
			Active:   true,
		},
	})

	if input.PlatformType != "agent" {
		t.Fatalf("PlatformType = %q, want agent for Unraid agent-backed storage", input.PlatformType)
	}
	if input.SourceType != "agent" {
		t.Fatalf("SourceType = %q, want agent", input.SourceType)
	}
	if len(input.Sources) != 1 || input.Sources[0] != string(unifiedresources.SourceAgent) {
		t.Fatalf("Sources = %#v, want [agent]", input.Sources)
	}

	payload := string(input.PlatformData)
	for _, snippet := range []string{
		`"platform":"unraid"`,
		`"type":"unraid-array"`,
		`"sources":["agent"]`,
	} {
		if !strings.Contains(payload, snippet) {
			t.Fatalf("PlatformData must contain %s, got %s", snippet, payload)
		}
	}
}

// TestMonitoringBroadcastDualSourceProxmoxAgentNodeKeepsProxmoxPlatformIdentity
// pins the Pi case: a Proxmox VE node with a linked Pulse host agent must
// resolve to platformType="proxmox-pve" and surface both sources in
// platformData. Regression guard for any future change that lets the agent
// facet override the Proxmox identity.
func TestMonitoringBroadcastDualSourceProxmoxAgentNodeKeepsProxmoxPlatformIdentity(t *testing.T) {
	input := monitorResourceToConvertInput(unifiedresources.Resource{
		ID:      "agent-pi-host",
		Type:    unifiedresources.ResourceTypeAgent,
		Name:    "pi",
		Status:  unifiedresources.StatusOnline,
		Sources: []unifiedresources.DataSource{unifiedresources.SourceProxmox, unifiedresources.SourceAgent},
		Proxmox: &unifiedresources.ProxmoxData{
			NodeName:   "pi",
			Instance:   "pi",
			PVEVersion: "8.3.1",
		},
		Agent: &unifiedresources.AgentData{
			AgentID:  "agent-pi",
			Hostname: "pi",
			OSName:   "Proxmox VE",
		},
	})

	if input.PlatformType != "proxmox-pve" {
		t.Fatalf("PlatformType = %q, want proxmox-pve for Proxmox node linked to host agent", input.PlatformType)
	}
	if input.SourceType != "hybrid" {
		t.Fatalf("SourceType = %q, want hybrid", input.SourceType)
	}
	wantSources := []string{"proxmox", "agent"}
	if len(input.Sources) != len(wantSources) {
		t.Fatalf("Sources = %#v, want %#v", input.Sources, wantSources)
	}
	for i, want := range wantSources {
		if input.Sources[i] != want {
			t.Fatalf("Sources[%d] = %q, want %q (full = %#v)", i, input.Sources[i], want, input.Sources)
		}
	}

	payload := string(input.PlatformData)
	for _, snippet := range []string{
		`"pveVersion":"8.3.1"`,
		`"sources":["proxmox","agent"]`,
	} {
		if !strings.Contains(payload, snippet) {
			t.Fatalf("PlatformData must contain %s, got %s", snippet, payload)
		}
	}
}

// TestMonitoringBroadcastDualSourceTrueNASAgentKeepsTrueNASPlatformIdentity
// pins the TrueNAS appliance + linked host agent case to platformType="truenas"
// so a future code path cannot quietly demote the appliance identity to
// "agent" once the agent facet is also populated.
func TestMonitoringBroadcastDualSourceTrueNASAgentKeepsTrueNASPlatformIdentity(t *testing.T) {
	input := monitorResourceToConvertInput(unifiedresources.Resource{
		ID:      "agent-truenas-host",
		Type:    unifiedresources.ResourceTypeAgent,
		Name:    "truenas-1",
		Status:  unifiedresources.StatusOnline,
		Sources: []unifiedresources.DataSource{unifiedresources.SourceTrueNAS, unifiedresources.SourceAgent},
		TrueNAS: &unifiedresources.TrueNASData{
			Hostname: "truenas-1",
			Version:  "TrueNAS-SCALE-24.10.0",
		},
		Agent: &unifiedresources.AgentData{
			AgentID:  "agent-truenas",
			Hostname: "truenas-1",
			OSName:   "TrueNAS SCALE",
		},
	})

	if input.PlatformType != "truenas" {
		t.Fatalf("PlatformType = %q, want truenas for TrueNAS appliance linked to host agent", input.PlatformType)
	}
	if input.SourceType != "hybrid" {
		t.Fatalf("SourceType = %q, want hybrid", input.SourceType)
	}
	wantSources := []string{"truenas", "agent"}
	if len(input.Sources) != len(wantSources) {
		t.Fatalf("Sources = %#v, want %#v", input.Sources, wantSources)
	}
	for i, want := range wantSources {
		if input.Sources[i] != want {
			t.Fatalf("Sources[%d] = %q, want %q (full = %#v)", i, input.Sources[i], want, input.Sources)
		}
	}

	payload := string(input.PlatformData)
	if !strings.Contains(payload, `"sources":["truenas","agent"]`) {
		t.Fatalf("PlatformData must contain hybrid source list, got %s", payload)
	}
}

// TestMonitoringBroadcastUnraidAgentHostKeepsAgentPlatformType pins the Tower
// case: an Unraid host whose only data source is the Pulse agent stays on
// platformType="agent". The frontend resolves the "Unraid" identity badge via
// host profile / OSName; the contract platformType must not promote OSName
// strings to platform identifiers.
func TestMonitoringBroadcastUnraidAgentHostKeepsAgentPlatformType(t *testing.T) {
	input := monitorResourceToConvertInput(unifiedresources.Resource{
		ID:      "agent-tower-host",
		Type:    unifiedresources.ResourceTypeAgent,
		Name:    "tower",
		Status:  unifiedresources.StatusOnline,
		Sources: []unifiedresources.DataSource{unifiedresources.SourceAgent},
		Agent: &unifiedresources.AgentData{
			AgentID:  "agent-tower",
			Hostname: "tower",
			OSName:   "Unraid OS 7.2.2",
		},
	})

	if input.PlatformType != "agent" {
		t.Fatalf("PlatformType = %q, want agent for single-source Unraid host agent", input.PlatformType)
	}
	if input.SourceType != "agent" {
		t.Fatalf("SourceType = %q, want agent", input.SourceType)
	}
	if len(input.Sources) != 1 || input.Sources[0] != "agent" {
		t.Fatalf("Sources = %#v, want [agent]", input.Sources)
	}

	payload := string(input.PlatformData)
	for _, snippet := range []string{
		`"osName":"Unraid OS 7.2.2"`,
		`"sources":["agent"]`,
	} {
		if !strings.Contains(payload, snippet) {
			t.Fatalf("PlatformData must contain %s, got %s", snippet, payload)
		}
	}
}

func TestMonitorSetMockModeAuthorizesBeforeResettingRuntimeState(t *testing.T) {
	data, err := os.ReadFile("monitor.go")
	if err != nil {
		t.Fatalf("failed to read monitor.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"if err := mock.SetEnabled(true); err != nil {",
		"if err := mock.SetEnabled(false); err != nil {",
		"return err",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor.go must contain %q", snippet)
		}
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

func TestConnectedInfrastructureSkipsChildPlatformResourceTypes(t *testing.T) {
	data, err := os.ReadFile("connected_infrastructure.go")
	if err != nil {
		t.Fatalf("failed to read connected_infrastructure.go: %v", err)
	}
	source := string(data)
	requiredSnippets := []string{
		"func connectedInfrastructureExplicitType(resource unifiedresources.Resource) unifiedresources.ResourceType {",
		"explicitType := connectedInfrastructureExplicitType(resource)",
		`if explicitType != "" && explicitType != unifiedresources.ResourceTypeAgent {`,
		`if explicitType != "" && explicitType != unifiedresources.ResourceTypePBS {`,
		`if explicitType != "" && explicitType != unifiedresources.ResourceTypePMG {`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(source, snippet) {
			t.Fatalf("connected_infrastructure.go must contain %q", snippet)
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
			`(modelNodes[i].Disk.Total == 0 || currentDiskSource == "")`,
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
		"monitor_pve_guest_builders.go":  {"Pool:     strings.TrimSpace(res.Pool)"},
		"monitor_pve_guest_lxc.go":       {"Pool:     strings.TrimSpace(res.Pool)"},
		"monitor_pve_node_vm_builder.go": {"Pool:      vm.Pool"},
		"monitor_polling_containers.go":  {"Pool:     strings.TrimSpace(container.Pool)"},
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
			"vmIDToHostAgent := prevGuests.hostAgentsByVMID",
			"m.pollNodeVMsWithClusterResourceBuilder(ctx, instanceName, n.Node, vms, client, prevVMByID, vmIDToHostAgent)",
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
		`storeWrites := make([]metrics.WriteMetric, 0)`,
		`appendStoreWrite("dockerContainer", targetID, "cpu", value)`,
		`appendStoreWrite("dockerContainer", targetID, "diskwrite", metric.Value)`,
		`m.metricsStore.WriteBatchSync(storeWrites)`,
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
		`storeWrites := make([]metrics.WriteMetric, 0)`,
		`appendStoreWrite("agent", targetID, "cpu", value)`,
		`appendStoreWrite("agent", targetID, "diskwrite", metric.Value)`,
		`m.metricsStore.WriteBatchSync(storeWrites)`,
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
		`storeWrites := make([]metrics.WriteMetric, 0)`,
		`appendStoreWrite("vm", targetID, "cpu", value)`,
		`appendStoreWrite("vm", targetID, "diskwrite", metric.Value)`,
		`m.metricsStore.WriteBatchSync(storeWrites)`,
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
				`m.recordGuestMetric("vm", vm.ID, vm.CPU*100, vm.Memory.Usage, vm.Disk.Usage, -1, -1, -1, -1, now)`,
			},
		},
		{
			file: "monitor_polling_containers.go",
			snippets: []string{
				"if !shouldSkipNativeMockStateMetricWrites() {",
				`m.recordGuestMetric("container", ct.ID, ct.CPU*100, ct.Memory.Usage, ct.Disk.Usage, -1, -1, -1, -1, now)`,
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
				"func (m *Monitor) prewarmMockDashboardChartCaches() {",
				"_, _ = m.mockStorageSummaryCapacityTrendCached(24 * time.Hour)",
			},
		},
		{
			file: "mock_chart_history.go",
			snippets: []string{
				"func mockCanonicalMetricSeries(resourceType, resourceID, metricType string, timestamps []time.Time) []MetricPoint {",
				"values := canonicalMetricSeries(resourceType, resourceID, metricType, timestamps)",
				"return lttb(points, chartDownsampleTarget)",
				"func (m *Monitor) mockStorageSummaryCapacityTrend(duration time.Duration) []MetricPoint {",
				`usageValues := canonicalMetricSeries("storage", storageID, "usage", timestamps)`,
			},
		},
		{
			file: "mock_metrics_history.go",
			snippets: []string{
				`cpu := mock.SampleMetric("k8s", metricID, "cpu", ts)`,
				`memory := mock.SampleMetric("k8s", metricID, "memory", ts)`,
				`ms.Write("k8s", metricID, "memory", memory, ts)`,
				"m.prewarmMockDashboardChartCaches()",
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

func TestMockChartCacheRemainsMonitorOwned(t *testing.T) {
	data, err := os.ReadFile("monitor.go")
	if err != nil {
		t.Fatalf("failed to read monitor.go: %v", err)
	}
	source := string(data)

	for _, snippet := range []string{
		"mockChartCacheMu    sync.RWMutex",
		"mockChartMapCache   map[mockChartMetricMapCacheKey]map[string][]MetricPoint",
	} {
		if !strings.Contains(source, snippet) {
			t.Fatalf("monitor.go must contain %q", snippet)
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

	startedAt := time.Now().UTC().Add(-1 * time.Hour)
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

func TestReloadAndRuntimeContextStayOnCanonicalMonitoringPath(t *testing.T) {
	requiredSnippets := map[string][]string{
		"reload.go": {
			"config.LoadWithoutLoggingInit()",
		},
		"monitor.go": {
			"func (m *Monitor) setRuntimeContext(ctx context.Context, hub *websocket.Hub) {",
			"func (m *Monitor) getRuntimeContext() context.Context {",
			"m.setRuntimeContext(ctx, wsHub)",
		},
		"monitor_backups.go": {
			"parentCtx := m.getRuntimeContext()",
		},
		"monitor_pve.go": {
			"parentCtx := m.getRuntimeContext()",
		},
		"monitor_pve_storage.go": {
			"parentCtx := m.getRuntimeContext()",
		},
		"monitor_pbs_pmg.go": {
			"parentCtx := m.getRuntimeContext()",
		},
		"../config/config.go": {
			"func LoadWithoutLoggingInit() (*Config, error) {",
			"return load(false)",
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

// TestPollersHonorDisabledInstanceFlag asserts that the PVE, PBS, and PMG
// pollers skip instances whose `Disabled` flag is set, both at client
// initialization/reconnect time and on every per-instance iteration of the
// poll loop. The unified connections ledger surfaces `Disabled` as a
// `paused` state, so the monitoring runtime must not drive API calls or
// mark a disabled instance reachable; that behavior is what keeps the
// ledger's pause semantics honest across restarts.
func TestPollersHonorDisabledInstanceFlag(t *testing.T) {
	expectations := map[string][]string{
		"monitor_client_init.go": {
			"if pve.Disabled {",
			"if pbsInst.Disabled {",
			"if pmgInst.Disabled {",
		},
		"monitor_client_reconnect.go": {
			"if pve.Disabled {",
			"if pbsInst.Disabled {",
		},
		"monitor_pve.go": {
			"if instanceCfg.Disabled {",
		},
		"monitor_pbs_pmg.go": {
			"if instanceCfg.Disabled {",
		},
	}

	for file, snippets := range expectations {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("failed to read %s: %v", file, err)
		}
		source := string(data)
		for _, snippet := range snippets {
			if !strings.Contains(source, snippet) {
				t.Fatalf("%s must contain %q so disabled instances stay off the monitoring hot path", file, snippet)
			}
		}
	}
}

// TestMonitorUptimeFallsBackToCanonicalResourceUptime locks the contract
// that the websocket broadcast converter must surface canonical
// Resource.Uptime when no platform-specific carve-out is populated. The
// vSphere adapter writes uptime onto Resource.Uptime only (no
// vmware-specific UptimeSeconds), so without this fallback WS broadcasts
// would silently drop uptime for VMware-backed rows even though the
// canonical REST `/api/resources` payload exposes it.
func TestMonitorUptimeFallsBackToCanonicalResourceUptime(t *testing.T) {
	t.Run("falls through to Resource.Uptime when no source carve-out is set", func(t *testing.T) {
		resource := unifiedresources.Resource{Uptime: 123456}
		got := monitorUptime(resource)
		if got == nil || *got != 123456 {
			t.Fatalf("monitorUptime() = %v, want 123456 from canonical Resource.Uptime", got)
		}
	})

	t.Run("agent / proxmox / docker / k8s carve-outs keep precedence", func(t *testing.T) {
		resource := unifiedresources.Resource{
			Uptime: 999999,
			Agent:  &unifiedresources.AgentData{UptimeSeconds: 100},
		}
		got := monitorUptime(resource)
		if got == nil || *got != 100 {
			t.Fatalf("monitorUptime() = %v, want 100 from agent (precedence over canonical Uptime)", got)
		}

		resource.Agent = nil
		resource.Proxmox = &unifiedresources.ProxmoxData{Uptime: 200}
		got = monitorUptime(resource)
		if got == nil || *got != 200 {
			t.Fatalf("monitorUptime() = %v, want 200 from proxmox (precedence over canonical Uptime)", got)
		}
	})

	t.Run("returns nil when nothing populates", func(t *testing.T) {
		if got := monitorUptime(unifiedresources.Resource{}); got != nil {
			t.Fatalf("monitorUptime() = %v, want nil when all sources empty", got)
		}
	})
}

// Tenant monitors must stamp their org's identity into notification webhook
// payloads via an org-backed resolver, so PSA/ticket-bridge receivers get
// tenant routing identity from the payload boundary and display-name renames
// propagate without a monitor restart.
func TestTenantMonitorWiresOrgIdentityIntoNotifications(t *testing.T) {
	mtp, _ := newTestTenantPersistence(t)
	baseCfg := &config.Config{DataPath: t.TempDir()}
	mtm := NewMultiTenantMonitor(baseCfg, mtp, nil)
	t.Cleanup(mtm.Stop)

	if err := mtp.SaveOrganization(&models.Organization{
		ID:          "org-acme",
		DisplayName: "Acme Corp",
	}); err != nil {
		t.Fatalf("SaveOrganization(org-acme) error = %v", err)
	}

	monitor, err := mtm.GetMonitor("org-acme")
	if err != nil {
		t.Fatalf("GetMonitor(org-acme) error = %v", err)
	}

	nm := monitor.GetNotificationManager()
	if nm == nil {
		t.Fatal("expected tenant monitor to have a notification manager")
	}

	tenantID, tenantName := nm.TenantIdentity()
	if tenantID != "org-acme" {
		t.Fatalf("TenantIdentity() id = %q, want %q", tenantID, "org-acme")
	}
	if tenantName != "Acme Corp" {
		t.Fatalf("TenantIdentity() name = %q, want %q", tenantName, "Acme Corp")
	}

	// Display-name renames must propagate lazily without monitor restarts.
	if err := mtp.SaveOrganization(&models.Organization{
		ID:          "org-acme",
		DisplayName: "Acme Corp Renamed",
	}); err != nil {
		t.Fatalf("SaveOrganization(rename) error = %v", err)
	}
	if _, tenantName = nm.TenantIdentity(); tenantName != "Acme Corp Renamed" {
		t.Fatalf("TenantIdentity() name after rename = %q, want %q", tenantName, "Acme Corp Renamed")
	}
}

// Instance-wide notification settings (webhook security allowlist, public
// URL) propagate through ForEachMonitor; it must visit every live tenant
// monitor so no org's manager is left observing stale security settings.
func TestForEachMonitorVisitsAllTenantMonitors(t *testing.T) {
	mtm := &MultiTenantMonitor{
		monitors: map[string]*Monitor{
			"default": {},
			"org-a":   {},
			"org-b":   {},
			"nil-org": nil,
		},
	}

	visited := 0
	mtm.ForEachMonitor(func(m *Monitor) {
		if m == nil {
			t.Fatal("ForEachMonitor must skip nil monitors")
		}
		visited++
	})
	if visited != 3 {
		t.Fatalf("ForEachMonitor visited %d monitors, want 3", visited)
	}

	// Nil receiver and nil callback are safe no-ops.
	var nilMTM *MultiTenantMonitor
	nilMTM.ForEachMonitor(func(*Monitor) { t.Fatal("nil receiver must not invoke callback") })
	mtm.ForEachMonitor(nil)
}
