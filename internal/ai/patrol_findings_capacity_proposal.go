// patrol_findings_capacity_proposal.go: glue between the patrol finding
// pipeline and the deterministic forecast-driven action templates in
// internal/ai/forecast.
//
// Lives in its own file (rather than in patrol_findings.go) because the
// projection is self-contained and easy to grow as more capacity-forecast
// resource types come online (e.g. PVE node disk, Ceph OSD usage). Adding a
// new resource type generally means: (1) register a template in
// capacity_action_templates.go, (2) extend metricForCapacityFinding /
// resourceTypeForCapacityFinding here, and that's it.
package ai

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/forecast"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// percentInTitle pulls a leading percentage value out of a finding title
// such as "Storage pool tank at 87.3% usage". This is best-effort - if a
// future signal detector phrases its title differently, the proposal still
// renders, just without a current-value snapshot.
var percentInTitle = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%`)

// buildCapacityActionProposal returns a wire-side ProposedActionPlan when
// the finding is a capacity finding for a resource type that has a
// registered template, or nil otherwise. The pipeline calls this before
// engine.CreatePlan so the proposal lands on the same RemediationPlan that
// FindingsPanel already loads from /api/ai/remediation/plans.
//
// Read-side and approval-gated. The template registry guarantees
// RequiresApproval=true; this helper additionally guarantees Allowed
// flows through unchanged (no escalation here).
func buildCapacityActionProposal(finding *Finding) *aicontracts.ProposedActionPlan {
	if finding == nil {
		return nil
	}
	if finding.Category != FindingCategoryCapacity {
		return nil
	}

	resourceType := normalizedCapacityResourceType(finding)
	metric := metricForCapacityFinding(finding, resourceType)
	if !forecast.HasCapacityActionTemplate(resourceType, metric) {
		return nil
	}

	currentValue := extractCurrentValue(finding)

	// PredictedValue / TimeToThreshold are best-effort: the forecast
	// service is available on the patrol service via SetForecastService
	// in some deployments, but the wire-in must not depend on it being
	// configured. When unavailable, we surface the current value alone -
	// the finding crossing threshold is itself sufficient signal to
	// propose remediation per the lane brief.
	predictedValue := currentValue
	thresholdValue := capacityThresholdForFinding(resourceType)

	plan := forecast.BuildActionPlanForFinding(forecast.CapacityFindingInput{
		FindingID:      finding.ID,
		ResourceID:     finding.ResourceID,
		ResourceName:   finding.ResourceName,
		ResourceType:   resourceType,
		Node:           finding.Node,
		Metric:         metric,
		CurrentValue:   currentValue,
		PredictedValue: predictedValue,
		ThresholdValue: thresholdValue,
		Now:            time.Now().UTC(),
	})
	if plan == nil {
		return nil
	}
	return projectCapacityProposal(plan, metric, currentValue, predictedValue, thresholdValue)
}

// normalizedCapacityResourceType maps the finding's ResourceType to a
// template-registry key. Findings carry resource_type values in
// historical conventions (storage, vm, system-container, pbs); we leave
// them as-is and let the registry's lower-case lookup do the matching.
func normalizedCapacityResourceType(finding *Finding) string {
	rt := strings.TrimSpace(finding.ResourceType)
	return rt
}

// metricForCapacityFinding returns the metric name the registry expects
// for the given resource type. Capacity findings today don't carry an
// explicit metric field, so we pick the canonical one for the resource
// type. Storage/PBS use usage_percent; guests use disk_usage_percent.
func metricForCapacityFinding(_ *Finding, resourceType string) string {
	switch strings.ToLower(strings.TrimSpace(resourceType)) {
	case "storage", "pbs", "pbs-datastore":
		return "usage_percent"
	case "vm", "qemu", "lxc", "system-container":
		return "disk_usage_percent"
	default:
		return ""
	}
}

// capacityThresholdForFinding returns the warning threshold Pulse uses to
// flag a resource as a capacity concern. We surface this on the proposal
// so the operator sees the same threshold that triggered the finding.
//
// These values mirror SignalThresholds defaults in patrol_signals.go;
// keeping the literal here avoids a dependency on the running
// PatrolService config and matches the brief's "or the finding evidence
// already exceeds a configured threshold" path.
func capacityThresholdForFinding(resourceType string) float64 {
	switch strings.ToLower(strings.TrimSpace(resourceType)) {
	case "storage", "pbs", "pbs-datastore":
		return 75.0
	case "vm", "qemu", "lxc", "system-container":
		return 85.0
	default:
		return 0
	}
}

// extractCurrentValue parses the trailing "X.Y%" out of a capacity
// finding title. The signal detectors in patrol_signals.go consistently
// format these as "Storage pool tank at 87.3% usage" / "High disk usage on
// foo: 92.1%". When parsing fails we return 0; the proposal still renders.
func extractCurrentValue(finding *Finding) float64 {
	if finding == nil {
		return 0
	}
	matches := percentInTitle.FindStringSubmatch(finding.Title)
	if len(matches) < 2 {
		matches = percentInTitle.FindStringSubmatch(finding.Description)
	}
	if len(matches) < 2 {
		return 0
	}
	v, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}
	return v
}

// projectCapacityProposal converts a unifiedresources.ActionPlan (the
// shape templates produce, suitable for the action engine) into the
// aicontracts.ProposedActionPlan wire projection that rides on the
// RemediationPlan to the frontend.
//
// We deliberately keep the projection narrow (no execution params, no
// resource version hashes) because the frontend only needs enough to
// render a card. The full ActionPlan is re-derived at execute time.
func projectCapacityProposal(
	plan *unifiedresources.ActionPlan,
	metric string,
	currentValue, predictedValue, thresholdValue float64,
) *aicontracts.ProposedActionPlan {
	if plan == nil {
		return nil
	}
	out := &aicontracts.ProposedActionPlan{
		ActionID:         plan.ActionID,
		Allowed:          plan.Allowed,
		RequiresApproval: plan.RequiresApproval,
		ApprovalPolicy:   string(plan.ApprovalPolicy),
		Message:          plan.Message,
		Source:           forecast.CapacityActionPlanSource,
		PlannedAt:        plan.PlannedAt,
		ExpiresAt:        plan.ExpiresAt,
		ProjectedMetric: &aicontracts.ProposedMetricSummary{
			Metric:         metric,
			CurrentValue:   currentValue,
			PredictedValue: predictedValue,
			ThresholdValue: thresholdValue,
		},
	}
	if plan.Preflight != nil {
		out.Preflight = &aicontracts.ProposedActionPreflight{
			Target:            plan.Preflight.Target,
			CurrentState:      plan.Preflight.CurrentState,
			IntendedChange:    plan.Preflight.IntendedChange,
			DryRunAvailable:   plan.Preflight.DryRunAvailable,
			DryRunSummary:     plan.Preflight.DryRunSummary,
			SafetyChecks:      append([]string(nil), plan.Preflight.SafetyChecks...),
			VerificationSteps: append([]string(nil), plan.Preflight.VerificationSteps...),
		}
	}
	return out
}
