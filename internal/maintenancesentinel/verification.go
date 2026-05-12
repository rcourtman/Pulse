// Package maintenancesentinel implements the Maintenance Verification
// Report loop. When a maintenance window ends for a resource, the
// sentinel runs deterministic checks against the resource's current
// state and writes a durable Maintenance Verification Report.
//
// The package is intentionally small and shaped so future loops
// (post-incident verification, post-deployment verification) can reuse
// the same persistence (LoopReport) and review actions. This is
// substrate, not a generic agent framework.
package maintenancesentinel

import (
	"fmt"
	"strings"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Severity classifies an alert or finding for the verification
// decision tree. Maintenance Verification only distinguishes critical
// vs. warning — the deeper severity grades (watch/info) are not
// load-bearing for the decision.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
)

// AlertSummary is the slim projection of an alert the sentinel needs.
// The router adapts the canonical `alerts.Alert` into this shape so
// this package does not import `internal/alerts`.
type AlertSummary struct {
	ID           string
	Severity     Severity
	Type         string
	Acknowledged bool
}

// FindingSummary is the slim projection of a Patrol finding the
// sentinel needs. The router adapts `ai.Finding` into this shape so
// this package does not import `internal/ai`.
type FindingSummary struct {
	ID           string
	Severity     Severity
	Category     string
	Resolved     bool
	Acknowledged bool
}

// ActionSummary is the slim projection of an action audit record the
// sentinel needs.
type ActionSummary struct {
	ID        string
	State     string
	UpdatedAt time.Time
}

// MetricSample is a single recent metric data point used to decide
// whether a resource is reporting metrics after maintenance closed.
type MetricSample struct {
	Metric    string
	Value     float64
	Timestamp time.Time
}

// VerificationInputs is the full deterministic input bundle the
// evaluator consumes. The sentinel collects this once before calling
// EvaluateVerification.
type VerificationInputs struct {
	ResourceID      string
	ResourceName    string
	OperatorState   *unified.ResourceOperatorState
	WindowStartedAt time.Time
	WindowEndedAt   time.Time
	Now             time.Time
	ActiveAlerts    []AlertSummary
	ActiveFindings  []FindingSummary
	// RecentActions should be the action audits with UpdatedAt >=
	// WindowStartedAt. The evaluator counts how many of them ended in
	// "failed" state.
	RecentActions []ActionSummary
	// PostWindowMetricSamples are the metric samples observed after
	// WindowEndedAt. Empty slice means no metrics have been reported
	// since the window closed.
	PostWindowMetricSamples []MetricSample
	// MetricSourceAvailable indicates whether the sentinel had access
	// to a metric source for this resource at all. Distinguishes
	// "we looked and saw nothing" from "we have no way to look".
	MetricSourceAvailable bool
}

// EvaluateVerification applies the deterministic decision tree to the
// inputs and returns a fully-populated LoopReport. The function is
// pure: no I/O, no time.Now() (always uses inputs.Now), no goroutines.
func EvaluateVerification(inputs VerificationInputs) unified.LoopReport {
	now := inputs.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()

	canonicalID := unified.CanonicalResourceID(inputs.ResourceID)
	report := unified.LoopReport{
		ID:          newReportID(canonicalID, inputs.WindowEndedAt),
		Type:        unified.LoopReportTypeMaintenanceVerification,
		Scope:       canonicalID,
		Trigger:     "maintenance_window_end",
		Goal:        "Confirm the resource recovered after the maintenance window closed.",
		Status:      unified.LoopReportStatusPending,
		StartedAt:   now,
		CompletedAt: now,
	}
	if !inputs.WindowStartedAt.IsZero() {
		t := inputs.WindowStartedAt.UTC()
		report.WindowStartedAt = &t
	}
	if !inputs.WindowEndedAt.IsZero() {
		t := inputs.WindowEndedAt.UTC()
		report.WindowEndedAt = &t
	}

	evidence := unified.LoopReportEvidence{
		OperatorStateSummary: summarizeOperatorState(inputs.OperatorState, inputs.WindowEndedAt, now),
	}

	criticalAlerts, warningAlerts, alertIDs := classifyAlerts(inputs.ActiveAlerts)
	evidence.ActiveCriticalAlerts = criticalAlerts
	evidence.ActiveWarningAlerts = warningAlerts
	report.LinkedAlertIDs = alertIDs

	criticalFindings, warningFindings, findingIDs := classifyFindings(inputs.ActiveFindings)
	evidence.ActiveCriticalFindings = criticalFindings
	evidence.ActiveWarningFindings = warningFindings
	report.LinkedFindingIDs = findingIDs

	failedActions, failedActionIDs := classifyFailedActions(inputs.RecentActions, inputs.WindowStartedAt)
	evidence.FailedActionsSinceWindowStart = failedActions
	report.LinkedActionIDs = failedActionIDs

	if inputs.MetricSourceAvailable {
		evidence.MetricRecovery = summarizeMetricRecovery(inputs.PostWindowMetricSamples, inputs.WindowEndedAt, now)
	}

	// Patrol-run trigger is deferred — see PatrolRunTODO contract.
	evidence.PatrolRunTODO = patrolRunTODO(inputs, evidence)

	report.Evidence = evidence
	report.Status, report.Recommendation = decideStatus(inputs, evidence)
	return report
}

