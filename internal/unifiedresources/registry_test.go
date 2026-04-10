package unifiedresources

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

func TestResourceRegistry_ListByType(t *testing.T) {
	rr := NewRegistry(nil)

	now := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "host-1",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "host-1",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{MachineID: "machine-1"},
		},
	})

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{
			SourceID: "vm-100",
			Resource: Resource{
				Type:     ResourceTypeVM,
				Name:     "vm-100",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"vm-100"}},
		},
		{
			SourceID: "vm-101",
			Resource: Resource{
				Type:     ResourceTypeVM,
				Name:     "vm-101",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"vm-101"}},
		},
		{
			SourceID: "ct-200",
			Resource: Resource{
				Type:     ResourceTypeSystemContainer,
				Name:     "ct-200",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"ct-200"}},
		},
	})

	got := rr.ListByType(ResourceTypeVM)
	if len(got) != 2 {
		t.Fatalf("expected 2 VMs, got %d", len(got))
	}
	for _, r := range got {
		if r.Type != ResourceTypeVM {
			t.Fatalf("expected all resources to be type %q, got %q", ResourceTypeVM, r.Type)
		}
	}

	// Deterministic ordering (sorted by ID).
	wantIDs := []string{
		rr.sourceSpecificID(ResourceTypeVM, SourceProxmox, "vm-100"),
		rr.sourceSpecificID(ResourceTypeVM, SourceProxmox, "vm-101"),
	}
	// Hash order isn't meaningful; the contract is lexicographic by ID.
	if wantIDs[0] > wantIDs[1] {
		wantIDs[0], wantIDs[1] = wantIDs[1], wantIDs[0]
	}
	if got[0].ID != wantIDs[0] || got[1].ID != wantIDs[1] {
		t.Fatalf("expected IDs %v, got [%s %s]", wantIDs, got[0].ID, got[1].ID)
	}

	// Returned resources should be copies (mutating the result does not mutate the registry).
	origName := got[0].Name
	got[0].Name = "mutated"
	if r, ok := rr.Get(got[0].ID); !ok || r == nil {
		t.Fatalf("expected Get(%q) to succeed", got[0].ID)
	} else if r.Name != origName {
		t.Fatalf("expected registry resource name %q, got %q", origName, r.Name)
	}
}

func TestResourceRegistry_ListByType_Empty(t *testing.T) {
	rr := NewRegistry(nil)
	if got := rr.ListByType(ResourceTypeVM); len(got) != 0 {
		t.Fatalf("expected empty result, got %d", len(got))
	}
}

func TestResourceRegistry_ListUsesDeterministicNameTieBreakers(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)

	rr.IngestResources([]Resource{
		{ID: "storage-z", Type: ResourceTypeStorage, Name: "backup-vault-a", Status: StatusOnline, LastSeen: now},
		{ID: "storage-b", Type: ResourceTypeStorage, Name: "backup-vault-a", Status: StatusOnline, LastSeen: now},
		{ID: "agent-1", Type: ResourceTypeAgent, Name: "alpha-host", Status: StatusOnline, LastSeen: now},
		{ID: "storage-a", Type: ResourceTypeStorage, Name: "backup-vault-a", Status: StatusOnline, LastSeen: now},
	})

	got := rr.List()
	gotIDs := make([]string, 0, len(got))
	for _, resource := range got {
		gotIDs = append(gotIDs, resource.ID)
	}

	assertStringSlice(t, gotIDs, []string{"agent-1", "storage-a", "storage-b", "storage-z"})
}

func TestResourceRegistry_MergesPhysicalDiskTemperatureAggregate(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceTrueNAS, []IngestRecord{
		{
			SourceID: "disk-sda",
			Resource: Resource{
				Type:     ResourceTypePhysicalDisk,
				Name:     "sda",
				Status:   StatusOnline,
				LastSeen: now,
				PhysicalDisk: &PhysicalDiskMeta{
					DevPath:     "/dev/sda",
					Model:       "Seagate Exos X18",
					Serial:      "SER-A",
					DiskType:    "sata",
					SizeBytes:   1_000_000,
					Health:      "PASSED",
					Temperature: 34,
					Wearout:     -1,
					TemperatureAggregate: &TemperatureAggregateMeta{
						WindowDays: 7,
						MinCelsius: 29.0,
						AvgCelsius: 32.7,
						MaxCelsius: 38.0,
					},
				},
			},
			Identity: ResourceIdentity{MachineID: "SER-A"},
		},
	})

	disks := rr.ListByType(ResourceTypePhysicalDisk)
	if len(disks) != 1 {
		t.Fatalf("expected 1 physical disk, got %d", len(disks))
	}
	aggregate := disks[0].PhysicalDisk.TemperatureAggregate
	if aggregate == nil {
		t.Fatalf("expected temperature aggregate on merged disk record: %+v", disks[0].PhysicalDisk)
	}
	if aggregate.WindowDays != 7 || aggregate.MinCelsius != 29.0 || aggregate.AvgCelsius != 32.7 || aggregate.MaxCelsius != 38.0 {
		t.Fatalf("unexpected merged temperature aggregate: %+v", aggregate)
	}

	aggregate.MaxCelsius = 99.0
	got, ok := rr.Get(disks[0].ID)
	if !ok || got == nil || got.PhysicalDisk == nil || got.PhysicalDisk.TemperatureAggregate == nil {
		t.Fatalf("expected stored physical disk record, got %+v", got)
	}
	if got.PhysicalDisk.TemperatureAggregate.MaxCelsius != 38.0 {
		t.Fatalf("expected registry clone isolation for temperature aggregate, got %+v", got.PhysicalDisk.TemperatureAggregate)
	}
}

func TestResourceRegistry_IngestsVMwareSourceAsCanonicalResources(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 30, 18, 30, 0, 0, time.UTC)

	rr.IngestRecords(SourceVMware, []IngestRecord{
		{
			SourceID: "vc-1:host:host-101",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "esxi-01.lab.local",
				Status:   StatusOnline,
				LastSeen: now,
				VMware: &VMwareData{
					ConnectionID:    "vc-1",
					ConnectionName:  "Lab VC",
					VCenterHost:     "vc.lab.local",
					ManagedObjectID: "host-101",
					EntityType:      "host",
					HostUUID:        "uuid-host-1",
				},
			},
			Identity: ResourceIdentity{
				DMIUUID:   "uuid-host-1",
				Hostnames: []string{"esxi-01.lab.local"},
			},
		},
	})

	agents := rr.ListByType(ResourceTypeAgent)
	if len(agents) != 1 {
		t.Fatalf("expected 1 VMware agent resource, got %d", len(agents))
	}
	agent := agents[0]
	if len(agent.Sources) != 1 || agent.Sources[0] != SourceVMware {
		t.Fatalf("expected VMware source ownership, got %+v", agent.Sources)
	}
	if agent.VMware == nil {
		t.Fatalf("expected VMware metadata on merged resource, got %+v", agent)
	}
	if got := agent.VMware.ConnectionID; got != "vc-1" {
		t.Fatalf("vmware connection id = %q, want vc-1", got)
	}
	if got := agent.Canonical.PrimaryID; got != "vmware:vc-1:host:host-101" {
		t.Fatalf("canonical primary id = %q, want vmware:vc-1:host:host-101", got)
	}
}

func TestBuildMetricsTargetForRegistryFallsBackToStoredMetricsTarget(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC)
	resourceID := CanonicalResourceID("app-demo")
	rr.resources[resourceID] = &Resource{
		ID:       resourceID,
		Type:     ResourceTypeAppContainer,
		Name:     "Nextcloud",
		Status:   StatusOnline,
		LastSeen: now,
		MetricsTarget: &MetricsTarget{
			ResourceType: "dockerContainer",
			ResourceID:   "nextcloud-web-1",
		},
	}

	target := BuildMetricsTargetForRegistry(rr, resourceID)
	if target == nil {
		t.Fatal("expected stored metrics target fallback")
	}
	if target.ResourceType != "dockerContainer" || target.ResourceID != "nextcloud-web-1" {
		t.Fatalf("unexpected fallback metrics target %+v", target)
	}

	target.ResourceID = "mutated"
	stored, ok := rr.Get(resourceID)
	if !ok || stored == nil || stored.MetricsTarget == nil {
		t.Fatalf("expected stored app resource with metrics target, got %+v", stored)
	}
	if stored.MetricsTarget.ResourceID != "nextcloud-web-1" {
		t.Fatalf("expected fallback target clone isolation, got %+v", stored.MetricsTarget)
	}
}

func TestResourceRegistryStoragePoolViewsCarryResolvedMetricsTarget(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 4, 1, 11, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceVMware, []IngestRecord{
		{
			SourceID: "vc-1:datastore:datastore-202",
			Resource: Resource{
				ID:       "storage-vmware-1",
				Type:     ResourceTypeStorage,
				Name:     "archive-tier",
				Status:   StatusOnline,
				LastSeen: now,
				Storage: &StorageMeta{
					Type:     "datastore",
					Platform: "vmware",
					Nodes:    []string{"esxi-01.lab.local"},
				},
				VMware: &VMwareData{
					ConnectionID:    "vc-1",
					EntityType:      "datastore",
					ManagedObjectID: "datastore-202",
					RuntimeHostName: "esxi-01.lab.local",
				},
			},
		},
	})

	pools := rr.StoragePools()
	if len(pools) != 1 {
		t.Fatalf("expected 1 storage pool view, got %d", len(pools))
	}
	if got := pools[0].SourceID(); got != "vc-1:datastore:datastore-202" {
		t.Fatalf("expected storage pool view to expose resolved metrics target, got %q", got)
	}
	if got := pools[0].Node(); got != "esxi-01.lab.local" {
		t.Fatalf("expected storage pool view node hint to remain VMware runtime host, got %q", got)
	}

	stored, ok := rr.Get(pools[0].ID())
	if !ok || stored == nil {
		t.Fatalf("expected stored storage resource, got %+v", stored)
	}
	if stored.MetricsTarget != nil {
		t.Fatalf("expected registry storage record to remain raw without persisted metrics target, got %+v", stored.MetricsTarget)
	}
}

