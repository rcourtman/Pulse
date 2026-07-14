package truenas

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestProviderFeatureFlagGatesFixtureRecords(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(false)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	provider := NewDefaultProvider()
	if got := len(provider.Records()); got != 0 {
		t.Fatalf("expected no records with feature disabled, got %d", got)
	}
}

func TestFeatureFlagDefaultsTrueNASOnUnlessExplicitlyDisabled(t *testing.T) {
	if !parseFeatureEnabled("") {
		t.Fatal("expected empty TrueNAS feature env to default to enabled")
	}
	if !parseFeatureEnabled("   ") {
		t.Fatal("expected blank TrueNAS feature env to default to enabled")
	}
	if parseFeatureEnabled("false") {
		t.Fatal("expected explicit false TrueNAS feature env to disable the integration")
	}
}

func TestRegistryIngestRecordsTreatsTrueNASAsGenericDataSource(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	fixtures := DefaultFixtures()
	provider := NewProvider(fixtures)
	records := provider.Records()
	if len(records) == 0 {
		t.Fatal("expected fixture records from provider")
	}

	registry := unifiedresources.NewRegistry(unifiedresources.NewMemoryStore())
	registry.IngestRecords(unifiedresources.SourceTrueNAS, records)

	resources := registry.List()
	wantCount := 1 + len(fixtures.Pools) + len(fixtures.Datasets) + len(fixtures.Disks) + len(fixtures.Apps) + len(fixtures.VMs) + len(fixtures.Shares)
	if len(resources) != wantCount {
		t.Fatalf("expected %d resources, got %d", wantCount, len(resources))
	}

	system := requireResource(t, resources, unifiedresources.ResourceTypeAgent, fixtures.System.Hostname)
	assertSourceTracking(t, *system, unifiedresources.SourceTrueNAS)
	if system.ChildCount != len(fixtures.Pools)+len(fixtures.Apps)+len(fixtures.VMs) {
		t.Fatalf("expected system child count %d, got %d", len(fixtures.Pools)+len(fixtures.Apps)+len(fixtures.VMs), system.ChildCount)
	}
	if system.TrueNAS == nil {
		t.Fatal("expected TrueNAS metadata on system record")
	}
	if system.TrueNAS.UptimeSeconds != fixtures.System.UptimeSeconds {
		t.Fatalf("expected uptime %d, got %d", fixtures.System.UptimeSeconds, system.TrueNAS.UptimeSeconds)
	}
	if system.TrueNAS.Version != fixtures.System.Version {
		t.Fatalf("expected version %q, got %q", fixtures.System.Version, system.TrueNAS.Version)
	}
	if system.Status != unifiedresources.StatusWarning {
		t.Fatalf("expected system status warning due to degraded storage, got %q", system.Status)
	}
	if system.TrueNAS.StorageRisk == nil {
		t.Fatal("expected rolled-up TrueNAS storage risk on system record")
	}
	if system.TrueNAS.StorageRisk.Level != "warning" {
		t.Fatalf("expected warning storage risk on system record, got %+v", system.TrueNAS.StorageRisk)
	}
	if system.TrueNAS.StorageRiskSummary == "" {
		t.Fatal("expected non-empty storage risk summary on system record")
	}
	if system.TrueNAS.StoragePostureSummary != system.TrueNAS.StorageRiskSummary {
		t.Fatalf("expected posture summary to match risk summary, got risk=%q posture=%q", system.TrueNAS.StorageRiskSummary, system.TrueNAS.StoragePostureSummary)
	}
	if !system.TrueNAS.ProtectionReduced || system.TrueNAS.ProtectionSummary == "" {
		t.Fatalf("expected protection semantics on system record, got %+v", system.TrueNAS)
	}
	if system.TrueNAS.StorageRiskSummary != system.TrueNAS.ProtectionSummary {
		t.Fatalf("expected risk summary to prefer protection summary, got risk=%q protection=%q", system.TrueNAS.StorageRiskSummary, system.TrueNAS.ProtectionSummary)
	}
	if len(system.Incidents) != 2 {
		t.Fatalf("expected 2 native incidents on system record, got %+v", system.Incidents)
	}

	pool := requireResource(t, resources, unifiedresources.ResourceTypeStorage, "tank")
	assertSourceTracking(t, *pool, unifiedresources.SourceTrueNAS)
	if pool.ParentID == nil || *pool.ParentID != system.ID {
		t.Fatalf("expected pool parent %q, got %+v", system.ID, pool.ParentID)
	}
	if pool.Storage == nil {
		t.Fatal("expected pool storage metadata")
	}
	if pool.Storage.ZFSPoolState != "ONLINE" {
		t.Fatalf("expected ZFSPoolState ONLINE, got %q", pool.Storage.ZFSPoolState)
	}
	if pool.Storage.Platform != "truenas" || pool.Storage.Topology != "pool" || pool.Storage.Protection != "zfs" {
		t.Fatalf("expected canonical storage metadata on pool, got %+v", pool.Storage)
	}
	assertDiskMetric(t, pool.Metrics, 30*1024*1024*1024*1024, 12*1024*1024*1024*1024)

	archivePool := requireResource(t, resources, unifiedresources.ResourceTypeStorage, "archive")
	if len(archivePool.Incidents) != 2 {
		t.Fatalf("expected archive pool incidents to include pool + disk alerts, got %+v", archivePool.Incidents)
	}
	if !hasIncidentCode(archivePool.Incidents, "truenas_volume_status") || !hasIncidentCode(archivePool.Incidents, "truenas_smart") {
		t.Fatalf("expected archive pool incidents to include volume and smart alerts, got %+v", archivePool.Incidents)
	}

	dataset := requireResource(t, resources, unifiedresources.ResourceTypeStorage, "tank/apps")
	assertSourceTracking(t, *dataset, unifiedresources.SourceTrueNAS)
	if dataset.ParentID == nil || *dataset.ParentID != pool.ID {
		t.Fatalf("expected dataset parent %q, got %+v", pool.ID, dataset.ParentID)
	}
	assertDiskMetric(t, dataset.Metrics, 18*1024*1024*1024*1024, 5*1024*1024*1024*1024)

	app := requireResource(t, resources, unifiedresources.ResourceTypeAppContainer, "Nextcloud")
	assertSourceTracking(t, *app, unifiedresources.SourceTrueNAS)
	if app.ParentID == nil || *app.ParentID != system.ID {
		t.Fatalf("expected app parent %q, got %+v", system.ID, app.ParentID)
	}
	if app.Docker == nil {
		t.Fatal("expected DockerData on TrueNAS app resource")
	}
	if app.Docker.ContainerID != "nextcloud" {
		t.Fatalf("expected app container ID nextcloud, got %q", app.Docker.ContainerID)
	}
	if app.Docker.Runtime != "docker" {
		t.Fatalf("expected app runtime docker, got %q", app.Docker.Runtime)
	}
	if app.TrueNAS == nil || app.TrueNAS.App == nil {
		t.Fatal("expected native TrueNAS app metadata on TrueNAS app resource")
	}
	if app.TrueNAS.App.ID != "nextcloud" || app.TrueNAS.App.State != "RUNNING" {
		t.Fatalf("unexpected native TrueNAS app identity/state: %+v", app.TrueNAS.App)
	}
	if app.TrueNAS.App.HumanVersion != "29.0.7" || !app.TrueNAS.App.UpgradeAvailable || !app.TrueNAS.App.ImageUpdatesAvailable {
		t.Fatalf("unexpected native TrueNAS app version/update metadata: %+v", app.TrueNAS.App)
	}
	if app.TrueNAS.App.ContainerCount != 2 || len(app.TrueNAS.App.Containers) != 2 {
		t.Fatalf("expected native TrueNAS app container inventory, got %+v", app.TrueNAS.App)
	}
	if len(app.TrueNAS.App.UsedPorts) != 1 || len(app.TrueNAS.App.UsedPorts[0].HostPorts) != 1 || app.TrueNAS.App.UsedPorts[0].HostPorts[0].HostPort != 30443 {
		t.Fatalf("unexpected native TrueNAS app ports: %+v", app.TrueNAS.App.UsedPorts)
	}
	if app.Status != unifiedresources.StatusOnline {
		t.Fatalf("expected Nextcloud status online, got %q", app.Status)
	}
	if app.Metrics == nil || app.Metrics.CPU == nil || app.Metrics.CPU.Percent != 18 {
		t.Fatalf("expected Nextcloud CPU metrics, got %+v", app.Metrics)
	}
	appTarget := registry.MetricsTarget(app.ID)
	if appTarget == nil || appTarget.ResourceType != "app-container" || appTarget.ResourceID != "system:truenas-main/app:nextcloud" {
		t.Fatalf("expected canonical Nextcloud metrics target, got %+v", appTarget)
	}
	if app.Docker.NetInRate != 2_100_000 || app.Docker.NetOutRate != 1_250_000 {
		t.Fatalf("expected Nextcloud network rates, got in=%v out=%v", app.Docker.NetInRate, app.Docker.NetOutRate)
	}

	vm := requireResource(t, resources, unifiedresources.ResourceTypeVM, "windows-lab")
	assertSourceTracking(t, *vm, unifiedresources.SourceTrueNAS)
	if vm.ParentID == nil || *vm.ParentID != system.ID {
		t.Fatalf("expected VM parent %q, got %+v", system.ID, vm.ParentID)
	}
	if vm.TrueNAS == nil || vm.TrueNAS.VM == nil {
		t.Fatal("expected native TrueNAS VM metadata on VM resource")
	}
	if vm.TrueNAS.VM.ID != "42" || vm.TrueNAS.VM.State != "RUNNING" {
		t.Fatalf("unexpected native TrueNAS VM identity/state: %+v", vm.TrueNAS.VM)
	}
	if vm.TrueNAS.VM.VCPUs != 4 || vm.TrueNAS.VM.MemoryBytes != 8*1024*1024*1024 {
		t.Fatalf("unexpected native TrueNAS VM compute metadata: %+v", vm.TrueNAS.VM)
	}
	if !vm.TrueNAS.VM.Autostart || !vm.TrueNAS.VM.SecureBoot || !vm.TrueNAS.VM.TrustedPlatformModule {
		t.Fatalf("unexpected native TrueNAS VM flags: %+v", vm.TrueNAS.VM)
	}
	if vm.TrueNAS.VM.DiskCount != 1 || vm.TrueNAS.VM.NICCount != 1 || vm.TrueNAS.VM.DisplayCount != 1 {
		t.Fatalf("unexpected native TrueNAS VM device counts: %+v", vm.TrueNAS.VM)
	}
	if vm.Status != unifiedresources.StatusOnline {
		t.Fatalf("expected windows-lab status online, got %q", vm.Status)
	}

	share := requireResource(t, resources, unifiedresources.ResourceTypeNetworkShare, "Media")
	assertSourceTracking(t, *share, unifiedresources.SourceTrueNAS)
	if share.ParentID == nil || *share.ParentID != requireResource(t, resources, unifiedresources.ResourceTypeStorage, "tank/media").ID {
		t.Fatalf("expected share parent to be tank/media dataset, got %+v", share.ParentID)
	}
	if share.TrueNAS == nil || share.TrueNAS.Share == nil {
		t.Fatal("expected native TrueNAS share metadata on network-share resource")
	}
	if share.TrueNAS.Share.Protocol != "SMB" || share.TrueNAS.Share.Path != "/mnt/tank/media" {
		t.Fatalf("unexpected native TrueNAS share metadata: %+v", share.TrueNAS.Share)
	}
	if !share.TrueNAS.Share.Enabled || !share.TrueNAS.Share.Browsable || !share.TrueNAS.Share.AccessBasedEnumeration || !share.TrueNAS.Share.AuditEnabled {
		t.Fatalf("expected native SMB share flags, got %+v", share.TrueNAS.Share)
	}
	if share.Status != unifiedresources.StatusOnline {
		t.Fatalf("expected Media share status online, got %q", share.Status)
	}

	disk := requireResource(t, resources, unifiedresources.ResourceTypePhysicalDisk, "sda")
	assertSourceTracking(t, *disk, unifiedresources.SourceTrueNAS)
	if disk.ParentID == nil || *disk.ParentID != pool.ID {
		t.Fatalf("expected disk parent %q, got %+v", pool.ID, disk.ParentID)
	}
	if disk.PhysicalDisk == nil {
		t.Fatal("expected physical disk metadata")
	}
	if disk.PhysicalDisk.DevPath != "/dev/sda" {
		t.Fatalf("expected dev path /dev/sda, got %q", disk.PhysicalDisk.DevPath)
	}
	if disk.PhysicalDisk.Model != "Seagate Exos X18" {
		t.Fatalf("expected model %q, got %q", "Seagate Exos X18", disk.PhysicalDisk.Model)
	}
	if disk.PhysicalDisk.Serial != "ZL0A1234" {
		t.Fatalf("expected serial %q, got %q", "ZL0A1234", disk.PhysicalDisk.Serial)
	}
	if disk.PhysicalDisk.DiskType != "sata" {
		t.Fatalf("expected disk type %q, got %q", "sata", disk.PhysicalDisk.DiskType)
	}
	if disk.PhysicalDisk.SizeBytes != 16*1024*1024*1024*1024 {
		t.Fatalf("expected size bytes %d, got %d", int64(16*1024*1024*1024*1024), disk.PhysicalDisk.SizeBytes)
	}
	if disk.PhysicalDisk.Health != "PASSED" {
		t.Fatalf("expected health PASSED, got %q", disk.PhysicalDisk.Health)
	}
	if disk.PhysicalDisk.Wearout != -1 {
		t.Fatalf("expected wearout -1, got %d", disk.PhysicalDisk.Wearout)
	}
	if disk.PhysicalDisk.RPM != 7200 {
		t.Fatalf("expected rpm 7200, got %d", disk.PhysicalDisk.RPM)
	}

	targets := registry.SourceTargets(dataset.ID)
	if len(targets) == 0 {
		t.Fatal("expected source targets for dataset")
	}
	found := false
	for _, target := range targets {
		if target.Source == unifiedresources.SourceTrueNAS && target.SourceID == "system:truenas-main/dataset:tank/apps" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected truenas source target for dataset, got %+v", targets)
	}
}

