package api

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
)

// overflowBaseDataDir is set during router initialization to allow the
// enforcement path to read billing-state overflow fields without requiring
// a handler reference.
var (
	overflowBaseDataDirMu sync.RWMutex
	overflowBaseDataDir   string

	deployReservationCounterMu sync.RWMutex
	deployReservationCounter   func(ctx context.Context) int
)

// SetDeployReservationCounter wires a callback that returns the number of
// monitored-system slots reserved by in-flight cluster deployments for the org
// in ctx.
func SetDeployReservationCounter(fn func(ctx context.Context) int) {
	deployReservationCounterMu.Lock()
	defer deployReservationCounterMu.Unlock()
	deployReservationCounter = fn
}

func deployReservedCount(ctx context.Context) int {
	deployReservationCounterMu.RLock()
	fn := deployReservationCounter
	deployReservationCounterMu.RUnlock()
	if fn == nil {
		return 0
	}
	return fn(ctx)
}

// SetOverflowBaseDataDir configures the base data directory used by the
// enforcement path to read OverflowGrantedAt from billing state.
func SetOverflowBaseDataDir(dir string) {
	overflowBaseDataDirMu.Lock()
	defer overflowBaseDataDirMu.Unlock()
	overflowBaseDataDir = dir
}

func maxMonitoredSystemsLimitForContext(ctx context.Context) int {
	service := getLicenseServiceForContext(ctx)
	if service == nil {
		return 0
	}
	status := service.Status()
	if status == nil {
		return 0
	}

	limit := status.MaxMonitoredSystems

	// Apply onboarding overflow bonus for free-tier orgs. The bonus only
	// makes sense on plans that actually have a cap — adding +1 to an
	// uncapped limit (0) would convert "unlimited" into a cap of 1.
	if status.Tier == licenseTierFreeValue && limit > 0 {
		var overflowGrantedAt *int64

		// Try evaluator first (covers hosted path with DatabaseSource).
		if eval := service.Evaluator(); eval != nil {
			overflowGrantedAt = eval.OverflowGrantedAt()
		}

		// Self-hosted fallback: read from billing state on disk.
		if overflowGrantedAt == nil {
			overflowBaseDataDirMu.RLock()
			baseDir := overflowBaseDataDir
			overflowBaseDataDirMu.RUnlock()

			if baseDir != "" {
				orgID := GetOrgID(ctx)
				if orgID == "" {
					orgID = "default"
				}
				store := config.NewFileBillingStore(baseDir)
				if bs, err := store.GetBillingState(orgID); err == nil && bs != nil {
					overflowGrantedAt = bs.OverflowGrantedAt
				}
			}
		}

		limit += overflowBonusFromLicensing(status.Tier, overflowGrantedAt, time.Now())
	}

	return limit
}

// monitoredSystemCount returns the canonical number of counted top-level
// monitored systems. Agent-backed and API-backed views share the same cap.
func monitoredSystemCount(monitor *monitoring.Monitor) int {
	return monitoredSystemUsage(monitor).count
}

func monitoredSystemCandidateStateFromEnabled(
	enabled bool,
) unifiedresources.MonitoredSystemCandidateState {
	if enabled {
		return unifiedresources.MonitoredSystemCandidateStateActive
	}
	return unifiedresources.MonitoredSystemCandidateStateInactive
}

type monitoredSystemUsageSnapshot struct {
	count             int
	readState         unifiedresources.ReadState
	available         bool
	unavailableReason string
}

func monitoredSystemUsage(monitor *monitoring.Monitor) monitoredSystemUsageSnapshot {
	usage := monitor.MonitoredSystemUsage()
	return monitoredSystemUsageSnapshot{
		count:             usage.Count,
		readState:         usage.ReadState,
		available:         usage.Available,
		unavailableReason: usage.UnavailableReason,
	}
}

type monitoredSystemLimitDecision struct {
	current                int
	limit                  int
	additional             int
	usageAvailable         bool
	usageUnavailableReason string
	exceeded               bool
	preview                *MonitoredSystemLedgerPreviewResponse
}

func legacyConnectionCounts(monitor *monitoring.Monitor) legacyConnectionCountsModel {
	return legacyConnectionCountsModel{}
}

