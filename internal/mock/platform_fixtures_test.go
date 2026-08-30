package mock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestUnifiedResourceSnapshotIncludesPlatformFixtures(t *testing.T) {
	previous := IsMockEnabled()
	mustSetEnabled(t, true)
	t.Cleanup(func() { mustSetEnabled(t, previous) })

	graph := CurrentFixtureGraph()
	legacyName := ""
	if len(graph.State.VMs) > 0 {
		legacyName = graph.State.VMs[0].Name
	} else if len(graph.State.Containers) > 0 {
		legacyName = graph.State.Containers[0].Name
	}
	if legacyName == "" {
		t.Fatal("expected canonical mock graph to include at least one legacy resource name")
	}
	if len(graph.PlatformFixtures.VMware.Hosts) == 0 {
		t.Fatal("expected canonical mock graph to include VMware host fixtures")
	}
	if len(graph.PlatformFixtures.VMware.Networks) == 0 {
		t.Fatal("expected canonical mock graph to include VMware network fixtures")
	}

	resources, freshness := UnifiedResourceSnapshot()
	if len(resources) == 0 {
		t.Fatal("expected unified resources in mock mode")
	}
	if freshness.IsZero() {
		t.Fatal("expected non-zero freshness for mock unified resources")
	}

	wantNames := map[string]bool{
		graph.PlatformFixtures.TrueNAS.System.Hostname: false,
		graph.PlatformFixtures.VMware.Hosts[0].Name:    false,
		graph.PlatformFixtures.VMware.Networks[0].Name: false,
		legacyName: false,
	}
	vmwareNetworkProjected := false
	for _, resource := range resources {
		if _, ok := wantNames[resource.Name]; ok {
			wantNames[resource.Name] = true
		}
		if resource.Name == graph.PlatformFixtures.VMware.Networks[0].Name &&
			resource.Type == unifiedresources.ResourceTypeNetwork &&
			slices.Contains(resource.Sources, unifiedresources.SourceVMware) {
			vmwareNetworkProjected = true
		}
	}
	for name, found := range wantNames {
		if !found {
			t.Fatalf("expected mock unified resources to include %q", name)
		}
	}
	if !vmwareNetworkProjected {
		t.Fatalf(
			"expected VMware fixture network %q to project as a canonical VMware network resource",
			graph.PlatformFixtures.VMware.Networks[0].Name,
		)
	}
}

func TestGenerateDisksForNodeUsesStableRealisticHealthEvidence(t *testing.T) {
	totals := map[string]int{}
	reportedWearout := 0
	unreportedWearout := 0
	lowWearout := 0
	rotationalModels := map[string]struct{}{
		"Seagate BarraCuda 4TB": {},
		"WD Red Pro 8TB":        {},
		"Toshiba X300 6TB":      {},
	}

	for i := 0; i < 300; i++ {
		node := models.Node{
			ID:       fmt.Sprintf("disk-health-node-%d", i),
			Name:     fmt.Sprintf("pve-%d", i),
			Instance: "mock-pve",
		}
		first := generateDisksForNode(node)
		second := generateDisksForNode(node)
		if len(first) != len(second) {
			t.Fatalf("disk count changed for stable node %q: %d then %d", node.ID, len(first), len(second))
		}

		for diskIndex := range first {
			disk := first[diskIndex]
			repeated := second[diskIndex]
			if disk.Health != repeated.Health || disk.Wearout != repeated.Wearout {
				t.Fatalf(
					"health evidence changed for %q disk %d: health=%q wearout=%d, then health=%q wearout=%d",
					node.ID,
					diskIndex,
					disk.Health,
					disk.Wearout,
					repeated.Health,
					repeated.Wearout,
				)
			}

			totals[disk.Health]++
			if disk.Wearout == 0 {
				t.Fatalf("mock disk %q encoded unreported endurance as spent 0%% life", disk.ID)
			}
			if _, rotational := rotationalModels[disk.Model]; rotational && disk.Wearout != -1 {
				t.Fatalf("rotational mock disk %q reports SSD life %d%%", disk.Model, disk.Wearout)
			}
			if disk.Wearout < 0 {
				unreportedWearout++
			} else {
				reportedWearout++
				if disk.Wearout <= 9 {
					lowWearout++
				}
			}
		}
	}

	if totals["PASSED"] <= totals["FAILED"]+totals["UNKNOWN"] {
		t.Fatalf("mock disk health is not predominantly healthy: %+v", totals)
	}
	for _, health := range []string{"PASSED", "UNKNOWN", "FAILED"} {
		if totals[health] == 0 {
			t.Fatalf("mock disk health does not exercise %q: %+v", health, totals)
		}
	}
	if reportedWearout == 0 || unreportedWearout == 0 || lowWearout == 0 {
		t.Fatalf(
			"mock wearout mix missing a required state: reported=%d unreported=%d low=%d",
			reportedWearout,
			unreportedWearout,
			lowWearout,
		)
	}
}

