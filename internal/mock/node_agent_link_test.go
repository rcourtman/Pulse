package mock

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Pulse sends a guest start only to an agent it can confirm runs on the
// guest's node: the agent carries the node's name, no other node has that
// name, and one of the agent's interfaces holds an address Proxmox lists for
// that node alone. Mock data has to be shaped like a real node and its agent,
// or every mock node reads as one whose agent cannot be confirmed.
func TestMockNodesAndTheirAgentsLookLikeOneMachine(t *testing.T) {
	state := buildFixtureState(DefaultConfig)
	if len(state.Nodes) == 0 {
		t.Fatal("no mock nodes")
	}
	hosts := make(map[string]models.Host, len(state.Hosts))
	for _, host := range state.Hosts {
		hosts[host.ID] = host
	}
	names := make(map[string]string)
	owners := make(map[string]string)
	for _, node := range state.Nodes {
		if other, taken := names[strings.ToLower(node.Name)]; taken {
			t.Fatalf("nodes %s and %s share the name %q", other, node.ID, node.Name)
		}
		names[strings.ToLower(node.Name)] = node.ID

		addresses := make(map[string]bool)
		for _, nic := range node.NetworkInterfaces {
			for _, address := range nic.Addresses {
				normalized := models.NodeLinkAddress(address)
				if normalized == "" {
					continue
				}
				if other, taken := owners[normalized]; taken && other != node.ID {
					t.Fatalf("nodes %s and %s both list %s", other, node.ID, normalized)
				}
				owners[normalized] = node.ID
				addresses[normalized] = true
			}
		}
		if len(addresses) == 0 {
			t.Fatalf("node %s lists no address", node.ID)
		}

		agent, ok := hosts[node.LinkedAgentID]
		if !ok || agent.LinkedNodeID != node.ID {
			t.Fatalf("node %s has no agent linked back to it: %q", node.ID, node.LinkedAgentID)
		}
		if !strings.EqualFold(agent.Hostname, node.Name) {
			t.Fatalf("node %s is named %q and its agent reports %q", node.ID, node.Name, agent.Hostname)
		}
		shared := false
		for _, nic := range agent.NetworkInterfaces {
			for _, address := range nic.Addresses {
				if addresses[models.NodeLinkAddress(address)] {
					shared = true
				}
			}
		}
		if !shared {
			t.Fatalf("the agent on node %s reports none of the node's addresses: %+v", node.ID, agent.NetworkInterfaces)
		}
	}
}

func TestMockNodeBridgeAddressesAreStablePerName(t *testing.T) {
	if first, again := mockNodeBridgeAddresses("pve7"), mockNodeBridgeAddresses("pve7"); strings.Join(first, ",") != strings.Join(again, ",") {
		t.Fatalf("pve7 got %v then %v", first, again)
	}
	seen := make(map[string]string)
	for _, name := range []string{"pve1", "pve2", "pve50", "pve229", "pve230", "delly", "minipc", "node-a", "node-b"} {
		addresses := mockNodeBridgeAddresses(name)
		if len(addresses) != 2 || models.NodeLinkAddress(addresses[0]) == "" || models.NodeLinkAddress(addresses[1]) == "" {
			t.Fatalf("%s got %v", name, addresses)
		}
		for _, address := range addresses {
			if other, taken := seen[address]; taken {
				t.Fatalf("%s and %s share %s", other, name, address)
			}
			seen[address] = name
		}
	}
}
