package unifiedresources

import (
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestIngestSnapshotIncludesPBSAndPMGInstances(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		PBSInstances: []models.PBSInstance{
			{
				ID:          "pbs-1",
				Name:        "pbs-main",
				Host:        "https://pbs.example.com:8007",
				Status:      "online",
				Version:     "3.2.1",
				CPU:         22.5,
				Memory:      48.0,
				MemoryUsed:  8 * 1024 * 1024 * 1024,
				MemoryTotal: 16 * 1024 * 1024 * 1024,
				Uptime:      86400,
				Datastores: []models.PBSDatastore{{
					Name:   "fast",
					Status: "online",
					Total:  100,
					Used:   96,
				}},
				BackupJobs: []models.PBSBackupJob{{ID: "job-1"}},
				JobHealthEvidence: []models.PBSJobHealthEvidence{{
					ID:             "sync-remote-a",
					Family:         "sync",
					Store:          "fast",
					LastRunState:   "OK",
					LastRunUPID:    "UPID:sync:1",
					LastRunEndtime: now.Add(-time.Hour).Unix(),
					Confidence:     "direct-task-match",
					EvidenceSource: "pbs-job-config",
					EvidenceScope:  "configured-job",
					Freshness: models.PBSJobHealthFreshness{
						ObservedAt:     now,
						LastRunEndTime: now.Add(-time.Hour),
						State:          "observed",
					},
					Posture: "healthy",
				}},
				ConnectionHealth: "online",
				LastSeen:         now,
			},
		},
		PMGInstances: []models.PMGInstance{
			{
				ID:               "pmg-1",
				Name:             "pmg-main",
				Host:             "https://pmg.example.com:8006",
				GuestURL:         "https://pmg.example.com/quarantine",
				Status:           "online",
				Version:          "8.2",
				ConnectionHealth: "connected",
				LastSeen:         now,
				LastUpdated:      now,
				MailStats: &models.PMGMailStats{
					CountTotal:      1900,
					BytesIn:         5_000_000,
					BytesOut:        4_000_000,
					SpamIn:          125,
					VirusIn:         4,
					JunkIn:          32,
					PregreetRejects: 7,
					UpdatedAt:       now,
				},
				MailCount: []models.PMGMailCountPoint{{Timestamp: now, Count: 1900, CountIn: 1200, CountOut: 700, Timeframe: "hour", Index: 1}},
				Nodes: []models.PMGNodeStatus{
					{
						Name:   "pmg-node-1",
						Status: "online",
						Uptime: 43200,
						QueueStatus: &models.PMGQueueStatus{
							Active:    12,
							Deferred:  5,
							Hold:      2,
							Incoming:  3,
							Total:     22,
							OldestAge: 1800,
							UpdatedAt: now,
						},
					},
				},
			},
		},
	}

	registry := NewRegistry(NewMemoryStore())
	registry.IngestSnapshot(snapshot)

	resources := registry.List()
	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	var pbsResource *Resource
	var datastoreResource *Resource
	var pmgResource *Resource
	for i := range resources {
		resource := resources[i]
		switch resource.Type {
		case ResourceTypePBS:
			pbsResource = &resource
		case ResourceTypeStorage:
			if resource.Storage != nil && resource.Storage.Platform == "pbs" {
				datastoreResource = &resource
			}
		case ResourceTypePMG:
			pmgResource = &resource
		}
	}

	if pbsResource == nil {
		t.Fatal("expected PBS resource")
	}
	if !containsDataSource(pbsResource.Sources, SourcePBS) {
		t.Fatalf("expected PBS source, got %+v", pbsResource.Sources)
	}
	if pbsResource.Metrics == nil || pbsResource.Metrics.CPU == nil {
		t.Fatalf("expected PBS CPU metrics, got %+v", pbsResource.Metrics)
	}
	if pbsResource.PBS == nil || pbsResource.PBS.DatastoreCount != 1 {
		t.Fatalf("expected PBS payload with datastore count, got %+v", pbsResource.PBS)
	}
	if pbsResource.PBS.JobHealthEvidenceCount != 1 || len(pbsResource.PBS.JobHealthEvidence) != 1 {
		t.Fatalf("expected PBS job health evidence ledger, got %+v", pbsResource.PBS)
	}
	if got := pbsResource.PBS.JobHealthEvidence[0]; got.Confidence != "direct-task-match" || got.EvidenceSource != "pbs-job-config" || got.EvidenceScope != "configured-job" || got.LastRunState != "OK" {
		t.Fatalf("expected direct PBS job evidence with raw fields, got %+v", got)
	}
	if pbsResource.Status != StatusWarning {
		t.Fatalf("expected PBS instance warning status from rolled-up datastore risk, got %q", pbsResource.Status)
	}
	if pbsResource.PBS.StorageRisk == nil || len(pbsResource.PBS.StorageRisk.Reasons) == 0 {
		t.Fatalf("expected PBS instance storage risk payload, got %+v", pbsResource.PBS)
	}
	if len(pbsResource.Incidents) == 0 || pbsResource.Incidents[0].Code != "capacity_runway_low" {
		t.Fatalf("expected rolled-up PBS incidents, got %+v", pbsResource.Incidents)
	}
	if datastoreResource == nil {
		t.Fatal("expected PBS datastore storage resource")
	}
	if datastoreResource.Storage == nil {
		t.Fatalf("expected PBS datastore storage metadata, got %+v", datastoreResource)
	}
	if datastoreResource.Storage.Platform != "pbs" || datastoreResource.Storage.Topology != "datastore" {
		t.Fatalf("expected PBS datastore platform/topology, got %+v", datastoreResource.Storage)
	}
	if datastoreResource.Status != StatusWarning {
		t.Fatalf("expected PBS datastore warning status from derived risk, got %q", datastoreResource.Status)
	}
	if datastoreResource.Storage.Risk == nil || len(datastoreResource.Storage.Risk.Reasons) == 0 {
		t.Fatalf("expected PBS datastore risk payload, got %+v", datastoreResource.Storage)
	}
	if len(datastoreResource.Incidents) == 0 || datastoreResource.Incidents[0].Code != "capacity_runway_low" {
		t.Fatalf("expected PBS datastore incidents, got %+v", datastoreResource.Incidents)
	}
	if datastoreResource.ParentID == nil || *datastoreResource.ParentID != pbsResource.ID {
		t.Fatalf("expected PBS datastore to be parented under PBS instance, got %+v", datastoreResource.ParentID)
	}

	if pmgResource == nil {
		t.Fatal("expected PMG resource")
	}
	if !containsDataSource(pmgResource.Sources, SourcePMG) {
		t.Fatalf("expected PMG source, got %+v", pmgResource.Sources)
	}
	if pmgResource.Metrics == nil {
		t.Fatalf("expected PMG metrics payload, got nil")
	}
	if pmgResource.PMG == nil || pmgResource.PMG.QueueTotal != 22 {
		t.Fatalf("expected PMG payload with queue totals, got %+v", pmgResource.PMG)
	}
	if pmgResource.PMG.HostURL != "https://pmg.example.com:8006" || pmgResource.PMG.GuestURL != "https://pmg.example.com/quarantine" {
		t.Fatalf("expected PMG URL payloads, got %+v", pmgResource.PMG)
	}
	if len(pmgResource.PMG.Nodes) != 1 || pmgResource.PMG.Nodes[0].QueueStatus == nil || pmgResource.PMG.Nodes[0].QueueStatus.OldestAge != 1800 {
		t.Fatalf("expected PMG queue oldest age, got %+v", pmgResource.PMG.Nodes)
	}
	if pmgResource.PMG.MailStats == nil || pmgResource.PMG.MailStats.CountTotal != 1900 || pmgResource.PMG.MailStats.JunkIn != 32 || pmgResource.PMG.MailStats.PregreetRejects != 7 {
		t.Fatalf("expected full PMG mail stats payload, got %+v", pmgResource.PMG.MailStats)
	}
	if len(pmgResource.PMG.MailCount) != 1 || pmgResource.PMG.MailCount[0].Index != 1 {
		t.Fatalf("expected PMG mail count points, got %+v", pmgResource.PMG.MailCount)
	}
}

