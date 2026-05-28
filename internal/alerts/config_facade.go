package alerts

import (
	"errors"

	alertconfig "github.com/rcourtman/pulse-go-rewrite/internal/alerts/config"
)

// Type aliases re-exported from alerts/config for backward compatibility.
// These guarantee compile-time type identity: alerts.AlertLevel = alertconfig.AlertLevel.
type AlertLevel = alertconfig.AlertLevel
type ActivationState = alertconfig.ActivationState
type HysteresisThreshold = alertconfig.HysteresisThreshold
type ThresholdConfig = alertconfig.ThresholdConfig
type QuietHours = alertconfig.QuietHours
type QuietHoursSuppression = alertconfig.QuietHoursSuppression
type EscalationLevel = alertconfig.EscalationLevel
type EscalationConfig = alertconfig.EscalationConfig
type GroupingConfig = alertconfig.GroupingConfig
type ScheduleConfig = alertconfig.ScheduleConfig
type FilterCondition = alertconfig.FilterCondition
type FilterStack = alertconfig.FilterStack
type CustomAlertRule = alertconfig.CustomAlertRule
type DockerThresholdConfig = alertconfig.DockerThresholdConfig
type PMGThresholdConfig = alertconfig.PMGThresholdConfig
type SnapshotAlertConfig = alertconfig.SnapshotAlertConfig
type BackupAlertConfig = alertconfig.BackupAlertConfig
type GuestLookup = alertconfig.GuestLookup
type AlertConfig = alertconfig.AlertConfig

const (
	AlertLevelWarning  = alertconfig.AlertLevelWarning
	AlertLevelCritical = alertconfig.AlertLevelCritical
	ActivationPending  = alertconfig.ActivationPending
	ActivationActive   = alertconfig.ActivationActive
	ActivationSnoozed  = alertconfig.ActivationSnoozed
)

var ErrAlertNotFound = errors.New("alert not found")

func NormalizeAlertConfigAliases(config *AlertConfig) {
	alertconfig.NormalizeAlertConfigAliases(config)
}

func NormalizeMetricTimeThresholds(input map[string]map[string]int) map[string]map[string]int {
	return alertconfig.NormalizeMetricTimeThresholds(input)
}

func NormalizeDockerIgnoredPrefixes(prefixes []string) []string {
	return alertconfig.NormalizeDockerIgnoredPrefixes(prefixes)
}

func CanonicalResourceTypeKeys(resourceType string) []string {
	return alertconfig.CanonicalResourceTypeKeys(resourceType)
}

func NormalizePoweredOffSeverity(level AlertLevel) AlertLevel {
	return alertconfig.NormalizePoweredOffSeverity(level)
}

func normalizePoweredOffSeverity(level AlertLevel) AlertLevel {
	return alertconfig.NormalizePoweredOffSeverity(level)
}

func ensureValidHysteresis(threshold *HysteresisThreshold, metricName string) {
	alertconfig.EnsureValidHysteresis(threshold, metricName)
}

func normalizeStorageDefaults(config *AlertConfig) { alertconfig.NormalizeStorageDefaults(config) }

func normalizeDockerThreshold(th HysteresisThreshold, defaultTrigger float64, metricName string) HysteresisThreshold {
	return alertconfig.NormalizeDockerThreshold(th, defaultTrigger, metricName)
}

func normalizeDockerDefaults(config *AlertConfig)   { alertconfig.NormalizeDockerDefaults(config) }
func normalizePMGDefaults(config *AlertConfig)      { alertconfig.NormalizePMGDefaults(config) }
func normalizeSnapshotDefaults(config *AlertConfig) { alertconfig.NormalizeSnapshotDefaults(config) }
func normalizeBackupDefaults(config *AlertConfig)   { alertconfig.NormalizeBackupDefaults(config) }
func normalizeNodeDefaults(config *AlertConfig)     { alertconfig.NormalizeNodeDefaults(config) }
func normalizeAgentDefaults(config *AlertConfig)    { alertconfig.NormalizeAgentDefaults(config) }
func normalizeKubernetesDefaults(config *AlertConfig) {
	alertconfig.NormalizeKubernetesDefaults(config)
}
func normalizeTrueNASDefaults(config *AlertConfig) { alertconfig.NormalizeTrueNASDefaults(config) }
func normalizeVMwareDefaults(config *AlertConfig)  { alertconfig.NormalizeVMwareDefaults(config) }
func normalizeGeneralSettings(config *AlertConfig) { alertconfig.NormalizeGeneralSettings(config) }
func normalizeTimeThresholds(config *AlertConfig)  { alertconfig.NormalizeTimeThresholds(config) }

func validateHysteresisThresholds(config *AlertConfig) {
	alertconfig.ValidateHysteresisThresholds(config)
}

func validateQuietHoursTimezone(config *AlertConfig) { alertconfig.ValidateQuietHoursTimezone(config) }
