package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
)

const maxNodesLicenseGateKey = "max_nodes"

func maxNodesLimitForContext(ctx context.Context) int {
	service := getLicenseServiceForContext(ctx)
	if service == nil {
		return 0
	}
	status := service.Status()
	if status == nil {
		return 0
	}
	return status.MaxNodes
}

func configuredNodeCount(cfg *config.Config) int {
	if cfg == nil {
		return 0
	}
	return len(cfg.PVEInstances) + len(cfg.PBSInstances) + len(cfg.PMGInstances)
}

func registeredNodeSlotCount(cfg *config.Config, monitor *monitoring.Monitor) int {
	count := configuredNodeCount(cfg)
	if monitor == nil {
		return count
	}

	state := monitor.GetLiveStateSnapshot()
	count += len(state.Hosts)
	count += len(state.DockerHosts)
	count += len(state.KubernetesClusters)
	return count
}

func writeMaxNodesLimitExceeded(w http.ResponseWriter, current, limit int) {
	WriteLicenseRequired(
		w,
		maxNodesLicenseGateKey,
		fmt.Sprintf("Node limit reached (%d/%d). Remove a node or upgrade your license.", current, limit),
	)
}

func enforceNodeLimitForConfigRegistration(
	w http.ResponseWriter,
	ctx context.Context,
	cfg *config.Config,
	monitor *monitoring.Monitor,
) bool {
	limit := maxNodesLimitForContext(ctx)
	if limit <= 0 {
		return false
	}

	current := registeredNodeSlotCount(cfg, monitor)
	if current+1 <= limit {
		return false
	}

	writeMaxNodesLimitExceeded(w, current, limit)
	return true
}

// enforceNodeLimitForReport is the shared implementation for all report-type node limit checks.
// targetsExisting should return true if the report corresponds to an already-registered node.
func enforceNodeLimitForReport(
	w http.ResponseWriter,
	ctx context.Context,
	monitor *monitoring.Monitor,
	targetsExisting func(models.StateSnapshot) bool,
) bool {
	limit := maxNodesLimitForContext(ctx)
	if limit <= 0 || monitor == nil {
		return false
	}

	snapshot := monitor.GetLiveStateSnapshot()
	if targetsExisting(snapshot) {
		return false
	}

	current := registeredNodeSlotCount(monitor.GetConfig(), monitor)
	if current+1 <= limit {
		return false
	}

	writeMaxNodesLimitExceeded(w, current, limit)
	return true
}

func enforceNodeLimitForHostReport(
	w http.ResponseWriter,
	ctx context.Context,
	monitor *monitoring.Monitor,
	report agentshost.Report,
	tokenRecord *config.APITokenRecord,
) bool {
	return enforceNodeLimitForReport(w, ctx, monitor, func(snapshot models.StateSnapshot) bool {
		return hostReportTargetsExistingHost(snapshot, report, tokenRecord)
	})
}

func enforceNodeLimitForDockerReport(
	w http.ResponseWriter,
	ctx context.Context,
	monitor *monitoring.Monitor,
	report agentsdocker.Report,
	tokenRecord *config.APITokenRecord,
) bool {
	return enforceNodeLimitForReport(w, ctx, monitor, func(snapshot models.StateSnapshot) bool {
		return dockerReportTargetsExistingHost(snapshot, report, tokenRecord)
	})
}

func enforceNodeLimitForKubernetesReport(
	w http.ResponseWriter,
	ctx context.Context,
	monitor *monitoring.Monitor,
	report agentsk8s.Report,
	tokenRecord *config.APITokenRecord,
) bool {
	return enforceNodeLimitForReport(w, ctx, monitor, func(snapshot models.StateSnapshot) bool {
		return kubernetesReportTargetsExistingCluster(snapshot, report, tokenRecord)
	})
}

func hostReportTargetsExistingHost(
	snapshot models.StateSnapshot,
	report agentshost.Report,
	tokenRecord *config.APITokenRecord,
) bool {
	hostname := strings.TrimSpace(report.Host.Hostname)
	tokenID := ""
	if tokenRecord != nil {
		tokenID = strings.TrimSpace(tokenRecord.ID)
	}

	candidates := collectNonEmptyStrings(
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

func dockerReportTargetsExistingHost(snapshot models.StateSnapshot, report agentsdocker.Report, tokenRecord *config.APITokenRecord) bool {
	hostname := strings.TrimSpace(report.Host.Hostname)
	agentKey := strings.TrimSpace(report.AgentKey())
	tokenID := ""
	if tokenRecord != nil {
		tokenID = strings.TrimSpace(tokenRecord.ID)
	}

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

func kubernetesReportTargetsExistingCluster(snapshot models.StateSnapshot, report agentsk8s.Report, tokenRecord *config.APITokenRecord) bool {
	identifier := kubernetesReportIdentifier(report)
	agentID := strings.TrimSpace(report.Agent.ID)
	clusterName := strings.TrimSpace(report.Cluster.Name)
	tokenID := ""
	if tokenRecord != nil {
		tokenID = strings.TrimSpace(tokenRecord.ID)
	}

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

func kubernetesReportIdentifier(report agentsk8s.Report) string {
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

func collectNonEmptyStrings(values ...string) []string {
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