func legacyConnectionCountsFromReadState(rs unifiedresources.ReadState) legacyConnectionCountsModel {
	return legacyConnectionCountsModel{}
}

func monitoredSystemLimitExceededPayload(decision monitoredSystemLimitDecision) map[string]interface{} {
	payload := map[string]interface{}{
		"error":       "license_required",
		"message":     monitoredSystemLimitExceededMessageFromLicensing(decision.current, decision.limit),
		"feature":     maxMonitoredSystemsLicenseGateKey,
		"upgrade_url": upgradeURLForFeatureFromLicensing(maxMonitoredSystemsLicenseGateKey),
	}
	if decision.preview != nil {
		payload["monitored_system_preview"] = decision.preview.NormalizeCollections()
	}
	return payload
}

func writeMaxMonitoredSystemsLimitExceeded(w http.ResponseWriter, decision monitoredSystemLimitDecision) {
	writePaymentRequired(w, monitoredSystemLimitExceededPayload(decision))
}

func writeMonitoredSystemUsageUnavailable(w http.ResponseWriter, reason string) {
	details := map[string]string{}
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		details["reason"] = trimmed
	}
	writeErrorResponse(
		w,
		http.StatusServiceUnavailable,
		"monitored_system_usage_unavailable",
		"Unable to verify monitored-system capacity right now",
		details,
	)
}

func monitoredSystemLimitDecisionFromAdditional(
	ctx context.Context,
	limit int,
	current int,
	additional int,
) monitoredSystemLimitDecision {
	if limit <= 0 {
		return monitoredSystemLimitDecision{
			limit:          limit,
			additional:     additional,
			usageAvailable: true,
		}
	}

	effectiveCurrent := current + deployReservedCount(ctx)
	return monitoredSystemLimitDecision{
		current:        effectiveCurrent,
		limit:          limit,
		additional:     additional,
		usageAvailable: true,
		exceeded:       additional > 0 && effectiveCurrent+additional > limit,
	}
}

func monitoredSystemLimitDecisionFromPreview(
	ctx context.Context,
	limit int,
	hasReplacement bool,
	preview unifiedresources.MonitoredSystemProjectionPreview,
) monitoredSystemLimitDecision {
	decision := monitoredSystemLimitDecisionFromAdditional(
		ctx,
		limit,
		preview.CurrentCount,
		preview.AdditionalCount,
	)
	resp := monitoredSystemLedgerPreviewResponse(ctx, hasReplacement, preview).NormalizeCollections()
	decision.preview = &resp
	return decision
}

func monitoredSystemLimitDecisionForCandidate(
	ctx context.Context,
	monitor *monitoring.Monitor,
	candidate unifiedresources.MonitoredSystemCandidate,
) monitoredSystemLimitDecision {
	limit := maxMonitoredSystemsLimitForContext(ctx)
	if !candidate.CountsTowardMonitoredSystems() {
		return monitoredSystemLimitDecision{
			limit:          limit,
			usageAvailable: true,
		}
	}
	if limit <= 0 {
		return monitoredSystemLimitDecision{
			limit:          limit,
			usageAvailable: true,
		}
	}

	usage := monitoredSystemUsage(monitor)
	if !usage.available {
		return monitoredSystemLimitDecision{
			limit:                  limit,
			usageUnavailableReason: usage.unavailableReason,
		}
	}

	projection := unifiedresources.PreviewMonitoredSystemCandidate(usage.readState, candidate)
	return monitoredSystemLimitDecisionFromPreview(ctx, limit, false, projection)
}

func monitoredSystemLimitDecisionForCandidateReplacement(
	ctx context.Context,
	monitor *monitoring.Monitor,
	replacement unifiedresources.MonitoredSystemReplacement,
	candidate unifiedresources.MonitoredSystemCandidate,
) monitoredSystemLimitDecision {
	limit := maxMonitoredSystemsLimitForContext(ctx)
	if !candidate.CountsTowardMonitoredSystems() {
		return monitoredSystemLimitDecision{
			limit:          limit,
			usageAvailable: true,
		}
	}
	if limit <= 0 {
		return monitoredSystemLimitDecision{
			limit:          limit,
			usageAvailable: true,
		}
	}

	usage := monitoredSystemUsage(monitor)
	if !usage.available {
		return monitoredSystemLimitDecision{
			limit:                  limit,
			usageUnavailableReason: usage.unavailableReason,
		}
	}

	projection := unifiedresources.PreviewMonitoredSystemCandidateReplacement(
		usage.readState,
		replacement,
		candidate,
	)
	return monitoredSystemLimitDecisionFromPreview(ctx, limit, true, projection)
}