func TestIngestSnapshotAssociatesPBSHostAgentPhysicalDisks(t *testing.T) {
	now := time.Now().UTC()
	registry := NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:        "agent-pbs-1",
				Hostname:  "pbs-one.local",
				MachineID: "machine-pbs-1",
				Status:    "online",
				LastSeen:  now,
				Sensors: models.HostSensorSummary{
					SMART: []models.HostDiskSMART{
						{
							Device:      "/dev/sda",
							Model:       "Backup Drive",
							Serial:      "PBS-DISK-1",
							Health:      "PASSED",
							Temperature: 36,
						},
					},
				},
			},
		},
		PBSInstances: []models.PBSInstance{
			{
				ID:       "pbs-1",
				Name:     "pbs-one",
				Host:     "https://pbs-one.example:8007",
				Status:   "online",
				LastSeen: now,
			},
		},
	})

	var hostResource *Resource
	var diskResource *Resource
	for _, resource := range registry.List() {
		switch resource.Type {
		case ResourceTypeAgent:
			if resource.Agent != nil && resource.Agent.AgentID == "agent-pbs-1" {
				copy := resource
				hostResource = &copy
			}
		case ResourceTypePhysicalDisk:
			if resource.PhysicalDisk != nil && resource.PhysicalDisk.Serial == "PBS-DISK-1" {
				copy := resource
				diskResource = &copy
			}
		}
	}
	if hostResource == nil || diskResource == nil {
		t.Fatalf("expected correlated PBS host and disk, host=%+v disk=%+v", hostResource, diskResource)
	}
	if !containsDataSource(hostResource.Sources, SourcePBS) {
		t.Fatalf("PBS host sources = %v, want PBS membership", hostResource.Sources)
	}
	if !containsDataSource(diskResource.Sources, SourceAgent) ||
		!containsDataSource(diskResource.Sources, SourcePBS) {
		t.Fatalf("PBS disk sources = %v, want agent and PBS", diskResource.Sources)
	}
	if diskResource.ParentID == nil || *diskResource.ParentID != hostResource.ID {
		t.Fatalf(
			"PBS disk parent = %v, want agent host %q to retain canonical parent authority",
			diskResource.ParentID,
			hostResource.ID,
		)
	}
	RefreshCanonicalMetadata(diskResource)
	if !containsPlatformScope(diskResource.PlatformScopes, "proxmox-pbs") {
		t.Fatalf("PBS disk platform scopes = %v, want proxmox-pbs", diskResource.PlatformScopes)
	}
}