func TestMergeVMwareDataMergesSignalFieldsWithoutDroppingExistingIdentity(t *testing.T) {
	existing := &VMwareData{
		ConnectionID:       "vc-1",
		ConnectionName:     "Lab VC",
		VCenterHost:        "vc.lab.local",
		ManagedObjectID:    "vm-101",
		EntityType:         "vm",
		HostUUID:           "uuid-host-1",
		DatacenterName:     "DC1",
		ClusterName:        "Cluster A",
		DatastoreNames:     []string{"primary-vmfs"},
		ConnectionState:    "connected",
		PowerState:         "poweredOn",
		OverallStatus:      "green",
		GuestHostname:      "app-01.internal",
		ActiveAlarmCount:   1,
		ActiveAlarmSummary: "old alarm",
		RecentTaskCount:    1,
		RecentTaskSummary:  "old task",
		SnapshotCount:      1,
	}
	accessible := false
	incoming := &VMwareData{
		ClusterName:         "Cluster A",
		RuntimeHostName:     "esxi-01.lab.local",
		DatastoreNames:      []string{"backup-nfs"},
		DatastoreAccessible: &accessible,
		GuestIPAddresses:    []string{"10.0.0.21"},
		OverallStatus:       "yellow",
		ActiveAlarmCount:    3,
		ActiveAlarmSummary:  "Host connection lost; datastore latency elevated",
		RecentTaskCount:     2,
		RecentTaskSummary:   "vMotion task completed",
		SnapshotCount:       4,
	}

	merged := mergeVMwareData(existing, incoming)
	if merged == nil {
		t.Fatal("expected merged VMware metadata")
	}
	if got := merged.ConnectionID; got != "vc-1" {
		t.Fatalf("connection id = %q, want vc-1", got)
	}
	if got := merged.ManagedObjectID; got != "vm-101" {
		t.Fatalf("managed object id = %q, want vm-101", got)
	}
	if got := merged.OverallStatus; got != "yellow" {
		t.Fatalf("overall status = %q, want yellow", got)
	}
	if got := merged.ActiveAlarmCount; got != 3 {
		t.Fatalf("active alarm count = %d, want 3", got)
	}
	if got := merged.ActiveAlarmSummary; got != "Host connection lost; datastore latency elevated" {
		t.Fatalf("active alarm summary = %q", got)
	}
	if got := merged.RecentTaskCount; got != 2 {
		t.Fatalf("recent task count = %d, want 2", got)
	}
	if got := merged.RecentTaskSummary; got != "vMotion task completed" {
		t.Fatalf("recent task summary = %q", got)
	}
	if got := merged.SnapshotCount; got != 4 {
		t.Fatalf("snapshot count = %d, want 4", got)
	}
	if got := merged.RuntimeHostName; got != "esxi-01.lab.local" {
		t.Fatalf("runtime host name = %q, want esxi-01.lab.local", got)
	}
	if got := merged.DatastoreNames; !reflect.DeepEqual(got, []string{"primary-vmfs", "backup-nfs"}) {
		t.Fatalf("datastore names = %#v", got)
	}
	if merged.DatastoreAccessible == nil || *merged.DatastoreAccessible {
		t.Fatalf("datastore accessible = %#v, want false", merged.DatastoreAccessible)
	}
	if got := merged.GuestHostname; got != "app-01.internal" {
		t.Fatalf("guest hostname = %q, want app-01.internal", got)
	}
	if got := merged.GuestIPAddresses; !reflect.DeepEqual(got, []string{"10.0.0.21"}) {
		t.Fatalf("guest ip addresses = %#v", got)
	}
}

func TestResourceRegistryClonesCarryPolicyMetadata(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 17, 12, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{
			SourceID: "vm-900",
			Resource: Resource{
				ID:       "vm-900",
				Type:     ResourceTypeVM,
				Name:     "customer-vm",
				Status:   StatusOnline,
				LastSeen: now,
				Tags:     []string{"customer-data"},
			},
			Identity: ResourceIdentity{
				Hostnames:   []string{"customer.internal"},
				IPAddresses: []string{"10.0.0.90"},
			},
		},
	})

	listed := rr.ListByType(ResourceTypeVM)
	if len(listed) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(listed))
	}
	if listed[0].Policy == nil {
		t.Fatal("expected list result to include policy metadata")
	}
	if got := listed[0].Policy.Sensitivity; got != ResourceSensitivityRestricted {
		t.Fatalf("policy sensitivity = %q, want %q", got, ResourceSensitivityRestricted)
	}
	if listed[0].AISafeSummary == "" {
		t.Fatal("expected aiSafeSummary on list result")
	}

	got, ok := rr.Get(listed[0].ID)
	if !ok || got == nil {
		t.Fatalf("expected Get(%q) to succeed", listed[0].ID)
	}
	if got.Policy == nil {
		t.Fatal("expected get result to include policy metadata")
	}
	if got.Policy == listed[0].Policy {
		t.Fatal("expected policy metadata to be cloned, not shared by pointer")
	}
}

func TestResourceRegistry_MonitoredSystemsSummarizeCanonicalTopLevelViews(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 17, 12, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "host-1",
			Resource: Resource{
				ID:       "host-1",
				Type:     ResourceTypeAgent,
				Name:     "lab-a",
				Status:   StatusOnline,
				LastSeen: now.Add(-2 * time.Minute),
				Agent: &AgentData{
					AgentID:   "agent-1",
					Hostname:  "lab-a",
					MachineID: "machine-1",
				},
			},
			Identity: ResourceIdentity{
				MachineID: "machine-1",
				Hostnames: []string{"lab-a"},
			},
		},
	})
	rr.IngestRecords(SourcePBS, []IngestRecord{
		{
			SourceID: "pbs-1",
			Resource: Resource{
				ID:       "pbs-1",
				Type:     ResourceTypePBS,
				Name:     "pbs-a",
				Status:   StatusOnline,
				LastSeen: now,
				PBS: &PBSData{
					InstanceID: "pbs-1",
					Hostname:   "lab-a",
					HostURL:    "https://lab-a:8007",
				},
			},
		},
	})
	rr.IngestRecords(SourcePMG, []IngestRecord{
		{
			SourceID: "pmg-1",
			Resource: Resource{
				ID:       "pmg-1",
				Type:     ResourceTypePMG,
				Name:     "pmg-b",
				Status:   StatusOffline,
				LastSeen: now.Add(-10 * time.Minute),
				PMG: &PMGData{
					InstanceID: "pmg-1",
					Hostname:   "mail-b",
				},
			},
		},
	})
	rr.IngestRecords(SourceK8s, []IngestRecord{
		{
			SourceID: "k8s-1",
			Resource: Resource{
				ID:       "k8s-1",
				Type:     ResourceTypeK8sCluster,
				Name:     "cluster-a",
				Status:   StatusOnline,
				LastSeen: now.Add(-30 * time.Second),
				Kubernetes: &K8sData{
					ClusterID: "cluster-a",
					AgentID:   "k8s-agent-1",
					Server:    "https://cluster-a.example:6443",
				},
			},
		},
	})

	systems := MonitoredSystems(rr)
	if len(systems) != 3 {
		t.Fatalf("MonitoredSystems() returned %d systems, want 3", len(systems))
	}

	byName := make(map[string]MonitoredSystemRecord, len(systems))
	for _, system := range systems {
		byName[system.Name] = system
	}

	hostSummary, ok := byName["lab-a"]
	if !ok {
		t.Fatalf("expected monitored systems to include lab-a, got %+v", systems)
	}
	if hostSummary.Type != "host" || hostSummary.Source != "multiple" {
		t.Fatalf("expected deduped host summary for lab-a, got %+v", hostSummary)
	}
	if !hostSummary.LastSeen.Equal(now) {
		t.Fatalf("expected deduped host last_seen %s, got %s", now, hostSummary.LastSeen)
	}

	k8sSummary, ok := byName["cluster-a"]
	if !ok {
		t.Fatalf("expected monitored systems to include cluster-a, got %+v", systems)
	}
	if k8sSummary.Type != "kubernetes-cluster" || k8sSummary.Source != "kubernetes" {
		t.Fatalf("expected kubernetes summary for cluster-a, got %+v", k8sSummary)
	}

	pmgSummary, ok := byName["pmg-b"]
	if !ok {
		t.Fatalf("expected monitored systems to include pmg-b, got %+v", systems)
	}
	if pmgSummary.Type != "pmg-server" || pmgSummary.Status != StatusOffline {
		t.Fatalf("expected PMG summary for pmg-b, got %+v", pmgSummary)
	}
}

func TestMonitoredSystemsExplainsStaleGroupedSourceWhileLastSeenStaysFresh(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC)

	agentResource := topLevelTestAgent("agent-host", "tower.local", "machine-1", "agent-1")
	agentResource.LastSeen = now.Add(-5 * time.Minute)
	dockerResource := topLevelTestDockerHost("docker-host", "tower.local", "docker-runtime-1", "agent-1")
	dockerResource.LastSeen = now.Add(-10 * time.Second)

	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "agent-host",
			Resource: agentResource,
		},
	})
	rr.IngestRecords(SourceDocker, []IngestRecord{
		{
			SourceID: "docker-host",
			Resource: dockerResource,
		},
	})

	rr.MarkStale(now, map[DataSource]time.Duration{
		SourceAgent:  60 * time.Second,
		SourceDocker: 60 * time.Second,
	})

	systems := MonitoredSystems(rr)
	if len(systems) != 1 {
		t.Fatalf("MonitoredSystems() returned %d systems, want 1", len(systems))
	}

	system := systems[0]
	if system.Status != StatusWarning {
		t.Fatalf("expected grouped monitored system status warning, got %+v", system)
	}
	if !system.LastSeen.Equal(dockerResource.LastSeen) {
		t.Fatalf("expected grouped last_seen %s, got %s", dockerResource.LastSeen, system.LastSeen)
	}
	if system.LatestIncludedSignal.Source != string(SourceDocker) {
		t.Fatalf("expected latest included signal source docker, got %+v", system)
	}
	if system.LatestIncludedSignal.Name != "tower.local" {
		t.Fatalf("expected latest included signal name tower.local, got %+v", system)
	}
	if system.LatestIncludedSignal.Type != "docker-host" {
		t.Fatalf("expected latest included signal type docker-host, got %+v", system)
	}
	if !system.LatestIncludedSignal.At.Equal(dockerResource.LastSeen) {
		t.Fatalf("expected latest included signal at %s, got %+v", dockerResource.LastSeen, system)
	}
	if system.StatusExplanation.Summary == "" {
		t.Fatal("expected grouped monitored system status explanation summary")
	}
	if system.StatusExplanation.Summary != "Pulse most recently heard from Docker for tower.local at 2026-03-23T11:59:50Z, but Agent data for tower.local is stale (last reported 2026-03-23T11:55:00Z), so this monitored system is warning." {
		t.Fatalf("unexpected grouped monitored system summary: %q", system.StatusExplanation.Summary)
	}
	if len(system.StatusExplanation.Reasons) != 1 {
		t.Fatalf("expected one stale grouped-source reason, got %+v", system.StatusExplanation.Reasons)
	}

	reason := system.StatusExplanation.Reasons[0]
	if reason.Kind != "source-stale" {
		t.Fatalf("expected stale source reason kind, got %+v", reason)
	}
	if reason.Source != string(SourceAgent) {
		t.Fatalf("expected agent source reason, got %+v", reason)
	}
	if reason.Status != "stale" {
		t.Fatalf("expected stale reason status, got %+v", reason)
	}
	if !reason.ReportedAt.Equal(agentResource.LastSeen) {
		t.Fatalf("expected stale reason reported_at %s, got %s", agentResource.LastSeen, reason.ReportedAt)
	}
	if reason.Summary == "" {
		t.Fatalf("expected stale reason summary, got %+v", reason)
	}
}

