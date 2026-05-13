package unifiedresources

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// LoopReportType is the discriminator that names the loop a report came
// from. Only one loop is implemented for the MVP — maintenance window
// verification — but the field stays in the contract so future loops can
// reuse the same persistence and review surfaces without a schema split.
type LoopReportType string

const (
	// LoopReportTypeMaintenanceVerification is the durable summary the
	// sentinel writes after a maintenance window ends for a resource. The
	// product-facing name is "Maintenance Verification Report"; this is
	// the internal type token used for storage and API filtering.
	LoopReportTypeMaintenanceVerification LoopReportType = "maintenance_verification"
)

const (
	MaintenanceVerificationActivityReported = "maintenance_verification_reported"

	maintenanceVerificationSourceAdapter ChangeSourceAdapter = "maintenance_sentinel"
)

// LoopReportStatus reports the verification outcome.
//
//   - healthy             — all deterministic checks passed.
//   - needs_review        — evidence is ambiguous; an operator should
//     look. Used when there is no metric history to compare, when the
//     resource is operator-marked intentionally offline, or when a
//     warning-level alert/finding is active.
//   - failed_verification — at least one strong negative signal
//     (critical alert, critical finding, recent failed action) is
//     present. The report does not auto-remediate; the operator
//     decides next steps.
//
// The set is intentionally small. Adding more states is a contract
// change because the UI and review action depend on the enum.
type LoopReportStatus string

const (
	LoopReportStatusPending            LoopReportStatus = "pending"
	LoopReportStatusHealthy            LoopReportStatus = "healthy"
	LoopReportStatusNeedsReview        LoopReportStatus = "needs_review"
	LoopReportStatusFailedVerification LoopReportStatus = "failed_verification"
)

// LoopReportUserOutcome records the operator's review verdict after
// reading a report. "" means the operator has not yet reviewed.
type LoopReportUserOutcome string

const (
	LoopReportUserOutcomeReviewed LoopReportUserOutcome = "reviewed"
)

// LoopReportEvidence captures the deterministic checks the sentinel
// ran. Each field is read-only operational evidence — counts and
// summaries — not executable commands or stale plans. The shape is
// stable so future review tooling can render every report uniformly.
type LoopReportEvidence struct {
	// OperatorStateSummary is a short human-readable summary of the
	// operator state at evaluation time (e.g.
	// "maintenance_window_ended", "intentionally_offline", "no
	// operator state").
	OperatorStateSummary string `json:"operatorStateSummary,omitempty"`

	// ActiveCriticalAlerts and ActiveWarningAlerts count alerts that
	// were active for this resource at evaluation time.
	ActiveCriticalAlerts int `json:"activeCriticalAlerts"`
	ActiveWarningAlerts  int `json:"activeWarningAlerts"`

	// ActiveCriticalFindings and ActiveWarningFindings count Patrol
	// findings active for the resource at evaluation time.
	ActiveCriticalFindings int `json:"activeCriticalFindings"`
	ActiveWarningFindings  int `json:"activeWarningFindings"`

	// FailedActionsSinceWindowStart is the number of action audits
	// targeting the resource whose state ended in "failed" with an
	// updated_at after the maintenance window started.
	FailedActionsSinceWindowStart int `json:"failedActionsSinceWindowStart"`

	// MetricRecovery describes the basic recent-metric check. Absent
	// when no metric source was available for the resource at
	// evaluation time.
	MetricRecovery *MetricRecoveryEvidence `json:"metricRecovery,omitempty"`

	// PatrolRunTODO is set when the deterministic evidence was
	// ambiguous and a scoped Patrol run would have helped, but
	// triggering one was not safe in this build. The value is the
	// reason string so the UI can surface what was missing.
	//
	// MVP: deterministic checks only; the Patrol run trigger is
	// deferred until the Patrol API surfaces a scoped per-resource
	// run entrypoint that does not race the global scheduler.
	PatrolRunTODO string `json:"patrolRunTodo,omitempty"`
}

// MetricRecoveryEvidence is a small summary of the metric values
// observed inside the maintenance window vs. immediately after it.
// "Recent metric recovery" is intentionally shallow for the MVP — a
// per-metric trend label rather than a model.
type MetricRecoveryEvidence struct {
	// MetricsObserved lists the metric short names the sentinel
	// inspected (e.g. "cpu", "memory").
	MetricsObserved []string `json:"metricsObserved,omitempty"`

	// SamplesAfterEnd is the number of metric samples observed after
	// the window ended. Zero means the resource has not reported any
	// metrics since the window closed — operator should look.
	SamplesAfterEnd int `json:"samplesAfterEnd"`

	// Trend describes the post-window metric trend in plain English.
	// Allowed values: "improving", "stable", "degrading", "unknown".
	Trend string `json:"trend,omitempty"`

	// Note is an optional explanation surfaced to the operator
	// alongside the trend.
	Note string `json:"note,omitempty"`
}

