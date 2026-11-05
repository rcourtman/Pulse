package unifiedresources

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"testing"
	"time"
)

// An explicit agent/guest link changes the canonical row's shape, not whether
// the linked machine runs Docker. The runtime source key remains its own.
func TestDockerGuestLinkKeepsRuntimeReadState(t *testing.T) {
	for _, kind := range []string{"vm", "lxc"} {
		for _, agentPrimary := range []bool{false, true} {
			direction := "guest-primary"
			if agentPrimary {
				direction = "agent-primary"
			}
			t.Run(kind+"/"+direction, func(t *testing.T) {
				now := time.Now().UTC()
				snapshot := models.StateSnapshot{LastUpdate: now,
					Hosts:       []models.Host{{ID: "home-agent", MachineID: "home-machine", Hostname: "docker1", Status: "online", LastSeen: now}},
					DockerHosts: []models.DockerHost{{ID: "home-runtime", AgentID: "home-agent", MachineID: "home-machine", Hostname: "docker1", DockerVersion: "27.0.0", Status: "online", LastSeen: now, Containers: []models.DockerContainer{{ID: "app-1", Name: "app", State: "running"}}}},
				}
				if kind == "vm" {
					snapshot.VMs = []models.VM{{ID: "home-pmx1-100", VMID: 100, Name: "docker1", Node: "pmx1", Instance: "home", Status: "running", LastSeen: now}}
				} else {
					snapshot.Containers = []models.Container{{ID: "home-pmx1-100", VMID: 100, Name: "docker1", Node: "pmx1", Instance: "home", Status: "running", Type: "lxc", LastSeen: now}}
				}
				store := NewMemoryStore()
				adapter := NewMonitorAdapter(NewRegistry(store))
				adapter.PopulateFromSnapshot(snapshot)
				var guestID string
				for _, r := range adapter.GetAll() {
					if r.Type == ResourceTypeVM || r.Type == ResourceTypeSystemContainer {
						guestID = r.ID
					}
				}
				hosts := adapter.DockerHosts()
				if len(hosts) != 1 {
					t.Fatalf("initial hosts=%d", len(hosts))
				}
				hostID := hosts[0].ID()
				workloadID := adapter.DockerContainers()[0].ID()
				primary := guestID
				if agentPrimary {
					primary = hostID
				}
				if err := store.AddLink(ResourceLink{ResourceA: guestID, ResourceB: hostID, PrimaryID: primary, CreatedAt: now}); err != nil {
					t.Fatal(err)
				}
				check := func(t *testing.T, view ReadState, wantID string) {
					t.Helper()
					got := view.DockerHosts()
					if len(got) != 1 {
						t.Fatalf("runtime hosts=%d, want one", len(got))
					}
					if got[0].ID() != wantID || got[0].HostSourceID() != "home-runtime" || got[0].AgentID() != "home-agent" || got[0].DockerVersion() != "27.0.0" {
						t.Fatalf("runtime identity changed: %s source=%s agent=%s", got[0].ID(), got[0].HostSourceID(), got[0].AgentID())
					}
					workloads := view.DockerContainers()
					if len(workloads) != 1 || workloads[0].ID() != workloadID || workloads[0].ParentID() != wantID {
						t.Fatalf("runtime's container detached: %+v", workloads)
					}
				}
				for _, stage := range []string{"linked", "namesake", "repeated"} {
					t.Run(stage, func(t *testing.T) {
						if stage == "namesake" {
							snapshot.VMs = append(snapshot.VMs, models.VM{ID: "remote-pmx1-100", VMID: 100, Name: "docker1", Node: "pmx1", Instance: "remote", Status: "running", LastSeen: now})
						}
						adapter.PopulateFromSnapshot(snapshot)
						check(t, adapter, guestID)
						seeded := NewRegistry(store)
						seeded.IngestResources(adapter.GetAll())
						check(t, seeded, guestID)
					})
				}
				if err := store.AddExclusion(ResourceExclusion{ResourceA: guestID, ResourceB: hostID, CreatedAt: now.Add(time.Second)}); err != nil {
					t.Fatal(err)
				}
				adapter.PopulateFromSnapshot(snapshot)
				check(t, adapter, hostID)
			})
		}
	}
}
