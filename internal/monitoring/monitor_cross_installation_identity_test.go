package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
)

// Start with a working cluster/agent, then add an independent connection. A
// display name does not grant permission to consolidate a discovered member
// hostname. These are source controls for all three #2681 surfaces, not a
// reproduction of the reporter's unknown configuration or native recovery.
func TestCrossInstallationIdentitySurvivesStandaloneAddition(t *testing.T) {
	for _, tc := range []struct {
		name, host string
		classified bool
		fqdnGuests bool
	}{
		{"unclassified distinct-address node", "https://203.0.113.10:8006", false, false},
		{"classified distinct-address node", "https://203.0.113.10:8006", true, false},
		{"unclassified member-hostname coincidence", "https://pmx1:8006", false, false},
		{"classified member-hostname coincidence", "https://pmx1:8006", true, false},
		{"unclassified FQDN connection and distinct guest domains", "https://pmx1.remote.example:8006", false, true},
		{"classified FQDN connection and distinct guest domains", "https://pmx1.remote.example:8006", true, true},
	} {
		name, classified := tc.name, tc.classified
		clusterName := ""
		if classified {
			clusterName = "Home"
		}
		t.Run(name, func(t *testing.T) {
			m := newTestMonitor(t)
			m.config = &config.Config{PVEInstances: []config.PVEInstance{
				{Name: "Home", Host: "https://home.example:8006", IsCluster: classified, ClusterName: clusterName, ClusterEndpoints: []config.ClusterEndpoint{{NodeName: "pmx1", Host: "https://pmx1:8006"}}},
			}}
			adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
			m.SetResourceStore(adapter)
			now := time.Now().UTC()
			backupAt := now.Add(-time.Hour)
			homeGuestName, remoteGuestName := "docker-vm", "docker-vm"
			if tc.fqdnGuests {
				homeGuestName, remoteGuestName = "docker-vm.home.example", "docker-vm.remote.example"
			}
			homeNodes := []models.Node{}
			for _, native := range []string{"pmx1", "pmx2", "pmx3"} {
				node := m.placeholderNodeForInstance("Home", &m.config.PVEInstances[0], native)
				node.Status = "online"
				node.ConnectionHealth = "healthy"
				node.LastSeen = now
				homeNodes = append(homeNodes, node)
			}
			m.state.UpdateNodesForInstance("Home", homeNodes)
			homeGuestID := makeGuestID("Home", "pmx1", 100)
			m.state.UpdateVMsForInstance("Home", []models.VM{{ID: homeGuestID, Instance: "Home", Node: "pmx1", VMID: 100, Name: homeGuestName, Status: "running", LastSeen: now}})
			m.state.UpdateStorageBackupsForInstance("Home", []models.StorageBackup{{Instance: "Home", Node: "pmx1", VMID: 100, Time: backupAt}})
			m.state.SyncGuestBackupTimes()
			adapter.PopulateFromSnapshot(m.state.GetSnapshot())
			token := &config.APITokenRecord{ID: "home-agent-token"}
			hostReport := agentshost.Report{
				Agent: agentshost.AgentInfo{ID: "home-agent", IntervalSeconds: 30},
				Host:  agentshost.HostInfo{ID: "home-machine", MachineID: "home-machine", Hostname: homeGuestName, Platform: "linux"}, Timestamp: now,
			}
			dockerReport := agentsdocker.Report{
				Agent:      agentsdocker.AgentInfo{ID: "home-agent", IntervalSeconds: 30},
				Host:       agentsdocker.HostInfo{MachineID: "home-machine", Hostname: homeGuestName, DockerVersion: "27.0.0", TotalCPU: 4},
				Containers: []agentsdocker.Container{{ID: "home-workload", Name: "app", State: "running"}}, Timestamp: now,
			}
			if _, err := m.ApplyHostReport(hostReport, token); err != nil {
				t.Fatal(err)
			}
			if _, err := m.ApplyDockerReport(dockerReport, token); err != nil {
				t.Fatal(err)
			}
			if len(adapter.DockerHosts()) != 1 || len(adapter.DockerContainers()) != 1 {
				t.Fatal("initial Docker report absent from read state")
			}
			homeCanonicalID := adapter.VMs()[0].ID()

			standalone := config.PVEInstance{Name: "Hetzner pmx1", Host: tc.host}
			m.config.PVEInstances = append(m.config.PVEInstances, standalone)
			remote := m.placeholderNodeForInstance(standalone.Name, &standalone, "pmx1")
			remote.Status = "offline"
			remote.ConnectionHealth = "unhealthy"
			remote.PendingUpdatesReason = "api_error"
			m.state.UpdateNodesForInstance(standalone.Name, []models.Node{remote})
			m.state.UpdateVMsForInstance(standalone.Name, []models.VM{{ID: makeGuestID(standalone.Name, "pmx1", 100), Instance: standalone.Name, Node: "pmx1", VMID: 100, Name: remoteGuestName, Status: "running", LastSeen: now}})
			m.normalizePVEConfigState()
			if len(m.config.PVEInstances) != 2 {
				t.Fatal("standalone connection retired on discovered hostname alone")
			}
			m.state.SyncGuestBackupTimes()
			adapter.PopulateFromSnapshot(m.state.GetSnapshot())

			// Hostname ambiguity may safely unbind an automatic association. It must
			// not discard the agent, its Docker inventory, or the original VM's data.
			hostReport.Timestamp = now.Add(time.Second)
			dockerReport.Timestamp = now.Add(time.Second)
			if _, err := m.ApplyHostReport(hostReport, token); err != nil {
				t.Fatal(err)
			}
			if _, err := m.ApplyDockerReport(dockerReport, token); err != nil {
				t.Fatal(err)
			}
			if len(adapter.Nodes()) != 4 {
				t.Fatalf("nodes=%d, want three home and one remote", len(adapter.Nodes()))
			}
			for _, node := range adapter.Nodes() {
				if node.Instance() == "Home" && node.ConnectionHealth() != "healthy" {
					t.Fatalf("remote error reached home node %s", node.SourceID())
				}
				if node.Instance() == standalone.Name && node.ConnectionHealth() != "unhealthy" {
					t.Fatal("remote failure hidden")
				}
			}
			if len(adapter.VMs()) != 2 {
				t.Fatalf("overlapping guests collapsed: %d", len(adapter.VMs()))
			}
			for _, vm := range adapter.VMs() {
				if vm.Instance() == "Home" && (vm.ID() != homeCanonicalID || !vm.LastBackup().Equal(backupAt)) {
					t.Fatal("original guest identity or backup was lost")
				}
				if vm.Instance() == standalone.Name && !vm.LastBackup().IsZero() {
					t.Fatal("home backup assigned to independent guest")
				}
			}
			guests, _ := buildGuestLookupsFromReadState(adapter, nil)
			if len(guests) != 2 {
				t.Fatal("backup lookup collapsed independent guests")
			}
			guestNodes := map[int]string{}
			populateGuestNodeMapFromReadState(adapter, "Home", guestNodes)
			if len(guestNodes) != 1 || guestNodes[100] != "pmx1" {
				t.Fatal("backup node lookup lost home scope")
			}
			if tc.fqdnGuests {
				hosts := m.state.GetSnapshot().Hosts
				if len(hosts) != 1 || hosts[0].LinkedVMID != homeGuestID {
					t.Fatalf("distinct guest FQDNs erased or redirected the original agent link: %+v", hosts)
				}
			}
			if len(adapter.DockerHosts()) != 1 || len(adapter.DockerContainers()) != 1 {
				t.Fatal("Docker monitoring disappeared after independent guest addition")
			}
			if adapter.DockerContainers()[0].Name() != "app" {
				t.Fatal("Docker workload identity changed")
			}
		})
	}
}
