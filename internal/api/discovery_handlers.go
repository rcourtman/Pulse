package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
	"github.com/rs/zerolog/log"
)

// AIConfigProvider provides access to the current AI configuration.
// This allows discovery handlers to show AI provider info without tight coupling.
type AIConfigProvider interface {
	GetAIConfig() *config.AIConfig
}

// Note: adminBypassEnabled() is defined in auth.go

// DiscoveryHandlers handles AI-powered infrastructure discovery endpoints.
type DiscoveryHandlers struct {
	service          *servicediscovery.Service
	config           *config.Config // For admin status checks
	aiConfigProvider AIConfigProvider
}

// NewDiscoveryHandlers creates new discovery handlers.
func NewDiscoveryHandlers(service *servicediscovery.Service, cfg *config.Config) *DiscoveryHandlers {
	return &DiscoveryHandlers{
		service: service,
		config:  cfg,
	}
}

// SetService sets the discovery service (used for late initialization after routes are registered).
func (h *DiscoveryHandlers) SetService(service *servicediscovery.Service) {
	h.service = service
}

// SetAIConfigProvider sets the AI config provider for showing AI provider info.
func (h *DiscoveryHandlers) SetAIConfigProvider(provider AIConfigProvider) {
	h.aiConfigProvider = provider
}

// getAIProviderInfo returns info about the current AI provider for discovery.
func (h *DiscoveryHandlers) getAIProviderInfo() *servicediscovery.AIProviderInfo {
	if h.aiConfigProvider == nil {
		return nil
	}

	aiCfg := h.aiConfigProvider.GetAIConfig()
	if aiCfg == nil || !aiCfg.Enabled {
		return nil
	}

	// Get the discovery model
	model := aiCfg.GetDiscoveryModel()
	if model == "" {
		return nil
	}

	// Parse the model to get provider
	provider, modelName := config.ParseModelString(model)

	// Determine if local
	isLocal := provider == config.AIProviderOllama

	// Build human-readable label
	var label string
	switch provider {
	case config.AIProviderOllama:
		label = "Local (Ollama)"
	case config.AIProviderAnthropic:
		label = "Cloud (Anthropic)"
	case config.AIProviderOpenAI:
		label = "Cloud (OpenAI)"
	case config.AIProviderOpenRouter:
		label = "Cloud (OpenRouter)"
	case config.AIProviderDeepSeek:
		label = "Cloud (DeepSeek)"
	case config.AIProviderGemini:
		label = "Cloud (Google Gemini)"
	default:
		label = "Cloud (" + provider + ")"
	}

	return &servicediscovery.AIProviderInfo{
		Provider: provider,
		Model:    modelName,
		IsLocal:  isLocal,
		Label:    label,
	}
}

// writeDiscoveryJSON writes a JSON response.
func writeDiscoveryJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// writeDiscoveryError writes a JSON error response.
func writeDiscoveryError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]any{
		"error":   true,
		"message": message,
	})
}

// isAdminRequest checks if the current request is from an admin user.
// In Pulse, all authenticated users have admin privileges except non-admin
// proxy auth users. This method checks all authentication methods.
func (h *DiscoveryHandlers) isAdminRequest(r *http.Request) bool {
	// Dev mode bypass - treat all requests as admin when enabled
	if adminBypassEnabled() {
		return true
	}

	if h.config == nil {
		return false // Default to non-admin if no config
	}

	// 1. If using proxy auth, check the admin role
	if h.config.ProxyAuthSecret != "" {
		if valid, _, isAdmin := CheckProxyAuth(h.config, r); valid {
			return isAdmin
		}
		return false
	}

	// 2. Check for basic auth (Pulse single-user admin credential)
	if _, _, ok := r.BasicAuth(); ok {
		return true
	}

	// 3. Check for valid session cookie (OIDC/SAML sessions)
	if cookie, err := r.Cookie("pulse_session"); err == nil && cookie.Value != "" {
		if ValidateSession(cookie.Value) {
			return true // Valid session = admin
		}
	}

	// 4. Check for valid API token (read-only check, safe under RLock)
	if token := r.Header.Get("X-API-Token"); token != "" {
		config.Mu.RLock()
		ok := h.config.IsValidAPIToken(token)
		config.Mu.RUnlock()
		if ok {
			return true // Valid API token = admin
		}
	}

	return false
}

