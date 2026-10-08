package unifiedresources

import (
	"net"
	"slices"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// A Proxmox node's link to a pulse-agent is Pulse's own inference, not an
// operator decision: monitoring links a node to the one agent whose endpoint
// address or hostname matches it, and inferLinkedHostsForProxmoxNodes and
// resolveLinkedResource corroborate that link. Cloned machines and reused
// short hostnames make it wrong at times, which is what the resources API's
// unlink and report-merge are for, so an operator's split overrides the link
// as it overrides identity matching in findMatch. A later POST
// /api/resources/{id}/link between the two rows joins them again.

// nodeAgentLinkSide is one side of a Proxmox node<->agent pair: the record's
// source ID, the identity it derives IDs from and the resource ID it holds in
// the registry, if any.
type nodeAgentLinkSide struct {
	sourceID string
	identity ResourceIdentity
	heldID   string
}

// operatorPairDecisions is the operator's manual decisions a node<->agent
// split is read from: a registry's own, or the copy the presentation filter
// holds outside the registry lock.
type operatorPairDecisions struct {
	exclusions map[string]time.Time
	linksByID  map[string][]ResourceLink
}

func (rr *ResourceRegistry) operatorPairDecisionsLocked() operatorPairDecisions {
	return operatorPairDecisions{exclusions: rr.exclusions, linksByID: rr.linksByID}
}

// indexLinksByID lists each manual link under both of its IDs.
func indexLinksByID(links []ResourceLink) map[string][]ResourceLink {
	if len(links) == 0 {
		return nil
	}
	index := make(map[string][]ResourceLink, 2*len(links))
	for _, link := range links {
		a, b := CanonicalResourceID(link.ResourceA), CanonicalResourceID(link.ResourceB)
		index[a] = append(index[a], link)
		if b != a {
			index[b] = append(index[b], link)
		}
	}
	return index
}

// nodeAgentSplit reports whether the operator split the node off a
// node<->agent pair: an exclusion names one of the node's own IDs against one
// of the agent's IDs, and no link between those IDs postdates it (a tie keeps
// the pair apart, as it does for one pair in effectiveManualPairDecisions).
//
// Unlink and report-merge name the merged resource's ID and the source-specific
// candidate IDs its SourceTargets list, and a split moves the node onto an ID
// it did not hold while joined, so decisions are read across sets rather
// than one exact pair. The node's own IDs are its source-specific ID and the
// ID its identity derives without a machine key. The agent's are its
// source-specific ID, the ID its identity derives, the ID it holds, and the
// ID the joined pair derives (the agent's machine-derived ID once a node
// takes its identity). An exclusion naming only the agent's candidate against
// the joined ID does not count: report-merge records that same pair when it
// splits another source, such as a Docker host, off the agent's
// machine-derived ID.
//
// A link between the two sets that postdates the newest exclusion wins: a
// relink names the rows a split left, a different pair than the exclusions
// report-merge recorded against the merged ID, which the store's
// one-decision-per-pair rule cannot match.
func (rr *ResourceRegistry) nodeAgentSplit(decisions operatorPairDecisions, node, agent nodeAgentLinkSide) bool {
	if len(decisions.exclusions) == 0 {
		return false
	}
	nodeIDs, agentIDs := rr.nodeAgentPairIDs(node, agent)
	var splitAt time.Time
	split := false
	for _, nodeID := range nodeIDs {
		for _, agentID := range agentIDs {
			if nodeID == agentID {
				continue
			}
			if at, ok := decisions.exclusions[exclusionKey(nodeID, agentID)]; ok && (!split || at.After(splitAt)) {
				split, splitAt = true, at
			}
		}
	}
	if !split {
		return false
	}
	for _, nodeID := range nodeIDs {
		for _, link := range decisions.linksByID[nodeID] {
			other := CanonicalResourceID(link.ResourceB)
			if other == nodeID {
				other = CanonicalResourceID(link.ResourceA)
			}
			if other != nodeID && slices.Contains(agentIDs, other) && link.CreatedAt.After(splitAt) {
				return false
			}
		}
	}
	return true
}

// nodeAgentPairIDs returns the node's own IDs and the agent's IDs a split is
// read across. A Proxmox node reports no machine ID or DMI UUID of its own:
// either one on its identity came from a linked agent or an identity pin, so
// the node's own ID is derived without them. The two IDs are then the same
// whether or not the node took the agent's identity in this ingest.
func (rr *ResourceRegistry) nodeAgentPairIDs(node, agent nodeAgentLinkSide) (nodeIDs, agentIDs []string) {
	nodeIDs = rr.proxmoxNodeOwnIDs(node.sourceID, node.identity)
	if agent.sourceID != "" {
		agentIDs = append(agentIDs, rr.sourceSpecificID(ResourceTypeAgent, SourceAgent, agent.sourceID))
	}
	if agent.sourceID != "" || agent.identity.MachineID != "" || agent.identity.DMIUUID != "" {
		agentIDs = append(agentIDs, rr.chooseNewID(ResourceTypeAgent, agent.identity, SourceAgent, agent.sourceID))
	}
	if held := CanonicalResourceID(agent.heldID); held != "" {
		agentIDs = append(agentIDs, held)
	}
	// The joined ID counts as the agent's even where it is also one of the
	// node's (an agent reporting no machine key joins under the node's own
	// ID): an exclusion splitting the node's candidate off it then splits
	// the node from the agent, and the equal pair is skipped.
	nodeIdentity := node.identity
	nodeIdentity.MachineID, nodeIdentity.DMIUUID = "", ""
	agentIDs = append(agentIDs, rr.chooseNewID(ResourceTypeAgent, mergeIdentity(nodeIdentity, agent.identity), SourceProxmox, node.sourceID))
	return nodeIDs, agentIDs
}

// proxmoxNodeOwnIDs returns the IDs a Proxmox node record holds without any
// agent: its source-specific ID and the ID its identity derives with the
// machine keys it cannot report itself removed.
func (rr *ResourceRegistry) proxmoxNodeOwnIDs(sourceID string, identity ResourceIdentity) []string {
	identity.MachineID, identity.DMIUUID = "", ""
	return []string{
		rr.sourceSpecificID(ResourceTypeAgent, SourceProxmox, sourceID),
		rr.chooseNewID(ResourceTypeAgent, identity, SourceProxmox, sourceID),
	}
}

// proxmoxNodeAgentSplitLocked applies nodeAgentSplit to the registry's own
// decisions. Callers hold rr.mu.
func (rr *ResourceRegistry) proxmoxNodeAgentSplitLocked(node, agent nodeAgentLinkSide) bool {
	return rr.nodeAgentSplit(rr.operatorPairDecisionsLocked(), node, agent)
}

// proxmoxNodePinOfSplitAgentLocked reports whether an identity pin a Proxmox
// node record would complete its machine keys from belongs to an agent the
// operator split the node from. A node reports no machine key of its own, so
// a pin's is an agent's, written while the pair was joined, and completing
// it would mint that agent's machine-derived ID and fold the node into the
// agent by ID or by identity. The pin's canonical ID and keys stand for the
// agent the node's link or an agent row holding the pin's ID names, so this
// holds while the agent is missing from the snapshot. The agent this
// snapshot's split dropped (rr.splitAgents) can be another agent than the
// pin's, so it refuses only a pin that can be its own.
func (rr *ResourceRegistry) proxmoxNodePinOfSplitAgentLocked(source DataSource, sourceID string, resource Resource, identity ResourceIdentity, pin ResourceIdentityPin) bool {
	if source != SourceProxmox || resource.Proxmox == nil || len(rr.exclusions) == 0 {
		return false
	}
	node := nodeAgentLinkSide{sourceID: sourceID, identity: identity}
	pinAgent := nodeAgentLinkSide{
		sourceID: normalizeSourceID(resource.Proxmox.LinkedAgentID),
		identity: ResourceIdentity{MachineID: pin.MachineID, DMIUUID: pin.DMIUUID},
		heldID:   pin.CanonicalID,
	}
	if rr.proxmoxNodeAgentSplitLocked(node, pinAgent) {
		return true
	}
	if held := agentOnlyRow(rr.resources[CanonicalResourceID(pin.CanonicalID)]); held != nil {
		if heldSourceID := normalizeSourceID(held.Agent.AgentID); heldSourceID != pinAgent.sourceID {
			pinAgent.sourceID = heldSourceID
			if rr.proxmoxNodeAgentSplitLocked(node, pinAgent) {
				return true
			}
		}
	}
	split, ok := rr.splitAgents[normalizeSourceID(sourceID)]
	return ok && pinBelongsToAgent(pin, split.identity) && rr.proxmoxNodeAgentSplitLocked(node, split)
}

// pinBelongsToAgent reports whether an identity pin can be the agent's: only
// a key both report that differs shows the pin is another agent's. An agent
// that stops reporting its machine ID keeps its split that way, while another
// agent with its own key does not claim the pin.
func pinBelongsToAgent(pin ResourceIdentityPin, agent ResourceIdentity) bool {
	if pinKey, agentKey := strings.TrimSpace(pin.MachineID), strings.TrimSpace(agent.MachineID); pinKey != "" && agentKey != "" {
		return strings.EqualFold(pinKey, agentKey)
	}
	if pinKey, agentKey := strings.TrimSpace(pin.DMIUUID), strings.TrimSpace(agent.DMIUUID); pinKey != "" && agentKey != "" {
		return strings.EqualFold(pinKey, agentKey)
	}
	return true
}

// joinedPairSideLocked reports whether id is an ID one side of a joined
// Proxmox node+agent row holds on its own, and that side's source: the
// node's own IDs (derived from the row's Proxmox facet, not its merged
// identity, whose first hostname can be the agent's) or the agent's
// source-specific ID, which an agent reporting no machine key holds when
// split.
func (rr *ResourceRegistry) joinedPairSideLocked(joined *Resource, id string) (DataSource, bool) {
	if joined == nil || joined.Proxmox == nil || joined.Agent == nil || CanonicalResourceType(joined.Type) != ResourceTypeAgent {
		return "", false
	}
	id = CanonicalResourceID(id)
	if agentID := normalizeSourceID(joined.Agent.AgentID); agentID != "" && id == rr.sourceSpecificID(ResourceTypeAgent, SourceAgent, agentID) {
		return SourceAgent, true
	}
	sourceID := normalizeSourceID(joined.Proxmox.SourceID)
	if sourceID == "" {
		return "", false
	}
	_, identity := resourceFromProxmoxNode(models.Node{
		ID:                sourceID,
		Name:              joined.Proxmox.NodeName,
		NativeNameAliases: joined.Proxmox.NodeAliases,
		Instance:          joined.Proxmox.Instance,
		ClusterName:       joined.Proxmox.ClusterName,
	}, nil)
	return SourceProxmox, slices.Contains(rr.proxmoxNodeOwnIDs(sourceID, identity), id)
}

// nodeAgentRowsSplit reports whether two listed rows are a Proxmox node and
// an agent the operator split, for the paths that join rows after ingest:
// manual links (an older link between the rows a split left loses to a newer
// report-merge) and the presentation host coalesce (the rows can hold IDs no
// single exclusion names, a clustered node's cluster-derived ID for one).
func (rr *ResourceRegistry) nodeAgentRowsSplit(decisions operatorPairDecisions, left, right *Resource) bool {
	node, agent := proxmoxNodeRow(left), agentOnlyRow(right)
	if node == nil || agent == nil {
		node, agent = proxmoxNodeRow(right), agentOnlyRow(left)
	}
	if node == nil || agent == nil {
		return false
	}
	return rr.nodeAgentSplit(decisions,
		nodeAgentLinkSide{sourceID: normalizeSourceID(node.Proxmox.SourceID), identity: node.Identity},
		nodeAgentLinkSide{sourceID: normalizeSourceID(agent.Agent.AgentID), identity: agent.Identity, heldID: agent.ID},
	)
}

func proxmoxNodeRow(resource *Resource) *Resource {
	if resource == nil || CanonicalResourceType(resource.Type) != ResourceTypeAgent ||
		resource.Proxmox == nil || resource.Agent != nil || strings.TrimSpace(resource.Proxmox.SourceID) == "" {
		return nil
	}
	return resource
}

func agentOnlyRow(resource *Resource) *Resource {
	if resource == nil || CanonicalResourceType(resource.Type) != ResourceTypeAgent ||
		resource.Agent == nil || resource.Proxmox != nil || strings.TrimSpace(resource.Agent.AgentID) == "" {
		return nil
	}
	return resource
}

// splitProxmoxNodeLink returns the node record and the linked host it takes
// identity from, dropping the host for a node the operator split from that
// host's agent. A linked node takes the agent's identity, and with it the
// agent's machine-derived canonical ID, before the agent is ingested, so
// refusing only the later merge would leave the node holding the agent's ID
// and machine key, and identity matching would join the agent to it again.
// Without the agent's identity the node keeps its own ID, hostname and
// endpoint address, which identity matching never merges on. rr.splitAgents
// remembers the agent for the pin guard, which reads the split from it where
// the node record itself does not name that agent; it is rewritten for every
// node on every snapshot, and nothing else reads it.
func (rr *ResourceRegistry) splitProxmoxNodeLink(node models.Node, host *models.Host) (models.Node, *models.Host) {
	nodeID := normalizeSourceID(node.ID)
	rr.mu.Lock()
	defer rr.mu.Unlock()
	delete(rr.splitAgents, nodeID)
	if host == nil || len(rr.exclusions) == 0 {
		return node, host
	}
	_, nodeIdentity := resourceFromProxmoxNode(node, nil)
	_, hostIdentity := resourceFromHost(*host)
	if !rr.proxmoxNodeAgentSplitLocked(
		nodeAgentLinkSide{sourceID: nodeID, identity: nodeIdentity},
		nodeAgentLinkSide{sourceID: normalizeSourceID(host.ID), identity: hostIdentity},
	) {
		return node, host
	}
	if rr.splitAgents == nil {
		rr.splitAgents = make(map[string]nodeAgentLinkSide)
	}
	rr.splitAgents[nodeID] = nodeAgentLinkSide{sourceID: normalizeSourceID(host.ID), identity: hostIdentity}
	return node, nil
}

func inferLinkedHostsForProxmoxNodes(nodes []models.Node, hostByID map[string]*models.Host) map[string]*models.Host {
	if len(nodes) == 0 || len(hostByID) == 0 {
		return nil
	}

	nodeByID := make(map[string]*models.Node, len(nodes))
	for i := range nodes {
		nodeID := strings.TrimSpace(nodes[i].ID)
		if nodeID == "" {
			continue
		}
		nodeByID[nodeID] = &nodes[i]
	}

	keyToHostID := make(map[string]string)
	ambiguousKeys := make(map[string]struct{})
	nodeIDToHostID := make(map[string]string)
	ambiguousNodeIDs := make(map[string]struct{})
	trustedHostIDs := make(map[string]struct{})
	hostClusterByID := make(map[string]string)
	hostClusterAmbiguous := make(map[string]struct{})
	hostProviderNodeByID := make(map[string]models.Node)
	hostProviderNodeAmbiguous := make(map[string]struct{})
	recordHostCluster := func(hostID string, node models.Node) {
		hostID = strings.TrimSpace(hostID)
		cluster := strings.TrimSpace(strings.ToLower(node.ClusterName))
		if hostID == "" || cluster == "" {
			return
		}
		if _, ambiguous := hostClusterAmbiguous[hostID]; ambiguous {
			return
		}
		if existing, ok := hostClusterByID[hostID]; ok && existing != cluster {
			delete(hostClusterByID, hostID)
			hostClusterAmbiguous[hostID] = struct{}{}
			return
		}
		hostClusterByID[hostID] = cluster
	}
	// Short hostnames and short endpoint aliases collide across estates
	// (#1753: pve01 in staging vs pve01 in production), so a host whose
	// trusted link pins it to one cluster must never be inferred for a node
	// from a different cluster, whichever inference path proposed it.
	hostClusterConflicts := func(hostID string, node models.Node) bool {
		hostCluster := hostClusterByID[strings.TrimSpace(hostID)]
		if hostCluster == "" {
			return false
		}
		nodeCluster := strings.TrimSpace(strings.ToLower(node.ClusterName))
		return nodeCluster != "" && nodeCluster != hostCluster
	}
	recordHostProviderNode := func(hostID string, node models.Node) {
		hostID = strings.TrimSpace(hostID)
		if hostID == "" {
			return
		}
		if _, ambiguous := hostProviderNodeAmbiguous[hostID]; ambiguous {
			return
		}
		if existing, ok := hostProviderNodeByID[hostID]; ok &&
			!proxmoxProviderNodesProveSameMachine(existing, node, hostByID[hostID]) {
			delete(hostProviderNodeByID, hostID)
			hostProviderNodeAmbiguous[hostID] = struct{}{}
			return
		}
		hostProviderNodeByID[hostID] = node
	}
	hostProviderConflicts := func(hostID string, node models.Node) bool {
		hostID = strings.TrimSpace(hostID)
		if _, ambiguous := hostProviderNodeAmbiguous[hostID]; ambiguous {
			return true
		}
		existing, ok := hostProviderNodeByID[strings.TrimSpace(hostID)]
		if !ok {
			return false
		}
		existingInstance := strings.TrimSpace(strings.ToLower(existing.Instance))
		candidateInstance := strings.TrimSpace(strings.ToLower(node.Instance))
		if existingInstance == "" || candidateInstance == "" || existingInstance == candidateInstance {
			return false
		}
		return !proxmoxProviderNodesProveSameMachine(existing, node, hostByID[hostID])
	}
	register := func(key, hostID string) {
		key = strings.TrimSpace(key)
		hostID = strings.TrimSpace(hostID)
		if key == "" || hostID == "" {
			return
		}
		if _, ambiguous := ambiguousKeys[key]; ambiguous {
			return
		}
		if existing, ok := keyToHostID[key]; ok && existing != hostID {
			delete(keyToHostID, key)
			ambiguousKeys[key] = struct{}{}
			return
		}
		keyToHostID[key] = hostID
	}
	registerNode := func(nodeID, hostID string) {
		nodeID = strings.TrimSpace(nodeID)
		hostID = strings.TrimSpace(hostID)
		if nodeID == "" || hostID == "" {
			return
		}
		if _, ambiguous := ambiguousNodeIDs[nodeID]; ambiguous {
			return
		}
		if existing, ok := nodeIDToHostID[nodeID]; ok && existing != hostID {
			delete(nodeIDToHostID, nodeID)
			ambiguousNodeIDs[nodeID] = struct{}{}
			return
		}
		nodeIDToHostID[nodeID] = hostID
	}

	for _, node := range nodes {
		hostID := strings.TrimSpace(node.LinkedAgentID)
		if hostID == "" {
			continue
		}
		host := hostByID[hostID]
		if host == nil || !trustedProxmoxNodeHostLink(node, *host) {
			continue
		}
		trustedHostIDs[hostID] = struct{}{}
		recordHostCluster(hostID, node)
		recordHostProviderNode(hostID, node)
		for _, key := range proxmoxNodeLinkKeys(node) {
			register(key, hostID)
		}
	}

	for hostID, host := range hostByID {
		if host == nil {
			continue
		}
		nodeID := strings.TrimSpace(host.LinkedNodeID)
		if nodeID == "" {
			continue
		}
		node := nodeByID[nodeID]
		if node == nil || !trustedHostProxmoxNodeLink(*host, *node) {
			continue
		}
		trustedHostIDs[hostID] = struct{}{}
		recordHostCluster(hostID, *node)
		recordHostProviderNode(hostID, *node)
		registerNode(nodeID, hostID)
		for _, key := range proxmoxNodeLinkKeys(*node) {
			register(key, hostID)
		}
	}

	out := make(map[string]*models.Host, len(nodes))
	for _, node := range nodes {
		nodeID := strings.TrimSpace(node.ID)
		if nodeID == "" {
			continue
		}

		if hostID := strings.TrimSpace(node.LinkedAgentID); hostID != "" {
			if host := hostByID[hostID]; host != nil && trustedProxmoxNodeHostLink(node, *host) {
				out[nodeID] = host
				continue
			}
		}

		if _, ambiguous := ambiguousNodeIDs[nodeID]; !ambiguous {
			if hostID := strings.TrimSpace(nodeIDToHostID[nodeID]); hostID != "" {
				if host := hostByID[hostID]; host != nil {
					out[nodeID] = host
					continue
				}
			}
		}

		inferredHostID := ""
		for _, key := range proxmoxNodeLinkKeys(node) {
			if _, ambiguous := ambiguousKeys[key]; ambiguous {
				continue
			}
			hostID := strings.TrimSpace(keyToHostID[key])
			if hostID == "" || hostClusterConflicts(hostID, node) || hostProviderConflicts(hostID, node) {
				continue
			}
			if inferredHostID != "" && inferredHostID != hostID {
				inferredHostID = ""
				break
			}
			inferredHostID = hostID
		}
		if inferredHostID == "" {
			for hostID := range trustedHostIDs {
				host := hostByID[hostID]
				if host == nil ||
					proxmoxNodeUsesProviderScopedIdentity(node) ||
					!proxmoxNodeCorroboratesHost(node, *host) {
					continue
				}
				if hostClusterConflicts(hostID, node) {
					continue
				}
				if hostProviderConflicts(hostID, node) {
					continue
				}
				if inferredHostID != "" && inferredHostID != hostID {
					inferredHostID = ""
					break
				}
				inferredHostID = hostID
			}
			if inferredHostID == "" {
				continue
			}
		}
		if host := hostByID[inferredHostID]; host != nil {
			out[nodeID] = host
		}
	}

	return out
}

// proxmoxProviderNodesProveSameMachine is the provider-scope boundary for
// lending one trusted host-agent identity to another node view. Native PVE
// names are commonly short and repeat across independent standalone sites, so
// a shared name is deliberately absent here. Distinct configured instances
// may share a host only when provider-owned evidence says they are the same
// (the same node identity or exact configured endpoint), or
// when both views independently match the trusted host's full endpoint/IP.
// A cluster name is only an operator-chosen display label and may be reused
// by independent estates, so equality is not provider identity evidence.
func proxmoxProviderNodesProveSameMachine(left, right models.Node, host *models.Host) bool {
	if leftID, rightID := strings.TrimSpace(left.ID), strings.TrimSpace(right.ID); leftID != "" && leftID == rightID {
		return true
	}
	if leftIdentity, rightIdentity := strings.TrimSpace(left.NodeIdentity), strings.TrimSpace(right.NodeIdentity); leftIdentity != "" && leftIdentity == rightIdentity {
		return true
	}
	leftInstance := strings.TrimSpace(strings.ToLower(left.Instance))
	rightInstance := strings.TrimSpace(strings.ToLower(right.Instance))
	if leftInstance != "" && leftInstance == rightInstance {
		return true
	}
	leftEndpoint := strings.TrimSpace(strings.ToLower(extractHostname(left.Host)))
	rightEndpoint := strings.TrimSpace(strings.ToLower(extractHostname(right.Host)))
	if leftEndpoint != "" && leftEndpoint == rightEndpoint {
		return true
	}
	return host != nil &&
		proxmoxNodeStronglyCorroboratesHost(left, *host) &&
		proxmoxNodeStronglyCorroboratesHost(right, *host)
}

func proxmoxNodeStronglyCorroboratesHost(node models.Node, host models.Host) bool {
	hostIPs := providerLinkNetworkIPs(host.NetworkInterfaces)
	if ip := providerLinkIP(host.ReportIP); ip != "" {
		hostIPs[ip] = struct{}{}
	}

	endpoint := strings.TrimSpace(strings.ToLower(extractHostname(node.Host)))
	if endpointIP := NormalizeIP(endpoint); endpointIP != "" {
		_, ok := hostIPs[endpointIP]
		return ok
	}
	hostname := NormalizeFullHostname(host.Hostname)
	if strings.Contains(endpoint, ".") && endpoint == hostname {
		return true
	}
	if nodeName := NormalizeFullHostname(node.Name); strings.Contains(nodeName, ".") && nodeName == hostname {
		return true
	}
	for ip := range providerLinkNetworkIPs(node.NetworkInterfaces) {
		if _, ok := hostIPs[ip]; ok {
			return true
		}
	}
	return false
}

func proxmoxNodeLinkKeys(node models.Node) []string {
	name := NormalizeHostname(node.Name)
	if name == "" {
		return nil
	}

	keys := make([]string, 0, 4)
	if proxmoxNodeUsesProviderScopedIdentity(node) {
		instance := strings.TrimSpace(strings.ToLower(node.Instance))
		if instance == "" {
			return nil
		}
		return []string{"provider:" + instance + ":" + name}
	}
	if cluster := strings.TrimSpace(strings.ToLower(node.ClusterName)); cluster != "" {
		keys = append(keys, "cluster:"+cluster+":"+name)
	}

	if endpoint := strings.TrimSpace(strings.ToLower(extractHostname(node.Host))); endpoint != "" {
		keys = append(keys, "endpoint-host:"+endpoint+":"+name)
		if short := NormalizeHostname(endpoint); short != "" && short != endpoint {
			keys = append(keys, "endpoint-host:"+short+":"+name)
		}
		if ip := NormalizeIP(endpoint); ip != "" {
			keys = append(keys, "endpoint-ip:"+ip+":"+name)
		}
	}

	return uniqueStrings(keys)
}

func trustedProxmoxNodeHostLink(node models.Node, host models.Host) bool {
	if strings.TrimSpace(host.LinkedNodeID) == strings.TrimSpace(node.ID) && strings.TrimSpace(node.ID) != "" {
		return true
	}
	if proxmoxNodeUsesProviderScopedIdentity(node) {
		return false
	}
	return proxmoxNodeCorroboratesHost(node, host)
}

func trustedHostProxmoxNodeLink(host models.Host, node models.Node) bool {
	hostLinkedNodeID := strings.TrimSpace(host.LinkedNodeID)
	nodeID := strings.TrimSpace(node.ID)
	if hostLinkedNodeID == "" || nodeID == "" || hostLinkedNodeID != nodeID {
		return false
	}
	if strings.TrimSpace(node.LinkedAgentID) == strings.TrimSpace(host.ID) && strings.TrimSpace(host.ID) != "" {
		return true
	}
	return proxmoxNodeCorroboratesHost(node, host)
}

func proxmoxNodeCorroboratesHost(node models.Node, host models.Host) bool {
	nodeName := NormalizeHostname(node.Name)
	hostName := NormalizeHostname(host.Hostname)
	if nodeName != "" && hostName != "" && nodeName == hostName {
		return true
	}

	endpoint := strings.TrimSpace(strings.ToLower(extractHostname(node.Host)))
	if endpoint == "" {
		return false
	}

	if ip := NormalizeIP(endpoint); ip != "" {
		if providerLinkIP(host.ReportIP) == ip {
			return true
		}
		if _, ok := providerLinkNetworkIPs(host.NetworkInterfaces)[ip]; ok {
			return true
		}
		return false
	}

	endpointHost := NormalizeHostname(endpoint)
	return endpointHost != "" && endpointHost == hostName
}

// Provider inference must not treat addresses repeated independently on each
// host as corroboration. Keep management bridges and private management IPs;
// reject only non-unicast addresses and recognisable host-local interfaces.
func providerLinkIP(address string) string {
	normalized := NormalizeIP(address)
	if ip := net.ParseIP(normalized); ip != nil && ip.IsGlobalUnicast() {
		return normalized
	}
	return ""
}

func providerLinkNetworkIPs(network []models.HostNetworkInterface) map[string]struct{} {
	ips := make(map[string]struct{})
	for _, nic := range network {
		name := strings.ToLower(strings.TrimSpace(nic.Name))
		if name == "lo" || strings.HasPrefix(name, "docker") {
			continue
		}
		if id, ok := strings.CutPrefix(name, "br-"); ok && len(id) == 12 && strings.Trim(id, "0123456789abcdef") == "" {
			continue
		}
		for _, address := range nic.Addresses {
			if ip := providerLinkIP(address); ip != "" {
				ips[ip] = struct{}{}
			}
		}
	}
	return ips
}
