package monitoring

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog/log"
)

func schedulerKey(instanceType InstanceType, name string) string {
	return string(instanceType) + "::" + name
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	copy := t
	return &copy
}

// safePercentage calculates percentage safely, returning 0 if divisor is 0
func safePercentage(used, total float64) float64 {
	if total == 0 {
		return 0
	}
	result := used / total * 100
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return 0
	}
	return result
}

// safeFloat ensures a float value is not NaN or Inf
func safeFloat(val float64) float64 {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return 0
	}
	return val
}

// makeGuestID generates a stable, canonical guest ID that includes instance, node, and VMID.
// Format: {instance}:{node}:{vmid} (e.g., "delly:minipc:201")
//
// Using colons as separators prevents ambiguity with dashes in instance/node names.
// This format ensures:
// - Unique IDs across all deployment scenarios (single agent, per-node agents, mixed)
// - Stable IDs that don't change when monitoring topology changes
// - Easy parsing to extract instance, node, and VMID components
//
// For clustered setups, the instance name is the cluster name.
// For standalone nodes, the instance name matches the node name.
func makeGuestID(instanceName string, node string, vmid int) string {
	return fmt.Sprintf("%s:%s:%d", instanceName, node, vmid)
}

// parseDurationEnv parses a duration from an environment variable, returning defaultVal if not set or invalid
func parseDurationEnv(key string, defaultVal time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	parsed, err := time.ParseDuration(val)
	if err != nil {
		log.Warn().
			Str("key", key).
			Str("value", val).
			Err(err).
			Dur("default", defaultVal).
			Msg("Failed to parse duration from environment variable, using default")
		return defaultVal
	}
	return parsed
}

// parsePositiveDurationEnv parses a duration from an environment variable and
// enforces a strictly positive value, otherwise returning defaultVal.
func parsePositiveDurationEnv(key string, defaultVal time.Duration) time.Duration {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return defaultVal
	}

	parsed, err := time.ParseDuration(val)
	if err != nil {
		log.Warn().
			Str("key", key).
			Str("value", val).
			Err(err).
			Dur("default", defaultVal).
			Msg("Failed to parse duration from environment variable, using default")
		return defaultVal
	}

	if parsed <= 0 {
		log.Warn().
			Str("key", key).
			Str("value", val).
			Dur("default", defaultVal).
			Msg("Environment duration must be greater than zero, using default")
		return defaultVal
	}

	return parsed
}

// parseIntEnv parses an integer from an environment variable, returning defaultVal if not set or invalid
func parseIntEnv(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		log.Warn().
			Str("key", key).
			Str("value", val).
			Err(err).
			Int("default", defaultVal).
			Msg("Failed to parse integer from environment variable, using default")
		return defaultVal
	}
	return parsed
}

// parseNonNegativeIntEnv parses an integer from an environment variable and
// enforces a non-negative value, otherwise returning defaultVal.
func parseNonNegativeIntEnv(key string, defaultVal int) int {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return defaultVal
	}

	parsed, err := strconv.Atoi(val)
	if err != nil {
		log.Warn().
			Str("key", key).
			Str("value", val).
			Err(err).
			Int("default", defaultVal).
			Msg("Failed to parse integer from environment variable, using default")
		return defaultVal
	}

	if parsed < 0 {
		log.Warn().
			Str("key", key).
			Str("value", val).
			Int("default", defaultVal).
			Msg("Environment integer must be non-negative, using default")
		return defaultVal
	}

	return parsed
}

func clampUint64ToInt64(val uint64) int64 {
	if val > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(val)
}

