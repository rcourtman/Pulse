package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// GuestMetadataHandler handles guest metadata operations
type GuestMetadataHandler struct {
	store *config.GuestMetadataStore
}

// NewGuestMetadataHandler creates a new guest metadata handler
func NewGuestMetadataHandler(dataPath string) *GuestMetadataHandler {
	return &GuestMetadataHandler{
		store: config.NewGuestMetadataStore(dataPath),
	}
}

// Reload reloads the guest metadata from disk
func (h *GuestMetadataHandler) Reload() error {
	return h.store.Load()
}

// HandleGetMetadata retrieves metadata for a specific guest or all guests
func (h *GuestMetadataHandler) HandleGetMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if requesting specific guest
	path := r.URL.Path
	// Handle both /api/guests/metadata and /api/guests/metadata/ 
	if path == "/api/guests/metadata" || path == "/api/guests/metadata/" {
		// Get all metadata
		w.Header().Set("Content-Type", "application/json")
		allMeta := h.store.GetAll()
		if allMeta == nil {
			// Return empty object instead of null
			json.NewEncoder(w).Encode(make(map[string]*config.GuestMetadata))
		} else {
			json.NewEncoder(w).Encode(allMeta)
		}
		return
	}
	
	// Get specific guest ID from path
	guestID := strings.TrimPrefix(path, "/api/guests/metadata/")
	
	w.Header().Set("Content-Type", "application/json")
	
	if guestID != "" {
		// Get specific guest metadata
		meta := h.store.Get(guestID)
		if meta == nil {
			// Return empty metadata instead of 404
			json.NewEncoder(w).Encode(&config.GuestMetadata{ID: guestID})
		} else {
			json.NewEncoder(w).Encode(meta)
		}
	} else {
		// This shouldn't happen with current routing, but handle it anyway
		http.Error(w, "Invalid request path", http.StatusBadRequest)
	}
}

// HandleUpdateMetadata updates metadata for a guest
func (h *GuestMetadataHandler) HandleUpdateMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	guestID := strings.TrimPrefix(r.URL.Path, "/api/guests/metadata/")
	if guestID == "" || guestID == "metadata" {
		http.Error(w, "Guest ID required", http.StatusBadRequest)
		return
	}

	var meta config.GuestMetadata
	if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate URL if provided
	if meta.CustomURL != "" {
		// Basic URL validation - just check it starts with http:// or https://
		if !strings.HasPrefix(meta.CustomURL, "http://") && !strings.HasPrefix(meta.CustomURL, "https://") {
			http.Error(w, "Custom URL must start with http:// or https://", http.StatusBadRequest)
			return
		}
	}

	if err := h.store.Set(guestID, &meta); err != nil {
		log.Error().Err(err).Str("guestID", guestID).Msg("Failed to save guest metadata")
		// Provide more specific error message
		errMsg := "Failed to save metadata"
		if strings.Contains(err.Error(), "permission") {
			errMsg = "Permission denied - check file permissions"
		} else if strings.Contains(err.Error(), "no space") {
			errMsg = "Disk full - cannot save metadata"
		}
		http.Error(w, errMsg, http.StatusInternalServerError)
		return
	}

	log.Info().Str("guestID", guestID).Str("url", meta.CustomURL).Msg("Updated guest metadata")
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&meta)
}

// HandleDeleteMetadata removes metadata for a guest
func (h *GuestMetadataHandler) HandleDeleteMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	guestID := strings.TrimPrefix(r.URL.Path, "/api/guests/metadata/")
	if guestID == "" || guestID == "metadata" {
		http.Error(w, "Guest ID required", http.StatusBadRequest)
		return
	}

	if err := h.store.Delete(guestID); err != nil {
		log.Error().Err(err).Str("guestID", guestID).Msg("Failed to delete guest metadata")
		http.Error(w, "Failed to delete metadata", http.StatusInternalServerError)
		return
	}

	log.Info().Str("guestID", guestID).Msg("Deleted guest metadata")
	
	w.WriteHeader(http.StatusNoContent)
}