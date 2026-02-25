package unifiedresources

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// HighConfidenceThreshold is the minimum IdentityMatcher confidence to merge
// two host entries as the same physical machine. Matches at or above this
// threshold (machine-id, DMI UUID, hostname+MAC) are considered safe.
// Lower-confidence matches (hostname-only, IP-only) are left separate.
const HighConfidenceThreshold = 0.90

// HostSource describes where a host was seen.
type HostSource struct {
	// Type is the connector category: "proxmox-pve", "proxmox-pbs", "proxmox-pmg",
	// "host-agent", "docker", "kubernetes", "truenas".
	Type string
	// SourceLabel is a short human-readable label: "proxmox", "agent", "docker", "kubernetes", "truenas".
	SourceLabel string
}

// ResolvedHost represents a single unique physical/virtual machine after
// cross-connector deduplication.
type ResolvedHost struct {
	// Name is the best display name chosen from the contributing sources.
	Name string
	// PrimaryType is the most specific connector type (e.g. "proxmox-pve").
	PrimaryType string
	// Status is the best known status across all sources.
	Status string
	// LastSeen is the most recent last-seen from any source (RFC3339 or "").
	LastSeen string
	// FirstSeen is the earliest first-seen from any source (RFC3339 or "").
	FirstSeen string
	// Sources lists all connectors that contributed to this host entry.
	Sources []HostSource
	// SourceLabels is a sorted, deduplicated list of human labels (e.g. "agent", "proxmox").
	SourceLabels []string
	// Provisional is true when the host has no runtime identity yet (config-only).
	Provisional bool
	// Identity is the merged identity signals for this host.
	Identity ResourceIdentity
}

// HostCandidate is a single host entry from any connector, ready for dedup.
type HostCandidate struct {
	// ID is a unique key within the source (e.g. node ID, host agent ID, docker host ID).
	ID        string
	Name      string
	Type      string // "proxmox-pve", "proxmox-pbs", etc.
	Source    string // "proxmox", "agent", "docker", "kubernetes", "truenas"
	Status    string // "online", "offline", "unknown"
	LastSeen  string // RFC3339 or ""
	FirstSeen string // RFC3339 or ""
	Identity  ResourceIdentity
	// Provisional means this entry was generated from config, not runtime.
	Provisional bool
}

// ResolvedHostSet holds the deduplication result.
type ResolvedHostSet struct {
	Hosts []ResolvedHost
}

// ResolveHosts takes a slice of HostCandidates from all connectors and returns
// a deduplicated set of unique hosts. It uses IdentityMatcher with high-confidence
// matching only (machine-id, DMI UUID, hostname+MAC). Weak matches stay separate.
func ResolveHosts(candidates []HostCandidate) *ResolvedHostSet {
	if len(candidates) == 0 {
		return &ResolvedHostSet{Hosts: nil}
	}

	// Phase 1: Build identity matcher from all candidates.
	matcher := NewIdentityMatcher()
	for i := range candidates {
		c := &candidates[i]
		matcher.Add(c.ID, c.Identity)
	}

	// Phase 2: Union-Find to group candidates that represent the same machine.
	parent := make(map[string]string, len(candidates))
	for i := range candidates {
		parent[candidates[i].ID] = candidates[i].ID
	}

	var find func(string) string
	find = func(id string) string {
		if parent[id] != id {
			parent[id] = find(parent[id])
		}
		return parent[id]
	}
	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	for i := range candidates {
		c := &candidates[i]
		matches := matcher.FindCandidates(c.Identity)
		for _, m := range matches {
			if m.ID == c.ID {
				continue
			}
			if m.Confidence >= HighConfidenceThreshold {
				union(c.ID, m.ID)
			}
		}
	}

	// Phase 3: Group candidates by root.
	groups := make(map[string][]*HostCandidate)
	for i := range candidates {
		c := &candidates[i]
		root := find(c.ID)
		groups[root] = append(groups[root], c)
	}

	// Phase 4: Build resolved hosts from groups.
	hosts := make([]ResolvedHost, 0, len(groups))
	for _, group := range groups {
		hosts = append(hosts, mergeGroup(group))
	}

	// Sort by type then name for stable output.
	sort.Slice(hosts, func(i, j int) bool {
		if hosts[i].PrimaryType != hosts[j].PrimaryType {
			return hosts[i].PrimaryType < hosts[j].PrimaryType
		}
		return hosts[i].Name < hosts[j].Name
	})

	return &ResolvedHostSet{Hosts: hosts}
}

