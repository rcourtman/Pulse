package unifiedresources

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

// TestMemoryStore_RecordActionAuditAppliesRedaction is an integration check
// at the registry-store boundary. The MemoryStore is the backing store the
// registry uses in tests and contract examples, and operator-authored audit
// fields must be scrubbed of secret shapes uniformly across both SQLite and
// in-memory persistence so test fixtures cannot accidentally exercise an
// unredacted path that production never sees.
func TestMemoryStore_RecordActionAuditAppliesRedaction(t *testing.T) {
	store := NewMemoryStore()
	now := time.Now().UTC()
	record := ActionAuditRecord{
		ID:        "act-redact",
		CreatedAt: now,
		UpdatedAt: now,
		State:     ActionStatePlanned,
		Request: ActionRequest{
			RequestID:      "req-redact",
			ResourceID:     "vm-100",
			CapabilityName: "pulse_control",
			Reason:         "rotate sk-leakedkey1234567 because it appeared in logs",
			RequestedBy:    "operator@example.com",
			Params: map[string]any{
				"command":    "curl -H 'Authorization: Bearer eyJleak' https://api.example.com",
				"targetType": "agent",
			},
		},
		Plan: ActionPlan{
			ActionID:  "act-redact",
			RequestID: "req-redact",
			Allowed:   true,
			PlanHash:  "deadbeef",
		},
	}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatalf("RecordActionAudit: %v", err)
	}

	stored, ok, err := store.GetActionAudit("act-redact")
	if err != nil {
		t.Fatalf("GetActionAudit: %v", err)
	}
	if !ok {
		t.Fatalf("expected stored audit, got none")
	}
	if strings.Contains(stored.Request.Reason, "sk-leakedkey1234567") {
		t.Fatalf("Reason still contains secret after persistence: %s", stored.Request.Reason)
	}
	if cmd, _ := stored.Request.Params["command"].(string); strings.Contains(cmd, "eyJleak") {
		t.Fatalf("Params[command] still contains Bearer token: %s", cmd)
	}
	// Plan fields must be untouched so PlanHash-based drift detection
	// continues to work.
	if stored.Plan.PlanHash != "deadbeef" {
		t.Fatalf("Plan.PlanHash mutated by redaction: %s", stored.Plan.PlanHash)
	}
}

func TestResourceRegistry_GetByReferenceResolvesSourceIDAndCanonicalAlias(t *testing.T) {
	rr := NewRegistry(nil)
	rr.IngestRecords(SourceProxmox, []IngestRecord{{
		SourceID: "delly:delly:101",
		Resource: Resource{
			Type: ResourceTypeSystemContainer,
			Name: "homeassistant",
		},
	}})
	rr.IngestResources([]Resource{{
		ID:   "agent-host-1",
		Type: ResourceTypeAgent,
		Name: "delly",
		Agent: &AgentData{
			AgentID:  "machine-1",
			Hostname: "delly.local",
		},
	}})

	resource, resolvedID, ok := rr.GetByReference("delly:delly:101")
	if !ok {
		t.Fatal("expected Proxmox source ID to resolve")
	}
	wantID := SourceSpecificID(ResourceTypeSystemContainer, SourceProxmox, "delly:delly:101")
	if resolvedID != wantID || resource.ID != wantID {
		t.Fatalf("source reference resolved to id=%q resource=%q, want %q", resolvedID, resource.ID, wantID)
	}

	resource, resolvedID, ok = rr.GetByReference("machine-1")
	if !ok {
		t.Fatal("expected canonical alias to resolve")
	}
	if resolvedID != "agent-host-1" || resource.ID != "agent-host-1" {
		t.Fatalf("alias reference resolved to id=%q resource=%q, want agent-host-1", resolvedID, resource.ID)
	}
}

func TestResourceRegistryAvailabilityLinkedResourceResolvesSourceReference(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Now().UTC()
	const serviceSourceID = "swarm-1:service:svc-api"
	rr.IngestRecords(SourceDocker, []IngestRecord{{
		SourceID: serviceSourceID,
		Resource: Resource{
			Type:     ResourceTypeDockerService,
			Name:     "api",
			Status:   StatusOnline,
			LastSeen: now,
		},
		Identity: ResourceIdentity{Hostnames: []string{"api"}},
	}})

	rr.IngestRecords(SourceAvailability, []IngestRecord{
		availabilityProbeRecord("probe-service", "api.example.test", &AvailabilityData{
			LinkedResourceID: serviceSourceID,
			Address:          "api.example.test",
			Protocol:         "tcp",
			Port:             8443,
			TargetKind:       "service",
			Enabled:          true,
			Available:        true,
		}),
	})

	if got := rr.ListByType(ResourceTypeNetworkEndpoint); len(got) != 0 {
		t.Fatalf("expected source-referenced probe to attach (0 endpoints), got %d", len(got))
	}
	services := rr.ListByType(ResourceTypeDockerService)
	if len(services) != 1 {
		t.Fatalf("expected 1 docker service, got %d", len(services))
	}
	service := services[0]
	if service.Availability == nil || service.Availability.TargetID != "probe-service" {
		t.Fatalf("expected availability facet probe-service on docker service, got %+v", service.Availability)
	}
	if service.Availability.LinkedResourceID != serviceSourceID {
		t.Fatalf("linkedResourceId = %q, want %q", service.Availability.LinkedResourceID, serviceSourceID)
	}
	if !hasDataSource(service.Sources, SourceAvailability) {
		t.Fatalf("expected docker service sources to include availability, got %v", service.Sources)
	}
}

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

func TestResourceRegistry_IngestRecordsPreservesAvailabilityEndpoints(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceAvailability, []IngestRecord{
		{
			SourceID: "energy-meter",
			Resource: Resource{
				Type:     ResourceTypeNetworkEndpoint,
				Name:     "Energy meter",
				Status:   StatusOffline,
				LastSeen: now,
				Sources:  []DataSource{SourceAvailability},
				Availability: &AvailabilityData{
					TargetID:            "energy-meter",
					Address:             "192.0.2.44",
					Protocol:            "icmp",
					Enabled:             true,
					Available:           false,
					ConsecutiveFailures: 2,
					FailureThreshold:    2,
				},
				Incidents: []ResourceIncident{{
					Provider: "availability",
					NativeID: "energy-meter",
					Code:     "availability_unreachable",
					Severity: storagehealth.RiskCritical,
					Source:   "availability",
					Summary:  "Energy meter is unreachable by ICMP probe",
				}},
			},
			Identity: ResourceIdentity{IPAddresses: []string{"192.0.2.44"}},
		},
	})

	got := rr.ListByType(ResourceTypeNetworkEndpoint)
	if len(got) != 1 {
		t.Fatalf("expected 1 network endpoint, got %d", len(got))
	}
	resource := got[0]
	if resource.Availability == nil {
		t.Fatal("expected availability metadata")
	}
	if resource.Availability.TargetID != "energy-meter" {
		t.Fatalf("target id = %q, want energy-meter", resource.Availability.TargetID)
	}
	if resource.Status != StatusOffline {
		t.Fatalf("status = %q, want %q", resource.Status, StatusOffline)
	}
	if len(resource.Incidents) != 1 || resource.Incidents[0].Code != "availability_unreachable" {
		t.Fatalf("expected availability incident, got %+v", resource.Incidents)
	}
	if resource.Canonical == nil || resource.Canonical.PrimaryID != "availability:energy-meter" {
		t.Fatalf("canonical identity = %+v, want primary availability:energy-meter", resource.Canonical)
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

func TestResourceRegistry_IngestResourcesSeedsMatcherForOverlayRecords(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)

	rr.IngestResources([]Resource{
		{
			ID:       "node-1",
			Type:     ResourceTypeAgent,
			Name:     "tower",
			Status:   StatusOnline,
			LastSeen: now,
			Sources:  []DataSource{SourceProxmox},
			Identity: ResourceIdentity{
				MachineID: "machine-1",
				Hostnames: []string{"tower"},
			},
			Proxmox: &ProxmoxData{
				NodeName: "tower",
				Instance: "lab",
			},
		},
	})

	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "host-1",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "tower",
				Status:   StatusOnline,
				LastSeen: now.Add(time.Minute),
				Agent: &AgentData{
					AgentID: "agent-1",
				},
			},
			Identity: ResourceIdentity{
				MachineID: "machine-1",
				Hostnames: []string{"tower"},
			},
		},
	})

	resources := rr.List()
	if len(resources) != 1 {
		t.Fatalf("expected overlay record to merge into seeded resource, got %d resources", len(resources))
	}
	if resources[0].Proxmox == nil || resources[0].Agent == nil {
		t.Fatalf("expected merged resource to keep seeded and overlay metadata, got %+v", resources[0])
	}
	if got := len(resources[0].Sources); got != 2 {
		t.Fatalf("expected merged sources from seed and overlay, got %d (%+v)", got, resources[0].Sources)
	}
}

