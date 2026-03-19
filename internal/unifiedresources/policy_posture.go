package unifiedresources

// PolicyPostureSummary captures the canonical data-governance posture derived
// from the unified resource registry.
type PolicyPostureSummary struct {
	TotalResources    int                           `json:"total_resources"`
	SensitivityCounts map[ResourceSensitivity]int   `json:"sensitivity_counts,omitempty"`
	RoutingCounts     map[ResourceRoutingScope]int  `json:"routing_counts,omitempty"`
	RedactionCounts   map[ResourceRedactionHint]int `json:"redaction_counts,omitempty"`
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
