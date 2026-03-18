package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
	"github.com/rs/zerolog/log"
)

// overflowBaseDataDir is set during router initialization to allow the
// enforcement path to read billing-state overflow fields without requiring
// a handler reference.
var (
	overflowBaseDataDirMu sync.RWMutex
	overflowBaseDataDir   string

	enforcementRecorderMu sync.RWMutex
	enforcementRecorder   *conversionRecorder
	enforcementHealth     *conversionPipelineHealth
	enforcementDisableAll func() bool

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

// SetEnforcementConversionRecorder wires the conversion event recorder for
// limit_blocked events emitted from monitored-system limit enforcement.
func SetEnforcementConversionRecorder(rec *conversionRecorder, health *conversionPipelineHealth, disableAll ...func() bool) {
	enforcementRecorderMu.Lock()
	defer enforcementRecorderMu.Unlock()
	enforcementRecorder = rec
	enforcementHealth = health
	if len(disableAll) > 0 {
		enforcementDisableAll = disableAll[0]
	}
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

	// Apply onboarding overflow bonus for free-tier orgs.
	if status.Tier == licenseTierFreeValue {
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
	if monitor == nil {
		return 0
	}
	if rs := monitor.GetUnifiedReadStateOrSnapshot(); rs != nil {
		return unifiedresources.MonitoredSystemCount(rs)
	}
	return 0
}

func legacyConnectionCounts(monitor *monitoring.Monitor) legacyConnectionCountsModel {
	return legacyConnectionCountsModel{}
}

func legacyConnectionCountsFromReadState(rs unifiedresources.ReadState) legacyConnectionCountsModel {
	return legacyConnectionCountsModel{}
}

func writeMaxMonitoredSystemsLimitExceeded(w http.ResponseWriter, current, limit int) {
	WriteLicenseRequired(
		w,
		maxMonitoredSystemsLicenseGateKey,
		monitoredSystemLimitExceededMessageFromLicensing(current, limit),
	)
}

func monitoredSystemLimitState(
	ctx context.Context,
	monitor *monitoring.Monitor,
	candidate unifiedresources.MonitoredSystemCandidate,
) (current, limit int, exceeded bool) {
	limit = maxMonitoredSystemsLimitForContext(ctx)
	if limit <= 0 || monitor == nil {
		return 0, limit, false
	}

	rs := monitor.GetUnifiedReadStateOrSnapshot()
	if rs != nil && unifiedresources.HasMatchingMonitoredSystem(rs, candidate) {
		return 0, limit, false
	}

	current = monitoredSystemCount(monitor)
	reserved := deployReservedCount(ctx)
	effective := current + reserved
	return effective, limit, effective+1 > limit
}

func enforceMonitoredSystemLimitForConfigRegistration(
	w http.ResponseWriter,
	ctx context.Context,
	_ *config.Config,
	monitor *monitoring.Monitor,
	candidate unifiedresources.MonitoredSystemCandidate,
) bool {
	current, limit, exceeded := monitoredSystemLimitState(ctx, monitor, candidate)
	if !exceeded {
		return false
	}

	emitLimitBlockedEvent(ctx, current, limit)
	writeMaxMonitoredSystemsLimitExceeded(w, current, limit)
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
	if monitor == nil {
		return false
	}

	hosts := monitor.GetLiveHostsSnapshot()
	// Updates to an already-known system are always permitted.
	if hostReportTargetsExistingHost(hosts, report, tokenRecord) {
		return false
	}

	candidate := unifiedresources.MonitoredSystemCandidate{
		Type:       unifiedresources.ResourceTypeAgent,
		Name:       report.Host.DisplayName,
		Hostname:   report.Host.Hostname,
		HostURL:    report.Host.ReportIP,
		AgentID:    report.Agent.ID,
		MachineID:  report.Host.MachineID,
		ResourceID: report.Host.ID,
	}
	current, limit, exceeded := monitoredSystemLimitState(ctx, monitor, candidate)
	if !exceeded {
		return false
	}

	emitLimitBlockedEvent(ctx, current, limit)
	writeMaxMonitoredSystemsLimitExceeded(w, current, limit)
	return true
}

// emitLimitBlockedEvent fires a limit_blocked conversion event. Fire-and-forget.
// Respects the DisableLocalUpgradeMetrics config flag.
func emitLimitBlockedEvent(ctx context.Context, current, limit int) {
	enforcementRecorderMu.RLock()
	rec := enforcementRecorder
	health := enforcementHealth
	disableAll := enforcementDisableAll
	enforcementRecorderMu.RUnlock()
	if rec == nil {
		return
	}
	if disableAll != nil && disableAll() {
		return
	}

	orgID := GetOrgID(ctx)
	if orgID == "" {
		orgID = "default"
	}
	now := time.Now().UnixMilli()
	event := conversionEvent{
		Type:           conversionEventLimitBlocked,
		OrgID:          orgID,
		Surface:        "monitored_system_enforcement",
		LimitKey:       "max_monitored_systems",
		CurrentValue:   int64(current),
		LimitValue:     int64(limit),
		Timestamp:      now,
		IdempotencyKey: fmt.Sprintf("backend:%s:%s:%s:%d", orgID, conversionEventLimitBlocked, "monitored_system_enforcement", now),
	}
	if err := rec.Record(event); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("Failed to record limit_blocked conversion event")
	} else {
		recordConversionEventMetric(event.Type, event.Surface)
		if health != nil {
			health.RecordEvent(event.Type)
		}
	}
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
	if monitor == nil {
		return false
	}

	tokenID := ""
	if tokenRecord != nil {
		tokenID = tokenRecord.ID
	}
	if dockerReportTargetsExistingHostFromLicensing(monitor.ReadSnapshot(), report, tokenID) {
		return false
	}

	candidate := unifiedresources.MonitoredSystemCandidate{
		Type:       unifiedresources.ResourceTypeAgent,
		Name:       report.Host.Name,
		Hostname:   report.Host.Hostname,
		AgentID:    report.Agent.ID,
		MachineID:  report.Host.MachineID,
		ResourceID: report.AgentKey(),
	}
	current, limit, exceeded := monitoredSystemLimitState(ctx, monitor, candidate)
	if !exceeded {
		return false
	}

	emitLimitBlockedEvent(ctx, current, limit)
	writeMaxMonitoredSystemsLimitExceeded(w, current, limit)
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
	if monitor == nil {
		return false
	}

	tokenID := ""
	if tokenRecord != nil {
		tokenID = tokenRecord.ID
	}
	if kubernetesReportTargetsExistingClusterFromLicensing(monitor.ReadSnapshot(), report, tokenID) {
		return false
	}

	candidate := unifiedresources.MonitoredSystemCandidate{
		Type:       unifiedresources.ResourceTypeK8sCluster,
		Name:       report.Cluster.Name,
		Hostname:   report.Cluster.Name,
		HostURL:    report.Cluster.Server,
		AgentID:    report.Agent.ID,
		ResourceID: kubernetesReportIdentifierFromLicensing(report),
	}
	current, limit, exceeded := monitoredSystemLimitState(ctx, monitor, candidate)
	if !exceeded {
		return false
	}

	emitLimitBlockedEvent(ctx, current, limit)
	writeMaxMonitoredSystemsLimitExceeded(w, current, limit)
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