func TestResourceRegistry_IngestResourcesRebuildsSourceMappingsForMetricsTargets(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 4, 21, 9, 0, 0, 0, time.UTC)

	rr.IngestResources([]Resource{
		{
			ID:       "agent-dashboard-1",
			Type:     ResourceTypeAgent,
			Name:     "delly",
			Status:   StatusOnline,
			LastSeen: now,
			Sources:  []DataSource{SourceProxmox, SourceAgent},
			Identity: ResourceIdentity{
				MachineID: "machine-delly",
				Hostnames: []string{"delly"},
			},
			Proxmox: &ProxmoxData{
				SourceID: "homelab-delly",
				NodeName: "delly",
				Instance: "homelab-entry",
			},
			Agent: &AgentData{
				AgentID:  "agent-source-delly",
				Hostname: "delly",
			},
		},
	})

	target := BuildMetricsTargetForRegistry(rr, "agent-dashboard-1")
	if target == nil {
		t.Fatal("expected seeded unified agent to rebuild a metrics target")
	}
	// The agent source ID wins over the proxmox node ID: it is the host.ID
	// key the agent metrics writer stores rows under.
	if target.ResourceType != "agent" || target.ResourceID != "agent-source-delly" {
		t.Fatalf("metrics target = %+v, want agent/agent-source-delly", target)
	}

	targets := rr.SourceTargets("agent-dashboard-1")
	if len(targets) != 2 {
		t.Fatalf("expected 2 source targets, got %d", len(targets))
	}

	foundAgent := false
	foundProxmox := false
	for _, sourceTarget := range targets {
		switch sourceTarget.Source {
		case SourceAgent:
			foundAgent = sourceTarget.SourceID == "agent-source-delly"
		case SourceProxmox:
			foundProxmox = sourceTarget.SourceID == "homelab-delly"
		}
	}
	if !foundAgent || !foundProxmox {
		t.Fatalf("expected rebuilt agent+proxmox source targets, got %+v", targets)
	}
}

func TestResourceRegistry_IngestResourcesSeedsHostScopedDockerContainerMappings(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 5, 19, 14, 0, 0, 0, time.UTC)
	host105 := "proxmox-lxc-docker:pve-a:node-a:105"
	host141 := "proxmox-lxc-docker:pve-a:node-a:141"

	rr.IngestResources([]Resource{
		{
			ID:       "docker-container-frigate-105",
			Type:     ResourceTypeAppContainer,
			Name:     "frigate",
			Status:   StatusOnline,
			LastSeen: now,
			Sources:  []DataSource{SourceDocker},
			Docker: &DockerData{
				HostSourceID:   host105,
				ContainerID:    "frigate",
				DisplayName:    "frigate",
				Runtime:        "docker",
				ContainerState: "running",
			},
		},
		{
			ID:       "docker-container-frigate-141",
			Type:     ResourceTypeAppContainer,
			Name:     "frigate",
			Status:   StatusOnline,
			LastSeen: now,
			Sources:  []DataSource{SourceDocker},
			Docker: &DockerData{
				HostSourceID:   host141,
				ContainerID:    "frigate",
				DisplayName:    "frigate",
				Runtime:        "docker",
				ContainerState: "running",
			},
		},
	})

	resources := rr.ListByType(ResourceTypeAppContainer)
	if len(resources) != 2 {
		t.Fatalf("expected seeded resources to preserve two host-scoped containers, got %d", len(resources))
	}

	assertSourceTarget := func(resourceID, wantSourceID string) {
		t.Helper()
		for _, target := range rr.SourceTargets(resourceID) {
			if target.Source == SourceDocker && target.SourceID == wantSourceID {
				wantCandidateID := SourceSpecificID(ResourceTypeAppContainer, SourceDocker, wantSourceID)
				if target.CandidateID != wantCandidateID {
					t.Fatalf("resource %q docker candidate ID = %q, want %q", resourceID, target.CandidateID, wantCandidateID)
				}
				return
			}
		}
		t.Fatalf("resource %q missing docker source target %q; got %+v", resourceID, wantSourceID, rr.SourceTargets(resourceID))
	}

	assertSourceTarget("docker-container-frigate-105", host105+"/container/frigate")
	assertSourceTarget("docker-container-frigate-141", host141+"/container/frigate")

	scopesByHost := make(map[string][]string, len(resources))
	for _, resource := range resources {
		if resource.Docker == nil {
			t.Fatalf("docker container resource missing Docker payload: %#v", resource)
		}
		scopesByHost[resource.Docker.HostSourceID] = resource.PlatformScopes
	}
	for _, host := range []string{host105, host141} {
		if got := scopesByHost[host]; !reflect.DeepEqual(got, []string{"proxmox-pve", "docker"}) {
			t.Fatalf("host %q platform scopes = %#v, want proxmox-pve + docker", host, got)
		}
	}
}

func TestResourceRegistry_IngestResourcesDerivesClusterWorkloadParentFromSeededProxmoxNode(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)

	rr.IngestResources([]Resource{
		{
			ID:       "agent-delly",
			Type:     ResourceTypeAgent,
			Name:     "delly",
			Status:   StatusOnline,
			LastSeen: now,
			Sources:  []DataSource{SourceProxmox, SourceAgent},
			Identity: ResourceIdentity{
				MachineID: "machine-delly",
				Hostnames: []string{"delly"},
			},
			Proxmox: &ProxmoxData{
				SourceID:    "homelab-delly",
				NodeName:    "delly",
				ClusterName: "homelab",
				Instance:    "delly",
			},
			Agent: &AgentData{
				AgentID:  "agent-source-delly",
				Hostname: "delly",
			},
		},
		{
			ID:       "system-container-cloudflared",
			Type:     ResourceTypeSystemContainer,
			Name:     "cloudflared",
			Status:   StatusOnline,
			LastSeen: now,
			Sources:  []DataSource{SourceProxmox},
			Identity: ResourceIdentity{Hostnames: []string{"cloudflared"}},
			Proxmox: &ProxmoxData{
				SourceID:    "delly:delly:104",
				NodeName:    "delly",
				ClusterName: "homelab",
				Instance:    "delly",
				VMID:        104,
			},
		},
	})

	child, ok := rr.Get("system-container-cloudflared")
	if !ok {
		t.Fatal("expected cloudflared resource")
	}
	if child.ParentID == nil || *child.ParentID != "agent-delly" {
		t.Fatalf("expected cloudflared parent agent-delly, got %+v", child.ParentID)
	}
	if child.ParentName != "delly" {
		t.Fatalf("expected cloudflared parent name delly, got %q", child.ParentName)
	}

	parent, ok := rr.Get("agent-delly")
	if !ok {
		t.Fatal("expected delly parent resource")
	}
	if parent.ChildCount != 1 {
		t.Fatalf("expected delly child count 1, got %d", parent.ChildCount)
	}
}