func TestRegistryIngestRecordsScopesTrueNASChildrenPerSystem(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	first := DefaultFixtures()
	second := DefaultFixtures()
	second.System.Hostname = "truenas-backup"
	second.System.MachineID = "truenas-backup-machine-id"
	second.System.CollectedAt = second.System.CollectedAt.Add(time.Minute)
	second.CollectedAt = second.CollectedAt.Add(time.Minute)
	for i := range second.Disks {
		second.Disks[i].Serial = strings.TrimSpace(second.Disks[i].Serial) + "-backup"
	}

	records := append(NewProvider(first).Records(), NewProvider(second).Records()...)
	registry := unifiedresources.NewRegistry(unifiedresources.NewMemoryStore())
	registry.IngestRecords(unifiedresources.SourceTrueNAS, records)

	resources := registry.List()
	systems := resourcesByNameAndType(resources, unifiedresources.ResourceTypeAgent, "")
	if len(systems) != 2 {
		t.Fatalf("expected 2 TrueNAS systems, got %d from %+v", len(systems), systems)
	}
	if !hasNamedResource(systems, "truenas-main") || !hasNamedResource(systems, "truenas-backup") {
		t.Fatalf("expected both TrueNAS system resources, got %+v", systems)
	}

	pools := resourcesByNameAndType(resources, unifiedresources.ResourceTypeStorage, "tank")
	poolCount := 0
	poolParents := map[string]struct{}{}
	for _, pool := range pools {
		if pool.Storage == nil || pool.Storage.Topology != "pool" {
			continue
		}
		poolCount++
		if pool.ParentName != "" {
			poolParents[pool.ParentName] = struct{}{}
		}
	}
	if poolCount != 2 {
		t.Fatalf("expected duplicate pool names to remain per system, got %d from %+v", poolCount, pools)
	}
	if _, ok := poolParents["truenas-main"]; !ok {
		t.Fatalf("expected tank pool parented to truenas-main, parents=%+v", poolParents)
	}
	if _, ok := poolParents["truenas-backup"]; !ok {
		t.Fatalf("expected tank pool parented to truenas-backup, parents=%+v", poolParents)
	}

	apps := resourcesByNameAndType(resources, unifiedresources.ResourceTypeAppContainer, "Nextcloud")
	if len(apps) != 2 {
		t.Fatalf("expected duplicate app names to remain per system, got %d from %+v", len(apps), apps)
	}
	appParents := map[string]struct{}{}
	for _, app := range apps {
		if app.ParentName != "" {
			appParents[app.ParentName] = struct{}{}
		}
	}
	if _, ok := appParents["truenas-main"]; !ok {
		t.Fatalf("expected Nextcloud app parented to truenas-main, parents=%+v", appParents)
	}
	if _, ok := appParents["truenas-backup"]; !ok {
		t.Fatalf("expected Nextcloud app parented to truenas-backup, parents=%+v", appParents)
	}
}

