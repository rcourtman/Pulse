package unifiedresources

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

// RelationshipPresentation captures the canonical human-readable fragments for
// a unified resource relationship.
type RelationshipPresentation struct {
	TypeLabel   string
	Direction   string
	Provenance  string
	StateLabel  string
	Confidence  string
	HasMetadata bool
}

// RelationshipTypeLabel returns the canonical label for a relationship type.
func RelationshipTypeLabel(t RelationshipType) string {
	switch t {
	case RelRunsOn:
		return "Runs on"
	case RelDependsOn:
		return "Depends on"
	case RelMountedTo:
		return "Mounted to"
	case RelExposedBy:
		return "Exposed by"
	case RelOwnedBy:
		return "Owned by"
	default:
		raw := strings.TrimSpace(strings.ReplaceAll(string(t), "_", " "))
		if raw == "" {
			return "Related to"
		}
		return strings.ToUpper(raw[:1]) + raw[1:]
	}
}

// DescribeRelationship returns the canonical presentation fragments for a
// unified resource relationship.
func DescribeRelationship(rel ResourceRelationship) RelationshipPresentation {
	presentation := RelationshipPresentation{
		TypeLabel: RelationshipTypeLabel(rel.Type),
		Direction: fmt.Sprintf("%s → %s", strings.TrimSpace(rel.SourceID), strings.TrimSpace(rel.TargetID)),
	}

	if rel.Discoverer != "" {
		presentation.Provenance = strings.TrimSpace(rel.Discoverer)
	}

	if rel.Active {
		presentation.StateLabel = "Active"
	} else {
		presentation.StateLabel = "Historical"
	}

	if rel.Confidence > 0 {
		presentation.Confidence = fmt.Sprintf("%.0f%%", rel.Confidence*100)
	}

	presentation.HasMetadata = len(rel.Metadata) > 0

	return presentation
}

// FormatResourceGraphContext returns the canonical AI prompt section for a
// resource's learned relationships.
func FormatResourceGraphContext(resource *Resource, limit int) string {
	if resource == nil || limit <= 0 || len(resource.Relationships) == 0 {
		return ""
	}

	if limit > len(resource.Relationships) {
		limit = len(resource.Relationships)
	}

	lines := make([]string, 0, limit)
	for _, rel := range resource.Relationships {
		if len(lines) >= limit {
			break
		}
		presentation := DescribeRelationship(rel)
		parts := []string{
			fmt.Sprintf("**%s** %s", presentation.TypeLabel, presentation.Direction),
		}
		if presentation.StateLabel != "" {
			parts = append(parts, presentation.StateLabel)
		}
		if presentation.Provenance != "" {
			parts = append(parts, fmt.Sprintf("discoverer %s", presentation.Provenance))
		}
		if presentation.Confidence != "" {
			parts = append(parts, fmt.Sprintf("confidence %s", presentation.Confidence))
		}
		if !rel.ObservedAt.IsZero() {
			parts = append(parts, fmt.Sprintf("observed %s", utils.FormatDurationAgo(time.Since(rel.ObservedAt).Truncate(time.Minute))))
		}
		if !rel.LastSeenAt.IsZero() {
			parts = append(parts, fmt.Sprintf("last seen %s", utils.FormatDurationAgo(time.Since(rel.LastSeenAt).Truncate(time.Minute))))
		}
		if presentation.HasMetadata {
			parts = append(parts, "metadata present")
		}
		lines = append(lines, strings.Join(parts, "; "))
	}

	if len(lines) == 0 {
		return ""
	}
	return "\n\n### Resource Graph\n" + strings.Join(lines, "\n")
}
