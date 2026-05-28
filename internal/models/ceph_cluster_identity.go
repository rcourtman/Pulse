package models

import (
	"sort"
	"strings"
)

const (
	CephClusterSourceProxmoxAPI = "proxmox-api"
	CephClusterSourceHostAgent  = "host-agent"
)

func normalizeCephClusterForState(cluster CephCluster, defaultSource string) CephCluster {
	cluster.ID = strings.TrimSpace(cluster.ID)
	cluster.Instance = strings.TrimSpace(cluster.Instance)
	cluster.FSID = strings.TrimSpace(cluster.FSID)
	cluster.Source = normalizeCephClusterSource(cluster.Source, cluster.Instance, defaultSource)
	cluster.InstanceAliases = normalizeCephInstanceAliases(cluster.Instance, cluster.InstanceAliases)
	return cluster
}

func normalizeCephClusterSource(source, instance, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case CephClusterSourceProxmoxAPI, "proxmox", "pve", "api":
		return CephClusterSourceProxmoxAPI
	case CephClusterSourceHostAgent, "agent", "host":
		return CephClusterSourceHostAgent
	}

	if fallback != "" {
		return normalizeCephClusterSource(fallback, instance, "")
	}
	if strings.HasPrefix(strings.TrimSpace(instance), "agent:") {
		return CephClusterSourceHostAgent
	}
	return CephClusterSourceProxmoxAPI
}

func normalizeCephInstanceAliases(instance string, aliases []string) []string {
	instance = strings.TrimSpace(instance)
	out := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" || alias == instance || containsString(out, alias) {
			continue
		}
		out = append(out, alias)
	}
	return out
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func cephClusterSourceRank(cluster CephCluster) int {
	switch normalizeCephClusterSource(cluster.Source, cluster.Instance, "") {
	case CephClusterSourceProxmoxAPI:
		return 2
	case CephClusterSourceHostAgent:
		return 1
	default:
		return 0
	}
}

func cephClusterFSIDKey(cluster CephCluster) string {
	return strings.ToLower(strings.TrimSpace(cluster.FSID))
}

func cephClustersSamePhysicalCluster(left, right CephCluster) bool {
	leftFSID := cephClusterFSIDKey(left)
	rightFSID := cephClusterFSIDKey(right)
	return leftFSID != "" && leftFSID == rightFSID
}

func mergeCephClusterForState(existing, incoming CephCluster) CephCluster {
	existing = normalizeCephClusterForState(existing, "")
	incoming = normalizeCephClusterForState(incoming, "")
	if !cephClustersSamePhysicalCluster(existing, incoming) {
		return incoming
	}

	primary := existing
	supplemental := incoming
	existingRank := cephClusterSourceRank(existing)
	incomingRank := cephClusterSourceRank(incoming)
	if incomingRank > existingRank || (incomingRank == existingRank && (incoming.ID == existing.ID || incoming.Instance == existing.Instance)) {
		primary = incoming
		supplemental = existing
	}

	return supplementCephCluster(primary, supplemental)
}

func supplementCephCluster(primary, supplemental CephCluster) CephCluster {
	aliases := append([]string(nil), primary.InstanceAliases...)
	if supplemental.Instance != "" {
		aliases = append(aliases, supplemental.Instance)
	}
	aliases = append(aliases, supplemental.InstanceAliases...)
	primary.InstanceAliases = normalizeCephInstanceAliases(primary.Instance, aliases)

	if primary.Health == "" {
		primary.Health = supplemental.Health
	}
	if primary.HealthMessage == "" {
		primary.HealthMessage = supplemental.HealthMessage
	}
	if primary.TotalBytes == 0 && supplemental.TotalBytes != 0 {
		primary.TotalBytes = supplemental.TotalBytes
		primary.UsedBytes = supplemental.UsedBytes
		primary.AvailableBytes = supplemental.AvailableBytes
		primary.UsagePercent = supplemental.UsagePercent
	}
	if primary.NumMons == 0 {
		primary.NumMons = supplemental.NumMons
	}
	if primary.NumMgrs == 0 {
		primary.NumMgrs = supplemental.NumMgrs
	}
	if primary.NumOSDs == 0 {
		primary.NumOSDs = supplemental.NumOSDs
		primary.NumOSDsUp = supplemental.NumOSDsUp
		primary.NumOSDsIn = supplemental.NumOSDsIn
	}
	if primary.NumPGs == 0 {
		primary.NumPGs = supplemental.NumPGs
	}
	if len(primary.Pools) == 0 && len(supplemental.Pools) > 0 {
		primary.Pools = append([]CephPool(nil), supplemental.Pools...)
	}
	if len(primary.Services) == 0 && len(supplemental.Services) > 0 {
		primary.Services = append([]CephServiceStatus(nil), supplemental.Services...)
	}
	if primary.LastUpdated.IsZero() {
		primary.LastUpdated = supplemental.LastUpdated
	}
	return primary
}

func upsertCephClusterInSlice(clusters []CephCluster, cluster CephCluster) ([]CephCluster, CephCluster) {
	cluster = normalizeCephClusterForState(cluster, "")
	for i, existing := range clusters {
		existing = normalizeCephClusterForState(existing, "")
		if existing.ID == cluster.ID {
			if cephClustersSamePhysicalCluster(existing, cluster) {
				cluster = mergeCephClusterForState(existing, cluster)
			}
			clusters[i] = cluster
			return clusters, cluster
		}
		if cephClustersSamePhysicalCluster(existing, cluster) {
			cluster = mergeCephClusterForState(existing, cluster)
			clusters[i] = cluster
			return clusters, cluster
		}
	}
	return append(clusters, cluster), cluster
}

func sortCephClustersForState(clusters []CephCluster) {
	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].Instance == clusters[j].Instance {
			if clusters[i].Name == clusters[j].Name {
				return clusters[i].ID < clusters[j].ID
			}
			return clusters[i].Name < clusters[j].Name
		}
		return clusters[i].Instance < clusters[j].Instance
	})
}
