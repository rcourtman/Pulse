package memory

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

// ChangeTypeLabel returns the canonical human-readable label for a detected
// memory change type.
func ChangeTypeLabel(changeType ChangeType) string {
	switch changeType {
	case ChangeCreated:
		return "Created"
	case ChangeDeleted:
		return "Deleted"
	case ChangeConfig:
		return "Config update"
	case ChangeStatus:
		return "Status change"
	case ChangeMigrated:
		return "Migration"
	case ChangeRestarted:
		return "Restart"
	case ChangeBackedUp:
		return "Backup"
	default:
		raw := strings.TrimSpace(strings.ReplaceAll(string(changeType), "_", " "))
		if raw == "" {
			return "Change"
		}
		return strings.ToUpper(raw[:1]) + raw[1:]
	}
}

// FormatRecentChangesContext returns the canonical markdown section for recent
// memory changes used in AI prompts and summaries.
func FormatRecentChangesContext(changes []Change, includeResourcePrefix bool, headingLevel string) string {
	if len(changes) == 0 {
		return ""
	}

	headingLevel = strings.TrimSpace(headingLevel)
	if headingLevel == "" {
		headingLevel = "##"
	}

	heading := fmt.Sprintf("\n%s Recent Changes", headingLevel)
	if includeResourcePrefix {
		heading = fmt.Sprintf("\n%s Recent Changes Across Infrastructure", headingLevel)
	}

	lines := []string{heading, "What changed recently:"}
	for _, change := range changes {
		entry := fmt.Sprintf("**%s** %s", ChangeTypeLabel(change.ChangeType), change.Description)
		if includeResourcePrefix {
			scope := strings.TrimSpace(change.ResourceID)
			if scope == "" {
				scope = strings.TrimSpace(change.ResourceName)
			}
			if scope == "" {
				scope = "resource"
			}
			if resourceType := strings.TrimSpace(change.ResourceType); resourceType != "" && !strings.Contains(scope, resourceType) {
				scope = fmt.Sprintf("%s (%s)", scope, resourceType)
			}
			entry = fmt.Sprintf("%s: %s", scope, entry)
		}
		ago := time.Since(change.DetectedAt).Truncate(time.Minute)
		lines = append(lines, fmt.Sprintf("- %s (%s)", entry, utils.FormatDurationAgo(ago)))
	}
	return strings.Join(lines, "\n")
}

// ChangeFromUnifiedResourceChange converts a canonical unified-resource change
// into the patrol-local memory representation.
func ChangeFromUnifiedResourceChange(change unifiedresources.ResourceChange) Change {
	detectedAt := change.ObservedAt
	if detectedAt.IsZero() && change.OccurredAt != nil {
		detectedAt = *change.OccurredAt
	}
	if detectedAt.IsZero() {
		detectedAt = time.Now()
	}

	return Change{
		ID:           strings.TrimSpace(change.ID),
		ResourceID:   strings.TrimSpace(change.ResourceID),
		ResourceName: strings.TrimSpace(change.ResourceID),
		ChangeType:   ChangeType(unifiedresources.ChangeKindLabel(change.Kind)),
		DetectedAt:   detectedAt,
		Description:  unifiedresources.FormatResourceChangeSummary(change),
	}
}

// ResourceChangeFromMemoryChange converts a patrol-local memory change back
// into the canonical unified-resource change shape.
func ResourceChangeFromMemoryChange(change Change) unifiedresources.ResourceChange {
	var related []string
	if resourceName := strings.TrimSpace(change.ResourceName); resourceName != "" {
		related = []string{resourceName}
	}

	return unifiedresources.ResourceChange{
		ID:               strings.TrimSpace(change.ID),
		ObservedAt:       change.DetectedAt,
		ResourceID:       strings.TrimSpace(change.ResourceID),
		Kind:             memoryChangeTypeToUnifiedChangeKind(change.ChangeType),
		SourceType:       unifiedresources.SourceHeuristic,
		Confidence:       unifiedresources.ConfidenceMedium,
		RelatedResources: related,
		Reason:           change.Description,
		Metadata: map[string]any{
			"memoryChangeType": string(change.ChangeType),
		},
	}
}

func memoryChangeTypeToUnifiedChangeKind(changeType ChangeType) unifiedresources.ChangeKind {
	switch changeType {
	case ChangeCreated, ChangeDeleted, ChangeStatus:
		return unifiedresources.ChangeStateTransition
	case ChangeConfig:
		return unifiedresources.ChangeConfigUpdate
	case ChangeMigrated:
		return unifiedresources.ChangeRelationship
	case ChangeRestarted:
		return unifiedresources.ChangeRestart
	case ChangeBackedUp:
		return unifiedresources.ChangeConfigUpdate
	default:
		return unifiedresources.ChangeAnomaly
	}
}