func TestMonitoredSystemsReportOnlineStatusWithoutUnknownBaselineBias(t *testing.T) {
	rr := NewRegistry(nil)

	proxmoxNode := topLevelTestProxmoxNode("proxmox-node", "tower", "proxmox-1", "https://tower.local:8006")
	proxmoxNode.LastSeen = time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{
			SourceID: "proxmox-node",
			Resource: proxmoxNode,
		},
	})

	systems := MonitoredSystems(rr)
	if len(systems) != 1 {
		t.Fatalf("MonitoredSystems() returned %d systems, want 1", len(systems))
	}
	if systems[0].Status != StatusOnline {
		t.Fatalf("MonitoredSystems()[0].Status = %q, want %q", systems[0].Status, StatusOnline)
	}
	if systems[0].StatusExplanation.Summary != "All included top-level collection paths currently report online status." {
		t.Fatalf("unexpected online status summary: %q", systems[0].StatusExplanation.Summary)
	}
}

func TestMonitoredSystemsGroupedOnlineSourcesStayOnline(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC)

	agentResource := topLevelTestAgent("agent-host", "tower.local", "machine-1", "agent-1")
	agentResource.LastSeen = now.Add(-1 * time.Minute)

	dockerResource := topLevelTestDockerHost("docker-host", "tower.local", "docker-runtime-1", "agent-1")
	dockerResource.LastSeen = now

	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "agent-host",
			Resource: agentResource,
		},
	})
	rr.IngestRecords(SourceDocker, []IngestRecord{
		{
			SourceID: "docker-host",
			Resource: dockerResource,
		},
	})

	systems := MonitoredSystems(rr)
	if len(systems) != 1 {
		t.Fatalf("MonitoredSystems() returned %d systems, want 1", len(systems))
	}
	if systems[0].Status != StatusOnline {
		t.Fatalf("MonitoredSystems()[0].Status = %q, want %q", systems[0].Status, StatusOnline)
	}
}

func TestMonitoredSystemsPreferOfflineOverWarning(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC)

	agentResource := topLevelTestAgent("agent-host", "tower.local", "machine-1", "agent-1")
	agentResource.Status = StatusWarning
	agentResource.LastSeen = now.Add(-1 * time.Minute)

	dockerResource := topLevelTestDockerHost("docker-host", "tower.local", "docker-runtime-1", "agent-1")
	dockerResource.Status = StatusOffline
	dockerResource.LastSeen = now

	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "agent-host",
			Resource: agentResource,
		},
	})
	rr.IngestRecords(SourceDocker, []IngestRecord{
		{
			SourceID: "docker-host",
			Resource: dockerResource,
		},
	})

	systems := MonitoredSystems(rr)
	if len(systems) != 1 {
		t.Fatalf("MonitoredSystems() returned %d systems, want 1", len(systems))
	}
	if systems[0].Status != StatusOffline {
		t.Fatalf("MonitoredSystems()[0].Status = %q, want %q", systems[0].Status, StatusOffline)
	}
}

func TestResourceRegistry_IngestRecords_UnknownSource(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC)

	customSource := DataSource("xcp")
	rr.IngestRecords(customSource, []IngestRecord{
		{
			SourceID: "host-1",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "xcp-host-1",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"xcp-host-1"}},
		},
	})

	hosts := rr.ListByType(ResourceTypeAgent)
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host for custom source, got %d", len(hosts))
	}
	if hosts[0].Name != "xcp-host-1" {
		t.Fatalf("expected host name xcp-host-1, got %q", hosts[0].Name)
	}
	targets := rr.SourceTargets(hosts[0].ID)
	if len(targets) != 1 {
		t.Fatalf("expected 1 source target, got %d", len(targets))
	}
	if targets[0].Source != customSource {
		t.Fatalf("expected custom source %q, got %q", customSource, targets[0].Source)
	}
	if targets[0].SourceID != "host-1" {
		t.Fatalf("expected source ID host-1, got %q", targets[0].SourceID)
	}
}

func TestResourceRegistry_IngestRecords_CanonicalizesSourceIDWhitespace(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{
			SourceID: " vm-100 ",
			Resource: Resource{
				Type:     ResourceTypeVM,
				Name:     "vm-100",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"vm-100"}},
		},
	})

	got := rr.sourceResourceID(SourceProxmox, "vm-100")
	if got == "" {
		t.Fatal("expected trimmed source ID to resolve canonical resource ID")
	}

	gotTrimmedVariant := rr.sourceResourceID(SourceProxmox, " vm-100 ")
	if gotTrimmedVariant != got {
		t.Fatalf("expected whitespace variants to resolve same resource ID, got %q want %q", gotTrimmedVariant, got)
	}

	targets := rr.SourceTargets(got)
	if len(targets) != 1 {
		t.Fatalf("expected 1 source target, got %d", len(targets))
	}
	if targets[0].SourceID != "vm-100" {
		t.Fatalf("expected canonical trimmed source ID, got %q", targets[0].SourceID)
	}
	if targets[0].CandidateID != rr.sourceSpecificID(ResourceTypeVM, SourceProxmox, "vm-100") {
		t.Fatalf("expected candidate ID to use canonical trimmed source ID, got %q", targets[0].CandidateID)
	}
}

func TestResourceRegistry_IngestSnapshotUnifiesLinkedProxmoxNodeViewsByHostIdentity(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:              "homelab-minipc",
				Name:            "minipc",
				Instance:        "homelab-entry",
				ClusterName:     "homelab",
				IsClusterMember: true,
				Host:            "https://10.0.0.5:8006",
				LinkedAgentID:   "host-1",
				Status:          "online",
				LastSeen:        now,
			},
			{
				ID:            "standalone-minipc",
				Name:          "minipc",
				Instance:      "minipc-standalone",
				Host:          "https://10.0.0.5:8006",
				LinkedAgentID: "host-1",
				Status:        "online",
				LastSeen:      now,
			},
		},
		Hosts: []models.Host{
			{
				ID:        "host-1",
				Hostname:  "minipc.local",
				MachineID: "machine-1",
				ReportIP:  "10.0.0.5",
				Status:    "online",
				LastSeen:  now,
				NetworkInterfaces: []models.HostNetworkInterface{
					{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"10.0.0.5/24"}},
				},
			},
		},
	})

	agents := rr.ListByType(ResourceTypeAgent)
	if len(agents) != 1 {
		t.Fatalf("expected 1 unified agent resource, got %d", len(agents))
	}
	resource := agents[0]
	if resource.Identity.MachineID != "machine-1" {
		t.Fatalf("MachineID = %q, want machine-1", resource.Identity.MachineID)
	}
	if !containsDataSource(resource.Sources, SourceAgent) || !containsDataSource(resource.Sources, SourceProxmox) {
		t.Fatalf("expected merged agent+proxmox sources, got %+v", resource.Sources)
	}
	if resource.Proxmox == nil || resource.Proxmox.NodeName != "minipc" {
		t.Fatalf("expected proxmox node metadata for minipc, got %+v", resource.Proxmox)
	}
}

func TestResourceRegistry_IngestSnapshotUnifiesAsymmetricLinkedProxmoxNodeViewsByHostIdentity(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:              "homelab-minipc",
				Name:            "minipc",
				Instance:        "homelab-entry",
				ClusterName:     "homelab",
				IsClusterMember: true,
				Host:            "https://10.0.0.5:8006",
				LinkedAgentID:   "host-1",
				Status:          "online",
				LastSeen:        now,
			},
			{
				ID:              "homelab-minipc-shadow",
				Name:            "minipc",
				Instance:        "homelab-shadow",
				ClusterName:     "homelab",
				IsClusterMember: true,
				Host:            "https://10.0.0.5:8006",
				Status:          "online",
				LastSeen:        now.Add(-time.Minute),
			},
		},
		Hosts: []models.Host{
			{
				ID:           "host-1",
				Hostname:     "minipc.local",
				MachineID:    "machine-1",
				ReportIP:     "10.0.0.5",
				Status:       "online",
				LastSeen:     now,
				LinkedNodeID: "homelab-minipc",
				NetworkInterfaces: []models.HostNetworkInterface{
					{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"10.0.0.5/24"}},
				},
			},
		},
	})

	agents := rr.ListByType(ResourceTypeAgent)
	if len(agents) != 1 {
		t.Fatalf("expected 1 unified agent resource, got %d", len(agents))
	}
	resource := agents[0]
	if resource.Identity.MachineID != "machine-1" {
		t.Fatalf("MachineID = %q, want machine-1", resource.Identity.MachineID)
	}
	if resource.Proxmox == nil {
		t.Fatalf("expected proxmox metadata")
	}
	if got := resource.Proxmox.LinkedAgentID; got != "host-1" {
		t.Fatalf("LinkedAgentID = %q, want host-1", got)
	}
	targets := rr.SourceTargets(resource.ID)
	if len(targets) != 3 {
		t.Fatalf("expected 3 source targets (2 proxmox + 1 agent), got %d", len(targets))
	}
}

func TestResourceRegistry_IngestSnapshotUnifiesHostLinkedProxmoxNodeViewsByHostIdentity(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:              "homelab-delly",
				Name:            "delly",
				Instance:        "homelab-entry",
				ClusterName:     "homelab",
				IsClusterMember: true,
				Host:            "https://10.0.0.9:8006",
				Status:          "online",
				LastSeen:        now,
			},
			{
				ID:              "homelab-delly-shadow",
				Name:            "delly",
				Instance:        "homelab-shadow",
				ClusterName:     "homelab",
				IsClusterMember: true,
				Host:            "https://10.0.0.9:8006",
				Status:          "online",
				LastSeen:        now.Add(-time.Minute),
			},
		},
		Hosts: []models.Host{
			{
				ID:           "host-1",
				Hostname:     "delly.local",
				MachineID:    "machine-delly",
				ReportIP:     "10.0.0.9",
				Status:       "online",
				LastSeen:     now,
				LinkedNodeID: "homelab-delly",
				NetworkInterfaces: []models.HostNetworkInterface{
					{Name: "eth0", MAC: "00:11:22:33:44:66", Addresses: []string{"10.0.0.9/24"}},
				},
			},
		},
	})

	agents := rr.ListByType(ResourceTypeAgent)
	if len(agents) != 1 {
		t.Fatalf("expected 1 unified agent resource, got %d", len(agents))
	}
	resource := agents[0]
	if resource.Identity.MachineID != "machine-delly" {
		t.Fatalf("MachineID = %q, want machine-delly", resource.Identity.MachineID)
	}
	if resource.Proxmox == nil {
		t.Fatalf("expected proxmox metadata")
	}
	if got := resource.Proxmox.LinkedAgentID; got != "host-1" {
		t.Fatalf("LinkedAgentID = %q, want host-1", got)
	}
	targets := rr.SourceTargets(resource.ID)
	if len(targets) != 3 {
		t.Fatalf("expected 3 source targets (2 proxmox + 1 agent), got %d", len(targets))
	}
}