func TestIngestSnapshotSkipsAmbiguousPBSHostAgentAssociation(t *testing.T) {
	now := time.Now().UTC()
	hosts := []models.Host{
		{
			ID:       "agent-pbs-a",
			Hostname: "pbs-one.local",
			Status:   "online",
			LastSeen: now,
			Sensors: models.HostSensorSummary{
				SMART: []models.HostDiskSMART{{Device: "/dev/sda", Serial: "PBS-A"}},
			},
		},
		{
			ID:       "agent-pbs-b",
			Hostname: "pbs-one.example",
			Status:   "online",
			LastSeen: now,
			Sensors: models.HostSensorSummary{
				SMART: []models.HostDiskSMART{{Device: "/dev/sdb", Serial: "PBS-B"}},
			},
		},
	}
	registry := NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		Hosts: hosts,
		PBSInstances: []models.PBSInstance{
			{
				ID:       "pbs-1",
				Name:     "pbs-one",
				Host:     "https://pbs-one.example:8007",
				Status:   "online",
				LastSeen: now,
			},
		},
	})

	for _, resource := range registry.ListByType(ResourceTypePhysicalDisk) {
		if containsDataSource(resource.Sources, SourcePBS) {
			t.Fatalf("ambiguous PBS host disk gained PBS source membership: %+v", resource)
		}
	}
}

// A PBS connection is often configured by IP or a DNS alias the agent never
// reports. The node hostname PBS reports about itself is then the only machine
// identity linking the host agent to the connection (#1723).
func TestIngestSnapshotAssociatesPBSHostAgentByReportedNodeName(t *testing.T) {
	now := time.Now().UTC()
	registry := NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:        "agent-pbs-1",
				Hostname:  "pbs-one.local",
				MachineID: "machine-pbs-1",
				ReportIP:  "10.9.9.9",
				Status:    "online",
				LastSeen:  now,
				Sensors: models.HostSensorSummary{
					SMART: []models.HostDiskSMART{{Device: "/dev/sda", Serial: "PBS-DISK-1"}},
				},
			},
		},
		PBSInstances: []models.PBSInstance{
			{
				ID:   "pbs-1",
				Name: "backup-connection",     // connection label, not the machine
				Host: "https://10.0.0.5:8007", // endpoint the agent never reports
				// Machine identity the PBS node reports about itself.
				NodeName: "pbs-one.local",
				Status:   "online",
				LastSeen: now,
			},
		},
	})

	var pbsResource *Resource
	var hostResource *Resource
	for _, resource := range registry.List() {
		switch resource.Type {
		case ResourceTypePBS:
			copy := resource
			pbsResource = &copy
		case ResourceTypeAgent:
			if resource.Agent != nil && resource.Agent.AgentID == "agent-pbs-1" {
				copy := resource
				hostResource = &copy
			}
		}
	}
	if pbsResource == nil || pbsResource.PBS == nil {
		t.Fatalf("expected PBS resource with payload, got %+v", pbsResource)
	}
	if pbsResource.PBS.NodeName != "pbs-one.local" {
		t.Fatalf("PBS nodeName = %q, want reported machine hostname", pbsResource.PBS.NodeName)
	}
	if hostResource == nil {
		t.Fatal("expected host agent resource")
	}
	if !containsDataSource(hostResource.Sources, SourcePBS) {
		t.Fatalf("PBS host sources = %v, want PBS membership via reported node name", hostResource.Sources)
	}
	if pbsResource.PBS.LinkedAgentID != "agent-pbs-1" {
		t.Fatalf("PBS linked agent = %q, want agent-pbs-1", pbsResource.PBS.LinkedAgentID)
	}
}

