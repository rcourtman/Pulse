package unifiedresources

import (
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestRefreshHostSocketPeerRelationships_MatchesMonitoredPeersOnly(t *testing.T) {
	now := time.Now().UTC()
	rr := NewRegistry(nil)

	state := models.StateSnapshot{
		Hosts: []models.Host{
			{
				ID:       "host-app",
				Hostname: "app-node",
				Status:   "online",
				LastSeen: now,
				NetworkInterfaces: []models.HostNetworkInterface{
					{Name: "eth0", Addresses: []string{"192.168.10.10/24"}},
				},
				ActivePeers: []models.HostNetworkPeer{
					// Outbound to monitored db-node on two ports -> collapsed into 1 edge
					{LocalIP: "192.168.10.10", RemoteIP: "192.168.10.20", RemotePort: 5432, Protocol: "tcp"},
					{LocalIP: "192.168.10.10", RemoteIP: "192.168.10.20", RemotePort: 6379, Protocol: "tcp"},
					// Outbound to unmonitored LAN IP -> must NOT emit an edge
					{LocalIP: "192.168.10.10", RemoteIP: "192.168.10.99", RemotePort: 80, Protocol: "tcp"},
					// Outbound to ambiguous IP shared by two VMs -> must NOT emit an edge
					{LocalIP: "192.168.10.10", RemoteIP: "192.168.10.50", RemotePort: 443, Protocol: "tcp"},
				},
			},
			{
				ID:       "host-db",
				Hostname: "db-node",
				Status:   "online",
				LastSeen: now,
				NetworkInterfaces: []models.HostNetworkInterface{
					{Name: "eth0", Addresses: []string{"192.168.10.20/24"}},
				},
				ActivePeers: []models.HostNetworkPeer{
					// Inbound from app-node on :5432 -> collapses with app-node's outbound edge!
					{LocalIP: "192.168.10.20", LocalPort: 5432, RemoteIP: "192.168.10.10", Protocol: "tcp", Inbound: true},
				},
			},
		},
		VMs: []models.VM{
			{
				ID:          "pve1:node1:101",
				VMID:        101,
				Name:        "dup-vm-a",
				Node:        "node1",
				Instance:    "pve1",
				Status:      "running",
				LastSeen:    now,
				IPAddresses: []string{"192.168.10.50"},
				Accelerators: []string{
					"GPU",
				},
			},
			{
				ID:          "pve1:node1:102",
				VMID:        102,
				Name:        "dup-vm-b",
				Node:        "node1",
				Instance:    "pve1",
				Status:      "running",
				LastSeen:    now,
				IPAddresses: []string{"192.168.10.50"},
			},
		},
	}

	rr.IngestSnapshot(state)

	all := rr.List()
	byName := make(map[string]Resource, len(all))
	for _, r := range all {
		byName[r.Name] = r
	}

	app := byName["app-node"]
	db := byName["db-node"]
	vmA := byName["dup-vm-a"]

	if !reflect.DeepEqual(vmA.Proxmox.Accelerators, []string{"GPU"}) {
		t.Fatalf("vmA accelerators = %v, want [GPU]", vmA.Proxmox.Accelerators)
	}

	var commRels []ResourceRelationship
	for _, rel := range app.Relationships {
		if rel.Type == RelCommunicatesWith {
			commRels = append(commRels, rel)
		}
	}
	if len(commRels) != 1 {
		t.Fatalf("expected 1 communicates_with relationship on app-node, got %+v", commRels)
	}
	rel := commRels[0]
	if rel.TargetID != db.ID || rel.Discoverer != hostSocketPeerRelationshipDiscoverer {
		t.Fatalf("unexpected relationship target/discoverer: %+v (want target=%s)", rel, db.ID)
	}
	if gotPorts, ok := rel.Metadata["ports"].([]int); !ok || !reflect.DeepEqual(gotPorts, []int{5432, 6379}) {
		t.Fatalf("relationship ports = %#v, want [5432 6379]", rel.Metadata["ports"])
	}

	// Clearing ActivePeers removes stale host_socket_table relationships cleanly.
	state.Hosts[0].ActivePeers = nil
	state.Hosts[1].ActivePeers = nil
	rr.IngestSnapshot(state)
	for _, r := range rr.List() {
		for _, existing := range r.Relationships {
			if existing.Type == RelCommunicatesWith {
				t.Fatalf("expected stale communicates_with edge to be removed, found %+v on %s", existing, r.Name)
			}
		}
	}
}

func TestRelationshipTypeLabel_CommunicatesWith(t *testing.T) {
	if got := RelationshipTypeLabel(RelCommunicatesWith); got != "Communicates with" {
		t.Fatalf("RelationshipTypeLabel(RelCommunicatesWith) = %q, want %q", got, "Communicates with")
	}
}