func TestUnifiedResourceSnapshotIncludesRuntimeNativeTabFixtures(t *testing.T) {
	previous := IsMockEnabled()
	mustSetEnabled(t, true)
	t.Cleanup(func() { mustSetEnabled(t, previous) })

	resources, _ := UnifiedResourceSnapshot()
	counts := make(map[unifiedresources.ResourceType]int)
	dockerStorageHosts := 0
	for _, resource := range resources {
		counts[resource.Type]++
		if resource.Docker != nil &&
			(resource.Docker.ImagesUsage != nil ||
				resource.Docker.ContainersUsage != nil ||
				resource.Docker.VolumesUsage != nil ||
				resource.Docker.BuildCacheUsage != nil) {
			dockerStorageHosts++
		}
	}

	requiredTypes := []unifiedresources.ResourceType{
		unifiedresources.ResourceTypeDockerImage,
		unifiedresources.ResourceTypeDockerVolume,
		unifiedresources.ResourceTypeDockerNetwork,
		unifiedresources.ResourceTypeDockerSwarmNode,
		unifiedresources.ResourceTypeK8sStorageClass,
		unifiedresources.ResourceTypeK8sPV,
		unifiedresources.ResourceTypeK8sPVC,
	}
	for _, resourceType := range requiredTypes {
		if counts[resourceType] == 0 {
			t.Fatalf("expected mock unified resources to include %s rows", resourceType)
		}
	}
	if dockerStorageHosts == 0 {
		t.Fatal("expected mock unified resources to include Docker / Podman storage usage on host rows")
	}
}

func TestUnifiedResourceSnapshotParentsDemoProxmoxWorkloads(t *testing.T) {
	previous := IsMockEnabled()
	mustSetEnabled(t, true)
	t.Cleanup(func() { mustSetEnabled(t, previous) })

	resources, _ := UnifiedResourceSnapshot()
	agentsByID := make(map[string]unifiedresources.Resource)
	for _, resource := range resources {
		if resource.Type == unifiedresources.ResourceTypeAgent {
			agentsByID[resource.ID] = resource
		}
	}

	proxmoxWorkloadCount := 0
	for _, resource := range resources {
		if resource.Type != unifiedresources.ResourceTypeVM && resource.Type != unifiedresources.ResourceTypeSystemContainer {
			continue
		}
		if !slices.Contains(resource.Sources, unifiedresources.SourceProxmox) {
			continue
		}
		proxmoxWorkloadCount++
		if resource.ParentID == nil || *resource.ParentID == "" {
			t.Fatalf("expected mock Proxmox workload %q (%s) to have a parent", resource.Name, resource.ID)
		}
		parent, ok := agentsByID[*resource.ParentID]
		if !ok {
			t.Fatalf("expected mock Proxmox workload %q parent %q to resolve to an agent", resource.Name, *resource.ParentID)
		}
		if resource.Proxmox != nil && parent.Proxmox != nil && resource.Proxmox.NodeName != parent.Proxmox.NodeName {
			t.Fatalf(
				"expected mock Proxmox workload %q parent node %q, got %q",
				resource.Name,
				resource.Proxmox.NodeName,
				parent.Proxmox.NodeName,
			)
		}
	}

	if proxmoxWorkloadCount == 0 {
		t.Fatal("expected mock unified resources to include Proxmox workloads")
	}
}

func TestSupplementalRecordsNormalizesVMwareAlias(t *testing.T) {
	records := SupplementalRecords(unifiedresources.DataSource("vmware-vsphere"))
	if len(records) == 0 {
		t.Fatal("expected records for vmware-vsphere alias")
	}
}

func TestSupplementalChangesNormalizesVMwareAlias(t *testing.T) {
	changes := SupplementalChanges(unifiedresources.DataSource("vmware-vsphere"))
	if len(changes) == 0 {
		t.Fatal("expected activity changes for vmware-vsphere alias")
	}
	if changes[0].Kind != unifiedresources.ChangeActivity || changes[0].SourceAdapter != unifiedresources.AdapterVMware {
		t.Fatalf("unexpected VMware activity change: %#v", changes[0])
	}
}