func TestStorageRiskSemanticsPrefersUnraidParitySummaryOverGenericDiskCounts(t *testing.T) {
	risk := &StorageRisk{
		Reasons: []StorageRiskReason{
			{Code: "unraid_disabled_disks", Severity: storagehealth.RiskCritical, Summary: "Unraid array reports 1 disabled disk(s)"},
			{Code: "unraid_parity_unavailable", Severity: storagehealth.RiskCritical, Summary: "Unraid parity protection is unavailable"},
		},
	}

	_, protectionReduced, _, protectionSummary, _ := StorageRiskSemantics(risk)
	if !protectionReduced {
		t.Fatal("expected protectionReduced=true")
	}
	if protectionSummary != "Unraid parity protection is unavailable" {
		t.Fatalf("protectionSummary = %q, want %q", protectionSummary, "Unraid parity protection is unavailable")
	}
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

func TestResourceRegistry_PreservesVMwareSourceOwnedDatastoreConsumers(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 3, 30, 18, 45, 0, 0, time.UTC)

	rr.IngestRecords(SourceVMware, []IngestRecord{
		{
			SourceID: "vc-1:datastore:datastore-201",
			Resource: Resource{
				Type:     ResourceTypeStorage,
				Name:     "nvme-primary",
				Status:   StatusOnline,
				LastSeen: now,
				Storage: &StorageMeta{
					Type:          "vmfs",
					Platform:      "vmware-vsphere",
					Topology:      "datastore",
					ConsumerCount: 2,
					ConsumerTypes: []string{string(ResourceTypeVM)},
					TopConsumers: []StorageConsumerMeta{
						{ResourceType: ResourceTypeVM, Name: "warehouse-api-01"},
						{ResourceType: ResourceTypeVM, Name: "etl-batch-01"},
					},
				},
				VMware: &VMwareData{
					ConnectionID:    "vc-1",
					ManagedObjectID: "datastore-201",
					EntityType:      "datastore",
				},
			},
		},
	})

	datastores := rr.ListByType(ResourceTypeStorage)
	if len(datastores) != 1 {
		t.Fatalf("expected 1 VMware datastore, got %d", len(datastores))
	}
	datastore := datastores[0]
	if datastore.Storage == nil {
		t.Fatalf("expected VMware datastore storage metadata, got %+v", datastore)
	}
	if got := datastore.Storage.ConsumerCount; got != 2 {
		t.Fatalf("consumer count = %d, want 2", got)
	}
	if got := datastore.Storage.ConsumerTypes; len(got) != 1 || got[0] != string(ResourceTypeVM) {
		t.Fatalf("consumer types = %#v, want [vm]", got)
	}
	if len(datastore.Storage.TopConsumers) != 2 {
		t.Fatalf("top consumers = %#v, want 2 VMware VM consumers", datastore.Storage.TopConsumers)
	}
	if datastore.Storage.TopConsumers[0].Name != "warehouse-api-01" {
		t.Fatalf("first top consumer = %#v, want warehouse-api-01", datastore.Storage.TopConsumers[0])
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
		NetworkHostNames:   []string{"esxi-01.lab.local"},
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
	clusterHA := true
	clusterDRS := false
	bootRetry := true
	incoming := &VMwareData{
		ClusterName:         "Cluster A",
		ClusterHAEnabled:    &clusterHA,
		ClusterDRSEnabled:   &clusterDRS,
		RuntimeHostName:     "esxi-01.lab.local",
		DatastoreNames:      []string{"backup-nfs"},
		NetworkType:         "STANDARD_PORTGROUP",
		NetworkHostNames:    []string{"esxi-02.lab.local"},
		NetworkVMNames:      []string{"app-01"},
		DatastoreAccessible: &accessible,
		GuestIPAddresses:    []string{"10.0.0.21"},
		OverallStatus:       "yellow",
		ActiveAlarmCount:    3,
		ActiveAlarmSummary:  "Host connection lost; datastore latency elevated",
		RecentTaskCount:     2,
		RecentTaskSummary:   "vMotion task completed",
		SnapshotCount:       4,
		CurrentSnapshotID:   "snapshot-202",
		SnapshotTree:        []VMwareSnapshotData{{Snapshot: "snapshot-202", Name: "pre-maintenance"}},
		NetworkAdapters:     []VMwareNetworkAdapterData{{NIC: "4000", NetworkName: "VM Network"}},
		VirtualDisks:        []VMwareVirtualDiskData{{Disk: "2000", Label: "Hard disk 1"}},
		Tools:               &VMwareToolsData{RunState: "RUNNING", VersionStatus: "CURRENT"},
		Hardware: &VMwareVMHardwareData{
			Version:   "VMX_20",
			BootRetry: &bootRetry,
			BootDevices: []VMwareBootDeviceData{{
				Type:  "DISK",
				Disks: []string{"2000"},
			}},
		},
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
	if merged.ClusterHAEnabled == nil || !*merged.ClusterHAEnabled {
		t.Fatalf("cluster HA enabled = %#v, want true", merged.ClusterHAEnabled)
	}
	if merged.ClusterDRSEnabled == nil || *merged.ClusterDRSEnabled {
		t.Fatalf("cluster DRS enabled = %#v, want false", merged.ClusterDRSEnabled)
	}
	if got := merged.DatastoreNames; !reflect.DeepEqual(got, []string{"primary-vmfs", "backup-nfs"}) {
		t.Fatalf("datastore names = %#v", got)
	}
	if got := merged.NetworkType; got != "STANDARD_PORTGROUP" {
		t.Fatalf("network type = %q, want STANDARD_PORTGROUP", got)
	}
	if got := merged.NetworkHostNames; !reflect.DeepEqual(got, []string{"esxi-01.lab.local", "esxi-02.lab.local"}) {
		t.Fatalf("network host names = %#v", got)
	}
	if got := merged.NetworkVMNames; !reflect.DeepEqual(got, []string{"app-01"}) {
		t.Fatalf("network VM names = %#v", got)
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
	if got := merged.CurrentSnapshotID; got != "snapshot-202" {
		t.Fatalf("current snapshot id = %q, want snapshot-202", got)
	}
	if got := merged.SnapshotTree; len(got) != 1 || got[0].Name != "pre-maintenance" {
		t.Fatalf("snapshot tree = %#v", got)
	}
	if got := merged.NetworkAdapters; len(got) != 1 || got[0].NetworkName != "VM Network" {
		t.Fatalf("network adapters = %#v", got)
	}
	if got := merged.VirtualDisks; len(got) != 1 || got[0].Label != "Hard disk 1" {
		t.Fatalf("virtual disks = %#v", got)
	}
	if got := merged.Tools; got == nil || got.RunState != "RUNNING" {
		t.Fatalf("tools = %#v", got)
	}
	if got := merged.Hardware; got == nil || got.Version != "VMX_20" || got.BootDevices[0].Disks[0] != "2000" {
		t.Fatalf("hardware = %#v", got)
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

func TestMonitoredSystemsMergeShortAndFQDNHostnamesForOneHost(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 4, 21, 21, 30, 0, 0, time.UTC)

	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "agent-qnap",
			Resource: Resource{
				ID:       "agent-qnap",
				Type:     ResourceTypeAgent,
				Name:     "qnap.local",
				Status:   StatusOnline,
				LastSeen: now.Add(-2 * time.Minute),
				Agent: &AgentData{
					AgentID:  "agent-qnap",
					Hostname: "qnap.local",
				},
			},
			Identity: ResourceIdentity{
				Hostnames: []string{"qnap.local"},
			},
		},
	})
	rr.IngestRecords(SourceDocker, []IngestRecord{
		{
			SourceID: "docker-qnap",
			Resource: Resource{
				ID:       "docker-qnap",
				Type:     ResourceTypeAgent,
				Name:     "qnap",
				Status:   StatusOnline,
				LastSeen: now,
				Docker: &DockerData{
					HostSourceID: "docker-qnap",
					Hostname:     "qnap",
				},
			},
			Identity: ResourceIdentity{
				Hostnames: []string{"qnap"},
			},
		},
	})

	systems := MonitoredSystems(rr)
	if len(systems) != 1 {
		t.Fatalf("MonitoredSystems() returned %d systems, want 1", len(systems))
	}
	if systems[0].Source != "multiple" {
		t.Fatalf("MonitoredSystems()[0].Source = %q, want %q", systems[0].Source, "multiple")
	}
	if systems[0].Explanation.Summary == "" {
		t.Fatal("expected grouped monitored-system explanation summary")
	}
	if !systems[0].LastSeen.Equal(now) {
		t.Fatalf("MonitoredSystems()[0].LastSeen = %s, want %s", systems[0].LastSeen, now)
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

func TestResourceRegistry_IngestSnapshotParentsClusterNamedProxmoxGuestsToMergedNode(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:              "homelab-delly",
				Name:            "delly",
				Instance:        "delly",
				ClusterName:     "homelab",
				IsClusterMember: true,
				Host:            "https://10.0.0.9:8006",
				LinkedAgentID:   "host-1",
				Status:          "online",
				LastSeen:        now,
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
		VMs: []models.VM{
			{
				ID:       "delly:delly:100",
				VMID:     100,
				Name:     "docker-vm",
				Node:     "delly",
				Instance: "delly",
				Status:   "running",
				LastSeen: now,
			},
		},
		Containers: []models.Container{
			{
				ID:       "delly:delly:104",
				VMID:     104,
				Name:     "cloudflared",
				Node:     "delly",
				Instance: "delly",
				Status:   "running",
				LastSeen: now,
			},
		},
	})

	agents := rr.ListByType(ResourceTypeAgent)
	if len(agents) != 1 {
		t.Fatalf("expected 1 merged delly resource, got %d", len(agents))
	}
	parentID := agents[0].ID

	vms := rr.ListByType(ResourceTypeVM)
	if len(vms) != 1 {
		t.Fatalf("expected 1 vm, got %d", len(vms))
	}
	if vms[0].ParentID == nil || *vms[0].ParentID != parentID {
		t.Fatalf("expected vm parent %q, got %+v", parentID, vms[0].ParentID)
	}
	if vms[0].ParentName == "" {
		t.Fatal("expected vm parent name to be derived")
	}
	if vms[0].Proxmox == nil || vms[0].Proxmox.LinkedAgentID != "host-1" {
		t.Fatalf("expected vm linked agent host-1, got %+v", vms[0].Proxmox)
	}

	containers := rr.ListByType(ResourceTypeSystemContainer)
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if containers[0].ParentID == nil || *containers[0].ParentID != parentID {
		t.Fatalf("expected container parent %q, got %+v", parentID, containers[0].ParentID)
	}
	if containers[0].Proxmox == nil || containers[0].Proxmox.LinkedAgentID != "host-1" {
		t.Fatalf("expected container linked agent host-1, got %+v", containers[0].Proxmox)
	}

	parent, ok := rr.Get(parentID)
	if !ok {
		t.Fatalf("expected parent %q", parentID)
	}
	if parent.ChildCount != 2 {
		t.Fatalf("expected parent child count 2, got %d", parent.ChildCount)
	}
}

func TestResourceRegistry_MergedNodeAgentMetricsTargetMatchesAgentStoreWriteKey(t *testing.T) {
	// Real-mode agent metrics land in the metrics store keyed by host.ID
	// (monitor_agents.go: m.metricsStore.Write("agent", host.ID, ...), the
	// write key is pinned by a canonical guardrail in internal/monitoring).
	// The registry's metrics target for the merged node+agent resource must
	// resolve to that same key, or every store reader that trusts the
	// target — performance reports via MetricsResourceID, the drawer
	// history endpoint — queries an ID with zero rows.
	rr := NewRegistry(nil)
	now := time.Date(2026, 6, 10, 10, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:              "homelab-delly",
				Name:            "delly",
				Instance:        "delly",
				ClusterName:     "homelab",
				IsClusterMember: true,
				Host:            "https://10.0.0.9:8006",
				LinkedAgentID:   "host-1",
				Status:          "online",
				LastSeen:        now,
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
		t.Fatalf("expected 1 merged delly resource, got %d", len(agents))
	}
	resource := agents[0]

	targets := rr.SourceTargets(resource.ID)
	if len(targets) != 2 {
		t.Fatalf("expected merged proxmox+agent source targets, got %+v", targets)
	}

	target := BuildMetricsTargetForRegistry(rr, resource.ID)
	if target == nil {
		t.Fatal("BuildMetricsTargetForRegistry() returned nil")
	}
	if target.ResourceType != "agent" {
		t.Fatalf("ResourceType = %q, want agent", target.ResourceType)
	}
	if target.ResourceID != "host-1" {
		t.Fatalf("ResourceID = %q, want host-1 (the host.ID key the agent metrics writer uses)", target.ResourceID)
	}
}

func TestResourceRegistry_ManualNodeMergeRewritesProxmoxGuestParentAndActionAgent(t *testing.T) {
	store := NewMemoryStore()
	if err := store.AddLink(ResourceLink{
		ResourceA: "agent-old-proxmox-node",
		ResourceB: "agent-current",
		PrimaryID: "agent-current",
	}); err != nil {
		t.Fatalf("add link: %v", err)
	}
	rr := NewRegistry(store)
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	staleParentID := "agent-old-proxmox-node"

	rr.IngestResources([]Resource{
		{
			ID:       staleParentID,
			Type:     ResourceTypeAgent,
			Name:     "delly",
			Status:   "online",
			LastSeen: now,
			Sources:  []DataSource{SourceProxmox},
			SourceStatus: map[DataSource]SourceStatus{
				SourceProxmox: {Status: "online", LastSeen: now},
			},
			Proxmox: &ProxmoxData{
				SourceID:    "homelab-delly",
				NodeName:    "delly",
				ClusterName: "homelab",
				Instance:    "delly",
			},
		},
		{
			ID:       "agent-current",
			Type:     ResourceTypeAgent,
			Name:     "delly",
			Status:   "online",
			LastSeen: now,
			Sources:  []DataSource{SourceAgent},
			SourceStatus: map[DataSource]SourceStatus{
				SourceAgent: {Status: "online", LastSeen: now},
			},
			Agent: &AgentData{
				AgentID:  "agent-delly",
				Hostname: "delly",
			},
		},
		{
			ID:       "system-container-grafana",
			Type:     ResourceTypeSystemContainer,
			Name:     "grafana",
			Status:   "online",
			LastSeen: now,
			ParentID: &staleParentID,
			Sources:  []DataSource{SourceProxmox},
			SourceStatus: map[DataSource]SourceStatus{
				SourceProxmox: {Status: "online", LastSeen: now},
			},
			Proxmox: &ProxmoxData{
				SourceID:    "delly:delly:124",
				NodeName:    "delly",
				ClusterName: "homelab",
				Instance:    "delly",
				VMID:        124,
			},
		},
	})

	containers := rr.ListByType(ResourceTypeSystemContainer)
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if containers[0].ParentID == nil || *containers[0].ParentID != "agent-current" {
		t.Fatalf("expected container parent agent-current, got %+v", containers[0].ParentID)
	}
	if containers[0].ParentName != "delly" {
		t.Fatalf("expected container parent name delly, got %q", containers[0].ParentName)
	}
	if containers[0].Proxmox == nil || containers[0].Proxmox.LinkedAgentID != "agent-delly" {
		t.Fatalf("expected container linked agent agent-delly, got %+v", containers[0].Proxmox)
	}
}

func TestResourceRegistry_ProxmoxGuestParentPrefersAgentBackedNodeDuplicate(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)

	rr.IngestResources([]Resource{
		{
			ID:       "agent-current",
			Type:     ResourceTypeAgent,
			Name:     "delly",
			Status:   "online",
			LastSeen: now,
			Sources:  []DataSource{SourceAgent, SourceProxmox},
			SourceStatus: map[DataSource]SourceStatus{
				SourceAgent:   {Status: "online", LastSeen: now},
				SourceProxmox: {Status: "online", LastSeen: now},
			},
			Agent: &AgentData{
				AgentID:  "agent-delly",
				Hostname: "delly",
			},
			Proxmox: &ProxmoxData{
				SourceID:    "homelab-delly",
				NodeName:    "delly",
				ClusterName: "homelab",
				Instance:    "delly",
			},
		},
		{
			ID:       "agent-old-proxmox-node",
			Type:     ResourceTypeAgent,
			Name:     "delly",
			Status:   "online",
			LastSeen: now,
			Sources:  []DataSource{SourceProxmox},
			SourceStatus: map[DataSource]SourceStatus{
				SourceProxmox: {Status: "online", LastSeen: now},
			},
			Proxmox: &ProxmoxData{
				SourceID:    "homelab-delly",
				NodeName:    "delly",
				ClusterName: "homelab",
				Instance:    "delly",
			},
		},
	})
	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{
			SourceID:       "delly:delly:124",
			ParentSourceID: "homelab-delly",
			Resource: Resource{
				Type:     ResourceTypeSystemContainer,
				Name:     "grafana",
				Status:   "online",
				LastSeen: now,
				Proxmox: &ProxmoxData{
					SourceID:    "delly:delly:124",
					NodeName:    "delly",
					ClusterName: "homelab",
					Instance:    "delly",
					VMID:        124,
				},
			},
			Identity: ResourceIdentity{
				Hostnames: []string{"grafana"},
			},
		},
	})

	containers := rr.ListByType(ResourceTypeSystemContainer)
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if containers[0].ParentID == nil || *containers[0].ParentID != "agent-current" {
		t.Fatalf("expected container parent agent-current, got %+v", containers[0].ParentID)
	}
	if containers[0].Proxmox == nil || containers[0].Proxmox.LinkedAgentID != "agent-delly" {
		t.Fatalf("expected container linked agent agent-delly, got %+v", containers[0].Proxmox)
	}
}