// A PBS API token cannot read GET /nodes, so its poll has no nodeName. The
// unique endpoint/interface match still identifies the host, even after PVE
// is added on that same machine and the agent folds into its PVE node view.
func TestIngestSnapshotPBSHostLinkSurvivesSideBySidePVEWithToken(t *testing.T) {
	now := time.Now().UTC()
	for _, withPVE := range []bool{false, true} {
		snapshot := models.StateSnapshot{
			Hosts: []models.Host{{
				ID: "agent-uuid", Hostname: "backup-host.local", ReportIP: "10.0.0.5",
				Status: "online", LastSeen: now,
				NetworkInterfaces: []models.HostNetworkInterface{{
					Name: "eth0", Addresses: []string{"10.0.0.5/24"},
				}},
			}},
			PBSInstances: []models.PBSInstance{{
				ID: "pbs-service", Name: "backup-connection", Host: "https://10.0.0.5:8007",
				Status: "online", LastSeen: now,
			}},
		}
		if withPVE {
			snapshot.Nodes = []models.Node{{
				ID: "pve/backup-host", Name: "backup-host", DisplayName: "PVE display label",
				Instance: "pve-connection", Host: "https://10.0.0.5:8006",
				LinkedAgentID: "agent-uuid", Status: "online", LastSeen: now,
			}}
			snapshot.Hosts[0].LinkedNodeID = "pve/backup-host"
		}
		registry := NewRegistry(nil)
		registry.IngestSnapshot(snapshot)
		var pbs *Resource
		var host *Resource
		for _, resource := range registry.ListForPresentation() {
			resource := resource
			if resource.Type == ResourceTypePBS {
				pbs = &resource
			}
			if resource.Agent != nil && resource.Agent.AgentID == "agent-uuid" {
				host = &resource
			}
		}
		if pbs == nil || host == nil {
			t.Fatalf("withPVE=%t: missing PBS or host resource: pbs=%+v host=%+v", withPVE, pbs, host)
		}
		if pbs.PBS == nil || pbs.PBS.LinkedAgentID != "agent-uuid" {
			t.Fatalf("withPVE=%t: PBS linked agent = %+v, want agent-uuid", withPVE, pbs.PBS)
		}
		if hostTarget := registry.MetricsTarget(host.ID); hostTarget == nil || *hostTarget != (MetricsTarget{ResourceType: "agent", ResourceID: "agent-uuid"}) {
			t.Fatalf("withPVE=%t: host target = %+v, want agent UUID", withPVE, hostTarget)
		}
		if withPVE && host.Proxmox == nil {
			t.Fatalf("PVE+PBS host lost PVE facet: %+v", host)
		}
	}
}

// #1723 includes PBS running inside PVE guests. A token-auth PBS connection
// cannot supply nodeName, and an in-guest Agent may omit interface addresses;
// the independently observed PVE guest IP plus the state-owned Agent/VM link
// still identify the machine without guessing from connection labels.
func TestIngestSnapshotAssociatesPBSGuestAgentByExactVMIP(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{
		Hosts: []models.Host{
			{ID: "agent-one", Hostname: "guest-one", LinkedVMID: "pve:node:103", Status: "online", LastSeen: now},
			{ID: "agent-two", Hostname: "guest-two", LinkedVMID: "pve:node:203", Status: "online", LastSeen: now},
		},
		VMs: []models.VM{
			{ID: "pve:node:103", VMID: 103, Name: "pbs-vm-one", Instance: "pve", Node: "node", Status: "running", IPAddresses: []string{"10.2.0.13"}, LastSeen: now},
			{ID: "pve:node:203", VMID: 203, Name: "pbs-vm-two", Instance: "pve", Node: "node", Status: "running", IPAddresses: []string{"10.2.0.23"}, LastSeen: now},
		},
		PBSInstances: []models.PBSInstance{
			{ID: "pbs-one", Name: "backup-one", Host: "https://10.2.0.13:8007", Status: "online", LastSeen: now},
			{ID: "pbs-two", Name: "backup-two", Host: "https://10.2.0.23:8007", Status: "online", LastSeen: now},
		},
	}
	registry := NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	links := map[string]string{}
	linkedHosts := map[string]bool{}
	for _, resource := range registry.ListForPresentation() {
		if resource.Type == ResourceTypePBS && resource.PBS != nil {
			links[resource.PBS.InstanceID] = resource.PBS.LinkedAgentID
		}
		if resource.Type == ResourceTypeAgent && resource.Agent != nil && containsDataSource(resource.Sources, SourcePBS) {
			linkedHosts[resource.Agent.AgentID] = true
			target := registry.MetricsTarget(resource.ID)
			if target == nil || *target != (MetricsTarget{ResourceType: "agent", ResourceID: resource.Agent.AgentID}) {
				t.Fatalf("host %s history target = %+v", resource.Agent.AgentID, target)
			}
		}
	}
	if links["pbs-one"] != "agent-one" || links["pbs-two"] != "agent-two" ||
		!linkedHosts["agent-one"] || !linkedHosts["agent-two"] {
		t.Fatalf("guest PBS links = %v, PBS-source hosts = %v", links, linkedHosts)
	}
}

