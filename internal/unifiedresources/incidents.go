package unifiedresources

import "github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"

func mergeResourceIncidents(existing, incoming []ResourceIncident) []ResourceIncident {
	if len(existing) == 0 {
		return cloneResourceIncidentSlice(incoming)
	}
	if len(incoming) == 0 {
		return cloneResourceIncidentSlice(existing)
	}

	merged := make([]ResourceIncident, 0, len(existing)+len(incoming))
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	appendUnique := func(incidents []ResourceIncident) {
		for _, incident := range incidents {
			key := incident.Provider + "|" + incident.NativeID + "|" + incident.Code + "|" + incident.Summary
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, incident)
		}
	}

	appendUnique(existing)
	appendUnique(incoming)
	return merged
}

func IncidentsStatus(base ResourceStatus, incidents []ResourceIncident) ResourceStatus {
	if len(incidents) == 0 {
		return base
	}
	level := highestIncidentSeverity(incidents)
	switch level {
	case storagehealth.RiskWarning, storagehealth.RiskCritical:
		if base == StatusOnline {
			return StatusWarning
		}
	}
	return base
}

func incidentsStatus(base ResourceStatus, incidents []ResourceIncident) ResourceStatus {
	return IncidentsStatus(base, incidents)
}

func highestIncidentSeverity(incidents []ResourceIncident) storagehealth.RiskLevel {
	level := storagehealth.RiskHealthy
	for _, incident := range incidents {
		if incidentSeverityRank(incident.Severity) > incidentSeverityRank(level) {
			level = incident.Severity
		}
	}
	return level
}

func incidentSeverityRank(level storagehealth.RiskLevel) int {
	switch level {
	case storagehealth.RiskCritical:
		return 4
	case storagehealth.RiskWarning:
		return 3
	case storagehealth.RiskMonitor:
		return 2
	case storagehealth.RiskHealthy:
		return 1
	default:
		return 0
	}
}