func TestFixtureGraphProjectsAvailabilityFixturesAsNetworkEndpoints(t *testing.T) {
	now := time.Date(2026, time.May, 6, 12, 0, 0, 0, time.UTC)

	graph := buildFixtureGraph(DefaultConfig, now)
	resources, freshness := graph.UnifiedResourceSnapshot()
	if freshness.IsZero() {
		t.Fatal("expected non-zero unified resource freshness")
	}

	var mqtt *unifiedresources.Resource
	var esphome *unifiedresources.Resource
	var door *unifiedresources.Resource
	for i := range resources {
		switch resources[i].Name {
		case "MQTT power meter":
			mqtt = &resources[i]
		case "ESPHome greenhouse sensor":
			esphome = &resources[i]
		case "Workshop door controller":
			door = &resources[i]
		}
	}
	if mqtt == nil {
		t.Fatal("expected MQTT power meter availability endpoint")
	}
	if mqtt.Type != unifiedresources.ResourceTypeNetworkEndpoint {
		t.Fatalf("MQTT resource type = %q, want network-endpoint", mqtt.Type)
	}
	if mqtt.Availability == nil || mqtt.Availability.Protocol != "tcp" || mqtt.Availability.Port != 1883 {
		t.Fatalf("unexpected MQTT availability metadata: %+v", mqtt.Availability)
	}
	if esphome == nil {
		t.Fatal("expected ESPHome greenhouse sensor availability endpoint")
	}
	if esphome.Availability == nil || esphome.Availability.Protocol != "tcp" || esphome.Availability.Port != 6053 {
		t.Fatalf("unexpected ESPHome availability metadata: %+v", esphome.Availability)
	}
	if !slices.Contains(esphome.Tags, "esphome") {
		t.Fatalf("expected ESPHome availability tag, got %+v", esphome.Tags)
	}
	if door == nil {
		t.Fatal("expected workshop door controller availability endpoint")
	}
	if door.Status != unifiedresources.StatusOffline {
		t.Fatalf("door controller status = %q, want offline", door.Status)
	}
	if door.Availability == nil || door.Availability.TargetID != "mock-availability-door-controller" {
		t.Fatalf("unexpected door availability metadata: %+v", door.Availability)
	}
	if len(door.Incidents) != 1 || door.Incidents[0].Code != "availability_unreachable" {
		t.Fatalf("expected availability incident, got %+v", door.Incidents)
	}
	if door.Canonical == nil || door.Canonical.PrimaryID != "availability:mock-availability-door-controller" {
		t.Fatalf("unexpected door canonical identity: %+v", door.Canonical)
	}
}

func TestAvailabilityFixtureRecordOmitsUnknownProbeTimes(t *testing.T) {
	now := time.Date(2026, time.May, 6, 12, 0, 0, 0, time.UTC)
	record, ok := availabilityFixtureRecord(AvailabilityFixture{
		Target: AvailabilityTargetFixture{
			ID:       "not-checked",
			Name:     "Not checked",
			Address:  "not-checked.example",
			Protocol: mockAvailabilityProbeICMP,
			Enabled:  true,
		},
	}, now)
	if !ok || record.Resource.Availability == nil {
		t.Fatalf("availabilityFixtureRecord() = (%+v, %t), want availability record", record, ok)
	}
	if record.Resource.Availability.LastChecked != nil {
		t.Fatalf("last checked = %v, want nil before the first probe", record.Resource.Availability.LastChecked)
	}
	if record.Resource.Availability.LastSuccess != nil {
		t.Fatalf("last success = %v, want nil before the first successful probe", record.Resource.Availability.LastSuccess)
	}
	if record.Resource.Availability.Evidence == nil ||
		record.Resource.Availability.Evidence.Completeness != operationaltrust.EvidencePartial {
		t.Fatalf("initial evidence = %+v, want partial envelope", record.Resource.Availability.Evidence)
	}
}

