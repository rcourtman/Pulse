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
	hostMetadataAgentBasePath = "/api/agents/metadata"
)

func hostMetadataPathParts(path string) (agentID string, isCollection bool, ok bool) {
	switch {
	case path == hostMetadataAgentBasePath || path == hostMetadataAgentBasePath+"/":
		return "", true, true
	case strings.HasPrefix(path, hostMetadataAgentBasePath+"/"):
		return strings.TrimPrefix(path, hostMetadataAgentBasePath+"/"), false, true
	default:
		return "", false, false
	}
}

// HostMetadataHandler handles agent metadata operations.
type HostMetadataHandler struct {
	mtPersistence *config.MultiTenantPersistence
	storeResolver func(context.Context) *config.HostMetadataStore
}

// NewHostMetadataHandler creates a new host metadata handler
func NewHostMetadataHandler(mtPersistence *config.MultiTenantPersistence) *HostMetadataHandler {
	return &HostMetadataHandler{
		mtPersistence: mtPersistence,
	}
}

// SetStoreResolver makes API reads and writes use the active monitor's store.
// The persistence-backed store remains the initialization/test fallback.
func (h *HostMetadataHandler) SetStoreResolver(
	resolver func(context.Context) *config.HostMetadataStore,
) {
	h.storeResolver = resolver
}

func (h *HostMetadataHandler) getStore(ctx context.Context) *config.HostMetadataStore {
	if h != nil && h.storeResolver != nil {
		if store := h.storeResolver(ctx); store != nil {
			return store
		}
	}
	orgID := "default"
	if ctx != nil {
		if requestOrgID := GetOrgID(ctx); requestOrgID != "" {
			orgID = requestOrgID
		}
	}
	p, _ := h.mtPersistence.GetPersistence(orgID)
	return p.GetHostMetadataStore()
}

// Store returns the underlying metadata store for default tenant
func (h *HostMetadataHandler) Store() *config.HostMetadataStore {
	return h.getStore(context.Background())
}

// HandleGetMetadata retrieves metadata for a specific agent or all agents.
func (h *HostMetadataHandler) HandleGetMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hostID, isCollection, ok := hostMetadataPathParts(r.URL.Path)
	if !ok {
		http.Error(w, "Invalid request path", http.StatusBadRequest)
		return
	}

	if isCollection {
		// Get all metadata.
		w.Header().Set("Content-Type", "application/json")
		store := h.getStore(r.Context())
		allMeta := store.GetAll()
		if allMeta == nil {
			// Return empty object instead of null
			json.NewEncoder(w).Encode(make(map[string]*config.HostMetadata))
		} else {
			json.NewEncoder(w).Encode(allMeta)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if hostID != "" {
		// Get specific agent metadata.
		store := h.getStore(r.Context())
		meta := store.Get(hostID)
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

// HandleUpdateMetadata updates metadata for an agent.
func (h *HostMetadataHandler) HandleUpdateMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hostID, isCollection, ok := hostMetadataPathParts(r.URL.Path)
	if !ok || isCollection || hostID == "" || hostID == "metadata" {
		http.Error(w, "Agent ID required", http.StatusBadRequest)
		return
	}

	var incoming config.HostMetadata
	fields, decoded := decodeBoundedMetadataPatch(w, r, &incoming)
	if !decoded {
		return
	}

	store := h.getStore(r.Context())
	meta := store.Get(hostID)
	if meta == nil {
		meta = &config.HostMetadata{ID: hostID}
	}
	meta.ID = hostID
	if metadataPatchHasField(fields, "customUrl") {
		if errMsg := validateCustomURL(incoming.CustomURL); errMsg != "" {
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}
		meta.CustomURL = incoming.CustomURL
	}
	if metadataPatchHasField(fields, "description") {
		meta.Description = incoming.Description
	}
	if metadataPatchHasField(fields, "tags") {
		meta.Tags = cloneStringSlice(incoming.Tags)
	}
	if metadataPatchHasField(fields, "notes") {
		meta.Notes = cloneStringSlice(incoming.Notes)
	}
	if metadataPatchHasField(fields, "commandsEnabled") {
		meta.CommandsEnabled = incoming.CommandsEnabled
	}

	if err := store.Set(hostID, meta); err != nil {
		log.Error().Err(err).Str("hostID", hostID).Msg("Failed to save host metadata")
		http.Error(w, metadataSaveErrorMessage(err), http.StatusInternalServerError)
		return
	}

	log.Info().Str("hostID", hostID).Str("url", meta.CustomURL).Msg("Updated host metadata")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
}

// HandleDeleteMetadata removes metadata for an agent.
func (h *HostMetadataHandler) HandleDeleteMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hostID, isCollection, ok := hostMetadataPathParts(r.URL.Path)
	if !ok || isCollection || hostID == "" || hostID == "metadata" {
		http.Error(w, "Agent ID required", http.StatusBadRequest)
		return
	}
	store := h.getStore(r.Context())
	if err := store.Delete(hostID); err != nil {
		log.Error().Err(err).Str("hostID", hostID).Msg("Failed to delete host metadata")
		http.Error(w, "Failed to delete metadata", http.StatusInternalServerError)
		return
	}

	log.Info().Str("hostID", hostID).Msg("Deleted host metadata")

	w.WriteHeader(http.StatusNoContent)
}
