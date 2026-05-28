package config

import (
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

func EnsureValidHysteresis(threshold *HysteresisThreshold, metricName string) {
	if threshold == nil {
		return
	}
	// Disabled thresholds don't need hysteresis validation
	if threshold.Trigger <= 0 {
		return
	}
	if threshold.Clear >= threshold.Trigger {
		log.Warn().
			Str("metric", metricName).
			Float64("trigger", threshold.Trigger).
			Float64("clear", threshold.Clear).
			Msg("Invalid hysteresis: clear >= trigger, auto-fixing")
		threshold.Clear = threshold.Trigger - 5
		if threshold.Clear < 0 {
			threshold.Clear = 0
		}
	}
}

func NormalizeStorageDefaults(config *AlertConfig) {
	if config.StorageDefault.Trigger < 0 {
		config.StorageDefault.Trigger = 85
		config.StorageDefault.Clear = 80
	} else if config.StorageDefault.Trigger == 0 {
		config.StorageDefault.Clear = 0
	} else if config.StorageDefault.Clear <= 0 {
		config.StorageDefault.Clear = config.StorageDefault.Trigger - 5
		if config.StorageDefault.Clear < 0 {
			config.StorageDefault.Clear = 0
		}
	}
}

func NormalizeDockerThreshold(th HysteresisThreshold, defaultTrigger float64, metricName string) HysteresisThreshold {
	normalized := th

	if normalized.Trigger < 0 {
		normalized.Trigger = defaultTrigger
	}

	if normalized.Trigger == 0 {
		if normalized.Clear < 0 {
			normalized.Clear = 0
		}
		return normalized
	}

	if normalized.Clear <= 0 {
		normalized.Clear = normalized.Trigger - 5
		if normalized.Clear < 0 {
			normalized.Clear = 0
		}
	}

	EnsureValidHysteresis(&normalized, metricName)
	return normalized
}

func NormalizeDockerDefaults(config *AlertConfig) {
	config.DockerDefaults.CPU = NormalizeDockerThreshold(config.DockerDefaults.CPU, 80, "docker.cpu")
	config.DockerDefaults.Memory = NormalizeDockerThreshold(config.DockerDefaults.Memory, 85, "docker.memory")
	config.DockerDefaults.Disk = NormalizeDockerThreshold(config.DockerDefaults.Disk, 85, "docker.disk")

	if config.DockerDefaults.RestartCount <= 0 {
		config.DockerDefaults.RestartCount = 3
	}
	if config.DockerDefaults.RestartWindow <= 0 {
		config.DockerDefaults.RestartWindow = 300
	}
	if config.DockerDefaults.MemoryWarnPct <= 0 {
		config.DockerDefaults.MemoryWarnPct = 90
	}
	if config.DockerDefaults.MemoryCriticalPct <= 0 {
		config.DockerDefaults.MemoryCriticalPct = 95
	}
	if config.DockerDefaults.ServiceWarnGapPct <= 0 {
		config.DockerDefaults.ServiceWarnGapPct = 10
	}
	if config.DockerDefaults.ServiceCritGapPct <= 0 {
		config.DockerDefaults.ServiceCritGapPct = 50
	}
	if config.DockerDefaults.ServiceCritGapPct > 0 &&
		config.DockerDefaults.ServiceCritGapPct < config.DockerDefaults.ServiceWarnGapPct {
		log.Warn().
			Int("warnGapPercent", config.DockerDefaults.ServiceWarnGapPct).
			Int("criticalGapPercent", config.DockerDefaults.ServiceCritGapPct).
			Msg("Adjusting Docker service critical gap to match warning gap")
		config.DockerDefaults.ServiceCritGapPct = config.DockerDefaults.ServiceWarnGapPct
	}
	if config.DockerDefaults.StatePoweredOffSeverity == "" {
		config.DockerDefaults.StatePoweredOffSeverity = AlertLevelWarning
	}
	config.DockerDefaults.StatePoweredOffSeverity = NormalizePoweredOffSeverity(config.DockerDefaults.StatePoweredOffSeverity)
	if config.DockerDefaults.UpdateAlertDelayHours == 0 {
		config.DockerDefaults.UpdateAlertDelayHours = 24
	}
}

func NormalizePMGDefaults(config *AlertConfig) {
	if config.PMGDefaults.QueueTotalWarning <= 0 {
		config.PMGDefaults.QueueTotalWarning = 500
	}
	if config.PMGDefaults.QueueTotalCritical <= 0 {
		config.PMGDefaults.QueueTotalCritical = 1000
	}
	if config.PMGDefaults.OldestMessageWarnMins <= 0 {
		config.PMGDefaults.OldestMessageWarnMins = 30
	}
	if config.PMGDefaults.OldestMessageCritMins <= 0 {
		config.PMGDefaults.OldestMessageCritMins = 60
	}
	if config.PMGDefaults.DeferredQueueWarn <= 0 {
		config.PMGDefaults.DeferredQueueWarn = 200
	}
	if config.PMGDefaults.DeferredQueueCritical <= 0 {
		config.PMGDefaults.DeferredQueueCritical = 500
	}
	if config.PMGDefaults.HoldQueueWarn <= 0 {
		config.PMGDefaults.HoldQueueWarn = 100
	}
	if config.PMGDefaults.HoldQueueCritical <= 0 {
		config.PMGDefaults.HoldQueueCritical = 300
	}
	if config.PMGDefaults.QuarantineSpamWarn <= 0 {
		config.PMGDefaults.QuarantineSpamWarn = 2000
	}
	if config.PMGDefaults.QuarantineSpamCritical <= 0 {
		config.PMGDefaults.QuarantineSpamCritical = 5000
	}
	if config.PMGDefaults.QuarantineVirusWarn <= 0 {
		config.PMGDefaults.QuarantineVirusWarn = 2000
	}
	if config.PMGDefaults.QuarantineVirusCritical <= 0 {
		config.PMGDefaults.QuarantineVirusCritical = 5000
	}
	if config.PMGDefaults.QuarantineGrowthWarnPct <= 0 {
		config.PMGDefaults.QuarantineGrowthWarnPct = 25
	}
	if config.PMGDefaults.QuarantineGrowthWarnMin <= 0 {
		config.PMGDefaults.QuarantineGrowthWarnMin = 250
	}
	if config.PMGDefaults.QuarantineGrowthCritPct <= 0 {
		config.PMGDefaults.QuarantineGrowthCritPct = 50
	}
	if config.PMGDefaults.QuarantineGrowthCritMin <= 0 {
		config.PMGDefaults.QuarantineGrowthCritMin = 500
	}
}

func NormalizeSnapshotDefaults(config *AlertConfig) {
	if config.SnapshotDefaults.WarningDays < 0 {
		config.SnapshotDefaults.WarningDays = 0
	}
	if config.SnapshotDefaults.CriticalDays < 0 {
		config.SnapshotDefaults.CriticalDays = 0
	}
	if config.SnapshotDefaults.CriticalDays > 0 && config.SnapshotDefaults.WarningDays > config.SnapshotDefaults.CriticalDays {
		config.SnapshotDefaults.WarningDays = config.SnapshotDefaults.CriticalDays
	}
	if config.SnapshotDefaults.CriticalDays == 0 && config.SnapshotDefaults.WarningDays > 0 {
		config.SnapshotDefaults.CriticalDays = config.SnapshotDefaults.WarningDays
	}
	if config.SnapshotDefaults.WarningSizeGiB < 0 {
		config.SnapshotDefaults.WarningSizeGiB = 0
	}
	if config.SnapshotDefaults.CriticalSizeGiB < 0 {
		config.SnapshotDefaults.CriticalSizeGiB = 0
	}
	if config.SnapshotDefaults.CriticalSizeGiB > 0 && config.SnapshotDefaults.WarningSizeGiB > config.SnapshotDefaults.CriticalSizeGiB {
		config.SnapshotDefaults.WarningSizeGiB = config.SnapshotDefaults.CriticalSizeGiB
	}
	if config.SnapshotDefaults.CriticalSizeGiB == 0 && config.SnapshotDefaults.WarningSizeGiB > 0 {
		config.SnapshotDefaults.CriticalSizeGiB = config.SnapshotDefaults.WarningSizeGiB
	}
}

func NormalizeBackupDefaults(config *AlertConfig) {
	if config.BackupDefaults.WarningDays < 0 {
		config.BackupDefaults.WarningDays = 0
	}
	if config.BackupDefaults.CriticalDays < 0 {
		config.BackupDefaults.CriticalDays = 0
	}
	if config.BackupDefaults.CriticalDays > 0 && config.BackupDefaults.WarningDays > config.BackupDefaults.CriticalDays {
		config.BackupDefaults.WarningDays = config.BackupDefaults.CriticalDays
	}
	if config.BackupDefaults.FreshHours <= 0 {
		config.BackupDefaults.FreshHours = 24
	}
	if config.BackupDefaults.StaleHours <= 0 {
		config.BackupDefaults.StaleHours = 72
	}
	if config.BackupDefaults.StaleHours < config.BackupDefaults.FreshHours {
		config.BackupDefaults.StaleHours = config.BackupDefaults.FreshHours
	}
	if config.BackupDefaults.AlertOrphaned == nil {
		alertOrphaned := true
		config.BackupDefaults.AlertOrphaned = &alertOrphaned
	}
	if len(config.BackupDefaults.IgnoreVMIDs) > 0 {
		seen := make(map[string]struct{}, len(config.BackupDefaults.IgnoreVMIDs))
		normalized := make([]string, 0, len(config.BackupDefaults.IgnoreVMIDs))
		for _, entry := range config.BackupDefaults.IgnoreVMIDs {
			value := strings.TrimSpace(entry)
			if value == "" {
				continue
			}
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			normalized = append(normalized, value)
		}
		config.BackupDefaults.IgnoreVMIDs = normalized
	}
}

func NormalizeNodeDefaults(config *AlertConfig) {
	if config.NodeDefaults.Temperature == nil || config.NodeDefaults.Temperature.Trigger < 0 {
		config.NodeDefaults.Temperature = &HysteresisThreshold{Trigger: 80, Clear: 75}
	} else if config.NodeDefaults.Temperature.Trigger == 0 {
		config.NodeDefaults.Temperature.Clear = 0
	} else if config.NodeDefaults.Temperature.Clear <= 0 {
		config.NodeDefaults.Temperature.Clear = config.NodeDefaults.Temperature.Trigger - 5
		if config.NodeDefaults.Temperature.Clear <= 0 {
			config.NodeDefaults.Temperature.Clear = 75
		}
	}
}

func NormalizeAgentDefaults(config *AlertConfig) {
	if config.AgentDefaults.CPU == nil || config.AgentDefaults.CPU.Trigger < 0 {
		config.AgentDefaults.CPU = &HysteresisThreshold{Trigger: 80, Clear: 75}
	} else if config.AgentDefaults.CPU.Trigger == 0 {
		config.AgentDefaults.CPU.Clear = 0
	} else if config.AgentDefaults.CPU.Clear <= 0 {
		config.AgentDefaults.CPU.Clear = config.AgentDefaults.CPU.Trigger - 5
		if config.AgentDefaults.CPU.Clear <= 0 {
			config.AgentDefaults.CPU.Clear = 75
		}
	}
	if config.AgentDefaults.Memory == nil || config.AgentDefaults.Memory.Trigger < 0 {
		config.AgentDefaults.Memory = &HysteresisThreshold{Trigger: 85, Clear: 80}
	} else if config.AgentDefaults.Memory.Trigger == 0 {
		config.AgentDefaults.Memory.Clear = 0
	} else if config.AgentDefaults.Memory.Clear <= 0 {
		config.AgentDefaults.Memory.Clear = config.AgentDefaults.Memory.Trigger - 5
		if config.AgentDefaults.Memory.Clear <= 0 {
			config.AgentDefaults.Memory.Clear = 80
		}
	}
	if config.AgentDefaults.Disk == nil || config.AgentDefaults.Disk.Trigger < 0 {
		config.AgentDefaults.Disk = &HysteresisThreshold{Trigger: 90, Clear: 85}
	} else if config.AgentDefaults.Disk.Trigger == 0 {
		config.AgentDefaults.Disk.Clear = 0
	} else if config.AgentDefaults.Disk.Clear <= 0 {
		config.AgentDefaults.Disk.Clear = config.AgentDefaults.Disk.Trigger - 5
		if config.AgentDefaults.Disk.Clear <= 0 {
			config.AgentDefaults.Disk.Clear = 85
		}
	}

	if config.AgentDefaults.DiskTemperature == nil || config.AgentDefaults.DiskTemperature.Trigger < 0 {
		config.AgentDefaults.DiskTemperature = &HysteresisThreshold{Trigger: 55, Clear: 50}
	} else if config.AgentDefaults.DiskTemperature.Trigger == 0 {
		config.AgentDefaults.DiskTemperature.Clear = 0
	} else if config.AgentDefaults.DiskTemperature.Clear <= 0 {
		config.AgentDefaults.DiskTemperature.Clear = config.AgentDefaults.DiskTemperature.Trigger - 5
		if config.AgentDefaults.DiskTemperature.Clear <= 0 {
			config.AgentDefaults.DiskTemperature.Clear = 50
		}
	}
	EnsureValidHysteresis(config.AgentDefaults.DiskTemperature, "agent.diskTemperature")

	NormalizeDiskFillByType(config)
	NormalizeDiskTempByType(config)
}

func normalizeThresholdPointer(
	current *HysteresisThreshold,
	defaultTrigger float64,
	defaultClear float64,
	metricName string,
) *HysteresisThreshold {
	if current == nil || current.Trigger < 0 {
		return &HysteresisThreshold{Trigger: defaultTrigger, Clear: defaultClear}
	}
	normalized := *current
	if normalized.Trigger == 0 {
		normalized.Clear = 0
		return &normalized
	}
	if normalized.Clear <= 0 {
		normalized.Clear = normalized.Trigger - 5
		if normalized.Clear < 0 {
			normalized.Clear = 0
		}
	}
	EnsureValidHysteresis(&normalized, metricName)
	return &normalized
}

func NormalizeKubernetesDefaults(config *AlertConfig) {
	config.KubernetesDefaults.CPU = normalizeThresholdPointer(config.KubernetesDefaults.CPU, 80, 75, "kubernetes.cpu")
	config.KubernetesDefaults.Memory = normalizeThresholdPointer(config.KubernetesDefaults.Memory, 85, 80, "kubernetes.memory")
	config.KubernetesDefaults.Disk = normalizeThresholdPointer(config.KubernetesDefaults.Disk, 90, 85, "kubernetes.disk")

	config.KubernetesDefaults.DiskRead = normalizeThresholdPointer(config.KubernetesDefaults.DiskRead, 0, 0, "kubernetes.diskRead")
	config.KubernetesDefaults.DiskWrite = normalizeThresholdPointer(config.KubernetesDefaults.DiskWrite, 0, 0, "kubernetes.diskWrite")
	config.KubernetesDefaults.NetworkIn = normalizeThresholdPointer(config.KubernetesDefaults.NetworkIn, 0, 0, "kubernetes.networkIn")
	config.KubernetesDefaults.NetworkOut = normalizeThresholdPointer(config.KubernetesDefaults.NetworkOut, 0, 0, "kubernetes.networkOut")
}

func NormalizeTrueNASDefaults(config *AlertConfig) {
	config.TrueNASDefaults.CPU = normalizeThresholdPointer(config.TrueNASDefaults.CPU, 80, 75, "truenas.cpu")
	config.TrueNASDefaults.Memory = normalizeThresholdPointer(config.TrueNASDefaults.Memory, 85, 80, "truenas.memory")
	config.TrueNASDefaults.Disk = normalizeThresholdPointer(config.TrueNASDefaults.Disk, 85, 80, "truenas.disk")
	config.TrueNASDefaults.Usage = normalizeThresholdPointer(config.TrueNASDefaults.Usage, 85, 80, "truenas.usage")
	config.TrueNASDefaults.Temperature = normalizeThresholdPointer(config.TrueNASDefaults.Temperature, 80, 75, "truenas.temperature")
	config.TrueNASDefaults.DiskRead = normalizeThresholdPointer(config.TrueNASDefaults.DiskRead, 0, 0, "truenas.diskRead")
	config.TrueNASDefaults.DiskWrite = normalizeThresholdPointer(config.TrueNASDefaults.DiskWrite, 0, 0, "truenas.diskWrite")
	config.TrueNASDefaults.NetworkIn = normalizeThresholdPointer(config.TrueNASDefaults.NetworkIn, 0, 0, "truenas.networkIn")
	config.TrueNASDefaults.NetworkOut = normalizeThresholdPointer(config.TrueNASDefaults.NetworkOut, 0, 0, "truenas.networkOut")

	config.TrueNASDiskDefaults.Temperature = normalizeThresholdPointer(config.TrueNASDiskDefaults.Temperature, 55, 50, "truenas.disk.temperature")
}

func NormalizeVMwareDefaults(config *AlertConfig) {
	config.VMwareDefaults.CPU = normalizeThresholdPointer(config.VMwareDefaults.CPU, 80, 75, "vmware.cpu")
	config.VMwareDefaults.Memory = normalizeThresholdPointer(config.VMwareDefaults.Memory, 85, 80, "vmware.memory")
	config.VMwareDefaults.Disk = normalizeThresholdPointer(config.VMwareDefaults.Disk, 90, 85, "vmware.disk")
	config.VMwareDefaults.Usage = normalizeThresholdPointer(config.VMwareDefaults.Usage, 85, 80, "vmware.usage")
	config.VMwareDefaults.DiskRead = normalizeThresholdPointer(config.VMwareDefaults.DiskRead, 0, 0, "vmware.diskRead")
	config.VMwareDefaults.DiskWrite = normalizeThresholdPointer(config.VMwareDefaults.DiskWrite, 0, 0, "vmware.diskWrite")
	config.VMwareDefaults.NetworkIn = normalizeThresholdPointer(config.VMwareDefaults.NetworkIn, 0, 0, "vmware.networkIn")
	config.VMwareDefaults.NetworkOut = normalizeThresholdPointer(config.VMwareDefaults.NetworkOut, 0, 0, "vmware.networkOut")
}

// diskFillByTypeDefaults returns the canonical per-type fill-% defaults.
// Keys are lowercase hardware type strings.
func diskFillByTypeDefaults() map[string]HysteresisThreshold {
	return map[string]HysteresisThreshold{
		"nvme": {Trigger: 92, Clear: 87},
		"sata": {Trigger: 90, Clear: 85},
		"hdd":  {Trigger: 85, Clear: 80},
	}
}

// NormalizeDiskFillByType ensures AlertConfig.DiskFillByType is seeded with
// lowercase nvme/sata/hdd defaults when nil, lowercases any existing keys,
// and resets non-positive trigger or clear values to the default for that
// key. Operator-customized positive values are preserved.
func NormalizeDiskFillByType(config *AlertConfig) {
	defaults := diskFillByTypeDefaults()
	if config.DiskFillByType == nil {
		copyMap := make(map[string]HysteresisThreshold, len(defaults))
		for k, v := range defaults {
			copyMap[k] = v
		}
		config.DiskFillByType = copyMap
		return
	}

	// Lowercase any non-lowercase keys, moving values into the canonical position.
	for key, value := range config.DiskFillByType {
		lower := strings.ToLower(strings.TrimSpace(key))
		if lower == key {
			continue
		}
		delete(config.DiskFillByType, key)
		if lower == "" {
			continue
		}
		if _, exists := config.DiskFillByType[lower]; !exists {
			config.DiskFillByType[lower] = value
		}
	}

	// Ensure all canonical keys are present and have positive trigger/clear values.
	for key, defaultVal := range defaults {
		current, ok := config.DiskFillByType[key]
		if !ok {
			config.DiskFillByType[key] = defaultVal
			continue
		}
		if current.Trigger <= 0 || current.Clear <= 0 {
			config.DiskFillByType[key] = defaultVal
		}
	}
}

// diskTempByTypeDefaults returns the canonical per-type SMART temperature defaults.
// Keys are lowercase HostDiskSMART.Type values.
func diskTempByTypeDefaults() map[string]HysteresisThreshold {
	return map[string]HysteresisThreshold{
		"nvme": {Trigger: 70, Clear: 65},
		"sas":  {Trigger: 65, Clear: 60},
		"sata": {Trigger: 55, Clear: 50},
	}
}

// NormalizeDiskTempByType ensures AlertConfig.DiskTempByType is seeded with
// lowercase nvme/sas/sata defaults when nil, lowercases any existing keys,
// and resets non-positive trigger or clear values to the default for that key.
// Operator-customized positive values are preserved.
func NormalizeDiskTempByType(config *AlertConfig) {
	defaults := diskTempByTypeDefaults()
	if config.DiskTempByType == nil {
		copyMap := make(map[string]HysteresisThreshold, len(defaults))
		for k, v := range defaults {
			copyMap[k] = v
		}
		config.DiskTempByType = copyMap
		return
	}

	for key, value := range config.DiskTempByType {
		lower := strings.ToLower(strings.TrimSpace(key))
		if lower == key {
			continue
		}
		delete(config.DiskTempByType, key)
		if lower == "" {
			continue
		}
		if _, exists := config.DiskTempByType[lower]; !exists {
			config.DiskTempByType[lower] = value
		}
	}

	for key, defaultVal := range defaults {
		current, ok := config.DiskTempByType[key]
		if !ok {
			config.DiskTempByType[key] = defaultVal
			continue
		}
		if current.Trigger <= 0 || current.Clear <= 0 {
			config.DiskTempByType[key] = defaultVal
		}
	}
}

func NormalizeGeneralSettings(config *AlertConfig) {
	if config.MinimumDelta <= 0 {
		config.MinimumDelta = 2.0
	}
	if config.SuppressionWindow <= 0 {
		config.SuppressionWindow = 5
	}
	if config.HysteresisMargin <= 0 {
		config.HysteresisMargin = 5.0
	}
	if config.ObservationWindowHours <= 0 {
		config.ObservationWindowHours = 24
	}
	if config.FlappingWindowSeconds <= 0 {
		config.FlappingWindowSeconds = 300
	}
	if config.FlappingThreshold <= 0 {
		config.FlappingThreshold = 5
	}
	if config.FlappingCooldownMinutes <= 0 {
		config.FlappingCooldownMinutes = 15
	}
}

func NormalizeTimeThresholds(config *AlertConfig) {
	NormalizeAlertConfigAliases(config)
	config.MetricTimeThresholds = normalizeMetricTimeThresholds(config.MetricTimeThresholds)

	const defaultDelaySeconds = 5
	if config.TimeThresholds == nil {
		config.TimeThresholds = make(map[string]int)
	}
	ensureDelay := func(key string) {
		delay, ok := config.TimeThresholds[key]
		if !ok || delay < 0 {
			config.TimeThresholds[key] = defaultDelaySeconds
		}
	}
	ensureDelay("guest")
	ensureDelay("node")
	ensureDelay("storage")
	ensureDelay("pbs")
	ensureDelay("agent")
	ensureDelay("k8s-cluster")
	ensureDelay("k8s-node")
	ensureDelay("k8s-deployment")
	ensureDelay("k8s-namespace")
	ensureDelay("pod")
	ensureDelay("truenas-system")
	ensureDelay("truenas-pool")
	ensureDelay("truenas-dataset")
	ensureDelay("truenas-disk")
	ensureDelay("vmware-host")
	ensureDelay("vmware-vm")
	ensureDelay("vmware-datastore")
	ensureDelay("vmware-network")
	if delay, ok := config.TimeThresholds["all"]; ok && delay < 0 {
		config.TimeThresholds["all"] = defaultDelaySeconds
	}
}

func ValidateHysteresisThresholds(config *AlertConfig) {
	EnsureValidHysteresis(config.GuestDefaults.CPU, "guest.cpu")
	EnsureValidHysteresis(config.GuestDefaults.Memory, "guest.memory")
	EnsureValidHysteresis(config.GuestDefaults.Disk, "guest.disk")
	EnsureValidHysteresis(config.NodeDefaults.CPU, "node.cpu")
	EnsureValidHysteresis(config.NodeDefaults.Memory, "node.memory")
	EnsureValidHysteresis(config.NodeDefaults.Temperature, "node.temperature")
	EnsureValidHysteresis(&config.StorageDefault, "storage")
	EnsureValidHysteresis(config.KubernetesDefaults.CPU, "kubernetes.cpu")
	EnsureValidHysteresis(config.KubernetesDefaults.Memory, "kubernetes.memory")
	EnsureValidHysteresis(config.KubernetesDefaults.Disk, "kubernetes.disk")
	EnsureValidHysteresis(config.TrueNASDefaults.CPU, "truenas.cpu")
	EnsureValidHysteresis(config.TrueNASDefaults.Memory, "truenas.memory")
	EnsureValidHysteresis(config.TrueNASDefaults.Disk, "truenas.disk")
	EnsureValidHysteresis(config.TrueNASDefaults.Usage, "truenas.usage")
	EnsureValidHysteresis(config.TrueNASDefaults.Temperature, "truenas.temperature")
	EnsureValidHysteresis(config.TrueNASDiskDefaults.Temperature, "truenas.disk.temperature")
	EnsureValidHysteresis(config.VMwareDefaults.CPU, "vmware.cpu")
	EnsureValidHysteresis(config.VMwareDefaults.Memory, "vmware.memory")
	EnsureValidHysteresis(config.VMwareDefaults.Disk, "vmware.disk")
	EnsureValidHysteresis(config.VMwareDefaults.Usage, "vmware.usage")
}

func ValidateQuietHoursTimezone(config *AlertConfig) {
	if config.Schedule.QuietHours.Enabled && config.Schedule.QuietHours.Timezone != "" {
		_, err := time.LoadLocation(config.Schedule.QuietHours.Timezone)
		if err != nil {
			log.Error().
				Err(err).
				Str("timezone", config.Schedule.QuietHours.Timezone).
				Msg("Invalid timezone in quiet hours config, disabling quiet hours")
			config.Schedule.QuietHours.Enabled = false
		}
	}
}

func normalizeMetricTimeThresholds(input map[string]map[string]int) map[string]map[string]int {
	if len(input) == 0 {
		return nil
	}

	normalized := make(map[string]map[string]int)
	for rawType, metrics := range input {
		typeKey := CanonicalAlertResourceType(rawType)
		if typeKey == "" || len(metrics) == 0 {
			continue
		}
		if typeKey != "all" && isUnsupportedLegacyAlertResourceType(typeKey) {
			continue
		}
		for rawMetric, delay := range metrics {
			metricKey := strings.ToLower(strings.TrimSpace(rawMetric))
			if metricKey == "" || delay < 0 {
				continue
			}
			if _, exists := normalized[typeKey]; !exists {
				normalized[typeKey] = make(map[string]int)
			}
			normalized[typeKey][metricKey] = delay
		}
	}

	if len(normalized) == 0 {
		return nil
	}

	return normalized
}

// NormalizeMetricTimeThresholds exposes normalization for other packages (e.g., config persistence).
func NormalizeMetricTimeThresholds(input map[string]map[string]int) map[string]map[string]int {
	return normalizeMetricTimeThresholds(input)
}

// NormalizeDockerIgnoredPrefixes trims, deduplicates, and lowercases comparison keys for ignored Docker containers.
func NormalizeDockerIgnoredPrefixes(prefixes []string) []string {
	if len(prefixes) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(prefixes))
	normalized := make([]string, 0, len(prefixes))

	for _, prefix := range prefixes {
		trimmed := strings.TrimSpace(prefix)
		if trimmed == "" {
			continue
		}

		lower := strings.ToLower(trimmed)
		if _, exists := seen[lower]; exists {
			continue
		}
		seen[lower] = struct{}{}
		normalized = append(normalized, trimmed)
	}

	if len(normalized) == 0 {
		return nil
	}

	return normalized
}
