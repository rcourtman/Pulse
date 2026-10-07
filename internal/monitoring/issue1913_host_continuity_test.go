package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// A reinstall can leave an earlier enrollment in continuity while only its
// successor reports. Hydrating that history must not replace the live agent
// in Infrastructure or turn its linked Proxmox node offline (#1913).
func TestHostContinuityPreservesLiveAgentAfterReenrollment(t *testing.T) {
	for _, linked := range []bool{false, true} {
		name := "standalone"
		if linked {
			name = "proxmox"
		}
		t.Run(name, func(t *testing.T) {
			now := time.Now().UTC()
			live := models.Host{
				ID: "current-enrollment", MachineID: "same-machine", Hostname: "home-pm",
				Platform: "linux", Status: "online", LastSeen: now, IntervalSeconds: 30,
				CPUUsage: 17, CPUCount: 8, AgentVersion: "6.4.1", TokenID: "current-token",
			}
			state := models.NewState()
			if linked {
				live.LinkedNodeID = "pve:home-pm"
				state.Nodes = []models.Node{{
					ID: live.LinkedNodeID, Name: live.Hostname, Instance: "home-pm",
					Host: "https://192.0.2.149:8006", Status: "online", LastSeen: now,
					LinkedAgentID: live.ID,
				}}
			}
			state.Hosts = []models.Host{live}
			continuity := config.NewHostContinuityStore(t.TempDir(), nil)
			for _, entry := range []config.HostContinuityEntry{
				{HostID: "retired-enrollment", MachineID: live.MachineID, Hostname: live.Hostname, Platform: "linux", LastSeen: now.Add(-time.Minute), TokenID: "retired-token"},
				{HostID: "absent-machine", MachineID: "other-machine", Hostname: "other-host", Platform: "linux", LastSeen: now.Add(-time.Hour)},
			} {
				if err := continuity.Upsert(entry); err != nil {
					t.Fatal(err)
				}
			}
			registry := unifiedresources.NewRegistry(nil)
			registry.IngestSnapshot(state.GetSnapshot())
			adapter := unifiedresources.NewMonitorAdapter(registry)
			m := &Monitor{state: state, resourceStore: adapter, hostContinuityStore: continuity}
			for round := 0; round < 3; round++ {
				hosts := m.HostsSnapshot()
				if len(hosts) != 2 {
					t.Fatalf("host count = %d, want live machine plus absent historical machine", len(hosts))
				}
				var foundLive, foundAbsent bool
				for _, host := range hosts {
					switch host.ID {
					case live.ID:
						foundLive = true
						if host.Status != "online" || host.CPUUsage != live.CPUUsage || host.TokenID != live.TokenID || host.LinkedNodeID != live.LinkedNodeID || !host.LastSeen.Equal(now) {
							t.Fatalf("round %d: history changed live observation: %+v", round, host)
						}
					case "absent-machine":
						foundAbsent = true
						if host.Status == "online" || !host.LastSeen.Equal(now.Add(-time.Hour)) {
							t.Fatalf("historical machine was presented as a live sighting: %+v", host)
						}
					default:
						t.Fatalf("round %d: retired identity replaced current enrollment: %s", round, host.ID)
					}
				}
				if !foundLive || !foundAbsent {
					t.Fatal("lost live observation or absent-machine continuity")
				}
				if linked {
					nodes := m.NodesSnapshot()
					if len(nodes) != 1 || nodes[0].Status != "online" || nodes[0].LinkedAgentID != live.ID {
						t.Fatalf("round %d: history changed Proxmox status or agent link: %+v", round, nodes)
					}
				}
			}
			if len(adapter.Hosts()) != 1 || adapter.Hosts()[0].AgentID() != live.ID {
				t.Fatal("read-time hydration mutated the live registry")
			}
			if len(continuity.RecentEntries(now.Add(-24*time.Hour))) != 2 {
				t.Fatal("read-time hydration deleted retained enrollment history")
			}
		})
	}
}

