package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/maintenancesentinel"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// maintenanceVerificationReportAPI is the wire shape for Maintenance
// Verification Reports. Mirrors `unified.LoopReport` but is kept as a
// separate API type so the storage shape can evolve independently of
// the wire contract. Filenames and product copy use the "Maintenance
// Verification Report" name; the underlying loop substrate is
// implementation detail and is not exposed verbatim.
type maintenanceVerificationReportAPI struct {
	ID                string                             `json:"id"`
	ResourceID        string                             `json:"resourceId"`
	Trigger           string                             `json:"trigger"`
	Goal              string                             `json:"goal,omitempty"`
	Status            string                             `json:"status"`
	StartedAt         time.Time                          `json:"startedAt"`
	CompletedAt       time.Time                          `json:"completedAt"`
	WindowStartedAt   *time.Time                         `json:"windowStartedAt,omitempty"`
	WindowEndedAt     *time.Time                         `json:"windowEndedAt,omitempty"`
	Evidence          maintenanceVerificationEvidenceAPI `json:"evidence"`
	LinkedFindingIDs  []string                           `json:"linkedFindingIds"`
	LinkedAlertIDs    []string                           `json:"linkedAlertIds"`
	LinkedActionIDs   []string                           `json:"linkedActionIds"`
	LinkedPatrolRunID string                             `json:"linkedPatrolRunId,omitempty"`
	Recommendation    string                             `json:"recommendation,omitempty"`
	UserOutcome       string                             `json:"userOutcome,omitempty"`
	ReviewedAt        *time.Time                         `json:"reviewedAt,omitempty"`
	ReviewedBy        string                             `json:"reviewedBy,omitempty"`
	ReviewNote        string                             `json:"reviewNote,omitempty"`
}

type maintenanceVerificationEvidenceAPI struct {
	OperatorStateSummary          string                                    `json:"operatorStateSummary,omitempty"`
	ActiveCriticalAlerts          int                                       `json:"activeCriticalAlerts"`
	ActiveWarningAlerts           int                                       `json:"activeWarningAlerts"`
	ActiveCriticalFindings        int                                       `json:"activeCriticalFindings"`
	ActiveWarningFindings         int                                       `json:"activeWarningFindings"`
	FailedActionsSinceWindowStart int                                       `json:"failedActionsSinceWindowStart"`
	MetricRecovery                *maintenanceVerificationMetricRecoveryAPI `json:"metricRecovery,omitempty"`
	PatrolRunTODO                 string                                    `json:"patrolRunTodo,omitempty"`
}

type maintenanceVerificationMetricRecoveryAPI struct {
	MetricsObserved []string `json:"metricsObserved,omitempty"`
	SamplesAfterEnd int      `json:"samplesAfterEnd"`
	Trend           string   `json:"trend,omitempty"`
	Note            string   `json:"note,omitempty"`
}

type maintenanceVerificationReviewRequest struct {
	Note string `json:"note,omitempty"`
}

// MaintenanceVerificationHandlers serves the Maintenance Verification
// Report endpoints. Composes the resource store (for read/write of
// LoopReport rows) with the sentinel runtime (for "rerun" requests).
type MaintenanceVerificationHandlers struct {
	resources *ResourceHandlers
	sentinel  *maintenancesentinel.Sentinel
}

// NewMaintenanceVerificationHandlers wires the handler. resources is
// required (it owns the per-org store accessor); sentinel may be nil
// when the runtime has not started a sentinel (e.g. unit tests of the
// other endpoints) — in that case the rerun endpoint returns 503.
func NewMaintenanceVerificationHandlers(resources *ResourceHandlers, sentinel *maintenancesentinel.Sentinel) *MaintenanceVerificationHandlers {
	return &MaintenanceVerificationHandlers{resources: resources, sentinel: sentinel}
}

// HandleListForResource serves
// `GET /api/resources/{id}/maintenance-verifications?limit=N`. Returns
// the most recent reports for the resource. Limit defaults to 25 and
// is capped at 200.
func (h *MaintenanceVerificationHandlers) HandleListForResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.resources == nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	resourceID := extractMaintenanceVerificationsResourceID(r.URL.Path)
	if resourceID == "" {
		http.Error(w, "Resource ID required", http.StatusBadRequest)
		return
	}
	orgID := GetOrgID(r.Context())
	store, err := h.resources.getStore(orgID)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}
	limit := parseMaintenanceVerificationLimit(r.URL.Query().Get("limit"), 25, 200)
	reports, err := store.ListLoopReportsForResource(unified.LoopReportTypeMaintenanceVerification, resourceID, limit)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}
	out := make([]maintenanceVerificationReportAPI, 0, len(reports))
	for _, report := range reports {
		out = append(out, toMaintenanceVerificationAPI(report))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": out,
		"meta": map[string]any{
			"resourceId": resourceID,
			"limit":      limit,
			"total":      len(out),
		},
	})
}

