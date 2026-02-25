package api

import (
	"context"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
)

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

func configuredNodeCount(cfg *config.Config, state models.StateSnapshot) int {
	if cfg == nil {
		return 0
	}
	// PVE: count actual discovered nodes (a single connection may be a multi-node cluster).
	// Fall back to connection count when the monitor has not populated nodes yet.
	pveCount := len(state.Nodes)
	if pveCount == 0 {
		pveCount = len(cfg.PVEInstances)
	}
	return configuredNodeCountFromLicensing(pveCount, len(cfg.PBSInstances), len(cfg.PMGInstances))
}

func registeredNodeSlotCount(cfg *config.Config, monitor *monitoring.Monitor) int {
	if monitor == nil {
		return configuredNodeCount(cfg, models.StateSnapshot{})
	}
	state := monitor.GetLiveStateSnapshot()
	count := configuredNodeCount(cfg, state)
	return registeredNodeSlotCountFromLicensing(count, state)
}

func writeMaxNodesLimitExceeded(w http.ResponseWriter, current, limit int) {
	WriteLicenseRequired(
		w,
		maxNodesLicenseGateKey,
		nodeLimitExceededMessageFromLicensing(current, limit),
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
	if !exceedsNodeLimitFromLicensing(current, 1, limit) {
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
	if !exceedsNodeLimitFromLicensing(current, 1, limit) {
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
	tokenID := ""
	if tokenRecord != nil {
		tokenID = tokenRecord.ID
	}
	return hostReportTargetsExistingHostFromLicensing(snapshot, report, tokenID)
}

func dockerReportTargetsExistingHost(snapshot models.StateSnapshot, report agentsdocker.Report, tokenRecord *config.APITokenRecord) bool {
	tokenID := ""
	if tokenRecord != nil {
		tokenID = tokenRecord.ID
	}
	return dockerReportTargetsExistingHostFromLicensing(snapshot, report, tokenID)
}

func kubernetesReportTargetsExistingCluster(snapshot models.StateSnapshot, report agentsk8s.Report, tokenRecord *config.APITokenRecord) bool {
	tokenID := ""
	if tokenRecord != nil {
		tokenID = tokenRecord.ID
	}
	return kubernetesReportTargetsExistingClusterFromLicensing(snapshot, report, tokenID)
}

func kubernetesReportIdentifier(report agentsk8s.Report) string {
	return kubernetesReportIdentifierFromLicensing(report)
}

func collectNonEmptyStrings(values ...string) []string {
	return collectNonEmptyStringsFromLicensing(values...)
}
