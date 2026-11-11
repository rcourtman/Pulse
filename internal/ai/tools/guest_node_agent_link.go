package tools

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// GuestNodeAgentLink says how sure Pulse is about which agent runs on the node
// hosting a guest.
type GuestNodeAgentLink int

const (
	// GuestNodeAgentNone: no agent is tied to the guest's node.
	GuestNodeAgentNone GuestNodeAgentLink = iota
	// GuestNodeAgentUnconfirmed: an agent is tied to a node for this guest,
	// and Pulse cannot show it is the one on the guest's own node.
	GuestNodeAgentUnconfirmed
	// GuestNodeAgentConfirmed: one agent, on the guest's own connection,
	// carrying the node's name, with an interface address Proxmox lists for
	// that node and no other, and no other node of that name on record.
	GuestNodeAgentConfirmed
)

// GuestNodeAgent is what Pulse knows about the agent on the node hosting a
// guest.
type GuestNodeAgent struct {
	// AgentID is set only when Link is GuestNodeAgentConfirmed. An agent Pulse
	// cannot confirm is never named: nothing may be sent to it, and nobody
	// should be told to change its settings for a guest it may not host.
	AgentID string
	Link    GuestNodeAgentLink
}

// GuestNodeAgentFor names the agent on the node hosting a guest, for a
// change to that guest. ConnectedAgentForGuestNode is the answer for a read
// and follows a unique hostname. A change must not: another machine can hold
// the node's name, and the same VMID there is a different guest. So this
// answer needs all of the following, and anything less is unconfirmed.
//
//   - The guest is on record and its node record exists.
//   - That node is on the guest's own connection and carries the guest's own
//     node name. A guest record can adopt a same-named node of another
//     connection as its parent when its own node record is missing.
//   - Exactly one agent is tied to the node: the node's own agent facet, the
//     host agents linked to it, or both when they agree.
//   - The agent carries the node's name, and no other node on record does.
//     Two nodes of one name are two machines Pulse cannot tell apart by
//     anything an agent says.
//   - One of the agent's own interfaces holds an address from the network
//     inventory Proxmox itself reports for the node, which no other node on
//     record lists. A node whose inventory Pulse cannot read has no such
//     address.
//
// What does not count: the link itself, which Pulse makes on a matching name
// when nothing contradicts it. The address Pulse reaches the node's API at,
// which can belong to a reverse proxy on another host. The agent's report
// address, which is a setting anyone can type.
//
// This is evidence, not proof of the machine. It cannot rule out a machine
// that is not a node on record, carries the node's name and holds one of the
// node's addresses. Proof would take the node attesting to its own Proxmox
// identity, which the agent does not do today.
//
// A machine monitored through two connections is one node record. Guests seen
// through the connection that record does not carry are unconfirmed here.
func GuestNodeAgentFor(rs unifiedresources.ReadState, guestID string) GuestNodeAgent {
	if rs == nil {
		return GuestNodeAgent{}
	}
	selected := unifiedresources.ResolveResourceReferenceContext(rs, guestID)
	if selected.Resource == nil {
		selected = unifiedresources.ResolveResourceContext(rs, guestID)
	}
	guest := selected.Resource
	if guest == nil || guest.Proxmox == nil || !selected.Location.Found {
		return GuestNodeAgent{}
	}
	reference := firstNonEmptyString(selected.Location.NodeResourceID, selected.Location.Node)
	nodes := rs.Nodes()
	for _, node := range nodes {
		if node == nil || node.ID() != reference {
			continue
		}
		candidates := guestNodeAgentCandidates(rs, node)
		if len(candidates) == 0 {
			return GuestNodeAgent{}
		}
		if len(candidates) != 1 {
			return GuestNodeAgent{Link: GuestNodeAgentUnconfirmed}
		}
		instance := strings.TrimSpace(guest.Proxmox.Instance)
		nodeName := strings.TrimSpace(guest.Proxmox.NodeName)
		ownNode := instance != "" && nodeName != "" &&
			strings.EqualFold(strings.TrimSpace(node.Instance()), instance) &&
			unifiedresources.HostnamesEquivalent(node.NodeName(), nodeName)
		for agentID, candidate := range candidates {
			if !ownNode || !guestNodeAgentSharesNodeAddress(nodes, node, candidate) {
				return GuestNodeAgent{Link: GuestNodeAgentUnconfirmed}
			}
			return GuestNodeAgent{AgentID: agentID, Link: GuestNodeAgentConfirmed}
		}
	}
	return GuestNodeAgent{}
}