func TestResourceRegistry_IngestSnapshotDerivesProxmoxWorkloadParentWhenNodeSourceIDUsesPreviousClusterAlias(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:              "mock-cluster-pve1",
				Name:            "pve1",
				DisplayName:     "West Production A",
				Instance:        "Core Fabric",
				ClusterName:     "Core Fabric",
				IsClusterMember: true,
				Status:          "online",
				LastSeen:        now,
			},
		},
		VMs: []models.VM{
			{
				ID:       "Core Fabric:pve1:100",
				VMID:     100,
				Name:     "checkout-web-01",
				Node:     "pve1",
				Instance: "Core Fabric",
				Status:   "running",
				LastSeen: now,
			},
		},
		Containers: []models.Container{
			{
				ID:       "Core Fabric:pve1:104",
				VMID:     104,
				Name:     "auth-service-01",
				Node:     "pve1",
				Instance: "Core Fabric",
				Status:   "running",
				LastSeen: now,
			},
		},
	})

	agents := rr.ListByType(ResourceTypeAgent)
	if len(agents) != 1 {
		t.Fatalf("expected 1 Proxmox parent resource, got %d", len(agents))
	}
	parentID := agents[0].ID

	vms := rr.ListByType(ResourceTypeVM)
	if len(vms) != 1 {
		t.Fatalf("expected 1 vm, got %d", len(vms))
	}
	if vms[0].ParentID == nil || *vms[0].ParentID != parentID {
		t.Fatalf("expected vm parent %q from node metadata fallback, got %+v", parentID, vms[0].ParentID)
	}
	if vms[0].ParentName != "West Production A" {
		t.Fatalf("expected vm parent name West Production A, got %q", vms[0].ParentName)
	}

	containers := rr.ListByType(ResourceTypeSystemContainer)
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if containers[0].ParentID == nil || *containers[0].ParentID != parentID {
		t.Fatalf("expected container parent %q from node metadata fallback, got %+v", parentID, containers[0].ParentID)
	}

	parent, ok := rr.Get(parentID)
	if !ok {
		t.Fatalf("expected parent %q", parentID)
	}
	if parent.ChildCount != 2 {
		t.Fatalf("expected parent child count 2, got %d", parent.ChildCount)
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

func TestResourceRegistry_IngestSnapshotSkipsVirtualBlockDevicesFromHostSMART(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)

	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:       "host-minipc",
				Hostname: "minipc",
				Status:   "online",
				LastSeen: now,
				Sensors: models.HostSensorSummary{
					SMART: []models.HostDiskSMART{
						{Device: "/dev/nvme0n1", Model: "Samsung 970 EVO", Serial: "REAL-DISK", Type: "nvme", Health: "PASSED"},
						{Device: "/dev/zd0", Model: "", Serial: "", Type: ""},
						{Device: "/dev/zd16", Model: "", Serial: "", Type: ""},
						{Device: "zram0", Model: "", Serial: "", Type: ""},
						{Device: "/dev/loop3", Model: "", Serial: "", Type: ""},
						{Device: "/dev/dm-1", Model: "", Serial: "", Type: ""},
					},
				},
			},
		},
	})

	disks := rr.ListByType(ResourceTypePhysicalDisk)
	if len(disks) != 1 {
		t.Fatalf("expected 1 physical disk resource (virtual devices filtered), got %d", len(disks))
	}
	if disks[0].PhysicalDisk == nil || disks[0].PhysicalDisk.Serial != "REAL-DISK" {
		t.Fatalf("expected only the real nvme disk, got %+v", disks[0].PhysicalDisk)
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
	if disk.Proxmox == nil || disk.Proxmox.NodeName != "tower" || disk.Proxmox.Instance != "pve-tower" {
		t.Fatalf("expected merged disk to retain Proxmox node identity, got %+v", disk.Proxmox)
	}
}