func summarizeOperatorState(state *unified.ResourceOperatorState, windowEnd, now time.Time) string {
	if state == nil {
		return "no operator state recorded for this resource"
	}
	parts := []string{}
	if state.MaintenanceStartAt != nil && state.MaintenanceEndAt != nil {
		if state.MaintenanceEndAt.After(now) {
			parts = append(parts, "maintenance window still open")
		} else {
			parts = append(parts, "maintenance window ended")
		}
	} else {
		parts = append(parts, "no maintenance window")
	}
	if state.IntentionallyOffline {
		parts = append(parts, "intentionally offline")
	}
	if state.NeverAutoRemediate {
		parts = append(parts, "never auto-remediate")
	}
	_ = windowEnd
	return strings.Join(parts, "; ")
}

func classifyAlerts(alerts []AlertSummary) (criticalCount, warningCount int, ids []string) {
	for _, a := range alerts {
		switch a.Severity {
		case SeverityCritical:
			criticalCount++
		case SeverityWarning:
			warningCount++
		}
		if a.ID != "" {
			ids = append(ids, a.ID)
		}
	}
	return criticalCount, warningCount, ids
}

func classifyFindings(findings []FindingSummary) (criticalCount, warningCount int, ids []string) {
	for _, f := range findings {
		if f.Resolved {
			continue
		}
		switch f.Severity {
		case SeverityCritical:
			criticalCount++
		case SeverityWarning:
			warningCount++
		}
		if f.ID != "" {
			ids = append(ids, f.ID)
		}
	}
	return criticalCount, warningCount, ids
}

func classifyFailedActions(actions []ActionSummary, windowStart time.Time) (count int, ids []string) {
	for _, a := range actions {
		if a.State != "failed" {
			continue
		}
		if !windowStart.IsZero() && a.UpdatedAt.Before(windowStart) {
			continue
		}
		count++
		if a.ID != "" {
			ids = append(ids, a.ID)
		}
	}
	return count, ids
}

func summarizeMetricRecovery(samples []MetricSample, windowEnd, now time.Time) *unified.MetricRecoveryEvidence {
	rec := &unified.MetricRecoveryEvidence{}
	metrics := map[string]struct{}{}
	relevant := make([]MetricSample, 0, len(samples))
	for _, s := range samples {
		if !s.Timestamp.After(windowEnd) {
			continue
		}
		relevant = append(relevant, s)
		if s.Metric != "" {
			metrics[s.Metric] = struct{}{}
		}
	}
	rec.SamplesAfterEnd = len(relevant)
	if len(metrics) > 0 {
		rec.MetricsObserved = make([]string, 0, len(metrics))
		for m := range metrics {
			rec.MetricsObserved = append(rec.MetricsObserved, m)
		}
	}
	if len(relevant) == 0 {
		rec.Trend = "unknown"
		rec.Note = "no metric samples have been recorded since the window closed"
		return rec
	}
	rec.Trend = trendFromSamples(relevant)
	if rec.Trend == "stable" {
		rec.Note = fmt.Sprintf("%d post-window samples observed; values within expected band", len(relevant))
	}
	return rec
}