func TestTrueNASResourcesFlowThroughUnifiedTypesWithoutSpecialCasing(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	registry := unifiedresources.NewRegistry(unifiedresources.NewMemoryStore())
	registry.IngestRecords(unifiedresources.SourceTrueNAS, NewDefaultProvider().Records())

	adapter := unifiedresources.NewMonitorAdapter(registry)
	resources := adapter.GetAll()
	if len(resources) == 0 {
		t.Fatal("expected resources from registry")
	}

	for _, resource := range resources {
		if string(resource.Type) == "truenas" {
			t.Fatalf("expected canonical render type, got truenas-specific type for %s", resource.ID)
		}
		switch resource.Type {
		case unifiedresources.ResourceTypeAgent, unifiedresources.ResourceTypeStorage, unifiedresources.ResourceTypePhysicalDisk, unifiedresources.ResourceTypeAppContainer, unifiedresources.ResourceTypeVM, unifiedresources.ResourceTypeNetworkShare:
		default:
			t.Fatalf("unexpected unified type for truenas fixture resource: %s (%s)", resource.Type, resource.ID)
		}

		frontend := models.ConvertResourceToFrontend(toFrontendInput(resource))
		if frontend.ID == "" {
			t.Fatalf("expected frontend conversion to preserve ID for %s", resource.ID)
		}
	}
}

