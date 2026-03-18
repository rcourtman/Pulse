package unifiedresources

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

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

func incidentsFromAssessment(provider, source, nativeIDPrefix string, assessment storagehealth.Assessment, startedAt time.Time) []ResourceIncident {
	if len(assessment.Reasons) == 0 {
		return nil
	}

	provider = strings.TrimSpace(provider)
	source = strings.TrimSpace(source)
	nativeIDPrefix = strings.TrimSpace(nativeIDPrefix)

	incidents := make([]ResourceIncident, 0, len(assessment.Reasons))
	for _, reason := range assessment.Reasons {
		code := strings.TrimSpace(reason.Code)
		summary := strings.TrimSpace(reason.Summary)
		if code == "" || summary == "" {
			continue
		}
		nativeID := code
		if nativeIDPrefix != "" {
			nativeID = nativeIDPrefix + ":" + code
		}
		incidents = append(incidents, ResourceIncident{
			Provider:  provider,
			NativeID:  nativeID,
			Code:      code,
			Severity:  reason.Severity,
			Source:    source,
			Summary:   summary,
			StartedAt: startedAt,
		})
	}
	if len(incidents) == 0 {
		return nil
	}
	return incidents
}