// LoopReport is the durable record the sentinel writes when a loop run
// produces an outcome an operator may want to review. Fields are
// minimal on purpose — this is shared substrate, not a generic agent
// framework.
//
// For the maintenance-verification loop:
//   - Scope is the canonical resource ID.
//   - Trigger is always "maintenance_window_end".
//   - WindowStartedAt and WindowEndedAt mirror the operator-set window
//     so future runs can de-duplicate against the (resource, window)
//     pair.
type LoopReport struct {
	ID                string                `json:"id"`
	Type              LoopReportType        `json:"type"`
	Scope             string                `json:"scope"`
	Trigger           string                `json:"trigger"`
	Goal              string                `json:"goal,omitempty"`
	Status            LoopReportStatus      `json:"status"`
	StartedAt         time.Time             `json:"startedAt"`
	CompletedAt       time.Time             `json:"completedAt"`
	WindowStartedAt   *time.Time            `json:"windowStartedAt,omitempty"`
	WindowEndedAt     *time.Time            `json:"windowEndedAt,omitempty"`
	Evidence          LoopReportEvidence    `json:"evidence"`
	LinkedFindingIDs  []string              `json:"linkedFindingIds,omitempty"`
	LinkedAlertIDs    []string              `json:"linkedAlertIds,omitempty"`
	LinkedActionIDs   []string              `json:"linkedActionIds,omitempty"`
	LinkedPatrolRunID string                `json:"linkedPatrolRunId,omitempty"`
	Recommendation    string                `json:"recommendation,omitempty"`
	UserOutcome       LoopReportUserOutcome `json:"userOutcome,omitempty"`
	ReviewedAt        *time.Time            `json:"reviewedAt,omitempty"`
	ReviewedBy        string                `json:"reviewedBy,omitempty"`
	ReviewNote        string                `json:"reviewNote,omitempty"`
}

// ErrLoopReportInvalid is returned when a record fails contract checks
// at the store boundary.
var ErrLoopReportInvalid = errors.New("loop_report_invalid")

// IsValidLoopReportType reports whether the value names a known loop
// report type. Unknown types are rejected at the store boundary so
// freeform discriminators cannot accumulate.
func IsValidLoopReportType(t LoopReportType) bool {
	switch t {
	case LoopReportTypeMaintenanceVerification:
		return true
	}
	return false
}

// IsValidLoopReportStatus reports whether the value names a known
// terminal or in-flight status.
func IsValidLoopReportStatus(s LoopReportStatus) bool {
	switch s {
	case LoopReportStatusPending,
		LoopReportStatusHealthy,
		LoopReportStatusNeedsReview,
		LoopReportStatusFailedVerification:
		return true
	}
	return false
}

// IsValidLoopReportUserOutcome reports whether the value names a known
// review verdict. Empty is valid — the operator has not reviewed yet.
func IsValidLoopReportUserOutcome(o LoopReportUserOutcome) bool {
	switch o {
	case "", LoopReportUserOutcomeReviewed:
		return true
	}
	return false
}

// NormalizeLoopReport trims string fields, canonicalizes the scope,
// and applies UTC to all timestamps. It does not validate — callers
// follow normalize with ValidateLoopReport.
func NormalizeLoopReport(r LoopReport) LoopReport {
	r.ID = strings.TrimSpace(r.ID)
	r.Scope = CanonicalResourceID(r.Scope)
	r.Trigger = strings.TrimSpace(r.Trigger)
	r.Goal = strings.TrimSpace(r.Goal)
	r.Recommendation = strings.TrimSpace(r.Recommendation)
	r.ReviewedBy = strings.TrimSpace(r.ReviewedBy)
	r.ReviewNote = strings.TrimSpace(r.ReviewNote)
	r.LinkedPatrolRunID = strings.TrimSpace(r.LinkedPatrolRunID)
	r.Evidence.OperatorStateSummary = strings.TrimSpace(r.Evidence.OperatorStateSummary)
	r.Evidence.PatrolRunTODO = strings.TrimSpace(r.Evidence.PatrolRunTODO)
	if r.Evidence.MetricRecovery != nil {
		r.Evidence.MetricRecovery.Trend = strings.TrimSpace(r.Evidence.MetricRecovery.Trend)
		r.Evidence.MetricRecovery.Note = strings.TrimSpace(r.Evidence.MetricRecovery.Note)
	}
	if !r.StartedAt.IsZero() {
		r.StartedAt = r.StartedAt.UTC()
	}
	if !r.CompletedAt.IsZero() {
		r.CompletedAt = r.CompletedAt.UTC()
	}
	if r.WindowStartedAt != nil {
		t := r.WindowStartedAt.UTC()
		r.WindowStartedAt = &t
	}
	if r.WindowEndedAt != nil {
		t := r.WindowEndedAt.UTC()
		r.WindowEndedAt = &t
	}
	if r.ReviewedAt != nil {
		t := r.ReviewedAt.UTC()
		r.ReviewedAt = &t
	}
	r.LinkedFindingIDs = trimAndDedupeStrings(r.LinkedFindingIDs)
	r.LinkedAlertIDs = trimAndDedupeStrings(r.LinkedAlertIDs)
	r.LinkedActionIDs = trimAndDedupeStrings(r.LinkedActionIDs)
	return r
}

