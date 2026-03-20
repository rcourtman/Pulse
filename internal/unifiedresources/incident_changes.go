package unifiedresources

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	MetadataAlertIdentifier = "alert_identifier"
	MetadataAlertType       = "alert_type"
	MetadataAlertLevel      = "alert_level"
	MetadataAlertMessage    = "alert_message"
	MetadataAlertValue      = "alert_value"
	MetadataAlertThreshold  = "alert_threshold"
	MetadataCommand         = "command"
	MetadataSuccess         = "success"
	MetadataOutputExcerpt   = "output_excerpt"
	MetadataRunbookID       = "runbook_id"
	MetadataOutcome         = "outcome"
	MetadataAutomatic       = "automatic"
	MetadataMessage         = "message"
)

const resourceChangeOutputExcerptLimit = 500

// AlertTimelineChange captures the alert-lifecycle details that should be
// durable in the canonical resource history rather than only incident memory.
type AlertTimelineChange struct {
	AlertIdentifier string
	AlertType       string
	AlertLevel      string
	AlertMessage    string
	AlertValue      float64
	AlertThreshold  float64
}

// BuildAlertTimelineChange constructs a canonical resource change for alert
// lifecycle events tied to a specific resource.
func BuildAlertTimelineChange(resourceID string, kind ChangeKind, occurredAt time.Time, actor string, alert AlertTimelineChange) *ResourceChange {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return nil
	}

	observedAt := occurredAt.UTC()
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}

	change := &ResourceChange{
		ID:         uuid.NewString(),
		ObservedAt: observedAt,
		ResourceID: resourceID,
		Kind:       kind,
		SourceType: SourceHeuristic,
		Confidence: ConfidenceHigh,
		Actor:      strings.TrimSpace(actor),
		Reason:     alertChangeReason(kind, alert),
		Metadata: map[string]any{
			MetadataAlertIdentifier: strings.TrimSpace(alert.AlertIdentifier),
			MetadataAlertType:       strings.TrimSpace(alert.AlertType),
			MetadataAlertLevel:      strings.TrimSpace(alert.AlertLevel),
			MetadataAlertMessage:    strings.TrimSpace(alert.AlertMessage),
			MetadataAlertValue:      alert.AlertValue,
			MetadataAlertThreshold:  alert.AlertThreshold,
		},
	}
	if !occurredAt.IsZero() {
		change.OccurredAt = cloneTimePtr(&occurredAt)
	}
	return change
}

// BuildCommandExecutionChange constructs a canonical resource change for a
// command that was executed against a resource in response to an incident.
func BuildCommandExecutionChange(resourceID, alertIdentifier, actor, command string, success bool, output string, details map[string]any) *ResourceChange {
	resourceID = strings.TrimSpace(resourceID)
	command = strings.TrimSpace(command)
	if resourceID == "" || command == "" {
		return nil
	}

	observedAt := time.Now().UTC()
	change := &ResourceChange{
		ID:         uuid.NewString(),
		ObservedAt: observedAt,
		OccurredAt: cloneTimePtr(&observedAt),
		ResourceID: resourceID,
		Kind:       ChangeCommandExecuted,
		SourceType: SourceAgentAction,
		Confidence: ConfidenceHigh,
		Actor:      strings.TrimSpace(actor),
		Reason:     commandExecutionReason(command, success),
		Metadata:   cloneChangeMetadata(details),
	}
	change.Metadata[MetadataAlertIdentifier] = strings.TrimSpace(alertIdentifier)
	change.Metadata[MetadataCommand] = command
	change.Metadata[MetadataSuccess] = success
	if excerpt := truncateResourceChangeOutput(output, resourceChangeOutputExcerptLimit); excerpt != "" {
		change.Metadata[MetadataOutputExcerpt] = excerpt
	}
	return change
}

// BuildRunbookExecutionChange constructs a canonical resource change for a
// runbook that was executed against a resource in response to an incident.
func BuildRunbookExecutionChange(resourceID, alertIdentifier, actor, runbookID, title, outcome string, automatic bool, message string, details map[string]any) *ResourceChange {
	resourceID = strings.TrimSpace(resourceID)
	runbookID = strings.TrimSpace(runbookID)
	if resourceID == "" || runbookID == "" {
		return nil
	}

	observedAt := time.Now().UTC()
	change := &ResourceChange{
		ID:         uuid.NewString(),
		ObservedAt: observedAt,
		OccurredAt: cloneTimePtr(&observedAt),
		ResourceID: resourceID,
		Kind:       ChangeRunbookExecuted,
		SourceType: SourceUserAction,
		Confidence: ConfidenceHigh,
		Actor:      strings.TrimSpace(actor),
		Reason:     runbookExecutionReason(title, outcome),
		Metadata:   cloneChangeMetadata(details),
	}
	if automatic {
		change.SourceType = SourceAgentAction
	}
	change.Metadata[MetadataAlertIdentifier] = strings.TrimSpace(alertIdentifier)
	change.Metadata[MetadataRunbookID] = runbookID
	change.Metadata[MetadataOutcome] = strings.TrimSpace(outcome)
	change.Metadata[MetadataAutomatic] = automatic
	if trimmedTitle := strings.TrimSpace(title); trimmedTitle != "" {
		change.Metadata["title"] = trimmedTitle
	}
	if trimmedMessage := strings.TrimSpace(message); trimmedMessage != "" {
		change.Metadata[MetadataMessage] = trimmedMessage
	}
	return change
}

func alertChangeReason(kind ChangeKind, alert AlertTimelineChange) string {
	message := strings.TrimSpace(alert.AlertMessage)
	switch kind {
	case ChangeAlertFired:
		if message != "" {
			return message
		}
		return "Alert fired"
	case ChangeAlertAcknowledged:
		if message != "" {
			return fmt.Sprintf("Alert acknowledged: %s", message)
		}
		return "Alert acknowledged"
	case ChangeAlertUnacknowledged:
		if message != "" {
			return fmt.Sprintf("Alert unacknowledged: %s", message)
		}
		return "Alert unacknowledged"
	case ChangeAlertResolved:
		if message != "" {
			return fmt.Sprintf("Alert resolved: %s", message)
		}
		return "Alert resolved"
	default:
		if message != "" {
			return message
		}
		return "Alert event recorded"
	}
}

func commandExecutionReason(command string, success bool) string {
	status := "failed"
	if success {
		status = "succeeded"
	}
	return fmt.Sprintf("Command %s: %s", status, command)
}

func runbookExecutionReason(title, outcome string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "runbook"
	}
	outcome = strings.TrimSpace(outcome)
	if outcome == "" {
		return fmt.Sprintf("Runbook %s executed", title)
	}
	return fmt.Sprintf("Runbook %s (%s)", title, outcome)
}

func cloneChangeMetadata(details map[string]any) map[string]any {
	if len(details) == 0 {
		return make(map[string]any)
	}
	cloned := make(map[string]any, len(details))
	for key, value := range details {
		cloned[key] = value
	}
	return cloned
}

func truncateResourceChangeOutput(output string, limit int) string {
	output = strings.TrimSpace(output)
	if output == "" || limit <= 0 {
		return ""
	}
	runes := []rune(output)
	if len(runes) <= limit {
		return output
	}
	return string(runes[:limit]) + "..."
}
