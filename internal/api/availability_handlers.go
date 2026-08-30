package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/tlsutil"
)

const availabilityTargetsPathPrefix = "/api/availability-targets/"

type AvailabilityHandlers struct {
	getPersistence  func(ctx context.Context) *config.ConfigPersistence
	getMonitor      func(ctx context.Context) *monitoring.Monitor
	licenseResolver licenseFeatureServiceResolver
}

type availabilityTargetResponse struct {
	config.AvailabilityTarget
	Status *monitoring.AvailabilityProbeStatus `json:"status,omitempty"`
}

type availabilityTestResponse struct {
	Success       bool                            `json:"success"`
	LatencyMillis int64                           `json:"latencyMillis"`
	Outcome       string                          `json:"outcome,omitempty"`
	Error         string                          `json:"error,omitempty"`
	Certificate   *tlsutil.CertificateObservation `json:"certificate,omitempty"`
}

func NewAvailabilityHandlers(
	getPersistence func(ctx context.Context) *config.ConfigPersistence,
	getMonitor func(ctx context.Context) *monitoring.Monitor,
	licenseResolver licenseFeatureServiceResolver,
) *AvailabilityHandlers {
	return &AvailabilityHandlers{
		getPersistence:  getPersistence,
		getMonitor:      getMonitor,
		licenseResolver: licenseResolver,
	}
}

// availabilityFeatureResolverFunc adapts a closure to the licensing
// FeatureServiceResolver so the router can resolve handlers that are
// constructed before the license handlers exist.
type availabilityFeatureResolverFunc func(ctx context.Context) licenseFeatureChecker

func (f availabilityFeatureResolverFunc) FeatureService(ctx context.Context) licenseFeatureChecker {
	if f == nil {
		return nil
	}
	return f(ctx)
}

// requireProbeAssignment gates only targets that carry a probe assignment.
// Unassigned targets are the community behaviour and must never consult the
// license path.
func (h *AvailabilityHandlers) requireProbeAssignment(w http.ResponseWriter, r *http.Request, target config.AvailabilityTarget) bool {
	if strings.TrimSpace(target.ProbeAgentID) == "" {
		return true
	}
	if h == nil || h.licenseResolver == nil {
		WriteLicenseRequired(w, featureExternalProbeValue, "license service unavailable")
		return false
	}
	service := h.licenseResolver.FeatureService(r.Context())
	if service == nil {
		WriteLicenseRequired(w, featureExternalProbeValue, "license service unavailable")
		return false
	}
	if err := service.RequireFeature(featureExternalProbeValue); err != nil {
		WriteLicenseRequired(w, featureExternalProbeValue, err.Error())
		return false
	}
	if !h.probeAgentExists(r.Context(), target.ProbeAgentID) {
		writeErrorResponse(w, http.StatusBadRequest, "unknown_probe_agent",
			"Probe agent "+target.ProbeAgentID+" is not a registered host agent", nil)
		return false
	}
	return true
}

func (h *AvailabilityHandlers) probeAgentExists(ctx context.Context, agentID string) bool {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return false
	}
	monitor := h.monitorForRequest(ctx)
	if monitor == nil {
		return false
	}
	for _, host := range monitor.GetLiveHostsSnapshot() {
		if strings.TrimSpace(host.ID) == agentID {
			return true
		}
	}
	return false
}

func (h *AvailabilityHandlers) HandleList(w http.ResponseWriter, r *http.Request) {
	if mock.IsMockEnabled() {
		writeJSON(w, http.StatusOK, mockAvailabilityTargetResponses())
		return
	}

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}
	targets, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "availability_load_failed", "Failed to load availability targets", map[string]string{"error": err.Error()})
		return
	}

	statuses := map[string]monitoring.AvailabilityProbeStatus{}
	if monitor := h.monitorForRequest(r.Context()); monitor != nil {
		statuses = monitor.AvailabilityStatusSnapshot()
	}
	responses := make([]availabilityTargetResponse, 0, len(targets))
	for _, target := range targets {
		response := availabilityTargetResponse{AvailabilityTarget: config.NormalizeAvailabilityTarget(target)}
		if status, ok := statuses[target.ID]; ok {
			statusCopy := status
			response.Status = &statusCopy
		}
		responses = append(responses, response)
	}
	writeJSON(w, http.StatusOK, responses)
}

