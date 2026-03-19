package memory

import (
	"fmt"
	"strings"
	"time"
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
		lines = append(lines, fmt.Sprintf("- %s (%s ago)", entry, formatDuration(ago)))
	}
	return strings.Join(lines, "\n")
}
