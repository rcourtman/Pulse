package unifiedresources

import (
	"encoding/json"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// A cluster label is a projection of provider association, not subnet membership.
// Keep the public discriminator covered: link hints themselves are not JSON fields.
func TestNASClusterProjectionPublicEvidence(t *testing.T) {
	for _, linked := range []bool{false, true} {
		name := "unlinked-shared-bridge"
		if linked {
			name = "retained-reciprocal-link"
		}
		t.Run(name, func(t *testing.T) {
			node := models.Node{ID: "pve-node", Name: "pve", Instance: "cluster", ClusterName: "cluster", IsClusterMember: true, Host: "https://192.0.2.10:8006", NetworkInterfaces: []models.HostNetworkInterface{{Name: "docker0", Addresses: []string{"172.17.0.1/16"}}}}
			host := models.Host{ID: "nas-agent", MachineID: "nas-machine", Hostname: "nas", NetworkInterfaces: []models.HostNetworkInterface{{Name: "eth0", Addresses: []string{"198.51.100.20/24"}}, {Name: "docker0", Addresses: []string{"172.17.0.1/16"}}}}
			if linked {
				host.LinkedNodeID = node.ID
				host.NodeLinkSource = "manual"
				node.LinkedAgentID = host.ID
			}
			registry := NewRegistry(NewMemoryStore())
			registry.IngestSnapshot(models.StateSnapshot{Nodes: []models.Node{node}, Hosts: []models.Host{host}})
			found := false
			for _, resource := range registry.ListForPresentation() {
				if resource.Agent == nil || resource.Agent.AgentID != host.ID {
					continue
				}
				found = true
				if (resource.Proxmox != nil) != linked {
					t.Fatalf("provider projection linked=%v: %+v", linked, resource.Proxmox)
				}
				if !linked && resource.Identity.ClusterName != "" {
					t.Fatalf("unlinked NAS inherited cluster %q", resource.Identity.ClusterName)
				}
				raw, err := json.Marshal(resource)
				if err != nil {
					t.Fatal(err)
				}
				var public map[string]any
				if err := json.Unmarshal(raw, &public); err != nil {
					t.Fatal(err)
				}
				agent := public["agent"].(map[string]any)
				if _, exposed := agent["linkedNodeId"]; exposed {
					t.Fatal("internal link hint unexpectedly exposed")
				}
				if linked {
					provider := public["proxmox"].(map[string]any)
					if provider["nodeName"] != node.Name || provider["clusterName"] != node.ClusterName {
						t.Fatalf("missing public association discriminator: %v", provider)
					}
				}
			}
			if !found {
				t.Fatal("NAS agent missing from presentation")
			}
		})
	}
}