// Regression for issue #1483: a legacy host agent reports an NVMe disk keyed by
// its controller ("nvme0 [nvme]") with no size, while the Proxmox disks/list poll
// reports the canonical namespace ("/dev/nvme0n1") with the real capacity. The
// merge must keep the authoritative Proxmox devPath and size, and only enrich
// with the agent's SMART temperature.
func TestResourceRegistry_AgentDiskDoesNotClobberProxmoxDevPath(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	const nvmeSize = int64(2000398934016)

	rr.IngestSnapshot(models.StateSnapshot{
		PhysicalDisks: []models.PhysicalDisk{
			{
				ID:          "pve-disk-1",
				Node:        "pve",
				Instance:    "pve",
				DevPath:     "/dev/nvme0n1",
				Model:       "KINGSTON SNV3S2000G",
				Serial:      "SERIAL-NVME-0",
				Type:        "nvme",
				Size:        nvmeSize,
				Health:      "PASSED",
				LastChecked: now,
			},
		},
		Hosts: []models.Host{
			{
				ID:       "host-pve",
				Hostname: "pve",
				Status:   "online",
				LastSeen: now,
				Sensors: models.HostSensorSummary{
					SMART: []models.HostDiskSMART{
						{
							Device:      "nvme0 [nvme]", // legacy controller label, no size
							Model:       "KINGSTON SNV3S2000G",
							Serial:      "SERIAL-NVME-0",
							Type:        "nvme",
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
	if disk.PhysicalDisk == nil {
		t.Fatal("expected physical disk metadata")
	}
	if disk.PhysicalDisk.DevPath != "/dev/nvme0n1" {
		t.Fatalf("devPath = %q, want canonical /dev/nvme0n1 (agent label must not clobber)", disk.PhysicalDisk.DevPath)
	}
	if disk.PhysicalDisk.SizeBytes != nvmeSize {
		t.Fatalf("sizeBytes = %d, want authoritative Proxmox capacity %d", disk.PhysicalDisk.SizeBytes, nvmeSize)
	}
	if disk.PhysicalDisk.Temperature != 37 {
		t.Fatalf("temperature = %d, want enriched agent SMART value 37", disk.PhysicalDisk.Temperature)
	}
}

func TestShouldReplacePhysicalDiskDevPath(t *testing.T) {
	cases := []struct {
		name     string
		existing string
		incoming string
		want     bool
	}{
		{"canonical not downgraded by scan label", "/dev/nvme0n1", "nvme0 [nvme]", false},
		{"canonical not downgraded by bare token", "/dev/nvme0n1", "nvme0n1", false},
		{"scan label upgraded to canonical", "nvme0 [nvme]", "/dev/nvme0n1", true},
		{"empty existing always replaced", "", "nvme0 [nvme]", true},
		{"empty incoming never replaces", "/dev/nvme0n1", "", false},
		{"both canonical last writer wins", "/dev/sda", "/dev/sdb", true},
	}
	for _, tc := range cases {
		if got := shouldReplacePhysicalDiskDevPath(tc.existing, tc.incoming); got != tc.want {
			t.Errorf("%s: shouldReplacePhysicalDiskDevPath(%q, %q) = %v, want %v", tc.name, tc.existing, tc.incoming, got, tc.want)
		}
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

func TestResourceRegistry_IngestSnapshotCreatesUnraidDisksWithoutSMART(t *testing.T) {
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
					{Mountpoint: "/mnt/user", Total: 10_000, Used: 7_000, Free: 3_000, Usage: 70},
				},
				Unraid: &models.HostUnraidStorage{
					ArrayStarted: true,
					ArrayState:   "STARTED",
					Disks: []models.HostUnraidDisk{
						{
							Name:        "disk1",
							Device:      "/dev/sdb",
							Role:        "data",
							Status:      "online",
							Model:       "WDC WD60EFRX",
							Serial:      "DATA-1",
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
						},
						{
							Name:       "cachepool",
							Device:     "/dev/sdc",
							Role:       "cache",
							Status:     "online",
							Model:      "SSD 2000GB",
							Serial:     "CACHE-1",
							Filesystem: "btrfs",
							Transport:  "sata",
							SizeBytes:  2_000_000_000_000,
							UsedBytes:  200,
							FreeBytes:  800,
						},
					},
				},
			},
		},
	})

	disks := rr.ListByType(ResourceTypePhysicalDisk)
	if len(disks) != 2 {
		t.Fatalf("expected 2 Unraid physical disks, got %d: %+v", len(disks), disks)
	}
	bySerial := map[string]Resource{}
	for _, disk := range disks {
		if disk.PhysicalDisk != nil {
			bySerial[disk.PhysicalDisk.Serial] = disk
		}
	}
	data := bySerial["DATA-1"]
	if data.PhysicalDisk == nil || data.PhysicalDisk.StorageRole != "data" || data.PhysicalDisk.StorageGroup != "unraid-array" {
		t.Fatalf("expected data disk to belong to unraid array, got %+v", data.PhysicalDisk)
	}
	if data.PhysicalDisk.SizeBytes != 6_000_000_000_000 || data.PhysicalDisk.Temperature != 31 {
		t.Fatalf("expected Unraid size/temp on data disk, got %+v", data.PhysicalDisk)
	}
	if !data.PhysicalDisk.SpunDown || data.PhysicalDisk.ReadCount != 11 || data.PhysicalDisk.WriteCount != 12 || data.PhysicalDisk.ErrorCount != 16 {
		t.Fatalf("expected Unraid disk counters on data disk, got %+v", data.PhysicalDisk)
	}
	cache := bySerial["CACHE-1"]
	if cache.PhysicalDisk == nil || cache.PhysicalDisk.StorageRole != "cache" || cache.PhysicalDisk.StorageGroup != "cachepool" {
		t.Fatalf("expected cache disk to belong to cachepool, got %+v", cache.PhysicalDisk)
	}

	storage := rr.ListByType(ResourceTypeStorage)
	var sawArray, sawCache bool
	for _, resource := range storage {
		if resource.Storage == nil {
			continue
		}
		switch resource.Storage.Type {
		case "unraid-array":
			sawArray = true
		case "unraid-cache-pool":
			sawCache = true
			if resource.Name != "cachepool" || resource.Metrics == nil || resource.Metrics.Disk == nil || resource.Metrics.Disk.Percent != 20 {
				t.Fatalf("unexpected cache storage resource: %+v", resource)
			}
		}
	}
	if !sawArray || !sawCache {
		t.Fatalf("expected array and cache storage resources, got %+v", storage)
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

func TestResourceRegistry_IngestSnapshotParentsInferredUnraidArrayDisksUnderStorage(t *testing.T) {
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
					Disks: []models.HostUnraidDisk{
						{Name: "md1p1", Device: "/dev/sde", Status: "online", Slot: 1},
					},
				},
				Sensors: models.HostSensorSummary{
					SMART: []models.HostDiskSMART{
						{
							Device: "sde [sat]",
							Model:  "Array Disk",
							Type:   "sata",
							Health: "UNKNOWN",
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
		t.Fatalf("expected inferred unraid disk parent to be storage resource %q, got %+v", storage[0].ID, disks[0].ParentID)
	}
	if disks[0].PhysicalDisk == nil || disks[0].PhysicalDisk.StorageRole != "data" || disks[0].PhysicalDisk.StorageGroup != "unraid-array" {
		t.Fatalf("expected inferred unraid data disk metadata, got %+v", disks[0].PhysicalDisk)
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

func TestResourceRegistry_IngestSnapshotProjectsCephPoolsAsStorage(t *testing.T) {
	now := time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC)
	rr := NewRegistry(nil)

	rr.IngestSnapshot(models.StateSnapshot{
		CephClusters: []models.CephCluster{
			{
				ID:          "Main-fsid123",
				Instance:    "Main",
				Name:        "Main Ceph",
				FSID:        "fsid123",
				Health:      "HEALTH_OK",
				LastUpdated: now,
				Pools: []models.CephPool{
					{
						ID:             1,
						Name:           "data_replication",
						StoredBytes:    70,
						AvailableBytes: 30,
						PercentUsed:    70,
					},
				},
			},
		},
	})

	storageResources := rr.ListByType(ResourceTypeStorage)
	pool := findStorageResource(storageResources, "data_replication", "cluster")
	if pool.Storage == nil {
		t.Fatalf("expected Ceph pool storage metadata, got %+v", pool)
	}
	if !pool.Storage.IsCeph || !pool.Storage.Shared || pool.Storage.Type != "ceph" {
		t.Fatalf("unexpected Ceph pool storage metadata: %+v", pool.Storage)
	}
	if pool.Metrics == nil || pool.Metrics.Disk == nil || pool.Metrics.Disk.Percent != 70 {
		t.Fatalf("expected Ceph pool disk metrics, got %+v", pool.Metrics)
	}

	target := BuildMetricsTargetForRegistry(rr, pool.ID)
	if target == nil {
		t.Fatalf("expected metrics target for Ceph pool storage")
	}
	wantID := models.CephPoolStorageID("Main", "data_replication")
	if target.ResourceType != "storage" || target.ResourceID != wantID {
		t.Fatalf("metrics target = %+v, want storage/%s", target, wantID)
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

func TestResourceRegistry_MergeTrueNASPayloadOnIdentityMatch(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)

	rr.IngestRecords(SourceAgent, []IngestRecord{
		{
			SourceID: "agent-archive",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "archive",
				Status:   StatusOnline,
				LastSeen: now,
				Agent: &AgentData{
					Hostname: "archive.local",
					OSName:   "Debian GNU/Linux",
				},
			},
			Identity: ResourceIdentity{
				MachineID: "archive-machine",
				Hostnames: []string{"archive.local"},
			},
		},
	})

	rr.IngestRecords(SourceTrueNAS, []IngestRecord{
		{
			SourceID: "truenas-archive",
			Resource: Resource{
				Type:     ResourceTypeAgent,
				Name:     "archive.local",
				Status:   StatusWarning,
				LastSeen: now.Add(time.Minute),
				TrueNAS: &TrueNASData{
					Hostname:              "archive.local",
					Version:               "24.10",
					UptimeSeconds:         86400,
					StorageRisk:           &StorageRisk{Level: storagehealth.RiskWarning},
					StorageRiskSummary:    "Pool tank is DEGRADED",
					StoragePostureSummary: "Pool tank is DEGRADED",
					ProtectionReduced:     true,
					ProtectionSummary:     "Pool redundancy is reduced",
				},
				Agent: &AgentData{
					Hostname: "archive.local",
					OSName:   "TrueNAS SCALE",
				},
			},
			Identity: ResourceIdentity{
				MachineID: "archive-machine",
				Hostnames: []string{"archive.local"},
			},
		},
	})

	resources := rr.ListByType(ResourceTypeAgent)
	if len(resources) != 1 {
		t.Fatalf("ListByType(ResourceTypeAgent) returned %d resources, want 1", len(resources))
	}
	resource := resources[0]
	if !hasDataSource(resource.Sources, SourceAgent) || !hasDataSource(resource.Sources, SourceTrueNAS) {
		t.Fatalf("sources = %#v, want agent and truenas", resource.Sources)
	}
	if resource.TrueNAS == nil {
		t.Fatal("expected TrueNAS payload to merge onto matched agent resource")
	}
	if resource.TrueNAS.Version != "24.10" || resource.TrueNAS.UptimeSeconds != 86400 {
		t.Fatalf("TrueNAS payload not refreshed: %+v", resource.TrueNAS)
	}
	if resource.TrueNAS.StorageRisk == nil || resource.TrueNAS.StorageRisk.Level != storagehealth.RiskWarning {
		t.Fatalf("TrueNAS storage risk not merged: %+v", resource.TrueNAS.StorageRisk)
	}
	if resource.Agent == nil || resource.Agent.OSName != "Debian GNU/Linux" {
		t.Fatalf("SourceAgent facet should remain authoritative, got %+v", resource.Agent)
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

func TestRegistryIngestSnapshotMergesLinkedKubernetesNodeIntoAgentResource(t *testing.T) {
	rr := NewRegistry(nil)

	rr.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:        "host-1",
				Hostname:  "prod-euw1-k8s-01",
				MachineID: "machine-k8s-1",
				Status:    "online",
			},
		},
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:      "cluster-1",
				Name:    "production",
				AgentID: "agent-cluster-1",
				Nodes: []models.KubernetesNode{
					{
						UID:   "node-1",
						Name:  "prod-euw1-k8s-01",
						Ready: true,
					},
				},
			},
		},
	})

	resources := rr.List()
	if got := len(resources); got != 2 {
		t.Fatalf("expected cluster + merged agent resource, got %d resources", got)
	}

	var cluster *Resource
	var host *Resource
	for i := range resources {
		switch resources[i].Type {
		case ResourceTypeK8sCluster:
			cluster = &resources[i]
		case ResourceTypeAgent:
			host = &resources[i]
		case ResourceTypeK8sNode:
			t.Fatalf("expected linked kubernetes node to merge into backing agent, got standalone node %+v", resources[i])
		}
	}

	if cluster == nil {
		t.Fatal("expected kubernetes cluster resource")
	}
	if host == nil {
		t.Fatal("expected merged agent resource")
	}
	if !hasDataSource(host.Sources, SourceAgent) || !hasDataSource(host.Sources, SourceK8s) {
		t.Fatalf("expected merged host sources to include agent+kubernetes, got %+v", host.Sources)
	}
	if host.Kubernetes == nil {
		t.Fatal("expected merged host to retain kubernetes node payload")
	}
	if got := strings.TrimSpace(host.Kubernetes.NodeName); got != "prod-euw1-k8s-01" {
		t.Fatalf("kubernetes node name = %q, want prod-euw1-k8s-01", got)
	}
	if host.ParentID == nil || strings.TrimSpace(*host.ParentID) != cluster.ID {
		t.Fatalf("expected merged host parent to point at kubernetes cluster %q, got %+v", cluster.ID, host.ParentID)
	}
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

// TestRegistryIngestKeepsDockerContainersScopedToTheirHost pins the fix for the
// "frigate@141" host re-attribution flicker on the Docker page. The bug was
// that app-container source keys did not include the docker host, so any
// scenario producing a colliding container source ID (a docker ps that
// briefly returns an empty ID and parseDockerInventoryContainerLine falls
// back to the container name, a future short-ID truncation, daemon-side
// identifier reset across recreate cycles, ...) routed two containers
// reported by different hosts to the same registry resource. mergeInto
// then overwrote one host's Docker payload with the other's, flipping
// HostSourceID and ParentID on every projection rebuild until the colliding
// inputs diverged again.
func TestRegistryIngestKeepsDockerContainersScopedToTheirHost(t *testing.T) {
	rr := NewRegistry(nil)
	// Two distinct docker hosts each reporting a container whose docker
	// source ID happens to collide (here: both fell back to the "frigate"
	// name because their docker ps lines were missing an ID).
	rr.IngestSnapshot(models.StateSnapshot{
		DockerHosts: []models.DockerHost{
			{
				ID:       "proxmox-lxc-docker:pve-a:node-a:105",
				Hostname: "frigate.lab",
				Status:   "online",
				Containers: []models.DockerContainer{{
					ID:    "frigate",
					Name:  "frigate",
					State: "running",
				}},
			},
			{
				ID:       "proxmox-lxc-docker:pve-a:node-a:141",
				Hostname: "homepage-docker.lab",
				Status:   "online",
				Containers: []models.DockerContainer{{
					ID:    "frigate",
					Name:  "frigate",
					State: "running",
				}},
			},
		},
	})

	resources := rr.ListByType(ResourceTypeAppContainer)
	if len(resources) != 2 {
		t.Fatalf("expected two distinct docker container resources (one per host); got %d", len(resources))
	}

	hostsSeen := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		if resource.Docker == nil {
			t.Fatalf("docker container resource missing Docker payload: %#v", resource)
		}
		if resource.Name != "frigate" {
			t.Fatalf("unexpected container name: %q", resource.Name)
		}
		if !reflect.DeepEqual(resource.PlatformScopes, []string{"proxmox-pve", "docker"}) {
			t.Fatalf("platform scopes = %#v, want proxmox-pve + docker", resource.PlatformScopes)
		}
		hostsSeen[strings.TrimSpace(resource.Docker.HostSourceID)] = struct{}{}
	}
	for _, want := range []string{
		"proxmox-lxc-docker:pve-a:node-a:105",
		"proxmox-lxc-docker:pve-a:node-a:141",
	} {
		if _, ok := hostsSeen[want]; !ok {
			t.Fatalf("expected a container attributed to host %q; got hosts %v", want, hostsSeen)
		}
	}
}

func TestRegistryIngestSnapshotPublishesDockerNativeInventory(t *testing.T) {
	now := time.Date(2026, 5, 24, 8, 0, 0, 0, time.UTC)
	rr := NewRegistry(NewMemoryStore())
	rr.IngestSnapshot(models.StateSnapshot{
		DockerHosts: []models.DockerHost{{
			ID:       "docker-host-1",
			Hostname: "edge",
			Runtime:  "docker",
			Status:   "online",
			LastSeen: now,
			Images: []models.DockerImage{{
				ID: "sha256:image1", RepoTags: []string{"repo/app:latest"}, SizeBytes: 1024,
			}},
			Volumes: []models.DockerVolume{{Name: "app-data", Driver: "local", SizeBytes: 2048}},
			Networks: []models.DockerNetwork{{
				ID: "net1", Name: "app-net", Driver: "bridge", Subnets: []models.DockerNetworkSubnet{{Subnet: "10.88.0.0/24", Gateway: "10.88.0.1"}},
			}},
			Tasks: []models.DockerTask{{
				ID: "task-1", ServiceID: "svc-1", ServiceName: "api", Slot: 1, DesiredState: "running", CurrentState: "running",
			}},
			Secrets: []models.DockerSecret{{ID: "secret-1", Name: "db-password", DriverName: "builtin"}},
			Configs: []models.DockerConfig{{ID: "config-1", Name: "nginx-conf", TemplatingDriver: "golang"}},
			Swarm: &models.DockerSwarmInfo{
				NodeID:     "node-1",
				NodeRole:   "manager",
				LocalState: "active",
			},
			Nodes: []models.DockerNode{{
				ID: "node-1", Hostname: "manager-1", Role: "manager", State: "ready",
			}},
		}},
	})

	counts := map[ResourceType]int{}
	for _, resource := range rr.List() {
		counts[resource.Type]++
	}
	for _, resourceType := range []ResourceType{
		ResourceTypeDockerImage,
		ResourceTypeDockerVolume,
		ResourceTypeDockerNetwork,
		ResourceTypeDockerTask,
		ResourceTypeDockerSwarmNode,
		ResourceTypeDockerSecret,
		ResourceTypeDockerConfig,
	} {
		if counts[resourceType] != 1 {
			t.Fatalf("expected one %s resource, got counts %#v", resourceType, counts)
		}
	}
}

func TestRegistryIngestSnapshotLinksDockerContainersToNetworks(t *testing.T) {
	now := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	rr := NewRegistry(NewMemoryStore())
	rr.IngestSnapshot(models.StateSnapshot{
		DockerHosts: []models.DockerHost{{
			ID:       "docker-host-1",
			Hostname: "edge",
			Runtime:  "docker",
			Status:   "online",
			LastSeen: now,
			Containers: []models.DockerContainer{{
				ID:    "container-1",
				Name:  "api",
				Image: "repo/api:latest",
				State: "running",
				Networks: []models.DockerContainerNetworkLink{{
					Name: "frontend",
					IPv4: "10.88.0.12",
				}},
			}},
			Networks: []models.DockerNetwork{{
				ID:     "network-1",
				Name:   "frontend",
				Driver: "bridge",
			}},
		}},
	})

	containers := rr.ListByType(ResourceTypeAppContainer)
	networks := rr.ListByType(ResourceTypeDockerNetwork)
	if len(containers) != 1 || len(networks) != 1 {
		t.Fatalf("expected one container and one network, got containers=%d networks=%d", len(containers), len(networks))
	}

	container := containers[0]
	network := networks[0]
	assertDockerNetworkAttachment := func(resource Resource) {
		t.Helper()
		for _, relationship := range resource.Relationships {
			if relationship.Type != RelAttachedTo {
				continue
			}
			if relationship.SourceID != container.ID || relationship.TargetID != network.ID {
				t.Fatalf("unexpected attachment relationship endpoints on %s: %+v", resource.ID, relationship)
			}
			if relationship.Discoverer != dockerAdapterRelationshipDiscoverer {
				t.Fatalf("relationship discoverer = %q, want %q", relationship.Discoverer, dockerAdapterRelationshipDiscoverer)
			}
			if relationship.Metadata["networkName"] != "frontend" || relationship.Metadata["ipv4"] != "10.88.0.12" {
				t.Fatalf("relationship metadata = %#v", relationship.Metadata)
			}
			return
		}
		t.Fatalf("expected docker network attachment relationship on %s, got %+v", resource.ID, resource.Relationships)
	}

	assertDockerNetworkAttachment(container)
	assertDockerNetworkAttachment(network)
}

// A live source must override a metric held by a stale higher-priority source.
func TestMergeMetric_LiveSourceOverridesStaleHigherPriority(t *testing.T) {
	now := time.Now().UTC()
	status := map[DataSource]SourceStatus{
		SourceAgent:   {Status: "stale", LastSeen: now.Add(-2 * time.Hour)},
		SourceProxmox: {Status: "online", LastSeen: now},
	}
	existing := &MetricValue{Value: 0, Percent: 0, Source: SourceAgent}
	incoming := &MetricValue{Value: 6.2, Percent: 6.2, Source: SourceProxmox}

	got := mergeMetric(existing, incoming, SourceProxmox, now, status)
	if got == nil || got.Percent != 6.2 || got.Source != SourceProxmox {
		t.Fatalf("expected live proxmox CPU to override stale agent CPU, got %+v", got)
	}
}

// When both sources are fresh, static priority still decides: the agent wins.
func TestMergeMetric_FreshHigherPriorityStillWins(t *testing.T) {
	now := time.Now().UTC()
	status := map[DataSource]SourceStatus{
		SourceAgent:   {Status: "online", LastSeen: now},
		SourceProxmox: {Status: "online", LastSeen: now},
	}
	existing := &MetricValue{Percent: 6.2, Source: SourceProxmox}
	incoming := &MetricValue{Percent: 12.5, Source: SourceAgent}

	got := mergeMetric(existing, incoming, SourceAgent, now, status)
	if got == nil || got.Percent != 12.5 || got.Source != SourceAgent {
		t.Fatalf("expected fresh agent CPU to win by priority, got %+v", got)
	}
}

// A stale source must not clobber a metric currently held by a live source.
func TestMergeMetric_StaleSourceDoesNotClobberLive(t *testing.T) {
	now := time.Now().UTC()
	status := map[DataSource]SourceStatus{
		SourceAgent:   {Status: "stale", LastSeen: now.Add(-2 * time.Hour)},
		SourceProxmox: {Status: "online", LastSeen: now},
	}
	existing := &MetricValue{Percent: 6.2, Source: SourceProxmox}
	incoming := &MetricValue{Percent: 0, Source: SourceAgent}

	got := mergeMetric(existing, incoming, SourceAgent, now, status)
	if got == nil || got.Percent != 6.2 || got.Source != SourceProxmox {
		t.Fatalf("expected live proxmox CPU to survive stale agent merge, got %+v", got)
	}
}

// The merged-source host shape from the homelab canonical-ID bug: a PVE node
// record that only knows cluster+hostname, and a pulse-agent record that
// knows the machine ID. Whichever record mints the canonical resource decides
// which identity key gets hashed, so without durable identity pins the
// canonical ID flips between boot windows (node-only) and steady state
// (agent present), fragmenting the change journal into per-boot eras.
func mergedHostNodeResource() Resource {
	return Resource{
		Type:    ResourceTypeAgent,
		Name:    "delly",
		Status:  StatusOnline,
		Proxmox: &ProxmoxData{SourceID: "homelab-delly", NodeName: "delly", ClusterName: "homelab"},
	}
}

func mergedHostAgentResource(machineID string) Resource {
	return Resource{
		Type:   ResourceTypeAgent,
		Name:   "delly",
		Status: StatusOnline,
		Agent:  &AgentData{AgentID: machineID, Hostname: "delly", MachineID: machineID},
	}
}

func TestMergedHostCanonicalIDStableAcrossRestartsAndIngestOrders(t *testing.T) {
	store := NewMemoryStore()

	const machineID = "7d465a78-test-machine-id"
	nodeIdentity := ResourceIdentity{Hostnames: []string{"delly"}, ClusterName: "homelab"}
	agentIdentity := ResourceIdentity{MachineID: machineID, Hostnames: []string{"delly"}}
	// IngestSnapshot merges the linked host's identity into the node record
	// when the agent is present in the snapshot.
	steadyIdentity := mergeIdentity(nodeIdentity, agentIdentity)

	steadyID := buildHashID(ResourceTypeAgent, "machine:"+machineID)
	bootEraID := buildHashID(ResourceTypeAgent, "cluster:homelab:delly")
	if steadyID == bootEraID {
		t.Fatalf("test setup broken: era IDs must differ")
	}

	// Steady-state boot: node record carries the merged identity and mints
	// the machine-keyed canonical ID; the agent record merges into it.
	steady := NewRegistry(store)
	if id := steady.ingest(SourceProxmox, "homelab-delly", mergedHostNodeResource(), steadyIdentity); id != steadyID {
		t.Fatalf("steady-state node ingest minted %q, want machine-keyed %q", id, steadyID)
	}
	if id := steady.ingest(SourceAgent, machineID, mergedHostAgentResource(machineID), agentIdentity); id != steadyID {
		t.Fatalf("steady-state agent ingest resolved %q, want %q", id, steadyID)
	}
	steady.PersistIdentityPins()

	// Restart into a boot window: the agent has not checked in yet, so the
	// node record only knows cluster+hostname. Before identity pins this
	// minted bootEraID and the change journal fragmented into a second era.
	bootWindow := NewRegistry(store)
	if id := bootWindow.ingest(SourceProxmox, "homelab-delly", mergedHostNodeResource(), nodeIdentity); id != steadyID {
		t.Fatalf("boot-window node ingest minted %q, want pinned %q", id, steadyID)
	}

	// Restart with the opposite ingest order: agent record first into an
	// empty registry, node record after.
	reversed := NewRegistry(store)
	if id := reversed.ingest(SourceAgent, machineID, mergedHostAgentResource(machineID), agentIdentity); id != steadyID {
		t.Fatalf("agent-first ingest minted %q, want %q", id, steadyID)
	}
	if id := reversed.ingest(SourceProxmox, "homelab-delly", mergedHostNodeResource(), nodeIdentity); id != steadyID {
		t.Fatalf("node-after-agent ingest resolved %q, want %q", id, steadyID)
	}
	if got := len(reversed.List()); got != 1 {
		t.Fatalf("expected one merged resource after reversed-order ingest, got %d", got)
	}
}

// A synthesized placeholder (e.g. an offline PVE node for an instance that
// has never completed a poll) carries a zero LastSeen. Ingest must preserve
// that zero instead of fabricating a fresh sighting, and the per-source
// delivery status must be "unknown", not "online" — an online stamp with a
// zero LastSeen is permanently exempt from stale-marking and masks the
// source's real delivery state (the loop behind commit 8372a22c5).
func TestIngestPreservesNeverSeenSightingAndStampsUnknownSourceStatus(t *testing.T) {
	rr := NewRegistry(nil)

	placeholder := topLevelTestProxmoxNode("proxmox-node", "tower", "proxmox-1", "https://tower.local:8006")
	placeholder.Status = StatusOffline
	placeholder.LastSeen = time.Time{}

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{SourceID: "proxmox-node", Resource: placeholder},
	})

	resources := rr.List()
	if len(resources) != 1 {
		t.Fatalf("List() returned %d resources, want 1", len(resources))
	}
	resource := resources[0]

	if !resource.LastSeen.IsZero() {
		t.Fatalf("resource.LastSeen = %s, want zero: never-seen resources must not get a fabricated sighting", resource.LastSeen)
	}
	status, ok := resource.SourceStatus[SourceProxmox]
	if !ok {
		t.Fatal("expected a proxmox source status entry")
	}
	if status.Status != "unknown" {
		t.Fatalf("SourceStatus[proxmox].Status = %q, want %q: a source that never delivered the resource must not be stamped online", status.Status, "unknown")
	}
	if !status.LastSeen.IsZero() {
		t.Fatalf("SourceStatus[proxmox].LastSeen = %s, want zero", status.LastSeen)
	}

	// Stale marking must leave a never-seen source alone in both directions.
	rr.MarkStale(time.Now().UTC(), nil)
	if got := rr.List()[0].SourceStatus[SourceProxmox].Status; got != "unknown" {
		t.Fatalf("SourceStatus[proxmox].Status after MarkStale = %q, want %q", got, "unknown")
	}
}

// A real sighting keeps the online stamp and the genuine timestamp on both
// the resource and the per-source delivery status.
func TestIngestStampsOnlineSourceStatusForRealSighting(t *testing.T) {
	rr := NewRegistry(nil)
	seen := time.Now().UTC().Add(-5 * time.Second).Truncate(time.Millisecond)

	node := topLevelTestProxmoxNode("proxmox-node", "tower", "proxmox-1", "https://tower.local:8006")
	node.LastSeen = seen

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{SourceID: "proxmox-node", Resource: node},
	})

	resource := rr.List()[0]
	if !resource.LastSeen.Equal(seen) {
		t.Fatalf("resource.LastSeen = %s, want %s", resource.LastSeen, seen)
	}
	status := resource.SourceStatus[SourceProxmox]
	if status.Status != "online" {
		t.Fatalf("SourceStatus[proxmox].Status = %q, want online", status.Status)
	}
	if !status.LastSeen.Equal(seen) {
		t.Fatalf("SourceStatus[proxmox].LastSeen = %s, want %s", status.LastSeen, seen)
	}
}