func TestFixtureGraphAttachesServiceAvailabilityFixturesToServiceResources(t *testing.T) {
	now := time.Date(2026, time.May, 6, 12, 0, 0, 0, time.UTC)

	graph := buildFixtureGraph(DefaultConfig, now)
	resources, _ := graph.UnifiedResourceSnapshot()

	// A configured check does not collapse into the resource it matches. It
	// keeps its own source-owned network-endpoint row, which owns probe status,
	// incidents and history, and additively projects a facet onto the matched
	// resource. Both therefore carry the same TargetID, so the matched service
	// has to be selected by resource type rather than by target alone.
	var dockerService *unifiedresources.Resource
	var kubernetesService *unifiedresources.Resource
	var dockerEndpoint *unifiedresources.Resource
	var kubernetesEndpoint *unifiedresources.Resource
	for i := range resources {
		availability := resources[i].Availability
		if availability == nil {
			continue
		}
		isEndpoint := resources[i].Type == unifiedresources.ResourceTypeNetworkEndpoint
		switch availability.TargetID {
		case "mock-availability-docker-frontend-service":
			if isEndpoint {
				dockerEndpoint = &resources[i]
			} else {
				dockerService = &resources[i]
			}
		case "mock-availability-k8s-checkout-api":
			if isEndpoint {
				kubernetesEndpoint = &resources[i]
			} else {
				kubernetesService = &resources[i]
			}
		}
	}

	if dockerEndpoint == nil {
		t.Fatal("Docker service check lost its source-owned network-endpoint row")
	}
	if kubernetesEndpoint == nil {
		t.Fatal("Kubernetes service check lost its source-owned network-endpoint row")
	}

	if dockerService == nil {
		t.Fatal("expected Docker service availability facet on curated mock service")
	}
	if dockerService.Type != unifiedresources.ResourceTypeDockerService {
		t.Fatalf("Docker service availability attached to %q, want docker-service", dockerService.Type)
	}
	if dockerService.Docker == nil || dockerService.Docker.ServiceID != "svc-frontend-0" {
		t.Fatalf("Docker service metadata = %+v, want service id svc-frontend-0", dockerService.Docker)
	}
	if dockerService.Availability.TargetKind != "service" ||
		dockerService.Availability.Protocol != "https" ||
		dockerService.Availability.Path != "/health" ||
		dockerService.Availability.LatencyMillis != 9 {
		t.Fatalf("unexpected Docker service availability facet: %+v", dockerService.Availability)
	}
	if !slices.Contains(dockerService.Sources, unifiedresources.SourceAvailability) {
		t.Fatalf("expected Docker service sources to include availability, got %+v", dockerService.Sources)
	}
	if dockerService.Availability.Certificate == nil ||
		dockerService.Availability.Certificate.TrustStatus != "trusted" ||
		dockerService.Availability.CertificateExpiryWarningDays != 30 {
		t.Fatalf("Docker service certificate posture = %+v", dockerService.Availability)
	}
	if dockerService.Availability.CorrelationState != unifiedresources.AvailabilityCorrelationAttached ||
		dockerService.Availability.Evidence == nil ||
		dockerService.Availability.Evidence.Subject.ResourceID != dockerService.ID {
		t.Fatalf("Docker service availability trust contract = %+v", dockerService.Availability)
	}
	if !hasChecksEdgeTo(dockerEndpoint, dockerService.ID) {
		t.Fatalf("Docker check relationships = %+v, want outgoing checks edge to %s", dockerEndpoint.Relationships, dockerService.ID)
	}

	if kubernetesService == nil {
		t.Fatal("expected Kubernetes service availability facet on curated mock service")
	}
	if kubernetesService.Type != unifiedresources.ResourceTypeK8sService {
		t.Fatalf("Kubernetes service availability attached to %q, want k8s-service", kubernetesService.Type)
	}
	if kubernetesService.Name != "checkout-api" {
		t.Fatalf("Kubernetes service name = %q, want checkout-api", kubernetesService.Name)
	}
	if kubernetesService.Availability.TargetKind != "service" ||
		kubernetesService.Availability.Protocol != "tcp" ||
		kubernetesService.Availability.Port != 8080 ||
		kubernetesService.Availability.LatencyMillis != 5 {
		t.Fatalf("unexpected Kubernetes service availability facet: %+v", kubernetesService.Availability)
	}
	if !slices.Contains(kubernetesService.Sources, unifiedresources.SourceAvailability) {
		t.Fatalf("expected Kubernetes service sources to include availability, got %+v", kubernetesService.Sources)
	}
	if !hasChecksEdgeTo(kubernetesEndpoint, kubernetesService.ID) {
		t.Fatalf("Kubernetes check relationships = %+v, want outgoing checks edge to %s", kubernetesEndpoint.Relationships, kubernetesService.ID)
	}
}

// hasChecksEdgeTo reports whether the source-owned check row carries the
// outgoing checks relationship to the resource it matched. The check row owns
// that edge; the matched resource carries only the projected facet.
func hasChecksEdgeTo(check *unifiedresources.Resource, matchedID string) bool {
	if check == nil {
		return false
	}
	for _, relationship := range check.Relationships {
		if relationship.Type == unifiedresources.RelChecks && relationship.TargetID == matchedID {
			return true
		}
	}
	return false
}

func TestBuildFixtureGraphRebasesPlatformFixtureTimestampsForDemoRuntime(t *testing.T) {
	now := time.Date(2026, time.March, 31, 17, 30, 0, 0, time.UTC)

	graph := buildFixtureGraph(DefaultConfig, now)

	if got := trueNASCollectedAt(graph.PlatformFixtures.TrueNAS); !got.Equal(now) {
		t.Fatalf("expected TrueNAS collectedAt %s, got %s", now, got)
	}
	if got := graph.PlatformFixtures.VMware.CollectedAt; !got.Equal(now) {
		t.Fatalf("expected VMware collectedAt %s, got %s", now, got)
	}
	if got := availabilityFixturesFreshness(graph.AvailabilityFixtures); got.IsZero() || got.Before(now.Add(-2*time.Minute)) || got.After(now) {
		t.Fatalf("expected availability fixture freshness near %s, got %s", now, got)
	}
	if got := graph.PlatformFixtures.TrueNAS.System.CollectedAt; got.IsZero() || got.Before(now.Add(-2*time.Minute)) || got.After(now) {
		t.Fatalf("expected rebased TrueNAS system collectedAt near %s, got %s", now, got)
	}
	if len(graph.PlatformFixtures.VMware.Hosts) == 0 || len(graph.PlatformFixtures.VMware.Hosts[0].RecentEvents) == 0 {
		t.Fatal("expected canonical VMware fixtures with recent events")
	}
	if got := graph.PlatformFixtures.VMware.Hosts[0].RecentEvents[0].CreatedAt; got.IsZero() || got.Before(now.Add(-2*time.Hour)) || got.After(now) {
		t.Fatalf("expected rebased VMware event timestamp near %s, got %s", now, got)
	}
}