func TestTrueNASDiskRecordsPopulatePhysicalDiskMeta(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	fixtures := DefaultFixtures()
	provider := NewProvider(fixtures)
	records := provider.Records()
	if len(records) == 0 {
		t.Fatal("expected fixture records from provider")
	}

	fixtureByName := make(map[string]Disk, len(fixtures.Disks))
	for _, disk := range fixtures.Disks {
		fixtureByName[disk.Name] = disk
	}

	var diskRecords []unifiedresources.IngestRecord
	for _, record := range records {
		if record.Resource.Type == unifiedresources.ResourceTypePhysicalDisk {
			diskRecords = append(diskRecords, record)
		}
	}
	if len(diskRecords) != len(fixtures.Disks) {
		t.Fatalf("expected %d disk records, got %d", len(fixtures.Disks), len(diskRecords))
	}

	for _, record := range diskRecords {
		fixture, ok := fixtureByName[record.Resource.Name]
		if !ok {
			t.Fatalf("unexpected disk record %q", record.Resource.Name)
		}
		if record.Resource.PhysicalDisk == nil {
			t.Fatalf("expected PhysicalDiskMeta for %q", record.Resource.Name)
		}

		meta := record.Resource.PhysicalDisk
		if meta.DevPath != "/dev/"+fixture.Name {
			t.Fatalf("expected dev path %q, got %q", "/dev/"+fixture.Name, meta.DevPath)
		}
		if meta.Model != fixture.Model {
			t.Fatalf("expected model %q, got %q", fixture.Model, meta.Model)
		}
		if meta.Serial != fixture.Serial {
			t.Fatalf("expected serial %q, got %q", fixture.Serial, meta.Serial)
		}
		if meta.DiskType != fixture.Transport {
			t.Fatalf("expected disk type %q, got %q", fixture.Transport, meta.DiskType)
		}
		if meta.SizeBytes != fixture.SizeBytes {
			t.Fatalf("expected size bytes %d, got %d", fixture.SizeBytes, meta.SizeBytes)
		}
		if meta.Wearout != -1 {
			t.Fatalf("expected wearout -1, got %d", meta.Wearout)
		}
		wantRPM := 0
		if fixture.Rotational {
			wantRPM = 7200
		}
		if meta.RPM != wantRPM {
			t.Fatalf("expected rpm %d, got %d", wantRPM, meta.RPM)
		}

		switch strings.ToUpper(strings.TrimSpace(fixture.Status)) {
		case "ONLINE":
			if record.Resource.Status != unifiedresources.StatusOnline {
				t.Fatalf("expected status online for %q, got %s", fixture.Name, record.Resource.Status)
			}
			if meta.Health != "PASSED" {
				t.Fatalf("expected health PASSED for %q, got %q", fixture.Name, meta.Health)
			}
			if meta.Risk != nil {
				t.Fatalf("expected no risk payload for healthy disk %q, got %+v", fixture.Name, meta.Risk)
			}
		case "DEGRADED":
			if record.Resource.Status != unifiedresources.StatusWarning {
				t.Fatalf("expected status warning for %q, got %s", fixture.Name, record.Resource.Status)
			}
			if meta.Health != "DEGRADED" {
				t.Fatalf("expected health DEGRADED for %q, got %q", fixture.Name, meta.Health)
			}
			if meta.Risk == nil {
				t.Fatalf("expected risk payload for degraded disk %q", fixture.Name)
			}
			if meta.Risk.Level != "warning" {
				t.Fatalf("expected warning risk for degraded disk %q, got %+v", fixture.Name, meta.Risk)
			}
			if !containsRiskReason(meta.Risk.Reasons, "truenas_disk_state") {
				t.Fatalf("expected truenas_disk_state reason for %q, got %+v", fixture.Name, meta.Risk.Reasons)
			}
			if !containsRiskReason(meta.Risk.Reasons, "truenas_smart") {
				t.Fatalf("expected truenas_smart reason for %q, got %+v", fixture.Name, meta.Risk.Reasons)
			}
			if len(record.Resource.Incidents) != 1 || record.Resource.Incidents[0].Code != "truenas_smart" {
				t.Fatalf("expected SMART incident on degraded disk %q, got %+v", fixture.Name, record.Resource.Incidents)
			}
		default:
			t.Fatalf("unhandled fixture status %q for %q", fixture.Status, fixture.Name)
		}
	}
}