// Continuity hydration clones the registry through resource ingest, which
// applies operator links, while the rebuild ingests supplemental records after
// the snapshot. A link to a record-sourced guest (an agent inside a vSphere VM)
// must hold in the published inventory either way, or the state the websocket
// broadcasts and the resources API seeds from flips with an unrelated absent
// machine. A saved enrollment only fills an absent machine: when the linked
// agent itself is gone, its continuity row stays separate and the guest keeps
// what vSphere reports.
func TestManualLinkToSupplementalGuestHoldsWithAndWithoutContinuity(t *testing.T) {
	now := time.Now().UTC()
	guestAgent := models.Host{
		ID: "host-app-guest", MachineID: "machine-app-guest", Hostname: "app-guest",
		Platform: "linux", Status: "online", LastSeen: now, IntervalSeconds: 30,
	}
	vmRecords := []unifiedresources.IngestRecord{{
		SourceID: "vc-1:vm:vm-42",
		Resource: unifiedresources.Resource{
			Type:       unifiedresources.ResourceTypeVM,
			Technology: "vmware",
			Name:       "app-guest",
			Status:     unifiedresources.StatusOnline,
			LastSeen:   now,
			VMware:     &unifiedresources.VMwareData{ConnectionID: "vc-1", ManagedObjectID: "vm-42", EntityType: "vm"},
		},
	}}

	unlinked := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	unlinked.PopulateSnapshotAndSupplemental(
		models.StateSnapshot{Hosts: []models.Host{guestAgent}, LastUpdate: now},
		map[unifiedresources.DataSource][]unifiedresources.IngestRecord{unifiedresources.SourceVMware: vmRecords},
	)
	if len(unlinked.VMs()) != 1 || len(unlinked.Hosts()) != 1 {
		t.Fatalf("unlinked estate = %d VMs, %d agents, want one of each", len(unlinked.VMs()), len(unlinked.Hosts()))
	}
	vmID, agentID := unlinked.VMs()[0].ID(), unlinked.Hosts()[0].ID()

	for _, tc := range []struct {
		name       string
		reporting  bool
		continuity []config.HostContinuityEntry
	}{
		{name: "no-continuity", reporting: true},
		{
			name:      "absent-machine-continuity",
			reporting: true,
			continuity: []config.HostContinuityEntry{{
				HostID: "absent-machine", MachineID: "other-machine", Hostname: "other-host",
				Platform: "linux", LastSeen: now.Add(-time.Hour),
			}},
		},
		{
			name: "linked-agent-continuity-only",
			continuity: []config.HostContinuityEntry{{
				HostID: guestAgent.ID, MachineID: guestAgent.MachineID, Hostname: guestAgent.Hostname,
				Platform: "linux", LastSeen: now.Add(-10 * time.Minute),
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			links := unifiedresources.NewMemoryStore()
			if err := links.AddLink(unifiedresources.ResourceLink{ResourceA: vmID, ResourceB: agentID, PrimaryID: vmID}); err != nil {
				t.Fatalf("add link: %v", err)
			}
			state := models.NewState()
			if tc.reporting {
				state.Hosts = []models.Host{guestAgent}
			}
			m := &Monitor{state: state}
			if len(tc.continuity) > 0 {
				continuity := config.NewHostContinuityStore(t.TempDir(), nil)
				for _, entry := range tc.continuity {
					if err := continuity.Upsert(entry); err != nil {
						t.Fatal(err)
					}
				}
				m.hostContinuityStore = continuity
			}
			m.SetResourceStore(unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(links)))
			m.SetSupplementalRecordsProvider(unifiedresources.SourceVMware, &testSupplementalInventoryReadinessProvider{
				source:  unifiedresources.SourceVMware,
				records: vmRecords,
				settled: true,
				readyAt: now,
			})

			resources, _ := m.UnifiedResourceSnapshot()
			var guest, agent *unifiedresources.Resource
			for i := range resources {
				switch resources[i].ID {
				case agentID:
					agent = &resources[i]
				case vmID:
					guest = &resources[i]
				}
			}
			if guest == nil {
				t.Fatalf("linked VM %s missing from published inventory", vmID)
			}
			if tc.reporting {
				if agent != nil {
					t.Fatalf("linked agent %s still published standalone", agentID)
				}
				if guest.VMware == nil || guest.Agent == nil {
					t.Fatalf("published VM vmware=%t agent=%t, want the VM carrying its agent", guest.VMware != nil, guest.Agent != nil)
				}
				return
			}
			if agent == nil || agent.Status != unifiedresources.StatusOffline {
				t.Fatalf("saved enrollment row = %+v, want its own offline row", agent)
			}
			if guest.Agent != nil || guest.Status != unifiedresources.StatusOnline {
				t.Fatalf("published VM agent=%t status=%s, want vSphere's online VM untouched by the saved enrollment", guest.Agent != nil, guest.Status)
			}
		})
	}
}
