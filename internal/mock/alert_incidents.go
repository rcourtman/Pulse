package mock

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/memory"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

const mockIncidentStartTolerance = time.Second

// buildAlertIncidentFixtures gives the alert demo one coherent occurrence
// model. History rows and incident timelines are enriched together so mock
// mode exercises acknowledgement, investigation, remediation, and resolution
// rather than presenting timeline controls backed by no data.
func buildAlertIncidentFixtures(history []models.Alert, now time.Time) ([]models.Alert, []*memory.Incident) {
	enriched := cloneMockAlerts(history)
	incidents := make([]*memory.Incident, 0, len(enriched))
	for index := range enriched {
		alert := &enriched[index]
		if strings.TrimSpace(alert.ID) == "" {
			continue
		}

		seed := stableMockIncidentSeed(alert.ID)
		active := strings.HasPrefix(alert.ID, "active-")
		duration := time.Duration(5+seed%236) * time.Minute
		resolvedAt := alert.StartTime.Add(duration)
		if !now.IsZero() && resolvedAt.After(now) {
			resolvedAt = now
		}
		if resolvedAt.Before(alert.StartTime) {
			resolvedAt = alert.StartTime
		}

		acknowledged := seed%4 == 0
		ackAt := alert.StartTime.Add(time.Duration(1+seed%4) * time.Minute)
		eventCeiling := resolvedAt
		if active {
			eventCeiling = now
		}
		ackAt = clampMockIncidentEventTime(ackAt, alert.StartTime, eventCeiling)
		if acknowledged {
			alert.Acknowledged = true
			alert.AckTime = cloneMockTime(ackAt)
			alert.AckUser = "demo-operator"
		}
		if active {
			alert.LastSeen = cloneMockTime(now)
		} else {
			alert.LastSeen = cloneMockTime(resolvedAt)
		}

		incident := &memory.Incident{
			ID:              "mock-incident-" + alert.ID,
			AlertIdentifier: alert.ID,
			AlertType:       alert.Type,
			Level:           alert.Level,
			ResourceID:      alert.ResourceID,
			ResourceName:    alert.ResourceName,
			ResourceType:    mockAlertResourceType(*alert),
			Node:            alert.Node,
			Instance:        alert.Instance,
			Message:         alert.Message,
			Status:          memory.IncidentStatusOpen,
			OpenedAt:        alert.StartTime,
			Acknowledged:    acknowledged,
			AckUser:         alert.AckUser,
			AckTime:         cloneMockTimePtr(alert.AckTime),
			Events: []memory.IncidentEvent{{
				ID:        "mock-event-fired-" + alert.ID,
				Type:      memory.IncidentEventAlertFired,
				Timestamp: alert.StartTime,
				Summary:   mockAlertFiredSummary(*alert),
				Details: map[string]interface{}{
					"type":      alert.Type,
					"level":     alert.Level,
					"value":     alert.Value,
					"threshold": alert.Threshold,
				},
			}},
		}

		if acknowledged {
			incident.Events = append(incident.Events, memory.IncidentEvent{
				ID:        "mock-event-acknowledged-" + alert.ID,
				Type:      memory.IncidentEventAlertAcknowledged,
				Timestamp: ackAt,
				Summary:   "Alert acknowledged",
				Details:   map[string]interface{}{"user": alert.AckUser},
			})
		}

		analysisCandidate := alert.StartTime.Add(2 * time.Minute)
		if active || analysisCandidate.Before(resolvedAt) {
			analysisAt := clampMockIncidentEventTime(analysisCandidate, alert.StartTime, eventCeiling)
			incident.Events = append(incident.Events, memory.IncidentEvent{
				ID:        "mock-event-analysis-" + alert.ID,
				Type:      memory.IncidentEventAnalysis,
				Timestamp: analysisAt,
				Summary:   "Pulse Patrol analysis completed",
				Details: map[string]interface{}{
					"finding": "Telemetry confirms the alert condition and identifies the affected resource.",
					"source":  "mock-fixture",
				},
			})
		}

		if seed%3 == 0 {
			runbookCandidate := alert.StartTime.Add(3 * time.Minute)
			if active || runbookCandidate.Before(resolvedAt) {
				runbookAt := clampMockIncidentEventTime(runbookCandidate, alert.StartTime, eventCeiling)
				incident.Events = append(incident.Events, memory.IncidentEvent{
					ID:        "mock-event-runbook-" + alert.ID,
					Type:      memory.IncidentEventRunbook,
					Timestamp: runbookAt,
					Summary:   "Runbook Verify resource health (succeeded)",
					Details: map[string]interface{}{
						"runbook_id": "mock-verify-health",
						"outcome":    "succeeded",
						"automatic":  false,
					},
				})
			}
		}

		if !active {
			incident.Status = memory.IncidentStatusResolved
			incident.ClosedAt = cloneMockTime(resolvedAt)
			incident.Events = append(incident.Events, memory.IncidentEvent{
				ID:        "mock-event-resolved-" + alert.ID,
				Type:      memory.IncidentEventAlertResolved,
				Timestamp: resolvedAt,
				Summary:   "Alert resolved",
				Details:   map[string]interface{}{"resolved_at": resolvedAt.Format(time.RFC3339)},
			})
		}

		sort.SliceStable(incident.Events, func(i, j int) bool {
			return incident.Events[i].Timestamp.Before(incident.Events[j].Timestamp)
		})
		incidents = append(incidents, incident)
	}
	return enriched, incidents
}