// guestNodeAgentCandidate is what one agent tied to a node observed about its
// own machine.
type guestNodeAgentCandidate struct {
	hostname string
	network  []unifiedresources.NetworkInterface
}

// guestNodeAgentCandidates lists every agent tied to a node, by agent ID: the
// node's own agent facet and each host agent linked to the node.
func guestNodeAgentCandidates(rs unifiedresources.ReadState, node *unifiedresources.NodeView) map[string]guestNodeAgentCandidate {
	candidates := make(map[string]guestNodeAgentCandidate)
	owned := unifiedresources.ResolveResourceContext(rs, node.ID())
	if owned.Resource != nil && owned.Resource.Agent != nil {
		if agentID := strings.TrimSpace(owned.Resource.Agent.AgentID); agentID != "" {
			candidates[agentID] = guestNodeAgentCandidate{
				hostname: owned.Resource.Agent.Hostname,
				network:  owned.Resource.Agent.NetworkInterfaces,
			}
		}
	}
	for _, host := range rs.Hosts() {
		if host == nil || host.LinkedNodeID() == "" || host.AgentID() == "" {
			continue
		}
		if canonicalLinkedResourceID(rs, host.LinkedNodeID()) != node.ID() {
			continue
		}
		if _, known := candidates[host.AgentID()]; known {
			continue
		}
		candidates[host.AgentID()] = guestNodeAgentCandidate{
			hostname: host.Hostname(),
			network:  host.NetworkInterfaces(),
		}
	}
	return candidates
}

// guestNodeAgentSharesNodeAddress reports whether an agent carries the node's
// name, which no other node on record carries, and one of its own interfaces
// holds an address that Proxmox lists for this node and for no other node.
func guestNodeAgentSharesNodeAddress(nodes []*unifiedresources.NodeView, node *unifiedresources.NodeView, agent guestNodeAgentCandidate) bool {
	if !unifiedresources.HostnamesEquivalent(agent.hostname, node.NodeName()) {
		return false
	}
	observed := linkAddresses(agent.network)
	if len(observed) == 0 {
		return false
	}
	for _, other := range nodes {
		if other != nil && other.ID() != node.ID() && unifiedresources.HostnamesEquivalent(other.NodeName(), node.NodeName()) {
			return false
		}
	}
	for address := range nodeLinkAddresses(node) {
		if _, ok := observed[address]; !ok {
			continue
		}
		shared := false
		for _, other := range nodes {
			if other == nil || other.ID() == node.ID() {
				continue
			}
			if _, held := nodeLinkAddresses(other)[address]; held {
				shared = true
				break
			}
		}
		if !shared {
			return true
		}
	}
	return false
}

// nodeLinkAddresses is the network inventory Proxmox itself reports for a
// node. The address Pulse dials is deliberately not among them: it says where
// the API answers, which can be a proxy, and not which machine the node is.
func nodeLinkAddresses(node *unifiedresources.NodeView) map[string]struct{} {
	return linkAddresses(node.NetworkInterfaces())
}

func linkAddresses(network []unifiedresources.NetworkInterface) map[string]struct{} {
	addresses := make(map[string]struct{})
	for _, nic := range network {
		if models.IsHostLocalLinkInterface(nic.Name) {
			continue
		}
		for _, address := range nic.Addresses {
			if normalized := models.NodeLinkAddress(address); normalized != "" {
				addresses[normalized] = struct{}{}
			}
		}
	}
	return addresses
}