func cloneStringFloatMap(src map[string]float64) map[string]float64 {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]float64, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func convertDockerServices(services []agentsdocker.Service) []models.DockerService {
	if len(services) == 0 {
		return nil
	}

	result := make([]models.DockerService, 0, len(services))
	for _, svc := range services {
		service := models.DockerService{
			ID:             svc.ID,
			Name:           svc.Name,
			Stack:          svc.Stack,
			Image:          svc.Image,
			Mode:           svc.Mode,
			DesiredTasks:   svc.DesiredTasks,
			RunningTasks:   svc.RunningTasks,
			CompletedTasks: svc.CompletedTasks,
		}

		if len(svc.Labels) > 0 {
			service.Labels = cloneStringMap(svc.Labels)
		}

		if len(svc.EndpointPorts) > 0 {
			ports := make([]models.DockerServicePort, len(svc.EndpointPorts))
			for i, port := range svc.EndpointPorts {
				ports[i] = models.DockerServicePort{
					Name:          port.Name,
					Protocol:      port.Protocol,
					TargetPort:    port.TargetPort,
					PublishedPort: port.PublishedPort,
					PublishMode:   port.PublishMode,
				}
			}
			service.EndpointPorts = ports
		}

		if svc.UpdateStatus != nil {
			update := &models.DockerServiceUpdate{
				State:   svc.UpdateStatus.State,
				Message: svc.UpdateStatus.Message,
			}
			if svc.UpdateStatus.CompletedAt != nil && !svc.UpdateStatus.CompletedAt.IsZero() {
				completed := *svc.UpdateStatus.CompletedAt
				update.CompletedAt = &completed
			}
			service.UpdateStatus = update
		}

		if svc.CreatedAt != nil && !svc.CreatedAt.IsZero() {
			created := *svc.CreatedAt
			service.CreatedAt = &created
		}
		if svc.UpdatedAt != nil && !svc.UpdatedAt.IsZero() {
			updated := *svc.UpdatedAt
			service.UpdatedAt = &updated
		}

		result = append(result, service)
	}

	return result
}

func convertDockerTasks(tasks []agentsdocker.Task) []models.DockerTask {
	if len(tasks) == 0 {
		return nil
	}

	result := make([]models.DockerTask, 0, len(tasks))
	for _, task := range tasks {
		modelTask := models.DockerTask{
			ID:            task.ID,
			ServiceID:     task.ServiceID,
			ServiceName:   task.ServiceName,
			Slot:          task.Slot,
			NodeID:        task.NodeID,
			NodeName:      task.NodeName,
			DesiredState:  task.DesiredState,
			CurrentState:  task.CurrentState,
			Error:         task.Error,
			Message:       task.Message,
			ContainerID:   task.ContainerID,
			ContainerName: task.ContainerName,
			CreatedAt:     task.CreatedAt,
		}

		if task.UpdatedAt != nil && !task.UpdatedAt.IsZero() {
			updated := *task.UpdatedAt
			modelTask.UpdatedAt = &updated
		}
		if task.StartedAt != nil && !task.StartedAt.IsZero() {
			started := *task.StartedAt
			modelTask.StartedAt = &started
		}
		if task.CompletedAt != nil && !task.CompletedAt.IsZero() {
			completed := *task.CompletedAt
			modelTask.CompletedAt = &completed
		}

		result = append(result, modelTask)
	}

	return result
}

func normalizeAgentVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	version = strings.TrimLeft(version, "vV")
	if version == "" {
		return ""
	}
	return "v" + version
}

func convertDockerSwarmInfo(info *agentsdocker.SwarmInfo) *models.DockerSwarmInfo {
	if info == nil {
		return nil
	}

	return &models.DockerSwarmInfo{
		NodeID:           info.NodeID,
		NodeRole:         info.NodeRole,
		LocalState:       info.LocalState,
		ControlAvailable: info.ControlAvailable,
		ClusterID:        info.ClusterID,
		ClusterName:      info.ClusterName,
		Scope:            info.Scope,
		Error:            info.Error,
	}
}

// pollStorageBackupsWithNodes polls backups using a provided nodes list to avoid duplicate GetNodes calls
func extractSnapshotName(volid string) string {
	if volid == "" {
		return ""
	}

	parts := strings.SplitN(volid, ":", 2)
	remainder := volid
	if len(parts) == 2 {
		remainder = parts[1]
	}

	if idx := strings.Index(remainder, "@"); idx >= 0 && idx+1 < len(remainder) {
		return strings.TrimSpace(remainder[idx+1:])
	}

	return ""
}