func TestFixtureGraphUpdateMetricsKeepsPlatformFixtureFreshnessCurrent(t *testing.T) {
	cfg := DefaultConfig
	cfg.RandomMetrics = false

	start := time.Date(2026, time.March, 31, 17, 30, 0, 0, time.UTC)
	later := start.Add(12 * time.Minute)

	graph := buildFixtureGraph(cfg, start)
	graph.UpdateMetrics(cfg, later)

	if got := trueNASCollectedAt(graph.PlatformFixtures.TrueNAS); !got.Equal(later) {
		t.Fatalf("expected rebased TrueNAS collectedAt %s, got %s", later, got)
	}
	if got := graph.PlatformFixtures.VMware.CollectedAt; !got.Equal(later) {
		t.Fatalf("expected rebased VMware collectedAt %s, got %s", later, got)
	}
	if got := availabilityFixturesFreshness(graph.AvailabilityFixtures); got.IsZero() || got.Before(later.Add(-2*time.Minute)) || got.After(later) {
		t.Fatalf("expected availability fixture freshness near %s, got %s", later, got)
	}
	if len(graph.PlatformFixtures.VMware.Hosts) == 0 || len(graph.PlatformFixtures.VMware.Hosts[0].RecentEvents) == 0 {
		t.Fatal("expected canonical VMware fixtures with host events")
	}
	if got := graph.PlatformFixtures.VMware.Hosts[0].RecentEvents[0].CreatedAt; got.Before(later.Add(-2*time.Hour)) || got.After(later) {
		t.Fatalf("expected VMware event timestamp to remain fresh near %s, got %s", later, got)
	}
}

func TestBuildFixtureGraphRefreshesPlatformFixtureMetricsFromCanonicalModel(t *testing.T) {
	now := time.Date(2026, time.March, 31, 17, 30, 45, 0, time.UTC)

	graph := buildFixtureGraph(DefaultConfig, now)

	system := graph.PlatformFixtures.TrueNAS.System
	if got, want := system.CPUPercent, SampleMetric("agent", system.Hostname, "cpu", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed TrueNAS system cpu %.6f, got %.6f", want, got)
	}

	pool := graph.PlatformFixtures.TrueNAS.Pools[0]
	poolID := TrueNASPoolMetricID(system.Hostname, pool.Name)
	if got, want := pool.UsedBytes, bytesFromPercent(pool.TotalBytes, SampleMetric("storage", poolID, "usage", now)); got != want {
		t.Fatalf("expected refreshed TrueNAS pool used bytes %d, got %d", want, got)
	}

	dataset := graph.PlatformFixtures.TrueNAS.Datasets[0]
	datasetID := TrueNASDatasetMetricID(system.Hostname, dataset.Name)
	datasetTotal := dataset.UsedBytes + dataset.AvailBytes
	if got, want := dataset.UsedBytes, bytesFromPercent(datasetTotal, SampleMetric("storage", datasetID, "usage", now)); got != want {
		t.Fatalf("expected refreshed TrueNAS dataset used bytes %d, got %d", want, got)
	}

	app := graph.PlatformFixtures.TrueNAS.Apps[1]
	if app.Stats == nil {
		t.Fatal("expected refreshed TrueNAS app stats")
	}
	appID := TrueNASAppMetricID(system.Hostname, app)
	if got, want := app.Stats.CPUPercent, SampleMetric("dockerContainer", appID, "cpu", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed TrueNAS app cpu %.6f, got %.6f", want, got)
	}
	if got, want := app.Stats.MemoryBytes, bytesFromPercent(system.MemoryTotalBytes, SampleMetric("dockerContainer", appID, "memory", now)); got != want {
		t.Fatalf("expected refreshed TrueNAS app memory bytes %d, got %d", want, got)
	}

	disk := graph.PlatformFixtures.TrueNAS.Disks[0]
	if got, want := disk.Temperature, int(math.Round(SampleMetric("disk", disk.Serial, "smart_temp", now))); got != want {
		t.Fatalf("expected refreshed TrueNAS disk temperature %d, got %d", want, got)
	}

	host := graph.PlatformFixtures.VMware.Hosts[0]
	if host.Metrics == nil || host.Metrics.CPUPercent == nil {
		t.Fatal("expected refreshed VMware host metrics")
	}
	if got, want := *host.Metrics.CPUPercent, SampleMetric("agent", "vc-mock-1:host:host-101", "cpu", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed VMware host cpu %.6f, got %.6f", want, got)
	}
	// Mock fixture must also synthesize uptime so the workloads table
	// renders real "N days" cells for vSphere hosts and VMs instead of
	// blank "0s" placeholders. Hosts get sys.uptime-style uptime only;
	// guest disk usage is VM-only because ESXi exposes no `guest` shape.
	if host.Metrics.UptimeSeconds == nil || *host.Metrics.UptimeSeconds <= 0 {
		t.Fatalf("expected refreshed VMware host uptime seconds, got %+v", host.Metrics.UptimeSeconds)
	}
	if host.Metrics.DiskTotalBytes != nil || host.Metrics.DiskUsedBytes != nil || host.Metrics.DiskPercent != nil {
		t.Fatalf("expected VMware host guest filesystem fields to stay nil, got total=%+v used=%+v percent=%+v", host.Metrics.DiskTotalBytes, host.Metrics.DiskUsedBytes, host.Metrics.DiskPercent)
	}

	var poweredOnVM *struct{}
	for _, vm := range graph.PlatformFixtures.VMware.VMs {
		if vm.PowerState != "POWERED_ON" || vm.Metrics == nil {
			continue
		}
		if vm.Metrics.UptimeSeconds == nil || *vm.Metrics.UptimeSeconds <= 0 {
			t.Fatalf("expected powered-on VMware VM %q to surface uptime, got %+v", vm.Name, vm.Metrics.UptimeSeconds)
		}
		if vm.Metrics.DiskTotalBytes == nil || *vm.Metrics.DiskTotalBytes <= 0 {
			t.Fatalf("expected powered-on VMware VM %q to surface guest disk total, got %+v", vm.Name, vm.Metrics.DiskTotalBytes)
		}
		if vm.Metrics.DiskUsedBytes == nil || *vm.Metrics.DiskUsedBytes < 0 {
			t.Fatalf("expected powered-on VMware VM %q to surface guest disk used, got %+v", vm.Name, vm.Metrics.DiskUsedBytes)
		}
		if vm.Metrics.DiskPercent == nil {
			t.Fatalf("expected powered-on VMware VM %q to surface guest disk percent", vm.Name)
		}
		marker := struct{}{}
		poweredOnVM = &marker
		break
	}
	if poweredOnVM == nil {
		t.Fatal("expected at least one powered-on VMware VM fixture to assert uptime + guest disk projection")
	}

	for _, vm := range graph.PlatformFixtures.VMware.VMs {
		if vm.PowerState != "POWERED_OFF" || vm.Metrics == nil {
			continue
		}
		if vm.Metrics.UptimeSeconds != nil {
			t.Fatalf("expected powered-off VMware VM %q to drop uptime, got %+v", vm.Name, vm.Metrics.UptimeSeconds)
		}
		if vm.Metrics.DiskTotalBytes != nil || vm.Metrics.DiskUsedBytes != nil || vm.Metrics.DiskPercent != nil {
			t.Fatalf("expected powered-off VMware VM %q to drop guest disk fields, got total=%+v used=%+v percent=%+v", vm.Name, vm.Metrics.DiskTotalBytes, vm.Metrics.DiskUsedBytes, vm.Metrics.DiskPercent)
		}
		break
	}

	datastore := graph.PlatformFixtures.VMware.Datastores[0]
	wantFree := datastore.Capacity - bytesFromPercent(datastore.Capacity, SampleMetric("storage", "vc-mock-1:datastore:"+datastore.Datastore, "usage", now))
	if datastore.FreeSpace != wantFree {
		t.Fatalf("expected refreshed VMware datastore free space %d, got %d", wantFree, datastore.FreeSpace)
	}
}