func TestResourceRegistry_IngestSnapshotUnifiesHostLinkedProxmoxNodeViewsAcrossEndpointForms(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:       "minipc-ip-view",
				Name:     "minipc",
				Instance: "standalone-ip",
				Host:     "https://10.0.0.5:8006",
				Status:   "online",
				LastSeen: now,
			},
			{
				ID:       "minipc-hostname-view",
				Name:     "minipc",
				Instance: "standalone-hostname",
				Host:     "https://minipc.local:8006",
				Status:   "online",
				LastSeen: now.Add(-time.Minute),
			},
		},
		Hosts: []models.Host{
			{
				ID:           "host-1",
				Hostname:     "minipc.local",
				MachineID:    "machine-minipc",
				ReportIP:     "10.0.0.5",
				Status:       "online",
				LastSeen:     now,
				LinkedNodeID: "minipc-ip-view",
				NetworkInterfaces: []models.HostNetworkInterface{
					{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"10.0.0.5/24"}},
				},
			},
		},
	})

	agents := rr.ListByType(ResourceTypeAgent)
	if len(agents) != 1 {
		t.Fatalf("expected 1 unified agent resource, got %d", len(agents))
	}
	resource := agents[0]
	if resource.Identity.MachineID != "machine-minipc" {
		t.Fatalf("MachineID = %q, want machine-minipc", resource.Identity.MachineID)
	}
	if resource.Proxmox == nil {
		t.Fatalf("expected proxmox metadata")
	}
	targets := rr.SourceTargets(resource.ID)
	if len(targets) != 3 {
		t.Fatalf("expected 3 source targets (2 proxmox + 1 agent), got %d", len(targets))
	}
}

func TestResourceRegistry_IngestSnapshotCreatesPhysicalDisksFromHostSMART(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:       "host-tower",
				Hostname: "tower",
				Status:   "online",
				LastSeen: now,
				Disks: []models.Disk{
					{Device: "/dev/sdb", Total: 12 * 1024, Mountpoint: "/mnt/disk1"},
				},
				Sensors: models.HostSensorSummary{
					SMART: []models.HostDiskSMART{
						{
							Device:      "/dev/sdb",
							Model:       "Seagate IronWolf",
							Serial:      "SERIAL-TOWER-1",
							Type:        "sata",
							Temperature: 37,
							Health:      "PASSED",
						},
					},
				},
			},
		},
	})

	disks := rr.ListByType(ResourceTypePhysicalDisk)
	if len(disks) != 1 {
		t.Fatalf("expected 1 physical disk resource, got %d", len(disks))
	}
	disk := disks[0]
	if !containsDataSource(disk.Sources, SourceAgent) {
		t.Fatalf("expected agent-backed physical disk source, got %+v", disk.Sources)
	}
	if disk.ParentID == nil {
		t.Fatalf("expected host parent on physical disk")
	}
	parent, ok := rr.Get(*disk.ParentID)
	if !ok || parent == nil || parent.Name != "tower" {
		t.Fatalf("expected disk parent to resolve to tower host, got %+v", parent)
	}
	if disk.PhysicalDisk == nil || disk.PhysicalDisk.Serial != "SERIAL-TOWER-1" {
		t.Fatalf("expected SMART-backed disk metadata, got %+v", disk.PhysicalDisk)
	}
	if disk.Identity.MachineID != "SERIAL-TOWER-1" {
		t.Fatalf("MachineID = %q, want SERIAL-TOWER-1", disk.Identity.MachineID)
	}
	if disk.PhysicalDisk.Risk != nil {
		t.Fatalf("expected healthy disk to have no risk payload, got %+v", disk.PhysicalDisk.Risk)
	}
}

func TestResourceRegistry_IngestSnapshotMergesAgentAndProxmoxPhysicalDisksByIdentity(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		PhysicalDisks: []models.PhysicalDisk{
			{
				ID:          "pve-disk-1",
				Node:        "tower",
				Instance:    "pve-tower",
				DevPath:     "/dev/sdb",
				Model:       "Seagate IronWolf",
				Serial:      "SERIAL-TOWER-1",
				Type:        "sata",
				Health:      "PASSED",
				Temperature: 34,
				LastChecked: now,
			},
		},
		Hosts: []models.Host{
			{
				ID:       "host-tower",
				Hostname: "tower",
				Status:   "online",
				LastSeen: now,
				Sensors: models.HostSensorSummary{
					SMART: []models.HostDiskSMART{
						{
							Device:      "/dev/sdb",
							Model:       "Seagate IronWolf",
							Serial:      "SERIAL-TOWER-1",
							Type:        "sata",
							Temperature: 37,
							Health:      "PASSED",
						},
					},
				},
			},
		},
	})

	disks := rr.ListByType(ResourceTypePhysicalDisk)
	if len(disks) != 1 {
		t.Fatalf("expected 1 merged physical disk resource, got %d", len(disks))
	}
	disk := disks[0]
	if !containsDataSource(disk.Sources, SourceAgent) || !containsDataSource(disk.Sources, SourceProxmox) {
		t.Fatalf("expected merged proxmox+agent disk sources, got %+v", disk.Sources)
	}
	if disk.PhysicalDisk == nil || disk.PhysicalDisk.Temperature != 37 {
		t.Fatalf("expected merged SMART temperature from agent disk, got %+v", disk.PhysicalDisk)
	}
}

func TestResourceRegistry_IngestSnapshotPropagatesUnraidDiskRole(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:       "host-tower",
				Hostname: "tower",
				Status:   "online",
				LastSeen: now,
				Unraid: &models.HostUnraidStorage{
					ArrayStarted: true,
					Disks: []models.HostUnraidDisk{
						{Name: "parity", Device: "/dev/sdb", Role: "parity", Status: "online", Serial: "SERIAL-TOWER-1"},
					},
				},
				Sensors: models.HostSensorSummary{
					SMART: []models.HostDiskSMART{
						{
							Device:      "/dev/sdb",
							Model:       "Seagate IronWolf",
							Serial:      "SERIAL-TOWER-1",
							Type:        "sata",
							Temperature: 37,
							Health:      "PASSED",
						},
					},
				},
			},
		},
	})

	disks := rr.ListByType(ResourceTypePhysicalDisk)
	if len(disks) != 1 {
		t.Fatalf("expected 1 physical disk resource, got %d", len(disks))
	}
	if disks[0].PhysicalDisk == nil {
		t.Fatal("expected physical disk metadata")
	}
	if got := disks[0].PhysicalDisk.StorageRole; got != "parity" {
		t.Fatalf("storageRole = %q, want parity", got)
	}
	if got := disks[0].PhysicalDisk.StorageGroup; got != "unraid-array" {
		t.Fatalf("storageGroup = %q, want unraid-array", got)
	}
}

func TestResourceRegistry_IngestSnapshotCreatesUnraidStorageResource(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:        "host-tower",
				Hostname:  "tower",
				Status:    "online",
				LastSeen:  now,
				MachineID: "machine-tower",
				Disks: []models.Disk{
					{Mountpoint: "/mnt/user", Total: 1000, Used: 400, Free: 600, Usage: 40},
				},
				Unraid: &models.HostUnraidStorage{
					ArrayStarted: true,
					ArrayState:   "STARTED",
					NumProtected: 1,
					Disks: []models.HostUnraidDisk{
						{Name: "parity", Role: "parity", Status: "online"},
						{Name: "disk1", Role: "data", Status: "online"},
					},
				},
			},
		},
	})

	storage := rr.ListByType(ResourceTypeStorage)
	if len(storage) != 1 {
		t.Fatalf("expected 1 unraid storage resource, got %d", len(storage))
	}
	resource := storage[0]
	if !containsDataSource(resource.Sources, SourceAgent) {
		t.Fatalf("expected agent-backed storage source, got %+v", resource.Sources)
	}
	if resource.ParentID == nil {
		t.Fatalf("expected host parent on unraid storage")
	}
	if resource.Storage == nil || resource.Storage.Type != "unraid-array" || resource.Storage.Platform != "unraid" {
		t.Fatalf("expected unraid storage metadata, got %+v", resource.Storage)
	}
	if resource.Metrics == nil || resource.Metrics.Disk == nil || resource.Metrics.Disk.Percent != 40 {
		t.Fatalf("expected disk metrics from unraid storage, got %+v", resource.Metrics)
	}
}

func TestResourceRegistry_IngestSnapshotParentsUnraidArrayDisksUnderStorage(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:        "host-tower",
				Hostname:  "tower",
				Status:    "online",
				LastSeen:  now,
				MachineID: "machine-tower",
				Unraid: &models.HostUnraidStorage{
					ArrayStarted: true,
					NumProtected: 1,
					Disks: []models.HostUnraidDisk{
						{Name: "parity", Device: "/dev/sdb", Role: "parity", Status: "online", Serial: "SERIAL-TOWER-1"},
					},
				},
				Sensors: models.HostSensorSummary{
					SMART: []models.HostDiskSMART{
						{
							Device:      "/dev/sdb",
							Model:       "Seagate IronWolf",
							Serial:      "SERIAL-TOWER-1",
							Type:        "sata",
							Temperature: 37,
							Health:      "PASSED",
						},
					},
				},
			},
		},
	})

	storage := rr.ListByType(ResourceTypeStorage)
	disks := rr.ListByType(ResourceTypePhysicalDisk)
	if len(storage) != 1 || len(disks) != 1 {
		t.Fatalf("expected 1 storage and 1 disk resource, got storage=%d disk=%d", len(storage), len(disks))
	}
	if disks[0].ParentID == nil || *disks[0].ParentID != storage[0].ID {
		t.Fatalf("expected unraid disk parent to be storage resource %q, got %+v", storage[0].ID, disks[0].ParentID)
	}
	if storage[0].ChildCount != 1 {
		t.Fatalf("expected storage child count 1, got %d", storage[0].ChildCount)
	}
}

