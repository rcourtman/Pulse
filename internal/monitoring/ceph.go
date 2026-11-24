package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	monerrors "github.com/rcourtman/pulse-go-rewrite/internal/monitoring/errors"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

// isCephStorageType returns true when the provided storage type represents a Ceph backend.
func isCephStorageType(storageType string) bool {
	switch strings.ToLower(strings.TrimSpace(storageType)) {
	case "rbd", "cephfs", "ceph":
		return true
	default:
		return false
	}
}

// pollCephCluster gathers Ceph cluster information when Ceph-backed storage is detected.
func (m *Monitor) pollCephCluster(ctx context.Context, instanceName string, client PVEClientInterface, cephDetected bool) {
	if !cephDetected {
		// Clear any previously cached Ceph data for this instance.
		m.state.UpdateCephClustersForInstance(instanceName, []models.CephCluster{})
		return
	}

	cephCtx := ctx
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 15*time.Second {
		var cancel context.CancelFunc
		cephCtx, cancel = context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
	}

	status, err := client.GetCephStatus(cephCtx)
	if err != nil {
		if monerrors.IsAuthError(err) {
			log.Warn().Err(err).Str("instance", instanceName).
				Msg("ceph status request denied (401/403) – grant the Pulse token Sys.Audit on path / to enable Ceph monitoring")
		} else {
			log.Debug().Err(err).Str("instance", instanceName).Msg("ceph status unavailable – preserving previous ceph state")
		}
		return
	}
	if status == nil {
		log.Debug().Str("instance", instanceName).Msg("ceph status response empty – clearing cached ceph state")
		m.state.UpdateCephClustersForInstance(instanceName, []models.CephCluster{})
		return
	}

	df, err := client.GetCephDF(cephCtx)
	if err != nil {
		log.Debug().Err(err).Str("instance", instanceName).Msg("ceph DF unavailable – continuing with status-only data")
	}

	cluster := buildCephClusterModel(instanceName, status, df)
	if cluster.ID == "" {
		// Ensure the cluster has a stable identifier; fall back to instance name.
		cluster.ID = instanceName
	}

	storedClusters := m.state.UpdateCephClustersForInstance(instanceName, []models.CephCluster{cluster})
	for _, cluster := range storedClusters {
		m.checkCephPoolStorage(cluster)
	}
}

// admitAgentCephCluster reports whether hostID's Ceph report may write the
// global record of the cluster it names. An agent names the cluster by the
// FSID in its own report, which every Ceph client can read, so the report
// alone does not show that the host belongs to that cluster. The record
// therefore stays with the agent that reported the cluster first for as long
// as that agent keeps reporting it: a different host naming the same cluster
// cannot replace its health, capacity or name, nor resolve or raise its
// alerts. Agents on several nodes of one cluster describe the same cluster,
// so the others take over only once the holder's reporting lease lapses, its
// host is removed, or it reports a different cluster.
func (m *Monitor) admitAgentCephCluster(clusterID, hostID string, now time.Time) bool {
	// The state matches clusters by ID and by case-insensitive FSID; an
	// agent's cluster ID is its FSID whenever it reports one.
	key := strings.ToLower(strings.TrimSpace(clusterID))

	m.cephAgentReportersMu.Lock()
	defer m.cephAgentReportersMu.Unlock()

	// An agent reports one cluster, so its hold on any other has ended, and
	// no lease outlives the longest reporting window. That bounds the map by
	// the number of reporting hosts, whatever FSIDs they send.
	for other, holder := range m.cephAgentReporters {
		if (other != key && holder.hostID == hostID) || now.Sub(holder.reportedAt) > hostMaximumHealthWindow {
			delete(m.cephAgentReporters, other)
		}
	}

	if holder, ok := m.cephAgentReporters[key]; ok && holder.hostID != hostID {
		if host, registered := m.state.GetHost(holder.hostID); registered &&
			hostAgentReportCurrent(holder.reportedAt, host.IntervalSeconds, now) {
			log.Warn().
				Str("hostId", hostID).
				Str("clusterId", clusterID).
				Str("reportingHostId", holder.hostID).
				Msg("Ignored Ceph cluster from host agent: another agent is reporting this cluster")
			return false
		}
	}

	if m.cephAgentReporters == nil {
		m.cephAgentReporters = make(map[string]cephAgentReporter)
	}
	m.cephAgentReporters[key] = cephAgentReporter{hostID: hostID, reportedAt: now}
	return true
}

