package unifiedresources

import "sort"

// ResourceRedactionHintOrder captures the canonical presentation order for
// redaction hints across backend and frontend policy surfaces.
var ResourceRedactionHintOrder = []ResourceRedactionHint{
	ResourceRedactionHostname,
	ResourceRedactionIPAddress,
	ResourceRedactionPlatformID,
	ResourceRedactionAlias,
	ResourceRedactionPath,
}

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
	counts := make(map[ResourceRedactionHint]int, len(policy.Routing.Redact))
	for _, hint := range policy.Routing.Redact {
		counts[hint]++
	}
	return ResourcePolicyRedactionLabelsFromCounts(counts)
}

// ResourcePolicyRedactionLabelsFromCounts returns the canonical labels for the
// redaction hints present in the provided count map.
func ResourcePolicyRedactionLabelsFromCounts(counts map[ResourceRedactionHint]int) []string {
	if len(counts) == 0 {
		return nil
	}

	labels := make([]string, 0, len(counts))
	seen := make(map[ResourceRedactionHint]struct{}, len(counts))

	for _, hint := range ResourceRedactionHintOrder {
		if counts[hint] <= 0 {
			continue
		}
		label := ResourceRedactionHintLabel(hint)
		if label == "" {
			continue
		}
		labels = append(labels, label)
		seen[hint] = struct{}{}
	}

	remaining := make([]string, 0, len(counts))
	for hint, count := range counts {
		if count <= 0 {
			continue
		}
		if _, ok := seen[hint]; ok {
			continue
		}
		label := ResourceRedactionHintLabel(hint)
		if label == "" {
			continue
		}
		remaining = append(remaining, label)
	}
	sort.Strings(remaining)
	return append(labels, remaining...)
}