func hasIncidentCode(incidents []unifiedresources.ResourceIncident, code string) bool {
	for _, incident := range incidents {
		if incident.Code == code {
			return true
		}
	}
	return false
}

func containsRiskReason(reasons []unifiedresources.PhysicalDiskRiskReason, code string) bool {
	for _, reason := range reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func requireResource(t *testing.T, resources []unifiedresources.Resource, resourceType unifiedresources.ResourceType, name string) *unifiedresources.Resource {
	t.Helper()
	for i := range resources {
		if resources[i].Type == resourceType && resources[i].Name == name {
			return &resources[i]
		}
	}
	t.Fatalf("missing resource type=%s name=%s", resourceType, name)
	return nil
}

func resourcesByNameAndType(resources []unifiedresources.Resource, resourceType unifiedresources.ResourceType, name string) []unifiedresources.Resource {
	out := make([]unifiedresources.Resource, 0)
	for _, resource := range resources {
		if resource.Type != resourceType {
			continue
		}
		if name != "" && resource.Name != name {
			continue
		}
		out = append(out, resource)
	}
	return out
}

func hasNamedResource(resources []unifiedresources.Resource, name string) bool {
	for _, resource := range resources {
		if resource.Name == name {
			return true
		}
	}
	return false
}

func assertSourceTracking(t *testing.T, resource unifiedresources.Resource, source unifiedresources.DataSource) {
	t.Helper()
	if !containsSource(resource.Sources, source) {
		t.Fatalf("expected source %s in %+v", source, resource.Sources)
	}
	if resource.SourceStatus == nil {
		t.Fatalf("expected source status for %s", resource.ID)
	}
	if _, ok := resource.SourceStatus[source]; !ok {
		t.Fatalf("expected source status entry for %s on %s", source, resource.ID)
	}
}

func assertDiskMetric(t *testing.T, metrics *unifiedresources.ResourceMetrics, total int64, used int64) {
	t.Helper()
	if metrics == nil || metrics.Disk == nil {
		t.Fatal("expected disk metric")
	}
	if metrics.Disk.Total == nil || *metrics.Disk.Total != total {
		t.Fatalf("expected total=%d, got %+v", total, metrics.Disk.Total)
	}
	if metrics.Disk.Used == nil || *metrics.Disk.Used != used {
		t.Fatalf("expected used=%d, got %+v", used, metrics.Disk.Used)
	}
	free := *metrics.Disk.Total - *metrics.Disk.Used
	if free <= 0 {
		t.Fatalf("expected positive free capacity, got %d", free)
	}
}

func containsSource(sources []unifiedresources.DataSource, source unifiedresources.DataSource) bool {
	for _, existing := range sources {
		if existing == source {
			return true
		}
	}
	return false
}

func toFrontendInput(resource unifiedresources.Resource) models.ResourceConvertInput {
	input := models.ResourceConvertInput{
		ID:           resource.ID,
		Type:         string(resource.Type),
		Name:         resource.Name,
		DisplayName:  resource.Name,
		SourceType:   firstSourceType(resource.Sources),
		ParentID:     stringValue(resource.ParentID),
		ClusterID:    resource.Identity.ClusterName,
		Status:       string(resource.Status),
		Tags:         resource.Tags,
		LastSeenUnix: resource.LastSeen.UnixMilli(),
	}
	input.CPU = metricToInput(metricValue(resource.Metrics, func(metrics *unifiedresources.ResourceMetrics) *unifiedresources.MetricValue { return metrics.CPU }))
	input.Memory = metricToInput(metricValue(resource.Metrics, func(metrics *unifiedresources.ResourceMetrics) *unifiedresources.MetricValue { return metrics.Memory }))
	input.Disk = metricToInput(metricValue(resource.Metrics, func(metrics *unifiedresources.ResourceMetrics) *unifiedresources.MetricValue { return metrics.Disk }))

	if resource.Metrics != nil && (resource.Metrics.NetIn != nil || resource.Metrics.NetOut != nil) {
		input.HasNetwork = true
		if resource.Metrics.NetIn != nil {
			input.NetworkRX = int64(math.Round(resource.Metrics.NetIn.Value))
		}
		if resource.Metrics.NetOut != nil {
			input.NetworkTX = int64(math.Round(resource.Metrics.NetOut.Value))
		}
	}
	input.Identity = identityToInput(resource.Identity)
	return input
}

func firstSourceType(sources []unifiedresources.DataSource) string {
	if len(sources) == 0 {
		return ""
	}
	return string(sources[0])
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func metricValue(
	metrics *unifiedresources.ResourceMetrics,
	pick func(*unifiedresources.ResourceMetrics) *unifiedresources.MetricValue,
) *unifiedresources.MetricValue {
	if metrics == nil {
		return nil
	}
	return pick(metrics)
}

func metricToInput(metric *unifiedresources.MetricValue) *models.ResourceMetricInput {
	if metric == nil {
		return nil
	}

	current := metric.Percent
	if current == 0 {
		current = metric.Value
	}
	if metric.Percent != 0 && metric.Value != 0 {
		current = math.Max(metric.Percent, metric.Value)
	}

	result := &models.ResourceMetricInput{Current: current}
	if metric.Total != nil {
		total := *metric.Total
		result.Total = &total
	}
	if metric.Used != nil {
		used := *metric.Used
		result.Used = &used
	}
	if result.Total != nil && result.Used != nil {
		free := *result.Total - *result.Used
		result.Free = &free
	}
	return result
}

func identityToInput(identity unifiedresources.ResourceIdentity) *models.ResourceIdentityInput {
	hostname := ""
	for _, candidate := range identity.Hostnames {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			hostname = trimmed
			break
		}
	}

	ips := make([]string, 0, len(identity.IPAddresses))
	for _, ip := range identity.IPAddresses {
		if trimmed := strings.TrimSpace(ip); trimmed != "" {
			ips = append(ips, trimmed)
		}
	}

	machineID := strings.TrimSpace(identity.MachineID)
	if hostname == "" && machineID == "" && len(ips) == 0 {
		return nil
	}

	return &models.ResourceIdentityInput{
		Hostname:  hostname,
		MachineID: machineID,
		IPs:       ips,
	}
}

func newConnectionProvider(t *testing.T, fixtures FixtureSnapshot, connectionID string) *Provider {
	t.Helper()
	provider := NewLiveProviderForConnection(&FixtureFetcher{Snapshot: fixtures}, connectionID)
	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh fixture-backed provider: %v", err)
	}
	return provider
}

