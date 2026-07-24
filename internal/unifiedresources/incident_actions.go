package unifiedresources

import "github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"

const (
	IncidentUrgencyNow     = "now"
	IncidentUrgencyToday   = "today"
	IncidentUrgencyPlan    = "plan"
	IncidentUrgencyMonitor = "monitor"
)

func IncidentActionForResource(resource *Resource, incident ResourceIncident, category string) (string, string) {
	baseType := resourceBaseType(resource)
	switch incident.Code {
	case "zfs_pool_state":
		if incident.Severity == storagehealth.RiskCritical {
			return IncidentUrgencyNow, "Inspect native pool and vdev status, preserve remaining redundancy, and restore the pool; replace hardware only when native evidence supports it"
		}
		return IncidentUrgencyToday, "Inspect native pool and vdev status and plan maintenance to restore ONLINE state"
	case "zfs_device_state", "zfs_device_missing":
		if incident.Severity == storagehealth.RiskCritical {
			return IncidentUrgencyNow, "Confirm the affected member in native topology, reconnect or replace it when supported by device evidence, and verify resilver completion"
		}
		return IncidentUrgencyToday, "Inspect the affected member and path, preserve redundancy, and plan evidence-backed maintenance"
	case "zfs_pool_errors", "zfs_device_errors", "zfs_scan_errors", "zfs_scan_failed":
		if incident.Severity == storagehealth.RiskCritical {
			return IncidentUrgencyNow, "Review the native scan and device evidence, inspect SMART and cabling, and replace hardware only when the evidence identifies it"
		}
		return IncidentUrgencyToday, "Inspect native error counters, SMART, and cabling, then run or review a scrub before deciding on replacement"
	}
	switch category {
	case IncidentCategoryProtection:
		if incident.Severity == storagehealth.RiskCritical {
			return IncidentUrgencyNow, "Restore protection immediately and replace or recover the affected member"
		}
		return IncidentUrgencyToday, "Investigate degraded protection and schedule maintenance to restore redundancy"
	case IncidentCategoryRebuild:
		if incident.Severity == storagehealth.RiskCritical {
			return IncidentUrgencyToday, "Investigate rebuild risk and avoid further changes until protection stabilizes"
		}
		return IncidentUrgencyMonitor, "Monitor rebuild progress and avoid risky storage changes until it completes"
	case IncidentCategoryCapacity:
		if incident.Severity == storagehealth.RiskCritical {
			return IncidentUrgencyNow, "Free space or expand capacity immediately"
		}
		return IncidentUrgencyToday, "Plan cleanup or capacity expansion soon"
	case IncidentCategoryRecoverability:
		if incident.Severity == storagehealth.RiskCritical {
			return IncidentUrgencyNow, "Restore backup target health immediately to protect recoverability"
		}
		return IncidentUrgencyToday, "Investigate backup target health and preserve backup coverage"
	case IncidentCategoryDiskHealth:
		if incident.Severity == storagehealth.RiskCritical {
			return IncidentUrgencyNow, "Replace the affected disk and confirm storage protection"
		}
		return IncidentUrgencyToday, "Investigate disk health and schedule replacement if degradation continues"
	case IncidentCategoryAvailability:
		if baseType == ResourceTypeAgent || baseType == ResourceTypeVM {
			if incident.Severity == storagehealth.RiskCritical {
				return IncidentUrgencyNow, "Restore resource availability immediately"
			}
			return IncidentUrgencyToday, "Investigate degraded resource availability"
		}
		if incident.Severity == storagehealth.RiskCritical {
			return IncidentUrgencyNow, "Restore storage availability immediately"
		}
		return IncidentUrgencyToday, "Investigate degraded storage availability"
	case IncidentCategoryWorkloadHealth:
		if incident.Severity == storagehealth.RiskCritical {
			return IncidentUrgencyNow, "Review the app and container logs, restore the failed workload, and confirm it remains running"
		}
		return IncidentUrgencyToday, "Confirm whether the app should be running, then start it or suppress the alert for intentional downtime"
	default:
		if baseType == ResourceTypeAgent || baseType == ResourceTypeVM {
			if incident.Severity == storagehealth.RiskCritical {
				return IncidentUrgencyToday, "Investigate resource health immediately"
			}
			return IncidentUrgencyPlan, "Review resource health and plan corrective action"
		}
		if incident.Severity == storagehealth.RiskCritical {
			return IncidentUrgencyToday, "Investigate storage health immediately"
		}
		return IncidentUrgencyPlan, "Review storage health and plan corrective action"
	}
}