func monitoredSystemLimitDecisionForRecordsFromUsage(
	ctx context.Context,
	limit int,
	usage monitoredSystemUsageSnapshot,
	recordsBySource map[unifiedresources.DataSource][]unifiedresources.IngestRecord,
) monitoredSystemLimitDecision {
	if limit <= 0 {
		return monitoredSystemLimitDecision{
			limit:          limit,
			usageAvailable: true,
		}
	}

	if !usage.available {
		return monitoredSystemLimitDecision{
			limit:                  limit,
			usageUnavailableReason: usage.unavailableReason,
		}
	}

	projection := unifiedresources.PreviewMonitoredSystemRecords(usage.readState, recordsBySource)
	return monitoredSystemLimitDecisionFromPreview(ctx, limit, false, projection)
}

func monitoredSystemLimitDecisionForRecordsReplacementFromUsage(
	ctx context.Context,
	limit int,
	usage monitoredSystemUsageSnapshot,
	replacement unifiedresources.MonitoredSystemReplacement,
	recordsBySource map[unifiedresources.DataSource][]unifiedresources.IngestRecord,
) monitoredSystemLimitDecision {
	if limit <= 0 {
		return monitoredSystemLimitDecision{
			limit:          limit,
			usageAvailable: true,
		}
	}

	if !usage.available {
		return monitoredSystemLimitDecision{
			limit:                  limit,
			usageUnavailableReason: usage.unavailableReason,
		}
	}

	projection := unifiedresources.PreviewMonitoredSystemRecordsReplacement(
		usage.readState,
		replacement,
		recordsBySource,
	)
	return monitoredSystemLimitDecisionFromPreview(ctx, limit, true, projection)
}

func monitoredSystemLimitDecisionForAdditionalSlots(
	ctx context.Context,
	monitor *monitoring.Monitor,
	additional int,
) monitoredSystemLimitDecision {
	limit := maxMonitoredSystemsLimitForContext(ctx)
	if limit <= 0 {
		return monitoredSystemLimitDecision{
			limit:          limit,
			additional:     additional,
			usageAvailable: true,
		}
	}

	usage := monitoredSystemUsage(monitor)
	if !usage.available {
		return monitoredSystemLimitDecision{
			limit:                  limit,
			usageUnavailableReason: usage.unavailableReason,
		}
	}

	return monitoredSystemLimitDecisionFromAdditional(ctx, limit, usage.count, additional)
}

func enforceMonitoredSystemLimitForConfigRegistration(
	w http.ResponseWriter,
	ctx context.Context,
	_ *config.Config,
	monitor *monitoring.Monitor,
	candidate unifiedresources.MonitoredSystemCandidate,
) bool {
	decision := monitoredSystemLimitDecisionForCandidate(ctx, monitor, candidate)
	if !decision.usageAvailable {
		writeMonitoredSystemUsageUnavailable(w, decision.usageUnavailableReason)
		return true
	}
	if !decision.exceeded {
		return false
	}

	writeMaxMonitoredSystemsLimitExceeded(w, decision)
	return true
}

func enforceMonitoredSystemLimitForConfigReplacement(
	w http.ResponseWriter,
	ctx context.Context,
	monitor *monitoring.Monitor,
	replacement unifiedresources.MonitoredSystemReplacement,
	candidate unifiedresources.MonitoredSystemCandidate,
) bool {
	decision := monitoredSystemLimitDecisionForCandidateReplacement(ctx, monitor, replacement, candidate)
	if !decision.usageAvailable {
		writeMonitoredSystemUsageUnavailable(w, decision.usageUnavailableReason)
		return true
	}
	if !decision.exceeded {
		return false
	}

	writeMaxMonitoredSystemsLimitExceeded(w, decision)
	return true
}

