package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// HostMetadataHandler handles host metadata operations
type HostMetadataHandler struct {
	store *config.HostMetadataStore
}

// NewHostMetadataHandler creates a new host metadata handler
func NewHostMetadataHandler(dataPath string) *HostMetadataHandler {
	return &HostMetadataHandler{
		store: config.NewHostMetadataStore(dataPath, nil),
	}
}

// Store returns the underlying metadata store
func (h *HostMetadataHandler) Store() *config.HostMetadataStore {
	return h.store
}

// HandleGetMetadata retrieves metadata for a specific host or all hosts
func (h *HostMetadataHandler) HandleGetMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if requesting specific host
	path := r.URL.Path
	// Handle both /api/hosts/metadata and /api/hosts/metadata/
	if path == "/api/hosts/metadata" || path == "/api/hosts/metadata/" {
		// Get all metadata
		w.Header().Set("Content-Type", "application/json")
		allMeta := h.store.GetAll()
		if allMeta == nil {
			// Return empty object instead of null
			json.NewEncoder(w).Encode(make(map[string]*config.HostMetadata))
		} else {
			json.NewEncoder(w).Encode(allMeta)
		}
		return
	}

	// Get specific host ID from path
	hostID := strings.TrimPrefix(path, "/api/hosts/metadata/")

	w.Header().Set("Content-Type", "application/json")

	if hostID != "" {
		// Get specific host metadata
		meta := h.store.Get(hostID)
		if meta == nil {
			// Return empty metadata instead of 404
			json.NewEncoder(w).Encode(&config.HostMetadata{ID: hostID})
		} else {
			json.NewEncoder(w).Encode(meta)
		}
	} else {
		// This shouldn't happen with current routing, but handle it anyway
		http.Error(w, "Invalid request path", http.StatusBadRequest)
	}
}

// HandleUpdateMetadata updates metadata for a host
func (h *HostMetadataHandler) HandleUpdateMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hostID := strings.TrimPrefix(r.URL.Path, "/api/hosts/metadata/")
	if hostID == "" || hostID == "metadata" {
		http.Error(w, "Host ID required", http.StatusBadRequest)
		return
	}

	// Limit request body to 16KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)

	var meta config.HostMetadata
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
			http.Error(w, "Invalid URL: missing host/domain (e.g., use https://192.168.1.100:8006 or https://myhost.local)", http.StatusBadRequest)
			return
		}

		// Check for incomplete URLs like "https://host."
		if strings.HasSuffix(parsedURL.Host, ".") && !strings.Contains(parsedURL.Host, "..") {
			http.Error(w, "Incomplete URL: '"+meta.CustomURL+"' - please enter a complete domain or IP address", http.StatusBadRequest)
			return
		}
	}

	if err := h.store.Set(hostID, &meta); err != nil {
		log.Error().Err(err).Str("hostID", hostID).Msg("Failed to save host metadata")
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

	log.Info().Str("hostID", hostID).Str("url", meta.CustomURL).Msg("Updated host metadata")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&meta)
}

// HandleDeleteMetadata removes metadata for a host
func (h *HostMetadataHandler) HandleDeleteMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hostID := strings.TrimPrefix(r.URL.Path, "/api/hosts/metadata/")
	if hostID == "" || hostID == "metadata" {
		http.Error(w, "Host ID required", http.StatusBadRequest)
		return
	}

	if err := h.store.Delete(hostID); err != nil {
		log.Error().Err(err).Str("hostID", hostID).Msg("Failed to delete host metadata")
		http.Error(w, "Failed to delete metadata", http.StatusInternalServerError)
		return
	}

	log.Info().Str("hostID", hostID).Msg("Deleted host metadata")

	w.WriteHeader(http.StatusNoContent)
}