// Datastore-derived storage entries written by the PBS poller (instance
// "pbs-<name>", type "pbs") are delivered on the PBS cadence, so they must be
// keyed to SourcePBS — judging them against the faster Proxmox stale
// threshold would flap healthy datastores stale between PBS polls. PVE also
// reports pbs-typed storage.cfg backends, and those stay SourceProxmox: they
// really are delivered by the PVE storage poll.
func TestIngestStorageRoutesPBSPollerEntriesToPBSSource(t *testing.T) {
	seen := time.Now().UTC().Add(-30 * time.Second).Truncate(time.Millisecond)
	snapshot := models.StateSnapshot{
		PBSInstances: []models.PBSInstance{{
			ID:       "pbs-backup",
			Name:     "backup",
			Host:     "https://pbs.local:8007",
			Status:   "online",
			LastSeen: seen,
		}},
		Storage: []models.Storage{
			{
				ID:       "pbs-backup-main",
				Name:     "main",
				Node:     "backup",
				Instance: "pbs-backup",
				Type:     "pbs",
				Status:   "available",
				LastSeen: seen,
			},
			{
				ID:       "home-pve1-pbs-remote",
				Name:     "pbs-remote",
				Node:     "pve1",
				Instance: "home",
				Type:     "pbs",
				Status:   "available",
				LastSeen: seen,
			},
		},
	}

	rr := NewRegistry(nil)
	rr.IngestSnapshot(snapshot)

	var pbsInstanceID string
	var pollerStorage, pveStorage *Resource
	for _, resource := range rr.List() {
		resource := resource
		switch {
		case resource.Type == ResourceTypePBS:
			pbsInstanceID = resource.ID
		case resource.Type == ResourceTypeStorage && resource.Proxmox != nil && resource.Proxmox.SourceID == "pbs-backup-main":
			pollerStorage = &resource
		case resource.Type == ResourceTypeStorage && resource.Proxmox != nil && resource.Proxmox.SourceID == "home-pve1-pbs-remote":
			pveStorage = &resource
		}
	}
	if pbsInstanceID == "" {
		t.Fatal("expected a PBS instance resource")
	}
	if pollerStorage == nil || pveStorage == nil {
		t.Fatalf("expected both storage resources (pollerStorage=%v, pveStorage=%v)", pollerStorage != nil, pveStorage != nil)
	}

	if len(pollerStorage.Sources) != 1 || pollerStorage.Sources[0] != SourcePBS {
		t.Fatalf("PBS poller storage Sources = %v, want [%s]", pollerStorage.Sources, SourcePBS)
	}
	status, ok := pollerStorage.SourceStatus[SourcePBS]
	if !ok {
		t.Fatal("expected a pbs source status entry on the PBS poller storage")
	}
	if !status.LastSeen.Equal(seen) {
		t.Fatalf("SourceStatus[pbs].LastSeen = %s, want %s", status.LastSeen, seen)
	}
	if pollerStorage.ParentID == nil || *pollerStorage.ParentID != pbsInstanceID {
		t.Fatalf("PBS poller storage ParentID = %v, want the PBS instance %q", pollerStorage.ParentID, pbsInstanceID)
	}

	if len(pveStorage.Sources) != 1 || pveStorage.Sources[0] != SourceProxmox {
		t.Fatalf("PVE-reported pbs-typed storage Sources = %v, want [%s]", pveStorage.Sources, SourceProxmox)
	}
}