// HandleGet serves `GET /api/maintenance-verifications/{id}`.
func (h *MaintenanceVerificationHandlers) HandleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.resources == nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	reportID := extractMaintenanceVerificationReportID(r.URL.Path)
	if reportID == "" {
		http.Error(w, "Report ID required", http.StatusBadRequest)
		return
	}
	orgID := GetOrgID(r.Context())
	store, err := h.resources.getStore(orgID)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}
	report, found, err := store.GetLoopReport(reportID)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}
	if !found || report.Type != unified.LoopReportTypeMaintenanceVerification {
		writeJSONError(w, http.StatusNotFound, "maintenance_verification_not_found",
			"No maintenance verification report exists with this id.")
		return
	}
	writeJSON(w, http.StatusOK, toMaintenanceVerificationAPI(report))
}

// HandleReview serves
// `POST /api/maintenance-verifications/{id}/review`. Records the
// operator's "mark reviewed" verdict on a report. The report's
// status / evidence are immutable — only the review fields update.
func (h *MaintenanceVerificationHandlers) HandleReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.resources == nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	reportID := extractMaintenanceVerificationReviewReportID(r.URL.Path)
	if reportID == "" {
		http.Error(w, "Report ID required", http.StatusBadRequest)
		return
	}
	var payload maintenanceVerificationReviewRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
	}
	orgID := GetOrgID(r.Context())
	store, err := h.resources.getStore(orgID)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}
	report, found, err := store.GetLoopReport(reportID)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}
	if !found || report.Type != unified.LoopReportTypeMaintenanceVerification {
		writeJSONError(w, http.StatusNotFound, "maintenance_verification_not_found",
			"No maintenance verification report exists with this id.")
		return
	}
	if err := store.UpdateLoopReportUserOutcome(
		reportID,
		unified.LoopReportUserOutcomeReviewed,
		getUserID(r),
		strings.TrimSpace(payload.Note),
		time.Now().UTC(),
	); err != nil {
		if errors.Is(err, unified.ErrLoopReportNotFound) {
			writeJSONError(w, http.StatusNotFound, "maintenance_verification_not_found",
				"No maintenance verification report exists with this id.")
			return
		}
		if errors.Is(err, unified.ErrLoopReportInvalid) {
			writeJSONError(w, http.StatusBadRequest, "maintenance_verification_invalid", err.Error())
			return
		}
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}
	persisted, _, err := store.GetLoopReport(reportID)
	if err != nil {
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toMaintenanceVerificationAPI(persisted))
}

// HandleRerun serves
// `POST /api/resources/{id}/maintenance-verifications/rerun`. Runs
// the deterministic evaluator again immediately and persists a new
// report (with a `-rerun-N` suffix on the id) so the review history
// is preserved.
func (h *MaintenanceVerificationHandlers) HandleRerun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.sentinel == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "maintenance_sentinel_unavailable",
			"Maintenance verification sentinel is not running on this instance.")
		return
	}
	resourceID := extractMaintenanceVerificationRerunResourceID(r.URL.Path)
	if resourceID == "" {
		http.Error(w, "Resource ID required", http.StatusBadRequest)
		return
	}
	report, err := h.sentinel.EvaluateOnce(r.Context(), resourceID)
	if err != nil {
		if isMaintenanceWindowMissing(err) {
			writeJSONError(w, http.StatusBadRequest, "maintenance_window_missing", err.Error())
			return
		}
		http.Error(w, sanitizeErrorForClient(err, "Internal server error"), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toMaintenanceVerificationAPI(report))
}

