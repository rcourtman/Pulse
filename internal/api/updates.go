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
	manager *updates.Manager
}

// NewUpdateHandlers creates new update handlers
func NewUpdateHandlers(manager *updates.Manager) *UpdateHandlers {
	return &UpdateHandlers{
		manager: manager,
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
