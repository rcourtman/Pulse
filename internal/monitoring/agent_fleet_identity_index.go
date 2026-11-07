package monitoring

import (
	"strings"
	"unicode"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// A lookup belongs to one captured diagnostic snapshot, never to a live
// registry or a saved identity binding. It only shortlists the same records
// the old scans examined; the diagnostic predicates still decide each match.
type agentFleetIdentityReader interface {
	hostPeer(agentFleetSubject) (models.Host, bool)
	dockerPeer(agentFleetSubject) (models.DockerHost, bool)
	duplicateHostPeer(agentFleetSubject) (models.Host, bool)
}

type agentFleetIdentityIndex struct {
	hosts          []models.Host
	dockerHosts    []models.DockerHost
	hostPeers      agentFleetPeerIndex
	dockerPeers    agentFleetPeerIndex
	hostsByMachine map[string][]int
}

type agentFleetPeerIndex struct {
	byAgent    map[string][]int
	byToken    map[string][]int
	byHostname map[string][]int
}

func newAgentFleetIdentityIndex(state models.StateSnapshot) *agentFleetIdentityIndex {
	index := &agentFleetIdentityIndex{
		hosts:          state.Hosts,
		dockerHosts:    state.DockerHosts,
		hostPeers:      newAgentFleetPeerIndex(),
		dockerPeers:    newAgentFleetPeerIndex(),
		hostsByMachine: make(map[string][]int),
	}
	for pos, host := range state.Hosts {
		index.hostPeers.add(pos, host.ID, host.ID, host.Hostname, host.TokenID)
		if machineID := strings.TrimSpace(host.MachineID); machineID != "" {
			index.hostsByMachine[machineID] = append(index.hostsByMachine[machineID], pos)
		}
	}
	for pos, host := range state.DockerHosts {
		if !host.Hidden {
			index.dockerPeers.add(pos, host.ID, host.AgentID, host.Hostname, host.TokenID)
		}
	}
	return index
}

func newAgentFleetPeerIndex() agentFleetPeerIndex {
	return agentFleetPeerIndex{
		byAgent:    make(map[string][]int),
		byToken:    make(map[string][]int),
		byHostname: make(map[string][]int),
	}
}

func (index agentFleetPeerIndex) add(pos int, id, agentID, hostname, tokenID string) {
	if id != "" {
		index.byAgent[id] = append(index.byAgent[id], pos)
	}
	if agentID != "" && agentID != id {
		index.byAgent[agentID] = append(index.byAgent[agentID], pos)
	}
	if tokenID != "" {
		index.byToken[tokenID] = append(index.byToken[tokenID], pos)
	}
	if hostname != "" {
		key := agentFleetHostnameFoldKey(hostname)
		index.byHostname[key] = append(index.byHostname[key], pos)
	}
}

// EqualFold uses Unicode simple folding, not strings.ToLower. In particular
// Kelvin sign/K and long-s/S must still share a candidate bucket. Do not trim
// whitespace, shorten an FQDN or remove root dots: split diagnostics never did.
func agentFleetHostnameFoldKey(hostname string) string {
	return strings.Map(func(r rune) rune {
		key := r
		for folded := unicode.SimpleFold(r); folded != r; folded = unicode.SimpleFold(folded) {
			if folded < key {
				key = folded
			}
		}
		return key
	}, hostname)
}

func (index agentFleetPeerIndex) first(subject agentFleetSubject, matches func(int) bool) int {
	best := -1
	for _, candidates := range [][]int{
		index.byAgent[subject.agentID],
		index.byToken[subject.tokenID],
		index.byHostname[agentFleetHostnameFoldKey(subject.hostname)],
	} {
		for _, pos := range candidates {
			if best >= 0 && pos >= best {
				break
			}
			if matches(pos) {
				best = pos
				break
			}
		}
	}
	// Pick source order across ALL signals, not agent-ID priority. An earlier
	// hostname/token match always won over a later agent-ID match in the scan.
	return best
}

func (index *agentFleetIdentityIndex) hostPeer(subject agentFleetSubject) (models.Host, bool) {
	pos := index.hostPeers.first(subject, func(pos int) bool {
		host := index.hosts[pos]
		return host.ID != subject.id && sameAgentIdentity(subject, host.ID, host.ID, host.Hostname, host.TokenID)
	})
	if pos < 0 {
		return models.Host{}, false
	}
	return index.hosts[pos], true
}

func (index *agentFleetIdentityIndex) dockerPeer(subject agentFleetSubject) (models.DockerHost, bool) {
	pos := index.dockerPeers.first(subject, func(pos int) bool {
		host := index.dockerHosts[pos]
		return host.ID != subject.id && sameAgentIdentity(subject, host.ID, host.AgentID, host.Hostname, host.TokenID)
	})
	if pos < 0 {
		return models.DockerHost{}, false
	}
	return index.dockerHosts[pos], true
}

func (index *agentFleetIdentityIndex) duplicateHostPeer(subject agentFleetSubject) (models.Host, bool) {
	if subject.host == nil {
		return models.Host{}, false
	}
	machineID := strings.TrimSpace(subject.host.MachineID)
	hostname := strings.TrimSpace(subject.host.Hostname)
	if machineID == "" || hostname == "" {
		return models.Host{}, false
	}
	for _, pos := range index.hostsByMachine[machineID] {
		peer := index.hosts[pos]
		if strings.TrimSpace(peer.ID) != strings.TrimSpace(subject.host.ID) &&
			unifiedresources.HostnamesEquivalent(peer.Hostname, hostname) {
			return peer, true
		}
	}
	return models.Host{}, false
}