// A connection label is chosen by the operator, not reported by PBS as its
// machine identity. It must not steal a corroborated guest Agent link merely
// because an unrelated Agent happens to have that hostname.
func TestIngestSnapshotPBSGuestLinkIgnoresConnectionLabelCollision(t *testing.T) {
	now := time.Now().UTC()
	registry := NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID: "unrelated-agent", Hostname: "backup-one", Status: "online", LastSeen: now,
				Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{Device: "/dev/sda", Serial: "UNRELATED-DISK"}}},
			},
			{
				ID: "guest-agent", Hostname: "guest-one", LinkedVMID: "pve:node:103", Status: "online", LastSeen: now,
				Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{Device: "/dev/sda", Serial: "GUEST-DISK"}}},
			},
		},
		VMs: []models.VM{{
			ID: "pve:node:103", VMID: 103, Instance: "pve", Node: "node", Status: "running",
			IPAddresses: []string{"10.2.0.13"}, LastSeen: now,
		}},
		PBSInstances: []models.PBSInstance{{
			ID: "pbs-one", Name: "backup-one", Host: "https://10.2.0.13:8007", Status: "online", LastSeen: now,
		}},
	})
	var linkedAgentID string
	for _, resource := range registry.ListForPresentation() {
		if resource.Type == ResourceTypePBS && resource.PBS != nil {
			linkedAgentID = resource.PBS.LinkedAgentID
		}
		if resource.Agent != nil && resource.Agent.AgentID == "unrelated-agent" && containsDataSource(resource.Sources, SourcePBS) {
			t.Fatalf("connection label attached unrelated Agent: %+v", resource)
		}
		if resource.PhysicalDisk != nil {
			switch resource.PhysicalDisk.Serial {
			case "UNRELATED-DISK":
				if containsDataSource(resource.Sources, SourcePBS) {
					t.Fatalf("connection label attached unrelated SMART disk: %+v", resource)
				}
			case "GUEST-DISK":
				if !containsDataSource(resource.Sources, SourcePBS) {
					t.Fatalf("corroborated guest SMART disk lost PBS source: %+v", resource)
				}
			}
		}
	}
	if linkedAgentID != "guest-agent" {
		t.Fatalf("PBS linked agent = %q, want corroborated guest-agent", linkedAgentID)
	}
}

func TestIngestSnapshotPBSConnectionLabelAloneDoesNotLinkAgent(t *testing.T) {
	now := time.Now().UTC()
	registry := NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		Hosts: []models.Host{{ID: "unrelated-agent", Hostname: "backup-one", Status: "online", LastSeen: now}},
		PBSInstances: []models.PBSInstance{{
			ID: "pbs-one", Name: "backup-one", Host: "https://10.2.0.13:8007", Status: "online", LastSeen: now,
		}},
	})
	for _, resource := range registry.ListForPresentation() {
		if resource.Type == ResourceTypePBS && resource.PBS != nil && resource.PBS.LinkedAgentID != "" {
			t.Fatalf("label-only PBS gained Agent link %q", resource.PBS.LinkedAgentID)
		}
		if resource.Agent != nil && containsDataSource(resource.Sources, SourcePBS) {
			t.Fatalf("label-only Agent gained PBS source: %+v", resource)
		}
	}
}

func TestIngestSnapshotPBSGuestIPAssociationFailsClosed(t *testing.T) {
	now := time.Now().UTC()
	base := func() models.StateSnapshot {
		return models.StateSnapshot{
			Hosts:        []models.Host{{ID: "agent-one", Hostname: "guest-one", LinkedVMID: "pve:node:103", LastSeen: now}},
			VMs:          []models.VM{{ID: "pve:node:103", VMID: 103, Name: "pbs-vm-one", Instance: "pve", Node: "node", Status: "running", IPAddresses: []string{"10.2.0.13"}, LastSeen: now}},
			PBSInstances: []models.PBSInstance{{ID: "pbs-one", Name: "backup-one", Host: "https://10.2.0.13:8007", LastSeen: now}},
		}
	}
	for _, test := range []struct {
		name   string
		mutate func(*models.StateSnapshot)
	}{
		{"same-ip-second-guest", func(s *models.StateSnapshot) {
			s.VMs = append(s.VMs, models.VM{ID: "pve:node:204", VMID: 204, Name: "unrelated", Instance: "pve", Node: "node", Status: "running", IPAddresses: []string{"10.2.0.13"}, LastSeen: now})
		}},
		{"second-agent-on-guest", func(s *models.StateSnapshot) {
			s.Hosts = append(s.Hosts, models.Host{ID: "agent-two", Hostname: "other", LinkedVMID: "pve:node:103", LastSeen: now})
		}},
		{"link-to-other-guest", func(s *models.StateSnapshot) {
			s.Hosts[0].LinkedVMID = "pve:node:999"
		}},
		{"nonunique-endpoint", func(s *models.StateSnapshot) {
			s.PBSInstances[0].Host = "https://127.0.0.1:8007"
			s.VMs[0].IPAddresses = []string{"127.0.0.1"}
		}},
		{"stopped-guest", func(s *models.StateSnapshot) {
			s.VMs[0].Status = "stopped"
		}},
		{"stale-guest", func(s *models.StateSnapshot) {
			s.VMs[0].LastSeen = now.Add(-10 * time.Minute)
		}},
		{"stale-agent", func(s *models.StateSnapshot) {
			s.Hosts[0].LastSeen = now.Add(-10 * time.Minute)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := base()
			test.mutate(&snapshot)
			registry := NewRegistry(nil)
			registry.IngestSnapshot(snapshot)
			for _, resource := range registry.ListForPresentation() {
				if resource.Type == ResourceTypePBS && resource.PBS != nil && resource.PBS.LinkedAgentID != "" {
					t.Fatalf("ambiguous PBS gained agent link %q", resource.PBS.LinkedAgentID)
				}
				if resource.Type == ResourceTypeAgent && containsDataSource(resource.Sources, SourcePBS) {
					t.Fatalf("ambiguous host gained PBS source: %+v", resource)
				}
			}
		})
	}
}

