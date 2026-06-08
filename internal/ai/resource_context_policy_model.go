package ai

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type unifiedResourcePolicyContext struct {
	posture           *unifiedresources.PolicyPostureSummary
	externalModel     bool
	localModel        bool
	cloudPrivacy      string
	sensitivityCounts map[unifiedresources.ResourceSensitivity]int
	routingCounts     map[unifiedresources.ResourceRoutingScope]int
	localOnlyCount    int
	redactionHints    []unifiedresources.ResourceRedactionHint
	redactionLabels   []string
}

func buildUnifiedResourcePolicyContext(posture *unifiedresources.PolicyPostureSummary, destinationModel, cloudPrivacy string) unifiedResourcePolicyContext {
	normalizedPrivacy, _ := config.NormalizeCloudContextPrivacy(cloudPrivacy)
	context := unifiedResourcePolicyContext{
		externalModel:     unifiedResourceContextUsesExternalModel(destinationModel),
		localModel:        unifiedResourceContextUsesLocalModel(destinationModel),
		cloudPrivacy:      normalizedPrivacy,
		posture:           posture,
		sensitivityCounts: map[unifiedresources.ResourceSensitivity]int{},
		routingCounts:     map[unifiedresources.ResourceRoutingScope]int{},
	}
	if posture == nil {
		return context
	}

	if posture.SensitivityCounts != nil {
		context.sensitivityCounts = posture.SensitivityCounts
	}
	if posture.RoutingCounts != nil {
		context.routingCounts = posture.RoutingCounts
		context.localOnlyCount = posture.RoutingCounts[unifiedresources.ResourceRoutingScopeLocalOnly]
	}

	context.redactionHints = resourcePolicyRedactionHintsFromCounts(posture.RedactionCounts)
	context.redactionLabels = unifiedresources.ResourceRedactionLabelsFromHints(context.redactionHints)

	return context
}

func unifiedResourceContextUsesExternalModel(destinationModel string) bool {
	destinationModel = strings.TrimSpace(destinationModel)
	if destinationModel == "" {
		return false
	}

	provider, _ := config.ParseModelString(destinationModel)
	return provider != config.AIProviderOllama
}

// unifiedResourceContextUsesLocalModel reports whether the destination is a KNOWN
// local (Ollama) model. An empty/unknown destination is neither external nor
// local, so it fails closed to redaction rather than being treated as local.
func unifiedResourceContextUsesLocalModel(destinationModel string) bool {
	destinationModel = strings.TrimSpace(destinationModel)
	if destinationModel == "" {
		return false
	}
	provider, _ := config.ParseModelString(destinationModel)
	return provider == config.AIProviderOllama
}

func (context unifiedResourcePolicyContext) hasGovernedResources() bool {
	return context.posture != nil && context.posture.TotalResources > 0
}

func (context unifiedResourcePolicyContext) includeResourceDetails(resource unifiedresources.Resource) bool {
	if !context.externalModel || resource.Policy == nil {
		return true
	}
	return resource.Policy.Routing.Scope != unifiedresources.ResourceRoutingScopeLocalOnly
}

// resourceLabel renders a resource's display name for the model-bound inventory
// context, honoring the cloud_context_privacy dial. Local models always get the
// real name (local is always full). For cloud models, the "full" dial shows real
// names EXCEPT for resources the engine routes local-only (the hard floor, which
// stays redacted even at full); "redacted"/"local_only" fall back to the governed
// label. This is the inventory-context half of the dial — the prefetch and the
// model-boundary sanitizer enforce the same posture on their paths.
func (context unifiedResourcePolicyContext) resourceLabel(name, aiSafeSummary string, policy *unifiedresources.ResourcePolicy) string {
	if context.allowsRealIdentifier(policy) {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			return trimmed
		}
	}
	return unifiedresources.ResourcePolicyLabel(name, aiSafeSummary, policy)
}

// allowsRealIdentifier reports whether the real resource identifier may be shown
// for this destination given the dial. Known-local (Ollama) => always. Cloud =>
// only at "full" and only when the resource is not routed local-only. An
// unknown/empty destination fails closed to redaction (it could reach a cloud
// model), preserving the historical safe default for the no-destination path.
func (context unifiedResourcePolicyContext) allowsRealIdentifier(policy *unifiedresources.ResourcePolicy) bool {
	if context.localModel {
		return true
	}
	if !context.externalModel {
		return false
	}
	if context.cloudPrivacy != config.CloudContextPrivacyFull {
		return false
	}
	if policy != nil && policy.Routing.Scope == unifiedresources.ResourceRoutingScopeLocalOnly {
		return false
	}
	return true
}

func (context unifiedResourcePolicyContext) filterDetailedResources(resources []unifiedresources.Resource) []unifiedresources.Resource {
	if !context.externalModel || len(resources) == 0 {
		return resources
	}

	filtered := make([]unifiedresources.Resource, 0, len(resources))
	for _, resource := range resources {
		if context.includeResourceDetails(resource) {
			filtered = append(filtered, resource)
		}
	}
	return filtered
}

func (context unifiedResourcePolicyContext) appendSummarySections(sections []string) []string {
	if !context.hasGovernedResources() {
		return sections
	}

	sections = append(sections, "\n### Data Governance")

	sensitivityParts := unifiedresources.ResourcePolicySensitivitySummaryFromCounts(context.sensitivityCounts)
	sections = append(sections, fmt.Sprintf("- Sensitivity: %s", strings.Join(sensitivityParts, ", ")))

	routingParts := unifiedresources.ResourcePolicyRoutingSummaryFromCounts(context.routingCounts)
	sections = append(sections, fmt.Sprintf("- Routing: %s", strings.Join(routingParts, ", ")))
	sections = append(sections, fmt.Sprintf("- Local-only resources: %d", context.localOnlyCount))
	if context.externalModel && context.localOnlyCount > 0 {
		sections = append(sections, fmt.Sprintf("- External model handling: %d local-only resources are represented only in aggregate and omitted from detailed context.", context.localOnlyCount))
	}

	if len(context.redactionLabels) > 0 {
		sections = append(sections, "\n### Policy Redaction Hints")
		sections = append(sections, fmt.Sprintf("- Redactions in use: %s", strings.Join(context.redactionLabels, ", ")))
	}

	return sections
}

func resourcePolicyRedactionHintsFromCounts(counts map[unifiedresources.ResourceRedactionHint]int) []unifiedresources.ResourceRedactionHint {
	if len(counts) == 0 {
		return nil
	}

	hints := make([]unifiedresources.ResourceRedactionHint, 0, len(counts))
	seen := make(map[unifiedresources.ResourceRedactionHint]struct{}, len(counts))

	for _, hint := range unifiedresources.ResourceRedactionHintOrder {
		if counts[hint] <= 0 {
			continue
		}
		hints = append(hints, hint)
		seen[hint] = struct{}{}
	}

	remaining := make([]string, 0, len(counts))
	remainingHints := make(map[string]unifiedresources.ResourceRedactionHint, len(counts))
	for hint, count := range counts {
		if count <= 0 {
			continue
		}
		if _, ok := seen[hint]; ok {
			continue
		}
		key := string(hint)
		remaining = append(remaining, key)
		remainingHints[key] = hint
	}
	sort.Strings(remaining)
	for _, key := range remaining {
		hints = append(hints, remainingHints[key])
	}

	return hints
}
