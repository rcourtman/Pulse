package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
)

const vmwareConnectionsPathPrefix = "/api/vmware/connections/"

// VMwareHandlers manages VMware vCenter connection CRUD and connectivity checks.
type VMwareHandlers struct {
	getPersistence func(ctx context.Context) *config.ConfigPersistence
	newClient      func(vmware.ClientConfig) (vmwareClient, error)

	statusMu sync.RWMutex
	statuses map[string]vmwareConnectionRuntimeStatus
}

type vmwareClient interface {
	TestConnection(ctx context.Context) (*vmware.InventorySummary, error)
	Close()
}

type vmwareConnectionResponse struct {
	config.VMwareVCenterInstance
	Test     *vmwareConnectionTestStatus     `json:"test,omitempty"`
	Observed *vmwareConnectionObservedStatus `json:"observed,omitempty"`
}

type vmwareConnectionTestStatus struct {
	LastAttemptAt *time.Time                 `json:"lastAttemptAt,omitempty"`
	LastSuccessAt *time.Time                 `json:"lastSuccessAt,omitempty"`
	LastError     *vmwareConnectionTestError `json:"lastError,omitempty"`
}

type vmwareConnectionTestError struct {
	At       *time.Time `json:"at,omitempty"`
	Message  string     `json:"message,omitempty"`
	Category string     `json:"category,omitempty"`
}

type vmwareConnectionObservedStatus struct {
	CollectedAt *time.Time `json:"collectedAt,omitempty"`
	Hosts       int        `json:"hosts"`
	VMs         int        `json:"vms"`
	Datastores  int        `json:"datastores"`
	VIRelease   string     `json:"viRelease,omitempty"`
}

type vmwareConnectionRuntimeStatus struct {
	Test     *vmwareConnectionTestStatus
	Observed *vmwareConnectionObservedStatus
}

// HandleAdd stores a new VMware vCenter connection.
func (h *VMwareHandlers) HandleAdd(w http.ResponseWriter, r *http.Request) {
	if !h.featureEnabled(w) {
		return
	}
	if mock.IsMockEnabled() {
		writeErrorResponse(w, http.StatusForbidden, "mock_mode_enabled", "Cannot modify connections in mock mode", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	defer r.Body.Close()

	var instance config.VMwareVCenterInstance
	if err := json.NewDecoder(r.Body).Decode(&instance); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", map[string]string{"error": err.Error()})
		return
	}

	instance.ID = strings.TrimSpace(instance.ID)
	if instance.ID == "" {
		instance.ID = config.NewVMwareVCenterInstance().ID
	}
	normalizeVMwareInstance(&instance)

	if err := instance.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}

	instances, err := persistence.LoadVMwareConfig()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "vmware_load_failed", "Failed to load VMware configuration", map[string]string{"error": err.Error()})
		return
	}

	instances = append(instances, instance)
	if err := persistence.SaveVMwareConfig(instances); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "vmware_save_failed", "Failed to save VMware configuration", map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, instance.Redacted())
}

// HandleList returns all configured VMware connections with sensitive fields redacted.
func (h *VMwareHandlers) HandleList(w http.ResponseWriter, r *http.Request) {
	if !h.featureEnabled(w) {
		return
	}

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}

	instances, err := persistence.LoadVMwareConfig()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "vmware_load_failed", "Failed to load VMware configuration", map[string]string{"error": err.Error()})
		return
	}

	responses := make([]vmwareConnectionResponse, 0, len(instances))
	for i := range instances {
		item := instances[i]
		item.ApplyDefaults()
		status := h.runtimeStatus(strings.TrimSpace(item.ID))
		responses = append(responses, vmwareConnectionResponse{
			VMwareVCenterInstance: item.Redacted(),
			Test:                  status.Test,
			Observed:              status.Observed,
		})
	}

	writeJSON(w, http.StatusOK, responses)
}