func TestResourceRegistry_IngestSnapshotDerivesStorageConsumers(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Storage: []models.Storage{
			{
				ID:       "cluster-a-pve1-local-lvm",
				Name:     "local-lvm",
				Node:     "pve1",
				Instance: "cluster-a",
				Type:     "lvmthin",
				Status:   "available",
				Enabled:  true,
				Active:   true,
			},
			{
				ID:       "cluster-a-pve2-local-lvm",
				Name:     "local-lvm",
				Node:     "pve2",
				Instance: "cluster-a",
				Type:     "lvmthin",
				Status:   "available",
				Enabled:  true,
				Active:   true,
			},
			{
				ID:       "cluster-a-cluster-ceph",
				Name:     "ceph",
				Node:     "cluster",
				Instance: "cluster-a",
				Type:     "rbd",
				Status:   "available",
				Enabled:  true,
				Active:   true,
				Shared:   true,
				Nodes:    []string{"pve1", "pve2"},
			},
			{
				ID:       "cluster-a-pve1-media",
				Name:     "media",
				Node:     "pve1",
				Instance: "cluster-a",
				Type:     "dir",
				Status:   "available",
				Enabled:  true,
				Active:   true,
				Path:     "/mnt/pve/media",
			},
		},
		VMs: []models.VM{
			{
				ID:       "vm-100",
				Name:     "app01",
				Node:     "pve1",
				Instance: "cluster-a",
				Status:   "running",
				LastSeen: now,
				Disks: []models.Disk{
					{Device: "local-lvm:vm-100-disk-0"},
					{Device: "ceph:vm-100-disk-1"},
				},
			},
		},
		Containers: []models.Container{
			{
				ID:       "ct-200",
				Name:     "media01",
				Node:     "pve1",
				Instance: "cluster-a",
				Status:   "running",
				LastSeen: now,
				Disks: []models.Disk{
					{Device: "/mnt/pve/media/subvol-200-disk-1"},
					{Device: "local-lvm:vm-200-disk-0"},
				},
			},
		},
	})

	storageResources := rr.ListByType(ResourceTypeStorage)

	local := findStorageResource(storageResources, "local-lvm", "pve1")
	if local.Storage == nil {
		t.Fatalf("expected local-lvm storage metadata, got %+v", local)
	}
	if got := local.Storage.ConsumerCount; got != 2 {
		t.Fatalf("local-lvm consumerCount = %d, want 2", got)
	}
	if got := local.Storage.ConsumerTypes; len(got) != 2 || got[0] != "system-container" || got[1] != "vm" {
		t.Fatalf("local-lvm consumerTypes = %v, want [system-container vm]", got)
	}
	if len(local.Storage.TopConsumers) != 2 {
		t.Fatalf("local-lvm topConsumers length = %d, want 2", len(local.Storage.TopConsumers))
	}
	if !hasStorageConsumer(local.Storage.TopConsumers, "app01", ResourceTypeVM, 1) {
		t.Fatalf("expected vm consumer on local-lvm, got %+v", local.Storage.TopConsumers)
	}
	if !hasStorageConsumer(local.Storage.TopConsumers, "media01", ResourceTypeSystemContainer, 1) {
		t.Fatalf("expected container consumer on local-lvm, got %+v", local.Storage.TopConsumers)
	}
	if got := local.Storage.ConsumerImpactSummary; got != "Affects 2 dependent resources: media01, app01" {
		t.Fatalf("local-lvm consumerImpactSummary = %q", got)
	}

	ceph := findStorageResource(storageResources, "ceph", "cluster")
	if ceph.Storage == nil || ceph.Storage.ConsumerCount != 1 {
		t.Fatalf("ceph consumerCount = %+v, want 1", ceph.Storage)
	}
	if !hasStorageConsumer(ceph.Storage.TopConsumers, "app01", ResourceTypeVM, 1) {
		t.Fatalf("expected vm consumer on ceph, got %+v", ceph.Storage.TopConsumers)
	}

	media := findStorageResource(storageResources, "media", "pve1")
	if media.Storage == nil || media.Storage.ConsumerCount != 1 {
		t.Fatalf("media consumerCount = %+v, want 1", media.Storage)
	}
	if !hasStorageConsumer(media.Storage.TopConsumers, "media01", ResourceTypeSystemContainer, 1) {
		t.Fatalf("expected container consumer on media, got %+v", media.Storage.TopConsumers)
	}

	otherLocal := findStorageResource(storageResources, "local-lvm", "pve2")
	if otherLocal.Storage == nil {
		t.Fatalf("expected second local-lvm storage metadata")
	}
	if otherLocal.Storage.ConsumerCount != 0 {
		t.Fatalf("expected pve2 local-lvm to remain without consumers, got %+v", otherLocal.Storage)
	}
}

func TestResourceRegistry_IngestSnapshotDerivesPBSDatastoreConsumers(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		PBSInstances: []models.PBSInstance{
			{
				ID:       "pbs-1",
				Name:     "pbs-main",
				Host:     "https://pbs-main.local:8007",
				Status:   "online",
				LastSeen: now,
				Datastores: []models.PBSDatastore{
					{Name: "backup-store", Status: "online"},
				},
			},
		},
		PBSBackups: []models.PBSBackup{
			{
				ID:         "pbs-1/backup-store/vm/100",
				Instance:   "pbs-main",
				Datastore:  "backup-store",
				Namespace:  "pve",
				BackupType: "vm",
				VMID:       "100",
				BackupTime: now,
			},
			{
				ID:         "pbs-1/backup-store/ct/200",
				Instance:   "pbs-main",
				Datastore:  "backup-store",
				Namespace:  "nat",
				BackupType: "ct",
				VMID:       "200",
				BackupTime: now,
			},
		},
		VMs: []models.VM{
			{
				ID:       "vm-100",
				Name:     "app01",
				Node:     "pve-1",
				Instance: "pve",
				Status:   "running",
				LastSeen: now,
				VMID:     100,
			},
		},
		Containers: []models.Container{
			{
				ID:       "ct-200",
				Name:     "media01",
				Node:     "pve-2",
				Instance: "pve-nat",
				Status:   "running",
				LastSeen: now,
				VMID:     200,
			},
		},
	})

	storageResources := rr.ListByType(ResourceTypeStorage)
	datastore := findStorageResourceByPlatform(storageResources, "backup-store", "pbs", "datastore")
	if datastore.Storage == nil {
		t.Fatalf("expected PBS datastore storage metadata, got %+v", datastore)
	}
	if got := datastore.Storage.ConsumerCount; got != 2 {
		t.Fatalf("backup-store consumerCount = %d, want 2", got)
	}
	if got := datastore.Storage.ConsumerTypes; len(got) != 2 || got[0] != "system-container" || got[1] != "vm" {
		t.Fatalf("backup-store consumerTypes = %v, want [system-container vm]", got)
	}
	if len(datastore.Storage.TopConsumers) != 2 {
		t.Fatalf("backup-store topConsumers length = %d, want 2", len(datastore.Storage.TopConsumers))
	}
	if !hasStorageConsumer(datastore.Storage.TopConsumers, "app01", ResourceTypeVM, 1) {
		t.Fatalf("expected vm consumer on backup-store, got %+v", datastore.Storage.TopConsumers)
	}
	if !hasStorageConsumer(datastore.Storage.TopConsumers, "media01", ResourceTypeSystemContainer, 1) {
		t.Fatalf("expected container consumer on backup-store, got %+v", datastore.Storage.TopConsumers)
	}
	if got := datastore.Storage.ConsumerImpactSummary; got != "Puts backups for 2 protected workloads at risk: media01, app01" {
		t.Fatalf("backup-store consumerImpactSummary = %q", got)
	}

	pbsResources := rr.ListByType(ResourceTypePBS)
	if len(pbsResources) != 1 {
		t.Fatalf("expected 1 PBS resource, got %d", len(pbsResources))
	}
	pbs := pbsResources[0]
	if pbs.PBS == nil {
		t.Fatalf("expected PBS payload, got %+v", pbs)
	}
	if got := pbs.PBS.ProtectedWorkloadCount; got != 2 {
		t.Fatalf("protectedWorkloadCount = %d, want 2", got)
	}
	if got := pbs.PBS.ProtectedWorkloadTypes; len(got) != 2 || got[0] != "system-container" || got[1] != "vm" {
		t.Fatalf("protectedWorkloadTypes = %v, want [system-container vm]", got)
	}
	if got := pbs.PBS.ProtectedWorkloadNames; len(got) != 2 || got[0] != "media01" || got[1] != "app01" {
		t.Fatalf("protectedWorkloadNames = %v, want [media01 app01] in sorted rollup order", got)
	}
	if got := pbs.PBS.AffectedDatastoreCount; got != 0 {
		t.Fatalf("affectedDatastoreCount = %d, want 0 without datastore risk", got)
	}
	if got := pbs.PBS.ProtectedWorkloadSummary; got != "Puts backups for 2 protected workloads at risk: media01, app01" {
		t.Fatalf("protectedWorkloadSummary = %q", got)
	}
	if got := pbs.PBS.PostureSummary; got != "Puts backups for 2 protected workloads at risk: media01, app01" {
		t.Fatalf("postureSummary = %q", got)
	}
}

func TestResourceRegistry_IngestResourcesDerivesPrimaryIncidentRollups(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestResources([]Resource{
		{
			ID:     "storage:tank",
			Type:   ResourceTypeStorage,
			Name:   "tank",
			Status: StatusWarning,
			Storage: &StorageMeta{
				Platform:   "truenas",
				Topology:   "pool",
				Protection: "zfs",
				Risk: &StorageRisk{
					Level: storagehealth.RiskWarning,
					Reasons: []StorageRiskReason{
						{Code: "capacity_runway_low", Severity: storagehealth.RiskWarning, Summary: "Storage tank is 92% full"},
						{Code: "zfs_pool_state", Severity: storagehealth.RiskWarning, Summary: "ZFS pool tank is DEGRADED"},
					},
				},
			},
			Incidents: []ResourceIncident{
				{Code: "capacity_runway_low", Severity: storagehealth.RiskWarning, Summary: "Storage tank is 92% full"},
				{Code: "zfs_pool_state", Severity: storagehealth.RiskWarning, Summary: "ZFS pool tank is DEGRADED"},
			},
		},
		{
			ID:     "pbs:main",
			Type:   ResourceTypePBS,
			Name:   "pbs-main",
			Status: StatusWarning,
			PBS: &PBSData{
				StorageRisk: &StorageRisk{
					Level: storagehealth.RiskWarning,
					Reasons: []StorageRiskReason{
						{Code: "pbs_datastore_state", Severity: storagehealth.RiskWarning, Summary: "PBS datastore archive is READ_ONLY"},
						{Code: "capacity_runway_low", Severity: storagehealth.RiskWarning, Summary: "PBS datastore fast is 96% full"},
					},
				},
				AffectedDatastoreSummary: "Affects 1 backup datastore: fast",
				ProtectedWorkloadSummary: "Puts backups for 2 protected workloads at risk: media01, app01",
				PostureSummary:           "Affects 1 backup datastore: fast. Puts backups for 2 protected workloads at risk: media01, app01",
			},
			Incidents: []ResourceIncident{
				{Code: "pbs_datastore_state", Severity: storagehealth.RiskWarning, Summary: "PBS datastore archive is READ_ONLY"},
				{Code: "capacity_runway_low", Severity: storagehealth.RiskWarning, Summary: "PBS datastore fast is 96% full"},
			},
		},
	})

	storage, ok := rr.Get("storage:tank")
	if !ok {
		t.Fatal("expected storage resource")
	}
	if storage.IncidentCount != 2 || storage.IncidentCode != "zfs_pool_state" || storage.IncidentSeverity != storagehealth.RiskWarning || storage.IncidentSummary != "ZFS pool tank is DEGRADED" || storage.IncidentCategory != IncidentCategoryProtection || storage.IncidentLabel != "Protection Reduced" || storage.IncidentPriority != 3400 || storage.IncidentImpactSummary != "" || storage.IncidentUrgency != IncidentUrgencyToday || storage.IncidentAction != "Investigate degraded protection and schedule maintenance to restore redundancy" {
		t.Fatalf("unexpected storage incident rollup %+v", storage)
	}

	pbs, ok := rr.Get("pbs:main")
	if !ok {
		t.Fatal("expected pbs resource")
	}
	if pbs.IncidentCount != 2 || pbs.IncidentCode != "pbs_datastore_state" || pbs.IncidentSeverity != storagehealth.RiskWarning || pbs.IncidentSummary != "PBS datastore archive is READ_ONLY" || pbs.IncidentCategory != IncidentCategoryRecoverability || pbs.IncidentLabel != "Backup Coverage At Risk" || pbs.IncidentPriority != 3500 || pbs.IncidentImpactSummary != "" || pbs.IncidentUrgency != IncidentUrgencyToday || pbs.IncidentAction != "Investigate backup target health and preserve backup coverage" {
		t.Fatalf("unexpected pbs incident rollup %+v", pbs)
	}
}

