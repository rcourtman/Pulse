package unifiedresources

import (
	"fmt"
	"strings"
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
