package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const trueNASConnectionsPathPrefix = "/api/truenas/connections/"

// TrueNASHandlers manages TrueNAS connection CRUD and connectivity checks.
type TrueNASHandlers struct {
	getPersistence func(ctx context.Context) *config.ConfigPersistence
	getConfig      func(ctx context.Context) *config.Config
	getMonitor     func(ctx context.Context) *monitoring.Monitor
	getPoller      func(ctx context.Context) *monitoring.TrueNASPoller
	newClient      func(truenas.ClientConfig) (trueNASClient, error)
}

type trueNASConnectionResponse struct {
	config.TrueNASInstance
	Poll     *monitoring.TrueNASConnectionPollStatus      `json:"poll,omitempty"`
	Observed *monitoring.TrueNASConnectionObservedSummary `json:"observed,omitempty"`
}

type trueNASClient interface {
	TestConnection(ctx context.Context) error
	Close()
}

// HandleAdd stores a new TrueNAS connection.
func (h *TrueNASHandlers) HandleAdd(w http.ResponseWriter, r *http.Request) {
	if !h.featureEnabled(w) {
		return
	}
	if mock.IsMockEnabled() {
		writeErrorResponse(w, http.StatusForbidden, "mock_mode_enabled", "Cannot modify connections in mock mode", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	defer r.Body.Close()

	var instance config.TrueNASInstance
	if err := json.NewDecoder(r.Body).Decode(&instance); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", map[string]string{"error": err.Error()})
		return
	}

	instance.ID = strings.TrimSpace(instance.ID)
	if instance.ID == "" {
		instance.ID = config.NewTrueNASInstance().ID
	}
	normalizeTrueNASInstance(&instance)

	if err := instance.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}

	if h.enforceMonitoredSystemLimit(w, r, instance) {
		return
	}

	existing, err := persistence.LoadTrueNASConfig()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "truenas_load_failed", "Failed to load TrueNAS configuration", map[string]string{"error": err.Error()})
		return
	}

	existing = append(existing, instance)
	if err := persistence.SaveTrueNASConfig(existing); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "truenas_save_failed", "Failed to save TrueNAS configuration", map[string]string{"error": err.Error()})
		return
	}

	redacted := instance.Redacted()
	writeJSON(w, http.StatusCreated, redacted)
}

// HandleList returns all configured TrueNAS connections with sensitive fields redacted.
func (h *TrueNASHandlers) HandleList(w http.ResponseWriter, r *http.Request) {
	if !h.featureEnabled(w) {
		return
	}
	if mock.IsMockEnabled() {
		writeJSON(w, http.StatusOK, mockTrueNASConnectionResponses())
		return
	}

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}

	instances, err := persistence.LoadTrueNASConfig()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "truenas_load_failed", "Failed to load TrueNAS configuration", map[string]string{"error": err.Error()})
		return
	}

	orgID := resolveTenantOrgID(r)
	var summaries map[string]monitoring.TrueNASConnectionSummary
	if h != nil && h.getPoller != nil {
		if poller := h.getPoller(r.Context()); poller != nil {
			summaries = poller.ConnectionSummaries(orgID, instances)
		}
	}

	redacted := make([]trueNASConnectionResponse, 0, len(instances))
	for i := range instances {
		item := instances[i]
		item.ApplyDefaults()
		response := trueNASConnectionResponse{
			TrueNASInstance: item.Redacted(),
		}
		if summary, ok := summaries[strings.TrimSpace(item.ID)]; ok {
			response.Poll = summary.Poll
			response.Observed = summary.Observed
		}
		redacted = append(redacted, response)
	}

	writeJSON(w, http.StatusOK, redacted)
}