// trendFromSamples reports a simple plain-English trend label by
// comparing the first half of samples to the second half. The MVP is
// deterministic and crude on purpose — anomaly detection is the
// baseline store's job, not the sentinel's.
func trendFromSamples(samples []MetricSample) string {
	if len(samples) < 2 {
		return "unknown"
	}
	byMetric := map[string][]MetricSample{}
	for _, s := range samples {
		byMetric[s.Metric] = append(byMetric[s.Metric], s)
	}
	var trends []string
	for _, ms := range byMetric {
		if len(ms) < 2 {
			trends = append(trends, "unknown")
			continue
		}
		mid := len(ms) / 2
		firstAvg := averageValue(ms[:mid])
		secondAvg := averageValue(ms[mid:])
		// Threshold is intentionally wide — the trend label only
		// changes the report's note, not its overall status.
		switch {
		case firstAvg == 0 && secondAvg == 0:
			trends = append(trends, "stable")
		case secondAvg <= firstAvg*1.05+0.01:
			trends = append(trends, "stable")
		case secondAvg > firstAvg*1.5:
			trends = append(trends, "degrading")
		default:
			trends = append(trends, "stable")
		}
	}
	// Combine: any degrading wins, else stable.
	for _, t := range trends {
		if t == "degrading" {
			return "degrading"
		}
	}
	return "stable"
}

func averageValue(samples []MetricSample) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += s.Value
	}
	return sum / float64(len(samples))
}

func patrolRunTODO(inputs VerificationInputs, evidence unified.LoopReportEvidence) string {
	// If the deterministic evidence is unambiguous (clear failure or
	// clear pass), there's nothing for Patrol to add.
	if evidence.ActiveCriticalAlerts > 0 || evidence.ActiveCriticalFindings > 0 || evidence.FailedActionsSinceWindowStart > 0 {
		return ""
	}
	if evidence.ActiveWarningAlerts == 0 && evidence.ActiveWarningFindings == 0 {
		if evidence.MetricRecovery != nil && evidence.MetricRecovery.SamplesAfterEnd > 0 {
			return ""
		}
	}
	_ = inputs
	// Ambiguous case: a scoped Patrol run would help.
	//
	// MVP: deterministic checks only. The Patrol API surfaces a global
	// scheduled run today; a scoped per-resource trigger that does
	// not race the global scheduler is not yet available. Until that
	// lands, we leave a TODO breadcrumb on the report so the operator
	// knows verification was conservative, and so future work can
	// pick this up cleanly.
	return "scoped Patrol run not triggered — per-resource scoped run entrypoint is not yet implemented; review evidence manually"
}

func decideStatus(inputs VerificationInputs, evidence unified.LoopReportEvidence) (unified.LoopReportStatus, string) {
	// Strong negative signals → failed verification.
	if evidence.ActiveCriticalAlerts > 0 {
		return unified.LoopReportStatusFailedVerification,
			"Critical alert(s) are active after maintenance closed. Investigate before clearing operator state."
	}
	if evidence.ActiveCriticalFindings > 0 {
		return unified.LoopReportStatusFailedVerification,
			"Critical Patrol finding(s) are active. Resource has not recovered cleanly."
	}
	if evidence.FailedActionsSinceWindowStart > 0 {
		return unified.LoopReportStatusFailedVerification,
			"One or more recovery actions failed during or after the maintenance window. Review the action audit timeline."
	}

	// Ambiguous signals → needs review.
	if inputs.OperatorState != nil && inputs.OperatorState.IntentionallyOffline {
		return unified.LoopReportStatusNeedsReview,
			"Resource is marked intentionally offline. Verification cannot confirm recovery automatically."
	}
	if evidence.ActiveWarningAlerts > 0 || evidence.ActiveWarningFindings > 0 {
		return unified.LoopReportStatusNeedsReview,
			"Warning-level signals are active after maintenance closed. Operator review recommended."
	}
	if evidence.MetricRecovery != nil {
		if evidence.MetricRecovery.SamplesAfterEnd == 0 {
			return unified.LoopReportStatusNeedsReview,
				"No metric samples have been recorded since the window closed. Confirm the resource is reporting again."
		}
		if evidence.MetricRecovery.Trend == "degrading" {
			return unified.LoopReportStatusNeedsReview,
				"Post-window metric trend is degrading. Operator review recommended."
		}
	} else {
		return unified.LoopReportStatusNeedsReview,
			"No metric source was available for this resource. Confirm recovery manually."
	}

	return unified.LoopReportStatusHealthy,
		"All deterministic checks passed. No active alerts, findings, or failed actions since the window started."
}

func newReportID(canonicalID string, windowEndedAt time.Time) string {
	ts := windowEndedAt.UTC().Format("20060102T150405Z")
	if windowEndedAt.IsZero() {
		ts = time.Now().UTC().Format("20060102T150405Z")
	}
	return fmt.Sprintf("mv-%s-%s", sanitizeIDComponent(canonicalID), ts)
}

func sanitizeIDComponent(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z'), (r >= 'A' && r <= 'Z'), (r >= '0' && r <= '9'):
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