// mergeGroup collapses a set of candidates for the same physical host into one ResolvedHost.
func mergeGroup(group []*HostCandidate) ResolvedHost {
	typePriority := map[string]int{
		"proxmox-pve": 0,
		"host-agent":  1,
		"docker":      2,
		"kubernetes":  3,
		"proxmox-pbs": 4,
		"proxmox-pmg": 5,
		"truenas":     6,
	}

	sort.Slice(group, func(i, j int) bool {
		pi, pj := typePriority[group[i].Type], typePriority[group[j].Type]
		if pi != pj {
			return pi < pj
		}
		return group[i].Name < group[j].Name
	})

	primary := group[0]
	rh := ResolvedHost{
		Name:        primary.Name,
		PrimaryType: primary.Type,
		Status:      primary.Status,
		LastSeen:    primary.LastSeen,
		FirstSeen:   primary.FirstSeen,
		Provisional: primary.Provisional,
		Identity:    primary.Identity,
	}

	labelSet := make(map[string]bool)

	for _, c := range group {
		rh.Sources = append(rh.Sources, HostSource{
			Type:        c.Type,
			SourceLabel: c.Source,
		})
		labelSet[c.Source] = true

		rh.Status = betterStatus(rh.Status, c.Status)

		if c.LastSeen > rh.LastSeen {
			rh.LastSeen = c.LastSeen
		}
		if rh.FirstSeen == "" || (c.FirstSeen != "" && c.FirstSeen < rh.FirstSeen) {
			rh.FirstSeen = c.FirstSeen
		}
		if !c.Provisional {
			rh.Provisional = false
		}
		rh.Identity = mergeIdentity(rh.Identity, c.Identity)
	}

	rh.SourceLabels = sortedKeys(labelSet)
	return rh
}

