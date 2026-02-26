package licensing

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
)

const MaxAgentsLicenseGateKey = "max_agents"

// MaxNodesLicenseGateKey is the legacy key used in billing.json and license JWTs
// before the node→agent rename. Kept for backwards-compat deserialization only.
const MaxNodesLicenseGateKey = "max_nodes"

func ExceedsAgentLimit(current, additions, limit int) bool {
	if limit <= 0 || additions <= 0 {
		return false
	}
	return current+additions > limit
}

func AgentLimitExceededMessage(current, limit int) string {
	return fmt.Sprintf("Agent limit reached (%d/%d). Remove an agent or upgrade your plan.", current, limit)
}

// AgentCount returns the number of installed Pulse Unified Agents.
// This is the only thing that counts toward the host/agent limit.
// One installed agent = one host. Resources discovered by the agent
// (VMs, containers, cluster nodes, pods, PVE/PBS/PMG) don't count separately.
func AgentCount(state models.StateSnapshot) int {
	return len(state.Hosts)
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