// enforceMonitoredSystemLimitForHostReport checks whether a new host report
// would create a new counted monitored system.
func enforceMonitoredSystemLimitForHostReport(
	w http.ResponseWriter,
	ctx context.Context,
	monitor *monitoring.Monitor,
	report agentshost.Report,
	tokenRecord *config.APITokenRecord,
) bool {
	if monitor != nil && monitor.HostReportMatchesKnownIdentity(report, tokenRecord) {
		return false
	}

	candidate := unifiedresources.MonitoredSystemCandidate{
		Source:     unifiedresources.SourceAgent,
		Type:       unifiedresources.ResourceTypeAgent,
		Name:       report.Host.DisplayName,
		Hostname:   report.Host.Hostname,
		HostURL:    report.Host.ReportIP,
		AgentID:    report.Agent.ID,
		MachineID:  report.Host.MachineID,
		ResourceID: report.Host.ID,
	}
	decision := monitoredSystemLimitDecisionForCandidate(ctx, monitor, candidate)
	if !decision.usageAvailable {
		writeMonitoredSystemUsageUnavailable(w, decision.usageUnavailableReason)
		return true
	}
	if !decision.exceeded {
		return false
	}

	writeMaxMonitoredSystemsLimitExceeded(w, decision)
	return true
}

// enforceMonitoredSystemLimitForDockerReport checks whether a docker report
// would create a new counted top-level monitored system.
func enforceMonitoredSystemLimitForDockerReport(
	w http.ResponseWriter,
	ctx context.Context,
	monitor *monitoring.Monitor,
	report agentsdocker.Report,
	tokenRecord *config.APITokenRecord,
) bool {
	tokenID := ""
	if tokenRecord != nil {
		tokenID = tokenRecord.ID
	}
	if monitor != nil && dockerReportTargetsExistingHostFromLicensing(monitor.ReadSnapshot(), report, tokenID) {
		return false
	}

	candidate := unifiedresources.MonitoredSystemCandidate{
		Source:     unifiedresources.SourceDocker,
		Type:       unifiedresources.ResourceTypeAgent,
		Name:       report.Host.Name,
		Hostname:   report.Host.Hostname,
		AgentID:    report.Agent.ID,
		MachineID:  report.Host.MachineID,
		ResourceID: report.AgentKey(),
	}
	decision := monitoredSystemLimitDecisionForCandidate(ctx, monitor, candidate)
	if !decision.usageAvailable {
		writeMonitoredSystemUsageUnavailable(w, decision.usageUnavailableReason)
		return true
	}
	if !decision.exceeded {
		return false
	}

	writeMaxMonitoredSystemsLimitExceeded(w, decision)
	return true
}

// enforceMonitoredSystemLimitForKubernetesReport checks whether a Kubernetes
// report would create a new counted cluster.
func enforceMonitoredSystemLimitForKubernetesReport(
	w http.ResponseWriter,
	ctx context.Context,
	monitor *monitoring.Monitor,
	report agentsk8s.Report,
	tokenRecord *config.APITokenRecord,
) bool {
	tokenID := ""
	if tokenRecord != nil {
		tokenID = tokenRecord.ID
	}
	if monitor != nil && kubernetesReportTargetsExistingClusterFromLicensing(monitor.ReadSnapshot(), report, tokenID) {
		return false
	}

	candidate := unifiedresources.MonitoredSystemCandidate{
		Source:     unifiedresources.SourceK8s,
		Type:       unifiedresources.ResourceTypeK8sCluster,
		Name:       report.Cluster.Name,
		Hostname:   report.Cluster.Name,
		HostURL:    report.Cluster.Server,
		AgentID:    report.Agent.ID,
		ResourceID: kubernetesReportIdentifierFromLicensing(report),
	}
	decision := monitoredSystemLimitDecisionForCandidate(ctx, monitor, candidate)
	if !decision.usageAvailable {
		writeMonitoredSystemUsageUnavailable(w, decision.usageUnavailableReason)
		return true
	}
	if !decision.exceeded {
		return false
	}

	writeMaxMonitoredSystemsLimitExceeded(w, decision)
	return true
}

func hostReportTargetsExistingHost(
	hosts []models.Host,
	report agentshost.Report,
	tokenRecord *config.APITokenRecord,
) bool {
	tokenID := ""
	if tokenRecord != nil {
		tokenID = tokenRecord.ID
	}
	return hostReportTargetsExistingHostsFromLicensing(hosts, report, tokenID)
}