func TestResourceRegistry_IngestSnapshotDerivesPhysicalDiskRisk(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)
	pending := int64(2)

	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:       "host-risky",
				Hostname: "tower",
				Status:   "online",
				LastSeen: now,
				Sensors: models.HostSensorSummary{
					SMART: []models.HostDiskSMART{
						{
							Device:      "/dev/sdc",
							Model:       "Seagate IronWolf",
							Serial:      "SERIAL-RISK-1",
							Type:        "sata",
							Temperature: 64,
							Health:      "PASSED",
							Attributes: &models.SMARTAttributes{
								PendingSectors: &pending,
							},
						},
					},
				},
			},
		},
	})

	disks := rr.ListByType(ResourceTypePhysicalDisk)
	if len(disks) != 1 {
		t.Fatalf("expected 1 physical disk resource, got %d", len(disks))
	}
	disk := disks[0]
	if disk.Status != StatusWarning {
		t.Fatalf("Status = %q, want %q", disk.Status, StatusWarning)
	}
	if disk.PhysicalDisk == nil || disk.PhysicalDisk.Risk == nil {
		t.Fatalf("expected disk risk payload, got %+v", disk.PhysicalDisk)
	}
	if disk.PhysicalDisk.Risk.Level != storagehealth.RiskCritical {
		t.Fatalf("risk level = %q, want %q", disk.PhysicalDisk.Risk.Level, storagehealth.RiskCritical)
	}
	if len(disk.PhysicalDisk.Risk.Reasons) == 0 || disk.PhysicalDisk.Risk.Reasons[0].Code != "pending_sectors" {
		t.Fatalf("expected pending sectors reason, got %+v", disk.PhysicalDisk.Risk.Reasons)
	}
}

func TestResourceRegistry_MergePhysicalDiskKeepsIncidentBackedRisk(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "agent-disk-sdc",
			Resource: Resource{
				Type:     ResourceTypePhysicalDisk,
				Name:     "sdc",
				Status:   StatusOnline,
				LastSeen: now,
				PhysicalDisk: &PhysicalDiskMeta{
					DevPath:     "/dev/sdc",
					Model:       "Seagate IronWolf",
					Serial:      "SERIAL-RISK-2",
					DiskType:    "sata",
					SizeBytes:   4_000_000_000_000,
					Health:      "PASSED",
					Temperature: 38,
					Wearout:     -1,
				},
			},
			Identity: ResourceIdentity{
				MachineID: "SERIAL-RISK-2",
				Hostnames: []string{"truenas-main"},
			},
		},
	})

	rr.IngestRecords(SourceTrueNAS, []IngestRecord{
		{
			SourceID: "truenas-disk-sdc",
			Resource: Resource{
				Type:     ResourceTypePhysicalDisk,
				Name:     "sdc",
				Status:   StatusWarning,
				LastSeen: now,
				PhysicalDisk: &PhysicalDiskMeta{
					DevPath:     "/dev/sdc",
					Model:       "Seagate IronWolf",
					Serial:      "SERIAL-RISK-2",
					DiskType:    "sata",
					SizeBytes:   4_000_000_000_000,
					Health:      "UNKNOWN",
					Temperature: 63,
					Wearout:     -1,
				},
				Incidents: []ResourceIncident{
					{
						Code:     "truenas_smart",
						Severity: storagehealth.RiskCritical,
						Summary:  "Device /dev/sdc has SMART test failures.",
					},
				},
			},
			Identity: ResourceIdentity{
				MachineID: "SERIAL-RISK-2",
				Hostnames: []string{"truenas-main"},
			},
		},
	})

	disks := rr.ListByType(ResourceTypePhysicalDisk)
	if len(disks) != 1 {
		t.Fatalf("expected 1 merged physical disk, got %d", len(disks))
	}
	disk := disks[0]
	if disk.PhysicalDisk == nil || disk.PhysicalDisk.Risk == nil {
		t.Fatalf("expected merged disk risk, got %+v", disk.PhysicalDisk)
	}
	if disk.PhysicalDisk.Risk.Level != storagehealth.RiskCritical {
		t.Fatalf("risk level = %q, want %q", disk.PhysicalDisk.Risk.Level, storagehealth.RiskCritical)
	}
	foundSmartReason := false
	for _, reason := range disk.PhysicalDisk.Risk.Reasons {
		if reason.Code == "truenas_smart" {
			foundSmartReason = true
			break
		}
	}
	if !foundSmartReason {
		t.Fatalf("expected SMART-backed risk reason after merge, got %+v", disk.PhysicalDisk.Risk.Reasons)
	}
}

func hasStorageConsumer(consumers []StorageConsumerMeta, name string, resourceType ResourceType, diskCount int) bool {
	for _, consumer := range consumers {
		if consumer.Name == name && consumer.ResourceType == resourceType && consumer.DiskCount == diskCount {
			return true
		}
	}
	return false
}

func findStorageResource(resources []Resource, name, node string) Resource {
	for _, resource := range resources {
		if resource.Name != name || resource.Proxmox == nil || resource.Proxmox.NodeName != node {
			continue
		}
		return resource
	}
	return Resource{}
}

func findStorageResourceByPlatform(resources []Resource, name, platform, topology string) Resource {
	for _, resource := range resources {
		if resource.Name != name || resource.Storage == nil {
			continue
		}
		if resource.Storage.Platform != platform || resource.Storage.Topology != topology {
			continue
		}
		return resource
	}
	return Resource{}
}

func TestResourceRegistry_BuildChildCounts_ReparentClearsOldParentCount(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 2, 12, 1, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{
			SourceID: "node-a",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "node-a",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"node-a"}},
		},
		{
			SourceID: "node-b",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "node-b",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"node-b"}},
		},
		{
			SourceID:       "vm-100",
			ParentSourceID: "node-a",
			Resource: Resource{
				Type:     ResourceTypeVM,
				Name:     "vm-100",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"vm-100"}},
		},
	})

	parentAID := rr.sourceSpecificID(ResourceTypeAgent, SourceProxmox, "node-a")
	parentBID := rr.sourceSpecificID(ResourceTypeAgent, SourceProxmox, "node-b")
	vmID := rr.sourceSpecificID(ResourceTypeVM, SourceProxmox, "vm-100")

	parentA, ok := rr.Get(parentAID)
	if !ok {
		t.Fatalf("expected parent A %q to exist", parentAID)
	}
	parentB, ok := rr.Get(parentBID)
	if !ok {
		t.Fatalf("expected parent B %q to exist", parentBID)
	}
	vm, ok := rr.Get(vmID)
	if !ok {
		t.Fatalf("expected vm %q to exist", vmID)
	}
	if parentA.ChildCount != 1 || parentB.ChildCount != 0 {
		t.Fatalf("expected initial child counts parentA=1 parentB=0, got parentA=%d parentB=%d", parentA.ChildCount, parentB.ChildCount)
	}
	if vm.ParentName != "node-a" {
		t.Fatalf("expected vm parent name %q, got %q", "node-a", vm.ParentName)
	}

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{
			SourceID: "node-a",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "node-a",
				Status:   StatusOnline,
				LastSeen: now.Add(30 * time.Second),
			},
			Identity: ResourceIdentity{Hostnames: []string{"node-a"}},
		},
		{
			SourceID: "node-b",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "node-b",
				Status:   StatusOnline,
				LastSeen: now.Add(30 * time.Second),
			},
			Identity: ResourceIdentity{Hostnames: []string{"node-b"}},
		},
		{
			SourceID:       "vm-100",
			ParentSourceID: "node-b",
			Resource: Resource{
				Type:     ResourceTypeVM,
				Name:     "vm-100",
				Status:   StatusOnline,
				LastSeen: now.Add(30 * time.Second),
			},
			Identity: ResourceIdentity{Hostnames: []string{"vm-100"}},
		},
	})

	parentA, ok = rr.Get(parentAID)
	if !ok {
		t.Fatalf("expected parent A %q to exist after reparent", parentAID)
	}
	parentB, ok = rr.Get(parentBID)
	if !ok {
		t.Fatalf("expected parent B %q to exist after reparent", parentBID)
	}
	vm, ok = rr.Get(vmID)
	if !ok {
		t.Fatalf("expected vm %q to exist after reparent", vmID)
	}
	if parentA.ChildCount != 0 || parentB.ChildCount != 1 {
		t.Fatalf("expected child counts parentA=0 parentB=1 after reparent, got parentA=%d parentB=%d", parentA.ChildCount, parentB.ChildCount)
	}
	if vm.ParentName != "node-b" {
		t.Fatalf("expected vm parent name %q after reparent, got %q", "node-b", vm.ParentName)
	}
}

func TestResourceRegistry_BuildChildCounts_SourceUpdateClearsRemovedParent(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 2, 12, 1, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{
			SourceID: "node-a",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "node-a",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"node-a"}},
		},
		{
			SourceID:       "vm-100",
			ParentSourceID: "node-a",
			Resource: Resource{
				Type:     ResourceTypeVM,
				Name:     "vm-100",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{Hostnames: []string{"vm-100"}},
		},
	})

	parentID := rr.sourceSpecificID(ResourceTypeAgent, SourceProxmox, "node-a")
	vmID := rr.sourceSpecificID(ResourceTypeVM, SourceProxmox, "vm-100")

	parent, ok := rr.Get(parentID)
	if !ok {
		t.Fatalf("expected parent %q to exist", parentID)
	}
	vm, ok := rr.Get(vmID)
	if !ok {
		t.Fatalf("expected vm %q to exist", vmID)
	}
	if parent.ChildCount != 1 {
		t.Fatalf("expected parent child count 1 before clearing parent, got %d", parent.ChildCount)
	}
	if vm.ParentID == nil || *vm.ParentID != parentID {
		t.Fatalf("expected vm parent id %q before clearing parent, got %+v", parentID, vm.ParentID)
	}

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{
			SourceID: "node-a",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "node-a",
				Status:   StatusOnline,
				LastSeen: now.Add(30 * time.Second),
			},
			Identity: ResourceIdentity{Hostnames: []string{"node-a"}},
		},
		{
			SourceID: "vm-100",
			Resource: Resource{
				Type:     ResourceTypeVM,
				Name:     "vm-100",
				Status:   StatusOnline,
				LastSeen: now.Add(30 * time.Second),
			},
			Identity: ResourceIdentity{Hostnames: []string{"vm-100"}},
		},
	})

	parent, ok = rr.Get(parentID)
	if !ok {
		t.Fatalf("expected parent %q to exist after clearing parent", parentID)
	}
	vm, ok = rr.Get(vmID)
	if !ok {
		t.Fatalf("expected vm %q to exist after clearing parent", vmID)
	}
	if parent.ChildCount != 0 {
		t.Fatalf("expected parent child count 0 after clearing parent, got %d", parent.ChildCount)
	}
	if vm.ParentID != nil {
		t.Fatalf("expected vm parent id to clear after source update, got %+v", vm.ParentID)
	}
	if vm.ParentName != "" {
		t.Fatalf("expected vm parent name to clear after source update, got %q", vm.ParentName)
	}
}