func containsPlatformScope(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

// Direct PBS/Agent correlation must use the same usable machine addresses as
// provider-link inference, not host-local addresses repeated on every server.
func TestPBSDirectHostLinkRejectsUnsafeObservations(t *testing.T) {
	now := time.Now().UTC()
	for _, test := range []struct {
		name     string
		endpoint string
		mutate   func(*models.Host)
	}{
		{"loopback-report", "127.0.0.1", func(h *models.Host) { h.ReportIP = "127.0.0.1" }},
		{"ipv6-loopback", "[::1]", func(h *models.Host) { h.ReportIP = "::1" }},
		{"unspecified-report", "0.0.0.0", func(h *models.Host) { h.ReportIP = "0.0.0.0" }},
		{"link-local-report", "169.254.2.3", func(h *models.Host) { h.ReportIP = "169.254.2.3" }},
		{"multicast-report", "224.0.0.2", func(h *models.Host) { h.ReportIP = "224.0.0.2" }},
		{"loopback-interface", "127.0.0.1", func(h *models.Host) {
			h.NetworkInterfaces = []models.HostNetworkInterface{{Name: "lo", Addresses: []string{"127.0.0.1/8"}}}
		}},
		{"docker-interface", "172.17.0.1", func(h *models.Host) {
			h.NetworkInterfaces = []models.HostNetworkInterface{{Name: "docker0", Addresses: []string{"172.17.0.1/16"}}}
		}},
		{"docker-generated-bridge", "172.22.0.1", func(h *models.Host) {
			h.NetworkInterfaces = []models.HostNetworkInterface{{Name: "br-abcdef123456", Addresses: []string{"172.22.0.1/16"}}}
		}},
		{"stale-report", "10.2.0.13", func(h *models.Host) { h.LastSeen = now.Add(-10 * time.Minute) }},
		{"missing-report-time", "10.2.0.13", func(h *models.Host) { h.LastSeen = time.Time{} }},
		{"missing-agent-id", "10.2.0.13", func(h *models.Host) { h.ID = "" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			host := models.Host{
				ID: "unrelated-agent", Hostname: "unrelated-host", ReportIP: "10.2.0.13",
				Status: "online", LastSeen: now,
				Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{Device: "/dev/sda", Serial: "UNRELATED-DISK"}}},
			}
			test.mutate(&host)
			registry := NewRegistry(nil)
			registry.IngestSnapshot(models.StateSnapshot{
				Hosts: []models.Host{host},
				PBSInstances: []models.PBSInstance{{
					ID: "pbs-service", Name: "backup", Host: "https://" + test.endpoint + ":8007",
					Status: "online", LastSeen: now,
				}},
			})
			for _, resource := range registry.ListForPresentation() {
				if resource.PBS != nil && resource.PBS.LinkedAgentID != "" {
					t.Errorf("unsafe observation linked PBS to %q", resource.PBS.LinkedAgentID)
				}
				if (resource.Agent != nil || resource.PhysicalDisk != nil) && containsDataSource(resource.Sources, SourcePBS) {
					t.Errorf("unsafe observation lent PBS ownership to %s (%s)", resource.ID, resource.Type)
				}
			}
		})
	}
}

// Corroboration stays possible for ordinary private management networks,
// explicit multi-NIC report-IP hints, and PBS-reported machine hostnames.
func TestPBSDirectHostLinkPreservesManagementEvidence(t *testing.T) {
	now := time.Now().UTC()
	for _, test := range []struct {
		name     string
		endpoint string
		nodeName string
		host     models.Host
	}{
		{"private-report", "10.2.0.13", "", models.Host{Hostname: "different", ReportIP: "10.2.0.13"}},
		{"interface-without-hostname", "10.2.0.13", "", models.Host{NetworkInterfaces: []models.HostNetworkInterface{{Name: "eth0", Addresses: []string{"10.2.0.13/24"}}}}},
		{"172-management-report", "172.22.0.1", "", models.Host{Hostname: "different", ReportIP: "172.22.0.1"}},
		{"management-bridge", "172.22.0.1", "", models.Host{Hostname: "different", NetworkInterfaces: []models.HostNetworkInterface{{Name: "vmbr0", Addresses: []string{"172.22.0.1/24"}}}}},
		{"custom-management-bridge", "172.22.0.1", "", models.Host{Hostname: "different", NetworkInterfaces: []models.HostNetworkInterface{{Name: "br-management", Addresses: []string{"172.22.0.1/24"}}}}},
		{"ipv6-management", "[fd00::13]", "", models.Host{Hostname: "different", NetworkInterfaces: []models.HostNetworkInterface{{Name: "eth0", Addresses: []string{"fd00::13/64"}}}}},
		{"reported-node-name", "10.2.0.13", "backup-host", models.Host{Hostname: "backup-host.local"}},
		{"endpoint-hostname", "backup-host.local", "", models.Host{Hostname: "backup-host"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			host := test.host
			host.ID, host.LastSeen = "host-agent", now
			instance := models.PBSInstance{Host: "https://" + test.endpoint + ":8007", NodeName: test.nodeName, LastSeen: now}
			if got := uniquePBSHostAgent(instance, []models.Host{host}, nil); got == nil || got.ID != host.ID {
				t.Fatalf("usable management evidence lost PBS Agent link: %+v", got)
			}
		})
	}
}

