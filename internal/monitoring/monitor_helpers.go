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
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
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

// makeGuestRateKey identifies one counter stream independently of placement.
// A live migration changes the canonical row ID's node coordinate but not the
// cumulative counter epoch. The configured instance remains in the key so
// duplicate cluster registrations never race on one baseline.
func makeGuestRateKey(instanceName, guestType string, vmid int) string {
	return fmt.Sprintf("pve:%s:%s:%d", instanceName, strings.ToLower(strings.TrimSpace(guestType)), vmid)
}

func pveCounterPresence(p proxmox.IOCounterPresence) models.IOCounterPresence {
	effective := p.Effective()
	return models.IOCounterPresence{
		Explicit:   true,
		DiskRead:   effective.DiskRead,
		DiskWrite:  effective.DiskWrite,
		NetworkIn:  effective.NetworkIn,
		NetworkOut: effective.NetworkOut,
	}
}

func observedAtOr(value, fallback time.Time) time.Time {
	if !value.IsZero() {
		return value
	}
	return fallback
}

func counterObservationTimes(observedAt time.Time) models.IOCounterObservationTimes {
	return models.IOCounterObservationTimes{
		DiskRead:   observedAt,
		DiskWrite:  observedAt,
		NetworkIn:  observedAt,
		NetworkOut: observedAt,
	}
}

func numericGuestRate(rate float64) (int64, bool) {
	if rate < 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return 0, false
	}
	return max(0, int64(rate)), true
}

func guestRateValues(diskRead, diskWrite, networkIn, networkOut float64) (int64, int64, int64, int64, models.IORateValidity) {
	diskReadValue, diskReadKnown := numericGuestRate(diskRead)
	diskWriteValue, diskWriteKnown := numericGuestRate(diskWrite)
	networkInValue, networkInKnown := numericGuestRate(networkIn)
	networkOutValue, networkOutKnown := numericGuestRate(networkOut)
	return diskReadValue, diskWriteValue, networkInValue, networkOutValue, models.IORateValidity{
		Explicit:   true,
		DiskRead:   diskReadKnown,
		DiskWrite:  diskWriteKnown,
		NetworkIn:  networkInKnown,
		NetworkOut: networkOutKnown,
	}
}

