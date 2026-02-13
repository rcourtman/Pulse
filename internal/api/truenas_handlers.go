package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
)

const trueNASConnectionsPathPrefix = "/api/truenas/connections/"

// TrueNASHandlers manages TrueNAS connection CRUD and connectivity checks.
type TrueNASHandlers struct {
	getPersistence func(ctx context.Context) *config.ConfigPersistence
	getConfig      func(ctx context.Context) *config.Config
	getMonitor     func(ctx context.Context) *monitoring.Monitor
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
	instance.Name = strings.TrimSpace(instance.Name)
	instance.Host = strings.TrimSpace(instance.Host)
	if instance.ID == "" {
		instance.ID = config.NewTrueNASInstance().ID
	}

	if err := instance.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}

	if h.enforceNodeLimit(w, r, persistence, 1) {
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

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}

	instances, err := persistence.LoadTrueNASConfig()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "truenas_load_failed", "Failed to load TrueNAS configuration", map[string]string{"error": err.Error()})
		return
	}

	redacted := make([]config.TrueNASInstance, 0, len(instances))
	for i := range instances {
		item := instances[i].Redacted()
		redacted = append(redacted, item)
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

	instance.Host = strings.TrimSpace(instance.Host)
	instance.APIKey = strings.TrimSpace(instance.APIKey)
	instance.Username = strings.TrimSpace(instance.Username)
	if err := instance.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	client, err := truenas.NewClient(truenas.ClientConfig{
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
		writeErrorResponse(w, http.StatusBadRequest, "truenas_invalid_config", "Invalid TrueNAS connection configuration", map[string]string{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := client.TestConnection(ctx); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "truenas_connection_failed", "Failed to connect to TrueNAS", map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *TrueNASHandlers) featureEnabled(w http.ResponseWriter) bool {
	if truenas.IsFeatureEnabled() {
		return true
	}
	writeErrorResponse(w, http.StatusNotFound, "truenas_disabled", "TrueNAS integration is not enabled", nil)
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

func (h *TrueNASHandlers) enforceNodeLimit(
	w http.ResponseWriter,
	r *http.Request,
	persistence *config.ConfigPersistence,
	additionalCount int,
) bool {
	limit := maxNodesLimitForContext(r.Context())
	if limit <= 0 {
		return false
	}

	var cfg *config.Config
	if h != nil && h.getConfig != nil {
		cfg = h.getConfig(r.Context())
	}

	var mon *monitoring.Monitor
	if h != nil && h.getMonitor != nil {
		mon = h.getMonitor(r.Context())
	}

	baseCount := registeredNodeSlotCount(cfg, mon)

	trueNASInstances, err := persistence.LoadTrueNASConfig()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "truenas_load_failed", "Failed to load TrueNAS configuration", map[string]string{"error": err.Error()})
		return true
	}

	current := baseCount + len(trueNASInstances)
	if current+additionalCount <= limit {
		return false
	}

	writeMaxNodesLimitExceeded(w, current, limit)
	return true
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