// ValidateLoopReport applies the contract checks the persistence layer
// must enforce before writing a record. Returns an
// ErrLoopReportInvalid-wrapped error on violation.
func ValidateLoopReport(r LoopReport) error {
	if r.ID == "" {
		return fmt.Errorf("%w: id is required", ErrLoopReportInvalid)
	}
	if !IsValidLoopReportType(r.Type) {
		return fmt.Errorf("%w: unknown report type %q", ErrLoopReportInvalid, r.Type)
	}
	if r.Scope == "" {
		return fmt.Errorf("%w: scope (canonical resource id) is required", ErrLoopReportInvalid)
	}
	if r.Trigger == "" {
		return fmt.Errorf("%w: trigger is required", ErrLoopReportInvalid)
	}
	if !IsValidLoopReportStatus(r.Status) {
		return fmt.Errorf("%w: unknown status %q", ErrLoopReportInvalid, r.Status)
	}
	if r.StartedAt.IsZero() {
		return fmt.Errorf("%w: startedAt is required", ErrLoopReportInvalid)
	}
	if r.CompletedAt.IsZero() {
		return fmt.Errorf("%w: completedAt is required", ErrLoopReportInvalid)
	}
	if r.CompletedAt.Before(r.StartedAt) {
		return fmt.Errorf("%w: completedAt must not be before startedAt", ErrLoopReportInvalid)
	}
	if !IsValidLoopReportUserOutcome(r.UserOutcome) {
		return fmt.Errorf("%w: unknown user outcome %q", ErrLoopReportInvalid, r.UserOutcome)
	}
	return nil
}

func trimAndDedupeStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// BuildLoopReportResourceChange projects an immutable loop report into the
// canonical resource timeline. The report remains the source-of-truth record;
// this change is the discoverable timeline evidence that lets existing
// change consumers explain why the resource's post-maintenance state changed.
func BuildLoopReportResourceChange(report LoopReport) (ResourceChange, bool) {
	report = NormalizeLoopReport(report)
	if report.ID == "" || report.Scope == "" || report.Type != LoopReportTypeMaintenanceVerification {
		return ResourceChange{}, false
	}
	observedAt := report.CompletedAt
	if observedAt.IsZero() {
		observedAt = report.StartedAt
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	observedAt = observedAt.UTC()

	var occurredAt *time.Time
	if report.WindowEndedAt != nil {
		t := report.WindowEndedAt.UTC()
		occurredAt = &t
	}

	metadata := map[string]any{
		"activityType": MaintenanceVerificationActivityReported,
		"reportId":     report.ID,
		"reportType":   string(report.Type),
		"status":       string(report.Status),
		"trigger":      report.Trigger,
	}
	if report.WindowStartedAt != nil {
		metadata["windowStartedAt"] = report.WindowStartedAt.UTC().Format(time.RFC3339)
	}
	if report.WindowEndedAt != nil {
		metadata["windowEndedAt"] = report.WindowEndedAt.UTC().Format(time.RFC3339)
	}
	if report.Recommendation != "" {
		metadata["recommendation"] = report.Recommendation
	}
	if len(report.LinkedAlertIDs) > 0 {
		metadata["linkedAlertCount"] = len(report.LinkedAlertIDs)
	}
	if len(report.LinkedFindingIDs) > 0 {
		metadata["linkedFindingCount"] = len(report.LinkedFindingIDs)
	}
	if len(report.LinkedActionIDs) > 0 {
		metadata["linkedActionCount"] = len(report.LinkedActionIDs)
	}
	if report.Evidence.OperatorStateSummary != "" {
		metadata["operatorStateSummary"] = report.Evidence.OperatorStateSummary
	}

	return ResourceChange{
		ID:            resourceChangeID("loop-report", report.Scope, report.ID, observedAt),
		ObservedAt:    observedAt,
		OccurredAt:    occurredAt,
		ResourceID:    report.Scope,
		Kind:          ChangeActivity,
		From:          report.Trigger,
		To:            string(report.Status),
		SourceType:    SourceHeuristic,
		SourceAdapter: maintenanceVerificationSourceAdapter,
		Confidence:    ConfidenceHigh,
		Reason:        "Maintenance verification reported " + string(report.Status),
		Metadata:      metadata,
	}, true
}
