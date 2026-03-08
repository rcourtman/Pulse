package unifiedresources

import "strings"

func (rr *ResourceRegistry) refreshIncidentRollupsLocked() {
	for _, resource := range rr.resources {
		refreshResourceIncidentRollup(resource)
	}
}

func refreshResourceIncidentRollup(resource *Resource) {
	if resource == nil {
		return
	}
	resource.IncidentCount = len(resource.Incidents)
	resource.IncidentCode = ""
	resource.IncidentSeverity = ""
	resource.IncidentSummary = ""
	if len(resource.Incidents) == 0 {
		return
	}

	best := resource.Incidents[0]
	bestRank := incidentSeverityRank(best.Severity)
	bestPreferred := incidentSummaryPreference(resource, best)
	for _, incident := range resource.Incidents[1:] {
		rank := incidentSeverityRank(incident.Severity)
		preferred := incidentSummaryPreference(resource, incident)
		switch {
		case rank > bestRank:
			best = incident
			bestRank = rank
			bestPreferred = preferred
		case rank == bestRank && preferred > bestPreferred:
			best = incident
			bestPreferred = preferred
		case rank == bestRank && preferred == bestPreferred:
			if summary := strings.TrimSpace(incident.Summary); summary != "" {
				bestSummary := strings.TrimSpace(best.Summary)
				if bestSummary == "" || summary < bestSummary {
					best = incident
					bestPreferred = preferred
				}
			}
		}
	}

	resource.IncidentCode = strings.TrimSpace(best.Code)
	resource.IncidentSeverity = best.Severity
	resource.IncidentSummary = strings.TrimSpace(best.Summary)
	resource.IncidentCategory = IncidentCategoryForResource(resource, best)
	resource.IncidentUrgency, resource.IncidentAction = IncidentActionForResource(resource, best, resource.IncidentCategory)
}

func incidentSummaryPreference(resource *Resource, incident ResourceIncident) int {
	summary := strings.TrimSpace(incident.Summary)
	if summary == "" || resource == nil {
		return 0
	}
	for i, preferred := range resourcePreferredIncidentSummaries(resource) {
		if summary == preferred {
			return len(resourcePreferredIncidentSummaries(resource)) - i
		}
	}
	return 0
}

func resourcePreferredIncidentSummaries(resource *Resource) []string {
	candidates := make([]string, 0, 8)
	appendCandidate := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range candidates {
			if existing == value {
				return
			}
		}
		candidates = append(candidates, value)
	}

	if resource.Storage != nil {
		appendCandidate(resource.Storage.ProtectionSummary)
		appendCandidate(resource.Storage.RebuildSummary)
		appendCandidate(resource.Storage.RiskSummary)
	}
	if resource.Agent != nil {
		appendCandidate(resource.Agent.ProtectionSummary)
		appendCandidate(resource.Agent.RebuildSummary)
		appendCandidate(resource.Agent.StorageRiskSummary)
	}
	if resource.PBS != nil {
		appendCandidate(StorageRiskSummary(resource.PBS.StorageRisk))
		appendCandidate(resource.PBS.AffectedDatastoreSummary)
		appendCandidate(resource.PBS.PostureSummary)
	}
	if resource.TrueNAS != nil {
		appendCandidate(resource.TrueNAS.ProtectionSummary)
		appendCandidate(resource.TrueNAS.RebuildSummary)
		appendCandidate(resource.TrueNAS.StorageRiskSummary)
	}

	return candidates
}