// parseBoolEnv parses a boolean from an environment variable, returning defaultVal if not set or invalid
func parseBoolEnv(key string, defaultVal bool) bool {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.ParseBool(val)
	if err != nil {
		log.Warn().
			Str("key", key).
			Str("value", val).
			Bool("default", defaultVal).
			Msg("Failed to parse boolean from environment variable, using default")
		return defaultVal
	}
	return parsed
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

func cloneStringIntMap(src map[string]int) map[string]int {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]int, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func cloneIntPtr(src *int) *int {
	if src == nil {
		return nil
	}
	out := *src
	return &out
}

func cloneFloat64Ptr(src *float64) *float64 {
	if src == nil {
		return nil
	}
	out := *src
	return &out
}

func cloneInt64Ptr(src *int64) *int64 {
	if src == nil {
		return nil
	}
	out := *src
	return &out
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

func convertDockerImages(images []agentsdocker.Image) []models.DockerImage {
	if len(images) == 0 {
		return nil
	}
	result := make([]models.DockerImage, 0, len(images))
	for _, image := range images {
		result = append(result, models.DockerImage{
			ID:              strings.TrimSpace(image.ID),
			RepoTags:        append([]string(nil), image.RepoTags...),
			RepoDigests:     append([]string(nil), image.RepoDigests...),
			SizeBytes:       image.SizeBytes,
			SharedSizeBytes: image.SharedSizeBytes,
			Containers:      image.Containers,
			CreatedAt:       image.CreatedAt,
			Labels:          cloneStringMap(image.Labels),
		}.NormalizeCollections())
	}
	return result
}

func convertDockerVolumes(volumes []agentsdocker.Volume) []models.DockerVolume {
	if len(volumes) == 0 {
		return nil
	}
	result := make([]models.DockerVolume, 0, len(volumes))
	for _, volume := range volumes {
		result = append(result, models.DockerVolume{
			Name:       strings.TrimSpace(volume.Name),
			Driver:     strings.TrimSpace(volume.Driver),
			Mountpoint: strings.TrimSpace(volume.Mountpoint),
			Scope:      strings.TrimSpace(volume.Scope),
			CreatedAt:  strings.TrimSpace(volume.CreatedAt),
			SizeBytes:  volume.SizeBytes,
			RefCount:   volume.RefCount,
			Labels:     cloneStringMap(volume.Labels),
			Options:    cloneStringMap(volume.Options),
		}.NormalizeCollections())
	}
	return result
}

func convertDockerNetworks(networks []agentsdocker.Network) []models.DockerNetwork {
	if len(networks) == 0 {
		return nil
	}
	result := make([]models.DockerNetwork, 0, len(networks))
	for _, network := range networks {
		subnets := make([]models.DockerNetworkSubnet, 0, len(network.Subnets))
		for _, subnet := range network.Subnets {
			subnets = append(subnets, models.DockerNetworkSubnet{
				Subnet:  strings.TrimSpace(subnet.Subnet),
				Gateway: strings.TrimSpace(subnet.Gateway),
			})
		}
		result = append(result, models.DockerNetwork{
			ID:         strings.TrimSpace(network.ID),
			Name:       strings.TrimSpace(network.Name),
			Driver:     strings.TrimSpace(network.Driver),
			Scope:      strings.TrimSpace(network.Scope),
			CreatedAt:  network.CreatedAt,
			EnableIPv4: network.EnableIPv4,
			EnableIPv6: network.EnableIPv6,
			Internal:   network.Internal,
			Attachable: network.Attachable,
			Ingress:    network.Ingress,
			ConfigOnly: network.ConfigOnly,
			Subnets:    subnets,
			Labels:     cloneStringMap(network.Labels),
			Options:    cloneStringMap(network.Options),
		}.NormalizeCollections())
	}
	return result
}

func convertDockerStorageUsage(usage *agentsdocker.StorageUsage) *models.DockerStorageUsage {
	if usage == nil {
		return nil
	}
	return &models.DockerStorageUsage{
		Images:     convertDockerStorageUsageBucket(usage.Images),
		Containers: convertDockerStorageUsageBucket(usage.Containers),
		Volumes:    convertDockerStorageUsageBucket(usage.Volumes),
		BuildCache: convertDockerStorageUsageBucket(usage.BuildCache),
	}
}

func convertDockerStorageUsageBucket(bucket agentsdocker.StorageUsageBucket) models.DockerStorageUsageBucket {
	return models.DockerStorageUsageBucket{
		TotalCount:       bucket.TotalCount,
		ActiveCount:      bucket.ActiveCount,
		TotalSizeBytes:   bucket.TotalSizeBytes,
		ReclaimableBytes: bucket.ReclaimableBytes,
	}
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

func convertDockerNodes(nodes []agentsdocker.Node) []models.DockerNode {
	if len(nodes) == 0 {
		return nil
	}

	result := make([]models.DockerNode, 0, len(nodes))
	for _, node := range nodes {
		modelNode := models.DockerNode{
			ID:                  strings.TrimSpace(node.ID),
			Hostname:            strings.TrimSpace(node.Hostname),
			Role:                strings.TrimSpace(node.Role),
			Availability:        strings.TrimSpace(node.Availability),
			State:               strings.TrimSpace(node.State),
			Message:             strings.TrimSpace(node.Message),
			Address:             strings.TrimSpace(node.Address),
			ManagerReachability: strings.TrimSpace(node.ManagerReachability),
			ManagerAddress:      strings.TrimSpace(node.ManagerAddress),
			Leader:              node.Leader,
			EngineVersion:       strings.TrimSpace(node.EngineVersion),
			OS:                  strings.TrimSpace(node.OS),
			Architecture:        strings.TrimSpace(node.Architecture),
			NanoCPUs:            node.NanoCPUs,
			MemoryBytes:         node.MemoryBytes,
			Labels:              cloneStringMap(node.Labels),
			EngineLabels:        cloneStringMap(node.EngineLabels),
			CreatedAt:           node.CreatedAt,
		}
		if node.UpdatedAt != nil && !node.UpdatedAt.IsZero() {
			updated := *node.UpdatedAt
			modelNode.UpdatedAt = &updated
		}
		result = append(result, modelNode.NormalizeCollections())
	}

	return result
}

func convertDockerSecrets(secrets []agentsdocker.Secret) []models.DockerSecret {
	if len(secrets) == 0 {
		return nil
	}

	result := make([]models.DockerSecret, 0, len(secrets))
	for _, secret := range secrets {
		modelSecret := models.DockerSecret{
			ID:               strings.TrimSpace(secret.ID),
			Name:             strings.TrimSpace(secret.Name),
			Labels:           cloneStringMap(secret.Labels),
			DriverName:       strings.TrimSpace(secret.DriverName),
			TemplatingDriver: strings.TrimSpace(secret.TemplatingDriver),
			CreatedAt:        secret.CreatedAt,
		}
		if secret.UpdatedAt != nil && !secret.UpdatedAt.IsZero() {
			updated := *secret.UpdatedAt
			modelSecret.UpdatedAt = &updated
		}
		result = append(result, modelSecret.NormalizeCollections())
	}

	return result
}

func convertDockerConfigs(configs []agentsdocker.Config) []models.DockerConfig {
	if len(configs) == 0 {
		return nil
	}

	result := make([]models.DockerConfig, 0, len(configs))
	for _, config := range configs {
		modelConfig := models.DockerConfig{
			ID:               strings.TrimSpace(config.ID),
			Name:             strings.TrimSpace(config.Name),
			Labels:           cloneStringMap(config.Labels),
			TemplatingDriver: strings.TrimSpace(config.TemplatingDriver),
			CreatedAt:        config.CreatedAt,
		}
		if config.UpdatedAt != nil && !config.UpdatedAt.IsZero() {
			updated := *config.UpdatedAt
			modelConfig.UpdatedAt = &updated
		}
		result = append(result, modelConfig.NormalizeCollections())
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

func hasReportableAgentDockerSwarmInfo(info *agentsdocker.SwarmInfo) bool {
	if info == nil {
		return false
	}
	state := strings.ToLower(strings.TrimSpace(info.LocalState))
	if state == "inactive" {
		return info.ControlAvailable ||
			strings.TrimSpace(info.ClusterID) != "" ||
			strings.TrimSpace(info.ClusterName) != "" ||
			strings.TrimSpace(info.Error) != ""
	}

	return strings.TrimSpace(info.NodeID) != "" ||
		state != "" ||
		info.ControlAvailable ||
		strings.TrimSpace(info.ClusterID) != "" ||
		strings.TrimSpace(info.ClusterName) != "" ||
		strings.TrimSpace(info.Error) != ""
}

func convertDockerSwarmInfo(info *agentsdocker.SwarmInfo) *models.DockerSwarmInfo {
	if !hasReportableAgentDockerSwarmInfo(info) {
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

func isLegacyAgent(agentType string) bool {
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
			Controller:  disk.Controller,
			Target:      disk.Target,
			SizeBytes:   disk.SizeBytes,
			Temperature: disk.Temperature,
			Health:      disk.Health,
			Standby:     disk.Standby,
			Pool:        disk.Pool,
			Collection:  diskinventory.CloneStatus(disk.Collection),
		}
		if disk.IO != nil {
			ioCopy := models.DiskIO{
				Device:     disk.IO.Device,
				ReadBytes:  disk.IO.ReadBytes,
				WriteBytes: disk.IO.WriteBytes,
				ReadOps:    disk.IO.ReadOps,
				WriteOps:   disk.IO.WriteOps,
				ReadTime:   disk.IO.ReadTime,
				WriteTime:  disk.IO.WriteTime,
				IOTime:     disk.IO.IOTime,
			}
			entry.IO = &ioCopy
		}
		if disk.Attributes != nil {
			entry.Attributes = convertAgentSMARTAttributes(disk.Attributes)
		}
		result = append(result, entry)
	}
	return result
}

func convertAgentGPUToModels(gpus []agentshost.GPUSensor) []models.HostGPUSensor {
	if len(gpus) == 0 {
		return nil
	}
	result := make([]models.HostGPUSensor, len(gpus))
	for i, gpu := range gpus {
		result[i] = models.HostGPUSensor{
			ID:                 gpu.ID,
			Name:               gpu.Name,
			TemperatureCelsius: cloneFloat64Ptr(gpu.TemperatureCelsius),
			UtilizationPercent: cloneFloat64Ptr(gpu.UtilizationPercent),
			MemoryUsedBytes:    cloneInt64Ptr(gpu.MemoryUsedBytes),
			MemoryTotalBytes:   cloneInt64Ptr(gpu.MemoryTotalBytes),
		}
	}
	return result
}

func convertAgentThermalStateToModels(src *agentshost.ThermalState) *models.HostThermalState {
	if src == nil {
		return nil
	}
	return &models.HostThermalState{
		Source:                  strings.TrimSpace(src.Source),
		Pressure:                strings.TrimSpace(src.Pressure),
		ThermalWarningLevel:     cloneIntPtr(src.ThermalWarningLevel),
		PerformanceWarningLevel: cloneIntPtr(src.PerformanceWarningLevel),
		CPUPowerStatus:          cloneIntPtr(src.CPUPowerStatus),
		LimitsPercent:           cloneStringIntMap(src.LimitsPercent),
	}
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
	clusterID := ceph.FSID
	if clusterID == "" {
		clusterID = "agent-ceph-" + hostID
	}

	cluster := models.CephCluster{
		ID:             clusterID,
		Instance:       hostname,
		Source:         models.CephClusterSourceHostAgent,
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
