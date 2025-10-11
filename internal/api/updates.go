package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rs/zerolog/log"
)

// UpdateHandlers handles update-related API requests
type UpdateHandlers struct {
	manager  *updates.Manager
	history  *updates.UpdateHistory
	registry *updates.UpdaterRegistry
}

// NewUpdateHandlers creates new update handlers
func NewUpdateHandlers(manager *updates.Manager, dataDir string) *UpdateHandlers {
	// Initialize update history using configured data directory
	// Empty string defaults to /var/lib/pulse for backward compatibility
	history, err := updates.NewUpdateHistory(dataDir)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize update history")
		// Continue without history - handlers will check for nil
	}

	// Initialize updater registry
	registry := updates.NewUpdaterRegistry()

	// Register adapters
	registry.Register("systemd", updates.NewInstallShAdapter(history))
	registry.Register("proxmoxve", updates.NewInstallShAdapter(history))
	registry.Register("docker", updates.NewDockerUpdater())
	registry.Register("aur", updates.NewAURUpdater())

	return &UpdateHandlers{
		manager:  manager,
		history:  history,
		registry: registry,
	}
}

// HandleCheckUpdates handles update check requests
func (h *UpdateHandlers) HandleCheckUpdates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Get channel from query parameter if provided
	channel := r.URL.Query().Get("channel")

	info, err := h.manager.CheckForUpdatesWithChannel(ctx, channel)
	if err != nil {
		log.Error().Err(err).Msg("Failed to check for updates")
		http.Error(w, "Failed to check for updates", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(info); err != nil {
		log.Error().Err(err).Msg("Failed to encode update info")
	}
}

// HandleApplyUpdate handles update application requests
func (h *UpdateHandlers) HandleApplyUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DownloadURL string `json:"downloadUrl"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.DownloadURL == "" {
		http.Error(w, "Download URL is required", http.StatusBadRequest)
		return
	}

	// Start update in background with a new context (not request context which gets cancelled)
	go func() {
		ctx := context.Background()
		if err := h.manager.ApplyUpdate(ctx, req.DownloadURL); err != nil {
			log.Error().Err(err).Msg("Failed to apply update")
		}
	}()

	// Return success immediately
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Update process started",
	})
}

// HandleUpdateStatus handles update status requests
func (h *UpdateHandlers) HandleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := h.manager.GetStatus()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Error().Err(err).Msg("Failed to encode update status")
	}
}

// HandleGetUpdatePlan returns update plan for current deployment
func (h *UpdateHandlers) HandleGetUpdatePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current version info to determine deployment type
	versionInfo, err := updates.GetCurrentVersion()
	if err != nil {
		http.Error(w, "Failed to get version info", http.StatusInternalServerError)
		return
	}

	// Get updater for deployment type
	updater, err := h.registry.Get(versionInfo.DeploymentType)
	if err != nil {
		http.Error(w, "No updater for deployment type", http.StatusNotFound)
		return
	}

	// Get version from query
	version := r.URL.Query().Get("version")
	if version == "" {
		http.Error(w, "version parameter required", http.StatusBadRequest)
		return
	}

	// Prepare update plan
	plan, err := updater.PrepareUpdate(r.Context(), updates.UpdateRequest{
		Version: version,
		Channel: r.URL.Query().Get("channel"),
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to prepare update plan")
		http.Error(w, "Failed to prepare update plan", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

// HandleListUpdateHistory returns update history
func (h *UpdateHandlers) HandleListUpdateHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.history == nil {
		http.Error(w, "Update history not available", http.StatusServiceUnavailable)
		return
	}

	// Parse query parameters
	filter := updates.HistoryFilter{
		Limit: 50, // Default limit
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		// Parse limit (simple implementation)
		filter.Limit = 50
	}

	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = updates.UpdateStatusType(status)
	}

	entries := h.history.ListEntries(filter)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// HandleGetUpdateHistoryEntry returns a specific update history entry
func (h *UpdateHandlers) HandleGetUpdateHistoryEntry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.history == nil {
		http.Error(w, "Update history not available", http.StatusServiceUnavailable)
		return
	}

	// Get event ID from URL path
	eventID := r.URL.Query().Get("id")
	if eventID == "" {
		http.Error(w, "event ID required", http.StatusBadRequest)
		return
	}

	entry, err := h.history.GetEntry(eventID)
	if err != nil {
		http.Error(w, "Entry not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}