func TestBuildFixtureGraphRefreshesStateMetricsFromCanonicalModel(t *testing.T) {
	now := time.Date(2026, time.March, 31, 17, 30, 45, 0, time.UTC)

	graph := buildFixtureGraph(DefaultConfig, now)

	node := graph.State.Nodes[0]
	if got, want := node.CPU*100, SampleMetric("node", node.ID, "cpu", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed node cpu %.6f, got %.6f", want, got)
	}
	if got, want := node.Memory.Usage, SampleMetric("node", node.ID, "memory", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed node memory %.6f, got %.6f", want, got)
	}

	var runningVMID string
	var runningVMCPU float64
	var runningVMMemory float64
	for _, vm := range graph.State.VMs {
		if vm.Status != "running" {
			continue
		}
		runningVMID = vm.ID
		runningVMCPU = vm.CPU * 100
		runningVMMemory = vm.Memory.Usage
		break
	}
	if runningVMID == "" {
		t.Fatal("expected at least one running VM in canonical fixture graph")
	}
	if got, want := runningVMCPU, SampleMetric("vm", runningVMID, "cpu", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed vm cpu %.6f, got %.6f", want, got)
	}
	if got, want := runningVMMemory, SampleMetric("vm", runningVMID, "memory", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed vm memory %.6f, got %.6f", want, got)
	}

	var onlineHostID string
	var onlineHostCPU float64
	var onlineHostMemory float64
	for _, host := range graph.State.Hosts {
		if host.Status == "offline" {
			continue
		}
		onlineHostID = host.ID
		onlineHostCPU = host.CPUUsage
		onlineHostMemory = host.Memory.Usage
		break
	}
	if onlineHostID == "" {
		t.Fatal("expected at least one online host in canonical fixture graph")
	}
	if got, want := onlineHostCPU, SampleMetric("agent", onlineHostID, "cpu", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed host cpu %.6f, got %.6f", want, got)
	}
	if got, want := onlineHostMemory, SampleMetric("agent", onlineHostID, "memory", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed host memory %.6f, got %.6f", want, got)
	}

	storage := graph.State.Storage[0]
	if got, want := storage.Usage, SampleMetric("storage", storage.ID, "usage", now); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected refreshed storage usage %.6f, got %.6f", want, got)
	}

	disk := graph.State.PhysicalDisks[0]
	resourceID := disk.Serial
	if resourceID == "" {
		resourceID = disk.ID
	}
	if resourceID == "" {
		resourceID = fmt.Sprintf("%s-%s-%s", disk.Instance, disk.Node, disk.DevPath)
	}
	if got, want := disk.Temperature, int(math.Round(SampleMetric("disk", resourceID, "smart_temp", now))); got != want {
		t.Fatalf("expected refreshed physical disk temperature %d, got %d", want, got)
	}
}

