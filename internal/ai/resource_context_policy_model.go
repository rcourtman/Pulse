package ai

import (
	"fmt"
	"sort"
	"strings"

	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type unifiedResourcePolicyContext struct {
	posture           *unifiedresources.PolicyPostureSummary
	sensitivityCounts map[unifiedresources.ResourceSensitivity]int
	routingCounts     map[unifiedresources.ResourceRoutingScope]int
	localOnlyCount    int
	redactionHints    []unifiedresources.ResourceRedactionHint
	redactionLabels   []string
}

func buildUnifiedResourcePolicyContext(posture *unifiedresources.PolicyPostureSummary) unifiedResourcePolicyContext {
	context := unifiedResourcePolicyContext{
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

func (context unifiedResourcePolicyContext) hasGovernedResources() bool {
	return context.posture != nil && context.posture.TotalResources > 0
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