// trackAgentCephCluster records the Ceph cluster hostID reports and retires
// the record of the cluster it reported before. A host belongs to one Ceph
// cluster, so the cluster it names now replaces the one it named earlier; left
// in state, every ID a host ever reported would keep a cluster and all of its
// pools there. An empty clusterID retires the record of a host that is being
// removed. A record another host agent still reports stays, and so does one
// the Proxmox API describes.
func (m *Monitor) trackAgentCephCluster(hostID, clusterID string) {
	key := strings.ToLower(strings.TrimSpace(clusterID))

	m.mu.Lock()
	defer m.mu.Unlock()

	previous := m.hostCephClusters[hostID]
	if previous == key {
		return
	}
	if key == "" {
		delete(m.hostCephClusters, hostID)
	} else {
		if m.hostCephClusters == nil {
			m.hostCephClusters = make(map[string]string)
		}
		m.hostCephClusters[hostID] = key
	}
	if previous == "" {
		return
	}
	for _, reported := range m.hostCephClusters {
		if reported == previous {
			return
		}
	}
	m.state.RemoveHostAgentCephCluster(previous)
}

func (m *Monitor) checkCephPoolStorage(cluster models.CephCluster) {
	storagePools := models.CephPoolStorage(cluster)
	if len(storagePools) == 0 {
		return
	}

	timestamp := time.Now()
	for _, storage := range storagePools {
		if m.metricsHistory != nil && !shouldSkipNativeMockStateMetricWrites() {
			m.metricsHistory.AddStorageMetric(storage.ID, "usage", storage.Usage, timestamp)
			m.metricsHistory.AddStorageMetric(storage.ID, "used", float64(storage.Used), timestamp)
			m.metricsHistory.AddStorageMetric(storage.ID, "total", float64(storage.Total), timestamp)
			m.metricsHistory.AddStorageMetric(storage.ID, "avail", float64(storage.Free), timestamp)
			if m.metricsStore != nil {
				m.metricsStore.Write("storage", storage.ID, "usage", storage.Usage, timestamp)
				m.metricsStore.Write("storage", storage.ID, "used", float64(storage.Used), timestamp)
				m.metricsStore.Write("storage", storage.ID, "total", float64(storage.Total), timestamp)
				m.metricsStore.Write("storage", storage.ID, "avail", float64(storage.Free), timestamp)
			}
		}
		if m.alertManager != nil {
			m.alertManager.CheckStorageWithCapacityTrend(storage, m.storageCapacityTrend(storage, timestamp))
		}
	}
}

// buildCephClusterModel converts the proxmox Ceph responses into the shared model representation.
func buildCephClusterModel(instanceName string, status *proxmox.CephStatus, df *proxmox.CephDF) models.CephCluster {
	clusterID := instanceName
	if status.FSID != "" {
		clusterID = fmt.Sprintf("%s-%s", instanceName, status.FSID)
	}

	totalBytes := int64(status.PGMap.BytesTotal)
	usedBytes := int64(status.PGMap.BytesUsed)
	availBytes := int64(status.PGMap.BytesAvail)

	if df != nil {
		if stats := df.Data.Stats; stats.TotalBytes > 0 {
			totalBytes = int64(stats.TotalBytes)
			usedBytes = int64(stats.TotalUsedBytes)
			availBytes = int64(stats.TotalAvailBytes)
		}
	}

	usagePercent := safePercentage(float64(usedBytes), float64(totalBytes))

	pools := make([]models.CephPool, 0)
	if df != nil {
		for _, pool := range df.Data.Pools {
			pools = append(pools, models.CephPool{
				ID:             pool.ID,
				Name:           pool.Name,
				StoredBytes:    int64(pool.Stats.BytesUsed),
				AvailableBytes: int64(pool.Stats.MaxAvail),
				Objects:        int64(pool.Stats.Objects),
				PercentUsed:    pool.Stats.PercentUsed,
			})
		}
	}

	services := make([]models.CephServiceStatus, 0)
	if status.ServiceMap.Services != nil {
		for serviceType, definition := range status.ServiceMap.Services {
			running := 0
			total := 0
			var offline []string
			for daemonName, daemon := range definition.Daemons {
				total++
				// Standby is a healthy running role for a manager, not an
				// offline daemon. Do not extend it to MON/OSD or infer health
				// for stopped/unknown daemons from the cluster summary.
				if strings.EqualFold(daemon.Status, "running") || strings.EqualFold(daemon.Status, "active") ||
					(strings.EqualFold(serviceType, "mgr") && strings.EqualFold(daemon.Status, "standby")) {
					running++
					continue
				}
				label := daemonName
				if daemon.Host != "" {
					label = fmt.Sprintf("%s@%s", daemonName, daemon.Host)
				}
				offline = append(offline, label)
			}
			serviceStatus := models.CephServiceStatus{
				Type:    serviceType,
				Running: running,
				Total:   total,
			}
			if len(offline) > 0 {
				serviceStatus.Message = fmt.Sprintf("Offline: %s", strings.Join(offline, ", "))
			}
			services = append(services, serviceStatus)
		}
	}

	healthMsg := summarizeCephHealth(status)
	healthChecks := cephHealthChecks(status)
	numMons := countCephMonitorDaemons(status)
	numMgrs := countCephManagerDaemons(status)

	cluster := models.CephCluster{
		ID:             clusterID,
		Instance:       instanceName,
		Source:         models.CephClusterSourceProxmoxAPI,
		Name:           "Ceph",
		FSID:           status.FSID,
		Health:         status.Health.Status,
		HealthMessage:  healthMsg,
		HealthChecks:   healthChecks,
		TotalBytes:     totalBytes,
		UsedBytes:      usedBytes,
		AvailableBytes: availBytes,
		UsagePercent:   usagePercent,
		NumMons:        numMons,
		NumMgrs:        numMgrs,
		NumOSDs:        status.OSDMap.NumOSDs,
		NumOSDsUp:      status.OSDMap.NumUpOSDs,
		NumOSDsIn:      status.OSDMap.NumInOSDs,
		NumPGs:         status.PGMap.NumPGs,
		Pools:          pools,
		Services:       services,
		LastUpdated:    time.Now(),
	}

	return cluster
}

