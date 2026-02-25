package licensing

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
)

const MaxNodesLicenseGateKey = "max_nodes"

func ExceedsNodeLimit(current, additions, limit int) bool {
	if limit <= 0 || additions <= 0 {
		return false
	}
	return current+additions > limit
}

func NodeLimitExceededMessage(current, limit int) string {
	return fmt.Sprintf("Node limit reached (%d/%d). Remove a node or upgrade your license.", current, limit)
}

// ConfiguredNodeCount returns the number of host slots occupied by configured
// Proxmox infrastructure. For PVE, the count is the number of *actual nodes*
// discovered at runtime (a single PVE connection may represent a multi-node
// cluster). PBS and PMG are always 1:1 (one connection = one host).
func ConfiguredNodeCount(pveNodes, pbsInstances, pmgInstances int) int {
	return pveNodes + pbsInstances + pmgInstances
}

func RegisteredNodeSlotCount(configuredCount int, state models.StateSnapshot) int {
	count := configuredCount
	count += len(state.Hosts)
	count += len(state.DockerHosts)
	// K8s: count individual nodes across all clusters, not the cluster count.
	for _, cluster := range state.KubernetesClusters {
		n := len(cluster.Nodes)
		if n == 0 {
			// Cluster registered but node list not yet populated — count as 1.
			n = 1
		}
		count += n
	}
	return count
}

// DeduplicatedHostCount returns the number of unique physical/virtual hosts
// after cross-connector deduplication. A machine seen via multiple connectors
// (e.g. Proxmox + host agent + Docker) counts once.
func DeduplicatedHostCount(
	state models.StateSnapshot,
	configPVE []unifiedresources.ConfigEntry,
	configPBS []unifiedresources.ConfigEntry,
	configPMG []unifiedresources.ConfigEntry,
	configTrueNAS []unifiedresources.ConfigEntry,
) int {
	candidates := unifiedresources.CollectHostCandidates(state, configPVE, configPBS, configPMG, configTrueNAS)
	resolved := unifiedresources.ResolveHosts(candidates)
	return len(resolved.Hosts)
}

// DeduplicatedHosts returns the full resolved host set after cross-connector
// deduplication. Used by the host ledger API.
func DeduplicatedHosts(
	state models.StateSnapshot,
	configPVE []unifiedresources.ConfigEntry,
	configPBS []unifiedresources.ConfigEntry,
	configPMG []unifiedresources.ConfigEntry,
	configTrueNAS []unifiedresources.ConfigEntry,
) *unifiedresources.ResolvedHostSet {
	candidates := unifiedresources.CollectHostCandidates(state, configPVE, configPBS, configPMG, configTrueNAS)
	return unifiedresources.ResolveHosts(candidates)
}

func HostReportTargetsExistingHost(
	snapshot models.StateSnapshot,
	report agentshost.Report,
	tokenID string,
) bool {
	hostname := strings.TrimSpace(report.Host.Hostname)
	tokenID = strings.TrimSpace(tokenID)

	candidates := CollectNonEmptyStrings(
		report.Host.ID,
		report.Host.MachineID,
		report.Agent.ID,
	)

	for _, existing := range snapshot.Hosts {
		for _, candidate := range candidates {
			if existing.ID == candidate || existing.MachineID == candidate {
				return true
			}
		}

		if hostname != "" && strings.EqualFold(existing.Hostname, hostname) {
			// Token-bound identity takes precedence when token is present.
			if tokenID == "" || (existing.TokenID != "" && existing.TokenID == tokenID) {
				return true
			}
		}
	}

	return false
}

func DockerReportTargetsExistingHost(snapshot models.StateSnapshot, report agentsdocker.Report, tokenID string) bool {
	hostname := strings.TrimSpace(report.Host.Hostname)
	agentKey := strings.TrimSpace(report.AgentKey())
	tokenID = strings.TrimSpace(tokenID)

	for _, existing := range snapshot.DockerHosts {
		if agentKey != "" && (existing.ID == agentKey || existing.AgentID == agentKey || existing.MachineID == agentKey) {
			return true
		}

		if hostname != "" && strings.EqualFold(existing.Hostname, hostname) {
			if tokenID == "" || (existing.TokenID != "" && existing.TokenID == tokenID) {
				return true
			}
		}
	}

	return false
}

func KubernetesReportTargetsExistingCluster(snapshot models.StateSnapshot, report agentsk8s.Report, tokenID string) bool {
	identifier := KubernetesReportIdentifier(report)
	agentID := strings.TrimSpace(report.Agent.ID)
	clusterName := strings.TrimSpace(report.Cluster.Name)
	tokenID = strings.TrimSpace(tokenID)

	for _, existing := range snapshot.KubernetesClusters {
		if identifier != "" && existing.ID == identifier {
			return true
		}
		if agentID != "" && existing.AgentID == agentID {
			return true
		}

		if tokenID != "" && existing.TokenID == tokenID {
			if identifier == "" || existing.ID == identifier {
				return true
			}
			if clusterName != "" && strings.EqualFold(existing.Name, clusterName) {
				return true
			}
		}

		if tokenID == "" && identifier == "" && clusterName != "" && strings.EqualFold(existing.Name, clusterName) {
			return true
		}
	}

	return false
}

func KubernetesReportIdentifier(report agentsk8s.Report) string {
	if v := strings.TrimSpace(report.Cluster.ID); v != "" {
		return v
	}
	if v := strings.TrimSpace(report.Agent.ID); v != "" {
		return v
	}
	stableKey := strings.TrimSpace(report.Cluster.Server) + "|" + strings.TrimSpace(report.Cluster.Context) + "|" + strings.TrimSpace(report.Cluster.Name)
	stableKey = strings.TrimSpace(stableKey)
	if stableKey == "||" || stableKey == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(stableKey))
	return hex.EncodeToString(sum[:])
}

func CollectNonEmptyStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