// A previously observed address may be reused. Registry replacement must
// withdraw the old host link and PBS disk membership, then safely select a
// fresh unique replacement without changing the service's History target.
func TestPBSDirectHostLinkWithdrawsAndReplacesStaleAgent(t *testing.T) {
	now := time.Now().UTC()
	host := models.Host{
		ID: "old-agent", Hostname: "backup-host", ReportIP: "10.2.0.13", LastSeen: now,
		Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{Device: "/dev/sda", Serial: "OLD-DISK"}}},
	}
	snapshot := models.StateSnapshot{
		Hosts:        []models.Host{host},
		PBSInstances: []models.PBSInstance{{ID: "pbs-service", Host: "https://10.2.0.13:8007", LastSeen: now}},
	}
	adapter := NewMonitorAdapter(NewRegistry(nil))
	for _, phase := range []string{"fresh", "stale", "replaced", "ambiguous"} {
		switch phase {
		case "stale":
			snapshot.PBSInstances[0].LastSeen = now.Add(10 * time.Minute)
		case "replaced":
			replacement := host
			replacement.ID, replacement.Hostname = "new-agent", "replacement-host"
			replacement.LastSeen = snapshot.PBSInstances[0].LastSeen
			replacement.Sensors = models.HostSensorSummary{SMART: []models.HostDiskSMART{{Device: "/dev/sda", Serial: "NEW-DISK"}}}
			snapshot.Hosts = append(snapshot.Hosts, replacement)
		case "ambiguous":
			snapshot.Hosts[0].LastSeen = snapshot.PBSInstances[0].LastSeen
		}
		adapter.replaceRegistry(snapshot, nil)
		registry := adapter.currentRegistry()
		want := ""
		if phase == "fresh" {
			want = "old-agent"
		} else if phase == "replaced" {
			want = "new-agent"
		}
		for _, resource := range registry.List() {
			if resource.PBS != nil {
				if resource.PBS.LinkedAgentID != want {
					t.Errorf("%s: PBS link = %q, want %q", phase, resource.PBS.LinkedAgentID, want)
				}
				if got := registry.MetricsTarget(resource.ID); got == nil || *got != (MetricsTarget{ResourceType: "agent", ResourceID: "pbs-service"}) {
					t.Errorf("%s: PBS service target changed: %+v", phase, got)
				}
			}
			if resource.Agent != nil {
				if got := containsDataSource(resource.Sources, SourcePBS); got != (resource.Agent.AgentID == want) {
					t.Errorf("%s: Agent %s PBS membership = %t", phase, resource.Agent.AgentID, got)
				}
			}
			if resource.PhysicalDisk != nil {
				wantMembership := phase == "fresh" && resource.PhysicalDisk.Serial == "OLD-DISK" ||
					phase == "replaced" && resource.PhysicalDisk.Serial == "NEW-DISK"
				if got := containsDataSource(resource.Sources, SourcePBS); got != wantMembership {
					t.Errorf("%s: disk %s PBS membership = %t", phase, resource.PhysicalDisk.Serial, got)
				}
			}
		}
	}
}