// HandleDelete removes a configured VMware vCenter connection by ID.
func (h *VMwareHandlers) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if !h.featureEnabled(w) {
		return
	}
	if mock.IsMockEnabled() {
		writeErrorResponse(w, http.StatusForbidden, "mock_mode_enabled", "Cannot modify connections in mock mode", nil)
		return
	}

	connectionID, ok := vmwareConnectionIDFromPath(r.URL.Path)
	if !ok {
		writeErrorResponse(w, http.StatusBadRequest, "missing_connection_id", "Connection ID is required", nil)
		return
	}

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}

	instances, err := persistence.LoadVMwareConfig()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "vmware_load_failed", "Failed to load VMware configuration", map[string]string{"error": err.Error()})
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
		writeErrorResponse(w, http.StatusNotFound, "vmware_not_found", "Connection not found", nil)
		return
	}

	instances = append(instances[:index], instances[index+1:]...)
	if err := persistence.SaveVMwareConfig(instances); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "vmware_save_failed", "Failed to save VMware configuration", map[string]string{"error": err.Error()})
		return
	}

	h.clearRuntimeStatus(connectionID)

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"id":      connectionID,
	})
}

// HandleUpdate replaces a configured VMware vCenter connection by ID while
// preserving unchanged masked secrets from the stored record.
func (h *VMwareHandlers) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.featureEnabled(w) {
		return
	}
	if mock.IsMockEnabled() {
		writeErrorResponse(w, http.StatusForbidden, "mock_mode_enabled", "Cannot modify connections in mock mode", nil)
		return
	}

	connectionID, ok := vmwareConnectionIDFromPath(r.URL.Path)
	if !ok {
		writeErrorResponse(w, http.StatusBadRequest, "missing_connection_id", "Connection ID is required", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	defer r.Body.Close()

	var instance config.VMwareVCenterInstance
	if err := json.NewDecoder(r.Body).Decode(&instance); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", map[string]string{"error": err.Error()})
		return
	}

	instance.ID = connectionID
	normalizeVMwareInstance(&instance)

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}

	instances, err := persistence.LoadVMwareConfig()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "vmware_load_failed", "Failed to load VMware configuration", map[string]string{"error": err.Error()})
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
		writeErrorResponse(w, http.StatusNotFound, "vmware_not_found", "Connection not found", nil)
		return
	}

	instance.PreserveMaskedSecrets(instances[index])
	if err := instance.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	instances[index] = instance
	if err := persistence.SaveVMwareConfig(instances); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "vmware_save_failed", "Failed to save VMware configuration", map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, instance.Redacted())
}

// HandleTestConnection validates connectivity for a proposed VMware vCenter connection.
func (h *VMwareHandlers) HandleTestConnection(w http.ResponseWriter, r *http.Request) {
	if !h.featureEnabled(w) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	defer r.Body.Close()

	var instance config.VMwareVCenterInstance
	if err := json.NewDecoder(r.Body).Decode(&instance); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", map[string]string{"error": err.Error()})
		return
	}

	normalizeVMwareInstance(&instance)
	if err := instance.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	h.writeConnectionTestResult(w, r, instance)
}

// HandleTestSavedConnection validates connectivity for one saved VMware
// connection using the server-owned stored secret material instead of relying
// on frontend redaction placeholders.
func (h *VMwareHandlers) HandleTestSavedConnection(w http.ResponseWriter, r *http.Request) {
	if !h.featureEnabled(w) {
		return
	}

	connectionID, ok := vmwareConnectionIDFromTestPath(r.URL.Path)
	if !ok {
		writeErrorResponse(w, http.StatusBadRequest, "missing_connection_id", "Connection ID is required", nil)
		return
	}

	persistence := h.persistenceForRequest(w, r.Context())
	if persistence == nil {
		return
	}

	instances, err := persistence.LoadVMwareConfig()
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "vmware_load_failed", "Failed to load VMware configuration", map[string]string{"error": err.Error()})
		return
	}

	for i := range instances {
		instance := instances[i]
		if strings.TrimSpace(instance.ID) != connectionID {
			continue
		}
		normalizeVMwareInstance(&instance)
		payload, hasPayload, ok := decodeOptionalVMwareInstanceRequest(w, r)
		if !ok {
			return
		}
		if hasPayload {
			payload.ID = connectionID
			normalizeVMwareInstance(&payload)
			payload.PreserveMaskedSecrets(instance)
			if err := payload.Validate(); err != nil {
				writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
				return
			}
			instance = payload
		}

		summary, invalidConfig, err := h.testConnectionInstance(r, instance)
		if err != nil {
			if !hasPayload {
				h.recordTestFailure(connectionID, err, time.Now().UTC())
			}
			h.writeConnectionFailure(w, invalidConfig, err)
			return
		}
		if !hasPayload {
			h.recordTestSuccess(connectionID, summary, time.Now().UTC())
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
		return
	}

	writeErrorResponse(w, http.StatusNotFound, "vmware_not_found", "Connection not found", nil)
}