func TestRegistryAvailabilityLinkAttachesFacetToKnownResource(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Now().UTC()
	rr.IngestRecords(SourceAgent, []IngestRecord{{
		SourceID: "host-1",
		Resource: Resource{Type: ResourceTypeAgent, Name: "host-1", Status: StatusOnline, LastSeen: now},
		Identity: ResourceIdentity{MachineID: "machine-1"},
	}})
	hostID := rr.ListByType(ResourceTypeAgent)[0].ID

	rr.IngestRecords(SourceAvailability, []IngestRecord{{
		SourceID: "probe-1",
		Resource: Resource{
			Type: ResourceTypeNetworkEndpoint, Name: "probe-1", Status: StatusOnline, LastSeen: now,
			Sources: []DataSource{SourceAvailability},
			Availability: &AvailabilityData{
				TargetID: "probe-1", LinkedResourceID: hostID, Address: "192.0.2.10",
				Protocol: "icmp", Enabled: true, Available: true,
			},
		},
		Identity: ResourceIdentity{IPAddresses: []string{"192.0.2.10"}},
	}})

	if got := rr.ListByType(ResourceTypeNetworkEndpoint); len(got) != 0 {
		t.Fatalf("expected 0 standalone network endpoints, got %d", len(got))
	}
	host, ok := rr.Get(hostID)
	if !ok || host.Availability == nil || host.Availability.TargetID != "probe-1" {
		t.Fatalf("expected availability facet probe-1 on host, got %+v", host.Availability)
	}
}

