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
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
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
// license slots reserved by in-flight cluster deployments for the org in ctx.
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
// limit_blocked events emitted from agent limit enforcement.
func SetEnforcementConversionRecorder(rec *conversionRecorder, health *conversionPipelineHealth, disableAll ...func() bool) {
	enforcementRecorderMu.Lock()
	defer enforcementRecorderMu.Unlock()
	enforcementRecorder = rec
	enforcementHealth = health
	if len(disableAll) > 0 {
		enforcementDisableAll = disableAll[0]
	}
}

func maxAgentsLimitForContext(ctx context.Context) int {
	service := getLicenseServiceForContext(ctx)
	if service == nil {
		return 0
	}
	status := service.Status()
	if status == nil {
		return 0
	}

	limit := status.MaxAgents

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

// agentCount returns the number of installed Pulse Unified Agents.
// Under the agents-only counting model, this is the only thing that counts
// toward the agent limit.
func agentCount(monitor *monitoring.Monitor) int {
	if monitor == nil {
		return 0
	}
	if rs := monitor.GetUnifiedReadState(); rs != nil {
		return len(rs.Hosts())
	}
	return len(monitor.GetLiveHostsSnapshot())
}

func legacyConnectionCounts(monitor *monitoring.Monitor) pkglicensing.LegacyConnectionCounts {
	if monitor == nil {
		return pkglicensing.LegacyConnectionCounts{}
	}
	return legacyConnectionCountsFromReadState(monitor.GetUnifiedReadStateOrSnapshot())
}

func legacyConnectionCountsFromReadState(rs unifiedresources.ReadState) pkglicensing.LegacyConnectionCounts {
	counts := pkglicensing.LegacyConnectionCounts{}
	if rs == nil {
		return counts
	}

	for _, infra := range rs.Infrastructure() {
		if infra == nil {
			continue
		}
		if infra.HasProxmox() && !infra.HasAgent() {
			counts.ProxmoxNodes++
		}
		if infra.HasDocker() && !infra.HasAgent() {
			counts.DockerHosts++
		}
	}

	counts.KubernetesClusters = int64(len(rs.K8sClusters()))
	return counts
}

func writeMaxAgentsLimitExceeded(w http.ResponseWriter, current, limit int) {
	WriteLicenseRequired(
		w,
		maxAgentsLicenseGateKey,
		agentLimitExceededMessageFromLicensing(current, limit),
	)
}

// enforceAgentLimitForConfigRegistration is a no-op under the agents-only model.
// PVE/PBS/PMG API connections don't count toward the agent limit.
func enforceAgentLimitForConfigRegistration(
	_ http.ResponseWriter,
	_ context.Context,
	_ *config.Config,
	_ *monitoring.Monitor,
) bool {
	return false
}

// enforceAgentLimitForHostReport checks whether a new host agent report would
// exceed the agent limit. Only Pulse Unified Agents count toward the limit.
func enforceAgentLimitForHostReport(
	w http.ResponseWriter,
	ctx context.Context,
	monitor *monitoring.Monitor,
	report agentshost.Report,
	tokenRecord *config.APITokenRecord,
) bool {
	limit := maxAgentsLimitForContext(ctx)
	if limit <= 0 || monitor == nil {
		return false
	}

	hosts := monitor.GetLiveHostsSnapshot()
	// Updates to an existing agent are always permitted.
	if hostReportTargetsExistingHost(hosts, report, tokenRecord) {
		return false
	}

	current := len(hosts)
	reserved := deployReservedCount(ctx)
	effective := current + reserved
	if effective+1 <= limit {
		return false
	}

	emitLimitBlockedEvent(ctx, effective, limit)
	writeMaxAgentsLimitExceeded(w, effective, limit)
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
		Surface:        "agent_enforcement",
		LimitKey:       "max_agents",
		CurrentValue:   int64(current),
		LimitValue:     int64(limit),
		Timestamp:      now,
		IdempotencyKey: fmt.Sprintf("backend:%s:%s:%s:%d", orgID, conversionEventLimitBlocked, "agent_enforcement", now),
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

// enforceAgentLimitForDockerReport is a no-op under the agents-only model.
// Docker hosts discovered via socket don't count toward the agent limit.
func enforceAgentLimitForDockerReport(
	_ http.ResponseWriter,
	_ context.Context,
	_ *monitoring.Monitor,
	_ agentsdocker.Report,
	_ *config.APITokenRecord,
) bool {
	return false
}

// enforceAgentLimitForKubernetesReport is a no-op under the agents-only model.
// K8s nodes discovered via cluster API don't count toward the agent limit.
func enforceAgentLimitForKubernetesReport(
	_ http.ResponseWriter,
	_ context.Context,
	_ *monitoring.Monitor,
	_ agentsk8s.Report,
	_ *config.APITokenRecord,
) bool {
	return false
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
