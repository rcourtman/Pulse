package models

import (
	"sort"
	"strings"
)

// CephClusterFSIDKey returns the key used to collapse the same physical Ceph
// cluster reported by more than one source. The FSID is globally unique per
// cluster; we fall back to the cluster ID only when an FSID is unavailable.
func CephClusterFSIDKey(c CephCluster) string {
	if fsid := strings.TrimSpace(c.FSID); fsid != "" {
		return fsid
	}
	return strings.TrimSpace(c.ID)
}

// cephClusterIsAgentSourced reports whether a cluster originated from a Pulse
// host-agent report rather than direct Proxmox API polling. Agent-sourced
// clusters carry an "agent:" instance prefix (see
// convertAgentCephToGlobalCluster in the monitoring package).
func cephClusterIsAgentSourced(c CephCluster) bool {
	return strings.HasPrefix(strings.TrimSpace(c.Instance), "agent:")
}

// cephClusterCompletenessScore approximates how much detail a source reported
// for a cluster. The Proxmox API path reports monitors, managers and full pool
// stats; a host-agent may report a subset.
func cephClusterCompletenessScore(c CephCluster) int {
	return c.NumMons + c.NumMgrs + len(c.Pools)
}

// cephClusterPreferred reports whether candidate should win over incumbent for
// the same FSID. See DedupeCephClusters for the ordering rationale.
func cephClusterPreferred(candidate, incumbent CephCluster) bool {
	// 1. A non-agent (Proxmox API) source is authoritative and its pool IDs
	//    match the polling path that already runs the alert checks.
	candAgent := cephClusterIsAgentSourced(candidate)
	incAgent := cephClusterIsAgentSourced(incumbent)
	if candAgent != incAgent {
		return !candAgent
	}

	// 2. Within the same class, prefer the more complete report.
	candScore := cephClusterCompletenessScore(candidate)
	incScore := cephClusterCompletenessScore(incumbent)
	if candScore != incScore {
		return candScore > incScore
	}

	// 3. Deterministic, stable tie-break so the winner never oscillates.
	if candidate.Instance != incumbent.Instance {
		return candidate.Instance < incumbent.Instance
	}
	return candidate.ID < incumbent.ID
}

// DedupeCephClusters collapses Ceph clusters that share an FSID to a single
// deterministic representation. The same physical cluster can be reported by
// both the Proxmox API poller (instance "<name>") and a Pulse host-agent
// (instance "agent:<host>"); left un-collapsed this produces two pool-ID
// namespaces, which drove duplicate/flapping alerts and a per-pool threshold
// value that flipped between the two identities (#1341).
//
// Every consumer — the frontend snapshot AND alert evaluation — must run on the
// same single identity per pool, so selection here is deterministic and stable
// rather than dependent on a fluctuating completeness score.
func DedupeCephClusters(clusters []CephCluster) []CephCluster {
	if len(clusters) <= 1 {
		return clusters
	}

	winners := make(map[string]CephCluster, len(clusters))
	order := make([]string, 0, len(clusters))
	for _, cluster := range clusters {
		key := CephClusterFSIDKey(cluster)
		existing, exists := winners[key]
		if !exists {
			winners[key] = cluster
			order = append(order, key)
			continue
		}
		if cephClusterPreferred(cluster, existing) {
			winners[key] = cluster
		}
	}

	deduped := make([]CephCluster, 0, len(order))
	for _, key := range order {
		deduped = append(deduped, winners[key])
	}
	sort.Slice(deduped, func(i, j int) bool {
		if deduped[i].Name != deduped[j].Name {
			return deduped[i].Name < deduped[j].Name
		}
		return deduped[i].ID < deduped[j].ID
	})
	return deduped
}
