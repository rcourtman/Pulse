package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// GuestMetadataHandler handles guest metadata operations
type GuestMetadataHandler struct {
	mtPersistence *config.MultiTenantPersistence
	storeResolver func(context.Context) *config.GuestMetadataStore
}

// NewGuestMetadataHandler creates a new guest metadata handler
func NewGuestMetadataHandler(mtPersistence *config.MultiTenantPersistence) *GuestMetadataHandler {
	return &GuestMetadataHandler{
		mtPersistence: mtPersistence,
	}
}

// SetStoreResolver makes API reads and writes use the active monitor's store.
// The persistence-backed store remains the initialization/test fallback.
func (h *GuestMetadataHandler) SetStoreResolver(
	resolver func(context.Context) *config.GuestMetadataStore,
) {
	h.storeResolver = resolver
}

func (h *GuestMetadataHandler) getStore(ctx context.Context) *config.GuestMetadataStore {
	if h != nil && h.storeResolver != nil {
		if store := h.storeResolver(ctx); store != nil {
			return store
		}
	}
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

// Reload reloads the request tenant's guest metadata from disk.
func (h *GuestMetadataHandler) Reload(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return h.getStore(ctx).Load()
}

// Store returns the underlying metadata store for the default tenant (Legacy support)
func (h *GuestMetadataHandler) Store() *config.GuestMetadataStore {
	return h.getStore(context.Background())
}

// HandleGetMetadata retrieves metadata for a specific guest or all guests
func (h *GuestMetadataHandler) HandleGetMetadata(w http.ResponseWriter, r *http.Request) {
	handleMetadataGetRequest(w, r, "/api/guests/metadata",
		func(ctx context.Context) map[string]*config.GuestMetadata { return h.getStore(ctx).GetAll() },
		func(ctx context.Context, id string) *config.GuestMetadata { return h.getStore(ctx).Get(id) },
		func(id string) *config.GuestMetadata { return &config.GuestMetadata{ID: id} },
	)
}

// HandleUpdateMetadata updates metadata for a guest
func (h *GuestMetadataHandler) HandleUpdateMetadata(w http.ResponseWriter, r *http.Request) {
	handleMetadataUpdateRequest(w, r, "/api/guests/metadata",
		"Guest ID required",
		"guestID",
		"Failed to save guest metadata",
		"Updated guest metadata",
		func(meta *config.GuestMetadata) string { return meta.CustomURL },
		func(ctx context.Context, id string, meta *config.GuestMetadata) error {
			return h.getStore(ctx).Set(id, meta)
		},
	)
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
