package unifiedresources

import "strings"

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
