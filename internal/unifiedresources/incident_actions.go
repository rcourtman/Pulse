package unifiedresources

import "github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"

const (
	IncidentUrgencyNow     = "now"
	IncidentUrgencyToday   = "today"
	IncidentUrgencyPlan    = "plan"
	IncidentUrgencyMonitor = "monitor"
)

func IncidentActionForResource(resource *Resource, incident ResourceIncident, category string) (string, string) {
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
		if incident.Severity == storagehealth.RiskCritical {
			return IncidentUrgencyNow, "Restore storage availability immediately"
		}
		return IncidentUrgencyToday, "Investigate degraded storage availability"
	default:
		if incident.Severity == storagehealth.RiskCritical {
			return IncidentUrgencyToday, "Investigate storage health immediately"
		}
		return IncidentUrgencyPlan, "Review storage health and plan corrective action"
	}
}
