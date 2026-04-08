package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// MonitoredSystemLedgerEntry represents a single counted top-level monitored
// system.
type MonitoredSystemLedgerEntry struct {
	Name                 string                                 `json:"name"`
	Type                 string                                 `json:"type"`
	Status               string                                 `json:"status"` // "online", "warning", "offline", "unknown"
	StatusExplanation    MonitoredSystemLedgerStatusExplanation `json:"status_explanation"`
	LatestIncludedSignal MonitoredSystemLedgerLatestSignal      `json:"latest_included_signal"`
	Source               string                                 `json:"source"`
	Explanation          MonitoredSystemLedgerExplanation       `json:"explanation"`
}

type MonitoredSystemLedgerLatestSignal struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Source string `json:"source"`
	At     string `json:"at"`
}

type MonitoredSystemLedgerStatusExplanation struct {
	Summary string                              `json:"summary"`
	Reasons []MonitoredSystemLedgerStatusReason `json:"reasons"`
}

type MonitoredSystemLedgerStatusReason struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Source     string `json:"source"`
	Status     string `json:"status"`
	ReportedAt string `json:"reported_at"`
	Summary    string `json:"summary"`
}

type MonitoredSystemLedgerExplanation struct {
	Summary  string                                    `json:"summary"`
	Reasons  []MonitoredSystemLedgerExplanationReason  `json:"reasons"`
	Surfaces []MonitoredSystemLedgerExplanationSurface `json:"surfaces"`
}

type MonitoredSystemLedgerExplanationReason struct {
	Kind    string `json:"kind"`
	Signal  string `json:"signal"`
	Summary string `json:"summary"`
}

type MonitoredSystemLedgerExplanationSurface struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Source string `json:"source"`
}

// MonitoredSystemLedgerResponse is the response for GET /api/license/monitored-system-ledger.
type MonitoredSystemLedgerResponse struct {
	Systems []MonitoredSystemLedgerEntry `json:"systems"`
	Total   int                          `json:"total"`
	Limit   int                          `json:"limit"` // 0 = unlimited
}

// MonitoredSystemLedgerExplainRequest describes an optional monitored-system
// preview to explain alongside the current counted ledger.
type MonitoredSystemLedgerExplainRequest struct {
	Candidate   *MonitoredSystemLedgerPreviewCandidate   `json:"candidate"`
	Replacement *MonitoredSystemLedgerPreviewReplacement `json:"replacement"`
}

// MonitoredSystemLedgerExplainResponse is the response for
// POST /api/license/monitored-system-ledger/explain.
type MonitoredSystemLedgerExplainResponse struct {
	Ledger  MonitoredSystemLedgerResponse         `json:"ledger"`
	Preview *MonitoredSystemLedgerPreviewResponse `json:"preview"`
}

// MonitoredSystemLedgerPreviewRequest describes one prospective monitored-system
// add or replacement to project through the canonical grouped ledger.
type MonitoredSystemLedgerPreviewRequest struct {
	Candidate   MonitoredSystemLedgerPreviewCandidate    `json:"candidate"`
	Replacement *MonitoredSystemLedgerPreviewReplacement `json:"replacement"`
}

