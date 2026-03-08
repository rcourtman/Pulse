package storagehealth

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func AssessHostRAIDArray(array models.HostRAIDArray) Assessment {
	assessment := Assessment{Level: RiskHealthy}
	addReason := func(code string, severity RiskLevel, summary string) {
		assessment.Reasons = append(assessment.Reasons, Reason{
			Code:     code,
			Severity: severity,
			Summary:  summary,
		})
		if severityRank(severity) > severityRank(assessment.Level) {
			assessment.Level = severity
		}
	}

	stateLower := strings.ToLower(strings.TrimSpace(array.State))
	isChecking := strings.Contains(stateLower, "check")
	isRebuilding := !isChecking && (strings.Contains(stateLower, "recover") ||
		strings.Contains(stateLower, "resync") ||
		(array.RebuildPercent > 0 && !strings.Contains(stateLower, "clean")))

	if strings.Contains(stateLower, "degraded") || array.FailedDevices > 0 || (array.TotalDevices > 0 && array.ActiveDevices > 0 && array.ActiveDevices < array.TotalDevices) {
		summary := fmt.Sprintf("RAID array %s is degraded", array.Device)
		if array.FailedDevices > 0 {
			summary = fmt.Sprintf("RAID array %s has %d failed device(s)", array.Device, array.FailedDevices)
		}
		addReason("raid_degraded", RiskCritical, summary)
	}

	switch stateLower {
	case "faulted", "offline", "removed", "unavail", "failed":
		addReason("raid_unavailable", RiskCritical, fmt.Sprintf("RAID array %s is %s", array.Device, strings.ToUpper(stateLower)))
	}

	if isRebuilding {
		summary := fmt.Sprintf("RAID array %s is rebuilding", array.Device)
		if array.RebuildPercent > 0 {
			summary = fmt.Sprintf("RAID array %s is rebuilding (%.0f%%)", array.Device, array.RebuildPercent)
		}
		addReason("raid_rebuilding", RiskWarning, summary)
	}

	sortReasons(&assessment)
	return assessment
}

func AssessZFSPool(pool models.ZFSPool) Assessment {
	assessment := Assessment{Level: RiskHealthy}
	addReason := func(code string, severity RiskLevel, summary string) {
		assessment.Reasons = append(assessment.Reasons, Reason{
			Code:     code,
			Severity: severity,
			Summary:  summary,
		})
		if severityRank(severity) > severityRank(assessment.Level) {
			assessment.Level = severity
		}
	}

	stateUpper := strings.ToUpper(strings.TrimSpace(pool.State))
	switch stateUpper {
	case "DEGRADED":
		addReason("zfs_pool_state", RiskWarning, fmt.Sprintf("ZFS pool %s is DEGRADED", pool.Name))
	case "FAULTED", "OFFLINE", "REMOVED", "UNAVAIL", "SUSPENDED":
		addReason("zfs_pool_state", RiskCritical, fmt.Sprintf("ZFS pool %s is %s", pool.Name, stateUpper))
	}

	if pool.ReadErrors > 0 || pool.WriteErrors > 0 || pool.ChecksumErrors > 0 {
		addReason(
			"zfs_pool_errors",
			RiskWarning,
			fmt.Sprintf("ZFS pool %s reports read=%d write=%d checksum=%d errors", pool.Name, pool.ReadErrors, pool.WriteErrors, pool.ChecksumErrors),
		)
	}

	for _, device := range pool.Devices {
		deviceState := strings.ToUpper(strings.TrimSpace(device.State))
		switch deviceState {
		case "", "ONLINE":
		case "DEGRADED":
			addReason("zfs_device_state", RiskWarning, fmt.Sprintf("ZFS device %s is DEGRADED", device.Name))
		default:
			addReason("zfs_device_state", RiskCritical, fmt.Sprintf("ZFS device %s is %s", device.Name, deviceState))
		}
	}

	sortReasons(&assessment)
	return assessment
}

