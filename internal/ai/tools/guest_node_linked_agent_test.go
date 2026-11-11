package tools

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type linkedNodeFixture struct {
	instance    string
	endpoint    string   // how Pulse reaches the node's API
	inventory   []string // addresses Proxmox reports for the node
	agentID     string   // the node's own agent facet, when set
	agentName   string
	agentIPs    []string
	agentReport string
}

func (f linkedNodeFixture) resource() unifiedresources.Resource {
	resource := unifiedresources.Resource{
		ID: "node-" + f.instance, Type: unifiedresources.ResourceTypeAgent, Name: "pmx1",
		Sources: []unifiedresources.DataSource{unifiedresources.SourceProxmox},
		Proxmox: &unifiedresources.ProxmoxData{Instance: f.instance, ClusterName: "homelab", NodeName: "pmx1", SourceID: f.instance + ":pmx1", HostURL: f.endpoint},
	}
	if len(f.inventory) > 0 {
		resource.Proxmox.NetworkInterfaces = []unifiedresources.NetworkInterface{{Name: "vmbr0", Addresses: f.inventory}}
	}
	if f.agentID != "" {
		name := f.agentName
		if name == "" {
			name = "pmx1"
		}
		resource.Sources = append(resource.Sources, unifiedresources.SourceAgent)
		resource.Agent = &unifiedresources.AgentData{AgentID: f.agentID, Hostname: name, ReportIP: f.agentReport}
		if len(f.agentIPs) > 0 {
			resource.Agent.NetworkInterfaces = []unifiedresources.NetworkInterface{{Name: "vmbr0", Addresses: f.agentIPs}}
		}
	}
	return resource
}

