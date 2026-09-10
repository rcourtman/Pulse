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
