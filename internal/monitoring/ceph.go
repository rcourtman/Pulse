package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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
		log.Debug().Err(err).Str("instance", instanceName).Msg("ceph status unavailable – preserving previous ceph state")
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

	m.state.UpdateCephClustersForInstance(instanceName, []models.CephCluster{cluster})
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
				if strings.EqualFold(daemon.Status, "running") || strings.EqualFold(daemon.Status, "active") {
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

	cluster := models.CephCluster{
		ID:             clusterID,
		Instance:       instanceName,
		Name:           "Ceph",
		FSID:           status.FSID,
		Health:         status.Health.Status,
		HealthMessage:  healthMsg,
		TotalBytes:     totalBytes,
		UsedBytes:      usedBytes,
		AvailableBytes: availBytes,
		UsagePercent:   usagePercent,
		NumMons:        countServiceDaemons(status.ServiceMap.Services, "mon"),
		NumMgrs:        countServiceDaemons(status.ServiceMap.Services, "mgr"),
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
