package monitoring

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"testing"
	"time"
)

func TestLinkedGuestDockerReportKeepsLastCompleteInventory(t *testing.T) {
	for _, kind := range []string{"vm", "lxc"} {
		t.Run(kind, func(t *testing.T) {
			m := newTestMonitor(t)
			store := unifiedresources.NewMemoryStore()
			adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(store))
			m.SetResourceStore(adapter)
			now := time.Now().UTC()
			token := &config.APITokenRecord{ID: "home-token"}
			if kind == "vm" {
				m.state.UpdateVMsForInstance("home", []models.VM{{ID: "home-pmx1-100", Instance: "home", Node: "pmx1", VMID: 100, Name: "docker1", Status: "running", LastSeen: now}})
			} else {
				m.state.UpdateContainersForInstance("home", []models.Container{{ID: "home-pmx1-100", Instance: "home", Node: "pmx1", VMID: 100, Name: "docker1", Status: "running", Type: "lxc", LastSeen: now}})
			}
			if _, err := m.ApplyHostReport(agentshost.Report{Agent: agentshost.AgentInfo{ID: "home-agent"}, Host: agentshost.HostInfo{ID: "home-agent", MachineID: "home-machine", Hostname: "docker1"}, Timestamp: now}, token); err != nil {
				t.Fatal(err)
			}
			report := agentsdocker.Report{Agent: agentsdocker.AgentInfo{ID: "home-agent"}, Host: agentsdocker.HostInfo{MachineID: "home-machine", Hostname: "docker1", DockerVersion: "27.0.0"}, Containers: []agentsdocker.Container{{ID: "app-1", Name: "payments", State: "running"}}, Timestamp: now}
			first, err := m.ApplyDockerReport(report, token)
			if err != nil {
				t.Fatal(err)
			}
			var guestID string
			for _, r := range adapter.GetAll() {
				if r.Type == unifiedresources.ResourceTypeVM || r.Type == unifiedresources.ResourceTypeSystemContainer {
					guestID = r.ID
				}
			}
			hostID := adapter.DockerHosts()[0].ID()
			if err := store.AddLink(unifiedresources.ResourceLink{ResourceA: guestID, ResourceB: hostID, PrimaryID: hostID}); err != nil {
				t.Fatal(err)
			}
			m.state.UpdateVMsForInstance("remote", []models.VM{{ID: "remote-pmx1-100", Instance: "remote", Node: "pmx1", VMID: 100, Name: "docker1", Status: "running", LastSeen: now}})
			adapter.PopulateFromSnapshot(m.state.GetSnapshot())
			for _, complete := range []bool{false, true} {
				report.Timestamp = report.Timestamp.Add(time.Second)
				report.InventoryComplete = &complete
				if complete {
					report.Containers = append(report.Containers, agentsdocker.Container{ID: "app-2", Name: "worker", State: "running"})
				}
				got, err := m.ApplyDockerReport(report, token)
				if err != nil {
					t.Fatal(err)
				}
				want := 1
				if complete {
					want = 2
				}
				if got.ID != first.ID || len(got.Containers) != want {
					t.Fatalf("complete=%t runtime=%s containers=%d want %s/%d", complete, got.ID, len(got.Containers), first.ID, want)
				}
				views := adapter.DockerHosts()
				if len(views) != 1 || views[0].ID() != guestID || views[0].HostSourceID() != first.ID || len(adapter.DockerContainers()) != want {
					t.Fatalf("complete=%t read-state hosts=%+v containers=%d", complete, views, len(adapter.DockerContainers()))
				}
			}
		})
	}
}