func TestRegistryMonitoredSystemCountDedupesAcrossTopLevelViews(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "host-1",
			Resource: Resource{
				ID:     "host-1",
				Type:   ResourceTypeAgent,
				Name:   "lab-a",
				Status: StatusOnline,
				Agent: &AgentData{
					AgentID:   "agent-1",
					Hostname:  "lab-a",
					MachineID: "machine-1",
				},
				Identity: ResourceIdentity{
					MachineID: "machine-1",
					Hostnames: []string{"lab-a"},
				},
			},
		},
	})
	rr.IngestRecords(SourcePBS, []IngestRecord{
		{
			SourceID: "pbs-1",
			Resource: Resource{
				ID:     "pbs-1",
				Type:   ResourceTypePBS,
				Name:   "pbs-a",
				Status: StatusOnline,
				PBS: &PBSData{
					InstanceID: "pbs-1",
					Hostname:   "lab-a",
					HostURL:    "https://lab-a:8007",
				},
			},
		},
	})
	rr.IngestRecords(SourcePMG, []IngestRecord{
		{
			SourceID: "pmg-1",
			Resource: Resource{
				ID:     "pmg-1",
				Type:   ResourceTypePMG,
				Name:   "pmg-b",
				Status: StatusOnline,
				PMG: &PMGData{
					InstanceID: "pmg-1",
					Hostname:   "mail-b",
				},
			},
		},
	})
	rr.IngestRecords(SourceK8s, []IngestRecord{
		{
			SourceID: "k8s-1",
			Resource: Resource{
				ID:     "k8s-1",
				Type:   ResourceTypeK8sCluster,
				Name:   "cluster-a",
				Status: StatusOnline,
				Kubernetes: &K8sData{
					ClusterID: "cluster-a",
					AgentID:   "k8s-agent-1",
					Server:    "https://cluster-a.example:6443",
				},
			},
		},
	})

	if got := MonitoredSystemCount(rr); got != 3 {
		t.Fatalf("MonitoredSystemCount() = %d, want 3", got)
	}
}

func TestRegistryHasMatchingMonitoredSystemUsesCanonicalHostIdentity(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestRecords(SourceTrueNAS, []IngestRecord{
		{
			SourceID: "truenas-1",
			Resource: Resource{
				ID:     "truenas-1",
				Type:   ResourceTypeAgent,
				Name:   "archive",
				Status: StatusOnline,
				TrueNAS: &TrueNASData{
					Hostname: "archive.local",
				},
				Identity: ResourceIdentity{
					Hostnames: []string{"archive.local"},
				},
			},
		},
	})

	if !HasMatchingMonitoredSystem(rr, MonitoredSystemCandidate{
		Type:     ResourceTypeAgent,
		Hostname: "archive.local",
		HostURL:  "https://archive.local",
	}) {
		t.Fatal("expected candidate to match existing counted system")
	}

	if HasMatchingMonitoredSystem(rr, MonitoredSystemCandidate{
		Type:     ResourceTypePBS,
		Hostname: "other.local",
		HostURL:  "https://other.local:8007",
	}) {
		t.Fatal("expected unrelated candidate not to match existing counted system")
	}
}

func TestRegistryIngestSnapshotPublishesCanonicalKubernetesMetricsTargets(t *testing.T) {
	rr := NewRegistry(nil)

	rr.IngestSnapshot(models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:          "cluster-1",
				Name:        "cluster-a",
				DisplayName: "Cluster A",
				AgentID:     "agent-cluster-1",
				Context:     "cluster-a",
				Nodes: []models.KubernetesNode{
					{
						UID:  "node-1",
						Name: "worker-1",
					},
				},
				Pods: []models.KubernetesPod{
					{
						UID:       "pod-1",
						Namespace: "default",
						Name:      "api-7dd5f6c7f9-jx2ql",
					},
				},
				Deployments: []models.KubernetesDeployment{
					{
						UID:       "deploy-1",
						Namespace: "default",
						Name:      "api",
					},
				},
			},
		},
	})

	assertMetricsTarget := func(resourceType ResourceType, want string) {
		t.Helper()
		resources := rr.ListByType(resourceType)
		if len(resources) != 1 {
			t.Fatalf("ListByType(%q) len = %d, want 1", resourceType, len(resources))
		}
		target := BuildMetricsTargetForRegistry(rr, resources[0].ID)
		if target == nil {
			t.Fatalf("BuildMetricsTargetForRegistry(%q) returned nil MetricsTarget", resources[0].ID)
		}
		if target.ResourceID != want {
			t.Fatalf("ListByType(%q) metrics target = %q, want %q", resourceType, target.ResourceID, want)
		}
	}

	assertMetricsTarget(ResourceTypeK8sCluster, "cluster-1")
	assertMetricsTarget(ResourceTypeK8sNode, "cluster-1:node:node-1")
	assertMetricsTarget(ResourceTypePod, "k8s:cluster-1:pod:pod-1")
	assertMetricsTarget(ResourceTypeK8sDeployment, "cluster-1:deployment:deploy-1")
}

func TestRegistryHasMatchingMonitoredSystemRejectsInvalidCandidate(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "host-1",
			Resource: Resource{
				ID:     "host-1",
				Type:   ResourceTypeAgent,
				Name:   "lab-a",
				Status: StatusOnline,
				Agent: &AgentData{
					AgentID:   "agent-1",
					Hostname:  "lab-a",
					MachineID: "machine-1",
				},
				Identity: ResourceIdentity{
					MachineID: "machine-1",
					Hostnames: []string{"lab-a"},
				},
			},
		},
	})

	if HasMatchingMonitoredSystem(rr, MonitoredSystemCandidate{}) {
		t.Fatal("expected empty candidate not to match an existing counted system")
	}

	if HasMatchingMonitoredSystem(rr, MonitoredSystemCandidate{
		Source:   SourceAgent,
		Name:     " ",
		Hostname: " ",
		HostURL:  " ",
	}) {
		t.Fatal("expected whitespace-only candidate not to match an existing counted system")
	}
}

func TestProjectMonitoredSystemRecords_DedupesVMwareHostAgainstExistingAgentCount(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "agent-host-1",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "esxi-01.lab.local",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{
				DMIUUID:   "uuid-host-1",
				Hostnames: []string{"esxi-01.lab.local"},
			},
		},
	})

	projection := ProjectMonitoredSystemRecords(rr, map[DataSource][]IngestRecord{
		SourceVMware: {
			{
				SourceID: "vc-1:host:host-101",
				Resource: Resource{
					Type:     ResourceTypeAgent,
					Name:     "esxi-01.lab.local",
					Status:   StatusOnline,
					LastSeen: now,
					VMware: &VMwareData{
						ConnectionID:    "vc-1",
						ConnectionName:  "Lab VC",
						ManagedObjectID: "host-101",
						EntityType:      "host",
						HostUUID:        "uuid-host-1",
					},
				},
				Identity: ResourceIdentity{
					DMIUUID:   "uuid-host-1",
					Hostnames: []string{"esxi-01.lab.local"},
				},
			},
		},
	})

	if projection.CurrentCount != 1 {
		t.Fatalf("CurrentCount = %d, want 1", projection.CurrentCount)
	}
	if projection.ProjectedCount != 1 {
		t.Fatalf("ProjectedCount = %d, want 1", projection.ProjectedCount)
	}
	if projection.AdditionalCount != 0 {
		t.Fatalf("AdditionalCount = %d, want 0", projection.AdditionalCount)
	}
}

func TestPreviewMonitoredSystemRecords_ReturnsCurrentAndProjectedSystemsForVMwareAdmission(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "agent-host-1",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "esxi-01.lab.local",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{
				DMIUUID:   "uuid-host-1",
				Hostnames: []string{"esxi-01.lab.local"},
			},
		},
	})

	preview := PreviewMonitoredSystemRecords(rr, map[DataSource][]IngestRecord{
		SourceVMware: {
			{
				SourceID: "vc-1:host:host-101",
				Resource: Resource{
					Type:     ResourceTypeAgent,
					Name:     "esxi-01.lab.local",
					Status:   StatusOnline,
					LastSeen: now,
					VMware: &VMwareData{
						ConnectionID:    "vc-1",
						ConnectionName:  "Lab VC",
						ManagedObjectID: "host-101",
						EntityType:      "host",
						HostUUID:        "uuid-host-1",
					},
				},
				Identity: ResourceIdentity{
					DMIUUID:   "uuid-host-1",
					Hostnames: []string{"esxi-01.lab.local"},
				},
			},
			{
				SourceID: "vc-1:host:host-102",
				Resource: Resource{
					Type:     ResourceTypeAgent,
					Name:     "esxi-02.lab.local",
					Status:   StatusOnline,
					LastSeen: now,
					VMware: &VMwareData{
						ConnectionID:    "vc-1",
						ConnectionName:  "Lab VC",
						ManagedObjectID: "host-102",
						EntityType:      "host",
						HostUUID:        "uuid-host-2",
					},
				},
				Identity: ResourceIdentity{
					DMIUUID:   "uuid-host-2",
					Hostnames: []string{"esxi-02.lab.local"},
				},
			},
		},
	})

	if preview.CurrentCount != 1 {
		t.Fatalf("CurrentCount = %d, want 1", preview.CurrentCount)
	}
	if preview.ProjectedCount != 2 {
		t.Fatalf("ProjectedCount = %d, want 2", preview.ProjectedCount)
	}
	if preview.AdditionalCount != 1 {
		t.Fatalf("AdditionalCount = %d, want 1", preview.AdditionalCount)
	}
	if len(preview.CurrentSystems) != 1 {
		t.Fatalf("len(CurrentSystems) = %d, want 1", len(preview.CurrentSystems))
	}
	if len(preview.ProjectedSystems) != 2 {
		t.Fatalf("len(ProjectedSystems) = %d, want 2", len(preview.ProjectedSystems))
	}
	if preview.CurrentSystems[0].Source != "agent" {
		t.Fatalf("CurrentSystems[0].Source = %q, want agent", preview.CurrentSystems[0].Source)
	}
}

