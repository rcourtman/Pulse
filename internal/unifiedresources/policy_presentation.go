package unifiedresources

import "sort"

// ResourceSensitivityLabel returns the canonical human-readable sensitivity label.
func ResourceSensitivityLabel(sensitivity ResourceSensitivity) string {
	switch sensitivity {
	case ResourceSensitivityPublic:
		return "Public"
	case ResourceSensitivityInternal:
		return "Internal"
	case ResourceSensitivitySensitive:
		return "Sensitive"
	case ResourceSensitivityRestricted:
		return "Restricted"
	default:
		return "Unclassified"
	}
}

// ResourceRoutingScopeLabel returns the canonical human-readable routing scope label.
func ResourceRoutingScopeLabel(scope ResourceRoutingScope) string {
	switch scope {
	case ResourceRoutingScopeCloudSummary:
		return "Cloud Summary"
	case ResourceRoutingScopeLocalFirst:
		return "Local First"
	case ResourceRoutingScopeLocalOnly:
		return "Local Only"
	default:
		return "Unrouted"
	}
}

// ResourceRedactionHintLabel returns the canonical human-readable redaction label.
func ResourceRedactionHintLabel(hint ResourceRedactionHint) string {
	switch hint {
	case ResourceRedactionHostname:
		return "Hostname"
	case ResourceRedactionIPAddress:
		return "IP Address"
	case ResourceRedactionPlatformID:
		return "Platform ID"
	case ResourceRedactionAlias:
		return "Alias"
	case ResourceRedactionPath:
		return "Path"
	default:
		return string(hint)
	}
}

// ResourcePolicyRedactionLabels returns the canonical human-readable labels for a policy's redaction hints.
func ResourcePolicyRedactionLabels(policy *ResourcePolicy) []string {
	if policy == nil || len(policy.Routing.Redact) == 0 {
		return nil
	}
	labels := make([]string, 0, len(policy.Routing.Redact))
	for _, hint := range policy.Routing.Redact {
		label := ResourceRedactionHintLabel(hint)
		if label == "" {
			continue
		}
		labels = append(labels, label)
	}
	sort.Strings(labels)
	return labels
}