func TestCloneTrueNASFixtureSnapshotIsolatesPoolHealthEvidence(t *testing.T) {
	original := truenas.FixtureSnapshot{Pools: []truenas.Pool{{
		Name: "tank",
		Scan: &truenas.PoolScan{
			Function: "RESILVER",
			State:    "SCANNING",
		},
		VDevs: []truenas.PoolVDev{{
			GUID:   "leaf-guid",
			Status: "UNAVAIL",
		}},
		DiskMembers: []truenas.PoolDiskMember{{
			GUID:   "leaf-guid",
			Status: "UNAVAIL",
		}},
	}}}

	cloned := cloneTrueNASFixtureSnapshot(original)
	cloned.Pools[0].Scan.State = "FINISHED"
	cloned.Pools[0].VDevs[0].Status = "ONLINE"
	cloned.Pools[0].DiskMembers[0].Status = "ONLINE"

	if original.Pools[0].Scan.State != "SCANNING" ||
		original.Pools[0].VDevs[0].Status != "UNAVAIL" ||
		original.Pools[0].DiskMembers[0].Status != "UNAVAIL" {
		t.Fatalf("clone mutated original pool evidence: %+v", original.Pools[0])
	}
}

func TestFixtureGraphMetricCohortsBoundPVEChurnAndCoverTheEstate(t *testing.T) {
	cfg := DefaultConfig
	cfg.NodeCount = 10
	cfg.VMsPerNode = 2
	cfg.LXCsPerNode = 2
	cfg.DockerHostCount = 0
	cfg.GenericHostCount = 0
	cfg.K8sClusterCount = 0
	cfg.RandomMetrics = true

	base := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	graph := buildFixtureGraph(cfg, base)
	initialVMSeen := make(map[string]time.Time, len(graph.State.VMs))
	initialContainerSeen := make(map[string]time.Time, len(graph.State.Containers))
	for _, vm := range graph.State.VMs {
		initialVMSeen[vm.ID] = vm.LastSeen
	}
	for _, container := range graph.State.Containers {
		initialContainerSeen[container.ID] = container.LastSeen
	}

	graph.UpdateMetricCohort(cfg, base.Add(cfg.UpdateInterval), 0, 5)
	selectedNodes := map[string]struct{}{
		graph.State.Nodes[0].Name: {},
		graph.State.Nodes[5].Name: {},
	}
	changedGuests := 0
	for _, vm := range graph.State.VMs {
		_, selected := selectedNodes[vm.Node]
		changed := !vm.LastSeen.Equal(initialVMSeen[vm.ID])
		if changed {
			changedGuests++
		}
		if changed != selected {
			t.Fatalf("VM %s on %s changed=%v, selected=%v", vm.ID, vm.Node, changed, selected)
		}
	}
	for _, container := range graph.State.Containers {
		_, selected := selectedNodes[container.Node]
		changed := !container.LastSeen.Equal(initialContainerSeen[container.ID])
		if changed {
			changedGuests++
		}
		if changed != selected {
			t.Fatalf("container %s on %s changed=%v, selected=%v", container.ID, container.Node, changed, selected)
		}
	}
	if total := len(graph.State.VMs) + len(graph.State.Containers); changedGuests <= 0 || changedGuests >= total/2 {
		t.Fatalf("expected a bounded non-empty guest cohort, changed=%d total=%d", changedGuests, total)
	}

	for cohort := 1; cohort < 5; cohort++ {
		graph.UpdateMetricCohort(cfg, base.Add(time.Duration(cohort+1)*cfg.UpdateInterval), cohort, 5)
	}
	for _, vm := range graph.State.VMs {
		if !vm.LastSeen.After(initialVMSeen[vm.ID]) {
			t.Fatalf("VM %s was not refreshed during a full cohort rotation", vm.ID)
		}
	}
	for _, container := range graph.State.Containers {
		if !container.LastSeen.After(initialContainerSeen[container.ID]) {
			t.Fatalf("container %s was not refreshed during a full cohort rotation", container.ID)
		}
	}
}