func TestPreviewMonitoredSystemRecordsReplacementOffsetsReplacedVMwareConnection(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "agent-host-1",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "esxi-01.lab.local",
				Status:   StatusOnline,
				LastSeen: now,
			},
			Identity: ResourceIdentity{
				DMIUUID:   "uuid-host-1",
				Hostnames: []string{"esxi-01.lab.local"},
			},
		},
	})
	rr.IngestRecords(SourceVMware, []IngestRecord{
		{
			SourceID: "vc-1:host:host-101",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "esxi-01.lab.local",
				Status:   StatusOnline,
				LastSeen: now,
				VMware: &VMwareData{
					ConnectionID:    "vc-1",
					ConnectionName:  "Lab VC",
					ManagedObjectID: "host-101",
					EntityType:      "host",
					HostUUID:        "uuid-host-1",
				},
			},
			Identity: ResourceIdentity{
				DMIUUID:   "uuid-host-1",
				Hostnames: []string{"esxi-01.lab.local"},
			},
		},
		{
			SourceID: "vc-1:host:host-102",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "esxi-02.lab.local",
				Status:   StatusOnline,
				LastSeen: now,
				VMware: &VMwareData{
					ConnectionID:    "vc-1",
					ConnectionName:  "Lab VC",
					ManagedObjectID: "host-102",
					EntityType:      "host",
					HostUUID:        "uuid-host-2",
				},
			},
			Identity: ResourceIdentity{
				DMIUUID:   "uuid-host-2",
				Hostnames: []string{"esxi-02.lab.local"},
			},
		},
	})

	preview := PreviewMonitoredSystemRecordsReplacement(rr, MonitoredSystemReplacement{
		Source: SourceVMware,
		Selector: MonitoredSystemReplacementSelector{
			ResourceID: "vc-1",
		},
	}, map[DataSource][]IngestRecord{
		SourceVMware: {
			{
				SourceID: "vc-1:host:host-101",
				Resource: Resource{
					Type:     ResourceTypeAgent,
					Name:     "esxi-01.lab.local",
					Status:   StatusOnline,
					LastSeen: now,
					VMware: &VMwareData{
						ConnectionID:    "vc-1",
						ConnectionName:  "Lab VC",
						ManagedObjectID: "host-101",
						EntityType:      "host",
						HostUUID:        "uuid-host-1",
					},
				},
				Identity: ResourceIdentity{
					DMIUUID:   "uuid-host-1",
					Hostnames: []string{"esxi-01.lab.local"},
				},
			},
			{
				SourceID: "vc-1:host:host-103",
				Resource: Resource{
					Type:     ResourceTypeAgent,
					Name:     "esxi-03.lab.local",
					Status:   StatusOnline,
					LastSeen: now,
					VMware: &VMwareData{
						ConnectionID:    "vc-1",
						ConnectionName:  "Lab VC",
						ManagedObjectID: "host-103",
						EntityType:      "host",
						HostUUID:        "uuid-host-3",
					},
				},
				Identity: ResourceIdentity{
					DMIUUID:   "uuid-host-3",
					Hostnames: []string{"esxi-03.lab.local"},
				},
			},
		},
	})

	if preview.CurrentCount != 2 {
		t.Fatalf("CurrentCount = %d, want 2", preview.CurrentCount)
	}
	if preview.ProjectedCount != 2 {
		t.Fatalf("ProjectedCount = %d, want 2", preview.ProjectedCount)
	}
	if preview.AdditionalCount != 0 {
		t.Fatalf("AdditionalCount = %d, want 0", preview.AdditionalCount)
	}
	if len(preview.CurrentSystems) != 2 {
		t.Fatalf("len(CurrentSystems) = %d, want 2", len(preview.CurrentSystems))
	}
	if len(preview.ProjectedSystems) != 2 {
		t.Fatalf("len(ProjectedSystems) = %d, want 2", len(preview.ProjectedSystems))
	}
	if preview.CurrentSystem != nil {
		t.Fatalf("CurrentSystem = %+v, want nil for multi-system replacement", preview.CurrentSystem)
	}
	if preview.ProjectedSystem != nil {
		t.Fatalf("ProjectedSystem = %+v, want nil for multi-system replacement", preview.ProjectedSystem)
	}
}

func TestPreviewMonitoredSystemRecordsReplacementScopesVMwareResourceIDToConnection(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceVMware, []IngestRecord{
		{
			SourceID: "vc-edit:host:host-101",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "esxi-01.lab.local",
				Status:   StatusOnline,
				LastSeen: now,
				VMware: &VMwareData{
					ConnectionID:    "vc-edit",
					ConnectionName:  "Edited VC",
					ManagedObjectID: "host-101",
					EntityType:      "host",
					HostUUID:        "uuid-host-1",
				},
			},
			Identity: ResourceIdentity{
				DMIUUID:   "uuid-host-1",
				Hostnames: []string{"esxi-01.lab.local"},
			},
		},
		{
			SourceID: "vc-other:host:host-201",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "esxi-collision.lab.local",
				Status:   StatusOnline,
				LastSeen: now,
				VMware: &VMwareData{
					ConnectionID:    "vc-other",
					ConnectionName:  "Other VC",
					ManagedObjectID: "host-201",
					EntityType:      "host",
					HostUUID:        "vc-edit",
				},
			},
			Identity: ResourceIdentity{
				DMIUUID:   "vc-edit",
				Hostnames: []string{"esxi-collision.lab.local"},
			},
		},
	})

	preview := PreviewMonitoredSystemRecordsReplacement(rr, MonitoredSystemReplacement{
		Source: SourceVMware,
		Selector: MonitoredSystemReplacementSelector{
			ResourceID: "vc-edit",
		},
	}, map[DataSource][]IngestRecord{
		SourceVMware: {
			{
				SourceID: "vc-edit:host:host-101",
				Resource: Resource{
					Type:     ResourceTypeAgent,
					Name:     "esxi-01.lab.local",
					Status:   StatusOnline,
					LastSeen: now,
					VMware: &VMwareData{
						ConnectionID:    "vc-edit",
						ConnectionName:  "Edited VC",
						ManagedObjectID: "host-101",
						EntityType:      "host",
						HostUUID:        "uuid-host-1",
					},
				},
				Identity: ResourceIdentity{
					DMIUUID:   "uuid-host-1",
					Hostnames: []string{"esxi-01.lab.local"},
				},
			},
		},
	})

	if preview.CurrentCount != 2 {
		t.Fatalf("CurrentCount = %d, want 2", preview.CurrentCount)
	}
	if preview.ProjectedCount != 2 {
		t.Fatalf("ProjectedCount = %d, want 2", preview.ProjectedCount)
	}
	if preview.AdditionalCount != 0 {
		t.Fatalf("AdditionalCount = %d, want 0", preview.AdditionalCount)
	}
	if len(preview.CurrentSystems) != 1 {
		t.Fatalf("len(CurrentSystems) = %d, want 1", len(preview.CurrentSystems))
	}
	if preview.CurrentSystems[0].Name != "esxi-01.lab.local" {
		t.Fatalf("CurrentSystems[0].Name = %q, want esxi-01.lab.local", preview.CurrentSystems[0].Name)
	}
	if len(preview.ProjectedSystems) != 1 {
		t.Fatalf("len(ProjectedSystems) = %d, want 1", len(preview.ProjectedSystems))
	}
	if preview.ProjectedSystems[0].Name != "esxi-01.lab.local" {
		t.Fatalf("ProjectedSystems[0].Name = %q, want esxi-01.lab.local", preview.ProjectedSystems[0].Name)
	}
}

func TestPreviewMonitoredSystemCandidateInactiveKeepsRegistryCountUnchanged(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "host-1",
			Resource: Resource{
				ID:     "host-1",
				Type:   ResourceTypeAgent,
				Name:   "tower.local",
				Status: StatusOnline,
				Agent: &AgentData{
					AgentID:   "agent-1",
					Hostname:  "tower.local",
					MachineID: "machine-1",
				},
				Identity: ResourceIdentity{
					MachineID: "machine-1",
					Hostnames: []string{"tower.local"},
				},
			},
		},
	})

	preview := PreviewMonitoredSystemCandidate(rr, MonitoredSystemCandidate{
		Source:   SourceTrueNAS,
		Type:     ResourceTypeAgent,
		Name:     "tower",
		Hostname: "tower.local",
		HostURL:  "https://tower.local",
		State:    MonitoredSystemCandidateStateInactive,
	})

	if preview.CurrentCount != 1 || preview.ProjectedCount != 1 || preview.AdditionalCount != 0 {
		t.Fatalf("unexpected inactive candidate counts: %+v", preview)
	}
	if len(preview.CurrentSystems) != 0 || len(preview.ProjectedSystems) != 0 {
		t.Fatalf("inactive candidate should not materialize preview systems: %+v", preview)
	}
}

func TestPreviewMonitoredSystemCandidateReplacementInactiveRemovesVMwareConnectionFromRegistry(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestRecords(SourceVMware, []IngestRecord{
		{
			SourceID: "vc-1:host:host-101",
			Resource: Resource{
				ID:     "vmware-host-101",
				Type:   ResourceTypeAgent,
				Name:   "esxi-01.lab.local",
				Status: StatusOnline,
				VMware: &VMwareData{
					ConnectionID:    "vc-1",
					ConnectionName:  "Lab VC",
					VCenterHost:     "vcsa.lab.local",
					ManagedObjectID: "host-101",
					EntityType:      "host",
					HostUUID:        "host-uuid-101",
				},
			},
			Identity: ResourceIdentity{
				DMIUUID:   "host-uuid-101",
				Hostnames: []string{"esxi-01.lab.local"},
			},
		},
	})

	preview := PreviewMonitoredSystemCandidateReplacement(rr, MonitoredSystemReplacement{
		Source: SourceVMware,
		Selector: MonitoredSystemReplacementSelector{
			ResourceID: "vc-1",
		},
	}, MonitoredSystemCandidate{
		Source:   SourceVMware,
		Type:     ResourceTypeAgent,
		Name:     "Lab VC",
		Hostname: "vcsa.lab.local",
		HostURL:  "https://vcsa.lab.local",
		State:    MonitoredSystemCandidateStateInactive,
	})

	if preview.CurrentCount != 1 || preview.ProjectedCount != 0 || preview.AdditionalCount != 0 {
		t.Fatalf("unexpected inactive replacement counts: %+v", preview)
	}
	if preview.CurrentSystem == nil {
		t.Fatal("expected current monitored system preview")
	}
	if preview.ProjectedSystem != nil {
		t.Fatalf("inactive replacement should not leave a projected system: %+v", preview.ProjectedSystem)
	}
}

func TestResourceRegistry_WorkloadsIncludeCanonicalAppContainers(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{
			SourceID: "vm-101",
			Resource: Resource{
				Type:     ResourceTypeVM,
				Name:     "db-01",
				Status:   StatusOnline,
				LastSeen: now,
			},
		},
	})
	rr.IngestRecords(SourceDocker, []IngestRecord{
		{
			SourceID: "ctr-201",
			Resource: Resource{
				Type:     ResourceTypeAppContainer,
				Name:     "metrics-sidecar",
				Status:   StatusOnline,
				LastSeen: now,
			},
		},
	})
	rr.IngestRecords(SourceTrueNAS, []IngestRecord{
		{
			SourceID: "app:nextcloud",
			Resource: Resource{
				Type:     ResourceTypeAppContainer,
				Name:     "Nextcloud",
				Status:   StatusOnline,
				LastSeen: now,
				TrueNAS:  &TrueNASData{Hostname: "truenas-main"},
				Docker: &DockerData{
					ContainerID:    "nextcloud",
					DisplayName:    "Nextcloud",
					Image:          "library/nextcloud:latest",
					Runtime:        "docker",
					ContainerState: "running",
				},
			},
		},
	})

	workloads := rr.Workloads()
	if len(workloads) != 3 {
		t.Fatalf("Workloads() returned %d records, want 3", len(workloads))
	}

	typesByName := make(map[string]ResourceType, len(workloads))
	for _, workload := range workloads {
		typesByName[workload.Name()] = workload.Type()
	}

	if got := typesByName["db-01"]; got != ResourceTypeVM {
		t.Fatalf("expected VM workload for db-01, got %q", got)
	}
	if got := typesByName["metrics-sidecar"]; got != ResourceTypeAppContainer {
		t.Fatalf("expected app-container workload for metrics-sidecar, got %q", got)
	}
	if got := typesByName["Nextcloud"]; got != ResourceTypeAppContainer {
		t.Fatalf("expected app-container workload for Nextcloud, got %q", got)
	}
}
