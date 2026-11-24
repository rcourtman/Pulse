package unifiedresources

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
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

// ResourcePolicyRedactionReference describes an additional product-originated
// resource reference that should follow the same governed prompt-label boundary
// as canonical resource identifiers.
type ResourcePolicyRedactionReference struct {
	Value string
	Hints []ResourceRedactionHint
}

// ResourcePolicyReference builds a redaction reference with a defensive copy of
// the supplied hint list.
func ResourcePolicyReference(value string, hints ...ResourceRedactionHint) ResourcePolicyRedactionReference {
	return ResourcePolicyRedactionReference{
		Value: strings.TrimSpace(value),
		Hints: append([]ResourceRedactionHint(nil), hints...),
	}
}

// ResourcePolicyRedactedText replaces exact resource identifiers that the
// canonical resource policy marks for redaction. It is intentionally
// value-based so alert, prompt, and export summaries cannot leak the same raw
// hostname, IP, alias, platform ID, or path through nearby free-form text.
func ResourcePolicyRedactedText(value string, resource Resource) string {
	// Text may be a streamed delta. Whitespace is content, not presentation
	// padding, and must survive redaction even when no policy applies.
	if value == "" || resource.Policy == nil {
		return value
	}

	return redactResourceIdentifiers(value, resourcePolicyRedactedIdentifiers(resource))
}

// ResourcePolicyTextRedactor redacts the identifiers of a set of governed
// resources from text that is not only about them, such as a whole
// provider-bound request. The identifiers are collected once and every text is
// redacted in a single pass, so the label written for one resource is never
// matched again by the identifiers of the next.
type ResourcePolicyTextRedactor struct {
	identifiers []string
}

// NewResourcePolicyTextRedactor collects the identifiers that policy hides for
// resources. An identifier in shared is also the ID of a resource whose own
// policy lets it through: the string is not confidential, and replacing it
// would hide that other resource rather than one of these.
func NewResourcePolicyTextRedactor(resources []Resource, shared map[string]struct{}) *ResourcePolicyTextRedactor {
	redactor := &ResourcePolicyTextRedactor{}
	seen := make(map[string]struct{})
	for _, resource := range resources {
		for _, identifier := range resourcePolicyRedactedIdentifiers(resource) {
			if _, ok := shared[identifier]; ok {
				continue
			}
			if _, ok := seen[identifier]; ok {
				continue
			}
			seen[identifier] = struct{}{}
			redactor.identifiers = append(redactor.identifiers, identifier)
		}
	}
	return redactor
}

// RedactedText is ResourcePolicyRedactedText for every resource of the
// redactor at once.
func (r *ResourcePolicyTextRedactor) RedactedText(value string) string {
	if r == nil || value == "" {
		return value
	}
	return redactResourceIdentifiers(value, r.identifiers)
}

// resourcePolicyRedactedIdentifiers returns the identifiers of the resource
// that its policy marks for redaction.
func resourcePolicyRedactedIdentifiers(resource Resource) []string {
	if resource.Policy == nil {
		return nil
	}
	var identifiers []string
	seen := make(map[string]struct{})
	for _, candidate := range resourcePolicyRedactionCandidates(resource) {
		if !resourcePolicyShouldRedactCandidate(resource.Policy, candidate.hint) {
			continue
		}
		if len([]rune(candidate.value)) < 3 {
			continue
		}
		if _, ok := seen[candidate.value]; ok {
			continue
		}
		seen[candidate.value] = struct{}{}
		identifiers = append(identifiers, candidate.value)
	}
	return identifiers
}

