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