// redactSensitiveFields removes sensitive data from a discovery for non-admin users.
// This creates a copy to avoid modifying the original.
func redactSensitiveFields(d *servicediscovery.ResourceDiscovery) *servicediscovery.ResourceDiscovery {
	if d == nil {
		return nil
	}
	// Create a shallow copy
	redacted := *d
	// Redact sensitive fields
	redacted.UserSecrets = nil      // Never expose to non-admins
	redacted.RawCommandOutput = nil // May contain sensitive output
	return &redacted
}

// HandleListDiscoveries handles GET /api/discovery
func (h *DiscoveryHandlers) HandleListDiscoveries(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "discovery service not configured")
		return
	}

	discoveries, err := h.service.ListDiscoveries()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list discoveries")
		writeDiscoveryError(w, http.StatusInternalServerError, "Failed to list discoveries")
		return
	}

	// Convert to summaries for list view
	summaries := make([]servicediscovery.DiscoverySummary, 0, len(discoveries))
	for _, d := range discoveries {
		summaries = append(summaries, d.ToSummary())
	}

	writeDiscoveryJSON(w, map[string]any{
		"discoveries": summaries,
		"total":       len(summaries),
	})
}

// HandleGetDiscovery handles GET /api/discovery/{type}/{host}/{id}
func (h *DiscoveryHandlers) HandleGetDiscovery(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "discovery service not configured")
		return
	}

	// Parse path: /api/discovery/{type}/{host}/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/discovery/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		writeDiscoveryError(w, http.StatusBadRequest, "Invalid path: expected /api/discovery/{type}/{host}/{id}")
		return
	}

	resourceType := servicediscovery.ResourceType(parts[0])
	hostID := parts[1]
	resourceID := parts[2]

	discovery, err := h.service.GetDiscoveryByResource(resourceType, hostID, resourceID)
	if err != nil {
		log.Error().Err(err).Str("type", string(resourceType)).Str("host", hostID).Str("id", resourceID).Msg("Failed to get discovery")
		writeDiscoveryError(w, http.StatusInternalServerError, "Failed to get discovery")
		return
	}

	if discovery == nil {
		writeDiscoveryError(w, http.StatusNotFound, "Discovery not found")
		return
	}

	// Redact sensitive fields for non-admin users
	if !h.isAdminRequest(r) {
		discovery = redactSensitiveFields(discovery)
	}

	writeDiscoveryJSON(w, discovery)
}