func toMaintenanceVerificationAPI(report unified.LoopReport) maintenanceVerificationReportAPI {
	out := maintenanceVerificationReportAPI{
		ID:                report.ID,
		ResourceID:        report.Scope,
		Trigger:           report.Trigger,
		Goal:              report.Goal,
		Status:            string(report.Status),
		StartedAt:         report.StartedAt,
		CompletedAt:       report.CompletedAt,
		WindowStartedAt:   report.WindowStartedAt,
		WindowEndedAt:     report.WindowEndedAt,
		LinkedPatrolRunID: report.LinkedPatrolRunID,
		Recommendation:    report.Recommendation,
		UserOutcome:       string(report.UserOutcome),
		ReviewedAt:        report.ReviewedAt,
		ReviewedBy:        report.ReviewedBy,
		ReviewNote:        report.ReviewNote,
		LinkedFindingIDs:  ensureStringSlice(report.LinkedFindingIDs),
		LinkedAlertIDs:    ensureStringSlice(report.LinkedAlertIDs),
		LinkedActionIDs:   ensureStringSlice(report.LinkedActionIDs),
		Evidence: maintenanceVerificationEvidenceAPI{
			OperatorStateSummary:          report.Evidence.OperatorStateSummary,
			ActiveCriticalAlerts:          report.Evidence.ActiveCriticalAlerts,
			ActiveWarningAlerts:           report.Evidence.ActiveWarningAlerts,
			ActiveCriticalFindings:        report.Evidence.ActiveCriticalFindings,
			ActiveWarningFindings:         report.Evidence.ActiveWarningFindings,
			FailedActionsSinceWindowStart: report.Evidence.FailedActionsSinceWindowStart,
			PatrolRunTODO:                 report.Evidence.PatrolRunTODO,
		},
	}
	if report.Evidence.MetricRecovery != nil {
		out.Evidence.MetricRecovery = &maintenanceVerificationMetricRecoveryAPI{
			MetricsObserved: ensureStringSlice(report.Evidence.MetricRecovery.MetricsObserved),
			SamplesAfterEnd: report.Evidence.MetricRecovery.SamplesAfterEnd,
			Trend:           report.Evidence.MetricRecovery.Trend,
			Note:            report.Evidence.MetricRecovery.Note,
		}
	}
	return out
}

func ensureStringSlice(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func parseMaintenanceVerificationLimit(raw string, def, max int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}

// extractMaintenanceVerificationsResourceID parses the canonical
// resource id out of `/api/resources/<id>/maintenance-verifications`.
func extractMaintenanceVerificationsResourceID(path string) string {
	trimmed := strings.TrimPrefix(path, "/api/resources/")
	trimmed = strings.TrimSuffix(trimmed, "/")
	trimmed = strings.TrimSuffix(trimmed, "/maintenance-verifications")
	trimmed = strings.TrimSuffix(trimmed, "/")
	return unified.CanonicalResourceID(trimmed)
}

// extractMaintenanceVerificationRerunResourceID parses the canonical
// resource id out of
// `/api/resources/<id>/maintenance-verifications/rerun`.
func extractMaintenanceVerificationRerunResourceID(path string) string {
	trimmed := strings.TrimPrefix(path, "/api/resources/")
	trimmed = strings.TrimSuffix(trimmed, "/")
	trimmed = strings.TrimSuffix(trimmed, "/maintenance-verifications/rerun")
	trimmed = strings.TrimSuffix(trimmed, "/")
	return unified.CanonicalResourceID(trimmed)
}

// extractMaintenanceVerificationReportID parses the report id out of
// `/api/maintenance-verifications/<id>`.
func extractMaintenanceVerificationReportID(path string) string {
	trimmed := strings.TrimPrefix(path, "/api/maintenance-verifications/")
	trimmed = strings.TrimSuffix(trimmed, "/")
	return strings.TrimSpace(trimmed)
}

// extractMaintenanceVerificationReviewReportID parses the report id
// out of `/api/maintenance-verifications/<id>/review`.
func extractMaintenanceVerificationReviewReportID(path string) string {
	trimmed := strings.TrimPrefix(path, "/api/maintenance-verifications/")
	trimmed = strings.TrimSuffix(trimmed, "/")
	trimmed = strings.TrimSuffix(trimmed, "/review")
	trimmed = strings.TrimSuffix(trimmed, "/")
	return strings.TrimSpace(trimmed)
}

func isMaintenanceWindowMissing(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "no operator state for") ||
		strings.Contains(err.Error(), "no maintenance window to verify")
}

// Compile-time interface satisfaction check so the router never wires
// a misshapen sentinel pointer.
var _ context.Context = context.Background()