// redactResourceIdentifiers replaces each identifier wherever it stands on its
// own. An occurrence that runs on into more identifier characters belongs to a
// different, longer identifier - the "agent" in "agent-4f2a", the "10.0.0.5"
// in "10.0.0.50" - and cutting it out would corrupt a reference to something
// these identifiers do not govern.
//
// Every identifier is matched against the original text and overlapping
// matches share one label, so the result never depends on the order of the
// identifiers and an inserted label is never scanned again.
func redactResourceIdentifiers(text string, identifiers []string) string {
	type match struct{ start, end int }
	var matches []match
	for _, identifier := range identifiers {
		if identifier == "" {
			continue
		}
		first, firstSize := utf8.DecodeRuneInString(identifier)
		last, _ := utf8.DecodeLastRuneInString(identifier)
		// An identifier that itself starts or ends with a delimiter, such as a
		// path, cannot be the middle of a longer word on that side.
		checkBefore, checkAfter := isResourceIdentifierRune(first), isResourceIdentifierRune(last)
		for search := 0; ; {
			offset := strings.Index(text[search:], identifier)
			if offset < 0 {
				break
			}
			start := search + offset
			end := start + len(identifier)
			if !(checkBefore && resourceIdentifierContinuesBefore(text[:start])) &&
				!(checkAfter && resourceIdentifierContinuesAfter(text[end:])) {
				matches = append(matches, match{start: start, end: end})
			}
			search = start + firstSize
		}
	}
	if len(matches) == 0 {
		return text
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].start < matches[j].start })

	var redacted strings.Builder
	covered := 0
	for _, m := range matches {
		if m.start >= covered {
			redacted.WriteString(text[covered:m.start])
			redacted.WriteString(ResourcePolicyRedactedLabel)
		}
		if m.end > covered {
			covered = m.end
		}
	}
	redacted.WriteString(text[covered:])
	return redacted.String()
}

// isResourceIdentifierRune reports whether r next to a match makes it part of
// a longer identifier. Scripts written without spaces between words have no
// such boundary, so only digits and cased letters count and names in those
// scripts keep matching anywhere.
func isResourceIdentifierRune(r rune) bool {
	return unicode.IsDigit(r) || unicode.IsLower(r) || unicode.IsUpper(r)
}

// Hyphens and underscores join the parts of one identifier only when another
// part follows, so a name at the end of a sentence is still whole. Dots join
// numeric segments (such as IPv4 octets) so 10.0.0.5 does not match inside
// 192.10.0.0.5, while remaining boundaries for hostnames like payroll.lan.
func isResourceIdentifierJoiner(r rune) bool {
	return r == '-' || r == '_'
}