func TestChooseStatusAvailabilityDoesNotOverrideHigherPriority(t *testing.T) {
	rr := NewRegistry(nil)
	now := time.Now().UTC()

	rr.IngestRecords(SourceProxmox, []IngestRecord{{
		SourceID: "delly:minipc:135",
		Resource: Resource{
			Type: ResourceTypeSystemContainer, Name: "grafana",
			Status: StatusOnline, LastSeen: now,
		},
		Identity: ResourceIdentity{IPAddresses: []string{"192.0.2.130"}},
	}})

	container := rr.ListByType(ResourceTypeSystemContainer)[0]

	rr.IngestRecords(SourceAvailability, []IngestRecord{{
		SourceID: "probe-grafana",
		Resource: Resource{
			Type: ResourceTypeNetworkEndpoint, Name: "probe-grafana",
			Status: StatusOffline, LastSeen: now,
			Sources: []DataSource{SourceAvailability},
			Availability: &AvailabilityData{
				TargetID: "probe-grafana", LinkedResourceID: container.ID,
				Address: "192.0.2.130", Protocol: "http", Port: 3000,
				Enabled: true, Available: false, ConsecutiveFailures: 3,
				FailureThreshold: 2,
			},
		},
		Identity: ResourceIdentity{IPAddresses: []string{"192.0.2.130"}},
	}})

	got, ok := rr.Get(container.ID)
	if !ok {
		t.Fatal("resource not found after availability ingest")
	}
	if got.Status != StatusOnline {
		t.Fatalf("availability source (offline) should not override proxmox (online); got %s", got.Status)
	}
	if got.Availability == nil || got.Availability.Available != false {
		t.Fatalf("availability facet should record unavailable, got %+v", got.Availability)
	}
}

func TestMarkStaleRecomputesFromRemainingFreshSources(t *testing.T) {
	rr := NewRegistry(nil)
	oldNow := time.Now().UTC().Add(-5 * time.Minute)
	recentNow := time.Now().UTC()

	rr.IngestRecords(SourceProxmox, []IngestRecord{{
		SourceID: "delly:delly",
		Resource: Resource{
			Type: ResourceTypeAgent, Name: "delly",
			Status: StatusOnline, LastSeen: oldNow,
		},
		Identity: ResourceIdentity{MachineID: "delly"},
	}})
	host := rr.ListByType(ResourceTypeAgent)[0]

	rr.IngestRecords(SourceAvailability, []IngestRecord{{
		SourceID: "probe-delly",
		Resource: Resource{
			Type: ResourceTypeNetworkEndpoint, Name: "probe-delly",
			Status: StatusOnline, LastSeen: recentNow,
			Sources: []DataSource{SourceAvailability},
			Availability: &AvailabilityData{
				TargetID: "probe-delly", LinkedResourceID: host.ID,
				Address: "192.0.2.5", Protocol: "tcp", Port: 8006,
				Enabled: true, Available: true,
			},
		},
		Identity: ResourceIdentity{IPAddresses: []string{"192.0.2.5"}},
	}})

	check, ok := rr.Get(host.ID)
	if !ok || check.Status != StatusOnline {
		t.Fatalf("expected online before stale, got %s", check.Status)
	}

	rr.MarkStale(recentNow, nil)

	got, ok := rr.Get(host.ID)
	if !ok {
		t.Fatal("resource not found after MarkStale")
	}
	proxmoxStatus := got.SourceStatus[SourceProxmox]
	if proxmoxStatus.Status != "stale" {
		t.Fatalf("expected proxmox source to be stale, got %s", proxmoxStatus.Status)
	}
	availStatus := got.SourceStatus[SourceAvailability]
	if availStatus.Status != "online" {
		t.Fatalf("expected availability source to still be fresh/online, got %s", availStatus.Status)
	}
	if got.Status != StatusOnline {
		t.Fatalf("status should be recomputed from remaining fresh availability source; got %s, want online", got.Status)
	}
}

func TestResourceRegistryUsesConfiguredProxmoxStaleThresholds(t *testing.T) {
	seen := time.Now().UTC().Add(-90 * time.Second).Truncate(time.Millisecond)
	snapshot := models.StateSnapshot{
		VMs: []models.VM{{
			ID:       "cluster-a:pve-a:101",
			Name:     "db",
			Node:     "pve-a",
			Instance: "cluster-a",
			VMID:     101,
			Status:   "running",
			Type:     "qemu",
			LastSeen: seen,
		}},
	}

	defaultRegistry := NewRegistry(nil)
	defaultRegistry.IngestSnapshot(snapshot)
	defaultVMs := defaultRegistry.ListByType(ResourceTypeVM)
	if len(defaultVMs) != 1 {
		t.Fatalf("default VM count = %d, want 1", len(defaultVMs))
	}
	if defaultVMs[0].Status != StatusWarning {
		t.Fatalf("default VM status = %q, want warning", defaultVMs[0].Status)
	}

	thresholds := map[DataSource]time.Duration{
		SourceProxmox: 10 * time.Minute,
	}
	configuredRegistry := NewRegistry(nil)
	configuredRegistry.IngestSnapshotWithStaleThresholds(snapshot, thresholds)
	configuredVMs := configuredRegistry.ListByType(ResourceTypeVM)
	if len(configuredVMs) != 1 {
		t.Fatalf("configured VM count = %d, want 1", len(configuredVMs))
	}
	if configuredVMs[0].Status != StatusOnline {
		t.Fatalf("configured VM status = %q, want online", configuredVMs[0].Status)
	}

	clonedRegistry := NewRegistry(nil)
	clonedRegistry.IngestResourcesWithStaleThresholds(configuredRegistry.List(), thresholds)
	clonedVMs := clonedRegistry.ListByType(ResourceTypeVM)
	if len(clonedVMs) != 1 {
		t.Fatalf("cloned VM count = %d, want 1", len(clonedVMs))
	}
	if clonedVMs[0].Status != StatusOnline {
		t.Fatalf("cloned VM status = %q, want online", clonedVMs[0].Status)
	}
	if status := clonedVMs[0].SourceStatus[SourceProxmox]; status.Status != "online" {
		t.Fatalf("cloned SourceStatus[proxmox] = %+v, want online", status)
	}
}