type MonitoredSystemLedgerPreviewCandidate struct {
	Source     string `json:"source"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	Hostname   string `json:"hostname"`
	HostURL    string `json:"host_url"`
	AgentID    string `json:"agent_id"`
	MachineID  string `json:"machine_id"`
	ResourceID string `json:"resource_id"`
}

type MonitoredSystemLedgerPreviewReplacement struct {
	Source     string `json:"source"`
	Name       string `json:"name"`
	Hostname   string `json:"hostname"`
	HostURL    string `json:"host_url"`
	AgentID    string `json:"agent_id"`
	MachineID  string `json:"machine_id"`
	ResourceID string `json:"resource_id"`
}

// MonitoredSystemLedgerPreviewResponse is the response for
// POST /api/license/monitored-system-ledger/preview.
type MonitoredSystemLedgerPreviewResponse struct {
	CurrentCount     int                          `json:"current_count"`
	ProjectedCount   int                          `json:"projected_count"`
	AdditionalCount  int                          `json:"additional_count"`
	Limit            int                          `json:"limit"`
	WouldExceedLimit bool                         `json:"would_exceed_limit"`
	Effect           string                       `json:"effect"`
	CurrentSystems   []MonitoredSystemLedgerEntry `json:"current_systems"`
	ProjectedSystems []MonitoredSystemLedgerEntry `json:"projected_systems"`
	CurrentSystem    *MonitoredSystemLedgerEntry  `json:"current_system"`
	ProjectedSystem  *MonitoredSystemLedgerEntry  `json:"projected_system"`
}

func EmptyMonitoredSystemLedgerResponse() MonitoredSystemLedgerResponse {
	return MonitoredSystemLedgerResponse{}.NormalizeCollections()
}

func (r MonitoredSystemLedgerResponse) NormalizeCollections() MonitoredSystemLedgerResponse {
	if r.Systems == nil {
		r.Systems = []MonitoredSystemLedgerEntry{}
	}
	for i := range r.Systems {
		r.Systems[i] = r.Systems[i].NormalizeCollections()
	}
	return r
}

func (r MonitoredSystemLedgerExplainResponse) NormalizeCollections() MonitoredSystemLedgerExplainResponse {
	r.Ledger = r.Ledger.NormalizeCollections()
	if r.Preview != nil {
		preview := r.Preview.NormalizeCollections()
		r.Preview = &preview
	}
	return r
}

func (e MonitoredSystemLedgerEntry) NormalizeCollections() MonitoredSystemLedgerEntry {
	if e.LatestIncludedSignal.Name == "" {
		e.LatestIncludedSignal.Name = "Unnamed source"
	}
	if e.LatestIncludedSignal.Type == "" {
		e.LatestIncludedSignal.Type = "system"
	}
	if e.StatusExplanation.Reasons == nil {
		e.StatusExplanation.Reasons = []MonitoredSystemLedgerStatusReason{}
	}
	if e.Explanation.Reasons == nil {
		e.Explanation.Reasons = []MonitoredSystemLedgerExplanationReason{}
	}
	if e.Explanation.Surfaces == nil {
		e.Explanation.Surfaces = []MonitoredSystemLedgerExplanationSurface{}
	}
	return e
}

func (r *Router) handleMonitoredSystemLedger(w http.ResponseWriter, req *http.Request) {
	usage := monitoredSystemUsage(r.getTenantMonitor(req.Context()))
	if !usage.available {
		writeMonitoredSystemUsageUnavailable(w)
		return
	}

	resp := monitoredSystemLedgerResponseFromReadState(req.Context(), usage.readState)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp.NormalizeCollections())
}

func (r *Router) handleMonitoredSystemLedgerPreview(w http.ResponseWriter, req *http.Request) {
	var previewReq MonitoredSystemLedgerPreviewRequest
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&previewReq); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", map[string]string{"error": err.Error()})
		return
	}

	candidate, err := previewReq.Candidate.toUnifiedCandidate()
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	monitor := r.getTenantMonitor(req.Context())
	usage := monitoredSystemUsage(monitor)
	if !usage.available {
		writeMonitoredSystemUsageUnavailable(w)
		return
	}

	var preview unifiedresources.MonitoredSystemProjectionPreview
	if previewReq.Replacement != nil {
		replacement, err := previewReq.Replacement.toUnifiedReplacement(candidate.Source)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
			return
		}
		preview = unifiedresources.PreviewMonitoredSystemCandidateReplacement(usage.readState, replacement, candidate)
	} else {
		preview = unifiedresources.PreviewMonitoredSystemCandidate(usage.readState, candidate)
	}

	if len(preview.ProjectedSystems) == 0 {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Candidate did not resolve to a canonical monitored system preview", nil)
		return
	}

	resp := monitoredSystemLedgerPreviewResponse(req.Context(), previewReq.Replacement != nil, preview)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp.NormalizeCollections())
}

func (r *Router) handleMonitoredSystemLedgerExplain(w http.ResponseWriter, req *http.Request) {
	var explainReq MonitoredSystemLedgerExplainRequest
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&explainReq); err != nil && err != io.EOF {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", map[string]string{"error": err.Error()})
		return
	}
	if explainReq.Candidate == nil && explainReq.Replacement != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", "candidate is required when replacement is provided", nil)
		return
	}

	usage := monitoredSystemUsage(r.getTenantMonitor(req.Context()))
	if !usage.available {
		writeMonitoredSystemUsageUnavailable(w)
		return
	}

	resp := MonitoredSystemLedgerExplainResponse{
		Ledger: monitoredSystemLedgerResponseFromReadState(req.Context(), usage.readState),
	}

	if explainReq.Candidate != nil {
		candidate, err := explainReq.Candidate.toUnifiedCandidate()
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
			return
		}

		var preview unifiedresources.MonitoredSystemProjectionPreview
		if explainReq.Replacement != nil {
			replacement, err := explainReq.Replacement.toUnifiedReplacement(candidate.Source)
			if err != nil {
				writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
				return
			}
			preview = unifiedresources.PreviewMonitoredSystemCandidateReplacement(
				usage.readState,
				replacement,
				candidate,
			)
		} else {
			preview = unifiedresources.PreviewMonitoredSystemCandidate(usage.readState, candidate)
		}

		if len(preview.ProjectedSystems) == 0 {
			writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Candidate did not resolve to a canonical monitored system preview", nil)
			return
		}

		previewResp := monitoredSystemLedgerPreviewResponse(
			req.Context(),
			explainReq.Replacement != nil,
			preview,
		).NormalizeCollections()
		resp.Preview = &previewResp
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp.NormalizeCollections())
}

func (r MonitoredSystemLedgerPreviewResponse) NormalizeCollections() MonitoredSystemLedgerPreviewResponse {
	if r.CurrentSystems == nil {
		r.CurrentSystems = []MonitoredSystemLedgerEntry{}
	}
	if r.ProjectedSystems == nil {
		r.ProjectedSystems = []MonitoredSystemLedgerEntry{}
	}
	for i := range r.CurrentSystems {
		r.CurrentSystems[i] = r.CurrentSystems[i].NormalizeCollections()
	}
	for i := range r.ProjectedSystems {
		r.ProjectedSystems[i] = r.ProjectedSystems[i].NormalizeCollections()
	}
	if r.CurrentSystem != nil {
		entry := r.CurrentSystem.NormalizeCollections()
		r.CurrentSystem = &entry
	}
	if r.ProjectedSystem != nil {
		entry := r.ProjectedSystem.NormalizeCollections()
		r.ProjectedSystem = &entry
	}
	return r
}

func monitoredSystemLedgerEntry(system unifiedresources.MonitoredSystemRecord) MonitoredSystemLedgerEntry {
	status := normalizeStatus(string(system.Status))
	latestIncludedSignal := monitoredSystemLedgerLatestSignal(system.LatestIncludedSignal)
	return MonitoredSystemLedgerEntry{
		Name:                 system.Name,
		Type:                 system.Type,
		Status:               status,
		StatusExplanation:    monitoredSystemLedgerStatusExplanation(system.StatusExplanation, status),
		LatestIncludedSignal: latestIncludedSignal,
		Source:               system.Source,
		Explanation:          monitoredSystemLedgerExplanation(system.Explanation),
	}
}

func monitoredSystemLedgerEntryPointer(system *unifiedresources.MonitoredSystemRecord) *MonitoredSystemLedgerEntry {
	if system == nil {
		return nil
	}
	entry := monitoredSystemLedgerEntry(*system)
	return &entry
}

func monitoredSystemLedgerEntries(
	systems []unifiedresources.MonitoredSystemRecord,
) []MonitoredSystemLedgerEntry {
	if len(systems) == 0 {
		return []MonitoredSystemLedgerEntry{}
	}
	entries := make([]MonitoredSystemLedgerEntry, 0, len(systems))
	for _, system := range systems {
		entries = append(entries, monitoredSystemLedgerEntry(system))
	}
	return entries
}

func monitoredSystemLedgerResponseFromReadState(
	ctx context.Context,
	rs unifiedresources.ReadState,
) MonitoredSystemLedgerResponse {
	systems := unifiedresources.MonitoredSystems(rs)
	entries := monitoredSystemLedgerEntries(systems)
	return MonitoredSystemLedgerResponse{
		Systems: entries,
		Total:   len(entries),
		Limit:   maxMonitoredSystemsLimitForContext(ctx),
	}
}

func monitoredSystemLedgerPreviewResponse(
	ctx context.Context,
	hasReplacement bool,
	preview unifiedresources.MonitoredSystemProjectionPreview,
) MonitoredSystemLedgerPreviewResponse {
	limit := maxMonitoredSystemsLimitForContext(ctx)
	decision := monitoredSystemLimitDecisionFromAdditional(ctx, limit, preview.CurrentCount, preview.AdditionalCount)
	currentSystems := monitoredSystemLedgerEntries(preview.CurrentSystems)
	projectedSystems := monitoredSystemLedgerEntries(preview.ProjectedSystems)

	resp := MonitoredSystemLedgerPreviewResponse{
		CurrentCount:     preview.CurrentCount,
		ProjectedCount:   preview.ProjectedCount,
		AdditionalCount:  preview.AdditionalCount,
		Limit:            limit,
		WouldExceedLimit: decision.exceeded,
		Effect:           monitoredSystemLedgerPreviewEffect(hasReplacement, preview),
		CurrentSystems:   currentSystems,
		ProjectedSystems: projectedSystems,
	}
	if len(currentSystems) == 1 {
		entry := currentSystems[0]
		resp.CurrentSystem = &entry
	}
	if len(projectedSystems) == 1 {
		entry := projectedSystems[0]
		resp.ProjectedSystem = &entry
	}
	return resp
}

// ---------------------------------------------------------------------------
// Status helpers
// ---------------------------------------------------------------------------

func normalizeStatus(s string) string {
	switch s {
	case "online", "warning", "offline", "unknown":
		return s
	default:
		return "unknown"
	}
}

func monitoredSystemLedgerStatusExplanation(
	explanation unifiedresources.MonitoredSystemStatusExplanation,
	status string,
) MonitoredSystemLedgerStatusExplanation {
	reasons := make([]MonitoredSystemLedgerStatusReason, 0, len(explanation.Reasons))
	for _, reason := range explanation.Reasons {
		reasons = append(reasons, MonitoredSystemLedgerStatusReason{
			Kind:       reason.Kind,
			Name:       reason.Name,
			Type:       reason.Type,
			Source:     reason.Source,
			Status:     normalizeMonitoredSystemLedgerReasonStatus(reason.Status),
			ReportedAt: formatMonitoredSystemTime(reason.ReportedAt),
			Summary:    reason.Summary,
		})
	}

	summary := explanation.Summary
	if summary == "" {
		summary = defaultMonitoredSystemLedgerStatusSummary(status)
	}

	return MonitoredSystemLedgerStatusExplanation{
		Summary: summary,
		Reasons: reasons,
	}
}

func defaultMonitoredSystemLedgerStatusSummary(status string) string {
	switch status {
	case "online":
		return "All included top-level collection paths currently report online status."
	case "warning":
		return "At least one included top-level collection path is degraded, so Pulse marks this monitored system as warning."
	case "offline":
		return "At least one included source is offline or disconnected, so Pulse marks this monitored system as offline."
	default:
		return "Pulse cannot determine a canonical runtime status for this monitored system yet."
	}
}

func normalizeMonitoredSystemLedgerReasonStatus(status string) string {
	switch status {
	case "online", "stale", "offline", "unknown":
		return status
	default:
		return "unknown"
	}
}

func normalizeMonitoredSystemLedgerSource(source string) string {
	switch source {
	case "agent", "docker", "kubernetes", "pbs", "pmg", "proxmox", "truenas", "vmware":
		return source
	default:
		return ""
	}
}

func monitoredSystemLedgerLatestSignal(
	signal unifiedresources.MonitoredSystemLatestSignal,
) MonitoredSystemLedgerLatestSignal {
	return MonitoredSystemLedgerLatestSignal{
		Name:   signal.Name,
		Type:   signal.Type,
		Source: normalizeMonitoredSystemLedgerSource(signal.Source),
		At:     formatMonitoredSystemTime(signal.At),
	}
}

func formatMonitoredSystemTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func monitoredSystemLedgerExplanation(
	explanation unifiedresources.MonitoredSystemGroupingExplanation,
) MonitoredSystemLedgerExplanation {
	reasons := make([]MonitoredSystemLedgerExplanationReason, 0, len(explanation.Reasons))
	for _, reason := range explanation.Reasons {
		reasons = append(reasons, MonitoredSystemLedgerExplanationReason{
			Kind:    reason.Kind,
			Signal:  reason.Signal,
			Summary: reason.Summary,
		})
	}

	surfaces := make([]MonitoredSystemLedgerExplanationSurface, 0, len(explanation.Surfaces))
	for _, surface := range explanation.Surfaces {
		surfaces = append(surfaces, MonitoredSystemLedgerExplanationSurface{
			Name:   surface.Name,
			Type:   surface.Type,
			Source: surface.Source,
		})
	}

	return MonitoredSystemLedgerExplanation{
		Summary:  explanation.Summary,
		Reasons:  reasons,
		Surfaces: surfaces,
	}
}

func (c MonitoredSystemLedgerPreviewCandidate) toUnifiedCandidate() (unifiedresources.MonitoredSystemCandidate, error) {
	source := normalizedMonitoredSystemLedgerDataSource(c.Source)
	if source == "" {
		return unifiedresources.MonitoredSystemCandidate{}, errMonitoredSystemLedgerValidation("candidate.source is required")
	}
	if strings.TrimSpace(c.Name) == "" &&
		strings.TrimSpace(c.Hostname) == "" &&
		strings.TrimSpace(c.HostURL) == "" &&
		strings.TrimSpace(c.ResourceID) == "" &&
		strings.TrimSpace(c.AgentID) == "" &&
		strings.TrimSpace(c.MachineID) == "" {
		return unifiedresources.MonitoredSystemCandidate{}, errMonitoredSystemLedgerValidation("candidate must include at least one canonical identifier or display field")
	}

	return unifiedresources.MonitoredSystemCandidate{
		Source:     source,
		Type:       unifiedresources.ResourceType(strings.TrimSpace(c.Type)),
		Name:       strings.TrimSpace(c.Name),
		Hostname:   strings.TrimSpace(c.Hostname),
		HostURL:    strings.TrimSpace(c.HostURL),
		AgentID:    strings.TrimSpace(c.AgentID),
		MachineID:  strings.TrimSpace(c.MachineID),
		ResourceID: strings.TrimSpace(c.ResourceID),
	}, nil
}

func (r MonitoredSystemLedgerPreviewReplacement) toUnifiedReplacement(
	fallbackSource unifiedresources.DataSource,
) (unifiedresources.MonitoredSystemReplacement, error) {
	source := normalizedMonitoredSystemLedgerDataSource(r.Source)
	if source == "" {
		source = fallbackSource
	}
	if source == "" {
		return unifiedresources.MonitoredSystemReplacement{}, errMonitoredSystemLedgerValidation("replacement.source is required")
	}
	if strings.TrimSpace(r.Name) == "" &&
		strings.TrimSpace(r.Hostname) == "" &&
		strings.TrimSpace(r.HostURL) == "" &&
		strings.TrimSpace(r.ResourceID) == "" &&
		strings.TrimSpace(r.AgentID) == "" &&
		strings.TrimSpace(r.MachineID) == "" {
		return unifiedresources.MonitoredSystemReplacement{}, errMonitoredSystemLedgerValidation("replacement must include at least one canonical selector field")
	}

	return unifiedresources.MonitoredSystemReplacement{
		Source: source,
		Selector: unifiedresources.MonitoredSystemReplacementSelector{
			Name:       strings.TrimSpace(r.Name),
			Hostname:   strings.TrimSpace(r.Hostname),
			HostURL:    strings.TrimSpace(r.HostURL),
			AgentID:    strings.TrimSpace(r.AgentID),
			MachineID:  strings.TrimSpace(r.MachineID),
			ResourceID: strings.TrimSpace(r.ResourceID),
		},
	}, nil
}

func normalizedMonitoredSystemLedgerDataSource(source string) unifiedresources.DataSource {
	normalized := normalizeMonitoredSystemLedgerSource(strings.ToLower(strings.TrimSpace(source)))
	if normalized == "" {
		return ""
	}
	return unifiedresources.DataSource(normalized)
}

func monitoredSystemLedgerPreviewEffect(
	hasReplacement bool,
	preview unifiedresources.MonitoredSystemProjectionPreview,
) string {
	currentCount := len(preview.CurrentSystems)
	projectedCount := len(preview.ProjectedSystems)

	if !hasReplacement {
		if preview.AdditionalCount == 0 && currentCount > 0 {
			return "attaches_existing"
		}
		if projectedCount > 1 && currentCount > 0 {
			return "mixed_existing_and_new"
		}
		if projectedCount > 1 {
			return "creates_multiple"
		}
		return "creates_new"
	}
	if currentCount == 0 {
		if projectedCount > 1 {
			return "creates_multiple"
		}
		return "creates_new"
	}
	if preview.AdditionalCount > 0 {
		return "splits_existing"
	}
	if currentCount > 1 || projectedCount > 1 {
		return "replaces_multiple"
	}
	return "replaces_existing"
}

func errMonitoredSystemLedgerValidation(message string) error {
	return monitoredSystemLedgerValidationError(message)
}

type monitoredSystemLedgerValidationError string

func (e monitoredSystemLedgerValidationError) Error() string {
	return string(e)
}
