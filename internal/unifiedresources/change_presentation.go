package unifiedresources

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

// ChangePresentation captures the canonical human-readable components for a
// resource change. Backend AI surfaces can render these fragments in different
// styles without duplicating the semantic mapping.
type ChangePresentation struct {
	KindLabel        string
	From             string
	To               string
	SourceType       string
	SourceAdapter    string
	Actor            string
	Reason           string
	RelatedResources []string
}

// ChangeKindLabel returns the canonical human-readable label for a change kind.
func ChangeKindLabel(kind ChangeKind) string {
	switch kind {
	case ChangeStateTransition:
		return "State transition"
	case ChangeActivity:
		return "Activity"
	case ChangeRestart:
		return "Restart"
	case ChangeConfigUpdate:
		return "Config update"
	case ChangeAnomaly:
		return "Metric anomaly"
	case ChangeRelationship:
		return "Relationship change"
	case ChangeCapability:
		return "Capability change"
	case ChangeAlertFired:
		return "Alert fired"
	case ChangeAlertAcknowledged:
		return "Alert acknowledged"
	case ChangeAlertUnacknowledged:
		return "Alert unacknowledged"
	case ChangeAlertResolved:
		return "Alert resolved"
	case ChangeCommandExecuted:
		return "Command executed"
	case ChangeRunbookExecuted:
		return "Runbook executed"
	default:
		raw := strings.TrimSpace(strings.ReplaceAll(string(kind), "_", " "))
		if raw == "" {
			return "Change"
		}
		return strings.ToUpper(raw[:1]) + raw[1:]
	}
}

// DescribeChange returns the canonical presentation fragments for a change.
func DescribeChange(change ResourceChange) ChangePresentation {
	presentation := ChangePresentation{
		KindLabel: ChangeKindLabel(change.Kind),
	}

	presentation.From = strings.TrimSpace(change.From)
	presentation.To = strings.TrimSpace(change.To)

	presentation.SourceType = strings.TrimSpace(string(change.SourceType))
	presentation.SourceAdapter = strings.TrimSpace(string(change.SourceAdapter))

	presentation.Actor = strings.TrimSpace(change.Actor)
	presentation.Reason = strings.TrimSpace(change.Reason)

	if len(change.RelatedResources) > 0 {
		presentation.RelatedResources = make([]string, 0, len(change.RelatedResources))
		for _, resourceID := range change.RelatedResources {
			if trimmed := strings.TrimSpace(resourceID); trimmed != "" {
				presentation.RelatedResources = append(presentation.RelatedResources, trimmed)
			}
		}
	}

	return presentation
}

func resourceStateSummary(resource Resource) string {
	status := resourceStatusString(resource.Status)
	if status == "" {
		status = "unknown"
	}
	return status
}

func resourceRestartSummary(resource Resource) string {
	parts := []string{resourceStateSummary(resource)}
	if resource.Docker != nil {
		parts = append(parts, fmt.Sprintf("docker.restartCount=%d", resource.Docker.RestartCount))
		if resource.Docker.UptimeSeconds > 0 {
			parts = append(parts, fmt.Sprintf("docker.uptimeSeconds=%d", resource.Docker.UptimeSeconds))
		}
	}
	if resource.Kubernetes != nil {
		parts = append(parts, fmt.Sprintf("kubernetes.restarts=%d", resource.Kubernetes.Restarts))
		if resource.Kubernetes.UptimeSeconds > 0 {
			parts = append(parts, fmt.Sprintf("kubernetes.uptimeSeconds=%d", resource.Kubernetes.UptimeSeconds))
		}
	}
	return strings.Join(parts, "|")
}

func resourceIncidentSummary(resource Resource) string {
	return resourceIncidentSummaryFromSlice(resource.Incidents)
}

func resourceIncidentSummaryFromSlice(incidents []ResourceIncident) string {
	if len(incidents) == 0 {
		return "none"
	}

	labels := make([]string, 0, len(incidents))
	for _, incident := range incidents {
		labels = append(labels, resourceIncidentLabel(incident))
	}

	sort.Strings(labels)
	if len(labels) == 1 {
		return labels[0]
	}
	if len(labels) <= 3 {
		return strings.Join(labels, ", ")
	}
	return fmt.Sprintf("%d incidents", len(labels))
}