func (h *VMwareHandlers) writeConnectionTestResult(
	w http.ResponseWriter,
	r *http.Request,
	instance config.VMwareVCenterInstance,
) {
	_, invalidConfig, err := h.testConnectionInstance(r, instance)
	if err != nil {
		h.writeConnectionFailure(w, invalidConfig, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *VMwareHandlers) writeConnectionFailure(w http.ResponseWriter, invalidConfig bool, err error) {
	if invalidConfig {
		writeErrorResponse(w, http.StatusBadRequest, "vmware_invalid_config", "Invalid VMware vCenter connection configuration", connectionFailureDetails(err))
		return
	}
	writeErrorResponse(w, http.StatusBadRequest, "vmware_connection_failed", "Failed to connect to VMware vCenter", connectionFailureDetails(err))
}

func connectionFailureDetails(err error) map[string]string {
	if err == nil {
		return nil
	}
	details := map[string]string{"error": err.Error()}
	if category := connectionErrorCategory(err); category != "" {
		details["category"] = category
	}
	return details
}

func connectionErrorCategory(err error) string {
	if err == nil {
		return ""
	}
	if connectionErr, ok := err.(*vmware.ConnectionError); ok {
		return strings.TrimSpace(connectionErr.Category)
	}
	return ""
}

func (h *VMwareHandlers) testConnectionInstance(
	r *http.Request,
	instance config.VMwareVCenterInstance,
) (*vmware.InventorySummary, bool, error) {
	newClient := h.newClient
	if newClient == nil {
		newClient = func(cfg vmware.ClientConfig) (vmwareClient, error) { return vmware.NewClient(cfg) }
	}

	client, err := newClient(vmware.ClientConfig{
		Host:               instance.Host,
		Port:               instance.Port,
		Username:           instance.Username,
		Password:           instance.Password,
		InsecureSkipVerify: instance.InsecureSkipVerify,
		Timeout:            10 * time.Second,
	})
	if err != nil {
		return nil, true, err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	summary, err := client.TestConnection(ctx)
	if err != nil {
		return nil, false, err
	}
	return summary, false, nil
}

func normalizeVMwareInstance(instance *config.VMwareVCenterInstance) {
	if instance == nil {
		return
	}
	instance.Name = strings.TrimSpace(instance.Name)
	instance.Host = strings.TrimSpace(instance.Host)
	instance.Username = strings.TrimSpace(instance.Username)
	instance.Password = strings.TrimSpace(instance.Password)
	instance.ApplyDefaults()
}

func decodeOptionalVMwareInstanceRequest(
	w http.ResponseWriter,
	r *http.Request,
) (config.VMwareVCenterInstance, bool, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", map[string]string{"error": err.Error()})
		return config.VMwareVCenterInstance{}, false, false
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return config.VMwareVCenterInstance{}, false, true
	}

	var instance config.VMwareVCenterInstance
	if err := json.Unmarshal(body, &instance); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", map[string]string{"error": err.Error()})
		return config.VMwareVCenterInstance{}, false, false
	}

	return instance, true, true
}

func (h *VMwareHandlers) featureEnabled(w http.ResponseWriter) bool {
	if vmware.IsFeatureEnabled() {
		return true
	}
	writeErrorResponse(w, http.StatusNotFound, "vmware_disabled", "VMware integration has been explicitly disabled", nil)
	return false
}

func (h *VMwareHandlers) persistenceForRequest(w http.ResponseWriter, ctx context.Context) *config.ConfigPersistence {
	if h == nil || h.getPersistence == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "vmware_unavailable", "VMware service unavailable", nil)
		return nil
	}
	persistence := h.getPersistence(ctx)
	if persistence == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "vmware_unavailable", "VMware service unavailable", nil)
		return nil
	}
	return persistence
}