func (h *AvailabilityHandlers) HandleAdd(w http.ResponseWriter, r *http.Request) {
	if mock.IsMockEnabled() {
		writeErrorResponse(w, http.StatusForbidden, "mock_mode_enabled", "Cannot modify connections in mock mode", nil)
		return
	}
	target, ok := decodeAvailabilityTargetRequest(w, r, config.NewAvailabilityTarget())
	if !ok {
		return
	}
	target = config.NormalizeAvailabilityTarget(target)
	target.ConfigRevision = 1
	if err := target.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}
	if !h.requireProbeAssignment(w, r, target) {
		return
	}

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}
	targets, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "availability_load_failed", "Failed to load availability targets", map[string]string{"error": err.Error()})
		return
	}
	for _, existing := range targets {
		if strings.TrimSpace(existing.ID) == target.ID {
			writeErrorResponse(w, http.StatusConflict, "availability_duplicate", "Availability target ID already exists", nil)
			return
		}
	}
	targets = append(targets, target)
	if err := persistence.SaveAvailabilityTargets(targets); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "availability_save_failed", "Failed to save availability targets", map[string]string{"error": err.Error()})
		return
	}
	h.refreshMonitor(r.Context())
	writeJSON(w, http.StatusCreated, target)
}

func (h *AvailabilityHandlers) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	if mock.IsMockEnabled() {
		writeErrorResponse(w, http.StatusForbidden, "mock_mode_enabled", "Cannot modify connections in mock mode", nil)
		return
	}
	targetID, ok := availabilityTargetIDFromPath(r.URL.Path)
	if !ok {
		writeErrorResponse(w, http.StatusBadRequest, "missing_target_id", "Availability target ID is required", nil)
		return
	}

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}
	targets, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "availability_load_failed", "Failed to load availability targets", map[string]string{"error": err.Error()})
		return
	}
	index := -1
	for i := range targets {
		if strings.TrimSpace(targets[i].ID) == targetID {
			index = i
			break
		}
	}
	if index < 0 {
		writeErrorResponse(w, http.StatusNotFound, "availability_not_found", "Availability target not found", nil)
		return
	}

	previous := config.NormalizeAvailabilityTarget(targets[index])
	target, ok := decodeAvailabilityTargetRequest(w, r, previous)
	if !ok {
		return
	}
	target.ID = targetID
	target = config.NormalizeAvailabilityTarget(target)
	target.ConfigRevision = previous.ConfigRevision
	if config.AvailabilityExecutionConfigChanged(previous, target) {
		target.ConfigRevision++
	}
	if err := target.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}
	if !h.requireProbeAssignment(w, r, target) {
		return
	}
	targets[index] = target
	if err := persistence.SaveAvailabilityTargets(targets); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "availability_save_failed", "Failed to save availability targets", map[string]string{"error": err.Error()})
		return
	}
	h.refreshMonitor(r.Context())
	writeJSON(w, http.StatusOK, target)
}

func (h *AvailabilityHandlers) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if mock.IsMockEnabled() {
		writeErrorResponse(w, http.StatusForbidden, "mock_mode_enabled", "Cannot modify connections in mock mode", nil)
		return
	}
	targetID, ok := availabilityTargetIDFromPath(r.URL.Path)
	if !ok {
		writeErrorResponse(w, http.StatusBadRequest, "missing_target_id", "Availability target ID is required", nil)
		return
	}

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}
	targets, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "availability_load_failed", "Failed to load availability targets", map[string]string{"error": err.Error()})
		return
	}
	index := -1
	for i := range targets {
		if strings.TrimSpace(targets[i].ID) == targetID {
			index = i
			break
		}
	}
	if index < 0 {
		writeErrorResponse(w, http.StatusNotFound, "availability_not_found", "Availability target not found", nil)
		return
	}
	targets = append(targets[:index], targets[index+1:]...)
	if err := persistence.SaveAvailabilityTargets(targets); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "availability_save_failed", "Failed to save availability targets", map[string]string{"error": err.Error()})
		return
	}
	if monitor := h.monitorForRequest(r.Context()); monitor != nil {
		if store := monitor.GetMetricsStore(); store != nil {
			store.DeleteAvailabilityTargetHistory(targetID)
		}
	}
	h.refreshMonitor(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "id": targetID})
}

