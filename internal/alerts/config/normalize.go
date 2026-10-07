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

// NormalizePBSDefaults ensures PBS server threshold defaults exist.
// Trigger=0 is allowed and means "disable alerting for this metric".
func NormalizePBSDefaults(config *AlertConfig) {
	config.PBSDefaults.CPU = normalizeThresholdPointer(config.PBSDefaults.CPU, 80, 75, "pbs.cpu")
	config.PBSDefaults.Memory = normalizeThresholdPointer(config.PBSDefaults.Memory, 85, 80, "pbs.memory")
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

// NormalizeRecoveryOverrides rewrites per-guest backup/snapshot overrides whose
// threshold values are exact copies of the current global defaults into sparse
// overrides that carry only the enabled flag. The legacy per-guest toggle
// persisted a full copy of the globals just to flip enabled, freezing the
// threshold values so later global edits silently stopped applying to those
// guests (#1126). Zero-valued fields inherit the global default at evaluation
// time, so the rewrite is behavior-preserving at the moment it runs and lets
// the override track future global changes. Overrides whose thresholds differ
// from the current globals are left alone: persisted values cannot distinguish
// an intentional override from a legacy copy made before the globals changed.
func NormalizeRecoveryOverrides(config *AlertConfig) {
	for id, override := range config.Overrides {
		changed := false
		if b := override.Backup; b != nil {
			if b.WarningDays == config.BackupDefaults.WarningDays &&
				b.CriticalDays == config.BackupDefaults.CriticalDays &&
				b.FreshHours == config.BackupDefaults.FreshHours &&
				b.StaleHours == config.BackupDefaults.StaleHours {
				override.Backup = &BackupAlertConfig{Enabled: b.Enabled}
				changed = true
			}
		}
		if s := override.Snapshot; s != nil {
			if s.WarningDays == config.SnapshotDefaults.WarningDays &&
				s.CriticalDays == config.SnapshotDefaults.CriticalDays &&
				s.WarningSizeGiB == config.SnapshotDefaults.WarningSizeGiB &&
				s.CriticalSizeGiB == config.SnapshotDefaults.CriticalSizeGiB {
				override.Snapshot = &SnapshotAlertConfig{Enabled: s.Enabled}
				changed = true
			}
		}
		if changed {
			config.Overrides[id] = override
		}
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
	config.NodeDefaults.Temperature = normalizeThresholdPointer(config.NodeDefaults.Temperature, 80, 75, "node.temperature")
}

func NormalizeAgentDefaults(config *AlertConfig) {
	config.AgentDefaults.CPU = normalizeThresholdPointer(config.AgentDefaults.CPU, 80, 75, "agent.cpu")
	config.AgentDefaults.Memory = normalizeThresholdPointer(config.AgentDefaults.Memory, 85, 80, "agent.memory")
	config.AgentDefaults.Disk = normalizeThresholdPointer(config.AgentDefaults.Disk, 90, 85, "agent.disk")
	config.AgentDefaults.DiskTemperature = normalizeThresholdPointer(config.AgentDefaults.DiskTemperature, 55, 50, "agent.diskTemperature")
	normalizeSMARTDefaults(&config.AgentDefaults)

	NormalizeDiskFillByType(config)
	NormalizeDiskTempByType(config)
}

func normalizeSMARTDefaults(config *ThresholdConfig) {
	if config.SMARTHealthFailure == nil {
		config.SMARTHealthFailure = smartIntPtr(1)
	} else if *config.SMARTHealthFailure != 0 {
		*config.SMARTHealthFailure = 1
	}
	config.SMARTReallocated = normalizeNonNegativeInt64(config.SMARTReallocated, 1)
	config.SMARTPending = normalizeNonNegativeInt64(config.SMARTPending, 1)
	config.SMARTUncorrectable = normalizeNonNegativeInt64(config.SMARTUncorrectable, 1)
	config.SMARTMediaErrors = normalizeNonNegativeInt64(config.SMARTMediaErrors, 1)
	config.SMARTCRCErrorDelta = normalizeNonNegativeInt64(config.SMARTCRCErrorDelta, 1)
	config.SMARTLifeWarning = normalizePercentage(config.SMARTLifeWarning, 10)
	config.SMARTLifeCritical = normalizePercentage(config.SMARTLifeCritical, 5)
	config.SMARTSpareWarning = normalizePercentage(config.SMARTSpareWarning, 20)
	config.SMARTSpareCritical = normalizePercentage(config.SMARTSpareCritical, 10)

	if *config.SMARTLifeWarning > 0 && *config.SMARTLifeCritical > *config.SMARTLifeWarning {
		*config.SMARTLifeCritical = *config.SMARTLifeWarning
	}
	if *config.SMARTSpareWarning > 0 && *config.SMARTSpareCritical > *config.SMARTSpareWarning {
		*config.SMARTSpareCritical = *config.SMARTSpareWarning
	}
}

func normalizeNonNegativeInt64(value *int64, fallback int64) *int64 {
	if value == nil || *value < 0 {
		return smartInt64Ptr(fallback)
	}
	return value
}

func normalizePercentage(value *int, fallback int) *int {
	if value == nil || *value < 0 {
		return smartIntPtr(fallback)
	}
	if *value > 100 {
		*value = 100
	}
	return value
}

func smartIntPtr(value int) *int { return &value }

func smartInt64Ptr(value int64) *int64 { return &value }

// normalizeThresholdPointer is the canonical hysteresis normalizer. A missing
// or negative threshold takes the factory default, a zero trigger means off,
// and a positive trigger always keeps a clear below it: a missing clear sits
// 5 points under the trigger, floored at 0 for triggers of 5 or less, never
// the factory clear (which would sit above a low trigger), and a clear at or
// above the trigger is repaired.
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

// NormalizeHysteresisThreshold exposes normalizeThresholdPointer for other
// packages (e.g., config persistence).
func NormalizeHysteresisThreshold(current *HysteresisThreshold, defaultTrigger, defaultClear float64, metricName string) *HysteresisThreshold {
	return normalizeThresholdPointer(current, defaultTrigger, defaultClear, metricName)
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
// and resets a non-positive trigger to the default for that key. A positive
// trigger is preserved and its clear follows normalizeThresholdPointer.
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

	// Ensure all canonical keys are present with a positive trigger.
	for key, defaultVal := range defaults {
		current, ok := config.DiskFillByType[key]
		if !ok || current.Trigger <= 0 {
			config.DiskFillByType[key] = defaultVal
			continue
		}
		config.DiskFillByType[key] = *normalizeThresholdPointer(&current, defaultVal.Trigger, defaultVal.Clear, "diskFillByType."+key)
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
// and resets a non-positive trigger to the default for that key. A positive
// trigger is preserved and its clear follows normalizeThresholdPointer.
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
		if !ok || current.Trigger <= 0 {
			config.DiskTempByType[key] = defaultVal
			continue
		}
		config.DiskTempByType[key] = *normalizeThresholdPointer(&current, defaultVal.Trigger, defaultVal.Clear, "diskTempByType."+key)
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
	EnsureValidHysteresis(config.PBSDefaults.CPU, "pbs.cpu")
	EnsureValidHysteresis(config.PBSDefaults.Memory, "pbs.memory")
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
	EnsureValidHysteresis(config.AgentDefaults.CPU, "agent.cpu")
	EnsureValidHysteresis(config.AgentDefaults.Memory, "agent.memory")
	EnsureValidHysteresis(config.AgentDefaults.Disk, "agent.disk")
	EnsureValidHysteresis(config.AgentDefaults.DiskTemperature, "agent.diskTemperature")
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

const (
	// DefaultCPUEvaluationWindowSeconds smooths short-lived CPU bursts while
	// remaining responsive enough for operational alerting.
	DefaultCPUEvaluationWindowSeconds = 5 * 60
	MaxMetricEvaluationWindowSeconds  = 60 * 60
)

var supportedWindowedMetrics = map[string]struct{}{
	"cpu":        {},
	"diskread":   {},
	"diskwrite":  {},
	"networkin":  {},
	"networkout": {},
}

// NormalizeMetricEvaluationWindows canonicalizes rolling-window settings.
// Explicit zero values are retained because zero means evaluate the current
// observation. An absent global CPU rule is seeded with the safe 5-minute
// default for both new and migrated configurations.
func NormalizeMetricEvaluationWindows(input map[string]map[string]int) map[string]map[string]int {
	normalized := make(map[string]map[string]int)
	for rawType, metrics := range input {
		typeKey := CanonicalAlertResourceType(rawType)
		if typeKey == "" || (typeKey != "all" && isUnsupportedLegacyAlertResourceType(typeKey)) {
			continue
		}
		for rawMetric, window := range metrics {
			metricKey := strings.ToLower(strings.TrimSpace(rawMetric))
			if _, supported := supportedWindowedMetrics[metricKey]; !supported || window < 0 {
				continue
			}
			if window > MaxMetricEvaluationWindowSeconds {
				window = MaxMetricEvaluationWindowSeconds
			}
			if _, exists := normalized[typeKey]; !exists {
				normalized[typeKey] = make(map[string]int)
			}
			normalized[typeKey][metricKey] = window
		}
	}

	if _, exists := normalized["all"]; !exists {
		normalized["all"] = make(map[string]int)
	}
	if _, exists := normalized["all"]["cpu"]; !exists {
		normalized["all"]["cpu"] = DefaultCPUEvaluationWindowSeconds
	}
	return normalized
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