func TestRegistryIngestRecordsKeepsSameHostnameSystemsDistinct(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	// The #1573/#1575 shape: two systems report the same hostname and no DMI
	// serial. Only the configured connections tell them apart.
	first := DefaultFixtures()
	first.System.MachineID = ""
	second := DefaultFixtures()
	second.System.MachineID = ""
	for i := range second.Disks {
		second.Disks[i].Serial = strings.TrimSpace(second.Disks[i].Serial) + "-second"
	}

	records := append(
		newConnectionProvider(t, first, "conn-first").Records(),
		newConnectionProvider(t, second, "conn-second").Records()...,
	)
	registry := unifiedresources.NewRegistry(unifiedresources.NewMemoryStore())
	registry.IngestRecords(unifiedresources.SourceTrueNAS, records)

	resources := registry.List()
	systems := resourcesByNameAndType(resources, unifiedresources.ResourceTypeAgent, "truenas-main")
	if len(systems) != 2 {
		t.Fatalf("expected 2 TrueNAS systems for the shared hostname, got %d from %+v", len(systems), systems)
	}
	if systems[0].ID == systems[1].ID {
		t.Fatalf("expected distinct canonical IDs for same-hostname systems, both got %s", systems[0].ID)
	}
	agentIDs := map[string]struct{}{}
	systemIDs := map[string]struct{}{}
	for _, system := range systems {
		systemIDs[system.ID] = struct{}{}
		if system.Agent == nil {
			t.Fatalf("expected agent data on TrueNAS system %s", system.ID)
		}
		agentIDs[system.Agent.AgentID] = struct{}{}
	}
	for _, want := range []string{"conn-first", "conn-second"} {
		if _, ok := agentIDs[want]; !ok {
			t.Fatalf("expected connection-scoped agent metric IDs, got %v", agentIDs)
		}
	}

	poolParents := map[string]struct{}{}
	poolCount := 0
	for _, pool := range resourcesByNameAndType(resources, unifiedresources.ResourceTypeStorage, "tank") {
		if pool.Storage == nil || pool.Storage.Topology != "pool" {
			continue
		}
		poolCount++
		if pool.ParentID == nil {
			t.Fatalf("expected tank pool %s to have a parent system", pool.ID)
		}
		if _, ok := systemIDs[*pool.ParentID]; !ok {
			t.Fatalf("expected tank pool %s parented to one of the systems, got parent %s", pool.ID, *pool.ParentID)
		}
		poolParents[*pool.ParentID] = struct{}{}
	}
	if poolCount != 2 || len(poolParents) != 2 {
		t.Fatalf("expected one tank pool per system, got %d pools across %d parents", poolCount, len(poolParents))
	}
}