func cephHealthChecks(status *proxmox.CephStatus) []models.CephHealthCheck {
	if status == nil || len(status.Health.Checks) == 0 {
		return nil
	}
	codes := make([]string, 0, len(status.Health.Checks))
	for code := range status.Health.Checks {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	checks := make([]models.CephHealthCheck, 0, len(codes))
	for _, code := range codes {
		check := status.Health.Checks[code]
		summary := extractCephCheckSummary(check.Summary)
		if summary == "" {
			for _, detail := range check.Detail {
				if strings.TrimSpace(detail.Message) != "" {
					summary = strings.TrimSpace(detail.Message)
					break
				}
			}
		}
		checks = append(checks, models.CephHealthCheck{
			Code:     strings.TrimSpace(code),
			Severity: strings.TrimSpace(check.Severity),
			Summary:  summary,
		})
	}
	return checks
}

func countCephMonitorDaemons(status *proxmox.CephStatus) int {
	if status == nil {
		return 0
	}
	if status.MonMap.NumMons > 0 {
		return status.MonMap.NumMons
	}
	if len(status.MonMap.Mons) > 0 {
		return len(status.MonMap.Mons)
	}
	if len(status.MonMap.Monitors) > 0 {
		return len(status.MonMap.Monitors)
	}
	if len(status.MonMap.QuorumNames) > 0 {
		return len(status.MonMap.QuorumNames)
	}
	if len(status.MonMap.Quorum) > 0 {
		return len(status.MonMap.Quorum)
	}
	// Ceph places quorum membership at the top level of the status payload.
	if len(status.QuorumNames) > 0 {
		return len(status.QuorumNames)
	}
	if len(status.Quorum) > 0 {
		return len(status.Quorum)
	}
	return countServiceDaemons(status.ServiceMap.Services, "mon")
}

func countCephManagerDaemons(status *proxmox.CephStatus) int {
	if status == nil {
		return 0
	}
	if status.MgrMap.NumMgrs > 0 {
		return status.MgrMap.NumMgrs
	}
	// Quincy+ drops num_mgrs/active_name from mgrmap; derive the count from
	// availability plus standbys (num_standbys on modern releases).
	active := 0
	if status.MgrMap.Available || status.MgrMap.ActiveName != "" {
		active = 1
	}
	standbys := max(status.MgrMap.NumStandbys, len(status.MgrMap.Standbys))
	if active+standbys > 0 {
		return active + standbys
	}
	return countServiceDaemons(status.ServiceMap.Services, "mgr")
}

// summarizeCephHealth extracts human-readable messages from the Ceph health payload.
func summarizeCephHealth(status *proxmox.CephStatus) string {
	if status == nil {
		return ""
	}

	messages := make([]string, 0)

	for _, summary := range status.Health.Summary {
		switch {
		case summary.Message != "":
			messages = append(messages, summary.Message)
		case summary.Summary != "":
			messages = append(messages, summary.Summary)
		}
	}

	for checkName, check := range status.Health.Checks {
		if msg := extractCephCheckSummary(check.Summary); msg != "" {
			messages = append(messages, fmt.Sprintf("%s: %s", checkName, msg))
			continue
		}
		for _, detail := range check.Detail {
			if detail.Message != "" {
				messages = append(messages, fmt.Sprintf("%s: %s", checkName, detail.Message))
				break
			}
		}
	}

	return strings.Join(messages, "; ")
}

// extractCephCheckSummary attempts to parse the flexible summary field in Ceph health checks into a message string.
func extractCephCheckSummary(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var obj struct {
		Message string `json:"message"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		if obj.Message != "" {
			return obj.Message
		}
		if obj.Summary != "" {
			return obj.Summary
		}
	}

	var list []struct {
		Message string `json:"message"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(raw, &list); err == nil {
		for _, item := range list {
			if item.Message != "" {
				return item.Message
			}
			if item.Summary != "" {
				return item.Summary
			}
		}
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}

	return ""
}

// countServiceDaemons returns the number of daemons defined for a given service type.
func countServiceDaemons(services map[string]proxmox.CephServiceDefinition, serviceType string) int {
	if services == nil {
		return 0
	}
	definition, ok := services[serviceType]
	if !ok {
		return 0
	}
	return len(definition.Daemons)
}