// HandleDelete removes a configured TrueNAS connection by ID.
func (h *TrueNASHandlers) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if !h.featureEnabled(w) {
		return
	}
	if mock.IsMockEnabled() {
		writeErrorResponse(w, http.StatusForbidden, "mock_mode_enabled", "Cannot modify connections in mock mode", nil)
		return
	}

	connectionID, ok := trueNASConnectionIDFromPath(r.URL.Path)
	if !ok {
		writeErrorResponse(w, http.StatusBadRequest, "missing_connection_id", "Connection ID is required", nil)
		return
	}

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}

	instances, err := persistence.LoadTrueNASConfig()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "truenas_load_failed", "Failed to load TrueNAS configuration", map[string]string{"error": err.Error()})
		return
	}

	index := -1
	for i := range instances {
		if strings.TrimSpace(instances[i].ID) == connectionID {
			index = i
			break
		}
	}
	if index < 0 {
		writeErrorResponse(w, http.StatusNotFound, "truenas_not_found", "Connection not found", nil)
		return
	}

	instances = append(instances[:index], instances[index+1:]...)
	if err := persistence.SaveTrueNASConfig(instances); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "truenas_save_failed", "Failed to save TrueNAS configuration", map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"id":      connectionID,
	})
}

// HandleUpdate replaces a configured TrueNAS connection by ID while preserving
// unchanged masked secrets from the stored record.
func (h *TrueNASHandlers) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.featureEnabled(w) {
		return
	}
	if mock.IsMockEnabled() {
		writeErrorResponse(w, http.StatusForbidden, "mock_mode_enabled", "Cannot modify connections in mock mode", nil)
		return
	}

	connectionID, ok := trueNASConnectionIDFromPath(r.URL.Path)
	if !ok {
		writeErrorResponse(w, http.StatusBadRequest, "missing_connection_id", "Connection ID is required", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	defer r.Body.Close()

	var instance config.TrueNASInstance
	if err := json.NewDecoder(r.Body).Decode(&instance); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", map[string]string{"error": err.Error()})
		return
	}

	instance.ID = connectionID
	normalizeTrueNASInstance(&instance)

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}

	instances, err := persistence.LoadTrueNASConfig()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "truenas_load_failed", "Failed to load TrueNAS configuration", map[string]string{"error": err.Error()})
		return
	}

	index := -1
	for i := range instances {
		if strings.TrimSpace(instances[i].ID) == connectionID {
			index = i
			break
		}
	}
	if index < 0 {
		writeErrorResponse(w, http.StatusNotFound, "truenas_not_found", "Connection not found", nil)
		return
	}

	instance.PreserveMaskedSecrets(instances[index])
	if err := instance.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}
	if h.enforceMonitoredSystemLimitReplacement(w, r, instances[index], instance) {
		return
	}

	instances[index] = instance
	if err := persistence.SaveTrueNASConfig(instances); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "truenas_save_failed", "Failed to save TrueNAS configuration", map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, instance.Redacted())
}

// HandleTestConnection validates connectivity for a proposed TrueNAS connection.
func (h *TrueNASHandlers) HandleTestConnection(w http.ResponseWriter, r *http.Request) {
	if !h.featureEnabled(w) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	defer r.Body.Close()

	var instance config.TrueNASInstance
	if err := json.NewDecoder(r.Body).Decode(&instance); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", map[string]string{"error": err.Error()})
		return
	}

	normalizeTrueNASInstance(&instance)
	if err := instance.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	h.writeConnectionTestResult(w, r, instance)
}