func isLegacyHostAgent(agentType string) bool {
	// Unified agent reports type="unified"
	// Legacy standalone agents have empty type
	return agentType != "unified"
}

func isLegacyDockerAgent(agentType string) bool {
	// Unified agent reports type="unified"
	// Legacy standalone agents have empty type
	return agentType != "unified"
}

// convertAgentSMARTToModels converts agent report S.M.A.R.T. data to the models.HostDiskSMART format.
func convertAgentSMARTToModels(smart []agentshost.DiskSMART) []models.HostDiskSMART {
	if len(smart) == 0 {
		return nil
	}
	result := make([]models.HostDiskSMART, 0, len(smart))
	for _, disk := range smart {
		entry := models.HostDiskSMART{
			Device:      disk.Device,
			Model:       disk.Model,
			Serial:      disk.Serial,
			WWN:         disk.WWN,
			Type:        disk.Type,
			Temperature: disk.Temperature,
			Health:      disk.Health,
			Standby:     disk.Standby,
		}
		if disk.Attributes != nil {
			entry.Attributes = convertAgentSMARTAttributes(disk.Attributes)
		}
		result = append(result, entry)
	}
	return result
}

// convertAgentSMARTAttributes converts agent SMARTAttributes to models SMARTAttributes.
func convertAgentSMARTAttributes(src *agentshost.SMARTAttributes) *models.SMARTAttributes {
	if src == nil {
		return nil
	}
	return &models.SMARTAttributes{
		PowerOnHours:         src.PowerOnHours,
		PowerCycles:          src.PowerCycles,
		ReallocatedSectors:   src.ReallocatedSectors,
		PendingSectors:       src.PendingSectors,
		OfflineUncorrectable: src.OfflineUncorrectable,
		UDMACRCErrors:        src.UDMACRCErrors,
		PercentageUsed:       src.PercentageUsed,
		AvailableSpare:       src.AvailableSpare,
		MediaErrors:          src.MediaErrors,
		UnsafeShutdowns:      src.UnsafeShutdowns,
	}
}

// convertAgentCephToModels converts agent report Ceph data to the models.HostCephCluster format.
func convertAgentCephToModels(ceph *agentshost.CephCluster) *models.HostCephCluster {
	if ceph == nil {
		return nil
	}

	collectedAt, _ := time.Parse(time.RFC3339, ceph.CollectedAt)

	result := &models.HostCephCluster{
		FSID: ceph.FSID,
		Health: models.HostCephHealth{
			Status: ceph.Health.Status,
			Checks: make(map[string]models.HostCephCheck),
		},
		MonMap: models.HostCephMonitorMap{
			Epoch:   ceph.MonMap.Epoch,
			NumMons: ceph.MonMap.NumMons,
		},
		MgrMap: models.HostCephManagerMap{
			Available: ceph.MgrMap.Available,
			NumMgrs:   ceph.MgrMap.NumMgrs,
			ActiveMgr: ceph.MgrMap.ActiveMgr,
			Standbys:  ceph.MgrMap.Standbys,
		},
		OSDMap: models.HostCephOSDMap{
			Epoch:   ceph.OSDMap.Epoch,
			NumOSDs: ceph.OSDMap.NumOSDs,
			NumUp:   ceph.OSDMap.NumUp,
			NumIn:   ceph.OSDMap.NumIn,
			NumDown: ceph.OSDMap.NumDown,
			NumOut:  ceph.OSDMap.NumOut,
		},
		PGMap: models.HostCephPGMap{
			NumPGs:           ceph.PGMap.NumPGs,
			BytesTotal:       ceph.PGMap.BytesTotal,
			BytesUsed:        ceph.PGMap.BytesUsed,
			BytesAvailable:   ceph.PGMap.BytesAvailable,
			DataBytes:        ceph.PGMap.DataBytes,
			UsagePercent:     ceph.PGMap.UsagePercent,
			DegradedRatio:    ceph.PGMap.DegradedRatio,
			MisplacedRatio:   ceph.PGMap.MisplacedRatio,
			ReadBytesPerSec:  ceph.PGMap.ReadBytesPerSec,
			WriteBytesPerSec: ceph.PGMap.WriteBytesPerSec,
			ReadOpsPerSec:    ceph.PGMap.ReadOpsPerSec,
			WriteOpsPerSec:   ceph.PGMap.WriteOpsPerSec,
		},
		CollectedAt: collectedAt,
	}

	// Convert monitors
	for _, mon := range ceph.MonMap.Monitors {
		result.MonMap.Monitors = append(result.MonMap.Monitors, models.HostCephMonitor{
			Name:   mon.Name,
			Rank:   mon.Rank,
			Addr:   mon.Addr,
			Status: mon.Status,
		})
	}

	// Convert health checks
	for name, check := range ceph.Health.Checks {
		result.Health.Checks[name] = models.HostCephCheck{
			Severity: check.Severity,
			Message:  check.Message,
			Detail:   check.Detail,
		}
	}

	// Convert health summary
	for _, s := range ceph.Health.Summary {
		result.Health.Summary = append(result.Health.Summary, models.HostCephHealthSummary{
			Severity: s.Severity,
			Message:  s.Message,
		})
	}

	// Convert pools
	for _, pool := range ceph.Pools {
		result.Pools = append(result.Pools, models.HostCephPool{
			ID:             pool.ID,
			Name:           pool.Name,
			BytesUsed:      pool.BytesUsed,
			BytesAvailable: pool.BytesAvailable,
			Objects:        pool.Objects,
			PercentUsed:    pool.PercentUsed,
		})
	}

	// Convert services
	for _, svc := range ceph.Services {
		result.Services = append(result.Services, models.HostCephService{
			Type:    svc.Type,
			Running: svc.Running,
			Total:   svc.Total,
			Daemons: svc.Daemons,
		})
	}

	return result
}

