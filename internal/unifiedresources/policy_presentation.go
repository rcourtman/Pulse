package unifiedresources

import (
	"fmt"
	"sort"
	"strings"
)

// ResourcePolicyRedactedLabel is the canonical human-readable label used when
// governed policy hides a value.
const ResourcePolicyRedactedLabel = "redacted by policy"

// ResourceSensitivityOrder captures the canonical presentation order for
// sensitivity counts across policy surfaces.
var ResourceSensitivityOrder = []ResourceSensitivity{
	ResourceSensitivityPublic,
	ResourceSensitivityInternal,
	ResourceSensitivitySensitive,
	ResourceSensitivityRestricted,
}

// ResourceRoutingScopeOrder captures the canonical presentation order for
// routing counts across policy surfaces.
var ResourceRoutingScopeOrder = []ResourceRoutingScope{
	ResourceRoutingScopeCloudSummary,
	ResourceRoutingScopeLocalFirst,
	ResourceRoutingScopeLocalOnly,
}

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

// ResourceRedactionLabelsFromHints returns canonical human-readable labels for a hint slice.
func ResourceRedactionLabelsFromHints(hints []ResourceRedactionHint) []string {
	if len(hints) == 0 {
		return nil
	}
	counts := make(map[ResourceRedactionHint]int, len(hints))
	for _, hint := range hints {
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

// ResourcePolicySensitivitySummaryFromCounts returns the canonical
// human-readable count summary for sensitivity posture.
func ResourcePolicySensitivitySummaryFromCounts(counts map[ResourceSensitivity]int) []string {
	if len(counts) == 0 {
		return nil
	}

	parts := make([]string, 0, len(ResourceSensitivityOrder))
	for _, sensitivity := range ResourceSensitivityOrder {
		parts = append(parts, fmt.Sprintf("%d %s",
			counts[sensitivity], ResourceSensitivityLabel(sensitivity)))
	}
	return parts
}

// ResourcePolicyRoutingSummaryFromCounts returns the canonical human-readable
// count summary for routing posture.
func ResourcePolicyRoutingSummaryFromCounts(counts map[ResourceRoutingScope]int) []string {
	if len(counts) == 0 {
		return nil
	}

	parts := make([]string, 0, len(ResourceRoutingScopeOrder))
	for _, scope := range ResourceRoutingScopeOrder {
		parts = append(parts, fmt.Sprintf("%d %s",
			counts[scope], ResourceRoutingScopeLabel(scope)))
	}
	return parts
}

// ResourcePolicySummaryLines returns the canonical human-readable summary lines
// for a single resource policy.
func ResourcePolicySummaryLines(policy *ResourcePolicy) []string {
	if policy == nil {
		return nil
	}

	lines := []string{
		fmt.Sprintf("Policy: sensitivity=%s, routing=%s",
			ResourceSensitivityLabel(policy.Sensitivity),
			ResourceRoutingScopeLabel(policy.Routing.Scope),
		),
	}

	if redactions := ResourcePolicyRedactionLabels(policy); len(redactions) > 0 {
		lines = append(lines, fmt.Sprintf("Redactions: %s", strings.Join(redactions, ", ")))
	}

	return lines
}

// ResourcePolicyGovernedSummaryPreamble returns the canonical opening guidance
// used for governed mention and prompt summaries.
func ResourcePolicyGovernedSummaryPreamble() string {
	return "Raw hostnames, paths, and local identifiers are withheld when governed resource policy requires redaction."
}

// ResourcePolicyGovernedSummaryFooter returns the canonical closing guidance
// used for governed mention summaries.
func ResourcePolicyGovernedSummaryFooter() string {
	return "Raw routing coordinates, bind mounts, hostnames, and discovery file paths withheld by canonical resource policy."
}

// FormatResourcePolicyGovernedSummary returns the canonical governed mention
// block for AI chat prefetch and related policy-aware summaries.
func FormatResourcePolicyGovernedSummary(summary string, policy *ResourcePolicy) string {
	lines := make([]string, 0, 4)
	lines = append(lines, "## Governed resource")
	if summary = strings.TrimSpace(summary); summary != "" {
		lines = append(lines, summary)
	}
	lines = append(lines, ResourcePolicySummaryLines(policy)...)
	lines = append(lines, ResourcePolicyGovernedSummaryFooter())
	return strings.Join(lines, "\n") + "\n\n"
}

// ResourcePolicyRedacts reports whether the policy redacts any of the provided hints.
func ResourcePolicyRedacts(policy *ResourcePolicy, hints ...ResourceRedactionHint) bool {
	if policy == nil {
		return false
	}
	for _, candidate := range policy.Routing.Redact {
		for _, hint := range hints {
			if candidate == hint {
				return true
			}
		}
	}
	return false
}

// ResourcePolicyLabel returns the governed display label for a resource.
func ResourcePolicyLabel(name, aiSafeSummary string, policy *ResourcePolicy) string {
	if ResourcePolicyUsesAISafeSummary(aiSafeSummary, policy) {
		return strings.TrimSpace(aiSafeSummary)
	}
	if ResourcePolicyRequiresGovernedSummary(policy) {
		return ResourcePolicyRedactedLabel
	}
	return strings.TrimSpace(name)
}

// ResourcePolicyRedactedValue returns the canonical redacted label when the
// provided policy hides the supplied value.
func ResourcePolicyRedactedValue(value string, policy *ResourcePolicy, hints ...ResourceRedactionHint) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if ResourcePolicyRedacts(policy, hints...) {
		return ResourcePolicyRedactedLabel
	}
	return value
}

// ResourceDisplayName returns the canonical resource display name fallback.
func ResourceDisplayName(resource Resource) string {
	if resource.Canonical != nil {
		if name := strings.TrimSpace(resource.Canonical.DisplayName); name != "" {
			return name
		}
	}
	if name := strings.TrimSpace(resource.Name); name != "" {
		return name
	}
	return strings.TrimSpace(resource.ID)
}

// ResourceClusterName returns the canonical cluster or topology name for a resource.
func ResourceClusterName(resource Resource) string {
	var name string
	switch {
	case strings.TrimSpace(resource.Identity.ClusterName) != "":
		name = strings.TrimSpace(resource.Identity.ClusterName)
	case resource.Proxmox != nil && strings.TrimSpace(resource.Proxmox.ClusterName) != "":
		name = strings.TrimSpace(resource.Proxmox.ClusterName)
	case resource.Kubernetes != nil && strings.TrimSpace(resource.Kubernetes.ClusterName) != "":
		name = strings.TrimSpace(resource.Kubernetes.ClusterName)
	case resource.Kubernetes != nil && strings.TrimSpace(resource.Kubernetes.SourceName) != "":
		name = strings.TrimSpace(resource.Kubernetes.SourceName)
	}
	if name == "" {
		return ""
	}
	return ResourcePolicyRedactedValue(name, resource.Policy,
		ResourceRedactionAlias,
		ResourceRedactionHostname,
		ResourceRedactionPlatformID,
	)
}

// ResourceIPSummary returns the canonical governed IP summary line for a resource.
func ResourceIPSummary(resource Resource, limit int) string {
	ips := resource.Identity.IPAddresses
	if len(ips) == 0 {
		return ""
	}
	if limit > 0 && len(ips) > limit {
		ips = ips[:limit]
	}
	return " - IPs " + ResourcePolicyRedactedValue(strings.Join(ips, ", "), resource.Policy, ResourceRedactionIPAddress)
}

// ResourcePolicyUsesAISafeSummary reports whether the canonical aiSafeSummary
// should be used instead of raw resource labels for governed output.
func ResourcePolicyUsesAISafeSummary(summary string, policy *ResourcePolicy) bool {
	if strings.TrimSpace(summary) == "" || policy == nil {
		return false
	}
	if policy.Routing.Scope == ResourceRoutingScopeLocalOnly {
		return true
	}
	return ResourcePolicyRedacts(policy,
		ResourceRedactionAlias,
		ResourceRedactionHostname,
		ResourceRedactionPlatformID,
	)
}

// ResourcePolicyRequiresGovernedSummary reports whether the policy requires a
// governed mention or prompt summary instead of raw resource identity fields.
func ResourcePolicyRequiresGovernedSummary(policy *ResourcePolicy) bool {
	if policy == nil {
		return false
	}
	if policy.Routing.Scope == ResourceRoutingScopeLocalOnly {
		return true
	}
	return len(policy.Routing.Redact) > 0
}