// HandleTestSavedConnection validates connectivity for one saved TrueNAS
// connection using the server-owned stored secret material instead of relying
// on frontend redaction placeholders.
func (h *TrueNASHandlers) HandleTestSavedConnection(w http.ResponseWriter, r *http.Request) {
	if !h.featureEnabled(w) {
		return
	}

	connectionID, ok := trueNASConnectionIDFromTestPath(r.URL.Path)
	if !ok {
		writeErrorResponse(w, http.StatusBadRequest, "missing_connection_id", "Connection ID is required", nil)
		return
	}

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}

	instances, err := persistence.LoadTrueNASConfig()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "truenas_load_failed", "Failed to load TrueNAS configuration", map[string]string{"error": err.Error()})
		return
	}

	for i := range instances {
		instance := instances[i]
		if strings.TrimSpace(instance.ID) != connectionID {
			continue
		}
		normalizeTrueNASInstance(&instance)
		payload, hasPayload, ok := decodeOptionalTrueNASInstanceRequest(w, r)
		if !ok {
			return
		} else if hasPayload {
			payload.ID = connectionID
			normalizeTrueNASInstance(&payload)
			payload.PreserveMaskedSecrets(instance)
			if err := payload.Validate(); err != nil {
				writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
				return
			}
			instance = payload
		}
		invalidConfig, err := h.testConnectionInstance(r, instance)
		if err != nil {
			if !hasPayload && h != nil && h.getPoller != nil {
				if poller := h.getPoller(r.Context()); poller != nil {
					poller.RecordConnectionTestFailure(resolveTenantOrgID(r), connectionID, instance, err, time.Now().UTC())
				}
			}
			if invalidConfig {
				writeErrorResponse(w, http.StatusBadRequest, "truenas_invalid_config", "Invalid TrueNAS connection configuration", map[string]string{"error": err.Error()})
				return
			}
			writeErrorResponse(w, http.StatusBadRequest, "truenas_connection_failed", "Failed to connect to TrueNAS", map[string]string{"error": err.Error()})
			return
		}
		if !hasPayload && h != nil && h.getPoller != nil {
			if poller := h.getPoller(r.Context()); poller != nil {
				poller.RecordConnectionTestSuccess(resolveTenantOrgID(r), connectionID, instance, time.Now().UTC())
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
		return
	}

	writeErrorResponse(w, http.StatusNotFound, "truenas_not_found", "Connection not found", nil)
}

func (h *TrueNASHandlers) writeConnectionTestResult(
	w http.ResponseWriter,
	r *http.Request,
	instance config.TrueNASInstance,
) {
	invalidConfig, err := h.testConnectionInstance(r, instance)
	if err != nil {
		if invalidConfig {
			writeErrorResponse(w, http.StatusBadRequest, "truenas_invalid_config", "Invalid TrueNAS connection configuration", map[string]string{"error": err.Error()})
			return
		}
		writeErrorResponse(w, http.StatusBadRequest, "truenas_connection_failed", "Failed to connect to TrueNAS", map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *TrueNASHandlers) testConnectionInstance(
	r *http.Request,
	instance config.TrueNASInstance,
) (bool, error) {
	newClient := h.newClient
	if newClient == nil {
		newClient = func(cfg truenas.ClientConfig) (trueNASClient, error) { return truenas.NewClient(cfg) }
	}

	client, err := newClient(truenas.ClientConfig{
		Host:               instance.Host,
		Port:               instance.Port,
		APIKey:             instance.APIKey,
		Username:           instance.Username,
		Password:           instance.Password,
		UseHTTPS:           instance.UseHTTPS,
		InsecureSkipVerify: instance.InsecureSkipVerify,
		Fingerprint:        instance.Fingerprint,
		Timeout:            10 * time.Second,
	})
	if err != nil {
		return true, err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := client.TestConnection(ctx); err != nil {
		return false, err
	}
	return false, nil
}

func normalizeTrueNASInstance(instance *config.TrueNASInstance) {
	if instance == nil {
		return
	}
	instance.Name = strings.TrimSpace(instance.Name)
	instance.Host = strings.TrimSpace(instance.Host)
	instance.APIKey = strings.TrimSpace(instance.APIKey)
	instance.Username = strings.TrimSpace(instance.Username)
	instance.Password = strings.TrimSpace(instance.Password)
	instance.Fingerprint = strings.TrimSpace(instance.Fingerprint)
	instance.ApplyDefaults()
}

func decodeOptionalTrueNASInstanceRequest(
	w http.ResponseWriter,
	r *http.Request,
) (config.TrueNASInstance, bool, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", map[string]string{"error": err.Error()})
		return config.TrueNASInstance{}, false, false
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return config.TrueNASInstance{}, false, true
	}

	var instance config.TrueNASInstance
	if err := json.Unmarshal(body, &instance); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", map[string]string{"error": err.Error()})
		return config.TrueNASInstance{}, false, false
	}

	return instance, true, true
}

func (h *TrueNASHandlers) featureEnabled(w http.ResponseWriter) bool {
	if truenas.IsFeatureEnabled() {
		return true
	}
	writeErrorResponse(w, http.StatusNotFound, "truenas_disabled", "TrueNAS integration has been explicitly disabled", nil)
	return false
}

func (h *TrueNASHandlers) persistenceForRequest(w http.ResponseWriter, ctx context.Context) *config.ConfigPersistence {
	if h == nil || h.getPersistence == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "truenas_unavailable", "TrueNAS service unavailable", nil)
		return nil
	}
	persistence := h.getPersistence(ctx)
	if persistence == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "truenas_unavailable", "TrueNAS service unavailable", nil)
		return nil
	}
	return persistence
}

