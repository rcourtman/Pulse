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