// Mirrors #1723's two PVE-hosted PBS machines and one non-PVE machine among
// nine Agents. The non-PVE case supplies observed interface-IP evidence: the
// test is not a claim that the reporter's VirtualBox Agent supplies that IP.
func TestPBSHostHistoryThreeServerTopology(t *testing.T) {
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{}
	for index := 1; index <= 9; index++ {
		host := models.Host{
			ID: fmt.Sprintf("agent-%d", index), Hostname: fmt.Sprintf("machine-%d", index), LastSeen: now,
			CPUUsage: float64(index * 10), Memory: models.Memory{Total: 1000, Used: int64(index * 100), Usage: float64(index * 10)},
			Disks:             []models.Disk{{Device: "/dev/sda", Total: 1000, Used: int64(index * 100), Usage: float64(index * 10)}},
			NetworkInterfaces: []models.HostNetworkInterface{{Name: "eth0", RXBytes: uint64(index * 11), TXBytes: uint64(index * 12)}},
			NetInRate:         float64(index * 11), NetOutRate: float64(index * 12),
			DiskReadRate: float64(index * 13), DiskWriteRate: float64(index * 14),
			DiskIO:  []models.DiskIO{{Device: "sda", ReadBytes: uint64(index * 13), WriteBytes: uint64(index * 14)}},
			Sensors: models.HostSensorSummary{SMART: []models.HostDiskSMART{{Device: "/dev/sda", Serial: fmt.Sprintf("DISK-%d", index)}}},
		}
		if index <= 2 {
			host.LinkedVMID = fmt.Sprintf("pve:node:%d", index+100)
			snapshot.VMs = append(snapshot.VMs, models.VM{
				ID: host.LinkedVMID, VMID: index + 100, Instance: "pve", Node: "node", Name: host.Hostname,
				Status: "running", IPAddresses: []string{fmt.Sprintf("10.2.0.%d", index)}, LastSeen: now,
			})
		} else if index == 3 {
			host.NetworkInterfaces[0].Addresses = []string{"10.2.0.3/24"}
		}
		snapshot.Hosts = append(snapshot.Hosts, host)
		if index <= 3 {
			snapshot.PBSInstances = append(snapshot.PBSInstances, models.PBSInstance{
				ID: fmt.Sprintf("pbs-%d", index), Name: fmt.Sprintf("backup-%d", index), Host: fmt.Sprintf("https://10.2.0.%d:8007", index),
				Status: "online", LastSeen: now, CPU: float64(index), Memory: float64(index * 2),
			})
		}
	}
	registry := NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	services, linkedHosts, linkedDisks := 0, 0, 0
	for _, resource := range registry.ListForPresentation() {
		if resource.PBS != nil {
			services++
			var index int
			// Registry order is not identity. Extract the fixture's stable ID.
			if _, err := fmt.Sscanf(resource.PBS.InstanceID, "pbs-%d", &index); err != nil {
				t.Fatal(err)
			}
			if resource.PBS.LinkedAgentID != fmt.Sprintf("agent-%d", index) {
				t.Errorf("%s linked to %q", resource.PBS.InstanceID, resource.PBS.LinkedAgentID)
			}
			if got := registry.MetricsTarget(resource.ID); got == nil || *got != (MetricsTarget{ResourceType: "agent", ResourceID: resource.PBS.InstanceID}) {
				t.Errorf("service target replaced with a host target: %+v", got)
			}
		}
		if resource.Agent != nil && containsDataSource(resource.Sources, SourcePBS) {
			linkedHosts++
			var index int
			if _, err := fmt.Sscanf(resource.Agent.AgentID, "agent-%d", &index); err != nil || index < 1 || index > 3 {
				t.Fatalf("unrelated Agent gained PBS membership: %s", resource.Agent.AgentID)
			}
			if got := registry.MetricsTarget(resource.ID); got == nil || *got != (MetricsTarget{ResourceType: "agent", ResourceID: resource.Agent.AgentID}) {
				t.Errorf("host target lost source-native Agent identity: %+v", got)
			}
			if resource.Metrics == nil {
				t.Fatalf("%s lost all Agent metrics", resource.Agent.AgentID)
			}
			for name, metric := range map[string]*MetricValue{"memory": resource.Metrics.Memory, "disk": resource.Metrics.Disk} {
				if metric == nil || metric.Source != SourceAgent || metric.Used == nil || *metric.Used != int64(index*100) {
					t.Errorf("%s %s lost distinct Agent usage: %+v", resource.Agent.AgentID, name, metric)
				}
			}
			for _, observation := range []struct {
				name   string
				metric *MetricValue
				want   float64
			}{
				{"CPU", resource.Metrics.CPU, float64(index * 10)},
				{"network in", resource.Metrics.NetIn, float64(index * 11)},
				{"network out", resource.Metrics.NetOut, float64(index * 12)},
				{"disk read", resource.Metrics.DiskRead, float64(index * 13)},
				{"disk write", resource.Metrics.DiskWrite, float64(index * 14)},
			} {
				if metric := observation.metric; metric == nil || metric.Source != SourceAgent || metric.Value != observation.want {
					t.Errorf("%s %s lost distinct Agent telemetry (want %g): %+v", resource.Agent.AgentID, observation.name, observation.want, metric)
				}
			}
		}
		if resource.PhysicalDisk != nil && containsDataSource(resource.Sources, SourcePBS) {
			linkedDisks++
			if resource.PhysicalDisk.Serial != "DISK-1" && resource.PhysicalDisk.Serial != "DISK-2" && resource.PhysicalDisk.Serial != "DISK-3" {
				t.Errorf("unrelated disk gained PBS membership: %s", resource.PhysicalDisk.Serial)
			}
		}
	}
	if services != 3 || linkedHosts != 3 || linkedDisks != 3 {
		t.Fatalf("three-server topology: services=%d linked hosts=%d linked disks=%d", services, linkedHosts, linkedDisks)
	}
}

func TestStatusFromStorageStateCoversPBSAndPVEVocabulary(t *testing.T) {
	// PVE storage and PBS datastores both report "available"/"unavailable";
	// the mapping has to be shared so neither surface falls back to unknown.
	cases := []struct {
		state string
		want  ResourceStatus
		known bool
	}{
		{state: "available", want: StatusOnline, known: true},
		{state: "ACTIVE", want: StatusOnline, known: true},
		{state: "degraded", want: StatusWarning, known: true},
		{state: "unavailable", want: StatusOffline, known: true},
		{state: "", want: StatusUnknown, known: false},
		{state: "mystery", want: StatusUnknown, known: false},
	}
	for _, tc := range cases {
		got, known := statusFromStorageState(tc.state)
		if got != tc.want || known != tc.known {
			t.Fatalf("statusFromStorageState(%q) = (%q, %v), want (%q, %v)", tc.state, got, known, tc.want, tc.known)
		}
	}
}
