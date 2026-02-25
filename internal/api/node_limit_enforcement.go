package api

import (
	"context"
	"fmt"
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

func maxNodesLimitForContext(ctx context.Context) int {
	service := getLicenseServiceForContext(ctx)
	if service == nil {
		return 0
	}
	status := service.Status()
	if status == nil {
		return 0
	}

	limit := status.MaxNodes

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
	configPVE, configPBS, configPMG := configToEntries(cfg)
	candidates := unifiedresources.CollectHostCandidates(state, configPVE, configPBS, configPMG, nil)
	resolved := unifiedresources.ResolveHosts(candidates)
	return len(resolved.Hosts)
}

// configToEntries converts config.Config to ConfigEntry slices for the dedup layer.
func configToEntries(cfg *config.Config) (pve, pbs, pmg []unifiedresources.ConfigEntry) {
	if cfg == nil {
		return nil, nil, nil
	}
	for _, p := range cfg.PVEInstances {
		pve = append(pve, unifiedresources.ConfigEntry{
			ID: p.Host, Name: p.Name, Host: p.Host,
		})
	}
	for _, p := range cfg.PBSInstances {
		pbs = append(pbs, unifiedresources.ConfigEntry{
			ID: p.Host, Name: p.Name, Host: p.Host,
		})
	}
	for _, p := range cfg.PMGInstances {
		pmg = append(pmg, unifiedresources.ConfigEntry{
			ID: p.Host, Name: p.Name, Host: p.Host,
		})
	}
	return pve, pbs, pmg
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
// newCandidates are the HostCandidates that the incoming report would add to the dedup set.
// The function projects the post-report dedup count and blocks if it exceeds the limit.
func enforceNodeLimitForReport(
	w http.ResponseWriter,
	ctx context.Context,
	monitor *monitoring.Monitor,
	newCandidates []unifiedresources.HostCandidate,
	targetsExisting func(models.StateSnapshot) bool,
) bool {
	limit := maxNodesLimitForContext(ctx)
	if limit <= 0 || monitor == nil {
		return false
	}

	snapshot := monitor.GetLiveStateSnapshot()
	// Updates to existing entities (including K8s cluster node growth) are
	// always permitted — limit enforcement only gates brand-new registrations.
	if targetsExisting(snapshot) {
		return false
	}

	// Project: current dedup count + what the new candidates would add after dedup.
	cfg := monitor.GetConfig()
	configPVE, configPBS, configPMG := configToEntries(cfg)
	existing := unifiedresources.CollectHostCandidates(snapshot, configPVE, configPBS, configPMG, nil)
	projected := append(existing, newCandidates...)
	resolvedProjected := unifiedresources.ResolveHosts(projected)
	projectedCount := len(resolvedProjected.Hosts)

	if projectedCount <= limit {
		return false
	}

	writeMaxNodesLimitExceeded(w, projectedCount, limit)
	return true
}

func enforceNodeLimitForHostReport(
	w http.ResponseWriter,
	ctx context.Context,
	monitor *monitoring.Monitor,
	report agentshost.Report,
	tokenRecord *config.APITokenRecord,
) bool {
	return enforceNodeLimitForReport(w, ctx, monitor,
		hostReportCandidates(report),
		func(snapshot models.StateSnapshot) bool {
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
	return enforceNodeLimitForReport(w, ctx, monitor,
		dockerReportCandidates(report),
		func(snapshot models.StateSnapshot) bool {
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
	return enforceNodeLimitForReport(w, ctx, monitor,
		k8sReportCandidates(report),
		func(snapshot models.StateSnapshot) bool {
			return kubernetesReportTargetsExistingCluster(snapshot, report, tokenRecord)
		})
}

// ---------------------------------------------------------------------------
// Report-to-candidate converters for projected dedup counting
// ---------------------------------------------------------------------------

func hostReportCandidates(report agentshost.Report) []unifiedresources.HostCandidate {
	hostname := strings.TrimSpace(report.Host.Hostname)
	id := strings.TrimSpace(report.Host.ID)
	if id == "" {
		id = hostname
	}
	ips, macs := extractReportNetworkIDs(report.Network)
	if report.Host.ReportIP != "" {
		ips = append(ips, report.Host.ReportIP)
	}
	return []unifiedresources.HostCandidate{{
		ID:     "report-host:" + id,
		Name:   hostname,
		Type:   "host-agent",
		Source: "agent",
		Status: "online",
		Identity: unifiedresources.ResourceIdentity{
			MachineID:    strings.TrimSpace(report.Host.MachineID),
			Hostnames:    collectNonEmptyStrings(hostname),
			IPAddresses:  ips,
			MACAddresses: macs,
		},
	}}
}

func dockerReportCandidates(report agentsdocker.Report) []unifiedresources.HostCandidate {
	hostname := strings.TrimSpace(report.Host.Hostname)
	agentKey := strings.TrimSpace(report.AgentKey())
	id := agentKey
	if id == "" {
		id = hostname
	}
	ips, macs := extractReportNetworkIDs(report.Host.Network)
	return []unifiedresources.HostCandidate{{
		ID:     "report-docker:" + id,
		Name:   hostname,
		Type:   "docker",
		Source: "docker",
		Status: "online",
		Identity: unifiedresources.ResourceIdentity{
			MachineID:    strings.TrimSpace(report.Host.MachineID),
			Hostnames:    collectNonEmptyStrings(hostname),
			IPAddresses:  ips,
			MACAddresses: macs,
		},
	}}
}

func k8sReportCandidates(report agentsk8s.Report) []unifiedresources.HostCandidate {
	clusterName := strings.TrimSpace(report.Cluster.Name)
	if clusterName == "" {
		clusterName = strings.TrimSpace(report.Cluster.ID)
	}

	if len(report.Nodes) == 0 {
		return []unifiedresources.HostCandidate{{
			ID:          "report-k8s-cluster:" + clusterName,
			Name:        clusterName,
			Type:        "kubernetes",
			Source:      "kubernetes",
			Status:      "online",
			Provisional: true,
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: collectNonEmptyStrings(clusterName),
			},
		}}
	}

	candidates := make([]unifiedresources.HostCandidate, 0, len(report.Nodes))
	for i, kn := range report.Nodes {
		nodeName := strings.TrimSpace(kn.Name)
		if nodeName == "" {
			nodeName = strings.TrimSpace(kn.UID)
		}
		nodeKey := nodeName
		if nodeKey == "" {
			nodeKey = fmt.Sprintf("idx-%d", i)
		}
		candidates = append(candidates, unifiedresources.HostCandidate{
			ID:     "report-k8s-node:" + clusterName + ":" + nodeKey,
			Name:   clusterName + "/" + nodeName,
			Type:   "kubernetes",
			Source: "kubernetes",
			Status: "online",
			Identity: unifiedresources.ResourceIdentity{
				Hostnames: collectNonEmptyStrings(nodeName),
			},
		})
	}
	return candidates
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

// extractReportNetworkIDs extracts IP addresses and MAC addresses from agent
// report network interfaces. This mirrors collectInterfaceIDs in the
// unifiedresources package but operates on the agent report type.
func extractReportNetworkIDs(ifaces []agentshost.NetworkInterface) (ips, macs []string) {
	for _, iface := range ifaces {
		if iface.MAC != "" {
			macs = append(macs, iface.MAC)
		}
		for _, addr := range iface.Addresses {
			ip := addr
			if idx := strings.IndexByte(ip, '/'); idx >= 0 {
				ip = ip[:idx]
			}
			if ip != "" {
				ips = append(ips, ip)
			}
		}
	}
	return ips, macs
}