// convertAgentCephToGlobalCluster converts agent Ceph data to the global CephCluster format
// used by the State.CephClusters list.
func convertAgentCephToGlobalCluster(ceph *agentshost.CephCluster, hostname, hostID string, timestamp time.Time) models.CephCluster {
	// Use FSID as the primary ID since it's unique per Ceph cluster
	id := ceph.FSID
	if id == "" {
		id = "agent-ceph-" + hostID
	}

	cluster := models.CephCluster{
		ID:             id,
		Instance:       "agent:" + hostname,
		Name:           hostname + " Ceph",
		FSID:           ceph.FSID,
		Health:         strings.TrimPrefix(ceph.Health.Status, "HEALTH_"),
		TotalBytes:     int64(ceph.PGMap.BytesTotal),
		UsedBytes:      int64(ceph.PGMap.BytesUsed),
		AvailableBytes: int64(ceph.PGMap.BytesAvailable),
		UsagePercent:   ceph.PGMap.UsagePercent,
		NumMons:        ceph.MonMap.NumMons,
		NumMgrs:        ceph.MgrMap.NumMgrs,
		NumOSDs:        ceph.OSDMap.NumOSDs,
		NumOSDsUp:      ceph.OSDMap.NumUp,
		NumOSDsIn:      ceph.OSDMap.NumIn,
		NumPGs:         ceph.PGMap.NumPGs,
		LastUpdated:    timestamp,
	}

	// Build health message from checks
	var healthMessages []string
	for _, check := range ceph.Health.Checks {
		if check.Message != "" {
			healthMessages = append(healthMessages, check.Message)
		}
	}
	if len(healthMessages) > 0 {
		cluster.HealthMessage = strings.Join(healthMessages, "; ")
	}

	// Convert pools
	for _, pool := range ceph.Pools {
		cluster.Pools = append(cluster.Pools, models.CephPool{
			ID:             pool.ID,
			Name:           pool.Name,
			StoredBytes:    int64(pool.BytesUsed),
			AvailableBytes: int64(pool.BytesAvailable),
			Objects:        int64(pool.Objects),
			PercentUsed:    pool.PercentUsed,
		})
	}

	// Convert services
	for _, svc := range ceph.Services {
		cluster.Services = append(cluster.Services, models.CephServiceStatus{
			Type:    svc.Type,
			Running: svc.Running,
			Total:   svc.Total,
		})
	}

	return cluster
}
