package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// DockerMetadataHandler handles Docker resource metadata operations
type DockerMetadataHandler struct {
	store *config.DockerMetadataStore
}

// NewDockerMetadataHandler creates a new Docker metadata handler
func NewDockerMetadataHandler(dataPath string) *DockerMetadataHandler {
	return &DockerMetadataHandler{
		store: config.NewDockerMetadataStore(dataPath),
	}
}

// HandleGetMetadata retrieves metadata for a specific Docker resource or all resources
func (h *DockerMetadataHandler) HandleGetMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if requesting specific resource
	path := r.URL.Path
	// Handle both /api/docker/metadata and /api/docker/metadata/
	if path == "/api/docker/metadata" || path == "/api/docker/metadata/" {
		// Get all metadata
		w.Header().Set("Content-Type", "application/json")
		allMeta := h.store.GetAll()
		if allMeta == nil {
			// Return empty object instead of null
			json.NewEncoder(w).Encode(make(map[string]*config.DockerMetadata))
		} else {
			json.NewEncoder(w).Encode(allMeta)
		}
		return
	}

	// Get specific resource ID from path
	resourceID := strings.TrimPrefix(path, "/api/docker/metadata/")

	w.Header().Set("Content-Type", "application/json")

	if resourceID != "" {
		// Get specific Docker resource metadata
		meta := h.store.Get(resourceID)
		if meta == nil {
			// Return empty metadata instead of 404
			json.NewEncoder(w).Encode(&config.DockerMetadata{ID: resourceID})
		} else {
			json.NewEncoder(w).Encode(meta)
		}
	} else {
		// This shouldn't happen with current routing, but handle it anyway
		http.Error(w, "Invalid request path", http.StatusBadRequest)
	}
}

// HandleUpdateMetadata updates metadata for a Docker resource
func (h *DockerMetadataHandler) HandleUpdateMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resourceID := strings.TrimPrefix(r.URL.Path, "/api/docker/metadata/")
	if resourceID == "" || resourceID == "metadata" {
		http.Error(w, "Resource ID required", http.StatusBadRequest)
		return
	}

	// Limit request body to 16KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)

	var meta config.DockerMetadata
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

	if err := h.store.Set(resourceID, &meta); err != nil {
		log.Error().Err(err).Str("resourceID", resourceID).Msg("Failed to save Docker metadata")
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

	log.Info().Str("resourceID", resourceID).Str("url", meta.CustomURL).Msg("Updated Docker metadata")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&meta)
}

// HandleDeleteMetadata removes metadata for a Docker resource
func (h *DockerMetadataHandler) HandleDeleteMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resourceID := strings.TrimPrefix(r.URL.Path, "/api/docker/metadata/")
	if resourceID == "" || resourceID == "metadata" {
		http.Error(w, "Resource ID required", http.StatusBadRequest)
		return
	}

	if err := h.store.Delete(resourceID); err != nil {
		log.Error().Err(err).Str("resourceID", resourceID).Msg("Failed to delete Docker metadata")
		http.Error(w, "Failed to delete metadata", http.StatusInternalServerError)
		return
	}

	log.Info().Str("resourceID", resourceID).Msg("Deleted Docker metadata")

	w.WriteHeader(http.StatusNoContent)
}