// A change to a guest goes only to an agent Pulse can show runs on the guest's
// own node. Pulse ties an agent to a node by name when nothing contradicts it,
// which is enough to show them together and not enough to start a guest: the
// machine that only shares the node's name holds a different guest under the
// same VMID.
func TestGuestNodeAgentForNeedsMoreThanANameOrALink(t *testing.T) {
	guest := func(change func(*unifiedresources.Resource)) unifiedresources.Resource {
		resource := unifiedresources.Resource{
			ID: unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeSystemContainer, "home", 112), Type: unifiedresources.ResourceTypeSystemContainer, Name: "gitea",
			Sources: []unifiedresources.DataSource{unifiedresources.SourceProxmox},
			Proxmox: &unifiedresources.ProxmoxData{Instance: "home", ClusterName: "homelab", NodeName: "pmx1", VMID: 112, SourceID: "home:pmx1:112"},
		}
		if change != nil {
			change(&resource)
		}
		return resource
	}
	plain := guest(nil)
	registry := func(resources ...unifiedresources.Resource) unifiedresources.ReadState {
		rs := unifiedresources.NewRegistry(nil)
		rs.IngestResources(resources)
		return rs
	}
	// A linked agent on the node itself: it reports an address Proxmox lists
	// for the node.
	own := linkedNodeFixture{instance: "home", endpoint: "https://192.168.1.10:8006", inventory: []string{"192.168.1.10/24"}, agentID: "agent-home", agentIPs: []string{"192.168.1.10/24"}}
	otherNodeID := "node-hetzner"
	adopted := guest(func(r *unifiedresources.Resource) { r.ParentID = &otherNodeID })
	hetzner := linkedNodeFixture{instance: "hetzner", endpoint: "https://192.168.9.9:8006", inventory: []string{"192.168.9.9/24"}, agentID: "agent-hetzner", agentIPs: []string{"192.168.9.9/24"}}

	for _, tc := range []struct {
		name    string
		rs      unifiedresources.ReadState
		guestID string
		want    string
		link    GuestNodeAgentLink
	}{
		{"the agent reports an address Proxmox lists for the node", registry(own.resource(), plain), plain.ID, "agent-home", GuestNodeAgentConfirmed},
		{
			// The address Pulse dials can be a reverse proxy on another host.
			// An agent that reports it, with the node's own inventory saying
			// otherwise or saying nothing, has proven nothing.
			"the agent reports only the address Pulse dials",
			registry(linkedNodeFixture{instance: "home", endpoint: "https://203.0.113.10:8006", inventory: []string{"192.168.1.10/24"}, agentID: "agent-proxy", agentName: "proxy", agentIPs: []string{"203.0.113.10/24"}}.resource(), plain),
			plain.ID, "", GuestNodeAgentUnconfirmed,
		},
		{
			"the agent reports the dialled address and Proxmox reports no inventory",
			registry(linkedNodeFixture{instance: "home", endpoint: "https://192.168.1.10:8006", agentID: "agent-home", agentIPs: []string{"192.168.1.10/24"}}.resource(), plain),
			plain.ID, "", GuestNodeAgentUnconfirmed,
		},
		{
			// Two nodes of one name are two machines Pulse cannot tell apart
			// by anything an agent says, so neither gets a start.
			"its own node beside a namesake on another connection",
			registry(own.resource(), hetzner.resource(), plain), plain.ID, "", GuestNodeAgentUnconfirmed,
		},
		{
			"its own node beside other nodes of other names",
			registry(own.resource(), unifiedresources.Resource{
				ID: "node-pmx2", Type: unifiedresources.ResourceTypeAgent, Name: "pmx2",
				Sources: []unifiedresources.DataSource{unifiedresources.SourceProxmox},
				Proxmox: &unifiedresources.ProxmoxData{
					Instance: "home", ClusterName: "homelab", NodeName: "pmx2", SourceID: "home:pmx2",
					NetworkInterfaces: []unifiedresources.NetworkInterface{{Name: "vmbr0", Addresses: []string{"192.168.1.11/24"}}},
				},
			}, plain),
			plain.ID, "agent-home", GuestNodeAgentConfirmed,
		},
		{
			"a node reached by name, matched by its inventory",
			registry(linkedNodeFixture{instance: "home", endpoint: "https://pmx1.lan:8006", inventory: []string{"192.168.1.10/24"}, agentID: "agent-home", agentIPs: []string{"192.168.1.10/24"}}.resource(), plain),
			plain.ID, "agent-home", GuestNodeAgentConfirmed,
		},
		{
			// The report address is a setting anyone can type on any machine.
			"only the agent's configured report address matches",
			registry(linkedNodeFixture{instance: "home", endpoint: "https://pmx1.lan:8006", inventory: []string{"203.0.113.10/24"}, agentID: "agent-behind-nat", agentName: "other", agentIPs: []string{"10.20.30.40/24"}, agentReport: "203.0.113.10"}.resource(), plain),
			plain.ID, "", GuestNodeAgentUnconfirmed,
		},
		{
			// An address can be held by more than one machine, public ones
			// included, so it never stands in for the name.
			"a public address under another name",
			registry(linkedNodeFixture{instance: "home", endpoint: "https://pmx1.lan:8006", inventory: []string{"203.0.113.10/24"}, agentID: "agent-home", agentName: "something-else", agentIPs: []string{"203.0.113.10/24"}}.resource(), plain),
			plain.ID, "", GuestNodeAgentUnconfirmed,
		},
		{
			// The link was made on the name alone: a machine called pmx1 that
			// shares no address with the node.
			"linked by name with no address in common",
			registry(linkedNodeFixture{instance: "home", endpoint: "https://192.168.1.10:8006", inventory: []string{"192.168.1.10/24"}, agentID: "agent-elsewhere", agentIPs: []string{"10.20.30.40/24"}}.resource(), plain),
			plain.ID, "", GuestNodeAgentUnconfirmed,
		},
		{
			"the node's addresses are unknown",
			registry(linkedNodeFixture{instance: "home", endpoint: "https://pmx1.lan:8006", agentID: "agent-home", agentIPs: []string{"192.168.1.10/24"}}.resource(), plain),
			plain.ID, "", GuestNodeAgentUnconfirmed,
		},
		{
			"a private address under another name",
			registry(linkedNodeFixture{instance: "home", endpoint: "https://192.168.1.10:8006", inventory: []string{"192.168.1.10/24"}, agentID: "agent-home", agentName: "something-else", agentIPs: []string{"192.168.1.10/24"}}.resource(), plain),
			plain.ID, "", GuestNodeAgentUnconfirmed,
		},
		{
			"the shared address belongs to another node as well",
			registry(
				linkedNodeFixture{instance: "home", endpoint: "https://pmx1.lan:8006", inventory: []string{"10.10.10.1/24"}, agentID: "agent-home", agentIPs: []string{"10.10.10.1/24"}}.resource(),
				linkedNodeFixture{instance: "hetzner", endpoint: "https://pmx1.example:8006", inventory: []string{"10.10.10.1/24"}}.resource(),
				plain,
			),
			plain.ID, "", GuestNodeAgentUnconfirmed,
		},
		{
			// Another node of the same name is on record and Pulse cannot
			// read its inventory. It may hold the same address, so the name
			// and the address together still prove nothing.
			"a same-named node whose inventory is unknown",
			registry(
				own.resource(),
				linkedNodeFixture{instance: "hetzner", endpoint: "https://pmx1.example:8006"}.resource(),
				plain,
			),
			plain.ID, "", GuestNodeAgentUnconfirmed,
		},
		{
			"a public address beside a same-named node whose inventory lists another",
			registry(
				linkedNodeFixture{instance: "home", endpoint: "https://pmx1.lan:8006", inventory: []string{"203.0.113.10/24"}, agentID: "agent-home", agentIPs: []string{"203.0.113.10/24"}}.resource(),
				linkedNodeFixture{instance: "hetzner", endpoint: "https://pmx1.example:8006", inventory: []string{"203.0.113.99/24"}}.resource(),
				plain,
			),
			plain.ID, "", GuestNodeAgentUnconfirmed,
		},
		{"a node with no agent tied to it", registry(linkedNodeFixture{instance: "home", endpoint: "https://192.168.1.10:8006"}.resource(), plain), plain.ID, "", GuestNodeAgentNone},
		{"the node record is missing", registry(plain), plain.ID, "", GuestNodeAgentNone},
		{"the record's parent is a namesake on another connection", registry(hetzner.resource(), adopted), adopted.ID, "", GuestNodeAgentUnconfirmed},
		{"the guest is not on record", registry(own.resource()), "guest-not-ingested-yet", "", GuestNodeAgentNone},
		{"no read state", nil, plain.ID, "", GuestNodeAgentNone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			found := GuestNodeAgentFor(tc.rs, tc.guestID)
			if found.AgentID != tc.want || found.Link != tc.link {
				t.Fatalf("GuestNodeAgentFor = %+v, want %q, link %d", found, tc.want, tc.link)
			}
			// An agent is named only when it is confirmed.
			if (found.AgentID != "") != (found.Link == GuestNodeAgentConfirmed) {
				t.Fatalf("an agent is named without being confirmed, or the reverse: %+v", found)
			}
		})
	}

	// Two agents tied to one node are never resolved by picking one.
	second := unifiedresources.Resource{
		ID: "host-second", Type: unifiedresources.ResourceTypeAgent, Name: "pmx1-b",
		Sources: []unifiedresources.DataSource{unifiedresources.SourceAgent},
		Agent: &unifiedresources.AgentData{
			AgentID: "agent-second", Hostname: "pmx1", LinkedNodeID: "node-home",
			NetworkInterfaces: []unifiedresources.NetworkInterface{{Name: "vmbr0", Addresses: []string{"192.168.1.10/24"}}},
		},
	}
	if found := GuestNodeAgentFor(registry(own.resource(), second, plain), plain.ID); found.AgentID != "" || found.Link != GuestNodeAgentUnconfirmed {
		t.Fatalf("two agents tied to one node resolved to %+v", found)
	}

	// A single agent Pulse cannot confirm is not named at all: nothing may be
	// sent to it, and nobody should be told to change its settings.
	byNameOnly := GuestNodeAgentFor(registry(linkedNodeFixture{instance: "home", endpoint: "https://192.168.1.10:8006", inventory: []string{"192.168.1.10/24"}, agentID: "agent-elsewhere", agentIPs: []string{"10.20.30.40/24"}}.resource(), plain), plain.ID)
	if byNameOnly.AgentID != "" || byNameOnly.Link != GuestNodeAgentUnconfirmed {
		t.Fatalf("an agent linked by name only = %+v, want it unnamed and unconfirmed", byNameOnly)
	}

	// The read path still follows a unique hostname where this one refuses,
	// which is why a start must not use it.
	namesake := []agentexec.ConnectedAgent{{AgentID: "agent-elsewhere", Hostname: "pmx1"}}
	unlinked := registry(linkedNodeFixture{instance: "home", endpoint: "https://192.168.1.10:8006"}.resource(), plain)
	if agent, found := ConnectedAgentForGuestNode(unlinked, plain.ID, "home", "pmx1", namesake); !found || agent.AgentID != "agent-elsewhere" {
		t.Fatalf("the read path no longer follows a unique hostname: %q, %v", agent.AgentID, found)
	}
	if found := GuestNodeAgentFor(unlinked, plain.ID); found.AgentID != "" || found.Link != GuestNodeAgentNone {
		t.Fatalf("a start would follow the hostname: %+v", found)
	}
}