func clampMockIncidentEventTime(candidate, openedAt, ceiling time.Time) time.Time {
	if !ceiling.IsZero() && candidate.After(ceiling) {
		candidate = ceiling
	}
	if candidate.Before(openedAt) {
		return openedAt
	}
	return candidate
}

func stableMockIncidentSeed(value string) uint64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(value))
	return hash.Sum64()
}

func mockAlertFiredSummary(alert models.Alert) string {
	if alert.Value != 0 || alert.Threshold != 0 {
		return fmt.Sprintf("Alert triggered: %s (%s %.1f >= %.1f)", alert.Type, alert.Level, alert.Value, alert.Threshold)
	}
	return fmt.Sprintf("Alert triggered: %s (%s)", alert.Type, alert.Level)
}

func mockAlertResourceType(alert models.Alert) string {
	if alert.Metadata == nil {
		return ""
	}
	value, _ := alert.Metadata["resourceType"].(string)
	return strings.TrimSpace(value)
}

func cloneMockIncident(incident *memory.Incident) *memory.Incident {
	if incident == nil {
		return nil
	}
	clone := *incident
	clone.ClosedAt = cloneMockTimePtr(incident.ClosedAt)
	clone.AckTime = cloneMockTimePtr(incident.AckTime)
	clone.Events = make([]memory.IncidentEvent, len(incident.Events))
	for index, event := range incident.Events {
		clone.Events[index] = event
		if event.Details != nil {
			clone.Events[index].Details = make(map[string]interface{}, len(event.Details))
			for key, value := range event.Details {
				clone.Events[index].Details[key] = value
			}
		}
	}
	return &clone
}

func cloneMockIncidents(incidents []*memory.Incident) []*memory.Incident {
	cloned := make([]*memory.Incident, 0, len(incidents))
	for _, incident := range incidents {
		if clone := cloneMockIncident(incident); clone != nil {
			cloned = append(cloned, clone)
		}
	}
	return cloned
}

func cloneMockAlerts(alerts []models.Alert) []models.Alert {
	cloned := make([]models.Alert, len(alerts))
	for index, alert := range alerts {
		cloned[index] = alert
		cloned[index].LastSeen = cloneMockTimePtr(alert.LastSeen)
		cloned[index].AckTime = cloneMockTimePtr(alert.AckTime)
		if alert.Metadata != nil {
			cloned[index].Metadata = make(map[string]interface{}, len(alert.Metadata))
			for key, value := range alert.Metadata {
				cloned[index].Metadata[key] = value
			}
		}
	}
	return cloned
}

func cloneMockTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	cloned := value
	return &cloned
}

func cloneMockTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	return cloneMockTime(*value)
}