func TestSupplementalRefreshCadenceTracksFreshnessBudget(t *testing.T) {
	t.Cleanup(func() { setMockUpdateInterval(DefaultConfig.UpdateInterval) })

	cases := []struct {
		name         string
		interval     time.Duration
		cohortCount  int
		everyTick    bool
		wantInterval time.Duration
	}{
		{"default rotation stays within budget", 2 * time.Second, mockMetricCohortCount, false, 20 * time.Second},
		{"rotation at the budget keeps the cohort cadence", 12 * time.Second, mockMetricCohortCount, false, 120 * time.Second},
		{"rotation past the budget refreshes per tick", 13 * time.Second, mockMetricCohortCount, true, 13 * time.Second},
		{"public demo interval refreshes per tick", 60 * time.Second, mockMetricCohortCount, true, 60 * time.Second},
		{"degenerate cohort count refreshes per tick", 2 * time.Second, 1, true, 2 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := supplementalRefreshEveryTick(tc.interval, tc.cohortCount); got != tc.everyTick {
				t.Fatalf("supplementalRefreshEveryTick(%s, %d) = %v, want %v", tc.interval, tc.cohortCount, got, tc.everyTick)
			}
			if tc.cohortCount == mockMetricCohortCount {
				setMockUpdateInterval(tc.interval)
				if got := SupplementalRefreshInterval(); got != tc.wantInterval {
					t.Fatalf("SupplementalRefreshInterval() = %s at %s ticks, want %s", got, tc.interval, tc.wantInterval)
				}
			}
		})
	}
}

func TestFixtureGraphSlowTicksKeepSupplementalFixturesFresh(t *testing.T) {
	t.Cleanup(func() { setMockUpdateInterval(DefaultConfig.UpdateInterval) })

	cfg := DefaultConfig
	cfg.NodeCount = 4
	cfg.VMsPerNode = 1
	cfg.LXCsPerNode = 1
	cfg.DockerHostCount = 0
	cfg.GenericHostCount = 0
	cfg.K8sClusterCount = 0

	base := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	// Fast ticks: a non-zero cohort must keep the once-per-rotation cadence.
	graph := buildFixtureGraph(cfg, base)
	collectedAt := trueNASCollectedAt(graph.PlatformFixtures.TrueNAS)
	graph.UpdateMetricCohort(cfg, base.Add(cfg.UpdateInterval), 3, mockMetricCohortCount)
	if got := trueNASCollectedAt(graph.PlatformFixtures.TrueNAS); !got.Equal(collectedAt) {
		t.Fatalf("fast ticks rebased supplemental fixtures on a non-zero cohort: %s -> %s", collectedAt, got)
	}

	// Slow ticks (the public demo's PULSE_MOCK_UPDATE_INTERVAL shape): the
	// rotation outlives the registry's freshness budget, so every cohort must
	// rebase provider-backed fixtures or their rows read as stale sources.
	cfg.UpdateInterval = 60 * time.Second
	graph = buildFixtureGraph(cfg, base)
	tick := base.Add(cfg.UpdateInterval)
	graph.UpdateMetricCohort(cfg, tick, 3, mockMetricCohortCount)
	if got := trueNASCollectedAt(graph.PlatformFixtures.TrueNAS); !got.Equal(tick) {
		t.Fatalf("slow ticks left TrueNAS fixtures at %s, want rebase to %s", got, tick)
	}
	if got := graph.PlatformFixtures.VMware.CollectedAt; !got.Equal(tick) {
		t.Fatalf("slow ticks left VMware fixtures at %s, want rebase to %s", got, tick)
	}
	if got := availabilityFixturesFreshness(graph.AvailabilityFixtures); !got.Equal(tick) {
		t.Fatalf("slow ticks left availability fixtures at %s, want rebase to %s", got, tick)
	}
}

func TestFixtureGraphMetricCohortProducesSparseUnifiedResourceChanges(t *testing.T) {
	cfg := DefaultConfig
	cfg.NodeCount = 50
	cfg.VMsPerNode = 10
	cfg.LXCsPerNode = 8
	cfg.RandomMetrics = true

	base := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	graph := buildFixtureGraph(cfg, base)
	before, _ := graph.UnifiedResourceSnapshot()

	graph.UpdateMetricCohort(cfg, base.Add(cfg.UpdateInterval), 0, mockMetricCohortCount)
	after, _ := graph.UnifiedResourceSnapshot()

	encodeBroadcastRelevant := func(resource unifiedresources.Resource) []byte {
		encoded, err := json.Marshal(resource)
		if err != nil {
			t.Fatalf("marshal resource %s: %v", resource.ID, err)
		}
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &payload); err != nil {
			t.Fatalf("decode resource %s: %v", resource.ID, err)
		}
		// ResourceFrontend intentionally omits the registry's ingestion timestamp.
		// It is regenerated whenever a throwaway mock registry is built and is not
		// part of the WebSocket resource payload.
		delete(payload, "updatedAt")
		encoded, err = json.Marshal(payload)
		if err != nil {
			t.Fatalf("re-marshal resource %s: %v", resource.ID, err)
		}
		return encoded
	}

	encodedBefore := make(map[string][]byte, len(before))
	for _, resource := range before {
		encodedBefore[resource.ID] = encodeBroadcastRelevant(resource)
	}

	changed := 0
	for _, resource := range after {
		encoded := encodeBroadcastRelevant(resource)
		if !bytes.Equal(encodedBefore[resource.ID], encoded) {
			changed++
		}
	}

	if changed == 0 || changed >= len(after)/2 {
		t.Fatalf("expected sparse unified resource churn, changed=%d total=%d", changed, len(after))
	}
}
