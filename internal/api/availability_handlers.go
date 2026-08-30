package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/availabilityprobe"
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
	Status      *monitoring.AvailabilityProbeStatus `json:"status,omitempty"`
	HTTPSecrets *availabilityHTTPSecretState        `json:"httpSecrets,omitempty"`
}

type availabilityHTTPSecretState struct {
	BodyConfigured        bool                                `json:"bodyConfigured"`
	PasswordConfigured    bool                                `json:"passwordConfigured"`
	BearerTokenConfigured bool                                `json:"bearerTokenConfigured"`
	Headers               []availabilityHTTPHeaderSecretState `json:"headers,omitempty"`
}

type availabilityHTTPHeaderSecretState struct {
	ID              string `json:"id"`
	ValueConfigured bool   `json:"valueConfigured"`
}

type availabilityTestResponse struct {
	Success          bool                                 `json:"success"`
	LatencyMillis    int64                                `json:"latencyMillis"`
	Outcome          string                               `json:"outcome,omitempty"`
	TransportOutcome string                               `json:"transportOutcome,omitempty"`
	Application      *availabilityprobe.ApplicationResult `json:"application,omitempty"`
	Error            string                               `json:"error,omitempty"`
	Certificate      *tlsutil.CertificateObservation      `json:"certificate,omitempty"`
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
	agentIDs := target.AssignedProbeAgentIDs()
	if len(agentIDs) == 0 {
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
	for _, agentID := range agentIDs {
		if !h.probeAgentExists(r.Context(), agentID) {
			writeErrorResponse(w, http.StatusBadRequest, "unknown_probe_agent",
				"Observation location agent "+agentID+" is not a registered host agent", nil)
			return false
		}
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
		response := availabilityTargetAPIResponse(target)
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
	writeJSON(w, http.StatusCreated, availabilityTargetAPIResponse(target))
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
	target, ok := decodeAvailabilityTargetRequest(w, r, availabilityTargetWithoutHTTPSecrets(previous))
	if !ok {
		return
	}
	target.ID = targetID
	target = mergeAvailabilityHTTPSecrets(previous, target)
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
	writeJSON(w, http.StatusOK, availabilityTargetAPIResponse(target))
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
	if strings.TrimSpace(target.ID) != "" {
		if h != nil && h.getPersistence != nil {
			if persistence := h.getPersistence(r.Context()); persistence != nil {
				if targets, err := persistence.LoadAvailabilityTargets(); err == nil {
					for _, saved := range targets {
						if strings.TrimSpace(saved.ID) == strings.TrimSpace(target.ID) {
							target = mergeAvailabilityHTTPSecrets(config.NormalizeAvailabilityTarget(saved), target)
							break
						}
					}
				}
			}
		}
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
		Success:          err == nil,
		LatencyMillis:    latencyMs,
		Outcome:          string(result.Outcome),
		TransportOutcome: string(result.TransportOutcome),
		Application:      result.Application,
		Certificate:      result.Certificate.Clone(),
	}
	if err != nil {
		response.Error = err.Error()
	}
	writeJSON(w, http.StatusOK, response)
}

func decodeAvailabilityTargetRequest(w http.ResponseWriter, r *http.Request, base config.AvailabilityTarget) (config.AvailabilityTarget, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	defer r.Body.Close()
	target := cloneAvailabilityTarget(base)
	body, err := io.ReadAll(r.Body)
	if err != nil || json.Unmarshal(body, &target) != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body", nil)
		return config.AvailabilityTarget{}, false
	}
	var compatibility struct {
		ProbeAgentID           *string   `json:"probeAgentId"`
		ObservationLocationIDs *[]string `json:"observationLocationIds"`
	}
	if json.Unmarshal(body, &compatibility) == nil && compatibility.ObservationLocationIDs == nil && compatibility.ProbeAgentID != nil {
		agentID := strings.TrimSpace(*compatibility.ProbeAgentID)
		if agentID == "" {
			target.ObservationLocationIDs = []string{config.AvailabilityObservationLocationLocal}
		} else {
			target.ObservationLocationIDs = []string{config.AvailabilityAgentObservationLocationID(agentID)}
		}
	} else if compatibility.ProbeAgentID != nil && compatibility.ObservationLocationIDs != nil {
		agentID := strings.TrimSpace(*compatibility.ProbeAgentID)
		if agentID != "" && len(*compatibility.ObservationLocationIDs) == 1 {
			// Full-object pre-location clients may echo the canonical field added by
			// the server while editing only probeAgentId. Honor that single-source
			// edit; multi-location clients send a set the legacy field cannot express.
			target.ObservationLocationIDs = []string{config.AvailabilityAgentObservationLocationID(agentID)}
		}
	}
	return target, true
}

func availabilityTargetAPIResponse(target config.AvailabilityTarget) availabilityTargetResponse {
	target = config.NormalizeAvailabilityTarget(target)
	response := availabilityTargetResponse{AvailabilityTarget: cloneAvailabilityTarget(target)}
	if target.HTTP == nil {
		return response
	}
	state := &availabilityHTTPSecretState{
		BodyConfigured:        target.HTTP.Body != nil && *target.HTTP.Body != "",
		PasswordConfigured:    target.HTTP.Authentication.Password != nil && *target.HTTP.Authentication.Password != "",
		BearerTokenConfigured: target.HTTP.Authentication.BearerToken != nil && *target.HTTP.Authentication.BearerToken != "",
	}
	response.HTTP.Body = nil
	response.HTTP.Authentication.Password = nil
	response.HTTP.Authentication.BearerToken = nil
	for i := range response.HTTP.Headers {
		configured := target.HTTP.Headers[i].Value != nil && *target.HTTP.Headers[i].Value != ""
		state.Headers = append(state.Headers, availabilityHTTPHeaderSecretState{
			ID: target.HTTP.Headers[i].ID, ValueConfigured: configured,
		})
		response.HTTP.Headers[i].Value = nil
	}
	response.HTTPSecrets = state
	return response
}

func availabilityTargetWithoutHTTPSecrets(target config.AvailabilityTarget) config.AvailabilityTarget {
	target = cloneAvailabilityTarget(target)
	if target.HTTP == nil {
		return target
	}
	target.HTTP.Body = nil
	target.HTTP.Authentication.Password = nil
	target.HTTP.Authentication.BearerToken = nil
	for i := range target.HTTP.Headers {
		target.HTTP.Headers[i].Value = nil
	}
	return target
}

func mergeAvailabilityHTTPSecrets(previous, next config.AvailabilityTarget) config.AvailabilityTarget {
	previous = config.NormalizeAvailabilityTarget(previous)
	next = cloneAvailabilityTarget(next)
	if previous.HTTP == nil || next.HTTP == nil {
		return next
	}
	// Write-only values may be reused while editing the same endpoint, but must
	// never follow a changed origin. Otherwise an address edit or unsaved test
	// could silently replay a stored credential to another server.
	if !sameAvailabilityHTTPOrigin(previous, next) {
		return next
	}
	if next.HTTP.Body == nil {
		next.HTTP.Body = cloneStringPointer(previous.HTTP.Body)
	}
	if next.HTTP.Authentication.Password == nil && next.HTTP.Authentication.Type == previous.HTTP.Authentication.Type {
		next.HTTP.Authentication.Password = cloneStringPointer(previous.HTTP.Authentication.Password)
	}
	if next.HTTP.Authentication.BearerToken == nil && next.HTTP.Authentication.Type == previous.HTTP.Authentication.Type {
		next.HTTP.Authentication.BearerToken = cloneStringPointer(previous.HTTP.Authentication.BearerToken)
	}
	previousHeaders := make(map[string]*string, len(previous.HTTP.Headers))
	for _, header := range previous.HTTP.Headers {
		previousHeaders[header.ID] = header.Value
	}
	for i := range next.HTTP.Headers {
		if next.HTTP.Headers[i].Value == nil {
			next.HTTP.Headers[i].Value = cloneStringPointer(previousHeaders[next.HTTP.Headers[i].ID])
		}
	}
	return next
}

func sameAvailabilityHTTPOrigin(previous, next config.AvailabilityTarget) bool {
	previousURL, err := previous.HTTPURL()
	if err != nil {
		return false
	}
	nextURL, err := next.HTTPURL()
	if err != nil {
		return false
	}
	previousPort := previousURL.Port()
	if previousPort == "" {
		previousPort = map[string]string{"http": "80", "https": "443"}[strings.ToLower(previousURL.Scheme)]
	}
	nextPort := nextURL.Port()
	if nextPort == "" {
		nextPort = map[string]string{"http": "80", "https": "443"}[strings.ToLower(nextURL.Scheme)]
	}
	return strings.EqualFold(previousURL.Scheme, nextURL.Scheme) &&
		strings.EqualFold(previousURL.Hostname(), nextURL.Hostname()) &&
		previousPort == nextPort
}

func cloneAvailabilityTarget(target config.AvailabilityTarget) config.AvailabilityTarget {
	clone := target
	clone.ObservationLocationIDs = append([]string(nil), target.ObservationLocationIDs...)
	if target.HTTP == nil {
		return clone
	}
	httpClone := *target.HTTP
	httpClone.Body = cloneStringPointer(target.HTTP.Body)
	httpClone.Authentication.Password = cloneStringPointer(target.HTTP.Authentication.Password)
	httpClone.Authentication.BearerToken = cloneStringPointer(target.HTTP.Authentication.BearerToken)
	httpClone.Headers = append([]config.AvailabilityHTTPHeader(nil), target.HTTP.Headers...)
	for i := range httpClone.Headers {
		httpClone.Headers[i].Value = cloneStringPointer(httpClone.Headers[i].Value)
	}
	clone.HTTP = &httpClone
	return clone
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
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