func (h *TrueNASHandlers) enforceMonitoredSystemLimit(
	w http.ResponseWriter,
	r *http.Request,
	instance config.TrueNASInstance,
) bool {
	var monitor *monitoring.Monitor
	if h != nil && h.getMonitor != nil {
		monitor = h.getMonitor(r.Context())
	}

	decision := monitoredSystemLimitDecisionForCandidate(r.Context(), monitor, trueNASMonitoredSystemCandidate(instance))
	if !decision.usageAvailable {
		writeMonitoredSystemUsageUnavailable(w)
		return true
	}
	if !decision.exceeded {
		return false
	}

	emitLimitBlockedEvent(r.Context(), decision.current, decision.limit)
	writeMaxMonitoredSystemsLimitExceeded(w, decision.current, decision.limit)
	return true
}

func (h *TrueNASHandlers) enforceMonitoredSystemLimitReplacement(
	w http.ResponseWriter,
	r *http.Request,
	current config.TrueNASInstance,
	next config.TrueNASInstance,
) bool {
	var monitor *monitoring.Monitor
	if h != nil && h.getMonitor != nil {
		monitor = h.getMonitor(r.Context())
	}

	replacementHost := pulseTokenHostCandidate(current.Host)
	decision := monitoredSystemLimitDecisionForCandidateReplacement(r.Context(), monitor, unifiedresources.MonitoredSystemReplacement{
		Source: unifiedresources.SourceTrueNAS,
		Matches: func(resource unifiedresources.Resource) bool {
			return resource.TrueNAS != nil && strings.EqualFold(strings.TrimSpace(resource.TrueNAS.Hostname), replacementHost)
		},
	}, trueNASMonitoredSystemCandidate(next))
	if !decision.usageAvailable {
		writeMonitoredSystemUsageUnavailable(w)
		return true
	}
	if !decision.exceeded {
		return false
	}

	emitLimitBlockedEvent(r.Context(), decision.current, decision.limit)
	writeMaxMonitoredSystemsLimitExceeded(w, decision.current, decision.limit)
	return true
}

func trueNASMonitoredSystemCandidate(instance config.TrueNASInstance) unifiedresources.MonitoredSystemCandidate {
	return unifiedresources.MonitoredSystemCandidate{
		Source:   unifiedresources.SourceTrueNAS,
		Type:     unifiedresources.ResourceTypeAgent,
		Name:     instance.Name,
		Hostname: pulseTokenHostCandidate(instance.Host),
		HostURL:  instance.Host,
	}
}

func trueNASConnectionIDFromPath(path string) (string, bool) {
	if !strings.HasPrefix(path, trueNASConnectionsPathPrefix) {
		return "", false
	}

	raw := strings.Trim(strings.TrimPrefix(path, trueNASConnectionsPathPrefix), "/")
	if raw == "" || strings.Contains(raw, "/") {
		return "", false
	}

	connectionID, err := url.PathUnescape(raw)
	if err != nil {
		return "", false
	}
	connectionID = strings.TrimSpace(connectionID)
	if connectionID == "" || strings.Contains(connectionID, "/") {
		return "", false
	}

	return connectionID, true
}

func trueNASConnectionIDFromTestPath(path string) (string, bool) {
	if !strings.HasPrefix(path, trueNASConnectionsPathPrefix) {
		return "", false
	}
	trimmed := strings.Trim(strings.TrimPrefix(path, trueNASConnectionsPathPrefix), "/")
	if !strings.HasSuffix(trimmed, "/test") {
		return "", false
	}
	raw := strings.TrimSuffix(trimmed, "/test")
	if raw == "" || strings.Contains(raw, "/") {
		return "", false
	}
	connectionID, err := url.PathUnescape(raw)
	if err != nil {
		return "", false
	}
	connectionID = strings.TrimSpace(connectionID)
	if connectionID == "" || strings.Contains(connectionID, "/") {
		return "", false
	}
	return connectionID, true
}