func endsWithJSONEscapeBoundary(prefix string) bool {
	if len(prefix) < 2 {
		return false
	}
	last := prefix[len(prefix)-1]
	switch last {
	case 'n', 'r', 't', 'f', 'b':
	default:
		return false
	}
	backslashes := 0
	for i := len(prefix) - 2; i >= 0 && prefix[i] == '\\'; i-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func resourceIdentifierContinuesBefore(prefix string) bool {
	if endsWithJSONEscapeBoundary(prefix) {
		return false
	}
	r, size := utf8.DecodeLastRuneInString(prefix)
	if size > 0 && r == '.' {
		prev, prevSize := utf8.DecodeLastRuneInString(prefix[:len(prefix)-size])
		return prevSize > 0 && unicode.IsDigit(prev)
	}
	if size > 0 && isResourceIdentifierJoiner(r) {
		r, size = utf8.DecodeLastRuneInString(prefix[:len(prefix)-size])
	}
	return size > 0 && isResourceIdentifierRune(r)
}

func resourceIdentifierContinuesAfter(suffix string) bool {
	r, size := utf8.DecodeRuneInString(suffix)
	if size > 0 && r == '.' {
		next, nextSize := utf8.DecodeRuneInString(suffix[size:])
		return nextSize > 0 && unicode.IsDigit(next)
	}
	if size > 0 && isResourceIdentifierJoiner(r) {
		r, size = utf8.DecodeRuneInString(suffix[size:])
	}
	return size > 0 && isResourceIdentifierRune(r)
}

// ResourcePolicyRedactedTextWithReferences redacts canonical resource
// identifiers plus additional structured references supplied by the caller.
// References represent prompt labels, handoff selectors, or transport fields
// that identify the same resource but are not guaranteed to be present on the
// canonical resource itself. When policy requires a governed summary, references
// are redacted as identity fields even if their individual hint is not listed,
// matching ResourcePolicyLabel's "safe summary over raw label" contract.
func ResourcePolicyRedactedTextWithReferences(value string, resource Resource, references ...ResourcePolicyRedactionReference) string {
	redacted := ResourcePolicyRedactedText(value, resource)
	if strings.TrimSpace(redacted) == "" || resource.Policy == nil || len(references) == 0 {
		return redacted
	}

	candidates := make([]ResourcePolicyRedactionReference, 0, len(references))
	seen := make(map[string]struct{}, len(references))
	for _, reference := range references {
		reference.Value = strings.TrimSpace(reference.Value)
		if reference.Value == "" || len([]rune(reference.Value)) < 3 {
			continue
		}
		if _, ok := seen[reference.Value]; ok {
			continue
		}
		seen[reference.Value] = struct{}{}
		reference.Hints = append([]ResourceRedactionHint(nil), reference.Hints...)
		candidates = append(candidates, reference)
	}
	if len(candidates) == 0 {
		return redacted
	}

	sort.Slice(candidates, func(i, j int) bool {
		if len(candidates[i].Value) == len(candidates[j].Value) {
			return candidates[i].Value < candidates[j].Value
		}
		return len(candidates[i].Value) > len(candidates[j].Value)
	})

	for _, candidate := range candidates {
		if !resourcePolicyShouldRedactReference(resource.Policy, candidate.Hints...) {
			continue
		}
		redacted = redactResourceIdentifiers(redacted, []string{candidate.Value})
	}
	return redacted
}

func resourcePolicyShouldRedactReference(policy *ResourcePolicy, hints ...ResourceRedactionHint) bool {
	if policy == nil {
		return false
	}
	if ResourcePolicyRequiresGovernedSummary(policy) {
		return true
	}
	return ResourcePolicyRedacts(policy, hints...)
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
	return ResourcePolicyRequiresGovernedSummary(policy)
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

type resourcePolicyRedactionCandidate struct {
	hint  ResourceRedactionHint
	value string
}

func resourcePolicyShouldRedactCandidate(policy *ResourcePolicy, hint ResourceRedactionHint) bool {
	if policy == nil {
		return false
	}
	if policy.Routing.Scope == ResourceRoutingScopeLocalOnly {
		return true
	}
	return ResourcePolicyRedacts(policy, hint)
}

func resourcePolicyRedactionCandidates(resource Resource) []resourcePolicyRedactionCandidate {
	var candidates []resourcePolicyRedactionCandidate
	add := func(hint ResourceRedactionHint, values ...string) {
		for _, value := range values {
			if value = strings.TrimSpace(value); value != "" {
				candidates = append(candidates, resourcePolicyRedactionCandidate{hint: hint, value: value})
			}
		}
	}

	add(ResourceRedactionHostname, resource.Name, canonicalHostname(resource))
	add(ResourceRedactionHostname, resource.Identity.Hostnames...)
	add(ResourceRedactionIPAddress, resource.Identity.IPAddresses...)

	if resource.Canonical != nil {
		add(ResourceRedactionHostname, resource.Canonical.DisplayName)
		add(ResourceRedactionPlatformID, resource.Canonical.PlatformID, resource.Canonical.PrimaryID)
		add(ResourceRedactionAlias, resource.Canonical.Aliases...)
	}

	add(ResourceRedactionPlatformID, canonicalPlatformID(resource), resource.ID)
	add(ResourceRedactionAlias, resource.Identity.ClusterName)

	if resource.Proxmox != nil {
		add(ResourceRedactionHostname, resource.Proxmox.NodeName)
		add(ResourceRedactionAlias, resource.Proxmox.ClusterName)
	}
	if resource.Kubernetes != nil {
		add(ResourceRedactionPlatformID, resource.Kubernetes.ClusterID)
		add(ResourceRedactionAlias, resource.Kubernetes.ClusterName, resource.Kubernetes.SourceName)
	}
	if resource.Storage != nil {
		add(ResourceRedactionPath, resource.Storage.Path)
	}

	return candidates
}