func TestIngestRecordsSucceedLegacyHostnameScopedCanonicalIDs(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	fixtures := DefaultFixtures()
	// A serial-less box: the retired client fallback minted its machine key
	// from the reported hostname.
	fixtures.System.MachineID = ""
	hostname := fixtures.System.Hostname
	legacySystemSourceID := "system:" + hostname

	legacySystemID := unifiedresources.MachineIdentityCanonicalID(unifiedresources.ResourceTypeAgent, hostname)
	legacyPoolID := unifiedresources.SourceSpecificID(
		unifiedresources.ResourceTypeStorage,
		unifiedresources.SourceTrueNAS,
		scopedPoolSourceID(legacySystemSourceID, "tank"),
	)

	store := unifiedresources.NewMemoryStore()
	for _, canonicalID := range []string{legacySystemID, legacyPoolID} {
		if err := store.SetResourceOperatorState(unifiedresources.ResourceOperatorState{
			CanonicalID:          canonicalID,
			IntentionallyOffline: true,
		}); err != nil {
			t.Fatalf("seed operator state for %s: %v", canonicalID, err)
		}
	}
	if err := store.UpsertResourceIdentityPins([]unifiedresources.ResourceIdentityPin{{
		CanonicalID:  legacySystemID,
		ResourceType: unifiedresources.ResourceTypeAgent,
		MachineID:    hostname,
		Hostname:     hostname,
	}}); err != nil {
		t.Fatalf("seed legacy identity pin: %v", err)
	}

	registry := unifiedresources.NewRegistry(store)
	registry.IngestRecords(unifiedresources.SourceTrueNAS, newConnectionProvider(t, fixtures, "conn-a").Records())

	newSystemID := unifiedresources.SourceSpecificID(unifiedresources.ResourceTypeAgent, unifiedresources.SourceTrueNAS, "system:conn-a")
	newPoolID := unifiedresources.SourceSpecificID(
		unifiedresources.ResourceTypeStorage,
		unifiedresources.SourceTrueNAS,
		scopedPoolSourceID("system:conn-a", "tank"),
	)
	system := mustResourceByID(t, registry.List(), newSystemID)
	if system.Agent == nil || system.Agent.AgentID != "conn-a" {
		t.Fatalf("expected connection-scoped system record, got %+v", system.Agent)
	}

	for oldID, newID := range map[string]string{legacySystemID: newSystemID, legacyPoolID: newPoolID} {
		if _, found, err := store.GetResourceOperatorState(oldID); err != nil || found {
			t.Fatalf("expected operator state to leave superseded ID %s (found=%v err=%v)", oldID, found, err)
		}
		state, found, err := store.GetResourceOperatorState(newID)
		if err != nil || !found {
			t.Fatalf("expected operator state re-keyed to %s (found=%v err=%v)", newID, found, err)
		}
		if !state.IntentionallyOffline {
			t.Fatalf("expected re-keyed operator state to keep its fields, got %+v", state)
		}
	}

	pins, err := store.ListResourceIdentityPins()
	if err != nil {
		t.Fatalf("list identity pins: %v", err)
	}
	for _, pin := range pins {
		if pin.CanonicalID == legacySystemID {
			t.Fatalf("expected superseded hostname-derived pin to be deleted, still present: %+v", pin)
		}
	}
}