func (h *AvailabilityHandlers) HandleTestConnection(w http.ResponseWriter, r *http.Request) {
	target, ok := decodeAvailabilityTargetRequest(w, r, config.NewAvailabilityTarget())
	if !ok {
		return
	}
	h.testTarget(w, r, target)
}

func (h *AvailabilityHandlers) HandleTestSavedConnection(w http.ResponseWriter, r *http.Request) {
	targetID, ok := availabilityTargetIDFromPath(strings.TrimSuffix(r.URL.Path, "/test"))
	if !ok {
		writeErrorResponse(w, http.StatusBadRequest, "missing_target_id", "Availability target ID is required", nil)
		return
	}
	if mock.IsMockEnabled() {
		if response, ok := mockAvailabilityTestResponse(targetID); ok {
			writeJSON(w, http.StatusOK, response)
			return
		}
		writeErrorResponse(w, http.StatusNotFound, "availability_not_found", "Availability target not found", nil)
		return
	}

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}
	targets, err := persistence.LoadAvailabilityTargets()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "availability_load_failed", "Failed to load availability targets", map[string]string{"error": err.Error()})
		return
	}
	for _, target := range targets {
		if strings.TrimSpace(target.ID) == targetID {
			h.testTarget(w, r, target)
			return
		}
	}
	writeErrorResponse(w, http.StatusNotFound, "availability_not_found", "Availability target not found", nil)
}

func (h *AvailabilityHandlers) testTarget(w http.ResponseWriter, r *http.Request, target config.AvailabilityTarget) {
	target = config.NormalizeAvailabilityTarget(target)
	if err := target.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}
	start := time.Now()
	result, err := monitoring.ProbeAvailabilityTargetDetailedResult(r.Context(), target)
	latencyMs := time.Since(start).Milliseconds()
	if err == nil && latencyMs == 0 {
		latencyMs = 1
	}
	response := availabilityTestResponse{
		Success:       err == nil,
		LatencyMillis: latencyMs,
		Outcome:       string(result.Outcome),
		Certificate:   result.Certificate.Clone(),
	}
	if err != nil {
		response.Error = err.Error()
	}
	writeJSON(w, http.StatusOK, response)
}

func decodeAvailabilityTargetRequest(w http.ResponseWriter, r *http.Request, base config.AvailabilityTarget) (config.AvailabilityTarget, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	defer r.Body.Close()
	target := base
	if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body", nil)
		return config.AvailabilityTarget{}, false
	}
	return target, true
}

func availabilityTargetIDFromPath(path string) (string, bool) {
	id := strings.TrimPrefix(path, availabilityTargetsPathPrefix)
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "/") {
		return "", false
	}
	return id, true
}

func (h *AvailabilityHandlers) persistenceForRequest(w http.ResponseWriter, ctx context.Context) *config.ConfigPersistence {
	if h == nil || h.getPersistence == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "availability_unavailable", "Availability target persistence is unavailable", nil)
		return nil
	}
	persistence := h.getPersistence(ctx)
	if persistence == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "availability_unavailable", "Availability target persistence is unavailable", nil)
		return nil
	}
	return persistence
}

func (h *AvailabilityHandlers) monitorForRequest(ctx context.Context) *monitoring.Monitor {
	if h == nil || h.getMonitor == nil {
		return nil
	}
	return h.getMonitor(ctx)
}

func (h *AvailabilityHandlers) refreshMonitor(ctx context.Context) {
	if monitor := h.monitorForRequest(ctx); monitor != nil {
		monitor.RefreshAvailabilityTargets()
	}
}
