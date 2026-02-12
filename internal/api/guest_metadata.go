package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// GuestMetadataHandler handles guest metadata operations
type GuestMetadataHandler struct {
	mtPersistence *config.MultiTenantPersistence
}

// NewGuestMetadataHandler creates a new guest metadata handler
func NewGuestMetadataHandler(mtPersistence *config.MultiTenantPersistence) *GuestMetadataHandler {
	return &GuestMetadataHandler{
		mtPersistence: mtPersistence,
	}
}

func (h *GuestMetadataHandler) getStore(ctx context.Context) *config.GuestMetadataStore {
	// Default to "default" org if none specified (though middleware should always set it)
	orgID := "default"
	if ctx != nil {
		if requestOrgID := GetOrgID(ctx); requestOrgID != "" {
			orgID = requestOrgID
		}
	}
	p, _ := h.mtPersistence.GetPersistence(orgID)
	return p.GetGuestMetadataStore()
}

// Reload reloads the guest metadata from disk
func (h *GuestMetadataHandler) Reload() error {
	// For multi-tenant, we might need to reload all loaded stores?
	// Or we just rely on lazy loading.
	// Since stores are cached in ConfigPersistence, we currently don't have an easy way to iterate all.
	// But stores load on init. Reload() method on store might be needed if modified on disk externally.
	// For now, this is a no-op or TODO for multi-tenant deep reload.
	// Actually, we can get "default" store and reload it for legacy compat.
	return h.getStore(context.Background()).Load()
}

// Store returns the underlying metadata store for the default tenant (Legacy support)
func (h *GuestMetadataHandler) Store() *config.GuestMetadataStore {
	return h.getStore(context.Background())
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
		store := h.getStore(r.Context())
		allMeta := store.GetAll()
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
		store := h.getStore(r.Context())
		meta := store.Get(guestID)
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

	store := h.getStore(r.Context())
	if err := store.Set(guestID, &meta); err != nil {
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

	store := h.getStore(r.Context())
	if err := store.Delete(guestID); err != nil {
		log.Error().Err(err).Str("guestID", guestID).Msg("Failed to delete guest metadata")
		http.Error(w, "Failed to delete metadata", http.StatusInternalServerError)
		return
	}

	log.Info().Str("guestID", guestID).Msg("Deleted guest metadata")

	w.WriteHeader(http.StatusNoContent)
}
