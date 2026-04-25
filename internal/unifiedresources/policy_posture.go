package unifiedresources

// PolicyPostureSummary captures the canonical data-governance posture derived
// from the unified resource registry.
type PolicyPostureSummary struct {
	TotalResources    int                           `json:"total_resources"`
	SensitivityCounts map[ResourceSensitivity]int   `json:"sensitivity_counts,omitempty"`
	RoutingCounts     map[ResourceRoutingScope]int  `json:"routing_counts,omitempty"`
	RedactionCounts   map[ResourceRedactionHint]int `json:"redaction_counts,omitempty"`
}

// ResourcePolicyPostureSummary is the camelCase REST contract for policy posture
// exposed through resource aggregations.
type ResourcePolicyPostureSummary struct {
	TotalResources    int                           `json:"totalResources"`
	SensitivityCounts map[ResourceSensitivity]int   `json:"sensitivityCounts"`
	RoutingCounts     map[ResourceRoutingScope]int  `json:"routingCounts"`
	RedactionCounts   map[ResourceRedactionHint]int `json:"redactionCounts"`
}

// SummarizePolicyPosture aggregates canonical policy posture across the given
// unified resources.
func SummarizePolicyPosture(resources []Resource) *PolicyPostureSummary {
	if len(resources) == 0 {
		return &PolicyPostureSummary{}
	}

	summary := &PolicyPostureSummary{
		TotalResources:    len(resources),
		SensitivityCounts: make(map[ResourceSensitivity]int),
		RoutingCounts:     make(map[ResourceRoutingScope]int),
		RedactionCounts:   make(map[ResourceRedactionHint]int),
	}

	for _, resource := range resources {
		if resource.Policy == nil {
			continue
		}
		summary.SensitivityCounts[resource.Policy.Sensitivity]++
		summary.RoutingCounts[resource.Policy.Routing.Scope]++
		for _, hint := range resource.Policy.Routing.Redact {
			summary.RedactionCounts[hint]++
		}
	}

	if len(summary.SensitivityCounts) == 0 {
		summary.SensitivityCounts = nil
	}
	if len(summary.RoutingCounts) == 0 {
		summary.RoutingCounts = nil
	}
	if len(summary.RedactionCounts) == 0 {
		summary.RedactionCounts = nil
	}

	return summary
}

// ResourcePolicyPostureContract converts the canonical policy posture summary
// into the resource API's camelCase, non-null collection contract.
func ResourcePolicyPostureContract(summary *PolicyPostureSummary) *ResourcePolicyPostureSummary {
	contract := &ResourcePolicyPostureSummary{}
	if summary != nil {
		contract.TotalResources = summary.TotalResources
		contract.SensitivityCounts = cloneResourceSensitivityCounts(summary.SensitivityCounts)
		contract.RoutingCounts = cloneResourceRoutingCounts(summary.RoutingCounts)
		contract.RedactionCounts = cloneResourceRedactionCounts(summary.RedactionCounts)
	}
	return contract.NormalizeCollections()
}

// EmptyResourcePolicyPostureSummary returns the canonical empty resource API
// policy posture contract.
func EmptyResourcePolicyPostureSummary() *ResourcePolicyPostureSummary {
	return (&ResourcePolicyPostureSummary{}).NormalizeCollections()
}

// NormalizeCollections keeps resource API policy posture maps as JSON objects
// instead of nulls.
func (summary *ResourcePolicyPostureSummary) NormalizeCollections() *ResourcePolicyPostureSummary {
	if summary == nil {
		return EmptyResourcePolicyPostureSummary()
	}
	if summary.SensitivityCounts == nil {
		summary.SensitivityCounts = map[ResourceSensitivity]int{}
	}
	if summary.RoutingCounts == nil {
		summary.RoutingCounts = map[ResourceRoutingScope]int{}
	}
	if summary.RedactionCounts == nil {
		summary.RedactionCounts = map[ResourceRedactionHint]int{}
	}
	return summary
}

func cloneResourceSensitivityCounts(in map[ResourceSensitivity]int) map[ResourceSensitivity]int {
	if in == nil {
		return nil
	}
	out := make(map[ResourceSensitivity]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneResourceRoutingCounts(in map[ResourceRoutingScope]int) map[ResourceRoutingScope]int {
	if in == nil {
		return nil
	}
	out := make(map[ResourceRoutingScope]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneResourceRedactionCounts(in map[ResourceRedactionHint]int) map[ResourceRedactionHint]int {
	if in == nil {
		return nil
	}
	out := make(map[ResourceRedactionHint]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