func resourceIncidentLabel(incident ResourceIncident) string {
	code := strings.TrimSpace(incident.Code)
	if code == "" {
		code = "incident"
	}
	if severity := strings.TrimSpace(string(incident.Severity)); severity != "" {
		code += fmt.Sprintf("[%s]", severity)
	}
	if summary := strings.TrimSpace(incident.Summary); summary != "" {
		code += fmt.Sprintf(":%s", summary)
	}
	return code
}

func resourceConfigSummary(resource Resource) string {
	return fmt.Sprintf("%s|%s|%s|%s", resource.Type, resource.Technology, resource.Name, resource.CustomURL)
}

// FormatResourceChangeSummary returns the canonical one-line human-readable
// summary used by AI prompt sections and recent-change feeds.
func FormatResourceChangeSummary(change ResourceChange) string {
	presentation := DescribeChange(change)
	summary := fmt.Sprintf("**%s**", presentation.KindLabel)

	if from, to := strings.TrimSpace(presentation.From), strings.TrimSpace(presentation.To); from != "" || to != "" {
		if from == "" {
			summary += fmt.Sprintf(" → %s", to)
		} else if to == "" {
			summary += fmt.Sprintf(" %s →", from)
		} else {
			summary += fmt.Sprintf(" %s → %s", from, to)
		}
	}

	provenance := make([]string, 0, 2)
	if sourceType := strings.TrimSpace(presentation.SourceType); sourceType != "" {
		provenance = append(provenance, sourceType)
	}
	if sourceAdapter := strings.TrimSpace(presentation.SourceAdapter); sourceAdapter != "" {
		provenance = append(provenance, sourceAdapter)
	}
	if len(provenance) > 0 {
		summary += fmt.Sprintf(" [%s]", strings.Join(provenance, "/"))
	}

	if actor := strings.TrimSpace(presentation.Actor); actor != "" {
		summary += fmt.Sprintf("; actor %s", actor)
	}
	if reason := strings.TrimSpace(presentation.Reason); reason != "" {
		summary += fmt.Sprintf("; %s", reason)
	}
	if len(presentation.RelatedResources) > 0 {
		summary += fmt.Sprintf("; related: %s", strings.Join(presentation.RelatedResources, ", "))
	}

	ago := "recently"
	if !change.ObservedAt.IsZero() {
		ago = utils.FormatDurationAgo(time.Since(change.ObservedAt).Truncate(time.Minute))
	}
	return summary + fmt.Sprintf(" (%s)", ago)
}

func resourceRelationshipSummary(relationships []ResourceRelationship) string {
	if len(relationships) == 0 {
		return ""
	}

	summaries := make([]string, 0, len(relationships))
	for _, relationship := range relationships {
		sourceID := CanonicalResourceID(relationship.SourceID)
		targetID := CanonicalResourceID(relationship.TargetID)
		if sourceID == "" && targetID == "" {
			continue
		}

		label := sourceID
		if label == "" {
			label = "?"
		}
		label += "->"
		if targetID == "" {
			label += "?"
		} else {
			label += targetID
		}
		if relationship.Type != "" {
			label += fmt.Sprintf("[%s]", RelationshipTypeLabel(relationship.Type))
		}
		summaries = append(summaries, label)
	}

	if len(summaries) == 0 {
		return ""
	}
	sort.Strings(summaries)
	if len(summaries) == 1 {
		return summaries[0]
	}
	if len(summaries) <= 3 {
		return strings.Join(summaries, ", ")
	}
	return fmt.Sprintf("%d relationships", len(summaries))
}

// FormatResourceRecentChangesContext returns the canonical markdown section
// used by AI prompt surfaces for recent unified-resource changes.
func FormatResourceRecentChangesContext(changes []ResourceChange, includeResourcePrefix bool, headingLevel string) string {
	if len(changes) == 0 {
		return ""
	}
	if strings.TrimSpace(headingLevel) == "" {
		headingLevel = "##"
	}

	heading := fmt.Sprintf("\n%s Recent Changes", headingLevel)
	if includeResourcePrefix {
		heading = fmt.Sprintf("\n%s Recent Changes Across Infrastructure", headingLevel)
	}

	lines := []string{heading, "What changed recently:"}
	for _, change := range changes {
		entry := FormatResourceChangeSummary(change)
		if includeResourcePrefix {
			if resourceID := strings.TrimSpace(change.ResourceID); resourceID != "" {
				entry = fmt.Sprintf("%s: %s", resourceID, entry)
			}
		}
		lines = append(lines, "- "+entry)
	}
	return strings.Join(lines, "\n")
}