func betterStatus(a, b string) string {
	rank := map[string]int{"online": 2, "offline": 1, "unknown": 0}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// mergeIdentity is defined in registry.go and reused here.

// ---------------------------------------------------------------------------
// Collecting candidates from StateSnapshot + config
// ---------------------------------------------------------------------------

// ConfigEntry is a minimal representation of a config-based connection.
type ConfigEntry struct {
	ID   string
	Name string
	Host string
}

// CollectHostCandidates gathers all host-level entries from a StateSnapshot
// and config into a flat list of HostCandidates ready for dedup.
func CollectHostCandidates(
	state models.StateSnapshot,
	configPVE []ConfigEntry,
	configPBS []ConfigEntry,
	configPMG []ConfigEntry,
	configTrueNAS []ConfigEntry,
) []HostCandidate {
	var candidates []HostCandidate

	// PVE nodes: prefer runtime state, fall back to config.
	if len(state.Nodes) > 0 {
		for _, n := range state.Nodes {
			candidates = append(candidates, pveNodeCandidate(n))
		}
	} else {
		for _, c := range configPVE {
			candidates = append(candidates, HostCandidate{
				ID:          "config-pve:" + c.ID,
				Name:        resolvedConfigDisplayName(c.Name, c.Host),
				Type:        "proxmox-pve",
				Source:      "proxmox",
				Status:      "unknown",
				Provisional: true,
				Identity:    ResourceIdentity{Hostnames: uniqueStrings([]string{c.Name, extractHostname(c.Host)})},
			})
		}
	}

	for _, c := range configPBS {
		candidates = append(candidates, HostCandidate{
			ID:       "config-pbs:" + c.ID,
			Name:     resolvedConfigDisplayName(c.Name, c.Host),
			Type:     "proxmox-pbs",
			Source:   "proxmox",
			Status:   pbsStatusFromState(c.Host, state),
			LastSeen: pbsLastSeenFromState(c.Host, state),
			Identity: ResourceIdentity{Hostnames: uniqueStrings([]string{c.Name, extractHostname(c.Host)})},
		})
	}

	for _, c := range configPMG {
		candidates = append(candidates, HostCandidate{
			ID:       "config-pmg:" + c.ID,
			Name:     resolvedConfigDisplayName(c.Name, c.Host),
			Type:     "proxmox-pmg",
			Source:   "proxmox",
			Status:   pmgStatusFromState(c.Host, state),
			LastSeen: pmgLastSeenFromState(c.Host, state),
			Identity: ResourceIdentity{Hostnames: uniqueStrings([]string{c.Name, extractHostname(c.Host)})},
		})
	}

	for _, c := range configTrueNAS {
		candidates = append(candidates, HostCandidate{
			ID:          "config-truenas:" + c.ID,
			Name:        resolvedConfigDisplayName(c.Name, c.Host),
			Type:        "truenas",
			Source:      "truenas",
			Status:      "unknown",
			Provisional: true,
			Identity:    ResourceIdentity{Hostnames: uniqueStrings([]string{c.Name, extractHostname(c.Host)})},
		})
	}

	for _, h := range state.Hosts {
		candidates = append(candidates, hostAgentCandidate(h))
	}

	for _, d := range state.DockerHosts {
		candidates = append(candidates, dockerHostCandidate(d))
	}

	for _, cluster := range state.KubernetesClusters {
		candidates = append(candidates, k8sCandidates(cluster, state.Hosts)...)
	}

	return candidates
}

// ---------------------------------------------------------------------------
// Per-source candidate builders
// ---------------------------------------------------------------------------

func pveNodeCandidate(n models.Node) HostCandidate {
	identity := ResourceIdentity{
		Hostnames: uniqueStrings([]string{n.Name, extractHostname(n.Host)}),
	}
	return HostCandidate{
		ID:       "pve:" + n.ID,
		Name:     resolvedPVENodeDisplayName(n.DisplayName, n.Name, n.ID),
		Type:     "proxmox-pve",
		Source:   "proxmox",
		Status:   resolvedNormalizeStatus(n.Status),
		LastSeen: resolvedFormatTime(n.LastSeen),
		Identity: identity,
	}
}

func hostAgentCandidate(h models.Host) HostCandidate {
	ips, macs := collectInterfaceIDs(h.NetworkInterfaces)
	if h.ReportIP != "" {
		ips = append(ips, h.ReportIP)
	}
	return HostCandidate{
		ID:       "host:" + h.ID,
		Name:     resolvedHostDisplayName(h.DisplayName, h.Hostname, h.ID),
		Type:     "host-agent",
		Source:   "agent",
		Status:   resolvedNormalizeStatus(h.Status),
		LastSeen: resolvedFormatTime(h.LastSeen),
		Identity: ResourceIdentity{
			MachineID:    strings.TrimSpace(h.MachineID),
			Hostnames:    uniqueStrings([]string{h.Hostname}),
			IPAddresses:  uniqueStrings(ips),
			MACAddresses: uniqueStrings(macs),
		},
	}
}

func dockerHostCandidate(d models.DockerHost) HostCandidate {
	ips, macs := collectInterfaceIDs(d.NetworkInterfaces)
	return HostCandidate{
		ID:       "docker:" + d.ID,
		Name:     resolvedDockerDisplayName(d.DisplayName, d.CustomDisplayName, d.Hostname, d.ID),
		Type:     "docker",
		Source:   "docker",
		Status:   resolvedNormalizeStatus(d.Status),
		LastSeen: resolvedFormatTime(d.LastSeen),
		Identity: ResourceIdentity{
			MachineID:    strings.TrimSpace(d.MachineID),
			Hostnames:    uniqueStrings([]string{d.Hostname}),
			IPAddresses:  uniqueStrings(ips),
			MACAddresses: uniqueStrings(macs),
		},
	}
}

// k8sCandidates returns one candidate per K8s node. If the cluster has no nodes,
// it returns a single candidate for the cluster itself (minimum 1 slot).
func k8sCandidates(cluster models.KubernetesCluster, hosts []models.Host) []HostCandidate {
	clusterName := resolvedK8sClusterDisplayName(cluster)

	if len(cluster.Nodes) == 0 {
		return []HostCandidate{{
			ID:          "k8s-cluster:" + cluster.ID,
			Name:        clusterName,
			Type:        "kubernetes",
			Source:      "kubernetes",
			Status:      resolvedNormalizeStatus(cluster.Status),
			LastSeen:    resolvedFormatTime(cluster.LastSeen),
			Provisional: true,
			Identity: ResourceIdentity{
				Hostnames: uniqueStrings([]string{cluster.Name}),
			},
		}}
	}

	candidates := make([]HostCandidate, 0, len(cluster.Nodes))
	for i, kn := range cluster.Nodes {
		nodeName := kn.Name
		if nodeName == "" {
			nodeName = kn.UID
		}

		// Build a unique candidate ID. Prefer UID, fall back to name, then index.
		nodeKey := kn.UID
		if nodeKey == "" {
			nodeKey = kn.Name
		}
		if nodeKey == "" {
			nodeKey = fmt.Sprintf("idx-%d", i)
		}

		identity := ResourceIdentity{
			Hostnames: uniqueStrings([]string{nodeName}),
		}

		// TODO: K8s node identity is weak — no machine-id or MAC available from
		// the Kubernetes API. Attempt to enrich from linked host agents by hostname
		// match. Without enrichment, K8s nodes only match by hostname which falls
		// below the high-confidence threshold and stays separate (safe behavior).
		// Note: In environments with duplicate short hostnames across domains/clusters,
		// the first-match enrichment may attach the wrong identity. This is a known
		// limitation; without additional signals from the K8s API, there's no way
		// to disambiguate.
		enrichK8sNodeIdentity(&identity, nodeName, hosts)

		status := "offline"
		if kn.Ready {
			status = "online"
		}

		candidates = append(candidates, HostCandidate{
			ID:       "k8s-node:" + cluster.ID + ":" + nodeKey,
			Name:     clusterName + "/" + nodeName,
			Type:     "kubernetes",
			Source:   "kubernetes",
			Status:   status,
			LastSeen: resolvedFormatTime(cluster.LastSeen),
			Identity: identity,
		})
	}

	return candidates
}

// enrichK8sNodeIdentity attempts to find a host agent with a matching hostname
// and copies its machine-id and MAC addresses to strengthen the K8s node's identity.
func enrichK8sNodeIdentity(identity *ResourceIdentity, nodeName string, hosts []models.Host) {
	if nodeName == "" || len(hosts) == 0 {
		return
	}
	normName := NormalizeHostname(nodeName)
	if normName == "" {
		return
	}
	for _, h := range hosts {
		if NormalizeHostname(h.Hostname) == normName {
			if h.MachineID != "" && identity.MachineID == "" {
				identity.MachineID = strings.TrimSpace(h.MachineID)
			}
			_, macs := collectInterfaceIDs(h.NetworkInterfaces)
			identity.MACAddresses = uniqueStrings(append(identity.MACAddresses, macs...))
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Display-name helpers (package-local copies to avoid circular api import)
// ---------------------------------------------------------------------------

func resolvedPVENodeDisplayName(display, name, id string) string {
	if display != "" {
		return display
	}
	if name != "" {
		return name
	}
	return id
}

func resolvedHostDisplayName(display, hostname, id string) string {
	if display != "" {
		return display
	}
	if hostname != "" {
		return hostname
	}
	return id
}

func resolvedDockerDisplayName(display, custom, hostname, id string) string {
	if custom != "" {
		return custom
	}
	if display != "" {
		return display
	}
	if hostname != "" {
		return hostname
	}
	return id
}

func resolvedK8sClusterDisplayName(cluster models.KubernetesCluster) string {
	if cluster.CustomDisplayName != "" {
		return cluster.CustomDisplayName
	}
	if cluster.DisplayName != "" {
		return cluster.DisplayName
	}
	if cluster.Name != "" {
		return cluster.Name
	}
	return cluster.ID
}

func resolvedConfigDisplayName(name, host string) string {
	if name != "" {
		return name
	}
	return host
}

func resolvedNormalizeStatus(s string) string {
	switch s {
	case "online", "offline":
		return s
	default:
		return "unknown"
	}
}

func resolvedFormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// ---------------------------------------------------------------------------
// State enrichment helpers
// ---------------------------------------------------------------------------

func pbsStatusFromState(host string, state models.StateSnapshot) string {
	for _, p := range state.PBSInstances {
		if p.Host == host {
			return resolvedNormalizeStatus(p.Status)
		}
	}
	return "unknown"
}

func pbsLastSeenFromState(host string, state models.StateSnapshot) string {
	for _, p := range state.PBSInstances {
		if p.Host == host {
			return resolvedFormatTime(p.LastSeen)
		}
	}
	return ""
}

func pmgStatusFromState(host string, state models.StateSnapshot) string {
	for _, p := range state.PMGInstances {
		if p.Host == host {
			return resolvedNormalizeStatus(p.Status)
		}
	}
	return "unknown"
}

func pmgLastSeenFromState(host string, state models.StateSnapshot) string {
	for _, p := range state.PMGInstances {
		if p.Host == host {
			return resolvedFormatTime(p.LastSeen)
		}
	}
	return ""
}
