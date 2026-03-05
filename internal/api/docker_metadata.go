package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

const (
	dockerRuntimeMetadataCollectionPath = "/api/docker/runtimes/metadata"
	dockerRuntimeMetadataPathPrefix     = "/api/docker/runtimes/metadata/"
)

// DockerMetadataHandler handles Docker resource metadata operations
type DockerMetadataHandler struct {
	mtPersistence *config.MultiTenantPersistence
}

// NewDockerMetadataHandler creates a new Docker metadata handler
func NewDockerMetadataHandler(mtPersistence *config.MultiTenantPersistence) *DockerMetadataHandler {
	return &DockerMetadataHandler{
		mtPersistence: mtPersistence,
	}
}

func (h *DockerMetadataHandler) getStore(ctx context.Context) *config.DockerMetadataStore {
	orgID := "default"
	if ctx != nil {
		if requestOrgID := GetOrgID(ctx); requestOrgID != "" {
			orgID = requestOrgID
		}
	}
	p, _ := h.mtPersistence.GetPersistence(orgID)
	return p.GetDockerMetadataStore()
}

// Store returns the underlying metadata store for default tenant
func (h *DockerMetadataHandler) Store() *config.DockerMetadataStore {
	return h.getStore(context.Background())
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
		store := h.getStore(r.Context())
		allMeta := store.GetAll()
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
		store := h.getStore(r.Context())
		meta := store.Get(resourceID)
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
	if errMsg := validateCustomURL(meta.CustomURL); errMsg != "" {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	store := h.getStore(r.Context())
	if err := store.Set(resourceID, &meta); err != nil {
		log.Error().Err(err).Str("resourceID", resourceID).Msg("Failed to save Docker metadata")
		http.Error(w, metadataSaveErrorMessage(err), http.StatusInternalServerError)
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

	store := h.getStore(r.Context())
	if err := store.Delete(resourceID); err != nil {
		log.Error().Err(err).Str("resourceID", resourceID).Msg("Failed to delete Docker metadata")
		http.Error(w, "Failed to delete metadata", http.StatusInternalServerError)
		return
	}

	log.Info().Str("resourceID", resourceID).Msg("Deleted Docker metadata")

	w.WriteHeader(http.StatusNoContent)
}

// HandleGetRuntimeMetadata retrieves metadata for a Docker runtime or all runtimes.
func (h *DockerMetadataHandler) HandleGetRuntimeMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if requesting a specific runtime.
	path := r.URL.Path
	if path == dockerRuntimeMetadataCollectionPath || path == dockerRuntimeMetadataCollectionPath+"/" {
		// Get all runtime metadata.
		w.Header().Set("Content-Type", "application/json")
		store := h.getStore(r.Context())
		allMeta := store.GetAllHostMetadata()
		if allMeta == nil {
			// Return empty object instead of null.
			json.NewEncoder(w).Encode(make(map[string]*config.DockerHostMetadata))
		} else {
			json.NewEncoder(w).Encode(allMeta)
		}
		return
	}

	// Get specific runtime ID from path.
	runtimeID := strings.TrimPrefix(path, dockerRuntimeMetadataPathPrefix)

	w.Header().Set("Content-Type", "application/json")

	if runtimeID != "" {
		// Get specific runtime metadata.
		store := h.getStore(r.Context())
		meta := store.GetHostMetadata(runtimeID)
		if meta == nil {
			// Return empty metadata instead of 404.
			json.NewEncoder(w).Encode(&config.DockerHostMetadata{})
		} else {
			json.NewEncoder(w).Encode(meta)
		}
	} else {
		// This shouldn't happen with current routing, but handle it anyway.
		http.Error(w, "Invalid request path", http.StatusBadRequest)
	}
}

// HandleUpdateRuntimeMetadata updates metadata for a container runtime.
func (h *DockerMetadataHandler) HandleUpdateRuntimeMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	runtimeID := strings.TrimPrefix(r.URL.Path, dockerRuntimeMetadataPathPrefix)
	if runtimeID == "" || runtimeID == "metadata" {
		http.Error(w, "Container runtime ID required", http.StatusBadRequest)
		return
	}

	// Limit request body to 16KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)

	var meta config.DockerHostMetadata
	if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate URL if provided
	if errMsg := validateCustomURL(meta.CustomURL); errMsg != "" {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	// Get existing metadata to merge with new data
	store := h.getStore(r.Context())
	existing := store.GetHostMetadata(runtimeID)
	if existing != nil {
		// Merge: only update fields that are provided
		if meta.CustomDisplayName != "" || existing.CustomDisplayName != "" {
			if meta.CustomDisplayName == "" {
				meta.CustomDisplayName = existing.CustomDisplayName
			}
		}
		// CustomURL can be explicitly cleared, so we don't merge it unless updating
		if meta.Notes == nil && existing.Notes != nil {
			meta.Notes = existing.Notes
		}
	}

	if err := store.SetHostMetadata(runtimeID, &meta); err != nil {
		log.Error().Err(err).Str("runtimeID", runtimeID).Msg("Failed to save container runtime metadata")
		http.Error(w, metadataSaveErrorMessage(err), http.StatusInternalServerError)
		return
	}

	log.Info().Str("runtimeID", runtimeID).Str("url", meta.CustomURL).Msg("Updated container runtime metadata")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&meta)
}

// HandleDeleteRuntimeMetadata removes metadata for a container runtime.
func (h *DockerMetadataHandler) HandleDeleteRuntimeMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	runtimeID := strings.TrimPrefix(r.URL.Path, dockerRuntimeMetadataPathPrefix)
	if runtimeID == "" || runtimeID == "metadata" {
		http.Error(w, "Container runtime ID required", http.StatusBadRequest)
		return
	}
	store := h.getStore(r.Context())
	if err := store.SetHostMetadata(runtimeID, nil); err != nil {
		log.Error().Err(err).Str("runtimeID", runtimeID).Msg("Failed to delete container runtime metadata")
		http.Error(w, "Failed to delete metadata", http.StatusInternalServerError)
		return
	}

	log.Info().Str("runtimeID", runtimeID).Msg("Deleted container runtime metadata")

	w.WriteHeader(http.StatusNoContent)
}