func AssessUnraidStorage(storage models.HostUnraidStorage) Assessment {
	assessment := Assessment{Level: RiskHealthy}
	addReason := func(code string, severity RiskLevel, summary string) {
		assessment.Reasons = append(assessment.Reasons, Reason{
			Code:     code,
			Severity: severity,
			Summary:  summary,
		})
		if severityRank(severity) > severityRank(assessment.Level) {
			assessment.Level = severity
		}
	}

	if storage.NumInvalid > 0 {
		addReason("unraid_invalid_disks", RiskCritical, fmt.Sprintf("Unraid array reports %d invalid disk(s)", storage.NumInvalid))
	}
	if storage.NumDisabled > 0 {
		addReason("unraid_disabled_disks", RiskCritical, fmt.Sprintf("Unraid array reports %d disabled disk(s)", storage.NumDisabled))
	}
	if storage.NumMissing > 0 {
		addReason("unraid_missing_disks", RiskCritical, fmt.Sprintf("Unraid array reports %d missing disk(s)", storage.NumMissing))
	}

	parityConfigured := false
	parityHealthy := false
	for _, disk := range storage.Disks {
		role := strings.ToLower(strings.TrimSpace(disk.Role))
		status := strings.ToLower(strings.TrimSpace(disk.Status))
		if role != "parity" {
			continue
		}
		parityConfigured = true
		if status == "online" {
			parityHealthy = true
			continue
		}
		if status != "" {
			addReason("unraid_parity_unavailable", RiskCritical, fmt.Sprintf("Unraid parity disk %s is %s", disk.Name, strings.ToUpper(status)))
		}
	}

	if storage.ArrayStarted && !parityConfigured {
		addReason("unraid_no_parity", RiskWarning, "Unraid array is running without parity protection")
	}
	if storage.ArrayStarted && parityConfigured && !parityHealthy {
		addReason("unraid_parity_unavailable", RiskCritical, "Unraid parity protection is unavailable")
	}

	if action := strings.TrimSpace(storage.SyncAction); action != "" {
		summary := fmt.Sprintf("Unraid array is running %s", action)
		if storage.SyncProgress > 0 {
			summary = fmt.Sprintf("Unraid array is running %s (%.0f%%)", action, storage.SyncProgress)
		}
		addReason("unraid_sync_active", RiskWarning, summary)
	}

	sortReasons(&assessment)
	return assessment
}

func AssessPBSDatastore(datastore models.PBSDatastore) Assessment {
	assessment := Assessment{Level: RiskHealthy}
	addReason := func(code string, severity RiskLevel, summary string) {
		assessment.Reasons = append(assessment.Reasons, Reason{
			Code:     code,
			Severity: severity,
			Summary:  summary,
		})
		if severityRank(severity) > severityRank(assessment.Level) {
			assessment.Level = severity
		}
	}

	name := strings.TrimSpace(datastore.Name)
	if name == "" {
		name = "datastore"
	}

	status := strings.ToUpper(strings.TrimSpace(datastore.Status))
	switch status {
	case "OFFLINE", "UNAVAILABLE", "ERROR", "FAILED":
		addReason("pbs_datastore_state", RiskCritical, fmt.Sprintf("PBS datastore %s is %s", name, status))
	case "DEGRADED", "WARN", "WARNING", "READ_ONLY":
		addReason("pbs_datastore_state", RiskWarning, fmt.Sprintf("PBS datastore %s is %s", name, status))
	}

	if errText := strings.TrimSpace(datastore.Error); errText != "" {
		severity := RiskWarning
		if assessment.Level == RiskCritical {
			severity = RiskCritical
		}
		addReason("pbs_datastore_error", severity, fmt.Sprintf("PBS datastore %s reports %s", name, errText))
	}

	usage := datastore.Usage
	if usage <= 0 && datastore.Total > 0 {
		usage = (float64(datastore.Used) / float64(datastore.Total)) * 100
	}
	switch {
	case usage >= 95:
		addReason("capacity_runway_low", RiskCritical, fmt.Sprintf("PBS datastore %s is %.0f%% full", name, usage))
	case usage >= 90:
		addReason("capacity_runway_low", RiskWarning, fmt.Sprintf("PBS datastore %s is %.0f%% full", name, usage))
	}

	sortReasons(&assessment)
	return assessment
}

func SummarizeAssessments(assessments ...Assessment) Assessment {
	summary := Assessment{Level: RiskHealthy}
	for _, assessment := range assessments {
		if severityRank(assessment.Level) > severityRank(summary.Level) {
			summary.Level = assessment.Level
		}
		summary.Reasons = append(summary.Reasons, assessment.Reasons...)
	}
	sortReasons(&summary)
	return summary
}

func sortReasons(assessment *Assessment) {
	sort.SliceStable(assessment.Reasons, func(i, j int) bool {
		left := assessment.Reasons[i]
		right := assessment.Reasons[j]
		if severityRank(left.Severity) != severityRank(right.Severity) {
			return severityRank(left.Severity) > severityRank(right.Severity)
		}
		return left.Code < right.Code
	})
}
