package api

import (
	"encoding/json"
	"net/http"
	"net/url"
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

// Store returns the underlying metadata store
func (h *GuestMetadataHandler) Store() *config.GuestMetadataStore {
	return h.store
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

	// Limit request body to 16KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)

	var meta config.GuestMetadata
	if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate URL if provided
	if meta.CustomURL != "" {
		// Parse and validate the URL
		parsedURL, err := url.Parse(meta.CustomURL)
		if err != nil {
			http.Error(w, "Invalid URL format: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Check scheme
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			http.Error(w, "URL must use http:// or https:// scheme", http.StatusBadRequest)
			return
		}

		// Check host is present and valid
		if parsedURL.Host == "" {
			http.Error(w, "Invalid URL: missing host/domain (e.g., use https://192.168.1.100:8006 or https://emby.local)", http.StatusBadRequest)
			return
		}

		// Check for incomplete URLs like "https://emby."
		if strings.HasSuffix(parsedURL.Host, ".") && !strings.Contains(parsedURL.Host, "..") {
			http.Error(w, "Incomplete URL: '"+meta.CustomURL+"' - please enter a complete domain or IP address", http.StatusBadRequest)
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