func (h *VMwareHandlers) runtimeStatus(connectionID string) vmwareConnectionRuntimeStatus {
	if h == nil {
		return vmwareConnectionRuntimeStatus{}
	}
	h.statusMu.RLock()
	defer h.statusMu.RUnlock()
	if h.statuses == nil {
		return vmwareConnectionRuntimeStatus{}
	}
	status, ok := h.statuses[connectionID]
	if !ok {
		return vmwareConnectionRuntimeStatus{}
	}
	return cloneVMwareRuntimeStatus(status)
}

func (h *VMwareHandlers) recordTestSuccess(connectionID string, summary *vmware.InventorySummary, at time.Time) {
	if h == nil {
		return
	}
	h.statusMu.Lock()
	defer h.statusMu.Unlock()
	if h.statuses == nil {
		h.statuses = make(map[string]vmwareConnectionRuntimeStatus)
	}
	current := h.statuses[connectionID]
	current.Test = &vmwareConnectionTestStatus{
		LastAttemptAt: timePointer(at),
		LastSuccessAt: timePointer(at),
	}
	if summary != nil {
		current.Observed = &vmwareConnectionObservedStatus{
			CollectedAt: timePointer(at),
			Hosts:       summary.Hosts,
			VMs:         summary.VMs,
			Datastores:  summary.Datastores,
			VIRelease:   strings.TrimSpace(summary.VIRelease),
		}
	}
	h.statuses[connectionID] = current
}

func (h *VMwareHandlers) recordTestFailure(connectionID string, err error, at time.Time) {
	if h == nil {
		return
	}
	h.statusMu.Lock()
	defer h.statusMu.Unlock()
	if h.statuses == nil {
		h.statuses = make(map[string]vmwareConnectionRuntimeStatus)
	}
	current := h.statuses[connectionID]
	test := current.Test
	if test == nil {
		test = &vmwareConnectionTestStatus{}
	}
	test.LastAttemptAt = timePointer(at)
	test.LastError = &vmwareConnectionTestError{
		At:       timePointer(at),
		Message:  err.Error(),
		Category: connectionErrorCategory(err),
	}
	current.Test = test
	h.statuses[connectionID] = current
}

func (h *VMwareHandlers) clearRuntimeStatus(connectionID string) {
	if h == nil {
		return
	}
	h.statusMu.Lock()
	defer h.statusMu.Unlock()
	if h.statuses == nil {
		return
	}
	delete(h.statuses, connectionID)
}

func cloneVMwareRuntimeStatus(status vmwareConnectionRuntimeStatus) vmwareConnectionRuntimeStatus {
	cloned := vmwareConnectionRuntimeStatus{}
	if status.Test != nil {
		test := *status.Test
		if test.LastAttemptAt != nil {
			test.LastAttemptAt = timePointer(*test.LastAttemptAt)
		}
		if test.LastSuccessAt != nil {
			test.LastSuccessAt = timePointer(*test.LastSuccessAt)
		}
		if test.LastError != nil {
			errCopy := *test.LastError
			if errCopy.At != nil {
				errCopy.At = timePointer(*errCopy.At)
			}
			test.LastError = &errCopy
		}
		cloned.Test = &test
	}
	if status.Observed != nil {
		observed := *status.Observed
		if observed.CollectedAt != nil {
			observed.CollectedAt = timePointer(*observed.CollectedAt)
		}
		cloned.Observed = &observed
	}
	return cloned
}

func timePointer(value time.Time) *time.Time {
	v := value
	return &v
}

func vmwareConnectionIDFromPath(path string) (string, bool) {
	if !strings.HasPrefix(path, vmwareConnectionsPathPrefix) {
		return "", false
	}

	raw := strings.Trim(strings.TrimPrefix(path, vmwareConnectionsPathPrefix), "/")
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

func vmwareConnectionIDFromTestPath(path string) (string, bool) {
	if !strings.HasPrefix(path, vmwareConnectionsPathPrefix) {
		return "", false
	}

	raw := strings.Trim(strings.TrimPrefix(path, vmwareConnectionsPathPrefix), "/")
	if !strings.HasSuffix(raw, "/test") {
		return "", false
	}
	raw = strings.TrimSuffix(raw, "/test")
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
