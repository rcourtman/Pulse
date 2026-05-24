package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
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

func cephPollContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= 15*time.Second {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, 15*time.Second)
}

func fetchCephClusterData(ctx context.Context, instanceName string, client PVEClientInterface) (*proxmox.CephStatus, *proxmox.CephDF, error) {
	cephCtx, cancel := cephPollContext(ctx)
	defer cancel()

	status, err := client.GetCephStatus(cephCtx)
	if err != nil {
		log.Debug().Err(err).Str("instance", instanceName).Msg("Ceph status unavailable - preserving previous Ceph state")
		return nil, nil, err
	}
	if status == nil {
		return nil, nil, nil
	}

	df, err := client.GetCephDF(cephCtx)
	if err != nil {
		log.Debug().Err(err).Str("instance", instanceName).Msg("Ceph DF unavailable - continuing with status-only data")
	}

	return status, df, nil
}

func normalizeCephPoolKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func cephPoolLookupCandidates(storage models.Storage) []string {
	candidates := make([]string, 0, 2)
	if pool := normalizeCephPoolKey(storage.Pool); pool != "" {
		candidates = append(candidates, pool)
	}
	if name := normalizeCephPoolKey(storage.Name); name != "" {
		candidates = append(candidates, name)
	}
	return slices.Compact(candidates)
}

func hydrateCephStorageUsageFromDF(storage []models.Storage, df *proxmox.CephDF) bool {
	if len(storage) == 0 || df == nil || len(df.Data.Pools) == 0 {
		return false
	}

	poolsByName := make(map[string]proxmox.CephDFPool, len(df.Data.Pools))
	for _, pool := range df.Data.Pools {
		key := normalizeCephPoolKey(pool.Name)
		if key == "" {
			continue
		}
		poolsByName[key] = pool
	}

	updated := false
	for idx := range storage {
		if !isCephStorageType(storage[idx].Type) {
			continue
		}

		var pool proxmox.CephDFPool
		found := false
		for _, candidate := range cephPoolLookupCandidates(storage[idx]) {
			match, ok := poolsByName[candidate]
			if !ok {
				continue
			}
			pool = match
			found = true
			break
		}
		if !found {
			continue
		}

		used := int64(pool.Stats.BytesUsed)
		free := int64(pool.Stats.MaxAvail)
		total := used + free
		if total <= 0 {
			continue
		}

		storage[idx].Used = used
		storage[idx].Free = free
		storage[idx].Total = total
		storage[idx].Usage = safePercentage(float64(used), float64(total))
		updated = true
	}

	return updated
}

func sanitizeCephPoolStorageComponent(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		allowed := (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' ||
			r == '.' ||
			r == ':' ||
			r == '-'
		if allowed {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}

	result := builder.String()
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return strings.Trim(result, "-")
}

func cephPoolStorageID(instanceName string, pool models.CephPool) string {
	instance := sanitizeCephPoolStorageComponent(instanceName)
	if instance == "" {
		instance = "ceph"
	}

	poolName := sanitizeCephPoolStorageComponent(pool.Name)
	if poolName == "" {
		poolName = fmt.Sprintf("pool-%d", pool.ID)
	}

	return fmt.Sprintf("%s-ceph-pool-%s", instance, poolName)
}

func cephPoolAlertStorageTargets(cluster models.CephCluster) []models.Storage {
	if len(cluster.Pools) == 0 {
		return nil
	}

	instanceName := strings.TrimSpace(cluster.Instance)
	if instanceName == "" {
		instanceName = cluster.ID
	}

	targets := make([]models.Storage, 0, len(cluster.Pools))
	for _, pool := range cluster.Pools {
		name := strings.TrimSpace(pool.Name)
		if name == "" {
			name = fmt.Sprintf("pool-%d", pool.ID)
		}

		used := pool.StoredBytes
		free := pool.AvailableBytes
		total := used + free
		usage := pool.PercentUsed
		if usage <= 0 && total > 0 {
			usage = safePercentage(float64(used), float64(total))
		}

		targets = append(targets, models.Storage{
			ID:       cephPoolStorageID(instanceName, pool),
			Name:     name,
			Node:     "cluster",
			Instance: instanceName,
			Type:     "ceph-pool",
			Status:   "available",
			Pool:     name,
			Total:    total,
			Used:     used,
			Free:     free,
			Usage:    usage,
			Content:  "ceph",
			Shared:   true,
			Enabled:  true,
			Active:   true,
		})
	}

	return targets
}

func cephPoolAlertStorageTargetsForInstance(state models.StateSnapshot, instanceName string) []models.Storage {
	instanceName = strings.TrimSpace(instanceName)
	targets := make([]models.Storage, 0)
	for _, cluster := range state.CephClusters {
		if instanceName != "" && strings.TrimSpace(cluster.Instance) != instanceName {
			continue
		}
		targets = append(targets, cephPoolAlertStorageTargets(cluster)...)
	}
	return targets
}

// pollCephCluster gathers Ceph cluster information when Ceph-backed storage is detected.
func (m *Monitor) pollCephCluster(ctx context.Context, instanceName string, client PVEClientInterface, cephDetected bool) {
	if !cephDetected {
		// Clear any previously cached Ceph data for this instance.
		m.state.UpdateCephClustersForInstance(instanceName, []models.CephCluster{})
		return
	}

	status, df, err := fetchCephClusterData(ctx, instanceName, client)
	if err != nil {
		return
	}
	if status == nil {
		log.Debug().Str("instance", instanceName).Msg("Ceph status response empty - clearing cached Ceph state")
		m.state.UpdateCephClustersForInstance(instanceName, []models.CephCluster{})
		return
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
	numMons := countCephMonitorDaemons(status)
	numMgrs := countCephManagerDaemons(status)

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

func countCephMonitorDaemons(status *proxmox.CephStatus) int {
	if status == nil {
		return 0
	}
	if status.MonMap.NumMons > 0 {
		return status.MonMap.NumMons
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
	if status.MgrMap.ActiveName != "" {
		return 1 + len(status.MgrMap.Standbys)
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
