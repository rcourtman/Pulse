package unifiedresources

import "strings"

func IncidentImpactSummaryForResource(resource *Resource) string {
	if resource == nil {
		return ""
	}
	if resource.PBS != nil {
		if summary := strings.TrimSpace(resource.PBS.ProtectedWorkloadSummary); summary != "" {
			return summary
		}
		if summary := summarizePBSProtectedWorkloads(resource.PBS.ProtectedWorkloadCount, resource.PBS.ProtectedWorkloadNames); summary != "" {
			return summary
		}
		if summary := strings.TrimSpace(resource.PBS.AffectedDatastoreSummary); summary != "" {
			return summary
		}
		if summary := summarizePBSAffectedDatastores(resource.PBS.AffectedDatastores); summary != "" {
			return summary
		}
	}
	if resource.Storage != nil {
		if summary := strings.TrimSpace(resource.Storage.ConsumerImpactSummary); summary != "" {
			return summary
		}
		if summary := StorageConsumerImpactSummary(resource.Storage); summary != "" {
			return summary
		}
	}
	return ""
}
