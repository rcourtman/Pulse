package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// overflowBaseDataDir is set during router initialization to allow the
// enforcement path to read billing-state overflow fields without requiring
// a handler reference.
var (
	overflowBaseDataDirMu sync.RWMutex
	overflowBaseDataDir   string
)

// SetOverflowBaseDataDir configures the base data directory used by the
// enforcement path to read OverflowGrantedAt from billing state.
func SetOverflowBaseDataDir(dir string) {
	overflowBaseDataDirMu.Lock()
	defer overflowBaseDataDirMu.Unlock()
	overflowBaseDataDir = dir
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
	if status.Tier == pkglicensing.TierFree {
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

		limit += pkglicensing.OverflowBonus(status.Tier, overflowGrantedAt, time.Now())
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
	state := monitor.GetLiveStateSnapshot()
	return len(state.Hosts)
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

	snapshot := monitor.GetLiveStateSnapshot()
	// Updates to an existing agent are always permitted.
	if hostReportTargetsExistingHost(snapshot, report, tokenRecord) {
		return false
	}

	current := len(snapshot.Hosts)
	if current+1 <= limit {
		return false
	}

	writeMaxAgentsLimitExceeded(w, current, limit)
	return true
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
