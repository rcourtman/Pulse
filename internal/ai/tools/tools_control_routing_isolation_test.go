package tools

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// A guest that reports another connection's canonical node resource ID as its
// bare node name must never route commands to that other connection's agent.
func TestConnectedAgentForGuestNodeDoesNotReadBareNodeNameAsNodeResourceID(t *testing.T) {
	now := time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)

	// Build a snapshot where pve-b has a linked agent and an unrelated
	// connection "lab" reports a guest whose node name equals pve-b's
	// canonical resource ID.
	rr := unifiedresources.NewRegistry(nil)
	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:            "pve-b-pve-b",
				Name:          "pve-b",
				Instance:      "pve-b",
				Host:          "https://10.20.0.11:8006",
				LinkedAgentID: "agent-pve-b",
				Status:        "online",
				LastSeen:      now,
			},
			{
				ID:       "lab-lab1",
				Name:     "lab1",
				Instance: "lab",
				Host:     "https://10.20.0.66:8006",
				Status:   "online",
				LastSeen: now,
			},
		},
		Hosts: []models.Host{
			{
				ID:           "agent-pve-b",
				Hostname:     "pve-b",
				MachineID:    "machine-pve-b",
				Status:       "online",
				LastSeen:     now,
				LinkedNodeID: "pve-b-pve-b",
			},
		},
		VMs: []models.VM{
			{
				ID:       "pve-b:pve-b:100",
				VMID:     100,
				Name:     "mail",
				Node:     "pve-b",
				Instance: "pve-b",
				Status:   "running",
				LastSeen: now,
			},
		},
	})

	var victimNodeID string
	for _, node := range rr.Nodes() {
		if node.Instance() == "pve-b" {
			victimNodeID = node.ID()
			break
		}
	}
	if victimNodeID == "" {
		t.Fatal("victim node not found in registry")
	}

	// Now ingest a second snapshot where connection "lab" reports a guest whose
	// bare Node name is the exact canonical resource ID of pve-b's node.
	rr.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:            "pve-b-pve-b",
				Name:          "pve-b",
				Instance:      "pve-b",
				Host:          "https://10.20.0.11:8006",
				LinkedAgentID: "agent-pve-b",
				Status:        "online",
				LastSeen:      now,
			},
			{
				ID:       "lab-lab1",
				Name:     "lab1",
				Instance: "lab",
				Host:     "https://10.20.0.66:8006",
				Status:   "online",
				LastSeen: now,
			},
		},
		Hosts: []models.Host{
			{
				ID:           "agent-pve-b",
				Hostname:     "pve-b",
				MachineID:    "machine-pve-b",
				Status:       "online",
				LastSeen:     now,
				LinkedNodeID: "pve-b-pve-b",
			},
		},
		VMs: []models.VM{
			{
				ID:       "pve-b:pve-b:100",
				VMID:     100,
				Name:     "mail",
				Node:     "pve-b",
				Instance: "pve-b",
				Status:   "running",
				LastSeen: now,
			},
			{
				ID:       "lab:" + victimNodeID + ":100",
				VMID:     100,
				Name:     "spoofed-vm",
				Node:     victimNodeID,
				Instance: "lab",
				Status:   "running",
				LastSeen: now,
			},
		},
	})

	var mailID, spoofedID string
	for _, vm := range rr.ListByType(unifiedresources.ResourceTypeVM) {
		switch vm.Name {
		case "mail":
			mailID = vm.ID
		case "spoofed-vm":
			spoofedID = vm.ID
		}
	}
	if mailID == "" || spoofedID == "" {
		t.Fatalf("expected both VMs in registry, got mail=%q spoofed=%q", mailID, spoofedID)
	}

	agents := []agentexec.ConnectedAgent{
		{AgentID: "agent-pve-b", Hostname: "pve-b"},
	}

	// Legitimate guest on pve-b still routes to agent-pve-b.
	if agent, ok := ConnectedAgentForGuestNode(rr, mailID, "pve-b", "pve-b", agents); !ok || agent.AgentID != "agent-pve-b" {
		t.Fatalf("legitimate guest agent = (%+v, %v), want agent-pve-b", agent, ok)
	}

	// Spoofed guest on lab reporting victimNodeID as its bare node name must NOT
	// route to agent-pve-b via ConnectedAgentForGuestNode, ConnectedAgentForNodeName,
	// routeCommandTarget, or queryCommandConnections.guestParentNode.
	if agent, ok := ConnectedAgentForGuestNode(rr, spoofedID, "lab", victimNodeID, agents); ok {
		t.Fatalf("spoofed guest resolved agent %+v through another connection's canonical node ID", agent)
	}
	if agent, ok := ConnectedAgentForNodeName(rr, "lab", victimNodeID, agents); ok {
		t.Fatalf("ConnectedAgentForNodeName resolved agent %+v for bare canonical node ID", agent)
	}
	if route := routeCommandTarget(rr, agents, spoofedID); route.AgentID != "" {
		t.Fatalf("routeCommandTarget routed spoofed guest to agent %q (%+v)", route.AgentID, route)
	}
	qcc := newQueryCommandConnections(rr, agents)
	if got := qcc.guestParentNode(spoofedID); got != nil && *got {
		t.Fatal("queryCommandConnections.guestParentNode reported spoofed guest connected to another connection's agent")
	}
}