func TestIngestRecordsDoNotCompleteTrueNASIdentityFromPins(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() {
		SetFeatureEnabled(previous)
	})

	fixtures := DefaultFixtures()
	fixtures.System.MachineID = ""
	hostname := fixtures.System.Hostname

	// A pulse-agent's pin for a host with the same name must not lend the
	// TrueNAS system its machine key: completion would merge every TrueNAS
	// connection sharing that hostname into the agent host.
	agentPinID := unifiedresources.MachineIdentityCanonicalID(unifiedresources.ResourceTypeAgent, "agent-machine-id")
	store := unifiedresources.NewMemoryStore()
	if err := store.UpsertResourceIdentityPins([]unifiedresources.ResourceIdentityPin{{
		CanonicalID:  agentPinID,
		ResourceType: unifiedresources.ResourceTypeAgent,
		MachineID:    "agent-machine-id",
		Hostname:     hostname,
	}}); err != nil {
		t.Fatalf("seed agent identity pin: %v", err)
	}

	registry := unifiedresources.NewRegistry(store)
	registry.IngestRecords(unifiedresources.SourceTrueNAS, newConnectionProvider(t, fixtures, "conn-a").Records())

	expectedID := unifiedresources.SourceSpecificID(unifiedresources.ResourceTypeAgent, unifiedresources.SourceTrueNAS, "system:conn-a")
	system := mustResourceByID(t, registry.List(), expectedID)
	if system.Identity.MachineID != "" {
		t.Fatalf("expected TrueNAS system identity to stay machine-keyless, got %q", system.Identity.MachineID)
	}
}

func mustResourceByID(t *testing.T, resources []unifiedresources.Resource, id string) unifiedresources.Resource {
	t.Helper()
	for _, resource := range resources {
		if resource.ID == id {
			return resource
		}
	}
	t.Fatalf("missing resource with canonical ID %s", id)
	return unifiedresources.Resource{}
}