// HandleTriggerDiscovery handles POST /api/discovery/{type}/{host}/{id}
func (h *DiscoveryHandlers) HandleTriggerDiscovery(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "discovery service not configured")
		return
	}

	// Parse path
	path := strings.TrimPrefix(r.URL.Path, "/api/discovery/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		writeDiscoveryError(w, http.StatusBadRequest, "Invalid path: expected /api/discovery/{type}/{host}/{id}")
		return
	}

	resourceType := servicediscovery.ResourceType(parts[0])
	hostID := parts[1]
	resourceID := parts[2]

	// Parse optional request body for force flag and hostname
	var reqBody struct {
		Force    bool   `json:"force"`
		Hostname string `json:"hostname"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
	}

	// Build discovery request
	req := servicediscovery.DiscoveryRequest{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		HostID:       hostID,
		Hostname:     reqBody.Hostname,
		Force:        reqBody.Force,
	}

	// If hostname not provided, try to use hostID
	if req.Hostname == "" {
		req.Hostname = hostID
	}

	discovery, err := h.service.DiscoverResource(r.Context(), req)
	if err != nil {
		log.Error().Err(err).
			Str("type", string(resourceType)).
			Str("host", hostID).
			Str("id", resourceID).
			Msg("Failed to trigger discovery")
		writeDiscoveryError(w, http.StatusInternalServerError, "Discovery failed: "+err.Error())
		return
	}

	// Redact sensitive fields for non-admin users
	if !h.isAdminRequest(r) {
		discovery = redactSensitiveFields(discovery)
	}

	writeDiscoveryJSON(w, discovery)
}

// HandleUpdateNotes handles PUT /api/discovery/{type}/{host}/{id}/notes
func (h *DiscoveryHandlers) HandleUpdateNotes(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "discovery service not configured")
		return
	}

	// Parse path
	path := strings.TrimPrefix(r.URL.Path, "/api/discovery/")
	path = strings.TrimSuffix(path, "/notes")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		writeDiscoveryError(w, http.StatusBadRequest, "Invalid path")
		return
	}

	resourceType := servicediscovery.ResourceType(parts[0])
	hostID := parts[1]
	resourceID := parts[2]

	// Build the full ID
	discoveryID := servicediscovery.MakeResourceID(resourceType, hostID, resourceID)

	// Parse request body
	var req servicediscovery.UpdateNotesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDiscoveryError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Only admins can set user_secrets (contains sensitive data like API tokens)
	isAdmin := h.isAdminRequest(r)
	if !isAdmin && len(req.UserSecrets) > 0 {
		writeDiscoveryError(w, http.StatusForbidden, "Only admins can set user_secrets")
		return
	}

	if err := h.service.UpdateNotes(discoveryID, req.UserNotes, req.UserSecrets); err != nil {
		log.Error().Err(err).Str("id", discoveryID).Msg("Failed to update notes")
		writeDiscoveryError(w, http.StatusInternalServerError, "Failed to update notes: "+err.Error())
		return
	}

	// Return updated discovery
	discovery, err := h.service.GetDiscovery(discoveryID)
	if err != nil {
		writeDiscoveryError(w, http.StatusInternalServerError, "Notes updated but failed to fetch result")
		return
	}

	// Redact sensitive fields for non-admin users
	if !isAdmin {
		discovery = redactSensitiveFields(discovery)
	}

	writeDiscoveryJSON(w, discovery)
}

// HandleDeleteDiscovery handles DELETE /api/discovery/{type}/{host}/{id}
func (h *DiscoveryHandlers) HandleDeleteDiscovery(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "discovery service not configured")
		return
	}

	// Parse path
	path := strings.TrimPrefix(r.URL.Path, "/api/discovery/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		writeDiscoveryError(w, http.StatusBadRequest, "Invalid path")
		return
	}

	resourceType := servicediscovery.ResourceType(parts[0])
	hostID := parts[1]
	resourceID := parts[2]

	discoveryID := servicediscovery.MakeResourceID(resourceType, hostID, resourceID)

	if err := h.service.DeleteDiscovery(discoveryID); err != nil {
		log.Error().Err(err).Str("id", discoveryID).Msg("Failed to delete discovery")
		writeDiscoveryError(w, http.StatusInternalServerError, "Failed to delete discovery")
		return
	}

	writeDiscoveryJSON(w, map[string]any{"success": true, "id": discoveryID})
}

// HandleGetProgress handles GET /api/discovery/{type}/{host}/{id}/progress
func (h *DiscoveryHandlers) HandleGetProgress(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "discovery service not configured")
		return
	}

	// Parse path
	path := strings.TrimPrefix(r.URL.Path, "/api/discovery/")
	path = strings.TrimSuffix(path, "/progress")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		writeDiscoveryError(w, http.StatusBadRequest, "Invalid path")
		return
	}

	resourceType := servicediscovery.ResourceType(parts[0])
	hostID := parts[1]
	resourceID := parts[2]

	discoveryID := servicediscovery.MakeResourceID(resourceType, hostID, resourceID)

	progress := h.service.GetProgress(discoveryID)
	if progress == nil {
		// Not currently scanning - check if we have a discovery
		discovery, err := h.service.GetDiscovery(discoveryID)
		if err == nil && discovery != nil {
			// Return completed status with all fields for frontend compatibility
			writeDiscoveryJSON(w, map[string]any{
				"resource_id":     discoveryID,
				"status":          "completed",
				"current_step":    "",
				"total_steps":     0,
				"completed_steps": 0,
				"started_at":      discovery.DiscoveredAt,
				"updated_at":      discovery.UpdatedAt,
			})
			return
		}

		// Return not_started status with all fields for frontend compatibility
		writeDiscoveryJSON(w, map[string]any{
			"resource_id":     discoveryID,
			"status":          "not_started",
			"current_step":    "",
			"total_steps":     0,
			"completed_steps": 0,
			"started_at":      "",
		})
		return
	}

	writeDiscoveryJSON(w, progress)
}

// HandleGetStatus handles GET /api/discovery/status
func (h *DiscoveryHandlers) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "discovery service not configured")
		return
	}

	status := h.service.GetStatus()

	// Add fingerprint change/stale stats
	changedCount, _ := h.service.GetChangedResourceCount()
	staleCount, _ := h.service.GetStaleResourceCount()

	status["changed_count"] = changedCount // Containers with changed fingerprints
	status["stale_count"] = staleCount     // Discoveries > 30 days old

	writeDiscoveryJSON(w, status)
}

// HandleUpdateSettings handles PUT /api/discovery/settings
// Allows updating discovery settings like the staleness threshold.
func (h *DiscoveryHandlers) HandleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "discovery service not configured")
		return
	}

	// Require admin privileges
	if !h.isAdminRequest(r) {
		writeDiscoveryError(w, http.StatusForbidden, "Admin privileges required")
		return
	}

	var req struct {
		MaxDiscoveryAgeDays int `json:"max_discovery_age_days"` // Days before rediscovery
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDiscoveryError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Update settings
	if req.MaxDiscoveryAgeDays > 0 {
		h.service.SetMaxDiscoveryAge(time.Duration(req.MaxDiscoveryAgeDays) * 24 * time.Hour)
		log.Info().Int("days", req.MaxDiscoveryAgeDays).Msg("Max discovery age updated via API")
	}

	// Return updated status
	status := h.service.GetStatus()
	changedCount, _ := h.service.GetChangedResourceCount()
	staleCount, _ := h.service.GetStaleResourceCount()
	status["changed_count"] = changedCount
	status["stale_count"] = staleCount

	writeDiscoveryJSON(w, status)
}

// HandleListByType handles GET /api/discovery/type/{type}
func (h *DiscoveryHandlers) HandleListByType(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "discovery service not configured")
		return
	}

	// Parse path
	path := strings.TrimPrefix(r.URL.Path, "/api/discovery/type/")
	resourceType := servicediscovery.ResourceType(path)

	discoveries, err := h.service.ListDiscoveriesByType(resourceType)
	if err != nil {
		log.Error().Err(err).Str("type", string(resourceType)).Msg("Failed to list discoveries by type")
		writeDiscoveryError(w, http.StatusInternalServerError, "Failed to list discoveries")
		return
	}

	summaries := make([]servicediscovery.DiscoverySummary, 0, len(discoveries))
	for _, d := range discoveries {
		summaries = append(summaries, d.ToSummary())
	}

	writeDiscoveryJSON(w, map[string]any{
		"discoveries": summaries,
		"total":       len(summaries),
		"type":        resourceType,
	})
}

// HandleListByHost handles GET /api/discovery/host/{host}
func (h *DiscoveryHandlers) HandleListByHost(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeDiscoveryError(w, http.StatusServiceUnavailable, "discovery service not configured")
		return
	}

	// Parse path
	hostID := strings.TrimPrefix(r.URL.Path, "/api/discovery/host/")

	discoveries, err := h.service.ListDiscoveriesByHost(hostID)
	if err != nil {
		log.Error().Err(err).Str("host", hostID).Msg("Failed to list discoveries by host")
		writeDiscoveryError(w, http.StatusInternalServerError, "Failed to list discoveries")
		return
	}

	summaries := make([]servicediscovery.DiscoverySummary, 0, len(discoveries))
	for _, d := range discoveries {
		summaries = append(summaries, d.ToSummary())
	}

	writeDiscoveryJSON(w, map[string]any{
		"discoveries": summaries,
		"total":       len(summaries),
		"host":        hostID,
	})
}

// HandleGetInfo handles GET /api/discovery/info/{type}
// Returns metadata about the discovery process: AI provider info and commands that will run.
func (h *DiscoveryHandlers) HandleGetInfo(w http.ResponseWriter, r *http.Request) {
	// Parse resource type from path
	path := strings.TrimPrefix(r.URL.Path, "/api/discovery/info/")
	resourceType := servicediscovery.ResourceType(path)

	// Get commands for this resource type
	commands := servicediscovery.GetCommandsForResource(resourceType)
	categories := servicediscovery.GetCommandCategories(resourceType)

	// Get AI provider info
	aiProvider := h.getAIProviderInfo()

	info := servicediscovery.DiscoveryInfo{
		AIProvider:        aiProvider,
		Commands:          commands,
		CommandCategories: categories,
	}

	writeDiscoveryJSON(w, info)
}
